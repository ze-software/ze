package server

import (
	"net/netip"
	"testing"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// buildBenchUpdate builds a realistic UPDATE RawMessage for benchmarking.
// Contains ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF attributes and an IPv4 NLRI.
func buildBenchUpdate() (plugin.PeerInfo, bgptypes.RawMessage) {
	// Minimal UPDATE payload: withdrawn(2) + attrs + NLRI
	// Attrs: ORIGIN(IGP) + AS_PATH(65001) + NEXT_HOP(10.0.0.1) + MED(100) + LOCAL_PREF(200)
	// NLRI: 10.0.0.0/24
	attrBuf := make([]byte, 256)
	off := 0

	// Withdrawn Routes Length = 0
	attrBuf[off] = 0
	attrBuf[off+1] = 0
	off += 2

	// Build attributes into a temp buffer, then write total length.
	attrStart := off + 2 // skip 2 bytes for attr length
	attrOff := attrStart

	// ORIGIN = IGP (code 1, flags 0x40, len 1, value 0)
	attrOff += attribute.WriteAttrTo(attribute.Origin(0), attrBuf, attrOff)
	// AS_PATH = [65001] (code 2, type AS_SEQUENCE)
	asp := &attribute.ASPath{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{65001}}}}
	attrOff += attribute.WriteAttrTo(asp, attrBuf, attrOff)
	// NEXT_HOP = 10.0.0.1 (code 3)
	nh := &attribute.NextHop{Addr: netip.MustParseAddr("10.0.0.1")}
	attrOff += attribute.WriteAttrTo(nh, attrBuf, attrOff)
	// MED = 100 (code 4)
	attrOff += attribute.WriteAttrTo(attribute.MED(100), attrBuf, attrOff)
	// LOCAL_PREF = 200 (code 5)
	attrOff += attribute.WriteAttrTo(attribute.LocalPref(200), attrBuf, attrOff)

	// Write total path attribute length.
	attrLen := attrOff - attrStart
	attrBuf[off] = byte(attrLen >> 8)
	attrBuf[off+1] = byte(attrLen)
	off = attrOff

	// NLRI: 10.0.0.0/24 = prefix-len(24) + 3 bytes
	attrBuf[off] = 24
	attrBuf[off+1] = 10
	attrBuf[off+2] = 0
	attrBuf[off+3] = 0
	off += 4

	payload := make([]byte, off)
	copy(payload, attrBuf[:off])

	ctxID := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, nil))
	wu := wireu.NewWireUpdate(payload, ctxID)
	attrsWire := attribute.NewAttributesWire(payload[4:4+attrLen], ctxID)

	peer := plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.254"),
		PeerAS:       65001,
		LocalAS:      65000,
		Name:         "peer1",
		GroupName:    "group1",
	}

	msg := bgptypes.RawMessage{
		Type:       message.TypeUPDATE,
		RawBytes:   payload,
		Direction:  "received",
		MessageID:  42,
		AttrsWire:  attrsWire,
		WireUpdate: wu,
	}

	return peer, msg
}

// BenchmarkJSONPath measures the JSON format + parse round-trip.
//
// VALIDATES: AC-7 — baseline for JSON path cost (format + ParseEvent).
func BenchmarkJSONPath(b *testing.B) {
	peer, msg := buildBenchUpdate()
	encoder := format.NewJSONEncoder("")

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		// Engine side: format to JSON text.
		jsonStr := formatMessageForSubscription(encoder, peer, msg, "parsed", "json")
		// Plugin side: parse JSON back to Event.
		event, parseErr := bgp.ParseEvent([]byte(jsonStr))
		if parseErr != nil {
			b.Fatal(parseErr)
		}
		_ = event.GetPeerAddress()
		_ = event.GetPeerASN()
	}
}

// BenchmarkStructuredPath measures the structured delivery path.
//
// VALIDATES: AC-7 — structured path has fewer allocs than JSON path.
func BenchmarkStructuredPath(b *testing.B) {
	peer, msg := buildBenchUpdate()

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		// Engine side: build StructuredEvent from PeerInfo + RawMessage.
		se := getStructuredEvent(peer, &msg)
		// Plugin side: read fields directly.
		_ = se.PeerAddress
		_ = se.PeerAS
		_ = se.MessageID
		// Read one attribute lazily (typical rpki path: AS_PATH only).
		if m, ok := se.RawMessage.(*bgptypes.RawMessage); ok && m.AttrsWire != nil {
			attr, attrErr := m.AttrsWire.Get(attribute.AttrASPath)
			if attrErr != nil {
				b.Fatal(attrErr)
			}
			_ = attr
		}
		rpc.PutStructuredEvent(se)
	}
}
