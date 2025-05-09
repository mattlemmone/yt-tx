package internal

import (
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// Init is the first command that will be run.
func (w WorkflowState) Init() tea.Cmd {
	// Start by fetching the title of the current job
	if w.CurrentJob != nil && w.CurrentJob.URL != "" {
		// CurrentStage is already "fetching_title" from NewWorkflow
		return func() tea.Msg {
			title, err := FetchTitle(w.CurrentJob.URL)
			if err != nil && title == "" { // If FetchTitle had an error and couldn't even get an ID
				// Propagate the error, URL is important for context
				return TitleFetchResult{URL: w.CurrentJob.URL, Title: "", Err: err}
			}
			// If err is not nil but title has fallback (ID), it's not a fatal error for fetching stage
			// The error from FetchTitle (e.g. empty title from yt-dlp) will be in Err field.
			return TitleFetchResult{URL: w.CurrentJob.URL, Title: title, Err: err}
		}
	}
	return nil
}

// View renders the UI for the current workflow state
func (w WorkflowState) View() string {
	if w.CurrentJob == nil {
		return "Initializing..." // Or some other appropriate message
	}

	title := w.CurrentJob.Title
	// If title is empty during fetching_title, use a placeholder
	if w.CurrentStage == "fetching_title" && title == "" {
		title = "Fetching title..."
	}

	switch w.CurrentStage {
	case "completed":
		if w.CurrentJob.Error != nil {
			return w.ProgressView.RenderFailed(w.CurrentJob.Error, w.CurrentJob.Title)
		}
		return w.ProgressView.RenderCompleted()
	case "failed":
		return w.ProgressView.RenderFailed(w.CurrentJob.Error, w.CurrentJob.Title)
	case "fetching_title":
		// Use 0, 1 for current/total jobs as it's a single job
		return w.ProgressView.RenderDownloading(0, 1, title) // Show "Fetching title..."
	case "downloading":
		return w.ProgressView.RenderDownloading(0, 1, title)
	case "processing":
		return w.ProgressView.RenderProcessing(0, 1, w.RawVTTDir) // RenderProcessing might need job title too eventually
	default:
		return "Unknown state"
	}
}

// Update handles state transitions in the workflow
func (w WorkflowState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if w.ReadyToQuit {
		return w, tea.Quit
	}

	if w.CurrentJob == nil && w.InitialURL == "" {
		// Nothing to do if no job is set up and no initial URL to create one
		// This case should ideally not be hit if NewWorkflow is used correctly
		return w, tea.Quit
	}

	// Initialize CurrentJob if it's nil but InitialURL is present
	// This path might be taken if the tea.Model is not initialized via NewWorkflow
	// but directly, which is less likely with current main.go structure.
	if w.CurrentJob == nil && w.InitialURL != "" {
		w.CurrentJob = &TranscriptJob{URL: w.InitialURL, Status: "pending"}
		w.CurrentStage = "fetching_title"
		// Cmd to fetch title will be triggered by Init or next appropriate step
	}

	switch msg := msg.(type) {
	case progress.FrameMsg:
		_, cmd := w.ProgressView.UpdateProgress(msg)
		return w, cmd

	case tea.WindowSizeMsg:
		// This ensures the progress bar re-renders correctly on resize.
		return w, func() tea.Msg { return progress.FrameMsg{} }

	case TitleFetchResult:
		if w.CurrentJob == nil || msg.URL != w.CurrentJob.URL {
			// Message for a different/old job, ignore
			return w, nil
		}
		if msg.Err != nil {
			// If fetching title failed critically (e.g., yt-dlp command error)
			w.CurrentJob.Error = msg.Err
			w.CurrentJob.Status = "failed"
			w.CurrentStage = "failed"
			// Even if title fetch fails, we go to finalizeWorkflow to show completion/error
			return w.finalizeWorkflow()
		}

		w.CurrentJob.Title = msg.Title
		if w.CurrentJob.Title == "" { // If title is empty string (e.g. yt-dlp returned empty)
			w.CurrentJob.Title = ExtractVideoID(w.CurrentJob.URL) // Fallback to ID
		}
		w.CurrentJob.Status = "downloading_subtitles"
		w.CurrentStage = "downloading"

		return w, func() tea.Msg {
			err := DownloadSubtitles(w.CurrentJob.URL, w.RawVTTDir)
			return DownloadCompletedMsg{Err: err}
		}

	case DownloadCompletedMsg:
		return w.handleDownloadCompleted(msg.Err)

	case ProcessingCompletedMsg:
		return w.handleProcessingCompleted(msg.Err)

	case WorkflowCompletedMsg:
		// This message signals that finalizeWorkflow has done its part,
		// and we can now prepare to quit.
		w.ReadyToQuit = true
		return w, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return w, tea.Quit
		default:
			return w, nil
		}

	default:
		return w, nil
	}
}

// handleDownloadCompleted handles the state transition after a download completes
func (w WorkflowState) handleDownloadCompleted(err error) (tea.Model, tea.Cmd) {
	if w.CurrentJob == nil {
		return w, tea.Quit // Should not happen
	}

	if err != nil {
		w.CurrentJob.Error = err
		w.CurrentJob.Status = "failed"
		w.CurrentStage = "failed"
		return w.finalizeWorkflow()
	}

	w.CurrentJob.Status = "processing_transcript"
	w.CurrentStage = "processing"

	return w, tea.Batch(
		ProcessTranscript(w.RawVTTDir, w.CleanedDir),  // This is already a tea.Cmd
		w.ProgressView.SetProgress(0.5),               // 50% after download
		func() tea.Msg { return progress.FrameMsg{} }, // Trigger a progress bar update
	)
}

// handleProcessingCompleted handles the state transition after processing completes
func (w WorkflowState) handleProcessingCompleted(err error) (tea.Model, tea.Cmd) {
	if w.CurrentJob == nil {
		return w, tea.Quit // Should not happen
	}

	if err != nil {
		w.CurrentJob.Error = err
		w.CurrentJob.Status = "failed"
		w.CurrentStage = "failed"
		// Even on error, we finalize to show the error message
	} else {
		w.CurrentJob.Status = "completed"
		// CurrentStage will be set to "completed" by finalizeWorkflow
	}

	// Whether success or failure in processing, we finalize the workflow.
	// finalizeWorkflow will set progress to 1.0 and stage to "completed" or reflect error.
	return w.finalizeWorkflow()
}

// finalizeWorkflow handles the completion of the job (success or failure)
func (w WorkflowState) finalizeWorkflow() (tea.Model, tea.Cmd) {
	if w.CurrentJob != nil && w.CurrentJob.Error != nil {
		w.CurrentStage = "failed"
	} else {
		w.CurrentStage = "completed"
	}

	// Always set progress to 1.0 at the end, failed or not, it's 100% of this attempt.
	return w, tea.Batch(
		w.ProgressView.SetProgress(1.0),
		func() tea.Msg { return progress.FrameMsg{} },    // Update progress bar view
		func() tea.Msg { return WorkflowCompletedMsg{} }, // Signal workflow (job) attempt is done
	)
}

// Add a stub for ProcessTranscript for now
// This function will eventually handle the actual transcript processing.
func ProcessTranscript(rawVTTDir, cleanedDir string) tea.Cmd {
	return func() tea.Msg {
		// Simulate processing by returning a success message immediately.
		// In a real scenario, this would involve file operations and could return an error.
		return ProcessingCompletedMsg{Err: nil} // TODO: Implement actual processing logic
	}
}
