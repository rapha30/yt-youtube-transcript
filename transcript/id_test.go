package transcript

import "testing"

func TestExtractVideoID(t *testing.T) {
	const want = "ZHjH9qSiJzQ"
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"https://www.youtube.com/watch?v=ZHjH9qSiJzQ", want, true},
		{"https://www.youtube.com/watch?v=ZHjH9qSiJzQ&t=10s", want, true},
		{"https://youtu.be/ZHjH9qSiJzQ", want, true},
		{"https://youtu.be/ZHjH9qSiJzQ?si=abc", want, true},
		{"https://www.youtube.com/shorts/ZHjH9qSiJzQ", want, true},
		{"https://www.youtube.com/embed/ZHjH9qSiJzQ", want, true},
		{"https://www.youtube.com/live/ZHjH9qSiJzQ", want, true},
		{"  ZHjH9qSiJzQ  ", want, true},
		{"ZHjH9qSiJzQ", want, true},
		{"https://example.com/not-a-video", "", false},
		{"", "", false},
		{"short", "", false},
	}
	for _, c := range cases {
		got, err := ExtractVideoID(c.in)
		if c.wantOK && (err != nil || got != c.want) {
			t.Errorf("ExtractVideoID(%q) = %q, %v; want %q, nil", c.in, got, err, c.want)
		}
		if !c.wantOK && err == nil {
			t.Errorf("ExtractVideoID(%q) = %q, nil; want error", c.in, got)
		}
	}
}

func TestAtoiSafe(t *testing.T) {
	for in, want := range map[string]int{"100": 100, " 42 ": 42, "": 0, "abc": 0, "-5": -5} {
		if got := atoiSafe(in); got != want {
			t.Errorf("atoiSafe(%q) = %d; want %d", in, got, want)
		}
	}
}
