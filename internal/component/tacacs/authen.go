// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption
// Related: author.go -- authorization messages (sibling protocol service)
// Related: acct.go -- accounting messages (sibling protocol service)

// RFC 8907 Section 5 -- TACACS+ Authentication messages.
package tacacs

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Authentication constants. RFC 8907 Section 5.
const (
	// Version bytes.
	verMajor    = 0xC0 // default (minor 0)
	verMinorOne = 0xC1 // PAP/CHAP/MSCHAP

	// Actions. RFC 8907 Section 5.1.
	authenActionLogin = 0x01

	// Authentication types. RFC 8907 Section 5.1.
	authenTypePAP = 0x02

	// Authentication services. RFC 8907 Section 5.1.
	authenServiceLogin = 0x01

	// Reply status codes. RFC 8907 Section 5.2.
	AuthenStatusPass    = 0x01
	AuthenStatusFail    = 0x02
	AuthenStatusGetPass = 0x05
	AuthenStatusRestart = 0x06
	AuthenStatusError   = 0x07
	AuthenStatusFollow  = 0x21
)

// AuthenStart is an authentication START packet body.
// RFC 8907 Section 5.1.
type AuthenStart struct {
	Action        uint8
	PrivLvl       uint8
	AuthenType    uint8
	AuthenService uint8
	User          string
	Port          string
	RemAddr       string
	Data          []byte // PAP password or CHAP response
}

// MarshalBinaryInto encodes an AuthenStart body into dst and returns
// the number of bytes written. Used by the client to write directly
// into a pooled wire buffer; dst MUST have capacity for the full body.
//
// Returns error if any variable field exceeds 255 bytes (uint8 length
// limit) or if dst is too small.
func (a *AuthenStart) MarshalBinaryInto(dst []byte) (int, error) {
	username, err := prepareWireUsername(a.User, false)
	if err != nil {
		return 0, err
	}
	if err := validateText(a.Port); err != nil {
		return 0, fmt.Errorf("port: %w", err)
	}
	if err := validateText(a.RemAddr); err != nil {
		return 0, fmt.Errorf("remote address: %w", err)
	}
	if a.AuthenType == authenTypePAP {
		// RFC 8907 Section 5.4.2.2: "The START packet MUST contain a
		// username and the data field MUST contain the PAP ASCII password."
		if username == "" {
			return 0, fmt.Errorf("PAP requires a username")
		}
		if err := validateText(string(a.Data)); err != nil {
			return 0, fmt.Errorf("PAP password: %w", err)
		}
	}
	userLen := len(username)
	portLen := len(a.Port)
	remLen := len(a.RemAddr)
	dataLen := len(a.Data)

	if userLen > 255 || portLen > 255 || remLen > 255 || dataLen > 255 {
		return 0, fmt.Errorf("field exceeds 255 bytes: user=%d port=%d rem=%d data=%d",
			userLen, portLen, remLen, dataLen)
	}
	need := 8 + userLen + portLen + remLen + dataLen
	if len(dst) < need {
		return 0, fmt.Errorf("authen start buffer too small: need %d, have %d", need, len(dst))
	}

	dst[0] = a.Action
	dst[1] = a.PrivLvl
	dst[2] = a.AuthenType
	dst[3] = a.AuthenService
	dst[4] = uint8(userLen)
	dst[5] = uint8(portLen)
	dst[6] = uint8(remLen)
	dst[7] = uint8(dataLen)

	off := 8
	off += copy(dst[off:], username)
	off += copy(dst[off:], a.Port)
	off += copy(dst[off:], a.RemAddr)
	off += copy(dst[off:], a.Data)

	return off, nil
}

// MarshalBinary encodes an AuthenStart body to a freshly-allocated
// slice. Retained for round-trip unit tests; production code paths use
// MarshalBinaryInto with a pooled buffer.
func (a *AuthenStart) MarshalBinary() ([]byte, error) {
	body := make([]byte, 8+255+len(a.Port)+len(a.RemAddr)+len(a.Data))
	n, err := a.MarshalBinaryInto(body)
	if err != nil {
		return nil, err
	}
	return body[:n], nil
}

// Version returns the appropriate version byte for this authen type.
// RFC 8907: PAP/CHAP/MSCHAP use minor version 1.
func (a *AuthenStart) Version() uint8 {
	if a.Action == authenActionLogin && a.AuthenType == authenTypePAP {
		return verMinorOne
	}
	return verMajor
}

// AuthenReply is an authentication REPLY packet body.
// RFC 8907 Section 5.2.
//
// Data is owned by the reply and remains valid after the client releases its
// pooled wire buffer. Authentication data carries no authorization privilege.
type AuthenReply struct {
	Status    uint8
	Flags     uint8
	ServerMsg string // Go string -- copies its backing bytes, safe post-Put
	Data      []byte // owns its backing memory
}

// UnmarshalAuthenReply decodes a REPLY body.
//
// The returned reply owns every variable-length field.
func UnmarshalAuthenReply(data []byte) (*AuthenReply, error) {
	if _, err := validateReplyBody(typeAuthentication, data); err != nil {
		return nil, err
	}

	serverMsgLen := binary.BigEndian.Uint16(data[2:4])
	dataLen := binary.BigEndian.Uint16(data[4:6])

	off := 6
	serverMsg := string(data[off : off+int(serverMsgLen)])
	off += int(serverMsgLen)

	replyData := bytes.Clone(data[off : off+int(dataLen)])

	return &AuthenReply{
		Status:    data[0],
		Flags:     data[1],
		ServerMsg: serverMsg,
		Data:      replyData,
	}, nil
}

// NewPAPAuthenStart creates a PAP LOGIN AuthenStart for SSH password auth.
// RFC 8907 Section 5.4.2: PAP embeds the password in the data field.
func NewPAPAuthenStart(username, password, port, remAddr string) *AuthenStart {
	return &AuthenStart{
		Action:        authenActionLogin,
		PrivLvl:       1, // default user privilege level
		AuthenType:    authenTypePAP,
		AuthenService: authenServiceLogin,
		User:          username,
		Port:          port,
		RemAddr:       remAddr,
		Data:          []byte(password),
	}
}
