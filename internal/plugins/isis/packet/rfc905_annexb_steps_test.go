// Design: docs/architecture/wire/isis.md -- Fletcher checksum Annex B step tests

package packet

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// annexBSender is a test-local generator that follows RFC 905 Annex B.3 except
// in the two steps a faulty sender can skip: keepField leaves the stale octets of
// the checksum field in the sums (B.3.1 not done), and c0Start, c1Start seed the
// running sums with a value other than zero (B.3.2 not done). It stands for a
// non-compliant sender whose output a compliant receiver must refuse.
func annexBSender(data []byte, checkOff int, keepField bool, c0Start, c1Start int) (x, y byte) {
	c0, c1 := c0Start, c1Start
	for i, octet := range data {
		b := int(octet)
		if !keepField && (i == checkOff || i == checkOff+1) {
			b = 0
		}
		c0 = (c0 + b) % 255
		c1 = (c1 + c0) % 255
	}
	length, n := len(data), checkOff+1
	xv := ((-c1+(length-n)*c0)%255 + 255) % 255
	yv := ((c1-(length-n+1)*c0)%255 + 255) % 255
	return byte(xv), byte(yv)
}

// sumsZero re-sums data octet by octet the way Annex B.4.2 states it and
// reports whether both C0 and C1 end at zero. The tests pass it a slice that
// leaves out an octet, to stand for a receiver that skips one.
func sumsZero(data []byte) bool {
	c0, c1 := 0, 0
	for _, b := range data {
		c0 = (c0 + int(b)) % 255
		c1 = (c1 + c0) % 255
	}
	return c0 == 0 && c1 == 0
}

// TestRFC905GeneratorZeroesTheFieldFirst proves Checksum reads the checksum
// field as zero, whatever octets it holds when the sums start.
//
// Goal: a PDU buffer is often reused, so the field can hold an old checksum.
// Method: the same 24-octet region twice, once with the field zeroed and once
// with stale non-zero octets in it. Checksum MUST return the same X and Y for
// both, equal to the independent Annex B.3 derivation over the zeroed region,
// and that pair placed in the stale region MUST verify.
//
// RFC requirement: RFC905-B.3.1-1 positive -- Checksum over a region whose field holds stale octets 0x5A,0xC3 returns the X,Y computed over the same region with the field set to zero, and the pair verifies.
func TestRFC905GeneratorZeroesTheFieldFirst(t *testing.T) {
	zeroed := rfc905Region()
	zeroed[12], zeroed[13] = 0, 0
	stale := rfc905Region()
	stale[12], stale[13] = 0x5A, 0xC3

	wantX, wantY := isisClosedForm(zeroed, 12)
	zx, zy := Checksum(zeroed, 12)
	if zx != wantX || zy != wantY {
		t.Fatalf("Checksum over the zeroed field = %02x%02x, want %02x%02x", zx, zy, wantX, wantY)
	}
	sx, sy := Checksum(stale, 12)
	if sx != wantX || sy != wantY {
		t.Fatalf("Checksum over the stale field = %02x%02x, want %02x%02x (the field read as zero)", sx, sy, wantX, wantY)
	}
	stale[12], stale[13] = sx, sy
	if !VerifyChecksum(stale) {
		t.Fatalf("checksum %02x%02x generated over a stale field refused", sx, sy)
	}
}

// TestRFC905SenderThatKeptTheFieldRefused proves a checksum computed without
// first zeroing the field is refused.
//
// Goal: the receive side is where a sender that skipped Annex B.3.1 is caught.
// Method: a faulty sender sums the region with the stale field octets in place.
// The unit first checks its pair differs from Checksum's, so the vector
// discriminates, then VerifyChecksum MUST accept the compliant pair and refuse
// the faulty one.
//
// RFC requirement: RFC905-B.3.1-1 negative -- a checksum whose sums included the stale field octets 0x5A,0xC3 instead of zero is refused by VerifyChecksum, while the pair computed over the zeroed field is accepted.
func TestRFC905SenderThatKeptTheFieldRefused(t *testing.T) {
	region := rfc905Region()
	region[12], region[13] = 0x5A, 0xC3
	x, y := annexBSender(region, 12, true, 0, 0)
	high, low := Checksum(region, 12)
	if x == high && y == low {
		t.Fatalf("faulty sender produced the compliant checksum %02x%02x; pick other stale octets", x, y)
	}

	region[12], region[13] = high, low
	if !VerifyChecksum(region) {
		t.Fatalf("compliant checksum %02x%02x refused", high, low)
	}
	region[12], region[13] = x, y
	if VerifyChecksum(region) {
		t.Fatalf("checksum %02x%02x summed over the stale field accepted", x, y)
	}
}

// TestRFC905GeneratorStartsFromZeroSums proves Checksum starts C0 and C1 at
// zero.
//
// Goal: with both sums starting at zero, an all-zero region leaves C0 and C1 at
// zero, so X and Y are zero, which Ze stores as 255 (Annex B.2 b). Method: an
// all-zero 24-octet region with the field at every offset from 0 to L-2, where
// Checksum MUST return 0xFF,0xFF, and a non-zero region, where it MUST equal the
// independent derivation that starts both sums at zero.
//
// RFC requirement: RFC905-B.3.2-1 positive -- Checksum returns 0xFF,0xFF (zero, stored as 255) for an all-zero region at every field offset, and equals the independent Annex B.3 derivation whose C0 and C1 start at zero.
func TestRFC905GeneratorStartsFromZeroSums(t *testing.T) {
	blank := make([]byte, 24)
	for off := 0; off+1 < len(blank); off++ {
		x, y := Checksum(blank, off)
		if x != 0xFF || y != 0xFF {
			t.Errorf("all-zero region, field at %d: Checksum = %02x%02x, want ffff", off, x, y)
		}
	}
	region := rfc905Region()
	wantX, wantY := isisClosedForm(region, 12)
	if x, y := Checksum(region, 12); x != wantX || y != wantY {
		t.Errorf("Checksum = %02x%02x, want %02x%02x from sums started at zero", x, y, wantX, wantY)
	}
}

// TestRFC905SenderWithSeededSumsRefused proves a checksum computed from sums
// that did not start at zero is refused.
//
// Goal: the receive side catches a sender that skipped Annex B.3.2. Method: a
// faulty sender seeds C0, or C1, with 1. The unit checks each faulty pair
// differs from Checksum's, then VerifyChecksum MUST accept the compliant pair
// and refuse the faulty one.
//
// RFC requirement: RFC905-B.3.2-1 negative -- a checksum computed with C0 or C1 initialized to 1 instead of zero is refused by VerifyChecksum, while the pair from sums started at zero is accepted.
func TestRFC905SenderWithSeededSumsRefused(t *testing.T) {
	cases := []struct {
		name    string
		c0Start int
		c1Start int
	}{
		{"c0-seeded", 1, 0},
		{"c1-seeded", 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			region := rfc905Region()
			x, y := annexBSender(region, 12, false, tc.c0Start, tc.c1Start)
			high, low := Checksum(region, 12)
			if x == high && y == low {
				t.Fatalf("faulty sender produced the compliant checksum %02x%02x", x, y)
			}
			region[12], region[13] = high, low
			if !VerifyChecksum(region) {
				t.Fatalf("compliant checksum %02x%02x refused", high, low)
			}
			region[12], region[13] = x, y
			if VerifyChecksum(region) {
				t.Fatalf("checksum %02x%02x from seeded sums accepted", x, y)
			}
		})
	}
}

// annexBLSP encodes a small L2 LSP and returns the PDU and the offset of the
// first octet of its checksummed region within it.
func annexBLSP(t *testing.T) (pdu []byte, regionStart int) {
	t.Helper()
	sys := types.SystemID{2, 2, 2, 2, 2, 2}
	in := &LSP{
		PDUType:           PDUTypeL2LSP,
		RemainingLifetime: 900,
		LSPID:             types.NewLSPID(types.NewSourceID(sys, 0), 0),
		SequenceNumber:    7,
		TLVs:              []TLV{ext135TLV(netip.MustParsePrefix("198.51.100.0/24"), 10)},
	}
	pdu = make([]byte, in.EncodedLen())
	in.WriteTo(pdu, 0)
	return pdu, CommonHeaderLen + lspRemLifetimeOff + types.LifetimeLen
}

// TestRFC905LSPPlacesXAtNAndYAtNPlus1 proves the LSP encoder stores X in the
// field's first octet and Y in the second.
//
// Goal: X and Y are solved for their positions, so each belongs in its own
// octet. Method: encode an LSP with WriteTo, compute X and Y independently over
// its checksummed region (field at region offset 12), and require X at octet n
// and Y at octet n+1 of the encoded PDU. The decoded LSP MUST verify. X and Y
// differ for this LSP, so a swap cannot pass.
//
// RFC requirement: RFC905-B.3.5-1 positive -- the PDU LSP.WriteTo encodes carries the independently derived X at the checksum field's first octet and Y at the next, and the decoded LSP verifies.
func TestRFC905LSPPlacesXAtNAndYAtNPlus1(t *testing.T) {
	pdu, regionStart := annexBLSP(t)
	region := pdu[regionStart:]
	x, y := isisClosedForm(region, 12)
	if x == y {
		t.Fatalf("X == Y (%02x): a swap would be invisible; change the LSP", x)
	}
	if region[12] != x || region[13] != y {
		t.Fatalf("encoded field = %02x%02x, want X=%02x at n and Y=%02x at n+1", region[12], region[13], x, y)
	}
	decoded, err := DecodePDU(pdu)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	if !decoded.LSP.VerifyChecksum() {
		t.Fatal("encoded LSP refused by LSP.VerifyChecksum")
	}
}

// TestRFC905LSPWithSwappedXYRefused proves a received LSP whose X and Y sit in
// each other's octets is refused.
//
// Goal: the receive path is where a sender that misplaced X and Y is caught.
// Method: encode an LSP, swap the two checksum octets, decode the PDU and call
// LSP.VerifyChecksum, the check the flooding path runs. The unswapped PDU MUST
// verify and the swapped one MUST NOT. X and Y differ, so the swap changes the
// octets.
//
// RFC requirement: RFC905-B.3.5-1 negative -- a received LSP with Y at the checksum field's first octet and X at the next is refused by LSP.VerifyChecksum, while the same LSP with X at n and Y at n+1 is accepted.
func TestRFC905LSPWithSwappedXYRefused(t *testing.T) {
	pdu, regionStart := annexBLSP(t)
	n := regionStart + 12
	if pdu[n] == pdu[n+1] {
		t.Fatalf("X == Y (%02x): a swap would be invisible; change the LSP", pdu[n])
	}
	good, err := DecodePDU(pdu)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	if !good.LSP.VerifyChecksum() {
		t.Fatal("LSP with X at n and Y at n+1 refused")
	}

	swapped := make([]byte, len(pdu))
	copy(swapped, pdu)
	swapped[n], swapped[n+1] = pdu[n+1], pdu[n]
	bad, err := DecodePDU(swapped)
	if err != nil {
		t.Fatalf("DecodePDU of the swapped PDU: %v", err)
	}
	if bad.LSP.VerifyChecksum() {
		t.Fatalf("LSP with Y=%02x at n and X=%02x at n+1 accepted", swapped[n], swapped[n+1])
	}
}

// TestRFC905VerifierAcceptsEveryPlacement proves VerifyChecksum accepts a
// correct checksum at every length and field offset tried.
//
// Goal: the Annex B.4.2 sums run over every octet from 1 to L, including the
// two checksum octets wherever they sit. Method: regions of 4, 9, 24 and 41
// octets, the field at every offset from 0 to L-2 (so also as the first and as
// the last two octets), X and Y from the independent derivation. VerifyChecksum
// MUST accept each.
//
// RFC requirement: RFC905-B.4.2-1 positive -- VerifyChecksum accepts regions of 4, 9, 24 and 41 octets carrying the independently derived X,Y at every field offset from the first to the last two octets.
func TestRFC905VerifierAcceptsEveryPlacement(t *testing.T) {
	for _, length := range []int{4, 9, 24, 41} {
		for off := 0; off+1 < length; off++ {
			region := make([]byte, length)
			for i := range region {
				region[i] = byte(i*29 + 3)
			}
			region[off], region[off+1] = isisClosedForm(region, off)
			if !VerifyChecksum(region) {
				t.Errorf("length %d, field at %d: correct checksum refused", length, off)
			}
		}
	}
}

// TestRFC905VerifierReadsTheFirstAndLastOctet proves VerifyChecksum does not
// skip the first or the last octet of the region.
//
// Goal: Annex B.4.2 runs from i = 1 to L. A receiver that starts at i = 2 or
// stops at L-1 accepts a region whose first or last octet was changed. Method:
// a valid 24-octet region whose first and last octets are zero, so the sums
// without either octet are also zero. Change the first octet, then the last, to
// 1. The unit checks that a receiver skipping that octet would accept the
// changed region, so the vector discriminates, then VerifyChecksum MUST refuse
// it and MUST accept the unchanged region.
//
// RFC requirement: RFC905-B.4.2-1 negative -- a region whose first or last octet changed from 0 to 1 is refused by VerifyChecksum, although sums that skip that octet end at zero; the unchanged region is accepted.
func TestRFC905VerifierReadsTheFirstAndLastOctet(t *testing.T) {
	valid := rfc905Region()
	valid[0], valid[len(valid)-1] = 0, 0
	valid[12], valid[13] = Checksum(valid, 12)
	if !VerifyChecksum(valid) {
		t.Fatal("valid region refused")
	}
	cases := []struct {
		name    string
		at      int
		skipped func(b []byte) []byte
	}{
		{"first-octet", 0, func(b []byte) []byte { return b[1:] }},
		{"last-octet", len(valid) - 1, func(b []byte) []byte { return b[:len(b)-1] }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := make([]byte, len(valid))
			copy(changed, valid)
			changed[tc.at] = 1
			if !sumsZero(tc.skipped(changed)) {
				t.Fatal("a receiver skipping the octet would refuse too; the vector does not discriminate")
			}
			if VerifyChecksum(changed) {
				t.Fatalf("region with octet %d changed to 1 accepted", tc.at)
			}
		})
	}
}
