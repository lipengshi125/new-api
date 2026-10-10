package perfmetrics

// RequestCountForTest sums the not-yet-flushed requests recorded for a model.
// Tests in other packages use it to assert how many samples a code path
// records, which the success rate alone cannot distinguish (one failure and
// two failures both read as 0%).
func RequestCountForTest(modelName string) int64 {
	var total int64
	hotBuckets.Range(func(key, value any) bool {
		if key.(bucketKey).model == modelName {
			total += value.(*atomicBucket).snapshot().requestCount
		}
		return true
	})
	return total
}
