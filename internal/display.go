package internal

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// ProgressView manages displaying progress information for transcript processing
type ProgressView struct {
	Progress progress.Model
}

// NewProgressView creates a new progress view
func NewProgressView() ProgressView {
	return ProgressView{
		Progress: progress.New(progress.WithDefaultGradient()),
	}
}

// RenderCompleted renders the completion view
func (v ProgressView) RenderCompleted() string {
	return "✅ All done!\n" + v.Progress.ViewAs(1.0) + "\n"
}

// RenderFailed renders the UI when a job has failed.
func (v ProgressView) RenderFailed(err error, title string) string {
	taskTitle := title
	if taskTitle == "" {
		taskTitle = "the job"
	}
	return fmt.Sprintf("❌ Error processing %s:\n%v\n%s\n", taskTitle, err, v.Progress.ViewAs(v.Progress.Percent()))
}

// RenderOverallFailure renders a summary if any jobs failed in a batch.
func (v ProgressView) RenderOverallFailure(jobs []TranscriptJob) string {
	var failedTitles []string
	for _, job := range jobs {
		if job.Error != nil {
			failedTitles = append(failedTitles, job.Title)
		}
	}
	if len(failedTitles) == 0 {
		// Should not be called if no failures, but as a fallback:
		return "⚠️ Some jobs may have encountered issues. Please check logs.\n" + v.Progress.ViewAs(1.0) + "\n"
	}
	return fmt.Sprintf("❌ Some jobs failed: %s\nPlease check individual errors if not displayed above.\n%s\n", strings.Join(failedTitles, ", "), v.Progress.ViewAs(1.0))
}

// RenderDownloading renders the UI when downloading subtitles
func (v ProgressView) RenderDownloading(currentJobIndex, totalJobs int, title string) string {
	header := fmt.Sprintf("[%d/%d] ", currentJobIndex+1, totalJobs)

	if title == "" || title == "Fetching title..." {
		header += "Fetching title..."
	} else {
		header += fmt.Sprintf("Downloading subtitles for: %s", title)
	}

	return fmt.Sprintf("%s\n%s\n", header, v.Progress.View())
}

// RenderProcessing renders the UI when processing subtitles
func (v ProgressView) RenderProcessing(currentJobIndex, totalJobs int, rawVTTDir string) string {
	header := fmt.Sprintf("[%d/%d] ", currentJobIndex+1, totalJobs)

	// Find newest VTT files
	pattern := GetNewestVTTPattern(rawVTTDir)
	files, _ := filepath.Glob(pattern)

	if len(files) > 0 {
		displayTitle := ExtractDisplayTitle(files[len(files)-1])
		header += fmt.Sprintf("Processing %s...", displayTitle)
	} else {
		header += "Processing..."
	}

	return fmt.Sprintf("%s\n%s\n", header, v.Progress.View())
}

// SetProgress updates the progress bar to the specified percentage
func (v *ProgressView) SetProgress(percent float64) tea.Cmd {
	return v.Progress.SetPercent(percent)
}

// UpdateProgress updates the progress bar based on animation frame
func (v *ProgressView) UpdateProgress(msg progress.FrameMsg) (progress.Model, tea.Cmd) {
	updatedProgress, cmd := v.Progress.Update(msg)
	v.Progress = updatedProgress.(progress.Model)
	return v.Progress, cmd
}
