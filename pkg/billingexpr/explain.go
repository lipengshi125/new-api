package billingexpr

import (
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/tidwall/gjson"
)

// ParamMultiplier 描述一个请求参数对计费的倍率或固定加价影响，用于日志展示。
// Ratio 和 ChargeUSD 互斥：Ratio > 0 时为乘法因子，ChargeUSD > 0 时为固定美元加价。
type ParamMultiplier struct {
	Name      string  `json:"name"`
	Value     string  `json:"value,omitempty"`      // 标量参数的值
	Count     int     `json:"count,omitempty"`      // 数组参数的元素个数
	Ratio     float64 `json:"ratio,omitempty"`      // 倍率（乘法因子），与 ChargeUSD 互斥
	ChargeUSD float64 `json:"charge_usd,omitempty"` // 固定加价（美元），与 Ratio 互斥
}

// ExplainParamMultipliers 在整个表达式中查找形如
// (param("x") == "v" ? 1.75 : 1) 的条件倍率——不论它写在 tier() 外面还是
// tier() 的价格参数里（tier("base", K * (...) + ...)），并按本次请求求值，
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

	// 遍历整棵树：倍率既可以写在 tier() 外（tier(...) * (cond)），
	// 也可以写在 tier() 的价格参数里（tier("base", K * (cond) + ...)）。
	var conditionals []*ast.ConditionalNode
	ast.Find(prog.Node(), func(node ast.Node) bool {
		if cond, ok := node.(*ast.ConditionalNode); ok {
			conditionals = append(conditionals, cond)
		}
		return false
	})

	var order []string
	ratios := make(map[string]float64)
	for _, cond := range conditionals {
		// 只认 (param(...) == 值 ? 倍率 : 1) 这种请求倍率写法，
		// 避免把 tier 的条件分支当成倍率。
		name := firstParamPath(cond.Cond)
		if name == "" {
			continue
		}
		if _, ok := numberLiteral(cond.Exp1); !ok {
			continue
		}
		fallback, ok := numberLiteral(cond.Exp2)
		if !ok || fallback != 1 {
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

// ExplainItemCharges 找出表达式中的 item_count("path") * K 与
// video_count("path") * K（加法计费项），按本次请求统计元素个数，
// 返回每项实际生效的加价总额（USD，ChargeUSD）。
// 与 ExplainParamMultipliers 一样仅用于日志展示，不影响计费结果。
func ExplainItemCharges(exprStr string, request RequestInput) []ParamMultiplier {
	tree, err := parseExprTree(exprStr)
	if err != nil {
		return nil
	}

	var order []string
	totals := make(map[string]float64)
	counts := make(map[string]int)
	ast.Find(tree, func(node ast.Node) bool {
		bin, ok := node.(*ast.BinaryNode)
		if !ok || bin.Operator != "*" {
			return false
		}
		path, perItem, ok := itemChargeFactor(bin.Left, bin.Right)
		if !ok {
			path, perItem, ok = itemChargeFactor(bin.Right, bin.Left)
		}
		if !ok {
			return false
		}
		if _, seen := totals[path]; !seen {
			order = append(order, path)
			count := int(requestItemCount(request.Body, path))
			counts[path] = count
		}
		totals[path] += perItem
		return false
	})

	result := make([]ParamMultiplier, 0, len(order))
	for _, path := range order {
		amount := totals[path] * float64(counts[path])
		if amount <= 0 {
			continue
		}
		result = append(result, ParamMultiplier{
			Name:      path,
			Count:     counts[path],
			ChargeUSD: amount,
		})
	}
	return result
}

// itemChargeFactor 判断 call * coef 是否为 item_count/video_count(path) * K。
func itemChargeFactor(callNode, coefNode ast.Node) (string, float64, bool) {
	call, ok := callNode.(*ast.CallNode)
	if !ok || len(call.Arguments) != 1 {
		return "", 0, false
	}
	callee, ok := call.Callee.(*ast.IdentifierNode)
	if !ok || (callee.Value != "item_count" && callee.Value != "video_count") {
		return "", 0, false
	}
	arg, ok := call.Arguments[0].(*ast.StringNode)
	if !ok {
		return "", 0, false
	}
	path := strings.TrimSpace(arg.Value)
	if path == "" {
		return "", 0, false
	}
	coef, literal := numberLiteral(coefNode)
	if !literal || coef <= 0 {
		return "", 0, false
	}
	return path, coef / 1_000_000, true
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
