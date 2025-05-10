package internal

import (
	"fmt"
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
func DownloadSubtitles(url, outputDir string) error {
	// Don't add .vtt extension - yt-dlp will add extensions automatically
	outputTemplate := filepath.Join(outputDir, "%(id)s--%(title).200s")

	return exec.Command("yt-dlp", "--quiet", url,
		"--skip-download", "--write-sub", "--write-auto-sub",
		"--sub-lang", "en", "--convert-subs", "vtt",
		"--restrict-filenames",
		"-o", outputTemplate,
	).Run()
}

// ExtractVideoID extracts the video ID from a YouTube URL
func ExtractVideoID(url string) string {
	// Check for standard YouTube URL format (v= parameter)
	if idx := strings.Index(url, "v="); idx != -1 {
		vidID := url[idx+2:] // Get everything after v=
		if ampIdx := strings.Index(vidID, "&"); ampIdx != -1 {
			// Cut at first ampersand if there are other parameters
			return vidID[:ampIdx]
		}
		return vidID
	}

	// Check for youtu.be format
	if idx := strings.Index(url, "youtu.be/"); idx != -1 {
		vidID := url[idx+len("youtu.be/"):]
		if questionIdx := strings.Index(vidID, "?"); questionIdx != -1 {
			// Cut at question mark if there are query parameters
			return vidID[:questionIdx]
		}
		return vidID
	}

	// Check for embed format
	if idx := strings.Index(url, "/embed/"); idx != -1 {
		vidID := url[idx+len("/embed/"):]
		if questionIdx := strings.Index(vidID, "?"); questionIdx != -1 {
			return vidID[:questionIdx]
		}
		return vidID
	}

	// If we can't extract cleanly, just return a shortened URL or the original if short
	if len(url) > 30 {
		return url[:27] + "..."
	}
	return url
}

var langAndVttExtRegex = regexp.MustCompile(`(?:\.[a-zA-Z]{2,3})?\.vtt$`) // Matches .vtt and optional .lang.vtt

// ExtractDisplayTitle gets a user-friendly title from a filename by stripping known extensions.
// It does not handle splitting of ID--Title structures; that should be done by the caller if needed.
func ExtractDisplayTitle(filename string) string {
	return langAndVttExtRegex.ReplaceAllString(filename, "")
}
