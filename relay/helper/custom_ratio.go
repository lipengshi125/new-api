package helper

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"sort"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// 自定义倍率中参数值的保留匹配模式（与前端 model-pricing 编辑器保持一致）
const (
	// CustomRatioMatchAny 参数存在且非空时（任意值）应用倍率
	CustomRatioMatchAny = "*"
	// CustomRatioMatchPerItem 按参数中的元素个数计费：倍率 = 配置倍率 × 个数
	// 数组取长度，非空标量计为 1
	CustomRatioMatchPerItem = "*count"
)

// MaxCustomRatioItemCount 按个数计费时的个数上限，防止超大数组把倍率放大到异常值
const MaxCustomRatioItemCount = 128

// customRatioParam 是从请求中提取出的单个参数
type customRatioParam struct {
	value string // 用于精确匹配的字符串形式
	count int    // 非空元素个数：数组为长度，非空标量为 1，空值/null 为 0
}

// ApplyCustomRatios 从请求中提取自定义参数并应用对应的倍率或固定加价配置。
// customRatios 格式: { param_name -> { param_value -> ratio } }
// customCharges 格式: { param_name -> { param_value -> charge_usd } }
// 例如倍率: { "resolution" -> { "1k": 1.0, "2k": 1.2, "4k": 1.5 } }
// 例如加价: { "videos" -> { "*count": 0.5 } } — 每个视频 URL +$0.50
// param_name 支持点号路径访问 JSON 嵌套字段（如 "input.images"）。
// param_value 除精确值外还支持保留模式：
//   - "*"      参数存在且非空时应用倍率/加价
//   - "*count" 倍率 = ratio × 元素个数；加价 = charge_usd × 元素个数
//
// 优先级：精确值 > "*count" > "*"。
// 注意：同一参数名不应同时出现在 customRatios 和 customCharges 中（admin UI 强制互斥）。
func ApplyCustomRatios(c *gin.Context, info *relaycommon.RelayInfo, customRatios, customCharges map[string]map[string]float64) {
	if len(customRatios) == 0 && len(customCharges) == 0 {
		return
	}

	bodyBytes := readReusableBody(c)
	// 渠道重试会再次调用，先清空上一次的命中记录
	info.CustomRatioHits = nil

	// 合并参数名集合：同一参数名可能同时出现在 ratios 和 charges 中（虽然 UI 应该禁止），
	// 但这里仍然独立处理，优先倍率、回退加价，避免意外重复计费。
	allParams := make(map[string]bool)
	for paramName := range customRatios {
		allParams[paramName] = true
	}
	for paramName := range customCharges {
		allParams[paramName] = true
	}

	for paramName := range allParams {
		param, exists := lookupCustomRatioParam(c, bodyBytes, paramName)
		if !exists {
			continue
		}

		valueRatioMap := customRatios[paramName]
		valueChargeMap := customCharges[paramName]

		// 命中记录：精确值记实际值；任意值/按个数记保留值和元素个数
		hit := billingexpr.ParamMultiplier{Name: paramName}
		matched := false

		// 优先级 1: 精确值匹配（倍率优先，加价回退）
		if ratio, ok := valueRatioMap[param.value]; ok {
			hit.Value, hit.Ratio = param.value, ratio
			info.PriceData.AddOtherRatio(paramName, hit.Ratio)
			matched = true
		} else if chargeUSD, ok := valueChargeMap[param.value]; ok {
			hit.Value, hit.ChargeUSD = param.value, chargeUSD
			info.PriceData.AddOtherCharge(paramName, hit.ChargeUSD)
			matched = true
		} else if param.count == 0 {
			// 参数为空或 null，不匹配任何模式
			continue
		} else if ratio, ok := valueRatioMap[CustomRatioMatchPerItem]; ok {
			// 优先级 2: "*count" 倍率模式
			count := min(param.count, MaxCustomRatioItemCount)
			hit.Value, hit.Count, hit.Ratio = CustomRatioMatchPerItem, count, ratio*float64(count)
			info.PriceData.AddOtherRatio(paramName, hit.Ratio)
			matched = true
		} else if chargeUSD, ok := valueChargeMap[CustomRatioMatchPerItem]; ok {
			// 优先级 2: "*count" 加价模式（每个元素 +chargeUSD）
			count := min(param.count, MaxCustomRatioItemCount)
			totalCharge := chargeUSD * float64(count)
			hit.Value, hit.Count, hit.ChargeUSD = CustomRatioMatchPerItem, count, totalCharge
			info.PriceData.AddOtherCharge(paramName, totalCharge)
			matched = true
		} else if ratio, ok := valueRatioMap[CustomRatioMatchAny]; ok {
			// 优先级 3: "*" 倍率模式
			hit.Value, hit.Count, hit.Ratio = CustomRatioMatchAny, param.count, ratio
			info.PriceData.AddOtherRatio(paramName, hit.Ratio)
			matched = true
		} else if chargeUSD, ok := valueChargeMap[CustomRatioMatchAny]; ok {
			// 优先级 3: "*" 加价模式（固定一次性加价）
			hit.Value, hit.Count, hit.ChargeUSD = CustomRatioMatchAny, param.count, chargeUSD
			info.PriceData.AddOtherCharge(paramName, chargeUSD)
			matched = true
		}

		if matched {
			info.CustomRatioHits = append(info.CustomRatioHits, hit)
			if hit.Ratio > 0 {
				logger.LogDebug(c.Request.Context(), fmt.Sprintf("Applied custom ratio: %s=%s (count=%d) -> ×%.2f", paramName, hit.Value, hit.Count, hit.Ratio))
			} else if hit.ChargeUSD > 0 {
				logger.LogDebug(c.Request.Context(), fmt.Sprintf("Applied custom charge: %s=%s (count=%d) -> +$%.2f", paramName, hit.Value, hit.Count, hit.ChargeUSD))
			}
		}
	}
	sort.Slice(info.CustomRatioHits, func(i, j int) bool {
		return info.CustomRatioHits[i].Name < info.CustomRatioHits[j].Name
	})
}

// readReusableBody 读取原始 body 并恢复，供后续流程继续使用
func readReusableBody(c *gin.Context) []byte {
	if c.Request.Body == nil {
		return nil
	}
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return bodyBytes
}

// lookupCustomRatioParam 按 JSON body > form-data > query 的顺序查找参数
func lookupCustomRatioParam(c *gin.Context, bodyBytes []byte, paramName string) (customRatioParam, bool) {
	if param, ok := lookupFromJSONBody(bodyBytes, paramName); ok {
		return param, true
	}
	if c.Request.MultipartForm != nil {
		if param, ok := lookupFromFormData(c.Request.MultipartForm, paramName); ok {
			return param, true
		}
	}
	return lookupFromQuery(c.Request.URL.Query(), paramName)
}

// lookupFromJSONBody 从 JSON body 中提取参数，paramName 支持点号嵌套路径
func lookupFromJSONBody(bodyBytes []byte, paramName string) (customRatioParam, bool) {
	if len(bodyBytes) == 0 || !gjson.ValidBytes(bodyBytes) {
		return customRatioParam{}, false
	}
	result := gjson.GetBytes(bodyBytes, paramName)
	if !result.Exists() {
		return customRatioParam{}, false
	}

	param := customRatioParam{value: fmt.Sprintf("%v", result.Value())}
	switch {
	case result.IsArray():
		param.count = len(result.Array())
	case result.IsObject():
		if len(result.Map()) > 0 {
			param.count = 1
		}
	case result.Type == gjson.Null:
		param.count = 0
	case result.Type == gjson.String:
		if result.Str != "" {
			param.count = 1
		}
	default:
		param.count = 1
	}
	return param, true
}

// lookupFromFormData 从 multipart form-data 中提取参数，文件字段同样计入个数
func lookupFromFormData(form *multipart.Form, paramName string) (customRatioParam, bool) {
	values := form.Value[paramName]
	files := form.File[paramName]
	if len(values) == 0 && len(files) == 0 {
		return customRatioParam{}, false
	}

	param := customRatioParam{count: len(files)}
	if len(values) > 0 {
		param.value = values[0]
	}
	for _, v := range values {
		if v != "" {
			param.count++
		}
	}
	return param, true
}

// lookupFromQuery 从 URL query 参数中提取
func lookupFromQuery(query url.Values, paramName string) (customRatioParam, bool) {
	values, ok := query[paramName]
	if !ok || len(values) == 0 {
		return customRatioParam{}, false
	}
	param := customRatioParam{value: values[0]}
	for _, v := range values {
		if v != "" {
			param.count++
		}
	}
	return param, true
}
