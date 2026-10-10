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

// VALIDATES: NOTE 9 of spec-lab-containers-least-privilege review round 1. A
// container outside a lab (the rib-fib terminal demo) that writes its
// namespace's net sysctls gets systempaths unconfined, plus the profile on a
// daemon that applies AppArmor, and is refused, naming the load command, on
// a host that has not loaded the profile.
// PREVENTS: /proc/sys made writable with docker-default left to deny the
// write, or with nothing confining it on a host that could.
func TestNetSysctlSecurityArgumentsFollowTheDaemon(t *testing.T) {
	writable := strings.Join(NetSysctlWriteArguments(), " ")
	for _, tc := range []struct {
		name, options, profiles, want string
		refused                       bool
	}{
		{"apparmor, profile loaded", withAppArmor, NetSysctlAppArmorProfileName + " (enforce)\n",
			writable + " --security-opt apparmor=" + NetSysctlAppArmorProfileName, false},
		{"no apparmor", `["name=seccomp,profile=builtin"]`, "", writable, false},
		{"apparmor, profile missing", withAppArmor, "docker-default (enforce)\n", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := labDaemon(t, tc.options, tc.profiles)
			arguments, err := newDocker(runner).NetSysctlSecurityArguments(t.Context())
			if tc.refused {
				if err == nil {
					t.Fatalf("an unloaded profile was accepted: %v", arguments)
				}
				requireCommandOnItsOwnLine(t, err, AppArmorLoadCommandFor(NetSysctlAppArmorProfileName))
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(arguments, " "); got != tc.want {
				t.Errorf("arguments = %q, want %q", got, tc.want)
			}
		})
	}
}

// VALIDATES: round-3 review NOTE 6. NetSysctlUnapplied quotes only the
// stderr line that carries runc's failure, and answers nil for a clean exit
// or a failure that is not the profile.
// PREVENTS: the refusal quoting a whole stderr tail of build output before
// the one line that says why the container did not start.
func TestNetSysctlUnappliedQuotesOnlyTheRuncLine(t *testing.T) {
	cause := "docker: Error response from daemon: failed to create task: runc create failed: " +
		"unable to apply apparmor profile: apparmor failed to apply profile: write " +
		"/proc/self/attr/apparmor/exec: no such file or directory: unknown."
	stderr := "Step 1/9 : FROM debian\n go: downloading golang.org/x/sys\n" + cause + "\nRun 'docker run --help' for more information\n"
	err := NetSysctlUnapplied(125, stderr)
	if err == nil {
		t.Fatal("runc's AppArmor failure answered no refusal")
	}
	message := err.Error()
	if !strings.Contains(message, "("+cause+")") {
		t.Errorf("the refusal does not quote runc's line alone:\n%s", message)
	}
	for _, noise := range []string{"Step 1/9", "downloading", "docker run --help"} {
		if strings.Contains(message, noise) {
			t.Errorf("the refusal quotes %q from the rest of stderr:\n%s", noise, message)
		}
	}
	if err := NetSysctlUnapplied(0, stderr); err != nil {
		t.Errorf("a clean exit answered %v", err)
	}
	if err := NetSysctlUnapplied(1, "exit status 1\n"); err != nil {
		t.Errorf("a failure that is not the profile answered %v", err)
	}
}
