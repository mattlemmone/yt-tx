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
	Jobs            []TranscriptJob // Changed back from *TranscriptJob
	CurrentJobIndex int             // Re-added
	TotalJobs       int             // Re-added
	CurrentStage    string          // "fetching_title", "downloading", "processing", "completed", "failed"
	ProgressView    ProgressView
	ReadyToQuit     bool // Manages quit sequence
	ProcessedFiles  []string
	RawVTTDir       string
	CleanedDir      string
	// InitialURL     string // Removed
}

// NewWorkflow creates a new workflow with initial state for the given URLs
func NewWorkflow(urls []string, rawVTTDir, cleanedDir string) WorkflowState {
	jobs := make([]TranscriptJob, len(urls))
	for i, url := range urls {
		jobs[i] = TranscriptJob{
			URL:    url,
			Status: "pending", // Initial status for each job
		}
	}

	initialStage := "fetching_title" // Overall workflow starts by fetching title for the first job
	if len(urls) == 0 {
		initialStage = "completed" // Or some other appropriate state if no URLs
	}

	return WorkflowState{
		Jobs:            jobs,
		CurrentJobIndex: 0,
		TotalJobs:       len(urls),
		CurrentStage:    initialStage,
		ProgressView:    NewProgressView(),
		ReadyToQuit:     false,
		ProcessedFiles:  []string{},
		RawVTTDir:       rawVTTDir,
		CleanedDir:      cleanedDir,
	}
}
