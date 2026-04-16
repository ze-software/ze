// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption
// Related: authen.go -- authentication messages (sibling protocol service)
// Related: author.go -- authorization messages (sibling protocol service)

// RFC 8907 Section 7 -- TACACS+ Accounting messages.
package tacacs

import (
	"encoding/binary"
	"fmt"
)

// Accounting constants. RFC 8907 Section 7.
const (
	// AcctRequest flag values. RFC 8907 Section 7.1.
	AcctFlagStart    = 0x02
	AcctFlagStop     = 0x04
	AcctFlagWatchdog = 0x08

	// AcctReply status codes. RFC 8907 Section 7.2.
	AcctStatusSuccess = 0x01
	AcctStatusError   = 0x02
)

// AcctRequest is an accounting REQUEST packet body.
// RFC 8907 Section 7.1.
type AcctRequest struct {
	Flags         uint8
	AuthenMethod  uint8
	PrivLvl       uint8
	AuthenType    uint8
	AuthenService uint8
	User          string
	Port          string
	RemAddr       string
	Args          []string
}

// MarshalBinaryInto encodes an AcctRequest body into dst and returns
// the number of bytes written. Used by the client to write directly
// into a pooled wire buffer; dst MUST have capacity for the full body.
//
// Returns error if any variable field exceeds 255 bytes (uint8 length
// limit) or if dst is too small.
func (a *AcctRequest) MarshalBinaryInto(dst []byte) (int, error) {
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
	need := 9 + argCount + varLen
	if len(dst) < need {
		return 0, fmt.Errorf("acct request buffer too small: need %d, have %d", need, len(dst))
	}

	dst[0] = a.Flags
	dst[1] = a.AuthenMethod
	dst[2] = a.PrivLvl
	dst[3] = a.AuthenType
	dst[4] = a.AuthenService
	dst[5] = uint8(userLen)
	dst[6] = uint8(portLen)
	dst[7] = uint8(remLen)
	dst[8] = uint8(argCount)

	off := 9
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

// MarshalBinary encodes an AcctRequest body to a freshly-allocated
// slice. Retained for round-trip unit tests; production code paths use
// MarshalBinaryInto with a pooled buffer.
func (a *AcctRequest) MarshalBinary() ([]byte, error) {
	varLen := len(a.User) + len(a.Port) + len(a.RemAddr)
	for _, arg := range a.Args {
		varLen += len(arg)
	}
	body := make([]byte, 9+len(a.Args)+varLen)
	n, err := a.MarshalBinaryInto(body)
	if err != nil {
		return nil, err
	}
	return body[:n], nil
}

// AcctReply is an accounting REPLY packet body.
// RFC 8907 Section 7.2.
//
// Memory lifetime: the Data field aliases the slice passed to
// UnmarshalAcctReply; see the AuthenReply godoc for the full rule.
// Production callers read only Status, so aliasing is safe for them; tests
// that assert on Data must hold the input buffer for the assertion window.
type AcctReply struct {
	ServerMsg string // Go string -- copies its backing bytes, safe post-Put
	Data      []byte // aliases input buffer; see struct doc
	Status    uint8
}

// UnmarshalAcctReply decodes an accounting REPLY body. The returned
// AcctReply.Data aliases the input slice (no allocation).
func UnmarshalAcctReply(data []byte) (*AcctReply, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("acct reply too short: %d bytes", len(data))
	}

	serverMsgLen := int(binary.BigEndian.Uint16(data[0:2]))
	dataLen := int(binary.BigEndian.Uint16(data[2:4]))
	status := data[4]
	totalLen := 5 + serverMsgLen + dataLen

	if len(data) < totalLen {
		return nil, fmt.Errorf("acct reply truncated: need %d, have %d", totalLen, len(data))
	}

	off := 5
	serverMsg := string(data[off : off+serverMsgLen])
	off += serverMsgLen

	return &AcctReply{
		ServerMsg: serverMsg,
		Data:      data[off : off+dataLen],
		Status:    status,
	}, nil
}
