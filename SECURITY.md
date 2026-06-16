# Security Policy

## Supported versions

This is a small tool released from `main`. Security fixes land on `main` and in the
next tagged release. Please use the latest release.

## Reporting a vulnerability

Please **do not open a public issue** for security problems.

Use GitHub's private vulnerability reporting:
**Security → Report a vulnerability** on this repository
(<https://github.com/rapha30/yt-youtube-transcript/security/advisories/new>).

Include the version (`yt-transcript -version`), repro steps, and impact. I'll acknowledge
and work on a fix; once released, the advisory is published with credit if you'd like.

## Scope notes

This tool scrapes undocumented YouTube endpoints over HTTPS. Reports about the *fetch*
breaking (YouTube changed something) are normal bugs — use a regular issue. Genuine
security issues (e.g. path traversal, code execution, memory-safety, supply-chain) go
through the private channel above.
