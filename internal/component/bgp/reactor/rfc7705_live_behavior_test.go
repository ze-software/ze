// Design: docs/architecture/plugin/rib-storage-design.md -- received routes and wire consumers.
// Related: rfc9687_rib_release_test.go -- running reactor and RIB plugin fixture.
package reactor

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/adj_rib_in"
	bgprib "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rs"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// lowLivePeer owns one remote connection and its bounded wire capture. Cleanup
// MUST close the connection and join the reader before releasing the fixture.
type lowLivePeer struct {
	peer    *Peer
	remote  net.Conn
	mu      sync.Mutex
	frames  [][]byte
	stopped chan struct{}
}

func lowLiveSettings(address string, local, remote uint32) *PeerSettings {
	s := NewPeerSettings(netip.MustParseAddr(address), local, remote, 0x0a0000fe)
	s.Connection = ConnectionPassive
	s.ReceiveHoldTime = 90 * time.Second
	s.NextHopMode = NextHopUnchanged
	s.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: local},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}
	return s
}

// lowLiveRouter uses the same server ownership and registered plugins as the
// daemon. Routes enter only over the remote socket, never through a RIB setter.
func lowLiveRouter(t *testing.T, settings ...*PeerSettings) (*Reactor, []*lowLivePeer) {
	t.Helper()
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	for _, s := range settings {
		if err := EnsureProcessBinding(s, "bgp-rib", "update state refresh", "update"); err != nil {
			t.Fatal(err)
		}
		if err := EnsureProcessBinding(s, "bgp-rs", "update-received state open-received refresh", "update"); err != nil {
			t.Fatal(err)
		}
		if err := EnsureProcessBinding(s, "bgp-adj-rib-in", "update-received state", "update"); err != nil {
			t.Fatal(err)
		}
		if err := r.AddPeer(s); err != nil {
			t.Fatal(err)
		}
	}
	srv := newBorrowedPluginServer(t, r, "bgp-rib", "bgp-adj-rib-in", "bgp-rs")
	r.SetPluginServer(srv)
	if err := r.StartWithContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	out := make([]*lowLivePeer, 0, len(settings))
	for _, s := range settings {
		p := r.peers[s.PeerKey()]
		lowEventually(t, func() bool { return p.currentSession() != nil && p.SessionState() == fsm.StateActive }, "peer active")
		server, remote := net.Pipe()
		v := &lowLivePeer{peer: p, remote: remote, stopped: make(chan struct{})}
		go v.readFrames()
		t.Cleanup(func() {
			// MUST close before joining: the reader may be waiting for a header.
			_ = remote.Close()
			_ = server.Close()
			<-v.stopped
		})
		if err := p.acceptConnection(server); err != nil {
			t.Fatal(err)
		}
		open := &message.Open{Version: 4, MyAS: uint16(s.PeerAS), HoldTime: 90, BGPIdentifier: 0x0a000001 + uint32(len(out)),
			OptionalParams: []byte{2, 6, 65, 4, 0, 0, byte(s.PeerAS >> 8), byte(s.PeerAS), 2, 6, 1, 4, 0, 1, 0, 1}}
		v.send(t, message.PackTo(open, nil))
		lowEventually(t, func() bool { return p.SessionState() == fsm.StateOpenConfirm }, "OPEN accepted")
		v.send(t, message.PackTo(message.NewKeepalive(), nil))
		lowEventually(t, func() bool { return p.State() == PeerStateEstablished && !p.pendingSync() }, "peer initial sync")
		out = append(out, v)
	}
	return r, out
}

// readFrames runs once per remote peer. Its owner MUST close remote and join
// stopped; at most 64 frames are retained for these bounded scenarios.
func (p *lowLivePeer) readFrames() {
	defer close(p.stopped)
	for {
		frame, err := core4271ReadMessage(p.remote)
		if err != nil {
			return
		}
		p.mu.Lock()
		if len(p.frames) < 64 {
			p.frames = append(p.frames, frame)
		}
		p.mu.Unlock()
	}
}

func (p *lowLivePeer) send(t *testing.T, frame []byte) {
	t.Helper()
	if err := p.remote.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.remote.Write(frame); err != nil {
		t.Fatal(err)
	}
}

func lowEventually(t *testing.T, ready func() bool, what string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for !ready() {
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func (p *lowLivePeer) announcement(prefix []byte) []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, frame := range p.frames {
		if frame[18] != 2 {
			continue
		}
		u, err := message.UnpackUpdate(frame[19:])
		if err != nil {
			continue
		}
		if bytes.Equal(u.NLRI, prefix) {
			return bytes.Clone(u.PathAttributes)
		}
	}
	return nil
}

func lowInstalledAttributes(peer string, prefix []byte) []byte {
	var attrs []byte
	bgprib.RIBDumpBridge.DumpRIB(registry.RIBDumpVisitor{
		OnPeer: func(address string, _ uint32, _ [4]byte, _ bool) uint16 {
			if address == peer {
				return 1
			}
			return 0
		},
		OnRoute: func(index, afi, safi uint16, bits uint8, nlri, attributes []byte) {
			if index == 1 && afi == 1 && safi == 1 && bits == prefix[0] && bytes.Equal(nlri, prefix[1:]) {
				attrs = bytes.Clone(attributes)
			}
		},
	})
	return attrs
}

func lowAssertAttribute(t *testing.T, attrs []byte, code attribute.AttributeCode, want []byte) {
	t.Helper()
	_, _, value, found := attribute.AttrFind(attrs, code)
	if !found || !bytes.Equal(value, want) {
		t.Fatalf("attribute %d = %x present=%v, want %x", code, value, found, want)
	}
}

// TestRFC7705NoPrependInstalledAndAdvertised inspects the running RIB and the
// internal neighbor after real ingress, including a path that already contains
// the legacy ASN: no-prepend must not strip a received occurrence either.
// RFC 7705 Section 3.3: "it MUST NOT append the \"Local AS\" ASN value in the AS_PATH attribute when installing the route or advertising that UPDATE to iBGP neighbors".
// RFC requirement: RFC7705-3.3-2 positive -- a no-prepend external source's exact AS_PATH is installed in the live RIB and sent to an internal neighbor without adding the legacy ASN.
// RFC requirement: RFC7705-3.3-2 negative -- an existing received legacy ASN remains exactly once in the installed and transmitted path; no-prepend neither inserts a duplicate nor strips the peer's path.
func TestRFC7705NoPrependInstalledAndAdvertised(t *testing.T) {
	source := lowLiveSettings("192.0.2.1", 65010, 65002)
	source.GlobalLocalAS = 65000
	source.LocalASNoPrepend = true
	source.LoopAllowOwnAS = 1
	dest := lowLiveSettings("192.0.2.2", 65000, 65000)
	_, peers := lowLiveRouter(t, source, dest)
	for i, path := range [][]byte{{2, 1, 0, 0, 0xfd, 0xea}, {2, 2, 0, 0, 0xfd, 0xea, 0, 0, 0xfd, 0xf2}} {
		prefix := []byte{24, 203, 0, byte(113 + i)}
		attrs := []byte{0x40, 1, 1, 0, 0x40, 2, byte(len(path))}
		attrs = append(attrs, path...)
		attrs = append(attrs, 0x40, 3, 4, 192, 0, 2, 1)
		peers[0].send(t, message.PackTo(&message.Update{PathAttributes: attrs, NLRI: prefix}, nil))
		lowEventually(t, func() bool { return lowInstalledAttributes("192.0.2.1", prefix) != nil }, "installed received route")
		lowAssertAttribute(t, lowInstalledAttributes("192.0.2.1", prefix), attribute.AttrASPath, path)
		lowEventually(t, func() bool { return peers[1].announcement(prefix) != nil }, "internal advertisement")
		lowAssertAttribute(t, peers[1].announcement(prefix), attribute.AttrASPath, path)
	}
}

// TestRFC7705MigrationWireSemantics compares native iBGP with both migration
// identities. The received route has a third-party first AS and LOCAL_PREF 231,
// so first-AS eBGP checks or external attribute discard cannot pass silently.
// RFC 7705 Section 4.2: "In each case, the BGP speaker MUST treat UPDATEs sent and received to this peer as if this was a natively configured iBGP session, as defined by [RFC4271] and [RFC4456]."
// RFC requirement: RFC7705-4.2-4 positive -- native and migrating sessions accept and install third-party AS_PATH and LOCAL_PREF unchanged; reflection sends those attributes with ORIGINATOR_ID and CLUSTER_LIST and no eBGP prepend.
// RFC requirement: RFC7705-4.2-4 negative -- both migration identities obey iBGP split horizon: a route from a non-client is not reflected to another non-client, while a reflector client receives the route with RFC4456 attributes.
func TestRFC7705MigrationWireSemantics(t *testing.T) {
	for _, tc := range []struct {
		name              string
		remote, migration uint32
	}{
		{"native", 65000, 0}, {"migration-retained", 65000, 65010}, {"migration-legacy", 65010, 65010},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := lowLiveSettings("192.0.2.1", 65000, tc.remote)
			source.MigrationAS = tc.migration
			dest := lowLiveSettings("192.0.2.2", 65000, tc.remote)
			dest.MigrationAS = tc.migration
			dest.RouteReflectorClient = true
			_, peers := lowLiveRouter(t, source, dest)
			path := []byte{2, 1, 0, 0, 0xfc, 0x00}
			attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6}
			attrs = append(attrs, path...)
			attrs = append(attrs, 0x40, 3, 4, 198, 51, 100, 1, 0x40, 5, 4, 0, 0, 0, 231)
			prefix := []byte{24, 203, 0, 113}
			peers[0].send(t, message.PackTo(&message.Update{PathAttributes: attrs, NLRI: prefix}, nil))
			lowEventually(t, func() bool { return lowInstalledAttributes("192.0.2.1", prefix) != nil }, "migration receive installation")
			lowAssertAttribute(t, lowInstalledAttributes("192.0.2.1", prefix), attribute.AttrASPath, path)
			lowAssertAttribute(t, lowInstalledAttributes("192.0.2.1", prefix), attribute.AttrLocalPref, []byte{0, 0, 0, 231})
			lowEventually(t, func() bool { return peers[1].announcement(prefix) != nil }, "migration reflected UPDATE")
			got := peers[1].announcement(prefix)
			lowAssertAttribute(t, got, attribute.AttrASPath, path)
			lowAssertAttribute(t, got, attribute.AttrLocalPref, []byte{0, 0, 0, 231})
			lowAssertAttribute(t, got, attribute.AttrOriginatorID, []byte{10, 0, 0, 1})
			cluster := make([]byte, 4)
			binary.BigEndian.PutUint32(cluster, dest.RouterID)
			lowAssertAttribute(t, got, attribute.AttrClusterList, cluster)
			// RFC 7705 Section 4.2 / RFC 4456 Section 9: the same route
			// must not cross the non-client-to-non-client boundary.
			f := newAIGPReplayFixture(t, nil)
			for _, p := range []*Peer{f.source, f.destination} {
				p.settings.LocalAS, p.settings.PeerAS = 65000, tc.remote
				p.settings.MigrationAS = tc.migration
				p.settings.NextHopMode = NextHopUnchanged
				p.refreshForwardFacts()
			}
			id := f.receive(t, buildUpdatePayload(attrs, prefix))
			update, present := f.r.recentUpdates.Get(id)
			if !present {
				t.Fatal("received route not cached")
			}
			if err := (&reactorAPIAdapter{r: f.r}).forwardUpdateCore(
				update, id, []*Peer{f.destination},
				forwardSourceInfo{resolved: true, isIBGP: true, globalLocalAS: 65000},
			); err == nil {
				t.Fatal("non-client internal route unexpectedly forwarded")
			}
			f.drain(t)
			if len(f.conn.written()) != 0 {
				t.Fatal("split horizon leaked an UPDATE")
			}
		})
	}
}
