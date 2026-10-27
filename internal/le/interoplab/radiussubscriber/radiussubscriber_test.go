// Design: docs/architecture/testing/interop.md -- the FreeRADIUS subscriber suite
// Related: radiussubscriber.go -- suiteFor, preflightContainer
// Related: checkers.go -- parseDetail, parseCapture, judgeCapture, replayScript, replyCode
//
// The tests build the real suite from the repository's scenario tree and read
// its topology and grants, and feed the checkers' parsers fixed bytes. No
// container starts.

package radiussubscriber

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// VALIDATES: the suite discovers every registered scenario, and ze and
// the LAC hold NET_ADMIN and the host's /dev/ppp while FreeRADIUS holds
// neither; the preflight runs the shared PPP device probe.
// PREVENTS: a scenario directory with no checker, and a peer that cannot
// create a kernel L2TP session or that gained a grant it does not need.
func TestSuiteTopologyAndGrants(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	suite, err := suiteFor(root, interoplab.Environment{Suffix: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(suite.Scenarios))
	for _, plan := range suite.Scenarios {
		names = append(names, plan.Source.Name)
		for _, peer := range plan.Peers {
			device := slices.Contains(peer.Arguments, interoplab.PPPDevice)
			netAdmin := slices.Contains(peer.Capabilities, netAdminCapability)
			wantGrant := peer.Name != peerServer
			if device != wantGrant || netAdmin != wantGrant {
				t.Errorf("%s/%s: /dev/ppp %v, NET_ADMIN %v, want both %v", plan.Source.Name, peer.Name, device, netAdmin, wantGrant)
			}
			if slices.Contains(peer.Arguments, "--privileged") {
				t.Errorf("%s/%s runs privileged", plan.Source.Name, peer.Name)
			}
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, ScenarioNames()) {
		t.Errorf("discovered %v, want %v", names, ScenarioNames())
	}
	if command := strings.Join(preflightContainer().Command, " "); !strings.Contains(command, interoplab.PPPDeviceProbe) {
		t.Errorf("preflight %q does not run the shared PPP device probe", command)
	}
	if err := preflightRefusal(interoplab.ParsePreflight("DEV_PPP=ok\n")); err != nil {
		t.Errorf("a host with the device was refused: %v", err)
	}
	if err := preflightRefusal(interoplab.ParsePreflight("DEV_PPP=missing\n")); err == nil ||
		!strings.HasSuffix(err.Error(), "\n  sudo modprobe ppp_generic") {
		t.Errorf("a host with no device answered %v", err)
	}
}

// VALIDATES: parseDetail reads each detail block's status and session id,
// quotes removed.
// PREVENTS: a checker that matches a Stop to the wrong Start.
func TestParseDetail(t *testing.T) {
	text := "Tue Oct 14 10:00:00 2026\n\tAcct-Status-Type = Start\n\tAcct-Session-Id = \"1-2-0123456789abcdef01234567-1\"\n\n" +
		"Tue Oct 14 10:00:05 2026\n\tAcct-Status-Type = Stop\n\tAcct-Session-Id = \"1-2-0123456789abcdef01234567-1\"\n"
	records := parseDetail(text)
	want := []accountingRecord{
		{status: "Start", sessionID: "1-2-0123456789abcdef01234567-1"},
		{status: "Stop", sessionID: "1-2-0123456789abcdef01234567-1"},
	}
	if !slices.Equal(records, want) {
		t.Errorf("records %+v, want %+v", records, want)
	}
	if acctSessionIDShape.FindStringSubmatch("1-2-1") != nil {
		t.Error("the HEAD id shape T-S-N passed as carrying a boot tag")
	}
}

// datagramOf builds a datagram whose octets are a bare 20-octet RADIUS header
// carrying code, identifier and authenticator.
func datagramOf(sourcePort uint16, code, identifier byte, authenticator [16]byte) coaDatagram {
	octets := make([]byte, 20)
	octets[0], octets[1] = code, identifier
	binary.BigEndian.PutUint16(octets[2:4], 20)
	copy(octets[4:20], authenticator[:])
	return coaDatagram{sourcePort: sourcePort, code: code, identifier: identifier, authenticator: authenticator, octets: octets}
}

// pcapOf builds a little-endian classic pcap of Ethernet/IPv4/UDP frames, one
// per datagram, each carrying the datagram's octets followed by padding
// octets of Ethernet padding past the UDP length.
func pcapOf(datagrams []coaDatagram, padding int) []byte {
	capture := make([]byte, 24)
	binary.LittleEndian.PutUint32(capture[0:4], 0xa1b2c3d4)
	for _, datagram := range datagrams {
		frame := make([]byte, 14+20+8+len(datagram.octets)+padding)
		frame[12], frame[13] = 0x08, 0x00
		frame[14] = 0x45
		frame[14+9] = 17
		binary.BigEndian.PutUint16(frame[34:36], datagram.sourcePort)
		binary.BigEndian.PutUint16(frame[36:38], 3799)
		binary.BigEndian.PutUint16(frame[38:40], uint16(8+len(datagram.octets)))
		copy(frame[42:], datagram.octets)
		record := make([]byte, 16)
		binary.LittleEndian.PutUint32(record[8:12], uint32(len(frame)))
		binary.LittleEndian.PutUint32(record[12:16], uint32(len(frame)))
		capture = append(capture, record...)
		capture = append(capture, frame...)
	}
	return capture
}

func sameDatagram(a, b coaDatagram) bool {
	return a.sourcePort == b.sourcePort && a.code == b.code && a.identifier == b.identifier &&
		a.authenticator == b.authenticator && bytes.Equal(a.octets, b.octets)
}

// VALIDATES: the capture parser recovers each datagram's port, Code,
// Identifier, Authenticator and octets, bounded by the UDP length rather than
// the frame; judgeCapture accepts a retransmission that kept port, Identifier
// and Authenticator and two clients whose octets differ in the port alone,
// and refuses each broken precondition.
// PREVENTS: a scenario that passes over requests that were never duplicates,
// and a replay that sends Ethernet padding as part of the RADIUS packet.
func TestJudgeCapture(t *testing.T) {
	auth := [16]byte{1, 2, 3}
	other := [16]byte{9}
	good := []coaDatagram{
		datagramOf(4000, 40, 7, auth),
		datagramOf(4000, 40, 7, auth),
		datagramOf(5000, 40, 11, other),
		datagramOf(5001, 40, 11, other),
	}
	parsed, err := parseCapture(pcapOf(good, 6))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(parsed, good, sameDatagram) {
		t.Fatalf("parsed %+v, want %+v", parsed, good)
	}
	if err := judgeCapture(parsed); err != nil {
		t.Errorf("good capture refused: %v", err)
	}

	broken := map[string]func([]coaDatagram){
		"retransmission from another port": func(d []coaDatagram) { d[1].sourcePort = 4001 },
		"retransmission re-signed":         func(d []coaDatagram) { d[1].authenticator = other },
		"clients on one port":              func(d []coaDatagram) { d[3].sourcePort = 5000 },
		"clients with two Authenticators":  func(d []coaDatagram) { d[3] = datagramOf(5001, 40, 11, auth) },
		"client with another Identifier":   func(d []coaDatagram) { d[3] = datagramOf(5001, 40, 12, other) },
		"client with a longer packet": func(d []coaDatagram) {
			d[3].octets = append(slices.Clone(d[3].octets), 0)
		},
		"clients not a Disconnect-Request": func(d []coaDatagram) {
			d[2], d[3] = datagramOf(5000, 43, 11, other), datagramOf(5001, 43, 11, other)
		},
	}
	for name, breakIt := range broken {
		datagrams := slices.Clone(good)
		breakIt(datagrams)
		if err := judgeCapture(datagrams); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := judgeCapture(good[:3]); err == nil {
		t.Error("three datagrams accepted")
	}
	if _, err := parseCapture(pcapOf(good, 0)[:30]); err != nil {
		t.Errorf("a truncated trailing record header is not an error: %v", err)
	}
	truncated := pcapOf(good, 0)
	if _, err := parseCapture(truncated[:len(truncated)-5]); err == nil {
		t.Error("a record claiming more octets than remain was accepted")
	}
	overlong := pcapOf(good[:1], 0)
	binary.BigEndian.PutUint16(overlong[24+16+38:], 8+20+1)
	if datagrams, err := parseCapture(overlong); err != nil || len(datagrams) != 0 {
		t.Errorf("a UDP length past the frame answered %+v, %v; want no datagram", datagrams, err)
	}
	short := pcapOf(good[:1], 0)
	binary.BigEndian.PutUint16(short[24+16+38:], 8+19)
	if datagrams, err := parseCapture(short); err != nil || len(datagrams) != 0 {
		t.Errorf("a UDP length shorter than a RADIUS header answered %+v, %v; want no datagram", datagrams, err)
	}
}

// VALIDATES: replayScript sends every octet, NUL, quote and percent included,
// as a \xHH escape inside one single-quoted printf argument to ze's CoA
// device, and replyCode reads the one Code od prints.
// PREVENTS: a replay whose octets differ from radclient's, which judgeCapture
// would then refuse as "differ beyond the port", and a missing answer read as
// Code 0.
func TestReplayScriptAndReplyCode(t *testing.T) {
	script := replayScript([]byte{0x28, 0x00, '\'', '%', 0xff})
	_, rest, found := strings.Cut(script, "printf '")
	if !found {
		t.Fatalf("no printf argument in %q", script)
	}
	argument, _, found := strings.Cut(rest, "'")
	if !found {
		t.Fatalf("unterminated printf argument in %q", script)
	}
	if argument != `\x28\x00\x27\x25\xff` {
		t.Errorf("printf argument %q", argument)
	}
	if !strings.Contains(script, "exec 3<>"+coaDevice) || coaDevice != "/dev/udp/172.26.0.2/3799" {
		t.Errorf("script does not open ze's CoA port %s: %q", coaTarget, script)
	}

	if code, err := replyCode("   42\n"); err != nil || code != radiusCodeDisconnectNAK {
		t.Errorf("replyCode(42) = %d, %v", code, err)
	}
	for _, output := range []string{"", " \n", "abc", "300"} {
		if code, err := replyCode(output); err == nil {
			t.Errorf("replyCode(%q) = %d, want an error", output, code)
		}
	}
}

// TestUnknownSelectorFailsBeforePeerSetup checks explicit selection precedence
// and environment selection without launching Docker or accepting an empty run.
func TestUnknownSelectorFailsBeforePeerSetup(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(selectorVariable, "missing-from-environment")
	for _, test := range []struct {
		selector string
		missing  string
	}{
		{selector: "missing-explicit", missing: "missing-explicit"},
		{missing: "missing-from-environment"},
	} {
		report := RunAt(t.Context(), root, test.selector)
		want := "no scenario matching '" + test.missing + "' found"
		if report.Code != 1 || report.SetupError != want {
			t.Fatalf("missing selector report = %+v, want %q", report, want)
		}
	}
}
