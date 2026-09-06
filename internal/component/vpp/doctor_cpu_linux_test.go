//go:build linux

package vpp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestEvaluateVPPCPUIsolation covers what the vpp-cpu-isolation doctor check
// reports for each host it can meet.
//
// VALIDATES: AC-5 -- worker cores outside the kernel's isolated set are
// reported, and so is a host that would not say which cores are isolated.
// PREVENTS: the silent case this feature exists to remove: VPP workers
// busy-polling on CPUs the Linux scheduler still owns, with nothing telling the
// operator.
func TestEvaluateVPPCPUIsolation(t *testing.T) {
	eightOnline := []uint8{0, 1, 2, 3, 4, 5, 6, 7}

	tests := []struct {
		name        string
		cpu         CPUSettings
		inv         CPUInventory
		wantCount   int
		wantMessage string
	}{
		{
			name: "workers on isolated cores are silent",
			cpu:  CPUSettings{MainCore: new(uint8(0)), Workers: new(uint8(2))},
			inv: CPUInventory{
				Online:         eightOnline,
				Isolated:       []uint8{2, 3, 4},
				IsolationKnown: true,
			},
			wantCount: 0,
		},
		{
			name: "workers on cores Linux still schedules are reported",
			cpu:  CPUSettings{MainCore: new(uint8(0)), Workers: new(uint8(2))},
			inv: CPUInventory{
				Online:         eightOnline,
				IsolationKnown: true,
			},
			wantCount:   1,
			wantMessage: "worker core 1-2 is not isolated",
		},
		{
			name: "an explicit list outside the isolated set is reported",
			cpu:  CPUSettings{MainCore: new(uint8(0)), WorkerCores: []uint8{2, 6}},
			inv: CPUInventory{
				Online:         eightOnline,
				Isolated:       []uint8{2, 3, 4},
				IsolationKnown: true,
			},
			wantCount:   1,
			wantMessage: "worker core 6 is not isolated",
		},
		{
			name: "a host that will not say is reported",
			cpu:  CPUSettings{MainCore: new(uint8(0)), Workers: new(uint8(2))},
			inv: CPUInventory{
				Online: eightOnline,
			},
			wantCount:   1,
			wantMessage: "does not report which CPUs are isolated",
		},
		{
			name: "no CPU left for Linux is reported",
			cpu:  CPUSettings{MainCore: new(uint8(1)), Workers: new(uint8(2))},
			inv: CPUInventory{
				Online:         []uint8{0, 1, 2, 3},
				Isolated:       []uint8{0, 1, 2, 3},
				IsolationKnown: true,
			},
			wantCount:   1,
			wantMessage: "the Linux control plane has no CPU of its own",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := evaluateVPPCPUIsolation(&tt.cpu, tt.inv)
			if len(diags) != tt.wantCount {
				t.Fatalf("got %d diagnostics %v, want %d", len(diags), diags, tt.wantCount)
			}
			if tt.wantCount == 0 {
				return
			}
			if diags[0].Code != doctorVPPCPUIsolationCode {
				t.Errorf("code = %q, want %q", diags[0].Code, doctorVPPCPUIsolationCode)
			}
			if diags[0].Severity != diagnostic.SeverityWarning {
				t.Errorf("severity = %v, want warning", diags[0].Severity)
			}
			if !strings.Contains(diags[0].Message, tt.wantMessage) {
				t.Errorf("message = %q, want it to contain %q", diags[0].Message, tt.wantMessage)
			}
		})
	}
}

// TestEvaluateVPPCPUIsolationUnknownBeatsEmpty is the guard test for the defect
// this feature is most exposed to. Both hosts below hold an empty Isolated
// slice, and they mean opposite things: one isolated nothing, the other could
// not be asked. A check that reads the second as the first goes quiet on the
// host where the operator most needs to hear from it.
func TestEvaluateVPPCPUIsolationUnknownBeatsEmpty(t *testing.T) {
	cpu := CPUSettings{MainCore: new(uint8(0)), Workers: new(uint8(1))}
	online := []uint8{0, 1, 2, 3}

	known := evaluateVPPCPUIsolation(&cpu, CPUInventory{Online: online, IsolationKnown: true})
	unknown := evaluateVPPCPUIsolation(&cpu, CPUInventory{Online: online})

	if len(known) != 1 || len(unknown) != 1 {
		t.Fatalf("known %v, unknown %v: both hosts owe the operator a diagnostic", known, unknown)
	}
	if known[0].Message == unknown[0].Message {
		t.Errorf("both hosts produced %q; a host that isolated nothing and a host that could not be asked are different answers", known[0].Message)
	}
}
