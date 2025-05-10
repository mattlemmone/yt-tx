package internal

import "sync"

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

// JobProcessingResult holds the outcome of processing a single job by a worker.
type JobProcessingResult struct {
	OriginalJobIndex int // To map the result back to the job in WorkflowState.Jobs
	ProcessedJob     TranscriptJob
	Err              error // An error that might have occurred during the entire job processing by the worker
}

// WorkflowState represents the application's workflow state
type WorkflowState struct {
	Jobs            []TranscriptJob
	CurrentJobIndex int // May become less central, represents last job detailed in UI or for sequential fallback
	TotalJobs       int
	CurrentStage    string // Overall workflow stage e.g., "processing_parallel", "completed"
	ProgressView    ProgressView
	ReadyToQuit     bool
	ProcessedFiles  []string
	TempDir         string
	CleanedDir      string
	ParallelWorkers int // Number of workers for parallel processing

	// Fields for parallelism
	jobQueue      chan int                 // Channel of job indices to process
	resultsChan   chan JobProcessingResult // Channel for workers to send results
	jobsCompleted int                      // Counter for completed jobs
	wg            *sync.WaitGroup
}

// NewWorkflow creates a new workflow with initial state for the given URLs
func NewWorkflow(urls []string, tempDir, cleanedDir string, parallelWorkers int) WorkflowState {
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
		TempDir:         tempDir,
		CleanedDir:      cleanedDir,
		ParallelWorkers: parallelWorkers,
		// Initialize new fields
		jobQueue:      make(chan int, len(urls)),      // Buffered channel for all job indices
		resultsChan:   make(chan JobProcessingResult), // Unbuffered for results
		jobsCompleted: 0,
		wg:            &sync.WaitGroup{},
	}
}
