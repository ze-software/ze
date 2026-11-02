package rib

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRIBAsksForkedGateInBatches proves bgp-rib asks a receive gate owned by
// another process once per selection, replay or refresh, never once per path.
// Method: three peers announce one prefix and the first peer two more; the
// gate is then announced with a command whose test dispatcher answers from
// canned verdicts (only 203.0.113.0/24 from 192.0.2.1 is eligible) and counts
// commands. One selection (gatherPrefixCandidatesLocked), one best-path replay
// (collectBestPaths) and one Adj-RIB-Out refresh (collectRibOutRoutes) must
// each send one command and keep only the eligible path.
func TestRIBAsksForkedGateInBatches(t *testing.T) {
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	ribevents.RegisterValidationLookup(nil, nil)
	r := newTestRIBManagerWithBus(newTestEventBus())

	eligiblePeer := netip.MustParseAddr("192.0.2.1")
	eligiblePrefix := netip.MustParsePrefix("203.0.113.0/24")
	receive := func(peer string, msgID uint64, nlri ...byte) {
		attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 9}
		body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
		body = append(body, nlri...)
		wu := wireu.NewWireUpdate(body, bgpctx.APIContextID)
		wire, err := wu.Attrs()
		if err != nil {
			t.Fatal(err)
		}
		r.handleReceivedStructured(&rpc.StructuredEvent{
			EventType: rpc.EventKindUpdate, PeerAddress: peer, PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000,
			MessageID:  msgID,
			RawMessage: &bgptypes.RawMessage{MessageID: msgID, RawBytes: body, WireUpdate: wu, AttrsWire: wire},
		})
	}
	receive("192.0.2.1", 10, 24, 203, 0, 113, 24, 198, 51, 100, 24, 192, 0, 2)
	receive("192.0.2.2", 11, 24, 203, 0, 113)
	receive("192.0.2.3", 12, 24, 203, 0, 113)

	const command = "request test gate eligibility"
	calls := 0
	release := ribevents.RegisterGateDispatch(func(_ context.Context, _ string, args ...string) (string, json.RawMessage, error) {
		calls++
		answer, err := ribevents.AnswerEligibility(args[0], func(key ribevents.ValidationRoute, _ uint64) bool {
			return key.Peer == eligiblePeer && key.Prefix == eligiblePrefix
		}, func(ribevents.ValidationRoute) bool { return true })
		if err != nil {
			return "", nil, err
		}
		data, err := json.Marshal(answer)
		return "done", data, err
	})
	t.Cleanup(release)
	ribevents.NoteValidationGate(ribevents.ValidationGateState{Enabled: true, Command: command})
	t.Cleanup(func() { ribevents.NoteValidationGate(ribevents.ValidationGateState{Enabled: false}) })

	r.peerMu.RLock()
	candidates := r.gatherPrefixCandidatesLocked(family.IPv4Unicast, eligiblePrefix)
	r.peerMu.RUnlock()
	releaseCandidates(candidates)
	if len(candidates) != 1 || candidates[0].PeerAddr != eligiblePeer.String() {
		t.Errorf("selection candidates = %d, want only the path from %s", len(candidates), eligiblePeer)
	}
	if calls != 1 {
		t.Errorf("selection asked the gate %d times, want 1", calls)
	}

	calls = 0
	for _, entry := range r.collectBestPaths()[family.IPv4Unicast] {
		if entry.Prefix != eligiblePrefix {
			t.Errorf("best-path replay carried ineligible %s", entry.Prefix)
		}
	}
	if calls != 1 {
		t.Errorf("best-path replay asked the gate %d times, want 1", calls)
	}

	calls = 0
	target := netip.MustParseAddr("192.0.2.200")
	r.ribOut[target] = testRibOutFamilyMap(map[family.Family]map[string]*Route{family.IPv4Unicast: {
		"203.0.113.0/24":  {SourcePeer: "192.0.2.1"},
		"198.51.100.0/24": {SourcePeer: "192.0.2.1"},
		"192.0.2.0/24":    {SourcePeer: "192.0.2.1"},
	}})
	r.peerMu.RLock()
	routes := r.collectRibOutRoutes(target, family.IPv4Unicast)
	r.peerMu.RUnlock()
	if len(routes) != 1 {
		t.Errorf("Adj-RIB-Out refresh carried %d routes, want only the eligible one", len(routes))
	}
	if calls != 1 {
		t.Errorf("Adj-RIB-Out refresh asked the gate %d times, want 1", calls)
	}
}
