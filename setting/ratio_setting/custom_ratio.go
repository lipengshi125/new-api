package ratio_setting

import (
	"github.com/QuantumNous/new-api/types"
)

// customRatiosMap stores custom ratios: model -> (param_name -> (param_value -> ratio))
var customRatiosMap = types.NewSyncMap[string, map[string]map[string]float64]()

// GetCustomRatios returns the custom ratios for a given model
func GetCustomRatios(modelName string) map[string]map[string]float64 {
	modelName = FormatMatchingModelName(modelName)
	ratios, ok := customRatiosMap.Get(modelName)
	if !ok {
		return nil
	}
	return ratios
}

// UpdateCustomRatiosByJSONString updates custom ratios from JSON string
func UpdateCustomRatiosByJSONString(jsonStr string) error {
	return types.LoadFromJsonStringWithCallback(customRatiosMap, jsonStr, InvalidateExposedDataCache)
}
