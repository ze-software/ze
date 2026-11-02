package cli

import "testing"

// TestWithWarningPutsTheWarningAboveTheAnswer checks how an interactive
// session shows an answer the command warned about.
//
// VALIDATES: the warning lines come first and the rendered answer after,
// the order `ze <command>` prints them (stderr before stdout); an answer that
// warns of nothing is unchanged.
// PREVENTS: the SSH session dropping a warning the exec channel shows.
func TestWithWarningPutsTheWarningAboveTheAnswer(t *testing.T) {
	if got := withWarning("warning: a\nwarning: b", "answer\n"); got != "warning: a\nwarning: b\nanswer\n" {
		t.Errorf("withWarning = %q, want the warning lines, then the answer", got)
	}
	if got := withWarning("", "answer\n"); got != "answer\n" {
		t.Errorf("withWarning with no warning = %q, want the answer unchanged", got)
	}
}
