// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption
// Related: authen.go -- authentication messages (sibling protocol service)
// Related: acct.go -- accounting messages (sibling protocol service)

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

// MarshalBinaryInto encodes an AuthorRequest body into dst and returns
// the number of bytes written. Used by the client to write directly
// into a pooled wire buffer; dst MUST have capacity for the full body.
//
// Returns error if any variable field exceeds 255 bytes (uint8 length
// limit) or if dst is too small.
func (a *AuthorRequest) MarshalBinaryInto(dst []byte) (int, error) {
	userLen := len(a.User)
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

	off += copy(dst[off:], a.User)
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
	varLen := len(a.User) + len(a.Port) + len(a.RemAddr)
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
// Memory lifetime: the Data field aliases the slice passed to
// UnmarshalAuthorResponse; see AuthenReply godoc for the full rule.
// ServerMsg and Args are Go strings and carry their own backing memory,
// so they remain safe once the input buffer is released.
type AuthorResponse struct {
	Status    uint8
	ServerMsg string   // safe post-Put (Go string copy)
	Data      []byte   // aliases input buffer; see struct doc
	Args      []string // each string copies its bytes, safe post-Put
}

// UnmarshalAuthorResponse decodes an authorization RESPONSE body. The
// returned AuthorResponse.Data aliases the input slice (no allocation).
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

	// Read arg lengths. argCount is bounded by uint8 (0-255) so the
	// int-slice alloc is at most 2 KB; this is a []int (not []byte) and
	// hence not within the "No make where pools exist" rule's scope.
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

	// Alias respData into the input slice.
	respData := data[off : off+dataLen]
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
