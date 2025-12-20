package message

import (
	"encoding/binary"
	"fmt"
)

// AS_TRANS is the 2-byte AS used when the real AS is 4 bytes (RFC 6793).
const AS_TRANS = 23456

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

// Pack serializes the OPEN to wire format.
// RFC 4271 Section 4.2 - OPEN message encoding follows the wire format:
//
//	+--------+--------+--------+--------+--------+--------+--------+--------+--------+--------+
//	|Version |    My AS (2)    |   Hold Time (2) |       BGP Identifier (4)        |Opt Len |
//	+--------+--------+--------+--------+--------+--------+--------+--------+--------+--------+
//	|                      Optional Parameters (variable)                                    |
//	+----------------------------------------------------------------------------------------+
func (o *Open) Pack(neg *Negotiated) ([]byte, error) {
	// Calculate body size: Version(1) + AS(2) + Hold(2) + ID(4) + OptLen(1) + Opt
	optLen := len(o.OptionalParams)
	bodyLen := 10 + optLen

	body := make([]byte, bodyLen)

	// RFC 4271 Section 4.2 - Version: 1-octet, current BGP version is 4
	body[0] = o.Version

	// RFC 4271 Section 4.2 - My Autonomous System: 2-octet unsigned integer
	// RFC 6793: Use AS_TRANS (23456) when real AS exceeds 65535
	myAS := o.MyAS
	if o.ASN4 > 0 && o.ASN4 > 65535 {
		myAS = AS_TRANS
	}
	binary.BigEndian.PutUint16(body[1:3], myAS)

	// RFC 4271 Section 4.2 - Hold Time: 2-octet unsigned integer
	binary.BigEndian.PutUint16(body[3:5], o.HoldTime)

	// RFC 4271 Section 4.2 - BGP Identifier: 4-octet unsigned integer
	binary.BigEndian.PutUint32(body[5:9], o.BGPIdentifier)

	// RFC 4271 Section 4.2 - Optional Parameters Length: 1-octet
	body[9] = byte(optLen)

	// RFC 4271 Section 4.2 - Optional Parameters: variable length
	copy(body[10:], o.OptionalParams)

	return packWithHeader(TypeOPEN, body), nil
}

// UnpackOpen parses an OPEN message body.
// RFC 4271 Section 4.2 - Decodes the OPEN message wire format fields:
// Version (1) + My AS (2) + Hold Time (2) + BGP Identifier (4) + Opt Parm Len (1) = 10 octets minimum
func UnpackOpen(data []byte) (*Open, error) {
	// RFC 4271 Section 4.2 - Minimum OPEN body is 10 octets (excluding header)
	// Full message minimum is 29 octets (19-byte header + 10-byte body)
	if len(data) < 10 {
		return nil, ErrShortRead
	}

	// RFC 4271 Section 4.2 - Optional Parameters Length field
	optLen := int(data[9])
	if len(data) < 10+optLen {
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

	// RFC 4271 Section 4.2 - Optional Parameters: variable length, starts at offset 10
	if optLen > 0 {
		o.OptionalParams = make([]byte, optLen)
		copy(o.OptionalParams, data[10:10+optLen])
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

// String returns a human-readable representation.
func (o *Open) String() string {
	as := uint32(o.MyAS)
	if o.ASN4 > 0 {
		as = o.ASN4
	}
	return fmt.Sprintf("OPEN AS%d RouterID=%s HoldTime=%d",
		as, o.RouterID(), o.HoldTime)
}
