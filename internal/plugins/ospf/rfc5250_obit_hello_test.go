// Design: docs/architecture/wire/ospf.md -- OSPF opaque LSA capability.
// Related: iface/iface.go -- buildHelloPacketLocked, the Hello Ze sends.
// Related: neighbor/table.go -- hello, which records what a received Hello carries.
//
// VALIDATES: RFC 5250 section 3.1: "the O-bit SHOULD NOT be set and MUST be ignored when
// received in packets other than Database Description packets." With opaque enabled, the
// Hellos Ze sends carry no O-bit while its Database Description carries it, and a Hello
// received with the O-bit set leaves the neighbor not opaque-capable.
// PREVENTS: the O-bit leaking into Hellos, and opaque capability learned from a Hello.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc5250OBitExchange runs router 10.0.0.1 with opaque enabled on the point-to-point eth0
// over a recording transport, delivers one Hello from 10.0.0.2 with Options E and O that
// lists 10.0.0.1 (so the adjacency reaches ExStart and Ze sends its first Database
// Description), and waits up to five seconds for Ze to send both a Hello and a Database
// Description. It returns what Ze sent and the neighbor snapshot rows.
func rfc5250OBitExchange(t *testing.T) ([]coldStartSend, []any) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","opaque":true,"areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point","hello-interval":1}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	backend := &coldStartBackend{}
	eng := newEngine(transport.New(backend))
	t.Cleanup(eng.shutdown)
	eng.setConfig(cfg)
	addressedTopology(eng)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}

	peer := ridOf("10.0.0.2")
	hello := ospfpacket.Hello{HelloInterval: 1, Options: types.OptionE | types.OptionO, Priority: 1,
		DeadInterval: uint32(DefaultDeadInterval), Neighbors: []types.RouterID{ridOf("10.0.0.1")}}
	p := ospfpacket.Packet{Header: ospfpacket.Header{Type: ospfpacket.PacketTypeHello, RouterID: peer, AreaID: cfg.Areas[0].AreaID}, Hello: &hello}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: 1, Src: netip.MustParseAddr("10.0.0.2"), Payload: buf})

	deadline := time.Now().Add(5 * time.Second)
	for {
		sends, fails := backend.snapshot()
		if len(fails) != 0 {
			t.Fatalf("Ze sent a packet that does not decode: %v", fails)
		}
		var hellos, dds int
		for _, s := range sends {
			if s.packet.Hello != nil {
				hellos++
			}
			if s.packet.DBDesc != nil {
				dds++
			}
		}
		if hellos > 0 && dds > 0 {
			return sends, eng.neighborSnapshot()
		}
		if time.Now().After(deadline) {
			t.Fatalf("after 5s Ze sent %d Hellos and %d Database Descriptions, want at least one of each", hellos, dds)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// RFC requirement: RFC5250-3.1-5 positive -- with opaque enabled on eth0, every Hello Ze
// sends there has the O-bit (0x40) clear, while the Database Description Ze sends to the
// same neighbor has it set.
func TestRFC5250HelloSentWithoutOBit(t *testing.T) {
	// Goal: the O-bit is a Database Description signal only. Method: a real engine over a
	// recording transport, the sent packets decoded.
	sends, _ := rfc5250OBitExchange(t)
	for _, s := range sends {
		if s.packet.Hello != nil && s.packet.Hello.Options.Has(types.OptionO) {
			t.Fatalf("Hello to %s carries the O-bit: options %#x", s.target, uint8(s.packet.Hello.Options))
		}
		if s.packet.DBDesc != nil && !s.packet.DBDesc.Options.Has(types.OptionO) {
			t.Fatalf("Database Description to %s lacks the O-bit with opaque enabled: options %#x", s.target, uint8(s.packet.DBDesc.Options))
		}
	}
}

// RFC requirement: RFC5250-3.1-5 negative -- a Hello received from 10.0.0.2 with the O-bit
// set, and no Database Description from it, leaves that neighbor not opaque-capable.
func TestRFC5250HelloOBitIgnoredOnReceipt(t *testing.T) {
	// Goal: opaque capability is never learned from a Hello. Method: the positive's
	// exchange, the neighbor snapshot read.
	_, rows := rfc5250OBitExchange(t)
	if len(rows) != 1 {
		t.Fatalf("neighbor rows = %d, want 1", len(rows))
	}
	snap, ok := rows[0].(ospfneighbor.Snapshot)
	if !ok {
		t.Fatalf("snapshot row type = %T, want neighbor.Snapshot", rows[0])
	}
	if snap.RouterID != "10.0.0.2" {
		t.Fatalf("neighbor = %s, want 10.0.0.2", snap.RouterID)
	}
	if snap.OpaqueCapable {
		t.Fatal("10.0.0.2 is opaque-capable after an O-bit Hello and no Database Description")
	}
}
