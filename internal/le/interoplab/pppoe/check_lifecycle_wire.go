// Design: docs/architecture/testing/interop.md -- D-8 PPPoE lifecycle wire observations.
// Related: check_lifecycle.go -- real accel-ppp and pppd scenario drivers.
package pppoe

import (
	"bytes"
	"crypto/md5" // #nosec G501 -- RFC 1994 CHAP-MD5 interoperability, not a new authentication scheme.
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
)

// lifecycleFrame owns its payload; the pcap reader reuses its record buffer.
type lifecycleFrame struct {
	at       time.Time
	source   [6]byte
	target   [6]byte
	sid      uint16
	protocol uint16
	code     byte
	id       byte
	data     []byte
}

// lifecycleFrames rejects truncation and respects both declared wire lengths,
// excluding Ethernet padding from the CHAP digest and LCP data observations.
func lifecycleFrames(capture []byte) ([]lifecycleFrame, error) {
	reader, err := pcap.NewReader(bytes.NewReader(capture))
	if err != nil {
		return nil, err
	}
	if reader.LinkType() != pcap.LinkTypeEthernet {
		return nil, errors.New("lifecycle capture is not Ethernet")
	}
	var frames []lifecycleFrame
	var record pcap.Record
	for {
		if err := reader.Next(&record); err != nil {
			if errors.Is(err, io.EOF) {
				return frames, nil
			}
			return nil, err
		}
		if len(frames) >= 512 {
			return nil, errors.New("lifecycle capture exhausted its 512-frame bound")
		}
		raw := record.Data
		if len(raw) < 20 {
			return nil, errors.New("short Ethernet/PPPoE frame")
		}
		if record.OriginalLen != len(raw) {
			return nil, errors.New("truncated lifecycle capture frame")
		}
		if raw[14] != 0x11 {
			return nil, errors.New("unexpected PPPoE version/type")
		}
		length := int(binary.BigEndian.Uint16(raw[18:20]))
		if length > len(raw)-20 {
			return nil, errors.New("short PPPoE payload")
		}
		frame := lifecycleFrame{at: record.Timestamp, sid: binary.BigEndian.Uint16(raw[16:18]), code: raw[15]}
		copy(frame.target[:], raw[:6])
		copy(frame.source[:], raw[6:12])
		etherType := binary.BigEndian.Uint16(raw[12:14])
		if etherType == 0x8863 {
			frames = append(frames, frame)
			continue
		}
		if etherType != 0x8864 {
			return nil, errors.New("non-PPPoE frame in lifecycle capture")
		}
		if frame.code != 0 {
			return nil, errors.New("nonzero PPPoE session code")
		}
		if length < 2 {
			return nil, errors.New("missing PPP protocol")
		}
		payload := raw[20 : 20+length]
		frame.protocol = binary.BigEndian.Uint16(payload[:2])
		if frame.protocol == 0xc223 || frame.protocol == 0xc021 || frame.protocol == 0x8021 {
			if len(payload) < 6 {
				return nil, errors.New("short PPP control packet")
			}
			controlLength := int(binary.BigEndian.Uint16(payload[4:6]))
			if controlLength != len(payload)-2 {
				return nil, errors.New("PPP control length does not match PPPoE payload")
			}
			frame.code, frame.id = payload[2], payload[3]
			frame.data = bytes.Clone(payload[6:])
		}
		frames = append(frames, frame)
	}
}

// rechallengeEvidence requires two complete, fresh challenges after the driver
// has proved IPCP and ping. No discovery or LCP/IPCP renegotiation may replace
// the original network-phase session. RFC 1994 Section 4.1 requires the copied
// Identifier and MD5(Identifier || secret || Challenge Value).
func rechallengeEvidence(frames []lifecycleFrame, local [6]byte) error {
	var challenge *lifecycleFrame
	var previous []byte
	var previousID byte
	var sid uint16
	responses, successes := 0, 0
	for i := range frames {
		frame := &frames[i]
		if frame.protocol == 0 {
			return errors.New("discovery or PADT appeared during reauthentication")
		}
		if frame.sid == 0 {
			return errors.New("zero PPPoE session ID during reauthentication")
		}
		if sid == 0 {
			sid = frame.sid
		}
		if frame.sid != sid {
			return errors.New("session ID changed during reauthentication")
		}
		if frame.protocol == 0x8021 {
			return errors.New("IPCP restarted during reauthentication")
		}
		if frame.protocol == 0xc021 && frame.code <= 6 {
			return errors.New("LCP restarted or terminated during reauthentication")
		}
		if frame.protocol != 0xc223 {
			continue
		}
		switch frame.code {
		case 1:
			if frame.target != local {
				return errors.New("CHAP Challenge was not addressed to Ze")
			}
			if challenge != nil {
				return errors.New("CHAP Challenge repeated before successful response")
			}
			if len(frame.data) < 2 {
				return errors.New("CHAP Challenge has no value")
			}
			size := int(frame.data[0])
			if size == 0 {
				return errors.New("CHAP Challenge has an empty value")
			}
			if size >= len(frame.data) {
				return errors.New("CHAP Challenge value is truncated")
			}
			value := frame.data[1 : 1+size]
			if successes > 0 {
				if frame.id == previousID {
					return errors.New("reauthentication reused its Challenge Identifier")
				}
				if bytes.Equal(value, previous) {
					return errors.New("reauthentication reused its Challenge Value")
				}
			}
			challenge = frame
			previous, previousID = value, frame.id
		case 2:
			if challenge == nil {
				// Activation can bisect an earlier exchange. Only a captured
				// Challenge starts one of the two complete exchanges we count.
				if successes == 0 {
					continue
				}
				return errors.New("CHAP Response without captured Challenge")
			}
			if frame.source != local {
				return errors.New("CHAP Response did not come from Ze")
			}
			if frame.target != challenge.source {
				return errors.New("CHAP Response addressed a different peer")
			}
			if frame.id != challenge.id {
				return errors.New("CHAP Response direction or Identifier mismatch")
			}
			if responses != successes {
				return errors.New("duplicate CHAP Response")
			}
			var input [1 + len(pppdPassword) + 255]byte
			input[0] = challenge.id
			offset := 1 + copy(input[1:], pppdPassword)
			offset += copy(input[offset:], previous)
			digest := md5.Sum(input[:offset]) // #nosec G401 -- the RFC 1994 wire digest under test.
			if len(frame.data) != 1+len(digest)+len(pppdUsername) {
				return errors.New("CHAP Response length mismatch")
			}
			if frame.data[0] != byte(len(digest)) {
				return errors.New("CHAP Response value size mismatch")
			}
			if !bytes.Equal(frame.data[1:17], digest[:]) {
				return errors.New("CHAP Response digest mismatch")
			}
			if string(frame.data[17:]) != pppdUsername {
				return errors.New("CHAP Response digest, value size or username mismatch")
			}
			responses++
		case 3:
			if challenge == nil {
				if successes == 0 {
					continue
				}
				return errors.New("CHAP Success without captured Challenge")
			}
			if responses != successes+1 {
				return errors.New("CHAP Success preceded the exact Response")
			}
			if frame.source != challenge.source {
				return errors.New("CHAP Success came from a different peer")
			}
			if frame.target != local {
				return errors.New("CHAP Success was not addressed to Ze")
			}
			if frame.id != challenge.id {
				return errors.New("CHAP Success did not acknowledge the exact Response")
			}
			successes++
			challenge = nil
		case 4:
			return errors.New("accel-ppp rejected reauthentication")
		default:
			return fmt.Errorf("unexpected CHAP code %d", frame.code)
		}
	}
	if successes < 2 {
		return fmt.Errorf("captured %d successful network-phase CHAP exchanges, need two", successes)
	}
	if challenge != nil {
		return errors.New("last CHAP reauthentication is incomplete")
	}
	return nil
}

// terminateEvidence isolates the RFC 1661 tlf path: no client PADT reaches Ze,
// and Ze sends a same-session PADT after its matching Terminate-Ack and at least
// one Restart time after the request. ZRC arms the timer before STA sends Ack.
func terminateEvidence(frames []lifecycleFrame, local [6]byte, sid uint16) error {
	var request *lifecycleFrame
	var acknowledged time.Time
	padts := 0
	for i := range frames {
		frame := &frames[i]
		if frame.sid != sid {
			return errors.New("termination capture changed session ID")
		}
		if frame.protocol == 0 {
			if frame.code != 0xa7 {
				return errors.New("new discovery appeared during termination")
			}
			if frame.source != local {
				return errors.New("client PADT reached Ze: tlf was not isolated")
			}
			if request == nil {
				return errors.New("Ze PADT preceded peer Terminate-Request")
			}
			if frame.target != request.source {
				return errors.New("Ze PADT addressed a different peer")
			}
			if acknowledged.IsZero() {
				return errors.New("Ze PADT preceded Terminate-Ack")
			}
			if frame.at.Sub(request.at) < 3*time.Second {
				return errors.New("Ze PADT preceded the three-second Restart timer")
			}
			padts++
			continue
		}
		if frame.protocol != 0xc021 {
			continue
		}
		if frame.code == 5 {
			if frame.target != local {
				return errors.New("Terminate-Request did not come from pppd")
			}
			if request == nil {
				request = frame
			}
		}
		if frame.code == 6 {
			if request == nil {
				return errors.New("Terminate-Ack preceded Terminate-Request")
			}
			if frame.source != local {
				return errors.New("Terminate-Ack did not come from Ze")
			}
			if frame.target != request.source {
				return errors.New("Terminate-Ack addressed a different peer")
			}
			if frame.id != request.id {
				return errors.New("Terminate-Ack direction or Identifier mismatch")
			}
			if acknowledged.IsZero() {
				acknowledged = frame.at
			}
		}
	}
	if padts != 1 {
		return fmt.Errorf("captured %d Ze PADTs after peer termination, want one", padts)
	}
	return nil
}
