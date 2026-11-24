package ppp

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

// VALIDATES: ParseIPv6CPOptions decodes the Interface-Identifier option
//
//	(type 1, length 10, 8-byte value).
func TestIPv6CPParseOptions(t *testing.T) {
	buf := []byte{1, 10, 0x02, 0x00, 0x5E, 0xFF, 0xFE, 0x00, 0x12, 0x34}
	opts, err := parseIPv6CPOptions(buf)
	if err != nil {
		t.Fatalf("ParseIPv6CPOptions: %v", err)
	}
	if !opts.HasInterfaceID {
		t.Fatalf("HasInterfaceID false, want true")
	}
	want := [8]byte{0x02, 0x00, 0x5E, 0xFF, 0xFE, 0x00, 0x12, 0x34}
	if opts.InterfaceID != want {
		t.Errorf("InterfaceID = %x, want %x", opts.InterfaceID, want)
	}
}

// VALIDATES: WriteIPv6CPOptions + ParseIPv6CPOptions round-trip
//
//	preserves the 8-byte identifier.
func TestIPv6CPRoundtrip(t *testing.T) {
	src := iPv6CPOptions{
		InterfaceID:    [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		HasInterfaceID: true,
	}
	buf := make([]byte, 16)
	n := writeIPv6CPOptions(buf, 0, src)
	if n != 10 {
		t.Fatalf("wrote %d bytes, want 10", n)
	}
	got, err := parseIPv6CPOptions(buf[:n])
	if err != nil {
		t.Fatalf("ParseIPv6CPOptions: %v", err)
	}
	if got != src {
		t.Errorf("roundtrip mismatch: got %x want %x", got.InterfaceID, src.InterfaceID)
	}
}

// VALIDATES: ParseIPv6CPOptions rejects wrong option length, truncated
//
//	buffer, header-under-minimum.
func TestIPv6CPParseRejects(t *testing.T) {
	cases := []struct {
		name    string
		buf     []byte
		wantErr error
	}{
		{"too short", []byte{1}, errOptionTooShort},
		{"len below header", []byte{1, 1}, errOptionLengthMismatch},
		{"len 9 (short)", []byte{1, 9, 1, 2, 3, 4, 5, 6, 7}, errIPv6CPBadOptionLen},
		{"len 11 (long)", []byte{1, 11, 1, 2, 3, 4, 5, 6, 7, 8, 9}, errIPv6CPBadOptionLen},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseIPv6CPOptions(tc.buf)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// VALIDATES: isValidIPv6CPInterfaceID rejects all-zero and all-ones.
//
//	All-zero is RFC 5072 §4.1's own case: every comparison outcome it
//	defines answers a received zero with a Configure-Nak or a
//	Configure-Reject, never an Ack. All-ones is ze's own exclusion and
//	rests on no sentence of that RFC.
func TestIPv6CPInterfaceIDValidity(t *testing.T) {
	var zero, ones [8]byte
	for i := range ones {
		ones[i] = 0xFF
	}
	good := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	if isValidIPv6CPInterfaceID(zero) {
		t.Error("all-zero accepted as valid")
	}
	if isValidIPv6CPInterfaceID(ones) {
		t.Error("all-ones accepted as valid")
	}
	if !isValidIPv6CPInterfaceID(good) {
		t.Error("good id rejected")
	}
}

// VALIDATES: generateIPv6CPInterfaceID produces a non-zero, non-all-ones
//
//	identifier across repeated draws (statistical confidence on an
//	untrusted RNG).
func TestGenerateIPv6CPInterfaceID(t *testing.T) {
	for i := range 32 {
		id, err := generateIPv6CPInterfaceID()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if !isValidIPv6CPInterfaceID(id) {
			t.Errorf("draw %d produced invalid id %x", i, id)
		}
	}
}

// VALIDATES: AC-9 -- IPv6CP's first CONFREQ proposes a locally-generated
//
//	8-byte Interface-Identifier, and carries EXACTLY ONE instance of the
//	Interface-Identifier option.
//
// PREVENTS: ze shipping with an all-zero or fixed identifier, or emitting a
// Configure-Request with zero or duplicate Interface-Identifier options.
//
// RFC requirement: RFC5072-4.1-1 positive -- a Configure-Request MUST contain exactly one
// instance of the Interface-Identifier option (§4.1): writeNCPOptions
// (internal/component/l2tp/ppp/ncp.go) builds a single IPv6CPOptions with HasInterfaceID
// set, and WriteIPv6CPOptions (ipv6cp.go) serializes exactly one Interface-Identifier
// option; the initial Configure-Request is parsed and its type-1 options counted == 1.
func TestIPv6CPProposesInterfaceID(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	pkt := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if pkt.Code != LCPConfigureRequest {
		t.Fatalf("first IPv6CP code = %d, want CR", pkt.Code)
	}
	opts, err := parseIPv6CPOptions(pkt.Data)
	if err != nil {
		t.Fatalf("ParseIPv6CPOptions: %v", err)
	}
	if !opts.HasInterfaceID {
		t.Fatalf("initial CR missing Interface-Identifier")
	}
	if !isValidIPv6CPInterfaceID(opts.InterfaceID) {
		t.Errorf("proposed identifier %x invalid (zero or all-ones)", opts.InterfaceID)
	}

	// RFC 5072 §4.1: the Configure-Request MUST carry exactly one
	// Interface-Identifier option. Count the type-1 options in the raw Data
	// directly, because ParseIPv6CPOptions collapses duplicates into a single
	// HasInterfaceID and would hide a second instance.
	iidCount := 0
	for off := 0; off < len(pkt.Data); {
		if len(pkt.Data)-off < 2 {
			break
		}
		optLen := int(pkt.Data[off+1])
		if optLen < 2 || off+optLen > len(pkt.Data) {
			break
		}
		if pkt.Data[off] == IPv6CPOptInterfaceID {
			iidCount++
		}
		off += optLen
	}
	if iidCount != 1 {
		t.Errorf("Configure-Request carries %d Interface-Identifier options, want exactly 1", iidCount)
	}

	// Keep time referenced so the linter does not flag it unused on
	// platforms where every other time usage goes through the helper.
	_ = time.Now
}

// TestIPv6CPSuggestionHasUniversalBitClear drives
// suggestIPv6CPInterfaceID across many draws, proving the "u" bit
// (canonical bit 6, the low-order bit of ipv6cpUniversalLocalBitMask
// within octet 0) is clear on every value ze suggests. Neither
// accel-ppp nor pppd clears this bit; ze does, since it never derives
// a suggestion from a globally unique EUI-48/EUI-64 identifier loaned
// to the peer, which is the only case the RFC exempts.
//
// VALIDATES: the "u" bit rule -- a suggested identifier's "u" bit is
// always zero.
// PREVENTS: a regression that suggests a value with the bit set,
// wrongly claiming a globally unique source.
//
// RFC requirement: RFC5072-4.1-9 positive -- "The 'u' (universal/local)
// bit of the suggested identifier MUST be set to zero (0) regardless
// of its source unless the globally unique EUI-48/EUI-64 derived
// identifier is provided for the exclusive use by the remote peer"
// (§4.1). Ze never takes that exception, so only the positive case is
// reachable.
func TestIPv6CPSuggestionHasUniversalBitClear(t *testing.T) {
	var local [8]byte // the Go zero value; suggestIPv6CPInterfaceID never legitimately draws it
	for i := range 64 {
		suggestion, err := suggestIPv6CPInterfaceID(local)
		if err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
		if suggestion[0]&ipv6cpUniversalLocalBitMask != 0 {
			t.Errorf("draw %d: suggestion %x has the \"u\" bit set", i, suggestion)
		}
	}
}

// TestIPv6CPTentativeInterfaceIDHasUniversalBitClear proves that every
// randomly generated tentative Interface-Identifier ze draws via
// generateIPv6CPInterfaceID and sends via requestIPv6CPInterfaceID has the
// universal/local ("u") bit (canonical bit 6, octet 0 mask 0x02) cleared, and
// that clearing the bit happens before isValidIPv6CPInterfaceID so a draw whose
// only set bit was 0x02 is rejected and redrawn instead of returned as zero.
//
// VALIDATES: RFC 5072 Section 4.1 non-IEEE / random tentative identifier "u"
// bit MUST be zero across 256 random draws, a scripted all-zero-after-clear
// draw, and an end-to-end initial IPv6CP Configure-Request through
// requestIPv6CPInterfaceID.
// PREVENTS: a regression that sends random tentative identifiers with bit 0x02
// set, falsely claiming IEEE global scope, or that returns all-zero after
// clearing 0x02.
//
// RFC requirement: RFC5072-4.1-10 positive -- every non-IEEE tentative
// Interface-Identifier drawn by generateIPv6CPInterfaceID (ipv6cp.go) across
// 256 draws and sent in the initial IPv6CP Configure-Request via
// requestIPv6CPInterfaceID (ncp.go) has octet 0 bit 0x02 clear, and a raw draw
// with only bit 0x02 set is rejected and redrawn rather than returned as zero.
//
// RFC requirement: RFC5072-4.1-11 positive -- every random tentative
// Interface-Identifier drawn by generateIPv6CPInterfaceID (ipv6cp.go) across
// 256 draws and sent in the initial IPv6CP Configure-Request via
// requestIPv6CPInterfaceID (ncp.go) has octet 0 bit 0x02 clear, and a raw draw
// with only bit 0x02 set is rejected and redrawn rather than returned as zero.
//
// MUTATION: remove `id[0] &^= ipv6cpUniversalLocalBitMask` from
// generateIPv6CPInterfaceID (ipv6cp.go): random draws fail with probability
// 1-2^-256 and the scripted Configure-Request subtest fails deterministically.
func TestIPv6CPTentativeInterfaceIDHasUniversalBitClear(t *testing.T) {
	t.Run("random draws", func(t *testing.T) {
		for i := range 256 {
			id, err := generateIPv6CPInterfaceID()
			if err != nil {
				t.Fatalf("draw %d: %v", i, err)
			}
			if !isValidIPv6CPInterfaceID(id) {
				t.Fatalf("draw %d: identifier %x is invalid", i, id)
			}
			if id[0]&ipv6cpUniversalLocalBitMask != 0 {
				t.Fatalf("draw %d: tentative identifier %x has the \"u\" bit set", i, id)
			}
		}
	})

	t.Run("redraws zero after u bit clear", func(t *testing.T) {
		onlyUBit := [ipv6cpInterfaceIDLen]byte{ipv6cpUniversalLocalBitMask, 0, 0, 0, 0, 0, 0, 0}
		next := [ipv6cpInterfaceIDLen]byte{ipv6cpUniversalLocalBitMask, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}
		saved := rand.Reader
		t.Cleanup(func() { rand.Reader = saved })
		rand.Reader = bytes.NewReader(append(onlyUBit[:], next[:]...))

		id, err := generateIPv6CPInterfaceID()
		if err != nil {
			t.Fatalf("generateIPv6CPInterfaceID: %v", err)
		}
		want := [ipv6cpInterfaceIDLen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}
		if id != want {
			t.Fatalf("generateIPv6CPInterfaceID() = %x, want %x", id, want)
		}
	})

	t.Run("requestIPv6CPInterfaceID initial configure-request", func(t *testing.T) {
		saved := rand.Reader
		// Register cleanup BEFORE newNCPTestDriverCfg so its LIFO cleanups
		// stop the session goroutine before rand.Reader is restored.
		t.Cleanup(func() { rand.Reader = saved })
		rand.Reader = bytes.NewReader(bytes.Repeat([]byte{0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02}, 64))

		td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
		defer td.cleanup()

		pkt := td.readPeerNCPPacket(t, ProtoIPv6CP)
		if pkt.Code != LCPConfigureRequest {
			t.Fatalf("first IPv6CP code = %d, want Configure-Request", pkt.Code)
		}
		opts, err := parseIPv6CPOptions(pkt.Data)
		if err != nil {
			t.Fatalf("parseIPv6CPOptions: %v", err)
		}
		if !opts.HasInterfaceID {
			t.Fatal("initial Configure-Request missing Interface-Identifier option")
		}
		want := [ipv6cpInterfaceIDLen]byte{0x00, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02}
		if opts.InterfaceID != want {
			t.Fatalf("initial Configure-Request Interface-Identifier = %x, want %x", opts.InterfaceID, want)
		}
		if opts.InterfaceID[0]&ipv6cpUniversalLocalBitMask != 0 {
			t.Fatalf("initial Configure-Request Interface-Identifier %x has the \"u\" bit set", opts.InterfaceID)
		}
	})
}

// TestIPv6CPGlobalEUI48TentativeIdentifierSetsUniversalBit checks the
// contrasting IEEE global EUI-48 path of RFC 5072 Section 4.1 (case 1):
// when a globally unique EUI-48 MAC is supplied, TentativeIPv6CPInterfaceID
// inverts the "u" bit to 1 rather than forcing it to 0 as non-IEEE cases 2
// and 3 mandate.
//
// VALIDATES: the "u" bit clear applies to non-IEEE / random identifiers, not
// to globally unique EUI-48 identifiers.
// PREVENTS: blindly clearing bit 0x02 on case-1 EUI-48 identifiers.
//
// RFC requirement: RFC5072-4.1-10 negative -- when a globally unique EUI-48
// MAC (00:11:22:33:44:55, universal/local bit 0) is supplied to
// TentativeIPv6CPInterfaceID (ipv6cp_client.go), the resulting modified
// EUI-64 identifier carries the "u" bit set to 1 (0x02) rather than 0.
//
// RFC requirement: RFC5072-4.1-11 negative -- when a globally unique EUI-48
// MAC (00:11:22:33:44:55, universal/local bit 0) is supplied to
// TentativeIPv6CPInterfaceID (ipv6cp_client.go), the resulting modified
// EUI-64 identifier carries the "u" bit set to 1 (0x02) rather than 0.
//
// MUTATION: clear ipv6cpUniversalLocalBitMask instead of inverting it in
// TentativeIPv6CPInterfaceID (ipv6cp_client.go): octet 0 becomes 0x00 instead
// of 0x02.
func TestIPv6CPGlobalEUI48TentativeIdentifierSetsUniversalBit(t *testing.T) {
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	id, err := TentativeIPv6CPInterfaceID(mac)
	if err != nil {
		t.Fatalf("TentativeIPv6CPInterfaceID: %v", err)
	}
	want := IPv6CPInterfaceID{0x02, 0x11, 0x22, 0xff, 0xfe, 0x33, 0x44, 0x55}
	if id != want {
		t.Fatalf("TentativeIPv6CPInterfaceID(%x) = %x, want %x", mac, id, want)
	}
	if id[0]&ipv6cpUniversalLocalBitMask != ipv6cpUniversalLocalBitMask {
		t.Fatalf("identifier %x has the \"u\" bit clear, want set for globally unique EUI-48", id)
	}
}

// TestIPv6CPLocallyAdministeredMACClearsUniversalBit verifies RFC 5072 Section
// 4.1 case 2 for a non-IEEE locally-administered link-layer MAC: inverting the
// IEEE U/L bit (set to 1 in the locally-administered MAC) produces a modified
// EUI-64 tentative identifier with the "u" bit set to zero (0).
//
// VALIDATES: TentativeIPv6CPInterfaceID produces the modified EUI-64 identifier
// with octet 0 bit 0x02 cleared when supplied a locally-administered MAC
// (02:11:22:33:44:55).
// PREVENTS: a regression that leaves the "u" bit set for a non-IEEE
// locally-administered link-layer address.
//
// RFC requirement: RFC5072-4.1-10 positive -- when a non-IEEE
// locally-administered link-layer MAC (02:11:22:33:44:55, universal/local bit
// 1) is supplied to TentativeIPv6CPInterfaceID (ipv6cp_client.go), the
// resulting modified EUI-64 identifier is 00:11:22:ff:fe:33:44:55 with octet 0
// bit 0x02 cleared to zero (0).
//
// MUTATION: remove the XOR with ipv6cpUniversalLocalBitMask in
// TentativeIPv6CPInterfaceID (ipv6cp_client.go): octet 0 remains 0x02 instead
// of 0x00.
func TestIPv6CPLocallyAdministeredMACClearsUniversalBit(t *testing.T) {
	mac := []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55}
	id, err := TentativeIPv6CPInterfaceID(mac)
	if err != nil {
		t.Fatalf("TentativeIPv6CPInterfaceID: %v", err)
	}
	if id[0]&ipv6cpUniversalLocalBitMask != 0 {
		t.Fatalf("identifier %x has the \"u\" bit set, want 0 for locally-administered MAC", id)
	}
	want := IPv6CPInterfaceID{0x00, 0x11, 0x22, 0xff, 0xfe, 0x33, 0x44, 0x55}
	if id != want {
		t.Fatalf("TentativeIPv6CPInterfaceID(%x) = %x, want %x", mac, id, want)
	}
}
