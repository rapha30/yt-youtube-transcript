package transcript

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// User-Agents. The ANDROID/IOS InnerTube clients are what keep YouTube's signed
// caption URLs working without a "pot" (proof-of-origin) token — the WEB client's
// URLs became pot-gated in mid-2025 and return an empty body.
const (
	innertubeURL = "https://www.youtube.com/youtubei/v1/player?prettyPrint=false"
	androidUA    = "com.google.android.youtube/20.10.38 (Linux; U; Android 11) gzip"
	iosUA        = "com.google.ios.youtube/20.10.4 (iPhone16,2; U; CPU iOS 18_0 like Mac OS X)"
	browserUA    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	// maxBodyBytes caps every response read so a hostile/oversized/MITM'd body can't
	// exhaust memory. Watch pages are ~1-2 MB and caption tracks far smaller.
	maxBodyBytes = 16 << 20 // 16 MiB
)

// captionTrack is the raw shape decoded from YouTube's player/watch responses.
type captionTrack struct {
	BaseURL string `json:"baseUrl"`
	Name    struct {
		SimpleText string `json:"simpleText"`
		Runs       []struct {
			Text string `json:"text"`
		} `json:"runs"`
	} `json:"name"`
	LanguageCode   string `json:"languageCode"`
	Kind           string `json:"kind"` // "asr" = auto-generated, "" = manual/uploaded
	IsTranslatable bool   `json:"isTranslatable"`
}

func (c captionTrack) label() string {
	if c.Name.SimpleText != "" {
		return c.Name.SimpleText
	}
	var sb strings.Builder
	for _, r := range c.Name.Runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

func (c captionTrack) toTrack() Track {
	return Track{
		BaseURL:      c.BaseURL,
		Name:         c.label(),
		LanguageCode: c.LanguageCode,
		Kind:         c.Kind,
		Translatable: c.IsTranslatable,
	}
}

type playerResponse struct {
	PlayabilityStatus struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"playabilityStatus"`
	VideoDetails struct {
		Title         string `json:"title"`
		VideoID       string `json:"videoId"`
		Author        string `json:"author"`
		LengthSeconds string `json:"lengthSeconds"`
	} `json:"videoDetails"`
	Captions struct {
		PlayerCaptionsTracklistRenderer struct {
			CaptionTracks []captionTrack `json:"captionTracks"`
		} `json:"playerCaptionsTracklistRenderer"`
	} `json:"captions"`
}

func (pr *playerResponse) tracks() []captionTrack {
	return pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks
}

func (pr *playerResponse) meta(fallbackID string) VideoMeta {
	id := pr.VideoDetails.VideoID
	if id == "" {
		id = fallbackID
	}
	return VideoMeta{
		ID:            id,
		Title:         pr.VideoDetails.Title,
		Author:        pr.VideoDetails.Author,
		LengthSeconds: atoiSafe(pr.VideoDetails.LengthSeconds),
	}
}

// clientContext is the InnerTube client identity sent in the player request.
type clientContext struct {
	ClientName        string `json:"clientName"`
	ClientVersion     string `json:"clientVersion"`
	AndroidSDKVersion int    `json:"androidSdkVersion,omitempty"`
	DeviceModel       string `json:"deviceModel,omitempty"`
	HL                string `json:"hl"`
	GL                string `json:"gl"`
}

type playerRequest struct {
	Context struct {
		Client clientContext `json:"client"`
	} `json:"context"`
	VideoID string `json:"videoId"`
}

// source is one acquisition method. Multiple sources are tried in order so a single
// throttled/blocked endpoint does not take the whole tool down.
type source struct {
	name  string
	fetch func(ctx context.Context, videoID string, client *http.Client) (*playerResponse, error)
}

func sources() []source {
	return []source{
		{"innertube-android", func(ctx context.Context, id string, client *http.Client) (*playerResponse, error) {
			return innertube(ctx, client, id, androidUA, clientContext{
				ClientName: "ANDROID", ClientVersion: "20.10.38", AndroidSDKVersion: 30, HL: "en", GL: "US",
			})
		}},
		{"innertube-ios", func(ctx context.Context, id string, client *http.Client) (*playerResponse, error) {
			return innertube(ctx, client, id, iosUA, clientContext{
				ClientName: "IOS", ClientVersion: "20.10.4", DeviceModel: "iPhone16,2", HL: "en", GL: "US",
			})
		}},
		{"watch-page", fetchWatchPage},
	}
}

func innertube(ctx context.Context, client *http.Client, videoID, ua string, cc clientContext) (*playerResponse, error) {
	// Marshal the body (rather than string-concatenating videoID) so a caller passing
	// an unsanitized ID can't break or inject into the JSON.
	var reqBody playerRequest
	reqBody.Context.Client = cc
	reqBody.VideoID = videoID
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, innertubeURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept-Language", "en")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var pr playerResponse
	if err := json.Unmarshal(raw, &pr); err != nil {
		return nil, fmt.Errorf("decoding player response: %w", err)
	}
	return &pr, nil
}

func fetchWatchPage(ctx context.Context, videoID string, client *http.Client) (*playerResponse, error) {
	watchURL := "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID) + "&hl=en"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, watchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cookie", "CONSENT=YES+cb")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	jsonStr, ok := extractBalancedJSON(string(raw), "ytInitialPlayerResponse")
	if !ok {
		return nil, fmt.Errorf("ytInitialPlayerResponse not found in watch page")
	}
	var pr playerResponse
	if err := json.Unmarshal([]byte(jsonStr), &pr); err != nil {
		return nil, fmt.Errorf("decoding watch-page player response: %w", err)
	}
	return &pr, nil
}

// extractBalancedJSON returns the first balanced {...} object that appears after marker.
// A regexp can't match balanced braces, so we scan with a depth counter, skipping
// over string literals (and their escapes).
func extractBalancedJSON(s, marker string) (string, bool) {
	i := strings.Index(s, marker)
	if i < 0 {
		return "", false
	}
	j := strings.IndexByte(s[i:], '{')
	if j < 0 {
		return "", false
	}
	start := i + j
	depth, inStr, esc := 0, false, false
	for k := start; k < len(s); k++ {
		c := s[k]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : k+1], true
			}
		}
	}
	return "", false
}

func getBody(ctx context.Context, client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	// Surface HTTP errors (e.g. 429 throttling) instead of feeding an error page to the
	// caption parsers, so the cascade's failure message stays diagnostic.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
}
