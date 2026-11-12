// Design: docs/architecture/system-architecture.md -- the capabilities the daemon runs with
// Related: capabilities.go -- daemonCapabilities, the declaration under test

//go:build linux

package privilege

import (
	"testing"

	"golang.org/x/sys/unix"
)

// TestDaemonCapabilityBitsAreTheKernels compares each capability the daemon
// declares with the constant golang.org/x/sys/unix generates from the kernel's
// include/uapi/linux/capability.h for its name.
//
// VALIDATES: every Bit in daemonCapabilities is the kernel's number for its Name,
// no name or bit appears twice, and each bit fits the 64-bit CapEff mask.
// PREVENTS: a hand-written capability number that is wrong, so the systemd unit
// names one capability while CheckPrivileges and the unit capability tests check
// another.
// METHOD: a lookup of each declared name in the unix constants; a name with no
// constant here fails rather than passes unchecked.
func TestDaemonCapabilityBitsAreTheKernels(t *testing.T) {
	kernel := map[string]uint{
		"CAP_NET_ADMIN":        unix.CAP_NET_ADMIN,
		"CAP_NET_RAW":          unix.CAP_NET_RAW,
		"CAP_NET_BIND_SERVICE": unix.CAP_NET_BIND_SERVICE,
		"CAP_BPF":              unix.CAP_BPF,
		"CAP_PERFMON":          unix.CAP_PERFMON,
		"CAP_SYS_ADMIN":        unix.CAP_SYS_ADMIN,
		"CAP_SYS_RESOURCE":     unix.CAP_SYS_RESOURCE,
	}
	names := map[string]bool{}
	bits := map[uint]bool{}
	for _, c := range DaemonCapabilities() {
		want, known := kernel[c.Name]
		if !known {
			t.Errorf("%s: no unix constant is listed for this name; add it to the lookup above", c.Name)
			continue
		}
		if c.Bit != want {
			t.Errorf("%s: Bit = %d, the kernel's number is %d", c.Name, c.Bit, want)
		}
		if c.Bit >= 64 {
			t.Errorf("%s: Bit %d does not fit the 64-bit CapEff mask", c.Name, c.Bit)
		}
		if names[c.Name] {
			t.Errorf("%s is declared twice", c.Name)
		}
		names[c.Name] = true
		if bits[c.Bit] {
			t.Errorf("bit %d is declared twice", c.Bit)
		}
		bits[c.Bit] = true
		if c.Use == "" {
			t.Errorf("%s has no Use, so its warning would say nothing", c.Name)
		}
	}
	if len(names) == 0 {
		t.Fatal("DaemonCapabilities answered no capability")
	}
}

// TestDaemonCapabilitiesAnswersACopy checks that a caller changing the answer
// leaves the declaration alone.
//
// VALIDATES: DaemonCapabilities answers a fresh slice each call.
// PREVENTS: a caller that edits the slice changing the set the unit grants.
func TestDaemonCapabilitiesAnswersACopy(t *testing.T) {
	first := DaemonCapabilities()
	first[0].Name = "CAP_CHANGED"
	if second := DaemonCapabilities(); second[0].Name == "CAP_CHANGED" {
		t.Fatal("DaemonCapabilities answered the declaration itself, not a copy")
	}
}
