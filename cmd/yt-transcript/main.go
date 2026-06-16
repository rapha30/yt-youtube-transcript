// Command yt-transcript fetches YouTube video transcripts (captions) from the
// command line, with no API key and no external dependencies.
//
// Usage:
//
//	yt-transcript [flags] <url-or-id> [<url-or-id> ...]
//
// Run with -h for the full flag list.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rapha30/yt-youtube-transcript/transcript"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

type options struct {
	lang, translate, out                    string
	list, srt, ts, stdout, nofile, noheader bool
}

func newFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("yt-transcript", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: yt-transcript [flags] <url-or-id> [<url-or-id> ...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Fetch YouTube transcripts (captions). Accepts watch?v=, youtu.be/,")
		fmt.Fprintln(os.Stderr, "/shorts/, /embed/, /live/ URLs or a bare 11-char video ID.")
		fmt.Fprintln(os.Stderr, "")
		fs.PrintDefaults()
	}
	return fs
}

func bindFlags(fs *flag.FlagSet, o *options) {
	fs.StringVar(&o.lang, "lang", "", "caption language code to fetch (default: original/auto-generated track)")
	fs.StringVar(&o.translate, "translate", "", "(experimental) auto-translate via YouTube tlang — often returns empty/429")
	fs.StringVar(&o.out, "out", "transcripts", "output directory for transcript files (created if missing)")
	fs.BoolVar(&o.list, "list", false, "list available caption tracks and exit")
	fs.BoolVar(&o.srt, "srt", false, "also write an .srt subtitle file")
	fs.BoolVar(&o.ts, "ts", false, "also write a [hh:mm:ss]-timestamped .txt file")
	fs.BoolVar(&o.stdout, "stdout", false, "print the plain transcript to stdout")
	fs.BoolVar(&o.nofile, "nofile", false, "do not write any file (pair with -stdout)")
	fs.BoolVar(&o.noheader, "noheader", false, "omit the metadata header at the top of the .txt file")
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	fs := newFlagSet()
	var o options
	bindFlags(fs, &o)
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Println("yt-transcript", version)
		return 0
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	exit := 0
	for _, arg := range fs.Args() {
		if err := process(ctx, arg, o); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			exit = 1
		}
	}
	return exit
}

func process(ctx context.Context, arg string, o options) error {
	id, err := transcript.ExtractVideoID(arg)
	if err != nil {
		return err
	}
	opts := transcript.Options{Lang: o.lang, Translate: o.translate}

	if o.list {
		meta, tracks, method, err := transcript.ListTracks(ctx, id, opts)
		if err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		fmt.Printf("%s — %q  (tracks via %s)\n", id, meta.Title, method)
		if len(tracks) == 0 {
			fmt.Println("  (no caption tracks)")
			return nil
		}
		for _, t := range tracks {
			kind := "manual"
			if t.IsAuto() {
				kind = "auto"
			}
			tr := ""
			if t.Translatable {
				tr = " [translatable]"
			}
			fmt.Printf("  %-6s %-6s %s%s\n", t.LanguageCode, kind, t.Name, tr)
		}
		return nil
	}

	res, err := transcript.Fetch(ctx, id, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", id, err)
	}
	text := res.Text()
	fmt.Printf("OK %s [%s] via %s — %q — %d segments, %d chars\n",
		id, res.Lang, res.Method, res.Meta.Title, len(res.Segments), len(text))

	if o.stdout {
		fmt.Println("\n--- transcript ---")
		fmt.Println(text)
		fmt.Println("--- end ---")
	}
	if o.nofile {
		return nil
	}

	head := ""
	if !o.noheader {
		head = res.Header(time.Now().Format("2006-01-02 15:04"))
	}
	base := outputBase(id, res.Lang, res.Meta.Title)

	writes := []struct {
		ext, content string
		on           bool
	}{
		{".txt", head + text + "\n", true},
		{".srt", res.SRT(), o.srt},
		{".timestamps.txt", head + res.Timestamps(), o.ts},
	}
	for _, w := range writes {
		if !w.on {
			continue
		}
		p, err := writeFile(o.out, base+w.ext, w.content)
		if err != nil {
			return err
		}
		fmt.Println("  ->", p)
	}
	return nil
}

var (
	unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	unsafeLang = regexp.MustCompile(`[^A-Za-z0-9-]+`) // lang codes: alnum + hyphen only
)

// outputBase builds "<title-slug>__<id>.<lang>" — title first so the output folder
// sorts by video name; id keeps it unique. Both lang and title are sanitized so a
// hostile value (e.g. a "-translate ../../x" or a spoofed languageCode) can't inject
// a path separator into the filename.
func outputBase(id, lang, title string) string {
	lang = unsafeLang.ReplaceAllString(lang, "")
	if lang == "" {
		lang = "und"
	}
	base := id + "." + lang
	if title == "" {
		return base
	}
	slug := strings.Trim(unsafeName.ReplaceAllString(title, "_"), "_")
	if len(slug) > 60 {
		slug = strings.Trim(slug[:60], "_")
	}
	if slug == "" {
		return base
	}
	return slug + "__" + id + "." + lang
}

func writeFile(dir, name, content string) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	p := filepath.Join(dir, name)
	// Defense in depth: never write outside the output directory.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	absP, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if absP != absDir && !strings.HasPrefix(absP, absDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("refusing to write outside %s: %s", dir, p)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		return "", err
	}
	return p, nil
}
