// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec prefix components
// RFC: rfc/short/rfc5575.md
// Overview: types.go — core FlowSpec types, constants, and interface
// Related: types_numeric.go — numeric/bitmask component implementations
// Related: types_vpn.go — FlowSpec VPN wrapper

package bgp_nlri_flowspec

import (
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wire"
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

// NewFlowDestPrefixComponentWithOffset creates an IPv6 destination prefix with offset.
// The offset field is defined in RFC 8956 for IPv6 FlowSpec.
func NewFlowDestPrefixComponentWithOffset(prefix netip.Prefix, offset uint8) FlowComponent {
	return &prefixComponent{compType: FlowDestPrefix, prefix: prefix, offset: offset}
}

// NewFlowSourcePrefixComponentWithOffset creates an IPv6 source prefix with offset.
// The offset field is defined in RFC 8956 for IPv6 FlowSpec.
func NewFlowSourcePrefixComponentWithOffset(prefix netip.Prefix, offset uint8) FlowComponent {
	return &prefixComponent{compType: FlowSourcePrefix, prefix: prefix, offset: offset}
}

func (c *prefixComponent) Type() FlowComponentType { return c.compType }
func (c *prefixComponent) Prefix() netip.Prefix    { return c.prefix }
func (c *prefixComponent) Offset() uint8           { return c.offset }

// Bytes returns the wire encoding per RFC 8955 Section 4.2.2.1-2.
// IPv4: <type (1), length (1), prefix (variable)>.
// IPv6: <type (1), length (1), offset (1), prefix (variable)> per RFC 8956.
func (c *prefixComponent) Bytes() []byte {
	bits := c.prefix.Bits()
	addr := c.prefix.Addr()

	// IPv6 FlowSpec prefixes include an offset byte (RFC 8956)
	if addr.Is6() {
		// Calculate bytes needed for the prefix data (from offset to prefix length)
		// The prefix length field includes offset
		prefixBytes := (bits + 7) / 8
		data := make([]byte, 3+prefixBytes)
		data[0] = byte(c.compType)
		data[1] = byte(bits)
		data[2] = c.offset

		ip6 := addr.As16()
		copy(data[3:], ip6[:prefixBytes])
		return data
	}

	// IPv4: RFC 8955 encoding - no offset byte
	prefixBytes := (bits + 7) / 8
	data := make([]byte, 2+prefixBytes)
	data[0] = byte(c.compType)
	data[1] = byte(bits)

	ip4 := addr.As4()
	copy(data[2:], ip4[:prefixBytes])
	return data
}

// String returns command-style format: "<type> <prefix>".
// Example: "destination 10.0.0.0/24" or "source 192.168.0.0/16".
func (c *prefixComponent) String() string {
	return fmt.Sprintf("%s %s", c.compType, c.prefix)
}

// Len returns the wire-format length in bytes.
func (c *prefixComponent) Len() int {
	bits := c.prefix.Bits()
	prefixBytes := (bits + 7) / 8
	if c.prefix.Addr().Is6() {
		return 3 + prefixBytes // type + length + offset + prefix
	}
	return 2 + prefixBytes // type + length + prefix
}

// WriteTo writes the component directly to buf at offset.
// Returns bytes written.
func (c *prefixComponent) WriteTo(buf []byte, off int) int {
	bits := c.prefix.Bits()
	addr := c.prefix.Addr()
	prefixBytes := (bits + 7) / 8

	pos := off
	buf[pos] = byte(c.compType)
	pos++
	buf[pos] = byte(bits)
	pos++

	if addr.Is6() {
		buf[pos] = c.offset
		pos++
		ip6 := addr.As16()
		copy(buf[pos:], ip6[:prefixBytes])
		pos += prefixBytes
	} else {
		ip4 := addr.As4()
		copy(buf[pos:], ip4[:prefixBytes])
		pos += prefixBytes
	}

	return pos - off
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
func parsePrefixComponent(t FlowComponentType, data []byte, family Family) (FlowComponent, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrFlowSpecTruncated
	}

	prefixLen := int(data[0])
	prefixBytes := (prefixLen + 7) / 8

	// IPv6 FlowSpec includes an offset byte per RFC 8956 Section 3.1
	var offset uint8
	headerLen := 1 // Just prefix length for IPv4
	if family.AFI == AFIIPv6 {
		if len(data) < 2 {
			return nil, nil, ErrFlowSpecTruncated
		}
		offset = data[1]
		headerLen = 2 // Prefix length + offset for IPv6
	}

	if len(data) < headerLen+prefixBytes {
		return nil, nil, ErrFlowSpecTruncated
	}

	// Build prefix - encoding matches RFC 4271 prefix encoding
	var addr netip.Addr
	if family.AFI == AFIIPv4 {
		var ip [4]byte
		copy(ip[:], data[1:1+prefixBytes])
		addr = netip.AddrFrom4(ip)
	} else {
		var ip [16]byte
		copy(ip[:], data[headerLen:headerLen+prefixBytes])
		addr = netip.AddrFrom16(ip)
	}

	prefix := netip.PrefixFrom(addr, prefixLen)

	var comp FlowComponent
	if t == FlowDestPrefix {
		if offset > 0 {
			comp = NewFlowDestPrefixComponentWithOffset(prefix, offset)
		} else {
			comp = NewFlowDestPrefixComponent(prefix)
		}
	} else {
		if offset > 0 {
			comp = NewFlowSourcePrefixComponentWithOffset(prefix, offset)
		} else {
			comp = NewFlowSourcePrefixComponent(prefix)
		}
	}

	return comp, data[headerLen+prefixBytes:], nil
}
