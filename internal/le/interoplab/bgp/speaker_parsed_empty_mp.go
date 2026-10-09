// Design: docs/architecture/testing/interop.md -- bounded native speaker and relay.
// Related: check_parsed_empty_mp.go -- releases each phase only after recipient evidence.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const parsedEmptyMPSource = "parsed-empty-mp-source"
const parsedEmptyMPSourceCapture = "/tmp/parsed-empty-mp-source.jsonl"
const parsedEmptyMPSourceReady = "/tmp/parsed-empty-mp-ready"

// runParsedEmptyMPSource owns one connection and one bounded phase controller.
// USR1 is registered before dialing; the checker MUST wait for both peers before
// releasing the seed, then for each recipient fence before releasing the next phase.
func runParsedEmptyMPSource(options speakerOptions, _ io.Writer) (resultErr error) {
	if options.asn != 65004 {
		return errors.New("parsed-empty-MP source requires ASN 65004")
	}
	if options.duration <= 0 {
		return errors.New("parsed-empty-MP source requires a positive lifetime")
	}
	if options.duration > 300*time.Second {
		return errors.New("parsed-empty-MP source lifetime exceeds 300 seconds")
	}
	ctx, cancel := context.WithTimeout(context.Background(), options.duration)
	defer cancel()
	control := make(chan os.Signal, 1)
	signal.Notify(control, syscall.SIGUSR1)
	defer signal.Stop(control)
	connection, err := dialExtendedRelay(ctx, options.connect)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, connection.Close()) }()
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.New("source has no lifetime deadline")
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}
	local, err := netip.ParseAddrPort(connection.LocalAddr().String())
	if err != nil {
		return err
	}
	if !local.Addr().Is4() {
		return errors.New("source requires IPv4 transport")
	}
	capture, err := os.OpenFile(parsedEmptyMPSourceCapture, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, capture.Close()) }()
	encoder := json.NewEncoder(capture)
	write := func(frame []byte) error {
		written, err := connection.Write(frame)
		if err != nil {
			return err
		}
		if written != len(frame) {
			return io.ErrShortWrite
		}
		row := extendedRelayFrame{Original: textbuf.StringHexUpper(frame)}
		if frame[18] == bgpOpen {
			row.Delivered = row.Original
		}
		return encoder.Encode(row)
	}
	// RFC 5492 Section 4 and RFC 6793 Section 3: source advertises ASN4,
	// IPv4/IPv6, and no ADD-PATH. It never mirrors the destination OPEN.
	open, err := speakerOpen(65004, 90, net.ParseIP(options.routerID), []bgpFamily{{afi: 1, safi: 1}, {afi: 2, safi: 1}}, false, false)
	if err != nil {
		return err
	}
	if err := write(open); err != nil {
		return err
	}
	opened, established := false, false
	phase := 0
	nextKeepalive := time.Now().Add(10 * time.Second)
	// The deadline bounds the lifetime; at most three externally fenced phases
	// write UPDATEs. No receive event creates a goroutine or another connection.
	for time.Now().Before(deadline) {
		select {
		case <-control:
			if !established {
				return errors.New("source released before establishment")
			}
			phase++
			if phase > 3 {
				return errors.New("source received too many phase releases")
			}
			// RFC 4271 Section 4.3, RFC 7606 Section 5.1 and RFC 4724 Section 2.
			for _, frame := range parsedEmptyMPPhase(phase, local.Addr()) {
				if err := write(frame); err != nil {
					return err
				}
			}
		default:
		}
		if established && !time.Now().Before(nextKeepalive) {
			if err := write(speakerKeepalive()); err != nil {
				return err
			}
			nextKeepalive = time.Now().Add(10 * time.Second)
		}
		kind, body, idle, err := readSpeakerMessage(connection, deadline)
		if err != nil {
			return fmt.Errorf("source lost its fenced connection: %w", err)
		}
		if idle {
			continue
		}
		switch kind {
		case bgpOpen:
			if opened {
				return errors.New("duplicate source OPEN")
			}
			// RFC 5492 Section 4: require the negotiated source context too.
			if err := parsedEmptyMPOpen(speakerMessage(kind, body), true); err != nil {
				return err
			}
			opened = true
			if err := write(speakerKeepalive()); err != nil {
				return err
			}
		case bgpKeepalive:
			if !opened {
				return errors.New("source KEEPALIVE before OPEN")
			}
			if !established {
				// Ze can publish its own Established state before this source
				// consumes Ze's KEEPALIVE. The checker MUST wait for both.
				established = true
				if err := os.WriteFile(parsedEmptyMPSourceReady, []byte("established\n"), 0o600); err != nil {
					return err
				}
			}
		case bgpUpdate:
			if !established {
				return errors.New("source UPDATE before establishment")
			}
		case bgpNotification:
			return fmt.Errorf("source received NOTIFICATION %x", body)
		default:
			return fmt.Errorf("source received unexpected message %d", kind)
		}
	}
	return errors.New("source lifetime expired before lab teardown")
}

// parsedEmptyMPPhase implements RFC 7606 Section 5.1: "Since older BGP speakers
// may not implement these restrictions, an implementation MUST still be prepared
// to receive these fields in any position or combination."
// Body layout: withdrawn-length:2, withdrawn, attributes-length:2, attributes, NLRI.
// Phase two deliberately mixes an owned legacy withdrawal with empty IPv6 MP_UNREACH.
func parsedEmptyMPPhase(phase int, nextHop netip.Addr) [][]byte {
	eor := speakerMessage(bgpUpdate, []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1})
	announce := func(subnet byte) []byte {
		body := []byte{0, 0, 0, 27, 0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xec, 0x40, 3, 4}
		body = append(body, nextHop.AsSlice()...)
		// Distinct MEDs prevent the worker's legitimate attribute-bucket merge
		// from erasing the separate announcement fences.
		body = append(body, 0x80, 4, 4, 0, 0, 0, subnet)
		body = append(body, 24, 198, 51, subnet)
		return speakerMessage(bgpUpdate, body)
	}
	switch phase {
	case 1:
		return [][]byte{speakerEOR(), eor, announce(100), announce(101)}
	case 2:
		return [][]byte{speakerMessage(bgpUpdate, []byte{0, 4, 24, 198, 51, 100, 0, 6, 0x80, 15, 3, 0, 2, 1}), announce(102)}
	case 3:
		return [][]byte{eor, announce(103)}
	default:
		panic("BUG: unfenced parsed-empty-MP source phase")
	}
}
