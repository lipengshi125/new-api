package billingexpr

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/tidwall/gjson"
)

// RunExpr compiles (with cache) and executes an expression string.
// The environment exposes:
//   - p, c             — prompt / completion tokens (auto-excluding separately-priced sub-categories)
//   - len              — total input context length for tier conditions (never reduced by sub-category exclusion)
//   - cr, cc, cc1h     — cache read / creation / creation-1h tokens
//   - tier(name, value) — trace callback that records which tier matched
//   - max, min, abs, ceil, floor — standard math helpers
//
// Returns the resulting float64 quota (before group ratio) and a TraceResult
// with side-channel info captured by tier() during execution.
func RunExpr(exprStr string, params TokenParams) (float64, TraceResult, error) {
	return RunExprWithRequest(exprStr, params, RequestInput{})
}

func RunExprWithRequest(exprStr string, params TokenParams, request RequestInput) (float64, TraceResult, error) {
	prog, err := CompileFromCache(exprStr)
	if err != nil {
		return 0, TraceResult{}, err
	}
	return runProgram(prog, params, request)
}

// RunExprByHash is like RunExpr but accepts a pre-computed hash for the cache
// lookup, avoiding a redundant SHA-256 computation when the caller already
// holds BillingSnapshot.ExprHash.
func RunExprByHash(exprStr, hash string, params TokenParams) (float64, TraceResult, error) {
	return RunExprByHashWithRequest(exprStr, hash, params, RequestInput{})
}

func RunExprByHashWithRequest(exprStr, hash string, params TokenParams, request RequestInput) (float64, TraceResult, error) {
	prog, err := CompileFromCacheByHash(exprStr, hash)
	if err != nil {
		return 0, TraceResult{}, err
	}
	return runProgram(prog, params, request)
}

func runProgram(prog *vm.Program, params TokenParams, request RequestInput) (float64, TraceResult, error) {
	trace := TraceResult{}
	out, err := expr.Run(prog, newRuntimeEnv(params, request, &trace))
	if err != nil {
		return 0, trace, fmt.Errorf("expr run error: %w", err)
	}
	f, ok := out.(float64)
	if !ok {
		return 0, trace, fmt.Errorf("expr result is %T, want float64", out)
	}
	return f, trace, nil
}

// newRuntimeEnv builds the evaluation environment; tier() records into trace.
func newRuntimeEnv(params TokenParams, request RequestInput, trace *TraceResult) map[string]interface{} {
	headers := normalizeHeaders(request.Headers)

	return map[string]interface{}{
		"p":    params.P,
		"c":    params.C,
		"len":  params.Len,
		"cr":   params.CR,
		"cc":   params.CC,
		"cc1h": params.CC1h,
		"img":  params.Img,
		"img_o": params.ImgO,
		"ai":   params.AI,
		"ao":   params.AO,
		"tier": func(name string, value float64) float64 {
			trace.MatchedTier = name
			trace.Cost = value
			return value
		},
		// fixed(amount)：每次固定价格（美元），换算成与 v1「$/1M」一致的系数，
		// 因此 fixed(0.16) 与 160000 等价
		"fixed": func(amount float64) float64 {
			return amount * 1_000_000
		},
		"header": func(key string) string {
			return headers[strings.ToLower(strings.TrimSpace(key))]
		},
		"param": func(path string) interface{} {
			path = strings.TrimSpace(path)
			if path == "" || len(request.Body) == 0 {
				return nil
			}
			result := gjson.GetBytes(request.Body, path)
			if !result.Exists() {
				return nil
			}
			return result.Value()
		},
		// video_seconds(path[, max])：上限由调用方在探测时校验（超出则拒绝请求），
		// 这里只返回已探测的总秒数
		"video_seconds": func(path string, _ ...float64) float64 {
			return request.MediaSeconds[strings.TrimSpace(path)]
		},
		// item_count(path)：请求体 path 处的非空元素个数（数组取非空元素数，
		// 非空标量计 1，缺失/空/null 为 0），封顶 MaxItemCount 防止超大数组放大计费。
		// video_count 是同义别名，path 可以是 videos、images、audios 或嵌套路径。
		"item_count":  func(path string) float64 { return requestItemCount(request.Body, path) },
		"video_count": func(path string) float64 { return requestItemCount(request.Body, path) },
		"has": func(source interface{}, substr string) bool {
			if source == nil || substr == "" {
				return false
			}
			return strings.Contains(fmt.Sprint(source), substr)
		},
		"hour":    func(tz string) int { return timeInZone(tz).Hour() },
		"minute":  func(tz string) int { return timeInZone(tz).Minute() },
		"weekday": func(tz string) int { return int(timeInZone(tz).Weekday()) },
		"month":   func(tz string) int { return int(timeInZone(tz).Month()) },
		"day":     func(tz string) int { return timeInZone(tz).Day() },
		"max":     math.Max,
		"min":   math.Min,
		"abs":   math.Abs,
		"ceil":  math.Ceil,
		"floor": math.Floor,
	}
}

// MaxItemCount 是 item_count/video_count 的计数上限（与自定义倍率「按个数」一致）。
const MaxItemCount = 128

func requestItemCount(body []byte, path string) float64 {
	path = strings.TrimSpace(path)
	if path == "" || len(body) == 0 {
		return 0
	}
	result := gjson.GetBytes(body, path)
	count := 0
	switch {
	case !result.Exists(), result.Type == gjson.Null:
		return 0
	case result.IsArray():
		for _, item := range result.Array() {
			if item.Type == gjson.Null {
				continue
			}
			if item.Type == gjson.String && strings.TrimSpace(item.Str) == "" {
				continue
			}
			if item.IsObject() && len(item.Map()) == 0 {
				continue
			}
			count++
		}
	case result.Type == gjson.String:
		if strings.TrimSpace(result.Str) != "" {
			count = 1
		}
	case result.IsObject():
		if len(result.Map()) > 0 {
			count = 1
		}
	default:
		count = 1
	}
	return float64(min(count, MaxItemCount))
}

func timeInZone(tz string) time.Time {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return time.Now().UTC()
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Now().UTC()
	}
	return time.Now().In(loc)
}

func normalizeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	normalized := make(map[string]string, len(headers))
	for key, value := range headers {
		k := strings.ToLower(strings.TrimSpace(key))
		v := strings.TrimSpace(value)
		if k == "" || v == "" {
			continue
		}
		normalized[k] = v
	}
	return normalized
}
