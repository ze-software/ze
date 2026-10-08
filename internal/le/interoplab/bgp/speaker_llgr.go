// Design: docs/architecture/testing/interop.md -- independent LLGR lifecycle proof.
// Related: speaker.go -- native speaker dispatch and bounded frame I/O.
package bgp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const llgrSourceOracle = "llgr-omitted-family-source"

// runLLGRSource sends fixed, asymmetric capabilities, never mirrored OPENs.
// The checker MUST send USR1 only after Ze and FRR have received its routes.
// This oracle alone registers USR1 and MUST release that registration on exit.
// The source MUST close only its BGP transport on control, then stay alive until
// its bounded lifetime ends. Unexpected close, NOTIFICATION or expiry before
// control is an error.
func runLLGRSource(options speakerOptions, _ io.Writer) error {
	if options.asn != 65004 && options.asn != 65005 {
		return errors.New("LLGR source requires ASN 65004 or 65005")
	}
	if options.duration <= 0 {
		return errors.New("LLGR source requires a positive lifetime")
	}
	if options.duration > 300*time.Second {
		return errors.New("LLGR source lifetime exceeds 300 seconds")
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
	return runLLGRSourceConnection(ctx, options, connection, control)
}

// runLLGRSourceConnection owns connection and MUST close it before returning.
// Its caller MUST supply a bounded context and relinquish the connection.
// The existing idle read bounds control handling without a second goroutine.
func runLLGRSourceConnection(ctx context.Context, options speakerOptions, connection net.Conn, control <-chan os.Signal) error {
	defer connection.Close() //nolint:errcheck // Also closes the transport on pre-control failures.
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.New("LLGR source has no lifetime deadline")
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}
	local, err := netip.ParseAddrPort(connection.LocalAddr().String())
	if err != nil {
		return err
	}
	if !local.Addr().Is4() {
		return errors.New("LLGR source requires IPv4 transport")
	}
	zeroLLST := options.asn == 65005
	var nextHopV6 netip.Addr
	if !zeroLLST {
		nextHopV6, err = netip.ParseAddr(options.sourceNextHopV6)
		if err != nil {
			return fmt.Errorf("LLGR IPv6 next hop: %w", err)
		}
		if !nextHopV6.Is6() {
			return errors.New("LLGR control requires an IPv6 next hop")
		}
	}
	// RFC 9494 Sections 3.1 and 4.2: fixed GR omission and per-family LLST.
	open, err := llgrSourceOpen(options, zeroLLST)
	if err != nil {
		return err
	}
	if _, err := connection.Write(open); err != nil {
		return err
	}
	opened, established := false, false
	nextKeepalive := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("LLGR source ended before transport-loss control: %w", err)
		}
		select {
		case <-control:
			if !established {
				return errors.New("LLGR transport-loss control before establishment")
			}
			if err := connection.Close(); err != nil {
				return fmt.Errorf("LLGR controlled transport close: %w", err)
			}
			// No NOTIFICATION, KEEPALIVEs or reconnect after control. PID 1
			// stays alive so Docker retains the source's next-hop interfaces.
			<-ctx.Done()
			return nil
		default:
		}
		if established {
			if !time.Now().Before(nextKeepalive) {
				if _, err := connection.Write(speakerKeepalive()); err != nil {
					return err
				}
				nextKeepalive = time.Now().Add(10 * time.Second)
			}
		}
		kind, body, idle, err := readSpeakerMessage(connection, deadline)
		if err != nil {
			return fmt.Errorf("LLGR source lost its fenced connection: %w", err)
		}
		if idle {
			continue
		}
		switch kind {
		case bgpOpen:
			if opened {
				return errors.New("duplicate OPEN at LLGR source")
			}
			if len(body) < 10 {
				return errors.New("short OPEN at LLGR source")
			}
			if body[0] != 4 {
				return errors.New("LLGR source received an unsupported BGP version")
			}
			if binary.BigEndian.Uint16(body[1:3]) != 65001 {
				return errors.New("LLGR source connected to an unexpected BGP peer")
			}
			opened = true
			if _, err := connection.Write(speakerKeepalive()); err != nil {
				return err
			}
		case bgpKeepalive:
			if !opened {
				return errors.New("KEEPALIVE before OPEN at LLGR source")
			}
			if established {
				continue
			}
			established = true
			// RFC 4271 Section 4.3 and RFC 4760 Section 3: real received routes.
			for _, frame := range llgrSourceUpdates(local.Addr(), nextHopV6, uint32(options.asn), zeroLLST) {
				if _, err := connection.Write(frame); err != nil {
					return err
				}
			}
		case bgpUpdate:
			if !established {
				return errors.New("UPDATE before KEEPALIVE at LLGR source")
			}
		case bgpNotification:
			return fmt.Errorf("LLGR source received NOTIFICATION %x", body)
		default:
			return fmt.Errorf("LLGR source received unexpected message %d", kind)
		}
	}
	return errors.New("LLGR source expired before transport-loss control")
}

// llgrSourceOpen follows RFC 9494 Section 4.2: "If the Graceful Restart
// Capability that was received does not list all AFIs/SAFIs supported by the
// session, then the GR Restart Time shall be deemed zero for those AFIs/SAFIs
// that are not listed." Code 64 contains only IPv6 (or no family for the
// negative source), although its Restart Time is deliberately nonzero.
//
// Capability bytes: code/length, then code64 [restart:2][AFI:2 SAFI:1 flags:1],
// code71 [AFI:2 SAFI:1 flags:1 LLST:3]. LLST begins at tuple offset 4.
func llgrSourceOpen(options speakerOptions, zeroLLST bool) ([]byte, error) {
	routerID, err := netip.ParseAddr(options.routerID)
	if err != nil {
		return nil, err
	}
	if !routerID.Is4() {
		return nil, errors.New("LLGR source router ID must be IPv4")
	}
	capabilities := []byte{1, 4, 0, 1, 0, 1, 65, 4, 0, 0, byte(options.asn >> 8), byte(options.asn)}
	if zeroLLST {
		capabilities = append(capabilities, 64, 2, 0, 20, 71, 7, 0, 1, 1, 0, 0, 0, 0)
	} else {
		capabilities = append(capabilities, 1, 4, 0, 2, 0, 1,
			64, 6, 0, 20, 0, 2, 1, 0,
			71, 14, 0, 1, 1, 0, 0, 0, 40, 0, 2, 1, 0, 0, 0, 40)
	}
	body := make([]byte, 0, 12+len(capabilities))
	body = append(body, 4, byte(options.asn>>8), byte(options.asn), 0, 90)
	id := routerID.As4()
	body = append(body, id[:]...)
	body = append(body, byte(len(capabilities)+2), 2, byte(len(capabilities)))
	body = append(body, capabilities...)
	return speakerMessage(bgpOpen, body), nil
}

// llgrSourceUpdates implements RFC 4271 Section 4.3: "The UPDATE message is
// used to transfer routing information between BGP peers." The fixed source
// AS and ordinary marker community distinguish these routes from baseline
// announcements; no source sends LLGR_STALE itself.
//
// UPDATE body: withdrawn-length:2, attributes-length:2, attributes, IPv4 NLRI.
// MP_REACH value (RFC 4760 Section 3): AFI:2 SAFI:1 NH-length:1 NH:16 reserved:1 NLRI.
func llgrSourceUpdates(local, nextHopV6 netip.Addr, asn uint32, zeroLLST bool) [][]byte {
	attributes := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, byte(asn >> 8), byte(asn),
		0xc0, 8, 4, byte(asn >> 8), byte(asn), 0, 94}
	nextHop := local.As4()
	v4attrs := append(append([]byte(nil), attributes...), 0x40, 3, 4)
	v4attrs = append(v4attrs, nextHop[:]...)
	third := byte(94)
	if zeroLLST {
		third = 95
	}
	body := make([]byte, 0, 8+len(v4attrs))
	body = append(body, 0, 0, 0, byte(len(v4attrs)))
	body = append(body, v4attrs...)
	body = append(body, 24, 198, 51, third)
	frames := make([][]byte, 0, 4)
	frames = append(frames, speakerMessage(bgpUpdate, body), speakerEOR())
	if zeroLLST {
		return frames
	}
	v6next := nextHopV6.As16()
	mp := make([]byte, 0, 4+len(v6next)+8)
	mp = append(mp, 0, 2, 1, 16)
	mp = append(mp, v6next[:]...)
	mp = append(mp, 0, 48, 0x20, 1, 0x0d, 0xb8, 0, 0x94)
	v6attrs := append(append([]byte(nil), attributes...), 0x80, 14, byte(len(mp)))
	v6attrs = append(v6attrs, mp...)
	body = make([]byte, 0, 4+len(v6attrs))
	body = append(body, 0, 0, 0, byte(len(v6attrs)))
	body = append(body, v6attrs...)
	return append(frames, speakerMessage(bgpUpdate, body),
		speakerMessage(bgpUpdate, []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1}))
}
