package helper

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
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

// 视频任务表达式：0.18/秒 ×（输出秒数 + 参考视频真实时长）× 分辨率倍率
const taskVideoExpr = `tier("base", 180000 * (max(0, min(3600, float(param("duration") ?? 0))) + video_seconds("videos"))) * (param("resolution") == "720p" ? 1.75 : 1) * (param("resolution") == "1080p" ? 3.5 : 1)`

// testMP4DataURL 生成只含 ftyp + moov/mvhd 的最小 MP4 data URL
func testMP4DataURL(seconds uint32) string {
	box := func(boxType string, payload []byte) []byte {
		out := make([]byte, 8, 8+len(payload))
		binary.BigEndian.PutUint32(out, uint32(8+len(payload)))
		copy(out[4:], boxType)
		return append(out, payload...)
	}
	mvhd := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhd[12:], 1000)
	binary.BigEndian.PutUint32(mvhd[16:], seconds*1000)
	binary.BigEndian.PutUint32(mvhd[20:], 0x00010000)
	binary.BigEndian.PutUint16(mvhd[24:], 0x0100)
	binary.BigEndian.PutUint32(mvhd[96:], 2)
	file := append(box("ftyp", []byte("isom\x00\x00\x02\x00isom")), box("moov", box("mvhd", mvhd))...)
	return "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(file)
}

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

	video10s := testMP4DataURL(10)
	video5s := testMP4DataURL(5)

	// 期望额度 = 价格 × QuotaPerUnit × 分组倍率
	cases := []struct {
		name          string
		body          string
		group         string
		expectedPrice float64
	}{
		{name: "720p output plus probed reference video", body: `{"duration":20,"resolution":"720p","videos":["` + video10s + `"]}`, group: "default", expectedPrice: 0.18 * 1.75 * 30},
		{name: "durations of multiple videos are summed", body: `{"duration":3,"resolution":"480p","videos":["` + video10s + `","` + video5s + `"]}`, group: "default", expectedPrice: 0.18 * 18},
		{name: "no videos bills output seconds only", body: `{"duration":"4","resolution":"480p"}`, group: "default", expectedPrice: 0.18 * 4},
		{name: "empty videos array bills output seconds only", body: `{"duration":4,"resolution":"1080p","videos":[]}`, group: "default", expectedPrice: 0.18 * 3.5 * 4},
		{name: "client supplied video_duration is ignored", body: `{"duration":4,"video_duration":-100}`, group: "default", expectedPrice: 0.18 * 4},
		{name: "group ratio applies", body: `{"duration":4,"resolution":"480p"}`, group: "vip", expectedPrice: 0.18 * 4},
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

func TestModelPriceHelperTieredTaskReusesProbedDurationOnRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loadTaskExprBillingConfig(t)

	body := `{"duration":2,"videos":["` + testMP4DataURL(10) + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx, info := newTaskExprContext(req, "default")

	first, err := ModelPriceHelperTieredTask(ctx, info)
	require.NoError(t, err)
	require.NotNil(t, info.TaskExprBilling)
	assert.Equal(t, 10.0, info.TaskExprBilling.MediaSeconds["videos"])

	// 重试时复用缓存的秒数，不重新探测
	second, err := ModelPriceHelperTieredTask(ctx, info)
	require.NoError(t, err)
	assert.Equal(t, first.Quota, second.Quota)
}

func TestModelPriceHelperTieredTaskBillingBreakdown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loadTaskExprBillingConfig(t)

	body := `{"duration":20,"resolution":"720p","videos":["` + testMP4DataURL(10) + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx, info := newTaskExprContext(req, "default")

	_, err := ModelPriceHelperTieredTask(ctx, info)
	require.NoError(t, err)
	require.NotNil(t, info.TaskExprBilling)
	assert.True(t, info.TaskExprBilling.HasInputSeconds)
	assert.Equal(t, 10.0, info.TaskExprBilling.InputSeconds)
	assert.Equal(t, 20.0, info.TaskExprBilling.OutputSeconds)
	require.Len(t, info.TaskExprBilling.Multipliers, 1)
	assert.Equal(t, "resolution", info.TaskExprBilling.Multipliers[0].Name)
	assert.Equal(t, "720p", info.TaskExprBilling.Multipliers[0].Value)
	assert.Equal(t, 1.75, info.TaskExprBilling.Multipliers[0].Ratio)
}

func TestModelPriceHelperTieredTaskMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loadTaskExprBillingConfig(t)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("duration", "4"))
	require.NoError(t, writer.WriteField("resolution", "720p"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/videos", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(1<<20))
	ctx, info := newTaskExprContext(req, "default")

	priceData, err := ModelPriceHelperTieredTask(ctx, info)
	require.NoError(t, err)
	assert.Equal(t, common.QuotaRound(0.18*1.75*4*common.QuotaPerUnit), priceData.Quota)
}

func TestModelPriceHelperTieredTaskVideoDurationLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})
	exprJSON, err := common.Marshal(map[string]string{
		"task-video-limited": `tier("base", 27000 * (max(0, min(30, float(param("duration") ?? 0))) + video_seconds("videos", 15)))`,
	})
	require.NoError(t, err)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    `{"task-video-limited":"tiered_expr"}`,
		"billing_setting.billing_expr":    string(exprJSON),
		"group_ratio_setting.group_ratio": `{"default":1}`,
	}))

	run := func(videos ...string) (int, error) {
		body := `{"duration":3,"videos":["` + strings.Join(videos, `","`) + `"]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx, info := newTaskExprContext(req, "default")
		info.OriginModelName = "task-video-limited"
		priceData, err := ModelPriceHelperTieredTask(ctx, info)
		return priceData.Quota, err
	}

	t.Run("total at the limit is accepted", func(t *testing.T) {
		quota, err := run(testMP4DataURL(10), testMP4DataURL(5))
		require.NoError(t, err)
		assert.Equal(t, common.QuotaRound(0.027*(3+15)*common.QuotaPerUnit), quota)
	})

	t.Run("total over the limit is rejected before any charge", func(t *testing.T) {
		quota, err := run(testMP4DataURL(10), testMP4DataURL(8))
		var limitErr *VideoDurationLimitError
		require.ErrorAs(t, err, &limitErr)
		assert.Equal(t, 18.0, limitErr.Seconds)
		assert.Equal(t, 15.0, limitErr.MaxSeconds)
		assert.Zero(t, quota, "no quota must be computed for a rejected request")
		assert.Contains(t, err.Error(), "15")
	})

	t.Run("single long video is rejected", func(t *testing.T) {
		_, err := run(testMP4DataURL(16))
		var limitErr *VideoDurationLimitError
		require.ErrorAs(t, err, &limitErr)
	})
}

func TestModelPriceHelperTieredTaskFixedPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	// fixed(0.15) 与旧写法 150000 必须扣相同额度，任务模型同样可用
	exprJSON, err := common.Marshal(map[string]string{
		"task-image-fixed":  `tier("base", fixed(0.15)) * (param("resolution") == "4k" ? 1.6 : 1)`,
		"task-image-legacy": `tier("base", 150000) * (param("resolution") == "4k" ? 1.6 : 1)`,
	})
	require.NoError(t, err)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    `{"task-image-fixed":"tiered_expr","task-image-legacy":"tiered_expr"}`,
		"billing_setting.billing_expr":    string(exprJSON),
		"group_ratio_setting.group_ratio": `{"default":1}`,
	}))

	quotaFor := func(modelName, body string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx, info := newTaskExprContext(req, "default")
		info.OriginModelName = modelName
		priceData, err := ModelPriceHelperTieredTask(ctx, info)
		require.NoError(t, err)
		return priceData.Quota
	}

	for _, body := range []string{`{"resolution":"1k"}`, `{"resolution":"4k"}`} {
		fixedQuota := quotaFor("task-image-fixed", body)
		assert.Equal(t, quotaFor("task-image-legacy", body), fixedQuota, body)
	}
	assert.Equal(t, common.QuotaRound(0.15*1.6*common.QuotaPerUnit), quotaFor("task-image-fixed", `{"resolution":"4k"}`))
}

func TestModelPriceHelperTieredTaskRejects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loadTaskExprBillingConfig(t)

	cases := []struct {
		name string
		body string
	}{
		{name: "non-numeric duration", body: `{"duration":"abc"}`},
		{name: "unreadable reference video", body: `{"duration":4,"videos":["data:video/mp4;base64,` + base64.StdEncoding.EncodeToString([]byte("not a video")) + `"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			ctx, info := newTaskExprContext(req, "default")

			_, err := ModelPriceHelperTieredTask(ctx, info)
			require.Error(t, err, "the request must be rejected instead of billed as free")
		})
	}
}
