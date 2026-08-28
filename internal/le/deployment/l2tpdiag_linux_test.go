//go:build linux

package deployment

import (
	"encoding/hex"
	"errors"
	"net"
	"os"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"

	nl "github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type recordingL2TPDiagnosticOps struct {
	calls          []string
	requests       map[string][]byte
	fail           map[string]error
	failAt         map[string]int
	dumpReply      map[string][][]byte
	seen           map[string]int
	nextFD         int
	tunnelMessage  []byte
	sessionMessage []byte
}

func newRecordingL2TPDiagnosticOps() *recordingL2TPDiagnosticOps {
	return &recordingL2TPDiagnosticOps{
		calls: []string{}, requests: map[string][]byte{}, fail: map[string]error{},
		failAt: map[string]int{}, seen: map[string]int{}, dumpReply: map[string][][]byte{},
		nextFD: 3,
	}
}

func (o *recordingL2TPDiagnosticOps) call(name string) error {
	o.calls = append(o.calls, name)
	o.seen[name]++
	at := o.failAt[name]
	if at == 0 || at == o.seen[name] {
		return o.fail[name]
	}
	return nil
}

func (o *recordingL2TPDiagnosticOps) Socket(domain, kind, protocol int) (int, error) {
	name := "socket inet"
	if domain == afPPPOX {
		name = "socket pppox"
	}
	if err := o.call(name); err != nil {
		return 0, err
	}
	fd := o.nextFD
	o.nextFD++
	return fd, nil
}

func (o *recordingL2TPDiagnosticOps) setReusePort(int) error {
	return o.call("setsockopt reuseport")
}

func (o *recordingL2TPDiagnosticOps) bindIPv4(int, [4]byte, uint16) error {
	return o.call("bind udp")
}

func (o *recordingL2TPDiagnosticOps) Family(string) (uint16, error) {
	if err := o.call("resolve family"); err != nil {
		return 0, err
	}
	return 42, nil
}

func (o *recordingL2TPDiagnosticOps) Execute(operation string, request *nl.NetlinkRequest) ([][]byte, error) {
	if err := o.call(operation); err != nil {
		return nil, err
	}
	raw := request.Serialize()
	payload := append([]byte(nil), raw[16:]...)
	o.requests[operation] = payload
	if reply, ok := o.dumpReply[operation]; ok {
		return reply, nil
	}
	switch operation {
	case "tunnel-create":
		o.tunnelMessage = payload
	case "session-create":
		o.sessionMessage = payload
	case "tunnel-dump":
		return [][]byte{append([]byte(nil), o.tunnelMessage...)}, nil
	case "session-dump":
		return [][]byte{append([]byte(nil), o.sessionMessage...)}, nil
	}
	return nil, nil
}

func (o *recordingL2TPDiagnosticOps) Connect(int, []byte) error {
	return o.call("connect pppox")
}

func (o *recordingL2TPDiagnosticOps) ioctlGetInt(int, uint) (int, error) {
	if err := o.call("ioctl get channel"); err != nil {
		return 0, err
	}
	return 7, nil
}

func (o *recordingL2TPDiagnosticOps) ioctlSetInt(_ int, request uint, _ int) error {
	if request == pppiocAttChan {
		return o.call("ioctl attach channel")
	}
	return o.call("ioctl connect unit")
}

func (o *recordingL2TPDiagnosticOps) ioctlGetSetInt(int, uint, int) (int, error) {
	if err := o.call("ioctl new unit"); err != nil {
		return 0, err
	}
	return 0, nil
}

func (o *recordingL2TPDiagnosticOps) openPPP() (int, error) {
	if err := o.call("open /dev/ppp"); err != nil {
		return 0, err
	}
	fd := o.nextFD
	o.nextFD++
	return fd, nil
}

func (o *recordingL2TPDiagnosticOps) procPPPoL2TP() string {
	o.calls = append(o.calls, "read /proc/net/pppol2tp")
	return "producer proc dump\n"
}

func (o *recordingL2TPDiagnosticOps) devPPPText() string {
	o.calls = append(o.calls, "stat /dev/ppp")
	return "  /dev/ppp mode=Dcrw------- size=0\n"
}

func (o *recordingL2TPDiagnosticOps) Link(name string) (diagnosticLink, error) {
	var tb textbuf.Buffer
	if err := o.call(tb.Str("link ").Str(name).String()); err != nil {
		return diagnosticLink{}, err
	}
	return diagnosticLink{Name: name, Index: 9, Type: "ppp", MTU: 1500, Flags: net.FlagUp, OperState: "up"}, nil
}

func (o *recordingL2TPDiagnosticOps) Operations() []string {
	return append([]string(nil), o.calls...)
}

func withRecordingL2TPDiagnosticOps(t *testing.T, operations *recordingL2TPDiagnosticOps) {
	t.Helper()
	oldFactory := newL2TPDiagnosticLinuxOps
	oldCheck := checkL2TPDiagnosticPrerequisites
	newL2TPDiagnosticLinuxOps = func() l2tpDiagnosticLinuxOps { return operations }
	checkL2TPDiagnosticPrerequisites = func(string) error { return nil }
	t.Cleanup(func() {
		newL2TPDiagnosticLinuxOps = oldFactory
		checkL2TPDiagnosticPrerequisites = oldCheck
	})
}

func TestPPPoXSockaddrKeepsEveryPackedOffsetAndReservedByte(t *testing.T) {
	options := defaultPPPoXDiagnosticOptions()
	got := packPPPoXSockaddr(3, options)
	want := mustDiagnosticHex(t, "1800010000000000000003000000020006a57f00000100000000000000000100010064006400")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packed sockaddr changed:\n got %x\nwant %x", got, want)
	}
	for _, offset := range []int{22, 23, 24, 25, 26, 27, 28, 29} {
		if got[offset] != 0 {
			t.Fatalf("reserved sockaddr byte %d = %02x", offset, got[offset])
		}
	}
}

func TestTunnelRequestKeepsExactAttributesAndReservedBytes(t *testing.T) {
	request := newTunnelV3Request(42, defaultTunnelDiagnosticOptions())
	got := request.Serialize()[16:]
	want := mustDiagnosticHex(t,
		"01010000080009000100000008000a00640000000500070003000000060002000000000006001a00a506000006001b00a606000008001800ac1e000108001900ac1e0002")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("protocol-v3 payload changed:\n got %x\nwant %x", got, want)
	}
}

func TestPPPoXCreateRequestsKeepExactAttributes(t *testing.T) {
	options := defaultPPPoXDiagnosticOptions()
	tunnel := newPPPoXTunnelRequest(42, 3, options).Serialize()[16:]
	wantTunnel := mustDiagnosticHex(t,
		"01010000080009000100000008000a0064000000050007000200000006000200000000000800170003000000")
	if !reflect.DeepEqual(tunnel, wantTunnel) {
		t.Fatalf("PPPoX tunnel payload changed:\n got %x\nwant %x", tunnel, wantTunnel)
	}
	session := newPPPoXSessionRequest(42, options).Serialize()[16:]
	wantSession := mustDiagnosticHex(t,
		"05010000080009000100000008000b000100000008000c00640000000600010007000000")
	if !reflect.DeepEqual(session, wantSession) {
		t.Fatalf("PPPoX session payload changed:\n got %x\nwant %x", session, wantSession)
	}
}

func TestPPPIoctlNumbersMatchTheLinuxABI(t *testing.T) {
	got := []uint{pppiocGChan, pppiocAttChan, pppiocNewUnit, pppiocConnect}
	want := []uint{0x80047437, 0x40047438, 0xc004743e, 0x4004743a}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PPP ioctl numbers changed: got %#x, want %#x", got, want)
	}
}

func TestPPPoXDiagnosticPreservesTheSocketNetlinkAndIoctlOrder(t *testing.T) {
	operations := newRecordingL2TPDiagnosticOps()
	withRecordingL2TPDiagnosticOps(t, operations)
	report, err := executeL2TPPPoXDiagnostic(defaultPPPoXDiagnosticOptions())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := successfulPPPoXOperations()
	if !reflect.DeepEqual(report.Operations, want) {
		t.Fatalf("operation order changed:\n got %q\nwant %q", report.Operations, want)
	}
	if report.Verdict != L2TPDiagnosticWorking {
		t.Fatalf("verdict = %q", report.Verdict)
	}
}

func successfulPPPoXOperations() []string {
	return []string{
		"socket inet", "setsockopt reuseport", "bind udp", "resolve family",
		"tunnel-create", "tunnel-dump", "session-create", "session-dump",
		"socket pppox", "connect pppox", "ioctl get channel", "open /dev/ppp",
		"ioctl attach channel", "open /dev/ppp", "ioctl new unit",
		"ioctl connect unit", "link ppp0",
	}
}

func operationsThrough(t *testing.T, operations []string, target string, occurrence int) []string {
	t.Helper()
	if occurrence == 0 {
		occurrence = 1
	}
	seen := 0
	for index, operation := range operations {
		if operation != target {
			continue
		}
		seen++
		if seen == occurrence {
			return append([]string(nil), operations[:index+1]...)
		}
	}
	t.Fatalf("operation %q occurrence %d is not in the success sequence", target, occurrence)
	return nil
}

func TestDumpFailureIsANoteAndDoesNotReplaceTheVerdict(t *testing.T) {
	operations := newRecordingL2TPDiagnosticOps()
	operations.fail["tunnel-dump"] = errors.New("dump unavailable")
	withRecordingL2TPDiagnosticOps(t, operations)
	report, err := executeL2TPTunnelDiagnostic(defaultTunnelDiagnosticOptions())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if report.Verdict != L2TPDiagnosticWorking || len(report.Dumps) != 1 || report.Dumps[0].Note != "dump unavailable" {
		t.Fatalf("dump note changed the proof: %#v", report)
	}
	if !strings.Contains(report.Output, "note: L2TP_CMD_TUNNEL_GET dump failed: dump unavailable") {
		t.Fatalf("missing producer note:\n%s", report.Output)
	}
}

func TestCreateFailureIsFatalAndRetainsNoInventedObject(t *testing.T) {
	operations := newRecordingL2TPDiagnosticOps()
	operations.fail["tunnel-create"] = errors.New("file exists")
	withRecordingL2TPDiagnosticOps(t, operations)
	report, err := executeL2TPTunnelDiagnostic(defaultTunnelDiagnosticOptions())
	if err == nil || report.Verdict != "" || len(report.Retained) != 0 {
		t.Fatalf("create failure = report %#v, error %v", report, err)
	}
	if slices.Contains(report.Operations, "tunnel-dump") {
		t.Fatalf("dump ran after fatal create: %q", report.Operations)
	}
}

func TestTunnelFamilyFailureIsFatalBeforeARequestExists(t *testing.T) {
	operations := newRecordingL2TPDiagnosticOps()
	operations.fail["resolve family"] = errors.New("family unavailable")
	withRecordingL2TPDiagnosticOps(t, operations)
	report, err := executeL2TPTunnelDiagnostic(defaultTunnelDiagnosticOptions())
	if err == nil || report.Verdict != "" || report.Output != "" {
		t.Fatalf("family failure = report %#v, error %v", report, err)
	}
	if !reflect.DeepEqual(report.Operations, []string{"resolve family"}) {
		t.Fatalf("operations after family failure = %q", report.Operations)
	}
}

func TestPPPoXPrerequisiteFailureHappensBeforeLinuxSideEffects(t *testing.T) {
	oldFactory := newL2TPDiagnosticLinuxOps
	oldCheck := checkL2TPDiagnosticPrerequisites
	called := false
	newL2TPDiagnosticLinuxOps = func() l2tpDiagnosticLinuxOps {
		called = true
		return newRecordingL2TPDiagnosticOps()
	}
	checkL2TPDiagnosticPrerequisites = func(string) error { return errors.New("CAP_NET_ADMIN unavailable") }
	t.Cleanup(func() {
		newL2TPDiagnosticLinuxOps = oldFactory
		checkL2TPDiagnosticPrerequisites = oldCheck
	})

	report, err := executeL2TPPPoXDiagnostic(defaultPPPoXDiagnosticOptions())
	if err == nil || report.Verdict != "" || called {
		t.Fatalf("prerequisite failure = report %#v, error %v, side effects %t", report, err, called)
	}
	if !strings.HasPrefix(report.Output, "=== L2TP PPPoL2TP Full-Path Diagnostic ===") {
		t.Fatalf("producer heading was lost:\n%s", report.Output)
	}
}

func TestConnectFailureReportsBothRetainedKernelObjects(t *testing.T) {
	operations := newRecordingL2TPDiagnosticOps()
	operations.fail["connect pppox"] = errors.New("protocol error")
	withRecordingL2TPDiagnosticOps(t, operations)
	report, err := executeL2TPPPoXDiagnostic(defaultPPPoXDiagnosticOptions())
	if err != nil {
		t.Fatalf("connect is a proof verdict, not an operating error: %v", err)
	}
	if report.Verdict != L2TPDiagnosticFailed || len(report.Retained) != 2 {
		t.Fatalf("connect failure report = %#v", report)
	}
	for _, object := range report.Retained {
		if !object.Retained {
			t.Fatalf("object was reported cleaned up: %#v", object)
		}
	}
	wantTail := []string{"read /proc/net/pppol2tp", "stat /dev/ppp"}
	if got := report.Operations[len(report.Operations)-2:]; !reflect.DeepEqual(got, wantTail) {
		t.Fatalf("failed-connect diagnostics = %q, want %q", got, wantTail)
	}
}

func TestSessionCreateFailureRetainsOnlyTheTunnel(t *testing.T) {
	operations := newRecordingL2TPDiagnosticOps()
	operations.fail["session-create"] = errors.New("invalid argument")
	withRecordingL2TPDiagnosticOps(t, operations)
	report, err := executeL2TPPPoXDiagnostic(defaultPPPoXDiagnosticOptions())
	if err == nil || report.Verdict != "" {
		t.Fatalf("session create failure = report %#v, error %v", report, err)
	}
	want := []L2TPDiagnosticObject{{Kind: "tunnel", ID: 1, PeerID: 100, Retained: true}}
	if !reflect.DeepEqual(report.Retained, want) {
		t.Fatalf("retained = %#v, want %#v", report.Retained, want)
	}
}

func TestPPPoXDiagnosticRepresentsEverySetupAndIoctlExit(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		failAt      int
		wantError   bool
		wantVerdict L2TPDiagnosticVerdict
		wantText    string
	}{
		{name: "UDP socket", operation: "socket inet", wantError: true},
		{name: "reuse port is advisory", operation: "setsockopt reuseport", wantVerdict: L2TPDiagnosticWorking,
			wantText: "note: SO_REUSEPORT refused"},
		{name: "bind", operation: "bind udp", wantError: true},
		{name: "family", operation: "resolve family", wantError: true},
		{name: "tunnel create", operation: "tunnel-create", wantError: true},
		{name: "tunnel dump is advisory", operation: "tunnel-dump", wantVerdict: L2TPDiagnosticWorking,
			wantText: "(tunnel dump failed:"},
		{name: "session create", operation: "session-create", wantError: true},
		{name: "session dump is advisory", operation: "session-dump", wantVerdict: L2TPDiagnosticWorking,
			wantText: "(session dump failed:"},
		{name: "PPPoX socket", operation: "socket pppox", wantError: true},
		{name: "connect", operation: "connect pppox", wantVerdict: L2TPDiagnosticFailed,
			wantText: "PPPOX CONNECT: FAILED:"},
		{name: "get channel", operation: "ioctl get channel", wantVerdict: L2TPDiagnosticFailed,
			wantText: "PPPIOCGCHAN: FAILED:"},
		{name: "open channel", operation: "open /dev/ppp", failAt: 1, wantError: true},
		{name: "attach channel", operation: "ioctl attach channel", wantVerdict: L2TPDiagnosticFailed,
			wantText: "PPPIOCATTCHAN: FAILED:"},
		{name: "open unit", operation: "open /dev/ppp", failAt: 2, wantError: true},
		{name: "new unit", operation: "ioctl new unit", wantVerdict: L2TPDiagnosticFailed,
			wantText: "PPPIOCNEWUNIT: FAILED:"},
		{name: "connect unit", operation: "ioctl connect unit", wantVerdict: L2TPDiagnosticFailed,
			wantText: "PPPIOCCONNECT: FAILED:"},
		{name: "link read is advisory", operation: "link ppp0", wantVerdict: L2TPDiagnosticWorking,
			wantText: "(link ppp0:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operations := newRecordingL2TPDiagnosticOps()
			operations.fail[test.operation] = errors.New("injected failure")
			operations.failAt[test.operation] = test.failAt
			withRecordingL2TPDiagnosticOps(t, operations)
			report, err := executeL2TPPPoXDiagnostic(defaultPPPoXDiagnosticOptions())
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError %t", err, test.wantError)
			}
			if report.Verdict != test.wantVerdict {
				t.Fatalf("verdict = %q, want %q", report.Verdict, test.wantVerdict)
			}
			if test.wantText != "" && !strings.Contains(report.Output, test.wantText) {
				t.Fatalf("output lacks %q:\n%s", test.wantText, report.Output)
			}
			successOperations := successfulPPPoXOperations()
			wantOperations := operationsThrough(t, successOperations, test.operation, test.failAt)
			switch {
			case test.wantVerdict == L2TPDiagnosticWorking:
				wantOperations = successOperations
			case test.operation == "connect pppox":
				wantOperations = append(wantOperations, "read /proc/net/pppol2tp", "stat /dev/ppp")
			}
			if !reflect.DeepEqual(report.Operations, wantOperations) {
				t.Fatalf("operation sequence after %q changed:\n got %q\nwant %q",
					test.operation, report.Operations, wantOperations)
			}
		})
	}
}

func TestPPPoXDumpRepresentsShortMalformedAndUnknownAttributes(t *testing.T) {
	operations := newRecordingL2TPDiagnosticOps()
	unknown := nl.NewRtAttr(99, []byte{0xaa, 0xbb}).Serialize()
	malformed := []byte{1, genlL2TPVersion, 0, 0, 3, 0, 9, 0}
	operations.dumpReply["tunnel-dump"] = [][]byte{
		{1, 2, 3},
		malformed,
		append([]byte{l2tpCmdTunnelGet, genlL2TPVersion, 0, 0}, unknown...),
	}
	withRecordingL2TPDiagnosticOps(t, operations)
	report, err := executeL2TPPPoXDiagnostic(defaultPPPoXDiagnosticOptions())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, want := range []string{
		"3 byte reply is shorter than the generic netlink header",
		"attributes do not parse:",
		"attr[99]=aabb",
	} {
		if !strings.Contains(report.Output, want) {
			t.Fatalf("dump output lacks %q:\n%s", want, report.Output)
		}
	}
}

func TestL2TPAttributeFormattingKeepsWidthsAddressesAndNestedStats(t *testing.T) {
	short := syscall.NetlinkRouteAttr{Attr: syscall.RtAttr{Type: l2tpAttrConnID}, Value: []byte{1, 2}}
	if got := formatL2TPDiagnosticAttr(short); !strings.Contains(got, "expected 4 bytes") {
		t.Fatalf("short scalar = %q", got)
	}
	address := syscall.NetlinkRouteAttr{Attr: syscall.RtAttr{Type: l2tpAttrIPSAddr}, Value: []byte{192, 0, 2, 1}}
	if got := formatL2TPDiagnosticAttr(address); got != "ip_saddr=192.0.2.1" {
		t.Fatalf("address = %q", got)
	}
	nested := nl.NewRtAttr(1, nl.Uint64Attr(9)).Serialize()
	stats := syscall.NetlinkRouteAttr{Attr: syscall.RtAttr{Type: l2tpAttrStats}, Value: nested}
	if got := formatL2TPDiagnosticAttr(stats); got != "stats: tx_packets=9" {
		t.Fatalf("stats = %q", got)
	}
}

func TestProcAndDeviceFailureDumpsKeepEveryProducerBranch(t *testing.T) {
	boom := errors.New("unavailable")
	if got := procPPPoL2TPDiagnosticText(nil, boom); got != "  (read /proc/net/pppol2tp: unavailable)\n" {
		t.Fatalf("proc error = %q", got)
	}
	if got := procPPPoL2TPDiagnosticText(nil, nil); got != "  (/proc/net/pppol2tp is empty)\n" {
		t.Fatalf("empty proc = %q", got)
	}
	if got := procPPPoL2TPDiagnosticText([]byte("kernel rows\n"), nil); got != "kernel rows\n" {
		t.Fatalf("proc data = %q", got)
	}

	if got := devPPPDiagnosticText(0, 0, unix.Stat_t{}, boom, nil); got != "  (stat /dev/ppp: unavailable)\n" {
		t.Fatalf("device stat error = %q", got)
	}
	if got := devPPPDiagnosticText(0o600, 4, unix.Stat_t{}, nil, nil); !strings.Contains(got, "is not a device node") {
		t.Fatalf("regular device path = %q", got)
	}
	character := os.ModeDevice | os.ModeCharDevice | 0o600
	if got := devPPPDiagnosticText(character, 0, unix.Stat_t{}, nil, boom); !strings.Contains(got, "(unix.Stat /dev/ppp: unavailable)") {
		t.Fatalf("unix.Stat error = %q", got)
	}
	stat := unix.Stat_t{Rdev: unix.Mkdev(108, 0), Uid: 12, Gid: 34}
	if got := devPPPDiagnosticText(character, 0, stat, nil, nil); !strings.Contains(got, "/dev/ppp device 108:0 uid=12 gid=34") {
		t.Fatalf("device identity = %q", got)
	}
}

func mustDiagnosticHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("decode expected hex: %v", err)
	}
	return decoded
}
