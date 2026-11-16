// Design: docs/labs/l2tp-interop.md -- independent scale control evidence.
// Related: scale_window.go -- actual wire and daemon reliable-state predicates.
package l2tp

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strings"
	"testing"
)

// TestScaleWindowEvidence proves the entire hold/release sequence from actual
// bytes, including the arming ICRQ before the first Nr mutation.
func TestScaleWindowEvidence(t *testing.T) {
	events := scaleWindowTestHeld()
	held, err := readScaleWindow(scaleWindowTestLog(t, events))
	if err != nil {
		t.Fatal(err)
	}
	if held.tunnel != 19 {
		t.Fatalf("tunnel %d, want Ze-local19", held.tunnel)
	}
	if held.released {
		t.Fatal("unreleased trace claimed checker-marker progress")
	}
	complete := scaleWindowTestRelease(events, "marker")
	got, err := readScaleWindow(scaleWindowTestLog(t, complete))
	if err != nil {
		t.Fatal(err)
	}
	if !got.released {
		t.Fatal("genuine marker ACK did not release the queued fifth response")
	}
	for _, tc := range []struct {
		name   string
		events []scaleWindowTestEvent
	}{
		{"truncated startup", events[1:]},
		{"fewer than five held requests", events[:len(events)-2]},
		{"fifth response before release", append(cloneScaleWindowEvents(events), scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 107, 7, 10, 14, 7)))},
		{"deadline replay", scaleWindowTestRelease(events, "deadline")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := readScaleWindow(scaleWindowTestLog(t, tc.events)); err == nil {
				t.Fatal("incomplete or invalid window proof passed")
			}
		})
	}
	withoutFifth := complete[:len(complete)-1]
	got, err = readScaleWindow(scaleWindowTestLog(t, withoutFifth))
	if err == nil {
		if got.released {
			t.Fatal("marker alone passed without a newly emitted queued fifth response")
		}
	}
	changed := cloneScaleWindowEvents(events)
	packet, err := hex.DecodeString(changed[9].Forwarded)
	if err != nil {
		t.Fatal(err)
	}
	packet[8] ^= 1
	changed[9].Forwarded = hex.EncodeToString(packet)
	if _, err := readScaleWindow(scaleWindowTestLog(t, changed)); err == nil {
		t.Fatal("Nr hold changed Ns without refusal")
	}
	duplicate := append(cloneScaleWindowEvents(events), events[len(events)-3])
	zlb := scaleWindowTestPacket(0, 29, 0, 7, 10, 0, 0)[:12]
	binary.BigEndian.PutUint16(zlb[2:4], 12)
	duplicate = append(duplicate, scaleWindowTestEventFor("ze", zlb))
	if _, err := readScaleWindow(scaleWindowTestLog(t, duplicate)); err != nil {
		t.Fatalf("retransmit/ZLB changed distinct outstanding count: %v", err)
	}
	forged := scaleWindowTestRelease(events, "marker")
	packet, err = hex.DecodeString(forged[len(forged)-2].Original)
	if err != nil {
		t.Fatal(err)
	}
	packet[len(packet)-1] ^= 1
	forged[len(forged)-2].Original = hex.EncodeToString(packet)
	forged[len(forged)-2].Forwarded = hex.EncodeToString(packet)
	if _, err := readScaleWindow(scaleWindowTestLog(t, forged)); err == nil {
		t.Fatal("fabricated control passed as a genuine held ACK replay")
	}
	advertised := cloneScaleWindowEvents(events)
	packet, err = hex.DecodeString(advertised[0].Original)
	if err != nil {
		t.Fatal(err)
	}
	packet = append(packet, 0x80, 8, 0, 0, 0, 10, 0, 4)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	advertised[0] = scaleWindowTestEventFor("lac", packet)
	if _, err := readScaleWindow(scaleWindowTestLog(t, advertised)); err == nil {
		t.Fatal("explicit SCCRQ receive window passed as omitted-default proof")
	}
}

// TestScaleWindowSerialArithmetic fixes modulo ordering at wrap and refuses
// equality and the ambiguous half-range as forward progress.
func TestScaleWindowSerialArithmetic(t *testing.T) {
	for _, tc := range []struct {
		left, right uint16
		before      bool
	}{
		{65535, 0, true}, {65534, 1, true}, {0, 65535, false}, {3, 3, false}, {0, 32768, false},
	} {
		if got := scaleSequenceBefore(tc.left, tc.right); got != tc.before {
			t.Fatalf("%d before %d = %t", tc.left, tc.right, got)
		}
	}
}

// TestScaleWindowPipelinedArm permits genuine replies queued before an ICCN
// warms CWND to four. Five further ICRQs still cross the held processing fence.
func TestScaleWindowPipelinedArm(t *testing.T) {
	events := scaleWindowTestHeld()[:3]
	events = append(events,
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, 2, 1, 14, 101)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, 3, 1, 14, 102)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 101, 1, 4, 14, 1)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 102, 2, 4, 14, 2)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, 4, 2, 14, 103)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 103, 3, 5, 14, 3)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, 5, 2, 14, 104)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(12, 19, 2, 6, 3, 0, 0)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 104, 4, 7, 14, 4)),
	)
	for ns := uint16(7); ns <= 11; ns++ {
		nr := min(ns-2, uint16(7))
		packet := scaleWindowTestPacket(10, 19, 0, ns, nr, 14, 98+ns)
		event := scaleWindowTestEventFor("lac", packet)
		binary.BigEndian.PutUint16(packet[10:12], 3)
		event.Forwarded = hex.EncodeToString(packet)
		events = append(events, event)
		if ns <= 8 {
			events = append(events, scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 98+ns, ns-2, ns+1, 14, ns-2)))
		}
	}
	held, err := readScaleWindow(scaleWindowTestLog(t, events))
	if err != nil {
		t.Fatalf("genuine pipelined hold refused: %v", err)
	}
	if held.released {
		t.Fatal("unreleased pipelined hold claimed progress")
	}
	raw := []byte(`{"tunnel-id":19,"ns":7,"nr":12,"peer-nr":3,"outstanding":4,"retransmit-count":0,"cwnd":4,"ssthresh":4,"peer-rws":4}`)
	if err := requireScaleReliable(raw, held); err != nil {
		t.Fatal(err)
	}
	replay := events[len(events)-1]
	replay.Forwarded, replay.Release = replay.Original, "marker"
	events = append(events, replay, scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 107, 7, 12, 14, 7)))
	released, err := readScaleWindow(scaleWindowTestLog(t, events))
	if err != nil {
		t.Fatalf("genuine queued response progress refused: %v", err)
	}
	if !released.released {
		t.Fatal("queued fifth control did not prove release progress")
	}
}

// TestScaleWindowPrearmQueuedResponse proves that the new fifth control may
// answer an intact prearm request, while five later requests are also processed.
func TestScaleWindowPrearmQueuedResponse(t *testing.T) {
	events := scaleWindowTestHeld()[:3]
	for ns := uint16(2); ns <= 8; ns++ {
		events = append(events, scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, ns, 1, 14, 99+ns)))
		if ns == 4 {
			zlb := scaleWindowTestPacket(0, 29, 0, 1, 5, 0, 0)[:12]
			binary.BigEndian.PutUint16(zlb[2:4], 12)
			events = append(events, scaleWindowTestEventFor("ze", zlb))
		}
	}
	events = append(events,
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 101, 1, 9, 14, 1)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 102, 2, 9, 14, 2)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(12, 19, 1, 9, 2, 0, 0)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 103, 3, 10, 14, 3)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(12, 19, 2, 10, 3, 0, 0)),
	)
	for ns := uint16(4); ns <= 6; ns++ {
		events = append(events, scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 100+ns, ns, 11, 14, ns)))
	}
	for ns := uint16(11); ns <= 15; ns++ {
		packet := scaleWindowTestPacket(10, 19, 0, ns, 7, 14, 97+ns)
		event := scaleWindowTestEventFor("lac", packet)
		binary.BigEndian.PutUint16(packet[10:12], 3)
		event.Forwarded = hex.EncodeToString(packet)
		events = append(events, event)
	}
	held, err := readScaleWindow(scaleWindowTestLog(t, events))
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"tunnel-id":19,"ns":7,"nr":16,"peer-nr":3,"outstanding":4,"retransmit-count":0,"cwnd":4,"ssthresh":4,"peer-rws":4}`)
	if err := requireScaleReliable(raw, held); err != nil {
		t.Fatal(err)
	}
	if _, err := readScaleWindow(scaleWindowTestLog(t, events[:len(events)-1])); err == nil {
		t.Fatal("prearm requests replaced the fifth postarm request")
	}
	replay := events[len(events)-1]
	replay.Forwarded, replay.Release = replay.Original, "marker"
	complete := append(cloneScaleWindowEvents(events), replay, scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 107, 7, 16, 14, 7)))
	released, err := readScaleWindow(scaleWindowTestLog(t, complete))
	if err != nil {
		t.Fatalf("queued prearm response refused: %v", err)
	}
	if !released.released {
		t.Fatal("queued prearm response made no progress")
	}
	complete[len(complete)-1] = scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 101, 7, 16, 14, 7))
	if _, err := readScaleWindow(scaleWindowTestLog(t, complete)); err == nil {
		t.Fatal("new Ns reused an already answered request")
	}
}

// TestScaleWindowReorderedControls accepts a fresh ICRP ahead of a queued
// predecessor, while the real ACKs and live fence still prove the entire hold.
func TestScaleWindowReorderedControls(t *testing.T) {
	events := scaleWindowTestReordered()
	held, err := readScaleWindow(scaleWindowTestLog(t, events))
	if err != nil {
		t.Fatalf("ICRP Ns4 before queued Ns3 refused: %v", err)
	}
	if held.baseline != 3 {
		t.Fatalf("held baseline %d, want 3", held.baseline)
	}
	if held.expectedNr != 15 {
		t.Fatalf("processed-request fence %d, want 15", held.expectedNr)
	}
	raw := []byte(`{"tunnel-id":19,"ns":7,"nr":15,"peer-nr":3,"outstanding":4,"retransmit-count":0,"cwnd":4,"ssthresh":4,"peer-rws":4}`)
	if err := requireScaleReliable(raw, held); err != nil {
		t.Fatal(err)
	}
	replay := events[len(events)-1]
	replay.Forwarded, replay.Release = replay.Original, "marker"
	complete := append(cloneScaleWindowEvents(events), replay,
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 106, 7, 15, 14, 7)))
	released, err := readScaleWindow(scaleWindowTestLog(t, complete))
	if err != nil {
		t.Fatalf("queued fifth response after authentic marker refused: %v", err)
	}
	if !released.released {
		t.Fatal("queued fifth response did not prove release progress")
	}
}

// TestScaleWindowReorderedRequests keeps the processing fence past every held
// request when the last two distinct LAC requests arrive in reverse Ns order.
func TestScaleWindowReorderedRequests(t *testing.T) {
	events := scaleWindowTestReordered()
	last := len(events) - 1
	events[last-1], events[last] = events[last], events[last-1]
	held, err := readScaleWindow(scaleWindowTestLog(t, events))
	if err != nil {
		t.Fatalf("reordered LAC ICRQs refused: %v", err)
	}
	if held.expectedNr != 15 {
		t.Fatalf("reordered processing fence %d, want 15", held.expectedNr)
	}
	raw := []byte(`{"tunnel-id":19,"ns":7,"nr":14,"peer-nr":3,"outstanding":4,"retransmit-count":0,"cwnd":4,"ssthresh":4,"peer-rws":4}`)
	if err := requireScaleReliable(raw, held); err == nil {
		t.Fatal("live receive fence omitted the highest held request")
	}
}

// TestScaleWindowReorderedIntegrity permits an identical retransmission, then
// refuses permanent holes, ACKs over a hole, Ns reuse and a false wrap.
func TestScaleWindowReorderedIntegrity(t *testing.T) {
	events := scaleWindowTestReordered()
	retransmit := scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 103, 3, 15, 14, 3))
	duplicate := append(cloneScaleWindowEvents(events), retransmit)
	if _, err := readScaleWindow(scaleWindowTestLog(t, duplicate)); err != nil {
		t.Fatalf("same ICRP with updated Nr counted as a new send: %v", err)
	}
	zeHole := append(cloneScaleWindowEvents(events[:14]), events[15:]...)
	lacHole := append(cloneScaleWindowEvents(events[:20]), events[21:]...)
	forged := cloneScaleWindowEvents(events)
	zlb := scaleWindowTestPacket(0, 19, 0, 10, 5, 0, 0)[:12]
	binary.BigEndian.PutUint16(zlb[2:4], 12)
	forged = slices.Insert(forged, 14, scaleWindowTestEventFor("lac", zlb))
	reused := append(cloneScaleWindowEvents(events),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 109, 3, 15, 14, 3)))
	wrapped := cloneScaleWindowEvents(events)
	wrapped[13] = scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 108, 65535, 10, 14, 4))
	for _, tc := range []struct {
		name   string
		events []scaleWindowTestEvent
	}{
		{"permanent Ze hole", zeHole},
		{"permanent LAC hole", lacHole},
		{"ACK over uncaptured predecessor", forged},
		{"conflicting Ns reuse", reused},
		{"wrap without complete sequence", wrapped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := readScaleWindow(scaleWindowTestLog(t, tc.events)); err == nil {
				t.Fatal("invalid reordered capture passed")
			}
		})
	}
}

// TestScaleTraceReorderWindowBound distinguishes a complete four-control
// inversion from a complete capture that jumps beyond the sender's window.
func TestScaleTraceReorderWindowBound(t *testing.T) {
	for _, tc := range []struct {
		name string
		last uint16
		pass bool
	}{
		{"last allowed Ze control first", 4, true},
		{"fifth Ze control first", 5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := scaleWindowTestHeld()[:3]
			events = append(events,
				scaleWindowTestEventFor("ze", scaleWindowTestPacket(6, 29, 0, tc.last, 2, 0, 0)))
			for ns := uint16(1); ns < tc.last; ns++ {
				events = append(events,
					scaleWindowTestEventFor("ze", scaleWindowTestPacket(6, 29, 0, ns, 2, 0, 0)))
			}
			_, err := readScaleTrace(scaleWindowTestLog(t, events))
			if tc.pass {
				if err != nil {
					t.Fatalf("complete reorder within four-control window refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("complete sequence masked a jump beyond the four-control window")
			}
		})
	}
}

// TestScaleReliableEvidence refuses a live first-RTO CWND1 and every missing
// or mismatched statistic; a wire-only estimate cannot replace the live fence.
func TestScaleReliableEvidence(t *testing.T) {
	held, err := readScaleWindow(scaleWindowTestLog(t, scaleWindowTestHeld()))
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]int{"tunnel-id": 19, "ns": 7, "nr": 10, "peer-nr": 3, "outstanding": 4, "retransmit-count": 0, "cwnd": 4, "ssthresh": 4, "peer-rws": 4}
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := requireScaleReliable(raw, held); err != nil {
		t.Fatal(err)
	}
	for key, wrong := range map[string]int{"tunnel-id": 20, "ns": 8, "nr": 8, "peer-nr": 4, "outstanding": 3, "cwnd": 1, "peer-rws": 16} {
		t.Run(key, func(t *testing.T) {
			mutated := make(map[string]int, len(fields))
			maps.Copy(mutated, fields)
			mutated[key] = wrong
			raw, err := json.Marshal(mutated)
			if err != nil {
				t.Fatal(err)
			}
			if err := requireScaleReliable(raw, held); err == nil {
				t.Fatal("incorrect live reliable state passed")
			}
			delete(mutated, key)
			raw, err = json.Marshal(mutated)
			if err != nil {
				t.Fatal(err)
			}
			if err := requireScaleReliable(raw, held); err == nil {
				t.Fatal("missing live reliable field passed")
			}
		})
	}
	for _, raw := range [][]byte{nil, []byte("null"), []byte("[]"), []byte("{}"), []byte("{")} {
		if err := requireScaleReliable(raw, held); err == nil {
			t.Fatalf("unknown reliable evidence passed: %s", raw)
		}
	}
}

// TestScaleWireSessionsEvidence joins all16 actual ICRP AssignedSessionIDs
// to daemon tuples, keeping LAC and Ze IDs distinct rather than assuming sid.
func TestScaleWireSessionsEvidence(t *testing.T) {
	events, population := scaleWindowTestSessions()
	if err := requireScaleWireSessions(scaleWindowTestLog(t, events), population); err != nil {
		t.Fatal(err)
	}
	if err := requireScaleWireSessions(scaleWindowTestLog(t, events[:len(events)-1]), population); err == nil {
		t.Fatal("missing ICRP passed for a live session")
	}
	for user, session := range population.byUser {
		session.local += 1000
		population.byUser[user] = session
		break
	}
	if err := requireScaleWireSessions(scaleWindowTestLog(t, events), population); err == nil {
		t.Fatal("wire/live Ze-local SID mismatch passed")
	}
}

// TestScalePeerCDNEvidence requires the actual peer's CDN for exactly the
// removed tuple, independently of accounting and later daemon disappearance.
func TestScalePeerCDNEvidence(t *testing.T) {
	_, population := scaleWindowTestSessions()
	target := population.byUser["scale-01"]
	packet := scaleWindowTestPacket(14, 19, 1, 20, 17, 14, 101)
	event := scaleWindowTestEventFor("lac", packet)
	if err := requireScalePeerCDN(scaleWindowTestLog(t, []scaleWindowTestEvent{event}), target); err != nil {
		t.Fatal(err)
	}
	for _, wrong := range []scaleWindowTestEvent{
		scaleWindowTestEventFor("ze", packet),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(14, 20, 1, 20, 17, 14, 101)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(14, 19, 2, 20, 17, 14, 101)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(14, 19, 1, 20, 17, 14, 102)),
	} {
		if err := requireScalePeerCDN(scaleWindowTestLog(t, []scaleWindowTestEvent{wrong}), target); err == nil {
			t.Fatal("another peer or tuple satisfied CDN evidence")
		}
	}
}

type scaleWindowTestEvent struct {
	Direction string `json:"direction"`
	Original  string `json:"original"`
	Forwarded string `json:"forwarded"`
	Release   string `json:"scale_release,omitempty"`
}

func scaleWindowTestHeld() []scaleWindowTestEvent {
	events := []scaleWindowTestEvent{
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(1, 0, 0, 0, 0, 9, 29)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(2, 29, 0, 0, 1, 9, 19)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(3, 19, 0, 1, 1, 0, 0)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, 2, 1, 14, 101)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 101, 1, 3, 14, 1)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, 3, 2, 14, 102)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 102, 2, 4, 14, 2)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, 4, 3, 14, 103)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 103, 3, 5, 14, 3)),
	}
	for index := uint16(5); index <= 9; index++ {
		nr := min(index-1, uint16(7))
		packet := scaleWindowTestPacket(10, 19, 0, index, nr, 14, 99+index)
		event := scaleWindowTestEventFor("lac", packet)
		binary.BigEndian.PutUint16(packet[10:12], 3)
		event.Forwarded = hex.EncodeToString(packet)
		events = append(events, event)
		if index <= 7 {
			events = append(events, scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 99+index, index-1, index+1, 14, index-1)))
		}
	}
	return events
}

func scaleWindowTestRelease(held []scaleWindowTestEvent, reason string) []scaleWindowTestEvent {
	events := cloneScaleWindowEvents(held)
	replay := events[len(events)-1]
	replay.Forwarded, replay.Release = replay.Original, reason
	return append(events, replay, scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 107, 7, 10, 14, 7)))
}

// scaleWindowTestReordered models Process returning the current ICRQ's ICRP4
// before OnReceive's previously queued ICRP3. Each ACK covers observed controls.
func scaleWindowTestReordered() []scaleWindowTestEvent {
	events := scaleWindowTestHeld()[:3]
	for ns := uint16(2); ns <= 8; ns++ {
		events = append(events, scaleWindowTestEventFor("lac",
			scaleWindowTestPacket(10, 19, 0, ns, 1, 14, 99+ns)))
	}
	events = append(events,
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 101, 1, 9, 14, 1)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 102, 2, 9, 14, 2)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, 9, 2, 14, 108)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 108, 4, 10, 14, 4)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 103, 3, 9, 14, 3)),
		scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, 10, 3, 14, 109)),
		scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 104, 5, 11, 14, 5)),
	)
	for ns := uint16(11); ns <= 14; ns++ {
		nr := min(ns-5, uint16(7))
		packet := scaleWindowTestPacket(10, 19, 0, ns, nr, 14, 99+ns)
		event := scaleWindowTestEventFor("lac", packet)
		binary.BigEndian.PutUint16(packet[10:12], 3)
		event.Forwarded = hex.EncodeToString(packet)
		events = append(events, event)
		if ns == 11 {
			events = append(events,
				scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 105, 6, 12, 14, 6)))
		}
	}
	return events
}

func scaleWindowTestSessions() ([]scaleWindowTestEvent, scalePopulation) {
	events := scaleWindowTestHeld()[:3]
	population := scalePopulation{byUser: make(map[string]scaleSession)}
	for index := uint16(1); index <= 16; index++ {
		username := fmt.Sprintf("scale-%02d", index)
		population.byUser[username] = scaleSession{19, index, 100 + index, username, netip.AddrFrom4([4]byte{192, 0, 2, byte(index)})}
		events = append(events,
			scaleWindowTestEventFor("lac", scaleWindowTestPacket(10, 19, 0, index+1, index, 14, 100+index)),
			scaleWindowTestEventFor("ze", scaleWindowTestPacket(11, 29, 100+index, index, index+2, 14, index)))
	}
	return events, population
}

func scaleWindowTestPacket(message, tunnel, session, ns, nr, attribute, value uint16) []byte {
	packet := controlPacket(message, tunnel, ns, nr)
	binary.BigEndian.PutUint16(packet[6:8], session)
	if attribute != 0 {
		packet = binary.BigEndian.AppendUint16(packet, 0x8008)
		packet = binary.BigEndian.AppendUint16(packet, 0)
		packet = binary.BigEndian.AppendUint16(packet, attribute)
		packet = binary.BigEndian.AppendUint16(packet, value)
		binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	}
	return packet
}

func scaleWindowTestEventFor(direction string, packet []byte) scaleWindowTestEvent {
	encoded := hex.EncodeToString(packet)
	return scaleWindowTestEvent{Direction: direction, Original: encoded, Forwarded: encoded}
}

func scaleWindowTestLog(t *testing.T, events []scaleWindowTestEvent) string {
	t.Helper()
	var output strings.Builder
	for _, event := range events {
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		output.Write(encoded)
		output.WriteByte('\n')
	}
	return output.String()
}

func cloneScaleWindowEvents(events []scaleWindowTestEvent) []scaleWindowTestEvent {
	return slices.Clone(events)
}

// TestScaleControlBounds refuses malformed headers and all AVPs, including
// attributes the identity projection does not otherwise need to interpret.
func TestScaleControlBounds(t *testing.T) {
	valid := scaleWindowTestPacket(1, 0, 0, 0, 0, 9, 29)
	if _, err := readScaleWireControl(valid); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func([]byte) []byte
	}{
		{"header truncated", func(packet []byte) []byte { return packet[:11] }},
		{"length mismatch", func(packet []byte) []byte { packet[3]--; return packet }},
		{"version", func(packet []byte) []byte { packet[1] = 3; return packet }},
		{"missing sequence flag", func(packet []byte) []byte { packet[0] &^= 8; return packet }},
		{"data-only flag", func(packet []byte) []byte { packet[0] |= 2; return packet }},
		{"AVP header truncated", func(packet []byte) []byte { return append(packet, 0x80) }},
		{"AVP length below header", func(packet []byte) []byte { packet[21] = 5; return packet }},
		{"AVP value truncated", func(packet []byte) []byte { packet[21] = 9; return packet }},
		{"AVP reserved flags", func(packet []byte) []byte { packet[20] |= 4; return packet }},
		{"first vendor", func(packet []byte) []byte { packet[15] = 1; return packet }},
		{"first attribute", func(packet []byte) []byte { packet[17] = 1; return packet }},
		{"first hidden", func(packet []byte) []byte { packet[12] |= 0x40; return packet }},
		{"first optional", func(packet []byte) []byte { packet[12] &^= 0x80; return packet }},
		{"assigned hidden", func(packet []byte) []byte { packet[20] |= 0x40; return packet }},
		{"assigned duplicate", func(packet []byte) []byte { return append(packet, packet[20:28]...) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packet := tc.change(slices.Clone(valid))
			if tc.name != "length mismatch" {
				if len(packet) >= 4 {
					binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
				}
			}
			if _, err := readScaleWireControl(packet); err == nil {
				t.Fatal("malformed control passed")
			}
		})
	}
}

// TestScaleCaptureBounds checks JSON/hex boundaries and validates transparency
// even for data rows that do not contribute to the control-window count.
func TestScaleCaptureBounds(t *testing.T) {
	for _, log := range []string{"", "null\n", "[]\n", "{\n", `{"direction":"other","original":"0002","forwarded":"0002"}`, `{"direction":"lac","original":"zz","forwarded":"zz"}`, `{"direction":"lac","original":"00","forwarded":"00"}`} {
		if _, err := readScaleWire(log); err == nil {
			t.Fatalf("invalid capture passed: %q", log)
		}
	}
	data := scaleWindowTestEventFor("lac", []byte{0, 2, 0, 19, 0, 1})
	if _, err := readScaleWire(scaleWindowTestLog(t, []scaleWindowTestEvent{data})); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		event scaleWindowTestEvent
	}{
		{"data modified", scaleWindowTestEvent{Direction: "lac", Original: data.Original, Forwarded: "000200130002"}},
		{"length changed", scaleWindowTestEvent{Direction: "lac", Original: data.Original, Forwarded: "0002"}},
		{"data replay", scaleWindowTestEvent{Direction: "lac", Original: data.Original, Forwarded: data.Original, Release: "marker"}},
		{"Ze replay", func() scaleWindowTestEvent {
			event := scaleWindowTestEventFor("ze", scaleWindowTestPacket(6, 29, 0, 0, 0, 0, 0))
			event.Release = "marker"
			return event
		}()},
		{"unknown replay", func() scaleWindowTestEvent {
			event := scaleWindowTestEventFor("lac", scaleWindowTestPacket(6, 19, 0, 0, 0, 0, 0))
			event.Release = "invented"
			return event
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := readScaleWire(scaleWindowTestLog(t, []scaleWindowTestEvent{tc.event})); err == nil {
				t.Fatal("invalid relay row passed")
			}
		})
	}
	if _, err := readScaleWire(strings.Repeat(" ", scaleWireBytesMax+1)); err == nil {
		t.Fatal("unbounded capture passed")
	}
}

// TestScaleReliableRequiredFields rejects every omitted and null producer field,
// even when a field's genuine zero (retransmit-count) is otherwise legal.
func TestScaleReliableRequiredFields(t *testing.T) {
	held, err := readScaleWindow(scaleWindowTestLog(t, scaleWindowTestHeld()))
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]any{"tunnel-id": 19, "ns": 7, "nr": 10, "peer-nr": 3, "outstanding": 4, "retransmit-count": 0, "cwnd": 4, "ssthresh": 4, "peer-rws": 4}
	for key := range fields {
		t.Run(key, func(t *testing.T) {
			for _, missing := range []bool{true, false} {
				mutated := make(map[string]any, len(fields))
				maps.Copy(mutated, fields)
				if missing {
					delete(mutated, key)
				} else {
					mutated[key] = nil
				}
				raw, err := json.Marshal(mutated)
				if err != nil {
					t.Fatal(err)
				}
				if err := requireScaleReliable(raw, held); err == nil {
					t.Fatal("missing/null reliable field passed")
				}
			}
		})
	}
	for _, tc := range []struct {
		key   string
		value any
	}{
		{"nr", 32777}, {"retransmit-count", -1}, {"ssthresh", 0}, {"tunnel-id", 65536}, {"ns", -1}, {"outstanding", "4"},
	} {
		mutated := make(map[string]any, len(fields))
		maps.Copy(mutated, fields)
		mutated[tc.key] = tc.value
		raw, err := json.Marshal(mutated)
		if err != nil {
			t.Fatal(err)
		}
		if err := requireScaleReliable(raw, held); err == nil {
			t.Fatalf("invalid %s=%v passed", tc.key, tc.value)
		}
	}
}

// TestScaleReliableBoundedProgress allows a live sample to advance after the
// log snapshot, but refuses progress outside the complete-from-zero bound.
func TestScaleReliableBoundedProgress(t *testing.T) {
	held, err := readScaleWindow(scaleWindowTestLog(t, scaleWindowTestHeld()))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		nr     uint16
		accept bool
	}{
		{11, true},
		// This is the validator's theoretical bound, not the expected
		// live frontier of the sixteen-session scenario.
		{scaleWireControlsMax, true},
		{scaleWireControlsMax + 1, false},
		{32777, false},
		{65535, false},
	} {
		raw := []byte(fmt.Sprintf(`{"tunnel-id":19,"ns":7,"nr":%d,"peer-nr":3,"outstanding":4,"retransmit-count":0,"cwnd":4,"ssthresh":4,"peer-rws":4}`, tc.nr))
		if err := requireScaleReliable(raw, held); (err == nil) != tc.accept {
			t.Fatalf("live Nr%d acceptance=%t, want %t: %v", tc.nr, err == nil, tc.accept, err)
		}
	}
}

// TestScaleWireIdentityBoundaries changes each side of the tuple separately and
// keeps foreign tunnels in the live population to prove per-relay selection.
func TestScaleWireIdentityBoundaries(t *testing.T) {
	events, population := scaleWindowTestSessions()
	population.byUser["foreign"] = scaleSession{20, 1, 101, "foreign", netip.MustParseAddr("192.0.2.100")}
	if err := requireScaleWireSessions(scaleWindowTestLog(t, events), population); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		index, offset int
		value         uint16
	}{
		{"wrong Ze tunnel", 4, 4, 30}, {"wrong LAC tunnel", 3, 4, 20},
		{"wrong ICRQ header SID", 3, 6, 1}, {"wrong ICRP header SID", 4, 6, 102},
		{"zero assigned Ze SID", 4, 26, 0}, {"zero assigned LAC SID", 3, 26, 0},
		{"duplicate Ze SID", 6, 26, 1}, {"missing distinct Ns", 4, 8, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := cloneScaleWindowEvents(events)
			packet, err := hex.DecodeString(changed[tc.index].Original)
			if err != nil {
				t.Fatal(err)
			}
			binary.BigEndian.PutUint16(packet[tc.offset:tc.offset+2], tc.value)
			changed[tc.index] = scaleWindowTestEventFor(changed[tc.index].Direction, packet)
			if err := requireScaleWireSessions(scaleWindowTestLog(t, changed), population); err == nil {
				t.Fatal("wrong wire identity passed")
			}
		})
	}
}

// FuzzScaleWireControl exercises the independent bounded decoder at every byte
// boundary; arbitrary external control bytes must return safely or decode.
func FuzzScaleWireControl(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0xc8, 2})
	f.Add(scaleWindowTestPacket(1, 0, 0, 0, 0, 9, 29))
	f.Add([]byte{0xc8, 2, 0, 12, 0, 19, 0, 0, 0, 1, 0, 1})
	f.Fuzz(func(t *testing.T, packet []byte) {
		if len(packet) > 65536 {
			return
		}
		if _, err := readScaleWireControl(packet); err != nil {
			return
		}
	})
}
