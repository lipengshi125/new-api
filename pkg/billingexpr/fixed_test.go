package billingexpr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixedPriceEquivalentToScaledCoefficient(t *testing.T) {
	body := []byte(`{"resolution":"4k"}`)
	for _, tc := range []struct {
		name     string
		fixed    string
		legacy   string
		expected float64
	}{
		{"flat per call", `tier("base", fixed(0.16))`, `tier("base", 160000)`, 160000},
		{"with request multiplier", `tier("base", fixed(0.15)) * (param("resolution") == "4k" ? 1.6 : 1)`, `tier("base", 150000) * (param("resolution") == "4k" ? 1.6 : 1)`, 240000},
		{"wrapped by editor", `(tier("base", fixed(0.15))) * (param("resolution") == "4k" ? 1.6 : 1)`, `(tier("base", 150000)) * (param("resolution") == "4k" ? 1.6 : 1)`, 240000},
		{"explicit free price", `tier("free", fixed(0))`, `tier("free", 0)`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixedCost, trace, err := RunExprWithRequest(tc.fixed, TokenParams{P: 1000, C: 500}, RequestInput{Body: body})
			require.NoError(t, err)
			legacyCost, _, err := RunExprWithRequest(tc.legacy, TokenParams{P: 1000, C: 500}, RequestInput{Body: body})
			require.NoError(t, err)
			assert.InDelta(t, tc.expected, fixedCost, 1e-6)
			assert.InDelta(t, legacyCost, fixedCost, 1e-6)
			assert.NotEmpty(t, trace.MatchedTier)
		})
	}
}

func TestFixedPriceIgnoresTokenUsageAndSupportsConditionalTiers(t *testing.T) {
	const expression = `(len <= 32000 ? tier("short", fixed(0.01)) : tier("long", p * 2 + c * 8)) * (param("fast") == true ? 2 : 1)`
	for _, tc := range []struct {
		name   string
		params TokenParams
		cost   float64
		tier   string
	}{
		{"zero usage still charges once", TokenParams{}, 20000, "short"},
		{"fixed branch ignores token prices", TokenParams{P: 32000, C: 9000, Len: 32000}, 20000, "short"},
		{"token branch uses actual usage", TokenParams{P: 32001, C: 100, Len: 32001}, 129604, "long"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cost, trace, err := RunExprWithRequest(expression, tc.params, RequestInput{Body: []byte(`{"fast":true}`)})
			require.NoError(t, err)
			assert.InDelta(t, tc.cost, cost, 1e-6)
			assert.Equal(t, tc.tier, trace.MatchedTier)
		})
	}
}

func TestFixedPriceRejectsInvalidLeavesIncludingUnselectedBranches(t *testing.T) {
	for _, expression := range []string{
		`true ? tier("ok", p) : tier("bad", fixed(-0.01))`,
		`tier("bad", fixed(p))`,
		`tier("bad", fixed(0.01 + 0.02))`,
		`tier("bad", fixed(1e308))`,
		`tier("bad", fixed(0.01) + p * 2)`,
		`tier("bad", p * fixed(0.01))`,
		`tier("a", fixed(0.01)) + tier("b", p * 2)`,
		`fixed(0.01)`,
		`tier("bad", fixed(0.01)) * p`,
		`tier("bad", fixed(0.01)) * (param("fast") == true ? -2 : 1)`,
		`tier("bad", fixed(0.01)) * (param("fast") == true ? 2 : 0)`,
	} {
		t.Run(expression, func(t *testing.T) {
			_, err := CompileFromCache(expression)
			require.Error(t, err)
		})
	}
}

func TestUsesFixedPricing(t *testing.T) {
	assert.True(t, UsesFixedPricing(`tier("base", fixed(0.16))`))
	// 未选中的分支也算，避免靠编译优化绕过
	assert.True(t, UsesFixedPricing(`true ? tier("tokens", p) : tier("request", fixed(0.01))`))
	assert.False(t, UsesFixedPricing(`tier("base", 160000)`))
	assert.False(t, UsesFixedPricing(`tier("base", p * 2 + c * 8)`))
}
