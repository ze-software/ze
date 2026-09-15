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
