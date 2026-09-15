// VALIDATES: the log command RPC registrations and the arg-parsing error branches
// of handleLogRecent (missing values, unknown option, non-positive count) and
// handleLogSet (usage on too few args), plus the happy path of handleLogRecent.
// PREVENTS: a malformed log-recent/log-set invocation being accepted, or the RPC
// set losing a wire method.

package cmd

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestRPCsRegistered(t *testing.T) {
	rpcs := RPCs()
	want := map[string]bool{"ze-bgp:log-levels": false, "ze-bgp:log-set": false, "ze-bgp:log-recent": false}
	for _, r := range rpcs {
		if _, ok := want[r.WireMethod]; ok {
			want[r.WireMethod] = true
		}
	}
	for method, seen := range want {
		if !seen {
			t.Errorf("RPCs() missing wire method %q", method)
		}
	}
}

func TestHandleLogRecentErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"level missing value", []string{"level"}},
		{"component missing value", []string{"component"}},
		{"count missing value", []string{"count"}},
		{"count non-numeric", []string{"count", "abc"}},
		{"count zero", []string{"count", "0"}},
		{"unknown option", []string{"bogus"}},
	} {
		resp, err := handleLogRecent(nil, tc.args)
		if err != nil {
			t.Errorf("%s: unexpected transport error %v", tc.name, err)
			continue
		}
		if resp.Status != plugin.StatusError {
			t.Errorf("%s: status = %v, want StatusError", tc.name, resp.Status)
		}
	}
}

func TestHandleLogRecentHappyPath(t *testing.T) {
	resp, err := handleLogRecent(nil, []string{"count", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Errorf("status = %v, want StatusDone", resp.Status)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("resp.Data is %T, want plugin.Map", resp.Data)
	}
	if _, ok := data["entries"]; !ok {
		t.Error("response missing entries key")
	}
}

func TestHandleLogSetUsage(t *testing.T) {
	resp, err := handleLogSet(nil, []string{"onlyone"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Errorf("status = %v, want StatusError for too few args", resp.Status)
	}
}

// registeredHandler answers the handler RPCs registers for a wire method, so a
// test drives the command through its registration rather than the function.
func registeredHandler(t *testing.T, wireMethod string) pluginserver.Handler {
	t.Helper()
	for _, r := range RPCs() {
		if r.WireMethod == wireMethod {
			return r.Handler
		}
	}
	t.Fatalf("RPCs() registers no handler for %q", wireMethod)
	return nil
}

// TestLogSetDisabledSilencesSubsystem drives `request log level <subsystem>
// disabled` through the registered log-set handler and checks that the
// subsystem then writes nothing, that `show log levels` reports it as
// disabled, and that a later level enables it again.
//
// VALIDATES: the enumeration value `disabled` the command offers is accepted
// and silences the subsystem.
// PREVENTS: the schema offering a level word the handler refuses.
func TestLogSetDisabledSilencesSubsystem(t *testing.T) {
	const subsystem = "test.log.set.disabled"
	logger := slogutil.Logger(subsystem)
	if !logger.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("precondition: a fresh logger starts at warn")
	}

	set := registeredHandler(t, "ze-bgp:log-set")
	resp, err := set(nil, []string{subsystem, "disabled"})
	if err != nil {
		t.Fatalf("log-set transport error: %v", err)
	}
	if resp.Status != plugin.StatusError && logger.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("request log level disabled answered done and left an ERROR record enabled")
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("request log level %s disabled: status %v, error %q, want StatusDone", subsystem, resp.Status, resp.Error)
	}

	levels := registeredHandler(t, "ze-bgp:log-levels")
	resp, err = levels(nil, nil)
	if err != nil {
		t.Fatalf("log-levels transport error: %v", err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("log-levels Data is %T, want plugin.Map", resp.Data)
	}
	reported, ok := data["levels"].(map[string]string)
	if !ok {
		t.Fatalf("log-levels levels is %T, want map[string]string", data["levels"])
	}
	if reported[subsystem] != "disabled" {
		t.Fatalf("show log levels reports %q for %s, want disabled", reported[subsystem], subsystem)
	}

	resp, err = set(nil, []string{subsystem, "info"})
	if err != nil {
		t.Fatalf("log-set transport error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("request log level %s info after disabled: status %v, error %q", subsystem, resp.Status, resp.Error)
	}
	if !logger.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("request log level info after disabled left the logger silent")
	}
}

// recentMessages answers the messages `show log recent` returns for args,
// through the registered log-recent handler.
func recentMessages(t *testing.T, args []string) []string {
	t.Helper()
	recent := registeredHandler(t, "ze-bgp:log-recent")
	resp, err := recent(nil, args)
	if err != nil {
		t.Fatalf("log-recent %v transport error: %v", args, err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("log-recent %v: status %v, error %q, want StatusDone", args, resp.Status, resp.Error)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("log-recent Data is %T, want plugin.Map", resp.Data)
	}
	entries, ok := data["entries"].([]map[string]any)
	if !ok {
		t.Fatalf("log-recent entries is %T, want []map[string]any", data["entries"])
	}
	messages := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry["component"] != "test.log.recent.level" {
			continue
		}
		message, ok := entry["message"].(string)
		if !ok {
			t.Fatalf("log-recent message is %T, want string", entry["message"])
		}
		level, ok := entry["level"].(string)
		if !ok {
			t.Fatalf("log-recent level is %T, want string", entry["level"])
		}
		messages = append(messages, message+" at "+level)
	}
	return messages
}

// TestLogRecentLevelFilterMatchesTypedWord writes one record at info and one
// at error, then drives `show log recent level <word>` through the registered
// log-recent handler with the words the enumeration offers. Each filter
// answers the entry written at that level and no other, the level field of
// the answer carries the spelling `show log levels` reports, and a word that
// names no level is refused.
//
// VALIDATES: the filter word an operator types selects the entries the ring
// stored, whatever case slog spells the level in.
// PREVENTS: every level filter answering no entry because the ring stored
// slog's upper-case name and the enumeration offers the lower-case one.
func TestLogRecentLevelFilterMatchesTypedWord(t *testing.T) {
	const subsystem = "test.log.recent.level"
	logger := slogutil.Logger(subsystem)
	if err := slogutil.SetLevel(subsystem, "info"); err != nil {
		t.Fatalf("SetLevel(info): %v", err)
	}
	logger.Info("recent filter probe")
	logger.Error("recent filter failure")

	got := recentMessages(t, []string{"level", "info"})
	if len(got) != 1 || got[0] != "recent filter probe at info" {
		t.Fatalf("show log recent level info answered %q, want the one info entry", got)
	}

	got = recentMessages(t, []string{"level", "err"})
	if len(got) != 1 || got[0] != "recent filter failure at error" {
		t.Fatalf("show log recent level err answered %q, want the one error entry", got)
	}

	got = recentMessages(t, []string{"level", "warn", "component", subsystem})
	if len(got) != 0 {
		t.Fatalf("show log recent level warn answered %q, want no entry", got)
	}

	recent := registeredHandler(t, "ze-bgp:log-recent")
	resp, err := recent(nil, []string{"level", "verbose"})
	if err != nil {
		t.Fatalf("log-recent transport error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("show log recent level verbose: status %v, want StatusError for a word that names no level", resp.Status)
	}
}
