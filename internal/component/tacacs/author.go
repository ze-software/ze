// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption
// Related: authen.go -- authentication messages (sibling protocol service)
// Related: acct.go -- accounting messages (sibling protocol service)

// RFC 8907 Section 6 -- TACACS+ Authorization messages.
package tacacs

import (
	"bytes"
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
	AuthorStatusFollow   = 0x21
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

// MarshalBinaryInto encodes an AuthorRequest body into dst and returns
// the number of bytes written. Used by the client to write directly
// into a pooled wire buffer; dst MUST have capacity for the full body.
//
// Returns error if any variable field exceeds 255 bytes (uint8 length
// limit) or if dst is too small.
func (a *AuthorRequest) MarshalBinaryInto(dst []byte) (int, error) {
	username, err := prepareWireUsername(a.User, true)
	if err != nil {
		return 0, err
	}
	if err := validateText(a.Port); err != nil {
		return 0, fmt.Errorf("port: %w", err)
	}
	if err := validateText(a.RemAddr); err != nil {
		return 0, fmt.Errorf("remote address: %w", err)
	}
	userLen := len(username)
	portLen := len(a.Port)
	remLen := len(a.RemAddr)
	argCount := len(a.Args)

	if userLen > 255 || portLen > 255 || remLen > 255 || argCount > 255 {
		return 0, fmt.Errorf("field exceeds 255: user=%d port=%d rem=%d args=%d",
			userLen, portLen, remLen, argCount)
	}
	for i, arg := range a.Args {
		if len(arg) > 255 {
			return 0, fmt.Errorf("arg[%d] exceeds 255 bytes: %d", i, len(arg))
		}
		if err := validateText(arg); err != nil {
			return 0, fmt.Errorf("arg[%d]: %w", i, err)
		}
	}

	varLen := userLen + portLen + remLen
	for _, arg := range a.Args {
		varLen += len(arg)
	}
	need := 8 + argCount + varLen
	if len(dst) < need {
		return 0, fmt.Errorf("author request buffer too small: need %d, have %d", need, len(dst))
	}

	dst[0] = a.AuthenMethod
	dst[1] = a.PrivLvl
	dst[2] = a.AuthenType
	dst[3] = a.AuthenService
	dst[4] = uint8(userLen)
	dst[5] = uint8(portLen)
	dst[6] = uint8(remLen)
	dst[7] = uint8(argCount)

	off := 8
	for _, arg := range a.Args {
		dst[off] = uint8(len(arg))
		off++
	}

	off += copy(dst[off:], username)
	off += copy(dst[off:], a.Port)
	off += copy(dst[off:], a.RemAddr)
	for _, arg := range a.Args {
		off += copy(dst[off:], arg)
	}

	return off, nil
}

// MarshalBinary encodes an AuthorRequest body to a freshly-allocated
// slice. Retained for round-trip unit tests; production code paths use
// MarshalBinaryInto with a pooled buffer.
func (a *AuthorRequest) MarshalBinary() ([]byte, error) {
	varLen := 255 + len(a.Port) + len(a.RemAddr)
	for _, arg := range a.Args {
		varLen += len(arg)
	}
	body := make([]byte, 8+len(a.Args)+varLen)
	n, err := a.MarshalBinaryInto(body)
	if err != nil {
		return nil, err
	}
	return body[:n], nil
}

// AuthorResponse is an authorization RESPONSE packet body.
// RFC 8907 Section 6.2.
//
// The reply owns all variable-length fields and remains valid after the pooled
// wire buffer is released.
type AuthorResponse struct {
	Status    uint8
	ServerMsg string   // safe post-Put (Go string copy)
	Data      []byte   // owns its backing memory
	Args      []string // each string copies its bytes, safe post-Put
}

// UnmarshalAuthorResponse decodes an authorization RESPONSE body.
func UnmarshalAuthorResponse(data []byte) (*AuthorResponse, error) {
	if _, err := validateReplyBody(typeAuthorization, data); err != nil {
		return nil, err
	}
	status := data[0]
	argCount := int(data[1])
	serverMsgLen := int(binary.BigEndian.Uint16(data[2:4]))
	dataLen := int(binary.BigEndian.Uint16(data[4:6]))
	off := 6 + argCount

	serverMsg := string(data[off : off+serverMsgLen])
	off += serverMsgLen

	respData := bytes.Clone(data[off : off+dataLen])
	off += dataLen

	args := make([]string, 0, argCount)
	for _, length := range data[6 : 6+argCount] {
		// RFC 8907 Section 4.1: zero-length fields "MUST be ignored,
		// and treated as if not present."
		if length == 0 {
			continue
		}
		args = append(args, string(data[off:off+int(length)]))
		off += int(length)
	}

	return &AuthorResponse{
		Status:    status,
		ServerMsg: serverMsg,
		Data:      respData,
		Args:      args,
	}, nil
}
