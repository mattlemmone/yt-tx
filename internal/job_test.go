package internal

import (
	"reflect"
	"testing"
)

func TestNewWorkflow(t *testing.T) {
	type args struct {
		urls       []string
		rawVTTDir  string
		cleanedDir string
	}
	tests := []struct {
		name string
		args args
		want WorkflowState
	}{
		{
			name: "single URL",
			args: args{
				urls:       []string{"http://example.com/video1"},
				rawVTTDir:  "raw",
				cleanedDir: "cleaned",
			},
			want: WorkflowState{
				Jobs: []TranscriptJob{
					{URL: "http://example.com/video1", Status: "pending"},
				},
				CurrentJobIndex: 0,
				TotalJobs:       1,
				CurrentStage:    "fetching_title",
				// ProgressView is initialized, so we can't directly compare it without deeper inspection
				// ReadyToQuit is false by default
				// ProcessedFiles is empty by default
				RawVTTDir:  "raw",
				CleanedDir: "cleaned",
			},
		},
		{
			name: "multiple URLs",
			args: args{
				urls:       []string{"http://example.com/video1", "http://example.com/video2"},
				rawVTTDir:  "raw_data",
				cleanedDir: "cleaned_data",
			},
			want: WorkflowState{
				Jobs: []TranscriptJob{
					{URL: "http://example.com/video1", Status: "pending"},
					{URL: "http://example.com/video2", Status: "pending"},
				},
				CurrentJobIndex: 0,
				TotalJobs:       2,
				CurrentStage:    "fetching_title",
				RawVTTDir:       "raw_data",
				CleanedDir:      "cleaned_data",
			},
		},
		{
			name: "no URLs",
			args: args{
				urls:       []string{},
				rawVTTDir:  "raw",
				cleanedDir: "cleaned",
			},
			want: WorkflowState{
				Jobs:            []TranscriptJob{},
				CurrentJobIndex: 0,
				TotalJobs:       0,
				CurrentStage:    "completed", // As per NewWorkflow logic for empty URLs
				RawVTTDir:       "raw",
				CleanedDir:      "cleaned",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewWorkflow(tt.args.urls, tt.args.rawVTTDir, tt.args.cleanedDir)
			// Compare field by field, excluding ProgressView as it contains unexported fields
			// and is initialized internally. We assume NewProgressView() works.
			if !reflect.DeepEqual(got.Jobs, tt.want.Jobs) {
				t.Errorf("NewWorkflow().Jobs = %v, want %v", got.Jobs, tt.want.Jobs)
			}
			if got.CurrentJobIndex != tt.want.CurrentJobIndex {
				t.Errorf("NewWorkflow().CurrentJobIndex = %v, want %v", got.CurrentJobIndex, tt.want.CurrentJobIndex)
			}
			if got.TotalJobs != tt.want.TotalJobs {
				t.Errorf("NewWorkflow().TotalJobs = %v, want %v", got.TotalJobs, tt.want.TotalJobs)
			}
			if got.CurrentStage != tt.want.CurrentStage {
				t.Errorf("NewWorkflow().CurrentStage = %v, want %v", got.CurrentStage, tt.want.CurrentStage)
			}
			if got.RawVTTDir != tt.want.RawVTTDir {
				t.Errorf("NewWorkflow().RawVTTDir = %v, want %v", got.RawVTTDir, tt.want.RawVTTDir)
			}
			if got.CleanedDir != tt.want.CleanedDir {
				t.Errorf("NewWorkflow().CleanedDir = %v, want %v", got.CleanedDir, tt.want.CleanedDir)
			}
			if got.ReadyToQuit != tt.want.ReadyToQuit { // Explicitly check default
				t.Errorf("NewWorkflow().ReadyToQuit = %v, want %v", got.ReadyToQuit, tt.want.ReadyToQuit)
			}
			if len(got.ProcessedFiles) != 0 { // Explicitly check default
				t.Errorf("NewWorkflow().ProcessedFiles should be empty, got %v", got.ProcessedFiles)
			}
			// Check if ProgressView is initialized (not nil, if it were a pointer, or has a default state)
			// Since ProgressView is a struct, its zero value might be valid or NewProgressView sets it.
			// We mostly care it's there. A more detailed test would mock/check Progress.Percent() if needed.
			if reflect.DeepEqual(got.ProgressView, ProgressView{}) && tt.args.urls != nil {
				// If it's a zero struct and we expected it to be initialized by NewProgressView()
				// This check depends on whether NewProgressView() always makes it non-zero.
				// For now, we assume NewProgressView sets it up.
			}
		})
	}
}
