package internal

// Messages for job state changes
type DownloadCompletedMsg struct{ Err error }
type ProcessingCompletedMsg struct{ Err error }

// WorkflowCompletedMsg indicates the workflow has completed
type WorkflowCompletedMsg struct{}

// TranscriptJob represents a single YouTube transcript processing job
type TranscriptJob struct {
	URL           string
	Title         string
	Status        string // "pending", "downloading", "processing", "completed", "failed"
	Error         error
	ProcessedFile string
}

// TitleFetchResult is a message containing the fetched title for a URL
type TitleFetchResult struct {
	URL   string
	Title string
	Err   error // Added to propagate errors from FetchTitle
}

// WorkflowState represents the application's workflow state
type WorkflowState struct {
	CurrentJob     *TranscriptJob // Changed from Jobs []TranscriptJob
	CurrentStage   string         // "fetching_title", "downloading", "processing", "completed", "failed"
	ProgressView   ProgressView
	ReadyToQuit    bool // Manages quit sequence
	ProcessedFiles []string
	RawVTTDir      string
	CleanedDir     string
	InitialURL     string // To store the initial URL for the single job
}

// NewWorkflow creates a new workflow with initial state for the given URL
func NewWorkflow(url string, rawVTTDir, cleanedDir string) WorkflowState {
	job := &TranscriptJob{
		URL:    url,
		Status: "pending", // Will transition to fetching_title in Init
	}

	return WorkflowState{
		CurrentJob:     job,
		InitialURL:     url,              // Store the initial URL
		CurrentStage:   "fetching_title", // Initial stage
		ProgressView:   NewProgressView(),
		ReadyToQuit:    false,
		ProcessedFiles: []string{},
		RawVTTDir:      rawVTTDir,
		CleanedDir:     cleanedDir,
	}
}
