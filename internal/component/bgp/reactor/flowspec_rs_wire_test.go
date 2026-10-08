// Design: docs/architecture/bgp/structural-forwarding.md -- mandatory FlowSpec authorization.
package reactor

import (
	"bytes"
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// RFC 8955 Section 4: "When advertising Flow Specifications, the Length of the
// Next-Hop Network Address MUST be set to 0. The Network Address of the Next-Hop
// field MUST be ignored."
// RFC requirement: RFC8955-4-3 positive -- real route-server ingress, covering-route authorization and recipient TCP deliver all four FlowSpec families with zero next-hop length and unchanged rule/RD/actions.
// RFC requirement: RFC8955-4-3 negative -- self/explicit rewrites and received IPv4/IPv6 next hops cannot alter or withhold an authorized FlowSpec rule.
// RFC requirement: RFC5575-4-8 positive -- the running route server advertises the exact native rule and rate action with an empty next hop.
// RFC requirement: RFC5575-4-8 negative -- received forwarding addresses are ignored even when they name the recipient, rather than causing a loop-gate withdrawal.
func TestFlowSpecRouteServerOmitsNextHop(t *testing.T) {
	for _, mode := range []string{"unchanged", "self", "explicit4", "explicit6"} {
		for _, received := range []string{"empty", "peer-address", "ipv6"} {
			t.Run(mode+"/"+received, func(t *testing.T) {
				peers := flowForwardLiveRouter(t, mode)
				// These covering routes enter the real RIB from the same originator
				// and AS as the rules. VPN covers retain the same route distinguisher.
				for _, cover := range []struct {
					fam       family.Family
					nlri, hop string
				}{
					{family.IPv4Unicast, "18c00002", "c0000201"},
					{family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, "700001010000ffff00010000c00002", "0000000000000000c0000201"},
					{family.IPv6Unicast, "2020010db8", "20010db8000000000000000000000001"},
					{family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}, "780001010000ffff0001000020010db8", "0000000000000000" + "20010db8000000000000000000000001"},
				} {
					body := flowForwardPayload(t, cover.fam, mustHex(t, cover.nlri), mustHex(t, cover.hop))
					if cover.fam == family.IPv4Unicast {
						body = mustHex(t, "000000144001010040020602010000fdea400304c000020118c00002")
					}
					peers[0].send(t, flowForwardFrame(body))
				}
				for _, tc := range flowForwardWireCases {
					t.Run(tc.name, func(t *testing.T) {
						raw := mustHex(t, tc.nlri)
						var hop []byte
						switch received {
						case "peer-address":
							hop = mustHex(t, "c0000202")
						case "ipv6":
							hop = mustHex(t, "20010db80000000000000000000000dead")
						}
						peers[0].send(t, flowForwardFrame(flowForwardPayload(t, tc.fam, raw, hop)))
						lowEventually(t, func() bool { return flowForwardReceived(peers[1], tc.fam) != nil }, "authorized FlowSpec recipient announcement")
						assertFlowForwardWire(t, flowForwardReceived(peers[1], tc.fam), tc.fam, raw)
					})
				}
			})
		}
	}
}

func flowForwardFrame(body []byte) []byte {
	frame := bytes.Repeat([]byte{0xff}, message.HeaderLen)
	n := message.HeaderLen + len(body)
	frame[16], frame[17], frame[18] = byte(n>>8), byte(n), 2
	return append(frame, body...)
}

func flowForwardReceived(peer *lowLivePeer, fam family.Family) []byte {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	for _, frame := range peer.frames {
		if frame[18] != 2 {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		if err != nil {
			continue
		}
		_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
		if found && len(mp) > 5 && mp[0] == byte(fam.AFI>>8) && mp[1] == byte(fam.AFI) && mp[2] == byte(fam.SAFI) {
			return bytes.Clone(frame[message.HeaderLen:])
		}
	}
	return nil
}

// Use the daemon's plugin-server ownership, registered RIB and route server,
// actual TCP and negotiated families. No test callback supplies authorization
// or forwards the received message on the route server's behalf.
func flowForwardLiveRouter(t *testing.T, mode string) []*lowLivePeer {
	t.Helper()
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	source := lowLiveSettings("192.0.2.1", 65000, 65002)
	dest := lowLiveSettings("192.0.2.2", 65000, 65003)
	source.RSClient, dest.RSClient = true, true
	flowForwardNextHopSettings(dest, mode)
	// The connected TCP endpoint supplies self; a synthetic address would
	// instead ask reactor startup to bind a nonlocal listener.
	dest.LocalAddress = netip.Addr{}
	settings := []*PeerSettings{source, dest}
	for _, s := range settings {
		for _, afi := range []capability.AFI{1, 2} {
			for _, safi := range []capability.SAFI{1, 128, 133, 134} {
				if afi != 1 || safi != 1 {
					s.Capabilities = append(s.Capabilities, &capability.Multiprotocol{AFI: afi, SAFI: safi})
				}
			}
		}
		for _, binding := range []struct{ name, receive string }{
			{"bgp-rib", "update state refresh"},
			{"bgp-rs", "update-received state open-received refresh"},
			{"bgp-adj-rib-in", "update-received state"},
		} {
			if err := EnsureProcessBinding(s, binding.name, binding.receive, "update"); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.AddPeer(s); err != nil {
			t.Fatal(err)
		}
	}
	srv := flowForwardPluginServer(t, r)
	r.SetPluginServer(srv)
	if err := r.StartWithContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	var peers []*lowLivePeer
	for _, s := range settings {
		peer := r.peers[s.PeerKey()]
		lowEventually(t, func() bool { return peer.currentSession() != nil && peer.SessionState() == fsm.StateActive }, "FlowSpec peer active")
		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
		remote, err := (&net.Dialer{}).DialContext(t.Context(), "tcp4", listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		server, err := listener.Accept()
		if err != nil {
			_ = remote.Close()
			t.Fatal(err)
		}
		_ = listener.Close()
		capture := &lowLivePeer{peer: peer, remote: remote, stopped: make(chan struct{})}
		go capture.readFrames()
		t.Cleanup(func() {
			_ = remote.Close()
			_ = server.Close()
			<-capture.stopped
		})
		if err := peer.acceptConnection(server); err != nil {
			t.Fatal(err)
		}
		params := []byte{2, 6, 65, 4, 0, 0, byte(s.PeerAS >> 8), byte(s.PeerAS)}
		for _, afi := range []byte{1, 2} {
			for _, safi := range []byte{1, 128, 133, 134} {
				params = append(params, 2, 6, 1, 4, 0, afi, 0, safi)
			}
		}
		open := &message.Open{Version: 4, MyAS: uint16(s.PeerAS), HoldTime: 90, BGPIdentifier: 0x0a000001 + uint32(len(peers)), OptionalParams: params}
		capture.send(t, message.PackTo(open, nil))
		lowEventually(t, func() bool { return peer.SessionState() == fsm.StateOpenConfirm }, "FlowSpec OPEN accepted")
		capture.send(t, message.PackTo(message.NewKeepalive(), nil))
		lowEventually(t, func() bool { return peer.State() == PeerStateEstablished && !peer.pendingSync() }, "FlowSpec initial sync")
		peers = append(peers, capture)
	}
	return peers
}

// The hub installs its server as the shared EventBus before spawning engines.
// Without that injection, authorization can race the first forward and no
// ValidationChange subscriber can replay the newly authorized retained rule.
func flowForwardPluginServer(t *testing.T, r *Reactor, extra ...plugin.PluginConfig) *pluginserver.Server {
	t.Helper()
	names := []string{"bgp-rib", "bgp-adj-rib-in", "bgp-rs"}
	var configs []plugin.PluginConfig
	for _, name := range names {
		configs = append(configs, plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"})
	}
	configs = append(configs, extra...)
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: configs}, &reactorAPIAdapter{r: r})
	if err != nil {
		t.Fatal(err)
	}
	originalBus := registry.GetEventBus()
	registry.SetEventBus(srv)
	t.Cleanup(func() { registry.SetEventBus(originalBus) })
	mgr := pluginmgr.NewManager()
	if err := mgr.StartAll(context.Background(), srv, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mgr.StopAll(context.Background()); err != nil {
			t.Error(err)
		}
	})
	srv.SetProcessSpawner(mgr)
	t.Cleanup(func() {
		srv.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Wait(ctx); err != nil {
			t.Error(err)
		}
	})
	if err := srv.StartWithContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := srv.WaitForStartupComplete(ctx); err != nil {
		t.Fatal(err)
	}
	return srv
}
