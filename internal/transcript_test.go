package internal

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDedupeLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"empty input", []string{}, []string{}},
		{"nil input", nil, []string{}},
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"consecutive duplicates", []string{"a", "a", "b", "c", "c", "c"}, []string{"a", "b", "c"}},
		{"non-consecutive duplicates", []string{"a", "b", "a", "c"}, []string{"a", "b", "a", "c"}},
		{"with consecutive empty strings", []string{"a", "", "", "b"}, []string{"a", "", "b"}},
		{"leading/trailing consecutive empty", []string{"", "", "a", "b", "", ""}, []string{"", "a", "b", ""}},
		{"all duplicates", []string{"x", "x", "x"}, []string{"x"}},
		{"all empty strings", []string{"", "", ""}, []string{""}},
		{"single line", []string{"hello"}, []string{"hello"}},
		{"single empty line", []string{""}, []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DedupeLines(tt.lines)
			if (tt.lines == nil || len(tt.lines) == 0) && (got == nil || len(got) != 0) {
				t.Errorf("DedupeLines() for empty/nil input: got %v (len %d), want non-nil empty slice []string{}", got, len(got))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DedupeLines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNumber(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"empty string", "", false},
		{"simple number", "123", true},
		{"number with spaces", " 123 ", false},
		{"not a number", "abc", false},
		{"mixed", "12a3", false},
		{"zero", "0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNumber(tt.s); got != tt.want {
				t.Errorf("IsNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTimestamp(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"valid timestamp", "00:00:01.000 --> 00:00:02.500", true},
		{"valid timestamp with extra", "00:00:01.000 --> 00:00:02.500 align:start position:0%", true}, // True because it checks prefix
		{"too short", "00:00:01.000 --> 00:00:02.50", false},
		{"missing arrow", "00:00:01.000 00:00:02.500", false},
		{"incorrect format", "00-00-01.000 --> 00-00-02.500", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTimestamp(tt.s); got != tt.want {
				t.Errorf("IsTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"empty string", "", ""},
		{"no tags", "hello world", "hello world"},
		{"simple tag", "<c>hello</c>", "hello"},
		{"multiple tags", "<c.colorE5E5E5><c. διάρκ>hello</c. διάρκ></c> world", "hello world"},
		{"unclosed tag at end", "hello <c.colo", "hello "},
		{"unclosed tag at start", "<c.colo hello", ""},
		{"mixed content", "hello <b>world</b> test", "hello world test"},
		{"self-closing like", "<br/> breaks", " breaks"}, // Interprets <br/> as a tag
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripHTMLTags(tt.s); got != tt.want {
				t.Errorf("StripHTMLTags() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRemoveVTTArtifacts(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"empty lines", []string{}, []string{}},
		{"only WEBVTT", []string{"WEBVTT"}, []string{}},
		{"only numbers", []string{"1", "23"}, []string{}},
		{"only timestamps", []string{"00:00:00.000 --> 00:00:01.000"}, []string{}},
		{"text with tags", []string{"<c>hello</c>"}, []string{"hello"}},
		{"mixed content", []string{"WEBVTT", "", "1", "00:00:00.000 --> 00:00:01.000", "hello world", "<c>another</c> line"}, []string{"hello world", "another line"}},
		{"no artifacts", []string{"clean line 1", "clean line 2"}, []string{"clean line 1", "clean line 2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveVTTArtifacts(tt.lines); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RemoveVTTArtifacts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNewestVTTPattern(t *testing.T) {
	tests := []struct {
		name      string
		rawVTTDir string
		want      string
	}{
		{"simple dir", "raw_vtt", filepath.Join("raw_vtt", "*.vtt")},
		{"nested dir", "data/raw", filepath.Join("data/raw", "*.vtt")},
		{"current dir", ".", filepath.Join(".", "*.vtt")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetNewestVTTPattern(tt.rawVTTDir); got != tt.want {
				t.Errorf("GetNewestVTTPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOutputFilePath(t *testing.T) {
	type args struct {
		vttPath    string
		cleanedDir string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "simple case",
			args: args{vttPath: filepath.Join("raw", "videoID--Video Title.en.vtt"), cleanedDir: "cleaned"},
			want: filepath.Join("cleaned", "Video Title.txt"),
		},
		{
			name: "no title in filename",
			args: args{vttPath: filepath.Join("raw", "videoID.en.vtt"), cleanedDir: "output"},
			want: filepath.Join("output", "videoID.txt"),
		},
		{
			name: "double vtt extension",
			args: args{vttPath: filepath.Join("somewhere", "id--A.B.C.vtt.vtt"), cleanedDir: "final"},
			want: filepath.Join("final", "A.B.C.txt"),
		},
		{
			name: "no en extension",
			args: args{vttPath: filepath.Join("downloads", "xyz--Another One.vtt"), cleanedDir: "textfiles"},
			want: filepath.Join("textfiles", "Another One.txt"),
		},
		{
			name: "path with spaces in dir and title",
			args: args{vttPath: filepath.Join("my vids", "test id--A Title With Spaces.en.vtt"), cleanedDir: "my texts"},
			want: filepath.Join("my texts", "A Title With Spaces.txt"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOutputFilePath(tt.args.vttPath, tt.args.cleanedDir); got != tt.want {
				t.Errorf("GetOutputFilePath() = %v, want %v", got, tt.want)
			}
		})
	}
}
