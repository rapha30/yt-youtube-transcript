// Package transcript fetches YouTube video transcripts (captions) with no API key and
// no external dependencies.
//
// It cascades through three independent acquisition methods so a single throttled or
// blocked endpoint does not break the whole flow:
//
//  1. innertube-android : POST youtubei/v1/player with an ANDROID client context.
//  2. innertube-ios     : same endpoint, IOS client — survives if ANDROID is throttled.
//  3. watch-page        : GET the watch page and parse ytInitialPlayerResponse.
//
// Within a method, the caption download itself falls back json3 → srv1 (XML).
//
// Why ANDROID/IOS and not WEB: YouTube's WEB-client caption URLs became "pot token"
// gated in mid-2025 and return an empty body; the ANDROID/IOS signed URLs still work.
package transcript

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// langCodeRe accepts BCP-47-ish codes ("en", "pt", "es-419", "zh-Hans"). It rejects
// anything that could escape a filename (slashes, dots, "..") at the API boundary.
var langCodeRe = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{1,8})*$`)

func (o Options) validate() error {
	if o.Lang != "" && !langCodeRe.MatchString(o.Lang) {
		return fmt.Errorf("invalid language code %q", o.Lang)
	}
	if o.Translate != "" && !langCodeRe.MatchString(o.Translate) {
		return fmt.Errorf("invalid translate language code %q", o.Translate)
	}
	return nil
}

// Track is a single caption track available for a video.
type Track struct {
	BaseURL      string // signed timedtext URL
	Name         string // human label, e.g. "English (auto-generated)"
	LanguageCode string // e.g. "en", "pt"
	Kind         string // "asr" = auto-generated, "" = manual/uploaded
	Translatable bool
}

// IsAuto reports whether the track is auto-generated (ASR).
func (t Track) IsAuto() bool { return t.Kind == "asr" }

// Segment is one timed caption cue.
type Segment struct {
	StartMs int
	DurMs   int
	Text    string
}

// VideoMeta holds the basic video details returned alongside the captions.
type VideoMeta struct {
	ID            string
	Title         string
	Author        string
	LengthSeconds int
}

// Result is a fetched transcript.
type Result struct {
	Meta     VideoMeta
	Track    Track     // the source track that was downloaded
	Segments []Segment // the caption cues
	Method   string    // which acquisition method succeeded
	Lang     string    // output language (== Translate target, else Track.LanguageCode)
}

// Options tunes a fetch.
type Options struct {
	// Lang is the caption language code to fetch. Empty means the original spoken
	// language (the auto-generated/asr track, else the first available track).
	Lang string
	// Translate, when set, requests YouTube auto-translation to this language code.
	// Note: YouTube frequently rejects translation on signed URLs (empty/429).
	Translate string
	// Client is the HTTP client to use. When nil, a 30s-timeout client is used.
	Client *http.Client
}

var defaultClient = &http.Client{Timeout: 30 * time.Second}

func (o Options) client() *http.Client {
	if o.Client != nil {
		return o.Client
	}
	return defaultClient
}

// ListTracks returns the video metadata and the available caption tracks, using the
// first acquisition method that yields any tracks. The returned string is the method name.
func ListTracks(ctx context.Context, videoID string, opts Options) (VideoMeta, []Track, string, error) {
	client := opts.client()
	var errs []string
	for _, src := range sources() {
		pr, err := src.fetch(ctx, videoID, client)
		if err != nil {
			errs = append(errs, src.name+": "+err.Error())
			continue
		}
		raw := pr.tracks()
		if len(raw) == 0 {
			errs = append(errs, methodNote(src.name, pr))
			continue
		}
		out := make([]Track, len(raw))
		for i, c := range raw {
			out[i] = c.toTrack()
		}
		return pr.meta(videoID), out, src.name, nil
	}
	return VideoMeta{}, nil, "", failure(errs)
}

// Fetch downloads a transcript for videoID. It walks the acquisition methods in order
// and returns the first one that yields non-empty caption text for the chosen language.
func Fetch(ctx context.Context, videoID string, opts Options) (*Result, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	client := opts.client()
	var errs []string
	for _, src := range sources() {
		pr, err := src.fetch(ctx, videoID, client)
		if err != nil {
			errs = append(errs, src.name+": "+err.Error())
			continue
		}
		if note := methodNote(src.name, pr); pr.PlayabilityStatus.Status != "" && pr.PlayabilityStatus.Status != "OK" {
			errs = append(errs, note) // record but keep going; metadata may still be usable
		}
		tracks := pr.tracks()
		if len(tracks) == 0 {
			errs = append(errs, src.name+": no caption tracks")
			continue
		}
		ct, err := chooseTrack(tracks, opts.Lang)
		if err != nil {
			errs = append(errs, src.name+": "+err.Error())
			continue
		}
		segs, err := fetchSegments(ctx, client, ct.BaseURL, opts.Translate)
		if err != nil {
			errs = append(errs, src.name+": "+err.Error())
			continue
		}
		lang := ct.LanguageCode
		if opts.Translate != "" {
			lang = opts.Translate
		}
		return &Result{
			Meta:     pr.meta(videoID),
			Track:    ct.toTrack(),
			Segments: segs,
			Method:   src.name,
			Lang:     lang,
		}, nil
	}
	return nil, failure(errs)
}

func methodNote(name string, pr *playerResponse) string {
	if pr.PlayabilityStatus.Status != "" && pr.PlayabilityStatus.Status != "OK" {
		return fmt.Sprintf("%s: %s (%s)", name, pr.PlayabilityStatus.Status, pr.PlayabilityStatus.Reason)
	}
	return name + ": no caption tracks"
}

func failure(errs []string) error {
	if len(errs) == 0 {
		return fmt.Errorf("all methods failed")
	}
	return fmt.Errorf("all methods failed:\n  - %s", strings.Join(errs, "\n  - "))
}
