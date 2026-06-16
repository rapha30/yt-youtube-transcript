---
name: Bug report
about: Something doesn't work (especially "installed and it broke on my setup")
title: ""
labels: bug
---

**What happened**
A clear description of the bug.

**Command / code**
The exact command you ran (or the library call), with the video URL/ID if relevant.

```
yt-transcript ...
```

**Output**
Paste the full output, including the `all methods failed: ...` lines if present.

**Environment**
- OS + arch:
- Go version (`go version`) or Docker image:
- yt-transcript version (`yt-transcript -version`):

**Notes**
YouTube changes its endpoints often. If `-version` works but fetching a specific
video returns empty, please include the video ID — it helps reproduce.
