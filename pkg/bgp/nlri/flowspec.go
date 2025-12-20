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

// FlowComponentType identifies a FlowSpec component (RFC 5575).
type FlowComponentType uint8

// FlowSpec component types (RFC 5575 Section 4).
const (
	FlowDestPrefix   FlowComponentType = 1  // Destination Prefix
	FlowSourcePrefix FlowComponentType = 2  // Source Prefix
	FlowIPProtocol   FlowComponentType = 3  // IP Protocol
	FlowPort         FlowComponentType = 4  // Port (src or dst)
	FlowDestPort     FlowComponentType = 5  // Destination Port
	FlowSourcePort   FlowComponentType = 6  // Source Port
	FlowICMPType     FlowComponentType = 7  // ICMP Type
	FlowICMPCode     FlowComponentType = 8  // ICMP Code
	FlowTCPFlags     FlowComponentType = 9  // TCP Flags
	FlowPacketLength FlowComponentType = 10 // Packet Length
	FlowDSCP         FlowComponentType = 11 // DSCP
	FlowFragment     FlowComponentType = 12 // Fragment
	FlowFlowLabel    FlowComponentType = 13 // Flow Label (IPv6)
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

// FlowOperator represents numeric operators in FlowSpec (RFC 5575 Section 4).
type FlowOperator byte

// Operator flags.
const (
	FlowOpEnd     FlowOperator = 0x80 // End of list
	FlowOpAnd     FlowOperator = 0x40 // AND (vs OR)
	FlowOpLenMask FlowOperator = 0x30 // Length mask (0=1byte, 1=2bytes, 2=4bytes)
	FlowOpLess    FlowOperator = 0x04 // Less than
	FlowOpGreater FlowOperator = 0x02 // Greater than
	FlowOpEqual   FlowOperator = 0x01 // Equal
	FlowOpNotEq   FlowOperator = 0x06 // Not equal (LT | GT)
)

// FlowMatch represents a single match condition with operator and value.
type FlowMatch struct {
	Op    FlowOperator // Comparison operator (GT, LT, EQ, etc.)
	And   bool         // AND with previous match (vs OR)
	Value uint64       // The value to match
}

// Fragment flags for FlowFragment component.
const (
	FlowFragDontFragment  FlowFragmentFlag = 0x01
	FlowFragIsFragment    FlowFragmentFlag = 0x02
	FlowFragFirstFragment FlowFragmentFlag = 0x04
	FlowFragLastFragment  FlowFragmentFlag = 0x08
)

// FlowFragmentFlag represents fragment matching flags.
type FlowFragmentFlag byte

// FlowComponent is the interface for FlowSpec components.
type FlowComponent interface {
	Type() FlowComponentType
	Bytes() []byte
	String() string
}

// FlowSpec represents a FlowSpec NLRI (RFC 5575).
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
// Components are sorted by type per RFC 5575 Section 4.
func (f *FlowSpec) ComponentBytes() []byte {
	// Sort components by type (RFC 5575 requires ordering)
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
func (f *FlowSpec) Bytes() []byte {
	if f.cached != nil {
		return f.cached
	}

	// Encode components
	data := f.ComponentBytes()

	// Add NLRI length prefix
	if len(data) < 240 {
		f.cached = append([]byte{byte(len(data))}, data...)
	} else {
		// Extended length (2 bytes)
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

// ParseFlowSpec parses a FlowSpec from wire format.
func ParseFlowSpec(family Family, data []byte) (*FlowSpec, error) {
	if len(data) == 0 {
		return nil, ErrFlowSpecTruncated
	}

	// Parse length
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

	fs := NewFlowSpec(family)
	remaining := data[offset : offset+nlriLen]

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
func parseFlowComponent(data []byte, family Family) (FlowComponent, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrFlowSpecTruncated
	}

	compType := FlowComponentType(data[0])

	switch compType {
	case FlowDestPrefix, FlowSourcePrefix:
		return parsePrefixComponent(compType, data[1:], family)
	case FlowIPProtocol, FlowPort, FlowDestPort, FlowSourcePort,
		FlowICMPType, FlowICMPCode, FlowTCPFlags, FlowPacketLength,
		FlowDSCP, FlowFragment, FlowFlowLabel:
		return parseNumericComponent(compType, data[1:])
	default:
		return nil, nil, ErrFlowSpecInvalidType
	}
}

// parsePrefixComponent parses a prefix-type component.
func parsePrefixComponent(t FlowComponentType, data []byte, family Family) (FlowComponent, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrFlowSpecTruncated
	}

	prefixLen := int(data[0])
	prefixBytes := (prefixLen + 7) / 8

	if len(data) < 1+prefixBytes {
		return nil, nil, ErrFlowSpecTruncated
	}

	// Build prefix
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

// parseNumericComponent parses a numeric-type component.
func parseNumericComponent(t FlowComponentType, data []byte) (FlowComponent, []byte, error) {
	if len(data) == 0 {
		return nil, nil, ErrFlowSpecTruncated
	}

	var matches []FlowMatch
	offset := 0

	for offset < len(data) {
		op := FlowOperator(data[offset])
		offset++

		// Determine value length from operator
		lenCode := (op & FlowOpLenMask) >> 4
		valueLen := 1 << lenCode
		if valueLen > 4 {
			valueLen = 4
		}

		if offset+valueLen > len(data) {
			return nil, nil, ErrFlowSpecTruncated
		}

		// Read value
		var value uint64
		for i := 0; i < valueLen; i++ {
			value = value<<8 | uint64(data[offset+i])
		}

		// Extract comparison operator (mask out EOL, AND, LEN bits)
		compOp := op &^ (FlowOpEnd | FlowOpAnd | FlowOpLenMask)

		matches = append(matches, FlowMatch{
			Op:    compOp,
			And:   op&FlowOpAnd != 0,
			Value: value,
		})
		offset += valueLen

		// Check for end of list
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

// Prefix components

type prefixComponent struct {
	compType FlowComponentType
	prefix   netip.Prefix
	offset   uint8 // IPv6 offset (0 for IPv4)
}

// NewFlowDestPrefixComponent creates a destination prefix component.
func NewFlowDestPrefixComponent(prefix netip.Prefix) FlowComponent {
	return &prefixComponent{compType: FlowDestPrefix, prefix: prefix}
}

// NewFlowSourcePrefixComponent creates a source prefix component.
func NewFlowSourcePrefixComponent(prefix netip.Prefix) FlowComponent {
	return &prefixComponent{compType: FlowSourcePrefix, prefix: prefix}
}

// NewFlowDestPrefixComponentWithOffset creates an IPv6 destination prefix with offset.
func NewFlowDestPrefixComponentWithOffset(prefix netip.Prefix, offset uint8) FlowComponent {
	return &prefixComponent{compType: FlowDestPrefix, prefix: prefix, offset: offset}
}

// NewFlowSourcePrefixComponentWithOffset creates an IPv6 source prefix with offset.
func NewFlowSourcePrefixComponentWithOffset(prefix netip.Prefix, offset uint8) FlowComponent {
	return &prefixComponent{compType: FlowSourcePrefix, prefix: prefix, offset: offset}
}

func (c *prefixComponent) Type() FlowComponentType { return c.compType }
func (c *prefixComponent) Prefix() netip.Prefix    { return c.prefix }

func (c *prefixComponent) Bytes() []byte {
	bits := c.prefix.Bits()
	addr := c.prefix.Addr()

	// IPv6 FlowSpec prefixes include an offset byte
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

	// IPv4: no offset
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

// Numeric components

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

func (c *numericComponent) Bytes() []byte {
	data := []byte{byte(c.compType)}

	for i, m := range c.matches {
		// Determine value length
		var lenCode, valueLen byte
		switch {
		case m.Value <= 0xFF:
			lenCode, valueLen = 0, 1
		case m.Value <= 0xFFFF:
			lenCode, valueLen = 1, 2
		default:
			lenCode, valueLen = 2, 4
		}

		// Build operator byte
		op := lenCode << 4
		if m.And {
			op |= byte(FlowOpAnd)
		}
		if i == len(c.matches)-1 {
			op |= byte(FlowOpEnd)
		}
		op |= byte(m.Op) // Add comparison operator (GT, LT, EQ, etc.)

		data = append(data, op)

		// Encode value
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

// Component constructors

// NewFlowNumericComponent creates a numeric component with explicit matches.
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

// NewFlowIPProtocolComponent creates an IP protocol component.
func NewFlowIPProtocolComponent(protocols ...uint8) FlowComponent {
	matches := make([]FlowMatch, len(protocols))
	for i, p := range protocols {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(p)}
	}
	return &numericComponent{compType: FlowIPProtocol, matches: matches}
}

// NewFlowPortComponent creates a port component (src or dst).
func NewFlowPortComponent(ports ...uint16) FlowComponent {
	matches := make([]FlowMatch, len(ports))
	for i, p := range ports {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(p)}
	}
	return &numericComponent{compType: FlowPort, matches: matches}
}

// NewFlowDestPortComponent creates a destination port component.
func NewFlowDestPortComponent(ports ...uint16) FlowComponent {
	matches := make([]FlowMatch, len(ports))
	for i, p := range ports {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(p)}
	}
	return &numericComponent{compType: FlowDestPort, matches: matches}
}

// NewFlowSourcePortComponent creates a source port component.
func NewFlowSourcePortComponent(ports ...uint16) FlowComponent {
	matches := make([]FlowMatch, len(ports))
	for i, p := range ports {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(p)}
	}
	return &numericComponent{compType: FlowSourcePort, matches: matches}
}

// NewFlowICMPTypeComponent creates an ICMP type component.
func NewFlowICMPTypeComponent(types ...uint8) FlowComponent {
	matches := make([]FlowMatch, len(types))
	for i, t := range types {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(t)}
	}
	return &numericComponent{compType: FlowICMPType, matches: matches}
}

// NewFlowICMPCodeComponent creates an ICMP code component.
func NewFlowICMPCodeComponent(codes ...uint8) FlowComponent {
	matches := make([]FlowMatch, len(codes))
	for i, c := range codes {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(c)}
	}
	return &numericComponent{compType: FlowICMPCode, matches: matches}
}

// NewFlowTCPFlagsComponent creates a TCP flags component.
// TCP flags use bitmask matching (no comparison operator).
func NewFlowTCPFlagsComponent(flags ...uint8) FlowComponent {
	matches := make([]FlowMatch, len(flags))
	for i, f := range flags {
		matches[i] = FlowMatch{Op: 0, Value: uint64(f)} // No operator for bitmask
	}
	return &numericComponent{compType: FlowTCPFlags, matches: matches}
}

// NewFlowTCPFlagsMatchComponent creates a TCP flags component with explicit matches.
func NewFlowTCPFlagsMatchComponent(matchList []FlowMatch) FlowComponent {
	return &numericComponent{compType: FlowTCPFlags, matches: matchList}
}

// NewFlowPacketLengthComponent creates a packet length component.
func NewFlowPacketLengthComponent(lengths ...uint16) FlowComponent {
	matches := make([]FlowMatch, len(lengths))
	for i, l := range lengths {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(l)}
	}
	return &numericComponent{compType: FlowPacketLength, matches: matches}
}

// NewFlowPacketLengthMatchComponent creates a packet length component with explicit matches.
func NewFlowPacketLengthMatchComponent(matchList []FlowMatch) FlowComponent {
	return &numericComponent{compType: FlowPacketLength, matches: matchList}
}

// NewFlowDSCPComponent creates a DSCP component.
func NewFlowDSCPComponent(values ...uint8) FlowComponent {
	matches := make([]FlowMatch, len(values))
	for i, v := range values {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(v)}
	}
	return &numericComponent{compType: FlowDSCP, matches: matches}
}

// NewFlowFragmentComponent creates a fragment component.
// Fragment uses bitmask matching (no comparison operator).
func NewFlowFragmentComponent(flags ...FlowFragmentFlag) FlowComponent {
	matches := make([]FlowMatch, len(flags))
	for i, f := range flags {
		matches[i] = FlowMatch{Op: 0, Value: uint64(f)} // No operator for bitmask
	}
	return &numericComponent{compType: FlowFragment, matches: matches}
}

// NewFlowFlowLabelComponent creates a flow-label component (IPv6 only).
// Flow-label is a 20-bit field encoded as uint32.
func NewFlowFlowLabelComponent(labels ...uint32) FlowComponent {
	matches := make([]FlowMatch, len(labels))
	for i, v := range labels {
		matches[i] = FlowMatch{Op: FlowOpEqual, Value: uint64(v)}
	}
	return &numericComponent{compType: FlowFlowLabel, matches: matches}
}

// ============================================================================
// FlowSpec VPN (RFC 5575 Section 6, SAFI 134)
// ============================================================================

// FlowSpecVPN wraps FlowSpec with a Route Distinguisher for VPN use.
type FlowSpecVPN struct {
	rd       RouteDistinguisher
	flowSpec *FlowSpec
	cached   []byte
}

// NewFlowSpecVPN creates a new FlowSpec VPN NLRI.
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

// Bytes returns the wire-format encoding.
// Format: Length (1-2 bytes) + RD (8 bytes) + FlowSpec components.
func (f *FlowSpecVPN) Bytes() []byte {
	if f.cached != nil {
		return f.cached
	}

	// Get component bytes (without FlowSpec length prefix)
	compBytes := f.flowSpec.ComponentBytes()

	// Total payload = RD (8) + components
	payloadLen := 8 + len(compBytes)

	// Build with length prefix
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

// ParseFlowSpecVPN parses a FlowSpec VPN from wire format.
func ParseFlowSpecVPN(family Family, data []byte) (*FlowSpecVPN, error) {
	if len(data) == 0 {
		return nil, ErrFlowSpecTruncated
	}

	// Parse length
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

	// Need at least 8 bytes for RD
	if nlriLen < 8 {
		return nil, ErrFlowSpecTruncated
	}

	// Parse RD
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
