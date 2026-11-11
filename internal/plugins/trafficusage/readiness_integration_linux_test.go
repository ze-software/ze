// Design: docs/architecture/traffic/traffic-usage.md -- Readiness: the doctor asks the kernel.
// Related: attach_linux.go -- probeTCX performs the real load and attach.

//go:build integration && linux

package trafficusage

import (
	"errors"
	"flag"
	"os"
	"syscall"
	"testing"
)

var tcxProbeExpected = flag.String("tcx-probe-expect", "", "Manual runtime proof: ready as root, unknown without BPF privileges")

// TestTCXProbeRuntimeReadiness asks the real kernel and checks the verdict and
// original errno against an expectation selected before the probe. Run the same
// host-built binary in Ze's system with -tcx-probe-expect=ready as root and
// -tcx-probe-expect=unknown as its ordinary user. A requested proof never skips.
func TestTCXProbeRuntimeReadiness(t *testing.T) {
	var want tcxReadiness
	switch *tcxProbeExpected {
	case "":
		t.Skip("manual runtime proof: supply -tcx-probe-expect=ready as root or =unknown without BPF privileges in Ze's system")
	case "ready":
		if os.Geteuid() != 0 {
			t.Fatal("the ready proof requires root in Ze's system")
		}
		want = tcxReady
	case "unknown":
		if os.Geteuid() == 0 {
			t.Fatal("the unknown proof requires Ze's ordinary user without BPF privileges")
		}
		want = tcxUnknown
	default:
		t.Fatalf("invalid -tcx-probe-expect=%q; use ready or unknown", *tcxProbeExpected)
	}

	got := probeTCX()
	t.Logf("uid=%d expected=%v measured=%v reason=%v", os.Geteuid(), want, got.state, got.reason)
	if got.state != want {
		t.Fatalf("probeTCX readiness = %v, want %v; reason: %v", got.state, want, got.reason)
	}
	if got.reason == nil {
		t.Fatal("probeTCX lost the kernel's evidence")
	}
	if want == tcxReady {
		if !errors.Is(got.reason, syscall.ENODEV) {
			t.Fatalf("the ready probe did not preserve the absent-interface ENODEV: %v", got.reason)
		}
		return
	}
	if errors.Is(got.reason, syscall.EPERM) {
		return
	}
	if !errors.Is(got.reason, syscall.EACCES) {
		t.Fatalf("the unknown probe did not preserve a permission refusal: %v", got.reason)
	}
}
