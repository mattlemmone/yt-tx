package internal

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// mockProgressView can be used if we need to control ProgressView behavior, but for now, we use the real one.

// Helper to create a default WorkflowState for testing
func newTestWorkflowState(urls []string) WorkflowState {
	return NewWorkflow(urls, "test_raw_vtt", "test_cleaned")
}

func TestWorkflowState_Init(t *testing.T) {
	t.Run("no jobs", func(t *testing.T) {
		wf := newTestWorkflowState([]string{})
		cmd := wf.Init()
		// Expect tea.Quit command or similar indicator for no jobs
		if cmd == nil { // Based on current Init, it returns tea.Quit or the fetch func
			// If NewWorkflow sets stage to completed and Init does nothing, this is fine.
			// Let's check the stage from NewWorkflow for no URLs
			if wf.CurrentStage != "completed" {
				t.Errorf("Expected CurrentStage to be 'completed' for no URLs, got %s", wf.CurrentStage)
			}
		} else {
			// If it returned tea.Quit directly (e.g. if TotalJobs == 0 then tea.Quit)
			// For now, Init calls a func that might return TitleFetchResult
			// Let's verify the expectation with the current Init logic more closely.
			// If no URLs, TotalJobs is 0. Init returns tea.Quit.
			// This needs to align with the actual tea.Quit command object if possible or type.
			// Direct comparison of functions is tricky. Let's assume tea.Quit has a sentinel value or type.
			// For now, a simple non-nil check for quit might be too vague.
			// The actual Init returns tea.Quit, so cmd should not be nil.
		}
	})

	t.Run("one job", func(t *testing.T) {
		wf := newTestWorkflowState([]string{"http://example.com/video1"})
		cmd := wf.Init()
		if cmd == nil {
			t.Fatal("Init() returned nil command, expected a command to fetch title")
		}
		// Execute the command and check the message type (this is more like integration testing the cmd)
		// For a unit test, we'd ideally check that the cmd is a function that, when called,
		// eventually calls FetchTitle("http://example.com/video1").
		// This is hard without deeper inspection or refactoring FetchTitle to be mockable.
		// For now, we trust Init sets up a func to do this.
		// Let's at least check job status and stage were updated by Init itself (if applicable).
		if wf.Jobs[0].Status != "fetching_title" {
			t.Errorf("Init() did not set first job status to 'fetching_title', got: %s", wf.Jobs[0].Status)
		}
		if wf.CurrentStage != "fetching_title" {
			t.Errorf("Init() did not set wf.CurrentStage to 'fetching_title', got: %s", wf.CurrentStage)
		}
	})
}

func TestWorkflowState_Update_TitleFetchResult(t *testing.T) {
	t.Run("successful title fetch", func(t *testing.T) {
		wf := newTestWorkflowState([]string{"http://example.com/video1"})
		wf.CurrentJobIndex = 0
		wf.Jobs[0].Status = "fetching_title" // Set initial state
		wf.CurrentStage = "fetching_title"

		msg := TitleFetchResult{URL: "http://example.com/video1", Title: "Test Title", Err: nil}
		newWfModel, cmd := wf.Update(msg)
		newWf := newWfModel.(WorkflowState)

		if newWf.Jobs[0].Title != "Test Title" {
			t.Errorf("Update with TitleFetchResult did not set job title. Got %s, want %s", newWf.Jobs[0].Title, "Test Title")
		}
		if newWf.Jobs[0].Status != "downloading_subtitles" {
			t.Errorf("Update with TitleFetchResult did not set job status. Got %s, want %s", newWf.Jobs[0].Status, "downloading_subtitles")
		}
		if newWf.CurrentStage != "downloading" {
			t.Errorf("Update with TitleFetchResult did not set workflow stage. Got %s, want %s", newWf.CurrentStage, "downloading")
		}
		if cmd == nil {
			t.Error("Update with TitleFetchResult should return a command to download subtitles")
		}
	})

	t.Run("failed title fetch", func(t *testing.T) {
		wf := newTestWorkflowState([]string{"http://example.com/video1"})
		wf.CurrentJobIndex = 0
		wf.Jobs[0].Status = "fetching_title"
		wf.CurrentStage = "fetching_title"

		testErr := errors.New("fetch failed")
		msg := TitleFetchResult{URL: "http://example.com/video1", Title: "", Err: testErr}
		newWfModel, cmd := wf.Update(msg)
		newWf := newWfModel.(WorkflowState)

		if newWf.Jobs[0].Error != testErr {
			t.Errorf("Update with failed TitleFetchResult did not set job error. Got %v, want %v", newWf.Jobs[0].Error, testErr)
		}
		if newWf.Jobs[0].Status != "failed" {
			t.Errorf("Update with failed TitleFetchResult did not set job status. Got %s, want %s", newWf.Jobs[0].Status, "failed")
		}
		// This should trigger handleJobCompletion, which might start next job or finalize
		// For a single job that fails at title fetch, it should go to finalizeWorkflow.
		// finalizeWorkflow sets CurrentStage to "completed" (or "failed" if it checks errors).
		// Let's check if CurrentJobIndex was incremented (so it's now >= TotalJobs)
		if newWf.CurrentJobIndex < newWf.TotalJobs {
			t.Errorf("Expected CurrentJobIndex to be >= TotalJobs after single job fails at title, got %d", newWf.CurrentJobIndex)
		}
		if newWf.CurrentStage != "completed" && newWf.CurrentStage != "failed" { // finalizeWorkflow sets this
			t.Errorf("Expected CurrentStage to be 'completed' or 'failed' after single job fails at title, got %s", newWf.CurrentStage)
		}
		if cmd == nil {
			t.Error("Update with failed TitleFetchResult should return a command (from finalize or next job)")
		}
	})

	t.Run("title fetch result for wrong URL", func(t *testing.T) {
		wf := newTestWorkflowState([]string{"http://example.com/video1"})
		wf.CurrentJobIndex = 0
		wf.Jobs[0].Status = "fetching_title"
		initialState := wf // Keep a copy for comparison

		msg := TitleFetchResult{URL: "http://some.other/video", Title: "Wrong Title", Err: nil}
		newWfModel, cmd := wf.Update(msg)

		if !reflect.DeepEqual(newWfModel.(WorkflowState).Jobs[0], initialState.Jobs[0]) {
			t.Errorf("Update with wrong URL TitleFetchResult should not change job state. Got %+v, want %+v", newWfModel.(WorkflowState).Jobs[0], initialState.Jobs[0])
		}
		if cmd != nil {
			t.Errorf("Update with wrong URL TitleFetchResult should not return a command. Got %T", cmd)
		}
	})
}

func TestWorkflowState_Update_DownloadCompletedMsg(t *testing.T) {
	wf := newTestWorkflowState([]string{"http://example.com/video1"})
	wf.CurrentJobIndex = 0
	wf.Jobs[0] = TranscriptJob{URL: "http://example.com/video1", Title: "Test Video", Status: "downloading_subtitles"}
	wf.CurrentStage = "downloading"

	t.Run("successful download", func(t *testing.T) {
		msg := DownloadCompletedMsg{Err: nil}
		newWfModel, cmd := wf.Update(msg)
		newWf := newWfModel.(WorkflowState)

		if newWf.Jobs[0].Status != "processing_transcript" {
			t.Errorf("Job status after download: got %s, want %s", newWf.Jobs[0].Status, "processing_transcript")
		}
		if newWf.CurrentStage != "processing" {
			t.Errorf("Workflow stage after download: got %s, want %s", newWf.CurrentStage, "processing")
		}
		if cmd == nil {
			t.Error("Expected a command for processing transcript")
		}
		// Check progress update
		// progressCmd := cmd.(tea.BatchMsg)[1].(tea.Cmd) // Assuming it's the SetProgress one
		// Here we'd need a way to inspect the command or mock ProgressView.SetProgress
	})

	t.Run("failed download", func(t *testing.T) {
		// Reset state for this sub-test if necessary, or use a fresh wf if tests modify it too much
		wfFail := newTestWorkflowState([]string{"http://example.com/video1"})
		wfFail.CurrentJobIndex = 0
		wfFail.Jobs[0] = TranscriptJob{URL: "http://example.com/video1", Title: "Test Video", Status: "downloading_subtitles"}
		wfFail.CurrentStage = "downloading"

		testErr := errors.New("download error")
		msg := DownloadCompletedMsg{Err: testErr}
		newWfModel, _ := wfFail.Update(msg)
		newWf := newWfModel.(WorkflowState)

		if newWf.Jobs[0].Status != "failed" {
			t.Errorf("Job status after failed download: got %s, want %s", newWf.Jobs[0].Status, "failed")
		}
		if newWf.Jobs[0].Error != testErr {
			t.Errorf("Job error after failed download: got %v, want %v", newWf.Jobs[0].Error, testErr)
		}
		// Check if it moves to next job or finalizes
		if newWf.CurrentJobIndex < newWf.TotalJobs {
			t.Errorf("Expected CurrentJobIndex to be >= TotalJobs after single job fails download, got %d", newWf.CurrentJobIndex)
		}
	})
}

func TestWorkflowState_Update_ProcessingCompletedMsg(t *testing.T) {
	wf := newTestWorkflowState([]string{"http://example.com/video1"})
	wf.CurrentJobIndex = 0
	wf.Jobs[0] = TranscriptJob{URL: "http://example.com/video1", Title: "Test Video", Status: "processing_transcript"}
	wf.CurrentStage = "processing"

	t.Run("successful processing", func(t *testing.T) {
		msg := ProcessingCompletedMsg{Err: nil}
		newWfModel, _ := wf.Update(msg)
		newWf := newWfModel.(WorkflowState)

		if newWf.Jobs[0].Status != "completed" {
			t.Errorf("Job status after processing: got %s, want %s", newWf.Jobs[0].Status, "completed")
		}
		// CurrentJobIndex should advance
		if newWf.CurrentJobIndex != 1 {
			t.Errorf("CurrentJobIndex after processing: got %d, want %d", newWf.CurrentJobIndex, 1)
		}
		if newWf.CurrentStage != "completed" { // finalizeWorkflow should set this
			t.Errorf("Workflow stage after processing single job: got %s, want %s", newWf.CurrentStage, "completed")
		}
	})

	t.Run("failed processing", func(t *testing.T) {
		wfFail := newTestWorkflowState([]string{"http://example.com/video1"})
		wfFail.CurrentJobIndex = 0
		wfFail.Jobs[0] = TranscriptJob{URL: "http://example.com/video1", Title: "Test Video", Status: "processing_transcript"}
		wfFail.CurrentStage = "processing"

		testErr := errors.New("processing error")
		msg := ProcessingCompletedMsg{Err: testErr}
		newWfModel, _ := wfFail.Update(msg)
		newWf := newWfModel.(WorkflowState)

		if newWf.Jobs[0].Status != "failed" {
			t.Errorf("Job status after failed processing: got %s, want %s", newWf.Jobs[0].Status, "failed")
		}
		// CurrentJobIndex should advance
		if newWf.CurrentJobIndex != 1 {
			t.Errorf("CurrentJobIndex after failed processing: got %d, want %d", newWf.CurrentJobIndex, 1)
		}
		if newWf.CurrentStage != "completed" { // finalizeWorkflow should set this
			t.Errorf("Workflow stage after failed processing single job: got %s, want %s", newWf.CurrentStage, "completed")
		}
	})
}

func TestWorkflowState_Update_KeyMsg(t *testing.T) {
	wf := newTestWorkflowState([]string{"http://example.com/video1"})

	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := wf.Update(ctrlCMsg)

	if cmd == nil { // tea.Quit is a function, so it won't be nil.
		t.Fatalf("Expected a tea.Quit command on Ctrl+C, got nil")
	}
	// To truly check if it's tea.Quit, you might need to compare func pointers or use a sentinel error/type
	// if tea.Quit were, for example, a specific error. This is a limitation of testing functions directly.
	// A common pattern is for tea.Quit to be a specific function value.
	// rf := reflect.ValueOf(cmd)
	// rq := reflect.ValueOf(tea.Quit)
	// if rf.Pointer() != rq.Pointer() {
	// 	t.Error("Expected tea.Quit command on Ctrl+C")
	// }
	// The above reflection is one way, but can be brittle. Assume non-nil is a good first step.
}

func TestProcessTranscript_Stub(t *testing.T) {
	cmd := ProcessTranscript("raw", "cleaned")
	if cmd == nil {
		t.Fatal("ProcessTranscript should return a command")
	}
	msg := cmd() // Execute the command
	if _, ok := msg.(ProcessingCompletedMsg); !ok {
		t.Errorf("ProcessTranscript cmd() did not return ProcessingCompletedMsg, got %T", msg)
	}
	// Check if error is nil as per stub
	if msg.(ProcessingCompletedMsg).Err != nil {
		t.Errorf("ProcessTranscript stub returned error: %v", msg.(ProcessingCompletedMsg).Err)
	}
}

func TestWorkflowState_View(t *testing.T) {
	t.Run("no jobs", func(t *testing.T) {
		wf := newTestWorkflowState([]string{})
		view := wf.View()
		if view != "No URLs provided. Exiting." {
			t.Errorf("View for no jobs: got %q, want %q", view, "No URLs provided. Exiting.")
		}
	})

	t.Run("fetching title state", func(t *testing.T) {
		wf := newTestWorkflowState([]string{"http://example.com"})
		// Init normally sets this up, but we can set manually for focused View test
		wf.CurrentJobIndex = 0
		wf.Jobs[0].Status = "fetching_title"
		wf.Jobs[0].Title = "" // Title is initially empty when fetching
		wf.CurrentStage = "fetching_title"
		view := wf.View()
		// Expected view now directly uses "Fetching title..." due to display.go change
		expectedViewSegment := "[1/1] Fetching title..."
		if !strings.Contains(view, expectedViewSegment) {
			t.Errorf("View for fetching_title: got %q, want to contain %q", view, expectedViewSegment)
		}
	})

	t.Run("downloading state", func(t *testing.T) {
		wf := newTestWorkflowState([]string{"http://example.com"})
		wf.CurrentJobIndex = 0
		wf.Jobs[0].Status = "downloading_subtitles"
		wf.Jobs[0].Title = "My Video"
		wf.CurrentStage = "downloading"
		view := wf.View()
		if !strings.Contains(view, "[1/1] Downloading subtitles for: My Video") {
			t.Errorf("View for downloading: got %q, want to contain %q", view, "[1/1] Downloading subtitles for: My Video")
		}
	})

	t.Run("completed state", func(t *testing.T) {
		wf := newTestWorkflowState([]string{"http://example.com"})
		wf.CurrentJobIndex = 1 // All jobs done
		wf.TotalJobs = 1
		wf.CurrentStage = "completed"
		view := wf.View()
		if !strings.Contains(view, "✅ All done!") {
			t.Errorf("View for completed: got %q, want to contain %q", view, "✅ All done!")
		}
	})

	t.Run("single job failed state", func(t *testing.T) {
		wf := newTestWorkflowState([]string{"http://example.com"})
		wf.CurrentJobIndex = 0
		wf.Jobs[0].Status = "failed"
		wf.Jobs[0].Title = "Failed Video"
		wf.Jobs[0].Error = errors.New("epic fail")
		wf.CurrentStage = "failed"
		view := wf.View()
		if !strings.Contains(view, "❌ Error processing Failed Video:") {
			t.Errorf("View for failed job: got %q, want to contain %q", view, "❌ Error processing Failed Video:")
		}
		if !strings.Contains(view, "epic fail") {
			t.Errorf("View for failed job: got %q, want to contain %q", view, "epic fail")
		}
	})
}
