# Contributing

Thanks for helping out. This is a small, focused tool — issues, bug reports, and PRs are
all welcome. "Installed and it broke on my setup" is genuinely useful feedback.

## Getting started

```sh
git clone https://github.com/rapha30/yt-youtube-transcript
cd yt-youtube-transcript
make test      # should pass offline, no network needed
make build     # ./yt-transcript
```

You need Go 1.23+. There are no third-party dependencies.

## Before opening a PR

Run the local gate:

```sh
make ci        # fmtcheck + vet + test-race + vuln
make lint      # golangci-lint (includes gosec)
```

All of these run in CI on every PR (Linux/macOS/Windows), plus CodeQL and a Docker build.

## Guidelines

- **Add a test for every change.** Pure logic → an offline unit test. Anything that hits
  YouTube → a test behind `//go:build network` (it must not run in the default suite).
- **Keep the `transcript/` package network-free to test**: route all HTTP through
  `Options.Client`. See existing tests for the fake-transport pattern.
- **Don't switch the acquisition method to the WEB client** — it's pot-token gated and
  returns empty. See `AGENTS.md` for the load-bearing rules.
- Keep functions small and names grep-friendly. Run `gofmt`.

## Reporting a fetch break

YouTube changes its endpoints often. If fetching stops working, run:

```sh
go test -tags network -run TestNetworkEachSource -v ./transcript/
```

and paste the output in an issue — it shows exactly which method broke.
