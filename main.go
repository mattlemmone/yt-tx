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
	cleanDirs  bool // Flag to control whether to clean directories before processing
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

// cleanDirectories removes all files from the directories to avoid processing old files
func cleanDirectories() error {
	if err := os.RemoveAll(rawVTTDir); err != nil {
		return err
	}
	if err := ensureDirectories(); err != nil {
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
	readyToQuit     bool              // Added to manage final quit sequence
	processedFiles  []string          // Track files processed in current run
	videoTitles     map[string]string // Map of video URLs to their titles
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
		processedFiles:  []string{},
		videoTitles:     make(map[string]string),
	}
}

func (m model) Init() tea.Cmd {
	// First fetch the title, then start the subtitle download and progress bar animation
	return tea.Batch(
		m.fetchTitleCmd(),
		func() tea.Msg { return progress.FrameMsg{} },
	)
}

// fetchVideoTitle uses yt-dlp to get the video title
func fetchVideoTitle(url string) string {
	cmd := exec.Command("yt-dlp", "--quiet", "--print", "title", url)
	output, err := cmd.Output()
	if err != nil {
		return extractVideoID(url) // Fallback to ID if title fetch fails
	}
	title := strings.TrimSpace(string(output))
	if title == "" {
		return extractVideoID(url) // Fallback if title is empty
	}
	return title
}

// fetchTitleCmd creates a command to fetch the video title
func (m model) fetchTitleCmd() tea.Cmd {
	return func() tea.Msg {
		url := m.urls[m.currentVideoIdx]
		title := fetchVideoTitle(url)
		return struct {
			url   string
			title string
		}{url, title}
	}
}

// fetchCmd downloads subtitles for the current video.
func (m model) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		url := m.urls[m.currentVideoIdx]
		// Don't add .vtt extension - yt-dlp will add extensions automatically
		outputTemplate := filepath.Join(rawVTTDir, "%(id)s--%(title).200s")

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

	// Remove all .vtt and .en extensions - they may appear in various combinations
	title = strings.TrimSuffix(title, ".vtt")
	title = strings.TrimSuffix(title, ".en")
	title = strings.TrimSuffix(title, ".vtt") // Handle potential double .vtt

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

// extractVideoID extracts the video ID from a YouTube URL
func extractVideoID(url string) string {
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
		vidID := url[idx+9:] // Get everything after youtu.be/
		if questionIdx := strings.Index(vidID, "?"); questionIdx != -1 {
			// Cut at question mark if there are query parameters
			return vidID[:questionIdx]
		}
		return vidID
	}

	// If we can't extract cleanly, just return a shortened URL
	if len(url) > 30 {
		return url[:27] + "..."
	}
	return url
}

// renderInProgress renders the UI when processing is in progress.
func (m model) renderInProgress() string {
	// Always show progress information, regardless of file availability
	header := fmt.Sprintf("[%d/%d] ", m.currentVideoIdx+1, m.total)

	// Find newest VTT files
	pattern := filepath.Join(rawVTTDir, "*.vtt")
	files, _ := filepath.Glob(pattern)

	// Add current task description
	if m.stage == "fetch" {
		currentURL := m.urls[m.currentVideoIdx]
		title, exists := m.videoTitles[currentURL]
		if !exists {
			// If we don't have the title yet, just show fetching title message
			header += "Fetching title..."
		} else {
			// Once we have the title, show downloading subtitles message
			header += fmt.Sprintf("Downloading subtitles for: %s", title)
		}
	} else if m.stage == "process" {
		if m.currentVideoIdx < len(files) && len(files) > 0 {
			displayTitle := extractDisplayTitle(files[len(files)-1])
			header += fmt.Sprintf("Processing %s...", displayTitle)
		} else {
			header += "Processing..."
		}
	}

	// Always show progress bar
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
		// Find newest VTT files in the raw directory (most recently downloaded)
		pattern := filepath.Join(rawVTTDir, "*.vtt")
		files, err := filepath.Glob(pattern)
		if err != nil {
			return collapseDoneMsg{err}
		}

		if len(files) == 0 {
			return collapseDoneMsg{fmt.Errorf("no VTT files found")}
		}

		// Find the most recently modified file, which should be from the current run
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

		if newestFile == "" {
			return collapseDoneMsg{fmt.Errorf("couldn't determine newest VTT file")}
		}

		outPath := getCleanedOutputPath(newestFile)

		// Clean and dedupe the VTT content
		output, err := cleanAndDedupeVTT(newestFile)
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

	// Not done yet, set stage for fetching title first
	m.stage = "fetch"

	// Calculate percentage based on completed videos
	percentComplete := float64(m.currentVideoIdx) / float64(m.total)
	setPercentCmd := m.progress.SetPercent(percentComplete)

	// Prepare batch of commands - first fetch title, then continue
	var cmds []tea.Cmd
	cmds = append(cmds, m.fetchTitleCmd(), setPercentCmd)

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

	// Handle title fetch results
	case struct {
		url   string
		title string
	}:
		m.videoTitles[msg.url] = msg.title
		// After getting the title, start the fetch process
		var cmds []tea.Cmd
		cmds = append(cmds, m.fetchCmd())

		// Continue animating progress bar
		updatedProgressModel, progressCmd := m.progress.Update(progress.FrameMsg{})
		m.progress = updatedProgressModel.(progress.Model)
		if progressCmd != nil {
			cmds = append(cmds, progressCmd)
		}
		return m, tea.Batch(cmds...)

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
	flag.BoolVar(&cleanDirs, "clean", true, "Clean directories before processing")
	flag.Parse()

	urls := flag.Args()
	if len(urls) == 0 {
		fmt.Println("Usage: transcript-cli [flags] <youtube-url> [<youtube-url>...]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Clean directories if flag is set to avoid processing old files
	if cleanDirs {
		if err := cleanDirectories(); err != nil {
			fmt.Printf("Error cleaning directories: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Just ensure directories exist
		if err := ensureDirectories(); err != nil {
			fmt.Printf("Error creating directories: %v\n", err)
			os.Exit(1)
		}
	}

	p := tea.NewProgram(initialModel(urls))
	p.Run()
}
