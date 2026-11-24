// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS attribute encoding
// Related: dict.go -- attribute type constants

package radius

import (
	"crypto/md5" //nolint:gosec // RFC 2865 Section 5.2 requires MD5 for User-Password encoding
	"encoding/binary"
	"fmt"
)

// EncodeUserPassword encodes a PAP password per RFC 2865 Section 5.2.
// The encoding XORs the password with MD5(secret+authenticator) in
// 16-byte blocks, chaining the previous ciphertext block into the next
// MD5 input.
func EncodeUserPassword(password, secret []byte, authenticator [AuthenticatorLen]byte) []byte {
	// RFC 2865 Section 5.2: pad password to multiple of 16, max 128 bytes.
	padLen := len(password)
	if padLen == 0 {
		padLen = 16
	} else if padLen%16 != 0 {
		padLen += 16 - padLen%16
	}
	if padLen > 128 {
		padLen = 128
	}

	padded := make([]byte, padLen)
	copy(padded, password)

	result := make([]byte, padLen)

	// First block: c[0] = p[0] XOR MD5(S + RA)
	h := md5.New() //nolint:gosec // RFC 2865 mandates MD5
	h.Write(secret)
	h.Write(authenticator[:])
	block := h.Sum(nil)

	for i := range 16 {
		if i < padLen {
			result[i] = padded[i] ^ block[i]
		}
	}

	// Subsequent blocks: c[i] = p[i] XOR MD5(S + c[i-1])
	for blockStart := 16; blockStart < padLen; blockStart += 16 {
		h.Reset()
		h.Write(secret)
		h.Write(result[blockStart-16 : blockStart])
		block = h.Sum(nil)
		for i := range 16 {
			if blockStart+i < padLen {
				result[blockStart+i] = padded[blockStart+i] ^ block[i]
			}
		}
	}

	return result
}

// EncodeCHAPPassword encodes a CHAP-Password attribute value.
// RFC 2865 Section 5.3: CHAP-Ident (1 byte) + CHAP-Response (16 bytes).
func EncodeCHAPPassword(identifier uint8, response []byte) []byte {
	val := make([]byte, 1+len(response))
	val[0] = identifier
	copy(val[1:], response)
	return val
}

// maxVSAValueLen is the largest Attribute-Specific payload a Vendor-Specific
// attribute value can carry while keeping the enclosing RADIUS attribute within
// MaxAttrLen (255 octets): 255 - Type(1) - Length(1) - VendorID(4) -
// VendorType(1) - VendorLength(1) = 247.
const maxVSAValueLen = MaxAttrLen - 8

// EncodeMSCHAP2Response encodes an MS-CHAP2-Response as a vendor-specific
// attribute value (RFC 2548 Section 2.3.2, RFC 2865 Section 5.26), ready for
// Attr{Type: AttrVendorSpecific, Value: ...}. Vendor 311, type 25.
// Value format: Ident(1) + Flags(1) + Peer-Challenge(16) + Reserved(8) + Response(24).
func EncodeMSCHAP2Response(identifier uint8, peerChallenge, ntResponse []byte) ([]byte, error) {
	if len(peerChallenge) != 16 {
		return nil, fmt.Errorf("radius: MS-CHAP2 peer challenge must be 16 bytes, got %d", len(peerChallenge))
	}
	if len(ntResponse) != 24 {
		return nil, fmt.Errorf("radius: MS-CHAP2 NT response must be 24 bytes, got %d", len(ntResponse))
	}

	// Ident(1) + Flags(1) + PeerChallenge(16) + Reserved(8) + Response(24) = 50
	const msValueLen = 50
	msValue := make([]byte, msValueLen)
	msValue[0] = identifier
	msValue[1] = 0 // Flags
	copy(msValue[2:18], peerChallenge)
	// msValue[18:26] = Reserved (zero)
	copy(msValue[26:50], ntResponse)

	// RFC 2865 Section 5.26.
	return EncodeVSA(VendorMicrosoft, MSCHAP2Response, msValue)
}

// EncodeMSCHAPChallenge encodes an MS-CHAP-Challenge as a vendor-specific
// attribute value (RFC 2548 Section 2.1.2, RFC 2865 Section 5.26), ready for
// Attr{Type: AttrVendorSpecific, Value: ...}. Vendor 311, type 11.
func EncodeMSCHAPChallenge(challenge []byte) ([]byte, error) {
	// RFC 2865 Section 5.26.
	return EncodeVSA(VendorMicrosoft, MSCHAPChallenge, challenge)
}

// EncodeVSA encodes the value of a Vendor-Specific attribute (everything after
// the outer Type(26) and Length octets), ready to place in
// Attr{Type: AttrVendorSpecific, Value: ...}. It is the inverse of DecodeVSA on
// the same octets.
//
// RFC 2865 Section 5.26: "It SHOULD be encoded as a sequence of vendor type /
// vendor length / value fields, as follows."
// RFC 2865 Section 5.26: "The high-order octet is 0 and the low-order 3 octets
// are the SMI Network Management Private Enterprise Code of the Vendor in
// network byte order, as defined in the "Assigned Numbers" RFC [6]."
//
// Wire format of the returned attribute value (RFC 2865 Section 5.26), with
// byte offsets:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                          Vendor-Id                            |  0..3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Vendor type  | Vendor length |    Attribute-Specific ...     |  4..5, 6..
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Packet.EncodeTo writes the outer Type (26) and Length octets.
func EncodeVSA(vendorID uint32, vendorType uint8, value []byte) ([]byte, error) {
	if len(value) > maxVSAValueLen {
		return nil, fmt.Errorf("radius: VSA value too long (%d > %d)", len(value), maxVSAValueLen)
	}
	vendorLen := uint8(2 + len(value)) // VendorType(1) + VendorLength(1) + Value
	valLen := 4 + int(vendorLen)       // VendorID(4) + vendor portion

	buf := make([]byte, valLen)
	binary.BigEndian.PutUint32(buf[0:4], vendorID)
	buf[4] = vendorType
	buf[5] = vendorLen
	copy(buf[6:], value)

	return buf, nil
}

// DecodeVSA decodes the value of a Vendor-Specific attribute (everything after
// the outer Type(26) and Length octets), the exact octets EncodeVSA produces
// and Packet.FindAllAttr(AttrVendorSpecific) returns.
//
// RFC 2865 Section 5.26: "It SHOULD be encoded as a sequence of vendor type /
// vendor length / value fields, as follows."
//
// Wire format of data (RFC 2865 Section 5.26), with byte offsets:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                          Vendor-Id                            |  0..3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Vendor type  | Vendor length |    Attribute-Specific ...     |  4..5, 6..
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Returns vendorID, vendorType, and a subslice of data holding the
// Attribute-Specific octets.
func DecodeVSA(data []byte) (vendorID uint32, vendorType uint8, value []byte, err error) {
	if len(data) < 6 {
		return 0, 0, nil, fmt.Errorf("radius: VSA too short (%d)", len(data))
	}
	vendorID = binary.BigEndian.Uint32(data[0:4])
	vendorType = data[4]
	vendorLen := int(data[5])
	if vendorLen < 2 {
		return 0, 0, nil, fmt.Errorf("radius: VSA vendor length invalid (%d)", vendorLen)
	}
	if 4+vendorLen > len(data) {
		return 0, 0, nil, fmt.Errorf("radius: VSA vendor length invalid (%d)", vendorLen)
	}
	value = data[6 : 4+vendorLen]
	return vendorID, vendorType, value, nil
}

// AttrUint32 encodes a uint32 value as a 4-byte attribute value.
func AttrUint32(v uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, v)
	return buf
}

// AttrString encodes a string as an attribute value.
func AttrString(s string) []byte {
	return []byte(s)
}

// AppendTextAttr appends a text attribute carrying value, and appends nothing
// when value is empty.
//
// RFC 2865 Section 5: "Text of length zero (0) MUST NOT be sent; omit the
// entire attribute instead."
//
// Every Access-Request builder routes a text attribute through here, so the
// rule is stated once for User-Name, NAS-Identifier and every other text type.
func AppendTextAttr(attrs []Attr, attrType uint8, value string) []Attr {
	if value == "" {
		return attrs
	}
	return append(attrs, Attr{Type: attrType, Value: AttrString(value)})
}

// AcceptedServiceType reports whether an Access-Accept authorizes a service
// this NAS provides. offered names the one Service-Type the caller implements,
// which is the value its Access-Request asked for.
//
// RFC 2865 Section 5.6: "A NAS is not required to implement all of these
// service types, and MUST treat unknown or unsupported Service-Types as though
// an Access-Reject had been received instead."
// RFC 2865 Section 1.1: "A NAS MUST treat a RADIUS access-accept authorizing an
// unavailable service as an access-reject instead."
//
// An Access-Accept carrying no Service-Type authorizes the service the
// Access-Request named, so it is accepted. A Service-Type that is not four
// octets names no service the NAS can identify, so it is unsupported and the
// Access-Accept is refused.
func AcceptedServiceType(resp *Packet, offered uint32) bool {
	value := resp.FindAttr(AttrServiceType)
	if value == nil {
		return true
	}
	serviceType, err := decodeUint32(value)
	if err != nil {
		return false
	}
	return serviceType == offered
}

// AcceptedFramedProtocol reports whether an Access-Accept authorizes framed
// access over a framing this NAS provides. offered names the one
// Framed-Protocol the caller carries.
//
// RFC 2865 Section 5.7: "This Attribute indicates the framing to be used for
// framed access." An Access-Accept naming another framing authorizes a service
// the NAS cannot bring up, so the caller treats it as a reject.
//
// An Access-Accept carrying no Framed-Protocol leaves the framing to the NAS,
// so it is accepted. A Framed-Protocol that is not four octets names no framing
// the NAS can identify, so the Access-Accept is refused.
func AcceptedFramedProtocol(resp *Packet, offered uint32) bool {
	value := resp.FindAttr(AttrFramedProtocol)
	if value == nil {
		return true
	}
	framing, err := decodeUint32(value)
	if err != nil {
		return false
	}
	return framing == offered
}

// decodeUint32 decodes a 4-byte attribute value as uint32.
func decodeUint32(data []byte) (uint32, error) {
	if len(data) != 4 {
		return 0, fmt.Errorf("radius: expected 4 bytes for uint32, got %d", len(data))
	}
	return binary.BigEndian.Uint32(data), nil
}
