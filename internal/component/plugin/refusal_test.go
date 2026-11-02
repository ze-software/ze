package plugin

import (
	"errors"
	"fmt"
	"testing"
)

// TestRefusalErrorKeepsTheOperatorText checks the two readings of a refusal a
// command wrote whole.
//
// VALIDATES: Error drops the leading "error: " (the reason a program reads,
// as Response.Error carries it) and Line keeps the text exactly as the local
// route prints it, minus the final newline, for a refusal that opens with
// "error: " and for one that does not (a usage page). RefusalLine finds the
// refusal through a wrap and answers false for any other error.
// PREVENTS: a surface printing "error: usage: ..." for a refusal `ze
// <command>` prints as "usage: ...", or "error: error: ..." for one it
// prints as "error: ...".
func TestRefusalErrorKeepsTheOperatorText(t *testing.T) {
	cases := []struct {
		diagnostic string
		reason     string
		line       string
	}{
		{diagnostic: "usage: show config list\n", reason: "usage: show config list", line: "usage: show config list"},
		{diagnostic: "error: no such key\n", reason: "no such key", line: "error: no such key"},
		{diagnostic: "error: two\nlines\n", reason: "two\nlines", line: "error: two\nlines"},
	}
	for _, tc := range cases {
		refusal := NewRefusalError(tc.diagnostic)
		if got := refusal.Error(); got != tc.reason {
			t.Errorf("NewRefusalError(%q).Error() = %q, want %q", tc.diagnostic, got, tc.reason)
		}
		line, refused := RefusalLine(fmt.Errorf("dispatch: %w", refusal))
		if !refused {
			t.Errorf("RefusalLine did not find the refusal %q through a wrap", tc.diagnostic)
		}
		if line != tc.line {
			t.Errorf("RefusalLine(%q) = %q, want %q", tc.diagnostic, line, tc.line)
		}
	}
	if line, refused := RefusalLine(errors.New("usage: plain")); refused {
		t.Errorf("RefusalLine of a plain error = %q, true; want false: the surface renders it its own way", line)
	}
}
