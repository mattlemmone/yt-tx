package internal

import "testing"

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"standard URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"standard URL with other params", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=10s", "dQw4w9WgXcQ"},
		{"shortened URL", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"shortened URL with params", "https://youtu.be/dQw4w9WgXcQ?t=10s", "dQw4w9WgXcQ"},
		{"shortened URL no www", "http://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"embedded URL", "https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"embedded URL with params", "https://www.youtube.com/embed/dQw4w9WgXcQ?autoplay=1", "dQw4w9WgXcQ"},
		{"no video ID param", "https://www.youtube.com/", "https://www.youtube.com/"},
		{"malformed URL with v=", "htps:/www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"malformed URL no v=, triggers shortening", "htps:/www.youtube.com/watch?id=dQw4w9WgXcQ", "htps:/www.youtube.com/watch..."},
		{"empty URL", "", ""},
		{"very long non-youtube URL, triggers shortening", "http://example.com/this/is/a/very/long/url/that/is/not/youtube/and/longer/than/30chars", "http://example.com/this/is/a..."},
		{"url exactly 30 chars, no shortening", "123456789012345678901234567890", "123456789012345678901234567890"},
		{"url 31 chars, shortening", "1234567890123456789012345678901", "123456789012345678901234567..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractVideoID(tt.url)
			if tt.name == "very long non-youtube URL, triggers shortening" {
				t.Logf("Test: %s\nInput URL: %q\nGot: %q (len %d)\nWant: %q (len %d)\nGot (bytes): %v\nWant (bytes): %v",
					tt.name, tt.url,
					got, len(got),
					tt.want, len(tt.want),
					[]byte(got), []byte(tt.want))
			}
			if got != tt.want {
				t.Errorf("ExtractVideoID() = %q, want %q", got, tt.want)
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
