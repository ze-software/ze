// Design: docs/guide/ddos-mitigation.md -- what a commit does to a live FlowSpec announcement
// Related: register.go (runEngine parseSections, replaceResponder), exit_test.go (the engine-pair harness)

package flowspec

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// configStatus sends one config-verify or config-apply callback and returns the
// status and error the plugin answered. The SDK answers a refusal as a result
// carrying status "error", not as an RPC error, so the caller reads both.
func (e *engineStub) configStatus(ctx context.Context, method string, input any) (status, reason string) {
	e.t.Helper()
	raw, err := e.mux.CallRPC(ctx, method, input)
	if err != nil {
		e.t.Fatalf("%s: %v", method, err)
	}
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		e.t.Fatalf("%s answer %s: %v", method, raw, err)
	}
	return out.Status, out.Error
}

// TestReloadWithoutTheSectionWithdrawsTheAnnouncement proves that a reload
// whose candidate no longer holds `ddos flowspec` is accepted at verify and
// withdraws the live rule at apply.
//
// VALIDATES: AC-12. Deleting the `ddos flowspec` block, or the parent `ddos`
// block, reaches the plugin as a config-verify whose section carries no
// flowspec body, then a config-apply. An absent section has no `action` to
// validate, so verify must accept it, and the apply must withdraw the rule
// through the section-removed path before the plugin is stopped.
// METHOD: runEngine over a real sdk.Plugin pair configured with action discard,
// one announcement through the bus, then the verify and apply a reload sends;
// the engine stub's record is read once the apply answer arrived.
// PREVENTS: a verify that validated the DefaultConfig of an absent section and
// refused it with `action "" must be rate-limit or discard`, so the reload
// was refused and the rule stayed at every peer.
func TestReloadWithoutTheSectionWithdrawsTheAnnouncement(t *testing.T) {
	cases := []struct {
		name string
		data string // the ddos/flowspec section body the reload delivers
	}{
		// The server sends an empty body for a root the new tree no longer holds.
		{name: "section removed", data: ""},
		// A `ddos` block that keeps another child and drops `flowspec`.
		{name: "ddos block without flowspec", data: `{"ddos":{"detect":{"enabled":"true"}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			e, bus := startEngineStub(t, nil)
			announceVictim(t, e, bus)

			status, reason := e.configStatus(ctx, "ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{
				Sections: []rpc.ConfigSection{{Root: configRoot, Data: tc.data}},
			})
			if status != rpc.StatusOK {
				t.Fatalf("config-verify of a candidate without ddos flowspec answered %q: %s, want %q",
					status, reason, rpc.StatusOK)
			}

			status, reason = e.configStatus(ctx, "ze-plugin-callback:config-apply", rpc.ConfigApplyInput{
				Sections: []rpc.ConfigDiffSection{{Root: configRoot}},
			})
			if status != rpc.StatusOK {
				t.Fatalf("config-apply after the accepted verify answered %q: %s, want %q",
					status, reason, rpc.StatusOK)
			}

			if got := countCommands(e.routeCommands(), "/flow del "); got != 1 {
				t.Fatalf("withdrawals for %s after the apply = %d, want 1 (commands: %v)",
					exitTestVictim, got, e.routeCommands())
			}
		})
	}
}
