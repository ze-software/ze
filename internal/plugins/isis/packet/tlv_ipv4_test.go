// Design: docs/architecture/wire/isis.md -- IPv4 TLV round-trip + prefix/metric boundary tests
package packet

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// VALIDATES: AC-12 -- TLV 132 (IP Interface Address) round-trips a list of
// 4-octet IPv4 addresses.
//
// RFC requirement: RFC1195-5.2-2 positive -- the IP Interface Address TLV 132 round-trips a list of 4-octet IPv4 addresses through encode/decode, so an IP-capable router can carry TLV 132 in its LSPs (RFC 1195 sec 5.2).
func TestISISTLVIPv4InterfaceAddr(t *testing.T) {
	in := IPv4InterfaceAddrTLV{Addresses: []netip.Addr{
		netip.MustParseAddr("192.0.2.1"),
		netip.MustParseAddr("10.0.0.254"),
	}}
	buf := make([]byte, 64)
	n := writeIPv4InterfaceAddrTLV(buf, 0, in)
	it := NewTLVIterator(buf[:n])
	typ, value, ok := it.Next()
	if !ok || typ != TLVIPInterfaceAddress {
		t.Fatalf("framing: ok=%v typ=%d", ok, typ)
	}
	out, err := DecodeIPv4InterfaceAddrTLV(value)
	if err != nil {
		t.Fatalf("DecodeIPv4InterfaceAddrTLV: %v", err)
	}
	if len(out.Addresses) != 2 || out.Addresses[0] != in.Addresses[0] || out.Addresses[1] != in.Addresses[1] {
		t.Errorf("addresses = %v, want %v", out.Addresses, in.Addresses)
	}
}

// VALIDATES: AC-7 -- TLV 135 (Extended IP Reachability) round-trips the 4-octet
// metric, the control octet (up/down + S bit + 6-bit prefix length), the packed
// prefix (ceil(len/8)), and, only when S is set, the sub-TLV-length octet and
// sub-TLVs. Also covers the 32-bit metric boundary and the prefix-length
// boundaries 0 and 32 (R-5: ceil(len/8) at the edges).
//
// RFC requirement: RFC1195-5.2-4 positive -- every TLV 135 IP reachability entry carries its 4-octet metric through encode/decode (covering the 32-bit boundary value), so no reachability entry is advertised without a metric (RFC 1195 sec 5.2: each reachability entry MUST carry a default metric).
func TestISISTLVIPv4RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   ExtIPReachEntry
	}{
		{"default-route-len0", ExtIPReachEntry{
			Metric: types.NewPrefixMetric(10),
			Prefix: netip.PrefixFrom(netip.MustParseAddr("0.0.0.0"), 0), // 0 prefix octets
		}},
		{"host-len32-max-metric", ExtIPReachEntry{
			Metric: types.NewPrefixMetric(types.MaxPrefixMetric), // 4294967295, 32-bit boundary
			Prefix: netip.PrefixFrom(netip.MustParseAddr("192.0.2.7"), 32),
		}},
		{"updown-set", ExtIPReachEntry{
			Metric: types.NewPrefixMetric(20),
			UpDown: true, // up/down bit in the CONTROL octet, not the metric
			Prefix: netip.PrefixFrom(netip.MustParseAddr("10.1.0.0"), 16),
		}},
		// RFC requirement: RFC5305-2-1 positive -- an unknown sub-TLV (type 1; RFC 5305 defines no sub-TLVs for TLV 135) is retained and round-trips through TLV 135 decode/encode rather than being rejected (RFC 5305 sec 2).
		{"with-subtlv", ExtIPReachEntry{
			Metric:  types.NewPrefixMetric(30),
			Prefix:  netip.PrefixFrom(netip.MustParseAddr("172.16.0.0"), 24),
			SubTLVs: []SubTLV{{Type: 1, Value: []byte{0xaa, 0xbb}}},
		}},
		{"len1", ExtIPReachEntry{
			Metric: types.NewPrefixMetric(5),
			Prefix: netip.PrefixFrom(netip.MustParseAddr("128.0.0.0"), 1), // 1 prefix octet
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := ExtendedIPReachTLV{Entries: []ExtIPReachEntry{tc.in}}
			buf := make([]byte, 256)
			n := in.WriteTo(buf, 0)
			it := NewTLVIterator(buf[:n])
			typ, value, ok := it.Next()
			if !ok || typ != TLVExtendedIPReach {
				t.Fatalf("framing: ok=%v typ=%d", ok, typ)
			}
			out, err := DecodeExtendedIPReachTLV(value)
			if err != nil {
				t.Fatalf("DecodeExtendedIPReachTLV: %v", err)
			}
			if len(out.Entries) != 1 {
				t.Fatalf("got %d entries, want 1", len(out.Entries))
			}
			got := out.Entries[0]
			if got.Metric.Value() != tc.in.Metric.Value() {
				t.Errorf("metric = %d, want %d", got.Metric.Value(), tc.in.Metric.Value())
			}
			if got.UpDown != tc.in.UpDown {
				t.Errorf("UpDown = %v, want %v", got.UpDown, tc.in.UpDown)
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
			// Re-encode stability.
			buf2 := make([]byte, 256)
			n2 := out.WriteTo(buf2, 0)
			if !bytes.Equal(buf[:n], buf2[:n2]) {
				t.Errorf("TLV 135 re-encode drift:\n got % x\nwant % x", buf2[:n2], buf[:n])
			}
		})
	}
}

// VALIDATES: AC-7 -- when the S (sub-TLV-present) bit is CLEAR, NO sub-TLV
// length octet is emitted (the entry is exactly metric+control+prefix). This is
// the framing trap the umbrella contract calls out explicitly.
func TestISISTLVIPv4NoSubTLVNoLengthOctet(t *testing.T) {
	in := ExtendedIPReachTLV{Entries: []ExtIPReachEntry{{
		Metric: types.NewPrefixMetric(10),
		Prefix: netip.PrefixFrom(netip.MustParseAddr("192.0.2.0"), 24),
	}}}
	buf := make([]byte, 64)
	n := in.WriteTo(buf, 0)
	it := NewTLVIterator(buf[:n])
	_, value, _ := it.Next()
	// metric(4) + control(1) + 3 prefix octets (/24) = 8, NO sub-TLV length.
	wantValueLen := types.PrefixMetricLen + 1 + 3
	if len(value) != wantValueLen {
		t.Fatalf("value len = %d, want %d (no sub-TLV length octet when S clear)", len(value), wantValueLen)
	}
	// The control octet's S bit must be 0.
	ctrl := value[types.PrefixMetricLen]
	if ctrl&extIPCtrlSubTLV != 0 {
		t.Errorf("S bit set in control octet %#02x when no sub-TLVs", ctrl)
	}
}

// VALIDATES: AC-11/R-5 -- TLV 135 decode rejects an over-32 prefix length and a
// prefix/sub-TLV block that overruns the value, without panicking.
func TestISISTLVIPv4Malformed(t *testing.T) {
	// Control octet claims prefix length 33 (> 32).
	bad := []byte{0, 0, 0, 10, 33}
	if _, err := DecodeExtendedIPReachTLV(bad); err == nil {
		t.Fatal("expected ErrLength for prefix length 33")
	}
	// /24 needs 3 prefix octets but only 1 present.
	short := []byte{0, 0, 0, 10, 24, 0xc0}
	if _, err := DecodeExtendedIPReachTLV(short); err == nil {
		t.Fatal("expected truncation for short prefix")
	}
	// S bit set, prefix /0, but no sub-TLV length octet.
	subShort := []byte{0, 0, 0, 10, extIPCtrlSubTLV}
	if _, err := DecodeExtendedIPReachTLV(subShort); err == nil {
		t.Fatal("expected truncation for missing sub-TLV length octet")
	}
}
