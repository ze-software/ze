// Design: docs/guide/bmp.md -- RFC 7854 Initiation identity and reporting.

package bmp

import (
	"encoding/json"
	"errors"
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/version"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RFC requirement: RFC7854-4.4-1 positive -- decoded String TLVs reach the
// sessions command in their transmitted order, including UTF-8 text.
func TestRFC7854InitiationStringsReportedInOrder(t *testing.T) {
	bp := &BMPPlugin{state: newBMPState()}
	want := []string{"maintenance starts", "région nord", "maintenance ends"}
	reportInitiationStrings(t, bp, want)
	if got := reportedInitiationStrings(t, bp); !slices.Equal(got, want) {
		t.Fatalf("reported strings = %q, want %q", got, want)
	}
}

// RFC requirement: RFC7854-4.4-1 negative -- repeated strings are neither
// collapsed nor sorted or reversed, and a later Initiation replaces the report.
func TestRFC7854InitiationStringsKeepDuplicates(t *testing.T) {
	bp := &BMPPlugin{state: newBMPState()}
	reportInitiationStrings(t, bp, []string{"old report"})
	want := []string{"zulu", "alpha", "zulu", "bravo"}
	reportInitiationStrings(t, bp, want)
	if got := reportedInitiationStrings(t, bp); !slices.Equal(got, want) {
		t.Fatalf("reported strings = %q, want %q", got, want)
	}
}

// RFC requirement: RFC7854-4.4-1 positive -- the Peer Up Information field's
// String TLVs reach the peers command in their transmitted order.
func TestRFC7854PeerUpStringsReportedInOrder(t *testing.T) {
	bp := &BMPPlugin{state: newBMPState()}
	want := []string{"customer handoff", "région sud", "backup circuit"}
	reportPeerUpStrings(t, bp, want)
	if got := reportedPeerUpStrings(t, bp); !slices.Equal(got, want) {
		t.Fatalf("reported peer strings = %q, want %q", got, want)
	}
}

// RFC requirement: RFC7854-4.4-1 negative -- Peer Up strings keep duplicates
// and order, and a new Peer Up without information removes the previous report.
func TestRFC7854PeerUpStringsKeepDuplicates(t *testing.T) {
	bp := &BMPPlugin{state: newBMPState()}
	want := []string{"zulu", "alpha", "zulu", "bravo"}
	reportPeerUpStrings(t, bp, want)
	if got := reportedPeerUpStrings(t, bp); !slices.Equal(got, want) {
		t.Fatalf("reported peer strings = %q, want %q", got, want)
	}
	reportPeerUpStrings(t, bp, nil)
	if got := reportedPeerUpStrings(t, bp); len(got) != 0 {
		t.Fatalf("Peer Up without Information retained strings %q", got)
	}
}

func reportPeerUpStrings(t *testing.T, bp *BMPPlugin, messages []string) {
	t.Helper()
	up := PeerUp{
		Peer:            testPeerHeader(),
		SentOpenMsg:     makeBGPOpen(65000, 0xc0000201),
		ReceivedOpenMsg: makeBGPOpen(65001, 0xc0000202),
	}
	for _, text := range messages {
		up.InfoTLVs = append(up.InfoTLVs, makeStringTLV(InitTLVString, text))
	}
	var wire [2048]byte
	n := writePeerUp(wire[:], 0, &up)
	decoded, err := DecodeMsg(wire[:n])
	if err != nil {
		t.Fatalf("decode Peer Up: %v", err)
	}
	bp.processMessage("192.0.2.8:12345", decoded)
	clear(wire[:])
}

func reportedPeerUpStrings(t *testing.T, bp *BMPPlugin) []string {
	t.Helper()
	status, data, err := bp.handleCommand("show bmp peers")
	if err != nil {
		t.Fatal(err)
	}
	if status != statusDone {
		t.Fatalf("peers command status = %s", status)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Peers []struct {
			Strings []string `json:"peer-up-strings"`
		} `json:"peers"`
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Peers) != 1 {
		t.Fatalf("reported %d peers, want one", len(result.Peers))
	}
	return result.Peers[0].Strings
}

func reportInitiationStrings(t *testing.T, bp *BMPPlugin, messages []string) {
	t.Helper()
	const remote = "192.0.2.8:12345"
	if bp.state.routerCount() == 0 {
		bp.state.addRouter(remote)
	}
	init := Initiation{TLVs: []TLV{
		makeStringTLV(InitTLVSysName, "edge.example"),
		makeStringTLV(InitTLVSysDescr, "monitored router"),
	}}
	for _, text := range messages {
		init.TLVs = append(init.TLVs, makeStringTLV(InitTLVString, text))
	}
	var wire [2048]byte
	n := writeInitiation(wire[:], 0, &init)
	decoded, err := DecodeMsg(wire[:n])
	if err != nil {
		t.Fatalf("decode Initiation: %v", err)
	}
	bp.processMessage(remote, decoded)
	// Receiver state must outlive the read buffer that carried the TLVs.
	clear(wire[:])
}

func reportedInitiationStrings(t *testing.T, bp *BMPPlugin) []string {
	t.Helper()
	status, data, err := bp.handleCommand("show bmp sessions")
	if err != nil {
		t.Fatal(err)
	}
	if status != statusDone {
		t.Fatalf("sessions command status = %s", status)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Sessions []struct {
			Strings []string `json:"initiation-strings"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Sessions) != 1 {
		t.Fatalf("reported %d sessions, want one", len(result.Sessions))
	}
	return result.Sessions[0].Strings
}

// RFC requirement: RFC7854-4.4-3 positive -- the system host and domain supplied
// over the SDK config rail identify the router on the collector's TCP stream.
func TestRFC7854InitiationUsesConfiguredSystemName(t *testing.T) {
	engine, session, first := identityCollector(t, "edge-west", "example.net")
	if got := initiationValue(t, first, InitTLVSysName); got != "edge-west.example.net" {
		t.Fatalf("sysName = %q, want configured FQDN", got)
	}
	// A failed transaction must restore the identity that its apply replaced.
	if err := engine.reloadSections([]rpc.ConfigSection{{Root: configRootSystem,
		Data: `{"system":{"host":"candidate","domain":"example.net"}}`}}); err != nil {
		t.Fatal(err)
	}
	engine.call(t, "ze-plugin-callback:config-rollback", struct {
		Reason string `json:"reason"`
	}{Reason: "another participant rejected the commit"})
	if got := initiationValue(t, captureInitiation(t, session), InitTLVSysName); got != "edge-west.example.net" {
		t.Fatalf("rollback sysName = %q, want previous identity", got)
	}
}

// RFC requirement: RFC7854-4.4-3 negative -- changing only system identity reaches
// the next Initiation on an existing sender, without the old constant name.
func TestRFC7854InitiationSystemNameChanges(t *testing.T) {
	engine, session, first := identityCollector(t, "edge-one", "example.net")
	if err := engine.reloadSections([]rpc.ConfigSection{{Root: configRootSystem,
		Data: `{"system":{"host":"edge-two.example.org","domain":"example.net"}}`}}); err != nil {
		t.Fatal(err)
	}
	got := initiationValue(t, captureInitiation(t, session), InitTLVSysName)
	if got != "edge-two.example.org" {
		t.Fatalf("changed sysName = %q, want new fully qualified name", got)
	}
	if got == initiationValue(t, first, InitTLVSysName) {
		t.Fatal("system identity change left sysName unchanged")
	}
}

// RFC requirement: RFC7854-4.4-2 positive -- sysDescr identifies the running Ze
// build, operating system release, and hardware architecture read from uname.
func TestRFC7854InitiationDescribesRunningSystem(t *testing.T) {
	var kernel unix.Utsname
	if err := unix.Uname(&kernel); err != nil {
		t.Fatal(err)
	}
	description := initiationValue(t, captureInitiation(t, &senderSession{}), InitTLVSysDescr)
	if !strings.HasPrefix(description, version.Short()+"; ") {
		t.Fatalf("sysDescr %q does not identify running build %q", description, version.Short())
	}
	for _, value := range []string{
		unix.ByteSliceToString(kernel.Sysname[:]),
		unix.ByteSliceToString(kernel.Release[:]),
		unix.ByteSliceToString(kernel.Machine[:]),
	} {
		if !strings.Contains(description, value) {
			t.Fatalf("sysDescr %q omits system identity %q", description, value)
		}
	}
}

// RFC requirement: RFC7854-4.4-2 negative -- a software identity change must
// change the sysDescr bytes; a constant daemon description cannot pass.
func TestRFC7854InitiationDescriptionChangesWithBuild(t *testing.T) {
	previousRelease, previousDate := version.Release(), version.BuildDate()
	t.Cleanup(func() { version.Stamp(previousRelease, previousDate) })
	version.Stamp("26.09.20", "2026-09-20T01:02:03Z")
	first := initiationValue(t, captureInitiation(t, &senderSession{}), InitTLVSysDescr)
	version.Stamp("26.09.21", "2026-09-21T04:05:06Z")
	second := initiationValue(t, captureInitiation(t, &senderSession{}), InitTLVSysDescr)
	if first == second {
		t.Fatal("sysDescr stayed constant after the running build identity changed")
	}
	if !strings.HasPrefix(second, version.Short()+"; ") {
		t.Fatalf("sysDescr %q does not carry changed build %q", second, version.Short())
	}
}

// Identity source errors must return the error and write no BMP bytes. In
// particular, neither unknown nor the old ze identity can substitute for a read.
func TestRFC7854InitiationIdentitySourceFailureWritesNothing(t *testing.T) {
	failure := errors.New("identity source unavailable")
	tests := []struct {
		name     string
		hostname func() (string, error)
		uname    func(*unix.Utsname) error
	}{
		{"hostname", func() (string, error) { return "", failure }, unix.Uname},
		{"kernel", func() (string, error) { return "edge", nil }, func(*unix.Utsname) error { return failure }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := newRecordingConn()
			ss := &senderSession{identity: func() (systemIdentity, error) {
				return readSystemIdentity(nil, test.hostname, test.uname)
			}}
			if err := ss.sendInitiation(conn); !errors.Is(err, failure) {
				t.Fatalf("sendInitiation error = %v, want source failure", err)
			}
			if got := conn.written(); len(got) != 0 {
				t.Fatalf("identity failure wrote BMP bytes: %x", got)
			}
		})
	}
}

func captureInitiation(t *testing.T, session *senderSession) *Initiation {
	t.Helper()
	conn := newRecordingConn()
	if err := session.sendInitiation(conn); err != nil {
		t.Fatal(err)
	}
	msg, err := DecodeMsg(conn.written())
	if err != nil {
		t.Fatal(err)
	}
	init, ok := msg.(*Initiation)
	if !ok {
		t.Fatalf("first BMP message = %T, want Initiation", msg)
	}
	return init
}

func initiationValue(t *testing.T, init *Initiation, typ uint16) string {
	t.Helper()
	for _, tlv := range init.TLVs {
		if tlv.Type == typ {
			return string(tlv.Value)
		}
	}
	t.Fatalf("Initiation omits TLV %d", typ)
	return ""
}

func identityCollector(t *testing.T, name, domain string) (*reloadEngine, *senderSession, *Initiation) {
	t.Helper()
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeLog(listener, "identity-listener") })
	bp := &BMPPlugin{stopCh: make(chan struct{})}
	engine := startReloadEngine(t, bp, "")
	t.Cleanup(func() {
		bp.stopSenders()
		bp.sessions.Wait()
	})
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	sections := []rpc.ConfigSection{
		{Root: configRootBGP, Data: `{"bgp":{"bmp":{"sender":{"collector":{"identity":{"address":"127.0.0.1","port":` + strconv.Quote(port) + `}}}}}}`},
		{Root: configRootSystem, Data: `{"system":{"host":` + strconv.Quote(name) + `,"domain":` + strconv.Quote(domain) + `}}`},
	}
	if err := engine.reloadSections(sections); err != nil {
		t.Fatal(err)
	}
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeLog(conn, "identity-collector") })
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	msg, err := readBMPFromPipe(conn)
	if err != nil {
		t.Fatal(err)
	}
	init, ok := msg.(*Initiation)
	if !ok {
		t.Fatalf("first message = %T, want Initiation", msg)
	}
	bp.mu.RLock()
	session := bp.senders[0]
	bp.mu.RUnlock()
	return engine, session, init
}
