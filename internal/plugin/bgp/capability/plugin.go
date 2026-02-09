package capability

// Plugin represents a capability provided by an external plugin.
// This allows plugins to inject arbitrary capability bytes into OPEN messages.
//
// RFC 5492 Section 4: Capabilities are encoded as TLV (Type-Length-Value).
// The plugin provides the raw value bytes, and Pack() wraps them in the TLV format.
type Plugin struct {
	code  Code   // Capability type code
	value []byte // Raw capability value (without TLV header)
}

// NewPlugin creates a capability from plugin-provided bytes.
// The code is the capability type code, and value is the raw capability data
// (not including the TLV header - just the value portion).
func NewPlugin(code uint8, value []byte) *Plugin {
	return &Plugin{
		code:  Code(code),
		value: value,
	}
}

// Code returns the capability type code.
func (p *Plugin) Code() Code {
	return p.code
}

// Pack returns the capability as a TLV (Type-Length-Value) triplet.
//
// Wire format:
//
//	+--------+--------+--------+...+--------+
//	| Code   | Length | Value (variable)   |
//	| (1)    | (1)    |                    |
//	+--------+--------+--------+...+--------+
func (p *Plugin) Pack() []byte {
	result := make([]byte, 2+len(p.value))
	result[0] = byte(p.code)
	result[1] = byte(len(p.value))
	copy(result[2:], p.value)
	return result
}

func (p *Plugin) Len() int { return 2 + len(p.value) }

func (p *Plugin) WriteTo(buf []byte, off int) int {
	writeCapabilityTo(buf, off, p.code, len(p.value))
	copy(buf[off+2:], p.value)
	return p.Len()
}
