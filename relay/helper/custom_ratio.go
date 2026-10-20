package helper

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"

	"github.com/QuantumNous/new-api/logger"
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

// ApplyCustomRatios 从请求中提取自定义参数并应用对应的倍率配置
// customRatios 格式: { param_name -> { param_value -> ratio } }
// 例如: { "resolution" -> { "1k": 1.0, "2k": 1.2, "4k": 1.5 } }
// param_name 支持点号路径访问 JSON 嵌套字段（如 "input.images"）。
// param_value 除精确值外还支持保留模式：
//   - "*"      参数存在且非空时应用倍率
//   - "*count" 倍率 = ratio × 元素个数（如 images 中每个 url 计一次）
//
// 优先级：精确值 > "*count" > "*"。
func ApplyCustomRatios(c *gin.Context, info *relaycommon.RelayInfo, customRatios map[string]map[string]float64) {
	if len(customRatios) == 0 {
		return
	}

	bodyBytes := readReusableBody(c)

	for paramName, valueRatioMap := range customRatios {
		param, exists := lookupCustomRatioParam(c, bodyBytes, paramName)
		if !exists {
			continue
		}

		if ratio, ok := valueRatioMap[param.value]; ok {
			info.PriceData.AddOtherRatio(paramName, ratio)
			logger.LogDebug(c.Request.Context(), fmt.Sprintf("Applied custom ratio: %s=%s -> x%.2f", paramName, param.value, ratio))
			continue
		}

		if param.count == 0 {
			continue
		}

		if ratio, ok := valueRatioMap[CustomRatioMatchPerItem]; ok {
			count := min(param.count, MaxCustomRatioItemCount)
			info.PriceData.AddOtherRatio(paramName, ratio*float64(count))
			logger.LogDebug(c.Request.Context(), fmt.Sprintf("Applied custom ratio: %s count=%d -> x%.2f", paramName, count, ratio*float64(count)))
			continue
		}

		if ratio, ok := valueRatioMap[CustomRatioMatchAny]; ok {
			info.PriceData.AddOtherRatio(paramName, ratio)
			logger.LogDebug(c.Request.Context(), fmt.Sprintf("Applied custom ratio: %s=* -> x%.2f", paramName, ratio))
		}
	}
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
