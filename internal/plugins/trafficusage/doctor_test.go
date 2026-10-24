// VALIDATES: the doctor check warns when traffic-usage is enabled and the host
// cannot attach eBPF TCX programs, says Unknown with the measuring command when
// the probe could not ask the kernel, and stays silent otherwise.
// PREVENTS: a readiness check that cannot fail: the Linux probe used to be a
// constant nil, so the one platform where TCX can be missing always read ready.
package trafficusage

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
)

func TestTrafficUsageDiagnostic(t *testing.T) {
	absent := tcxReadinessResult{state: tcxAbsent, reason: errors.New("unsupported")}
	if d := trafficUsageDiagnostic(false, absent); d != nil {
		t.Errorf("disabled -> no diagnostic, got %v", d)
	}
	if d := trafficUsageDiagnostic(true, tcxReadinessResult{state: tcxReady}); d != nil {
		t.Errorf("ready -> no diagnostic, got %v", d)
	}
	d := trafficUsageDiagnostic(true, tcxReadinessResult{state: tcxAbsent, reason: errNoTCX})
	if len(d) != 1 {
		t.Fatalf("enabled + absent -> 1 diagnostic, got %d", len(d))
	}
	if d[0].Code != "doctor-traffic-usage-ebpf" {
		t.Errorf("code = %q, want doctor-traffic-usage-ebpf", d[0].Code)
	}
	if d[0].Severity != "warning" {
		t.Errorf("severity = %q, want warning", d[0].Severity)
	}
}

// TestTrafficUsageReadinessCanFail checks that the readiness answer is read from
// what the kernel said to the probe, and that each kernel answer reaches a
// distinct verdict: the kernel's "no such device" for the probe's absent
// interface means TCX exists, "not supported" means it does not, and a refused
// probe means nobody knows yet, so the operator is told how to measure it.
func TestTrafficUsageReadinessCanFail(t *testing.T) {
	cases := []struct {
		name  string
		probe error
		want  tcxReadiness
	}{
		{"kernel answered ENODEV for the probe interface", fmt.Errorf("attach tcx link: %w", syscall.ENODEV), tcxReady},
		{"kernel has no TCX attach", fmt.Errorf("attach: %w", errNoTCX), tcxAbsent},
		{"process may not load a program", fmt.Errorf("load program: %w", syscall.EPERM), tcxUnknown},
		{"process may not attach", syscall.EACCES, tcxUnknown},
		{"any other errno", syscall.EINVAL, tcxUnknown},
	}
	for _, tc := range cases {
		got := classifyTCXProbe(tc.probe)
		if got.state != tc.want {
			t.Errorf("%s: state = %v, want %v", tc.name, got.state, tc.want)
		}
		if got.reason == nil {
			t.Errorf("%s: no reason carried", tc.name)
		}
	}

	unknown := trafficUsageDiagnostic(true, classifyTCXProbe(syscall.EPERM))
	if len(unknown) != 1 {
		t.Fatalf("enabled + unknown -> 1 diagnostic, got %d", len(unknown))
	}
	if unknown[0].Code != "doctor-traffic-usage-ebpf-unknown" {
		t.Errorf("code = %q, want doctor-traffic-usage-ebpf-unknown", unknown[0].Code)
	}
	if !strings.Contains(unknown[0].Message, "sudo ze doctor") {
		t.Errorf("unknown message names no measuring command: %q", unknown[0].Message)
	}

	unset := trafficUsageDiagnostic(true, tcxReadinessResult{})
	if len(unset) != 1 || unset[0].Code != "doctor-traffic-usage-ebpf-unknown" {
		t.Errorf("a probe with no verdict must read unknown, got %v", unset)
	}
}
