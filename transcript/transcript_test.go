package transcript

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeTransport routes requests to a user-supplied function so the whole acquisition
// cascade can be exercised without touching the network.
type fakeTransport struct {
	fn func(*http.Request) (*http.Response, error)
}

func (f fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f.fn(r) }

func httpResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func playerWith(baseURL string) string {
	return `{"playabilityStatus":{"status":"OK"},` +
		`"videoDetails":{"title":"T","videoId":"abc12345678","author":"chan","lengthSeconds":"100"},` +
		`"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[` +
		`{"baseUrl":"` + baseURL + `","name":{"simpleText":"Portuguese (auto-generated)"},` +
		`"languageCode":"pt","kind":"asr","isTranslatable":true}]}}}`
}

const json3Body = `{"events":[` +
	`{"tStartMs":0,"dDurationMs":1000,"segs":[{"utf8":"hello"}]},` +
	`{"tStartMs":1000,"dDurationMs":1000,"segs":[{"utf8":"world"}]}]}`

func clientWith(fn func(*http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{Transport: fakeTransport{fn: fn}}
}

func isPlayer(r *http.Request) bool {
	return r.Method == http.MethodPost && r.URL.Path == "/youtubei/v1/player"
}

func isAndroid(r *http.Request) bool {
	return strings.Contains(r.Header.Get("User-Agent"), "android")
}

func TestFetchAndroidSuccess(t *testing.T) {
	client := clientWith(func(r *http.Request) (*http.Response, error) {
		switch {
		case isPlayer(r):
			return httpResp(200, playerWith("https://cc.test/cap?fmt=srv3")), nil
		case r.URL.Host == "cc.test":
			return httpResp(200, json3Body), nil
		}
		return httpResp(404, ""), nil
	})

	res, err := Fetch(context.Background(), "abc12345678", Options{Client: client})
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if res.Method != "innertube-android" {
		t.Errorf("method = %q; want innertube-android", res.Method)
	}
	if len(res.Segments) != 2 || res.Text() != "hello world" {
		t.Errorf("segments = %+v / text = %q", res.Segments, res.Text())
	}
	if res.Lang != "pt" || res.Meta.Title != "T" || res.Meta.LengthSeconds != 100 {
		t.Errorf("meta wrong: lang=%q title=%q len=%d", res.Lang, res.Meta.Title, res.Meta.LengthSeconds)
	}
}

func TestFetchFallsBackToIOS(t *testing.T) {
	// android yields a track whose caption download is empty → cascade must try ios.
	client := clientWith(func(r *http.Request) (*http.Response, error) {
		switch {
		case isPlayer(r) && isAndroid(r):
			return httpResp(200, playerWith("https://cc.test/android?fmt=srv3")), nil
		case isPlayer(r):
			return httpResp(200, playerWith("https://cc.test/ios?fmt=srv3")), nil
		case r.URL.Host == "cc.test" && strings.HasPrefix(r.URL.Path, "/android"):
			return httpResp(200, ""), nil // empty → android fails
		case r.URL.Host == "cc.test" && strings.HasPrefix(r.URL.Path, "/ios"):
			return httpResp(200, json3Body), nil
		}
		return httpResp(404, ""), nil
	})

	res, err := Fetch(context.Background(), "abc12345678", Options{Client: client})
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if res.Method != "innertube-ios" {
		t.Errorf("method = %q; want innertube-ios", res.Method)
	}
	if res.Text() != "hello world" {
		t.Errorf("text = %q", res.Text())
	}
}

func TestFetchAllFail(t *testing.T) {
	// No method yields usable captions.
	client := clientWith(func(r *http.Request) (*http.Response, error) {
		if isPlayer(r) {
			return httpResp(200, `{"playabilityStatus":{"status":"OK"},"captions":{}}`), nil
		}
		return httpResp(200, "<html>no player response here</html>"), nil // watch page without marker
	})

	_, err := Fetch(context.Background(), "abc12345678", Options{Client: client})
	if err == nil {
		t.Fatal("expected error when all methods fail")
	}
	if !strings.Contains(err.Error(), "all methods failed") {
		t.Errorf("error = %v; want 'all methods failed'", err)
	}
}

// TestPlayerBodyIsSafeJSON proves the request body is built safely: a videoID with
// quotes (which would break a string-concatenated body) still produces valid JSON.
func TestPlayerBodyIsSafeJSON(t *testing.T) {
	var gotBody []byte
	client := clientWith(func(r *http.Request) (*http.Response, error) {
		switch {
		case isPlayer(r):
			gotBody, _ = io.ReadAll(r.Body)
			return httpResp(200, playerWith("https://cc.test/cap?fmt=srv3")), nil
		case r.URL.Host == "cc.test":
			return httpResp(200, json3Body), nil
		}
		return httpResp(404, ""), nil
	})

	const tricky = `abc","injected":"x`
	if _, err := Fetch(context.Background(), tricky, Options{Client: client}); err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	var parsed struct {
		VideoID string `json:"videoId"`
	}
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("request body is not valid JSON: %v\nbody: %s", err, gotBody)
	}
	if parsed.VideoID != tricky {
		t.Errorf("videoId = %q; want %q", parsed.VideoID, tricky)
	}
}

func TestFetchRejectsBadLang(t *testing.T) {
	// Validation runs before any HTTP, so a nil client is never reached.
	for _, o := range []Options{{Translate: "../../x"}, {Lang: "en/../x"}, {Lang: "a b"}} {
		if _, err := Fetch(context.Background(), "abc12345678", o); err == nil {
			t.Errorf("Fetch(%+v) should reject invalid language code", o)
		}
	}
	// Valid codes must pass validation (this one fails later on the nil transport,
	// but NOT with an "invalid language" error).
	_, err := Fetch(context.Background(), "abc12345678", Options{Lang: "pt-BR", Client: clientWith(func(*http.Request) (*http.Response, error) {
		return httpResp(200, `{"playabilityStatus":{"status":"OK"},"captions":{}}`), nil
	})})
	if err != nil && strings.Contains(err.Error(), "invalid") {
		t.Errorf("valid lang pt-BR wrongly rejected: %v", err)
	}
}

func TestListTracks(t *testing.T) {
	client := clientWith(func(r *http.Request) (*http.Response, error) {
		if isPlayer(r) {
			return httpResp(200, playerWith("https://cc.test/cap?fmt=srv3")), nil
		}
		return httpResp(404, ""), nil
	})

	meta, tracks, method, err := ListTracks(context.Background(), "abc12345678", Options{Client: client})
	if err != nil {
		t.Fatalf("ListTracks error: %v", err)
	}
	if method != "innertube-android" || meta.Title != "T" {
		t.Errorf("method=%q title=%q", method, meta.Title)
	}
	if len(tracks) != 1 || tracks[0].LanguageCode != "pt" || !tracks[0].IsAuto() {
		t.Errorf("tracks = %+v", tracks)
	}
}
