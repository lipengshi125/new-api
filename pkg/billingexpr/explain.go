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
