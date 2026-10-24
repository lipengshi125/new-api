package billingexpr

import (
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/tidwall/gjson"
)

// ParamMultiplier 描述一个请求参数对计费的倍率影响，用于日志展示。
type ParamMultiplier struct {
	Name  string  `json:"name"`
	Value string  `json:"value,omitempty"` // 标量参数的值
	Count int     `json:"count,omitempty"` // 数组参数的元素个数
	Ratio float64 `json:"ratio"`
}

// ExplainParamMultipliers 解析表达式顶层的乘法因子，找出形如
// (param("x") == "v" ? 1.75 : 1) 的条件倍率，并按本次请求求值，
// 返回每个参数实际生效的倍率（同一参数的多个因子相乘）。
// 未命中的参数记为 ×1，使"参数值 → 倍率"在日志中完整可见。
// 仅用于展示：求值失败的因子会被忽略，不影响计费结果。
func ExplainParamMultipliers(exprStr string, request RequestInput) []ParamMultiplier {
	prog, err := CompileFromCache(exprStr)
	if err != nil {
		return nil
	}
	version, _ := ParseExprVersion(exprStr)
	compileEnv := getCompileEnv(version)
	runtimeEnv := newRuntimeEnv(TokenParams{}, request, &TraceResult{})

	var factors []ast.Node
	collectMultiplyFactors(prog.Node(), &factors)

	var order []string
	ratios := make(map[string]float64)
	for _, factor := range factors {
		cond, ok := factor.(*ast.ConditionalNode)
		if !ok {
			continue
		}
		name := firstParamPath(cond.Cond)
		if name == "" {
			continue
		}
		matched, ok := evalNode(cond.Cond, compileEnv, runtimeEnv).(bool)
		if !ok {
			continue
		}
		branch := cond.Exp2
		if matched {
			branch = cond.Exp1
		}
		ratio, ok := toFloat(evalNode(branch, compileEnv, runtimeEnv))
		if !ok {
			continue
		}
		if _, seen := ratios[name]; !seen {
			order = append(order, name)
			ratios[name] = 1
		}
		ratios[name] *= ratio
	}

	result := make([]ParamMultiplier, 0, len(order))
	for _, name := range order {
		item := ParamMultiplier{Name: name, Ratio: ratios[name]}
		value := gjson.GetBytes(request.Body, name)
		if value.IsArray() {
			item.Count = len(value.Array())
		} else if value.Exists() {
			item.Value = value.String()
		}
		result = append(result, item)
	}
	return result
}

// ExplainUnitPrice 从表达式的 tier 叶子读出基础单价（美元，不含参数倍率与分组倍率），
// 用于在日志中展示「基础价格」。识别的写法：
//   - tier("x", fixed(A))           → 按次，单价 A
//   - tier("x", K)                  → 按次，单价 K/1e6
//   - tier("x", K * (秒数项...))    → 按秒，单价 K/1e6
//
// 只看结构不求值；识别不了（如多档位或 token 计费）时返回 ok=false。
func ExplainUnitPrice(exprStr string) (unitPrice float64, unit string, ok bool) {
	tree, err := parseExprTree(exprStr)
	if err != nil {
		return 0, "", false
	}
	var factors []ast.Node
	collectMultiplyFactors(tree, &factors)

	var leaf *ast.CallNode
	for _, factor := range factors {
		call, isCall := factor.(*ast.CallNode)
		if !isCall {
			continue
		}
		callee, isIdent := call.Callee.(*ast.IdentifierNode)
		if !isIdent || callee.Value != "tier" || len(call.Arguments) != 2 {
			continue
		}
		if leaf != nil {
			return 0, "", false // 多个 tier 叶子，基础价格不唯一
		}
		leaf = call
	}
	if leaf == nil {
		return 0, "", false
	}

	price := leaf.Arguments[1]
	if fixedCall, isCall := price.(*ast.CallNode); isCall {
		if callee, isIdent := fixedCall.Callee.(*ast.IdentifierNode); isIdent && callee.Value == "fixed" && len(fixedCall.Arguments) == 1 {
			if amount, literal := numberLiteral(fixedCall.Arguments[0]); literal {
				return amount, "request", true
			}
		}
		return 0, "", false
	}
	if coef, literal := numberLiteral(price); literal {
		return coef / 1_000_000, "request", true
	}
	if bin, isBin := price.(*ast.BinaryNode); isBin && bin.Operator == "*" {
		coef, literal := numberLiteral(bin.Left)
		if literal && !containsTokenVar(bin.Right) && usesSecondsSource(bin.Right) {
			return coef / 1_000_000, "second", true
		}
	}
	return 0, "", false
}

func containsTokenVar(node ast.Node) bool {
	return ast.Find(node, func(part ast.Node) bool {
		identifier, ok := part.(*ast.IdentifierNode)
		if !ok {
			return false
		}
		switch identifier.Value {
		case "p", "c", "len", "cr", "cc", "cc1h", "img", "img_o", "ai", "ao":
			return true
		}
		return false
	}) != nil
}

// usesSecondsSource 判断秒数部分是否读取了请求参数或视频时长
func usesSecondsSource(node ast.Node) bool {
	return ast.Find(node, func(part ast.Node) bool {
		identifier, ok := part.(*ast.IdentifierNode)
		return ok && (identifier.Value == "param" || identifier.Value == "video_seconds")
	}) != nil
}

func collectMultiplyFactors(node ast.Node, factors *[]ast.Node) {
	if bin, ok := node.(*ast.BinaryNode); ok && bin.Operator == "*" {
		collectMultiplyFactors(bin.Left, factors)
		collectMultiplyFactors(bin.Right, factors)
		return
	}
	*factors = append(*factors, node)
}

// firstParamPath 返回条件中第一个 param("path") 的字面量路径
func firstParamPath(node ast.Node) string {
	path := ""
	ast.Find(node, func(n ast.Node) bool {
		if path != "" {
			return false
		}
		call, ok := n.(*ast.CallNode)
		if !ok || len(call.Arguments) != 1 {
			return false
		}
		callee, ok := call.Callee.(*ast.IdentifierNode)
		if !ok || callee.Value != "param" {
			return false
		}
		if arg, ok := call.Arguments[0].(*ast.StringNode); ok {
			path = strings.TrimSpace(arg.Value)
		}
		return false
	})
	return path
}

func evalNode(node ast.Node, compileEnv, runtimeEnv map[string]interface{}) interface{} {
	prog, err := expr.Compile(node.String(), expr.Env(compileEnv))
	if err != nil {
		return nil
	}
	out, err := expr.Run(prog, runtimeEnv)
	if err != nil {
		return nil
	}
	return out
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
