// Design: docs/architecture/testing/interop.md -- RFC 8654 foreign-peer wire evidence.
// Related: check_extended_message.go -- compares original and delivered OPENs.
// FRR 10.3.1: https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_open.c
// FRR gates its packet size bilaterally. This test transport changes capability 6
// only, so FRR can produce and decode large UPDATEs while Ze sees either direction.
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

const extendedCaptureBase = "/tmp/extended"

// extendedRelayFrame is one complete frame forwarded to the other implementation.
// Original is always the sender's frame. Delivered exists only for an OPEN; every
// other message is forwarded byte-for-byte. No frame is recorded before its write.
type extendedRelayFrame struct {
	Original  string `json:"original"`
	Delivered string `json:"delivered,omitempty"`
}

// runExtendedRelay keeps the fixture container queryable after a fatal session
// ends. Its transport finishes first; the retained diagnostic and capture files
// remain available until the configured deadline or Docker teardown.
func runExtendedRelay(options speakerOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), options.duration)
	defer cancel()
	// RFC 8654 Sections 3 and 4: the transport rewrites OPEN capabilities only.
	resultErr := runExtendedRelaySession(ctx, options)
	diagnostic := ""
	if resultErr != nil {
		diagnostic = resultErr.Error()
	}
	if err := writeJSON(options.result+"-result.json", map[string]string{"transport-error": diagnostic}); err != nil {
		return errors.Join(resultErr, err)
	}
	<-ctx.Done()
	return resultErr
}

// runExtendedRelaySession owns exactly two socket workers for one session. It MUST
// close both connections and join both workers before returning. Reconnects are
// forbidden: a reset must not replace the session whose original OPEN was checked.
// RFC 8654 Section 3: "The BGP Extended Message Capability is a new BGP capability
// [RFC5492] defined with Capability Code 6 and Capability Length 0."
func runExtendedRelaySession(ctx context.Context, options speakerOptions) (resultErr error) {
	frr, err := dialExtendedRelay(ctx, options.relayPeer)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, frr.Close()) }()
	ze, err := dialExtendedRelay(ctx, options.connect)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, ze.Close()) }()
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.New("relay context has no deadline")
	}
	if err := frr.SetDeadline(deadline); err != nil {
		return err
	}
	if err := ze.SetDeadline(deadline); err != nil {
		return err
	}
	results := make(chan error, 2)
	// RFC 8654 Sections 3 and 4. FRR sees capability 6 regardless of Ze's
	// original OPEN. This does NOT change the OPEN Ze generated or its state.
	go forwardExtendedRelay(frr, ze, true, options.result+"-ze.jsonl", results)
	// RFC 8654 Sections 3 and 4. Ze sees the scenario's remote advertisement.
	go forwardExtendedRelay(ze, frr, options.relayExtended, options.result+"-frr.jsonl", results)
	first := <-results
	// Expiring both sockets stops the other worker without double-closing them.
	stopErr := errors.Join(frr.SetDeadline(time.Now()), ze.SetDeadline(time.Now()))
	second := <-results
	return errors.Join(first, second, stopErr)
}

// dialExtendedRelay waits only for container startup, bounded by the session
// context. The harness starts the speaker sidecars before the FRR container.
func dialExtendedRelay(ctx context.Context, address string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: time.Second}
	for ctx.Err() == nil {
		connection, err := dialer.DialContext(ctx, "tcp", address)
		if err == nil {
			return connection, nil
		}
		if err := sleepContext(ctx, 250*time.Millisecond); err != nil {
			return nil, fmt.Errorf("relay dial %s: %w", address, err)
		}
	}
	return nil, fmt.Errorf("relay dial %s: %w", address, ctx.Err())
}

// forwardExtendedRelay is one session worker. runExtendedRelaySession MUST stop its
// sockets and receive its result before returning. The 256-frame ceiling and
// socket deadline bound disk use, loops and all blocking I/O.
// RFC 8654 Section 2: "BGP Extended Messages have a maximum message size of
// 65,535 octets."
func forwardExtendedRelay(target, source net.Conn, advertise bool, path string, results chan<- error) {
	// RFC 8654 Sections 2-4: capture complete messages and change only OPEN code6.
	results <- captureExtendedRelay(target, source, advertise, path)
}

// captureExtendedRelay retains complete original frames after their writes.
// RFC 8654 Section 4: "The BGP Extended Message Capability applies to all messages
// except for OPEN and KEEPALIVE messages."
func captureExtendedRelay(target io.Writer, source io.Reader, advertise bool, path string) (resultErr error) {
	capture, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, capture.Close()) }()
	encoder := json.NewEncoder(capture)
	var frame [65535]byte
	var rewritten [4096]byte
	for range 256 {
		// RFC 8654 Section 2: retain complete frames, including >4096 UPDATEs.
		octets, err := readExtendedFrame(source, frame[:])
		if err != nil {
			return err
		}
		original := frame[:octets]
		delivered := original
		row := extendedRelayFrame{Original: textbuf.StringHexUpper(original)}
		if original[18] == bgpOpen {
			// RFC 8654 Section 3: the only wire transformation in this relay.
			octets, _, err = rewriteExtendedOpen(rewritten[:], original, advertise)
			if err != nil {
				return err
			}
			delivered = rewritten[:octets]
			row.Delivered = textbuf.StringHexUpper(delivered)
		}
		written, err := target.Write(delivered)
		if err != nil {
			return err
		}
		if written != len(delivered) {
			return io.ErrShortWrite
		}
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	return errors.New("extended relay exceeded 256 frames")
}

// readExtendedFrame uses the wire length, never a TCP read's size.
// RFC 4271 Section 4.1: "This 2-octet unsigned integer indicates the total
// length of the message, including the header in octets."
// RFC 8654 Section 2: "BGP Extended Messages have a maximum message size of
// 65,535 octets."
// Wire offsets: [0:16] marker | [16:18] length | [18] type | [19:] body.
func readExtendedFrame(source io.Reader, frame []byte) (int, error) {
	if len(frame) < bgpHeaderLength {
		return 0, io.ErrShortBuffer
	}
	if _, err := io.ReadFull(source, frame[:bgpHeaderLength]); err != nil {
		return 0, err
	}
	for _, octet := range frame[:16] {
		if octet != 0xff {
			return 0, errors.New("relay received invalid BGP marker")
		}
	}
	octets := int(binary.BigEndian.Uint16(frame[16:18]))
	if octets < bgpHeaderLength {
		return 0, errors.New("relay received short BGP length")
	}
	if octets > len(frame) {
		return 0, io.ErrShortBuffer
	}
	_, err := io.ReadFull(source, frame[bgpHeaderLength:octets])
	return octets, err
}

// rewriteExtendedOpen removes all capability 6 TLVs, then optionally adds one.
// It never mutates the input: the checker needs Ze's ORIGINAL advertisement.
// RFC 8654 Section 3: "The BGP Extended Message Capability is a new BGP capability
// [RFC5492] defined with Capability Code 6 and Capability Length 0."
// Wire offsets: [19] version | [20:22] AS | [22:24] hold | [24:28] ID |
// [28] optional length | [29:] parameter(type,length,value); type 2 contains
// capability(code,length,value). These fixtures use ordinary OPEN parameters,
// not RFC 9072 extended optional parameters, and reject that form explicitly.
func rewriteExtendedOpen(target, frame []byte, advertise bool) (int, bool, error) {
	if len(frame) < 29 {
		return 0, false, errors.New("short relay OPEN")
	}
	if frame[18] != bgpOpen {
		return 0, false, errors.New("relay rewrite requires OPEN")
	}
	if int(frame[28])+29 != len(frame) {
		return 0, false, errors.New("invalid relay OPEN optional length")
	}
	if len(target) < len(frame)+4 {
		return 0, false, io.ErrShortBuffer
	}
	copy(target[:29], frame[:29])
	written := 29
	found := false
	for offset := 29; offset < len(frame); {
		if offset+2 > len(frame) {
			return 0, false, errors.New("truncated OPEN parameter")
		}
		end := offset + 2 + int(frame[offset+1])
		if end > len(frame) {
			return 0, false, errors.New("truncated OPEN parameter value")
		}
		if frame[offset] != 2 {
			written += copy(target[written:], frame[offset:end])
			offset = end
			continue
		}
		start := written
		target[written] = 2
		written += 2
		for capability := offset + 2; capability < end; {
			if capability+2 > end {
				return 0, false, errors.New("truncated OPEN capability")
			}
			next := capability + 2 + int(frame[capability+1])
			if next > end {
				return 0, false, errors.New("truncated OPEN capability value")
			}
			if frame[capability] == 6 {
				if next != capability+2 {
					return 0, false, errors.New("nonempty extended-message capability")
				}
				found = true
			} else {
				written += copy(target[written:], frame[capability:next])
			}
			capability = next
		}
		if written == start+2 {
			written = start
		} else {
			target[start+1] = byte(written - start - 2)
		}
		offset = end
	}
	if advertise {
		copy(target[written:], []byte{2, 2, 6, 0})
		written += 4
	}
	if written-29 > 254 {
		return 0, false, errors.New("relay OPEN requires extended optional parameters")
	}
	target[28] = byte(written - 29)
	binary.BigEndian.PutUint16(target[16:18], uint16(written))
	return written, found, nil
}
