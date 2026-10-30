// Design: docs/architecture/testing/interop.md -- RFC 2661 unknown mandatory AVP interoperability.
// Related: avprelay/main.go -- transparent relay of the real xl2tpd exchange.
package l2tp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// relayFromLAC is the relay's direction label for a packet the LAC sent.
const relayFromLAC = "lac"

func avpRelayPeer(suffix string, network interoplab.Network) (interoplab.PeerConfig, error) {
	ze, err := addressAtHost(network.IPv4, 2)
	if err != nil {
		return interoplab.PeerConfig{}, err
	}
	lac, err := addressAtHost(network.IPv4, 3)
	if err != nil {
		return interoplab.PeerConfig{}, err
	}
	return interoplab.PeerConfig{
		Name: peerAVPRelay, Container: "ze-l2tp-avp-relay-" + suffix,
		Image: "avp-relay", Host: 4,
		Command: []string{netip.AddrPortFrom(ze, 1701).String(), netip.AddrPortFrom(lac, 1702).String()},
		Ready: &interoplab.ReadyProbe{
			Command: []string{"test", "-f", "/run/ready"}, Timeout: 15 * time.Second, Interval: time.Second,
		},
	}, nil
}

const peerAVPRelay = "avp-relay"

// checkUnknownMandatoryAVP first admits an optional unknown AVP on xl2tpd's
// SCCCN and proves PPP traffic. Only then does it arm the relay to append the
// same AVP with M=1 to xl2tpd's next real HELLO. It never fabricates a message,
// changes sequencing, stops a peer, or substitutes for the external daemon.
// RFC 2661 Section 4.1: "If the M bit is set on an unrecognized AVP within
// a message associated with the overall tunnel, the entire tunnel (and all
// sessions within) MUST be terminated." With M=0 it "MUST be ignored."
func checkUnknownMandatoryAVP(ctx context.Context, lab labOperations, _ time.Duration) error {
	for _, message := range []string{logSessionIPAssigned, "subscriber route inject"} {
		if err := waitPeerLog(ctx, lab, peerZe, message, 60*time.Second); err != nil {
			return err
		}
	}
	for _, peer := range []string{peerZe, peerLAC} {
		if _, err := waitPPPInterface(ctx, lab, peer, 30*time.Second); err != nil {
			return err
		}
	}
	if _, err := lab.Exec(ctx, peerLAC, []string{"ping", "-c", "3", "-W", "3", localPPPAddress}, nil); err != nil {
		return fmt.Errorf("optional AVP positive-control traffic: %w", err)
	}
	logs, err := lab.Logs(ctx, peerAVPRelay, 1000)
	if err != nil {
		return err
	}
	if err := optionalAVPEvidence(logs.Text); err != nil {
		return err
	}
	if _, err := lab.Exec(ctx, peerAVPRelay, []string{"touch", "/run/mandatory"}, nil); err != nil {
		return err
	}
	// xl2tpd 1.3.20 schedules HELLO every HELLO_DELAY (60 seconds) in
	// control.c:hello. Poll the wire event, not elapsed time.
	wire, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 90 * time.Second, Interval: time.Second, Description: "xl2tpd HELLO M=1 and Ze StopCCN result=2 error=8",
	}, func(probeCtx context.Context) (string, error) {
		result, readErr := lab.Logs(probeCtx, peerAVPRelay, 1000)
		if readErr != nil {
			return "", readErr
		}
		if parseErr := mandatoryAVPRefusal(result.Text); parseErr != nil {
			return result.Text, parseErr
		}
		return result.Text, nil
	}, func(string) bool { return true })
	if err != nil {
		return err
	}
	if err := waitPeerLog(ctx, lab, peerLAC, "Connection closed to", 30*time.Second); err != nil {
		return err
	}
	if err := waitPeerLog(ctx, lab, peerZe, "subscriber routes withdrawn", 30*time.Second); err != nil {
		return err
	}
	if err := waitL2TPClean(ctx, lab, 30*time.Second); err != nil {
		return err
	}
	// Retain the actual before/after datagrams in the native job log before
	// Suite removes the relay container, including on a successful run.
	_, err = fmt.Fprintln(os.Stdout, "L2TP-AVP-WIRE optional control, mandatory refusal and both-peer teardown observed\n"+wire)
	return err
}

type avpWireObservation struct {
	Direction string `json:"direction"`
	Original  string `json:"original"`
	Forwarded string `json:"forwarded"`
}

func optionalAVPEvidence(text string) error {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event avpWireObservation
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return fmt.Errorf("decode relay observation: %w", err)
		}
		if event.Direction != relayFromLAC {
			continue
		}
		if _, err := injectedControl(event, 3, false); err == nil {
			return nil
		}
	}
	return errors.New("no real xl2tpd SCCCN with only an appended unknown optional AVP")
}

func mandatoryAVPRefusal(text string) error {
	var hello []byte
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event avpWireObservation
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return fmt.Errorf("decode relay observation: %w", err)
		}
		if event.Direction == relayFromLAC {
			packet, err := injectedControl(event, 6, true)
			if err == nil {
				hello = packet
			}
			continue
		}
		if hello == nil {
			continue
		}
		if event.Direction != "ze" {
			continue
		}
		packet, err := hex.DecodeString(event.Forwarded)
		if err != nil {
			return fmt.Errorf("decode forwarded packet: %w", err)
		}
		if event.Original != event.Forwarded {
			return errors.New("relay modified Ze's answer")
		}
		if err := requireStopCCN(packet, hello); err == nil {
			return nil
		}
	}
	return errors.New("missing matching StopCCN result=2 error=8 acknowledging the injected M=1 HELLO")
}

// injectedControl proves all original bytes except the length survive, including
// the peer's tunnel/session IDs and Ns/Nr. Type 65534 is outside RFC 2661's
// IETF catalog; H and reserved bits remain clear, isolating unknown-type handling.
func injectedControl(event avpWireObservation, message uint16, mandatory bool) ([]byte, error) {
	original, err := hex.DecodeString(event.Original)
	if err != nil {
		return nil, err
	}
	packet, err := hex.DecodeString(event.Forwarded)
	if err != nil {
		return nil, err
	}
	if err := requireControl(original, message); err != nil {
		return nil, err
	}
	if len(packet) != len(original)+6 {
		return nil, errors.New("injection did not append exactly six octets")
	}
	if err := requireControl(packet, message); err != nil {
		return nil, err
	}
	if !bytes.Equal(packet[:2], original[:2]) {
		return nil, errors.New("injection changed header flags")
	}
	if !bytes.Equal(packet[4:len(original)], original[4:]) {
		return nil, errors.New("injection changed original control bytes")
	}
	want := [6]byte{0, 6, 0, 0, 0xff, 0xfe}
	if mandatory {
		want[0] = 0x80
	}
	if !bytes.Equal(packet[len(original):], want[:]) {
		return nil, errors.New("injection is not unknown IETF AVP 65534 with expected M bit")
	}
	return packet, nil
}

func requireControl(packet []byte, message uint16) error {
	if len(packet) < 20 {
		return errors.New("short control packet")
	}
	if binary.BigEndian.Uint16(packet[:2]) != 0xc802 {
		return errors.New("unexpected control header flags")
	}
	if int(binary.BigEndian.Uint16(packet[2:4])) != len(packet) {
		return errors.New("incorrect control length")
	}
	if binary.BigEndian.Uint16(packet[6:8]) != 0 {
		return errors.New("control message has a session ID")
	}
	if !bytes.Equal(packet[12:18], []byte{0x80, 8, 0, 0, 0, 0}) {
		return errors.New("missing leading Message Type AVP")
	}
	if binary.BigEndian.Uint16(packet[18:20]) != message {
		return errors.New("different control message type")
	}
	return nil
}

func requireStopCCN(packet, hello []byte) error {
	if err := requireControl(packet, 4); err != nil {
		return err
	}
	if binary.BigEndian.Uint16(packet[10:12]) != binary.BigEndian.Uint16(hello[8:10])+1 {
		return errors.New("StopCCN does not acknowledge injected HELLO")
	}
	resultSeen, tunnelSeen := false, false
	for offset := 20; offset < len(packet); {
		if len(packet)-offset < 6 {
			return errors.New("truncated StopCCN AVP header")
		}
		length := int(binary.BigEndian.Uint16(packet[offset:]) & 0x3ff)
		if length < 6 {
			return errors.New("short StopCCN AVP")
		}
		if length > len(packet)-offset {
			return errors.New("truncated StopCCN AVP value")
		}
		if binary.BigEndian.Uint16(packet[offset+2:]) != 0 {
			return errors.New("unexpected vendor StopCCN AVP")
		}
		switch binary.BigEndian.Uint16(packet[offset+4:]) {
		case 1:
			if resultSeen {
				return errors.New("duplicate Result Code")
			}
			if length < 10 {
				return errors.New("StopCCN has no Error Code")
			}
			if !bytes.Equal(packet[offset+6:offset+10], []byte{0, 2, 0, 8}) {
				return errors.New("StopCCN is not result=2 error=8")
			}
			resultSeen = true
		case 9:
			if tunnelSeen {
				return errors.New("duplicate Assigned Tunnel ID")
			}
			if length != 8 {
				return errors.New("bad Assigned Tunnel ID length")
			}
			if !bytes.Equal(packet[offset+6:offset+8], hello[4:6]) {
				return errors.New("StopCCN names another tunnel")
			}
			tunnelSeen = true
		default:
			// The wire AVP catalog is open; unrelated AVPs do not prove either field.
		}
		offset += length
	}
	if !resultSeen {
		return errors.New("missing Result Code")
	}
	if !tunnelSeen {
		return errors.New("missing Assigned Tunnel ID")
	}
	return nil
}
