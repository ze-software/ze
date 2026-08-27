//go:build linux && ze_test

// Design: test/draft/ui/le-l2tp-diagnostics-answers.ci -- built-binary syscall recording
// Related: l2tpdiag_linux_ops.go -- the real Linux boundary
//
// This file compiles only into a ze_test binary. Production builds compile the
// real syscall boundary in l2tpdiag_linux_ops.go instead.

package deployment

import (
	"encoding/hex"
	"encoding/json"
	"net"
	"os"

	nl "github.com/vishvananda/netlink/nl"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const l2tpDiagnosticRecordEnv = "ZE_TEST_L2TP_DIAGNOSTIC_RECORD"

func l2tpDiagnosticRecordingEnabled() bool { return os.Getenv(l2tpDiagnosticRecordEnv) != "" }

func configuredL2TPDiagnosticLinuxOps() l2tpDiagnosticLinuxOps {
	if path := os.Getenv(l2tpDiagnosticRecordEnv); path != "" {
		return newRecordedL2TPDiagnosticOps(path)
	}
	return newSystemL2TPDiagnosticOps()
}

type recordedL2TPDiagnosticOps struct {
	path           string
	calls          []string
	nextFD         int
	tunnelMessage  []byte
	sessionMessage []byte
}

type recordedL2TPDiagnosticCall struct {
	Operation string `json:"operation"`
	Payload   string `json:"payload,omitempty"`
}

func newRecordedL2TPDiagnosticOps(path string) *recordedL2TPDiagnosticOps {
	return &recordedL2TPDiagnosticOps{path: path, calls: []string{}, nextFD: 3}
}

func (o *recordedL2TPDiagnosticOps) record(operation string, payload []byte) error {
	call := recordedL2TPDiagnosticCall{Operation: operation}
	if len(payload) > 0 {
		call.Payload = hex.EncodeToString(payload)
	}
	encoded, err := json.Marshal(call)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	file, err := os.OpenFile(o.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(encoded); err != nil {
		file.Close() //nolint:errcheck // preserve the write error
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	o.calls = append(o.calls, operation)
	return nil
}

func (o *recordedL2TPDiagnosticOps) Socket(domain, _, _ int) (int, error) {
	operation := "socket inet"
	if domain == afPPPOX {
		operation = "socket pppox"
	}
	if err := o.record(operation, nil); err != nil {
		return 0, err
	}
	fd := o.nextFD
	o.nextFD++
	return fd, nil
}

func (o *recordedL2TPDiagnosticOps) SetReusePort(int) error {
	return o.record("setsockopt reuseport", nil)
}

func (o *recordedL2TPDiagnosticOps) BindIPv4(_ int, address [4]byte, port uint16) error {
	payload := []byte{address[0], address[1], address[2], address[3], byte(port >> 8), byte(port)}
	return o.record("bind udp", payload)
}

func (o *recordedL2TPDiagnosticOps) Family(string) (uint16, error) {
	if err := o.record("resolve family", nil); err != nil {
		return 0, err
	}
	return 42, nil
}

func (o *recordedL2TPDiagnosticOps) Execute(operation string, request *nl.NetlinkRequest) ([][]byte, error) {
	raw := request.Serialize()
	payload := append([]byte(nil), raw[16:]...)
	if err := o.record(operation, payload); err != nil {
		return nil, err
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

func (o *recordedL2TPDiagnosticOps) Connect(_ int, address []byte) error {
	return o.record("connect pppox", address)
}

func (o *recordedL2TPDiagnosticOps) IoctlGetInt(int, uint) (int, error) {
	if err := o.record("ioctl get channel", nil); err != nil {
		return 0, err
	}
	return 7, nil
}

func (o *recordedL2TPDiagnosticOps) IoctlSetInt(_ int, request uint, _ int) error {
	if request == pppiocAttChan {
		return o.record("ioctl attach channel", nil)
	}
	return o.record("ioctl connect unit", nil)
}

func (o *recordedL2TPDiagnosticOps) IoctlGetSetInt(int, uint, int) (int, error) {
	if err := o.record("ioctl new unit", nil); err != nil {
		return 0, err
	}
	return 0, nil
}

func (o *recordedL2TPDiagnosticOps) OpenPPP() (int, error) {
	if err := o.record("open /dev/ppp", nil); err != nil {
		return 0, err
	}
	fd := o.nextFD
	o.nextFD++
	return fd, nil
}

func (o *recordedL2TPDiagnosticOps) ProcPPPoL2TP() string {
	if err := o.record("read /proc/net/pppol2tp", nil); err != nil {
		var tb textbuf.Buffer
		return tb.Str("  (record /proc/net/pppol2tp: ").Err(err).Str(")\n").String()
	}
	return "recorded pppol2tp state\n"
}

func (o *recordedL2TPDiagnosticOps) DevPPP() string {
	if err := o.record("stat /dev/ppp", nil); err != nil {
		var tb textbuf.Buffer
		return tb.Str("  (record /dev/ppp: ").Err(err).Str(")\n").String()
	}
	return "  /dev/ppp mode=Dcrw------- size=0\n  /dev/ppp device 108:0 uid=0 gid=0\n"
}

func (o *recordedL2TPDiagnosticOps) Link(name string) (diagnosticLink, error) {
	var tb textbuf.Buffer
	if err := o.record(tb.Str("link ").Str(name).String(), nil); err != nil {
		return diagnosticLink{}, err
	}
	return diagnosticLink{Name: name, Index: 9, Type: "ppp", MTU: 1500, Flags: net.FlagUp, OperState: "up"}, nil
}

func (o *recordedL2TPDiagnosticOps) Operations() []string {
	return append([]string(nil), o.calls...)
}

