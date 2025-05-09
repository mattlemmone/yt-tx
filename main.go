package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
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

// model holds the application state
type model struct {
	urls            []string
	currentVideoIdx int
	total           int
	stage           string
	spinner         spinner.Model
	progress        progress.Model
	readyToQuit     bool // Added to manage final quit sequence
}

func initialModel(urls []string) model {
	sp := spinner.New(spinner.WithSpinner(spinner.Moon))
	pg := progress.New(progress.WithDefaultGradient())
	return model{
		urls:            urls,
		currentVideoIdx: 0,
		total:           len(urls),
		stage:           "fetch",
		spinner:         sp,
		progress:        pg,
		readyToQuit:     false, // Explicitly initialize
	}
}

func (m model) Init() tea.Cmd {
	// start spinner, first fetch, and progress bar animation
	var cmds []tea.Cmd
	cmds = append(cmds, spinner.Tick, m.fetchCmd())

	updatedProgressModel, progressCmd := m.progress.Update(progress.FrameMsg{})
	m.progress = updatedProgressModel.(progress.Model)
	if progressCmd != nil {
		cmds = append(cmds, progressCmd)
	}
	return tea.Batch(cmds...)
}

func (m model) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		url := m.urls[m.currentVideoIdx]
		err := exec.Command("yt-dlp", "--quiet", url,
			"--skip-download", "--write-sub", "--write-auto-sub",
			"--sub-lang", "en", "--convert-subs", "vtt",
			"--restrict-filenames",
			"-o", filepath.Join(rawVTTDir, "%(id)s--%(title).200s.vtt"),
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

func isNumber(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isTimestamp(s string) bool {
	// Matches 00:00:00.000 --> 00:00:00.000
	return len(s) >= 29 && s[2] == ':' && s[5] == ':' && s[8] == '.' && strings.Contains(s, "-->")
}

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
	return filepath.Join(cleanedDir, title+".clean.txt")
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

func (m model) processCmd() tea.Cmd {
	return func() tea.Msg {
		pattern := filepath.Join(rawVTTDir, "*.vtt")
		files, _ := filepath.Glob(pattern)
		if m.currentVideoIdx >= len(files) {
			return collapseDoneMsg{fmt.Errorf("no VTT file at index %d", m.currentVideoIdx)}
		}

		file := files[m.currentVideoIdx]

		outPath := getCleanedOutputPath(file)
		output, err := cleanAndDedupeVTT(file)
		if err != nil {
			return collapseDoneMsg{err}
		}
		err = os.WriteFile(outPath, []byte(output), 0644)
		return collapseDoneMsg{err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If we are ready to quit, do it now.
	if m.readyToQuit {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// Handle progress bar animation
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case fetchDoneMsg:
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
		cmds = append(cmds, m.processCmd(), spinner.Tick, setPercentCmd)
		if progressAnimCmd != nil {
			cmds = append(cmds, progressAnimCmd)
		}
		return m, tea.Batch(cmds...)

	case collapseDoneMsg:
		m.currentVideoIdx++

		if m.currentVideoIdx >= m.total {
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

		// Not done yet, set stage for next fetch
		m.stage = "fetch"

		// Calculate percentage based on completed videos
		percentComplete := float64(m.currentVideoIdx) / float64(m.total)
		setPercentCmd := m.progress.SetPercent(percentComplete)

		// Prepare batch of commands
		var cmds []tea.Cmd
		cmds = append(cmds, m.fetchCmd(), spinner.Tick, setPercentCmd)

		// Update animation and get the latest model state for progress bar
		updatedProgressModel, progressAnimCmd := m.progress.Update(progress.FrameMsg{})
		m.progress = updatedProgressModel.(progress.Model)
		if progressAnimCmd != nil {
			cmds = append(cmds, progressAnimCmd)
		}
		return m, tea.Batch(cmds...)

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

func (m model) View() string {
	if m.stage == "done" {
		return "✅ All done!\n" + m.progress.ViewAs(1.0) + "\n"
	}

	pattern := filepath.Join(rawVTTDir, "*.vtt")
	files, _ := filepath.Glob(pattern)
	if m.currentVideoIdx >= len(files) || len(files) == 0 {
		return fmt.Sprintf("[%d/%d] Waiting for files...\n%s\n", m.currentVideoIdx+1, m.total, m.spinner.View())
	}
	file := files[m.currentVideoIdx]
	base := filepath.Base(file)
	titleParts := strings.SplitN(base, "--", 2)
	var displayTitle string
	if len(titleParts) > 1 {
		displayTitle = titleParts[1]
	} else {
		displayTitle = base
	}
	header := fmt.Sprintf("[%d/%d] Processing %s...", m.currentVideoIdx+1, m.total, displayTitle)
	return fmt.Sprintf("%s\n%s\n%s\n", header, m.spinner.View(), m.progress.View())
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

	os.MkdirAll(rawVTTDir, 0755)
	os.MkdirAll(cleanedDir, 0755)

	p := tea.NewProgram(initialModel(urls))
	p.Run()
}
