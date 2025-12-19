package nlri

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
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
	FlowOpNegate  FlowOperator = 0x02 // Negate
	FlowOpLess    FlowOperator = 0x04 // Less than
	FlowOpGreater FlowOperator = 0x02 // Greater than
	FlowOpEqual   FlowOperator = 0x01 // Equal
)

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

// Bytes returns the wire-format encoding.
func (f *FlowSpec) Bytes() []byte {
	if f.cached != nil {
		return f.cached
	}

	// Encode components
	var data []byte
	for _, c := range f.components {
		data = append(data, c.Bytes()...)
	}

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
		FlowDSCP, FlowFragment:
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

	var values []uint64
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
		values = append(values, value)
		offset += valueLen

		// Check for end of list
		if op&FlowOpEnd != 0 {
			break
		}
	}

	comp := &numericComponent{
		compType: t,
		values:   values,
	}

	return comp, data[offset:], nil
}

// Prefix components

type prefixComponent struct {
	compType FlowComponentType
	prefix   netip.Prefix
}

// NewFlowDestPrefixComponent creates a destination prefix component.
func NewFlowDestPrefixComponent(prefix netip.Prefix) FlowComponent {
	return &prefixComponent{compType: FlowDestPrefix, prefix: prefix}
}

// NewFlowSourcePrefixComponent creates a source prefix component.
func NewFlowSourcePrefixComponent(prefix netip.Prefix) FlowComponent {
	return &prefixComponent{compType: FlowSourcePrefix, prefix: prefix}
}

func (c *prefixComponent) Type() FlowComponentType { return c.compType }
func (c *prefixComponent) Prefix() netip.Prefix    { return c.prefix }

func (c *prefixComponent) Bytes() []byte {
	bits := c.prefix.Bits()
	prefixBytes := (bits + 7) / 8

	data := make([]byte, 2+prefixBytes)
	data[0] = byte(c.compType)
	data[1] = byte(bits)

	addr := c.prefix.Addr()
	if addr.Is4() {
		ip4 := addr.As4()
		copy(data[2:], ip4[:prefixBytes])
	} else {
		ip6 := addr.As16()
		copy(data[2:], ip6[:prefixBytes])
	}

	return data
}

func (c *prefixComponent) String() string {
	return fmt.Sprintf("%s=%s", c.compType, c.prefix)
}

// Numeric components

type numericComponent struct {
	compType FlowComponentType
	values   []uint64
}

func (c *numericComponent) Type() FlowComponentType { return c.compType }
func (c *numericComponent) Values() []uint64        { return c.values }

func (c *numericComponent) Bytes() []byte {
	data := []byte{byte(c.compType)}

	for i, v := range c.values {
		// Determine value length
		var lenCode, valueLen byte
		switch {
		case v <= 0xFF:
			lenCode, valueLen = 0, 1
		case v <= 0xFFFF:
			lenCode, valueLen = 1, 2
		default:
			lenCode, valueLen = 2, 4
		}

		op := lenCode << 4
		if i == len(c.values)-1 {
			op |= byte(FlowOpEnd)
		}
		op |= byte(FlowOpEqual) // Default to equality

		data = append(data, op)

		// Encode value
		switch valueLen {
		case 1:
			data = append(data, byte(v))
		case 2:
			data = append(data, byte(v>>8), byte(v))
		case 4:
			var buf [4]byte
			binary.BigEndian.PutUint32(buf[:], uint32(v)) //nolint:gosec // Flowspec value size validated
			data = append(data, buf[:]...)
		}
	}

	return data
}

func (c *numericComponent) String() string {
	parts := make([]string, len(c.values))
	for i, v := range c.values {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("%s=%s", c.compType, strings.Join(parts, ","))
}

// Component constructors

// NewFlowIPProtocolComponent creates an IP protocol component.
func NewFlowIPProtocolComponent(protocols ...uint8) FlowComponent {
	values := make([]uint64, len(protocols))
	for i, p := range protocols {
		values[i] = uint64(p)
	}
	return &numericComponent{compType: FlowIPProtocol, values: values}
}

// NewFlowPortComponent creates a port component (src or dst).
func NewFlowPortComponent(ports ...uint16) FlowComponent {
	values := make([]uint64, len(ports))
	for i, p := range ports {
		values[i] = uint64(p)
	}
	return &numericComponent{compType: FlowPort, values: values}
}

// NewFlowDestPortComponent creates a destination port component.
func NewFlowDestPortComponent(ports ...uint16) FlowComponent {
	values := make([]uint64, len(ports))
	for i, p := range ports {
		values[i] = uint64(p)
	}
	return &numericComponent{compType: FlowDestPort, values: values}
}

// NewFlowSourcePortComponent creates a source port component.
func NewFlowSourcePortComponent(ports ...uint16) FlowComponent {
	values := make([]uint64, len(ports))
	for i, p := range ports {
		values[i] = uint64(p)
	}
	return &numericComponent{compType: FlowSourcePort, values: values}
}

// NewFlowICMPTypeComponent creates an ICMP type component.
func NewFlowICMPTypeComponent(types ...uint8) FlowComponent {
	values := make([]uint64, len(types))
	for i, t := range types {
		values[i] = uint64(t)
	}
	return &numericComponent{compType: FlowICMPType, values: values}
}

// NewFlowICMPCodeComponent creates an ICMP code component.
func NewFlowICMPCodeComponent(codes ...uint8) FlowComponent {
	values := make([]uint64, len(codes))
	for i, c := range codes {
		values[i] = uint64(c)
	}
	return &numericComponent{compType: FlowICMPCode, values: values}
}

// NewFlowTCPFlagsComponent creates a TCP flags component.
func NewFlowTCPFlagsComponent(flags ...uint8) FlowComponent {
	values := make([]uint64, len(flags))
	for i, f := range flags {
		values[i] = uint64(f)
	}
	return &numericComponent{compType: FlowTCPFlags, values: values}
}

// NewFlowPacketLengthComponent creates a packet length component.
func NewFlowPacketLengthComponent(lengths ...uint16) FlowComponent {
	values := make([]uint64, len(lengths))
	for i, l := range lengths {
		values[i] = uint64(l)
	}
	return &numericComponent{compType: FlowPacketLength, values: values}
}

// NewFlowDSCPComponent creates a DSCP component.
func NewFlowDSCPComponent(values ...uint8) FlowComponent {
	vals := make([]uint64, len(values))
	for i, v := range values {
		vals[i] = uint64(v)
	}
	return &numericComponent{compType: FlowDSCP, values: vals}
}

// NewFlowFragmentComponent creates a fragment component.
func NewFlowFragmentComponent(flags ...FlowFragmentFlag) FlowComponent {
	values := make([]uint64, len(flags))
	for i, f := range flags {
		values[i] = uint64(f)
	}
	return &numericComponent{compType: FlowFragment, values: values}
}
