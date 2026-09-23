package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsImageTaskPath guards the boundary between the async image-task
// endpoints (POST /v1/images, GET /v1/images/{task_id}) and the pre-existing
// synchronous image endpoints. Misclassifying a synchronous path would route it
// through task submission and break image generation entirely.
func TestIsImageTaskPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// async task endpoints
		{"/v1/images", true},
		{"/v1/images/", true},
		{"/v1/images/task_abc123", true},

		// synchronous endpoints must not be treated as tasks
		{"/v1/images/generations", false},
		{"/v1/images/edits", false},
		{"/v1/images/variations", false},

		// unrelated paths
		{"/v1/videos", false},
		{"/v1/images/task_abc/content", false},
		{"/v1/chat/completions", false},
		{"/v1/imagesfoo", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, isImageTaskPath(tt.path))
		})
	}
}
