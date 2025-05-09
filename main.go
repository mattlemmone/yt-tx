package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultRawVTTDir  = "raw_vtt"
	defaultCleanedDir = "cleaned"
)

var (
	rawVTTDir  string
	cleanedDir string
)

// Messages to signal task completion
type fetchDoneMsg struct{ err error }
type collapseDoneMsg struct{ err error }
type finalRenderMsg struct{}

// ensureDirectories ensures that the required directories exist.
func ensureDirectories() error {
	if err := os.MkdirAll(rawVTTDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(cleanedDir, 0755); err != nil {
		return err
	}
	return nil
}

// model holds the application state
type model struct {
	urls            []string
	currentVideoIdx int
	total           int
	stage           string
	progress        progress.Model
	readyToQuit     bool // Added to manage final quit sequence
}

func initialModel(urls []string) model {
	pg := progress.New(progress.WithDefaultGradient())
	return model{
		urls:            urls,
		currentVideoIdx: 0,
		total:           len(urls),
		stage:           "fetch",
		progress:        pg,
		readyToQuit:     false, // Explicitly initialize
	}
}

func (m model) Init() tea.Cmd {
	// start first fetch and progress bar animation
	var cmds []tea.Cmd
	cmds = append(cmds, m.fetchCmd())

	updatedProgressModel, progressCmd := m.progress.Update(progress.FrameMsg{})
	m.progress = updatedProgressModel.(progress.Model)
	if progressCmd != nil {
		cmds = append(cmds, progressCmd)
	}
	return tea.Batch(cmds...)
}

// fetchCmd downloads subtitles for the current video.
func (m model) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		url := m.urls[m.currentVideoIdx]
		outputTemplate := filepath.Join(rawVTTDir, "%(id)s--%(title).200s.vtt")

		err := exec.Command("yt-dlp", "--quiet", url,
			"--skip-download", "--write-sub", "--write-auto-sub",
			"--sub-lang", "en", "--convert-subs", "vtt",
			"--restrict-filenames",
			"-o", outputTemplate,
		).Run()

		return fetchDoneMsg{err}
	}
}

// dedupeLines removes consecutive duplicate lines from a slice of strings.
func dedupeLines(lines []string) []string {
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

// isNumber checks if a string consists only of digits.
func isNumber(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// isTimestamp checks if a string looks like a VTT timestamp.
func isTimestamp(s string) bool {
	// Matches 00:00:00.000 --> 00:00:00.000
	return len(s) >= 29 && s[2] == ':' && s[5] == ':' && s[8] == '.' && strings.Contains(s, "-->")
}

// stripHTMLTags removes HTML tags from a string.
func stripHTMLTags(s string) string {
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

// cleanTranscriptLines applies the cleaning logic to a slice of lines.
func cleanTranscriptLines(lines []string) []string {
	var outLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "WEBVTT" {
			continue
		}
		if isNumber(line) {
			continue
		}
		if isTimestamp(line) {
			continue
		}
		line = stripHTMLTags(line)
		if line == "" {
			continue
		}
		outLines = append(outLines, line)
	}
	return outLines
}

// getCleanedOutputPath returns the output path for the cleaned transcript, using only the title part of the filename.
func getCleanedOutputPath(vttPath string) string {
	base := filepath.Base(vttPath)
	titleParts := strings.SplitN(base, "--", 2)
	var title string
	if len(titleParts) > 1 {
		title = titleParts[1]
	} else {
		title = base
	}
	title = strings.TrimSuffix(title, ".vtt")
	return filepath.Join(cleanedDir, title+".txt")
}

// extractDisplayTitle gets a user-friendly title from a filename.
func extractDisplayTitle(filename string) string {
	base := filepath.Base(filename)
	titleParts := strings.SplitN(base, "--", 2)
	if len(titleParts) > 1 {
		return titleParts[1]
	}
	return base
}

// cleanAndDedupeVTT reads a VTT file, cleans and dedupes its lines, and returns the result as a string.
func cleanAndDedupeVTT(vttPath string) (string, error) {
	input, err := os.ReadFile(vttPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(input), "\n")
	cleaned := cleanTranscriptLines(lines)
	final := dedupeLines(cleaned)
	return strings.Join(final, "\n"), nil
}

// renderInProgress renders the UI when processing is in progress.
func (m model) renderInProgress() string {
	pattern := filepath.Join(rawVTTDir, "*.vtt")
	files, _ := filepath.Glob(pattern)
	if m.currentVideoIdx >= len(files) || len(files) == 0 {
		return fmt.Sprintf("[%d/%d] Waiting for files...\n", m.currentVideoIdx+1, m.total)
	}

	displayTitle := extractDisplayTitle(files[m.currentVideoIdx])
	header := fmt.Sprintf("[%d/%d] Processing %s...", m.currentVideoIdx+1, m.total, displayTitle)
	return fmt.Sprintf("%s\n%s\n", header, m.progress.View())
}

func (m model) View() string {
	if m.stage == "done" {
		return "✅ All done!\n" + m.progress.ViewAs(1.0) + "\n"
	}

	return m.renderInProgress()
}

// processCmd processes the downloaded VTT file for the current video.
func (m model) processCmd() tea.Cmd {
	return func() tea.Msg {
		// Find VTT files in the raw directory
		pattern := filepath.Join(rawVTTDir, "*.vtt")
		files, err := filepath.Glob(pattern)
		if err != nil {
			return collapseDoneMsg{err}
		}

		if m.currentVideoIdx >= len(files) {
			return collapseDoneMsg{fmt.Errorf("no VTT file at index %d", m.currentVideoIdx)}
		}

		// Get the current file to process
		file := files[m.currentVideoIdx]
		outPath := getCleanedOutputPath(file)

		// Clean and dedupe the VTT content
		output, err := cleanAndDedupeVTT(file)
		if err != nil {
			return collapseDoneMsg{err}
		}

		// Write the cleaned output
		err = os.WriteFile(outPath, []byte(output), 0644)
		return collapseDoneMsg{err}
	}
}

// handleFetchDone processes the fetchDoneMsg
func (m model) handleFetchDone() (tea.Model, tea.Cmd) {
	m.stage = "process"

	// Each video is 50% for fetch, 50% for process
	percentComplete := float64(m.currentVideoIdx) / float64(m.total)
	// Add half of one video's percentage for the fetch step
	if m.total > 0 {
		percentComplete += 0.5 / float64(m.total)
	}
	setPercentCmd := m.progress.SetPercent(percentComplete)

	// Get the command for progress bar animation and the updated model
	updatedProgressModel, progressAnimCmd := m.progress.Update(progress.FrameMsg{})
	m.progress = updatedProgressModel.(progress.Model)

	var cmds []tea.Cmd
	cmds = append(cmds, m.processCmd(), setPercentCmd)
	if progressAnimCmd != nil {
		cmds = append(cmds, progressAnimCmd)
	}
	return m, tea.Batch(cmds...)
}

// handleProcessDone processes the collapseDoneMsg
func (m model) handleProcessDone() (tea.Model, tea.Cmd) {
	m.currentVideoIdx++

	if m.currentVideoIdx >= m.total {
		return m.handleAllProcessingComplete()
	}

	// Not done yet, set stage for next fetch
	m.stage = "fetch"

	// Calculate percentage based on completed videos
	percentComplete := float64(m.currentVideoIdx) / float64(m.total)
	setPercentCmd := m.progress.SetPercent(percentComplete)

	// Prepare batch of commands
	var cmds []tea.Cmd
	cmds = append(cmds, m.fetchCmd(), setPercentCmd)

	// Update animation and get the latest model state for progress bar
	updatedProgressModel, progressAnimCmd := m.progress.Update(progress.FrameMsg{})
	m.progress = updatedProgressModel.(progress.Model)
	if progressAnimCmd != nil {
		cmds = append(cmds, progressAnimCmd)
	}
	return m, tea.Batch(cmds...)
}

// handleAllProcessingComplete handles when all videos are processed
func (m model) handleAllProcessingComplete() (tea.Model, tea.Cmd) {
	m.stage = "done"
	// Get the command to set progress to 100%
	setPercentCmd := m.progress.SetPercent(1.0)

	// Force the progress bar to process a FrameMsg to update its internal view state
	// and get the latest model
	var cmds []tea.Cmd
	updatedProgressModel, progressUpdateCmd := m.progress.Update(progress.FrameMsg{})
	m.progress = updatedProgressModel.(progress.Model)

	cmds = append(cmds, setPercentCmd) // Add the command to set percentage
	if progressUpdateCmd != nil {
		cmds = append(cmds, progressUpdateCmd)
	}
	// Add command to send finalRenderMsg after progress bar has updated
	cmds = append(cmds, func() tea.Msg { return finalRenderMsg{} })
	return m, tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If we are ready to quit, do it now.
	if m.readyToQuit {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	// Handle progress bar animation
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case fetchDoneMsg:
		return m.handleFetchDone()

	case collapseDoneMsg:
		return m.handleProcessDone()

	case finalRenderMsg: // Added to handle quit after final render
		if m.stage == "done" { // Ensure we are in the done state
			m.readyToQuit = true // Set flag to quit on next Update cycle
			return m, nil        // No command, allow one more render cycle
		}
		return m, nil // Should ideally not be reached if stage isn't "done"

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		default:
			return m, nil
		}

	default:
		return m, nil
	}
}

func main() {
	flag.StringVar(&rawVTTDir, "raw_vtt_dir", defaultRawVTTDir, "Directory for downloaded VTT files")
	flag.StringVar(&cleanedDir, "cleaned_dir", defaultCleanedDir, "Directory for deduplicated transcript files")
	flag.Parse()

	urls := flag.Args()
	if len(urls) == 0 {
		fmt.Println("Usage: transcript-cli [flags] <youtube-url> [<youtube-url>...]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Ensure directories exist before starting the program
	if err := ensureDirectories(); err != nil {
		fmt.Printf("Error creating directories: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(urls))
	p.Run()
}
