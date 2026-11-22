package host

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
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
// Method: the operator error the fallback prints when the route found no
// listener, the one cause whose fix is a start.
func TestShowHostTrendOfflineRefusalNamesTheDaemon(t *testing.T) {
	noListener := command.NewDaemonUnanswered("no ze listens at 127.0.0.1:2222 (connection refused)", "ze start")
	text := trendOfflineRefusal(noListener).Error()
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

// VALIDATES: AC-32 -- the refusal of show host trend blames the cause the
// route met and ends on that cause's fix, for each cause the CLI client
// classifies: no listener, a listener this account holds no credentials for,
// and a listener whose connection failed.
// PREVENTS: "no daemon answered" with the fix `ze start` while a daemon runs,
// which is what the refusal said for every failure when it worded the cause
// itself.
// Method: the operator error for each cause, as command.DaemonUnanswered
// carries it; the first line must hold the cause's own words and the last
// line its own fix, and `ze start` must appear only for the missing listener.
func TestShowHostTrendOfflineRefusalBlamesTheRealCause(t *testing.T) {
	tests := []struct {
		name string
		why  string
		fix  string
	}{
		{"no-listener", "no ze listens at 127.0.0.1:2222 (connection refused)", "ze start"},
		{
			"credentials-unavailable",
			"a ze listens at 127.0.0.1:2222, but this account has no credentials to log in to it (no stored username)",
			`ze cli --user <name> -c "show host trend"`,
		},
		{
			"connection-failed",
			"something listens at 127.0.0.1:2222, but the connection to it failed (handshake timed out)",
			"ze status",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := trendOfflineRefusal(command.NewDaemonUnanswered(tt.why, tt.fix)).Error()
			lines := strings.Split(text, "\n")
			if len(lines) != 3 {
				t.Fatalf("refusal has %d lines, want problem, why and fix:\n%s", len(lines), text)
			}
			if lines[0] != "show host trend needs the daemon, and "+tt.why {
				t.Errorf("problem line = %q, want it to end on the cause %q", lines[0], tt.why)
			}
			if lines[2] != "fix: "+tt.fix {
				t.Errorf("fix line = %q, want %q", lines[2], "fix: "+tt.fix)
			}
			if tt.fix != "ze start" && strings.Contains(text, "ze start") {
				t.Errorf("a daemon that runs is answered with a start:\n%s", text)
			}
		})
	}
}

// VALIDATES: AC-32 -- the fallback prints that refusal and exits 1, so the
// route adds no "answered in this process" note to a command that was not
// answered.
// Method: the registered handler, run with a cause, with stderr captured.
func TestShowHostTrendOfflinePrintsTheCauseItWasHanded(t *testing.T) {
	credentials := command.NewDaemonUnanswered(
		"a ze listens at 127.0.0.1:2222, but this account has no credentials to log in to it (no stored username)",
		`ze cli --user <name> -c "show host trend"`)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = writer
	code := runShowTrendOffline(commandtest.Args(), credentials)
	os.Stderr = saved
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	printed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("exit = %d, want 1: the trend was not answered", code)
	}
	want := "error: " + trendOfflineRefusal(credentials).Error() + "\n"
	if string(printed) != want {
		t.Errorf("stderr = %q, want %q", printed, want)
	}
}
