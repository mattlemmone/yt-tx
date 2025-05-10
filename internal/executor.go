package internal

import (
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// Init is the first command that will be run.
func (w WorkflowState) Init() tea.Cmd {
	if w.TotalJobs == 0 {
		return tea.Quit // No jobs, nothing to do
	}
	// Start by fetching the title of the first job
	w.Jobs[0].Status = "fetching_title"
	w.CurrentStage = "fetching_title" // Set overall stage
	return func() tea.Msg {
		url := w.Jobs[0].URL
		title, err := FetchTitle(url)
		// Error handling for FetchTitle: return result with error to be handled in Update
		return TitleFetchResult{URL: url, Title: title, Err: err}
	}
}

// View renders the UI for the current workflow state
func (w WorkflowState) View() string {
	if w.TotalJobs == 0 {
		return "No URLs provided. Exiting."
	}

	if w.CurrentJobIndex >= w.TotalJobs {
		// This can happen if all jobs are processed and we are in the completed stage
		if w.CurrentStage == "completed" {
			// Check if any job failed to display a general error message
			for _, job := range w.Jobs {
				if job.Error != nil {
					return w.ProgressView.RenderOverallFailure(w.Jobs)
				}
			}
			return w.ProgressView.RenderCompleted()
		}
		return "Processing complete. Quitting..." // Or some other final state before quit
	}

	currentJob := w.Jobs[w.CurrentJobIndex]
	title := currentJob.Title

	switch w.CurrentStage { // This stage reflects the current *overall* step for the current job
	case "completed": // Overall completion, all jobs done
		// This case in View might be tricky if CurrentJobIndex is already past TotalJobs.
		// The logic above (CurrentJobIndex >= TotalJobs) handles the final screen better.
		// For individual job success that leads to this, it's handled by advancing index.
		return w.ProgressView.RenderCompleted()
	case "failed": // A specific job failed, or overall failure
		// If CurrentJobIndex is valid, show specific job failure
		if w.CurrentJobIndex < w.TotalJobs && currentJob.Error != nil {
			return w.ProgressView.RenderFailed(currentJob.Error, currentJob.Title)
		}
		// Otherwise, it might be a more general failure or end of batch with errors.
		return w.ProgressView.RenderOverallFailure(w.Jobs)
	case "fetching_title":
		jobTitle := currentJob.Title
		if jobTitle == "" {
			jobTitle = "Fetching title..."
		}
		return w.ProgressView.RenderDownloading(w.CurrentJobIndex, w.TotalJobs, jobTitle)
	case "downloading":
		return w.ProgressView.RenderDownloading(w.CurrentJobIndex, w.TotalJobs, title)
	case "processing":
		return w.ProgressView.RenderProcessing(w.CurrentJobIndex, w.TotalJobs, w.RawVTTDir)
	default:
		return fmt.Sprintf("Unknown state: %s, Job: %d/%d", w.CurrentStage, w.CurrentJobIndex+1, w.TotalJobs)
	}
}

// Update handles state transitions in the workflow
func (w WorkflowState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if w.ReadyToQuit {
		return w, tea.Quit
	}

	if w.TotalJobs == 0 {
		return w, tea.Quit // Should have been caught by Init, but as a safeguard
	}

	switch msg := msg.(type) {
	case progress.FrameMsg:
		progModel, cmd := w.ProgressView.UpdateProgress(msg)
		w.ProgressView.Progress = progModel // Update the progress model on w
		return w, cmd

	case tea.WindowSizeMsg:
		return w, func() tea.Msg { return progress.FrameMsg{} }

	case TitleFetchResult:
		jobIdx := -1
		for i := range w.Jobs {
			if w.Jobs[i].URL == msg.URL {
				jobIdx = i
				break
			}
		}

		if jobIdx == -1 || jobIdx != w.CurrentJobIndex { // Should not happen if logic is correct
			return w, nil // Message for a job we aren't expecting or is old
		}

		currentJob := &w.Jobs[jobIdx]
		if msg.Err != nil {
			currentJob.Error = msg.Err
			currentJob.Status = "failed"
			// Decide if we stop all or continue with next job
			// For now, let's mark as failed and try to move to the next job
			return w.handleJobCompletion(true) // True indicates current job had an error
		}

		currentJob.Title = msg.Title
		if currentJob.Title == "" { // Fallback if yt-dlp provides empty title
			currentJob.Title = ExtractVideoID(currentJob.URL)
		}
		currentJob.Status = "downloading_subtitles"
		w.CurrentStage = "downloading" // Set overall stage for this job processing

		return w, func() tea.Msg {
			err := DownloadSubtitles(currentJob.URL, w.RawVTTDir)
			// Include URL in DownloadCompletedMsg if needed for context, though not strictly now
			return DownloadCompletedMsg{Err: err}
		}

	case DownloadCompletedMsg:
		return w.handleDownloadCompleted(msg.Err)

	case ProcessingCompletedMsg:
		return w.handleProcessingCompleted(msg.Err)

	case WorkflowCompletedMsg:
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

// handleJobCompletion is a new helper to manage moving to the next job or finalizing
func (w WorkflowState) handleJobCompletion(currentJobFailed bool) (tea.Model, tea.Cmd) {
	if currentJobFailed {
		// The job's error and status are already set
	}

	w.CurrentJobIndex++

	if w.CurrentJobIndex >= w.TotalJobs {
		return w.finalizeWorkflow()
	}

	// Start next job: Fetch its title
	nextJob := &w.Jobs[w.CurrentJobIndex]
	nextJob.Status = "fetching_title"
	w.CurrentStage = "fetching_title"

	// Calculate progress for display so far
	// Each job contributes 1/TotalJobs to progress.
	// For simplicity, let's say completing a job (even if failed) counts as its portion done for overall batch progress.
	percentComplete := float64(w.CurrentJobIndex) / float64(w.TotalJobs)

	return w, tea.Batch(
		w.ProgressView.SetProgress(percentComplete),
		func() tea.Msg { return progress.FrameMsg{} },
		func() tea.Msg {
			title, err := FetchTitle(nextJob.URL)
			return TitleFetchResult{URL: nextJob.URL, Title: title, Err: err}
		},
	)
}

// handleDownloadCompleted handles the state transition after a download completes
func (w WorkflowState) handleDownloadCompleted(err error) (tea.Model, tea.Cmd) {
	if w.CurrentJobIndex >= w.TotalJobs {
		return w, tea.Quit
	} // Safety check

	currentJob := &w.Jobs[w.CurrentJobIndex]
	if err != nil {
		currentJob.Error = err
		currentJob.Status = "failed"
		return w.handleJobCompletion(true)
	}

	currentJob.Status = "processing_transcript"
	w.CurrentStage = "processing"

	// Calculate progress: index * (1/total) for completed jobs + 0.5 * (1/total) for current download part
	percentComplete := (float64(w.CurrentJobIndex) + 0.5) / float64(w.TotalJobs)
	if w.TotalJobs == 0 {
		percentComplete = 0
	} // Avoid division by zero

	return w, tea.Batch(
		ProcessTranscript(w.RawVTTDir, w.CleanedDir),
		w.ProgressView.SetProgress(percentComplete),
		func() tea.Msg { return progress.FrameMsg{} },
	)
}

// handleProcessingCompleted handles the state transition after processing completes
func (w WorkflowState) handleProcessingCompleted(err error) (tea.Model, tea.Cmd) {
	if w.CurrentJobIndex >= w.TotalJobs {
		return w, tea.Quit
	} // Safety check

	currentJob := &w.Jobs[w.CurrentJobIndex]
	if err != nil {
		currentJob.Error = err
		currentJob.Status = "failed"
		return w.handleJobCompletion(true)
	} else {
		currentJob.Status = "completed"
		// Add to list of successfully processed files if needed, e.g. currentJob.ProcessedFile
		// w.ProcessedFiles = append(w.ProcessedFiles, currentJob.ProcessedFile)
		return w.handleJobCompletion(false)
	}
}

// finalizeWorkflow handles the completion of all jobs
func (w WorkflowState) finalizeWorkflow() (tea.Model, tea.Cmd) {
	w.CurrentStage = "completed" // Overall workflow is complete

	return w, tea.Batch(
		w.ProgressView.SetProgress(1.0),
		func() tea.Msg { return progress.FrameMsg{} },
		func() tea.Msg { return WorkflowCompletedMsg{} },
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
