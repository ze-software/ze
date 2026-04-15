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

// MarshalBinary encodes an AcctRequest body.
// Returns error if any variable field exceeds 255 bytes (uint8 length limit).
func (a *AcctRequest) MarshalBinary() ([]byte, error) {
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

	// Calculate total size: 9 fixed + argCount arg lengths + variable fields.
	varLen := userLen + portLen + remLen
	for _, arg := range a.Args {
		varLen += len(arg)
	}
	body := make([]byte, 9+argCount+varLen)

	body[0] = a.Flags
	body[1] = a.AuthenMethod
	body[2] = a.PrivLvl
	body[3] = a.AuthenType
	body[4] = a.AuthenService
	body[5] = uint8(userLen)
	body[6] = uint8(portLen)
	body[7] = uint8(remLen)
	body[8] = uint8(argCount)

	// Arg lengths.
	off := 9
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

// AcctReply is an accounting REPLY packet body.
// RFC 8907 Section 7.2.
type AcctReply struct {
	ServerMsg string
	Data      []byte
	Status    uint8
}

// UnmarshalAcctReply decodes an accounting REPLY body.
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

	replyData := make([]byte, dataLen)
	copy(replyData, data[off:off+dataLen])

	return &AcctReply{
		ServerMsg: serverMsg,
		Data:      replyData,
		Status:    status,
	}, nil
}
