// Design: docs/labs/l2tp-interop.md -- full-session PPP framing evidence.
// Related: ppp_framing.go -- complete capture, startup and compressed Echo predicates.
package l2tp

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type pppFramingTestEvent struct {
	direction string
	ordinal   int
	armed     *bool
	datagram  []byte
}

// pppFramingTestEvents builds the full sequence independently of the relay:
// paired Configure exchanges, data, four probes/replies and compressed Echo.
func pppFramingTestEvents() []pppFramingTestEvent {
	var events []pppFramingTestEvent
	ordinal := 0
	armed := false
	add := func(direction string, protocol uint16, packet []byte) {
		order := 0
		var phase *bool
		if direction != relayInjected {
			ordinal++
			order = ordinal
			phase = new(bool)
			*phase = armed
		}
		events = append(events, pppFramingTestEvent{direction, order, phase, pppDatagram(protocol, packet)})
	}
	zeOptions := []byte{5, 6, 0x11, 0x22, 0x33, 0x44}
	peerOptions := []byte{5, 6, 0x55, 0x66, 0x77, 0x88}
	add(relayFromZe, protocolLCP, framingTestPacket(1, 1, zeOptions))
	add(relayFromLAC, protocolLCP, framingTestPacket(2, 1, zeOptions))
	add(relayFromLAC, protocolLCP, framingTestPacket(1, 2, peerOptions))
	add(relayFromZe, protocolLCP, framingTestPacket(2, 2, peerOptions))
	add(relayFromZe, 0x0021, []byte{0x45, 0, 0, 20})
	armed = true
	add(relayFromLAC, 0x0021, []byte{0x45, 0, 0, 20}) // HEAD logs this armed trigger before injecting.
	for _, probe := range []struct {
		protocol uint16
		code     byte
	}{
		{protocolLCP, probeCodeLCP}, {protocolLCP, probeCodeLCP},
		{protocolIPCP, probeCodeIPCP}, {protocolIPCP, probeCodeIPCP},
	} {
		add(relayInjected, probe.protocol, codeRejectProbe(probe.code))
	}
	for index := range 2 {
		add(relayFromZe, protocolLCP, codeReject(byte(index+1), codeRejectProbe(probeCodeLCP), pppCodeRejectMRU))
	}
	for index := range 2 {
		add(relayFromZe, protocolIPCP, codeReject(byte(index+1), codeRejectProbe(probeCodeIPCP), pppCodeRejectMRU))
	}
	echo := framingTestEcho(codeEchoRequest, 0x55667788)
	compressed := pppDatagram(protocolLCP, echo)
	compressed = append(compressed[:6], compressed[8:]...)
	events = append(events, pppFramingTestEvent{relayCompressedPPP, 0, nil, compressed})
	add(relayFromZe, protocolLCP, framingTestEcho(codeEchoReply, 0x11223344))
	return events
}

func framingTestPacket(code, id byte, data []byte) []byte {
	packet := make([]byte, 4+len(data))
	packet[0], packet[1] = code, id
	binary.BigEndian.PutUint16(packet[2:], uint16(len(packet)))
	copy(packet[4:], data)
	return packet
}

func framingTestEcho(code byte, magic uint32) []byte {
	data := make([]byte, 4+len(compressedPPPData))
	binary.BigEndian.PutUint32(data, magic)
	copy(data[4:], compressedPPPData)
	return framingTestPacket(code, compressedPPPIdentifier, data)
}

func framingTestLog(t *testing.T, events []pppFramingTestEvent) string {
	t.Helper()
	var log strings.Builder
	for _, event := range events {
		encoded := hex.EncodeToString(event.datagram)
		row := struct {
			avpWireObservation
			DataOrdinal     int   `json:"data_ordinal"`
			CodeRejectArmed *bool `json:"code_reject_armed,omitempty"`
		}{avpWireObservation{event.direction, encoded, encoded}, event.ordinal, event.armed}
		line, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		log.Write(line)
		log.WriteByte('\n')
	}
	return log.String()
}

// TestPPPFullSessionFraming proves a complete useful capture succeeds and
// missing framing, startup, continuity or phase witnesses are refused.
func TestPPPFullSessionFraming(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]pppFramingTestEvent) []pppFramingTestEvent
	}{
		{name: "first data missing", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { return e[1:] }},
		{name: "ordinal gap", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { e[1].ordinal++; return e }},
		{name: "LCP Address missing", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { e[0].datagram[6] = 0xfe; return e }},
		{name: "IP Control missing", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { e[4].datagram[7] = 0; return e }},
		{name: "startup request missing", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { e[0].datagram[10] = 9; return e }},
		{name: "LCP reject witness missing", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { e[10].datagram[10] = 9; return e }},
		{name: "IP witness missing", change: func(e []pppFramingTestEvent) []pppFramingTestEvent {
			e[4].datagram[8], e[4].datagram[9] = 0, 0x57
			return e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := requirePPPFullSessionFraming(framingTestLog(t, tc.change(pppFramingTestEvents()))); err == nil {
				t.Fatal("accepted the incomplete or unframed capture")
			}
		})
	}
	if err := requirePPPFullSessionFraming(framingTestLog(t, pppFramingTestEvents())); err != nil {
		t.Fatal(err)
	}
	if err := requirePPPFullSessionFraming(""); err == nil {
		t.Fatal("accepted an empty capture")
	}
	if _, err := framingPPPFrames("{not json"); err == nil {
		t.Fatal("accepted malformed JSON")
	}
}

// TestPPPArmedPhasePreservation excludes startup alone, preserving HEAD's
// armed trigger and all subsequent normal control packets and original probes.
func TestPPPArmedPhasePreservation(t *testing.T) {
	log := framingTestLog(t, pppFramingTestEvents())
	full, err := framingPPPFrames(log)
	if err != nil {
		t.Fatal(err)
	}
	if err := sessionKept(full); err == nil {
		t.Fatal("the old predicate ignored startup Configure packets")
	}
	stable, err := armedPPPFrames(log)
	if err != nil {
		t.Fatal(err)
	}
	if err := injectedProbes(stable); err != nil {
		t.Fatal(err)
	}
	if err := sessionKept(stable); err != nil {
		t.Fatal(err)
	}
	for _, code := range []byte{1, 2, 5, 6} {
		t.Run(string([]byte{'c', 'o', 'd', 'e', '-', '0' + code}), func(t *testing.T) {
			events := pppFramingTestEvents()
			events[5].datagram = pppDatagram(protocolLCP, framingTestPacket(code, 0x71, nil))
			// Rebuild HEAD's marker-gated log with its unchanged fixture writer.
			// Its original reader and predicate MUST reject this same trigger.
			var legacy strings.Builder
			for _, event := range events {
				if event.direction != relayInjected {
					if event.armed == nil {
						continue
					}
					if !*event.armed {
						continue
					}
				}
				legacy.WriteString(wireLine(t, event.direction, event.datagram, event.datagram))
			}
			oldFrames, err := relayPPPFrames(legacy.String())
			if err != nil {
				t.Fatal(err)
			}
			if err := sessionKept(oldFrames); err == nil {
				t.Fatal("HEAD accepted the regression fixture")
			}
			frames, err := armedPPPFrames(framingTestLog(t, events))
			if err != nil {
				t.Fatal(err)
			}
			if len(frames) == 0 || frames[0].packet[0] != code {
				t.Fatal("dropped the armed trigger before first injection")
			}
			if err := sessionKept(frames); err == nil {
				t.Fatal("accepted HEAD-rejected armed Configure/Terminate trigger")
			}
		})
	}
}

// TestPPPArmedPhaseMetadata refuses missing, retreating and absent witnesses,
// rather than accepting a shortened stability interval or dropping probes.
func TestPPPArmedPhaseMetadata(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]pppFramingTestEvent) []pppFramingTestEvent
	}{
		{name: "missing startup flag", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { e[0].armed = nil; return e }},
		{name: "missing trigger flag", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { e[5].armed = nil; return e }},
		{name: "phase retreats", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { *e[10].armed = false; return e }},
		{name: "probe before witness", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { *e[5].armed = false; return e }},
		{name: "no armed witness", change: func(e []pppFramingTestEvent) []pppFramingTestEvent { return e[:5] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := armedPPPFrames(framingTestLog(t, tc.change(pppFramingTestEvents()))); err == nil {
				t.Fatal("accepted invalid armed-phase evidence")
			}
		})
	}
}

// TestPPPCompressedEchoEvidence requires one exact compressed request and its
// ordered response with the acknowledged Magic-Numbers and preserved Data.
func TestPPPCompressedEchoEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]pppFrame) []pppFrame
	}{
		{name: "request missing", change: func(f []pppFrame) []pppFrame { return append(f[:14], f[15:]...) }},
		{name: "request duplicated", change: func(f []pppFrame) []pppFrame { return append(f, f[14]) }},
		{name: "peer Magic wrong", change: func(f []pppFrame) []pppFrame { f[14].packet[4]++; return f }},
		{name: "reply Identifier wrong", change: func(f []pppFrame) []pppFrame { f[15].packet[1]++; return f }},
		{name: "reply Magic wrong", change: func(f []pppFrame) []pppFrame { f[15].packet[4]++; return f }},
		{name: "reply Data wrong", change: func(f []pppFrame) []pppFrame { f[15].packet[8]++; return f }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames, err := framingPPPFrames(framingTestLog(t, pppFramingTestEvents()))
			if err != nil {
				t.Fatal(err)
			}
			if err := requireCompressedPPPEcho(tc.change(frames)); err == nil {
				t.Fatal("accepted an uncorrelated compressed exchange")
			}
		})
	}
	frames, err := framingPPPFrames(framingTestLog(t, pppFramingTestEvents()))
	if err != nil {
		t.Fatal(err)
	}
	if err := requireCompressedPPPEcho(frames); err != nil {
		t.Fatal(err)
	}
	if _, err := acknowledgedPPPMagic([]byte{8, 2}); err == nil {
		t.Fatal("accepted negotiated ACFC")
	}
}

// TestPPPFramingAssertionAttribution preserves assertion 7's anchor before
// Wait flattens a typed cause, instead of letting an earlier poll own it.
func TestPPPFramingAssertionAttribution(t *testing.T) {
	const name = "ppp-code-reject-xl2tpd"
	tagged := pppCodeRejectFailure(name, 2, &pppFramingError{errors.New("missing ff 03")})
	if !strings.HasPrefix(tagged.Error(), "scenario "+name+" assertion 7:") {
		t.Fatal(tagged)
	}
	opaque := errors.New("wait never measured peer state: " + tagged.Error())
	outer := pppCodeRejectFailure(name, 2, opaque)
	if !strings.Contains(outer.Error(), "scenario "+name+" assertion 7:") {
		t.Fatal(outer)
	}
}
