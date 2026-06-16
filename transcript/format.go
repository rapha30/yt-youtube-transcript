package transcript

import (
	"fmt"
	"regexp"
	"strings"
)

var collapseWhitespace = regexp.MustCompile(`\s+`)

// Text returns the transcript as a single flowing paragraph: every run of whitespace
// (spaces, tabs, CR, LF) is collapsed to a single space.
func (r *Result) Text() string {
	parts := make([]string, len(r.Segments))
	for i, s := range r.Segments {
		parts[i] = s.Text
	}
	out := collapseWhitespace.ReplaceAllString(strings.Join(parts, " "), " ")
	return strings.TrimSpace(out)
}

// SRT returns the transcript as an SRT subtitle document.
func (r *Result) SRT() string {
	var sb strings.Builder
	for i, s := range r.Segments {
		end := s.StartMs + s.DurMs
		if i+1 < len(r.Segments) && end > r.Segments[i+1].StartMs {
			end = r.Segments[i+1].StartMs
		}
		fmt.Fprintf(&sb, "%d\n%s --> %s\n%s\n\n", i+1, stamp(s.StartMs, true), stamp(end, true), s.Text)
	}
	return sb.String()
}

// Timestamps returns one "[hh:mm:ss] text" line per cue.
func (r *Result) Timestamps() string {
	var sb strings.Builder
	for _, s := range r.Segments {
		fmt.Fprintf(&sb, "[%s] %s\n", stamp(s.StartMs, false), s.Text)
	}
	return sb.String()
}

// Header returns a few "# ..." metadata lines describing the transcript, suitable for
// prepending to a .txt file. fetchedAt is rendered with the given layout-ready string.
func (r *Result) Header(fetchedAt string) string {
	var sb strings.Builder
	if r.Meta.Title != "" {
		fmt.Fprintf(&sb, "# %s\n", r.Meta.Title)
	}
	if r.Meta.Author != "" {
		fmt.Fprintf(&sb, "# channel: %s\n", r.Meta.Author)
	}
	fmt.Fprintf(&sb, "# https://www.youtube.com/watch?v=%s\n", r.Meta.ID)
	meta := fmt.Sprintf("# language: %s · %d segments · via %s", r.langDesc(), len(r.Segments), r.Method)
	if d := durationLabel(r.Meta.LengthSeconds); d != "" {
		meta += " · duration " + d
	}
	sb.WriteString(meta + "\n")
	if fetchedAt != "" {
		fmt.Fprintf(&sb, "# fetched: %s\n", fetchedAt)
	}
	sb.WriteString("# " + strings.Repeat("-", 56) + "\n\n")
	return sb.String()
}

func (r *Result) langDesc() string {
	if r.Lang != r.Track.LanguageCode {
		return fmt.Sprintf("%s (translated from %s)", r.Lang, r.Track.LanguageCode)
	}
	kind := "manual"
	if r.Track.IsAuto() {
		kind = "auto-generated"
	}
	return fmt.Sprintf("%s (%s)", r.Lang, kind)
}

func stamp(ms int, comma bool) string {
	if ms < 0 {
		ms = 0
	}
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	s := ms / 1000
	mil := ms % 1000
	if comma {
		return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, mil)
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func durationLabel(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	h, m, s := seconds/3600, (seconds%3600)/60, seconds%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
