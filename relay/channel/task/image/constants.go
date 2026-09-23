package image

// ModelList is intentionally empty: async image models are provisioned per
// channel by the operator, not hard-coded here. The relay resolves the model
// from the request and the channel's own model list.
var ModelList = []string{}

var ChannelName = "image"
