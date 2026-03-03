// Design: docs/architecture/wire/nlri.md — NLRI encoding and decoding
//
// Package nlri implements BGP Network Layer Reachability Information encoding.
//
// This file contains shared helper functions for NLRI encoding.
package nlri

// PrefixBytes returns the number of bytes needed for a prefix of given bit length.
//
// RFC 4271 Section 4.3 - UPDATE Message Format:
// "The Prefix field contains an IP address prefix, followed by enough
// trailing bits to make the end of the field fall on an octet boundary.".
func PrefixBytes(bits int) int {
	return (bits + 7) / 8
}

// WriteLabelStack writes MPLS labels to buf at offset.
// Returns number of bytes written.
//
// RFC 3032 - MPLS Label Stack Encoding (for BGP, 3 bytes per label):
//
//	Byte 0: label[19:12]
//	Byte 1: label[11:4]
//	Byte 2: label[3:0] | TC[2:0] | S
//
// The S (bottom-of-stack) bit is set on the last label only.
// TC (Traffic Class) is always 0 for BGP label encoding.
//
// NOTE: RFC 3032 data plane uses 4 bytes (includes TTL).
// BGP uses 3 bytes (no TTL) per RFC 8277.
func WriteLabelStack(buf []byte, off int, labels []uint32) int {
	for i, label := range labels {
		pos := off + i*3
		buf[pos] = byte(label >> 12)
		buf[pos+1] = byte(label >> 4)
		buf[pos+2] = byte(label<<4) & 0xF0
		if i == len(labels)-1 {
			buf[pos+2] |= 0x01 // RFC 3107: S (bottom-of-stack) bit
		}
	}
	return len(labels) * 3
}
