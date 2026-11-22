// Design: docs/architecture/traffic/traffic-usage.md -- Readiness: the outcome of the daemon's own attach.
// Related: attach_linux.go -- Available, Attach and markAttachError meet the real kernel.
// Related: monitor.go -- reconcileLocked records the outcome the doctor check reports.

//go:build integration && linux

package trafficusage

import (
	"errors"
	"flag"
	"os"
	"syscall"
	"testing"
	"time"
)

var tcxAttachExpected = flag.String("tcx-attach-expect", "", "Manual runtime proof: attached as root, refused without BPF privileges")

// loopbackIndex is the ifindex the kernel gives lo in every network namespace.
const loopbackIndex = 1

// TestTCXAttachRuntimeOutcome attaches the accounting programs to lo through
// the plugin's own path (Available, then Attach, from the monitor) on the real
// kernel, and checks the recorded outcome, the doctor finding and the kernel's
// errno against an expectation selected before the attach. Run the same
// host-built binary in Ze's system with -tcx-attach-expect=attached as root
// and -tcx-attach-expect=refused as its ordinary user. A requested proof never
// skips. The root run attaches to lo and detaches before it returns.
func TestTCXAttachRuntimeOutcome(t *testing.T) {
	var want attachState
	switch *tcxAttachExpected {
	case "":
		t.Skip("manual runtime proof: supply -tcx-attach-expect=attached as root or =refused without BPF privileges in Ze's system")
	case "attached":
		if os.Geteuid() != 0 {
			t.Fatal("the attached proof requires root in Ze's system")
		}
		want = attachStateAttached
	case "refused":
		if os.Geteuid() == 0 {
			t.Fatal("the refused proof requires Ze's ordinary user without BPF privileges")
		}
		want = attachStateRefused
	default:
		t.Fatalf("invalid -tcx-attach-expect=%q; use attached or refused", *tcxAttachExpected)
	}

	m := newMonitor(newAttacher(), stubResolver(map[string]int{"lo": loopbackIndex}))
	defer m.Stop()
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(defaultMaxEntries, false, "lo")})

	m.mu.Lock()
	got := m.outcomes["lo"]
	m.mu.Unlock()
	t.Logf("uid=%d expected=%v measured=%v cause=%q fix=%q", os.Geteuid(), want, got.state, got.cause, got.remedy)
	if got.state != want {
		t.Fatalf("attach outcome = %v, want %v; cause: %s", got.state, want, got.cause)
	}
	findings, err := m.doctorFindings()
	if err != nil {
		t.Fatalf("doctorFindings: %v", err)
	}
	if want == attachStateAttached {
		if len(findings) != 0 {
			t.Fatalf("an attached interface reports %v", findings)
		}
		return
	}
	if len(findings) != 1 || findings[0].Code != codeAttachRefused || findings[0].Fix == "" {
		t.Fatalf("a refused attach reports %v, want one %s finding with a fix", findings, codeAttachRefused)
	}

	// The recorded cause is text; the errno is read from the attacher's own
	// error, through the same two steps the monitor takes.
	att := newAttacher()
	kernelErr := att.Available()
	if kernelErr == nil {
		_, kernelErr = att.Attach(loopbackIndex, "lo", defaultMaxEntries, false)
	}
	if !errors.Is(kernelErr, syscall.EPERM) && !errors.Is(kernelErr, syscall.EACCES) {
		t.Fatalf("the refusal did not preserve the kernel's permission errno: %v", kernelErr)
	}
}
