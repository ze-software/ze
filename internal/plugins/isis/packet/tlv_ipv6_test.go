// Design: plan/spec-isis-2-wire.md -- IPv6 TLV round-trip + prefix/metric boundary tests
package packet

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// VALIDATES: AC-12 -- TLV 232 (IPv6 Interface Address) round-trips a list of
// 16-octet IPv6 addresses.
func TestISISTLVIPv6InterfaceAddr(t *testing.T) {
	in := IPv6InterfaceAddrTLV{Addresses: []netip.Addr{
		netip.MustParseAddr("fe80::1"),
		netip.MustParseAddr("2001:db8::1"),
	}}
	buf := make([]byte, 128)
	n := writeIPv6InterfaceAddrTLV(buf, 0, in)
	it := NewTLVIterator(buf[:n])
	typ, value, ok := it.Next()
	if !ok || typ != TLVIPv6InterfaceAddress {
		t.Fatalf("framing: ok=%v typ=%d", ok, typ)
	}
	out, err := DecodeIPv6InterfaceAddrTLV(value)
	if err != nil {
		t.Fatalf("DecodeIPv6InterfaceAddrTLV: %v", err)
	}
	if len(out.Addresses) != 2 || out.Addresses[0] != in.Addresses[0] || out.Addresses[1] != in.Addresses[1] {
		t.Errorf("addresses = %v, want %v", out.Addresses, in.Addresses)
	}
}

// VALIDATES: AC-8 -- TLV 236 (IPv6 Reachability) round-trips the 4-octet metric,
// the flags octet (U/X/S), the prefix-length octet, the packed prefix
// (ceil(len/8)), and, only when S is set, the sub-TLV-length octet and
// sub-TLVs. Covers the 32-bit metric boundary and the prefix-length boundaries
// 0 and 128.
func TestISISTLVIPv6RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   IPv6ReachEntry
	}{
		{"default-len0", IPv6ReachEntry{
			Metric: types.NewPrefixMetric(10),
			Prefix: netip.PrefixFrom(netip.MustParseAddr("::"), 0),
		}},
		{"host-len128-max-metric", IPv6ReachEntry{
			Metric: types.NewPrefixMetric(types.MaxPrefixMetric), // 4294967295, 32-bit boundary
			Prefix: netip.PrefixFrom(netip.MustParseAddr("2001:db8::1"), 128),
		}},
		{"updown-and-external", IPv6ReachEntry{
			Metric:   types.NewPrefixMetric(20),
			UpDown:   true,
			External: true,
			Prefix:   netip.PrefixFrom(netip.MustParseAddr("2001:db8:1::"), 48),
		}},
		{"with-subtlv-len64", IPv6ReachEntry{
			Metric:  types.NewPrefixMetric(30),
			Prefix:  netip.PrefixFrom(netip.MustParseAddr("2001:db8:2::"), 64),
			SubTLVs: []SubTLV{{Type: 1, Value: []byte{0xde, 0xad}}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := IPv6ReachabilityTLV{Entries: []IPv6ReachEntry{tc.in}}
			buf := make([]byte, 256)
			n := in.WriteTo(buf, 0)
			it := NewTLVIterator(buf[:n])
			typ, value, ok := it.Next()
			if !ok || typ != TLVIPv6Reachability {
				t.Fatalf("framing: ok=%v typ=%d", ok, typ)
			}
			out, err := DecodeIPv6ReachabilityTLV(value)
			if err != nil {
				t.Fatalf("DecodeIPv6ReachabilityTLV: %v", err)
			}
			if len(out.Entries) != 1 {
				t.Fatalf("got %d entries, want 1", len(out.Entries))
			}
			got := out.Entries[0]
			if got.Metric.Value() != tc.in.Metric.Value() {
				t.Errorf("metric = %d, want %d", got.Metric.Value(), tc.in.Metric.Value())
			}
			if got.UpDown != tc.in.UpDown || got.External != tc.in.External {
				t.Errorf("flags U=%v X=%v, want U=%v X=%v", got.UpDown, got.External, tc.in.UpDown, tc.in.External)
			}
			if got.Prefix != tc.in.Prefix {
				t.Errorf("Prefix = %v, want %v", got.Prefix, tc.in.Prefix)
			}
			if len(got.SubTLVs) != len(tc.in.SubTLVs) {
				t.Fatalf("sub-TLVs = %d, want %d", len(got.SubTLVs), len(tc.in.SubTLVs))
			}
			for i := range tc.in.SubTLVs {
				if got.SubTLVs[i].Type != tc.in.SubTLVs[i].Type ||
					!bytes.Equal(got.SubTLVs[i].Value, tc.in.SubTLVs[i].Value) {
					t.Errorf("sub-TLV[%d] mismatch", i)
				}
			}
			buf2 := make([]byte, 256)
			n2 := out.WriteTo(buf2, 0)
			if !bytes.Equal(buf[:n], buf2[:n2]) {
				t.Errorf("TLV 236 re-encode drift:\n got % x\nwant % x", buf2[:n2], buf[:n])
			}
		})
	}
}

// VALIDATES: the TLV 236 flags-octet bit assignment matches RFC 5308 sec 2's
// MSB-first U|X|S layout: U=0x80, X=0x40, S=0x20. This pins the wire contract so
// the bit layout cannot drift away from the RFC (an interop peer encodes per the
// RFC, so X and S MUST sit at 0x40 and 0x20 respectively).
// PREVENTS: a silent X/S flag-bit swap that would make Ze and FRR disagree on the
// external / sub-TLV-present bits (regression for the B6 finding that had X=0x20,
// S=0x40 -- the reverse of the RFC).
func TestISISTLVIPv6FlagBits(t *testing.T) {
	// RFC 5308 sec 2: flags octet is U|X|S|Reserve(5) MSB-first.
	if ipv6ReachFlagUpDown != 0x80 || ipv6ReachFlagExternal != 0x40 || ipv6ReachFlagSubTLV != 0x20 {
		t.Fatalf("TLV 236 flag bits = U%#02x X%#02x S%#02x, want U0x80 X0x40 S0x20 (RFC 5308 sec 2)",
			ipv6ReachFlagUpDown, ipv6ReachFlagExternal, ipv6ReachFlagSubTLV)
	}
	// Encode an entry with only the external bit set and confirm the flags
	// octet has exactly bit 0x40 (X), not 0x20.
	in := IPv6ReachabilityTLV{Entries: []IPv6ReachEntry{{
		Metric:   types.NewPrefixMetric(1),
		External: true,
		Prefix:   netip.PrefixFrom(netip.MustParseAddr("2001:db8::"), 32),
	}}}
	buf := make([]byte, 64)
	n := in.WriteTo(buf, 0)
	it := NewTLVIterator(buf[:n])
	_, value, _ := it.Next()
	flags := value[types.PrefixMetricLen]
	if flags != 0x40 {
		t.Errorf("flags octet = %#02x, want 0x40 (external X only, RFC 5308 sec 2)", flags)
	}
	// Encode an entry with only a sub-TLV (S set) and confirm S is at 0x20.
	inS := IPv6ReachabilityTLV{Entries: []IPv6ReachEntry{{
		Metric:  types.NewPrefixMetric(1),
		Prefix:  netip.PrefixFrom(netip.MustParseAddr("2001:db8::"), 32),
		SubTLVs: []SubTLV{{Type: 1, Value: []byte{0xaa}}},
	}}}
	bufS := make([]byte, 64)
	nS := inS.WriteTo(bufS, 0)
	it = NewTLVIterator(bufS[:nS])
	_, valueS, _ := it.Next()
	if flagsS := valueS[types.PrefixMetricLen]; flagsS != 0x20 {
		t.Errorf("flags octet = %#02x, want 0x20 (sub-TLV-present S only, RFC 5308 sec 2)", flagsS)
	}
}

// VALIDATES: AC-8 -- when S is CLEAR, no sub-TLV length octet is emitted.
func TestISISTLVIPv6NoSubTLVNoLengthOctet(t *testing.T) {
	in := IPv6ReachabilityTLV{Entries: []IPv6ReachEntry{{
		Metric: types.NewPrefixMetric(10),
		Prefix: netip.PrefixFrom(netip.MustParseAddr("2001:db8::"), 32),
	}}}
	buf := make([]byte, 64)
	n := in.WriteTo(buf, 0)
	it := NewTLVIterator(buf[:n])
	_, value, _ := it.Next()
	// metric(4) + flags(1) + prefixlen(1) + 4 prefix octets (/32) = 10.
	wantValueLen := types.PrefixMetricLen + 1 + 1 + 4
	if len(value) != wantValueLen {
		t.Fatalf("value len = %d, want %d (no sub-TLV length octet when S clear)", len(value), wantValueLen)
	}
}

// VALIDATES: AC-11/R-5 -- TLV 236 decode rejects an over-128 prefix length and
// truncated prefix/sub-TLV blocks without panicking.
func TestISISTLVIPv6Malformed(t *testing.T) {
	// prefix length 129 (> 128).
	bad := []byte{0, 0, 0, 10, 0x00, 129}
	if _, err := DecodeIPv6ReachabilityTLV(bad); err == nil {
		t.Fatal("expected ErrLength for prefix length 129")
	}
	// /64 needs 8 prefix octets but only 2 present.
	short := []byte{0, 0, 0, 10, 0x00, 64, 0x20, 0x01}
	if _, err := DecodeIPv6ReachabilityTLV(short); err == nil {
		t.Fatal("expected truncation for short prefix")
	}
}
