package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputBase(t *testing.T) {
	cases := []struct {
		id, lang, title, want string
	}{
		{"abc12345678", "pt", "Claude Code: Resultado BIZARRO", "Claude_Code_Resultado_BIZARRO__abc12345678.pt"},
		{"abc12345678", "en", "", "abc12345678.en"},
		{"abc12345678", "en", "!!!", "abc12345678.en"}, // slug collapses to empty → id only
	}
	for _, c := range cases {
		if got := outputBase(c.id, c.lang, c.title); got != c.want {
			t.Errorf("outputBase(%q,%q,%q) = %q; want %q", c.id, c.lang, c.title, got, c.want)
		}
	}
}

func TestOutputBaseSanitizesLang(t *testing.T) {
	// a hostile lang (e.g. from -translate or a spoofed languageCode) must not
	// introduce path separators or ".." into the filename.
	for _, lang := range []string{"../../etc", "..\\..\\x", "/abs/path", "a/b"} {
		got := outputBase("abc12345678", lang, "Title")
		if strings.ContainsAny(got, `/\`) || strings.Contains(got, "..") {
			t.Errorf("outputBase lang=%q not sanitized: %q", lang, got)
		}
	}
}

func TestWriteFileRefusesEscape(t *testing.T) {
	dir := t.TempDir()
	if _, err := writeFile(dir, filepath.Join("..", "escape.txt"), "x"); err == nil {
		t.Error("writeFile should refuse a name that escapes the output dir")
	}
	if _, err := writeFile(dir, "ok.txt", "x"); err != nil {
		t.Errorf("writeFile rejected a valid name: %v", err)
	}
}

func TestOutputBaseTruncates(t *testing.T) {
	long := strings.Repeat("a", 200)
	got := outputBase("abc12345678", "pt", long)
	// title slug capped at 60 chars, then "__<id>.pt"
	if !strings.HasSuffix(got, "__abc12345678.pt") {
		t.Errorf("missing id suffix: %q", got)
	}
	slug := strings.TrimSuffix(got, "__abc12345678.pt")
	if len(slug) > 60 {
		t.Errorf("slug not truncated: %d chars", len(slug))
	}
}
