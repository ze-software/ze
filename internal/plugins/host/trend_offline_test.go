package host

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// VALIDATES: with no daemon, show host trend reaches its own fallback, not the
// inventory fallback of show host, which would read trend as an unknown
// section.
// Method: the longest-prefix lookup the CLI runs when the daemon is
// unreachable; the show host fallback would answer it with "trend" left over
// as its section argument.
func TestShowHostTrendOfflineHasItsOwnFallback(t *testing.T) {
	handler, rest := command.LookupOfflineFallback([]string{"show", "host", "trend"})
	if handler == nil {
		t.Fatal("no offline fallback answers show host trend")
	}
	if len(rest) != 0 {
		t.Errorf("show host trend matched a shorter fallback, leaving %q as its arguments", rest)
	}
}

// VALIDATES: the offline answer says the daemon is needed and why, and ends on
// the command that starts it.
// Method: the operator error the fallback prints.
func TestShowHostTrendOfflineRefusalNamesTheDaemon(t *testing.T) {
	text := trendOfflineRefusal().Error()
	lines := strings.Split(text, "\n")
	if len(lines) != 3 {
		t.Fatalf("refusal has %d lines, want problem, why and fix:\n%s", len(lines), text)
	}
	if !strings.Contains(lines[0], "daemon") {
		t.Errorf("problem line %q does not name the daemon", lines[0])
	}
	if lines[2] != "fix: ze start" {
		t.Errorf("fix line = %q, want %q", lines[2], "fix: ze start")
	}
}
