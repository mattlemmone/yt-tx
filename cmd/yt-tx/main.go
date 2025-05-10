package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattlemmone/yt-tx/internal"
)

const (
	defaultRawVTTDir  = "raw_vtt"
	defaultCleanedDir = "cleaned"
)

// TranscriptApp wraps the workflow
type TranscriptApp struct {
	workflow internal.WorkflowState
}

// Init initializes the application
func (a TranscriptApp) Init() tea.Cmd {
	return a.workflow.Init()
}

// Update updates the application state
func (a TranscriptApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedWorkflow, cmd := a.workflow.Update(msg)
	a.workflow = updatedWorkflow.(internal.WorkflowState)
	return a, cmd
}

// View renders the application state
func (a TranscriptApp) View() string {
	return a.workflow.View()
}

func main() {
	// Parse command line flags
	var (
		rawVTTDir  string
		cleanedDir string
		cleanDirs  bool
	)

	flag.StringVar(&rawVTTDir, "raw_vtt_dir", defaultRawVTTDir, "Directory for downloaded VTT files")
	flag.StringVar(&cleanedDir, "cleaned_dir", defaultCleanedDir, "Directory for deduplicated transcript files")
	flag.BoolVar(&cleanDirs, "clean", true, "Clean directories before processing")
	flag.Parse()

	urls := flag.Args()
	if len(urls) == 0 {
		fmt.Println("Usage: yt-tx [flags] <youtube-url> [<youtube-url>...]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Clean or create directories
	if cleanDirs {
		if err := internal.CleanDirectories(rawVTTDir, cleanedDir); err != nil {
			fmt.Printf("Error cleaning directories: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Just ensure directories exist
		if err := internal.EnsureDirectories(rawVTTDir, cleanedDir); err != nil {
			fmt.Printf("Error creating directories: %v\n", err)
			os.Exit(1)
		}
	}

	// Create a new program
	p := tea.NewProgram(TranscriptApp{
		workflow: internal.NewWorkflow(urls, rawVTTDir, cleanedDir), // Pass the full urls slice
	})

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
