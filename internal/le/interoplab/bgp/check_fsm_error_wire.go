// Design: docs/architecture/testing/interop.md -- RFC 6608 original-connection evidence.
// Related: check_fsm_error_test.go -- pure positive and vacuity branches.
package bgp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
)

// judgeFSMRelay replays the retained delivery order, not the relay's verdict.
// The captured native OPEN must be the bytes later replayed; only delivered
// frames count as state witnesses. The control is a real successful handshake
// with a configured route, not a short silence interval.
func judgeFSMRelay(report fsmRelayReport, test fsmRelayCase) error {
	if !report.Finished {
		return errors.New("FSM relay has no terminal capture boundary")
	}
	if report.Problem != "" {
		return errors.New("FSM relay transport: " + report.Problem)
	}
	if report.Personality != test.personality {
		return errors.New("FSM relay personality differs from scenario")
	}
	if len(report.Frames) == 0 || len(report.Frames) > fsmCaptureFramesMax {
		return errors.New("FSM relay capture frame count is missing or unbounded")
	}
	var nativeOpen []byte
	var nativeOpened, zeOpened, nativeDelivered, zeKA, nativeKA, route, triggered bool
	notifications := 0
	for _, row := range report.Frames {
		frame, err := hex.DecodeString(row.Frame)
		if err != nil {
			return err
		}
		if len(frame) < bgpHeaderLength {
			return errors.New("short FSM capture frame")
		}
		n, err := validateSpeakerMessageHeader(frame[:bgpHeaderLength])
		if err != nil {
			return err
		}
		if n != len(frame) {
			return errors.New("FSM capture frame length differs from header")
		}
		if row.Source != fsmZe && row.Source != fsmNative {
			return errors.New("unknown FSM capture source")
		}
		if row.Replay {
			if row.Source != fsmNative || !row.Delivered || !bytes.Equal(frame, nativeOpen) {
				return errors.New("FSM replay is not the captured native OPEN")
			}
			if triggered {
				return errors.New("second FSM trigger")
			}
			switch test.subcode {
			case 2:
				if !zeKA || nativeKA {
					return errors.New("OPEN replay lacks OpenConfirm witness")
				}
			case 3:
				if !zeKA || !nativeKA || !route {
					return errors.New("OPEN replay lacks Established route witness")
				}
			default:
				return errors.New("OPEN replay in a non-replay scenario")
			}
			triggered = true
			continue
		}
		switch frame[18] {
		case bgpOpen:
			if notifications != 0 {
				return errors.New("OPEN after FSM notification")
			}
			if err := validateSpeakerOpen(frame[bgpHeaderLength:]); err != nil {
				return err
			}
			endpoint, asn := report.ZeEndpoint, uint16(65001)
			if row.Source == fsmNative {
				endpoint, asn = report.NativeEndpoint, 65002
			}
			host, _, err := net.SplitHostPort(endpoint)
			if err != nil {
				return err
			}
			if binary.BigEndian.Uint16(frame[20:22]) != asn || !bytes.Equal(frame[24:28], net.ParseIP(host).To4()) {
				return errors.New("captured OPEN identity differs from actual endpoint")
			}
			if row.Source == fsmNative {
				if nativeOpened {
					return errors.New("replacement native OPEN")
				}
				nativeOpened, nativeDelivered = true, row.Delivered
				nativeOpen = frame
				if row.Delivered != (test.subcode != 1) {
					return errors.New("native OPEN delivery violates scenario gate")
				}
			} else {
				if zeOpened || !row.Delivered {
					return errors.New("missing or replacement Ze OPEN")
				}
				zeOpened = true
			}
		case bgpKeepalive:
			if row.Source == fsmNative && !row.Delivered && notifications == 1 {
				continue
			}
			if !nativeOpened || !zeOpened || notifications != 0 {
				return errors.New("KEEPALIVE lacks original OPENs")
			}
			switch {
			case row.Source == fsmZe:
				if !nativeDelivered || !row.Delivered {
					return errors.New("ze KEEPALIVE lacks delivered native OPEN")
				}
				zeKA = true
			case test.subcode == 1:
				if nativeDelivered || !row.Delivered || triggered {
					return errors.New("KEEPALIVE trigger is not in OpenSent")
				}
				triggered, nativeKA = true, true
			default:
				if row.Delivered != (test.subcode != 2) {
					return errors.New("native KEEPALIVE delivery violates scenario gate")
				}
				nativeKA = row.Delivered
			}
		case bgpUpdate:
			if row.Source == fsmNative && !row.Delivered && notifications == 1 {
				continue
			}
			if row.Source != fsmZe || !row.Delivered || !zeKA || !nativeKA {
				return errors.New("route witness preceded valid handshake")
			}
			if fsmRouteWitness(frame) {
				route = true
			}
		case bgpNotification:
			if test.subcode == 0 || !triggered || row.Source != fsmZe || !row.Delivered {
				return errors.New("notification lacks the intended state trigger")
			}
			if !bytes.Equal(frame[bgpHeaderLength:], []byte{5, test.subcode, test.message}) {
				return errors.New("FSM notification code, subcode or single Data octet is wrong")
			}
			notifications++
		default:
			return fmt.Errorf("unexpected captured message type %d", frame[18])
		}
	}
	if test.subcode == 0 {
		if !route || !zeKA || !nativeKA || notifications != 0 || report.ZeEOF || report.NativeEOF {
			return errors.New("valid control lacks a live Established route interval")
		}
		return nil
	}
	if !triggered || notifications != 1 || !report.ZeEOF || !report.NativeEOF {
		return errors.New("FSM error lacks exactly one notification and both natural EOFs")
	}
	return nil
}

// judgeFSMControlSnapshot judges the final answers, rather than relying on an
// earlier polling success. The fresh relay report follows both native queries.
func judgeFSMControlSnapshot(document, routes, neighbor string, initial, final fsmRelayReport, test fsmRelayCase) error {
	var peers map[string]struct {
		State string `json:"bgpState"`
	}
	if err := json.Unmarshal([]byte(document), &peers); err != nil {
		return err
	}
	peer, found := peers[neighbor]
	if !found || peer.State != stateEstablished {
		return errors.New("FRR final control neighbor is not Established")
	}
	if present, measured := jsonHasRoute(routes, fsmWitnessPrefix); !measured || !present {
		return errors.New("FRR final control answer lacks the decoded witness prefix")
	}
	if initial.ZeEndpoint != final.ZeEndpoint || initial.NativeEndpoint != final.NativeEndpoint {
		return errors.New("FSM final control report replaced an original connection endpoint")
	}
	return judgeFSMRelay(final, test)
}

// FRR 10.7.1 bgp_notify_receive stores the code, subcode and Data; bgp_notify_print
// logs its parsed length/hex, and bgp_show_peer_reset exposes the received reset.
// Sources: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_packet.c
// https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_debug.c
// https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_vty.c
// In the OpenSent scenario FRR itself stays OpenConfirm: it has not received
// Ze's KEEPALIVE. bgp_establish (bgp_fsm.c) has therefore not transferred the
// accepted doppelganger to the configured peer. The config peer's reset fields
// do not describe this connection; its parsed receive log and captured natural
// EOF establish receipt and closure. The other cases let FRR reach Established
// and additionally require its configured peer's received-reset fields.
func judgeFRRFSMNotification(document, log, neighbor string, test fsmRelayCase) error {
	if err := judgeFRRFSMNotificationLog(log, neighbor, test); err != nil {
		return err
	}
	if test.subcode == 1 {
		return nil
	}
	var peers map[string]struct {
		Code  string `json:"lastErrorCodeSubcode"`
		DueTo string `json:"lastResetDueTo"`
		State string `json:"bgpState"`
	}
	if err := json.Unmarshal([]byte(document), &peers); err != nil {
		return err
	}
	peer, found := peers[neighbor]
	if !found {
		return errors.New("FRR omitted the relay neighbor")
	}
	if peer.Code != fmt.Sprintf("05%02X", test.subcode) || !strings.Contains(strings.ToLower(peer.DueTo), "notification received") || peer.State == stateEstablished {
		return errors.New("FRR did not parse the expected received FSM reset")
	}
	return nil
}

func judgeFRRFSMNotificationLog(log, neighbor string, test fsmRelayCase) error {
	head := fmt.Sprintf("received from neighbor %s 5/%d ", neighbor, test.subcode)
	tail := fmt.Sprintf(" 1 bytes %02x", test.message)
	for line := range strings.SplitSeq(log, "\n") {
		if strings.Contains(line, "%NOTIFICATION") && strings.Contains(line, head) && strings.HasSuffix(strings.TrimSpace(line), tail) {
			return nil
		}
	}
	return errors.New("FRR parsed notification log lacks exactly one expected Data octet")
}
