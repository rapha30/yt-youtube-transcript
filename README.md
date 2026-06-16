# yt-transcript

[![CI](https://github.com/rapha30/yt-youtube-transcript/actions/workflows/ci.yml/badge.svg)](https://github.com/rapha30/yt-youtube-transcript/actions/workflows/ci.yml)
[![CodeQL](https://github.com/rapha30/yt-youtube-transcript/actions/workflows/codeql.yml/badge.svg)](https://github.com/rapha30/yt-youtube-transcript/actions/workflows/codeql.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/rapha30/yt-youtube-transcript.svg)](https://pkg.go.dev/github.com/rapha30/yt-youtube-transcript)
[![Go Report Card](https://goreportcard.com/badge/github.com/rapha30/yt-youtube-transcript)](https://goreportcard.com/report/github.com/rapha30/yt-youtube-transcript)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Fetch the transcript (captions) of any YouTube video — **no API key, no external
dependencies** (Go standard library only). Use it as a CLI or as a Go library.

```console
$ yt-transcript -stdout -nofile "https://youtu.be/ZHjH9qSiJzQ"
OK ZHjH9qSiJzQ [pt] via innertube-android — "Claude Code + VS Code + Skills" — 536 segments, 19238 chars

--- transcript ---
Essa página aqui, ela foi criada com o Gemini...
--- end ---
```

## Why it exists

YouTube's **WEB**-client caption URLs became "pot token" gated in mid-2025 and now
return an empty body. This tool uses the **ANDROID/IOS InnerTube** clients, whose
signed caption URLs still work without any token, and cascades through three
independent acquisition methods so a single throttled/blocked endpoint doesn't take
the whole thing down.

| # | Method | Downloads? | Saves the day when… |
|---|--------|------------|---------------------|
| 1 | `innertube-android` | ✅ | primary |
| 2 | `innertube-ios` | ✅ | ANDROID is throttled/blocked |
| 3 | `watch-page` (scrape `ytInitialPlayerResponse`) | ⚠️ lists tracks; download empty on pot-gated videos | the `youtubei` POST endpoint is blocked (a totally different request) |

Within a method, the caption download also falls back `json3 → srv1` (XML). The tool
reports which method won (`via innertube-android`).

## Install

```sh
# Go 1.23+ — install the CLI
go install github.com/rapha30/yt-youtube-transcript/cmd/yt-transcript@latest

# Prebuilt binaries: see the Releases page.

# Docker — a multi-arch image is published to GHCR on every release and main build:
docker run --rm ghcr.io/rapha30/yt-youtube-transcript -stdout -nofile "https://youtu.be/VIDEO_ID"
```

Build from source (no Go toolchain needed for the Docker path):

```sh
git clone https://github.com/rapha30/yt-youtube-transcript
cd yt-youtube-transcript
make build      # produces ./yt-transcript

# or build the image locally:
docker build -t yt-transcript .
docker run --rm yt-transcript -stdout -nofile "https://youtu.be/VIDEO_ID"
```

## CLI usage

```sh
yt-transcript [flags] <url-or-id> [<url-or-id> ...]
```

Accepts `watch?v=`, `youtu.be/`, `/shorts/`, `/embed/`, `/live/` URLs or a bare 11-char ID.

```sh
# basic — writes ./transcripts/<title>__<id>.<lang>.txt
yt-transcript "https://youtu.be/ZHjH9qSiJzQ"

# several at once
yt-transcript <url1> <url2> <url3>

# list available caption tracks
yt-transcript -list <url>

# specific language (prefers a manual track over auto-generated)
yt-transcript -lang en <url>

# print to stdout, write nothing (handy for piping / agents)
yt-transcript -stdout -nofile <url>

# also write .srt and a [hh:mm:ss] timestamped file
yt-transcript -srt -ts <url>

# choose output directory
yt-transcript -out ./out <url>
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-lang <code>` | original (asr) | caption language to fetch |
| `-translate <code>` | — | **experimental** YouTube auto-translation (often empty/429) |
| `-out <dir>` | `transcripts` | output directory (created if missing) |
| `-list` | — | list caption tracks and exit |
| `-srt` | — | also write `.srt` |
| `-ts` | — | also write `.timestamps.txt` |
| `-stdout` | — | print the plain transcript to stdout |
| `-nofile` | — | don't write any file (pair with `-stdout`) |
| `-noheader` | — | omit the `# …` metadata header from the `.txt` |
| `-version` | — | print version and exit |

### Output

- **Folder**: `transcripts/` by default (`-out` to change).
- **Name**: `<title-slug>__<id>.<lang>.<ext>` — title first so the folder sorts by
  video name; the id keeps it unique. Re-running overwrites (deterministic name).
- **Header**: `.txt` / `.timestamps.txt` start with a `# …` block (title, channel, URL,
  language/method/duration, date). Use `-noheader` for pure text; `.srt` never gets one.
- **Language**: without `-lang`, the **auto-generated (asr)** track is used = the
  original spoken language (so a video with 20 translated tracks still gives you the original).

## Library usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/rapha30/yt-youtube-transcript/transcript"
)

func main() {
	id, err := transcript.ExtractVideoID("https://youtu.be/ZHjH9qSiJzQ")
	if err != nil {
		log.Fatal(err)
	}
	res, err := transcript.Fetch(context.Background(), id, transcript.Options{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s via %s\n", res.Meta.Title, res.Method)
	fmt.Println(res.Text()) // also: res.SRT(), res.Timestamps()
}
```

`Options` accepts a custom `*http.Client` (and `context.Context` is honored for
cancellation/timeouts), which makes the cascade fully testable offline.

## Translation caveat

`-translate` requests YouTube's `tlang` auto-translation. As of testing it frequently
returns an empty body or `429` on the signed ANDROID/IOS URLs, so it's marked
experimental. For another language, fetch the original and translate it yourself.

## Development

```sh
make test          # offline unit tests (no network)
make test-race     # with the race detector
make test-network  # live YouTube probe (the canary for "did YouTube change?")
make lint          # golangci-lint (includes gosec)
make vuln          # govulncheck
make cover         # coverage report
make ci            # fmtcheck + vet + test-race + vuln
```

Tests are layered: pure functions and the **full acquisition cascade** run offline via
a fake `http.RoundTripper`; the live integration tests sit behind `-tags network` so the
default `go test ./...` never flakes on YouTube.

## Security

- **CodeQL** static analysis on every push/PR.
- **gosec** (via golangci-lint) for SAST.
- **govulncheck** for known vulnerabilities in code/stdlib.
- **Dependabot** keeps GitHub Actions, the Docker base image, and the (zero) Go deps current.
- **Private vulnerability reporting**: see [SECURITY.md](SECURITY.md).

## How fragile is this?

It scrapes undocumented YouTube endpoints, so it *will* break when YouTube changes
them. That's why the network tests probe each method independently — when they go red,
they tell you which method died. If both ANDROID and IOS ever stop working at once, the
next step is generating a `pot` (proof-of-origin) token, which needs a JS runtime
(BotGuard) and is out of scope for a stdlib-only tool. See `yt-dlp` /
`youtube-transcript-api` for that.

## Disclaimer

For personal/educational use. You are responsible for complying with YouTube's Terms of
Service and applicable copyright law in your jurisdiction.

## License

[MIT](LICENSE) © 22ehnutz
