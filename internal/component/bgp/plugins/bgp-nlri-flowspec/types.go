// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin
// RFC: rfc/short/rfc5575.md
// Detail: types_prefix.go — prefix component (Type 1-2) struct, constructors, parsing
// Detail: types_numeric.go — numeric/bitmask component (Types 3-13) struct, constructors, parsing
// Detail: types_vpn.go — FlowSpec VPN wrapper (SAFI 134) struct, methods, parsing
//
// Package flowspec implements FlowSpec NLRI types for the flowspec plugin.
// RFC 8955: Dissemination of Flow Specification Rules (IPv4 FlowSpec)
// RFC 8956: Dissemination of Flow Specification Rules for IPv6
package bgp_nlri_flowspec

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
)

// Type aliases for nlri types used by FlowSpec.
// This allows the flowspec package to be self-contained while reusing nlri definitions.
type (
	Family             = nlri.Family
	AFI                = nlri.AFI
	SAFI               = nlri.SAFI
	RouteDistinguisher = nlri.RouteDistinguisher
)

// Re-export type from nlri for RouteDistinguisher.
type RDType = nlri.RDType

// Re-export constants from nlri for local use.
const (
	AFIIPv4         = nlri.AFIIPv4
	AFIIPv6         = nlri.AFIIPv6
	SAFIFlowSpec    = nlri.SAFIFlowSpec
	SAFIFlowSpecVPN = nlri.SAFIFlowSpecVPN
	RDType0         = nlri.RDType0
	RDType1         = nlri.RDType1
	RDType2         = nlri.RDType2
)

// Re-export family constants from nlri.
var (
	IPv4FlowSpec    = nlri.IPv4FlowSpec
	IPv6FlowSpec    = nlri.IPv6FlowSpec
	IPv4FlowSpecVPN = nlri.IPv4FlowSpecVPN
	IPv6FlowSpecVPN = nlri.IPv6FlowSpecVPN
)

// ParseRouteDistinguisher parses an 8-byte Route Distinguisher.
var ParseRouteDistinguisher = nlri.ParseRouteDistinguisher

// FlowSpec errors.
var (
	ErrFlowSpecTruncated       = errors.New("flowspec: truncated data")
	ErrFlowSpecInvalidType     = errors.New("flowspec: invalid component type")
	ErrFlowSpecInvalidOperator = errors.New("flowspec: invalid operator")
)

// FlowComponentType identifies a FlowSpec component.
// RFC 8955 Section 4.2.2 defines the component types for IPv4 FlowSpec.
// RFC 8956 extends this for IPv6 (component type 13 - Flow Label).
type FlowComponentType uint8

// FlowSpec component types per RFC 8955 Section 4.2.2.
// Each component type defines a matching criterion for traffic classification.
// Components MUST follow strict type ordering by increasing numerical order
// (RFC 8955 Section 4.2).
const (
	FlowDestPrefix   FlowComponentType = 1  // Type 1: Destination Prefix (RFC 8955 Section 4.2.2.1)
	FlowSourcePrefix FlowComponentType = 2  // Type 2: Source Prefix (RFC 8955 Section 4.2.2.2)
	FlowIPProtocol   FlowComponentType = 3  // Type 3: IP Protocol (RFC 8955 Section 4.2.2.3)
	FlowPort         FlowComponentType = 4  // Type 4: Port (src or dst) (RFC 8955 Section 4.2.2.4)
	FlowDestPort     FlowComponentType = 5  // Type 5: Destination Port (RFC 8955 Section 4.2.2.5)
	FlowSourcePort   FlowComponentType = 6  // Type 6: Source Port (RFC 8955 Section 4.2.2.6)
	FlowICMPType     FlowComponentType = 7  // Type 7: ICMP Type (RFC 8955 Section 4.2.2.7)
	FlowICMPCode     FlowComponentType = 8  // Type 8: ICMP Code (RFC 8955 Section 4.2.2.8)
	FlowTCPFlags     FlowComponentType = 9  // Type 9: TCP Flags (RFC 8955 Section 4.2.2.9)
	FlowPacketLength FlowComponentType = 10 // Type 10: Packet Length (RFC 8955 Section 4.2.2.10)
	FlowDSCP         FlowComponentType = 11 // Type 11: DSCP (RFC 8955 Section 4.2.2.11)
	FlowFragment     FlowComponentType = 12 // Type 12: Fragment (RFC 8955 Section 4.2.2.12)
	FlowFlowLabel    FlowComponentType = 13 // Type 13: Flow Label - IPv6 only (RFC 8956)
)

// String returns the command-style component type name.
// These names match the API input syntax for round-trip compatibility.
func (t FlowComponentType) String() string {
	switch t {
	case FlowDestPrefix:
		return "destination"
	case FlowSourcePrefix:
		return "source"
	case FlowIPProtocol:
		return "protocol"
	case FlowPort:
		return "port"
	case FlowDestPort:
		return "destination-port"
	case FlowSourcePort:
		return "source-port"
	case FlowICMPType:
		return "icmp-type"
	case FlowICMPCode:
		return "icmp-code"
	case FlowTCPFlags:
		return "tcp-flags"
	case FlowPacketLength:
		return "packet-length"
	case FlowDSCP:
		return "dscp"
	case FlowFragment:
		return "fragment"
	case FlowFlowLabel:
		return "flow-label"
	default:
		return fmt.Sprintf("type(%d)", t)
	}
}

// FlowOperator represents the numeric operator byte in FlowSpec.
// RFC 8955 Section 4.2.1.1 defines the Numeric Operator (numeric_op) format:
//
//	  0   1   2   3   4   5   6   7
//	+---+---+---+---+---+---+---+---+
//	| e | a |  len  | 0 |lt |gt |eq |
//	+---+---+---+---+---+---+---+---+
//
// e (bit 0): end-of-list - set in the last {op, value} pair
// a (bit 1): AND bit - if unset, OR with previous; if set, AND with previous
// len (bits 2-3): value length as (1 << len): 1, 2, 4, or 8 octets
// bit 4: reserved, MUST be 0
// lt (bit 5): less-than comparison.
// gt (bit 6): greater-than comparison.
// eq (bit 7): equality comparison.
type FlowOperator byte

// Numeric operator flags per RFC 8955 Section 4.2.1.1.
// The lt, gt, eq bits can be combined per RFC 8955 Table 1:
//
//	lt=0 gt=0 eq=0: false (never matches)
//	lt=0 gt=0 eq=1: == (equal)
//	lt=0 gt=1 eq=0: > (greater than)
//	lt=0 gt=1 eq=1: >= (greater than or equal)
//	lt=1 gt=0 eq=0: < (less than)
//	lt=1 gt=0 eq=1: <= (less than or equal)
//	lt=1 gt=1 eq=0: != (not equal)
//	lt=1 gt=1 eq=1: true (always matches)
const (
	FlowOpEnd     FlowOperator = 0x80 // Bit 0: End of list marker
	FlowOpAnd     FlowOperator = 0x40 // Bit 1: AND with previous (vs OR)
	FlowOpLenMask FlowOperator = 0x30 // Bits 2-3: Length field (1<<len bytes)
	FlowOpLess    FlowOperator = 0x04 // Bit 5: Less than
	FlowOpGreater FlowOperator = 0x02 // Bit 6: Greater than
	FlowOpEqual   FlowOperator = 0x01 // Bit 7: Equal
	FlowOpNotEq   FlowOperator = 0x06 // LT | GT = Not equal (RFC 8955 Table 1)
)

// Bitmask operator flags per RFC 8955 Section 4.2.1.2.
// Used for TCP flags (Type 9) and Fragment (Type 12) components.
//
//	Byte format: [e][a][len][0][0][not][m]
//	- e, a, len: same as numeric operator (bits 0-3)
//	- not (bit 5): logical negation of operation
//	- m (bit 6): Match bit - exact match vs any-bit-set
//
// Match semantics:
//   - m=0: (data AND value) != 0 (any bit set)
//   - m=1: (data AND value) == value (exact match)
//   - not=1: logical negation of the above
const (
	FlowOpNot   FlowOperator = 0x02 // Bit 5: NOT operator (negate result)
	FlowOpMatch FlowOperator = 0x01 // Bit 6: Match operator (exact match vs any-bit-set)
)

// FlowMatch represents a single {operator, value} pair in a numeric component.
// RFC 8955 Section 4.2.1.1 defines the encoding as operator byte followed by value.
// Multiple FlowMatch entries form a logical expression evaluated left-to-right,
// where the AND operator has higher priority than OR.
type FlowMatch struct {
	Op    FlowOperator // Comparison operator bits (GT, LT, EQ combinations)
	And   bool         // AND with previous match (vs OR) - RFC 8955 Section 4.2.1.1 'a' bit
	Value uint64       // The value to match against packet field
}

// Fragment bitmask flags for FlowFragment component (Type 12).
// RFC 8955 Section 4.2.2.12 defines the fragment bitmask operand:
//
//	  0   1   2   3   4   5   6   7
//	+---+---+---+---+---+---+---+---+
//	| 0 | 0 | 0 | 0 |LF |FF |IsF|DF |
//	+---+---+---+---+---+---+---+---+
//
// DF (bit 0): Don't Fragment - match if IP Header Flags Bit-1 (DF) is 1.
// IsF (bit 1): Is a Fragment - match if Fragment Offset is not 0.
// FF (bit 2): First Fragment - match if Fragment Offset is 0 AND MF is 1.
// LF (bit 3): Last Fragment - match if Fragment Offset is not 0 AND MF is 0.
const (
	FlowFragDontFragment  FlowFragmentFlag = 0x01 // DF: Don't Fragment flag set
	FlowFragIsFragment    FlowFragmentFlag = 0x02 // IsF: Is a fragment (offset != 0)
	FlowFragFirstFragment FlowFragmentFlag = 0x04 // FF: First fragment (offset=0, MF=1)
	FlowFragLastFragment  FlowFragmentFlag = 0x08 // LF: Last fragment (offset!=0, MF=0)
)

// FlowFragmentFlag represents fragment matching flags per RFC 8955 Section 4.2.2.12.
type FlowFragmentFlag byte

// FlowComponent is the interface for FlowSpec components.
// Each component represents a matching criterion as defined in RFC 8955 Section 4.2.2.
type FlowComponent interface {
	Type() FlowComponentType
	Bytes() []byte
	// Len returns the wire-format length in bytes.
	Len() int
	// WriteTo writes the component to buf at offset, returning bytes written.
	WriteTo(buf []byte, off int) int
	String() string
}

// FlowSpec represents a FlowSpec NLRI per RFC 8955 Section 4.
// A FlowSpec is an n-tuple of matching criteria encoded as BGP NLRI.
// The NLRI format is defined in RFC 8955 Figure 1:
//
//	+-------------------------------+
//	|    length (0xnn or 0xfnnn)    |
//	+-------------------------------+
//	|    NLRI value   (variable)    |
//	+-------------------------------+
//
// AFI=1, SAFI=133 for IPv4 FlowSpec (RFC 8955 Section 4).
// AFI=2, SAFI=133 for IPv6 FlowSpec (RFC 8956).
type FlowSpec struct {
	family     Family
	components []FlowComponent
	cached     []byte
}

// NewFlowSpec creates a new FlowSpec NLRI.
func NewFlowSpec(family Family) *FlowSpec {
	return &FlowSpec{
		family:     family,
		components: make([]FlowComponent, 0, 4),
	}
}

// Family returns the address family.
func (f *FlowSpec) Family() Family {
	return f.family
}

// Components returns the FlowSpec components.
func (f *FlowSpec) Components() []FlowComponent {
	return f.components
}

// AddComponent adds a component to the FlowSpec.
func (f *FlowSpec) AddComponent(c FlowComponent) {
	f.components = append(f.components, c)
	f.cached = nil // Invalidate cache
}

// ComponentBytes returns the wire-format encoding of components without length prefix.
// This is used for FlowSpec VPN where the VPN wrapper provides its own length.
// Components are sorted by type per RFC 8955 Section 4.2:
// "Components MUST follow strict type ordering by increasing numerical order.".
func (f *FlowSpec) ComponentBytes() []byte {
	// Sort components by type (RFC 8955 Section 4.2 requires strict ordering)
	sorted := make([]FlowComponent, len(f.components))
	copy(sorted, f.components)
	slices.SortFunc(sorted, func(a, b FlowComponent) int {
		return int(a.Type()) - int(b.Type())
	})

	// Estimate capacity: each component has at least 2 bytes (type + operator/value)
	// but typically more. We use 4 bytes per component as a reasonable estimate.
	data := make([]byte, 0, len(sorted)*4)
	for _, c := range sorted {
		data = append(data, c.Bytes()...)
	}
	return data
}

// Bytes returns the wire-format encoding (with length prefix).
// RFC 8955 Section 4.1 defines the length encoding:
// - If length < 240 (0xf0): single octet length.
// - If length >= 240: 2-octet extended length with high nibble 0xf.
//
// Example from RFC 8955: 239 -> 0xef (1 octet), 240 -> 0xf0f0 (2 octets).
func (f *FlowSpec) Bytes() []byte {
	if f.cached != nil {
		return f.cached
	}

	// Encode components
	data := f.ComponentBytes()

	// Add NLRI length prefix per RFC 8955 Section 4.1
	if len(data) < 240 {
		// Single octet length (values 0x00-0xef)
		f.cached = append([]byte{byte(len(data))}, data...)
	} else {
		// Extended length (2 bytes): 0xfnnn format
		// High nibble is 0xf, remaining 12 bits encode length (max 4095)
		f.cached = make([]byte, 2+len(data))
		f.cached[0] = 0xF0 | byte(len(data)>>8)
		f.cached[1] = byte(len(data))
		copy(f.cached[2:], data)
	}

	return f.cached
}

// Len returns the length in bytes.
func (f *FlowSpec) Len() int {
	return len(f.Bytes())
}

// PathID returns 0 (FlowSpec doesn't use ADD-PATH).
func (f *FlowSpec) PathID() uint32 {
	return 0
}

// HasPathID returns false.
func (f *FlowSpec) HasPathID() bool {
	return false
}

// SupportsAddPath returns false - FlowSpec doesn't support ADD-PATH per RFC 8955.
func (f *FlowSpec) SupportsAddPath() bool {
	return false
}

// String returns command-style representation for API round-trip compatibility.
// Format: flow <component>+ where each component is "<type> <values>".
func (f *FlowSpec) String() string {
	if len(f.components) == 0 {
		return "flow"
	}
	parts := make([]string, len(f.components))
	for i, c := range f.components {
		parts[i] = c.String()
	}
	return "flow " + strings.Join(parts, " ")
}

// ComponentString returns just the components without the "flow" prefix.
// Used by FlowSpecVPN to embed components after the RD.
func (f *FlowSpec) ComponentString() string {
	if len(f.components) == 0 {
		return ""
	}
	parts := make([]string, len(f.components))
	for i, c := range f.components {
		parts[i] = c.String()
	}
	return strings.Join(parts, " ")
}

// ParseFlowSpec parses a FlowSpec from wire format per RFC 8955 Section 4.
// The NLRI consists of a length field followed by component data.
// RFC 8955 Section 4.1 defines length encoding:
// - Single octet if < 240.
// - Two octets (0xfnnn format) if >= 240.
func ParseFlowSpec(family Family, data []byte) (*FlowSpec, error) {
	if len(data) == 0 {
		return nil, ErrFlowSpecTruncated
	}

	// Parse length per RFC 8955 Section 4.1
	nlriLen := int(data[0])
	offset := 1
	if nlriLen >= 240 {
		// Extended length: high nibble is 0xf, extract 12-bit length
		if len(data) < 2 {
			return nil, ErrFlowSpecTruncated
		}
		nlriLen = int(data[0]&0x0F)<<8 | int(data[1])
		offset = 2
	}

	if len(data) < offset+nlriLen {
		return nil, ErrFlowSpecTruncated
	}

	fs := NewFlowSpec(family)
	remaining := data[offset : offset+nlriLen]

	// Parse components - RFC 8955 Section 4.2:
	// "A specific packet is considered to match the Flow Specification when
	// it matches the intersection (AND) of all the components present"
	for len(remaining) > 0 {
		comp, rest, err := parseFlowComponent(remaining, family)
		if err != nil {
			return nil, err
		}
		fs.components = append(fs.components, comp)
		remaining = rest
	}

	return fs, nil
}

// parseFlowComponent parses a single FlowSpec component.
// RFC 8955 Section 4.2.2: "The encoding of each of the components begins
// with a type field (1 octet) followed by a variable length parameter.".
func parseFlowComponent(data []byte, family Family) (FlowComponent, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrFlowSpecTruncated
	}

	compType := FlowComponentType(data[0])

	switch compType {
	case FlowDestPrefix, FlowSourcePrefix:
		// Type 1-2: Prefix components (RFC 8955 Section 4.2.2.1-2)
		return parsePrefixComponent(compType, data[1:], family)
	case FlowIPProtocol, FlowPort, FlowDestPort, FlowSourcePort,
		FlowICMPType, FlowICMPCode, FlowTCPFlags, FlowPacketLength,
		FlowDSCP, FlowFragment, FlowFlowLabel:
		// Type 3-13: Numeric/bitmask components (RFC 8955 Section 4.2.2.3-12)
		return parseNumericComponent(compType, data[1:])
	default:
		// RFC 8955 Section 4.2: unknown component type is malformed NLRI
		return nil, nil, ErrFlowSpecInvalidType
	}
}

// componentLen returns total length of all components in wire format.
func (f *FlowSpec) componentLen() int {
	n := 0
	for _, c := range f.components {
		n += c.Len()
	}
	return n
}

// writeComponentsSorted writes components in type order (RFC 8955 requirement).
// Returns bytes written. Uses iteration over type IDs to avoid allocation.
func (f *FlowSpec) writeComponentsSorted(buf []byte, off int) int {
	pos := off
	// RFC 8955 Section 4.2: Components MUST follow strict type ordering
	// Iterate type IDs 1-13 to write in sorted order without allocating
	for typeID := FlowComponentType(1); typeID <= FlowFlowLabel; typeID++ {
		for _, c := range f.components {
			if c.Type() == typeID {
				pos += c.WriteTo(buf, pos)
			}
		}
	}
	return pos - off
}

// WriteTo writes the FlowSpec NLRI directly to buf at offset (zero-alloc).
// RFC 8955 Section 4.1: Length encoding + sorted components.
func (f *FlowSpec) WriteTo(buf []byte, off int) int {
	// Fallback: if we have cached bytes but no components (parsed FlowSpec
	// where components weren't reconstructed), use cached bytes
	if len(f.components) == 0 && f.cached != nil {
		return copy(buf[off:], f.cached)
	}

	pos := off

	// Calculate component data length
	dataLen := f.componentLen()

	// Write NLRI length prefix per RFC 8955 Section 4.1
	if dataLen < 240 {
		buf[pos] = byte(dataLen)
		pos++
	} else {
		// Extended length: 0xfnnn format
		buf[pos] = 0xF0 | byte(dataLen>>8)
		buf[pos+1] = byte(dataLen)
		pos += 2
	}

	// Write components in sorted order
	pos += f.writeComponentsSorted(buf, pos)

	return pos - off
}
