// Design: docs/guide/bmp.md -- what a new BMP session carries before incremental monitoring.
// RFC: rfc/short/rfc7854.md

package bmp

import (
	"bytes"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RFC requirement: RFC7854-x-18 positive -- the Initiation the sender opens a
// session with carries a sysName TLV (type 2) with a non-empty value.
//
// VALIDATES: the sysName half of Section 4.3 on the octets a collector reads.
// Method: the collector end of a pipe decodes the Initiation sendInitiation writes.
func TestRFC7854InitiationCarriesSysName(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	ss := &senderSession{name: "test", stopCh: make(chan struct{})}
	result := asyncRead(server)
	if err := ss.sendInitiation(client); err != nil {
		t.Fatalf("sendInitiation: %v", err)
	}
	res := <-result
	if res.err != nil {
		t.Fatalf("read initiation: %v", res.err)
	}
	init, ok := res.msg.(*Initiation)
	if !ok {
		t.Fatalf("first message = %T, want *Initiation", res.msg)
	}
	found := 0
	for _, tlv := range init.TLVs {
		if tlv.Type != InitTLVSysName {
			continue
		}
		found++
		if len(tlv.Value) == 0 {
			t.Error("sysName TLV carries an empty value")
		}
	}
	if found != 1 {
		t.Errorf("Initiation TLVs %v carry %d sysName TLVs (type %d), want 1", init.TLVs, found, InitTLVSysName)
	}
}

// openingPeer is the address of the peer whose session never reaches
// Established in the session-opening fixture.
var openingPeer = netip.MustParseAddr("192.0.2.9")

// addOpeningPeer shows the plugin a second peer that exchanged OPENs and sent
// one UPDATE but whose session never reported Up.
func addOpeningPeer(bp *BMPPlugin) {
	open := fabricateLocRIBOpen(localIdentity{asn: 65009, routerID: 0xc0000209})
	for _, direction := range []rpc.MessageDirection{rpc.DirectionSent, rpc.DirectionReceived} {
		bp.handleStructuredEvent(&rpc.StructuredEvent{
			PeerAddress: openingPeer.String(), PeerAS: 65009, Direction: direction,
			EventType:  rpc.EventKindOpen,
			RawMessage: &bgptypes.RawMessage{Type: msgtype.TypeOPEN, RawBytes: open[message.HeaderLen:]},
		})
	}
	bp.handleStructuredEvent(&rpc.StructuredEvent{
		PeerAddress: openingPeer.String(), PeerAS: 65009, RemoteRouterID: 0xc0000209,
		Direction: rpc.DirectionReceived, EventType: rpc.EventKindUpdate,
		RawMessage: &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: announceBody(7), Timestamp: time.Time{}},
	})
}

// peerAddrOf returns the IPv4 address a per-peer header carries.
func peerAddrOf(p *PeerHeader) netip.Addr {
	return netip.AddrFrom4([4]byte(p.Address[12:16]))
}

// openingStream primes one collector over the fixture of one Established peer
// (192.0.2.1, one received route) and one peer that never came up.
func openingStream(t *testing.T) []any {
	t.Helper()
	bp := replayPlugin(t)
	replayUpdate(bp, rpc.DirectionReceived, announceBody(1), time.Time{}, 0)
	addOpeningPeer(bp)
	_, conn := primeReplayCollector(t, bp)
	return decodeBMPStream(t, conn.written())
}

// RFC requirement: RFC7854-x-16 positive -- a new session carries a Peer Up for
// the monitored peer that is Established (192.0.2.1), before anything else.
// RFC requirement: RFC7854-x-16 negative -- the Peer Up is only for peers "in the
// Established state": a peer that exchanged OPENs but never came up gets none.
//
// VALIDATES: the Peer Up set of a new session is exactly the Established peers.
// Method: prime a collector over one Established and one opening peer, and count
// the Peer Ups per address on the bytes the collector received.
func TestRFC7854SessionPeerUpOnlyForEstablishedPeers(t *testing.T) {
	msgs := openingStream(t)
	established := netip.MustParseAddr("192.0.2.1")
	ups := map[netip.Addr]int{}
	for _, m := range msgs {
		up, ok := m.(*PeerUp)
		if !ok {
			continue
		}
		ups[peerAddrOf(&up.Peer)]++
	}
	if ups[established] != 1 {
		t.Errorf("Peer Up for the Established peer: %d, want 1 (all: %v)", ups[established], ups)
	}
	if ups[openingPeer] != 0 {
		t.Errorf("Peer Up for a peer that never reached Established: %d, want 0", ups[openingPeer])
	}
	if len(msgs) == 0 {
		t.Fatal("the collector received nothing")
	}
	if _, ok := msgs[0].(*PeerUp); !ok {
		t.Errorf("first message = %T, want the Peer Up", msgs[0])
	}
}

// RFC requirement: RFC7854-x-17 positive -- after the Peer Up, the session
// carries the Established peer's Adj-RIB-In route as a pre-policy (L clear)
// Route Monitoring, and every Route Monitoring comes after the Peer Up.
// RFC requirement: RFC7854-x-17 negative -- the dump is the Adj-RIBs-In of the
// peers the Peer Ups announced: the UPDATE of a peer that never reached
// Established is not dumped.
//
// VALIDATES: the order Peer Up then Adj-RIB-In contents, and the scope of the dump.
// Method: the same two-peer fixture, read in order off the collector's bytes.
func TestRFC7854SessionDumpsAdjRIBInAfterThePeerUp(t *testing.T) {
	msgs := openingStream(t)
	sawUp := false
	sawRoute := false
	for i, m := range msgs {
		switch v := m.(type) {
		case *PeerUp:
			sawUp = true
		case *RouteMonitoring:
			if !sawUp {
				t.Fatalf("message %d is Route Monitoring before any Peer Up", i)
			}
			if peerAddrOf(&v.Peer) == openingPeer {
				t.Errorf("message %d dumps a route of a peer that never reached Established", i)
			}
			if bytes.Equal(v.BGPUpdate[message.HeaderLen:], announceBody(1)) {
				sawRoute = true
				if v.Peer.isPostPolicy() {
					t.Errorf("Adj-RIB-In route carries the L flag (flags %#x), want pre-policy", v.Peer.Flags)
				}
			}
		}
	}
	if !sawRoute {
		t.Error("the Established peer's Adj-RIB-In route never arrived")
	}
}
