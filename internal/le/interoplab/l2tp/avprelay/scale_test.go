// Design: docs/labs/l2tp-interop.md -- a saturated default L2TP control window.
// Related: scale.go -- genuine cumulative-ACK hold and replay.
package main

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

// TestScaleWindowHold proves that three genuine ACKs arm a four-message
// window, only Nr changes, and a release replays a genuine control verbatim.
func TestScaleWindowHold(t *testing.T) {
	window := newScaleWindow(0, 0)
	now := time.Unix(100, 0)
	scaleWarmWindow(t, window, now, 0, 0)
	for offset := range uint16(4) {
		packet := scaleTestControl(11, 3+offset, 5+offset)
		got := scaleForward(t, window, packet, "ze", now)
		if !bytes.Equal(got, packet) {
			t.Fatal("holding acknowledgements changed Ze control")
		}
	}
	held := scaleTestControl(10, 5, 7)
	before := bytes.Clone(held)
	got := scaleForward(t, window, held, directionLAC, now)
	if binary.BigEndian.Uint16(got[10:12]) != 3 {
		t.Fatal("Nr did not remain at the genuine warm-up baseline")
	}
	binary.BigEndian.PutUint16(before[10:12], 3)
	if !bytes.Equal(got, before) {
		t.Fatal("hold changed Ns, AVPs, tunnel or session identity")
	}
	replay, present, err := window.release(now.Add(time.Millisecond), true)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("marker release lost the real cumulative ACK")
	}
	if replay.reason != scaleReleaseMarker {
		t.Fatal("marker release did not retain its provenance")
	}
	if !bytes.Equal(replay.packet, scaleTestControl(10, 5, 7)) {
		t.Fatal("release fabricated or changed the genuine held control")
	}
	next := scaleTestControl(10, 6, 7)
	if got := scaleForward(t, window, next, directionLAC, now); !bytes.Equal(got, next) {
		t.Fatal("release did not restore transparent forwarding")
	}
}

// TestScaleWindowWarmupCountsACKs excludes duplicate cumulative ACKs from
// warm-up, so one real acknowledgement cannot arm a four-message window.
func TestScaleWindowWarmupCountsACKs(t *testing.T) {
	window := newScaleWindow(0, 0)
	now := time.Unix(100, 0)
	scaleForward(t, window, scaleTestControl(1, 0, 0), directionLAC, now)
	scaleForward(t, window, scaleTestControl(2, 0, 1), "ze", now)
	ack := scaleTestControl(3, 1, 1)
	for range 4 {
		scaleForward(t, window, ack, directionLAC, now)
	}
	scaleForward(t, window, scaleTestControl(10, 2, 1), directionLAC, now)
	scaleForward(t, window, scaleTestControl(11, 1, 3), "ze", now)
	second := scaleTestControl(10, 3, 2)
	if got := scaleForward(t, window, second, directionLAC, now); !bytes.Equal(got, second) {
		t.Fatal("duplicate ACKs armed the hold before three genuine acknowledgements")
	}
}

// TestScaleWindowDuplicateAndZLB excludes retransmissions and pure ACKs
// from the distinct outstanding count, but refuses a fifth unique control.
func TestScaleWindowDuplicateAndZLB(t *testing.T) {
	window := newScaleWindow(0, 0)
	now := time.Unix(100, 0)
	scaleWarmWindow(t, window, now, 0, 0)
	for offset := range uint16(4) {
		packet := scaleTestControl(11, 3+offset, 5)
		scaleForward(t, window, packet, "ze", now)
		scaleForward(t, window, packet, "ze", now)
		zlb := scaleTestControl(0, 4+offset, 5)
		scaleForward(t, window, zlb, "ze", now)
	}
	if _, err := window.forwardControl(scaleTestControl(11, 7, 5), "ze", now); err == nil {
		t.Fatal("a fifth distinct outstanding control passed the four-message cap")
	}
}

// TestScaleWindowWrap crosses the 16-bit boundary in both Ns and cumulative
// Nr without treating a wrapped ACK as old or counting a duplicate twice.
func TestScaleWindowWrap(t *testing.T) {
	window := newScaleWindow(65534, 65530)
	now := time.Unix(100, 0)
	scaleWarmWindow(t, window, now, 65534, 65530)
	for offset := range uint16(4) {
		scaleForward(t, window, scaleTestControl(11, 1+offset, 65535), "ze", now)
	}
	packet := scaleTestControl(10, 65535, 5)
	got := scaleForward(t, window, packet, directionLAC, now)
	if binary.BigEndian.Uint16(got[10:12]) != 1 {
		t.Fatal("wrapped Nr did not remain at the genuine baseline")
	}
}

// TestScaleWindowDeadline proves an idle socket can observe the marker,
// automatic release is distinguishable, and absence is an explicit guard.
func TestScaleWindowDeadline(t *testing.T) {
	window := newScaleWindow(0, 0)
	now := time.Unix(100, 0)
	absolute := now.Add(5 * time.Minute)
	if got := window.readDeadline(now, absolute); !got.Equal(absolute) {
		t.Fatal("warm-up introduced idle polling into the normal deadline")
	}
	_, presentBefore, errBefore := window.release(now, false)
	if errBefore != nil {
		t.Fatal(errBefore)
	}
	if presentBefore {
		t.Fatal("unarmed idle check produced a replay")
	}
	scaleWarmWindow(t, window, now, 0, 0)
	deadline := window.readDeadline(now, absolute)
	if deadline.After(now.Add(30 * time.Millisecond)) {
		t.Fatal("held socket cannot observe an idle release marker promptly")
	}
	scaleForward(t, window, scaleTestControl(11, 3, 5), "ze", now)
	scaleForward(t, window, scaleTestControl(10, 5, 4), directionLAC, now)
	replay, present, err := window.release(now.Add(5*time.Second), false)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("deadline failed to release the genuine held ACK")
	}
	if replay.reason != scaleReleaseDeadline {
		t.Fatal("deadline release could be mistaken for checker approval")
	}
}

// TestScaleWindowReleaseAbsent checks valid no-replay outcomes through an
// unfinished hold, an idle deadline release and the completed lifecycle.
func TestScaleWindowReleaseAbsent(t *testing.T) {
	window := newScaleWindow(0, 0)
	now := time.Unix(100, 0)
	scaleWarmWindow(t, window, now, 0, 0)
	for _, check := range []struct {
		name      string
		elapsed   time.Duration
		requested bool
	}{
		{"unfinished", time.Millisecond, false},
		{"idle-deadline", scaleHoldLimit, false},
		{"completed", scaleHoldLimit + time.Millisecond, true},
	} {
		t.Run(check.name, func(t *testing.T) {
			_, present, err := window.release(now.Add(check.elapsed), check.requested)
			if err != nil {
				t.Fatal(err)
			}
			if present {
				t.Fatal("no-action release produced a replay")
			}
		})
	}
	absolute := now.Add(5 * time.Minute)
	if got := window.readDeadline(now, absolute); !got.Equal(absolute) {
		t.Fatal("idle deadline release retained hold polling")
	}
	packet := scaleTestControl(10, 5, 3)
	if got := scaleForward(t, window, packet, directionLAC, now); !bytes.Equal(got, packet) {
		t.Fatal("idle deadline release did not restore transparent forwarding")
	}
}

// TestScaleWindowReleaseWithoutACK refuses marker approval without a genuine
// advancing ACK and explicitly reports that no replay accompanies the error.
func TestScaleWindowReleaseWithoutACK(t *testing.T) {
	window := newScaleWindow(0, 0)
	now := time.Unix(100, 0)
	scaleWarmWindow(t, window, now, 0, 0)
	_, present, err := window.release(now, true)
	if err == nil {
		t.Fatal("marker release accepted without a genuine held ACK")
	}
	if present {
		t.Fatal("refused marker release produced a replay")
	}
}

// TestScaleControlMalformed refuses truncated headers, wrong lengths,
// incomplete AVPs, unknown direction and a gap in the observed Ns sequence.
func TestScaleControlMalformed(t *testing.T) {
	for _, packet := range [][]byte{nil, {0xc8, 2}, {0xc8, 2, 0, 12}, scaleTestControl(10, 1, 0)} {
		window := newScaleWindow(0, 0)
		if _, err := window.forwardControl(packet, directionLAC, time.Unix(100, 0)); err == nil {
			t.Fatalf("malformed or incomplete trace passed: %x", packet)
		}
	}
	packet := scaleTestControl(1, 0, 0)
	packet[13] = 7
	if _, err := newScaleWindow(0, 0).forwardControl(packet, directionLAC, time.Unix(100, 0)); err == nil {
		t.Fatal("truncated Message-Type AVP passed")
	}
	if _, err := newScaleWindow(0, 0).forwardControl(scaleTestControl(1, 0, 0), "unknown", time.Unix(100, 0)); err == nil {
		t.Fatal("unknown direction passed")
	}
}

// TestRelayModeSelection keeps the old cap and rejects unknown private modes
// before readiness, without opening a socket or invoking feature callbacks.
func TestRelayModeSelection(t *testing.T) {
	transport := relayIO{deadline: time.Unix(100, 0).Add(5 * time.Minute)}
	ordinary, err := selectRelayMode("", &transport)
	if err != nil {
		t.Fatal(err)
	}
	if ordinary.controlLimit != 256 {
		t.Fatal("scale changed the established modes' control cap")
	}
	scale, err := selectRelayMode("scale", &transport)
	if err != nil {
		t.Fatal(err)
	}
	if scale.controlLimit <= ordinary.controlLimit {
		t.Fatal("scale retained the single-session control cap")
	}
	if _, err := selectRelayMode("unknown", &transport); err == nil {
		t.Fatal("unknown mode silently selected a protocol policy")
	}
}

func scaleWarmWindow(t *testing.T, window *scaleWindow, now time.Time, ze, lac uint16) {
	t.Helper()
	scaleForward(t, window, scaleTestControl(1, lac, ze), directionLAC, now)
	scaleForward(t, window, scaleTestControl(2, ze, lac+1), "ze", now)
	scaleForward(t, window, scaleTestControl(3, lac+1, ze+1), directionLAC, now)
	scaleForward(t, window, scaleTestControl(10, lac+2, ze+1), directionLAC, now)
	scaleForward(t, window, scaleTestControl(11, ze+1, lac+3), "ze", now)
	scaleForward(t, window, scaleTestControl(10, lac+3, ze+2), directionLAC, now)
	scaleForward(t, window, scaleTestControl(11, ze+2, lac+4), "ze", now)
	scaleForward(t, window, scaleTestControl(10, lac+4, ze+3), directionLAC, now)
}

func scaleForward(t *testing.T, window *scaleWindow, packet []byte, direction string, now time.Time) []byte {
	t.Helper()
	got, err := window.forwardControl(packet, direction, now)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func scaleTestControl(message, ns, nr uint16) []byte {
	length := 20
	if message == 0 {
		length = 12
	}
	packet := make([]byte, length)
	for index, value := range []uint16{0xc802, uint16(length), 19, 0, ns, nr} {
		binary.BigEndian.PutUint16(packet[index*2:], value)
	}
	if message != 0 {
		for index, value := range []uint16{0x8008, 0, 0, message} {
			binary.BigEndian.PutUint16(packet[12+index*2:], value)
		}
	}
	return packet
}

// FuzzScaleControl keeps the peer-controlled parser non-mutating and safe for
// arbitrary lengths; successful ZLB classification must have no payload.
func FuzzScaleControl(f *testing.F) {
	f.Add(scaleTestControl(10, 0, 0))
	f.Add(scaleTestControl(0, 65535, 0))
	f.Add([]byte{0xc8, 2})
	f.Fuzz(func(t *testing.T, packet []byte) {
		before := bytes.Clone(packet)
		// RFC 2661 Sections 3.1 and 4.4.1.
		control, err := readScaleControl(packet)
		if !bytes.Equal(packet, before) {
			t.Fatal("control parser modified its input")
		}
		if err != nil {
			return
		}
		if control.zlb {
			if len(packet) != 12 {
				t.Fatal("a successful ZLB decode retained payload")
			}
		}
	})
}
