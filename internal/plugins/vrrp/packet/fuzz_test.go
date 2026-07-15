package packet

import (
	"net/netip"
	"testing"
)

// FuzzDecode feeds arbitrary bytes into Decode across all three receive paths
// (v2/IPv4, v3/IPv4, v3/IPv6, selected by sel%3). Decode must never panic, and
// every ACCEPTED packet must satisfy the ordered-ladder invariants and
// round-trip (for encodable, canonical-checksum packets). Seed corpus = the
// golden vectors plus one mutation per ladder row (bfd fuzz_test.go:104 model).
//
// VALIDATES: codec robustness under adversarial input (AC-7).
// PREVENTS: panic on malformed input, or a packet escaping the ladder.
func FuzzDecode(f *testing.F) {
	mutate := func(in []byte, fn func([]byte)) []byte {
		c := append([]byte{}, in...)
		fn(c)
		return c
	}
	g := mustHex(f, goldenV3v4Hex)
	v2 := mustHex(f, goldenV2Hex)

	seeds := []struct {
		sel  byte
		data []byte
	}{
		{0, v2},
		{1, g},
		{1, mustHex(f, goldenV3v4CompatHex)},
		{2, mustHex(f, goldenV3v6Hex)},
		{1, g[:4]}, // row 1 truncated
		{1, mutate(g, func(b []byte) { b[0] = 0x51 })},              // row 2 bad version
		{1, mutate(g, func(b []byte) { b[0] = 0x32 })},              // row 3 bad type
		{1, mutate(g, func(b []byte) { b[3] = 0 })},                 // row 8 count zero
		{1, mutate(g, func(b []byte) { b[4], b[5] = 0, 0 })},        // row 11 interval zero
		{1, append(append([]byte{}, g...), 0, 0, 0, 0, 0, 0, 0, 0)}, // row 9 length
		{0, mutate(v2, func(b []byte) { b[4] = 1 })},                // row 10 auth type
	}
	for _, s := range seeds {
		f.Add(s.sel, s.data)
	}

	src4 := netip.AddrFrom4([4]byte{192, 0, 2, 251})
	src6 := netip.AddrFrom16([16]byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xc8})

	f.Fuzz(func(t *testing.T, sel byte, data []byte) {
		var meta RxMeta
		var localVer uint8
		switch sel % 3 {
		case 0:
			meta = RxMeta{TTL: 255, Src: src4, Dst: MulticastV4, Family: V4}
			localVer = VersionV2
		case 1:
			meta = RxMeta{TTL: 255, Src: src4, Dst: MulticastV4, Family: V4}
			localVer = VersionV3
		default:
			meta = RxMeta{TTL: 255, Src: src6, Dst: MulticastV6, Family: V6}
			localVer = VersionV3
		}
		lookup := func(uint8) (Local, bool) { return Local{Version: localVer, AdverIntervalMS: 1000}, true }

		adv, err := Decode(data, meta, lookup)
		if err != nil {
			return
		}

		// Accepted-packet invariants (must mirror the ladder).
		if len(data) < HeaderLen {
			t.Fatalf("accepted packet shorter than %d bytes", HeaderLen)
		}
		if adv.Version != localVer {
			t.Fatalf("accepted version %d, want %d", adv.Version, localVer)
		}
		if adv.Count < 1 {
			t.Fatal("accepted count 0")
		}
		addrSize := 4
		if meta.Family == V6 {
			addrSize = 16
		}
		wantLen := HeaderLen + int(adv.Count)*addrSize
		if localVer == VersionV2 {
			wantLen += 8 // Authentication Data trailer
		}
		if len(data) != wantLen {
			t.Fatalf("accepted len %d, want exact %d", len(data), wantLen)
		}
		if adv.MsgOnlyChecksum && (localVer != VersionV3 || meta.Family != V4) {
			t.Fatal("MsgOnlyChecksum set outside v3/IPv4")
		}
		for i := range adv.VIPCount() {
			if !adv.VIPAt(i).IsValid() {
				t.Fatalf("VIPAt(%d) invalid within bounds", i)
			}
		}
		if adv.VIPAt(adv.VIPCount()).IsValid() {
			t.Fatal("VIPAt past bounds returned a valid address")
		}

		// Round-trip only encodable packets that used the canonical (pseudo-header)
		// checksum form, since re-encoding produces that form.
		if adv.VIPCount() <= MaxVIPs && !adv.MsgOnlyChecksum {
			re := Advertisement{Version: adv.Version, Family: adv.Family, VRID: adv.VRID,
				Priority: adv.Priority, AdverIntervalMS: adv.AdverIntervalMS, VIPs: adv.AppendVIPs(nil)}
			buf := make([]byte, MaxLenV3v6)
			n := re.WriteTo(buf, 0)
			FillChecksum(buf, 0, n, meta.Src, meta.Dst)
			got, err := Decode(buf[:n], meta, lookup)
			if err != nil {
				t.Fatalf("round-trip decode failed: %v", err)
			}
			if got.Version != adv.Version || got.VRID != adv.VRID || got.Priority != adv.Priority ||
				got.Count != adv.Count || got.AdverIntervalMS != adv.AdverIntervalMS {
				t.Fatalf("round-trip mismatch:\ngot:  %+v\nwant: %+v", got, adv)
			}
		}
	})
}
