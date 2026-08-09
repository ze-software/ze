// Design: docs/architecture/ospf/ospf-1-types.md -- OSPFv2 Fletcher and Internet checksums
// Related: lsakey.go -- LSA header offsets include the LS checksum field

package types

const (
	fletcherModulus                  = 255
	lsaChecksumOffsetInCoveredRegion = 14
	minLSAChecksumRegionLen          = LSAHeaderLen - 2
)

// FletcherChecksum computes the OSPFv2 LSA Fletcher-16 checksum over a covered window.
//
// RFC 2328 Section 12.1.7: the LS checksum is the ISO Fletcher checksum over
// the complete LSA excluding LS age. Callers pass that covered window: the LSA
// bytes starting at the Options field, with the two LS Age bytes already
// excluded. RFC 905 Annex B: the checksum field is treated as zero during
// generation, then two octets are chosen so the sums over the final region are
// both zero.
func FletcherChecksum(data []byte) uint16 {
	if len(data) < minLSAChecksumRegionLen {
		return 0
	}
	high, low := fletcherGenerate(data, lsaChecksumOffsetInCoveredRegion)
	return uint16(high)<<8 | uint16(low)
}

// FletcherVerify reports whether data carries a valid non-zero OSPF LSA checksum.
//
// data is the same covered window accepted by FletcherChecksum: the LSA starting
// at the Options field, excluding the two LS Age bytes.
func FletcherVerify(data []byte) bool {
	if len(data) < minLSAChecksumRegionLen {
		return false
	}
	if data[lsaChecksumOffsetInCoveredRegion] == 0 && data[lsaChecksumOffsetInCoveredRegion+1] == 0 {
		return false
	}
	return fletcherVerify(data)
}

func fletcherGenerate(data []byte, checkOff int) (byte, byte) {
	if len(data) == 0 || checkOff < 0 || checkOff+1 >= len(data) {
		return 0, 0
	}
	var c0, c1 int
	for i := range data {
		b := int(data[i])
		if i == checkOff || i == checkOff+1 {
			b = 0
		}
		c0 = (c0 + b) % fletcherModulus
		c1 = (c1 + c0) % fletcherModulus
	}
	m := len(data) - checkOff
	x := subMod(mulMod(m-1, c0), c1)
	y := subMod(c1, mulMod(m, c0))
	return normalizeFletcher(x), normalizeFletcher(y)
}

func fletcherVerify(data []byte) bool {
	var c0, c1 int
	for _, b := range data {
		c0 = (c0 + int(b)) % fletcherModulus
		c1 = (c1 + c0) % fletcherModulus
	}
	return c0 == 0 && c1 == 0
}

func mulMod(a, b int) int {
	r := (a * b) % fletcherModulus
	if r < 0 {
		r += fletcherModulus
	}
	return r
}

func subMod(a, b int) int {
	r := (a - b) % fletcherModulus
	if r < 0 {
		r += fletcherModulus
	}
	return r
}

func normalizeFletcher(v int) byte {
	v %= fletcherModulus
	if v < 0 {
		v += fletcherModulus
	}
	if v == 0 {
		return fletcherModulus
	}
	return byte(v)
}

func internetChecksum(data []byte) uint16 {
	sum := internetSum(data)
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

func internetChecksumValid(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sum := internetSum(data)
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return uint16(sum) == 0xffff
}

// InternetChecksumPair computes the RFC 1071 one's-complement checksum over two
// contiguous 16-bit-aligned segments. OSPF packet callers use this to apply the
// common-header rule that excludes the 8-byte Authentication field without
// allocating a temporary concatenated packet.
func InternetChecksumPair(first, second []byte) uint16 {
	sum := internetSum(first) + internetSum(second)
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// InternetChecksumPairValid verifies a checksum over the same two-segment window
// accepted by InternetChecksumPair.
func InternetChecksumPairValid(first, second []byte) bool {
	if len(first)+len(second) == 0 {
		return false
	}
	sum := internetSum(first) + internetSum(second)
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return uint16(sum) == 0xffff
}

func internetSum(data []byte) uint32 {
	var sum uint32
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	return sum
}
