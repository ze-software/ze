// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: kernelcap.go -- the enrolment these probes answer for
// Detail: probe_linux.go -- the native netlink and procfs reads
// Detail: probe_other.go -- the answer off Linux, where neither capability exists

package kernelcap

import (
	"errors"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/env"
)

const (
	// procRootEnv names the /proc root the capability probes and the doctor
	// checks read. It carries the doctor spelling because it is the doctor
	// tier's root: internal/component/doctor reads the same key through
	// ProcPath, so a functional test points ONE variable at its fixture tree.
	procRootEnv = "ze.test.doctor.procfs-root"

	// xfrmStateEnv forces the XFRM probe's answer, so a functional test reaches
	// the absent and cannot-determine branches on a host whose kernel is
	// healthy. Without it those branches would only be exercised where XFRM
	// happens to be missing, which is the vacuity trap of
	// ai/rules/interop-and-goal-validation.md.
	xfrmStateEnv = "ze.test.kernelcap.xfrm"

	envTypeString = "string"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         procRootEnv,
	Type:        envTypeString,
	Description: "Override /proc root path for doctor and kernel capability functional tests",
	Private:     true,
})

var _ = env.MustRegister(env.EnvEntry{
	Key:         xfrmStateEnv,
	Type:        envTypeString,
	Description: "Force the XFRM kernel capability probe answer: present, absent or unknown (test infrastructure)",
	Private:     true,
})

// errXFRMForced is what the test override reports. It names itself so an
// operator who finds it in a diagnostic knows the answer was injected.
var errXFRMForced = errors.New("forced answer (ze.test.kernelcap.xfrm)")

// ProcPath joins parts under the /proc root, honoring the test override. It is
// the one place the root is decided, so a fixture tree covers every procfs
// read in the doctor tier.
func ProcPath(parts ...string) string {
	root := env.Get(procRootEnv)
	if root == "" {
		root = "/proc"
	}
	all := make([]string, 0, len(parts)+1)
	all = append(all, root)
	all = append(all, parts...)
	return filepath.Join(all...)
}

// MPLSPlatformLabelsPath names the sysctl the kernel creates when it holds an
// AF_MPLS forwarding table. Exported so the fib kernel backend writes the same
// path the probe reads.
func MPLSPlatformLabelsPath() string {
	return ProcPath("sys", "net", "mpls", "platform_labels")
}

// forcedXFRM returns the injected probe answer and true when the test override
// names one.
func forcedXFRM() (Result, bool) {
	return forcedXFRMFor(env.Get(xfrmStateEnv))
}

// forcedXFRMFor classifies one override spelling. An unknown spelling is treated
// as no override rather than as an answer: a typo in a test variable must not
// decide whether a daemon starts.
func forcedXFRMFor(value string) (Result, bool) {
	switch value {
	case "present":
		return Result{State: StatePresent}, true
	case "absent":
		return Result{State: StateAbsent, Reason: errXFRMForced}, true
	case "unknown":
		return Result{State: StateUnknown, Reason: errXFRMForced}, true
	default:
		return Result{}, false
	}
}
