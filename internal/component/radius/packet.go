// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS wire format
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

// VerifyCoARequestAuth checks the authenticator of a CoA-Request or
// Disconnect-Request. RFC 5176 Section 3.5: same formula as
// Accounting-Request (MD5 over Code+ID+Length+16-zero-octets+Attrs+Secret).
// Uses constant-time comparison.
func VerifyCoARequestAuth(data, secret []byte) bool {
	if len(data) < MinPacketLen {
		return false
	}
	pktLen := int(binary.BigEndian.Uint16(data[2:4]))
	if pktLen < MinPacketLen || pktLen > len(data) {
		return false
	}
	expected := AccountingRequestAuth(data, pktLen, secret)
	return subtle.ConstantTimeCompare(data[4:4+AuthenticatorLen], expected[:]) == 1
}

// VerifyMessageAuthenticator checks the RADIUS Message-Authenticator
// attribute (type 80) when present. RFC 3579 Section 3.2 computes the
// HMAC-MD5 over the packet with the Message-Authenticator value zeroed.
func VerifyMessageAuthenticator(data, secret []byte) bool {
	if len(data) < MinPacketLen {
		return false
	}
	pktLen := int(binary.BigEndian.Uint16(data[2:4]))
	if pktLen < MinPacketLen || pktLen > len(data) {
		return false
	}

	buf := make([]byte, pktLen)
	copy(buf, data[:pktLen])
	off := HeaderLen
	for off < pktLen {
		if off+2 > pktLen {
			return false
		}
		attrType := buf[off]
		attrLen := int(buf[off+1])
		if attrLen < 2 || off+attrLen > pktLen {
			return false
		}
		if attrType == AttrMessageAuthenticator {
			if attrLen != 2+AuthenticatorLen {
				return false
			}
			received := data[off+2 : off+attrLen]
			clear(buf[off+2 : off+attrLen])
			mac := hmac.New(md5.New, secret) //nolint:gosec // RFC 3579 mandates HMAC-MD5.
			mac.Write(buf)
			expected := mac.Sum(nil)
			return hmac.Equal(received, expected)
		}
		off += attrLen
	}
	return false
}

// AccountingRequestAuth computes the authenticator for an Accounting-Request.
// RFC 2866 Section 3: MD5(Code+ID+Length+16zero+Attributes+Secret).
// RFC 5176 Section 3.5: same formula for CoA-Request and Disconnect-Request.
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
