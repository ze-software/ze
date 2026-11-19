// Design: docs/architecture/testing/interop.md -- the FreeRADIUS subscriber suite, l2tp-lns-radius-framed-route-hidden-xl2tpd
// Related: check_framed_route_hidden.go -- the parsers and judges these tests feed fixed bytes
//
// No container starts: the tests build L2TP and RADIUS octets by hand, wrap
// them in the pcap radiussubscriber_test.go's pcapOf writes, and read the
// scenario plan the real suite builds.

package radiussubscriber

import (
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// l2tpAVP is one AVP of a hand-built control message.
type l2tpAVP struct {
	flags uint16
	attr  uint16
	value []byte
}

// l2tpControl builds an L2TPv2 control message (T, L and S set) carrying avps.
func l2tpControl(avps ...l2tpAVP) []byte {
	message := make([]byte, 12)
	binary.BigEndian.PutUint16(message[0:2], l2tpTypeBit|l2tpLengthBit|l2tpSequenceBit|l2tpVersion)
	for _, avp := range avps {
		header := make([]byte, avpHeaderOctets)
		binary.BigEndian.PutUint16(header[0:2], avp.flags|uint16(avpHeaderOctets+len(avp.value)))
		binary.BigEndian.PutUint16(header[4:6], avp.attr)
		message = append(message, header...)
		message = append(message, avp.value...)
	}
	binary.BigEndian.PutUint16(message[2:4], uint16(len(message)))
	return message
}

func messageType(kind uint16) l2tpAVP {
	return l2tpAVP{flags: 0x8000, attr: avpMessageType, value: binary.BigEndian.AppendUint16(nil, kind)}
}

// VALIDATES: an ICRQ whose Assigned Session ID carries H reads as hidden, one
// in clear as not hidden, one without AVP 14 as absent; another message type,
// a data message and an AVP whose length overruns the message are no ICRQ;
// judgeHiddenICRQs accepts only captures whose every ICRQ hid AVP 14.
// PREVENTS: a scenario that passes over a LAC that hid nothing, where the
// session coming up proves nothing about ze revealing a hidden AVP.
func TestICRQAssignedSessionID(t *testing.T) {
	randomVector := l2tpAVP{flags: 0x8000, attr: 36, value: make([]byte, 16)}
	hidden := l2tpControl(messageType(l2tpMessageICRQ), randomVector,
		l2tpAVP{flags: 0x8000 | avpHiddenBit, attr: avpAssignedSessionID, value: make([]byte, 16)})
	inClear := l2tpControl(messageType(l2tpMessageICRQ), l2tpAVP{flags: 0x8000, attr: avpAssignedSessionID, value: []byte{0, 7}})
	absent := l2tpControl(messageType(l2tpMessageICRQ), randomVector)
	sccrq := l2tpControl(messageType(1), l2tpAVP{flags: 0x8000 | avpHiddenBit, attr: avpAssignedSessionID, value: make([]byte, 16)})
	// The last AVP, the hidden Assigned Session ID, claims more octets than
	// the message holds after it.
	overrun := append([]byte(nil), hidden...)
	binary.BigEndian.PutUint16(overrun[len(overrun)-22:], 0x8000|avpHiddenBit|30)

	for name, test := range map[string]struct {
		payload []byte
		want    icrqSessionID
		isICRQ  bool
	}{
		"hidden":           {payload: hidden, want: icrqSessionID{present: true, hidden: true}, isICRQ: true},
		"clear":            {payload: inClear, want: icrqSessionID{present: true}, isICRQ: true},
		"absent":           {payload: absent, isICRQ: true},
		"not an ICRQ":      {payload: sccrq},
		"data message":     {payload: []byte{0x40, 0x02, 0, 4, 0, 1, 0, 1}},
		"short":            {payload: []byte{0xc8}},
		"truncated":        {payload: hidden[:len(hidden)-3]},
		"overrunning AVP":  {payload: overrun},
		"Message Type off": {payload: l2tpControl(randomVector, messageType(l2tpMessageICRQ))},
	} {
		got, isICRQ := icrqAssignedSessionID(test.payload)
		if isICRQ != test.isICRQ || got != test.want {
			t.Errorf("%s: answered %+v, ICRQ %v; want %+v, ICRQ %v", name, got, isICRQ, test.want, test.isICRQ)
		}
	}

	icrqs := capturedICRQs(mustFrames(t, pcapOf([]coaDatagram{{sourcePort: 1702, octets: sccrq}, {sourcePort: 1702, octets: hidden}}, 4)))
	if len(icrqs) != 1 || judgeHiddenICRQs(icrqs) != nil {
		t.Errorf("a capture of an SCCRQ and a hidden ICRQ answered %+v, %v", icrqs, judgeHiddenICRQs(icrqs))
	}
	for name, capture := range map[string][]coaDatagram{
		"clear":  {{sourcePort: 1702, octets: hidden}, {sourcePort: 1702, octets: inClear}},
		"absent": {{sourcePort: 1702, octets: absent}},
		"none":   {{sourcePort: 1702, octets: sccrq}},
	} {
		if err := judgeHiddenICRQs(capturedICRQs(mustFrames(t, pcapOf(capture, 0)))); err == nil {
			t.Errorf("%s: a capture with no hidden Assigned Session ID was accepted", name)
		}
	}
}

// radiusPacket builds a RADIUS packet of code whose attributes are attributes.
func radiusPacket(code byte, attributes ...[]byte) []byte {
	packet := make([]byte, radiusHeaderOctets)
	packet[0], packet[1] = code, 9
	for _, attribute := range attributes {
		packet = append(packet, attribute...)
	}
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	return packet
}

// VALIDATES: an Access-Accept carrying an 18-octet attribute 80 reads as
// signed, wherever it sits; one without, one whose attribute overruns and one
// whose Length overruns the datagram do not; capturedAccepts skips other codes
// and judgeSignedAccepts refuses any unsigned Access-Accept and an empty
// capture.
// PREVENTS: the scenario's Message-Authenticator check passing over an
// unsigned answer, or over a capture holding no Access-Accept at all.
func TestCarriesMessageAuthenticator(t *testing.T) {
	signature := append([]byte{radiusAttributeMessageAuthenticator, radiusMessageAuthenticatorOctets}, make([]byte, 16)...)
	// Framed-Route is attribute 22 (RFC 2865 Section 5.22); its Length octet
	// is set once the value is in.
	framedRoute := append([]byte{22, 0}, "203.0.113.128/25 203.0.113.77 1"...)
	framedRoute[1] = byte(len(framedRoute))
	first := radiusPacket(radiusCodeAccessAccept, signature, framedRoute)
	last := radiusPacket(radiusCodeAccessAccept, framedRoute, signature)
	unsigned := radiusPacket(radiusCodeAccessAccept, framedRoute)
	overrun := radiusPacket(radiusCodeAccessAccept, []byte{22, 40, 'x'})
	longer := radiusPacket(radiusCodeAccessAccept, signature)
	binary.BigEndian.PutUint16(longer[2:4], uint16(len(longer)+1))

	for name, test := range map[string]struct {
		octets []byte
		want   bool
	}{
		"first": {first, true}, "last": {last, true}, "unsigned": {unsigned, false},
		"attribute overrun": {overrun, false}, "length overrun": {longer, false}, "short": {first[:19], false},
	} {
		if got := carriesMessageAuthenticator(test.octets); got != test.want {
			t.Errorf("%s: signed %v, want %v", name, got, test.want)
		}
	}

	reject := radiusPacket(3, framedRoute)
	accepts := capturedAccepts(mustFrames(t, pcapOf([]coaDatagram{{sourcePort: 1812, octets: reject}, {sourcePort: 1812, octets: first}}, 2)))
	if len(accepts) != 1 || judgeSignedAccepts(accepts) != nil {
		t.Errorf("a capture of a Reject and a signed Accept answered %+v, %v", accepts, judgeSignedAccepts(accepts))
	}
	if err := judgeSignedAccepts(capturedAccepts(mustFrames(t, pcapOf([]coaDatagram{{octets: first}, {octets: unsigned}}, 0)))); err == nil {
		t.Error("a capture holding an unsigned Access-Accept was accepted")
	}
	if err := judgeSignedAccepts(nil); err == nil {
		t.Error("a capture holding no Access-Accept was accepted")
	}
}

func mustFrames(t *testing.T, capture []byte) [][]byte {
	t.Helper()
	frames, err := captureFrames(capture)
	if err != nil {
		t.Fatal(err)
	}
	return frames
}

// VALIDATES: judgeFramedRoute accepts the one device route iproute2 prints
// for the Framed-Route, proto 253 metric 1 through the session's device, and
// refuses a gateway, another device, protocol or metric, and a second route.
// PREVENTS: a Framed-Route installed via the RADIUS gateway, or a route some
// other source installed, passing as ze's device route.
func TestJudgeFramedRoute(t *testing.T) {
	good := `[{"dst":"203.0.113.128/25","dev":"ppp1","protocol":"253","metric":1,"flags":[]}]`
	if err := judgeFramedRoute(decodeRoutes(t, good), "ppp1"); err != nil {
		t.Errorf("the device route was refused: %v", err)
	}
	for name, output := range map[string]string{
		"via":      `[{"dst":"203.0.113.128/25","gateway":"203.0.113.77","dev":"ppp1","protocol":"253","metric":1}]`,
		"device":   `[{"dst":"203.0.113.128/25","dev":"ppp0","protocol":"253","metric":1}]`,
		"protocol": `[{"dst":"203.0.113.128/25","dev":"ppp1","protocol":"boot","metric":1}]`,
		"metric":   `[{"dst":"203.0.113.128/25","dev":"ppp1","protocol":"253"}]`,
		"two":      `[{"dst":"203.0.113.128/25","dev":"ppp1","protocol":"253","metric":1},{"dst":"203.0.113.128/25","dev":"ppp1","protocol":"253","metric":2}]`,
		"none":     `[]`,
	} {
		if err := judgeFramedRoute(decodeRoutes(t, output), "ppp1"); err == nil {
			t.Errorf("%s: accepted %s", name, output)
		}
	}
}

func decodeRoutes(t *testing.T, output string) []kernelRoute {
	t.Helper()
	var routes []kernelRoute
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		t.Fatal(err)
	}
	return routes
}

// VALIDATES: only this scenario's plan mounts its own users file on the
// server and its own xl2tpd.conf on the LAC; the other scenarios keep the
// lab's.
// PREVENTS: the hidden-AVP LAC config leaking into every scenario, or the
// scenario running with the lab's LAC, which hides nothing.
func TestFramedRouteHiddenMounts(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	suite, err := suiteFor(root, interoplab.Environment{Suffix: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range suite.Scenarios {
		own := plan.Source.Name == scenarioFramedRouteHidden
		for _, peer := range plan.Peers {
			for _, mount := range peer.Mounts {
				if mount.Target != lacXL2TPDConf {
					continue
				}
				fromScenario := filepath.Dir(mount.Source) == plan.Source.Directory
				if fromScenario != own {
					t.Errorf("%s: %s mounts %s", plan.Source.Name, mount.Target, mount.Source)
				}
			}
			if own && peer.Name == peerServer {
				for _, mount := range peer.Mounts {
					if mount.Target == serverUsers && mount.Source != filepath.Join(plan.Source.Directory, "users") {
						t.Errorf("the server mounts users from %s", mount.Source)
					}
				}
			}
		}
	}
}
