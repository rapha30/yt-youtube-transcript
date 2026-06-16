package transcript

import (
	"strings"
	"testing"
)

func sampleResult() *Result {
	return &Result{
		Meta:   VideoMeta{ID: "abc12345678", Title: "My Video", Author: "Chan", LengthSeconds: 3661},
		Track:  Track{LanguageCode: "pt", Kind: "asr"},
		Lang:   "pt",
		Method: "innertube-android",
		Segments: []Segment{
			{StartMs: 0, DurMs: 1000, Text: "hello"},
			{StartMs: 1000, DurMs: 1500, Text: "world"},
		},
	}
}

func TestStamp(t *testing.T) {
	cases := []struct {
		ms    int
		comma bool
		want  string
	}{
		{0, true, "00:00:00,000"},
		{1500, true, "00:00:01,500"},
		{3661000, false, "01:01:01"},
		{-5, true, "00:00:00,000"},
	}
	for _, c := range cases {
		if got := stamp(c.ms, c.comma); got != c.want {
			t.Errorf("stamp(%d,%v) = %q; want %q", c.ms, c.comma, got, c.want)
		}
	}
}

func TestDurationLabel(t *testing.T) {
	for in, want := range map[int]string{0: "", -1: "", 90: "1:30", 3661: "1:01:01", 59: "0:59"} {
		if got := durationLabel(in); got != want {
			t.Errorf("durationLabel(%d) = %q; want %q", in, got, want)
		}
	}
}

func TestText(t *testing.T) {
	// mix of double spaces, CRLF and a tab — all must collapse to single spaces
	r := &Result{Segments: []Segment{{Text: "a  b"}, {Text: "c\r\n d"}, {Text: "e\tf"}}}
	if got := r.Text(); got != "a b c d e f" {
		t.Errorf("Text() = %q; want collapsed 'a b c d e f'", got)
	}
}

func TestSRT(t *testing.T) {
	got := sampleResult().SRT()
	if !strings.HasPrefix(got, "1\n00:00:00,000 --> 00:00:01,000\nhello\n\n") {
		t.Errorf("SRT first cue wrong:\n%q", got)
	}
	if !strings.Contains(got, "2\n00:00:01,000 --> 00:00:02,500\nworld") {
		t.Errorf("SRT second cue wrong:\n%q", got)
	}
}

func TestTimestamps(t *testing.T) {
	got := sampleResult().Timestamps()
	if got != "[00:00:00] hello\n[00:00:01] world\n" {
		t.Errorf("Timestamps() = %q", got)
	}
}

func TestHeader(t *testing.T) {
	got := sampleResult().Header("2026-06-16 07:45")
	for _, want := range []string{
		"# My Video",
		"# channel: Chan",
		"# https://www.youtube.com/watch?v=abc12345678",
		"language: pt (auto-generated)",
		"via innertube-android",
		"duration 1:01:01",
		"# fetched: 2026-06-16 07:45",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Header() missing %q in:\n%s", want, got)
		}
	}
}

func TestHeaderTranslated(t *testing.T) {
	r := sampleResult()
	r.Lang = "es"
	r.Track.LanguageCode = "pt"
	if got := r.Header(""); !strings.Contains(got, "es (translated from pt)") {
		t.Errorf("translated header wrong:\n%s", got)
	}
}
