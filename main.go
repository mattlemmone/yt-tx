package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
)

// Messages to signal task completion
type fetchDoneMsg struct{ err error }
type cleanDoneMsg struct{ err error }
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
			"-o", filepath.Join("raw_vtt", "%(id)s--%(title).200s.vtt"),
		).Run()
		return fetchDoneMsg{err}
	}
}

func (m model) cleanCmd() tea.Cmd {
	return func() tea.Msg {
		pattern := filepath.Join("raw_vtt", "*.vtt")
		files, _ := filepath.Glob(pattern)
		file := files[m.idx]
		title := strings.SplitN(filepath.Base(file), "--", 2)[1]
		os.MkdirAll("transcripts", 0755)
		err := exec.Command("bash", "-c",
			fmt.Sprintf(
				`sed -E '/^WEBVTT/d; /^[0-9]+$/d; /^[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3} -->/d; s/<[^>]+>//g; /^$/d' '%s' > 'transcripts/%s.txt'`,
				file, title,
			),
		).Run()
		return cleanDoneMsg{err}
	}
}

func (m model) collapseCmd() tea.Cmd {
	return func() tea.Msg {
		pattern := filepath.Join("transcripts", "*.txt")
		files, _ := filepath.Glob(pattern)
		file := files[m.idx]
		os.MkdirAll("cleaned", 0755)
		out := strings.TrimSuffix(filepath.Base(file), ".txt") + ".clean.txt"
		err := exec.Command("bash", "-c",
			fmt.Sprintf(
				`awk 'NF && $0!=prev{print; prev=$0}' '%s' > 'cleaned/%s'`,
				file, out,
			),
		).Run()
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
		m.stage = "clean"
		return m, tea.Batch(m.cleanCmd(), spinner.Tick)

	case cleanDoneMsg:
		m.stage = "collapse"
		return m, tea.Batch(m.collapseCmd(), spinner.Tick)

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

	// Always show a single “Processing” message with the video title
	pattern := filepath.Join("raw_vtt", "*.vtt")
	files, _ := filepath.Glob(pattern)
	file := files[m.idx]
	title := strings.SplitN(filepath.Base(file), "--", 2)[1]

	header := fmt.Sprintf("[%d/%d] Processing %s...", m.idx+1, m.total, title)
	return fmt.Sprintf("%s\n%s\n", header, m.spinner.View())
}

func main() {
	urls := os.Args[1:]
	if len(urls) == 0 {
		fmt.Println("Usage: transcript-cli <youtube-url> [<youtube-url>...]")
		os.Exit(1)
	}

	// Prepare directories
	os.MkdirAll("raw_vtt", 0755)
	os.MkdirAll("transcripts", 0755)
	os.MkdirAll("cleaned", 0755)

	p := tea.NewProgram(initialModel(urls))
	p.Start()
}