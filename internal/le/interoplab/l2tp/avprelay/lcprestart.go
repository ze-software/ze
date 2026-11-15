// Design: docs/labs/l2tp-interop.md -- configured Restart timing against pppd.
// Related: pppinject.go -- bounded L2TP header parsing and PPP frame construction.
// RFC 1661 Sections 4.1 and 4.6 -- see rfc/short/rfc1661.md.
package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/netip"
	"time"
)

const restartMarker = "/run/lcp-restart"
const restartIdentifier = 0x61

// restartProbe belongs to the relay's sequential socket loop. A nil pointer
// means the checker has not triggered a restart; no timing claim is then made.
// It is not safe for concurrent use. Docker teardown ends its sole owner.
type restartProbe struct {
	started time.Time
}

// restartWire records elapsed monotonic time in the relay process, rather than
// Docker log arrival times. Empty Forwarded means the LAC's frame was dropped.
type restartWire struct {
	wireObservation
	ElapsedNanos int64 `json:"elapsed_nanos"`
	Restart      bool  `json:"restart"`
}

func encodeWire(encoder *json.Encoder, event wireObservation, probe *restartProbe) error {
	if probe == nil {
		return encoder.Encode(event)
	}
	return encoder.Encode(restartWire{event, time.Since(probe.started).Nanoseconds(), true})
}

// relayRestartData leaves the normal relay untouched until the marker exists
// and a LAC frame supplies the live tunnel/session header. It injects exactly
// one Configure-Request, then drops every LAC LCP frame while preserving L2TP
// control messages. The original data loop's deadline and datagram cap bound it.
func relayRestartData(conn *net.UDPConn, encoder *json.Encoder, packet []byte,
	direction string, target netip.AddrPort, injected *injection) (bool, error) {
	fresh := false
	if injected.restart == nil {
		armed, err := markerExists(restartMarker)
		if err != nil {
			return false, err
		}
		if !armed {
			return false, nil
		}
		if direction != directionLAC {
			return false, nil
		}
		injected.restart = &restartProbe{started: time.Now()}
		fresh = true
	}
	start, err := dataHeaderLength(packet)
	if err != nil {
		return true, err
	}
	drop := false
	if direction == directionLAC {
		drop = binary.BigEndian.Uint16(packet[start+2:]) == protocolLCP
	}
	original := hex.EncodeToString(packet)
	forwarded := ""
	if !drop {
		if _, err := conn.WriteToUDPAddrPort(packet, target); err != nil {
			return true, err
		}
		forwarded = original
	}
	if err := encodeWire(encoder, wireObservation{direction, original, forwarded}, injected.restart); err != nil {
		return true, err
	}
	if !fresh {
		return true, nil
	}
	// RFC 1661 Section 5.1: "The options field is variable in length, and
	// contains the list of zero or more Configuration Options that the sender
	// desires to negotiate." An empty request avoids inventing option values.
	frame := pppFrameBehind(packet[:start], protocolLCP, 1, restartIdentifier, nil)
	if _, err := conn.WriteToUDPAddrPort(frame, target); err != nil {
		return true, err
	}
	encoded := hex.EncodeToString(frame)
	return true, encodeWire(encoder, wireObservation{"inject", encoded, encoded}, injected.restart)
}
