// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption
// Related: authen.go -- authentication messages (sibling protocol service)
// Related: author.go -- authorization messages (sibling protocol service)

// RFC 8907 Section 7 -- TACACS+ Accounting messages.
package tacacs

import (
	"bytes"
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
	// RFC 8907 Section 7.2: "The STOP flag MUST NOT be set in
	// conjunction with the WATCHDOG flag." Table 2 also excludes START+STOP.
	switch a.Flags {
	case AcctFlagStart, AcctFlagStop, AcctFlagWatchdog, AcctFlagWatchdog | AcctFlagStart:
	default:
		return 0, fmt.Errorf("invalid accounting flags: %#x", a.Flags)
	}
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

	off += copy(dst[off:], username)
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
	varLen := 255 + len(a.Port) + len(a.RemAddr)
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
// The reply owns its variable-length fields after the pooled wire buffer is
// released by SendAccounting.
type AcctReply struct {
	ServerMsg string // Go string -- copies its backing bytes, safe post-Put
	Data      []byte // owns its backing memory
	Status    uint8
}

// UnmarshalAcctReply decodes an accounting REPLY body.
func UnmarshalAcctReply(data []byte) (*AcctReply, error) {
	if _, err := validateReplyBody(typeAccounting, data); err != nil {
		return nil, err
	}

	serverMsgLen := int(binary.BigEndian.Uint16(data[0:2]))
	dataLen := int(binary.BigEndian.Uint16(data[2:4]))
	status := data[4]

	off := 5
	serverMsg := string(data[off : off+serverMsgLen])
	off += serverMsgLen

	return &AcctReply{
		ServerMsg: serverMsg,
		Data:      bytes.Clone(data[off : off+dataLen]),
		Status:    status,
	}, nil
}
