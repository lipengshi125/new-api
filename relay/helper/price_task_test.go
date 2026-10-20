package helper

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 视频任务表达式：0.18/秒 ×（输出秒数 + 参考视频秒数，各自钳制在 0~3600）× 分辨率倍率
const taskVideoExpr = `tier("base", 180000 * (max(0, min(3600, float(param("duration") ?? 0))) + max(0, min(3600, float(param("video_duration") ?? 0))))) * (param("resolution") == "720px" ? 1.75 : 1) * (param("resolution") == "1080px" ? 3.5 : 1)`

func loadTaskExprBillingConfig(t *testing.T) {
	t.Helper()
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	exprJSON, err := common.Marshal(map[string]string{"task-video-model": taskVideoExpr})
	require.NoError(t, err)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    `{"task-video-model":"tiered_expr"}`,
		"billing_setting.billing_expr":    string(exprJSON),
		"group_ratio_setting.group_ratio": `{"default":1,"vip":0.5}`,
	}))
}

func newTaskExprContext(req *http.Request, group string) (*gin.Context, *relaycommon.RelayInfo) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	ctx.Set("group", group)
	info := &relaycommon.RelayInfo{
		OriginModelName: "task-video-model",
		UserGroup:       group,
		UsingGroup:      group,
		RequestHeaders:  map[string]string{"Content-Type": req.Header.Get("Content-Type")},
	}
	return ctx, info
}

func TestModelPriceHelperTieredTaskJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loadTaskExprBillingConfig(t)

	// 期望额度 = 价格 × QuotaPerUnit × 分组倍率
	cases := []struct {
		name          string
		body          string
		group         string
		expectedPrice float64
	}{
		{name: "720px output plus reference video", body: `{"duration":20,"resolution":"720px","video_duration":10}`, group: "default", expectedPrice: 0.18 * 1.75 * 30},
		{name: "string numbers are accepted", body: `{"duration":"4","resolution":"480px"}`, group: "default", expectedPrice: 0.18 * 4},
		{name: "1080px", body: `{"duration":20,"resolution":"1080px","video_duration":10}`, group: "default", expectedPrice: 0.18 * 3.5 * 30},
		{name: "negative reference duration cannot reduce charge", body: `{"duration":20,"resolution":"720px","video_duration":-100}`, group: "default", expectedPrice: 0.18 * 1.75 * 20},
		{name: "huge reference duration is clamped", body: `{"duration":4,"video_duration":99999}`, group: "default", expectedPrice: 0.18 * (4 + 3600)},
		{name: "group ratio applies", body: `{"duration":4,"resolution":"480px"}`, group: "vip", expectedPrice: 0.18 * 4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			ctx, info := newTaskExprContext(req, tc.group)

			priceData, err := ModelPriceHelperTieredTask(ctx, info)
			require.NoError(t, err)

			groupRatio := map[string]float64{"default": 1, "vip": 0.5}[tc.group]
			expectedQuota := common.QuotaRound(tc.expectedPrice * common.QuotaPerUnit * groupRatio)
			assert.Equal(t, expectedQuota, priceData.Quota)
			assert.Equal(t, expectedQuota, priceData.QuotaToPreConsume)
			assert.InDelta(t, tc.expectedPrice, priceData.ModelPrice, 1e-9)
			assert.True(t, priceData.UsePrice, "expression task billing is final at submit and must skip polling settlement")
			assert.Nil(t, priceData.OtherRatios(), "expression result already includes duration/resolution; no extra multipliers")
		})
	}
}

func TestModelPriceHelperTieredTaskMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loadTaskExprBillingConfig(t)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("duration", "4"))
	require.NoError(t, writer.WriteField("resolution", "720px"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/videos", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(1<<20))
	ctx, info := newTaskExprContext(req, "default")

	priceData, err := ModelPriceHelperTieredTask(ctx, info)
	require.NoError(t, err)
	assert.Equal(t, common.QuotaRound(0.18*1.75*4*common.QuotaPerUnit), priceData.Quota)
}

func TestModelPriceHelperTieredTaskRejectsInvalidParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loadTaskExprBillingConfig(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(`{"duration":4,"video_duration":"abc"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx, info := newTaskExprContext(req, "default")

	_, err := ModelPriceHelperTieredTask(ctx, info)
	require.Error(t, err, "a non-numeric duration must reject the request instead of billing it as free")
}
