//go:build network

// These tests hit the real YouTube endpoints. They are excluded from the default
// `go test ./...` run and only execute with `go test -tags network ./...`.
//
// They are the canary: when YouTube changes something and a method stops working,
// these fail and tell you which one. Keep a stable, caption-bearing video here.
package transcript

import (
	"context"
	"testing"
	"time"
)

const probeVideoID = "ZHjH9qSiJzQ"

func TestNetworkFetch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := Fetch(ctx, probeVideoID, Options{})
	if err != nil {
		t.Fatalf("Fetch(%s) failed — YouTube may have changed: %v", probeVideoID, err)
	}
	if len(res.Segments) == 0 {
		t.Fatalf("Fetch returned 0 segments")
	}
	t.Logf("OK via %s: %d segments, lang=%s, title=%q", res.Method, len(res.Segments), res.Lang, res.Meta.Title)
}

// TestNetworkEachSource probes every acquisition method independently so a partial
// breakage (e.g. android dead but ios alive) is visible instead of hidden by the cascade.
func TestNetworkEachSource(t *testing.T) {
	ctx := context.Background()
	working := 0
	for _, src := range sources() {
		pr, err := src.fetch(ctx, probeVideoID, defaultClient)
		if err != nil {
			t.Logf("%-18s fetch err: %v", src.name, err)
			continue
		}
		tracks := pr.tracks()
		if len(tracks) == 0 {
			t.Logf("%-18s no tracks (status=%s)", src.name, pr.PlayabilityStatus.Status)
			continue
		}
		ct, _ := chooseTrack(tracks, "")
		segs, derr := fetchSegments(ctx, defaultClient, ct.BaseURL, "")
		if derr != nil {
			t.Logf("%-18s tracks=%d DOWNLOAD ERR: %v", src.name, len(tracks), derr)
			continue
		}
		working++
		t.Logf("%-18s OK tracks=%d segments=%d", src.name, len(tracks), len(segs))
	}
	if working == 0 {
		t.Fatal("no acquisition method is working against live YouTube")
	}
}
