// Design: docs/architecture/testing/interop.md -- strict complete BGP capture.
package bgp

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// validateSpeakerHeader is shared by the socket observer and the transparent
// relay. Callers MUST read exactly the returned frame length before using it.
func validateSpeakerHeader(header []byte) (int, error) {
	if len(header) != bgpHeaderLength {
		return 0, errors.New("incomplete BGP header")
	}
	for _, octet := range header[:16] {
		if octet != 0xff {
			return 0, errors.New("invalid BGP marker")
		}
	}
	length := int(binary.BigEndian.Uint16(header[16:18]))
	if length < bgpHeaderLength {
		return 0, errors.New("BGP length is shorter than its header")
	}
	return length, nil
}

// validateSpeakerMessageHeader adds message-specific shape to the shared outer
// framer. A generic relay can retain an invalid UPDATE for diagnosis; the native
// observer MUST NOT use one as successful route-discard evidence.
func validateSpeakerMessageHeader(header []byte) (int, error) {
	length, err := validateSpeakerHeader(header)
	if err != nil {
		return 0, err
	}
	// RFC 4271 Sections 4.2-4.5 and RFC 8654 Section 4: OPEN and
	// KEEPALIVE never acquire the extended-message size allowance.
	switch header[18] {
	case bgpOpen:
		if length < 29 || length > 4096 {
			return 0, fmt.Errorf("invalid OPEN length %d", length)
		}
	case bgpUpdate:
		if length < 23 {
			return 0, fmt.Errorf("invalid UPDATE length %d", length)
		}
	case bgpNotification:
		if length < 21 {
			return 0, fmt.Errorf("invalid NOTIFICATION length %d", length)
		}
	case bgpKeepalive:
		if length != bgpHeaderLength {
			return 0, fmt.Errorf("invalid KEEPALIVE length %d", length)
		}
	}
	return length, nil
}

// validateSpeakerOpen checks the actual peer's fixed fields and delegates the
// optional-parameter/capability framing to the existing capability codec.
// These native observers request ordinary OPENs, not RFC 9072 extended ones.
func validateSpeakerOpen(body []byte) error {
	if len(body) < 10 {
		return errors.New("short OPEN body")
	}
	if body[0] != 4 {
		return fmt.Errorf("BGP version %d, want 4", body[0])
	}
	if binary.BigEndian.Uint16(body[1:3]) == 0 {
		return errors.New("OPEN advertises AS zero")
	}
	hold := binary.BigEndian.Uint16(body[3:5])
	if hold != 0 && hold < 3 {
		return fmt.Errorf("invalid OPEN hold time %d", hold)
	}
	if binary.BigEndian.Uint32(body[5:9]) == 0 {
		return errors.New("OPEN advertises identifier zero")
	}
	if int(body[9])+10 != len(body) || body[9] == 255 {
		return errors.New("OPEN optional-parameter length mismatch or unsupported extended envelope")
	}
	_, err := capability.ParseFromOptionalParams(body[10:], false)
	return err
}
