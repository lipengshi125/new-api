package billingexpr

import (
	"strings"
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

func TestVideoSecondsLimitSpecs(t *testing.T) {
	specs, err := VideoSecondsSpecs(`tier("base", 27000 * (float(param("duration") ?? 0) + video_seconds("videos", 15)))`)
	require.NoError(t, err)
	assert.Equal(t, []VideoSecondsSpec{{Path: "videos", MaxSeconds: 15}}, specs)

	// 不带上限的旧写法保持不变
	specs, err = VideoSecondsSpecs(`tier("base", 27000 * video_seconds("videos"))`)
	require.NoError(t, err)
	assert.Equal(t, []VideoSecondsSpec{{Path: "videos"}}, specs)

	// 两个参数时仍能求值，上限不影响计算结果
	cost, _, err := RunExprWithRequest(`tier("base", 27000 * video_seconds("videos", 15))`, TokenParams{}, RequestInput{MediaSeconds: map[string]float64{"videos": 10}})
	require.NoError(t, err)
	assert.InDelta(t, 270000, cost, 1e-6)

	for _, expression := range []string{
		`tier("base", 27000 * video_seconds("videos", 0))`,
		`tier("base", 27000 * video_seconds("videos", -5))`,
		`tier("base", 27000 * video_seconds("videos", param("max")))`,
		`tier("base", 27000 * video_seconds("videos", 15, 20))`,
		`tier("base", 27000 * video_seconds(param("path")))`,
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

// TestItemCount pins item_count/video_count: the count of non-empty elements at
// any request path (videos, images, audios, nested), usable additively inside a
// tier price and as a request-condition multiplier, capped at MaxItemCount.
func TestItemCount(t *testing.T) {
	body := []byte(`{"videos":["a.mp4","b.mp4"],"images":["x","", null,"z"],"audios":[],"input":{"refs":[{"url":"u"},{}]},"cover":"c.png","many":[` +
		strings.TrimSuffix(strings.Repeat(`"u",`, MaxItemCount+20), ",") + `]}`)
	request := RequestInput{Body: body}

	tests := []struct {
		name string
		expr string
		want float64
	}{
		{"videos array", `item_count("videos")`, 2},
		{"video_count alias", `video_count("videos")`, 2},
		{"images skips empty and null", `item_count("images")`, 2},
		{"empty array", `item_count("audios")`, 0},
		{"missing path", `item_count("masks")`, 0},
		{"nested path skips empty object", `item_count("input.refs")`, 1},
		{"scalar counts as one", `item_count("cover")`, 1},
		{"capped", `item_count("many")`, MaxItemCount},
		// +$0.5 per video on top of a $0.2 base: 0.2*1e6 + 2*0.5*1e6
		{"additive per-item price", `tier("base", 200000 + item_count("videos") * 500000)`, 1_200_000},
		{"combined with resolution multiplier", `tier("base", 200000 + item_count("videos") * 500000) * (param("resolution") == "4k" ? 1.6 : 1)`, 1_200_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, _, err := RunExprWithRequest(tt.expr, TokenParams{}, request)
			require.NoError(t, err)
			assert.InDelta(t, tt.want, cost, 1e-6)
		})
	}

	// item_count 可作为 fixed() 计费的请求条件倍率
	cost, _, err := RunExprWithRequest(`tier("base", fixed(0.2)) * (item_count("videos") > 1 ? 1.5 : 1)`, TokenParams{}, request)
	require.NoError(t, err)
	assert.InDelta(t, 300000, cost, 1e-6)

	for _, expression := range []string{
		`tier("base", 200000 + item_count(param("path")) * 500000)`,
		`tier("base", 200000 + item_count() * 500000)`,
		`tier("base", 200000 + video_count("videos", 3) * 500000)`,
	} {
		t.Run(expression, func(t *testing.T) {
			_, err := CompileFromCache(expression)
			require.Error(t, err)
		})
	}
}
