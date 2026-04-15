// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption

// RFC 8907 Section 6 -- TACACS+ Authorization messages.
package tacacs

import (
	"encoding/binary"
	"fmt"
)

// Authorization constants. RFC 8907 Section 6.
const (
	// AuthenMethod for authorization. RFC 8907 Section 6.1.
	AuthenMethodTACACS = 0x06

	// AuthorResponse status codes. RFC 8907 Section 6.2.
	AuthorStatusPassAdd  = 0x01
	AuthorStatusPassRepl = 0x02
	AuthorStatusFail     = 0x10
	AuthorStatusError    = 0x11
)

// AuthorRequest is an authorization REQUEST packet body.
// RFC 8907 Section 6.1.
type AuthorRequest struct {
	AuthenMethod  uint8
	PrivLvl       uint8
	AuthenType    uint8
	AuthenService uint8
	User          string
	Port          string
	RemAddr       string
	Args          []string // "key=value" or "key*value" pairs
}

// MarshalBinary encodes an AuthorRequest body.
// Returns error if any variable field exceeds 255 bytes (uint8 length limit).
func (a *AuthorRequest) MarshalBinary() ([]byte, error) {
	userLen := len(a.User)
	portLen := len(a.Port)
	remLen := len(a.RemAddr)
	argCount := len(a.Args)

	if userLen > 255 || portLen > 255 || remLen > 255 || argCount > 255 {
		return nil, fmt.Errorf("field exceeds 255: user=%d port=%d rem=%d args=%d",
			userLen, portLen, remLen, argCount)
	}
	for i, arg := range a.Args {
		if len(arg) > 255 {
			return nil, fmt.Errorf("arg[%d] exceeds 255 bytes: %d", i, len(arg))
		}
	}

	// Calculate total size: 8 fixed + argCount arg lengths + variable fields.
	varLen := userLen + portLen + remLen
	for _, arg := range a.Args {
		varLen += len(arg)
	}
	body := make([]byte, 8+argCount+varLen)

	body[0] = a.AuthenMethod
	body[1] = a.PrivLvl
	body[2] = a.AuthenType
	body[3] = a.AuthenService
	body[4] = uint8(userLen)
	body[5] = uint8(portLen)
	body[6] = uint8(remLen)
	body[7] = uint8(argCount)

	// Arg lengths.
	off := 8
	for _, arg := range a.Args {
		body[off] = uint8(len(arg))
		off++
	}

	// Variable fields.
	off += copy(body[off:], a.User)
	off += copy(body[off:], a.Port)
	off += copy(body[off:], a.RemAddr)
	for _, arg := range a.Args {
		off += copy(body[off:], arg)
	}

	return body, nil
}

// AuthorResponse is an authorization RESPONSE packet body.
// RFC 8907 Section 6.2.
type AuthorResponse struct {
	Status    uint8
	ServerMsg string
	Data      []byte
	Args      []string
}

// UnmarshalAuthorResponse decodes an authorization RESPONSE body.
func UnmarshalAuthorResponse(data []byte) (*AuthorResponse, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("author response too short: %d bytes", len(data))
	}

	status := data[0]
	argCount := int(data[1])
	serverMsgLen := int(binary.BigEndian.Uint16(data[2:4]))
	dataLen := int(binary.BigEndian.Uint16(data[4:6]))

	minLen := 6 + argCount + serverMsgLen + dataLen
	if len(data) < minLen {
		return nil, fmt.Errorf("author response truncated: need at least %d, have %d", minLen, len(data))
	}

	// Read arg lengths.
	off := 6
	argLens := make([]int, argCount)
	for i := range argCount {
		argLens[i] = int(data[off])
		off++
	}

	// Adjust minimum length with actual arg sizes.
	totalArgLen := 0
	for _, al := range argLens {
		totalArgLen += al
	}
	if len(data) < off+serverMsgLen+dataLen+totalArgLen {
		return nil, fmt.Errorf("author response truncated with args")
	}

	serverMsg := string(data[off : off+serverMsgLen])
	off += serverMsgLen

	respData := make([]byte, dataLen)
	copy(respData, data[off:off+dataLen])
	off += dataLen

	args := make([]string, argCount)
	for i, al := range argLens {
		args[i] = string(data[off : off+al])
		off += al
	}

	return &AuthorResponse{
		Status:    status,
		ServerMsg: serverMsg,
		Data:      respData,
		Args:      args,
	}, nil
}
