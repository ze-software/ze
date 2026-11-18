//go:build ze_bgp

// Design: docs/functional-tests.md -- reload assertions cover the drained scenario.
// RFC naming: untagged -- tests synthetic observer phase accounting; test/reload/rfc8212-import-owner-failure.ci checks real reload policy and wire behavior.
package fixture

import (
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC8212ReloadDownWindow consumes native state events. The startup removal
// belongs to an earlier Peer incarnation; a DOWN inside the scenario still fails.
func TestRFC8212ReloadDownWindow(t *testing.T) {
	for _, during := range []bool{false, true} {
		name := "startup removal retained"
		if during {
			name = "scenario down refused"
		}
		t.Run(name, func(t *testing.T) {
			wire := newRFC8212ReloadWire()
			old := plugin.PeerInfo{Address: netip.MustParseAddr(addrLoopbackSecond), PeerOwner: 3, SessionGeneration: 1}
			startup := string(format.AppendStateChange(nil, &old, rpc.SessionStateDown, rpc.ReasonPeerRemoved, nil, "json"))
			if err := wire.receive(startup); err != nil {
				t.Fatal(err)
			}
			current := plugin.PeerInfo{Address: old.Address, PeerOwner: 7, DestinationSession: 26}
			if err := wire.receive(string(format.AppendStateChange(nil, &current, rpc.SessionStateUp, "", nil, "json"))); err != nil {
				t.Fatal(err)
			}
			start := wire.markDiagnosticPhase("scenario-start-after-event-barrier")
			if during {
				current.SessionGeneration = 1
				if err := wire.receive(string(format.AppendStateChange(nil, &current, rpc.SessionStateDown, rpc.ReasonTCPFailure, nil, "json"))); err != nil {
					t.Fatal(err)
				}
			}
			wire.markDiagnosticPhase("scenario-returned")
			end := wire.markDiagnosticPhase("scenario-complete-after-event-barrier")
			err := wire.checkDiagnosticDownWindow(start, end)
			if during && (err == nil || !strings.Contains(err.Error(), "1 DOWN events between drained scenario boundaries")) {
				t.Fatalf("scenario DOWN did not fail its phase: %v", err)
			}
			if !during && err != nil {
				t.Fatalf("startup removal failed the later scenario: %v", err)
			}
			wire.mu.Lock()
			defer wire.mu.Unlock()
			if start != 1 || wire.downs != end || !strings.Contains(strings.Join(wire.diagnosticTranscript, "\n"), strconv.Quote(startup)) {
				t.Fatalf("startup evidence was discarded: start=%d end=%d absolute=%d transcript=%q", start, end, wire.downs, wire.diagnosticTranscript)
			}
		})
	}
}
