//go:build linux

package vrrp

import (
	"fmt"
	"maps"
	"testing"
)

// fakeSysctl backs the sysctlRead/sysctlWrite seams with an in-memory map so the
// refcount and save/restore logic is testable without touching /proc.
type fakeSysctl struct {
	values map[string]string
	writes []string // ordered "path=value" log
}

func newFakeSysctl(seed map[string]string) *fakeSysctl {
	f := &fakeSysctl{values: map[string]string{}}
	maps.Copy(f.values, seed)
	return f
}

// install wires the fake into the package seams and resets the refcount state,
// returning a restore func for t.Cleanup.
func (f *fakeSysctl) install(t *testing.T) {
	t.Helper()
	origR, origW := sysctlRead, sysctlWrite
	sysctlRead = func(path string) (string, error) {
		v, ok := f.values[path]
		if !ok {
			return "", fmt.Errorf("no such knob %q", path)
		}
		return v, nil
	}
	sysctlWrite = func(path, value string) error {
		f.values[path] = value
		f.writes = append(f.writes, path+"="+value)
		return nil
	}
	dataplaneMu.Lock()
	parentRefs = map[string]int{}
	parentSaved = map[string][]sysctlKV{}
	globalRefs = 0
	globalHave = false
	globalSaved = sysctlKV{}
	dataplaneMu.Unlock()
	t.Cleanup(func() { sysctlRead, sysctlWrite = origR, origW })
}

func (f *fakeSysctl) get(path string) string { return f.values[path] }

// TestDataplaneApplyIPv4SetsRecipe proves an IPv4 group sets the full virtual-MAC
// recipe on the parent, the macvlan, and the global knob.
func TestDataplaneApplyIPv4SetsRecipe(t *testing.T) {
	f := newFakeSysctl(map[string]string{
		allRPFilterPath():              "1",
		ipv4Conf("eth0", "arp_ignore"): "0",
		ipv4Conf("eth0", "arp_filter"): "0",
		ipv4Conf("eth0", "rp_filter"):  "1",
	})
	f.install(t)

	if err := applyDataplaneSysctls("eth0", "zv4-2-10", familyIPv4); err != nil {
		t.Fatalf("apply: %v", err)
	}

	want := map[string]string{
		ipv4Conf("zv4-2-10", "arp_ignore"): "1",
		ipv4Conf("zv4-2-10", "rp_filter"):  "0",
		ipv4Conf("eth0", "arp_ignore"):     "1",
		ipv4Conf("eth0", "arp_filter"):     "1",
		ipv4Conf("eth0", "rp_filter"):      "1",
		allRPFilterPath():                  "0",
	}
	for path, val := range want {
		if got := f.get(path); got != val {
			t.Errorf("%s = %q, want %q", path, got, val)
		}
	}
}

// TestDataplaneIPv6IsNoop proves an IPv6 group touches no sysctls. IPv6 needs
// none: ND resolves the VIP to the virtual MAC natively because Neighbor
// Solicitation targets the VIP's solicited-node multicast group, which only the
// macvlan joins (the parent does not hold the VIP), so the parent never competes
// -- unlike IPv4's broadcast ARP. Validated in QEMU (bridge topology, 6/6 to the
// virtual MAC with zero sysctls, no cold-start race; plan/spec-vrrp-6).
func TestDataplaneIPv6IsNoop(t *testing.T) {
	f := newFakeSysctl(map[string]string{allRPFilterPath(): "1"})
	f.install(t)

	if err := applyDataplaneSysctls("eth0", "zv6-2-10", familyIPv6); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(f.writes) != 0 {
		t.Errorf("IPv6 group wrote %d sysctls, want 0: %v", len(f.writes), f.writes)
	}
}

// TestDataplaneRestoreOnLastGroup proves the parent and global knobs are restored
// to their pre-VRRP values only when the last group on the parent is torn down.
func TestDataplaneRestoreOnLastGroup(t *testing.T) {
	f := newFakeSysctl(map[string]string{
		allRPFilterPath():              "1",
		ipv4Conf("eth0", "arp_ignore"): "0",
		ipv4Conf("eth0", "arp_filter"): "0",
		ipv4Conf("eth0", "rp_filter"):  "2",
	})
	f.install(t)

	// Two groups share eth0.
	if err := applyDataplaneSysctls("eth0", "zv4-2-10", familyIPv4); err != nil {
		t.Fatalf("apply g1: %v", err)
	}
	if err := applyDataplaneSysctls("eth0", "zv4-2-20", familyIPv4); err != nil {
		t.Fatalf("apply g2: %v", err)
	}

	// First teardown must NOT restore (the other group still needs the recipe).
	revertDataplaneSysctls("eth0", "zv4-2-10", familyIPv4)
	if got := f.get(ipv4Conf("eth0", "arp_ignore")); got != "1" {
		t.Fatalf("after first revert arp_ignore = %q, want still 1", got)
	}
	if got := f.get(allRPFilterPath()); got != "0" {
		t.Fatalf("after first revert all.rp_filter = %q, want still 0", got)
	}

	// Last teardown restores the saved values.
	revertDataplaneSysctls("eth0", "zv4-2-20", familyIPv4)
	for path, want := range map[string]string{
		ipv4Conf("eth0", "arp_ignore"): "0",
		ipv4Conf("eth0", "arp_filter"): "0",
		ipv4Conf("eth0", "rp_filter"):  "2",
		allRPFilterPath():              "1",
	} {
		if got := f.get(path); got != want {
			t.Errorf("after last revert %s = %q, want restored %q", path, got, want)
		}
	}
}
