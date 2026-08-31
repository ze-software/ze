// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS wire format
// RFC: rfc/short/rfc2865.md -- Section 3 packet format and Response Authenticator
// RFC: rfc/short/rfc2866.md -- Section 3 Accounting-Request Authenticator
// RFC: rfc/short/rfc2869.md -- Section 5.14 Message-Authenticator on a response
// RFC: rfc/short/rfc5176.md -- Section 2.3 Request Authenticator, Section 3.4 Message-Authenticator
// Related: dict.go -- packet codes and attribute type constants
// Related: attr.go -- attribute encode/decode helpers

package radius

import (
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // RFC 2865 requires MD5 for authenticator computation
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
)

// Packet is a decoded RADIUS packet.
type Packet struct {
	Code          uint8
	Identifier    uint8
	Authenticator [AuthenticatorLen]byte
	Attrs         []Attr
}

// Attr is a decoded RADIUS attribute (Type-Length-Value).
type Attr struct {
	Type  uint8
	Value []byte
}

// RandomAuthenticator generates a cryptographically random 16-byte authenticator.
func RandomAuthenticator() ([AuthenticatorLen]byte, error) {
	var auth [AuthenticatorLen]byte
	if _, err := rand.Read(auth[:]); err != nil {
		return auth, fmt.Errorf("radius: random authenticator: %w", err)
	}
	return auth, nil
}

// EncodeTo writes the packet into buf starting at offset off.
// Returns the number of bytes written. The caller MUST provide a
// buffer of at least MaxPacketLen bytes.
//
// RFC 2865 Section 3: Code(1) + Identifier(1) + Length(2) + Authenticator(16) + Attributes.
func (p *Packet) EncodeTo(buf []byte, off int) (int, error) {
	start := off

	if len(buf)-off < HeaderLen {
		return 0, errors.New("radius: buffer too small for header")
	}

	buf[off] = p.Code
	off++
	buf[off] = p.Identifier
	off++

	lengthPos := off
	off += 2

	copy(buf[off:off+AuthenticatorLen], p.Authenticator[:])
	off += AuthenticatorLen

	for _, a := range p.Attrs {
		// RFC 2865 Section 5: "Text of length zero (0) MUST NOT be sent; omit the
		// entire attribute instead." The next paragraph repeats it for String, and
		// address, integer and time are each four octets, so no RADIUS data type
		// has a legal zero-octet value. Omitting is what the RFC asks for.
		//
		// AppendTextAttr (attr.go) holds the same rule on the caller's side, where
		// the empty value is known by name. This is the paired check at the wire
		// boundary, so an attribute assembled any other way cannot go out empty.
		if len(a.Value) == 0 {
			continue
		}

		attrLen := 2 + len(a.Value)
		if attrLen > MaxAttrLen {
			return 0, fmt.Errorf("radius: attribute type %d too long (%d)", a.Type, attrLen)
		}
		if off+attrLen > start+MaxPacketLen {
			return 0, errors.New("radius: packet exceeds 4096 bytes")
		}
		buf[off] = a.Type
		buf[off+1] = uint8(attrLen)
		copy(buf[off+2:], a.Value)
		off += attrLen
	}

	totalLen := off - start
	binary.BigEndian.PutUint16(buf[lengthPos:], uint16(totalLen))

	return totalLen, nil
}

// Decode parses a RADIUS packet from wire bytes.
func Decode(data []byte) (*Packet, error) {
	if len(data) < MinPacketLen {
		return nil, fmt.Errorf("radius: packet too short (%d < %d)", len(data), MinPacketLen)
	}
	if len(data) > MaxPacketLen {
		return nil, fmt.Errorf("radius: packet too long (%d > %d)", len(data), MaxPacketLen)
	}

	pktLen := int(binary.BigEndian.Uint16(data[2:4]))
	if pktLen < MinPacketLen || pktLen > len(data) {
		return nil, fmt.Errorf("radius: invalid length field %d (data %d)", pktLen, len(data))
	}

	p := &Packet{
		Code:       data[0],
		Identifier: data[1],
	}
	copy(p.Authenticator[:], data[4:4+AuthenticatorLen])

	off := HeaderLen
	for off < pktLen {
		if off+2 > pktLen {
			return nil, errors.New("radius: truncated attribute header")
		}
		attrType := data[off]
		attrLen := int(data[off+1])
		if attrLen < 2 || off+attrLen > pktLen {
			return nil, fmt.Errorf("radius: invalid attribute length %d at offset %d", attrLen, off)
		}
		val := make([]byte, attrLen-2)
		copy(val, data[off+2:off+attrLen])
		p.Attrs = append(p.Attrs, Attr{Type: attrType, Value: val})
		off += attrLen
	}

	return p, nil
}

// FindAttr returns the value of the first attribute with the given type, or nil.
func (p *Packet) FindAttr(attrType uint8) []byte {
	for _, a := range p.Attrs {
		if a.Type == attrType {
			return a.Value
		}
	}
	return nil
}

// FindAllAttr returns all attribute values with the given type.
func (p *Packet) FindAllAttr(attrType uint8) [][]byte {
	var result [][]byte
	for _, a := range p.Attrs {
		if a.Type == attrType {
			result = append(result, a.Value)
		}
	}
	return result
}

// ResponseAuthenticator computes the expected response authenticator.
// RFC 2865 Section 3: MD5(Code+ID+Length+RequestAuth+Attributes+Secret).
func ResponseAuthenticator(code, id uint8, length uint16, requestAuth [AuthenticatorLen]byte, attrs, secret []byte) [AuthenticatorLen]byte {
	h := md5.New() //nolint:gosec // RFC 2865 mandates MD5
	h.Write([]byte{code, id})
	var lenBuf [2]byte
	binary.BigEndian.PutUint16(lenBuf[:], length)
	h.Write(lenBuf[:])
	h.Write(requestAuth[:])
	h.Write(attrs)
	h.Write(secret)
	var auth [AuthenticatorLen]byte
	copy(auth[:], h.Sum(nil))
	return auth
}

// VerifyResponseAuth checks that a response packet's authenticator matches
// the expected value. Uses constant-time comparison.
func VerifyResponseAuth(response []byte, requestAuth [AuthenticatorLen]byte, secret []byte) bool {
	if len(response) < MinPacketLen {
		return false
	}
	pktLen := binary.BigEndian.Uint16(response[2:4])
	if int(pktLen) > len(response) {
		return false
	}
	expected := ResponseAuthenticator(
		response[0], response[1], pktLen,
		requestAuth, response[HeaderLen:pktLen], secret,
	)
	return subtle.ConstantTimeCompare(response[4:4+AuthenticatorLen], expected[:]) == 1
}

// VerifyCoARequestAuth checks the Request Authenticator of a CoA-Request or
// Disconnect-Request. It reports whether a Dynamic Authorization Server may act
// on the datagram. Uses constant-time comparison.
//
// RFC 5176 Section 2.3: "In Request packets, the Authenticator value is a
// 16-octet MD5 [RFC1321] checksum, called the Request Authenticator.  The
// Request Authenticator is calculated the same way as for an
// Accounting-Request, specified in [RFC2866]." AccountingRequestAuth is that
// formula.
//
// The octet stream the MD5 covers, with byte offsets:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Code      |  Identifier   |            Length             |  0..3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|      Request Authenticator (16 octets of zero, substituted)   |  4..19
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Attributes, Message-Authenticator value AS RECEIVED ...      |  20..Length-1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Shared secret ...                                            |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// RFC 5176 Section 3.4: "The Message-Authenticator Attribute is calculated and
// inserted in the packet before the Request Authenticator is calculated." The
// attribute therefore holds its real HMAC in the stream this MD5 covers, so it
// is passed through unchanged rather than zeroed.
func VerifyCoARequestAuth(data, secret []byte) bool {
	if len(data) < MinPacketLen {
		return false
	}
	pktLen := int(binary.BigEndian.Uint16(data[2:4]))
	if pktLen < MinPacketLen || pktLen > len(data) {
		return false
	}
	expected := AccountingRequestAuth(data[:pktLen], pktLen, secret)
	return subtle.ConstantTimeCompare(data[4:4+AuthenticatorLen], expected[:]) == 1
}

// VerifyCoAMessageAuthenticator checks the Message-Authenticator attribute
// (type 80) of a CoA-Request or Disconnect-Request. It reports whether a
// Dynamic Authorization Server may act on the datagram.
//
// A datagram carrying no Message-Authenticator fails, because this function
// answers one question only: does a present attribute verify. RFC 5176 Section
// 3.4 makes the attribute a MAY, so absence is not a refusal, and the caller
// decides. coaListener.handlePacket
// (internal/component/l2tp/plugins/authradius/coa.go) reads presence itself and
// calls this function only when the attribute is there; the absent case is the
// `require-message-authenticator` leaf's business. A datagram whose attribute
// list cannot be walked fails too.
//
// RFC 5176 Section 3.4: "A Dynamic Authorization Server receiving a CoA-Request
// or Disconnect-Request with a Message-Authenticator Attribute present MUST
// calculate the correct value of the Message-Authenticator and silently discard
// the packet if it does not match the value sent."
//
// RFC 5176 Section 3.4: "Message-Authenticator = HMAC-MD5 (Type, Identifier,
// Length, Request Authenticator, Attributes)".
//
// The octet stream the HMAC covers, with byte offsets:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Code      |  Identifier   |            Length             |  0..3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|      Request Authenticator (16 octets of zero, substituted)   |  4..19
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Attributes, Message-Authenticator value zeroed ...           |  20..Length-1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// The wire datagram carries a computed Request Authenticator at offsets 4 to
// 19, because RFC 5176 Section 3.4 computes the Message-Authenticator first and
// the Request Authenticator over the finished packet. Sixteen zeros are
// substituted there before the HMAC runs. An Access-Request differs and is not
// this function's business: RFC 3579 Section 3.2 carries a random nonce in that
// field and hashes it as it stands.
func VerifyCoAMessageAuthenticator(data, secret []byte) bool {
	if len(data) < MinPacketLen {
		return false
	}
	pktLen := int(binary.BigEndian.Uint16(data[2:4]))
	if pktLen < MinPacketLen || pktLen > len(data) {
		return false
	}

	buf := make([]byte, pktLen)
	copy(buf, data[:pktLen])
	maOff, present, ok := messageAuthenticatorValueOffset(buf)
	if !ok || !present {
		return false
	}
	var received [AuthenticatorLen]byte
	copy(received[:], buf[maOff:maOff+AuthenticatorLen])

	// RFC 5176 Section 3.4: "When the HMAC-MD5 message integrity check is
	// calculated the Request Authenticator field and Message-Authenticator
	// Attribute MUST each be considered to be sixteen octets of zero."
	clear(buf[4 : 4+AuthenticatorLen])
	clear(buf[maOff : maOff+AuthenticatorLen])

	mac := hmac.New(md5.New, secret) //nolint:gosec // RFC 5176 Section 3.4 mandates HMAC-MD5.
	mac.Write(buf)
	return hmac.Equal(received[:], mac.Sum(nil))
}

// verifyResponseMessageAuthenticator checks the Message-Authenticator
// attribute (type 80) of an Access-Accept, Access-Reject or Access-Challenge
// against the Request Authenticator of the Access-Request it answers. It
// reports whether a RADIUS client may accept the datagram.
//
// A datagram carrying no Message-Authenticator passes. RFC 2869 Section 5.14
// conditions the obligation on the attribute being present, and RFC 2869
// Section 5.19 Note 1 makes it mandatory only for a packet that also carries
// an EAP-Message attribute. A datagram whose attribute list cannot be walked
// fails.
//
// RFC 2869 Section 5.14: "A RADIUS Client receiving an Access-Accept,
// Access-Reject or Access-Challenge with a Message-Authenticator Attribute
// present MUST calculate the correct value of the Message-Authenticator and
// silently discard the packet if it does not match the value sent."
//
// RFC 2869 Section 5.14: "For Access-Challenge, Access-Accept, and
// Access-Reject packets, the Message-Authenticator is calculated as follows,
// using the Request-Authenticator from the Access-Request this packet is in
// reply to: Message-Authenticator = HMAC-MD5 (Type, Identifier, Length,
// Request Authenticator, Attributes)". The same section adds "When the
// checksum is calculated the signature string should be considered to be
// sixteen octets of zero."
//
// The octet stream the HMAC covers, with byte offsets:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Code      |  Identifier   |            Length             |  0..3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|         Request Authenticator (16 octets, substituted)        |  4..19
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|    Attributes, Message-Authenticator value zeroed ...         |  20..Length-1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// The wire datagram carries the Response Authenticator at offsets 4 to 19,
// because RFC 2869 Section 5.14 computes the Message-Authenticator first and
// the Response Authenticator over the finished packet. The Request
// Authenticator is substituted there before the HMAC runs.
func verifyResponseMessageAuthenticator(data []byte, requestAuth [AuthenticatorLen]byte, secret []byte) bool {
	if len(data) < MinPacketLen {
		return false
	}
	pktLen := int(binary.BigEndian.Uint16(data[2:4]))
	if pktLen < MinPacketLen || pktLen > len(data) {
		return false
	}

	buf := make([]byte, pktLen)
	copy(buf, data[:pktLen])
	maOff, present, ok := messageAuthenticatorValueOffset(buf)
	if !ok {
		return false
	}
	if !present {
		return true
	}

	var received [AuthenticatorLen]byte
	copy(received[:], buf[maOff:maOff+AuthenticatorLen])
	copy(buf[4:4+AuthenticatorLen], requestAuth[:])
	clear(buf[maOff : maOff+AuthenticatorLen])
	mac := hmac.New(md5.New, secret) //nolint:gosec // RFC 2869 Section 5.14 mandates HMAC-MD5.
	mac.Write(buf)
	return hmac.Equal(received[:], mac.Sum(nil))
}

func messageAuthenticatorValueOffset(data []byte) (offset int, present, ok bool) {
	off := HeaderLen
	for off < len(data) {
		if off+2 > len(data) {
			return 0, false, false
		}
		attrType := data[off]
		attrLen := int(data[off+1])
		if attrLen < 2 || off+attrLen > len(data) {
			return 0, false, false
		}
		if attrType == AttrMessageAuthenticator {
			if attrLen != 2+AuthenticatorLen {
				return 0, false, false
			}
			return off + 2, true, true
		}
		off += attrLen
	}
	return 0, false, true
}

// AccountingRequestAuth computes the authenticator for an Accounting-Request.
// RFC 2866 Section 3: MD5(Code+ID+Length+16zero+Attributes+Secret).
// RFC 5176 Section 2.3: same formula for CoA-Request and Disconnect-Request.
func AccountingRequestAuth(buf []byte, length int, secret []byte) [AuthenticatorLen]byte {
	h := md5.New() //nolint:gosec // RFC 2866 mandates MD5
	h.Write(buf[:4])
	var zeros [AuthenticatorLen]byte
	h.Write(zeros[:])
	h.Write(buf[HeaderLen:length])
	h.Write(secret)
	var auth [AuthenticatorLen]byte
	copy(auth[:], h.Sum(nil))
	return auth
}
