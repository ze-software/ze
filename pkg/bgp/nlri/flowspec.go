// Package nlri implements BGP Network Layer Reachability Information types.
package nlri

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

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

// String returns a human-readable component type name.
func (t FlowComponentType) String() string {
	switch t {
	case FlowDestPrefix:
		return "dest-prefix"
	case FlowSourcePrefix:
		return "source-prefix"
	case FlowIPProtocol:
		return "protocol"
	case FlowPort:
		return "port"
	case FlowDestPort:
		return "dest-port"
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
// lt (bit 5): less-than comparison
// gt (bit 6): greater-than comparison
// eq (bit 7): equality comparison
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
// DF (bit 0): Don't Fragment - match if IP Header Flags Bit-1 (DF) is 1
// IsF (bit 1): Is a Fragment - match if Fragment Offset is not 0
// FF (bit 2): First Fragment - match if Fragment Offset is 0 AND MF is 1
// LF (bit 3): Last Fragment - match if Fragment Offset is not 0 AND MF is 0
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
// "Components MUST follow strict type ordering by increasing numerical order."
func (f *FlowSpec) ComponentBytes() []byte {
	// Sort components by type (RFC 8955 Section 4.2 requires strict ordering)
	sorted := make([]FlowComponent, len(f.components))
	copy(sorted, f.components)
	slices.SortFunc(sorted, func(a, b FlowComponent) int {
		return int(a.Type()) - int(b.Type())
	})

	var data []byte
	for _, c := range sorted {
		data = append(data, c.Bytes()...)
	}
	return data
}

// Bytes returns the wire-format encoding (with length prefix).
// RFC 8955 Section 4.1 defines the length encoding:
// - If length < 240 (0xf0): single octet length
// - If length >= 240: 2-octet extended length with high nibble 0xf
//
// Example from RFC 8955: 239 -> 0xef (1 octet), 240 -> 0xf0f0 (2 octets)
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

// String returns a human-readable representation.
func (f *FlowSpec) String() string {
	parts := make([]string, len(f.components))
	for i, c := range f.components {
		parts[i] = c.String()
	}
	return fmt.Sprintf("flowspec(%s)", strings.Join(parts, " "))
}

// ParseFlowSpec parses a FlowSpec from wire format per RFC 8955 Section 4.
// The NLRI consists of a length field followed by component data.
// RFC 8955 Section 4.1 defines length encoding:
// - Single octet if < 240
// - Two octets (0xfnnn format) if >= 240
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
// with a type field (1 octet) followed by a variable length parameter."
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

// parsePrefixComponent parses a prefix-type component (Type 1 or 2).
// RFC 8955 Section 4.2.2.1-2 defines the encoding:
//
//	<type (1 octet), length (1 octet), prefix (variable)>
//
// The length and prefix fields are encoded as in BGP UPDATE messages (RFC 4271).
func parsePrefixComponent(t FlowComponentType, data []byte, family Family) (FlowComponent, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrFlowSpecTruncated
	}

	prefixLen := int(data[0])
	prefixBytes := (prefixLen + 7) / 8

	if len(data) < 1+prefixBytes {
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
		copy(ip[:], data[1:1+prefixBytes])
		addr = netip.AddrFrom16(ip)
	}

	prefix := netip.PrefixFrom(addr, prefixLen)

	var comp FlowComponent
	if t == FlowDestPrefix {
		comp = NewFlowDestPrefixComponent(prefix)
	} else {
		comp = NewFlowSourcePrefixComponent(prefix)
	}

	return comp, data[1+prefixBytes:], nil
}

// parseNumericComponent parses a numeric-type component (Types 3-12).
// RFC 8955 Section 4.2.1.1 defines the numeric operator format.
// The component consists of a list of {operator, value} pairs.
// Encoding: <type (1 octet), [numeric_op, value]+>
func parseNumericComponent(t FlowComponentType, data []byte) (FlowComponent, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrFlowSpecTruncated
	}

	var matches []FlowMatch
	offset := 0

	for offset < len(data) {
		op := FlowOperator(data[offset])
		offset++

		// Determine value length from operator's len field (bits 2-3)
		// RFC 8955 Section 4.2.1.1: length = 1 << len (1, 2, 4, or 8 octets)
		lenCode := (op & FlowOpLenMask) >> 4
		valueLen := 1 << lenCode
		if valueLen > 4 {
			// Note: RFC allows 8 octets (len=11), but we cap at 4 for uint32 values
			valueLen = 4
		}

		if offset+valueLen > len(data) {
			return nil, nil, ErrFlowSpecTruncated
		}

		// Read value in network byte order (big-endian)
		var value uint64
		for i := 0; i < valueLen; i++ {
			value = value<<8 | uint64(data[offset+i])
		}

		// Extract comparison operator bits (mask out EOL, AND, LEN bits)
		// The remaining bits are lt, gt, eq per RFC 8955 Table 1
		compOp := op &^ (FlowOpEnd | FlowOpAnd | FlowOpLenMask)

		matches = append(matches, FlowMatch{
			Op:    compOp,
			And:   op&FlowOpAnd != 0,
			Value: value,
		})
		offset += valueLen

		// Check for end of list (RFC 8955: 'e' bit set in last pair)
		if op&FlowOpEnd != 0 {
			break
		}
	}

	comp := &numericComponent{
		compType: t,
		matches:  matches,
	}

	return comp, data[offset:], nil
}

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

// Bytes returns the wire encoding per RFC 8955 Section 4.2.2.1-2.
// IPv4: <type (1), length (1), prefix (variable)>
// IPv6: <type (1), length (1), offset (1), prefix (variable)> per RFC 8956
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

func (c *prefixComponent) String() string {
	return fmt.Sprintf("%s=%s", c.compType, c.prefix)
}

// Numeric components (Types 3-12)
// RFC 8955 Section 4.2.2.3-12 defines numeric component types.
// These use the numeric_op operator format from Section 4.2.1.1.

type numericComponent struct {
	compType FlowComponentType
	matches  []FlowMatch
}

func (c *numericComponent) Type() FlowComponentType { return c.compType }

// Matches returns the match conditions.
func (c *numericComponent) Matches() []FlowMatch { return c.matches }

// Values returns just the values (for backwards compatibility).
func (c *numericComponent) Values() []uint64 {
	vals := make([]uint64, len(c.matches))
	for i, m := range c.matches {
		vals[i] = m.Value
	}
	return vals
}

// Bytes returns the wire encoding per RFC 8955 Section 4.2.1.1.
// Format: <type (1 octet), [numeric_op, value]+>
func (c *numericComponent) Bytes() []byte {
	data := []byte{byte(c.compType)}

	for i, m := range c.matches {
		// Determine value length - RFC 8955 Section 4.2.1.1:
		// len field encodes (1 << len) bytes: 0=1, 1=2, 2=4, 3=8 octets
		var lenCode, valueLen byte
		switch {
		case m.Value <= 0xFF:
			lenCode, valueLen = 0, 1
		case m.Value <= 0xFFFF:
			lenCode, valueLen = 1, 2
		default:
			lenCode, valueLen = 2, 4
		}

		// Build operator byte per RFC 8955 Section 4.2.1.1:
		// [e][a][len:2][0][lt][gt][eq]
		op := lenCode << 4
		if m.And {
			op |= byte(FlowOpAnd) // Set 'a' bit
		}
		if i == len(c.matches)-1 {
			op |= byte(FlowOpEnd) // Set 'e' bit on last pair
		}
		op |= byte(m.Op) // Add comparison operator bits (lt, gt, eq)

		data = append(data, op)

		// Encode value in network byte order (big-endian)
		switch valueLen {
		case 1:
			data = append(data, byte(m.Value))
		case 2:
			data = append(data, byte(m.Value>>8), byte(m.Value))
		case 4:
			var buf [4]byte
			binary.BigEndian.PutUint32(buf[:], uint32(m.Value)) //nolint:gosec // Flowspec value size validated
			data = append(data, buf[:]...)
		}
	}

	return data
}

func (c *numericComponent) String() string {
	parts := make([]string, len(c.matches))
	for i, m := range c.matches {
		var prefix string
		if m.And && i > 0 {
			prefix = "&"
		}
		switch m.Op {
		case FlowOpGreater:
			parts[i] = fmt.Sprintf("%s>%d", prefix, m.Value)
		case FlowOpLess:
			parts[i] = fmt.Sprintf("%s<%d", prefix, m.Value)
		case FlowOpEqual:
			parts[i] = fmt.Sprintf("%s=%d", prefix, m.Value)
		case FlowOpNotEq:
			parts[i] = fmt.Sprintf("%s!=%d", prefix, m.Value)
		case FlowOpGreater | FlowOpEqual:
			parts[i] = fmt.Sprintf("%s>=%d", prefix, m.Value)
		case FlowOpLess | FlowOpEqual:
			parts[i] = fmt.Sprintf("%s<=%d", prefix, m.Value)
		default:
			parts[i] = fmt.Sprintf("%s%d", prefix, m.Value)
		}
	}
	return fmt.Sprintf("%s[%s]", c.compType, strings.Join(parts, " "))
}

// ============================================================================
// Component constructors
// ============================================================================

// NewFlowNumericComponent creates a numeric component with explicit matches.
// This is the general constructor for any numeric component type (3-12).
func NewFlowNumericComponent(compType FlowComponentType, matches []FlowMatch) FlowComponent {
	return &numericComponent{compType: compType, matches: matches}
}

// Helper to convert simple values to FlowMatch with equality operator.
func valuesToMatches(values []uint64, op FlowOperator) []FlowMatch {
	matches := make([]FlowMatch, len(values))
	for i, v := range values {
		matches[i] = FlowMatch{Op: op, Value: v}
	}
	return matches
}

// NewFlowIPProtocolComponent creates an IP protocol component (Type 3).
// RFC 8955 Section 4.2.2.3: Matches the IP protocol value octet.
// Values SHOULD be encoded as single octet (len=00).
func NewFlowIPProtocolComponent(protocols ...uint8) FlowComponent {
	matches := make([]FlowMatch, len(protocols))
	for i, p := range protocols {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(p)}
	}
	return &numericComponent{compType: FlowIPProtocol, matches: matches}
}

// NewFlowPortComponent creates a port component (Type 4).
// RFC 8955 Section 4.2.2.4: Matches source OR destination TCP/UDP ports.
// Values SHOULD be encoded as 1- or 2-octet quantities.
func NewFlowPortComponent(ports ...uint16) FlowComponent {
	matches := make([]FlowMatch, len(ports))
	for i, p := range ports {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(p)}
	}
	return &numericComponent{compType: FlowPort, matches: matches}
}

// NewFlowDestPortComponent creates a destination port component (Type 5).
// RFC 8955 Section 4.2.2.5: Matches the destination port of TCP/UDP.
// Values SHOULD be encoded as 1- or 2-octet quantities.
func NewFlowDestPortComponent(ports ...uint16) FlowComponent {
	matches := make([]FlowMatch, len(ports))
	for i, p := range ports {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(p)}
	}
	return &numericComponent{compType: FlowDestPort, matches: matches}
}

// NewFlowSourcePortComponent creates a source port component (Type 6).
// RFC 8955 Section 4.2.2.6: Matches the source port of TCP/UDP.
// Values SHOULD be encoded as 1- or 2-octet quantities.
func NewFlowSourcePortComponent(ports ...uint16) FlowComponent {
	matches := make([]FlowMatch, len(ports))
	for i, p := range ports {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(p)}
	}
	return &numericComponent{compType: FlowSourcePort, matches: matches}
}

// NewFlowICMPTypeComponent creates an ICMP type component (Type 7).
// RFC 8955 Section 4.2.2.7: Matches the type field of an ICMP packet.
// Values SHOULD be encoded as single octet (len=00).
// Only ICMP packets (IP protocol=1) can match when this component is present.
func NewFlowICMPTypeComponent(types ...uint8) FlowComponent {
	matches := make([]FlowMatch, len(types))
	for i, t := range types {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(t)}
	}
	return &numericComponent{compType: FlowICMPType, matches: matches}
}

// NewFlowICMPCodeComponent creates an ICMP code component (Type 8).
// RFC 8955 Section 4.2.2.8: Matches the code field of an ICMP packet.
// Values SHOULD be encoded as single octet (len=00).
// Only ICMP packets (IP protocol=1) can match when this component is present.
func NewFlowICMPCodeComponent(codes ...uint8) FlowComponent {
	matches := make([]FlowMatch, len(codes))
	for i, c := range codes {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(c)}
	}
	return &numericComponent{compType: FlowICMPCode, matches: matches}
}

// NewFlowTCPFlagsComponent creates a TCP flags component (Type 9).
// RFC 8955 Section 4.2.2.9: Uses bitmask_op operator (not numeric_op).
// Values MUST be encoded as 1- or 2-octet bitmask (len=00 or len=01).
// Only TCP packets (IP protocol=6) can match when this component is present.
func NewFlowTCPFlagsComponent(flags ...uint8) FlowComponent {
	matches := make([]FlowMatch, len(flags))
	for i, f := range flags {
		matches[i] = FlowMatch{Op: 0, Value: uint64(f)} // bitmask_op, not numeric_op
	}
	return &numericComponent{compType: FlowTCPFlags, matches: matches}
}

// NewFlowTCPFlagsMatchComponent creates a TCP flags component with explicit matches.
// Allows specifying bitmask operator bits (NOT, Match) per RFC 8955 Section 4.2.1.2.
func NewFlowTCPFlagsMatchComponent(matchList []FlowMatch) FlowComponent {
	return &numericComponent{compType: FlowTCPFlags, matches: matchList}
}

// NewFlowPacketLengthComponent creates a packet length component (Type 10).
// RFC 8955 Section 4.2.2.10: Matches total IP packet length (excluding L2).
// Values SHOULD be encoded as 1- or 2-octet quantities.
func NewFlowPacketLengthComponent(lengths ...uint16) FlowComponent {
	matches := make([]FlowMatch, len(lengths))
	for i, l := range lengths {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(l)}
	}
	return &numericComponent{compType: FlowPacketLength, matches: matches}
}

// NewFlowPacketLengthMatchComponent creates a packet length component with explicit matches.
// Allows specifying range matches (e.g., >=100 AND <=200).
func NewFlowPacketLengthMatchComponent(matchList []FlowMatch) FlowComponent {
	return &numericComponent{compType: FlowPacketLength, matches: matchList}
}

// NewFlowDSCPComponent creates a DSCP component (Type 11).
// RFC 8955 Section 4.2.2.11: Matches the 6-bit DSCP field.
// Values MUST be encoded as single octet (len=00).
// The six least significant bits contain the DSCP value.
func NewFlowDSCPComponent(values ...uint8) FlowComponent {
	matches := make([]FlowMatch, len(values))
	for i, v := range values {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(v)}
	}
	return &numericComponent{compType: FlowDSCP, matches: matches}
}

// NewFlowFragmentComponent creates a fragment component (Type 12).
// RFC 8955 Section 4.2.2.12: Uses bitmask_op operator.
// Bitmask MUST be encoded as single octet (len=00).
// See FlowFragmentFlag constants for valid bitmask values.
func NewFlowFragmentComponent(flags ...FlowFragmentFlag) FlowComponent {
	matches := make([]FlowMatch, len(flags))
	for i, f := range flags {
		matches[i] = FlowMatch{Op: 0, Value: uint64(f)} // bitmask_op, not numeric_op
	}
	return &numericComponent{compType: FlowFragment, matches: matches}
}

// NewFlowFlowLabelComponent creates a flow-label component (Type 13, IPv6 only).
// RFC 8956 defines this component for IPv6 FlowSpec.
// Flow-label is a 20-bit field encoded as uint32.
func NewFlowFlowLabelComponent(labels ...uint32) FlowComponent {
	matches := make([]FlowMatch, len(labels))
	for i, v := range labels {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(v)}
	}
	return &numericComponent{compType: FlowFlowLabel, matches: matches}
}

// ============================================================================
// FlowSpec VPN (RFC 8955 Section 8, SAFI 134)
// ============================================================================

// FlowSpecVPN wraps FlowSpec with a Route Distinguisher for VPN use.
// RFC 8955 Section 8 defines the VPNv4 Flow Specification (AFI=1, SAFI=134).
// The NLRI format per RFC 8955 Figure 7:
//
//	+--------------------------------+
//	| length (0xnn or 0xfnnn)        |
//	+--------------------------------+
//	| Route Distinguisher (8 octets) |
//	+--------------------------------+
//	|    NLRI value  (variable)      |
//	+--------------------------------+
type FlowSpecVPN struct {
	rd       RouteDistinguisher
	flowSpec *FlowSpec
	cached   []byte
}

// NewFlowSpecVPN creates a new FlowSpec VPN NLRI (SAFI 134).
// RFC 8955 Section 8: "This document defines an additional BGP NLRI type
// (AFI=1, SAFI=134) value, which can be used to propagate Flow Specification
// in a BGP/MPLS VPN environment."
func NewFlowSpecVPN(family Family, rd RouteDistinguisher) *FlowSpecVPN {
	// Convert SAFI to FlowSpecVPN if needed
	fsFamily := family
	if family.SAFI == SAFIFlowSpecVPN {
		fsFamily = Family{AFI: family.AFI, SAFI: SAFIFlowSpec}
	}
	return &FlowSpecVPN{
		rd:       rd,
		flowSpec: NewFlowSpec(fsFamily),
	}
}

// Family returns the address family (with SAFI 134).
func (f *FlowSpecVPN) Family() Family {
	return Family{AFI: f.flowSpec.family.AFI, SAFI: SAFIFlowSpecVPN}
}

// RD returns the Route Distinguisher.
func (f *FlowSpecVPN) RD() RouteDistinguisher {
	return f.rd
}

// FlowSpec returns the underlying FlowSpec.
func (f *FlowSpecVPN) FlowSpec() *FlowSpec {
	return f.flowSpec
}

// AddComponent adds a component to the FlowSpec.
func (f *FlowSpecVPN) AddComponent(c FlowComponent) {
	f.flowSpec.AddComponent(c)
	f.cached = nil
}

// Components returns the FlowSpec components.
func (f *FlowSpecVPN) Components() []FlowComponent {
	return f.flowSpec.Components()
}

// Bytes returns the wire-format encoding per RFC 8955 Section 8.
// Format: Length (1-2 bytes) + RD (8 bytes) + FlowSpec components.
// RFC 8955 Section 8: "The NLRI length field shall include both the
// 8 octets of the Route Distinguisher as well as the subsequent
// Flow Specification NLRI value."
func (f *FlowSpecVPN) Bytes() []byte {
	if f.cached != nil {
		return f.cached
	}

	// Get component bytes (without FlowSpec length prefix)
	compBytes := f.flowSpec.ComponentBytes()

	// Total payload = RD (8) + components per RFC 8955 Section 8
	payloadLen := 8 + len(compBytes)

	// Build with length prefix per RFC 8955 Section 4.1
	if payloadLen < 240 {
		f.cached = make([]byte, 1+payloadLen)
		f.cached[0] = byte(payloadLen)
		copy(f.cached[1:9], f.rd.Bytes())
		copy(f.cached[9:], compBytes)
	} else {
		f.cached = make([]byte, 2+payloadLen)
		f.cached[0] = 0xF0 | byte(payloadLen>>8)
		f.cached[1] = byte(payloadLen)
		copy(f.cached[2:10], f.rd.Bytes())
		copy(f.cached[10:], compBytes)
	}

	return f.cached
}

// Len returns the length in bytes.
func (f *FlowSpecVPN) Len() int {
	return len(f.Bytes())
}

// PathID returns 0 (FlowSpecVPN doesn't use ADD-PATH).
func (f *FlowSpecVPN) PathID() uint32 {
	return 0
}

// HasPathID returns false.
func (f *FlowSpecVPN) HasPathID() bool {
	return false
}

// String returns a human-readable representation.
func (f *FlowSpecVPN) String() string {
	return fmt.Sprintf("flowspec-vpn(rd:%s %s)", f.rd, f.flowSpec)
}

// ParseFlowSpecVPN parses a FlowSpec VPN from wire format per RFC 8955 Section 8.
// The NLRI consists of length + Route Distinguisher (8 octets) + FlowSpec components.
func ParseFlowSpecVPN(family Family, data []byte) (*FlowSpecVPN, error) {
	if len(data) == 0 {
		return nil, ErrFlowSpecTruncated
	}

	// Parse length per RFC 8955 Section 4.1
	nlriLen := int(data[0])
	offset := 1
	if nlriLen >= 240 {
		if len(data) < 2 {
			return nil, ErrFlowSpecTruncated
		}
		nlriLen = int(data[0]&0x0F)<<8 | int(data[1])
		offset = 2
	}

	if len(data) < offset+nlriLen {
		return nil, ErrFlowSpecTruncated
	}

	// Need at least 8 bytes for RD per RFC 8955 Section 8
	if nlriLen < 8 {
		return nil, ErrFlowSpecTruncated
	}

	// Parse Route Distinguisher (RFC 4364)
	rd, err := ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		return nil, err
	}

	// Parse FlowSpec components (remaining data after RD)
	fsFamily := Family{AFI: family.AFI, SAFI: SAFIFlowSpec}
	fs := NewFlowSpec(fsFamily)

	remaining := data[offset+8 : offset+nlriLen]
	for len(remaining) > 0 {
		comp, rest, err := parseFlowComponent(remaining, fsFamily)
		if err != nil {
			return nil, err
		}
		fs.components = append(fs.components, comp)
		remaining = rest
	}

	return &FlowSpecVPN{
		rd:       rd,
		flowSpec: fs,
	}, nil
}
