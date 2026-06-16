package transcript

import (
	"strings"
	"testing"
)

func TestCaptionURL(t *testing.T) {
	cases := []struct {
		base, format, tlang, want string
	}{
		{"https://x/t?v=1&fmt=srv3", "json3", "", "https://x/t?v=1&fmt=json3"},
		{"https://x/t?v=1&fmt=srv3", "json3", "es", "https://x/t?v=1&fmt=json3&tlang=es"},
		{"https://x/t?v=1", "json3", "", "https://x/t?v=1&fmt=json3"},
		{"https://x/t", "json3", "", "https://x/t?fmt=json3"},
		{"https://x/t?fmt=vtt&v=1", "srv1", "", "https://x/t?fmt=srv1&v=1"},
		{"https://x/t?v=1&fmt=srv3&tlang=de", "json3", "es", "https://x/t?v=1&fmt=json3&tlang=es"},
	}
	for _, c := range cases {
		if got := captionURL(c.base, c.format, c.tlang); got != c.want {
			t.Errorf("captionURL(%q,%q,%q) = %q; want %q", c.base, c.format, c.tlang, got, c.want)
		}
	}
}

func track(lang, kind string) captionTrack {
	var ct captionTrack
	ct.LanguageCode = lang
	ct.Kind = kind
	ct.BaseURL = "https://x/" + lang + "/" + kind
	return ct
}

func TestChooseTrack(t *testing.T) {
	if _, err := chooseTrack(nil, ""); err == nil {
		t.Error("empty tracks should error")
	}

	tracks := []captionTrack{track("ar", ""), track("en", "asr"), track("en", ""), track("pt", "")}

	// no lang → prefer asr (original spoken language)
	got, err := chooseTrack(tracks, "")
	if err != nil || got.LanguageCode != "en" || got.Kind != "asr" {
		t.Errorf("default = %+v, %v; want en/asr", got, err)
	}

	// no lang, no asr → first track
	got, _ = chooseTrack([]captionTrack{track("ar", ""), track("pt", "")}, "")
	if got.LanguageCode != "ar" {
		t.Errorf("no-asr default = %q; want ar", got.LanguageCode)
	}

	// lang en → prefer manual over asr
	got, err = chooseTrack(tracks, "en")
	if err != nil || got.LanguageCode != "en" || got.Kind != "" {
		t.Errorf("lang en = %+v, %v; want en/manual", got, err)
	}

	// lang with only asr available → returns asr
	got, err = chooseTrack([]captionTrack{track("fr", "asr")}, "fr")
	if err != nil || got.Kind != "asr" {
		t.Errorf("lang fr (asr only) = %+v, %v; want fr/asr", got, err)
	}

	// missing lang → error
	if _, err := chooseTrack(tracks, "zz"); err == nil {
		t.Error("missing lang should error")
	}
}

func TestParseJSON3(t *testing.T) {
	raw := []byte(`{"events":[
		{"tStartMs":0,"dDurationMs":1000,"segs":[{"utf8":"hello"},{"utf8":" world"}]},
		{"tStartMs":1000,"dDurationMs":500,"segs":[{"utf8":"\n"}]},
		{"tStartMs":2000,"dDurationMs":500,"segs":[{"utf8":"again"}]},
		{"tStartMs":3000,"dDurationMs":500}
	]}`)
	segs := parseJSON3(raw)
	if len(segs) != 2 {
		t.Fatalf("got %d segments; want 2 (blank/segless skipped)", len(segs))
	}
	if segs[0].Text != "hello world" || segs[0].StartMs != 0 || segs[0].DurMs != 1000 {
		t.Errorf("seg0 = %+v", segs[0])
	}
	if segs[1].Text != "again" || segs[1].StartMs != 2000 {
		t.Errorf("seg1 = %+v", segs[1])
	}
	if parseJSON3([]byte("not json")) != nil {
		t.Error("invalid json should return nil")
	}
}

func TestParseSRV1(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="utf-8" ?><transcript>` +
		`<text start="0.44" dur="5.56">tom &amp; jerry</text>` +
		`<text start="2.84" dur="1.5">  </text>` +
		`<text start="6.0" dur="2.0">second line</text>` +
		`</transcript>`)
	segs := parseSRV1(raw)
	if len(segs) != 2 {
		t.Fatalf("got %d segments; want 2 (blank skipped)", len(segs))
	}
	if segs[0].Text != "tom & jerry" {
		t.Errorf("seg0 text = %q; want unescaped 'tom & jerry'", segs[0].Text)
	}
	if segs[0].StartMs != 440 || segs[0].DurMs != 5560 {
		t.Errorf("seg0 timing = %d/%d; want 440/5560", segs[0].StartMs, segs[0].DurMs)
	}
	if !strings.Contains(segs[1].Text, "second line") {
		t.Errorf("seg1 = %q", segs[1].Text)
	}
}

func TestParseSRV1Rounding(t *testing.T) {
	// 2.01s*1000 is 2009.999… in float; truncation gives 2009, rounding gives 2010.
	raw := []byte(`<transcript><text start="2.01" dur="0.03">x</text></transcript>`)
	segs := parseSRV1(raw)
	if len(segs) != 1 {
		t.Fatalf("got %d segments; want 1", len(segs))
	}
	if segs[0].StartMs != 2010 || segs[0].DurMs != 30 {
		t.Errorf("start=%d dur=%d; want 2010/30 (rounded, not truncated)", segs[0].StartMs, segs[0].DurMs)
	}
}
