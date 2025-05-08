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

- `-raw_vtt_dir` Directory for downloaded VTT files (default: raw_vtt)
- `-cleaned_dir` Directory for cleaned transcript files (default: cleaned)

Example:

```bash
./yt-tx -raw_vtt_dir=myvtt -cleaned_dir=mycleaned \
  https://www.youtube.com/watch?v=<id1>
```

Processed files are written to two directories in your working folder:

- `raw_vtt/` → downloaded `.vtt` files
- `cleaned/` → `.clean.txt` files (cleaned and deduplicated)

## Directory Structure

```text
├── main.go
├── yt-tx               # built binary
├── raw_vtt/           # downloaded .vtt files
└── cleaned/           # final .clean.txt files
```

## Transcript Cleaning and Deduplication

This project no longer uses inline bash scripting for transcript cleaning and deduplication. All processing is done in Go for portability and testability. The cleaning step removes WEBVTT headers, numeric lines, timestamps, and HTML tags. The deduplication step removes consecutive duplicate lines, which is needed because YouTube subtitles often repeat lines for overlapping cues.

## Running Tests

Unit tests are provided for the transcript cleaning and deduplication logic. To run the tests:

```
go test
```

This will run all tests in `main_test.go`.

## License

MIT
