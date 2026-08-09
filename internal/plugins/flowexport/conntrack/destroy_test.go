// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- conntrack destroy-event parser test

package conntrack

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/mdlayher/netlink"
)

// buildEvent encodes a synthetic ctnetlink destroy message body: a 4-byte
// nfgenmsg header followed by the nested CTA attributes the parser reads. It
// mirrors the kernel wire layout so parseConntrackEvent is exercised exactly as
// it would be against a live NFNLGRP_CONNTRACK_DESTROY message.
func buildEvent(t *testing.T, src, dst netip.Addr, proto uint8, sport, dport uint16, fwdB, fwdP, revB, revP uint64) []byte {
	t.Helper()

	ae := netlink.NewAttributeEncoder()
	ae.ByteOrder = binary.BigEndian

	ipSrcType, ipDstType := uint16(ctaIPv4Src), uint16(ctaIPv4Dst)
	if src.Is6() {
		ipSrcType, ipDstType = ctaIPv6Src, ctaIPv6Dst
	}

	ae.Nested(ctaTupleOrig, func(nae *netlink.AttributeEncoder) error {
		nae.Nested(ctaTupleIP, func(ip *netlink.AttributeEncoder) error {
			ip.Bytes(ipSrcType, src.AsSlice())
			ip.Bytes(ipDstType, dst.AsSlice())
			return nil
		})
		nae.Nested(ctaTupleProto, func(pr *netlink.AttributeEncoder) error {
			pr.Uint8(ctaProtoNum, proto)
			pr.Uint16(ctaProtoSrcPort, sport)
			pr.Uint16(ctaProtoDstPort, dport)
			return nil
		})
		return nil
	})
	ae.Nested(ctaCountersOrig, func(c *netlink.AttributeEncoder) error {
		c.Uint64(ctaCountersBytes, fwdB)
		c.Uint64(ctaCountersPackets, fwdP)
		return nil
	})
	ae.Nested(ctaCountersReply, func(c *netlink.AttributeEncoder) error {
		c.Uint64(ctaCountersBytes, revB)
		c.Uint64(ctaCountersPackets, revP)
		return nil
	})

	attrs, err := ae.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// 4-byte nfgenmsg header: family, version, res_id (the parser skips it).
	hdr := []byte{2, 0, 0, 0}
	if src.Is6() {
		hdr[0] = 10 // AF_INET6
	}
	return append(hdr, attrs...)
}

func TestParseConntrackEventIPv4(t *testing.T) {
	src := netip.MustParseAddr("192.0.2.1")
	dst := netip.MustParseAddr("198.51.100.2")
	data := buildEvent(t, src, dst, 6, 1234, 443, 1000, 10, 500, 5)

	e, ok := parseConntrackEvent(data)
	if !ok {
		t.Fatal("parse returned false for a valid event")
	}
	if e.SrcAddr != src {
		t.Errorf("src = %v, want %v", e.SrcAddr, src)
	}
	if e.DstAddr != dst {
		t.Errorf("dst = %v, want %v", e.DstAddr, dst)
	}
	if e.Protocol != 6 {
		t.Errorf("protocol = %d, want 6", e.Protocol)
	}
	if e.SrcPort != 1234 {
		t.Errorf("src port = %d, want 1234", e.SrcPort)
	}
	if e.DstPort != 443 {
		t.Errorf("dst port = %d, want 443", e.DstPort)
	}
	// Bytes/packets are the sum of both directions.
	if e.Bytes != 1500 {
		t.Errorf("bytes = %d, want 1500 (fwd+rev)", e.Bytes)
	}
	if e.Packets != 15 {
		t.Errorf("packets = %d, want 15 (fwd+rev)", e.Packets)
	}
	// No CTA_TIMESTAMP in the fixture: LastSeen falls back to now (non-zero).
	if e.LastSeen.IsZero() {
		t.Error("LastSeen should fall back to now when no timestamp is present")
	}
}

func TestParseConntrackEventIPv6(t *testing.T) {
	src := netip.MustParseAddr("2001:db8::1")
	dst := netip.MustParseAddr("2001:db8::2")
	data := buildEvent(t, src, dst, 17, 53, 5353, 200, 2, 0, 0)

	e, ok := parseConntrackEvent(data)
	if !ok {
		t.Fatal("parse returned false for a valid IPv6 event")
	}
	if e.SrcAddr != src || e.DstAddr != dst {
		t.Errorf("addrs = %v/%v, want %v/%v", e.SrcAddr, e.DstAddr, src, dst)
	}
	if e.Protocol != 17 {
		t.Errorf("protocol = %d, want 17 (UDP)", e.Protocol)
	}
	if e.Bytes != 200 || e.Packets != 2 {
		t.Errorf("counters = %d bytes / %d pkts, want 200/2", e.Bytes, e.Packets)
	}
}

func TestParseConntrackEventTooShort(t *testing.T) {
	if _, ok := parseConntrackEvent([]byte{1, 2}); ok {
		t.Error("expected false for a buffer shorter than the nfgenmsg header")
	}
}

func TestParseConntrackEventNoTuple(t *testing.T) {
	// Only an nfgenmsg header, no attributes: no usable tuple.
	if _, ok := parseConntrackEvent([]byte{2, 0, 0, 0}); ok {
		t.Error("expected false when no CTA_TUPLE_ORIG is present")
	}
}
