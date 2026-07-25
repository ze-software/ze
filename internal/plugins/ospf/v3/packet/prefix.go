// Design: plan/learned/969-ospfv3-2-wire.md -- OSPFv3 IPv6 prefix encode/decode (both carriage forms).
// RFC: rfc/short/rfc5340.md (§A.4.1 IPv6 prefix representation)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// prefixHeaderLen is the fixed 4-octet prefix header in the repeating-entry
// carriage form: PrefixLength(1) + PrefixOptions(1) + 16-bit field(2).
const prefixHeaderLen = 4

// Prefix is a decoded OSPFv3 IPv6 prefix (RFC 5340 §A.4.1). Field16 is the
// type-specific 16-bit field that follows PrefixOptions: a Metric in the
// Intra-Area-Prefix-LSA, Reserved (0) in the Link-LSA, and Reserved (0) in the
// inlined inter-area/external carriers. Address holds exactly ByteLen octets of
// the prefix, padded to a 32-bit word with zero bits past the prefix length.
type Prefix struct {
	Length  types.PrefixLength
	Options types.PrefixOptions
	Field16 uint16
	Address []byte
}

// decodePrefix parses one prefix in the repeating-entry carriage form
// (PrefixLength + PrefixOptions + 16-bit field + AddressPrefix) from buf[off:]
// and returns the decoded prefix and the byte count it consumed. It bound-checks
// the 4-octet header and the address bytes, and validates the zero padding past
// the prefix length (RFC 5340 §A.4.1).
func decodePrefix(buf []byte, off int) (Prefix, int, error) {
	if off < 0 || off+prefixHeaderLen > len(buf) {
		return Prefix{}, 0, ErrTruncated
	}
	plen, err := types.NewPrefixLength(buf[off])
	if err != nil {
		return Prefix{}, 0, err
	}
	opts := types.PrefixOptions(buf[off+1])
	field := readUint16(buf, off+2)
	addrLen := plen.ByteLen()
	addrOff := off + prefixHeaderLen
	if addrOff+addrLen > len(buf) {
		return Prefix{}, 0, ErrTruncated
	}
	addr := buf[addrOff : addrOff+addrLen]
	// RFC 5340 §A.4.1: "bits ... beyond the prefix length ... MUST be zero."
	if err := plen.ValidatePadding(addr); err != nil {
		return Prefix{}, 0, err
	}
	out := Prefix{Length: plen, Options: opts, Field16: field, Address: addr}
	return out, prefixHeaderLen + addrLen, nil
}

// decodeInlinePrefix parses the inlined carriage form used by the Inter-Area-
// Prefix, AS-External, and NSSA LSAs: the LSA lays PrefixLength, PrefixOptions,
// and its 16-bit field individually in its fixed part, then the AddressPrefix.
// lenOff, optsOff, and field16Off are the LSA-body offsets of those three
// fields, addrOff is the offset of the AddressPrefix; the function returns the
// decoded prefix and the AddressPrefix byte count it consumed.
func decodeInlinePrefix(buf []byte, lenOff, optsOff, field16Off, addrOff int) (Prefix, int, error) {
	if lenOff < 0 || optsOff < 0 || field16Off < 0 || addrOff < 0 {
		return Prefix{}, 0, ErrTruncated
	}
	if lenOff >= len(buf) || optsOff >= len(buf) || field16Off+2 > len(buf) {
		return Prefix{}, 0, ErrTruncated
	}
	plen, err := types.NewPrefixLength(buf[lenOff])
	if err != nil {
		return Prefix{}, 0, err
	}
	opts := types.PrefixOptions(buf[optsOff])
	field := readUint16(buf, field16Off)
	addrLen := plen.ByteLen()
	if addrOff+addrLen > len(buf) {
		return Prefix{}, 0, ErrTruncated
	}
	addr := buf[addrOff : addrOff+addrLen]
	if err := plen.ValidatePadding(addr); err != nil {
		return Prefix{}, 0, err
	}
	return Prefix{Length: plen, Options: opts, Field16: field, Address: addr}, addrLen, nil
}

// encodedLen returns the repeating-entry on-wire size of the prefix.
func (p Prefix) encodedLen() int { return prefixHeaderLen + p.Length.ByteLen() }

// writeTo serializes the prefix in the repeating-entry carriage form into buf at
// off and returns the new offset. The address slice is padded or truncated to
// ByteLen so a caller-supplied address that is shorter than the padded width
// still emits the correct on-wire length with zero padding.
func (p Prefix) writeTo(buf []byte, off int) int {
	buf[off] = byte(p.Length)
	buf[off+1] = byte(p.Options)
	off += 2
	off += writeUint16(buf, off, p.Field16)
	off = writePrefixAddress(buf, off, p.Length, p.Address)
	return off
}

// writePrefixAddress writes the ByteLen-padded AddressPrefix into buf at off,
// copying at most ByteLen octets from addr and zero-filling the remainder.
func writePrefixAddress(buf []byte, off int, plen types.PrefixLength, addr []byte) int {
	addrLen := plen.ByteLen()
	n := copy(buf[off:off+addrLen], addr)
	for i := n; i < addrLen; i++ {
		buf[off+i] = 0
	}
	return off + addrLen
}
