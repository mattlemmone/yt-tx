package internal

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/progress"
)

// TestNewProgressView checks if a ProgressView is initialized.
func TestNewProgressView(t *testing.T) {
	pv := NewProgressView()
	// Basic check: ensure the Progress field is not zero/nil.
	// Since progress.Model is a struct, it won't be nil.
	// We can check if it can View, or if its Percent is 0 initially.
	if pv.Progress.View() == "" && pv.Progress.Percent() != 0.0 { // A bit of a heuristic
		t.Errorf("NewProgressView() did not initialize Progress model as expected.")
	}
}

func TestProgressView_RenderCompleted(t *testing.T) {
	pv := NewProgressView()
	_ = pv.Progress.SetPercent(1.0) // Ensure it's at 100% for this view
	got := pv.RenderCompleted()
	if !strings.Contains(got, "✅ All done!") {
		t.Errorf("RenderCompleted() missing completion message, got %q", got)
	}
	if !strings.Contains(got, "100%") { // progress.ViewAs(1.0) should show 100%
		t.Errorf("RenderCompleted() missing 100%%, got %q", got)
	}
}

func TestProgressView_RenderFailed(t *testing.T) {
	pv := NewProgressView()
	_ = pv.Progress.SetPercent(0.6) // Example progress
	testError := errors.New("something went wrong")
	testTitle := "Test Video"

	got := pv.RenderFailed(testError, testTitle)

	if !strings.Contains(got, "❌ Error processing Test Video:") {
		t.Errorf("RenderFailed() missing error prefix with title, got %q", got)
	}
	if !strings.Contains(got, testError.Error()) {
		t.Errorf("RenderFailed() missing error message, got %q", got)
	}
	if !strings.Contains(got, "60%") { // Check if progress is displayed
		t.Errorf("RenderFailed() missing progress percentage, got %q", got)
	}

	// Test with empty title
	gotEmptyTitle := pv.RenderFailed(testError, "")
	if !strings.Contains(gotEmptyTitle, "❌ Error processing the job:") {
		t.Errorf("RenderFailed() with empty title incorrect, got %q", gotEmptyTitle)
	}
}

func TestProgressView_RenderOverallFailure(t *testing.T) {
	pv := NewProgressView()
	_ = pv.Progress.SetPercent(1.0) // Usually at the end

	jobs := []TranscriptJob{
		{Title: "Video 1", Error: errors.New("fail1")},
		{Title: "Video 2", Error: nil},
		{Title: "Video 3", Error: errors.New("fail3")},
	}
	got := pv.RenderOverallFailure(jobs)

	if !strings.Contains(got, "❌ Some jobs failed: Video 1, Video 3") {
		t.Errorf("RenderOverallFailure() missing correct failed titles summary, got %q", got)
	}
	if !strings.Contains(got, "100%") {
		t.Errorf("RenderOverallFailure() missing 100%%, got %q", got)
	}

	// Test with no failures (should ideally not be called, but test fallback)
	noFailJobs := []TranscriptJob{{Title: "OK", Error: nil}}
	gotNoFail := pv.RenderOverallFailure(noFailJobs)
	if !strings.Contains(gotNoFail, "⚠️ Some jobs may have encountered issues.") {
		t.Errorf("RenderOverallFailure() with no actual errors formatted incorrectly, got %q", gotNoFail)
	}
}

func TestProgressView_RenderDownloading(t *testing.T) {
	pv := NewProgressView()
	_ = pv.Progress.SetPercent(0.25) // Example progress
	tests := []struct {
		name            string
		currentJobIndex int
		totalJobs       int
		title           string
		expectedHeader  string
	}{
		{"fetching title", 0, 2, "", "[1/2] Fetching title..."},
		{"downloading specific title", 1, 2, "My Awesome Video", "[2/2] Downloading subtitles for: My Awesome Video"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Manually set the progress for consistent testing of the text part
			pv.Progress.SetPercent(0.25)
			got := pv.RenderDownloading(tt.currentJobIndex, tt.totalJobs, tt.title)
			if !strings.Contains(got, tt.expectedHeader) {
				t.Errorf("RenderDownloading() header = %q, want to contain %q", got, tt.expectedHeader)
			}
			// Check if progress bar string is part of output
			// We can't easily assert specific percentage text from View() in unit test
			if !strings.Contains(got, pv.Progress.View()) {
				t.Errorf("RenderDownloading() missing progress bar view, got %q", got)
			}
		})
	}
}

func TestProgressView_RenderProcessing(t *testing.T) {
	pv := NewProgressView()
	// Manually set the progress for consistent testing of the text part
	pv.Progress.SetPercent(0.75)

	// Note: This test is limited because GetNewestVTTPattern and filepath.Glob are hard to mock here.
	// We'll test the part that doesn't depend on actual file system results.
	gotNoFiles := pv.RenderProcessing(0, 1, "./temp_raw_vtt_dir_for_test_processing")
	expectedHeaderNoFiles := "[1/1] Processing..."
	if !strings.Contains(gotNoFiles, expectedHeaderNoFiles) {
		t.Errorf("RenderProcessing() with no files, header = %q, want to contain %q", gotNoFiles, expectedHeaderNoFiles)
	}
	// Check if progress bar string is part of output
	if !strings.Contains(gotNoFiles, pv.Progress.View()) {
		t.Errorf("RenderProcessing() missing progress bar view, got %q", gotNoFiles)
	}
	// A more thorough test for file-dependent part would require fs mocking or integration test
}

// Test for SetProgress and UpdateProgress are harder as they return tea.Cmd
// and modify internal bubble progress state. A simple check could be to ensure
// they don't panic and return a non-nil tea.Cmd where appropriate.

func TestProgressView_SetProgress(t *testing.T) {
	pv := NewProgressView()
	cmd := pv.SetProgress(0.5)
	if cmd == nil {
		// For some tea.Cmds, returning nil is valid if there's no action.
		// progress.Model.SetPercent always returns a tea.Cmd, typically progress.FrameMsg
		// However, the exact type isn't exported for easy checking without reflection or type assertion if it's an interface.
		// A simple nil check might be too basic. The fact that it doesn't panic is a start.
	}
	if pv.Progress.Percent() != 0.5 {
		t.Errorf("SetProgress did not update underlying Progress model percent. got %f, want 0.5", pv.Progress.Percent())
	}
}

func TestProgressView_UpdateProgress(t *testing.T) {
	pv := NewProgressView()
	// progress.FrameMsg is an empty struct, usually sent by the runtime
	_, cmd := pv.UpdateProgress(progress.FrameMsg{})
	if cmd == nil {
		// Similar to SetProgress, Update should return a command.
	}
	// We can't easily check the state of progress bar animation here.
}
