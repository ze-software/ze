// VALIDATES: parseConntrackEvent maps CTA_TIMESTAMP start/stop into StartTime and
// LastSeen (rather than falling back to now) and rejects a structurally malformed
// attribute buffer instead of returning a half-parsed FlowEntry.
// PREVENTS: a destroy-event final record reporting the wrong flow duration, or a
// truncated multicast message being accepted as a valid teardown.

package conntrack

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/mdlayher/netlink"
)

// buildEventWithTimestamp mirrors buildEvent but also nests a CTA_TIMESTAMP with
// the given start/stop nanosecond values, so the timestamp-parsing branch of
// parseConntrackEvent is exercised.
func buildEventWithTimestamp(t *testing.T, src, dst netip.Addr, proto uint8, sport, dport uint16, startNS, stopNS uint64) []byte {
	t.Helper()

	ae := netlink.NewAttributeEncoder()
	ae.ByteOrder = binary.BigEndian

	ae.Nested(ctaTupleOrig, func(nae *netlink.AttributeEncoder) error {
		nae.Nested(ctaTupleIP, func(ip *netlink.AttributeEncoder) error {
			ip.Bytes(ctaIPv4Src, src.AsSlice())
			ip.Bytes(ctaIPv4Dst, dst.AsSlice())
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
		c.Uint64(ctaCountersBytes, 100)
		c.Uint64(ctaCountersPackets, 1)
		return nil
	})
	ae.Nested(ctaTimestamp, func(ts *netlink.AttributeEncoder) error {
		ts.Uint64(ctaTimestampStart, startNS)
		ts.Uint64(ctaTimestampStop, stopNS)
		return nil
	})

	attrs, err := ae.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return append([]byte{2, 0, 0, 0}, attrs...)
}

func TestParseConntrackEventTimestamps(t *testing.T) {
	src := netip.MustParseAddr("192.0.2.10")
	dst := netip.MustParseAddr("198.51.100.20")
	start := time.Unix(1_600_000_000, 123)
	stop := time.Unix(1_600_000_042, 456)

	data := buildEventWithTimestamp(t, src, dst, 6, 40000, 80, uint64(start.UnixNano()), uint64(stop.UnixNano()))

	e, ok := parseConntrackEvent(data)
	if !ok {
		t.Fatal("parse returned false for a valid event with timestamps")
	}
	if !e.StartTime.Equal(start) {
		t.Errorf("StartTime = %v, want %v (from CTA_TIMESTAMP_START)", e.StartTime, start)
	}
	if !e.LastSeen.Equal(stop) {
		t.Errorf("LastSeen = %v, want %v (from CTA_TIMESTAMP_STOP, not now-fallback)", e.LastSeen, stop)
	}
}

func TestParseConntrackEventMalformedAttrs(t *testing.T) {
	// Valid 4-byte nfgenmsg header followed by an NLA header claiming a length
	// (0xffff) far larger than the bytes that follow: the decoder must error and
	// parseConntrackEvent must reject the message rather than returning a
	// half-populated entry.
	bad := []byte{2, 0, 0, 0, 0xff, 0xff, 0x01, 0x00}
	if _, ok := parseConntrackEvent(bad); ok {
		t.Error("expected false for a malformed attribute buffer")
	}
}
