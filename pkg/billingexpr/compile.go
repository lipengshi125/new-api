package billingexpr

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/vm"
)

const maxCacheSize = 256

// DefaultExprVersion is used when an expression string has no version prefix.
const DefaultExprVersion = 1

// ParseExprVersion extracts the version tag and body from an expression string.
// Format: "v1:tier(...)" → version=1, body="tier(...)".
// No prefix defaults to DefaultExprVersion.
func ParseExprVersion(exprStr string) (version int, body string) {
	if strings.HasPrefix(exprStr, "v1:") {
		return 1, exprStr[3:]
	}
	return DefaultExprVersion, exprStr
}

type cachedEntry struct {
	prog     *vm.Program
	usedVars map[string]bool
	version  int
}

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]*cachedEntry, 64)
)

// compileEnvPrototypeV1 is the v1 type-checking prototype used at compile time.
var compileEnvPrototypeV1 = map[string]interface{}{
	"p":    float64(0),
	"c":    float64(0),
	"len":  float64(0),
	"cr":   float64(0),
	"cc":   float64(0),
	"cc1h": float64(0),
	"img":  float64(0),
	"img_o": float64(0),
	"ai":   float64(0),
	"ao":   float64(0),
	"tier":                   func(string, float64) float64 { return 0 },
	"fixed":                  func(float64) float64 { return 0 },
	"header":                 func(string) string { return "" },
	"param":                  func(string) interface{} { return nil },
	"has":                    func(interface{}, string) bool { return false },
	"video_seconds":          func(string, ...float64) float64 { return 0 },
	"hour":                   func(string) int { return 0 },
	"minute":                 func(string) int { return 0 },
	"weekday":                func(string) int { return 0 },
	"month":                  func(string) int { return 0 },
	"day":                    func(string) int { return 0 },
	"max":                    math.Max,
	"min":                    math.Min,
	"abs":                    math.Abs,
	"ceil":                   math.Ceil,
	"floor":                  math.Floor,
}

// parseExprTree parses the expression body (version prefix stripped) into an
// AST without type checking or optimization.
func parseExprTree(exprStr string) (ast.Node, error) {
	_, body := ParseExprVersion(exprStr)
	tree, err := parser.Parse(body)
	if err != nil {
		return nil, err
	}
	return tree.Node, nil
}

func getCompileEnv(version int) map[string]interface{} {
	switch version {
	default:
		return compileEnvPrototypeV1
	}
}

// CompileFromCache compiles an expression string, using a cached program when
// available. The cache is keyed by the SHA-256 hex digest of the expression.
func CompileFromCache(exprStr string) (*vm.Program, error) {
	return compileFromCacheByHash(exprStr, ExprHashString(exprStr))
}

// CompileFromCacheByHash is like CompileFromCache but accepts a pre-computed
// hash, useful when the caller already has the BillingSnapshot.ExprHash.
func CompileFromCacheByHash(exprStr, hash string) (*vm.Program, error) {
	return compileFromCacheByHash(exprStr, hash)
}

func compileFromCacheByHash(exprStr, hash string) (*vm.Program, error) {
	cacheMu.RLock()
	if entry, ok := cache[hash]; ok {
		cacheMu.RUnlock()
		return entry.prog, nil
	}
	cacheMu.RUnlock()

	version, body := ParseExprVersion(exprStr)
	// 在编译优化之前校验 fixed()，防止不可达分支里的非法 fixed 绕过校验
	tree, err := parseExprTree(exprStr)
	if err != nil {
		return nil, fmt.Errorf("expr compile error: %w", err)
	}
	if containsFixed(tree) {
		if err := validateFixedPricingTree(tree); err != nil {
			return nil, fmt.Errorf("expr compile error: %w", err)
		}
	}
	if err := validateVideoSecondsCalls(tree); err != nil {
		return nil, fmt.Errorf("expr compile error: %w", err)
	}
	prog, err := expr.Compile(body, expr.Env(getCompileEnv(version)), expr.AsFloat64())
	if err != nil {
		return nil, fmt.Errorf("expr compile error: %w", err)
	}

	vars := extractUsedVars(prog)

	cacheMu.Lock()
	if len(cache) >= maxCacheSize {
		cache = make(map[string]*cachedEntry, 64)
	}
	cache[hash] = &cachedEntry{prog: prog, usedVars: vars, version: version}
	cacheMu.Unlock()

	return prog, nil
}

// ExprVersion returns the version of a cached expression. Returns DefaultExprVersion
// if the expression hasn't been compiled yet or is empty.
func ExprVersion(exprStr string) int {
	if exprStr == "" {
		return DefaultExprVersion
	}
	hash := ExprHashString(exprStr)
	cacheMu.RLock()
	if entry, ok := cache[hash]; ok {
		cacheMu.RUnlock()
		return entry.version
	}
	cacheMu.RUnlock()
	v, _ := ParseExprVersion(exprStr)
	return v
}

func extractUsedVars(prog *vm.Program) map[string]bool {
	vars := make(map[string]bool)
	node := prog.Node()
	ast.Find(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.IdentifierNode); ok {
			vars[id.Value] = true
		}
		return false
	})
	return vars
}

// validateVideoSecondsCalls 校验 video_seconds(path[, max])：path 必须是字符串字面量，
// max 若提供必须是正数字面量，且最多两个参数。
func validateVideoSecondsCalls(tree ast.Node) error {
	var err error
	ast.Find(tree, func(n ast.Node) bool {
		call, ok := n.(*ast.CallNode)
		if !ok || err != nil {
			return false
		}
		callee, ok := call.Callee.(*ast.IdentifierNode)
		if !ok || callee.Value != "video_seconds" {
			return false
		}
		if len(call.Arguments) < 1 || len(call.Arguments) > 2 {
			err = fmt.Errorf("video_seconds takes a path and an optional max seconds")
			return false
		}
		if _, isString := call.Arguments[0].(*ast.StringNode); !isString {
			err = fmt.Errorf("video_seconds path must be a string literal")
			return false
		}
		if len(call.Arguments) == 2 {
			limit, literal := numberLiteral(call.Arguments[1])
			if !literal || limit <= 0 || math.IsNaN(limit) || math.IsInf(limit, 0) {
				err = fmt.Errorf("video_seconds max seconds must be a positive numeric literal")
			}
		}
		return false
	})
	return err
}

// VideoSecondsSpec 是表达式中一次 video_seconds(path[, max]) 的调用：
// MaxSeconds > 0 表示参考视频总时长上限，超出时拒绝请求。
type VideoSecondsSpec struct {
	Path       string
	MaxSeconds float64
}

// VideoSecondsSpecs returns the literal request paths (and optional duration
// limits) passed to video_seconds() in an expression, so callers know which
// media URLs to probe and which limits to enforce before evaluation.
// The same path used with different limits keeps the strictest one.
func VideoSecondsSpecs(exprStr string) ([]VideoSecondsSpec, error) {
	tree, err := parseExprTree(exprStr)
	if err != nil {
		return nil, err
	}
	if _, err := CompileFromCache(exprStr); err != nil {
		return nil, err
	}
	index := make(map[string]int)
	var specs []VideoSecondsSpec
	var specErr error
	ast.Find(tree, func(n ast.Node) bool {
		call, ok := n.(*ast.CallNode)
		if !ok || specErr != nil {
			return false
		}
		callee, ok := call.Callee.(*ast.IdentifierNode)
		if !ok || callee.Value != "video_seconds" || len(call.Arguments) == 0 {
			return false
		}
		arg, ok := call.Arguments[0].(*ast.StringNode)
		if !ok {
			return false
		}
		path := strings.TrimSpace(arg.Value)
		if path == "" {
			return false
		}
		maxSeconds := 0.0
		if len(call.Arguments) >= 2 {
			limit, literal := numberLiteral(call.Arguments[1])
			if !literal || limit <= 0 || math.IsNaN(limit) || math.IsInf(limit, 0) {
				specErr = fmt.Errorf("video_seconds limit must be a positive numeric literal")
				return false
			}
			maxSeconds = limit
		}
		if i, seen := index[path]; seen {
			if maxSeconds > 0 && (specs[i].MaxSeconds == 0 || maxSeconds < specs[i].MaxSeconds) {
				specs[i].MaxSeconds = maxSeconds
			}
			return false
		}
		index[path] = len(specs)
		specs = append(specs, VideoSecondsSpec{Path: path, MaxSeconds: maxSeconds})
		return false
	})
	if specErr != nil {
		return nil, specErr
	}
	return specs, nil
}

// VideoSecondsPaths returns only the paths of VideoSecondsSpecs.
func VideoSecondsPaths(exprStr string) ([]string, error) {
	specs, err := VideoSecondsSpecs(exprStr)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(specs))
	for _, spec := range specs {
		paths = append(paths, spec.Path)
	}
	return paths, nil
}

// UsedVars returns the set of identifier names referenced by an expression.
// The result is cached alongside the compiled program. Returns nil for empty input.
func UsedVars(exprStr string) map[string]bool {
	if exprStr == "" {
		return nil
	}
	hash := ExprHashString(exprStr)
	cacheMu.RLock()
	if entry, ok := cache[hash]; ok {
		cacheMu.RUnlock()
		return entry.usedVars
	}
	cacheMu.RUnlock()

	// Compile (and cache) to populate usedVars
	if _, err := compileFromCacheByHash(exprStr, hash); err != nil {
		return nil
	}
	cacheMu.RLock()
	entry, ok := cache[hash]
	cacheMu.RUnlock()
	if ok {
		return entry.usedVars
	}
	return nil
}

// InvalidateCache clears the compiled-expression cache.
// Called when billing rules are updated.
func InvalidateCache() {
	cacheMu.Lock()
	cache = make(map[string]*cachedEntry, 64)
	cacheMu.Unlock()
}
