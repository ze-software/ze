package tftpserver

import "testing"

// TestBindDeviceForFallsBackToName pins the install/provision fix: when no
// iface backend is loaded (GetBackend()==nil, as in the `ze-setup install
// remote` scenario where provision configures the interface directly via
// netlink), bindDeviceFor returns the configured interface name verbatim so the
// listener still binds. The previous code treated the resolve error as fatal
// and dropped the interface, leaving the TFTP server with zero listeners.
func TestBindDeviceForFallsBackToName(t *testing.T) {
	for _, name := range []string{"enp2s0", "lo", "eth0"} {
		got, err := bindDeviceFor(name)
		if got != name {
			t.Errorf("bindDeviceFor(%q) device = %q, want %q (no backend -> verbatim name)", name, got, name)
		}
		// With no iface backend loaded (the unit-test default, as in the
		// install/provision scenario) Resolve must surface an error so the
		// caller can log the fallback.
		if err == nil {
			t.Errorf("bindDeviceFor(%q) err = nil, want non-nil (no backend loaded)", name)
		}
	}
}
