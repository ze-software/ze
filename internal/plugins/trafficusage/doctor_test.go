// VALIDATES: the traffic-usage-attach runtime check reports, per configured
// interface, what the monitor's last attach established: a permission refusal
// with the remedy privilege.CapabilityRemedy names, a kernel without TCX, an
// attach whose answer names no cause, and a configured interface that does not
// exist; an attached or down interface reports nothing, and a disabled
// traffic-usage probes nothing and reports nothing. A failure is logged once
// per change of outcome, with the code the check reports it under.
// PREVENTS: a readiness verdict measured in a process that does not load the
// programs; a systemd repair offered where ze never established it runs under
// the service; an attach refusal logged once a second with no doctor code; a
// disabled feature probed for eBPF capabilities.
package trafficusage

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// fakeRemedy stands in privilege.CapabilityRemedy, naming the capabilities it
// was asked for so a test sees what the monitor asked.
func fakeRemedy(missing ...string) string {
	return "remedy for " + strings.Join(missing, "+")
}

// captureLog routes the package logger into a buffer for one test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := logger()
	setLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { setLogger(previous) })
	return &buf
}

// presence is a resolver answer for one interface.
type presence struct {
	index int
	up    bool
}

// presenceResolver resolves the mapped names, up or down; any other name is
// absent.
func presenceResolver(m map[string]presence) ifaceResolver {
	return func(name string) (int, bool, bool) {
		p, ok := m[name]
		return p.index, p.up, ok
	}
}

// TestAttachOutcomesReportedPerInterface drives one reconcile over six
// configured interfaces, each reaching a different outcome, and reads the
// runtime check's answer.
func TestAttachOutcomesReportedPerInterface(t *testing.T) {
	captureLog(t)
	fa := newFakeAttacher()
	fa.failWith["eth0"] = fmt.Errorf("load eBPF collection: %w", syscall.EPERM)
	fa.failWith["eth1"] = fmt.Errorf("attach TCX ingress: %w", errNoTCX)
	fa.failWith["eth2"] = fmt.Errorf("attach TCX ingress: %w", syscall.EINVAL)
	m := newMonitor(fa, presenceResolver(map[string]presence{
		"eth0": {10, true}, "eth1": {11, true}, "eth2": {12, true},
		"eth4": {14, true}, "eth5": {15, false},
	}))
	m.remedy = fakeRemedy
	cfg := &Config{Enabled: true, Interval: time.Second,
		Interfaces: ifcList(1024, false, "eth0", "eth1", "eth2", "eth3", "eth4", "eth5")}
	if err := m.Reconcile(cfg); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	findings, err := m.doctorFindings()
	if err != nil {
		t.Fatalf("doctorFindings: %v", err)
	}
	want := []struct {
		iface, code, fix string
	}{
		{"eth0", codeAttachRefused, "remedy for CAP_BPF+CAP_NET_ADMIN"},
		{"eth1", codeTCXAbsent, "set traffic usage enabled false"},
		{"eth2", codeAttachUnknown, "ze show log recent level error component traffic.usage"},
		{"eth3", codeInterfaceMissing, "delete traffic usage interfaces interface eth3"},
	}
	if len(findings) != len(want) {
		t.Fatalf("findings = %d (%v), want %d: attached eth4 and down eth5 report nothing", len(findings), findings, len(want))
	}
	declared := attachCheckDecl().Codes
	for i, w := range want {
		got := findings[i]
		if got.Code != w.code {
			t.Errorf("finding %d: code = %q, want %q", i, got.Code, w.code)
		}
		if !strings.Contains(got.Message, w.iface) {
			t.Errorf("finding %d: message does not name %s: %q", i, w.iface, got.Message)
		}
		if got.Fix != w.fix {
			t.Errorf("finding %d: fix = %q, want %q", i, got.Fix, w.fix)
		}
		if got.Severity != "warning" {
			t.Errorf("finding %d: severity = %q, want warning", i, got.Severity)
		}
		if !slices.Contains(declared, got.Code) {
			t.Errorf("finding %d: code %q is not declared by the check", i, got.Code)
		}
	}
	if !strings.Contains(findings[0].Message, syscall.EPERM.Error()) {
		t.Errorf("the refusal does not carry the kernel's answer: %q", findings[0].Message)
	}
	if _, ok := fa.attached[14]; !ok {
		t.Error("eth4 should attach beside the failing interfaces")
	}
}

// TestRefusalRemedyIsPrivilegeCapabilityRemedy checks that the production
// monitor takes its fix from privilege.CapabilityRemedy, which decides whether
// the systemd repair applies, and that only a permission refusal carries it.
func TestRefusalRemedyIsPrivilegeCapabilityRemedy(t *testing.T) {
	captureLog(t)
	fa := newFakeAttacher()
	fa.failWith["eth0"] = syscall.EACCES
	fa.failWith["eth1"] = syscall.EINVAL
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10, "eth1": 11}))
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0", "eth1")})

	findings, err := m.doctorFindings()
	if err != nil {
		t.Fatalf("doctorFindings: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(findings))
	}
	if want := privilege.CapabilityRemedy("CAP_BPF", "CAP_NET_ADMIN"); findings[0].Fix != want {
		t.Errorf("refusal fix = %q, want privilege.CapabilityRemedy's %q", findings[0].Fix, want)
	}
	if findings[1].Fix == findings[0].Fix {
		t.Errorf("an unknown failure carries the capability remedy %q", findings[1].Fix)
	}
}

// TestAttachRefusalLoggedOncePerChange runs the snapshot tick over a refused
// interface and checks the refusal is logged once and not retried, that a
// configuration apply retries it without logging an unchanged outcome, and
// that a changed outcome is logged once more.
func TestAttachRefusalLoggedOncePerChange(t *testing.T) {
	logs := captureLog(t)
	fa := newFakeAttacher()
	fa.failWith["eth0"] = fmt.Errorf("attach TCX ingress: %w", syscall.EPERM)
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10}))
	m.remedy = fakeRemedy
	cfg := &Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0")}

	_ = m.Reconcile(cfg)
	for range 3 {
		m.onSnapshot(nil)
	}
	if n := strings.Count(logs.String(), codeAttachRefused); n != 1 {
		t.Fatalf("refusal logged %d times over a reconcile and three snapshots, want 1:\n%s", n, logs)
	}
	if fa.attempts != 1 {
		t.Fatalf("attach attempts = %d, want 1: a snapshot must not retry a settled refusal", fa.attempts)
	}

	_ = m.Reconcile(cfg)
	if fa.attempts != 2 {
		t.Fatalf("attach attempts = %d after a configuration apply, want 2", fa.attempts)
	}
	if n := strings.Count(logs.String(), codeAttachRefused); n != 1 {
		t.Fatalf("an unchanged refusal was logged again on reload: %d lines", n)
	}

	fa.failWith["eth0"] = fmt.Errorf("attach TCX ingress: %w", syscall.EINVAL)
	_ = m.Reconcile(cfg)
	m.onSnapshot(nil)
	m.onSnapshot(nil)
	if fa.attempts != 5 {
		t.Fatalf("attach attempts = %d, want 5: an unknown failure is retried every snapshot", fa.attempts)
	}
	if n := strings.Count(logs.String(), codeAttachUnknown); n != 1 {
		t.Fatalf("unknown failure logged %d times, want 1 for one change of outcome", n)
	}

	delete(fa.failWith, "eth0")
	m.onSnapshot(nil)
	if _, ok := fa.attached[10]; !ok {
		t.Fatal("eth0 not attached once the kernel accepted it")
	}
	if findings, err := m.doctorFindings(); err != nil || len(findings) != 0 {
		t.Fatalf("an attached interface reports %v (%v), want nothing", findings, err)
	}
}

// TestAvailabilityFailureIsTheOutcome checks that the attacher's availability
// is asked once per pass, only when an interface is to be attached, and that
// its refusal is each interface's outcome without an attach attempt.
func TestAvailabilityFailureIsTheOutcome(t *testing.T) {
	captureLog(t)
	fa := newFakeAttacher()
	fa.available = fmt.Errorf("remove memlock rlimit: %w", syscall.EPERM)
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10, "eth1": 11}))
	m.remedy = fakeRemedy
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0", "eth1")})

	if fa.availableCalls != 1 {
		t.Errorf("Available calls = %d, want 1 per pass", fa.availableCalls)
	}
	if fa.attempts != 0 {
		t.Errorf("attach attempts = %d after availability refused, want 0", fa.attempts)
	}
	findings, err := m.doctorFindings()
	if err != nil {
		t.Fatalf("doctorFindings: %v", err)
	}
	if len(findings) != 2 || findings[0].Code != codeAttachRefused || findings[1].Code != codeAttachRefused {
		t.Fatalf("findings = %v, want two refusals", findings)
	}
}

// TestDisabledTrafficUsageProbesAndReportsNothing is the negative half: with
// traffic-usage disabled, configured interfaces are neither probed nor
// reported, and disabling it clears the outcomes an enabled run recorded.
func TestDisabledTrafficUsageProbesAndReportsNothing(t *testing.T) {
	captureLog(t)
	fa := newFakeAttacher()
	fa.available = syscall.EPERM
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10}))
	m.remedy = fakeRemedy

	_ = m.Reconcile(&Config{Enabled: false, Interfaces: ifcList(1024, false, "eth0")})
	m.onSnapshot(nil)
	if fa.availableCalls != 0 || fa.attempts != 0 {
		t.Fatalf("disabled: Available called %d times, Attach %d, want neither", fa.availableCalls, fa.attempts)
	}
	if findings, err := m.doctorFindings(); err != nil || len(findings) != 0 {
		t.Fatalf("disabled: findings %v (%v), want none", findings, err)
	}

	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0")})
	if findings, _ := m.doctorFindings(); len(findings) != 1 {
		t.Fatalf("enabled and refused: findings %v, want one", findings)
	}
	_ = m.Reconcile(&Config{Enabled: false})
	if findings, err := m.doctorFindings(); err != nil || len(findings) != 0 {
		t.Fatalf("disabled again: findings %v (%v), want none", findings, err)
	}
}

// TestDoctorFindingsBeforeConfiguration checks that the check says it cannot
// answer before the plugin received its configuration, rather than reading an
// empty outcome set as every interface being accounted.
func TestDoctorFindingsBeforeConfiguration(t *testing.T) {
	m := newMonitor(newFakeAttacher(), stubResolver(nil))
	findings, err := m.doctorFindings()
	if !errors.Is(err, errNotConfigured) {
		t.Fatalf("before configuration: findings %v, err %v, want errNotConfigured", findings, err)
	}
}

// TestAttachCheckCodesAreExplained checks the check is declared in the runtime
// phase and that init registered an explanation for every code it declares.
func TestAttachCheckCodesAreExplained(t *testing.T) {
	decl := attachCheckDecl()
	if decl.Phase != rpc.DoctorPhaseRuntime {
		t.Errorf("phase = %q, want runtime: the outcomes live in the plugin's process", decl.Phase)
	}
	for _, code := range decl.Codes {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("code %s has no registered explanation", code)
		}
	}
}
