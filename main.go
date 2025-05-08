package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// model holds the application state
type model struct {
	urls    []string
	idx     int
	total   int
	stage   string
	spinner spinner.Model
}

func initialModel(urls []string) model {
	sp := spinner.New(spinner.WithSpinner(spinner.Moon))
	return model{
		urls:    urls,
		idx:     0,
		total:   len(urls),
		stage:   "fetch",
		spinner: sp,
	}
}

func (m model) Init() tea.Cmd {
	// start spinner and first fetch
	return tea.Batch(spinner.Tick, m.fetchCmd())
}

func (m model) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		url := m.urls[m.idx]
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

func (m model) processCmd() tea.Cmd {
	return func() tea.Msg {
		pattern := filepath.Join(rawVTTDir, "*.vtt")
		files, _ := filepath.Glob(pattern)
		file := files[m.idx]
		os.MkdirAll(cleanedDir, 0755)
		base := filepath.Base(file)

		titleParts := strings.SplitN(base, "--", 2)
		var title string
		if len(titleParts) > 1 {
			title = titleParts[1]
		} else {
			title = base
		}
		title = strings.TrimSuffix(title, ".vtt")
		outPath := filepath.Join(cleanedDir, title+".clean.txt")
		// Clean and dedupe in one go
		input, err := os.ReadFile(file)
		if err != nil {
			return collapseDoneMsg{err}
		}
		lines := strings.Split(string(input), "\n")
		cleaned := cleanTranscriptLines(lines)
		final := dedupeLines(cleaned)
		err = os.WriteFile(outPath, []byte(strings.Join(final, "\n")), 0644)
		return collapseDoneMsg{err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case fetchDoneMsg:
		m.stage = "process"
		return m, tea.Batch(m.processCmd(), spinner.Tick)

	case collapseDoneMsg:
		m.idx++
		if m.idx >= m.total {
			m.stage = "done"
			return m, tea.Quit
		}
		m.stage = "fetch"
		return m, tea.Batch(m.fetchCmd(), spinner.Tick)

	default:
		return m, nil
	}
}

func (m model) View() string {
	if m.stage == "done" {
		return "✅ All done!\n"
	}

	pattern := filepath.Join(rawVTTDir, "*.vtt")
	files, _ := filepath.Glob(pattern)
	if m.idx >= len(files) || len(files) == 0 {
		return fmt.Sprintf("[%d/%d] Waiting for files...\n%s\n", m.idx+1, m.total, m.spinner.View())
	}
	file := files[m.idx]
	base := filepath.Base(file)
	titleParts := strings.SplitN(base, "--", 2)
	var displayTitle string
	if len(titleParts) > 1 {
		displayTitle = titleParts[1]
	} else {
		displayTitle = base
	}
	header := fmt.Sprintf("[%d/%d] Processing %s...", m.idx+1, m.total, displayTitle)
	return fmt.Sprintf("%s\n%s\n", header, m.spinner.View())
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
