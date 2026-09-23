package dto

const (
	ImageTaskStatusQueued     = "queued"
	ImageTaskStatusInProgress = "in_progress"
	ImageTaskStatusCompleted  = "completed"
	ImageTaskStatusFailed     = "failed"
)

// OpenAIImageTask is the response body for the async image endpoints
// (POST /v1/images and GET /v1/images/{id}). It mirrors the shape of
// OpenAIVideo so clients can poll images and videos with the same logic.
type OpenAIImageTask struct {
	ID          string                 `json:"id"`
	Object      string                 `json:"object"`
	Model       string                 `json:"model"`
	Status      string                 `json:"status"`
	Progress    int                    `json:"progress"`
	CreatedAt   int64                  `json:"created_at"`
	CompletedAt int64                  `json:"completed_at,omitempty"`
	Size        string                 `json:"size,omitempty"`
	Data        []OpenAIImageTaskDatum `json:"data,omitempty"`
	Error       *OpenAIImageTaskError  `json:"error,omitempty"`
}

// OpenAIImageTaskDatum carries one produced image. Either URL or B64JSON is
// set, matching the OpenAI images response shape.
type OpenAIImageTaskDatum struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type OpenAIImageTaskError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

func NewOpenAIImageTask() *OpenAIImageTask {
	return &OpenAIImageTask{
		Object: "image.task",
		Status: ImageTaskStatusQueued,
	}
}
