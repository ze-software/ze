// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- MD4 for MS-CHAPv2 NtPasswordHash
// RFC: rfc/short/rfc2759.md -- NtPasswordHash requires MD4 (Section 8)

package eap

import "encoding/binary"

// md4Sum computes the MD4 digest of data per RFC 1320.
// MD4 is cryptographically broken. Used only because MS-CHAPv2 mandates it.
func md4Sum(data []byte) [16]byte {
	var a, b, c, d uint32 = 0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476

	orig := len(data)
	padded := make([]byte, len(data), len(data)+1+64+8)
	copy(padded, data)
	padded = append(padded, 0x80)
	for len(padded)%64 != 56 {
		padded = append(padded, 0)
	}
	var bits [8]byte
	binary.LittleEndian.PutUint64(bits[:], uint64(orig)*8)
	padded = append(padded, bits[:]...)

	for off := 0; off < len(padded); off += 64 {
		var x [16]uint32
		for i := range x {
			x[i] = binary.LittleEndian.Uint32(padded[off+i*4:])
		}
		aa, bb, cc, dd := a, b, c, d

		r1idx := [16]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
		r1s := [4]uint{3, 7, 11, 19}
		for j, i := range r1idx {
			f := (b & c) | (^b & d)
			a += f + x[i]
			a = a<<r1s[j%4] | a>>(32-r1s[j%4])
			a, b, c, d = d, a, b, c
		}

		r2idx := [16]int{0, 4, 8, 12, 1, 5, 9, 13, 2, 6, 10, 14, 3, 7, 11, 15}
		r2s := [4]uint{3, 5, 9, 13}
		for j, i := range r2idx {
			f := (b & c) | (b & d) | (c & d)
			a += f + x[i] + 0x5a827999
			a = a<<r2s[j%4] | a>>(32-r2s[j%4])
			a, b, c, d = d, a, b, c
		}

		r3idx := [16]int{0, 8, 4, 12, 2, 10, 6, 14, 1, 9, 5, 13, 3, 11, 7, 15}
		r3s := [4]uint{3, 9, 11, 15}
		for j, i := range r3idx {
			f := b ^ c ^ d
			a += f + x[i] + 0x6ed9eba1
			a = a<<r3s[j%4] | a>>(32-r3s[j%4])
			a, b, c, d = d, a, b, c
		}

		a += aa
		b += bb
		c += cc
		d += dd
	}

	var out [16]byte
	binary.LittleEndian.PutUint32(out[0:], a)
	binary.LittleEndian.PutUint32(out[4:], b)
	binary.LittleEndian.PutUint32(out[8:], c)
	binary.LittleEndian.PutUint32(out[12:], d)
	return out
}
