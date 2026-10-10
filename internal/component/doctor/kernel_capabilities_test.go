package doctor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/kernelcap"
)

// VALIDATES: AC-1's answer shape. `ze doctor kernel-capabilities --json` writes
// one row per enrolled capability under "capabilities", with the kebab-case
// keys a Docker-host check reads, and "ready" is true only when every row is
// present.
// PREVENTS: a host check that proceeds on an unknown or absent row, or on an
// answer with no rows at all, which is what a build with no enrolment gives and
// which proves nothing about the kernel.
func TestKernelCapabilitiesAnswer(t *testing.T) {
	present := kernelcap.Row{Subsystem: "alpha", Kernel: "CONFIG_ALPHA", State: "present"}
	absent := kernelcap.Row{Subsystem: "bravo", Kernel: "CONFIG_BRAVO", State: "absent", Reason: "no such family"}
	unknown := kernelcap.Row{Subsystem: "charlie", Kernel: "CONFIG_CHARLIE", State: "unknown", Reason: "EPERM"}
	denied := kernelcap.Row{Subsystem: "delta", Kernel: "CONFIG_DELTA", State: "denied", Reason: "AppArmor profile docker-default (enforce)"}

	for _, tc := range []struct {
		name  string
		rows  []kernelcap.Row
		ready bool
	}{
		{"all present", []kernelcap.Row{present}, true},
		{"one absent", []kernelcap.Row{present, absent}, false},
		{"one unknown", []kernelcap.Row{present, unknown}, false},
		// AC-16: a probe the host's policy denied is not a pass for a Docker host.
		{"one denied", []kernelcap.Row{present, denied}, false},
		{"nothing enrolled", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			ready, err := writeKernelCapabilities(&out, tc.rows, true)
			if err != nil {
				t.Fatalf("write: %v", err)
			}
			if ready != tc.ready {
				t.Errorf("ready = %v, want %v", ready, tc.ready)
			}
			var answer struct {
				Ready        bool             `json:"ready"`
				Capabilities []map[string]any `json:"capabilities"`
			}
			if err := json.Unmarshal(out.Bytes(), &answer); err != nil {
				t.Fatalf("answer is not JSON: %v\n%s", err, out.String())
			}
			if answer.Ready != tc.ready {
				t.Errorf("JSON ready = %v, want %v", answer.Ready, tc.ready)
			}
			if len(answer.Capabilities) != len(tc.rows) {
				t.Fatalf("%d JSON rows for %d capabilities", len(answer.Capabilities), len(tc.rows))
			}
			if answer.Capabilities == nil {
				t.Error("capabilities is null; a consumer must read an empty list, never a missing one")
			}
			for i, row := range tc.rows {
				got := answer.Capabilities[i]
				if got["subsystem"] != row.Subsystem || got["kernel"] != row.Kernel || got["state"] != row.State {
					t.Errorf("row %d = %v, want %+v", i, got, row)
				}
			}
		})
	}
}

// VALIDATES: the text form names each row's state, subsystem, symbol and reason,
// and says why a build with nothing enrolled is not ready.
// PREVENTS: an operator reading "ready" off an empty list.
func TestKernelCapabilitiesText(t *testing.T) {
	var out bytes.Buffer
	rows := []kernelcap.Row{
		{Subsystem: "alpha", Kernel: "CONFIG_ALPHA", State: "present"},
		{Subsystem: "bravo", Kernel: "CONFIG_BRAVO", State: "absent", Reason: "no such family"},
	}
	ready, err := writeKernelCapabilities(&out, rows, false)
	if err != nil || ready {
		t.Fatalf("ready=%v err=%v, want not ready", ready, err)
	}
	for _, want := range []string{"present", "alpha", "CONFIG_ALPHA", "absent", "bravo", "no such family", "not ready"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("text does not name %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	if ready, _ := writeKernelCapabilities(&out, nil, false); ready {
		t.Error("an empty enrolment reported ready")
	}
	if !strings.Contains(out.String(), "no kernel capability is enrolled") {
		t.Errorf("empty enrolment not explained:\n%s", out.String())
	}
}
