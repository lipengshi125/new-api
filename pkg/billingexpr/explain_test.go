package billingexpr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExplainParamMultipliers(t *testing.T) {
	const exprStr = `tier("base", 180000 * (max(0, min(3600, float(param("duration") ?? 0))) + video_seconds("videos"))) * (param("resolution") == "720p" ? 1.75 : 1) * (param("resolution") == "1080p" ? 3.5 : 1) * (count(param("images") ?? [], true) > 0 ? 1.2 : 1)`

	tests := []struct {
		name     string
		body     string
		expected []ParamMultiplier
	}{
		{
			name: "matched values and array count",
			body: `{"duration":20,"resolution":"720p","images":["a","b"]}`,
			expected: []ParamMultiplier{
				{Name: "resolution", Value: "720p", Ratio: 1.75},
				{Name: "images", Count: 2, Ratio: 1.2},
			},
		},
		{
			name: "unmatched factors of the same param multiply to 1",
			body: `{"duration":20,"resolution":"480p"}`,
			expected: []ParamMultiplier{
				{Name: "resolution", Value: "480p", Ratio: 1},
				{Name: "images", Ratio: 1},
			},
		},
		{
			name: "second factor of the same param applies",
			body: `{"resolution":"1080p"}`,
			expected: []ParamMultiplier{
				{Name: "resolution", Value: "1080p", Ratio: 3.5},
				{Name: "images", Ratio: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExplainParamMultipliers(exprStr, RequestInput{Body: []byte(tt.body)})
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestExplainParamMultipliersIgnoresTierOnlyExpr(t *testing.T) {
	got := ExplainParamMultipliers(`tier("base", p * 2 + c * 8)`, RequestInput{Body: []byte(`{"resolution":"720p"}`)})
	assert.Empty(t, got)
}
