// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4271.md — OPEN message format (Section 4.2)
// Overview: message.go — Message interface and writeHeader
// Related: notification.go — NOTIFICATION message parsing and encoding
// Related: keepalive.go — KEEPALIVE message encoding
// Related: routerefresh.go — ROUTE-REFRESH message encoding
// Related: update.go — UPDATE message wire representation

package message

import (
	"encoding/binary"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/core/textbuf"
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
func (o *Open) Type() msgtype.MessageType {
	return msgtype.TypeOPEN
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

// writeFixedFields encodes the common OPEN fixed fields (Version, MyAS, HoldTime, BGPIdentifier)
// starting at bodyOff. Returns 9 (the number of bytes written).
// RFC 4271 Section 4.2 - fixed field layout: Version(1) + MyAS(2) + HoldTime(2) + BGPID(4).
func (o *Open) writeFixedFields(buf []byte, bodyOff int) {
	buf[bodyOff] = o.Version
	myAS := o.MyAS
	if o.ASN4 > 0 && o.ASN4 > 65535 {
		myAS = AS_TRANS
	}
	binary.BigEndian.PutUint16(buf[bodyOff+1:], myAS)
	binary.BigEndian.PutUint16(buf[bodyOff+3:], o.HoldTime)
	binary.BigEndian.PutUint32(buf[bodyOff+5:], o.BGPIdentifier)
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
	writeHeader(buf, off, msgtype.TypeOPEN, totalLen)

	bodyOff := off + HeaderLen
	o.writeFixedFields(buf, bodyOff)
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
	writeHeader(buf, off, msgtype.TypeOPEN, totalLen)

	bodyOff := off + HeaderLen
	o.writeFixedFields(buf, bodyOff)
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
// If Non-Ext OP Len (data[9]) is 255 AND Non-Ext OP Type (data[10]) is 255,
// use extended format with 2-byte length.
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
	// Non-Ext OP Len must be 255 (reserved marker, not valid as standard length).
	// "If the Non-Ext OP Len is not 255, the Non-Ext OP Type field and the
	// Extended Opt. Parm. Length field SHOULD be treated as part of the original
	// Optional Parameters."
	if optLen == 255 && len(data) > 10 && data[10] == ExtendedParamMarker {
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
	id := o.BGPIdentifier
	return textbuf.StringAddr(netip.AddrFrom4([4]byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}))
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

// ValidateBGPIdentifier checks the BGP Identifier of a RECEIVED OPEN per RFC 6286.
//
// RFC 6286 Section 2.1: "The BGP Identifier is a 4-octet, unsigned, non-zero integer that
// should be unique within an AS."
//
// RFC 6286 Section 2.2: "If the BGP Identifier field of the OPEN message is zero, or if it
// is the same as the BGP Identifier of the local BGP speaker and the message is from an
// internal peer, then the Error Subcode is set to 'Bad BGP Identifier'."
//
// The self-identifier half is gated on the peer being internal on purpose: an EXTERNAL peer
// may legitimately carry the same identifier as this speaker (that is what AS-wide rather
// than global uniqueness means), and RFC 6286 Section 2.3 -- not this validator -- resolves a
// connection collision between two speakers that share one.
//
// localID is this speaker's BGP Identifier; internal reports whether the peer is in the same
// AS as this speaker. Returns nil if valid, or a *Notification carrying OPEN Message Error /
// Bad BGP Identifier. The Data field is empty: RFC 4271 Section 6.2 defines no data for this
// subcode (unlike Unsupported Version and Unacceptable Hold Time, which echo the value).
func (o *Open) ValidateBGPIdentifier(localID uint32, internal bool) error {
	zero := o.BGPIdentifier == 0
	selfFromInternal := internal && o.BGPIdentifier == localID
	if !zero && !selfFromInternal {
		return nil
	}
	return &Notification{
		ErrorCode:    NotifyOpenMessage,
		ErrorSubcode: NotifyOpenBadBGPID,
	}
}

// String returns a human-readable representation.
func (o *Open) String() string {
	as := uint32(o.MyAS)
	if o.ASN4 > 0 {
		as = o.ASN4
	}
	var b textbuf.Buffer
	return b.Reset().Str("OPEN AS").Uint32(as).Str(" RouterID=").Str(o.RouterID()).Str(" HoldTime=").Uint16(o.HoldTime).String()
}
