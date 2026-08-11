package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRemoveOtherRatioDropsMultiplier guards the per-count (按次) billing
// invariant: once a duration multiplier is removed, it must no longer affect
// the applied quota. Per-second video adaptors add "seconds" as an OtherRatio,
// and per-count models strip it so the request's seconds value cannot inflate
// the per-call price (e.g. seconds:15 must not turn a x1 price into x15).
func TestRemoveOtherRatioDropsMultiplier(t *testing.T) {
	p := &PriceData{}
	p.AddOtherRatio("seconds", 15)
	p.AddOtherRatio("size", 1.666667)

	assert.InDelta(t, 15.0*1.666667, p.OtherRatioMultiplier(), 1e-9)

	p.RemoveOtherRatio("seconds")

	assert.False(t, p.HasOtherRatio("seconds"))
	// Only the size multiplier survives; seconds no longer scales the quota.
	assert.InDelta(t, 1.666667, p.OtherRatioMultiplier(), 1e-9)
	assert.InDelta(t, 100.0*1.666667, p.ApplyOtherRatiosToFloat(100.0), 1e-9)
}

// TestRemoveOtherRatioNilAndMissingKey ensures removal is safe on an empty map
// and a no-op for absent keys.
func TestRemoveOtherRatioNilAndMissingKey(t *testing.T) {
	p := &PriceData{}
	p.RemoveOtherRatio("seconds") // nil map, must not panic

	p.AddOtherRatio("seconds", 3)
	p.RemoveOtherRatio("duration") // absent key, must leave seconds intact

	assert.True(t, p.HasOtherRatio("seconds"))
	assert.Equal(t, 3.0, p.OtherRatioMultiplier())
}
