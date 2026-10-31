// Design: docs/architecture/testing/interop.md -- Code-Reject PPP frame injection.
// Related: main.go -- the relay loop that hands every data datagram here.
// RFC 1661 Section 5.6 -- see rfc/short/rfc1661.md.
package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"os"
)

// codeRejectMarker is the file the ppp-code-reject-xl2tpd checker creates once
// the PPP session carries traffic. Until it exists the relay forwards data
// datagrams and logs none of them.
const codeRejectMarker = "/run/code-reject"

// terminateMarker is the file the checker creates once it has read Ze's
// Code-Rejects. The next LAC datagram after it is the trigger for one IPCP
// Terminate-Request, once per relay, which ends the scenario's IPCP.
const terminateMarker = "/run/ipcp-terminate"

// terminateIdentifier is the Identifier of the injected Terminate-Request,
// which Ze's Terminate-Ack must carry (RFC 1661 Section 5.5).
const terminateIdentifier = 0x5a

// codeTerminateRequest is the LCP and IPCP Terminate-Request Code (RFC 1661
// Section 5.5).
const codeTerminateRequest = 5

// injection records what the relay has sent Ze, so each injection happens
// once per relay.
type injection struct {
	codeReject bool
	terminate  bool
}

// L2TP data message header flags, RFC 2661 Section 3.1.
const (
	flagType     = 0x8000
	flagLength   = 0x4000
	flagSequence = 0x0800
	flagOffset   = 0x0200
	versionMask  = 0x000f
)

// PPP protocol numbers of the probes.
const (
	protocolLCP  = 0xc021
	protocolIPCP = 0x8021
)

// probeIdentifier is the one Identifier all four probes carry, so a Ze that
// copied the rejected packet's Identifier would answer all four with it.
const probeIdentifier = 0x42

// probeDataOctets is each probe's Data length. With its 4-octet header a
// probe is 204 octets, longer than the 128-octet MRU the scenario's pppd
// requests, so Ze can answer within that MRU only by cutting the copy.
const probeDataOctets = 200

// probe is one PPP packet the relay sends Ze as the LAC.
type probe struct {
	protocol uint16
	code     byte
}

// codeRejectProbes are sent in this order. Code 99 is no LCP Code (RFC 1661
// Section 5), and Code 12 is none of IPCP's (RFC 1332 Section 2 allows Codes
// 1 through 7 only), so each must draw a Code-Reject. Two of each protocol give
// each protocol two Code-Rejects whose Identifiers can be compared.
var codeRejectProbes = [...]probe{
	{protocolLCP, 99}, {protocolLCP, 99}, {protocolIPCP, 12}, {protocolIPCP, 12},
}

// relayData forwards one data datagram to target. Once the checker's marker
// exists it also logs every data datagram in both directions, unfiltered, so
// the checker decides what each one is and refuses one it cannot read. The
// first datagram the LAC sends after the marker is the trigger: the relay forwards it, then sends Ze the four
// probes on that datagram's tunnel and session, once per relay. The first LAC
// datagram after terminateMarker is the trigger for one IPCP
// Terminate-Request the same way. The relay neither sequences nor
// acknowledges data messages, so it refuses to inject into a session that
// carries Ns and Nr.
func relayData(conn *net.UDPConn, encoder *json.Encoder, packet []byte,
	direction string, target netip.AddrPort, injected *injection) error {
	if _, err := conn.WriteToUDPAddrPort(packet, target); err != nil {
		return err
	}
	armed, err := markerExists(codeRejectMarker)
	if err != nil {
		return err
	}
	if !armed {
		return nil
	}
	encoded := hex.EncodeToString(packet)
	if err := encoder.Encode(wireObservation{direction, encoded, encoded}); err != nil {
		return err
	}
	if direction != "lac" {
		return nil
	}
	if !injected.codeReject {
		frames, err := codeRejectFrames(packet)
		if err != nil {
			return err
		}
		injected.codeReject = true
		return injectFrames(conn, encoder, target, frames)
	}
	if injected.terminate {
		return nil
	}
	armed, err = markerExists(terminateMarker)
	if err != nil {
		return err
	}
	if !armed {
		return nil
	}
	start, err := dataHeaderLength(packet)
	if err != nil {
		return err
	}
	injected.terminate = true
	frame := pppFrameBehind(packet[:start], protocolIPCP, codeTerminateRequest, terminateIdentifier, nil)
	return injectFrames(conn, encoder, target, [][]byte{frame})
}

// injectFrames sends each frame to target and logs it as the relay's own.
func injectFrames(conn *net.UDPConn, encoder *json.Encoder, target netip.AddrPort, frames [][]byte) error {
	for _, frame := range frames {
		if _, err := conn.WriteToUDPAddrPort(frame, target); err != nil {
			return err
		}
		if err := encoder.Encode(wireObservation{"inject", "", hex.EncodeToString(frame)}); err != nil {
			return err
		}
	}
	return nil
}

func markerExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		// The absent marker holds the injection back until the checker asks for it.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// dataHeaderLength answers where the PPP frame starts in an L2TP data message.
// It refuses a control message, another version, a sequenced data message and
// a frame that does not open with the PPP Address and Control octets.
//
// RFC 2661 Section 3.1 header, in the order its optional fields appear:
//
//	octet 0-1  flags: T L x x S x O P x x x x Ver
//	           Length (2, when L), Tunnel ID (2), Session ID (2),
//	           Ns (2) and Nr (2) (when S), Offset Size (2) and its pad (when O)
//
// The Linux PPPoL2TP transmit path writes Address 0xff and Control 0x03 before
// the 2-octet PPP Protocol field, so a frame the relay accepts holds at least
// those four octets after the header.
func dataHeaderLength(packet []byte) (int, error) {
	if len(packet) < 6 {
		return 0, errors.New("short L2TP data message")
	}
	flags := binary.BigEndian.Uint16(packet)
	if flags&flagType != 0 {
		return 0, errors.New("a control message, not a data message")
	}
	if flags&versionMask != 2 {
		return 0, errors.New("not L2TP version 2")
	}
	if flags&flagSequence != 0 {
		return 0, errors.New("data message carries Ns and Nr: an injected frame would break the sequence")
	}
	start := 2
	if flags&flagLength != 0 {
		start += 2
	}
	start += 4
	if flags&flagOffset != 0 {
		if len(packet) < start+2 {
			return 0, errors.New("short L2TP offset field")
		}
		start += 2 + int(binary.BigEndian.Uint16(packet[start:]))
	}
	if len(packet) < start+4 {
		return 0, errors.New("short PPP frame")
	}
	if packet[start] != 0xff {
		return 0, errors.New("PPP frame has no Address octet 0xff")
	}
	if packet[start+1] != 0x03 {
		return 0, errors.New("PPP frame has no Control octet 0x03")
	}
	return start, nil
}

// codeRejectFrames builds the four probes on trigger's L2TP header, so they
// carry the live tunnel and session IDs. A header with a Length field gets
// each frame's own length.
//
// Each frame: the trigger's L2TP header, then 0xff 0x03, the 2-octet PPP
// Protocol, and the packet: Code (1), Identifier (1), Length (2), Data (200
// octets, octet i holding i).
func codeRejectFrames(trigger []byte) ([][]byte, error) {
	start, err := dataHeaderLength(trigger)
	if err != nil {
		return nil, err
	}
	data := make([]byte, probeDataOctets)
	for i := range data {
		data[i] = byte(i)
	}
	frames := make([][]byte, 0, len(codeRejectProbes))
	for _, p := range codeRejectProbes {
		frames = append(frames, pppFrameBehind(trigger[:start], p.protocol, p.code, probeIdentifier, data))
	}
	return frames, nil
}

// pppFrameBehind builds one PPP control packet behind a copy of an L2TP data
// header, which carries the live tunnel and session. A header with a Length
// field gets the frame's own length.
//
// The frame: the header, then 0xff 0x03, the 2-octet PPP Protocol, and the
// packet: Code (1), Identifier (1), Length (2), Data.
func pppFrameBehind(header []byte, protocol uint16, code, identifier byte, data []byte) []byte {
	start := len(header)
	frame := make([]byte, start+4+4+len(data))
	copy(frame, header)
	if binary.BigEndian.Uint16(frame)&flagLength != 0 {
		binary.BigEndian.PutUint16(frame[2:], uint16(len(frame)))
	}
	frame[start] = 0xff
	frame[start+1] = 0x03
	binary.BigEndian.PutUint16(frame[start+2:], protocol)
	packet := frame[start+4:]
	packet[0] = code
	packet[1] = identifier
	binary.BigEndian.PutUint16(packet[2:], uint16(len(packet)))
	copy(packet[4:], data)
	return frame
}
