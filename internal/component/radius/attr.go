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

// EncodeMSCHAP2Response encodes an MS-CHAP2-Response as a vendor-specific
// attribute (RFC 2548). The outer VSA wraps vendor 311, type 25.
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

	return EncodeVSA(VendorMicrosoft, MSCHAP2Response, msValue)
}

// EncodeMSCHAPChallenge encodes an MS-CHAP-Challenge as a vendor-specific
// attribute. Vendor 311, type 11.
func EncodeMSCHAPChallenge(challenge []byte) ([]byte, error) {
	return EncodeVSA(VendorMicrosoft, MSCHAPChallenge, challenge)
}

// EncodeVSA encodes a vendor-specific attribute.
// RFC 2865 Section 5.26: Type(26) + Length + VendorID(4) + VendorType(1) + VendorLength(1) + Value.
func EncodeVSA(vendorID uint32, vendorType uint8, value []byte) ([]byte, error) {
	// Outer: Type(1) + Length(1) + VendorID(4) + VendorType(1) + VendorLength(1) + Value
	if 2+len(value) > 255 {
		return nil, fmt.Errorf("radius: VSA value too long (%d > 253)", len(value))
	}
	vendorLen := uint8(2 + len(value)) // VendorType + VendorLength + Value
	totalLen := 6 + int(vendorLen)     // Type + Length + VendorID(4) + vendor portion

	buf := make([]byte, totalLen)
	buf[0] = AttrVendorSpecific
	buf[1] = uint8(totalLen)
	binary.BigEndian.PutUint32(buf[2:6], vendorID)
	buf[6] = vendorType
	buf[7] = vendorLen
	copy(buf[8:], value)

	return buf, nil
}

// DecodeVSA decodes a vendor-specific attribute value (everything after
// the outer Type+Length). Returns vendorID, vendorType, vendorValue.
func DecodeVSA(data []byte) (vendorID uint32, vendorType uint8, value []byte, err error) {
	if len(data) < 6 {
		return 0, 0, nil, fmt.Errorf("radius: VSA too short (%d)", len(data))
	}
	vendorID = binary.BigEndian.Uint32(data[0:4])
	vendorType = data[4]
	vendorLen := int(data[5])
	if vendorLen < 2 || 4+vendorLen > len(data) {
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

// DecodeUint32 decodes a 4-byte attribute value as uint32.
func DecodeUint32(data []byte) (uint32, error) {
	if len(data) != 4 {
		return 0, fmt.Errorf("radius: expected 4 bytes for uint32, got %d", len(data))
	}
	return binary.BigEndian.Uint32(data), nil
}
