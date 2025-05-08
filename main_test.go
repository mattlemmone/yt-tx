package main

import (
	"os"
	"strings"
	"testing"
)

func TestIsNumber(t *testing.T) {
	cases := map[string]bool{
		"123": true,
		"001": true,
		"abc": false,
		"12a": false,
		"":    false,
	}
	for input, want := range cases {
		if got := isNumber(input); got != want {
			t.Errorf("isNumber(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestIsTimestamp(t *testing.T) {
	cases := map[string]bool{
		"00:00:01.000 --> 00:00:02.000": true,
		"01:23:45.678 --> 12:34:56.789": true,
		"not a timestamp":               false,
		"00:00:00.000":                  false,
		"":                              false,
	}
	for input, want := range cases {
		if got := isTimestamp(input); got != want {
			t.Errorf("isTimestamp(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestStripHTMLTags(t *testing.T) {
	cases := map[string]string{
		"hello <b>world</b>":   "hello world",
		"<i>test</i>":          "test",
		"no tags":              "no tags",
		"<a href='x'>link</a>": "link",
	}
	for input, want := range cases {
		if got := stripHTMLTags(input); got != want {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", input, got, want)
		}
	}
}

// Helper: cleanTranscriptLines applies the cleaning logic to a slice of lines.
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

// Helper: dedupeLines removes consecutive duplicate lines from a slice.

func TestCleanTranscriptLines(t *testing.T) {
	input := []string{
		"WEBVTT",
		"1",
		"00:00:00.000 --> 00:00:01.000",
		"<font color=\"#CCCCCC\">Hello</font>",
		"",
		"2",
		"00:00:01.000 --> 00:00:02.000",
		"World",
	}
	want := []string{"Hello", "World"}
	got := cleanTranscriptLines(input)
	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDedupeLines(t *testing.T) {
	input := []string{"Hello", "Hello", "World", "World", "World", "Test"}
	want := []string{"Hello", "World", "Test"}
	got := dedupeLines(input)
	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
