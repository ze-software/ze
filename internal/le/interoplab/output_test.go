package interoplab

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestOutputExcerptBoundsThePeersAnswer proves an assertion failure carries a
// short answer whole, a long one cut to its tail with a note that says so, and an
// empty one named as empty.
//
// VALIDATES: OutputExcerpt keeps the last 40 lines and at most 4096 bytes, notes
// the cut, and never starts inside a rune.
// PREVENTS: a red that floods the report with a full routing table, or a cut that
// hides that anything was cut.
func TestOutputExcerptBoundsThePeersAnswer(t *testing.T) {
	t.Parallel()

	if got := OutputExcerpt("  Paths: (2 available, best #1)\n"); got != "Paths: (2 available, best #1)" {
		t.Errorf("a short answer must come back whole and trimmed, got %q", got)
	}
	if got := OutputExcerpt(" \n\t"); got != "(the peer printed nothing)" {
		t.Errorf("an empty answer must be named as empty, got %q", got)
	}

	var lines []string
	for index := range 100 {
		lines = append(lines, "line "+strconv.Itoa(index))
	}
	got := OutputExcerpt(strings.Join(lines, "\n"))
	if !strings.HasPrefix(got, "(output cut: showing the last 40 of 100 lines, ") {
		t.Errorf("a long answer must say it was cut, got %q", got)
	}
	if !strings.HasSuffix(got, "\nline 60\nline 61") && !strings.HasSuffix(got, "line 99") {
		t.Errorf("a long answer must keep its tail, got %q", got)
	}
	if strings.Contains(got, "line 59\n") {
		t.Errorf("the 41st line from the end must be cut, got %q", got)
	}
	if !strings.Contains(got, "\nline 60\n") {
		t.Errorf("the 40th line from the end must be kept, got %q", got)
	}

	wide := strings.Repeat("é", 3000)
	got = OutputExcerpt(wide)
	body := got[strings.IndexByte(got, '\n')+1:]
	if len(body) > outputExcerptBytesMax {
		t.Errorf("the excerpt holds %d bytes, want at most %d", len(body), outputExcerptBytesMax)
	}
	if !strings.HasPrefix(body, "é") {
		t.Errorf("a byte cut must start on a whole rune, got %q", body[:4])
	}
	if !strings.HasPrefix(got, "(output cut: showing the last 1 of 1 lines, ") {
		t.Errorf("a byte cut must say it was cut, got %q", got[:60])
	}
}

// TestWaitTimeoutCarriesTheLastAnswer proves a wait that measured the peer but
// never saw it ready names what the peer last answered, for text and for a typed
// value, and bounds a long answer the same way a one-shot assertion does.
//
// VALIDATES: Wait's not-ready error ends with "last output:" and the excerpt of
// the last measured value; a string passes through OutputExcerpt, a struct prints
// as %v.
// PREVENTS: a timed-out wait that says only "timed out", so diagnosing the red
// needs a temporary code edit to print the peer's state.
func TestWaitTimeoutCarriesTheLastAnswer(t *testing.T) {
	t.Parallel()

	options := WaitOptions{Timeout: 20 * time.Millisecond, Interval: time.Millisecond, Description: "route"}
	_, _, err := Wait(t.Context(), options, func(context.Context) (string, error) {
		return "  Paths: (1 available, best #1)\n", nil
	}, func(string) bool { return false })
	want := "wait for route timed out before the peer became ready; last output:\nPaths: (1 available, best #1)"
	if err == nil || err.Error() != want {
		t.Fatalf("text wait error = %v, want %q", err, want)
	}

	type counters struct{ Received, Sent int }
	_, _, err = Wait(t.Context(), options, func(context.Context) (counters, error) {
		return counters{Received: 3, Sent: 1}, nil
	}, func(counters) bool { return false })
	if err == nil || !strings.HasSuffix(err.Error(), "last output:\n{3 1}") {
		t.Fatalf("typed wait error = %v, want it to end with the last value {3 1}", err)
	}

	var long strings.Builder
	for line := range 100 {
		long.WriteString("route ")
		long.WriteString(strconv.Itoa(line))
		long.WriteByte('\n')
	}
	_, _, err = Wait(t.Context(), options, func(context.Context) (string, error) {
		return long.String(), nil
	}, func(string) bool { return false })
	if err == nil || !strings.Contains(err.Error(), "(output cut: showing the last 40 of 100 lines") {
		t.Fatalf("long wait error = %v, want the excerpt's cut note", err)
	}
	if strings.Contains(err.Error(), "route 59\n") {
		t.Fatalf("long wait error kept a line before the last 40: %v", err)
	}
}
