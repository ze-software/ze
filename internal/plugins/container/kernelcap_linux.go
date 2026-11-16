// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: register_kernelcap_linux.go -- the init() that enrolls what this file builds
// Related: doctor.go -- the pair of codes each enrolment names
//
// A container cannot run without what its limits need of the kernel. Each
// such option is a capability of its own, so ze doctor names the missing one,
// and validate, the start and a reload refuse a config that holds a container
// on a kernel without it. The enrolments are Linux-only because containers
// are.

//go:build linux

package container

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
)

// capabilityCPULimit is the hard CPU limit of a container, its cpu statement.
// Every capability this plugin enrolls is named container-NAME, and NAME is
// the word its two codes carry.
const capabilityCPULimit = "container-cpu-limit"

// cpuBandwidthSysctl is the kernel setting that exists only in a kernel built
// with CONFIG_CFS_BANDWIDTH: kernel.sched_cfs_bandwidth_slice_us, which the
// scheduler registers under that option alone.
const cpuBandwidthSysctl = "sched_cfs_bandwidth_slice_us"

// containerKernelCapabilities answers every kernel capability this plugin
// enrolls. It holds the hard CPU limit today: the options of the other limits
// and of the confinement join it with the code that needs them.
func containerKernelCapabilities() []kernelcap.Capability {
	return []kernelcap.Capability{{
		Subsystem:   capabilityCPULimit,
		Component:   pluginName,
		Kernel:      "CONFIG_CFS_BANDWIDTH",
		ConfigLeaf:  "a container instance, for its cpu limit,",
		CodeAbsent:  codeKernelCPULimitUnavailable,
		CodeUnknown: codeKernelCPULimitUnknown,
		Order:       790,
		InUse:       containerInUse,
		Probe:       cpuLimitProbe,
	}}
}

// containerInUse reports whether the configuration holds a container to run:
// an instance whose enabled leaf is not false. The leaf defaults to true
// (enabledDefault), so an instance without it counts. A disabled container
// asks nothing of the kernel, so a config that holds only disabled ones is
// not judged; the commit that enables one is.
func containerInUse(tree *config.Tree) bool {
	if tree == nil {
		return false
	}
	root := tree.GetContainer(configRoot)
	if root == nil {
		return false
	}
	for _, instance := range root.GetList(listInstance) {
		if instance == nil {
			continue
		}
		// A value the rule set would refuse still counts: only a stated
		// false says the container does not run.
		if enabled, _ := instance.Get(leafEnabled); enabled != "false" {
			return true
		}
	}
	return false
}

// cpuLimitProbe reports whether the kernel holds CPU bandwidth control. It
// reads procfs and changes nothing.
func cpuLimitProbe() kernelcap.Result {
	return sysctlPresence(kernelcap.ProcPath("sys", "kernel"))
}

// sysctlPresence reports whether the CPU bandwidth setting exists in
// directory, which is /proc/sys/kernel or a test's stand-in for it. Only
// existence is read. A directory that cannot be read answers unknown, never
// absent: the setting was not looked for.
func sysctlPresence(directory string) kernelcap.Result {
	if _, err := os.Stat(directory); err != nil {
		return kernelcap.Result{State: kernelcap.StateUnknown, Reason: err}
	}
	_, err := os.Stat(filepath.Join(directory, cpuBandwidthSysctl))
	if err == nil {
		return kernelcap.Result{State: kernelcap.StatePresent}
	}
	if errors.Is(err, fs.ErrNotExist) {
		return kernelcap.Result{State: kernelcap.StateAbsent, Reason: err}
	}
	return kernelcap.Result{State: kernelcap.StateUnknown, Reason: err}
}
