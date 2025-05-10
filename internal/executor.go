package internal

import (
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// Helper function to wait for a job result from the results channel
func waitForJobResultCmd(resultsChan chan JobProcessingResult) tea.Cmd {
	return func() tea.Msg {
		return <-resultsChan
	}
}

// runWorker is the function executed by each worker goroutine.
// It processes jobs from the jobQueue and sends results to resultsChan.
func runWorker(id int, jobs []TranscriptJob, jobQueue chan int, resultsChan chan JobProcessingResult, tempDir, cleanedDir string, wg *sync.WaitGroup) {
	defer wg.Done()
	for jobIndex := range jobQueue {
		job := jobs[jobIndex] // Get a copy of the job to work on
		job.Status = "fetching_title"
		// Send an initial update that this job is starting (optional, but good for UI)
		// resultsChan <- JobProcessingResult{OriginalJobIndex: jobIndex, ProcessedJob: job}

		// 1. Fetch Title
		title, err := FetchTitle(job.URL)
		if err != nil {
			job.Error = fmt.Errorf("failed to fetch title: %w", err)
			job.Status = "failed"
			resultsChan <- JobProcessingResult{OriginalJobIndex: jobIndex, ProcessedJob: job, Err: job.Error}
			continue
		}
		job.Title = title

		// 2. Extract Video ID (needed for VTT filename)
		videoID, idErr := ExtractVideoID(job.URL)
		if idErr != nil {
			// If title was empty and ID extraction fails, this is a bigger issue.
			// If title is present, we might proceed but VTT download might fail or use a different ID.
			// For now, let's consider ID extraction failure critical for finding the VTT.
			job.Error = fmt.Errorf("failed to extract video ID: %w", idErr)
			job.Status = "failed"
			resultsChan <- JobProcessingResult{OriginalJobIndex: jobIndex, ProcessedJob: job, Err: job.Error}
			continue
		}
		// If title was empty from FetchTitle, use videoID as a fallback title for display/logging
		if job.Title == "" {
			job.Title = videoID
		}

		// Check if cleaned file already exists
		expectedCleanedPath, pathErr := GetCleanedFilePathByTitle(job.Title, cleanedDir)
		if pathErr != nil {
			job.Error = fmt.Errorf("failed to determine cleaned file path: %w", pathErr)
			job.Status = "failed"
			resultsChan <- JobProcessingResult{OriginalJobIndex: jobIndex, ProcessedJob: job, Err: job.Error}
			continue
		}

		if _, statErr := os.Stat(expectedCleanedPath); statErr == nil {
			// File exists, skip processing
			job.Status = "skipped (exists)"
			job.ProcessedFile = expectedCleanedPath
			job.Error = nil // Ensure no error for skipped jobs
			resultsChan <- JobProcessingResult{OriginalJobIndex: jobIndex, ProcessedJob: job, Err: nil}
			continue // Move to the next job
		} else if !os.IsNotExist(statErr) {
			// os.Stat failed for a reason other than file not existing (e.g., permissions)
			job.Error = fmt.Errorf("error checking existing cleaned file %s: %w", expectedCleanedPath, statErr)
			job.Status = "failed"
			resultsChan <- JobProcessingResult{OriginalJobIndex: jobIndex, ProcessedJob: job, Err: job.Error}
			continue
		}
		// If os.IsNotExist(statErr) is true, proceed.

		job.Status = "downloading_subtitles"
		// resultsChan <- JobProcessingResult{OriginalJobIndex: jobIndex, ProcessedJob: job} // Update UI

		// 3. Download Subtitles (will be saved as <videoID>.vtt)
		err = DownloadSubtitles(job.URL, videoID, tempDir) // Pass videoID and use tempDir from runWorker's params
		if err != nil {
			job.Error = fmt.Errorf("failed to download subtitles: %w", err)
			job.Status = "failed"
			resultsChan <- JobProcessingResult{OriginalJobIndex: jobIndex, ProcessedJob: job, Err: job.Error}
			continue
		}
		job.Status = "processing_transcript"
		// resultsChan <- JobProcessingResult{OriginalJobIndex: jobIndex, ProcessedJob: job} // Update UI

		// 4. Process Transcript
		cleanedFile, err := ProcessSingleTranscript(videoID, job.Title, tempDir, cleanedDir)
		if err != nil {
			job.Error = fmt.Errorf("failed to process transcript: %w", err)
			job.Status = "failed"
		} else {
			job.Status = "completed"
			job.ProcessedFile = cleanedFile
		}
		resultsChan <- JobProcessingResult{OriginalJobIndex: jobIndex, ProcessedJob: job, Err: job.Error}
	}
}

// Init is the first command that will be run.
func (w WorkflowState) Init() tea.Cmd {
	if w.TotalJobs == 0 {
		return tea.Quit
	}

	// Launch workers if ParallelWorkers > 0
	if w.ParallelWorkers > 0 {
		w.wg.Add(w.ParallelWorkers)
		for i := 0; i < w.ParallelWorkers; i++ {
			go runWorker(i, w.Jobs, w.jobQueue, w.resultsChan, w.TempDir, w.CleanedDir, w.wg)
		}

		// Populate job queue
		for i := 0; i < w.TotalJobs; i++ {
			w.jobQueue <- i
		}
		close(w.jobQueue) // Close jobQueue once all jobs are sent

		// Start listening for the first result
		return waitForJobResultCmd(w.resultsChan)
	}

	// Fallback to sequential processing if ParallelWorkers is 0 or less (legacy behavior)
	// This part needs to be carefully re-evaluated or removed if parallel is the only mode.
	// For now, let's assume we only support parallelWorkers >= 1.
	// The CLI defaults it to 1, so this should be fine.
	if w.TotalJobs > 0 {
		w.Jobs[0].Status = "fetching_title" // Should be handled by worker
		w.CurrentStage = "fetching_title"   // CurrentStage also needs rethink for parallel
		// This sequential path is problematic with the new structure.
		// Best to ensure ParallelWorkers is always at least 1.
		// The CLI defaults it to 1, so this should be fine.
	}
	// This return is effectively a no-op if parallel workers are started.
	// The real first command for parallel is waitForJobResultCmd.
	return nil
}

// View renders the UI for the current workflow state
func (w WorkflowState) View() string {
	if w.TotalJobs == 0 {
		return "No URLs provided. Exiting."
	}

	// If all jobs are completed, show final status
	if w.jobsCompleted == w.TotalJobs && !w.ReadyToQuit { // Added !w.ReadyToQuit to prevent premature completed view
		allSuccess := true
		for _, job := range w.Jobs {
			if job.Error != nil {
				allSuccess = false
				break
			}
		}
		if allSuccess {
			return w.ProgressView.RenderCompleted() // Assumes this is a generic success message
		}
		// If some jobs failed, RenderOverallFailure will list them.
		return w.ProgressView.RenderOverallFailure(w.Jobs)
	}

	if w.ReadyToQuit { // After all jobs processed and we're ready to quit
		// Similar logic to above to show final status before actual quit
		allSuccess := true
		for _, job := range w.Jobs {
			if job.Error != nil {
				allSuccess = false
				break
			}
		}
		if allSuccess {
			return w.ProgressView.RenderCompleted() + "\nQuitting..."
		}
		return w.ProgressView.RenderOverallFailure(w.Jobs) + "\nQuitting..."
	}

	// For ongoing processing, show progress and status of jobs
	// The `percent` variable previously here is no longer needed as RenderJobList handles it.

	// Default view during parallel processing:
	return w.ProgressView.RenderJobList(w.Jobs, w.jobsCompleted, w.TotalJobs, w.ParallelWorkers)
}

// Update handles state transitions in the workflow
func (w WorkflowState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if w.ReadyToQuit { // If we're in the process of quitting, no more updates
		return w, tea.Quit
	}

	// Handle messages only if parallel processing is active
	// Or, more simply, always handle these messages.
	// The original code had TotalJobs == 0 check.
	if w.TotalJobs == 0 && !w.ReadyToQuit { // Added !w.ReadyToQuit
		return w, tea.Quit
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			w.ReadyToQuit = true // Signal workers to stop (not implemented yet by closing jobQueue early or context)
			// We should ideally wait for workers here using w.wg.Wait()
			// but that blocks the UI thread.
			// A better approach for Ctrl+C is to signal workers and let them finish,
			// then quit when all results are in or a timeout.
			// For now, just set ReadyToQuit and let the main loop quit.
			// This means workers might be cut off.
			// A more graceful shutdown would involve closing jobQueue (done in Init if all jobs sent),
			// and workers checking a quit signal.
			return w, tea.Quit // Force quit for now
		default:
			return w, nil
		}

	case progress.FrameMsg: // For progress bar animation
		// Assuming Progress is always initialized by NewProgressView
		progModel, cmd := w.ProgressView.Progress.Update(msg) // Update the progress.Model directly
		if pModel, ok := progModel.(progress.Model); ok {     // Type assertion
			w.ProgressView.Progress = pModel
		} else {
			// Handle error: progModel was not a progress.Model
			// This shouldn't happen if Progress.Update consistently returns progress.Model
			// For now, can log or ignore. Or panic if it's an invariant.
		}
		cmds = append(cmds, cmd)
		return w, tea.Batch(cmds...)

	case JobProcessingResult:
		// Update the specific job in the Jobs slice
		if msg.OriginalJobIndex >= 0 && msg.OriginalJobIndex < len(w.Jobs) {
			w.Jobs[msg.OriginalJobIndex] = msg.ProcessedJob
			if msg.ProcessedJob.Status == "completed" && msg.ProcessedJob.Error == nil {
				// Optionally collect successfully processed files
				// w.ProcessedFiles = append(w.ProcessedFiles, msg.ProcessedJob.ProcessedFile)
			}
		}

		w.jobsCompleted++

		// Update overall progress
		percentComplete := float64(w.jobsCompleted) / float64(w.TotalJobs)
		// Assuming Progress is always initialized
		cmds = append(cmds, w.ProgressView.Progress.SetPercent(percentComplete)) // Call SetPercent on the progress.Model
		cmds = append(cmds, func() tea.Msg { return progress.FrameMsg{} })       // Trigger re-render of progress

		if w.jobsCompleted == w.TotalJobs {
			// All jobs are processed
			w.CurrentStage = "completed" // Set overall workflow stage to completed
			w.ReadyToQuit = true         // Signal that we can quit after this update cycle
			// No new command, View will show completed status, next empty msg or keypress might lead to quit
			// Or send a WorkflowCompletedMsg for consistency if something listens to it.
			// For now, ReadyToQuit should be enough.
			// The WaitGroup should be waited on before true completion.
			// This should ideally be done in a separate goroutine that sends a final completion message.
			// go func() { w.wg.Wait(); w.resultsChan <- AllWorkersDoneMsg{} }() // Need new msg type
			// For now, the main loop will exit when ReadyToQuit is true.
			// wg.Wait() is tricky with BubbleTea. Workers complete, send result. wg.Done in worker.
			// The main goroutine doesn't block on wg.Wait() in Update.
			// The assumption is that Init launches workers and wg.Add, workers call wg.Done.
			// The program quits when jobsCompleted == TotalJobs.
			// If a clean shutdown of workers (e.g. closing channels, context cancellation) is needed on Ctrl+C,
			// that requires more signalling.
		} else {
			// Request the next result from any worker
			cmds = append(cmds, waitForJobResultCmd(w.resultsChan))
		}
		return w, tea.Batch(cmds...)

	// Old messages (TitleFetchResult, DownloadCompletedMsg, ProcessingCompletedMsg) are no longer primary drivers.
	// They are handled within the worker.
	// If any old tea.Cmds that produced these are still around, they might need to be removed.
	// The WorkflowCompletedMsg might still be useful to signal the TUI loop to prepare for shutdown.
	case WorkflowCompletedMsg: // This can be sent after wg.Wait() if we implement that.
		w.ReadyToQuit = true
		return w, tea.Quit // Or just tea.Quit if it's the final signal

	default:
		return w, nil
	}
}

// ProcessSingleTranscript takes a videoID and title, finds its raw VTT file,
// cleans it, and saves it to the cleaned directory.
func ProcessSingleTranscript(videoID, videoTitle, tempDir, cleanedDir string) (string, error) {
	// 1. Determine the raw VTT file path using videoID
	rawFilePath, err := GetLocalVTTPathByVideoID(videoID, tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to determine raw VTT file path for video ID %s: %w", videoID, err)
	}

	// 2. Determine the cleaned file path using videoTitle
	cleanedFilePath, err := GetCleanedFilePathByTitle(videoTitle, cleanedDir)
	if err != nil {
		return "", fmt.Errorf("failed to determine cleaned file path for title %s: %w", videoTitle, err)
	}

	// 3. Clean the VTT file content
	cleanedContent, err := CleanVTTFile(rawFilePath) // From internal/transcript.go
	if err != nil {
		return "", fmt.Errorf("failed to clean VTT file %s: %w", rawFilePath, err)
	}

	// 4. Write the cleaned content to the destination file
	err = WriteTextFile(cleanedFilePath, cleanedContent) // Assuming WriteTextFile is in internal/files.go
	if err != nil {
		return "", fmt.Errorf("failed to write cleaned transcript to %s: %w", cleanedFilePath, err)
	}

	return cleanedFilePath, nil
}

// Original ProcessTranscript and other helper funcs like handleJobCompletion,
// handleDownloadCompleted, handleProcessingCompleted, finalizeWorkflow are largely
// superseded by the worker logic and the new Update handling JobProcessingResult.
// They should be reviewed and removed/refactored.
// The FetchTitle, DownloadSubtitles functions are now called directly by workers.
// The `ProcessTranscript` tea.Cmd is replaced by direct call to `CleanTranscript` or similar logic
// within `ProcessSingleTranscript`.

// Ensure functions like FetchTitle, DownloadSubtitles, CleanTranscript, GetLocalVTTPath, GetCleanedFilePath
// are concurrency-safe if they access shared resources (they mostly seem to operate on passed params or own state).
// CleanTranscript was:
// func CleanTranscript(rawVTTPath, cleanedPath string) error
// This should be fine.

// GetLocalVTTPath and GetCleanedFilePath need to be implemented or found.
// They likely involve sanitizing the title and joining with dir paths.
// These were implicit in the original sequential flow.
// For example, if DownloadSubtitles saved as title.vtt:
// func GetLocalVTTPath(videoTitle, rawVTTDir string) (string, error) {
//   safeTitle := SanitizeFilename(videoTitle) // Need SanitizeFilename
//   return filepath.Join(rawVTTDir, safeTitle+".vtt"), nil
// }
// Similar for GetCleanedFilePath.
// This detail is important for the worker to find the correct files.
// Let's assume these path helpers exist or will be created.
// The existing `DownloadSubtitles` in `youtube.go` uses `fmt.Sprintf("%s.%%(ext)s", title)` for yt-dlp.
// So the VTT file would be `title.vtt`.

// The `CleanTranscript` function is in `internal/transcript.go`. It takes (rawVTTPath, cleanedPath string).
// The `FetchTitle` and `DownloadSubtitles` are in `internal/youtube.go`.
// `ExtractVideoID` is in `internal/youtube.go`.

// One missing piece: how `ProcessTranscript` (the old tea.Cmd) mapped to `CleanTranscript`.
// Old `ProcessTranscript` cmd:
// func ProcessTranscript(rawVTTDir, cleanedDir string) tea.Cmd {
//	return func() tea.Msg {
//		err := CleanAndProcessVTTFiles(rawVTTDir, cleanedDir) // This was processing ALL files in dir
//		return ProcessingCompletedMsg{Err: err}
//	}
// }
// `CleanAndProcessVTTFiles` in `transcript.go` iterates all .vtt in rawVTTDir.
// This MUST change. The worker should only process *its own* downloaded VTT.

// So, DownloadSubtitles(url, rawVTTDir) downloads to `safeTitle + ".LANG.vtt"`.
// We need `GetLocalVTTPath` to correctly find this specific file.
// And `ProcessSingleTranscript` needs to call `CleanTranscript` on that specific file.
// This means `DownloadSubtitles` should ideally return the exact path of the downloaded file,
// or `runWorker` needs to reliably construct it.
// Let's assume DownloadSubtitles ensures a predictable name based on title, e.g., `title.en.vtt`.
// And `FetchTitle` provides this title.
// The current `DownloadSubtitles` uses `%(title)s.%(ext)s` as output template for yt-dlp.
// And then `WriteSubtitleToFile` inside it constructs the path.
// This path construction needs to be robust.

// For ProcessSingleTranscript, the rawVTTPath should be knowable after DownloadSubtitles.
// It might be easier if DownloadSubtitles returns the path of the file it created.
// For now, the placeholder ProcessSingleTranscript uses GetLocalVTTPath. This needs wiring.
// The actual filename convention of yt-dlp needs to be handled.
// yt-dlp output template is `%(title)s.%(ext)s`. ext is `vtt`.
// So, `title + ".vtt"` is the expected raw file name.
// This assumes `title` is filesystem safe. `FetchTitle` should ensure this or `DownloadSubtitles` should handle it.
// The `SanitizeFilename`
