package internal

import "testing"

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{"standard URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"standard URL with other params", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=10s", "dQw4w9WgXcQ", false},
		{"shortened URL", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"shortened URL with params", "https://youtu.be/dQw4w9WgXcQ?t=10s", "dQw4w9WgXcQ", false},
		{"shortened URL no www", "http://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"embedded URL", "https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"embedded URL with params", "https://www.youtube.com/embed/dQw4w9WgXcQ?autoplay=1", "dQw4w9WgXcQ", false},
		{"no video ID param (not a valid video watch/embed/short URL)", "https://www.youtube.com/", "", true},
		{"malformed URL with v= (still extracts if v= is present)", "htps:/www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"malformed URL no v= (not a YouTube URL)", "htps:/www.youtube.com/watch?id=dQw4w9WgXcQ", "", true},
		{"empty URL", "", "", true},
		{"very long non-youtube URL", "http://example.com/this/is/a/very/long/url/that/is/not/youtube/and/longer/than/30chars", "", true},
		{"url exactly 30 chars, not youtube", "123456789012345678901234567890", "", true},
		{"url 31 chars, not youtube", "1234567890123456789012345678901", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractVideoID(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractVideoID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractVideoID() got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractDisplayTitle(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"strips .en.vtt", "videoID--My Video Title.en.vtt", "videoID--My Video Title"},
		{"strips .vtt", "videoID--My Video Title.vtt", "videoID--My Video Title"},
		{"ID only, strips .en.vtt", "videoID.en.vtt", "videoID"},
		{"ID only, strips .vtt", "videoID.vtt", "videoID"},
		{"strips .en.vtt from title with prefixed --", "--Actual Title.en.vtt", "--Actual Title"},
		{"strips only extensions from complex name", "id-part1--part2.en.vtt", "id-part1--part2"},
		{"empty filename", "", ""},
		{".vtt only", ".vtt", ""},
		{".en.vtt only", ".en.vtt", ""},
		{"filename is just title, strips .en.vtt", "My Video Title.en.vtt", "My Video Title"},
		{"filename is just title, strips .vtt", "My Video Title.vtt", "My Video Title"},
		{"already clean title (no extensions)", "MyVideoTitle", "MyVideoTitle"},
		{"already clean with hyphens", "videoID--title-part", "videoID--title-part"},
		{"unknown extension left alone", "video.eng.txt", "video.eng.txt"},
		{"only lang code, no vtt", "video.en", "video.en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractDisplayTitle(tt.filename); got != tt.want {
				t.Errorf("ExtractDisplayTitle(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}
