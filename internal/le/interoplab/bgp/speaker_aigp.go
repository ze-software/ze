// Design: docs/architecture/testing/interop.md -- AIGP received-wire proof.
package bgp

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const aigpWireCapture = "/tmp/aigp-wire.jsonl" //nolint:gosec // Fixed fixture capture filename, not a credential.

// runAIGPWireRecipient records one real recipient session. It never reconnects
// or originates subject routes; the checker reads complete received frames.
func runAIGPWireRecipient(options speakerOptions, _ io.Writer) (resultErr error) {
	if options.duration <= 0 || options.duration > 300*time.Second {
		return errors.New("AIGP wire recipient requires a lifetime in (0,300s]")
	}
	ctx, cancel := context.WithTimeout(context.Background(), options.duration)
	defer cancel()
	connection, err := dialExtendedRelay(ctx, options.connect)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, connection.Close()) }()
	deadline, _ := ctx.Deadline()
	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}
	capture, err := os.OpenFile(aigpWireCapture, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, capture.Close()) }()
	encoder := json.NewEncoder(capture)
	open, err := speakerOpen(uint32(options.asn), uint16(options.holdTime), net.ParseIP(options.routerID), nil, false, false)
	if err != nil {
		return err
	}
	if _, err := connection.Write(open); err != nil {
		return err
	}
	opened, established := false, false
	nextKeepalive := time.Now().Add(10 * time.Second)
	var raw textbuf.Buffer
	for frames := 0; frames < 256 && time.Now().Before(deadline); {
		if established && !time.Now().Before(nextKeepalive) {
			if _, err := connection.Write(speakerKeepalive()); err != nil {
				return err
			}
			nextKeepalive = time.Now().Add(10 * time.Second)
		}
		header, idle, err := readSpeakerExact(connection, bgpHeaderLength, deadline, true)
		if err != nil {
			return fmt.Errorf("AIGP wire recipient lost its original session: %w", err)
		}
		if idle {
			continue
		}
		length := int(binary.BigEndian.Uint16(header[16:18]))
		if length < bgpHeaderLength || length > 4096 {
			return errors.New("AIGP recipient received an invalid message length")
		}
		body, _, err := readSpeakerExact(connection, length-bgpHeaderLength, deadline, false)
		if err != nil {
			return err
		}
		kind := header[18]
		frames++
		raw.Reset().HexUpper(header).HexUpper(body)
		if err := encoder.Encode(extendedRelayFrame{Original: raw.String()}); err != nil {
			return err
		}
		switch kind {
		case bgpOpen:
			if opened || len(body) < 10 || body[0] != 4 {
				return errors.New("invalid or duplicate OPEN at AIGP wire recipient")
			}
			opened = true
			if _, err := connection.Write(speakerKeepalive()); err != nil {
				return err
			}
		case bgpKeepalive:
			if !opened {
				return errors.New("KEEPALIVE before OPEN at AIGP wire recipient")
			}
			if !established {
				established = true
				if _, err := connection.Write(speakerEOR()); err != nil {
					return err
				}
			}
		case bgpUpdate:
			if !established {
				return errors.New("UPDATE before establishment at AIGP wire recipient")
			}
		case bgpNotification:
			return fmt.Errorf("AIGP wire recipient received NOTIFICATION %x", body)
		default:
			return fmt.Errorf("AIGP wire recipient received message type %d", kind)
		}
	}
	return errors.New("AIGP wire recipient exhausted its frame or lifetime bound")
}
