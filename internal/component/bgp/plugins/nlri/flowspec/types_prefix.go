// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec prefix components
// RFC: rfc/short/rfc5575.md
// Overview: types.go — core FlowSpec types, constants, and interface
// Related: types_numeric.go — numeric/bitmask component implementations
// Related: types_vpn.go — FlowSpec VPN wrapper

package flowspec

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// Prefix components (Type 1: Destination, Type 2: Source)
// RFC 8955 Section 4.2.2.1-2 defines the prefix component encoding.

type prefixComponent struct {
	compType FlowComponentType
	prefix   netip.Prefix
	offset   uint8 // IPv6 offset per RFC 8956 (0 for IPv4)
}

// NewFlowDestPrefixComponent creates a destination prefix component (Type 1).
// RFC 8955 Section 4.2.2.1: Defines the destination prefix to match.
func NewFlowDestPrefixComponent(prefix netip.Prefix) FlowComponent {
	return &prefixComponent{compType: FlowDestPrefix, prefix: prefix}
}

// NewFlowSourcePrefixComponent creates a source prefix component (Type 2).
// RFC 8955 Section 4.2.2.2: Defines the source prefix to match.
func NewFlowSourcePrefixComponent(prefix netip.Prefix) FlowComponent {
	return &prefixComponent{compType: FlowSourcePrefix, prefix: prefix}
}

// newFlowDestPrefixComponentWithOffset creates an IPv6 destination prefix with offset.
// The offset field is defined in RFC 8956 for IPv6 FlowSpec.
func newFlowDestPrefixComponentWithOffset(prefix netip.Prefix, offset uint8) FlowComponent {
	return &prefixComponent{compType: FlowDestPrefix, prefix: flowPrefixPattern(prefix, offset), offset: offset}
}

// newFlowSourcePrefixComponentWithOffset creates an IPv6 source prefix with offset.
// The offset field is defined in RFC 8956 for IPv6 FlowSpec.
func newFlowSourcePrefixComponentWithOffset(prefix netip.Prefix, offset uint8) FlowComponent {
	return &prefixComponent{compType: FlowSourcePrefix, prefix: flowPrefixPattern(prefix, offset), offset: offset}
}

// Skipped address bits are not part of an IPv6 pattern. Normalize once so
// configured and received components use the same precedence representation.
func flowPrefixPattern(prefix netip.Prefix, offset uint8) netip.Prefix {
	if offset == 0 {
		return prefix.Masked()
	}
	addr := prefix.Addr().As16()
	clear(addr[:int(offset)/8])
	addr[int(offset)/8] &= 0xff >> (offset % 8)
	return netip.PrefixFrom(netip.AddrFrom16(addr), prefix.Bits()).Masked()
}

func (c *prefixComponent) Type() FlowComponentType { return c.compType }
func (c *prefixComponent) Prefix() netip.Prefix    { return c.prefix }
func (c *prefixComponent) Offset() uint8           { return c.offset }

// Bytes returns the wire encoding per RFC 8955 Section 4.2.2.1-2.
// IPv4: <type (1), length (1), prefix (variable)>.
// IPv6: <type (1), length (1), offset (1), prefix (variable)> per RFC 8956.
func (c *prefixComponent) Bytes() []byte {
	data := make([]byte, c.Len())
	c.WriteTo(data, 0)
	return data
}

// String returns command-style format: "<keyword> <prefix>".
// Example: "destination-ipv4 10.0.0.0/24" or "source-ipv6 2001:db8::/32".
//
// The keyword names the family, and this component holds the address that
// decides it. FlowComponentType.String() answers from the type code alone,
// which RFC 8955 and RFC 8956 share for these two, so it cannot.
func (c *prefixComponent) String() string {
	return c.keyword() + " " + c.prefix.String()
}

// keyword answers the component keyword for this prefix, by family.
func (c *prefixComponent) keyword() string {
	v6 := c.prefix.Addr().Is6()
	if c.compType == FlowSourcePrefix {
		if v6 {
			return kwSourceIPv6
		}
		return kwSourceIPv4
	}
	if v6 {
		return kwDestinationIPv6
	}
	return kwDestinationIPv4
}

// Len returns the wire-format length in bytes.
func (c *prefixComponent) Len() int {
	bits := c.prefix.Bits()
	if c.prefix.Addr().Is6() {
		return 3 + (bits-int(c.offset)+7)/8
	}
	return 2 + (bits+7)/8
}

// WriteTo writes the component directly to buf at offset.
// Returns bytes written.
func (c *prefixComponent) WriteTo(buf []byte, off int) int {
	bits := c.prefix.Bits()
	addr := c.prefix.Masked().Addr()
	buf[off] = byte(c.compType)
	buf[off+1] = byte(bits)
	if addr.Is6() {
		buf[off+2] = c.offset
		ip := addr.As16()
		patternBits := bits - int(c.offset)
		size := (patternBits + 7) / 8
		start, shift := int(c.offset)/8, c.offset%8
		for i := range size {
			b := ip[start+i] << shift
			if shift != 0 && start+i+1 < len(ip) {
				b |= ip[start+i+1] >> (8 - shift)
			}
			buf[off+3+i] = b
		}
		if size != 0 && patternBits%8 != 0 {
			buf[off+2+size] &= 0xff << (8 - patternBits%8)
		}
		return 3 + size
	}
	ip := addr.As4()
	size := (bits + 7) / 8
	copy(buf[off+2:], ip[:size])
	return 2 + size
}

// validate checks the address layout before a configured component is attached
// to its enclosing NLRI. RFC 8956 Section 3.1 -- see rfc/short/rfc8956.md.
func (c *prefixComponent) validate(afi AFI) error {
	if !c.prefix.IsValid() {
		return fmt.Errorf("flowspec: invalid prefix")
	}
	if c.prefix.Addr().Is4() != (afi == AFIIPv4) {
		return fmt.Errorf("flowspec: prefix address family differs from NLRI")
	}
	if c.offset == 0 {
		return nil
	}
	if afi != AFIIPv6 {
		return fmt.Errorf("flowspec: IPv4 prefix has a nonzero offset")
	}
	if int(c.offset) >= c.prefix.Bits() {
		return fmt.Errorf("flowspec: prefix offset must be below its length")
	}
	return nil
}

// CheckedWriteTo validates capacity before writing.
func (c *prefixComponent) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := c.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return c.WriteTo(buf, off), nil
}

// parsePrefixComponent parses a prefix-type component (Type 1 or 2).
// RFC 8955 Section 4.2.2.1-2 defines the IPv4 encoding:
//
//	<type (1 octet), length (1 octet), prefix (variable)>
//
// RFC 8956 Section 3.1 defines the IPv6 encoding with offset field:
//
//	<type (1 octet), length (1 octet), offset (1 octet), prefix (variable)>
//
// The offset field in IPv6 allows matching on a portion of the prefix.
func parsePrefixComponent(t FlowComponentType, data []byte, fam Family) (FlowComponent, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrFlowSpecTruncated
	}
	prefixLen := int(data[0])
	var offset uint8
	headerLen, maximum := 1, 32
	if fam.AFI == AFIIPv6 {
		if len(data) < 2 {
			return nil, nil, ErrFlowSpecTruncated
		}
		offset = data[1]
		headerLen, maximum = 2, 128
	}
	if prefixLen > maximum || (offset != 0 && int(offset) >= prefixLen) {
		return nil, nil, fmt.Errorf("flowspec: invalid prefix length %d at offset %d", prefixLen, offset)
	}
	prefixBytes := (prefixLen - int(offset) + 7) / 8
	if len(data) < headerLen+prefixBytes {
		return nil, nil, ErrFlowSpecTruncated
	}
	var addr netip.Addr
	if fam.AFI == AFIIPv4 {
		var ip [4]byte
		copy(ip[:], data[headerLen:headerLen+prefixBytes])
		addr = netip.AddrFrom4(ip)
	} else {
		var ip [16]byte
		start, shift := int(offset)/8, offset%8
		for i, b := range data[headerLen : headerLen+prefixBytes] {
			ip[start+i] |= b >> shift
			if shift != 0 && start+i+1 < len(ip) {
				ip[start+i+1] |= b << (8 - shift)
			}
		}
		addr = netip.AddrFrom16(ip)
	}
	prefix := netip.PrefixFrom(addr, prefixLen).Masked()
	var comp FlowComponent
	if t == FlowDestPrefix {
		comp = newFlowDestPrefixComponentWithOffset(prefix, offset)
	} else {
		comp = newFlowSourcePrefixComponentWithOffset(prefix, offset)
	}
	return comp, data[headerLen+prefixBytes:], nil
}
