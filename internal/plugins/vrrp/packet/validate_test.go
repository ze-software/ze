package packet

import (
	"errors"
	"net/netip"
	"testing"
)

func checkFields(t *testing.T, adv Advertisement, ver, fam, vrid, prio uint8, count int, ms uint32) {
	t.Helper()
	if adv.Version != ver || adv.Family != fam || adv.VRID != vrid || adv.Priority != prio ||
		int(adv.Count) != count || adv.AdverIntervalMS != ms {
		t.Fatalf("field mismatch: got %+v", adv)
	}
}

// encodeValid encodes adv into a fresh buffer with a valid checksum.
func encodeValid(t testing.TB, adv Advertisement, src, dst netip.Addr) []byte {
	t.Helper()
	buf := make([]byte, MaxLenV3v6)
	n := adv.WriteTo(buf, 0)
	FillChecksum(buf, 0, n, src, dst)
	return buf[:n]
}

// reChecksum zeroes and recomputes the checksum in place after a byte mutation.
func reChecksum(buf []byte, src, dst netip.Addr) {
	buf[6], buf[7] = 0, 0
	FillChecksum(buf, 0, len(buf), src, dst)
}

// -----------------------------------------------------------------------------
// Decode golden tests (AC-3)
// -----------------------------------------------------------------------------

// VALIDATES: AC-3 -- v2 golden decodes field-equal, interval 1000 ms.
//
// A conformant v2 advertisement passes every §7.1 receive-validation MUST, so this
// golden decode is the positive case for each ladder row (the matching negatives
// live in TestValidationOrder, TestNegativeReferenceBugs, TestDecodeV2IntervalMismatchDiscard
// and TestDecodeV2ChecksumCorrupt):
// RFC requirement: RFC3768-7.1-1 positive -- wire version 2 accepted (Decode validate.go:133,149)
// RFC requirement: RFC3768-7.1-2 positive -- complete packet incl the 8-byte auth trailer accepted (validate.go:179-185)
// RFC requirement: RFC3768-7.1-3 positive -- a valid v2 checksum verifies (verifyReceived checksum.go:132)
// RFC requirement: RFC3768-7.1-4 positive -- the configured VRID resolves via lookup (validate.go:143)
// RFC requirement: RFC3768-7.1-5 positive -- Auth Type 0 (No Authentication) matches and passes (validate.go:191)
// RFC requirement: RFC3768-7.1-8 positive -- Adver Interval equal to the local config accepted (validate.go:205)
// RFC requirement: RFC3768-5.2.3-2 positive -- TTL 255 accepted (validate.go:163)
// RFC requirement: RFC3768-5.3.2-1 positive -- Type 1 ADVERTISEMENT accepted (validate.go:138)
// RFC requirement: RFC3768-5.3.6-1 positive -- Auth Type 0 accepted; a non-zero type would discard (validate.go:191).
func TestDecodeGoldenV2(t *testing.T) {
	adv, err := Decode(mustHex(t, goldenV2Hex), metaV2(t), lookupConst(VersionV2, 1000))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	checkFields(t, adv, VersionV2, V4, 10, 200, 2, 1000)
	if adv.VIPAt(0) != addr(t, "192.0.2.1") || adv.VIPAt(1) != addr(t, "192.0.2.2") {
		t.Fatalf("VIPs: %v %v", adv.VIPAt(0), adv.VIPAt(1))
	}
	if adv.MsgOnlyChecksum {
		t.Fatal("v2 must not set MsgOnlyChecksum")
	}
}

// VALIDATES: AC-3 -- v3 IPv4 golden decodes field-equal, interval 1000 ms. Uses
// the canonical pseudo-header golden (G2c), the form ze transmits, so the decode
// is not flagged; the message-only form's flagging is covered by
// TestDecodeV3IPv4MsgOnlyChecksumCompat.
func TestDecodeGoldenV3IPv4(t *testing.T) {
	adv, err := Decode(mustHex(t, goldenV3v4CompatHex), metaV3v4(t), lookupConst(VersionV3, 1000))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	checkFields(t, adv, VersionV3, V4, 10, 200, 2, 1000)
	if adv.VIPAt(0) != addr(t, "192.0.2.1") || adv.VIPAt(1) != addr(t, "192.0.2.2") {
		t.Fatalf("VIPs: %v %v", adv.VIPAt(0), adv.VIPAt(1))
	}
	if adv.MsgOnlyChecksum {
		t.Fatal("the pseudo-header golden is ze's canonical form; it must not set MsgOnlyChecksum")
	}
}

// VALIDATES: AC-3 -- v3 IPv6 golden decodes field-equal, interval 1000 ms,
// first VIP link-local.
func TestDecodeGoldenV3IPv6(t *testing.T) {
	adv, err := Decode(mustHex(t, goldenV3v6Hex), metaV3v6(t), lookupConst(VersionV3, 1000))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	checkFields(t, adv, VersionV3, V6, 10, 100, 2, 1000)
	if adv.VIPAt(0) != addr(t, "fe80::1") || adv.VIPAt(1) != addr(t, "2001:db8::1") {
		t.Fatalf("VIPs: %v %v", adv.VIPAt(0), adv.VIPAt(1))
	}
}

// -----------------------------------------------------------------------------
// Ordered validation ladder (AC-4, TestValidationOrder): earlier rows win
// -----------------------------------------------------------------------------

// VALIDATES: AC-4 -- when multiple ladder rows are violated, the earliest row's
// typed error is returned, for every adjacent pair of the 13-row ladder.
// PREVENTS: reordering that would surface a mystery error (e.g. checksum for a
// spurious auth trailer, RFC 9568 Section 7.1 ordering rationale).
func TestValidationOrder(t *testing.T) {
	src4 := addr(t, "192.0.2.251")
	dst4 := MulticastV4
	src6 := addr(t, "fe80::c8")
	dst6 := MulticastV6

	t.Run("1<2 truncated beats version", func(t *testing.T) {
		// 4-byte packet: too short AND version nibble 5.
		_, err := Decode([]byte{0x51, 0, 0, 0}, metaV3v4(t), lookupConst(VersionV3, 1000))
		if !errors.Is(err, ErrTruncated) {
			t.Fatalf("got %v, want ErrTruncated", err)
		}
	})

	t.Run("2<3 version beats type", func(t *testing.T) {
		b := encodeValid(t, advV3v4(t), src4, dst4)
		b[0] = 0x52 // version 5 (bad) + type 2 (bad)
		_, err := Decode(b, metaV3v4(t), lookupConst(VersionV3, 1000))
		if !errors.Is(err, ErrVersion) {
			t.Fatalf("got %v, want ErrVersion", err)
		}
	})

	t.Run("3<4 type beats vrid", func(t *testing.T) {
		// RFC requirement: RFC3768-5.3.2-1 negative -- a non-ADVERTISEMENT type (2) is discarded with ErrType (validate.go:138)
		b := encodeValid(t, advV3v4(t), src4, dst4)
		b[0] = 0x32 // version 3 + type 2 (bad)
		_, err := Decode(b, metaV3v4(t), func(uint8) (Local, bool) { return Local{}, false })
		if !errors.Is(err, ErrType) {
			t.Fatalf("got %v, want ErrType", err)
		}
	})

	t.Run("4<5 vrid beats version-match", func(t *testing.T) {
		// RFC requirement: RFC3768-7.1-4 negative -- a VRID not configured on the interface is discarded with ErrUnknownVRID (validate.go:143)
		b := encodeValid(t, advV3v4(t), src4, dst4)
		_, err := Decode(b, metaV3v4(t), func(uint8) (Local, bool) { return Local{Version: VersionV2}, false })
		if !errors.Is(err, ErrUnknownVRID) {
			t.Fatalf("got %v, want ErrUnknownVRID", err)
		}
	})

	t.Run("5<6 version-match beats checksum", func(t *testing.T) {
		b := encodeValid(t, advV3v4(t), src4, dst4)
		b[6], b[7] = 0, 0 // corrupt checksum
		// local version 2 mismatches wire version 3.
		_, err := Decode(b, metaV3v4(t), lookupConst(VersionV2, 1000))
		if !errors.Is(err, ErrVersion) {
			t.Fatalf("got %v, want ErrVersion", err)
		}
	})

	t.Run("6<7 checksum beats ttl", func(t *testing.T) {
		b := encodeValid(t, advV3v4(t), src4, dst4)
		b[6], b[7] = 0, 0 // corrupt checksum
		meta := metaV3v4(t)
		meta.TTL = 64
		_, err := Decode(b, meta, lookupConst(VersionV3, 1000))
		if !errors.Is(err, ErrChecksum) {
			t.Fatalf("got %v, want ErrChecksum", err)
		}
	})

	t.Run("7<8 ttl beats count-zero", func(t *testing.T) {
		adv := advV3v4(t)
		adv.VIPs = nil // count 0
		b := encodeValid(t, adv, src4, dst4)
		meta := metaV3v4(t)
		meta.TTL = 64
		_, err := Decode(b, meta, lookupConst(VersionV3, 1000))
		if !errors.Is(err, ErrTTL) {
			t.Fatalf("got %v, want ErrTTL", err)
		}
	})

	t.Run("8<9 count-zero beats length", func(t *testing.T) {
		adv := advV3v4(t)
		adv.VIPs = nil
		b := encodeValid(t, adv, src4, dst4) // 8-byte, count 0
		b = append(b, 0, 0, 0, 0)            // trailing zeros -> length also wrong
		reChecksum(b, src4, dst4)
		_, err := Decode(b, metaV3v4(t), lookupConst(VersionV3, 1000))
		if !errors.Is(err, ErrCountZero) {
			t.Fatalf("got %v, want ErrCountZero", err)
		}
	})

	t.Run("9<10 length beats auth-type", func(t *testing.T) {
		// RFC requirement: RFC3768-7.1-2 negative -- a v2 packet whose length does not match Count IP Addrs plus the 8-byte auth trailer is discarded as incomplete with ErrLength (validate.go:183)
		b := encodeValid(t, advV2(t), src4, dst4) // v2, 24 bytes
		b[3] = 3                                  // count lies (says 3, only 2 present)
		b[4] = 1                                  // auth type 1 (bad)
		reChecksum(b, src4, dst4)
		_, err := Decode(b, metaV2(t), lookupConst(VersionV2, 1000))
		if !errors.Is(err, ErrLength) {
			t.Fatalf("got %v, want ErrLength", err)
		}
	})

	t.Run("10<11 auth-type beats interval-zero", func(t *testing.T) {
		b := encodeValid(t, advV2(t), src4, dst4)
		b[4] = 1 // auth type 1 (bad)
		b[5] = 0 // interval 0 (bad)
		reChecksum(b, src4, dst4)
		_, err := Decode(b, metaV2(t), lookupConst(VersionV2, 1000))
		if !errors.Is(err, ErrAuthType) {
			t.Fatalf("got %v, want ErrAuthType", err)
		}
	})

	t.Run("11<12 interval-zero beats mismatch", func(t *testing.T) {
		b := encodeValid(t, advV2(t), src4, dst4)
		b[5] = 0 // interval 0 (bad) -- would also mismatch local 1000
		reChecksum(b, src4, dst4)
		_, err := Decode(b, metaV2(t), lookupConst(VersionV2, 1000))
		if !errors.Is(err, ErrIntervalZero) {
			t.Fatalf("got %v, want ErrIntervalZero", err)
		}
	})

	t.Run("12 v2 interval mismatch", func(t *testing.T) {
		// RFC requirement: RFC3768-7.1-8 negative -- a v2 Adver Interval differing from the local config is discarded with ErrV2IntervalMismatch (validate.go:205)
		b := encodeValid(t, advV2(t), src4, dst4) // interval 1 s = 1000 ms
		_, err := Decode(b, metaV2(t), lookupConst(VersionV2, 2000))
		if !errors.Is(err, ErrV2IntervalMismatch) {
			t.Fatalf("got %v, want ErrV2IntervalMismatch", err)
		}
	})

	t.Run("13 v3v6 first vip not link-local", func(t *testing.T) {
		adv := advV3v6(t)
		adv.VIPs = []netip.Addr{addr(t, "2001:db8::1"), addr(t, "2001:db8::2")} // first not link-local
		b := encodeValid(t, adv, src6, dst6)
		_, err := Decode(b, metaV3v6(t), lookupConst(VersionV3, 1000))
		if !errors.Is(err, ErrFirstNotLinkLocal) {
			t.Fatalf("got %v, want ErrFirstNotLinkLocal", err)
		}
	})
}

// -----------------------------------------------------------------------------
// v3 vs v2 interval semantics (AC-5)
// -----------------------------------------------------------------------------

// VALIDATES: AC-5 -- a v3 advert whose interval differs from local config
// decodes cleanly and surfaces the received interval for FSM adoption (holo/
// uvrrpd adopt-bug prevented).
func TestDecodeV3IntervalMismatchNotError(t *testing.T) {
	adv := advV3v4(t)
	adv.AdverIntervalMS = 3000 // 300 cs, differs from local 1000
	b := encodeValid(t, adv, addr(t, "192.0.2.251"), MulticastV4)
	got, err := Decode(b, metaV3v4(t), lookupConst(VersionV3, 1000))
	if err != nil {
		t.Fatalf("v3 mismatch must not error: %v", err)
	}
	if got.AdverIntervalMS != 3000 {
		t.Fatalf("interval not surfaced: got %d ms, want 3000", got.AdverIntervalMS)
	}
}

// VALIDATES: AC-5 -- a v2 advert with mismatched interval is discarded.
func TestDecodeV2IntervalMismatchDiscard(t *testing.T) {
	// RFC requirement: RFC3768-7.1-8 negative -- a v2 Adver Interval differing from the local config discards the packet (validate.go:205).
	adv := advV2(t)
	adv.AdverIntervalMS = 2000 // 2 s, differs from local 1000
	b := encodeValid(t, adv, addr(t, "192.0.2.251"), MulticastV4)
	_, err := Decode(b, metaV2(t), lookupConst(VersionV2, 1000))
	if !errors.Is(err, ErrV2IntervalMismatch) {
		t.Fatalf("got %v, want ErrV2IntervalMismatch", err)
	}
}

// TestDecodeV2ChecksumCorrupt is the negative for the v2 receive-checksum MUST: a
// v2 advertisement whose checksum does not verify is discarded.
func TestDecodeV2ChecksumCorrupt(t *testing.T) {
	// RFC requirement: RFC3768-7.1-3 negative -- a v2 packet with a corrupted checksum is discarded with ErrChecksum (verifyReceived checksum.go:132, Decode validate.go:157).
	b := mustHex(t, goldenV2Hex)
	b[7] ^= 0xff // flip the checksum low byte so the one's-complement sum no longer folds to all-ones
	if _, err := Decode(b, metaV2(t), lookupConst(VersionV2, 1000)); !errors.Is(err, ErrChecksum) {
		t.Fatalf("corrupted v2 checksum: got %v, want ErrChecksum", err)
	}
}

// -----------------------------------------------------------------------------
// v3/IPv4 legacy RFC 5798 dual-accept (AC-10, N1, N1b)
// -----------------------------------------------------------------------------

// VALIDATES: AC-10 -- v3/IPv4 dual-accept. G2c (RFC 5798 pseudo-header sum
// 0xDEFB) is the CANONICAL form ze and the deployed base send: accepted, NOT
// flagged. G2 (RFC 9568 message-only sum 0x828A) is accepted but flagged
// MsgOnlyChecksum so an operator sees a strict-RFC-9568 peer. A payload failing
// BOTH sums yields ErrChecksum.
func TestDecodeV3IPv4MsgOnlyChecksumCompat(t *testing.T) {
	// G2c (pseudo-header, src 192.0.2.251, dst 224.0.0.18) is the canonical form.
	adv, err := Decode(mustHex(t, goldenV3v4CompatHex), metaV3v4(t), lookupConst(VersionV3, 1000))
	if err != nil {
		t.Fatalf("G2c (pseudo-header) must be accepted: %v", err)
	}
	if adv.MsgOnlyChecksum {
		t.Fatal("G2c is the pseudo-header form ze sends; it must NOT set MsgOnlyChecksum")
	}

	// G2 (message-only) is accepted, and flagged as the strict-RFC-9568 form.
	adv2, err := Decode(mustHex(t, goldenV3v4Hex), metaV3v4(t), lookupConst(VersionV3, 1000))
	if err != nil {
		t.Fatalf("G2 (message-only) must be accepted: %v", err)
	}
	if !adv2.MsgOnlyChecksum {
		t.Fatal("G2 is the message-only form; it must set MsgOnlyChecksum (checksum-rfc9568-message-only)")
	}

	// A payload failing both the pseudo-header and message-only sums -> ErrChecksum.
	bad := mustHex(t, goldenV3v4Hex)
	bad[6], bad[7] = 0, 0
	_, err = Decode(bad, metaV3v4(t), lookupConst(VersionV3, 1000))
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("both-fail payload: got %v, want ErrChecksum", err)
	}
}

// -----------------------------------------------------------------------------
// IPv4 header strip helper (AC-6, N3)
// -----------------------------------------------------------------------------

func ipv4Datagram(ihl int, ttl byte, payload []byte) []byte {
	hdrLen := ihl * 4
	dg := make([]byte, hdrLen+len(payload))
	dg[0] = 0x40 | byte(ihl&0x0F)
	dg[8] = ttl
	copy(dg[12:16], []byte{192, 0, 2, 251})
	copy(dg[16:20], []byte{224, 0, 0, 18})
	copy(dg[hdrLen:], payload)
	return dg
}

// VALIDATES: AC-6 -- StripIPv4Header honors IHL 5/6/15 (variable options),
// extracts TTL/src/dst, and rejects IHL<5 and header-longer-than-datagram; the
// stripped payload of an options-bearing datagram decodes (holo fixed-20-byte
// strip bug prevented, N3).
func TestStripIPv4HeaderIHL(t *testing.T) {
	golden := mustHex(t, goldenV3v4Hex)
	for _, ihl := range []int{5, 6, 15} {
		t.Run("ihl="+itoa(ihl), func(t *testing.T) {
			dg := ipv4Datagram(ihl, 255, golden)
			payload, meta, err := StripIPv4Header(dg)
			if err != nil {
				t.Fatalf("strip: %v", err)
			}
			if meta.TTL != 255 || meta.Family != V4 ||
				meta.Src != addr(t, "192.0.2.251") || meta.Dst != addr(t, "224.0.0.18") {
				t.Fatalf("meta wrong: %+v", meta)
			}
			if len(payload) != len(golden) {
				t.Fatalf("payload len %d, want %d (IHL not honored)", len(payload), len(golden))
			}
			// N3: the stripped payload decodes cleanly.
			if _, err := Decode(payload, meta, lookupConst(VersionV3, 1000)); err != nil {
				t.Fatalf("decode stripped payload: %v", err)
			}
		})
	}

	// IHL 4 (< minimum) -> ErrIPv4BadIHL.
	dg := ipv4Datagram(5, 255, golden)
	dg[0] = 0x44 // version 4, IHL 4
	if _, _, err := StripIPv4Header(dg); !errors.Is(err, ErrIPv4BadIHL) {
		t.Fatalf("IHL 4: got %v, want ErrIPv4BadIHL", err)
	}

	// Header claims 60 bytes but datagram is only 30 -> ErrIPv4HeaderShort.
	short := make([]byte, 30)
	short[0] = 0x4F // version 4, IHL 15 (60 bytes)
	if _, _, err := StripIPv4Header(short); !errors.Is(err, ErrIPv4HeaderShort) {
		t.Fatalf("short header: got %v, want ErrIPv4HeaderShort", err)
	}

	// Datagram shorter than the minimum 20-byte header -> ErrIPv4HeaderShort.
	if _, _, err := StripIPv4Header(make([]byte, 10)); !errors.Is(err, ErrIPv4HeaderShort) {
		t.Fatalf("tiny datagram: got %v, want ErrIPv4HeaderShort", err)
	}
}

// -----------------------------------------------------------------------------
// Error -> reason mapping (TestErrorReasonMapping)
// -----------------------------------------------------------------------------

// VALIDATES: Reason() is total and injective over the receive-validation
// taxonomy (ip-header intentionally shared by the two strip errors); the
// accepted-outcome label checksum-rfc9568-message-only and the engine-raised
// address-list label are present and collision-free; encode-side errors and nil
// are deliberately NOT rx reasons (mapping closed over the taxonomy).
func TestErrorReasonMapping(t *testing.T) {
	table := []struct {
		err    error
		reason string
	}{
		{ErrTruncated, "truncated"},
		{ErrVersion, "version"},
		{ErrType, "type"},
		{ErrUnknownVRID, "vrid"},
		{ErrChecksum, "checksum"},
		{ErrTTL, "ttl"},
		{ErrCountZero, "count-zero"},
		{ErrLength, "length"},
		{ErrAuthType, "auth-type"},
		{ErrIntervalZero, "interval-zero"},
		{ErrV2IntervalMismatch, "interval-mismatch"},
		{ErrFirstNotLinkLocal, "linklocal"},
		{ErrIPv4HeaderShort, "ip-header"},
		{ErrIPv4BadIHL, "ip-header"},
	}
	seen := map[string]int{}
	for _, row := range table {
		got := Reason(row.err)
		if got != row.reason {
			t.Errorf("Reason(%v) = %q, want %q", row.err, got, row.reason)
		}
		if got == "" {
			t.Errorf("Reason(%v) is empty (mapping not total)", row.err)
		}
		seen[got]++
	}
	for label, n := range seen {
		if n > 1 && label != "ip-header" {
			t.Errorf("label %q maps from %d errors (not injective)", label, n)
		}
	}

	if ReasonMsgOnlyChecksum != "checksum-rfc9568-message-only" {
		t.Errorf("ReasonMsgOnlyChecksum = %q", ReasonMsgOnlyChecksum)
	}
	if ReasonAddressList != "address-list" {
		t.Errorf("ReasonAddressList = %q", ReasonAddressList)
	}
	if _, dup := seen[ReasonMsgOnlyChecksum]; dup {
		t.Error("compat label collides with an error reason")
	}
	if _, dup := seen[ReasonAddressList]; dup {
		t.Error("address-list label collides with an error reason")
	}

	for _, e := range []error{nil, ErrIntervalRange, ErrCountRange, ErrVRIDRange, errors.New("other")} {
		if r := Reason(e); r != "" {
			t.Errorf("Reason(%v) = %q, want empty (not an rx reason)", e, r)
		}
	}
}

// -----------------------------------------------------------------------------
// Negative tests from verified reference-implementation bugs (N1-N10)
// -----------------------------------------------------------------------------

// VALIDATES: every verified holo-vrrp / uvrrpd bug from the spec's negative
// table is provably not replicated (umbrella R-1).
func TestNegativeReferenceBugs(t *testing.T) {
	src4 := addr(t, "192.0.2.251")
	dst4 := MulticastV4
	src6 := addr(t, "fe80::c8")
	dst6 := MulticastV6

	t.Run("N1 pseudo-header-checksum-accepted", func(t *testing.T) {
		// The RFC 5798 pseudo-header form (G2c) must be accepted -- a reference
		// implementation that rejected it could not talk to keepalived. It is
		// ze's own tx form, so it is the canonical, unflagged case.
		adv, err := Decode(mustHex(t, goldenV3v4CompatHex), metaV3v4(t), lookupConst(VersionV3, 1000))
		if err != nil || adv.MsgOnlyChecksum {
			t.Fatalf("G2c: err=%v msgOnly=%v, want accept+canonical", err, adv.MsgOnlyChecksum)
		}
	})

	t.Run("N1b both-sums-fail-rejected", func(t *testing.T) {
		bad := mustHex(t, goldenV3v4Hex)
		bad[6], bad[7] = 0, 0
		if _, err := Decode(bad, metaV3v4(t), lookupConst(VersionV3, 1000)); !errors.Is(err, ErrChecksum) {
			t.Fatalf("got %v, want ErrChecksum", err)
		}
	})

	t.Run("N2 v3-with-v2-auth-trailer", func(t *testing.T) {
		// Append an 8-byte zero trailer: checksum still folds (zeros), so the
		// LENGTH check must fire, not checksum.
		b := mustHex(t, goldenV3v4Hex)
		b = append(b, 0, 0, 0, 0, 0, 0, 0, 0)
		if _, err := Decode(b, metaV3v4(t), lookupConst(VersionV3, 1000)); !errors.Is(err, ErrLength) {
			t.Fatalf("got %v, want ErrLength", err)
		}
	})

	t.Run("N3 ipv4-options-strip", func(t *testing.T) {
		dg := ipv4Datagram(6, 255, mustHex(t, goldenV3v4Hex))
		payload, meta, err := StripIPv4Header(dg)
		if err != nil {
			t.Fatalf("strip: %v", err)
		}
		if _, err := Decode(payload, meta, lookupConst(VersionV3, 1000)); err != nil {
			t.Fatalf("decode after IHL=6 strip: %v", err)
		}
	})

	t.Run("N4 ttl-not-255", func(t *testing.T) {
		// RFC requirement: RFC3768-5.2.3-2 negative -- a received packet with TTL != 255 is discarded with ErrTTL (validate.go:163)
		meta := metaV3v4(t)
		meta.TTL = 64
		if _, err := Decode(mustHex(t, goldenV3v4Hex), meta, lookupConst(VersionV3, 1000)); !errors.Is(err, ErrTTL) {
			t.Fatalf("got %v, want ErrTTL", err)
		}
	})

	t.Run("N5 count-zero", func(t *testing.T) {
		adv := advV3v4(t)
		adv.VIPs = nil
		b := encodeValid(t, adv, src4, dst4) // exact length for count 0 (8 bytes)
		if _, err := Decode(b, metaV3v4(t), lookupConst(VersionV3, 1000)); !errors.Is(err, ErrCountZero) {
			t.Fatalf("got %v, want ErrCountZero", err)
		}
	})

	t.Run("N6 v3-interval-zero", func(t *testing.T) {
		b := encodeValid(t, advV3v4(t), src4, dst4)
		b[4], b[5] = 0, 0 // 12-bit interval = 0
		reChecksum(b, src4, dst4)
		if _, err := Decode(b, metaV3v4(t), lookupConst(VersionV3, 1000)); !errors.Is(err, ErrIntervalZero) {
			t.Fatalf("got %v, want ErrIntervalZero", err)
		}
	})

	t.Run("N7 v6-count-255-lazy-bounds", func(t *testing.T) {
		const count = 255
		b := make([]byte, HeaderLen+count*16)
		b[0] = (VersionV3 << 4) | TypeAdvertisement
		b[1] = 10  // vrid
		b[2] = 100 // priority
		b[3] = count
		b[5] = 0x64 // 100 cs
		fe80 := addr(t, "fe80::1").As16()
		copy(b[8:24], fe80[:]) // first VIP link-local
		FillChecksum(b, 0, len(b), src6, dst6)

		var adv Advertisement
		allocs := testing.AllocsPerRun(50, func() {
			var err error
			adv, err = Decode(b, metaV3v6(t), lookupConst(VersionV3, 1000))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
		})
		if allocs != 0 {
			t.Fatalf("hostile count=255 decode allocated %v/run, want 0", allocs)
		}
		if adv.VIPCount() != count {
			t.Fatalf("VIPCount = %d, want %d", adv.VIPCount(), count)
		}
		if !adv.VIPAt(count - 1).IsValid() {
			t.Fatal("VIPAt(254) should be valid")
		}
		if adv.VIPAt(count).IsValid() {
			t.Fatal("VIPAt(255) should be out-of-bounds (zero Addr)")
		}
	})

	t.Run("N8 v2-auth-type-1", func(t *testing.T) {
		// RFC requirement: RFC3768-5.3.6-1 negative -- a v2 Auth Type != 0 (1 = simple password) is discarded with ErrAuthType (validate.go:191)
		// RFC requirement: RFC3768-7.1-5 negative -- a v2 Auth Type not matching the only supported method (0 = No Authentication) is discarded (validate.go:191)
		b := encodeValid(t, advV2(t), src4, dst4)
		b[4] = 1 // simple-password auth
		reChecksum(b, src4, dst4)
		if _, err := Decode(b, metaV2(t), lookupConst(VersionV2, 1000)); !errors.Is(err, ErrAuthType) {
			t.Fatalf("got %v, want ErrAuthType", err)
		}
	})

	t.Run("N9 v2-at-v3-group-and-inverse", func(t *testing.T) {
		// RFC requirement: RFC3768-7.1-1 negative -- a wire version that is not 2 (v2 packet at a v3-configured group, and the inverse) is discarded with ErrVersion (validate.go:133,149)
		if _, err := Decode(mustHex(t, goldenV2Hex), metaV2(t), lookupConst(VersionV3, 1000)); !errors.Is(err, ErrVersion) {
			t.Fatalf("v2@v3: got %v, want ErrVersion", err)
		}
		if _, err := Decode(mustHex(t, goldenV3v4Hex), metaV3v4(t), lookupConst(VersionV2, 1000)); !errors.Is(err, ErrVersion) {
			t.Fatalf("v3@v2: got %v, want ErrVersion", err)
		}
	})

	t.Run("N10 4095cs-decodes-to-40950ms", func(t *testing.T) {
		adv := advV3v4(t)
		adv.AdverIntervalMS = 40950 // 4095 cs
		b := encodeValid(t, adv, src4, dst4)
		got, err := Decode(b, metaV3v4(t), lookupConst(VersionV3, 40950))
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if got.AdverIntervalMS != 40950 {
			t.Fatalf("interval drift: got %d ms, want 40950", got.AdverIntervalMS)
		}
	})
}

// VALIDATES: decode-side priority is raw 0..255 (0 = resign, 255 = owner both
// pass through), and AppendVIPs copies out of the lazy view.
func TestBoundaryPriorityAndAppendVIPs(t *testing.T) {
	for _, prio := range []uint8{0, 255} {
		adv := advV3v4(t)
		adv.Priority = prio
		b := encodeValid(t, adv, addr(t, "192.0.2.251"), MulticastV4)
		got, err := Decode(b, metaV3v4(t), lookupConst(VersionV3, 1000))
		if err != nil {
			t.Fatalf("prio %d: %v", prio, err)
		}
		if got.Priority != prio {
			t.Fatalf("prio %d not preserved: got %d", prio, got.Priority)
		}
	}

	got, err := Decode(mustHex(t, goldenV3v4Hex), metaV3v4(t), lookupConst(VersionV3, 1000))
	if err != nil {
		t.Fatal(err)
	}
	vips := got.AppendVIPs(nil)
	if len(vips) != 2 || vips[0] != addr(t, "192.0.2.1") || vips[1] != addr(t, "192.0.2.2") {
		t.Fatalf("AppendVIPs = %v", vips)
	}
}
