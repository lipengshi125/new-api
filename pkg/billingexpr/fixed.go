package billingexpr

import (
	"fmt"
	"math"

	"github.com/expr-lang/expr/ast"
)

// fixed(amount) — a flat per-call price in the expression's currency unit
// (USD), e.g. tier("base", fixed(0.16)). Ported from upstream new-api so
// per-call pricing can be written as a real price instead of the scaled
// coefficient form tier("base", 160000); both forms evaluate to the same cost
// (fixed returns amount * 1e6, matching the v1 "$/1M" quota conversion).
//
// Validation mirrors upstream: every fixed() must be the direct price of a
// tier leaf, its argument must be a finite, non-negative numeric literal, and a
// fixed leaf may only be combined with conditional tiers and request-condition
// multipliers — never with token prices — so each execution has exactly one
// unambiguous per-call charge.

// UsesFixedPricing reports whether an expression contains fixed(), including
// unselected branches.
func UsesFixedPricing(expression string) bool {
	tree, err := parseExprTree(expression)
	return err == nil && containsFixed(tree)
}

func containsFixed(node ast.Node) bool {
	return ast.Find(node, func(part ast.Node) bool {
		identifier, ok := part.(*ast.IdentifierNode)
		return ok && identifier.Value == "fixed"
	}) != nil
}

func containsPricingMarker(node ast.Node) bool {
	return ast.Find(node, func(part ast.Node) bool {
		identifier, ok := part.(*ast.IdentifierNode)
		return ok && (identifier.Value == "tier" || identifier.Value == "fixed")
	}) != nil
}

func numberLiteral(node ast.Node) (float64, bool) {
	switch value := node.(type) {
	case *ast.IntegerNode:
		return float64(value.Value), true
	case *ast.FloatNode:
		return value.Value, true
	default:
		return 0, false
	}
}

func usesRequestProbe(node ast.Node) bool {
	return ast.Find(node, func(part ast.Node) bool {
		identifier, ok := part.(*ast.IdentifierNode)
		if !ok {
			return false
		}
		switch identifier.Value {
		case "param", "header", "hour", "minute", "weekday", "month", "day":
			return true
		default:
			return false
		}
	}) != nil
}

// isRequestPriceMultiplier matches (cond ? r : 1) where cond reads the request
// or time and r is a finite, non-negative literal.
func isRequestPriceMultiplier(node ast.Node) bool {
	conditional, ok := node.(*ast.ConditionalNode)
	if !ok || !usesRequestProbe(conditional.Cond) || containsPricingMarker(conditional.Cond) {
		return false
	}
	multiplier, multiplierOK := numberLiteral(conditional.Exp1)
	fallback, fallbackOK := numberLiteral(conditional.Exp2)
	return multiplierOK && fallbackOK && fallback == 1 &&
		multiplier >= 0 && !math.IsNaN(multiplier) && !math.IsInf(multiplier, 0)
}

func validateFixedPricingTree(node ast.Node) error {
	switch part := node.(type) {
	case *ast.ConditionalNode:
		if containsPricingMarker(part.Cond) {
			break
		}
		if err := validateFixedPricingTree(part.Exp1); err != nil {
			return err
		}
		return validateFixedPricingTree(part.Exp2)
	case *ast.BinaryNode:
		if part.Operator != "*" {
			break
		}
		if isRequestPriceMultiplier(part.Right) {
			return validateFixedPricingTree(part.Left)
		}
		if isRequestPriceMultiplier(part.Left) {
			return validateFixedPricingTree(part.Right)
		}
	case *ast.CallNode:
		callee, ok := part.Callee.(*ast.IdentifierNode)
		if !ok || callee.Value != "tier" || len(part.Arguments) != 2 || containsPricingMarker(part.Arguments[0]) {
			break
		}
		price := part.Arguments[1]
		if fixed, ok := price.(*ast.CallNode); ok {
			function, direct := fixed.Callee.(*ast.IdentifierNode)
			if direct && function.Value == "fixed" && len(fixed.Arguments) == 1 {
				amount, literal := numberLiteral(fixed.Arguments[0])
				if literal && amount >= 0 && !math.IsNaN(amount) && !math.IsInf(amount*1_000_000, 0) {
					return nil
				}
				return fmt.Errorf("fixed price must be a finite, non-negative numeric literal")
			}
		}
		if !containsPricingMarker(price) {
			return nil
		}
	}
	return fmt.Errorf("fixed pricing requires tier(name, fixed(amount)) leaves, conditional tiers and request multipliers; token and fixed charges cannot be combined in one leaf")
}
