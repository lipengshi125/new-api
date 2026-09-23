package common

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateImageTaskRequestNBound protects the billing invariant that `n`,
// which becomes a quota multiplier for async image tasks, cannot exceed
// dto.MaxImageN. An unbounded or wrapped-negative n would otherwise overflow
// quota calculation into a credit.
func TestValidateImageTaskRequestNBound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// TaskRelayInfo is embedded as a pointer and is initialized by
	// GenRelayInfo for RelayFormatTask; the fixture mirrors that.
	newContext := func(body string) (*gin.Context, *RelayInfo) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		return c, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}
	}

	boundErr := fmt.Sprintf("n must be an integer between 1 and %d", dto.MaxImageN)

	tests := []struct {
		name    string
		body    string
		wantErr string
		wantN   *uint
	}{
		{
			name:    "wrapped negative n is rejected",
			body:    `{"model":"image-2.5","prompt":"a cat","n":18446744073686646784}`,
			wantErr: boundErr,
		},
		{
			name:    "n above max is rejected",
			body:    fmt.Sprintf(`{"model":"image-2.5","prompt":"a cat","n":%d}`, dto.MaxImageN+1),
			wantErr: boundErr,
		},
		{
			name:    "zero n is rejected",
			body:    `{"model":"image-2.5","prompt":"a cat","n":0}`,
			wantErr: boundErr,
		},
		{
			name:  "n at max is accepted",
			body:  fmt.Sprintf(`{"model":"image-2.5","prompt":"a cat","n":%d}`, dto.MaxImageN),
			wantN: uintPtr(dto.MaxImageN),
		},
		{
			name:  "absent n stays nil",
			body:  `{"model":"image-2.5","prompt":"a cat"}`,
			wantN: nil,
		},
		{
			name:    "missing model is rejected",
			body:    `{"prompt":"a cat"}`,
			wantErr: "model field is required",
		},
		{
			name:    "blank prompt is rejected",
			body:    `{"model":"image-2.5","prompt":"   "}`,
			wantErr: "prompt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, info := newContext(tt.body)
			taskErr := ValidateImageTaskRequest(c, info)

			if tt.wantErr != "" {
				require.NotNil(t, taskErr)
				assert.Contains(t, taskErr.Message, tt.wantErr)
				assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
				return
			}

			require.Nil(t, taskErr)
			req, err := GetTaskRequest(c)
			require.NoError(t, err)
			assert.Equal(t, tt.wantN, req.N)
			assert.Equal(t, constant.TaskActionImageGenerate, info.Action)
		})
	}
}

// TestValidateImageTaskRequestImageAliases verifies the single-image aliases
// collapse into Images, which downstream adaptors read.
func TestValidateImageTaskRequestImageAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "input_reference becomes images",
			body: `{"model":"m","prompt":"p","input_reference":"https://example.com/a.png"}`,
			want: []string{"https://example.com/a.png"},
		},
		{
			name: "image becomes images",
			body: `{"model":"m","prompt":"p","image":"https://example.com/b.png"}`,
			want: []string{"https://example.com/b.png"},
		},
		{
			name: "explicit images is preserved",
			body: `{"model":"m","prompt":"p","images":["https://example.com/c.png"],"image":"https://example.com/ignored.png"}`,
			want: []string{"https://example.com/c.png"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images", bytes.NewBufferString(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			require.Nil(t, ValidateImageTaskRequest(c, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}))
			req, err := GetTaskRequest(c)
			require.NoError(t, err)
			assert.Equal(t, tt.want, req.Images)
		})
	}
}

func uintPtr(v uint) *uint { return &v }
