// VALIDATES: advisory English readability scoring (Flesch-Kincaid grade,
// sentence word counts, clause nesting detection, YAML front matter line
// preservation, and fenced/inline code exclusion) via `le doc ste readability`.
// PREVENTS: code blocks inflating English grade scores, front matter shifting
// line numbers, or unreadable missing files exiting 0.
package docste

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountSyllables(t *testing.T) {
	t.Parallel()
	cases := []struct {
		word string
		want int
	}{
		{word: "ze", want: 1},
		{word: "route", want: 1},
		{word: "router", want: 2},
		{word: "simple", want: 2},
		{word: "table", want: 2},
		{word: "diagnostics", want: 4},
		{word: "configuration", want: 5},
	}
	for _, tc := range cases {
		if got := countSyllables(tc.word); got != tc.want {
			t.Errorf("countSyllables(%q) = %d, want %d", tc.word, got, tc.want)
		}
	}
}

func TestAnalyzeReadabilitySimpleVsDense(t *testing.T) {
	t.Parallel()

	simpleDoc := strings.Join([]string{
		"---",
		"title: Simple post",
		"date: 2026-11-27",
		"---",
		"",
		"## Heading",
		"",
		"The router starts fast ([docs](https://example.com/a)). It keeps every peer up. You read one clean screen.",
		"",
		"```sh",
		"ze cli -c \"show system triage | json\" --very-long-flag-that-is-not-prose",
		"```",
	}, "\n")

	simple := AnalyzeReadability("simple.md", simpleDoc)
	if simple.Sentences != 3 {
		t.Fatalf("simple.Sentences = %d, want 3 (code block and front matter must be excluded)", simple.Sentences)
	}
	if simple.Rating != "simple" {
		t.Errorf("simple.Rating = %q (grade %.1f), want simple", simple.Rating, simple.FleschKincaidGrade)
	}
	if len(simple.Flagged) != 0 {
		t.Errorf("simple.Flagged = %+v, want empty", simple.Flagged)
	}

	denseDoc := strings.Join([]string{
		"---",
		"title: Dense post",
		"---",
		"",
		"Once you have worked on a router that contains its own faults, keeps enough clear evidence in memory, and exposes that state over CLI or MCP while four hundred sessions run, going back to scrolling logs, tcpdump, and vendor queues feels like working in the dark.",
	}, "\n")

	dense := AnalyzeReadability("dense.md", denseDoc)
	if dense.Sentences != 1 {
		t.Fatalf("dense.Sentences = %d, want 1", dense.Sentences)
	}
	if len(dense.Flagged) != 1 {
		t.Fatalf("dense.Flagged = %+v, want 1 flag", dense.Flagged)
	}
	flag := dense.Flagged[0]
	if flag.Line != 5 {
		t.Errorf("flag.Line = %d, want 5 (front matter line numbers must stay exact)", flag.Line)
	}
	if flag.Reason != "long-and-nested" {
		t.Errorf("flag.Reason = %q, want long-and-nested", flag.Reason)
	}
	if flag.Words <= readabilityLongWords {
		t.Errorf("flag.Words = %d, want > %d", flag.Words, readabilityLongWords)
	}
	if flag.ClauseBreaks < readabilityMaxPunctBreaks {
		t.Errorf("flag.ClauseBreaks = %d, want >= %d", flag.ClauseBreaks, readabilityMaxPunctBreaks)
	}
}

func TestReadabilityActionCLIAndMissingFile(t *testing.T) {
	dir := t.TempDir()
	sample := filepath.Join(dir, "post.md")
	body := "---\ntitle: Test\n---\n\nThe router keeps sessions up. You check one screen.\n"
	if err := os.WriteFile(sample, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	payload, code := Answer([]string{"readability", "file", sample})
	if code != 0 {
		t.Fatalf("readability exit code = %d, want 0", code)
	}
	report, ok := payload.(ReadabilityReport)
	if !ok {
		t.Fatalf("readability answered %T, want ReadabilityReport", payload)
	}
	if report.DocumentsReviewed != 1 {
		t.Errorf("DocumentsReviewed = %d, want 1", report.DocumentsReviewed)
	}
	assertKebabKeys(t, report)
	if !strings.Contains(report.Text(), "ste readability:") {
		t.Errorf("report.Text() missing summary line: %q", report.Text())
	}

	_, missingCode := Answer([]string{"readability", "file", filepath.Join(dir, "absent.md")})
	if missingCode == 0 {
		t.Errorf("readability on missing file exited 0, want non-zero")
	}
}
