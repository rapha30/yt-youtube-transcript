package transcript

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"math"
	"net/http"
	"regexp"
	"strings"
)

// chooseTrack selects a caption track.
//   - lang != "" : must match languageCode; a manual track is preferred over the asr one.
//   - lang == "" : the original spoken language — prefer the asr (auto-generated) track,
//     otherwise the first track (avoids accidentally picking a translated track).
func chooseTrack(tracks []captionTrack, lang string) (captionTrack, error) {
	if len(tracks) == 0 {
		return captionTrack{}, fmt.Errorf("no caption tracks")
	}
	if lang != "" {
		var asr *captionTrack
		for i := range tracks {
			if strings.EqualFold(tracks[i].LanguageCode, lang) {
				if tracks[i].Kind != "asr" {
					return tracks[i], nil
				}
				if asr == nil {
					asr = &tracks[i]
				}
			}
		}
		if asr != nil {
			return *asr, nil
		}
		return captionTrack{}, fmt.Errorf("no track for language %q", lang)
	}
	for i := range tracks {
		if tracks[i].Kind == "asr" {
			return tracks[i], nil
		}
	}
	return tracks[0], nil
}

type json3Payload struct {
	Events []struct {
		TStartMs    int `json:"tStartMs"`
		DDurationMs int `json:"dDurationMs"`
		Segs        []struct {
			Utf8 string `json:"utf8"`
		} `json:"segs"`
	} `json:"events"`
}

type srv1Payload struct {
	Texts []struct {
		Start float64 `xml:"start,attr"`
		Dur   float64 `xml:"dur,attr"`
		Text  string  `xml:",chardata"`
	} `xml:"text"`
}

var (
	fmtParamRe   = regexp.MustCompile(`(\bfmt=)[^&]*`)
	tlangParamRe = regexp.MustCompile(`(\btlang=)[^&]*`)
)

// captionURL points a signed timedtext baseURL at a chosen format and optionally
// requests an auto-translation via tlang. Existing fmt/tlang values are replaced
// in place (preserving the rest of the signed query exactly); missing ones are
// appended. Replacing in place matters: stripping a leading "?fmt=..." would drop
// the "?" and corrupt the query.
func captionURL(baseURL, format, translateTo string) string {
	u := baseURL
	if fmtParamRe.MatchString(u) {
		u = fmtParamRe.ReplaceAllString(u, "${1}"+format)
	} else {
		u = appendParam(u, "fmt", format)
	}
	if translateTo != "" {
		if tlangParamRe.MatchString(u) {
			u = tlangParamRe.ReplaceAllString(u, "${1}"+translateTo)
		} else {
			u = appendParam(u, "tlang", translateTo)
		}
	}
	return u
}

func appendParam(u, key, value string) string {
	sep := "&"
	if !strings.Contains(u, "?") {
		sep = "?"
	}
	return u + sep + key + "=" + value
}

// fetchSegments downloads a caption track, trying json3 first then srv1 (XML) as a
// format-level fallback.
func fetchSegments(ctx context.Context, client *http.Client, baseURL, translateTo string) ([]Segment, error) {
	if raw, err := getBody(ctx, client, captionURL(baseURL, "json3", translateTo)); err == nil && len(raw) > 0 {
		if segs := parseJSON3(raw); len(segs) > 0 {
			return segs, nil
		}
	}
	if raw, err := getBody(ctx, client, captionURL(baseURL, "srv1", translateTo)); err == nil && len(raw) > 0 {
		if segs := parseSRV1(raw); len(segs) > 0 {
			return segs, nil
		}
	}
	return nil, fmt.Errorf("caption track returned no usable text (json3 and srv1 both empty)")
}

func parseJSON3(raw []byte) []Segment {
	var j json3Payload
	if json.Unmarshal(raw, &j) != nil {
		return nil
	}
	var segs []Segment
	for _, ev := range j.Events {
		if len(ev.Segs) == 0 {
			continue
		}
		var sb strings.Builder
		for _, s := range ev.Segs {
			sb.WriteString(s.Utf8)
		}
		if text := strings.TrimSpace(sb.String()); text != "" {
			segs = append(segs, Segment{StartMs: ev.TStartMs, DurMs: ev.DDurationMs, Text: text})
		}
	}
	return segs
}

func parseSRV1(raw []byte) []Segment {
	var x srv1Payload
	if xml.Unmarshal(raw, &x) != nil {
		return nil
	}
	var segs []Segment
	for _, t := range x.Texts {
		// YouTube's srv1 chardata is double-encoded (e.g. "&amp;#39;"): encoding/xml
		// unescapes one level, html.UnescapeString the second. Round (not truncate)
		// the float seconds → ms, since float math leaves values 1ms low otherwise.
		if text := strings.TrimSpace(html.UnescapeString(t.Text)); text != "" {
			segs = append(segs, Segment{
				StartMs: int(math.Round(t.Start * 1000)),
				DurMs:   int(math.Round(t.Dur * 1000)),
				Text:    text,
			})
		}
	}
	return segs
}
