package constant

type TaskPlatform string

const (
	TaskPlatformSuno       TaskPlatform = "suno"
	TaskPlatformMidjourney              = "mj"
	// TaskPlatformImage is the async image-task platform (POST /v1/images,
	// GET /v1/images/{id}). It is keyed by name rather than channel type
	// because the same channel can serve both sync images and async image
	// tasks, and the platform is what the polling loop dispatches on.
	TaskPlatformImage = "image"
)

const (
	SunoActionMusic  = "MUSIC"
	SunoActionLyrics = "LYRICS"

	TaskActionGenerate = "generate"
	// TaskActionImageGenerate marks an async image task. The polling loop only
	// forwards task_id and action to FetchTask, so the action is what lets an
	// adaptor tell an image task from a video task when resolving upstream URLs.
	TaskActionImageGenerate     = "imageGenerate"
	TaskActionTextGenerate      = "textGenerate"
	TaskActionFirstTailGenerate = "firstTailGenerate"
	TaskActionReferenceGenerate = "referenceGenerate"
	TaskActionRemix             = "remixGenerate"
)

var SunoModel2Action = map[string]string{
	"suno_music":  SunoActionMusic,
	"suno_lyrics": SunoActionLyrics,
}
