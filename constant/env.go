package constant

var StreamingTimeout int
var DifyDebug bool
var MaxFileDownloadMB int
var StreamScannerMaxBufferMB int
var ForceStreamOption bool
var CountToken bool
var GetMediaToken bool
var GetMediaTokenNotStream bool
var UpdateTask bool
var MaxRequestBodyMB int
var AnonymousRequestBodyLimitKB int
var AzureDefaultAPIVersion string
var NotifyLimitCount int
var NotificationLimitDurationMinute int
var GenerateDefaultToken bool
var ErrorLogEnabled bool
var TaskQueryLimit int
var TaskTimeoutMinutes int

// TaskPollingIntervalSeconds is how often one async-task polling round starts.
// A round fetches every unfinished task one-by-one from upstream, so this is the
// upstream request rate divided by the concurrent task count. Lowering it cuts
// the result latency clients see but multiplies upstream load and the risk of
// being rate limited; the scheduler never overlaps rounds, so the effective
// interval is max(this, one round's duration).
var TaskPollingIntervalSeconds int

// temporary variable for sora patch, will be removed in future
var TaskPricePatches []string

// TrustedRedirectDomains is a list of trusted domains for redirect URL validation.
// Domains support subdomain matching (e.g., "example.com" matches "sub.example.com").
var TrustedRedirectDomains []string
