// Design: docs/labs/l2tp-interop.md -- independent scale control evidence.
// Related: scale.go -- collection of live session and reliable state.
// RFC 2661 Sections 3.1, 4.4.1, 5.3 and 5.8 -- see rfc/short/rfc2661.md.
package l2tp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const scaleWindowWidth = 4
const scaleWireRowsMax = 65536
const scaleWireBytesMax = 32 << 20
const scaleWireControlsMax = 4096
const scaleWireSessions = 16

// scaleWindowEvidence is returned only after a complete hold is decoded. Its
// zero value is invalid. Callers MUST obtain it through readScaleWindow before
// using requireScaleReliable; released means actual new response progress.
type scaleWindowEvidence struct {
	tunnel     uint16
	baseline   uint16
	expectedNr uint16
	released   bool
}

// readScaleWindow is independent of the relay's window implementation: it
// reconstructs the sequence space and actual ACKs from complete wire bytes.
// RFC 2661 Section 5.8 -- ZLBs and retransmissions consume no additional Ns.
func readScaleWindow(log string) (scaleWindowEvidence, error) {
	trace, err := readScaleTrace(log)
	if err != nil {
		return scaleWindowEvidence{}, err
	}
	arm, baseline, err := scaleWindowArm(trace)
	if err != nil {
		return scaleWindowEvidence{}, err
	}
	proof := scaleWindowEvidence{tunnel: trace.local, baseline: baseline}
	outstanding := make(map[uint16]bool, scaleWindowWidth)
	requests := make(map[uint16]bool, scaleWireSessions)
	responses := make(map[uint16]bool, scaleWireSessions)
	var requestCount int
	var replay []byte
	var replayNr uint16
	var marker bool
	var fifth bool
	for index := range trace.events {
		event := &trace.events[index]
		if !event.control {
			continue
		}
		if event.lac {
			if event.release != "" {
				if event.release != "marker" {
					return scaleWindowEvidence{}, errors.New("scale hold released on its deadline instead of the checker marker")
				}
				if marker {
					return scaleWindowEvidence{}, errors.New("scale trace repeats the release marker")
				}
				if len(outstanding) != scaleWindowWidth {
					return scaleWindowEvidence{}, errors.New("scale release lacks four outstanding Ze controls")
				}
				if requestCount < scaleWindowWidth+1 {
					return scaleWindowEvidence{}, errors.New("scale release precedes the fifth held ICRQ")
				}
				if !bytes.Equal(event.original, replay) {
					return scaleWindowEvidence{}, errors.New("scale release is not the latest genuine held ACK packet")
				}
				if !bytes.Equal(event.original, event.forwarded) {
					return scaleWindowEvidence{}, errors.New("scale release modifies the genuine held ACK packet")
				}
				marker = true
			} else if index >= arm {
				if !marker {
					if event.sent.nr != baseline {
						return scaleWindowEvidence{}, errors.New("scale hold did not pin every LAC Nr to its baseline")
					}
					if scaleSequenceBefore(baseline, event.packet.nr) {
						if len(replay) == 0 {
							replay, replayNr = event.original, event.packet.nr
						} else if !scaleSequenceBefore(event.packet.nr, replayNr) {
							replay, replayNr = event.original, event.packet.nr
						}
					}
				}
			}
			if marker {
				if !bytes.Equal(event.original, event.forwarded) {
					return scaleWindowEvidence{}, errors.New("scale relay kept modifying Nr after marker release")
				}
			}
			for ns := range outstanding {
				if scaleSequenceBefore(ns, event.sent.nr) {
					delete(outstanding, ns)
				}
			}
			if !event.fresh {
				continue
			}
			if event.packet.message != 10 {
				continue
			}
			if event.packet.session != 0 {
				return scaleWindowEvidence{}, errors.New("ICRQ header Session ID is not zero")
			}
			remote, ok := event.packet.attributes[14]
			if !ok {
				return scaleWindowEvidence{}, errors.New("ICRQ has no Assigned Session ID")
			}
			if remote == 0 {
				return scaleWindowEvidence{}, errors.New("ICRQ assigns session ID zero")
			}
			if requests[remote] {
				return scaleWindowEvidence{}, errors.New("ICRQs reuse a LAC session ID within one tunnel")
			}
			requests[remote] = true
			if index < arm {
				continue
			}
			if marker {
				continue
			}
			requestCount++
			if requestCount == scaleWindowWidth+1 {
				proof.expectedNr = event.packet.ns + 1
			}
			continue
		}
		if !event.fresh {
			continue
		}
		outstanding[event.packet.ns] = true
		if len(outstanding) > scaleWindowWidth {
			return scaleWindowEvidence{}, errors.New("fifth distinct Ze control exceeded the default receive window")
		}
		if event.packet.message == 11 {
			if !requests[event.packet.session] {
				return scaleWindowEvidence{}, errors.New("received Ze ICRP has no intact preceding ICRQ")
			}
			if responses[event.packet.session] {
				return scaleWindowEvidence{}, errors.New("distinct Ze ICRPs answer the same session request")
			}
			if event.packet.attributes[14] == 0 {
				return scaleWindowEvidence{}, errors.New("ICRP lacks a nonzero Ze Assigned Session ID")
			}
			responses[event.packet.session] = true
		}
		if index >= arm {
			if !marker {
				if event.packet.message != 11 {
					return scaleWindowEvidence{}, errors.New("held Ze control is not a session response")
				}
			} else if event.packet.ns == baseline+scaleWindowWidth {
				if event.packet.message != 11 {
					return scaleWindowEvidence{}, errors.New("postrelease fifth Ze control is not an ICRP")
				}
				fifth = true
			}
		}
	}
	if requestCount < scaleWindowWidth+1 {
		return scaleWindowEvidence{}, errors.New("scale hold lacks five distinct intact response-producing ICRQs")
	}
	if marker {
		if !fifth {
			return scaleWindowEvidence{}, errors.New("marker released ACKs but no queued fifth Ze response progressed")
		}
		proof.released = true
		return proof, nil
	}
	if len(outstanding) != scaleWindowWidth {
		return scaleWindowEvidence{}, errors.New("scale hold did not reach four distinct outstanding Ze controls")
	}
	for offset := range uint16(scaleWindowWidth) {
		if _, exists := outstanding[baseline+offset]; !exists {
			return scaleWindowEvidence{}, errors.New("held outstanding Ze controls do not start at the ACK baseline")
		}
	}
	return proof, nil
}

// requireScaleReliable consumes the exact unwrapped handleReliable object.
// Pointer fields distinguish a missing or null statistic from a real zero.
func requireScaleReliable(raw []byte, held scaleWindowEvidence) error {
	if held.tunnel == 0 {
		return errors.New("reliable evidence has no validated control-window hold")
	}
	if held.released {
		return errors.New("reliable hold fence was sampled after release")
	}
	if len(raw) > 4096 {
		return errors.New("reliable state exceeds its bounded object size")
	}
	var state struct {
		Tunnel      *uint16 `json:"tunnel-id"`
		Ns          *uint16 `json:"ns"`
		Nr          *uint16 `json:"nr"`
		PeerNr      *uint16 `json:"peer-nr"`
		Outstanding *int    `json:"outstanding"`
		Retransmits *int    `json:"retransmit-count"`
		CWND        *uint16 `json:"cwnd"`
		Threshold   *uint16 `json:"ssthresh"`
		RWS         *uint16 `json:"peer-rws"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return fmt.Errorf("decode actual reliable state: %w", err)
	}
	if state.Tunnel == nil {
		return errors.New("reliable state lacks tunnel-id")
	}
	if state.Ns == nil {
		return errors.New("reliable state lacks ns")
	}
	if state.Nr == nil {
		return errors.New("reliable state lacks nr")
	}
	if state.PeerNr == nil {
		return errors.New("reliable state lacks peer-nr")
	}
	if state.Outstanding == nil {
		return errors.New("reliable state lacks outstanding")
	}
	if state.Retransmits == nil {
		return errors.New("reliable state lacks retransmit-count")
	}
	if state.CWND == nil {
		return errors.New("reliable state lacks cwnd")
	}
	if state.Threshold == nil {
		return errors.New("reliable state lacks ssthresh")
	}
	if state.RWS == nil {
		return errors.New("reliable state lacks peer-rws")
	}
	if *state.Tunnel != held.tunnel {
		return errors.New("reliable state belongs to another tunnel")
	}
	if *state.Ns != held.baseline+scaleWindowWidth {
		return errors.New("reliable ns does not bound exactly four held Ze controls")
	}
	if *state.PeerNr != held.baseline {
		return errors.New("reliable peer-nr advanced beyond the held ACK baseline")
	}
	if *state.Outstanding != scaleWindowWidth {
		return errors.New("reliable state does not have four outstanding controls")
	}
	if *state.CWND != scaleWindowWidth {
		return errors.New("reliable CWND is not warmed to four")
	}
	if *state.RWS != scaleWindowWidth {
		return errors.New("reliable peer-rws is not the omitted-AVP default of four")
	}
	if *state.Retransmits < 0 {
		return errors.New("reliable retransmit count is negative")
	}
	if *state.Threshold == 0 {
		return errors.New("reliable slow-start threshold is zero")
	}
	if *state.Nr != held.expectedNr {
		if !scaleSequenceBefore(held.expectedNr, *state.Nr) {
			return errors.New("reliable nr has not processed the fifth held ICRQ")
		}
	}
	return nil
}

// requireScaleWireSessions joins one relay's local tunnel and sixteen actual
// ICRP identities to the full live population; a LAC sid is never a Ze sid.
func requireScaleWireSessions(log string, population scalePopulation) error {
	trace, err := readScaleTrace(log)
	if err != nil {
		return err
	}
	requests := make(map[uint16]bool, scaleWireSessions)
	replies := make(map[uint16]uint16, scaleWireSessions)
	locals := make(map[uint16]bool, scaleWireSessions)
	for index := range trace.events {
		event := &trace.events[index]
		if !event.control {
			continue
		}
		if !event.fresh {
			continue
		}
		if event.lac {
			if event.packet.message != 10 {
				continue
			}
			if event.packet.session != 0 {
				return errors.New("ICRQ header Session ID is not zero")
			}
			remote, ok := event.packet.attributes[14]
			if !ok {
				return errors.New("ICRQ lacks its LAC Assigned Session ID")
			}
			if remote == 0 {
				return errors.New("ICRQ assigns zero LAC session ID")
			}
			if requests[remote] {
				return errors.New("distinct ICRQs share one LAC session ID")
			}
			requests[remote] = true
			continue
		}
		if event.packet.message != 11 {
			continue
		}
		if !requests[event.packet.session] {
			return errors.New("ICRP header Session ID has no preceding LAC ICRQ")
		}
		local, ok := event.packet.attributes[14]
		if !ok {
			return errors.New("ICRP lacks Ze Assigned Session ID")
		}
		if local == 0 {
			return errors.New("ICRP assigns zero Ze session ID")
		}
		if locals[local] {
			return errors.New("ICRPs assign one Ze session ID to different calls")
		}
		if _, exists := replies[event.packet.session]; exists {
			return errors.New("distinct ICRPs answer the same LAC session")
		}
		locals[local], replies[event.packet.session] = true, local
	}
	if len(requests) != scaleWireSessions {
		return fmt.Errorf("relay has %d ICRQs, want sixteen", len(requests))
	}
	if len(replies) != scaleWireSessions {
		return fmt.Errorf("relay has %d ICRPs, want sixteen", len(replies))
	}
	var count int
	for _, session := range population.byUser {
		if session.tunnel != trace.local {
			continue
		}
		count++
		if replies[session.remote] != session.local {
			return fmt.Errorf("wire/live identity differs for tunnel %d Ze SID %d LAC SID %d", session.tunnel, session.local, session.remote)
		}
	}
	if count != scaleWireSessions {
		return fmt.Errorf("live tunnel %d has %d sessions, want sixteen", trace.local, count)
	}
	return nil
}

// requireScalePeerCDN accepts an isolated tail, because the caller selects the
// target's own relay after stopping its exact pppd. An absent Assigned Session
// ID is permitted; when present it MUST identify that same LAC session.
func requireScalePeerCDN(log string, target scaleSession) error {
	events, err := readScaleWire(log)
	if err != nil {
		return err
	}
	for index := range events {
		event := &events[index]
		if !event.control {
			continue
		}
		if !event.lac {
			continue
		}
		if event.packet.message != 14 {
			continue
		}
		if event.packet.tunnel != target.tunnel {
			continue
		}
		if event.packet.session != target.local {
			continue
		}
		if remote, present := event.packet.attributes[14]; present {
			if remote != target.remote {
				return errors.New("peer CDN Assigned Session ID identifies another LAC session")
			}
		}
		return nil
	}
	return fmt.Errorf("no peer CDN for exact tunnel %d Ze SID %d LAC SID %d", target.tunnel, target.local, target.remote)
}

// The capture is bounded below half the sequence space. Equality and the
// ambiguous half-range are not forward progress, including at uint16 wrap.
// RFC 2661 Section 5.8 -- see rfc/short/rfc2661.md.
func scaleSequenceBefore(left, right uint16) bool {
	distance := right - left
	if distance == 0 {
		return false
	}
	return distance < 0x8000
}

// scaleWireControl and scaleWireEvent are private validated decoding results.
// Their zero values are not decoded packets. They borrow immutable bytes from
// readScaleWire and are owned by one checker invocation, with no shared state.
type scaleWireControl struct {
	tunnel, session, ns, nr, message uint16
	attributes                       map[uint16]uint16
	zlb                              bool
}

type scaleWireEvent struct {
	original, forwarded []byte
	packet, sent        scaleWireControl
	control, lac, fresh bool
	release             string
}

type scaleWireTrace struct {
	events        []scaleWireEvent
	local, remote uint16
}

// readScaleWire checks every row, including transparency of normal data. The
// caller owns a bounded snapshot, so parser work cannot grow with a live stream.
func readScaleWire(log string) ([]scaleWireEvent, error) {
	if len(log) > scaleWireBytesMax {
		return nil, errors.New("scale capture exceeds its 32 MiB bound")
	}
	events := make([]scaleWireEvent, 0, min(strings.Count(log, "\n")+1, scaleWireControlsMax))
	controls := 0
	for line := range strings.SplitSeq(log, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(events) == scaleWireRowsMax {
			return nil, errors.New("scale capture exceeds its row bound")
		}
		var row struct {
			Direction string `json:"direction"`
			Original  string `json:"original"`
			Forwarded string `json:"forwarded"`
			Release   string `json:"scale_release"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("decode scale capture row: %w", err)
		}
		if row.Direction != relayFromLAC {
			if row.Direction != "ze" {
				return nil, fmt.Errorf("scale capture has unknown direction %q", row.Direction)
			}
		}
		if len(row.Original) > 2*65535 {
			return nil, errors.New("scale original datagram exceeds the L2TP length bound")
		}
		if len(row.Forwarded) != len(row.Original) {
			return nil, errors.New("scale relay changed datagram length")
		}
		original, err := hex.DecodeString(row.Original)
		if err != nil {
			return nil, fmt.Errorf("decode scale original bytes: %w", err)
		}
		forwarded, err := hex.DecodeString(row.Forwarded)
		if err != nil {
			return nil, fmt.Errorf("decode scale forwarded bytes: %w", err)
		}
		if len(original) < 2 {
			return nil, errors.New("scale capture lacks L2TP flags")
		}
		event := scaleWireEvent{original: original, forwarded: forwarded, lac: row.Direction == relayFromLAC, release: row.Release}
		if original[0]&0x80 == 0 {
			if row.Release != "" {
				return nil, errors.New("scale release labels a data datagram")
			}
			if !bytes.Equal(original, forwarded) {
				return nil, errors.New("scale relay modified a data datagram")
			}
			events = append(events, event)
			continue
		}
		controls++
		if controls > scaleWireControlsMax {
			return nil, errors.New("scale capture exceeds its control-message bound")
		}
		event.packet, err = readScaleWireControl(original)
		if err != nil {
			return nil, err
		}
		event.sent, err = readScaleWireControl(forwarded)
		if err != nil {
			return nil, err
		}
		event.control = true
		if !bytes.Equal(original[:10], forwarded[:10]) {
			return nil, errors.New("scale relay modified control bytes outside Nr")
		}
		if !bytes.Equal(original[12:], forwarded[12:]) {
			return nil, errors.New("scale relay modified control AVPs")
		}
		if !event.lac {
			if !bytes.Equal(original, forwarded) {
				return nil, errors.New("scale relay modified Ze control Nr")
			}
			if row.Release != "" {
				return nil, errors.New("scale release labels a Ze control")
			}
		}
		if row.Release != "" {
			if row.Release != "marker" {
				if row.Release != "deadline" {
					return nil, fmt.Errorf("unknown scale release provenance %q", row.Release)
				}
			}
		}
		events = append(events, event)
	}
	if len(events) == 0 {
		return nil, errors.New("scale capture is empty")
	}
	return events, nil
}

// readScaleWireControl validates all headers and AVP lengths independently of
// the relay decoder. Only identity/window AVPs are projected into scalar fields.
// RFC 2661 Sections 3.1 and 4.4.1 -- see rfc/short/rfc2661.md.
func readScaleWireControl(packet []byte) (scaleWireControl, error) {
	if len(packet) < 12 {
		return scaleWireControl{}, errors.New("scale control header is truncated")
	}
	if len(packet) > 65535 {
		return scaleWireControl{}, errors.New("scale control exceeds its length field")
	}
	if binary.BigEndian.Uint16(packet[:2]) != 0xc802 {
		return scaleWireControl{}, errors.New("scale control flags/version are invalid")
	}
	if int(binary.BigEndian.Uint16(packet[2:4])) != len(packet) {
		return scaleWireControl{}, errors.New("scale control declared length differs from its datagram")
	}
	control := scaleWireControl{
		tunnel: binary.BigEndian.Uint16(packet[4:6]), session: binary.BigEndian.Uint16(packet[6:8]),
		ns: binary.BigEndian.Uint16(packet[8:10]), nr: binary.BigEndian.Uint16(packet[10:12]),
		zlb: len(packet) == 12, attributes: make(map[uint16]uint16, 4),
	}
	if control.zlb {
		return control, nil
	}
	for offset := 12; offset < len(packet); {
		if len(packet)-offset < 6 {
			return scaleWireControl{}, errors.New("scale control AVP header is truncated")
		}
		avp := packet[offset:]
		flags := binary.BigEndian.Uint16(avp[:2])
		length := int(flags & 0x03ff)
		if length < 6 {
			return scaleWireControl{}, errors.New("scale control AVP length is below its header")
		}
		if length > len(avp) {
			return scaleWireControl{}, errors.New("scale control AVP value is truncated")
		}
		if flags&0x3c00 != 0 {
			return scaleWireControl{}, errors.New("scale control AVP reserved flags are set")
		}
		vendor, attribute := binary.BigEndian.Uint16(avp[2:4]), binary.BigEndian.Uint16(avp[4:6])
		if offset == 12 {
			if vendor != 0 {
				return scaleWireControl{}, errors.New("scale first AVP has a non-IETF vendor")
			}
			if attribute != 0 {
				return scaleWireControl{}, errors.New("scale first AVP is not Message Type")
			}
			if flags != 0x8008 {
				return scaleWireControl{}, errors.New("scale Message Type flags/length are invalid")
			}
			control.message = binary.BigEndian.Uint16(avp[6:8])
		}
		if vendor == 0 {
			// This projection reads only Message Type and the identity/window
			// AVPs; all other AVPs still receive the full bounds checks above.
			switch attribute {
			case 0, 9, 10, 14:
				if length != 8 {
					return scaleWireControl{}, errors.New("scale identity/window AVP has invalid length")
				}
				if flags&0x4000 != 0 {
					return scaleWireControl{}, errors.New("scale identity/window AVP is hidden")
				}
				if _, exists := control.attributes[attribute]; exists {
					return scaleWireControl{}, errors.New("scale control repeats an identity/window AVP")
				}
				control.attributes[attribute] = binary.BigEndian.Uint16(avp[6:8])
			default:
				// The wire attribute catalog is open; uninterpreted AVPs
				// remain intact and bounded instead of receiving defaults.
			}
		}
		offset += length
	}
	return control, nil
}

// readScaleTrace requires startup and every distinct Ns from zero. Retransmit
// bodies must remain identical, while their cumulative Nr may advance.
func readScaleTrace(log string) (scaleWireTrace, error) {
	events, err := readScaleWire(log)
	if err != nil {
		return scaleWireTrace{}, err
	}
	trace := scaleWireTrace{events: events}
	seen := [2]map[uint16][]byte{make(map[uint16][]byte), make(map[uint16][]byte)}
	var next [2]uint16
	var startup int
	for index := range trace.events {
		event := &trace.events[index]
		if !event.control {
			continue
		}
		direction := 0
		if event.lac {
			direction = 1
		}
		packet := &event.packet
		if packet.nr != next[1-direction] {
			if !scaleSequenceBefore(packet.nr, next[1-direction]) {
				return scaleWireTrace{}, errors.New("scale Nr acknowledges an uncaptured control message")
			}
		}
		if packet.zlb {
			if startup == 3 {
				if event.lac {
					if packet.tunnel != trace.local {
						return scaleWireTrace{}, errors.New("LAC ZLB addresses another tunnel")
					}
				} else if packet.tunnel != trace.remote {
					return scaleWireTrace{}, errors.New("received Ze ZLB addresses another tunnel")
				}
			}
			if packet.session != 0 {
				return scaleWireTrace{}, errors.New("ZLB has a nonzero Session ID")
			}
			continue
		}
		if packet.ns == next[direction] {
			event.fresh = true
			seen[direction][packet.ns] = event.original
			next[direction]++
		} else {
			previous, exists := seen[direction][packet.ns]
			if !exists {
				return scaleWireTrace{}, errors.New("scale capture has a missing or out-of-order distinct Ns")
			}
			if !bytes.Equal(previous[:10], event.original[:10]) {
				return scaleWireTrace{}, errors.New("scale retransmit changed its control header")
			}
			if !bytes.Equal(previous[12:], event.original[12:]) {
				return scaleWireTrace{}, errors.New("scale retransmit changed its AVPs")
			}
		}
		if !event.fresh {
			continue
		}
		if startup < 3 {
			if err := trace.startup(packet, event.lac, startup); err != nil {
				return scaleWireTrace{}, err
			}
			startup++
			continue
		}
		if event.lac {
			if packet.tunnel != trace.local {
				return scaleWireTrace{}, errors.New("LAC control destination is not Ze's assigned tunnel ID")
			}
		} else if packet.tunnel != trace.remote {
			return scaleWireTrace{}, errors.New("received Ze control destination is not the LAC's assigned tunnel ID")
		}
	}
	if startup != 3 {
		return scaleWireTrace{}, errors.New("scale capture lacks the complete SCCRQ/SCCRP/SCCCN startup")
	}
	return trace, nil
}

func (trace *scaleWireTrace) startup(packet *scaleWireControl, lac bool, index int) error {
	if packet.session != 0 {
		return errors.New("scale startup control has a nonzero Session ID")
	}
	if index == 0 {
		if !lac {
			return errors.New("scale capture does not begin with the LAC SCCRQ")
		}
		if packet.message != 1 {
			return errors.New("scale capture lacks initial SCCRQ")
		}
		if packet.tunnel != 0 {
			return errors.New("initial SCCRQ destination Tunnel ID is nonzero")
		}
		if packet.nr != 0 {
			return errors.New("initial SCCRQ Nr does not start at zero")
		}
		if _, present := packet.attributes[10]; present {
			return errors.New("SCCRQ explicitly advertises a receive window instead of omitting it")
		}
		trace.remote = packet.attributes[9]
		if trace.remote == 0 {
			return errors.New("SCCRQ lacks nonzero LAC Assigned Tunnel ID")
		}
		return nil
	}
	if index == 1 {
		if lac {
			return errors.New("scale startup lacks Ze's SCCRP")
		}
		if packet.message != 2 {
			return errors.New("scale startup second message is not SCCRP")
		}
		if packet.tunnel != trace.remote {
			return errors.New("SCCRP does not address the LAC Assigned Tunnel ID")
		}
		trace.local = packet.attributes[9]
		if trace.local == 0 {
			return errors.New("SCCRP lacks nonzero Ze Assigned Tunnel ID")
		}
		return nil
	}
	if !lac {
		return errors.New("scale startup lacks the LAC SCCCN")
	}
	if packet.message != 3 {
		return errors.New("scale startup third message is not SCCCN")
	}
	if packet.tunnel != trace.local {
		return errors.New("SCCCN does not address Ze's assigned tunnel ID")
	}
	return nil
}

// scaleWindowArm derives the arming packet retrospectively. That real ACK is
// forwarded before holding begins, and an ICRQ on it belongs to the hold too.
func scaleWindowArm(trace scaleWireTrace) (int, uint16, error) {
	mutation := -1
	var baseline uint16
	for index := range trace.events {
		event := &trace.events[index]
		if !event.control {
			continue
		}
		if event.packet.nr != event.sent.nr {
			mutation, baseline = index, event.sent.nr
			break
		}
	}
	if mutation == -1 {
		return 0, 0, errors.New("scale capture never exercised an Nr hold")
	}
	pending := make(map[uint16]bool, scaleWindowWidth)
	acknowledged := 0
	for index := range mutation {
		event := &trace.events[index]
		if !event.control {
			continue
		}
		if !event.lac {
			if event.fresh {
				pending[event.packet.ns] = true
			}
			continue
		}
		for ns := range pending {
			if scaleSequenceBefore(ns, event.packet.nr) {
				delete(pending, ns)
				acknowledged++
			}
		}
		if event.packet.nr == baseline {
			if acknowledged >= scaleWindowWidth-1 {
				return index, baseline, nil
			}
		}
	}
	return 0, 0, errors.New("scale hold lacks three genuine warm-up ACKs to its baseline")
}
