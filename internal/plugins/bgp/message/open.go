package message

import (
	"encoding/binary"
	"fmt"
)

// AS_TRANS is the 2-byte AS used when the real AS is 4 bytes (RFC 6793).
const AS_TRANS = 23456

// RFC 9072 - Extended Optional Parameters constants.
const (
	// ExtendedParamMarker is the marker value (0xFF) for extended format.
	ExtendedParamMarker = 0xFF
)

// Open represents a BGP OPEN message.
// RFC 4271 Section 4.2 - OPEN Message Format
//
// The OPEN message contains:
//   - Version (1 octet): Protocol version number, current BGP version is 4
//   - My Autonomous System (2 octets): AS number of the sender
//   - Hold Time (2 octets): Proposed Hold Timer value in seconds
//   - BGP Identifier (4 octets): BGP Identifier of the sender
//   - Optional Parameters Length (1 octet): Length of optional parameters
//   - Optional Parameters (variable): List of optional parameters
//
// The minimum length of the OPEN message is 29 octets (including the message header).
type Open struct {
	// RFC 4271 Section 4.2 - Version: 1-octet unsigned integer, current BGP version is 4
	Version uint8

	// RFC 4271 Section 4.2 - My Autonomous System: 2-octet unsigned integer
	// Note: Use AS_TRANS (23456) if ASN4 is set per RFC 6793
	MyAS uint16

	// RFC 4271 Section 4.2 - Hold Time: 2-octet unsigned integer
	// Must be either zero or at least three seconds
	HoldTime uint16

	// RFC 4271 Section 4.2 - BGP Identifier: 4-octet unsigned integer
	// Set to an IP address assigned to the BGP speaker, same for all peers
	BGPIdentifier uint32

	// ASN4 is the 4-byte AS number if > 65535.
	// When set, MyAS should be AS_TRANS (23456) per RFC 6793.
	ASN4 uint32

	// RFC 4271 Section 4.2 - Optional Parameters: variable length field
	// Contains a list of optional parameters encoded as TLV triplets.
	// RFC 3392 defines the Capabilities Optional Parameter.
	OptionalParams []byte
}

// Type returns the message type (OPEN).
func (o *Open) Type() MessageType {
	return TypeOPEN
}

// Len returns the total message length in bytes.
// RFC 4271 Section 4.2 - Header (19) + Version (1) + MyAS (2) + HoldTime (2) +
// BGP ID (4) + OptLen (1) + OptParams. Extended format adds 3 bytes for markers + len.
// Context is ignored (context-independent).
func (o *Open) Len(_ *EncodingContext) int {
	optLen := len(o.OptionalParams)
	if optLen > 255 {
		// RFC 9072: Extended format adds 4 bytes (NonExtLen + NonExtType + ExtLen)
		return HeaderLen + 10 + 4 + optLen
	}
	return HeaderLen + 10 + optLen
}

// WriteTo writes the complete OPEN message to buf at offset.
// Returns number of bytes written.
// RFC 4271 Section 4.2 - OPEN message format.
// RFC 9072 Section 2 - Extended format if OptionalParams > 255 bytes.
func (o *Open) WriteTo(buf []byte, off int, _ *EncodingContext) int {
	optLen := len(o.OptionalParams)

	if optLen > 255 {
		return o.writeToExtended(buf, off)
	}

	totalLen := HeaderLen + 10 + optLen
	writeHeader(buf, off, TypeOPEN, totalLen)

	bodyOff := off + HeaderLen
	// Version
	buf[bodyOff] = o.Version
	// My AS (use AS_TRANS if ASN4 > 65535)
	myAS := o.MyAS
	if o.ASN4 > 0 && o.ASN4 > 65535 {
		myAS = AS_TRANS
	}
	binary.BigEndian.PutUint16(buf[bodyOff+1:], myAS)
	// Hold Time
	binary.BigEndian.PutUint16(buf[bodyOff+3:], o.HoldTime)
	// BGP Identifier
	binary.BigEndian.PutUint32(buf[bodyOff+5:], o.BGPIdentifier)
	// Opt Param Length
	buf[bodyOff+9] = byte(optLen)
	// Optional Parameters
	copy(buf[bodyOff+10:], o.OptionalParams)

	return totalLen
}

// writeToExtended writes OPEN with RFC 9072 extended format.
func (o *Open) writeToExtended(buf []byte, off int) int {
	optLen := len(o.OptionalParams)
	totalLen := HeaderLen + 10 + 4 + optLen
	writeHeader(buf, off, TypeOPEN, totalLen)

	bodyOff := off + HeaderLen
	// Version
	buf[bodyOff] = o.Version
	// My AS
	myAS := o.MyAS
	if o.ASN4 > 0 && o.ASN4 > 65535 {
		myAS = AS_TRANS
	}
	binary.BigEndian.PutUint16(buf[bodyOff+1:], myAS)
	// Hold Time
	binary.BigEndian.PutUint16(buf[bodyOff+3:], o.HoldTime)
	// BGP Identifier
	binary.BigEndian.PutUint32(buf[bodyOff+5:], o.BGPIdentifier)
	// RFC 9072: Extended format markers
	buf[bodyOff+9] = ExtendedParamMarker  // Non-Ext OP Len = 255
	buf[bodyOff+10] = ExtendedParamMarker // Non-Ext OP Type = 255
	// Extended Length
	binary.BigEndian.PutUint16(buf[bodyOff+11:], uint16(optLen)) //nolint:gosec // optLen validated ≤ maxOptLen (65535)
	// Optional Parameters
	copy(buf[bodyOff+13:], o.OptionalParams)

	return totalLen
}

// UnpackOpen parses an OPEN message body.
//
// RFC 4271 Section 4.2 - OPEN message wire format:
//
//	+--------+--------+--------+--------+--------+--------+--------+--------+--------+--------+
//	|Version |    My AS (2)    |   Hold Time (2) |       BGP Identifier (4)        |Opt Len |
//	+--------+--------+--------+--------+--------+--------+--------+--------+--------+--------+
//	|                      Optional Parameters (variable)                                    |
//	+----------------------------------------------------------------------------------------+
//
// RFC 9072 Section 2 - Extended format when Optional Parameters exceed 255 bytes:
//
//	+--------+--------+--------+--------+--------+--------+--------+--------+--------+--------+
//	|Version |    My AS (2)    |   Hold Time (2) |       BGP Identifier (4)        |Non-Ext |
//	+--------+--------+--------+--------+--------+--------+--------+--------+--------+--------+
//	|Non-Ext |   Extended Opt. Parm. Length (2)  |      Optional Parameters (var)            |
//	+--------+--------+--------+--------+--------+--------+--------+--------+--------+--------+
//
// RFC 4271 Section 4.2 - Decodes the OPEN message wire format fields:
// Version (1) + My AS (2) + Hold Time (2) + BGP Identifier (4) + Opt Parm Len (1) = 10 octets minimum
//
// RFC 9072 Section 2 - Also handles extended optional parameters format:
// If the first Optional Parameter type is 255, use extended format with 2-byte length.
func UnpackOpen(data []byte) (*Open, error) {
	// RFC 4271 Section 4.2 - Minimum OPEN body is 10 octets (excluding header)
	// Full message minimum is 29 octets (19-byte header + 10-byte body)
	if len(data) < 10 {
		return nil, ErrShortRead
	}

	o := &Open{
		// RFC 4271 Section 4.2 - Version: offset 0, 1 octet
		Version: data[0],
		// RFC 4271 Section 4.2 - My Autonomous System: offset 1-2, 2 octets
		MyAS: binary.BigEndian.Uint16(data[1:3]),
		// RFC 4271 Section 4.2 - Hold Time: offset 3-4, 2 octets
		HoldTime: binary.BigEndian.Uint16(data[3:5]),
		// RFC 4271 Section 4.2 - BGP Identifier: offset 5-8, 4 octets
		BGPIdentifier: binary.BigEndian.Uint32(data[5:9]),
	}

	// RFC 4271 Section 4.2 - Optional Parameters Length field
	optLen := int(data[9])

	// RFC 9072 Section 2 - Check for extended format
	// "If the value of the 'Non-Ext OP Type' field is 255, then the encoding
	// described above is used for the Optional Parameters length."
	if optLen != 0 && len(data) > 10 && data[10] == ExtendedParamMarker {
		// Extended format: need at least 4 bytes after fixed fields
		// (Non-Ext OP Len + Non-Ext OP Type + Extended Length)
		if len(data) < 13 {
			return nil, ErrShortRead
		}

		// RFC 9072 Section 2 - Extended Optional Parameters Length is 2 octets
		extOptLen := int(binary.BigEndian.Uint16(data[11:13]))
		if len(data) < 13+extOptLen {
			return nil, ErrShortRead
		}

		if extOptLen > 0 {
			o.OptionalParams = make([]byte, extOptLen)
			copy(o.OptionalParams, data[13:13+extOptLen])
		}
	} else {
		// Standard format
		if len(data) < 10+optLen {
			return nil, ErrShortRead
		}

		if optLen > 0 {
			o.OptionalParams = make([]byte, optLen)
			copy(o.OptionalParams, data[10:10+optLen])
		}
	}

	return o, nil
}

// RouterID returns the BGP Identifier as a dotted-decimal string.
// RFC 4271 Section 4.2 - BGP Identifier is a 4-octet unsigned integer
// representing an IP address assigned to the BGP speaker.
func (o *Open) RouterID() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		(o.BGPIdentifier>>24)&0xFF,
		(o.BGPIdentifier>>16)&0xFF,
		(o.BGPIdentifier>>8)&0xFF,
		o.BGPIdentifier&0xFF,
	)
}

// ValidateHoldTime checks the Hold Time value per RFC 4271.
// RFC 4271 Section 4.2: "Hold Time MUST be either zero or at least three seconds."
// RFC 4271 Section 6.2: "An implementation MUST reject Hold Time values of one or two seconds."
//
// Returns nil if valid, or a *Notification with Unacceptable Hold Time if invalid.
func (o *Open) ValidateHoldTime() error {
	// RFC 4271: Hold Time must be 0 or >= 3
	if o.HoldTime != 0 && o.HoldTime < 3 {
		return &Notification{
			ErrorCode:    NotifyOpenMessage,
			ErrorSubcode: NotifyOpenUnacceptableHoldTime,
			Data:         []byte{byte(o.HoldTime >> 8), byte(o.HoldTime)},
		}
	}
	return nil
}

// String returns a human-readable representation.
func (o *Open) String() string {
	as := uint32(o.MyAS)
	if o.ASN4 > 0 {
		as = o.ASN4
	}
	return fmt.Sprintf("OPEN AS%d RouterID=%s HoldTime=%d",
		as, o.RouterID(), o.HoldTime)
}
