// VALIDATES: the doctor check warns only when traffic-usage is enabled AND eBPF
// is unavailable, and stays silent otherwise.
// PREVENTS: a false-positive warning on an unconfigured/healthy system, or a
// silent failure when the operator enables accounting on a kernel without TCX.

package trafficusage

import (
	"errors"
	"testing"
)

func TestTrafficUsageDiagnostic(t *testing.T) {
	if d := trafficUsageDiagnostic(false, errors.New("unsupported")); d != nil {
		t.Errorf("disabled -> no diagnostic, got %v", d)
	}
	if d := trafficUsageDiagnostic(true, nil); d != nil {
		t.Errorf("available -> no diagnostic, got %v", d)
	}
	d := trafficUsageDiagnostic(true, errors.New("no CAP_BPF"))
	if len(d) != 1 {
		t.Fatalf("enabled + unavailable -> 1 diagnostic, got %d", len(d))
	}
	if d[0].Code != "doctor-traffic-usage-ebpf" {
		t.Errorf("code = %q, want doctor-traffic-usage-ebpf", d[0].Code)
	}
	if d[0].Severity != "warning" {
		t.Errorf("severity = %q, want warning", d[0].Severity)
	}
}
