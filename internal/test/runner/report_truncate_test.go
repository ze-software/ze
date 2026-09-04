package runner

import (
	"strconv"
	"strings"
	"testing"
)

func numberedLines(n int) string {
	lines := make([]string, 0, n)
	for i := range n {
		lines = append(lines, "line"+strconv.Itoa(i))
	}
	return strings.Join(lines, "\n")
}

// VALIDATES: a truncated capture keeps its TAIL as well as its head.
//
// PREVENTS: the failure mode that cost a session several hours on 2026-09-03.
// Head-only truncation is indistinguishable from a producer that went silent,
// and a fixture's per-round diagnostics were read as never having been printed
// when in fact they were relayed and cut. A wrong conclusion was drawn, written
// into a journal row and a spec, committed, and later retracted.
func TestTruncateOutputKeepsTheTailNotJustTheHead(t *testing.T) {
	got := truncateOutput(numberedLines(100), 30)

	if !strings.Contains(got, "line0") {
		t.Errorf("the head is gone; want line0 in:\n%s", got)
	}
	if !strings.Contains(got, "line99") {
		t.Errorf("the TAIL is gone, which is the whole point of this function; want line99 in:\n%s", got)
	}
	if strings.Contains(got, "line50") {
		t.Errorf("the middle survived, so nothing was actually elided:\n%s", got)
	}
	if !strings.Contains(got, "70 lines elided") {
		t.Errorf("the elision line does not say how much went; got:\n%s", got)
	}
}

// VALIDATES: the bound is honoured, so a report cannot grow past what its
// caller asked for, and short input is returned untouched.
func TestTruncateOutputRespectsItsBoundAndPassesShortInputThrough(t *testing.T) {
	got := truncateOutput(numberedLines(100), 30)
	// 30 content lines plus the one elision line.
	if n := len(strings.Split(got, "\n")); n != 31 {
		t.Errorf("truncated to %d lines, want 31 (30 content + 1 elision)", n)
	}

	short := numberedLines(30)
	if truncateOutput(short, 30) != short {
		t.Error("input at the bound was modified; it must pass through untouched")
	}
	if truncateOutput("", 30) != "" {
		t.Error("empty input was modified")
	}
}

// VALIDATES: an odd bound splits without losing or duplicating a line, which is
// where an off-by-one in the head/tail split would show.
func TestTruncateOutputSplitsAnOddBoundCleanly(t *testing.T) {
	got := truncateOutput(numberedLines(21), 7)
	if n := len(strings.Split(got, "\n")); n != 8 {
		t.Errorf("odd bound produced %d lines, want 8 (7 content + 1 elision)", n)
	}
	if !strings.Contains(got, "line0") || !strings.Contains(got, "line20") {
		t.Errorf("odd bound lost an end:\n%s", got)
	}
	if strings.Contains(got, "line10") {
		t.Errorf("odd bound kept the middle:\n%s", got)
	}
}
