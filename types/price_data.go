package types

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

type GroupRatioInfo struct {
	GroupRatio        float64
	GroupSpecialRatio float64
	HasSpecialRatio   bool
}

type PriceData struct {
	FreeModel            bool
	ModelPrice           float64
	ModelRatio           float64
	CompletionRatio      float64
	CacheRatio           float64
	CacheCreationRatio   float64
	CacheCreation5mRatio float64
	CacheCreation1hRatio float64
	ImageRatio           float64
	AudioRatio           float64
	AudioCompletionRatio float64
	otherRatios          map[string]float64
	// otherCharges 是附加的固定加价（单位：美元），与 otherRatios 完全独立：
	// 倍率是乘法因子，加价是直接加到基础额度上的固定金额，两者绝不能混用同一条路径，
	// 否则 "+0.5" 会被 OtherRatioMultiplier 之类的乘法逻辑误当成 "×0.5"。
	otherCharges      map[string]float64
	UsePrice          bool
	Quota             int // 按次计费的最终额度（MJ / Task）
	QuotaToPreConsume int // 按量计费的预消耗额度
	GroupRatioInfo    GroupRatioInfo
}

func (p *PriceData) AddOtherRatio(key string, ratio float64) {
	if !isValidOtherRatio(ratio) {
		return
	}
	if p.otherRatios == nil {
		p.otherRatios = make(map[string]float64)
	}
	p.otherRatios[key] = ratio
}

// AddOtherCharge records a flat USD surcharge (e.g. "+0.5 per video uploaded"),
// kept entirely separate from otherRatios so it is never multiplied in by
// OtherRatioMultiplier/ApplyOtherRatiosToFloat/ApplyOtherRatiosToDecimal.
// Rejects non-positive, NaN, and +Inf amounts for the same overflow-safety
// reason AddOtherRatio does.
func (p *PriceData) AddOtherCharge(key string, amountUSD float64) {
	if !isValidOtherRatio(amountUSD) {
		return
	}
	if p.otherCharges == nil {
		p.otherCharges = make(map[string]float64)
	}
	p.otherCharges[key] = amountUSD
}

// RemoveOtherCharge removes a previously recorded flat USD surcharge.
func (p *PriceData) RemoveOtherCharge(key string) {
	if p.otherCharges == nil {
		return
	}
	delete(p.otherCharges, key)
}

// ReplaceOtherCharges resets all flat USD surcharges to the given map (same
// shape/semantics as ReplaceOtherRatios, used when replaying a task's stored
// billing context on retry/settlement). Returns true when at least one valid
// charge survived.
func (p *PriceData) ReplaceOtherCharges(charges map[string]float64) bool {
	p.otherCharges = nil
	for key, amountUSD := range charges {
		p.AddOtherCharge(key, amountUSD)
	}
	return len(p.otherCharges) > 0
}

// OtherCharges returns a defensive copy of the recorded flat USD surcharges,
// or nil if none are set.
func (p *PriceData) OtherCharges() map[string]float64 {
	if len(p.otherCharges) == 0 {
		return nil
	}
	charges := make(map[string]float64, len(p.otherCharges))
	for key, amountUSD := range p.otherCharges {
		if isValidOtherRatio(amountUSD) {
			charges[key] = amountUSD
		}
	}
	if len(charges) == 0 {
		return nil
	}
	return charges
}

// OtherChargesTotalUSD sums all recorded flat USD surcharges.
func (p *PriceData) OtherChargesTotalUSD() float64 {
	total := 0.0
	for _, amountUSD := range p.otherCharges {
		if isValidOtherRatio(amountUSD) {
			total += amountUSD
		}
	}
	return total
}

func (p *PriceData) RemoveOtherRatio(key string) {
	if p.otherRatios == nil {
		return
	}
	delete(p.otherRatios, key)
}

func (p *PriceData) ReplaceOtherRatios(ratios map[string]float64) bool {
	p.otherRatios = nil
	for key, ratio := range ratios {
		p.AddOtherRatio(key, ratio)
	}
	return len(p.otherRatios) > 0
}

func (p *PriceData) HasOtherRatio(key string) bool {
	ratio, ok := p.otherRatios[key]
	return ok && isValidOtherRatio(ratio)
}

func (p *PriceData) OtherRatios() map[string]float64 {
	if len(p.otherRatios) == 0 {
		return nil
	}
	ratios := make(map[string]float64, len(p.otherRatios))
	for key, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) {
			ratios[key] = ratio
		}
	}
	if len(ratios) == 0 {
		return nil
	}
	return ratios
}

func (p *PriceData) OtherRatioMultiplier() float64 {
	multiplier := 1.0
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			multiplier *= ratio
		}
	}
	return multiplier
}

func (p *PriceData) ApplyOtherRatiosToFloat(value float64) float64 {
	return value * p.OtherRatioMultiplier()
}

func (p *PriceData) ApplyOtherRatiosToDecimal(value decimal.Decimal) decimal.Decimal {
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			value = value.Mul(decimal.NewFromFloat(ratio))
		}
	}
	return value
}

func (p *PriceData) RemoveOtherRatiosFromFloat(value float64) float64 {
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			value /= ratio
		}
	}
	return value
}

func isValidOtherRatio(ratio float64) bool {
	// NaN/Inf would poison every downstream quota multiplication
	// (int(NaN * quota) wraps to a negative charge).
	return ratio > 0 && !math.IsInf(ratio, 1)
}

func (p *PriceData) ToSetting() string {
	return fmt.Sprintf("ModelPrice: %f, ModelRatio: %f, CompletionRatio: %f, CacheRatio: %f, GroupRatio: %f, UsePrice: %t, CacheCreationRatio: %f, CacheCreation5mRatio: %f, CacheCreation1hRatio: %f, QuotaToPreConsume: %d, ImageRatio: %f, AudioRatio: %f, AudioCompletionRatio: %f", p.ModelPrice, p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.GroupRatioInfo.GroupRatio, p.UsePrice, p.CacheCreationRatio, p.CacheCreation5mRatio, p.CacheCreation1hRatio, p.QuotaToPreConsume, p.ImageRatio, p.AudioRatio, p.AudioCompletionRatio)
}
