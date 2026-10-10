//go:build linux

// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: dataplane_linux.go -- the sysctl writes this test records
//
// The VRRP interop scenarios run ze in a container confined by the generic lab
// AppArmor profile ze-lab-net-sysctl (owner decision D-7). A sysctl VRRP writes
// that the profile denies fails the group's setup on an AppArmor host, so the
// set of writes is taken from the producer itself, never from a copy.

package vrrp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: D-7. Every sysctl applyDataplaneSysctls writes, for an IPv4 and
// an IPv6 group, is one the shared lab profile grants.
// PREVENTS: a lab profile that denies a VRRP write (the deleted ze-lab-vrrp
// denied net/ipv4/icmp_errors_use_inbound_ifaddr), found only when a VRRP
// scenario fails on an AppArmor host.
// Method: the fake sysctl seams record each write; the recorded paths are
// turned into keys relative to /proc/sys and judged by NetSysctlGranted, the
// predicate TestNetSysctlGrantedFollowsTheProfile pins to the profile text.
// MUTATION: narrowing netSysctlProcSysKeep to "net/ipv4/conf/" turns this red
// on icmp_errors_use_inbound_ifaddr.
func TestVRRPWritesOnlyLabGrantedSysctls(t *testing.T) {
	const parent, vmac = "eth0", "vrrp4-1"
	seed := map[string]string{}
	for _, kv := range append(globalDataplaneSysctls(), parentSysctls(parent)...) {
		seed[kv.path] = "0"
	}
	procSys := filepath.Dir(procNetRoot) + "/"
	for _, family := range []string{familyIPv4, familyIPv6} {
		fake := newFakeSysctl(seed)
		fake.install(t)
		if err := applyDataplaneSysctls(parent, vmac, family); err != nil {
			t.Fatalf("%s: applyDataplaneSysctls: %v", family, err)
		}
		if len(fake.writes) == 0 {
			t.Fatalf("%s: applyDataplaneSysctls recorded no write, so nothing was judged", family)
		}
		for _, write := range fake.writes {
			path, _, _ := strings.Cut(write, "=")
			key, under := strings.CutPrefix(path, procSys)
			if !under {
				t.Errorf("%s: VRRP writes %s, outside %s", family, path, procSys)
				continue
			}
			if !interoplab.NetSysctlGranted(key) {
				t.Errorf("%s: VRRP writes %s, which %s denies", family, key, interoplab.NetSysctlAppArmorProfileName)
			}
		}
	}
}
