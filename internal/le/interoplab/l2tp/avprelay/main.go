// Design: docs/architecture/testing/interop.md -- transparent xl2tpd AVP injection.
// Related: pppinject.go -- the PPP frames sent once the code-reject marker is set.
// Related: lcprestart.go -- configured-retry loss after a live LCP restart.
// This process is not an L2TP peer: it maintains no protocol state, and the
// real xl2tpd and Ze own sequencing, ACKs and retransmits. It originates no
// control datagram. A checker can arm a bounded PPP injection: the Code-Reject
// probes, or one Configure-Request followed by loss of LAC LCP replies.
// RFC 2661 Section 4.1 -- see rfc/short/rfc2661.md.
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"time"
)

// directionLAC identifies datagrams received from the LAC at the relay boundary.
const directionLAC = "lac"

func main() {
	if err := run(); err != nil {
		slog.Error("L2TP AVP relay failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 3 {
		return errors.New("usage: avp-relay <ze-ip:port> <lac-ip:port> [mode]")
	}
	if len(os.Args) > 4 {
		return errors.New("usage: avp-relay <ze-ip:port> <lac-ip:port> [mode]")
	}
	ze, err := netip.ParseAddrPort(os.Args[1])
	if err != nil {
		return err
	}
	lac, err := netip.ParseAddrPort(os.Args[2])
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 1701})
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // Docker owns process teardown; no buffered writes remain.

	// One sequential relay owns the socket until Docker tears down the lab.
	// The deadline and datagram cap also bound a failed or noisy scenario.
	deadline := time.Now().Add(5 * time.Minute)
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	var buffer [65536]byte
	encoder := json.NewEncoder(os.Stdout)
	transport := relayIO{conn, encoder, ze, lac, deadline}
	name := ""
	if len(os.Args) == 4 {
		name = os.Args[3]
	}
	mode, err := selectRelayMode(name, &transport)
	if err != nil {
		return err
	}
	if err := os.WriteFile("/run/ready", []byte("ready\n"), 0o600); err != nil {
		return err
	}
	controls := 0
	for range 65536 {
		if !time.Now().Before(deadline) {
			return errors.New("relay deadline exceeded")
		}
		readDeadline, err := mode.beforeRead(time.Now())
		if err != nil {
			return err
		}
		if err := conn.SetReadDeadline(readDeadline); err != nil {
			return err
		}
		// Every mode shares this fixed-capacity buffer. inject MUST receive
		// six spare bytes, even when the selected mode never injects.
		count, source, err := conn.ReadFromUDPAddrPort(buffer[:len(buffer)-6])
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				if time.Now().Before(deadline) {
					continue
				}
			}
			return err
		}
		target := ze
		direction := directionLAC
		switch source {
		case lac:
		case ze:
			target, direction = lac, "ze"
		default:
			return fmt.Errorf("unexpected UDP source %s", source)
		}
		packet := buffer[:count]
		control := count >= 2 && packet[0]&0x80 != 0
		if !control {
			if err := mode.data(packet, direction, target); err != nil {
				return err
			}
			continue
		}
		controls++
		if controls > mode.controlLimit {
			return errors.New("control datagram limit exceeded")
		}
		if err := mode.control(packet, direction, target); err != nil {
			return err
		}
	}
	return errors.New("datagram limit exceeded")
}

// wireObservation is one relayed or injected datagram, as the checkers read it
// from the relay's standard output.
type wireObservation struct {
	Direction string `json:"direction"`
	Original  string `json:"original"`
	Forwarded string `json:"forwarded"`
}

func mandatoryArmed() (bool, error) {
	_, err := os.Stat("/run/mandatory")
	if errors.Is(err, os.ErrNotExist) {
		// The absent marker guards injection until PPP traffic has succeeded.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// inject changes only SCCCN (M=0) and, after the checker arms it, HELLO
// (M=1). The caller MUST provide six spare bytes in packet's capacity.
// RFC 2661 Section 4.1: "If the M bit is not set, an unrecognized AVP MUST be ignored."
// The appended AVP is six octets: flags/length at 0, vendor at 2, type at 4.
// Its length is 6, vendor is 0, type is 65534, and only M changes between inputs.
func inject(packet []byte, mandatory bool) []byte {
	if len(packet) < 20 {
		return packet
	}
	if binary.BigEndian.Uint16(packet[:2]) != 0xc802 {
		return packet
	}
	if int(binary.BigEndian.Uint16(packet[2:4])) != len(packet) {
		return packet
	}
	if binary.BigEndian.Uint16(packet[12:14]) != 0x8008 {
		return packet
	}
	if binary.BigEndian.Uint32(packet[14:18]) != 0 {
		return packet
	}
	message := binary.BigEndian.Uint16(packet[18:20])
	flags := byte(0)
	switch message {
	case 3: // SCCCN remains the optional positive control, even on retransmit.
	case 6:
		if !mandatory {
			return packet
		}
		flags = 0x80
	default:
		// The open wire message catalog is forwarded without interpretation.
		return packet
	}
	packet = append(packet, flags, 6, 0, 0, 0xff, 0xfe)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	return packet
}
