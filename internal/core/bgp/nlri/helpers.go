// Design: docs/architecture/wire/nlri.md — NLRI encoding and decoding
//
// Package nlri implements BGP Network Layer Reachability Information encoding.
//
// This file contains shared helper functions for NLRI encoding.
package nlri

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
)

// WirePrefixToKey converts NLRI wire prefix bytes [prefix-len][prefix-bytes...]
// to a netip.Prefix value type. Returns (prefix, true) on success.
func WirePrefixToKey(wire []byte, fam family.Family) (netip.Prefix, bool) {
	if len(wire) == 0 {
		return netip.Prefix{}, false
	}
	prefixLen := int(wire[0])
	byteCount := (prefixLen + 7) / 8
	if len(wire) < 1+byteCount {
		return netip.Prefix{}, false
	}
	var buf [16]byte
	copy(buf[:], wire[1:1+byteCount])
	addrLen := 4
	if fam.AFI == family.AFIIPv6 {
		addrLen = 16
	}
	addr, ok := netip.AddrFromSlice(buf[:addrLen])
	if !ok {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, prefixLen), true
}

// PrefixBytes returns the number of bytes needed for a prefix of given bit length.
//
// RFC 4271 Section 4.3 - UPDATE Message Format:
// "The Prefix field contains an IP address prefix, followed by enough
// trailing bits to make the end of the field fall on an octet boundary.".
func PrefixBytes(bits int) int {
	return (bits + 7) / 8
}

// WriteLabelStack writes MPLS label stack ENTRIES to buf at offset.
// Returns number of bytes written.
//
// RFC 3032 Section 2.1 - label stack entry (3 octets in BGP, which RFC 8277
// Section 2.1 carries without the data plane's TTL octet):
//
//	Byte 0: label[19:12]
//	Byte 1: label[11:4]
//	Byte 2: label[3:0] | TC[2:0] | S
//
// The entry is written whole, so a traffic class a peer set survives a relay.
// LabelEntryFor builds one from a bare label for a caller that has no entry.
//
// The S bit is the one field this function OWNS rather than copies: RFC 3032
// Section 2.1 says "this bit is set to one for the last entry in the label
// stack, and zero for all other label stack entries", which is a property of
// the stack's shape and not of any entry's data. A caller that reorders,
// truncates or concatenates stacks cannot be asked to maintain it.
func WriteLabelStack(buf []byte, off int, entries []uint32) int {
	for i, entry := range entries {
		pos := off + i*3
		if i == len(entries)-1 {
			entry |= 0x000001
		} else {
			entry &^= 0x000001
		}
		buf[pos] = byte(entry >> 16)
		buf[pos+1] = byte(entry >> 8)
		buf[pos+2] = byte(entry)
	}
	return len(entries) * 3
}

// WriteLabelValues writes a stack built from 20-bit LABEL VALUES, with a zero
// traffic class on every entry and the bottom-of-stack bit on the last.
// Returns number of bytes written.
//
// For a speaker that ORIGINATES the stack: an operator's config, a CLI
// argument. It allocates nothing, which is why a caller on the UPDATE build
// path uses it rather than widening the slice with LabelEntriesFor first.
//
// A speaker RELAYING a stack it parsed calls WriteLabelStack with the entries,
// so the traffic class the peer set is not replaced by this function's zero.
func WriteLabelValues(buf []byte, off int, labels []uint32) int {
	for i, label := range labels {
		pos := off + i*3
		buf[pos] = byte(label >> 12)
		buf[pos+1] = byte(label >> 4)
		buf[pos+2] = byte(label<<4) & 0xF0
		if i == len(labels)-1 {
			buf[pos+2] |= 0x01 // RFC 3032 Section 2.1: S (bottom-of-stack) bit
		}
	}
	return len(labels) * 3
}

// labeledNLRILengthBitsMax is the largest value the one-octet Length field of
// a labeled NLRI holds (RFC 8277 Section 2.2: "The Length field consists of a
// single octet.").
const labeledNLRILengthBitsMax = 255

// LabeledNLRIOctetsMax is the longest labeled unicast NLRI WriteLabeledNLRI
// writes: the Length octet and the 255 bits it can count, rounded up.
const LabeledNLRIOctetsMax = 1 + (labeledNLRILengthBitsMax+7)/8

// LabeledNLRILabelsMax is the deepest label stack a labeled NLRI holds: each
// label takes 24 of the 255 bits the Length field can count.
const LabeledNLRILabelsMax = labeledNLRILengthBitsMax / 24

// WriteLabeledNLRI writes at buf[off:] the labeled unicast NLRI (SAFI 4) that
// binds a label stack to a prefix, as an UPDATE carries it, and returns the
// octets written. It is the inverse of nlrisplit.ExtractLabels, and the one
// declaration of this layout for a caller that holds the two parts apart.
//
// prefixNLRI is [prefix length (1, bits)][prefix (ceil(bits/8))], with no Path
// Identifier: under ADD-PATH the caller writes the identifier ahead of off.
// labels are 20-bit label values, top of stack first.
//
// RFC 8277 Section 2.3 (Section 2.2 is the same layout with one label):
//
//	off+0       Length (1): the bits that follow, 24 per label plus the prefix's
//	off+1       N entries of 3 octets: Label (20 bits), Rsrv (3 bits), S (1 bit)
//	off+1+3N    Prefix, ceil(prefix bits / 8) octets
//
// RFC 8277 Section 2.3: "Note that for each label, the length is increased by
// 24 bits (20 bits in the Label field, plus 3 bits in the Rsrv field, plus 1 S
// bit)." WriteLabelValues writes Rsrv as zero and S on the last entry alone.
//
// It answers false, having written nothing, for an empty stack, a prefixNLRI
// shorter than its own prefix length, a Length past 255 bits, and a buf too
// short: none of them is a labeled NLRI this function can write whole.
func WriteLabeledNLRI(buf []byte, off int, prefixNLRI []byte, labels []uint32) (int, bool) {
	if len(labels) == 0 {
		return 0, false
	}
	if len(prefixNLRI) == 0 {
		return 0, false
	}
	prefixBits := int(prefixNLRI[0])
	lengthBits := prefixBits + 24*len(labels)
	if lengthBits > labeledNLRILengthBitsMax {
		return 0, false
	}
	prefixOctets := PrefixBytes(prefixBits)
	if len(prefixNLRI) < 1+prefixOctets {
		return 0, false
	}
	octets := 1 + 3*len(labels) + prefixOctets
	if len(buf) < off+octets {
		return 0, false
	}
	buf[off] = byte(lengthBits)
	labelOctets := WriteLabelValues(buf, off+1, labels)
	copy(buf[off+1+labelOctets:], prefixNLRI[1:1+prefixOctets])
	return octets, true
}
