// VALIDATES: the log command RPC registrations and the arg-parsing error branches
// of handleLogRecent (missing values, unknown option, non-positive count) and
// handleLogSet (usage on too few args), plus the happy path of handleLogRecent.
// PREVENTS: a malformed log-recent/log-set invocation being accepted, or the RPC
// set losing a wire method.

package cmd

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
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
