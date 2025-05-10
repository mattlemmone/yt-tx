# yt-tx

A command-line tool for fetching and cleaning YouTube subtitles into plain-text transcripts.
It's just a yt-dlp wrapper.

## Features

- Fetches manual or auto-generated English VTT subtitles via `yt-dlp`
- Strips timestamps, cue IDs, and styling tags
- Collapses duplicate lines
- Interactive CLI with spinners (Bubble Tea + Bubbles)

## Prerequisites

- Go ≥1.18
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)

## Usage

```bash
./yt-tx [flags] \
  https://www.youtube.com/watch?v=<id1> \
  https://www.youtube.com/watch?v=<id2>
```

### Flags

- `-cleaned_dir` Directory for cleaned transcript files (default: cleaned)
- `-p` Number of parallel workers to process videos (default: 1, for sequential processing)

Example:

```bash
# Process sequentially
./yt-tx https://www.youtube.com/watch?v=<id1>

# Process with 4 parallel workers
./yt-tx -p 4 \
  https://www.youtube.com/watch?v=<id1> \
  https://www.youtube.com/watch?v=<id2> \
  https://www.youtube.com/watch?v=<id3> \
  https://www.youtube.com/watch?v=<id4>

# Customize output directory and process with 2 workers
./yt-tx -cleaned_dir=my_cleaned_transcripts -p 2 \
  https://www.youtube.com/watch?v=<id1> \
  https://www.youtube.com/watch?v=<id2>
```

Processed files involve two main directories in your working folder:

- `tmp/` → temporary directory for downloaded `.vtt` files (cleaned after each run)
- `cleaned/` → final `.txt` files (cleaned and deduplicated transcripts)

## Directory Structure

```text
├── cmd/yt-tx/main.go  # Main application
├── internal/          # Core logic (files, youtube, transcript, etc.)
├── yt-tx              # built binary
├── tmp/               # temporary .vtt files (cleaned after run)
└── cleaned/           # final .txt files
```

## Transcript Cleaning and Deduplication

This project no longer uses inline bash scripting for transcript cleaning and deduplication. All processing is done in Go for portability and testability. The cleaning step removes WEBVTT headers, numeric lines, timestamps, and HTML tags. The deduplication step removes consecutive duplicate lines, which is needed because YouTube subtitles often repeat lines for overlapping cues.

## Running Tests

Unit tests are provided for the core logic, including transcript cleaning and processing. To run all tests in the project:

```bash
go test ./...
```

This command will discover and run tests in the current directory and all sub-directories.

## License

MIT
