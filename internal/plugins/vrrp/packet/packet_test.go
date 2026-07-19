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
// message-only form, accepted on rx. G2c is the RFC 5798 pseudo-header form
// (0xDEFB is the pseudo-header sum for src 192.0.2.251 -> dst 224.0.0.18): it is
// what ze now TRANSMITS for v3/IPv4, because keepalived and the deployed base
// require it (see checksum.go FillChecksum), and it is also accepted on rx.
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
		return Local{Version: version, AdverIntervalMS: ms}, true
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
	FillChecksum(buf, 0, n, addr(t, "192.0.2.251"), MulticastV4)
	want := mustHex(t, goldenV2Hex)
	if n != len(want) {
		t.Fatalf("wrote %d bytes, want %d", n, len(want))
	}
	if !bytes.Equal(buf[:n], want) {
		t.Fatalf("v2 encode mismatch:\ngot:  % x\nwant: % x", buf[:n], want)
	}
}

// VALIDATES: AC-2 -- v3 IPv4 encode produces exact G2c bytes (reserved nibble 0,
// 12-bit interval = 100 cs, RFC 5798 pseudo-header checksum 0xDEFB -- ze's tx
// form for keepalived interop).
// PREVENTS: reserve-nibble leak, cs/ms confusion, checksum-form regression.
func TestEncodeGoldenV3IPv4(t *testing.T) {
	adv := advV3v4(t)
	buf := make([]byte, MaxLenV3v4)
	n := adv.WriteTo(buf, 0)
	FillChecksum(buf, 0, n, addr(t, "192.0.2.251"), MulticastV4)
	// Expect the pseudo-header form (G2c), not message-only (G2): ze transmits
	// the RFC 5798 pseudo-header checksum for v3/IPv4 so keepalived and the
	// deployed base accept it (checksum.go FillChecksum). 0xDEFB is the
	// pseudo-header sum for this exact src (192.0.2.251) and dst (224.0.0.18).
	want := mustHex(t, goldenV3v4CompatHex)
	if n != len(want) {
		t.Fatalf("wrote %d bytes, want %d", n, len(want))
	}
	if !bytes.Equal(buf[:n], want) {
		t.Fatalf("v3 IPv4 encode mismatch:\ngot:  % x\nwant: % x", buf[:n], want)
	}
}

// VALIDATES: AC-2 -- v3 IPv6 encode produces exact G3 bytes (RFC 8200
// pseudo-header checksum 0x3F5D).
// PREVENTS: wrong pseudo-header composition, wrong upper-layer length.
func TestEncodeGoldenV3IPv6(t *testing.T) {
	adv := advV3v6(t)
	buf := make([]byte, MaxLenV3v6)
	n := adv.WriteTo(buf, 0)
	FillChecksum(buf, 0, n, addr(t, "fe80::c8"), MulticastV6)
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
	FillChecksum(buf, prefix, n, addr(t, "192.0.2.251"), MulticastV4)
	for i := range prefix {
		if buf[i] != 0xAA {
			t.Fatalf("byte %d clobbered: %#x", i, buf[i])
		}
	}
	// Pseudo-header form (G2c), the v3/IPv4 tx checksum -- see TestEncodeGoldenV3IPv4.
	want := mustHex(t, goldenV3v4CompatHex)
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
					FillChecksum(buf, 0, n, fam.src, fam.meta.Dst)

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
					if got.MsgOnlyChecksum {
						t.Fatal("ze's own tx uses the pseudo-header form, so a round-tripped packet must not set MsgOnlyChecksum")
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
	FillChecksum(buf, 0, n, addr(t, "192.0.2.251"), MulticastV4)
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
		FillChecksum(buf, 0, n, src, MulticastV4)
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
