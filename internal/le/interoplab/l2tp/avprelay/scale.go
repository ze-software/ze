// Design: docs/labs/l2tp-interop.md -- the exercised default control window.
// Related: relaymode.go -- private mode registration and the shared socket loop.
// RFC 2661 Sections 3.1, 4.4.1, 5.8 and Appendix A -- see rfc/short/rfc2661.md.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"time"
)

const scaleReleaseMarkerPath = "/run/scale-release"
const scaleControlLimit = 4096
const scaleWindowSize = 4
const scaleHoldLimit = 5 * time.Second
const scaleReadInterval = 20 * time.Millisecond

func init() {
	registerRelayMode("scale", newScaleRelayMode)
}

func newScaleRelayMode(transport *relayIO) relayMode {
	window := newScaleWindow(0, 0)
	return relayMode{
		controlLimit: scaleControlLimit,
		control: func(packet []byte, direction string, target netip.AddrPort) error {
			original := hex.EncodeToString(packet)
			// RFC 2661 Section 5.8 and Appendix A.
			forwarded, err := window.forwardControl(packet, direction, time.Now())
			if err != nil {
				return err
			}
			if _, err := transport.conn.WriteToUDPAddrPort(forwarded, target); err != nil {
				return err
			}
			return transport.encoder.Encode(wireObservation{direction, original, hex.EncodeToString(forwarded)})
		},
		data: func(packet []byte, direction string, target netip.AddrPort) error {
			if _, err := transport.conn.WriteToUDPAddrPort(packet, target); err != nil {
				return err
			}
			encoded := hex.EncodeToString(packet)
			return transport.encoder.Encode(wireObservation{direction, encoded, encoded})
		},
		beforeRead: func(now time.Time) (time.Time, error) {
			requested, err := markerExists(scaleReleaseMarkerPath)
			if err != nil {
				return time.Time{}, err
			}
			replay, present, err := window.release(now, requested)
			if err != nil {
				return time.Time{}, err
			}
			// No replay is the explicit guard for warm-up, an unfinished
			// hold, or an already released hold; no control is fabricated.
			if present {
				if _, err := transport.conn.WriteToUDPAddrPort(replay.packet, transport.ze); err != nil {
					return time.Time{}, err
				}
				encoded := hex.EncodeToString(replay.packet)
				event := scaleReleaseObservation{
					wireObservation{directionLAC, encoded, encoded}, replay.reason.String(),
				}
				if err := transport.encoder.Encode(event); err != nil {
					return time.Time{}, err
				}
			}
			return window.readDeadline(now, transport.deadline), nil
		},
	}
}

type scaleReleaseObservation struct {
	wireObservation
	ScaleRelease string `json:"scale_release"`
}

type scaleRelease uint8

const (
	scaleReleaseUnspecified scaleRelease = iota
	scaleReleaseMarker
	scaleReleaseDeadline
)

func (reason scaleRelease) String() string {
	switch reason {
	case scaleReleaseUnspecified:
		return "unspecified"
	case scaleReleaseMarker:
		return "marker"
	case scaleReleaseDeadline:
		return "deadline"
	default:
		panic("BUG: invalid scale release reason")
	}
}

// scaleWindow is owned by one relay loop and is not safe for concurrent use.
// Its fixed four slots bound distinct outstanding controls; a duplicate and a
// ZLB cannot add a slot. Pending metadata is ordered by Ns, while zeNext marks
// the first uncaptured predecessor; packet arrival order is never changed.
// Only the hold retains one genuine packet beyond the common read buffer's
// lifetime, so that one copy has an independent owner.
type scaleWindow struct {
	zeNext  uint16
	lacNext uint16
	peerNr  uint16
	pending [scaleWindowSize]uint16
	count   int
	stage   scaleStage
}

type scaleStage interface{ scaleStage() }

type scaleWarmup struct{ cwnd, threshold, counter uint16 }
type scaleHold struct {
	baseline uint16
	until    time.Time
	packet   []byte
	nr       uint16
}
type scaleFinished struct{}

func (*scaleWarmup) scaleStage()  {}
func (*scaleHold) scaleStage()    {}
func (scaleFinished) scaleStage() {}

// newScaleWindow seeds the observed sequence space; runtime callers MUST use
// zero for both peers. Explicit seeds let the wrap test cross modulo 65536.
// RFC 2661 Section 5.8: "The message sequence number, Ns, begins at 0."
func newScaleWindow(ze, lac uint16) *scaleWindow {
	return &scaleWindow{
		zeNext: ze, lacNext: lac, peerNr: ze,
		stage: &scaleWarmup{cwnd: 1, threshold: scaleWindowSize},
	}
}

// forwardControl preserves every byte except held LAC Nr. It observes the
// real ACK before arming, so that ACK actually reaches Ze and warms its CWND.
// RFC 2661 Section 5.8: "All control messages take up one slot in the control message sequence number space, except the ZLB acknowledgement."
func (window *scaleWindow) forwardControl(packet []byte, direction string, now time.Time) ([]byte, error) {
	// RFC 2661 Sections 3.1 and 4.4.1.
	control, err := readScaleControl(packet)
	if err != nil {
		return nil, err
	}
	switch direction {
	case "ze":
		// RFC 2661 Section 5.8.
		if err := window.observeZe(control); err != nil {
			return nil, err
		}
		return packet, nil
	case directionLAC:
		// RFC 2661 Section 5.8.
		if err := observeScaleNs(&window.lacNext, control); err != nil {
			return nil, err
		}
		if scaleBefore(window.zeNext, control.nr) {
			return nil, errors.New("LAC Nr acknowledges an unobserved Ze control")
		}
		if hold, active := window.stage.(*scaleHold); active {
			if scaleBefore(hold.baseline, control.nr) {
				if !scaleBefore(control.nr, hold.nr) {
					// Deliberate copy: this genuine packet must outlive the
					// reusable read buffer until a later marker or deadline.
					hold.packet = bytes.Clone(packet)
					hold.nr = control.nr
				}
			}
			if control.nr == hold.baseline {
				return packet, nil
			}
			forwarded := bytes.Clone(packet)
			binary.BigEndian.PutUint16(forwarded[10:12], hold.baseline)
			return forwarded, nil
		}
		// RFC 2661 Section 5.8 and Appendix A.
		window.acknowledge(control.nr)
		if warmup, warming := window.stage.(*scaleWarmup); warming {
			if warmup.cwnd == scaleWindowSize {
				window.stage = &scaleHold{baseline: control.nr, nr: control.nr, until: now.Add(scaleHoldLimit)}
			}
		}
		return packet, nil
	default:
		// Direction is an open boundary string from the wire logger.
		return nil, fmt.Errorf("unknown scale relay direction %q", direction)
	}
}

// observeZe counts distinct controls within four sequence positions from the
// genuine peer ACK. Sorting metadata rather than packets permits legal UDP
// reordering while the contiguous zeNext frontier still refuses ACKs over holes.
// A repeated outstanding Ns conservatively models warm-up loss.
// RFC 2661 Section 5.8: "Messages arriving out of order may be queued for in-order delivery when the missing messages are received, or they may be discarded requiring a retransmission by the peer."
// RFC 2661 Section 5.8: "Thus, Ns is not incremented after a ZLB message is sent."
func (window *scaleWindow) observeZe(control scaleControl) error {
	if control.zlb {
		return nil
	}
	for index := range window.count {
		if window.pending[index] == control.ns {
			if warmup, warming := window.stage.(*scaleWarmup); warming {
				// RFC 2661 Appendix A: "When congestion occurs (indicated by the triggering of a retransmission) one half of the CWND is saved in SSTHRESH, and CWND is set to one."
				// A wire batch cannot identify Tick boundaries; resetting
				// for each repeat can only underestimate warm-up, never arm
				// early. The checker still requires actual live CWND four.
				warmup.threshold = max(warmup.cwnd/2, 1)
				warmup.cwnd, warmup.counter = 1, 0
			}
			return nil
		}
	}
	if scaleBefore(control.ns, window.peerNr) {
		return nil
	}
	if window.count == len(window.pending) {
		return errors.New("fifth distinct Ze control exceeds the four-message window")
	}
	if control.ns-window.peerNr >= scaleWindowSize {
		return errors.New("gap in observed Ze control sequence")
	}
	index := window.count
	for index > 0 {
		if !scaleBefore(control.ns, window.pending[index-1]) {
			break
		}
		window.pending[index] = window.pending[index-1]
		index--
	}
	window.pending[index] = control.ns
	window.count++
	for index := range window.count {
		if window.pending[index] == window.zeNext {
			window.zeNext++
		}
	}
	return nil
}

// acknowledge advances only over actual captured Ze controls, not an Nr delta
// guessed from duplicate ACKs or ZLB Ns.
// RFC 2661 Appendix A: "During slow start, CWND is increased by one packet each time an ACK (explicit ZLB or piggybacked) is received."
func (window *scaleWindow) acknowledge(nr uint16) {
	if !scaleBefore(window.peerNr, nr) {
		return
	}
	window.peerNr = nr
	for range len(window.pending) {
		if window.count == 0 {
			break
		}
		if !scaleBefore(window.pending[0], nr) {
			break
		}
		copy(window.pending[:], window.pending[1:window.count])
		window.count--
		window.pending[window.count] = 0
		if warmup, warming := window.stage.(*scaleWarmup); warming {
			if warmup.cwnd < warmup.threshold {
				warmup.cwnd++
			} else {
				// RFC 2661 Appendix A: "That is, CWND is increased by one packet after CWND new ACKs have been received."
				warmup.counter++
				if warmup.counter >= warmup.cwnd {
					warmup.cwnd++
					warmup.counter = 0
				}
			}
			warmup.cwnd = min(warmup.cwnd, scaleWindowSize)
		}
	}
}

// scaleReplay carries a genuine held packet only when release reports presence.
// Its zero value carries no replay and MUST NOT be consumed without presence.
type scaleReplay struct {
	packet []byte
	reason scaleRelease
}

// release reports presence only when a hold releases a genuine saved packet.
// Callers MUST check presence before consuming the replay's packet or reason.
// A replay is the saved real packet, never a synthetic protocol exchange. Ze
// processes its Nr before classifying duplicate Ns or ZLB in OnReceive.
func (window *scaleWindow) release(now time.Time, requested bool) (scaleReplay, bool, error) {
	hold, active := window.stage.(*scaleHold)
	if !active {
		return scaleReplay{}, false, nil
	}
	reason := scaleReleaseMarker
	if !requested {
		if now.Before(hold.until) {
			return scaleReplay{}, false, nil
		}
		reason = scaleReleaseDeadline
	}
	window.stage = scaleFinished{}
	if len(hold.packet) == 0 {
		if requested {
			return scaleReplay{}, false, errors.New("scale release requested before a genuine advancing LAC ACK was held")
		}
		// The deadline still restores transparency if no advancing ACK
		// ever arrived. Missing replay evidence cannot satisfy the checker.
		return scaleReplay{}, false, nil
	}
	// RFC 2661 Section 5.8.
	window.acknowledge(hold.nr)
	return scaleReplay{hold.packet, reason}, true, nil
}

// readDeadline polls only an active hold. Other lifecycle states retain the
// absolute socket bound, and a quiet hold cannot miss its release marker.
func (window *scaleWindow) readDeadline(now, absolute time.Time) time.Time {
	if hold, active := window.stage.(*scaleHold); active {
		deadline := minTime(now.Add(scaleReadInterval), hold.until)
		return minTime(deadline, absolute)
	}
	return absolute
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}

type scaleControl struct {
	ns, nr uint16
	zlb    bool
}

// readScaleControl reads the complete control datagram before any mutation.
// The fixed header is flags/version at0, length2, tunnel4, session6, Ns8, Nr10;
// AVPs begin at12, each flags/length0, vendor2, attribute4, value6.
// RFC 2661 Section 3.1: "The S bit MUST be set to 1 for control messages."
func readScaleControl(packet []byte) (scaleControl, error) {
	if len(packet) < 12 {
		return scaleControl{}, errors.New("truncated scale control header")
	}
	flags := binary.BigEndian.Uint16(packet[:2])
	if flags&0xc80f != 0xc802 {
		return scaleControl{}, errors.New("scale control lacks required T/L/S/version fields")
	}
	if flags&0x0300 != 0 {
		return scaleControl{}, errors.New("scale control contains data-only Offset or Priority flag")
	}
	if int(binary.BigEndian.Uint16(packet[2:4])) != len(packet) {
		return scaleControl{}, errors.New("scale control declared length differs from datagram")
	}
	control := scaleControl{
		ns: binary.BigEndian.Uint16(packet[8:10]), nr: binary.BigEndian.Uint16(packet[10:12]),
		zlb: len(packet) == 12,
	}
	if control.zlb {
		return control, nil
	}
	for offset := 12; offset < len(packet); {
		if len(packet)-offset < 6 {
			return scaleControl{}, errors.New("truncated scale control AVP header")
		}
		avp := packet[offset:]
		length := int(binary.BigEndian.Uint16(avp[:2]) & 0x03ff)
		if length < 6 {
			return scaleControl{}, errors.New("scale control AVP length is below its header")
		}
		if length > len(avp) {
			return scaleControl{}, errors.New("truncated scale control AVP value")
		}
		if offset == 12 {
			// RFC 2661 Section 4.4.1: "The Message Type AVP MUST be the first AVP in a message, immediately following the control message header (defined in section 3.1)."
			if length != 8 {
				return scaleControl{}, errors.New("scale control Message Type AVP length is not eight")
			}
			if binary.BigEndian.Uint32(avp[2:6]) != 0 {
				return scaleControl{}, errors.New("scale control does not begin with Message Type AVP")
			}
			if avp[0]&0x40 != 0 {
				return scaleControl{}, errors.New("scale control Message Type AVP is hidden")
			}
		}
		offset += length
	}
	return control, nil
}

// observeScaleNs accepts old retransmissions but refuses a missing capture slot.
// RFC 2661 Section 5.8: "The sequence number is thus a free running counter represented modulo 65536."
func observeScaleNs(next *uint16, control scaleControl) error {
	if control.zlb {
		return nil
	}
	if control.ns == *next {
		*next++
		return nil
	}
	if scaleBefore(control.ns, *next) {
		return nil
	}
	return errors.New("gap in observed LAC control sequence")
}

// scaleBefore compares within the half-range used by the bounded lab capture.
// RFC 2661 Section 5.8: "The sequence number is thus a free running counter represented modulo 65536."
func scaleBefore(left, right uint16) bool {
	distance := right - left
	if distance == 0 {
		return false
	}
	return distance < 0x8000
}
