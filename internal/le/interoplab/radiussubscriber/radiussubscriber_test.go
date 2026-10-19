// Design: docs/architecture/testing/interop.md -- the FreeRADIUS subscriber suite
// Related: radiussubscriber.go -- suiteFor, preflightContainer
// Related: checkers.go -- parseDetail, parseCapture, judgeCapture
//
// The tests build the real suite from the repository's scenario tree and read
// its topology and grants, and feed the checkers' parsers fixed bytes. No
// container starts.

package radiussubscriber

import (
	"encoding/binary"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// VALIDATES: the suite discovers exactly its two named scenarios, and ze and
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

// pcapOf builds a little-endian classic pcap of Ethernet/IPv4/UDP frames, one
// per datagram, each carrying a 20-octet RADIUS header.
func pcapOf(datagrams []coaDatagram) []byte {
	capture := make([]byte, 24)
	binary.LittleEndian.PutUint32(capture[0:4], 0xa1b2c3d4)
	for _, datagram := range datagrams {
		frame := make([]byte, 14+20+8+20)
		frame[12], frame[13] = 0x08, 0x00
		frame[14] = 0x45
		frame[14+9] = 17
		binary.BigEndian.PutUint16(frame[34:36], datagram.sourcePort)
		binary.BigEndian.PutUint16(frame[36:38], 3799)
		frame[42], frame[43] = datagram.code, datagram.identifier
		copy(frame[46:62], datagram.authenticator[:])
		record := make([]byte, 16)
		binary.LittleEndian.PutUint32(record[8:12], uint32(len(frame)))
		binary.LittleEndian.PutUint32(record[12:16], uint32(len(frame)))
		capture = append(capture, record...)
		capture = append(capture, frame...)
	}
	return capture
}

// VALIDATES: the capture parser recovers each datagram's port, Code,
// Identifier and Authenticator; judgeCapture accepts a retransmission that
// kept all three and two clients that differ in the port alone, and refuses
// each broken precondition.
// PREVENTS: a scenario that passes over requests that were never duplicates.
func TestJudgeCapture(t *testing.T) {
	auth := [16]byte{1, 2, 3}
	other := [16]byte{9}
	good := []coaDatagram{
		{sourcePort: 4000, code: 40, identifier: 7, authenticator: auth},
		{sourcePort: 4000, code: 40, identifier: 7, authenticator: auth},
		{sourcePort: 5000, code: 40, identifier: 7, authenticator: other},
		{sourcePort: 5001, code: 40, identifier: 7, authenticator: other},
	}
	parsed, err := parseCapture(pcapOf(good))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(parsed, good) {
		t.Fatalf("parsed %+v, want %+v", parsed, good)
	}
	if err := judgeCapture(parsed); err != nil {
		t.Errorf("good capture refused: %v", err)
	}

	broken := map[string]func([]coaDatagram){
		"retransmission from another port": func(d []coaDatagram) { d[1].sourcePort = 4001 },
		"retransmission re-signed":         func(d []coaDatagram) { d[1].authenticator = other },
		"clients on one port":              func(d []coaDatagram) { d[3].sourcePort = 5000 },
		"clients with two Authenticators":  func(d []coaDatagram) { d[3].authenticator = auth },
		"client with another Identifier":   func(d []coaDatagram) { d[3].identifier = 8 },
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
	if _, err := parseCapture(pcapOf(good)[:30]); err != nil {
		t.Errorf("a truncated trailing record header is not an error: %v", err)
	}
	truncated := pcapOf(good)
	if _, err := parseCapture(truncated[:len(truncated)-5]); err == nil {
		t.Error("a record claiming more octets than remain was accepted")
	}
}
