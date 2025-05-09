package internal

import (
	"os"
	"path/filepath"
)

// FindNewestFile finds the most recently modified file matching a pattern
func FindNewestFile(pattern string) (string, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", nil
	}

	var newestFile string
	var newestTime int64

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		modTime := info.ModTime().Unix()
		if modTime > newestTime {
			newestFile = file
			newestTime = modTime
		}
	}

	return newestFile, nil
}

// WriteTextFile writes text content to a file
func WriteTextFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadTextFile reads a text file and returns its content
func ReadTextFile(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// EnsureDirectories ensures that the required directories exist.
func EnsureDirectories(rawVTTDir, cleanedDir string) error {
	if err := os.MkdirAll(rawVTTDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(cleanedDir, 0755); err != nil {
		return err
	}
	return nil
}

// CleanDirectories removes all files from the directories to avoid processing old files
func CleanDirectories(rawVTTDir, cleanedDir string) error {
	if err := os.RemoveAll(rawVTTDir); err != nil {
		return err
	}
	if err := os.RemoveAll(cleanedDir); err != nil {
		return err
	}
	if err := EnsureDirectories(rawVTTDir, cleanedDir); err != nil {
		return err
	}
	return nil
}
