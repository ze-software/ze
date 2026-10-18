package packet

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------------
// Shared golden vectors and fixtures (used across packet/checksum/validate tests)
// -----------------------------------------------------------------------------

// Golden byte vectors from spec-vrrp-1-packet.md, checksums hand-verified via an
// independent RFC 1071 computation (see TestChecksumRFC1071). G2 is the RFC 9568
// message-only form, what a v3/IPv4 group sends and accepts by default. G2c is
// the RFC 5798 pseudo-header form (0xDEFB is the pseudo-header sum for src
// 192.0.2.251 -> dst 224.0.0.18), what a group configured `checksum
// rfc5798-compatible` sends and accepts, for keepalived and the deployed base.
const (
	goldenV2Hex         = "210ac802000192edc0000201c00002020000000000000000"                                 // G1  v2 IPv4, cksum 0x92ED
	goldenV3v4Hex       = "310ac8020064828ac0000201c0000202"                                                 // G2  v3 IPv4, cksum 0x828A message-only
	goldenV3v4CompatHex = "310ac8020064defbc0000201c0000202"                                                 // G2c v3 IPv4, cksum 0xDEFB legacy pseudo-header
	goldenV3v6Hex       = "310a640200643f5dfe80000000000000000000000000000120010db8000000000000000000000001" // G3  v3 IPv6, cksum 0x3F5D pseudo-header
)

func mustHex(t testing.TB, s string) []byte {
	t.Helper()
	clean := strings.NewReplacer(" ", "", "\n", "", "\t", "").Replace(s)
	b, err := hex.DecodeString(clean)
	if err != nil {
		t.Fatalf("hex decode %q: %v", s, err)
	}
	return b
}

func addr(t testing.TB, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("parse addr %q: %v", s, err)
	}
	return a
}

// lookupConst returns a Lookup that always resolves to the given local group.
func lookupConst(version uint8, ms uint32) Lookup {
	return func(uint8) (Local, bool) {
		return Local{Version: version, AdverIntervalMS: ms, Checksum: ChecksumRFC9568}, true
	}
}

// metaV2/V3v4/V3v6 build the RxMeta a valid golden packet arrives with.
func metaV2(t testing.TB) RxMeta {
	return RxMeta{TTL: 255, Src: addr(t, "192.0.2.10"), Dst: MulticastV4, Family: V4}
}
func metaV3v4(t testing.TB) RxMeta {
	return RxMeta{TTL: 255, Src: addr(t, "192.0.2.251"), Dst: MulticastV4, Family: V4}
}
func metaV3v6(t testing.TB) RxMeta {
	return RxMeta{TTL: 255, Src: addr(t, "fe80::c8"), Dst: MulticastV6, Family: V6}
}

func advV2(t testing.TB) Advertisement {
	return Advertisement{Version: VersionV2, Family: V4, VRID: 10, Priority: 200, AdverIntervalMS: 1000,
		VIPs: []netip.Addr{addr(t, "192.0.2.1"), addr(t, "192.0.2.2")}}
}
func advV3v4(t testing.TB) Advertisement {
	return Advertisement{Version: VersionV3, Family: V4, VRID: 10, Priority: 200, AdverIntervalMS: 1000,
		VIPs: []netip.Addr{addr(t, "192.0.2.1"), addr(t, "192.0.2.2")}}
}
func advV3v6(t testing.TB) Advertisement {
	return Advertisement{Version: VersionV3, Family: V6, VRID: 10, Priority: 100, AdverIntervalMS: 1000,
		VIPs: []netip.Addr{addr(t, "fe80::1"), addr(t, "2001:db8::1")}}
}

// -----------------------------------------------------------------------------
// Encode golden tests (AC-1, AC-2)
// -----------------------------------------------------------------------------

// VALIDATES: AC-1 -- v2 encode produces the exact G1 bytes (auth type 0, Adver
// Int byte = 1 second, 8-byte zero auth trailer, checksum 0x92ED).
// PREVENTS: field shift, wrong unit, missing trailer, wrong checksum.
func TestEncodeGoldenV2(t *testing.T) {
	// RFC requirement: RFC3768-7.2-1 positive -- WriteTo fills every VRRP field from the advertisement state and FillChecksum computes the checksum, producing the exact golden bytes (packet.go:251, checksum.go:86).
	adv := advV2(t)
	buf := make([]byte, MaxLenV2)
	n := adv.WriteTo(buf, 0)
	FillChecksum(buf, 0, n, addr(t, "192.0.2.251"), MulticastV4, ChecksumRFC9568)
	want := mustHex(t, goldenV2Hex)
	if n != len(want) {
		t.Fatalf("wrote %d bytes, want %d", n, len(want))
	}
	if !bytes.Equal(buf[:n], want) {
		t.Fatalf("v2 encode mismatch:\ngot:  % x\nwant: % x", buf[:n], want)
	}
}

// VALIDATES: AC-2 -- v3 IPv4 encode produces exact G2 bytes by default
// (reserved nibble 0, 12-bit interval = 100 cs, RFC 9568 message-only checksum
// 0x828A) and exact G2c bytes in rfc5798-compatible mode (pseudo-header 0xDEFB).
// PREVENTS: reserve-nibble leak, cs/ms confusion, a checksum form other than the
// group's configured one.
//
// RFC requirement: RFC9568-5.2.6-1 positive -- WriteTo writes the Reserve nibble as zero on transmission; only the low 12 bits of bytes 4-5 carry the interval (packet.go:269)
// RFC requirement: RFC5798-5.2.6-1 positive -- WriteTo writes the rsvd nibble as zero on transmission, the first half of RFC 5798 Section 5.2.6; only the low 12 bits of bytes 4-5 carry the interval (packet.go:269)
// RFC requirement: RFC9568-7.2-1 positive -- WriteTo fills the Version, Type, VRID, Priority, Count, Reserve, Max Advertise Interval and IPv4 address fields from the Virtual Router's advertisement state, each equal to the golden octets (packet.go:251).
// RFC requirement: RFC9568-7.2-5 positive -- FillChecksum in the default ChecksumRFC9568 mode computes the VRRPv3 IPv4 checksum over the VRRP message alone, the golden G2 bytes with checksum 0x828A (FillChecksum checksum.go).
// RFC requirement: RFC9568-5.2.8-2 positive -- the default IPv4 checksum covers only the VRRP message from the Version field to the last IPv4 address, so it equals 0x828A and not the pseudo-header sum 0xDEFB for the same source and destination (FillChecksum, pseudoSumFor checksum.go).
// RFC requirement: RFC5798-7.2-1 positive -- WriteTo fills every VRRP field from the virtual router's advertisement state and FillChecksum in rfc5798-compatible mode computes the RFC 5798 pseudo-header checksum, producing the exact golden G2c bytes (packet.go:251, checksum.go).
func TestEncodeGoldenV3IPv4(t *testing.T) {
	cases := []struct {
		name string
		mode ChecksumMode
		want string
	}{
		{"rfc9568-message-only", ChecksumRFC9568, goldenV3v4Hex},
		{"rfc5798-compatible-pseudo-header", ChecksumRFC5798Compatible, goldenV3v4CompatHex},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adv := advV3v4(t)
			buf := make([]byte, MaxLenV3v4)
			n := adv.WriteTo(buf, 0)
			FillChecksum(buf, 0, n, addr(t, "192.0.2.251"), MulticastV4, tc.mode)
			if buf[4]&0xF0 != 0 {
				t.Fatalf("Reserve nibble = %#x, want 0 on transmission", buf[4]&0xF0)
			}
			want := mustHex(t, tc.want)
			if n != len(want) {
				t.Fatalf("wrote %d bytes, want %d", n, len(want))
			}
			if !bytes.Equal(buf[:n], want) {
				t.Fatalf("v3 IPv4 encode mismatch:\ngot:  % x\nwant: % x", buf[:n], want)
			}
		})
	}
}

// VALIDATES: a v3 IPv4 advertisement is accepted only in the checksum form its
// group is configured for: G2 (message-only) under the default mode, G2c
// (pseudo-header) under rfc5798-compatible, and each is refused with
// ErrChecksum under the other mode.
// PREVENTS: the dual-accept that kept a message-only advertisement whose
// checksum was wrong when its pseudo-header sum matched.
//
// RFC requirement: RFC9568-5.2.8-2 positive -- under the default mode an IPv4 advert carrying the message-only checksum (G2, 0x828A) decodes (verifyReceived checksum.go, Decode validate.go).
// RFC requirement: RFC9568-5.2.8-2 negative -- under the default mode the same advert carrying the RFC 5798 pseudo-header checksum (G2c, 0xDEFB) is discarded with ErrChecksum, so a checksum that includes the pseudo-header is not accepted (verifyReceived checksum.go, Decode validate.go).
// RFC requirement: RFC9568-7.2-5 negative -- a v3 IPv4 group configured rfc5798-compatible discards the message-only G2 advert with ErrChecksum and accepts only the pseudo-header G2c form it computes itself (verifyReceived, pseudoSumFor checksum.go).
func TestDecodeV3IPv4ChecksumOnlyConfiguredForm(t *testing.T) {
	cases := []struct {
		name   string
		mode   ChecksumMode
		golden string
		accept bool
	}{
		{"rfc9568-accepts-message-only", ChecksumRFC9568, goldenV3v4Hex, true},
		{"rfc9568-refuses-pseudo-header", ChecksumRFC9568, goldenV3v4CompatHex, false},
		{"rfc5798-compatible-accepts-pseudo-header", ChecksumRFC5798Compatible, goldenV3v4CompatHex, true},
		{"rfc5798-compatible-refuses-message-only", ChecksumRFC5798Compatible, goldenV3v4Hex, false},
		{"unspecified-refuses-message-only", ChecksumUnspecified, goldenV3v4Hex, false},
		{"unspecified-refuses-pseudo-header", ChecksumUnspecified, goldenV3v4CompatHex, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookup := func(uint8) (Local, bool) {
				return Local{Version: VersionV3, AdverIntervalMS: 1000, Checksum: tc.mode}, true
			}
			_, err := Decode(mustHex(t, tc.golden), metaV3v4(t), lookup)
			if tc.accept && err != nil {
				t.Fatalf("Decode: %v, want accept", err)
			}
			if !tc.accept && !errors.Is(err, ErrChecksum) {
				t.Fatalf("Decode err = %v, want ErrChecksum", err)
			}
		})
	}
}

// VALIDATES: AC-2 -- v3 IPv6 encode produces exact G3 bytes (RFC 8200
// pseudo-header checksum 0x3F5D).
// PREVENTS: wrong pseudo-header composition, wrong upper-layer length.
//
// RFC requirement: RFC9568-7.2-1 positive -- WriteTo fills the Version, Type, VRID, Priority, Count, Reserve, Max Advertise Interval and IPv6 address fields from the Virtual Router's advertisement state, each equal to the golden octets (packet.go:251).
func TestEncodeGoldenV3IPv6(t *testing.T) {
	adv := advV3v6(t)
	buf := make([]byte, MaxLenV3v6)
	n := adv.WriteTo(buf, 0)
	FillChecksum(buf, 0, n, addr(t, "fe80::c8"), MulticastV6, ChecksumRFC9568)
	want := mustHex(t, goldenV3v6Hex)
	if n != len(want) {
		t.Fatalf("wrote %d bytes, want %d", n, len(want))
	}
	if !bytes.Equal(buf[:n], want) {
		t.Fatalf("v3 IPv6 encode mismatch:\ngot:  % x\nwant: % x", buf[:n], want)
	}
}

// VALIDATES: WriteTo honors a non-zero offset and leaves earlier bytes intact
// (bfd control_test.go:109 model).
// PREVENTS: offset argument ignored or misapplied.
func TestWriteToOffset(t *testing.T) {
	const prefix = 8
	buf := make([]byte, prefix+MaxLenV3v4)
	for i := range prefix {
		buf[i] = 0xAA
	}
	adv := advV3v4(t)
	n := adv.WriteTo(buf, prefix)
	FillChecksum(buf, prefix, n, addr(t, "192.0.2.251"), MulticastV4, ChecksumRFC9568)
	for i := range prefix {
		if buf[i] != 0xAA {
			t.Fatalf("byte %d clobbered: %#x", i, buf[i])
		}
	}
	// Message-only form (G2), the default v3/IPv4 tx checksum -- see TestEncodeGoldenV3IPv4.
	want := mustHex(t, goldenV3v4Hex)
	if !bytes.Equal(buf[prefix:prefix+n], want) {
		t.Fatalf("offset encode mismatch:\ngot:  % x\nwant: % x", buf[prefix:prefix+n], want)
	}
}

// -----------------------------------------------------------------------------
// Round-trip matrix (AC-3): encode -> decode preserves every field
// -----------------------------------------------------------------------------

// VALIDATES: encode->decode round-trips across {v2, v3v4, v3v6} x counts
// {1,3,16} x priorities {0,1,100,254,255}; AdverIntervalMS stays 1000 ms.
// PREVENTS: silent field re-ordering, unit drift, VIP truncation.
func TestRoundTrip(t *testing.T) {
	counts := []int{1, 3, 16}
	priorities := []uint8{0, 1, 100, 254, 255}
	families := []struct {
		name    string
		version uint8
		family  uint8
		maxLen  int
		meta    RxMeta
		src     netip.Addr
		vip     func(i int) netip.Addr
	}{
		{"v2", VersionV2, V4, MaxLenV2, metaV2(t), addr(t, "192.0.2.251"), func(i int) netip.Addr {
			return netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)})
		}},
		{"v3v4", VersionV3, V4, MaxLenV3v4, metaV3v4(t), addr(t, "192.0.2.251"), func(i int) netip.Addr {
			return netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)})
		}},
		{"v3v6", VersionV3, V6, MaxLenV3v6, metaV3v6(t), addr(t, "fe80::c8"), func(i int) netip.Addr {
			if i == 0 {
				return addr(t, "fe80::1") // first IPv6 VIP MUST be link-local
			}
			return netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, byte(i)})
		}},
	}

	for _, fam := range families {
		for _, count := range counts {
			for _, prio := range priorities {
				name := fam.name + "/count=" + itoa(count) + "/prio=" + itoa(int(prio))
				t.Run(name, func(t *testing.T) {
					vips := make([]netip.Addr, count)
					for i := range vips {
						vips[i] = fam.vip(i)
					}
					adv := Advertisement{Version: fam.version, Family: fam.family, VRID: 10,
						Priority: prio, AdverIntervalMS: 1000, VIPs: vips}
					buf := make([]byte, fam.maxLen)
					n := adv.WriteTo(buf, 0)
					FillChecksum(buf, 0, n, fam.src, fam.meta.Dst, ChecksumRFC9568)

					got, err := Decode(buf[:n], fam.meta, lookupConst(fam.version, 1000))
					if err != nil {
						t.Fatalf("Decode: %v", err)
					}
					if got.Version != fam.version || got.Family != fam.family || got.VRID != 10 ||
						got.Priority != prio || int(got.Count) != count || got.AdverIntervalMS != 1000 {
						t.Fatalf("field mismatch: got %+v", got)
					}
					if got.VIPCount() != count {
						t.Fatalf("VIPCount = %d, want %d", got.VIPCount(), count)
					}
					for i := range vips {
						if got.VIPAt(i) != vips[i] {
							t.Fatalf("VIP[%d] = %v, want %v", i, got.VIPAt(i), vips[i])
						}
					}
				})
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Boundary tests (AC-9) -- encode side asserts range via Validate()
// -----------------------------------------------------------------------------

// VALIDATES: encode-side VRID boundary (1..255): 0 rejected.
func TestBoundaryVRID(t *testing.T) {
	base := advV3v4(t)
	base.VRID = 1
	if err := base.Validate(); err != nil {
		t.Fatalf("vrid 1 rejected: %v", err)
	}
	base.VRID = 255
	if err := base.Validate(); err != nil {
		t.Fatalf("vrid 255 rejected: %v", err)
	}
	base.VRID = 0
	if err := base.Validate(); !errors.Is(err, ErrVRIDRange) {
		t.Fatalf("vrid 0: got %v, want ErrVRIDRange", err)
	}
}

// VALIDATES: encode-side count boundary (1..16): 0 and 17 rejected, 16 accepted.
func TestBoundaryCount(t *testing.T) {
	mk := func(n int) Advertisement {
		a := advV3v4(t)
		a.VIPs = make([]netip.Addr, n)
		for i := range a.VIPs {
			a.VIPs[i] = netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)})
		}
		return a
	}
	if err := mk(16).Validate(); err != nil {
		t.Fatalf("count 16 rejected: %v", err)
	}
	if err := mk(1).Validate(); err != nil {
		t.Fatalf("count 1 rejected: %v", err)
	}
	if err := mk(0).Validate(); !errors.Is(err, ErrCountRange) {
		t.Fatalf("count 0: got %v, want ErrCountRange", err)
	}
	if err := mk(17).Validate(); !errors.Is(err, ErrCountRange) {
		t.Fatalf("count 17: got %v, want ErrCountRange", err)
	}
}

// RFC requirement: RFC5798-7.2-1 negative -- an advertisement interval outside the wire range of the configured version is refused by Validate with ErrIntervalRange rather than filled into a packet, for the v2 seconds range and for the v3 centisecond range alike (packet.go:201).
// VALIDATES: encode-side interval boundaries for both wire ranges (AC-9).
// v3: 10..40950 (multiples of 10); v2: 1000..255000 (multiples of 1000).
// PREVENTS: cs/s/ms confusion and 12-bit/8-bit wire overflow (R-2, N10).
func TestBoundaryInterval(t *testing.T) {
	v3 := advV3v4(t)
	for _, ms := range []uint32{10, 40950} { // last valid extremes
		v3.AdverIntervalMS = ms
		if err := v3.Validate(); err != nil {
			t.Fatalf("v3 interval %d ms rejected: %v", ms, err)
		}
	}
	for _, ms := range []uint32{0, 40960} { // below / above 12-bit cs cap
		v3.AdverIntervalMS = ms
		if err := v3.Validate(); !errors.Is(err, ErrIntervalRange) {
			t.Fatalf("v3 interval %d ms: got %v, want ErrIntervalRange", ms, err)
		}
	}

	v2 := advV2(t)
	for _, ms := range []uint32{1000, 255000} {
		v2.AdverIntervalMS = ms
		if err := v2.Validate(); err != nil {
			t.Fatalf("v2 interval %d ms rejected: %v", ms, err)
		}
	}
	for _, ms := range []uint32{0, 999, 256000} {
		v2.AdverIntervalMS = ms
		if err := v2.Validate(); !errors.Is(err, ErrIntervalRange) {
			t.Fatalf("v2 interval %d ms: got %v, want ErrIntervalRange", ms, err)
		}
	}

	// N10: 4095 cs (40950 ms) round-trips with no rounding drift.
	adv := advV3v4(t)
	adv.AdverIntervalMS = 40950
	buf := make([]byte, MaxLenV3v4)
	n := adv.WriteTo(buf, 0)
	FillChecksum(buf, 0, n, addr(t, "192.0.2.251"), MulticastV4, ChecksumRFC9568)
	got, err := Decode(buf[:n], metaV3v4(t), lookupConst(VersionV3, 40950))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.AdverIntervalMS != 40950 {
		t.Fatalf("interval drift: got %d ms, want 40950", got.AdverIntervalMS)
	}
}

// -----------------------------------------------------------------------------
// Zero-allocation decode (AC-8) and constants
// -----------------------------------------------------------------------------

// VALIDATES: AC-8 -- happy-path Decode of each golden vector allocates 0 heap
// objects, including VIPAt access (lazy view, netip value conversions, A-2).
// PREVENTS: hidden per-packet allocation (materialized VIP slice, error wrap).
func TestDecodeZeroAlloc(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		meta RxMeta
		look Lookup
	}{
		{"v2", mustHex(t, goldenV2Hex), metaV2(t), lookupConst(VersionV2, 1000)},
		{"v3v4", mustHex(t, goldenV3v4Hex), metaV3v4(t), lookupConst(VersionV3, 1000)},
		{"v3v6", mustHex(t, goldenV3v6Hex), metaV3v6(t), lookupConst(VersionV3, 1000)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			avg := testing.AllocsPerRun(200, func() {
				adv, err := Decode(tc.data, tc.meta, tc.look)
				if err != nil {
					t.Fatalf("Decode: %v", err)
				}
				if adv.VIPAt(0).BitLen() == 0 {
					t.Fatal("VIPAt(0) empty")
				}
			})
			if avg != 0 {
				t.Fatalf("Decode allocated %v objects/run, want 0", avg)
			}
		})
	}
}

// VALIDATES: exported protocol constants match the RFCs and MaxLen sizing.
// PREVENTS: silent drift of the sizes the transport (child 4) depends on.
func TestConstants(t *testing.T) {
	if MaxLenV2 != 80 || MaxLenV3v4 != 72 || MaxLenV3v6 != 264 {
		t.Fatalf("MaxLen drift: v2=%d v3v4=%d v3v6=%d", MaxLenV2, MaxLenV3v4, MaxLenV3v6)
	}
	if ProtoNumber != 112 {
		t.Fatalf("ProtoNumber = %d, want 112", ProtoNumber)
	}
	if MulticastV4 != addr(t, "224.0.0.18") || MulticastV6 != addr(t, "ff02::12") {
		t.Fatalf("multicast drift: v4=%v v6=%v", MulticastV4, MulticastV6)
	}
	// RFC requirement: RFC3768-7.2-2 positive -- the virtual-router MAC is 00-00-5E-00-01-{VRID}, the L2 source identity the tx socket egresses by binding to this vMAC macvlan (packet.go:97; backend_linux.go:133).
	// RFC requirement: RFC9568-7.2-2 positive -- the Virtual Router MAC is 00-00-5E-00-01-{VRID} for IPv4 and 00-00-5E-00-02-{VRID} for IPv6, and it is the L2 source identity the tx socket egresses by binding to that vMAC macvlan (packet.go:97; backend_linux.go:133,179).
	// RFC requirement: RFC5798-7.2-2 positive -- the virtual router MAC is 00-00-5E-00-01-{VRID} for IPv4 and 00-00-5E-00-02-{VRID} for IPv6, and it is the source link-layer identity the tx socket egresses by binding to that vMAC macvlan (packet.go:97; backend_linux.go:133,179).
	mac := VirtualMAC(V4, 10)
	if mac != [6]byte{0x00, 0x00, 0x5e, 0x00, 0x01, 0x0a} {
		t.Fatalf("v4 VirtualMAC = % x", mac)
	}
	mac6 := VirtualMAC(V6, 10)
	if mac6 != [6]byte{0x00, 0x00, 0x5e, 0x00, 0x02, 0x0a} {
		t.Fatalf("v6 VirtualMAC = % x", mac6)
	}
}

// -----------------------------------------------------------------------------
// Benchmarks (allocation regression guard)
// -----------------------------------------------------------------------------

func BenchmarkDecode(b *testing.B) {
	data := mustHex(b, goldenV3v4Hex)
	meta := metaV3v4(b)
	look := lookupConst(VersionV3, 1000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		adv, err := Decode(data, meta, look)
		if err != nil {
			b.Fatal(err)
		}
		_ = adv.VIPAt(0)
	}
}

func BenchmarkEncode(b *testing.B) {
	adv := advV3v4(b)
	buf := make([]byte, MaxLenV3v4)
	src := addr(b, "192.0.2.251")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		n := adv.WriteTo(buf, 0)
		FillChecksum(buf, 0, n, src, MulticastV4, ChecksumRFC9568)
	}
}

// itoa is a tiny allocation-tolerant helper for subtest names.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
