// Design: docs/architecture/appliance/installer-initrd.md -- R-6 fault-injection hook tests

//go:build linux && ze_installer && ze_installer_fault

package disk

import "testing"

func TestParseFaultParam(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"absent", "console=ttyS0 ze.server=10.0.0.1 panic=-1", ""},
		{"panic-goroutine", "ip=dhcp ze.fault=panic-goroutine panic=-1", "panic-goroutine"},
		{"first-wins", "ze.fault=a ze.fault=b", "a"},
		{"empty-value", "ze.fault= panic=-1", ""},
		{"trailing", "panic=-1 ze.fault=panic-goroutine", "panic-goroutine"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseFaultParam(tc.line); got != tc.want {
				t.Fatalf("parseFaultParam(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

// injectFault must be a no-op (return promptly, no panic, no block) for an
// empty or unrecognised fault value. The "panic-goroutine" path is proven by
// the QEMU fault-injection scenario, not here: it deliberately blocks the
// caller and reboots.
func TestInjectFaultNoopForUnknown(t *testing.T) {
	cfg := InstallConfig{Source: sourceHTTP}
	injectFault(cfg, "")            // empty: nothing armed
	injectFault(cfg, "not-a-fault") // unknown: logged and ignored
	// Reaching here without blocking or panicking is the assertion.
}
