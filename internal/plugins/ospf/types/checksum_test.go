// Design: docs/architecture/ospf/ospf-1-types.md -- OSPF checksum algorithm vectors and boundaries

package types

import (
	"bytes"
	"testing"
)

func testLSABytes() []byte {
	return []byte{
		0x00, 0x10,
		byte(OptionE), byte(LSTypeRouter),
		1, 2, 3, 4,
		5, 6, 7, 8,
		0x80, 0x00, 0x00, 0x01,
		0x00, 0x00,
		0x00, 0x18,
		9, 10, 11, 12,
	}
}

// VALIDATES: AC-10 - OSPF LSA Fletcher accepts the covered window beginning at Options.
// PREVENTS: downstream codec code passing `lsa[2:]` and getting a checksum over the wrong bytes.
//
// RFC requirement: RFC905-x-6 positive -- OSPF LSA Fletcher over the covered window starting at Options (FletcherChecksum, checksum.go:20-26).
// RFC requirement: RFC905-x-7 positive -- encodes the checksum then decodes it: FletcherVerify accepts the placed vector (checksum.go:32-40).
func TestFletcherRFC905Vectors(t *testing.T) {
	lsa := testLSABytes()
	covered := lsa[2:]
	checksum := FletcherChecksum(covered)
	const want uint16 = 0x15ff
	if checksum != want {
		t.Fatalf("FletcherChecksum = %#04x, want %#04x", checksum, want)
	}
	covered[lsaChecksumOffsetInCoveredRegion] = byte(checksum >> 8)
	covered[lsaChecksumOffsetInCoveredRegion+1] = byte(checksum)
	if !FletcherVerify(covered) {
		t.Fatalf("FletcherVerify rejected vector checksum %#04x over % x", checksum, covered)
	}
}

// VALIDATES: AC-10 - LS Age is outside the Fletcher covered range.
// PREVENTS: age changes in flight invalidating an otherwise correct LSA checksum.
//
// RFC requirement: RFC905-x-6 positive -- mutating the excluded LS Age bytes leaves the Fletcher checksum unchanged, proving coverage starts at Options (FletcherChecksum, checksum.go:20-26).
func TestFletcherIgnoresLSAge(t *testing.T) {
	lsa := testLSABytes()
	before := FletcherChecksum(lsa[2:])
	lsa[0] ^= 0xff
	lsa[1] ^= 0xff
	after := FletcherChecksum(lsa[2:])
	if before != after {
		t.Fatalf("Fletcher changed after LS Age changed: before=%#04x after=%#04x", before, after)
	}
}

// VALIDATES: AC-11 - RFC 1071 Internet checksum sums words and verifies to 0xFFFF.
// PREVENTS: OSPF packets from being rejected because checksum generation and verification disagree.
//
// RFC requirement: RFC1071-1-3 positive -- internetChecksum stores the bitwise-NOT of the folded 16-bit sum; the exact vector 0x1411 fails if the complement is omitted (internetChecksum, checksum.go:97-103).
// RFC requirement: RFC1071-1-6 positive -- the carry-producing vector drives the fold loop until no high bits remain before inverting (internetChecksum fold, checksum.go:99-101).
func TestInternetChecksumRFC1071Vectors(t *testing.T) {
	data := []byte{0x00, 0x01, 0x00, 0x00, 0xf4, 0xf5, 0xf6, 0xf7}
	checksum := internetChecksum(data)
	const want uint16 = 0x1411
	if checksum != want {
		t.Fatalf("internetChecksum = %#04x, want %#04x", checksum, want)
	}
	data[2] = byte(checksum >> 8)
	data[3] = byte(checksum)
	if !internetChecksumValid(data) {
		t.Fatalf("internetChecksumValid rejected % x", data)
	}
}

// VALIDATES: downstream OSPF packet checksum can exclude the 8-byte auth field without allocation.
// PREVENTS: packet codec building a temporary concatenated buffer for RFC 1071 coverage.
func TestInternetChecksumPairMatchesConcatenatedWindow(t *testing.T) {
	first := []byte{0x02, 0x01, 0x00, 0x18, 10, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0}
	second := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	joined := append(append([]byte(nil), first...), second...)
	checksum := InternetChecksumPair(first, second)
	if checksum != internetChecksum(joined) {
		t.Fatalf("InternetChecksumPair = %#04x, concatenated = %#04x", checksum, internetChecksum(joined))
	}
	first[12] = byte(checksum >> 8)
	first[13] = byte(checksum)
	if !InternetChecksumPairValid(first, second) {
		t.Fatalf("InternetChecksumPairValid rejected first=% x second=% x", first, second)
	}
}

// VALIDATES: AC-11 - odd-length Internet checksum windows are padded with zero for the sum only.
// PREVENTS: losing the final odd byte when summing OSPF packets.
//
// RFC requirement: RFC1071-1-4 positive -- an odd-length window is padded with one zero octet for the sum only (transmitted length unchanged); the exact vector 0x97cb fails if the trailing byte is dropped (internetSum odd-byte path, checksum.go:146-148).
func TestInternetChecksumOddLength(t *testing.T) {
	data := []byte{0x12, 0x34, 0x56}
	checksum := internetChecksum(data)
	const want uint16 = 0x97cb
	if checksum != want {
		t.Fatalf("internetChecksum odd length = %#04x, want %#04x", checksum, want)
	}
}

// VALIDATES: AC-10, AC-11 - checksum helpers allocate zero and do not mutate caller-owned windows.
// PREVENTS: checksum validation copying or altering shared wire buffers.
func TestChecksumNoAlloc(t *testing.T) {
	lsa := testLSABytes()
	covered := lsa[2:]
	packet := []byte{0x02, 0x01, 0x00, 0x18, 10, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	origLSA := append([]byte(nil), lsa...)
	origPacket := append([]byte(nil), packet...)
	if allocs := testing.AllocsPerRun(1000, func() { _ = FletcherChecksum(covered) }); allocs != 0 {
		t.Fatalf("FletcherChecksum allocated %.2f times, want 0", allocs)
	}
	if allocs := testing.AllocsPerRun(1000, func() { _ = internetChecksum(packet) }); allocs != 0 {
		t.Fatalf("internetChecksum allocated %.2f times, want 0", allocs)
	}
	if !bytes.Equal(lsa, origLSA) {
		t.Fatalf("FletcherChecksum mutated input: got % x want % x", lsa, origLSA)
	}
	if !bytes.Equal(packet, origPacket) {
		t.Fatalf("internetChecksum mutated input: got % x want % x", packet, origPacket)
	}
}
