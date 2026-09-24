//go:build linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- setup must preserve shared sysctls on failure.
package vrrp

import (
	"errors"
	"path/filepath"
	"testing"
)

// A failure after changing global state must restore it and leave no reference
// behind; the next successful group must restore the original state on teardown.
func TestDataplaneApplyRollsBackSharedFailure(t *testing.T) {
	icmp := filepath.Join(procNetRoot, "ipv4", "icmp_errors_use_inbound_ifaddr")
	seed := map[string]string{
		icmp: "0", allRPFilterPath(): "2",
		ipv4Conf("eth0", "arp_ignore"): "0",
		ipv4Conf("eth0", "arp_filter"): "0",
		ipv4Conf("eth0", "rp_filter"):  "2",
	}
	f := newFakeSysctl(seed)
	f.install(t)
	write := sysctlWrite
	failure := errors.New("write refused")
	sysctlWrite = func(path, value string) error {
		if path == ipv4Conf("eth0", "arp_filter") && value == "1" {
			return failure
		}
		return write(path, value)
	}
	if err := applyDataplaneSysctls("eth0", "zv4-first", familyIPv4); !errors.Is(err, failure) {
		t.Fatalf("apply = %v, want write refusal", err)
	}
	for path, want := range seed {
		if got := f.get(path); got != want {
			t.Fatalf("failed apply left %s=%s, want %s", path, got, want)
		}
	}
	sysctlWrite = write
	if err := applyDataplaneSysctls("eth0", "zv4-second", familyIPv4); err != nil {
		t.Fatal(err)
	}
	revertDataplaneSysctls("eth0", "zv4-second", familyIPv4)
	for path, want := range seed {
		if got := f.get(path); got != want {
			t.Fatalf("successful lifecycle left %s=%s, want %s", path, got, want)
		}
	}
}

// An unreadable original value must not be overwritten: removal could not
// restore that value, and a later group must not inherit the partial setup.
func TestDataplaneApplyRefusesUnreadableOriginal(t *testing.T) {
	f := newFakeSysctl(map[string]string{allRPFilterPath(): "2"})
	f.install(t)
	if err := applyDataplaneSysctls("eth0", "zv4-first", familyIPv4); err == nil {
		t.Fatal("unreadable shared sysctl was accepted")
	}
	if got := f.get(allRPFilterPath()); got != "2" {
		t.Fatalf("failed read changed all.rp_filter to %s", got)
	}
}

func TestDataplaneGlobalRollbackFailureRetainsOriginal(t *testing.T) {
	icmp := filepath.Join(procNetRoot, "ipv4", "icmp_errors_use_inbound_ifaddr")
	seed := map[string]string{
		icmp: "0", allRPFilterPath(): "2",
		ipv4Conf("eth0", "arp_ignore"): "0",
		ipv4Conf("eth0", "arp_filter"): "0",
		ipv4Conf("eth0", "rp_filter"):  "2",
	}
	f := newFakeSysctl(seed)
	f.install(t)
	write := sysctlWrite
	setupErr, restoreErr := errors.New("setup refused"), errors.New("restore refused")
	failSetup, failRestore := true, true
	sysctlWrite = func(path, value string) error {
		if failSetup && path == icmp && value == "1" {
			return setupErr
		}
		if failRestore && path == allRPFilterPath() && value == "2" {
			return restoreErr
		}
		return write(path, value)
	}
	err := applyDataplaneSysctls("eth0", "zv4-failed", familyIPv4)
	if !errors.Is(err, setupErr) || !errors.Is(err, restoreErr) {
		t.Fatalf("apply error = %v, want both setup and rollback failures", err)
	}
	if got := f.get(allRPFilterPath()); got != "0" {
		t.Fatalf("rollback failure did not leave the changed value: %s", got)
	}
	failSetup = false
	if err := applyDataplaneSysctls("eth0", "zv4-blocked", familyIPv4); !errors.Is(err, restoreErr) {
		t.Fatalf("apply overwrote an outstanding original: %v", err)
	}
	failRestore = false
	for _, device := range []string{"zv4-one", "zv4-two"} {
		if err := applyDataplaneSysctls("eth0", device, familyIPv4); err != nil {
			t.Fatal(err)
		}
	}
	revertDataplaneSysctls("eth0", "zv4-one", familyIPv4)
	if f.get(allRPFilterPath()) != "0" || f.get(icmp) != "1" {
		t.Fatal("first teardown restored shared state still owned by the second group")
	}
	revertDataplaneSysctls("eth0", "zv4-two", familyIPv4)
	for path, want := range seed {
		if got := f.get(path); got != want {
			t.Fatalf("last teardown restored %s=%s, want original %s", path, got, want)
		}
	}
}

func TestDataplaneParentRollbackAndTeardownFailuresRetainOriginals(t *testing.T) {
	icmp := filepath.Join(procNetRoot, "ipv4", "icmp_errors_use_inbound_ifaddr")
	seed := map[string]string{
		icmp: "0", allRPFilterPath(): "2",
		ipv4Conf("eth0", "arp_ignore"): "0",
		ipv4Conf("eth0", "arp_filter"): "0",
		ipv4Conf("eth0", "rp_filter"):  "2",
	}
	f := newFakeSysctl(seed)
	f.install(t)
	write := sysctlWrite
	setupErr, restoreErr := errors.New("parent setup refused"), errors.New("restore refused")
	failSetup, failRestore := true, true
	sysctlWrite = func(path, value string) error {
		if failSetup && path == ipv4Conf("eth0", "arp_filter") && value == "1" {
			return setupErr
		}
		if failRestore && ((path == allRPFilterPath() && value == "2") ||
			(path == ipv4Conf("eth0", "arp_ignore") && value == "0")) {
			return restoreErr
		}
		return write(path, value)
	}
	err := applyDataplaneSysctls("eth0", "zv4-failed", familyIPv4)
	if !errors.Is(err, setupErr) || !errors.Is(err, restoreErr) {
		t.Fatalf("apply error = %v, want setup and rollback failures", err)
	}
	if f.get(allRPFilterPath()) != "0" || f.get(ipv4Conf("eth0", "arp_ignore")) != "1" {
		t.Fatal("failure did not leave both parent and global restoration outstanding")
	}
	failSetup = false
	if err := applyDataplaneSysctls("eth0", "zv4-blocked", familyIPv4); !errors.Is(err, restoreErr) {
		t.Fatalf("apply accepted an outstanding restoration: %v", err)
	}
	failRestore = false
	if err := applyDataplaneSysctls("eth0", "zv4-good", familyIPv4); err != nil {
		t.Fatal(err)
	}
	failRestore = true
	revertDataplaneSysctls("eth0", "zv4-good", familyIPv4)
	if f.get(allRPFilterPath()) != "0" || f.get(ipv4Conf("eth0", "arp_ignore")) != "1" {
		t.Fatal("last-teardown failure did not leave restoration outstanding")
	}
	failRestore = false
	revertDataplaneSysctls("eth0", "zv4-good", familyIPv4)
	for path, want := range seed {
		if got := f.get(path); got != want {
			t.Fatalf("teardown retry restored %s=%s, want original %s", path, got, want)
		}
	}
}

func TestDataplaneFailedSecondParentPreservesActiveGlobals(t *testing.T) {
	icmp := filepath.Join(procNetRoot, "ipv4", "icmp_errors_use_inbound_ifaddr")
	seed := map[string]string{
		icmp: "0", allRPFilterPath(): "2",
		ipv4Conf("eth0", "arp_ignore"): "0",
		ipv4Conf("eth0", "arp_filter"): "0",
		ipv4Conf("eth0", "rp_filter"):  "2",
		ipv4Conf("eth1", "arp_ignore"): "0",
		ipv4Conf("eth1", "arp_filter"): "0",
		ipv4Conf("eth1", "rp_filter"):  "2",
	}
	f := newFakeSysctl(seed)
	f.install(t)
	if err := applyDataplaneSysctls("eth0", "zv4-first", familyIPv4); err != nil {
		t.Fatal(err)
	}
	write := sysctlWrite
	failure := errors.New("second parent refused")
	sysctlWrite = func(path, value string) error {
		if (path == ipv4Conf("eth1", "arp_filter") && value == "1") ||
			(path == ipv4Conf("eth1", "arp_ignore") && value == "0") {
			return failure
		}
		return write(path, value)
	}
	if err := applyDataplaneSysctls("eth1", "zv4-failed", familyIPv4); !errors.Is(err, failure) {
		t.Fatalf("second-parent setup = %v, want refusal", err)
	}
	if f.get(allRPFilterPath()) != "0" || f.get(icmp) != "1" {
		t.Fatal("failed second parent restored globals still owned by the active first group")
	}
	revertDataplaneSysctls("eth0", "zv4-first", familyIPv4)
	if f.get(allRPFilterPath()) != "2" || f.get(icmp) != "0" {
		t.Fatal("failed second parent leaked a namespace-wide group reference")
	}
	sysctlWrite = write
	if err := applyDataplaneSysctls("eth1", "zv4-second", familyIPv4); err != nil {
		t.Fatal(err)
	}
	revertDataplaneSysctls("eth0", "zv4-first", familyIPv4)
	if f.get(allRPFilterPath()) != "0" || f.get(icmp) != "1" {
		t.Fatal("zero-reference teardown released another parent's live group")
	}
	revertDataplaneSysctls("eth1", "zv4-second", familyIPv4)
	for path, want := range seed {
		if got := f.get(path); got != want {
			t.Fatalf("second-parent retry left %s=%s, want original %s", path, got, want)
		}
	}
}
