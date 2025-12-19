package message

import (
	"encoding/binary"
	"fmt"
)

// AS_TRANS is the 2-byte AS used when the real AS is 4 bytes (RFC 6793).
const AS_TRANS = 23456

// Open represents a BGP OPEN message (RFC 4271 Section 4.2).
type Open struct {
	Version       uint8
	MyAS          uint16 // 2-byte AS (use AS_TRANS if ASN4 is set)
	HoldTime      uint16
	BGPIdentifier uint32 // Router ID

	// ASN4 is the 4-byte AS number if > 65535.
	// When set, MyAS should be AS_TRANS (23456).
	ASN4 uint32

	// OptionalParams contains raw optional parameters (capabilities).
	// Capability parsing is handled separately.
	OptionalParams []byte
}

// Type returns the message type (OPEN).
func (o *Open) Type() MessageType {
	return TypeOPEN
}

// Pack serializes the OPEN to wire format.
func (o *Open) Pack(neg *Negotiated) ([]byte, error) {
	// Calculate body size
	optLen := len(o.OptionalParams)
	bodyLen := 10 + optLen // Version(1) + AS(2) + Hold(2) + ID(4) + OptLen(1) + Opt

	body := make([]byte, bodyLen)

	// Version
	body[0] = o.Version

	// AS (use AS_TRANS if 4-byte AS)
	myAS := o.MyAS
	if o.ASN4 > 0 && o.ASN4 > 65535 {
		myAS = AS_TRANS
	}
	binary.BigEndian.PutUint16(body[1:3], myAS)

	// Hold Time
	binary.BigEndian.PutUint16(body[3:5], o.HoldTime)

	// BGP Identifier
	binary.BigEndian.PutUint32(body[5:9], o.BGPIdentifier)

	// Optional Parameters Length
	body[9] = byte(optLen)

	// Optional Parameters
	copy(body[10:], o.OptionalParams)

	return packWithHeader(TypeOPEN, body), nil
}

// UnpackOpen parses an OPEN message body.
func UnpackOpen(data []byte) (*Open, error) {
	if len(data) < 10 {
		return nil, ErrShortRead
	}

	optLen := int(data[9])
	if len(data) < 10+optLen {
		return nil, ErrShortRead
	}

	o := &Open{
		Version:       data[0],
		MyAS:          binary.BigEndian.Uint16(data[1:3]),
		HoldTime:      binary.BigEndian.Uint16(data[3:5]),
		BGPIdentifier: binary.BigEndian.Uint32(data[5:9]),
	}

	if optLen > 0 {
		o.OptionalParams = make([]byte, optLen)
		copy(o.OptionalParams, data[10:10+optLen])
	}

	return o, nil
}

// RouterID returns the BGP Identifier as a dotted-decimal string.
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
