# AGENTS.md

Rules for AI coding agents working in this repo. Read before editing.

## Layout

- `transcript/` — the library package (no CLI concerns, no `os.Exit`, no flags).
- `cmd/yt-transcript/` — the CLI: flag parsing, file output, stdout. Imports the library.
- Tests live next to the code. Network tests are behind `//go:build network`.

## Hard rules

- **Acquisition method = ANDROID/IOS InnerTube + watch-page fallback. NEVER switch to the
  WEB client** — its caption URLs are pot-token gated and return empty bodies.
- **Keep the library network-free to test.** All HTTP goes through `Options.Client`; never
  call `http.DefaultClient` or `http.Get` directly. This is what makes the cascade testable
  offline with a fake `RoundTripper`.
- **Every new behaviour gets a test.** Pure logic → offline unit test. Anything that hits
  YouTube → a `//go:build network` test, never the default suite.
- **`context.Context` is threaded through every request.** Don't drop it.
- Don't strip provenance comments (the "why ANDROID not WEB" notes). They're load-bearing.

## Commands

```sh
make test          # offline unit tests — must pass
make test-race     # race detector
make test-network  # live YouTube (only when touching fetch logic)
make fmtcheck vet  # formatting + vet
make lint vuln     # golangci-lint (gosec) + govulncheck
```

Before committing: `make ci` must be green (`fmtcheck + vet + test-race + vuln`).

## Gotchas

- `captionURL` replaces the `fmt`/`tlang` query values **in place** — do not go back to
  strip-and-append, it corrupts URLs where `fmt` is the first query param.
- Default language (no `-lang`) = the `asr`/auto-generated track = original spoken language.
- `-translate` is experimental and usually returns empty/429; don't present it as reliable.
