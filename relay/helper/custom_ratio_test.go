package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyCustomRatios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	manyImages := `"u"` + strings.Repeat(`,"u"`, MaxCustomRatioItemCount+10)

	tests := []struct {
		name     string
		body     string
		ratios   map[string]map[string]float64
		expected map[string]float64
	}{
		{
			name:     "exact value match",
			body:     `{"resolution":"4k"}`,
			ratios:   map[string]map[string]float64{"resolution": {"1k": 1, "4k": 1.5}},
			expected: map[string]float64{"resolution": 1.5},
		},
		{
			name:     "exact value takes precedence over wildcard",
			body:     `{"resolution":"4k"}`,
			ratios:   map[string]map[string]float64{"resolution": {"4k": 1.5, "*": 3}},
			expected: map[string]float64{"resolution": 1.5},
		},
		{
			name:     "wildcard matches any non-empty value",
			body:     `{"images":["https://a.png"]}`,
			ratios:   map[string]map[string]float64{"images": {"*": 2}},
			expected: map[string]float64{"images": 2},
		},
		{
			name:     "wildcard ignores empty array",
			body:     `{"images":[]}`,
			ratios:   map[string]map[string]float64{"images": {"*": 2}},
			expected: nil,
		},
		{
			name:     "wildcard ignores empty string and null",
			body:     `{"a":"","b":null}`,
			ratios:   map[string]map[string]float64{"a": {"*": 2}, "b": {"*": 2}},
			expected: nil,
		},
		{
			name:     "per item multiplies by array length",
			body:     `{"images":["https://a.png","https://b.png","https://c.png"]}`,
			ratios:   map[string]map[string]float64{"images": {"*count": 2}},
			expected: map[string]float64{"images": 6},
		},
		{
			name:     "per item counts scalar as one",
			body:     `{"image":"https://a.png"}`,
			ratios:   map[string]map[string]float64{"image": {"*count": 2}},
			expected: map[string]float64{"image": 2},
		},
		{
			name:     "per item takes precedence over wildcard",
			body:     `{"images":["a","b"]}`,
			ratios:   map[string]map[string]float64{"images": {"*count": 2, "*": 5}},
			expected: map[string]float64{"images": 4},
		},
		{
			name:     "per item count is bounded",
			body:     `{"images":[` + manyImages + `]}`,
			ratios:   map[string]map[string]float64{"images": {"*count": 1}},
			expected: map[string]float64{"images": MaxCustomRatioItemCount},
		},
		{
			name:     "nested path",
			body:     `{"input":{"images":["a","b"]}}`,
			ratios:   map[string]map[string]float64{"input.images": {"*count": 1.5}},
			expected: map[string]float64{"input.images": 3},
		},
		{
			name:     "missing parameter applies nothing",
			body:     `{"prompt":"hi"}`,
			ratios:   map[string]map[string]float64{"images": {"*": 2, "*count": 2}},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(tt.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			info := &relaycommon.RelayInfo{}

			ApplyCustomRatios(ctx, info, tt.ratios)

			assert.Equal(t, tt.expected, info.PriceData.OtherRatios())
		})
	}
}

func TestApplyCustomRatiosRecordsHitsForLog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"resolution":"720p","videos":["a.mp4","b.mp4"],"images":["x","y","z"],"audios":[]}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{}

	ApplyCustomRatios(ctx, info, map[string]map[string]float64{
		"resolution": {"720p": 1.75},
		"videos":     {"*": 1.5},
		"images":     {"*count": 2},
		"audios":     {"*": 3},
	})

	// 按参数名排序；空数组不命中，不出现在日志里
	assert.Equal(t, []billingexpr.ParamMultiplier{
		{Name: "images", Value: "*count", Count: 3, Ratio: 6},
		{Name: "resolution", Value: "720p", Ratio: 1.75},
		{Name: "videos", Value: "*", Count: 2, Ratio: 1.5},
	}, info.CustomRatioHits)

	// 渠道重试再次调用时不会重复累积
	ApplyCustomRatios(ctx, info, map[string]map[string]float64{"videos": {"*": 1.5}})
	assert.Len(t, info.CustomRatioHits, 1)
}

func TestApplyCustomRatiosQueryAndBodyReuse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations?quality=hd", strings.NewReader(`{"prompt":"hi"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{}

	ApplyCustomRatios(ctx, info, map[string]map[string]float64{"quality": {"*": 1.2}})

	assert.Equal(t, map[string]float64{"quality": 1.2}, info.PriceData.OtherRatios())
	body := make([]byte, 64)
	n, _ := ctx.Request.Body.Read(body)
	require.Equal(t, `{"prompt":"hi"}`, string(body[:n]), "request body must stay readable for the upstream adaptor")
}
