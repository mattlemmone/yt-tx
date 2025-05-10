package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattlemmone/yt-tx/internal"
)

const (
	defaultCleanedDir = "cleaned"
	tempDirName       = "tmp"
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
		cleanedDir      string
		parallelWorkers int
	)

	flag.StringVar(&cleanedDir, "cleaned_dir", defaultCleanedDir, "Directory for deduplicated transcript files")
	flag.IntVar(&parallelWorkers, "p", 1, "Number of parallel workers to process videos")
	flag.Parse()

	urls := flag.Args()
	if len(urls) == 0 {
		fmt.Println("Usage: yt-tx [flags] <youtube-url> [<youtube-url>...]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create/clean directories
	if err := internal.CleanDirectories(tempDirName, cleanedDir); err != nil {
		fmt.Printf("Error preparing directories: %v\n", err)
		os.Exit(1)
	}

	// Create a new program
	p := tea.NewProgram(TranscriptApp{
		workflow: internal.NewWorkflow(urls, tempDirName, cleanedDir, parallelWorkers), // Pass the full urls slice
	})

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
