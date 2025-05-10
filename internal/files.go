package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var invalidPathChars = regexp.MustCompile(`[^a-zA-Z0-9-_\.]+`)
var multipleSeparators = regexp.MustCompile(`--+`)
var multipleUnderscores = regexp.MustCompile(`__+`)
var multipleDots = regexp.MustCompile(`\\.+`)

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

// SanitizeFilename replaces or removes characters that are typically problematic in filenames.
// This is a basic version; more robust sanitization might be needed depending on OS and filesystems.
func SanitizeFilename(name string) string {
	// Replace common separators or problematic chars with hyphen
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "\"", "-") // Replace literal double quotes
	name = strings.ReplaceAll(name, "'", "-")  // Single quotes
	name = strings.ReplaceAll(name, "?", "")
	name = strings.ReplaceAll(name, "*", "")

	// Use regex for broader sanitization (remove anything not in the allowed set)
	// This regex allows alphanumeric, hyphen, underscore, dot.
	sanitized := invalidPathChars.ReplaceAllString(name, "")

	// Reduce multiple hyphens/underscores/dots to single ones
	sanitized = multipleSeparators.ReplaceAllString(sanitized, "-")
	sanitized = multipleUnderscores.ReplaceAllString(sanitized, "_")
	sanitized = multipleDots.ReplaceAllString(sanitized, ".")

	// Trim leading/trailing hyphens or underscores
	sanitized = strings.Trim(sanitized, "-_")

	// Limit length (optional, but good practice)
	maxLength := 100 // Arbitrary limit
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
		// Ensure it doesn't end mid-UTF8 char if cutting aggressively; simple slice is okay for basic ASCII/common UTF-8
		sanitized = strings.TrimRight(sanitized, "-_") // Clean up again if cut left a trailing hyphen
	}
	if sanitized == "" {
		return "default_filename"
	}
	return sanitized
}

// GetLocalVTTPathByVideoID constructs the path for a raw VTT file based on its video ID.
// Assumes VTT files are named <videoID>.vtt.
func GetLocalVTTPathByVideoID(videoID string, rawVTTDir string) (string, error) {
	if videoID == "" {
		return "", fmt.Errorf("videoID cannot be empty when constructing raw VTT path")
	}
	// No sanitization needed for videoID as it's usually filesystem-safe.
	// yt-dlp saves it as <videoID>.vtt
	return filepath.Join(rawVTTDir, videoID+".vtt"), nil
}

// GetCleanedFilePathByTitle constructs the path for a cleaned transcript file based on the video title.
func GetCleanedFilePathByTitle(videoTitle string, cleanedDir string) (string, error) {
	if videoTitle == "" {
		return "", fmt.Errorf("videoTitle cannot be empty when constructing cleaned file path")
	}
	safeTitle := SanitizeFilename(videoTitle)
	return filepath.Join(cleanedDir, safeTitle+".txt"), nil
}
