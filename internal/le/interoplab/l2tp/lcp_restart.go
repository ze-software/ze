// Design: docs/labs/l2tp-interop.md -- configured LCP Restart timing on the wire.
// Related: avprelay/lcprestart.go -- one restart injection and selective LCP loss.
// Peer source: https://github.com/ppp-project/ppp/blob/v2.5.5/pppd/fsm.c
// fsm_rconfreq in Opened restarts pppd and answers the received request.
package l2tp

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// restartWire is the relay's independent observation, before Docker transports
// its log. ElapsedNanos is measured on one monotonic clock from the injection.
type restartWire struct {
	Direction    string `json:"direction"`
	Original     string `json:"original"`
	Forwarded    string `json:"forwarded"`
	ElapsedNanos int64  `json:"elapsed_nanos"`
	Restart      bool   `json:"restart"`
}

type restartEvidence struct {
	requests []time.Duration
	replies  int
	injected int
	ended    bool
	endedAt  time.Duration
	tunnel   uint16
	session  uint16
}

// checkLCPRestart first proves a real pppd session carries traffic. The relay
// then injects one legal Configure-Request and loses pppd's LCP responses while
// preserving the tunnel. Exact wire counts and the eventual Ze CDN are the
// oracle; neither a daemon log nor an idle peer establishes a successful test.
func checkLCPRestart(ctx context.Context, lab labOperations, _ time.Duration) error {
	if err := waitPeerLog(ctx, lab, peerZe, logSessionIPLine, 60*time.Second); err != nil {
		return err
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return fmt.Errorf("answering pppd baseline: %w", err)
	}
	if _, err := lab.Exec(ctx, peerAVPRelay, []string{commandTouch, "/run/lcp-restart"}, nil); err != nil {
		return err
	}
	// This bounded ping supplies a LAC data header. Its reply is deliberately
	// immaterial after LCP goes down; the healthy ping above is the control.
	if err := lab.ExecDetached(ctx, peerLAC, []string{commandPing, "-c", "1", "-W", "1", localPPPAddress}, nil); err != nil {
		return err
	}
	evidence, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 8 * time.Second, Interval: 100 * time.Millisecond,
		Description: "configured LCP retries followed by Ze's session CDN",
	}, func(probeCtx context.Context) (restartEvidence, error) {
		logs, err := lab.Logs(probeCtx, peerAVPRelay, 1000)
		if err != nil {
			return restartEvidence{}, err
		}
		return readRestartEvidence(logs.Text)
	}, func(value restartEvidence) bool { return value.ended })
	if err != nil {
		return err
	}
	if err := requireRestartEvidence(evidence); err != nil {
		return err
	}
	if err := stopLACDaemon(ctx, lab); err != nil {
		return err
	}
	return waitL2TPClean(ctx, lab, 30*time.Second)
}

// readRestartEvidence only reads observations belonging to the armed restart.
// The log query caps the input at 1000 lines. Every observed packet is decoded,
// and a malformed packet is an error rather than an absent event.
func readRestartEvidence(text string) (restartEvidence, error) {
	var evidence restartEvidence
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event restartWire
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return evidence, fmt.Errorf("read restart relay: %w", err)
		}
		if !event.Restart {
			continue
		}
		if event.Direction != relayFromLAC {
			if event.Forwarded != event.Original {
				return evidence, errors.New("relay did not forward a Ze or injected packet unchanged")
			}
		}
		if event.ElapsedNanos < 0 {
			return evidence, errors.New("relay recorded negative elapsed time")
		}
		packet, err := hex.DecodeString(event.Original)
		if err != nil {
			return evidence, err
		}
		if len(packet) < 6 {
			return evidence, errors.New("short restart datagram")
		}
		if packet[0]&0x80 != 0 {
			if event.Direction != relayFromZe {
				continue
			}
			// RFC 2661 Section 4.4.1: the leading Message Type AVP names
			// CDN (14). A ZLB has no AVPs and cannot end the observation.
			if len(packet) < 20 {
				continue
			}
			if binary.BigEndian.Uint16(packet[18:20]) != 14 {
				continue
			}
			if binary.BigEndian.Uint16(packet[:2]) != 0xc802 {
				return evidence, errors.New("unexpected CDN header flags")
			}
			if int(binary.BigEndian.Uint16(packet[2:4])) != len(packet) {
				return evidence, errors.New("incorrect CDN Length")
			}
			// RFC 2661 Section 4.4.1: "The Message Type AVP MUST be the
			// first AVP in a message, immediately following the control
			// message header (defined in section 3.1)."
			if binary.BigEndian.Uint16(packet[12:14]) != 0x8008 {
				return evidence, errors.New("CDN has no leading Message Type AVP")
			}
			if binary.BigEndian.Uint32(packet[14:18]) != 0 {
				return evidence, errors.New("CDN has no leading Message Type AVP")
			}
			if binary.BigEndian.Uint16(packet[4:6]) != evidence.tunnel {
				return evidence, errors.New("CDN names another tunnel")
			}
			if binary.BigEndian.Uint16(packet[6:8]) != evidence.session {
				return evidence, errors.New("CDN names another session")
			}
			if !evidence.ended {
				evidence.ended = true
				evidence.endedAt = time.Duration(event.ElapsedNanos)
			}
			continue
		}
		protocol, information, err := pppInformation(packet)
		if err != nil {
			return evidence, err
		}
		if protocol != protocolLCP {
			continue
		}
		if len(information) < 4 {
			return evidence, errors.New("short observed LCP packet")
		}
		length := int(binary.BigEndian.Uint16(information[2:4]))
		if length < 4 {
			return evidence, errors.New("invalid observed LCP Length")
		}
		if length > len(information) {
			return evidence, errors.New("truncated observed LCP packet")
		}
		if event.Direction == relayInjected {
			if information[0] != codeConfigureRequest {
				return evidence, errors.New("relay injected another LCP Code")
			}
			if information[1] != 0x61 {
				return evidence, errors.New("relay injected another Identifier")
			}
			if length != 4 {
				return evidence, errors.New("relay injected unexpected Configure-Request options")
			}
			evidence.injected++
			continue
		}
		if event.Direction == relayFromLAC {
			if event.Forwarded != "" {
				return evidence, errors.New("relay forwarded LAC LCP during the silent-peer experiment")
			}
			// pppd fsm_rconfreq answers with Ack, Nak or Reject. Seeing
			// one proves the independent peer actually received Ze's request.
			if information[0] >= 2 {
				if information[0] <= 4 {
					evidence.replies++
				}
			}
			continue
		}
		if event.Direction == relayFromZe {
			if information[0] == codeConfigureRequest {
				if evidence.ended {
					return evidence, errors.New("received Configure-Request from Ze after session CDN")
				}
				// Tunnel and Session precede the optional sequence/offset fields.
				flags := binary.BigEndian.Uint16(packet)
				offset := 2
				if flags&0x4000 != 0 {
					offset += 2
				}
				tunnel := binary.BigEndian.Uint16(packet[offset : offset+2])
				session := binary.BigEndian.Uint16(packet[offset+2 : offset+4])
				if len(evidence.requests) != 0 {
					if tunnel != evidence.tunnel {
						return evidence, errors.New("Configure-Requests cross tunnels")
					}
					if session != evidence.session {
						return evidence, errors.New("Configure-Requests cross sessions")
					}
				}
				evidence.tunnel = tunnel
				evidence.session = session
				evidence.requests = append(evidence.requests, time.Duration(event.ElapsedNanos))
			}
		}
	}
	return evidence, nil
}

// requireRestartEvidence rejects a fixed three-second timer, an ignored request
// budget, a peer that did not answer, and premature session teardown. The wide
// upper margin permits scheduling delay while refusing even a1s period.
func requireRestartEvidence(evidence restartEvidence) error {
	if evidence.injected != 1 {
		return fmt.Errorf("restart injections = %d, want 1", evidence.injected)
	}
	if evidence.replies == 0 {
		return errors.New("no real pppd Configure reply was observed and dropped")
	}
	if len(evidence.requests) != 3 {
		return fmt.Errorf("received %d Configure-Requests from Ze, want exactly 3", len(evidence.requests))
	}
	for index := 1; index < len(evidence.requests); index++ {
		gap := evidence.requests[index] - evidence.requests[index-1]
		if gap < 350*time.Millisecond {
			return fmt.Errorf("Configure-Request gap %s is shorter than configured 500ms", gap)
		}
		if gap > 900*time.Millisecond {
			return fmt.Errorf("Configure-Request gap %s does not follow configured 500ms", gap)
		}
	}
	if !evidence.ended {
		return errors.New("received no CDN from Ze ending the exhausted session")
	}
	grace := evidence.endedAt - evidence.requests[2]
	if grace < 350*time.Millisecond {
		return fmt.Errorf("session ended %s after the last request, before configured timeout", grace)
	}
	if grace > 900*time.Millisecond {
		return fmt.Errorf("session ended %s after last request, beyond configured timeout", grace)
	}
	return nil
}
