package irr

import (
	"context"
	"slices"
	"strings"
	"testing"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	irryang "github.com/ze-software/ze/internal/plugins/irr/yang"
)

// runWorker starts the owner's worker, as OnStarted does, and returns the stop
// that cancels it and waits for it to return. After stop the test may read the
// fake resolver the worker wrote.
func (h *ownerHarness) runWorker() (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	h.owner.start(ctx)
	return func() {
		cancel()
		h.owner.stopped()
	}
}

// VALIDATES: `update irr asn <asn>` and `clear irr asn <asn>` refuse a value
// outside the 32-bit AS number range, or one that is no number, instead of
// narrowing it into another AS number, and never ask PeeringDB or the IRR
// server for it. Both ends of the range reach the handler with the number the
// operator typed. (Replaces filter_irr TestUpdateASNRejectsOutOfRange, whose
// command moved to the owner.)
// METHOD: run each row through handleCommand on a running worker over a fake
// resolver, then stop the worker and read every question the resolver got.
// PREVENTS: "4294967296" becoming AS 0 and a lookup the operator never asked
// for, or a refusal that names a truncated AS number.
func TestUpdateIRRAsnRejectsOutOfRange(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		valid   bool
		number  uint32
		wantErr string
	}{
		{name: "zero", args: []string{"0"}, valid: true, number: 0},
		{name: "last valid", args: []string{"4294967295"}, valid: true, number: 4294967295},
		{name: "first invalid above", args: []string{"4294967296"}, wantErr: "is not an AS number"},
		{name: "far above", args: []string{"18446744073709551615"}, wantErr: "is not an AS number"},
		{name: "negative", args: []string{"-1"}, wantErr: "is not an AS number"},
		{name: "not a number", args: []string{"asn65001"}, wantErr: "is not an AS number"},
		{name: "empty", args: []string{""}, wantErr: "is not an AS number"},
		{name: "missing", args: nil, wantErr: "needs one <asn>"},
	}
	for _, command := range []string{cmdUpdateIRRAsn, cmdClearIRRAsn} {
		for _, tc := range tests {
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				h := newHarness(t, newFakeState(), defaultOwnerConfig())
				stop := h.runWorker()
				status, data, err := h.owner.handleCommand(command, tc.args)
				stop()

				if !tc.valid {
					if status != statusError || err == nil {
						t.Fatalf("%s %q: status %q err %v, want %q and an error", command, tc.args, status, err, statusError)
					}
					if !strings.Contains(err.Error(), tc.wantErr) {
						t.Fatalf("%s %q: error %q, want it to contain %q", command, tc.args, err, tc.wantErr)
					}
					if len(h.resolver.asked) != 0 || len(h.resolver.queries) != 0 {
						t.Fatalf("%s %q asked PeeringDB %v and IRR %v, want nothing", command, tc.args, h.resolver.asked, h.resolver.queries)
					}
					return
				}
				if err != nil && strings.Contains(err.Error(), "is not an AS number") {
					t.Fatalf("%s %q refused a valid AS number: %v", command, tc.args, err)
				}
				if command == cmdClearIRRAsn {
					cleared, ok := data.(clearAnswer)
					if status != statusDone || !ok || cleared.ASN != tc.number {
						t.Fatalf("%s %q answered %q %+v %v, want the clear of AS %d", command, tc.args, status, data, err, tc.number)
					}
					if len(h.resolver.asked) != 0 {
						t.Fatalf("%s %q asked PeeringDB %v, want nothing", command, tc.args, h.resolver.asked)
					}
					return
				}
				if !slices.Equal(h.resolver.asked, []uint32{tc.number}) {
					t.Fatalf("%s %q asked PeeringDB %v, want exactly [%d]", command, tc.args, h.resolver.asked, tc.number)
				}
			})
		}
	}
}

// VALIDATES: the irr forwarders hand the plugin the value the dispatcher bound.
// matchCommandTokens binds a value whose YANG leaf is spelled like the
// command's last keyword as a typed selector and hands the forwarder empty
// args (internal/component/plugin/server/command.go), so each forwarder reads
// the value back by the leaf name. (Replaces filter_irr
// TestUpdateIRRForwardersRecoverTheBoundValue, whose commands moved here.)
// METHOD: the leaf names the forwarders read must be the ones ze-irr-cmd.yang
// declares, and a selector bound under each must come back as the one
// argument the owner's handler reads.
// PREVENTS: the owner answering `update irr asn 65001` with its usage line.
func TestUpdateIRRForwardersRecoverTheBoundValue(t *testing.T) {
	for _, leaf := range []string{leafASN, leafASSet} {
		if !strings.Contains(irryang.ZeIrrCmdYANG, "leaf "+leaf+" {") {
			t.Errorf("ze-irr-cmd.yang declares no leaf %q", leaf)
		}
	}
	tests := []struct {
		name      string
		selectors map[string]string
		leaf      string
		want      string
	}{
		{"asn", map[string]string{"asn": "65001"}, leafASN, "65001"},
		{"as-set", map[string]string{"as-set": "AS-TEST"}, leafASSet, "AS-TEST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pluginserver.CommandContext{Selectors: tt.selectors}
			got := ctx.ArgsOrSelector(nil, tt.leaf)
			if len(got) != 1 || got[0] != tt.want {
				t.Fatalf("args = %v, want [%s]", got, tt.want)
			}
		})
	}
}
