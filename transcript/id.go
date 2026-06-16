package transcript

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	idFromURLRe = regexp.MustCompile(`(?:v=|youtu\.be/|/shorts/|/embed/|/live/|/v/)([A-Za-z0-9_-]{11})`)
	bareIDRe    = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
)

// ExtractVideoID pulls an 11-character YouTube video ID out of a URL
// (watch?v=, youtu.be/, /shorts/, /embed/, /live/, /v/) or accepts a bare ID.
func ExtractVideoID(input string) (string, error) {
	s := strings.TrimSpace(input)
	if bareIDRe.MatchString(s) && !strings.Contains(s, "/") {
		return s, nil
	}
	if m := idFromURLRe.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("could not extract a video ID from %q", input)
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
