// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: apparmor.go -- ProbeAppArmorProfile, the profile these tests read
//
// The profile Ze ships for the kernel probe container grants exactly what the
// probe does, so the grant is read against the probe's own paths here.

package kernelcap

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: AC-20 (D-7). The profile names itself, grants the three mounts
// remountSysctlWritable makes and the one write the MPLS probe makes, carries
// neither docker-default's blanket mount deny nor a deny that covers the
// label-space write, and keeps docker-default's read denies.
// PREVENTS: a profile that still blocks the probe (a deny outranks every allow,
// so a kept `deny @{PROC}/sys/[^k]** w,` would block the write), and one that
// grants more than the probe needs.
func TestProbeAppArmorProfileGrantsExactlyTheProbe(t *testing.T) {
	profile := ProbeAppArmorProfile()
	lines := strings.Split(profile, "\n")
	has := func(line string) bool {
		return slices.ContainsFunc(lines, func(l string) bool { return strings.TrimSpace(l) == line })
	}

	for _, want := range []string{
		"profile " + ProbeAppArmorProfileName + " flags=(attach_disconnected,mediate_deleted) {",
		"mount options=(rw, rprivate) -> /,",
		"mount options=(rw, bind) /proc/sys/ -> /proc/sys/,",
		"remount options=(rw, bind) /proc/sys/,",
		"deny @{PROC}/kcore rwklx,",
		"deny @{PROC}/sysrq-trigger rwklx,",
		"deny /sys/kernel/security/** rwklx,",
		"deny @{PROC}/sys/[^n]** w,",
		"deny @{PROC}/sys/net/mpls/platform_label[^s]** w,",
		"deny @{PROC}/sys/net/mpls/platform_labels{?,/}** w,",
	} {
		if !has(want) {
			t.Errorf("the profile lacks %q:\n%s", want, profile)
		}
	}
	for _, banned := range []string{"deny mount,", "deny @{PROC}/sys/[^k]** w,", "mount,"} {
		if has(banned) {
			t.Errorf("the profile carries %q, which blocks the probe or grants every mount", banned)
		}
	}
}

// VALIDATES: the write denies leave exactly the probe's label-space path open:
// every deny is anchored on a strict prefix of it, and the chain covers each
// position of that path, so any other path under /proc/sys diverges from it at
// some position a deny names.
// PREVENTS: a hand-edited chain that skips a position and opens a sysctl.
func TestProbeAppArmorProfileWriteDeniesFollowThePath(t *testing.T) {
	keep := strings.TrimPrefix(MPLSPlatformLabelsPath(), ProcPath()+"/")
	if keep != "sys/net/mpls/platform_labels" {
		t.Fatalf("the label-space path is %q under %q, so the profile's @{PROC} anchor is wrong", keep, ProcPath())
	}
	tail := strings.TrimPrefix(keep, "sys/")
	covered := 0
	for line := range strings.SplitSeq(ProbeAppArmorProfile(), "\n") {
		rule, found := strings.CutPrefix(strings.TrimSpace(line), "deny @{PROC}/sys/")
		if !found {
			continue
		}
		prefix, class, ok := strings.Cut(rule, "[^")
		if !ok {
			continue
		}
		if !strings.HasPrefix(tail, prefix) || len(prefix) >= len(tail) {
			t.Errorf("deny %q is not anchored on a strict prefix of %q", line, tail)
			continue
		}
		if !strings.HasPrefix(class, string(tail[len(prefix)])+"]** w,") {
			t.Errorf("deny %q does not exclude %q at position %d", line, tail[len(prefix)], len(prefix))
		}
		covered++
	}
	if covered != len(tail) {
		t.Errorf("the chain covers %d positions of %q, want %d", covered, tail, len(tail))
	}
}
