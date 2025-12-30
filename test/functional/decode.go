package functional

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strings"
)

// DecodedMessage holds a human-readable representation of a BGP message.
type DecodedMessage struct {
	Type       string
	Length     int
	TypeCode   byte
	Attributes []DecodedAttribute
	NLRI       []string
	Withdrawn  []string
	Raw        string // Original hex for reference
}

// DecodedAttribute holds a decoded path attribute.
type DecodedAttribute struct {
	Code  byte
	Name  string
	Value string
	Flags byte
}

// DecodeMessage decodes a BGP message from hex string to human-readable format.
func DecodeMessage(hexStr string) (*DecodedMessage, error) {
	// Remove colons and whitespace
	hexStr = strings.ReplaceAll(hexStr, ":", "")
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	hexStr = strings.ToUpper(hexStr)

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	return DecodeMessageBytes(data)
}

// DecodeMessageBytes decodes a BGP message from raw bytes.
func DecodeMessageBytes(data []byte) (*DecodedMessage, error) {
	if len(data) < 19 {
		return nil, fmt.Errorf("message too short: %d bytes", len(data))
	}

	msg := &DecodedMessage{
		Length:   int(binary.BigEndian.Uint16(data[16:18])),
		TypeCode: data[18],
		Raw:      strings.ToUpper(hex.EncodeToString(data)),
	}

	switch msg.TypeCode {
	case 1:
		msg.Type = "OPEN"
		decodeOpen(msg, data[19:])
	case 2:
		msg.Type = "UPDATE"
		decodeUpdate(msg, data[19:])
	case 3:
		msg.Type = "NOTIFICATION"
		decodeNotification(msg, data[19:])
	case 4:
		msg.Type = "KEEPALIVE"
	case 5:
		msg.Type = "ROUTE-REFRESH"
	default:
		msg.Type = fmt.Sprintf("UNKNOWN(%d)", msg.TypeCode)
	}

	return msg, nil
}

func decodeOpen(msg *DecodedMessage, body []byte) {
	if len(body) < 10 {
		return
	}

	version := body[0]
	asn := binary.BigEndian.Uint16(body[1:3])
	holdTime := binary.BigEndian.Uint16(body[3:5])
	routerID := netip.AddrFrom4([4]byte{body[5], body[6], body[7], body[8]})
	optParamLen := body[9]

	msg.Attributes = append(msg.Attributes, DecodedAttribute{
		Name:  "version",
		Value: fmt.Sprintf("%d", version),
	})
	msg.Attributes = append(msg.Attributes, DecodedAttribute{
		Name:  "asn",
		Value: fmt.Sprintf("%d", asn),
	})
	msg.Attributes = append(msg.Attributes, DecodedAttribute{
		Name:  "hold-time",
		Value: fmt.Sprintf("%d", holdTime),
	})
	msg.Attributes = append(msg.Attributes, DecodedAttribute{
		Name:  "router-id",
		Value: routerID.String(),
	})
	msg.Attributes = append(msg.Attributes, DecodedAttribute{
		Name:  "opt-param-len",
		Value: fmt.Sprintf("%d", optParamLen),
	})
}

func decodeUpdate(msg *DecodedMessage, body []byte) {
	if len(body) < 4 {
		return
	}

	// Withdrawn routes length
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	if len(body) < 2+withdrawnLen+2 {
		return
	}

	// Parse withdrawn routes
	if withdrawnLen > 0 {
		msg.Withdrawn = decodeNLRI(body[2 : 2+withdrawnLen])
	}

	// Path attributes length
	attrOffset := 2 + withdrawnLen
	attrLen := int(binary.BigEndian.Uint16(body[attrOffset : attrOffset+2]))
	if len(body) < attrOffset+2+attrLen {
		return
	}

	// Parse path attributes
	if attrLen > 0 {
		msg.Attributes = decodePathAttributes(body[attrOffset+2 : attrOffset+2+attrLen])
	}

	// Parse NLRI
	nlriOffset := attrOffset + 2 + attrLen
	if nlriOffset < len(body) {
		msg.NLRI = decodeNLRI(body[nlriOffset:])
	}
}

func decodeNotification(msg *DecodedMessage, body []byte) {
	if len(body) < 2 {
		return
	}

	errCode := body[0]
	errSubcode := body[1]

	msg.Attributes = append(msg.Attributes, DecodedAttribute{
		Name:  "error-code",
		Value: fmt.Sprintf("%d (%s)", errCode, notificationErrorName(errCode)),
	})
	msg.Attributes = append(msg.Attributes, DecodedAttribute{
		Name:  "error-subcode",
		Value: fmt.Sprintf("%d", errSubcode),
	})

	if len(body) > 2 {
		msg.Attributes = append(msg.Attributes, DecodedAttribute{
			Name:  "data",
			Value: hex.EncodeToString(body[2:]),
		})
	}
}

func notificationErrorName(code byte) string {
	names := map[byte]string{
		1: "Message Header Error",
		2: "OPEN Message Error",
		3: "UPDATE Message Error",
		4: "Hold Timer Expired",
		5: "FSM Error",
		6: "Cease",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return "Unknown"
}

func decodePathAttributes(data []byte) []DecodedAttribute {
	var attrs []DecodedAttribute
	offset := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		flags := data[offset]
		code := data[offset+1]

		// Determine header length and value length
		hdrLen := 3
		var valueLen int
		if flags&0x10 != 0 { // Extended length
			if offset+4 > len(data) {
				break
			}
			valueLen = int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
			hdrLen = 4
		} else {
			if offset+3 > len(data) {
				break
			}
			valueLen = int(data[offset+2])
		}

		if offset+hdrLen+valueLen > len(data) {
			break
		}

		value := data[offset+hdrLen : offset+hdrLen+valueLen]
		attr := DecodedAttribute{
			Code:  code,
			Name:  attrCodeName(code),
			Flags: flags,
			Value: decodeAttrValue(code, value),
		}
		attrs = append(attrs, attr)

		offset += hdrLen + valueLen
	}

	return attrs
}

func decodeNLRI(data []byte) []string {
	var prefixes []string
	offset := 0

	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		prefixLen := int(data[offset])
		offset++

		// Calculate bytes needed for prefix
		byteLen := (prefixLen + 7) / 8
		if offset+byteLen > len(data) {
			break
		}

		// Build prefix bytes
		prefixBytes := make([]byte, 4)
		copy(prefixBytes, data[offset:offset+byteLen])

		addr := netip.AddrFrom4([4]byte{prefixBytes[0], prefixBytes[1], prefixBytes[2], prefixBytes[3]})
		prefix := netip.PrefixFrom(addr, prefixLen)
		prefixes = append(prefixes, prefix.String())

		offset += byteLen
	}

	return prefixes
}

func attrCodeName(code byte) string {
	names := map[byte]string{
		1:  "ORIGIN",
		2:  "AS_PATH",
		3:  "NEXT_HOP",
		4:  "MED",
		5:  "LOCAL_PREF",
		6:  "ATOMIC_AGGREGATE",
		7:  "AGGREGATOR",
		8:  "COMMUNITIES",
		9:  "ORIGINATOR_ID",
		10: "CLUSTER_LIST",
		14: "MP_REACH_NLRI",
		15: "MP_UNREACH_NLRI",
		16: "EXT_COMMUNITIES",
		17: "AS4_PATH",
		18: "AS4_AGGREGATOR",
		32: "LARGE_COMMUNITIES",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return fmt.Sprintf("ATTR_%d", code)
}

func decodeAttrValue(code byte, value []byte) string {
	switch code {
	case 1: // ORIGIN
		if len(value) >= 1 {
			origins := []string{"IGP", "EGP", "INCOMPLETE"}
			if int(value[0]) < len(origins) {
				return origins[value[0]]
			}
		}
		return hex.EncodeToString(value)

	case 2: // AS_PATH
		return decodeASPath(value)

	case 3: // NEXT_HOP
		if len(value) == 4 {
			addr := netip.AddrFrom4([4]byte{value[0], value[1], value[2], value[3]})
			return addr.String()
		}
		return hex.EncodeToString(value)

	case 4: // MED
		if len(value) == 4 {
			return fmt.Sprintf("%d", binary.BigEndian.Uint32(value))
		}
		return hex.EncodeToString(value)

	case 5: // LOCAL_PREF
		if len(value) == 4 {
			return fmt.Sprintf("%d", binary.BigEndian.Uint32(value))
		}
		return hex.EncodeToString(value)

	case 8: // COMMUNITIES
		return decodeCommunities(value)

	case 16: // EXT_COMMUNITIES
		return decodeExtCommunities(value)

	default:
		return hex.EncodeToString(value)
	}
}

func decodeASPath(data []byte) string {
	if len(data) == 0 {
		return "[]"
	}

	var parts []string
	offset := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		segType := data[offset]
		segLen := int(data[offset+1])
		offset += 2

		// Assume 4-byte ASNs (common now), try 2-byte if it doesn't fit
		asnSize := 4
		if offset+segLen*4 > len(data) {
			asnSize = 2
		}
		if offset+segLen*asnSize > len(data) {
			break
		}

		var asns []string
		for i := 0; i < segLen; i++ {
			var asn uint32
			if asnSize == 4 {
				asn = binary.BigEndian.Uint32(data[offset : offset+4])
			} else {
				asn = uint32(binary.BigEndian.Uint16(data[offset : offset+2]))
			}
			asns = append(asns, fmt.Sprintf("%d", asn))
			offset += asnSize
		}

		switch segType {
		case 1: // AS_SET
			parts = append(parts, "{"+strings.Join(asns, " ")+"}")
		case 2: // AS_SEQUENCE
			parts = append(parts, strings.Join(asns, " "))
		default:
			parts = append(parts, strings.Join(asns, " "))
		}
	}

	return "[" + strings.Join(parts, " ") + "]"
}

func decodeCommunities(data []byte) string {
	var comms []string
	for i := 0; i+4 <= len(data); i += 4 {
		high := binary.BigEndian.Uint16(data[i : i+2])
		low := binary.BigEndian.Uint16(data[i+2 : i+4])
		comms = append(comms, fmt.Sprintf("%d:%d", high, low))
	}
	return strings.Join(comms, " ")
}

func decodeExtCommunities(data []byte) string {
	var comms []string
	for i := 0; i+8 <= len(data); i += 8 {
		comms = append(comms, fmt.Sprintf("0x%s", hex.EncodeToString(data[i:i+8])))
	}
	return strings.Join(comms, " ")
}

// String returns a human-readable representation.
func (m *DecodedMessage) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s (len=%d)\n", m.Type, m.Length))

	for _, attr := range m.Attributes {
		sb.WriteString(fmt.Sprintf("  %s: %s\n", attr.Name, attr.Value))
	}

	if len(m.NLRI) > 0 {
		sb.WriteString(fmt.Sprintf("  NLRI: %s\n", strings.Join(m.NLRI, ", ")))
	}

	if len(m.Withdrawn) > 0 {
		sb.WriteString(fmt.Sprintf("  WITHDRAWN: %s\n", strings.Join(m.Withdrawn, ", ")))
	}

	return sb.String()
}

// ColoredString returns a colored human-readable representation.
func (m *DecodedMessage) ColoredString(c *Colors) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("  %s%s%s (len=%d)\n", c.Cyan("type:"), "      ", m.Type, m.Length))

	for _, attr := range m.Attributes {
		sb.WriteString(fmt.Sprintf("    %s: %s\n", c.Gray(attr.Name), attr.Value))
	}

	if len(m.NLRI) > 0 {
		sb.WriteString(fmt.Sprintf("  %s%s%s\n", c.Gray("nlri:"), "      ", strings.Join(m.NLRI, ", ")))
	}

	if len(m.Withdrawn) > 0 {
		sb.WriteString(fmt.Sprintf("  %s%s%s\n", c.Gray("withdrawn:"), " ", strings.Join(m.Withdrawn, ", ")))
	}

	return sb.String()
}

// Diff compares two messages and returns a human-readable diff.
func Diff(expected, received string) string {
	expMsg, expErr := DecodeMessage(expected)
	rcvMsg, rcvErr := DecodeMessage(received)

	var sb strings.Builder
	sb.WriteString("\n--- Expected vs Received ---\n")

	if expErr != nil {
		sb.WriteString(fmt.Sprintf("Expected (decode error): %v\n", expErr))
		sb.WriteString(fmt.Sprintf("  Raw: %s\n", expected))
	} else {
		sb.WriteString(fmt.Sprintf("Expected: %s", expMsg.String()))
	}

	sb.WriteString("\n")

	if rcvErr != nil {
		sb.WriteString(fmt.Sprintf("Received (decode error): %v\n", rcvErr))
		sb.WriteString(fmt.Sprintf("  Raw: %s\n", received))
	} else {
		sb.WriteString(fmt.Sprintf("Received: %s", rcvMsg.String()))
	}

	// Show attribute differences if both decoded successfully
	if expErr == nil && rcvErr == nil {
		sb.WriteString("\nDifferences:\n")

		// Build maps for comparison
		expAttrs := make(map[string]string)
		rcvAttrs := make(map[string]string)

		for _, a := range expMsg.Attributes {
			expAttrs[a.Name] = a.Value
		}
		for _, a := range rcvMsg.Attributes {
			rcvAttrs[a.Name] = a.Value
		}

		// Find differences
		allKeys := make(map[string]bool)
		for k := range expAttrs {
			allKeys[k] = true
		}
		for k := range rcvAttrs {
			allKeys[k] = true
		}

		hasDiff := false
		for key := range allKeys {
			expVal, hasExp := expAttrs[key]
			rcvVal, hasRcv := rcvAttrs[key]

			switch {
			case !hasExp:
				sb.WriteString(fmt.Sprintf("  + %s: %s (unexpected)\n", key, rcvVal))
				hasDiff = true
			case !hasRcv:
				sb.WriteString(fmt.Sprintf("  - %s: %s (missing)\n", key, expVal))
				hasDiff = true
			case expVal != rcvVal:
				sb.WriteString(fmt.Sprintf("  ~ %s: expected=%s, got=%s\n", key, expVal, rcvVal))
				hasDiff = true
			}
		}

		// NLRI differences
		expNLRI := strings.Join(expMsg.NLRI, ",")
		rcvNLRI := strings.Join(rcvMsg.NLRI, ",")
		if expNLRI != rcvNLRI {
			sb.WriteString(fmt.Sprintf("  ~ NLRI: expected=%s, got=%s\n", expNLRI, rcvNLRI))
			hasDiff = true
		}

		if !hasDiff {
			sb.WriteString("  (no attribute differences detected - check raw bytes)\n")
		}
	}

	return sb.String()
}

// ColoredDiff compares two messages with colored output.
func ColoredDiff(expected, received string, c *Colors) string {
	expMsg, expErr := DecodeMessage(expected)
	rcvMsg, rcvErr := DecodeMessage(received)

	var sb strings.Builder

	if expErr != nil || rcvErr != nil {
		// Fallback to plain diff
		return Diff(expected, received)
	}

	// Build maps for comparison
	expAttrs := make(map[string]string)
	rcvAttrs := make(map[string]string)

	for _, a := range expMsg.Attributes {
		expAttrs[a.Name] = a.Value
	}
	for _, a := range rcvMsg.Attributes {
		rcvAttrs[a.Name] = a.Value
	}

	// Find differences
	allKeys := make(map[string]bool)
	for k := range expAttrs {
		allKeys[k] = true
	}
	for k := range rcvAttrs {
		allKeys[k] = true
	}

	for key := range allKeys {
		expVal, hasExp := expAttrs[key]
		rcvVal, hasRcv := rcvAttrs[key]

		switch {
		case !hasExp:
			sb.WriteString(fmt.Sprintf("  %s: %s (unexpected)\n", key, c.Red("+"+rcvVal)))
		case !hasRcv:
			sb.WriteString(fmt.Sprintf("  %s: %s (missing)\n", key, c.Green("-"+expVal)))
		case expVal != rcvVal:
			sb.WriteString(fmt.Sprintf("  %s: %s %s\n", key, c.Green("-"+expVal), c.Red("+"+rcvVal)))
		}
	}

	// NLRI differences
	expNLRI := strings.Join(expMsg.NLRI, ",")
	rcvNLRI := strings.Join(rcvMsg.NLRI, ",")
	if expNLRI != rcvNLRI {
		sb.WriteString(fmt.Sprintf("  NLRI: %s %s\n", c.Green("-"+expNLRI), c.Red("+"+rcvNLRI)))
	}

	// Find byte-level differences
	byteDiff := findByteDiff(expected, received)
	if byteDiff != "" {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c.Gray("raw diff:"), byteDiff))
	}

	return sb.String()
}

// findByteDiff finds the first differing bytes between two hex strings.
func findByteDiff(exp, rcv string) string {
	exp = strings.ReplaceAll(strings.ReplaceAll(exp, ":", ""), " ", "")
	rcv = strings.ReplaceAll(strings.ReplaceAll(rcv, ":", ""), " ", "")

	minLen := len(exp)
	if len(rcv) < minLen {
		minLen = len(rcv)
	}

	for i := 0; i < minLen; i += 2 {
		end := i + 2
		if end > minLen {
			end = minLen
		}
		if exp[i:end] != rcv[i:end] {
			bytePos := i / 2
			return fmt.Sprintf("byte %d: %s vs %s", bytePos, exp[i:end], rcv[i:end])
		}
	}

	if len(exp) != len(rcv) {
		return fmt.Sprintf("length: %d vs %d", len(exp)/2, len(rcv)/2)
	}

	return ""
}
