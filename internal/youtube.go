package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// FetchTitle uses yt-dlp to get the video title
func FetchTitle(url string) (string, error) {
	cmd := exec.Command("yt-dlp", "--quiet", "--print", "title", url)
	output, err := cmd.Output()
	if err != nil {
		// Return an error and an empty title if yt-dlp fails
		// The caller can then decide to use ExtractVideoID as a fallback
		return "", fmt.Errorf("yt-dlp failed to fetch title: %w", err)
	}
	title := strings.TrimSpace(string(output))
	if title == "" {
		// If yt-dlp returns an empty title, this is also an issue
		// Caller can use ExtractVideoID as a fallback
		return "", fmt.Errorf("yt-dlp returned an empty title")
	}
	return title, nil
}

// DownloadSubtitles downloads subtitles for a YouTube video using yt-dlp
// It now accepts videoID to confirm file creation.
func DownloadSubtitles(url, videoID, outputDir string) error {
	// Output template uses video ID for the raw VTT filename for predictability.
	// yt-dlp will add the .vtt extension.
	outputTemplate := filepath.Join(outputDir, "%(id)s")

	cmd := exec.Command("yt-dlp", "--quiet", url,
		"--skip-download", "--write-sub", "--write-auto-sub",
		"--sub-lang", "en", "--convert-subs", "vtt",
		"--restrict-filenames",
		"-o", outputTemplate,
	)
	err := cmd.Run()
	if err != nil {
		return err // yt-dlp command itself failed
	}

	// After yt-dlp command runs, verify the expected file was created
	expectedVTTPath := filepath.Join(outputDir, videoID+".vtt")
	if _, statErr := os.Stat(expectedVTTPath); os.IsNotExist(statErr) {
		// yt-dlp ran successfully but the file doesn't exist.
		// This can happen if no subtitles were found.
		return fmt.Errorf("yt-dlp completed but subtitle file %s was not created (likely no subtitles found for lang 'en')", expectedVTTPath)
	} else if statErr != nil {
		// Some other error trying to stat the file (e.g., permissions)
		return fmt.Errorf("error checking for subtitle file %s after download: %w", expectedVTTPath, statErr)
	}

	return nil // File exists
}

// ExtractVideoID extracts the video ID from a YouTube URL
func ExtractVideoID(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	// Check for standard YouTube URL format (v= parameter)
	if idx := strings.Index(url, "v="); idx != -1 {
		vidID := url[idx+2:] // Get everything after v=
		if ampIdx := strings.Index(vidID, "&"); ampIdx != -1 {
			// Cut at first ampersand if there are other parameters
			return vidID[:ampIdx], nil
		}
		return vidID, nil
	}

	// Check for youtu.be format
	if idx := strings.Index(url, "youtu.be/"); idx != -1 {
		vidID := url[idx+len("youtu.be/"):]
		if questionIdx := strings.Index(vidID, "?"); questionIdx != -1 {
			// Cut at question mark if there are query parameters
			return vidID[:questionIdx], nil
		}
		return vidID, nil
	}

	// Check for embed format
	if idx := strings.Index(url, "/embed/"); idx != -1 {
		vidID := url[idx+len("/embed/"):]
		if questionIdx := strings.Index(vidID, "?"); questionIdx != -1 {
			return vidID[:questionIdx], nil
		}
		return vidID, nil
	}

	// If we can't extract cleanly, it's not a recognized YouTube URL
	return "", fmt.Errorf("not a recognized YouTube URL: %s", url)
}

var langAndVttExtRegex = regexp.MustCompile(`(?:\.[a-zA-Z]{2,3})?\.vtt$`) // Matches .vtt and optional .lang.vtt

// ExtractDisplayTitle gets a user-friendly title from a filename by stripping known extensions.
// It does not handle splitting of ID--Title structures; that should be done by the caller if needed.
func ExtractDisplayTitle(filename string) string {
	return langAndVttExtRegex.ReplaceAllString(filename, "")
}
