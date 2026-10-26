// Design: docs/architecture/bgp/structural-forwarding.md -- reactor-native RS forwarding
// Related: forward_rs.go -- reactorForwardRSSection destination selection
// Related: peer_forward_facts.go -- rsRelaySendGranted, the fast path's send permission
// Related: send_permission.go -- filterPermittedPeers, the cached rail's send permission
//
// VALIDATES: the route-server fast path forwards only to a destination whose
// attach block grants the route server `send [ update ]`, on the first pass and
// on the per-field retry.
// PREVENTS: a destination the cached rail refuses receiving the source's UPDATE
// on rs-fast-path.
package reactor

import (
	"bytes"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"

	"github.com/stretchr/testify/require"
)

// rsFixtureRelay is the process name of a route server in this package's
// fixtures. It is registered with RelaysOnRSFastPath, as bgp-rs is, because
// the reactor package cannot link bgp-rs and the fast path reaches only a
// peer granting `send [ update ]` to such a process.
const rsFixtureRelay = "rs-fixture-relay"

func init() {
	err := registry.Register(registry.Registration{
		Name:               rsFixtureRelay,
		Description:        "route-server fixture for the reactor fast path",
		RunEngine:          func(net.Conn) int { return 0 },
		CLIHandler:         func([]string) int { return 0 },
		RelaysOnRSFastPath: true,
	})
	if err != nil {
		panic("BUG: register " + rsFixtureRelay + ": " + err.Error())
	}
}

// rsRelaySendBinding attaches the fixture route server with `send [ update ]`,
// the grant a fast-path destination needs.
func rsRelaySendBinding() ProcessBinding {
	return ProcessBinding{
		PluginName: rsFixtureRelay,
		Send:       map[string]bool{bgpevents.SendUpdate: true},
	}
}

// rsRelayReceiveBinding attaches the fixture route server with
// `receive [ update-received ]`, the grant a fast-path source needs: the route
// server's cached forward relays only the UPDATEs it receives.
func rsRelayReceiveBinding() ProcessBinding {
	return ProcessBinding{
		PluginName: rsFixtureRelay,
		Receive:    map[string]events.Direction{bgpevents.EventUpdate: events.DirReceived},
	}
}

// TestRSFastPathGateHonorsSourceRelayReceive proves the fast path relays only a
// source the route server's cached forward would relay. Method: drive the real
// gate in notifyMessageReceiver for one UPDATE from an rs-fast-path source to a
// granted destination, once per source attach block; ReactorForwarded is set
// only when the fast path dispatched. A source that feeds the route server none
// of its UPDATEs (no binding, the sent direction only, or the received
// direction to a process that is no route server) is relayed by nobody on the
// cached rail, because the delivery graph never hands bgp-rs the event; before
// the fix the fast path relayed it all the same
// (plan/journal/guard-added-to-one-half-of-a-pair.md, 2026-10-26).
func TestRSFastPathGateHonorsSourceRelayReceive(t *testing.T) {
	for _, tc := range []struct {
		name          string
		bindings      []ProcessBinding
		wantForwarded bool
	}{
		{"no_binding", nil, false},
		{"sent_direction_only", []ProcessBinding{{PluginName: rsFixtureRelay,
			Receive: map[string]events.Direction{bgpevents.EventUpdate: events.DirSent}}}, false},
		{"not_a_route_server", []ProcessBinding{{PluginName: "sequence",
			Receive: map[string]events.Direction{bgpevents.EventUpdate: events.DirReceived}}}, false},
		{"update_received", []ProcessBinding{rsRelayReceiveBinding()}, true},
		{"wildcard", []ProcessBinding{{PluginName: rsFixtureRelay, ReceiveAll: true}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reactor := New(&Config{ListenAddr: "127.0.0.1:0"})
			reactor.rsForwardingEnabled = true

			sourceAddr := mustParseAddr("10.0.0.1")
			settings := NewPeerSettings(sourceAddr, 65000, 65001, 0x01020304)
			settings.RSFastPath = true
			settings.ProcessBindings = tc.bindings
			require.NoError(t, reactor.AddPeer(settings))
			reactor.mu.RLock()
			source := reactor.peers[settings.PeerKey()]
			reactor.mu.RUnlock()
			source.setState(PeerStateEstablished)
			// An established session holds a forwarding snapshot
			// (setEncodingContexts); the gate reads the receive grant from it.
			source.refreshForwardFacts()

			targetCtx := bgpctx.EncodingContextForASN4(true)
			targetCtxID, err := bgpctx.Registry.Register(targetCtx)
			require.NoError(t, err)
			target := makeRSPeer(t, "10.0.0.2", 65002, targetCtx, targetCtxID)
			reactor.mu.Lock()
			reactor.peers[target.Settings().PeerKey()] = target
			reactor.mu.Unlock()

			var forwarded atomic.Bool
			delivered := make(chan struct{}, 1)
			reactor.setMessageReceiver(&testDeliveryReceiver{
				onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
					if msg.ReactorForwarded {
						forwarded.Store(true)
					}
					select {
					case delivered <- struct{}{}:
					default:
					}
				},
			})
			stop := startTestDelivery(t, reactor, sourceAddr, deliveryChannelCapacity)
			defer stop()

			payload := testUpdatePayload()
			wireUpdate := wireu.NewWireUpdate(payload, 0)
			_ = reactor.notifyMessageReceiver(sourceAddr, msgtype.TypeUPDATE, payload, wireUpdate, 0, rpc.DirectionReceived, testPoolBuf(t), nil, "", 0)

			select {
			case <-delivered:
			case <-time.After(5 * time.Second):
				t.Fatal("delivery callback did not run")
			}
			if forwarded.Load() != tc.wantForwarded {
				t.Fatalf("ReactorForwarded = %v, want %v for source bindings %+v",
					forwarded.Load(), tc.wantForwarded, tc.bindings)
			}
		})
	}
}

// TestReactorForwardRSHonorsRelaySendPermission proves the route-server fast
// path reaches only the destinations the route server's cached forward may
// reach. Method: three destinations, one granting the route server
// `send [ update ]`, one attaching the route server with no send list, one
// granting `send [ update ]` to a process that is no route server; the fast
// path forwards one UPDATE, and only the first destination may receive bytes.
// The peer without the grant is what the cached rail refuses
// (filterPermittedPeers); before the fix the fast path wrote to it all the
// same (plan/journal/guard-added-to-one-half-of-a-pair.md, 2026-10-26).
func TestReactorForwardRSHonorsRelaySendPermission(t *testing.T) {
	h := newFanoutHarnessWith(t, 3, 1, fanoutOpts{})
	r := h.adapter.r
	source := r.peers[netip.AddrPortFrom(h.update.SourcePeerIP, DefaultBGPPort)]
	if source == nil {
		t.Fatal("source fixture missing")
	}
	h.update.receivedPeer = source
	h.update.receivedGeneration = source.forwardGeneration.Load()

	bindings := [3][]ProcessBinding{
		{rsRelaySendBinding()},
		{{PluginName: rsFixtureRelay}},
		{{PluginName: "sequence", Send: map[string]bool{bgpevents.SendUpdate: true}}},
	}
	var output [3]bytes.Buffer
	var sessions [3]*Session
	for i, peer := range h.dests {
		peer.settings.ProcessBindings = bindings[i]
		peer.refreshForwardFacts()
		sessions[i] = rsSelectionWriter(t, peer, &output[i])
	}

	skipped, dispatched := reactorForwardRS(r, h.update, 1, h.update.SourcePeerIP, source)
	// The two refused destinations go back to the route server, whose cached
	// rail refuses and reports them (filterPermittedPeers,
	// sendPermissionDenied), exactly as it does with no fast path.
	wantSkipped := []netip.AddrPort{h.dests[1].Settings().PeerKey(), h.dests[2].Settings().PeerKey()}
	slices.SortFunc(skipped, netip.AddrPort.Compare)
	slices.SortFunc(wantSkipped, netip.AddrPort.Compare)
	if !slices.Equal(skipped, wantSkipped) {
		t.Fatalf("skipped %v; want %v, the destinations granting the route server no send [ update ]", skipped, wantSkipped)
	}
	if dispatched != 1 {
		t.Fatalf("dispatched %d; want 1, the one destination granting the route server send", dispatched)
	}
	for i, session := range sessions {
		if err := session.bufWriter.Flush(); err != nil {
			t.Fatal(err)
		}
		if i != 0 && output[i].Len() != 0 {
			t.Fatalf("destination %d grants the route server no send [ update ] and received %x", i, output[i].Bytes())
		}
	}
	if output[0].Len() <= message.HeaderLen {
		t.Fatal("the destination granting the route server send [ update ] received no UPDATE")
	}
}

// TestReactorForwardRSRetryRechecksRelaySend proves the per-field retry pass
// rereads the send permission rather than trusting the first pass's selection:
// a destination whose grant is revoked between the passes is sent nothing.
func TestReactorForwardRSRetryRechecksRelaySend(t *testing.T) {
	h := newFanoutHarnessWith(t, 2, 1, fanoutOpts{})
	r := h.adapter.r
	source := r.peers[netip.AddrPortFrom(h.update.SourcePeerIP, DefaultBGPPort)]
	if source == nil {
		t.Fatal("source fixture missing")
	}
	h.update.receivedPeer = source
	h.update.receivedGeneration = source.forwardGeneration.Load()

	var output [2]bytes.Buffer
	var sessions [2]*Session
	for i, peer := range h.dests {
		sessions[i] = rsSelectionWriter(t, peer, &output[i])
	}
	h.dests[1].settings.ProcessBindings = nil
	h.dests[1].refreshForwardFacts()

	_, dispatched, retry := reactorForwardRSSection(r, h.update, h.update.WireUpdate, h.dests, 1, h.update.SourcePeerIP, source)
	if dispatched != 1 || len(retry) != 0 {
		t.Fatalf("dispatched %d, retry %d; want 1, 0", dispatched, len(retry))
	}
	for i, session := range sessions {
		if err := session.bufWriter.Flush(); err != nil {
			t.Fatal(err)
		}
		if i == 1 && output[i].Len() != 0 {
			t.Fatalf("revoked destination received %x on the retry pass", output[i].Bytes())
		}
	}
	if output[0].Len() <= message.HeaderLen {
		t.Fatal("the granted destination received no UPDATE on the retry pass")
	}
}

// TestRSFastPathWarnsOnceForAMissingGrant proves an rs-fast-path peer that
// lacks a route-server grant is reported once, with the attach line that fixes
// it, and not again on a rebuild that changes nothing. Method: capture the
// forward logger, build the snapshot of a peer per attach block, then rebuild
// it the way an interface event does (refreshForwardFactsIfLive) and count the
// warnings. A peer without rs-fast-path, or with both grants, says nothing.
// PREVENTS: the fast path declining a source or passing over a destination with
// nothing in the log above debug, which made the grant rule invisible.
func TestRSFastPathWarnsOnceForAMissingGrant(t *testing.T) {
	both := ProcessBinding{
		PluginName: rsFixtureRelay,
		Receive:    map[string]events.Direction{bgpevents.EventUpdate: events.DirReceived},
		Send:       map[string]bool{bgpevents.SendUpdate: true},
	}
	for _, tc := range []struct {
		name       string
		fastPath   bool
		bindings   []ProcessBinding
		wantAction string
	}{
		// With no binding the fix names the first registered route server,
		// which in this test binary need not be the fixture.
		{"no_binding", true, nil, " { receive [ update-received ]; send [ update ]; }` to this peer"},
		{"receive_only", true, []ProcessBinding{rsRelayReceiveBinding()}, "add `attach process " + rsFixtureRelay + " { send [ update ]; }` to this peer"},
		{"send_only", true, []ProcessBinding{rsRelaySendBinding()}, "add `attach process " + rsFixtureRelay + " { receive [ update-received ]; }` to this peer"},
		{"both_grants", true, []ProcessBinding{both}, ""},
		{"no_fast_path", false, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sink bytes.Buffer
			prev := fwdLogger
			fwdLogger = func() *slog.Logger {
				return slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
			}
			t.Cleanup(func() { fwdLogger = prev })

			settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0x01020304)
			settings.RSFastPath = tc.fastPath
			settings.ProcessBindings = tc.bindings
			peer := NewPeer(settings)
			peer.refreshForwardFacts()
			peer.refreshForwardFactsIfLive()

			warnings := strings.Count(sink.String(), "level=WARN")
			if tc.wantAction == "" {
				if warnings != 0 {
					t.Fatalf("want no warning, got %q", sink.String())
				}
				return
			}
			if warnings != 1 {
				t.Fatalf("want one warning across two snapshots, got %d: %q", warnings, sink.String())
			}
			if !strings.Contains(sink.String(), tc.wantAction) {
				t.Fatalf("warning %q does not carry the fix %q", sink.String(), tc.wantAction)
			}
		})
	}
}
