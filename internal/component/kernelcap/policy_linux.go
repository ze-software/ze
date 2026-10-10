//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: apparmor.go -- ProbeAppArmorProfile, the grant a denied reason names
// Related: probe_linux.go -- the MPLS probe steps classified here
//
// A probe step can fail because the kernel lacks the feature, because ze lacks
// a capability, or because a security policy confining ze refused the step.
// Only the last is fixed in the host's policy, so it is told apart (D-7): the
// kernel answers a policy refusal with EACCES, and /proc/self/attr names the
// confining label. A missing capability answers EPERM and stays unknown.

package kernelcap

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// confiningPolicy names the security policy confining this process, or answers
// empty when none does. A var so a unit test names one.
var confiningPolicy = readConfiningPolicy

// readConfiningPolicy reads the AppArmor label (/proc/self/attr/apparmor/current,
// Linux 5.8 and later), then the LSM-neutral one (/proc/self/attr/current, which
// carries an SELinux context or an older kernel's AppArmor label). An unreadable
// file, an empty label and AppArmor's `unconfined` all answer empty: no policy is
// known to confine ze, so an EACCES is not blamed on one.
func readConfiningPolicy() string {
	if label := readLabel(ProcPath("self", "attr", "apparmor", "current")); label != "" {
		return "AppArmor profile " + label
	}
	if label := readLabel(ProcPath("self", "attr", "current")); label != "" {
		return "security label " + label
	}
	return ""
}

// readLabel answers one attr file's label, empty when it is unreadable, empty,
// or unconfined. The kernel ends the label with a newline, and with a NUL on
// some kernels.
func readLabel(path string) string {
	data, err := os.ReadFile(path) //nolint:gosec // a fixed procfs path under the probe root
	if err != nil {
		return ""
	}
	label := strings.TrimSpace(strings.TrimRight(string(data), "\x00"))
	if label == "unconfined" {
		return ""
	}
	return label
}

// stepFailed answers the result for a probe step that failed with err: denied
// when the kernel refused it with EACCES and a policy confines this process,
// unknown otherwise. A denied reason names the policy and the profile that
// grants the probe its steps, and wraps err.
func stepFailed(err error) Result {
	if !errors.Is(err, unix.EACCES) {
		return Result{State: StateUnknown, Reason: err}
	}
	policy := confiningPolicy()
	if policy == "" {
		return Result{State: StateUnknown, Reason: err}
	}
	return Result{State: StateDenied, Reason: fmt.Errorf("%w; %s refused it; the probe needs a mount of /proc/sys in a private mount namespace"+
		" and the write to %s, which Ze's AppArmor profile %s grants",
		err, policy, MPLSPlatformLabelsPath(), ProbeAppArmorProfileName)}
}
