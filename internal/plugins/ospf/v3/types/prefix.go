// Design: docs/architecture/ospf/ospfv3-1-types.md -- IPv6 prefix length + options encoding.
// RFC: rfc/short/rfc5340.md (§A.4.1 IPv6 prefix representation, PrefixOptions)
//
// OSPFv3 encodes an IPv6 prefix as a PrefixLength (bits), a PrefixOptions octet, and the
// prefix bytes padded UP to a 32-bit word boundary: ByteLen = ((PrefixLength+31)/32)*4.
// Bits beyond PrefixLength in the final word MUST be zero on the wire.

package types

// MaxPrefixLength is the longest IPv6 prefix in bits.
const MaxPrefixLength = 128

// PrefixLength is an IPv6 prefix length in bits (0..128).
type PrefixLength uint8

// NewPrefixLength validates bits as 0..128.
func NewPrefixLength(bits uint8) (PrefixLength, error) {
	if bits > MaxPrefixLength {
		return 0, ErrOutOfRange
	}
	return PrefixLength(bits), nil
}

// wordLen returns the number of 32-bit words used to carry the prefix bytes.
func (p PrefixLength) wordLen() int { return (int(p) + 31) / 32 }

// ByteLen returns the padded prefix byte length on the wire: wordLen * 4.
func (p PrefixLength) ByteLen() int { return p.wordLen() * 4 }

// ValidatePadding checks that b is exactly ByteLen bytes and that every bit beyond the
// prefix length is zero (RFC 5340 §A.4.1 padding must be zero).
func (p PrefixLength) ValidatePadding(b []byte) error {
	if len(b) != p.ByteLen() {
		return ErrWrongLength
	}
	bits := int(p)
	for i := bits; i < len(b)*8; i++ {
		if b[i/8]&(0x80>>(uint(i)%8)) != 0 {
			return ErrMalformed
		}
	}
	return nil
}

// PrefixOptions is the OSPFv3 8-bit prefix options octet.
type PrefixOptions uint8

// PrefixOptions bits (RFC 5340 §A.4.1.1).
const (
	OptPrefixNU PrefixOptions = 0x01 // NU: prefix excluded from IPv6 unicast calculations
	OptPrefixLA PrefixOptions = 0x02 // LA: prefix is an actual local interface address
	OptPrefixP  PrefixOptions = 0x08 // P: propagate (NSSA, set so the prefix is re-advertised)
	OptPrefixDN PrefixOptions = 0x10 // DN: down-bit (loop prevention for L3VPN)
)

// Has reports whether all the bits in mask are set.
func (o PrefixOptions) Has(mask PrefixOptions) bool { return o&mask == mask }

// NoUnicast reports the NU-bit.
func (o PrefixOptions) NoUnicast() bool { return o.Has(OptPrefixNU) }

// LocalAddress reports the LA-bit.
func (o PrefixOptions) LocalAddress() bool { return o.Has(OptPrefixLA) }

// Propagate reports the P-bit.
func (o PrefixOptions) Propagate() bool { return o.Has(OptPrefixP) }

// Down reports the DN-bit.
func (o PrefixOptions) Down() bool { return o.Has(OptPrefixDN) }
