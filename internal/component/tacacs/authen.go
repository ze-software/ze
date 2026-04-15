// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption
// Related: author.go -- authorization messages (sibling protocol service)
// Related: acct.go -- accounting messages (sibling protocol service)

// RFC 8907 Section 5 -- TACACS+ Authentication messages.
package tacacs

import (
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
	AuthenStatusPass  = 0x01
	AuthenStatusFail  = 0x02
	AuthenStatusError = 0x07
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

// MarshalBinary encodes an AuthenStart body.
// Returns error if any variable field exceeds 255 bytes (uint8 length limit).
func (a *AuthenStart) MarshalBinary() ([]byte, error) {
	userLen := len(a.User)
	portLen := len(a.Port)
	remLen := len(a.RemAddr)
	dataLen := len(a.Data)

	if userLen > 255 || portLen > 255 || remLen > 255 || dataLen > 255 {
		return nil, fmt.Errorf("field exceeds 255 bytes: user=%d port=%d rem=%d data=%d",
			userLen, portLen, remLen, dataLen)
	}

	body := make([]byte, 8+userLen+portLen+remLen+dataLen)
	body[0] = a.Action
	body[1] = a.PrivLvl
	body[2] = a.AuthenType
	body[3] = a.AuthenService
	body[4] = uint8(userLen)
	body[5] = uint8(portLen)
	body[6] = uint8(remLen)
	body[7] = uint8(dataLen)

	off := 8
	off += copy(body[off:], a.User)
	off += copy(body[off:], a.Port)
	off += copy(body[off:], a.RemAddr)
	copy(body[off:], a.Data)

	return body, nil
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
type AuthenReply struct {
	Status    uint8
	Flags     uint8
	ServerMsg string
	Data      []byte
}

// UnmarshalAuthenReply decodes a REPLY body.
func UnmarshalAuthenReply(data []byte) (*AuthenReply, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("authen reply too short: %d bytes", len(data))
	}

	serverMsgLen := binary.BigEndian.Uint16(data[2:4])
	dataLen := binary.BigEndian.Uint16(data[4:6])
	totalLen := 6 + int(serverMsgLen) + int(dataLen)

	if len(data) < totalLen {
		return nil, fmt.Errorf("authen reply truncated: need %d, have %d", totalLen, len(data))
	}

	off := 6
	serverMsg := string(data[off : off+int(serverMsgLen)])
	off += int(serverMsgLen)

	replyData := make([]byte, dataLen)
	copy(replyData, data[off:off+int(dataLen)])

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
