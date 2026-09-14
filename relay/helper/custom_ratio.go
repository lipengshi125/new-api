package helper

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// ApplyCustomRatios 从请求中提取自定义参数并应用对应的倍率配置
// customRatios 格式: { param_name -> { param_value -> ratio } }
// 例如: { "resolution" -> { "1k": 1.0, "2k": 1.2, "4k": 1.5 } }
func ApplyCustomRatios(c *gin.Context, info *relaycommon.RelayInfo, customRatios map[string]map[string]float64) {
	if len(customRatios) == 0 {
		return
	}

	// 1. 尝试从 JSON body 提取
	extractedParams := extractFromJSONBody(c)

	// 2. 尝试从 form-data 提取
	if c.Request.MultipartForm != nil {
		formParams := extractFromFormData(c.Request.MultipartForm)
		for k, v := range formParams {
			if _, exists := extractedParams[k]; !exists {
				extractedParams[k] = v
			}
		}
	}

	// 3. 尝试从 query 参数提取
	queryParams := extractFromQuery(c.Request.URL.Query())
	for k, v := range queryParams {
		if _, exists := extractedParams[k]; !exists {
			extractedParams[k] = v
		}
	}

	// 4. 应用匹配的倍率
	for paramName, valueRatioMap := range customRatios {
		if paramValue, exists := extractedParams[paramName]; exists {
			if ratio, ok := valueRatioMap[paramValue]; ok {
				info.PriceData.AddOtherRatio(paramName, ratio)
				logger.LogDebug(c.Request.Context(), fmt.Sprintf("Applied custom ratio: %s=%s -> x%.2f", paramName, paramValue, ratio))
			}
		}
	}
}

// extractFromJSONBody 从 JSON body 中提取参数（一级字段）
func extractFromJSONBody(c *gin.Context) map[string]string {
	result := make(map[string]string)

	// 读取原始 body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return result
	}

	// 恢复 body 供后续使用
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// 解析 JSON
	var bodyMap map[string]interface
	if err := common.Unmarshal(bodyBytes, &bodyMap); err != nil {
		return result
	}

	// 提取字符串/数字类型的一级字段
	for key, value := range bodyMap {
		result[key] = fmt.Sprintf("%v", value)
	}

	return result
}

// extractFromFormData 从 multipart form-data 中提取参数
func extractFromFormData(form *multipart.Form) map[string]string {
	result := make(map[string]string)

	for key, values := range form.Value {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}

	return result
}

// extractFromQuery 从 URL query 参数中提取
func extractFromQuery(query url.Values) map[string]string {
	result := make(map[string]string)

	for key, values := range query {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}

	return result
}
