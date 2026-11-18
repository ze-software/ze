// Design: docs/architecture/plugin/rib-storage-design.md -- ordinary selected peer-up replay.
// Related: flowspec_rs_wire_test.go -- independent wire fixtures and action assertions.
package reactor

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// RFC naming: untagged -- useful FlowSpec survival when auxiliary replay stands down.
func TestOrdinarySelectedFlowSpecReplaysToReplacementRecipient(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	settings := []*PeerSettings{
		acceptingLiveSettings(t, "192.0.2.241", 65000, 65002),
		acceptingLiveSettings(t, "192.0.2.242", 65000, 65000),
	}
	for _, setting := range settings {
		setting.NextHopMode = NextHopAuto
		setting.Capabilities = append(setting.Capabilities, &capability.Multiprotocol{AFI: 1, SAFI: 133})
	}
	peers := lowLiveStart(t, lowLiveOptions{withoutRS: true}, settings...)
	loc := locrib.Default()
	if loc == nil {
		t.Fatal("ordinary FlowSpec fixture has no daemon Routing Table")
	}
	protocol := redistevents.RegisterProtocol("ordinary-flowspec-replay-test")
	host := netip.PrefixFrom(settings[0].Address, 32)
	loc.Insert(family.IPv4Unicast, host, locrib.Path{Source: protocol, Metric: 7})
	t.Cleanup(func() { loc.Remove(family.IPv4Unicast, host, protocol, 0) })
	source := ordinaryFlowSpecConnect(t, peers[0], 0x0a00f101)
	cover := netip.MustParsePrefix("198.19.246.0/24")
	ownershipLiveSend(t, source, nil, synthesizedNextHopAttrs(settings[0].Address, 65002), cover)
	ordinarySelectedStored(t, source, cover)
	lowEventually(t, func() bool {
		_, found := loc.Best(family.IPv4Unicast, cover)
		return found
	}, "ordinary FlowSpec covering route selected")
	raw := []byte{5, 1, 24, 198, 19, 246}
	if source.peer.sourceOwner == 0 {
		t.Fatal("ordinary FlowSpec source has no received-path owner")
	}
	key := ribevents.ValidationRoute{Peer: settings[0].Address, PeerOwner: source.peer.sourceOwner, Family: fam, NLRI: string(raw)}
	source.send(t, flowForwardFrame(flowForwardPayload(t, fam, raw, nil)))
	lowEventually(t, func() bool { return ribevents.RouteEligible(key, 0) }, "ordinary retained FlowSpec rule authorized")
	updates := source.peer.Stats().UpdatesReceived
	late := ordinaryFlowSpecConnect(t, peers[1], 0x0a00f102)
	oldSession := peers[1].currentSession()
	oldToken, owner := peers[1].selectedStateDestination()
	if oldToken == 0 {
		t.Fatal("late recipient has no captured destination Session")
	}
	if owner == "" {
		t.Fatal("late recipient has no ordinary selected owner")
	}
	ordinaryFlowSpecAssertReplay(t, source, late, fam, raw)
	lowLiveDepart(t, late)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	select {
	case <-late.stopped:
	case <-ctx.Done():
		t.Fatal("departed FlowSpec recipient capture did not stop")
	}
	lowEventually(t, func() bool {
		current := peers[1].currentSession()
		if current == nil {
			return false
		}
		if current == oldSession {
			return false
		}
		return peers[1].SessionState() == fsm.StateActive
	}, "fresh FlowSpec recipient Session")
	replacement := ordinaryFlowSpecConnect(t, peers[1], 0x0a00f103)
	token, replacementOwner := peers[1].selectedStateDestination()
	if token == 0 {
		t.Fatal("replacement FlowSpec recipient has no captured Session")
	}
	if token == oldToken {
		t.Fatal("replacement FlowSpec recipient reused the old token")
	}
	if replacementOwner != owner {
		t.Fatal("replacement FlowSpec recipient changed ordinary owner")
	}
	ordinaryFlowSpecAssertReplay(t, source, replacement, fam, raw)
	if got := source.peer.Stats().UpdatesReceived; got != updates {
		t.Errorf("FlowSpec replay received %d source UPDATEs, started at %d", got, updates)
	}
}

func ordinaryFlowSpecAssertReplay(t *testing.T, source, recipient *lowLivePeer, fam family.Family, raw []byte) {
	t.Helper()
	lowEventually(t, func() bool { return len(flowForwardFrames(recipient, fam)) > 0 }, "selected FlowSpec peer-up replay")
	forwardabilitySettle(t, source, recipient)
	for _, body := range flowForwardFrames(recipient, fam) {
		assertFlowForwardWire(t, body, fam, raw)
	}
}

// Admit the actual transport after capture starts and negotiate both families.
func ordinaryFlowSpecConnect(t *testing.T, peer *Peer, identifier uint32) *lowLivePeer {
	t.Helper()
	lowEventually(t, func() bool {
		if peer.currentSession() == nil {
			return false
		}
		return peer.SessionState() == fsm.StateActive
	}, "ordinary FlowSpec peer Active")
	server, remote := net.Pipe()
	live := &lowLivePeer{peer: peer, remote: remote, stopped: make(chan struct{})}
	live.startCapture(t)
	t.Cleanup(func() {
		if err := remote.Close(); err != nil {
			t.Errorf("close FlowSpec remote: %v", err)
		}
		if err := server.Close(); err != nil {
			t.Errorf("close FlowSpec server: %v", err)
		}
		<-live.stopped
	})
	if err := peer.acceptConnection(server); err != nil {
		t.Fatal(err)
	}
	settings := peer.Settings()
	params := []byte{2, 6, 65, 4, 0, 0, byte(settings.PeerAS >> 8), byte(settings.PeerAS),
		2, 6, 1, 4, 0, 1, 0, 1, 2, 6, 1, 4, 0, 1, 0, 133}
	open := &message.Open{Version: 4, MyAS: uint16(settings.PeerAS), HoldTime: 90,
		BGPIdentifier: identifier, OptionalParams: params}
	live.send(t, message.PackTo(open, nil))
	lowEventually(t, func() bool { return peer.SessionState() == fsm.StateOpenConfirm }, "ordinary FlowSpec OPEN accepted")
	live.send(t, message.PackTo(message.NewKeepalive(), nil))
	lowEventually(t, func() bool {
		if peer.State() != PeerStateEstablished {
			return false
		}
		return !peer.pendingSync()
	}, "ordinary FlowSpec initial replay settled")
	return live
}
