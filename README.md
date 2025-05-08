# yt-tx

A command-line tool for fetching and cleaning YouTube subtitles into plain-text transcripts.
It's just a yt-dlp wrapper.

## Features

* Fetches manual or auto-generated English VTT subtitles via `yt-dlp`
* Strips timestamps, cue IDs, and styling tags
* Collapses duplicate lines
* Interactive CLI with spinners (Bubble Tea + Bubbles)

## Prerequisites

* Go ≥1.18
* [yt-dlp](https://github.com/yt-dlp/yt-dlp)

## Usage

```bash
./yt-tx \
  https://www.youtube.com/watch?v=<id1> \
  https://www.youtube.com/watch?v=<id2>
```

Processed files are written to three directories in your working folder, assuming you may be interested in the intermediary formatting:

* `raw_vtt/` → downloaded `.vtt` files
* `transcripts/` → `.txt` files (timestamps removed)
* `cleaned/` → `.clean.txt` files (duplicates collapsed)

## Directory Structure

```text
├── main.go
├── yt-tx               # built binary
├── raw_vtt/           # downloaded .vtt files
├── transcripts/       # intermediate .txt transcripts
└── cleaned/           # final .clean.txt files
```

## License

MIT
