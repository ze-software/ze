// Design: docs/architecture/ospf/ospfv3-1-types.md -- OSPFv3 24-bit Options bitset.
// RFC: rfc/short/rfc5340.md (§A.2 Options), rfc/short/rfc5838.md (AF-bit)
//
// OSPFv3 widens the OSPFv2 8-bit Options to 24 bits, carried in Hello, DD, and several
// LSAs. This leaf type stores the field in a uint32, validates the 24-bit bound, and
// serializes the 3 wire octets big-endian.

package types

// Options is the OSPFv3 24-bit Options bitset (stored in a uint32).
type Options uint32

// OSPFv3 Options bits (RFC 5340 §A.2, RFC 5838 §2.4 AF).
const (
	OptV6 Options = 0x000001 // V6-bit: include the router/link in IPv6 routing calculations
	OptE  Options = 0x000002 // E-bit: AS-external LSAs flooded (not a stub area)
	OptN  Options = 0x000008 // N-bit: NSSA support
	OptR  Options = 0x000010 // R-bit: router is an active participant (forwards transit)
	// OptAF is the AF-bit (RFC 5838 §2.4): a router that supports multiple address
	// families sets it in the Hello and DD Options. Bit 8 (0x000100) per the IANA
	// "OSPFv3 Options (24 bits)" registry; it does not collide with V6/E/N/R.
	OptAF Options = 0x000100
)

// OptionsFromBytes reads 3 big-endian octets from b[off:] into Options.
func OptionsFromBytes(b []byte, off int) (Options, error) {
	if off < 0 || off+3 > len(b) {
		return 0, ErrWrongLength
	}
	return Options(uint32(b[off])<<16 | uint32(b[off+1])<<8 | uint32(b[off+2])), nil
}

// WriteTo writes the 3 big-endian octets into buf at off and returns 3.
func (o Options) WriteTo(buf []byte, off int) int {
	buf[off] = byte(o >> 16)
	buf[off+1] = byte(o >> 8)
	buf[off+2] = byte(o)
	return 3
}

// Has reports whether all the bits in mask are set.
func (o Options) Has(mask Options) bool { return o&mask == mask }

// V6 reports the V6-bit.
func (o Options) V6() bool { return o.Has(OptV6) }

// External reports the E-bit (AS-external LSAs flooded).
func (o Options) External() bool { return o.Has(OptE) }

// Router reports the R-bit (active router).
func (o Options) Router() bool { return o.Has(OptR) }

// NSSA reports the N-bit (NSSA support).
func (o Options) NSSA() bool { return o.Has(OptN) }

// AF reports the AF-bit (RFC 5838 §2.4: address-family support).
func (o Options) AF() bool { return o.Has(OptAF) }

// SetAF returns o with the AF-bit set (RFC 5838 §2.4).
func (o Options) SetAF() Options { return o | OptAF }
