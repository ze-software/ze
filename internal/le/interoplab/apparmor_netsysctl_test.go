// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: apparmor_netsysctl.go -- the generic net sysctl profile these tests read
//
// A lab peer that writes its network namespace's sysctls while it runs gets
// /proc/sys made writable and runs under ze-lab-net-sysctl, which denies every
// write outside net/ (owner decision D-7, one generic profile).

package interoplab

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: AC-2 of spec-lab-containers-least-privilege. The profile is
// registered, keeps docker-default's mount denial, and admits /proc/sys writes
// under net/ only.
// PREVENTS: a generic lab profile that silently grants kernel-wide sysctls or
// mounts.
func TestNetSysctlAppArmorProfileGrantsOnlyNetWrites(t *testing.T) {
	profile, registered := LookupAppArmorProfile(NetSysctlAppArmorProfileName)
	if !registered {
		t.Fatalf("profile %q is not registered, so no host can load it", NetSysctlAppArmorProfileName)
	}
	if !slices.Contains(AppArmorProfileNames(), NetSysctlAppArmorProfileName) {
		t.Errorf("AppArmorProfileNames lacks %s", NetSysctlAppArmorProfileName)
	}
	text := profile.Text()
	for _, want := range []string{
		"profile " + NetSysctlAppArmorProfileName + " flags=",
		"  deny mount,\n",
		"  deny @{PROC}/sys/[^n]** w,\n",
		"  deny @{PROC}/sys/n[^e]** w,\n",
		"  deny @{PROC}/sys/ne[^t]** w,\n",
		"  deny @{PROC}/sys/net[^/]** w,\n",
		"  deny @{PROC}/sysrq-trigger rwklx,\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the profile lacks %q:\n%s", want, text)
		}
	}
	for _, refused := range []string{"mount options", "remount", "unconfined)", "deny @{PROC}/sys/[^k]"} {
		if strings.Contains(text, refused) {
			t.Errorf("the profile carries %q:\n%s", refused, text)
		}
	}
}

// VALIDATES: NetSysctlGranted answers the profile's grant for a sysctl key.
// PREVENTS: a lab test proving its writes are granted against a different
// rule than the profile's.
func TestNetSysctlGrantedFollowsTheProfile(t *testing.T) {
	for key, want := range map[string]bool{
		"net.ipv4.ipfrag_high_thresh": true,
		"net.mpls.conf.prot0.input":   true,
		"net.ipv4.ip_forward":         true,
		"kernel.core_pattern":         false,
		"vm.nr_hugepages":             false,
		"netfilter.nf_conntrack_max":  false,
		"network":                     false,
	} {
		if got := NetSysctlGranted(key); got != want {
			t.Errorf("NetSysctlGranted(%q) = %v, want %v", key, got, want)
		}
	}
	arguments := NetSysctlWriteArguments()
	if !slices.Contains(arguments, "systempaths=unconfined") {
		t.Errorf("NetSysctlWriteArguments = %v, want systempaths=unconfined", arguments)
	}
}
