// Design: docs/architecture/container/runtime.md -- the doctor codes the container plugin owns
// Related: register.go -- registerContainerDiagnosticCodes, which publishes this table

package container

import "github.com/ze-software/ze/internal/core/diagnostic"

// The doctor codes this plugin owns, one per problem class. Every container
// code starts with doctor-container-. The codes that follow a container's
// state are the operator surface package's, and each kernel capability brings
// a pair of its own with its enrolment.
const (
	codeNotPrivileged     = "doctor-container-not-privileged"
	codeCgroupUnavailable = "doctor-container-cgroup-unavailable"
	codeSealCopiesBinary  = "doctor-container-seal-copies-binary"
	codeOutOfMemory       = "doctor-container-out-of-memory"
	codeProcessLimit      = "doctor-container-process-limit"
	codeStopKilled        = "doctor-container-stop-killed"
	codeCleanupIncomplete = "doctor-container-cleanup-incomplete"
)

// exampleShowDoctor is the daemon's doctor answer, where a finding under one
// of these codes is read.
const exampleShowDoctor = "ze show doctor"

// containerDiagnosticCodes is the explanation of each code. It is registered
// from register.go, so a build without the plugin holds none of them.
var containerDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeNotPrivileged,
		Title:       "Containers need a daemon that runs as root",
		Description: "The configuration holds a container, and the process that judged it does not run as root. Ze creates a container's namespaces, control group and mounts as root, so it refuses the configuration. Containers run on the appliance and under a root daemon. Run the daemon as root, or remove the container block.",
		Examples:    []string{exampleShowDoctor, "ze show diagnostic " + codeNotPrivileged},
		HostVerdict: true,
	},
	{
		Code:        codeCgroupUnavailable,
		Title:       "The control group hierarchy cannot hold containers",
		Description: "The cgroup v2 hierarchy is not mounted at /sys/fs/cgroup, or a controller a container's limits need cannot be enabled there. The runtime library accepts that mount point only, so no container can be limited and none is started. Mount the unified hierarchy at /sys/fs/cgroup and delegate its memory, pids and cpu controllers to the daemon.",
		Examples:    []string{exampleShowDoctor, "ze show diagnostic " + codeCgroupUnavailable},
		HostVerdict: true,
	},
	{
		Code:        codeSealCopiesBinary,
		Title:       "Each container start copies the ze binary into memory",
		Description: "A container's first process runs from a sealed copy of the ze binary, so the container cannot overwrite the binary on disk. The host refuses the sealed overlay mount, so the runtime library copies the whole binary into memory at each start instead. Containers run, and each start costs the size of the binary in memory. Allow the overlay mount on this host.",
		Examples:    []string{exampleShowDoctor, "ze show diagnostic " + codeSealCopiesBinary},
		HostVerdict: true,
	},
	{
		Code:        codeOutOfMemory,
		Title:       "A container was killed for memory",
		Description: "A container allocated past its memory limit and the kernel killed inside it. The daemon and every other container kept running, and the container's restart policy applied. The finding names the container and its limit. Raise the limit with the container's memory statement, or reduce what the workload uses.",
		Examples:    []string{exampleShowDoctor, "ze show diagnostic " + codeOutOfMemory},
	},
	{
		Code:        codeProcessLimit,
		Title:       "A container reached its process limit",
		Description: "A container tried to create more processes and threads than its process limit allows, and the kernel refused the excess. The container keeps running with the processes it has. The finding names the container and its limit. Raise the limit with the container's process-limit statement.",
		Examples:    []string{exampleShowDoctor, "ze show diagnostic " + codeProcessLimit},
	},
	{
		Code:        codeStopKilled,
		Title:       "A container ignored its stop signal and was killed",
		Description: "Ze sent a container its stop signal, the container was still running when its stop timeout ended, and Ze killed every process of it through its control group. Work the container had not saved is lost. The finding names the container. Raise its stop-timeout statement, or set the stop-signal the workload obeys.",
		Examples:    []string{exampleShowDoctor, "ze show diagnostic " + codeStopKilled},
	},
	{
		Code:        codeCleanupIncomplete,
		Title:       "A container left processes, a control group or a mount behind",
		Description: "A previous run of the daemon left a container's processes, control group or root mount in place, or the cleanup of a container could not finish. Ze removes what it finds before it starts a container again, and reports what the kernel refused to remove. The finding names the container. Restart the container, or reboot when the kernel refuses the removal.",
		Examples:    []string{exampleShowDoctor, "ze show diagnostic " + codeCleanupIncomplete},
	},
}
