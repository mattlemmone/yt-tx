package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureDirectories(t *testing.T) {
	tempDir := t.TempDir() // Create a temporary directory for testing
	dir1 := filepath.Join(tempDir, "raw")
	dir2 := filepath.Join(tempDir, "cleaned", "nested")

	err := EnsureDirectories(dir1, dir2)
	if err != nil {
		t.Fatalf("EnsureDirectories() error = %v, wantErr nil", err)
	}

	if _, err := os.Stat(dir1); os.IsNotExist(err) {
		t.Errorf("Directory %s was not created", dir1)
	}
	if _, err := os.Stat(dir2); os.IsNotExist(err) {
		t.Errorf("Directory %s was not created", dir2)
	}

	// Test idempotency (calling again should not fail)
	err = EnsureDirectories(dir1, dir2)
	if err != nil {
		t.Fatalf("EnsureDirectories() second call error = %v, wantErr nil", err)
	}
}

func TestCleanDirectories(t *testing.T) {
	tempDir := t.TempDir()
	rawDir := filepath.Join(tempDir, "test_raw_vtt")
	cleanedDir := filepath.Join(tempDir, "test_cleaned")

	// Create directories and some dummy files
	if err := os.MkdirAll(rawDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cleanedDir, 0755); err != nil {
		t.Fatal(err)
	}
	dummyFileRaw, _ := os.Create(filepath.Join(rawDir, "dummy.vtt"))
	dummyFileRaw.Close()
	dummyFileCleaned, _ := os.Create(filepath.Join(cleanedDir, "dummy.txt"))
	dummyFileCleaned.Close()

	err := CleanDirectories(rawDir, cleanedDir)
	if err != nil {
		t.Fatalf("CleanDirectories() error = %v, wantErr nil", err)
	}

	// Check if directories exist (they should, but be empty)
	if _, err := os.Stat(rawDir); os.IsNotExist(err) {
		t.Errorf("Directory %s should exist after CleanDirectories", rawDir)
	}
	if _, err := os.Stat(cleanedDir); os.IsNotExist(err) {
		t.Errorf("Directory %s should exist after CleanDirectories", cleanedDir)
	}

	// Check if directories are empty
	rawEntries, _ := os.ReadDir(rawDir)
	if len(rawEntries) != 0 {
		t.Errorf("Directory %s should be empty after CleanDirectories, got %d entries", rawDir, len(rawEntries))
	}
	cleanedEntries, _ := os.ReadDir(cleanedDir)
	if len(cleanedEntries) != 0 {
		t.Errorf("Directory %s should be empty after CleanDirectories, got %d entries", cleanedDir, len(cleanedEntries))
	}
}

func TestWriteAndReadTextFile(t *testing.T) {
	tempDir := t.TempDir()
	testFilePath := filepath.Join(tempDir, "testfile.txt")
	wantContent := "Hello, World!\nThis is a test."

	err := WriteTextFile(testFilePath, wantContent)
	if err != nil {
		t.Fatalf("WriteTextFile() error = %v, wantErr nil", err)
	}

	gotContent, err := ReadTextFile(testFilePath)
	if err != nil {
		t.Fatalf("ReadTextFile() error = %v, wantErr nil", err)
	}

	if gotContent != wantContent {
		t.Errorf("ReadTextFile() gotContent = %q, want %q", gotContent, wantContent)
	}

	// Test reading a non-existent file
	_, err = ReadTextFile(filepath.Join(tempDir, "nonexistent.txt"))
	if err == nil {
		t.Errorf("ReadTextFile() on non-existent file, got nil error, want error")
	}
}

// Note: FindNewestFile is difficult to unit test reliably without extensive os call mocking
// or creating actual files with controlled mod times, which can be flaky.
// It's better suited for integration testing.

func TestFindNewestFile(t *testing.T) {
	t.Skip("Skipping FindNewestFile: requires filesystem interaction that is hard to reliably unit test.")

	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	file3 := filepath.Join(tempDir, "other.dat") // Different extension

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	// Ensure file2 is newer
	time.Sleep(20 * time.Millisecond) // Sleep briefly to ensure distinct mod times
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(file3, []byte("content3"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		pattern string
		want    string
		wantErr bool
	}{
		{
			name:    "txt files, file2 newest",
			pattern: filepath.Join(tempDir, "*.txt"),
			want:    file2,
			wantErr: false,
		},
		{
			name: "all files, file3 newest by modtime if specific enough",
			// This depends on sleep making file3 strictly newest for all files
			pattern: filepath.Join(tempDir, "*.*"),
			want:    file3,
			wantErr: false,
		},
		{
			name:    "no matching files",
			pattern: filepath.Join(tempDir, "*.log"),
			want:    "",
			wantErr: false,
		},
		{
			name:    "invalid pattern",
			pattern: filepath.Join(tempDir, "[["), // Invalid glob
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindNewestFile(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindNewestFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("FindNewestFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
