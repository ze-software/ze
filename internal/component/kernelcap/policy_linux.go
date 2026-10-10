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

// securityModule is the Linux security module a confining label belongs to.
type securityModule uint8

const (
	securityModuleUnspecified securityModule = iota
	securityModuleAppArmor
	securityModuleSELinux
	// securityModuleOther is a label whose module the probe cannot name.
	securityModuleOther
)

// confinement is the security policy confining this process. The zero value
// means no policy is known to confine it.
type confinement struct {
	module securityModule
	label  string
}

// String names the confinement for an operator: the module and its label.
func (c confinement) String() string {
	switch c.module {
	case securityModuleAppArmor:
		return "AppArmor profile " + c.label
	case securityModuleSELinux:
		return "SELinux context " + c.label
	case securityModuleOther:
		return "security label " + c.label
	case securityModuleUnspecified:
		return "no confining policy"
	}
	panic("BUG: a confinement with an unnamed securityModule")
}

// confiningPolicy answers the security policy confining this process, or the
// zero confinement when none does. A var so a unit test names one.
var confiningPolicy = readConfiningPolicy

// readConfiningPolicy reads the AppArmor label (/proc/self/attr/apparmor/current,
// Linux 5.8 and later), then the LSM-neutral one (/proc/self/attr/current, which
// carries an SELinux context or an older kernel's AppArmor label). An unreadable
// file, an empty label and AppArmor's `unconfined` all answer the zero value: no
// policy is known to confine ze, so an EACCES is not blamed on one.
func readConfiningPolicy() confinement {
	if label := readLabel(ProcPath("self", "attr", "apparmor", "current")); label != "" {
		return confinement{module: securityModuleAppArmor, label: label}
	}
	if label := readLabel(ProcPath("self", "attr", "current")); label != "" {
		return classifyLabel(label)
	}
	return confinement{}
}

// classifyLabel attributes an LSM-neutral label to the module whose shape it
// has. AppArmor writes `name (mode)`; SELinux writes a context of at least
// three colon-separated fields, `user:role:type[:level]`.
func classifyLabel(label string) confinement {
	if strings.HasSuffix(label, ")") && strings.Contains(label, " (") {
		return confinement{module: securityModuleAppArmor, label: label}
	}
	if strings.Count(label, ":") >= 2 {
		return confinement{module: securityModuleSELinux, label: label}
	}
	return confinement{module: securityModuleOther, label: label}
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
// unknown otherwise. A denied reason names the policy, and under AppArmor the
// profile that grants the probe its steps, and wraps err.
func stepFailed(err error) Result {
	if !errors.Is(err, unix.EACCES) {
		return Result{State: StateUnknown, Reason: err}
	}
	policy := confiningPolicy()
	if policy.module == securityModuleUnspecified {
		return Result{State: StateUnknown, Reason: err}
	}
	if policy.module != securityModuleAppArmor {
		// Ze ships an AppArmor profile only, so a refusal under another module
		// names the steps to grant in that module's policy, not Ze's profile.
		return Result{State: StateDenied, Reason: fmt.Errorf("%w; %s refused it; the probe needs a mount of /proc/sys in a private mount namespace"+
			" and the write to %s, which this host's policy must grant",
			err, policy, MPLSPlatformLabelsPath())}
	}
	return Result{State: StateDenied, Reason: fmt.Errorf("%w; %s refused it; the probe needs a mount of /proc/sys in a private mount namespace"+
		" and the write to %s, which Ze's AppArmor profile %s grants",
		err, policy, MPLSPlatformLabelsPath(), ProbeAppArmorProfileName)}
}
