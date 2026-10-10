// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: kernelcap.go -- the enrolment these probes answer for
// Detail: probe_linux.go -- the native netlink and procfs reads
// Detail: probe_other.go -- the answer off Linux, where neither capability exists

package kernelcap

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
)

const (
	// procRootEnv names the /proc root the capability probes and the doctor
	// checks read. It carries the doctor spelling because it is the doctor
	// tier's root: internal/component/doctor reads the same key through
	// ProcPath, so a functional test points ONE variable at its fixture tree.
	procRootEnv = "ze.test.doctor.procfs-root"

	// forceEnv forces the probe answer of named enrolled capabilities, so a
	// functional test reaches the absent and cannot-determine branches of any
	// enrolment on a host whose kernel is healthy. Without it those branches
	// would only be exercised where the feature happens to be missing, which is
	// the vacuity trap of ai/rules/interop-and-goal-validation.md. The value is
	// a comma-separated list of <subsystem>=<present|absent|unknown>.
	forceEnv = "ze.test.kernelcap.force"

	envTypeString = "string"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         procRootEnv,
	Type:        envTypeString,
	Description: "Override /proc root path for doctor and kernel capability functional tests",
	Private:     true,
})

var _ = env.MustRegister(env.EnvEntry{
	Key:         forceEnv,
	Type:        envTypeString,
	Description: "Force enrolled kernel capability probe answers: <subsystem>=<present|absent|unknown>[,...] (test infrastructure)",
	Private:     true,
})

// errForced is what a forced answer reports. It names the variable so an
// operator who finds it in a diagnostic knows the answer was injected.
var errForced = errors.New("forced answer (" + forceEnv + ")")

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

// probe answers for one enrolled capability: the forced answer when the test
// variable names its subsystem, the capability's own probe otherwise. Every
// reader of the enrolment probes through here, so a forced answer reaches
// doctor, the start and reload gates, validate and ProbeAll alike.
func probe(capability *Capability) Result {
	if forced, ok := forcedFor(capability.Subsystem, env.Get(forceEnv)); ok {
		return forced
	}
	return capability.Probe()
}

// forcedFor returns the answer value forces on subsystem, and true when it
// forces one. An entry whose state is not one of the three spellings is treated
// as no override rather than as an answer: a typo in a test variable must not
// decide whether a daemon starts. The variable is read on the control plane
// only (doctor, start, reload, validate), so it is parsed at each read.
func forcedFor(subsystem, value string) (Result, bool) {
	for entry := range strings.SplitSeq(value, ",") {
		name, state, found := strings.Cut(strings.TrimSpace(entry), "=")
		if !found {
			continue
		}
		if name != subsystem {
			continue
		}
		return forcedState(state)
	}
	return Result{}, false
}

// forcedState classifies one state spelling.
func forcedState(state string) (Result, bool) {
	switch state {
	case "present":
		return Result{State: StatePresent}, true
	case "absent":
		return Result{State: StateAbsent, Reason: errForced}, true
	case "unknown":
		return Result{State: StateUnknown, Reason: errForced}, true
	default:
		return Result{}, false
	}
}
