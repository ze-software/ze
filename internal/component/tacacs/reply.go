// Design: docs/architecture/aaa-tacacs.md
// Related: client.go -- validates replies before retaining a connection.
// Related: authen.go, author.go, acct.go -- decode validated bodies.
// RFC 8907 Sections 4.5, 5.2, 6.2 and 7.2 -- see rfc/short/rfc8907.md.
package tacacs

import (
	"encoding/binary"
	"fmt"
)

// validateReplyBody checks lengths before any field is interpreted. Body layouts
// (byte offsets) are:
//
// Authentication: [0 status][1 flags][2:4 msg len][4:6 data len][6: msg, data]
// Authorization:  [0 status][1 argc ][2:4 msg len][4:6 data len][6: arg lengths]
//
//	[6+argc: msg, data, args]
//
// Accounting:     [0:2 msg len][2:4 data len][4 status][5: msg, data]
//
// The header's uint32 length bounds body. No wire-controlled allocation is made.
func validateReplyBody(kind uint8, body []byte) (uint8, error) {
	fixed := 6
	if kind == typeAccounting {
		fixed = 5
	}
	if len(body) < fixed {
		return 0, ErrBadSecret
	}
	status := body[0]
	messageLen, dataLen := 0, 0
	if kind == typeAccounting {
		status = body[4]
		messageLen = int(binary.BigEndian.Uint16(body[0:2]))
		dataLen = int(binary.BigEndian.Uint16(body[2:4]))
	}
	argCount := 0
	if kind == typeAuthorization {
		argCount = int(body[1])
	}
	if kind != typeAccounting {
		messageLen = int(binary.BigEndian.Uint16(body[2:4]))
		dataLen = int(binary.BigEndian.Uint16(body[4:6]))
	}
	if len(body) < fixed+argCount {
		return 0, ErrBadSecret
	}
	total := fixed + argCount + messageLen + dataLen
	for _, length := range body[fixed : fixed+argCount] {
		total += int(length)
	}
	// RFC 8907 Section 4.5: "If the sum is not identical to the cleartext
	// datalength value from the header, the packet MUST be discarded and an
	// ERROR signaled."
	if total != len(body) {
		return 0, ErrBadSecret
	}
	messageOff := fixed + argCount
	if err := validateText(string(body[messageOff : messageOff+messageLen])); err != nil {
		return 0, fmt.Errorf("reply server message: %w", err)
	}
	// Authentication data is expressly binary (Section 5.2). Authorization
	// and accounting data are display strings (Sections 6.2 and 7.2).
	if kind != typeAuthentication {
		if err := validateText(string(body[messageOff+messageLen:])); err != nil {
			return 0, fmt.Errorf("reply data or arguments: %w", err)
		}
	}
	return status, nil
}
