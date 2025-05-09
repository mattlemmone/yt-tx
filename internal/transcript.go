package internal

import (
	"path/filepath"
	"strings"
)

// DedupeLines removes consecutive duplicate lines from a slice of strings.
func DedupeLines(lines []string) []string {
	var outLines []string
	prev := ""
	for _, line := range lines {
		if line != "" && line != prev {
			outLines = append(outLines, line)
			prev = line
		}
	}
	return outLines
}

// IsNumber checks if a string consists only of digits.
func IsNumber(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// IsTimestamp checks if a string looks like a VTT timestamp.
func IsTimestamp(s string) bool {
	// Matches 00:00:00.000 --> 00:00:00.000
	return len(s) >= 29 && s[2] == ':' && s[5] == ':' && s[8] == '.' && strings.Contains(s, "-->")
}

// StripHTMLTags removes HTML tags from a string.
func StripHTMLTags(s string) string {
	var out strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// RemoveVTTArtifacts applies the cleaning logic to a slice of lines to remove VTT artifacts.
func RemoveVTTArtifacts(lines []string) []string {
	var outLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "WEBVTT" {
			continue
		}
		if IsNumber(line) {
			continue
		}
		if IsTimestamp(line) {
			continue
		}
		line = StripHTMLTags(line)
		if line == "" {
			continue
		}
		outLines = append(outLines, line)
	}
	return outLines
}

// CleanVTTFile reads a VTT file, cleans and dedupes its lines, and returns the result as a string.
func CleanVTTFile(vttPath string) (string, error) {
	content, err := ReadTextFile(vttPath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(content, "\n")
	cleaned := RemoveVTTArtifacts(lines)
	final := DedupeLines(cleaned)
	return strings.Join(final, "\n"), nil
}

// SaveCleanedTranscript processes a VTT file and outputs a cleaned text file
func SaveCleanedTranscript(vttPath, cleanedDir string) error {
	output, err := CleanVTTFile(vttPath)
	if err != nil {
		return err
	}

	outPath := GetOutputFilePath(vttPath, cleanedDir)
	return WriteTextFile(outPath, output)
}

// GetNewestVTTPattern returns a glob pattern for finding VTT files
func GetNewestVTTPattern(rawVTTDir string) string {
	return filepath.Join(rawVTTDir, "*.vtt")
}

// GetOutputFilePath returns the output path for the cleaned transcript.
func GetOutputFilePath(vttPath, cleanedDir string) string {
	base := filepath.Base(vttPath)
	titleParts := strings.SplitN(base, "--", 2)
	var title string
	if len(titleParts) > 1 {
		title = titleParts[1]
	} else {
		title = base
	}

	// Remove all .vtt and .en extensions - they may appear in various combinations
	title = strings.TrimSuffix(title, ".vtt")
	title = strings.TrimSuffix(title, ".en")
	title = strings.TrimSuffix(title, ".vtt") // Handle potential double .vtt

	return filepath.Join(cleanedDir, title+".txt")
}
