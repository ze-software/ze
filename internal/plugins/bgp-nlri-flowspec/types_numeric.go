// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec numeric/bitmask components
// Design: rfc/short/rfc5575.md
// Related: types.go — core FlowSpec types, constants, and interface
// Related: types_prefix.go — prefix component implementations
// Related: types_vpn.go — FlowSpec VPN wrapper

package bgp_nlri_flowspec

import (
	"encoding/binary"
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wire"
)

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

// Values returns just the match values as a plain slice.
func (c *numericComponent) Values() []uint64 {
	vals := make([]uint64, len(c.matches))
	for i, m := range c.matches {
		vals[i] = m.Value
	}
	return vals
}

// Bytes returns the wire encoding per RFC 8955 Section 4.2.1.1.
// Format: <type (1 octet), [numeric_op, value]+>.
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
		default: // > 0xFFFF
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

// String returns command-style format: "<type> <op><value>...".
// For bitmask types (TCP flags, Fragment), uses named flags instead of numbers.
// Example: "destination-port =80 =443" or "tcp-flags syn&ack".
func (c *numericComponent) String() string {
	// Handle bitmask types specially
	if c.compType == FlowTCPFlags || c.compType == FlowFragment {
		return c.bitmaskString()
	}
	return c.numericString()
}

// numericString formats numeric components with operators.
// NOTE: The parser automatically sets And=true for second+ values based on position,
// so we do NOT output & prefix for numeric components (only for bitmask types).
//
// Special case: protocol component uses a custom parser that accepts plain numeric
// values without operator prefix, so we output "protocol 6" not "protocol =6".
func (c *numericComponent) numericString() string {
	parts := make([]string, len(c.matches))
	for i, m := range c.matches {
		// Protocol uses custom parser that doesn't handle operator prefix
		if c.compType == FlowIPProtocol {
			parts[i] = fmt.Sprintf("%d", m.Value)
			continue
		}

		switch m.Op &^ (FlowOpEnd | FlowOpAnd | FlowOpLenMask) { //nolint:exhaustive // Mask out non-comparison bits
		case FlowOpGreater:
			parts[i] = fmt.Sprintf(">%d", m.Value)
		case FlowOpLess:
			parts[i] = fmt.Sprintf("<%d", m.Value)
		case FlowOpEqual:
			parts[i] = fmt.Sprintf("=%d", m.Value)
		case FlowOpNotEq:
			parts[i] = fmt.Sprintf("!=%d", m.Value)
		case FlowOpGreater | FlowOpEqual:
			parts[i] = fmt.Sprintf(">=%d", m.Value)
		case FlowOpLess | FlowOpEqual:
			parts[i] = fmt.Sprintf("<=%d", m.Value)
		default: // no operator bits set — treat as equality
			parts[i] = fmt.Sprintf("=%d", m.Value)
		}
	}
	return fmt.Sprintf("%s %s", c.compType, strings.Join(parts, " "))
}

// bitmaskString formats TCP flags and Fragment components with named flags.
func (c *numericComponent) bitmaskString() string {
	parts := make([]string, len(c.matches))
	for i, m := range c.matches {
		var prefix string
		if m.And && i > 0 {
			prefix = "&"
		}
		// Check for bitmask operator bits (NOT, Match)
		if m.Op&FlowOpNot != 0 {
			prefix += "!"
		}
		if m.Op&FlowOpMatch != 0 {
			prefix += "="
		}

		var flagStr string
		if c.compType == FlowTCPFlags {
			flagStr = tcpFlagsToString(uint8(m.Value)) //nolint:gosec // TCP flags are 8-bit
		} else {
			flagStr = fragmentFlagsToString(FlowFragmentFlag(m.Value))
		}
		parts[i] = prefix + flagStr
	}
	return fmt.Sprintf("%s %s", c.compType, strings.Join(parts, " "))
}

// tcpFlagsToString converts TCP flags byte to named flags.
// RFC 8955 Section 4.2.2.9: TCP flags are in order FIN, SYN, RST, PSH, ACK, URG, ECE, CWR.
func tcpFlagsToString(flags uint8) string {
	var names []string
	if flags&0x01 != 0 {
		names = append(names, "fin")
	}
	if flags&0x02 != 0 {
		names = append(names, "syn")
	}
	if flags&0x04 != 0 {
		names = append(names, "rst")
	}
	if flags&0x08 != 0 {
		names = append(names, "psh")
	}
	if flags&0x10 != 0 {
		names = append(names, "ack")
	}
	if flags&0x20 != 0 {
		names = append(names, "urg")
	}
	if flags&0x40 != 0 {
		names = append(names, "ece")
	}
	if flags&0x80 != 0 {
		names = append(names, "cwr")
	}
	if len(names) == 0 {
		return "0"
	}
	return strings.Join(names, "&")
}

// fragmentFlagsToString converts fragment flags to named flags.
// RFC 8955 Section 4.2.2.12: Fragment bitmask operand.
func fragmentFlagsToString(flags FlowFragmentFlag) string {
	var names []string
	if flags&FlowFragDontFragment != 0 {
		names = append(names, "dont-fragment")
	}
	if flags&FlowFragIsFragment != 0 {
		names = append(names, "is-fragment")
	}
	if flags&FlowFragFirstFragment != 0 {
		names = append(names, "first-fragment")
	}
	if flags&FlowFragLastFragment != 0 {
		names = append(names, "last-fragment")
	}
	if len(names) == 0 {
		return "0"
	}
	return strings.Join(names, "&")
}

// Len returns the wire-format length in bytes.
func (c *numericComponent) Len() int {
	n := 1 // type byte
	for _, m := range c.matches {
		n++ // operator byte
		switch {
		case m.Value <= 0xFF:
			n++
		case m.Value <= 0xFFFF:
			n += 2
		default: // > 0xFFFF
			n += 4
		}
	}
	return n
}

// WriteTo writes the component directly to buf at offset.
// Returns bytes written.
func (c *numericComponent) WriteTo(buf []byte, off int) int {
	pos := off
	buf[pos] = byte(c.compType)
	pos++

	for i, m := range c.matches {
		// Determine value length
		var lenCode, valueLen byte
		switch {
		case m.Value <= 0xFF:
			lenCode, valueLen = 0, 1
		case m.Value <= 0xFFFF:
			lenCode, valueLen = 1, 2
		default: // > 0xFFFF
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
		op |= byte(m.Op)

		buf[pos] = op
		pos++

		// Encode value
		switch valueLen {
		case 1:
			buf[pos] = byte(m.Value)
			pos++
		case 2:
			buf[pos] = byte(m.Value >> 8)
			buf[pos+1] = byte(m.Value)
			pos += 2
		case 4:
			binary.BigEndian.PutUint32(buf[pos:], uint32(m.Value)) //nolint:gosec // Flowspec value size validated
			pos += 4
		}
	}

	return pos - off
}

// CheckedWriteTo validates capacity before writing.
func (c *numericComponent) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := c.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return c.WriteTo(buf, off), nil
}

// parseNumericComponent parses a numeric-type component (Types 3-12).
// RFC 8955 Section 4.2.1.1 defines the numeric operator format.
// The component consists of a list of {operator, value} pairs.
// Encoding: <type (1 octet), [numeric_op, value]+>.
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
		valueLen := min(1<<lenCode,
			// Note: RFC allows 8 octets (len=11), but we cap at 4 for uint32 values
			4)

		if offset+valueLen > len(data) {
			return nil, nil, ErrFlowSpecTruncated
		}

		// Read value in network byte order (big-endian)
		var value uint64
		for i := range valueLen {
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

// ============================================================================
// Component constructors
// ============================================================================

// NewFlowNumericComponent creates a numeric component with explicit matches.
// This is the general constructor for any numeric component type (3-12).
func NewFlowNumericComponent(compType FlowComponentType, matches []FlowMatch) FlowComponent {
	return &numericComponent{compType: compType, matches: matches}
}

// Helper to convert simple values to FlowMatch with equality operator.
//
//nolint:unused // Prepared for FlowSpec config parsing (not yet implemented)
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

// NewFlowFragmentMatchComponent creates a fragment component with explicit matches.
// Allows specifying bitmask operator bits (NOT, Match) per RFC 8955 Section 4.2.1.2.
func NewFlowFragmentMatchComponent(matchList []FlowMatch) FlowComponent {
	return &numericComponent{compType: FlowFragment, matches: matchList}
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
