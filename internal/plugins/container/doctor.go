// Design: docs/architecture/container/runtime.md -- the doctor codes the container plugin owns and its runtime checks
// Related: register.go -- registerContainerDiagnosticCodes, which publishes the code table
// Related: container.go -- the engine entry, which declares the runtime checks and answers them
// Related: supervise.go -- the problem records the runtime checks read
// Related: kernelcap_linux.go -- the kernel enrolments, each with its pair of codes

package container

import (
	"errors"
	"strconv"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The doctor codes this plugin owns, one per problem class. Every container
// code starts with doctor-container-. codeFailed follows the failed state,
// which the supervisor sets where a start fails; the other codes that follow
// a container's state are the operator surface package's.
const (
	codeNotPrivileged     = "doctor-container-not-privileged"
	codeCgroupUnavailable = "doctor-container-cgroup-unavailable"
	codeSealCopiesBinary  = "doctor-container-seal-copies-binary"
	codeOutOfMemory       = "doctor-container-out-of-memory"
	codeProcessLimit      = "doctor-container-process-limit"
	codeStopKilled        = "doctor-container-stop-killed"
	codeCleanupIncomplete = "doctor-container-cleanup-incomplete"
	codeFailed            = "doctor-container-failed"
)

// The pair of codes of each kernel capability this plugin enrolls
// (kernelcap_linux.go). A capability is named container-NAME, and its codes
// are doctor-container-kernel-NAME-unavailable and
// doctor-container-kernel-NAME-unknown. They are declared here, on every
// platform, so the code table is the same wherever the plugin is built.
const (
	codeKernelCPULimitUnavailable = "doctor-container-kernel-cpu-limit-unavailable"
	codeKernelCPULimitUnknown     = "doctor-container-kernel-cpu-limit-unknown"
)

// problemKind names one problem of a container that its state does not show
// and that a runtime doctor check reports. Zero is reserved and names no
// kind, so a record nobody filled never reads as a problem.
type problemKind uint8

const (
	// problemOutOfMemory: the kernel killed inside the container for memory.
	problemOutOfMemory problemKind = iota + 1
	// problemProcessLimit: the kernel refused a fork past the process limit.
	problemProcessLimit
	// problemStopKilled: the container outlived its stop timeout and was
	// killed through its control group.
	problemStopKilled
	// problemCleanupIncomplete: processes, a control group or a mount of the
	// container were left behind.
	problemCleanupIncomplete
	// problemFailed: the container's last start failed and it holds the
	// failed state. Its record carries the cause and the fix of that start,
	// and it lasts only while the state does.
	problemFailed
)

// The severities of the runtime findings of this plugin. A warning reports
// something that happened to one container that runs, or ran, while the
// daemon and the other containers kept running. An error reports a container
// the configuration enables and that does not run.
const (
	severityWarning = "warning"
	severityError   = "error"
)

// problemDetail is what one record says beyond its kind, for a kind whose
// finding depends on what happened: the cause in plain words, and the command
// that fixes it. A kind whose finding is the same for every container records
// the zero detail, and its finding takes the check's own text and fix: an
// empty cause or fix is that guard, never a missing answer.
type problemDetail struct {
	cause string
	fix   string
}

// runtimeDoctorCheck is one runtime doctor check the plugin answers from its
// own process, where the supervisor's records live, internal and forked alike.
type runtimeDoctorCheck struct {
	// decl is what the Stage 1 registration declares. It names one code.
	decl rpc.DoctorCheckDecl
	// kind is the recorded problem the check reports.
	kind problemKind
	// severity is severityWarning or severityError.
	severity string
	// problem is the finding's text after "container NAME ": what happened
	// and why it matters. A record that carries a cause adds it after this
	// text.
	problem string
	// fix answers the command that fixes the problem for the named container,
	// for a record that carries no fix of its own.
	fix func(name string) string
}

// runtimeDoctorChecks holds every runtime check the plugin declares at Stage
// 1. Each is declared once, here: the registration's DoctorChecks
// (runtimeDoctorCheckDecls) and the doctor callback (answerDoctorCheck) both
// read this list, so a check that is declared is a check that is answered.
var runtimeDoctorChecks = []runtimeDoctorCheck{
	{
		decl:     rpc.DoctorCheckDecl{Name: "container-out-of-memory", Phase: rpc.DoctorPhaseRuntime, Order: 791, Codes: []string{codeOutOfMemory}},
		kind:     problemOutOfMemory,
		severity: severityWarning,
		problem:  "was killed by the kernel for memory: it allocated past its memory limit, and its restart policy applied",
		fix:      func(name string) string { return setInstanceStatement(name, "memory", "MEBIBYTES") },
	},
	{
		decl:     rpc.DoctorCheckDecl{Name: "container-process-limit", Phase: rpc.DoctorPhaseRuntime, Order: 792, Codes: []string{codeProcessLimit}},
		kind:     problemProcessLimit,
		severity: severityWarning,
		problem:  "reached its process limit: the kernel refused the processes above it, and the container keeps running with the ones it has",
		fix:      func(name string) string { return setInstanceStatement(name, "process-limit", "COUNT") },
	},
	{
		decl:     rpc.DoctorCheckDecl{Name: "container-stop-killed", Phase: rpc.DoctorPhaseRuntime, Order: 793, Codes: []string{codeStopKilled}},
		kind:     problemStopKilled,
		severity: severityWarning,
		problem:  "ignored its stop signal and was killed when its stop timeout ended: work it had not saved is lost",
		fix:      func(name string) string { return setInstanceStatement(name, "stop-timeout", "SECONDS") },
	},
	{
		decl:     rpc.DoctorCheckDecl{Name: "container-cleanup-incomplete", Phase: rpc.DoctorPhaseRuntime, Order: 794, Codes: []string{codeCleanupIncomplete}},
		kind:     problemCleanupIncomplete,
		severity: severityWarning,
		problem:  "left processes, a control group or a mount behind: they hold memory and names its next start needs",
		fix:      restartCommand,
	},
	{
		decl:     rpc.DoctorCheckDecl{Name: "container-failed", Phase: rpc.DoctorPhaseRuntime, Order: 795, Codes: []string{codeFailed}},
		kind:     problemFailed,
		severity: severityError,
		problem:  "cannot run: its last start failed, and Ze starts it again at the next configuration commit or restart command",
		fix:      restartCommand,
	},
}

// restartCommand answers the command that restarts the named container.
func restartCommand(name string) string {
	return "ze " + cmdRequestRestart + " " + selectorKeyword + " " + name
}

// setInstanceStatement answers the config statement that sets leaf of the
// named container. value is the upper-case word that stands for the larger
// value the operator chooses.
func setInstanceStatement(name, leaf, value string) string {
	return "set " + configRoot + " " + listInstance + " " + name + " " + leaf + " " + value
}

// runtimeDoctorCheckDecls answers the declarations the plugin sends at Stage
// 1: the checks over the supervisor's records, then the checks over the
// image store (storeDoctorChecks).
func runtimeDoctorCheckDecls() []rpc.DoctorCheckDecl {
	decls := make([]rpc.DoctorCheckDecl, 0, len(runtimeDoctorChecks)+len(storeDoctorChecks))
	for i := range runtimeDoctorChecks {
		decls = append(decls, runtimeDoctorChecks[i].decl)
	}
	for i := range storeDoctorChecks {
		decls = append(decls, storeDoctorChecks[i].decl)
	}
	return decls
}

// answerDoctorCheck answers show doctor's request for the check named name:
// one finding, with its fix, for each container that holds a recorded problem
// of the check's kind. A record that carries a cause adds it to the message,
// and one that carries a fix answers it in place of the check's. A check over
// the image store (storeDoctorChecks) is answered by the store. It only
// reads. A name the plugin did not declare is refused: the daemon asks only
// for declared checks, so only a Ze defect reaches that branch, and an empty
// answer would read as a healthy box.
func (s *supervisor) answerDoctorCheck(name string) ([]rpc.DoctorCheckDiagnostic, error) {
	for i := range runtimeDoctorChecks {
		check := &runtimeDoctorChecks[i]
		if check.decl.Name != name {
			continue
		}
		records := s.problemsOf(check.kind)
		findings := make([]rpc.DoctorCheckDiagnostic, 0, len(records))
		for _, record := range records {
			message := "container " + record.name + " " + check.problem
			if record.detail.cause != "" {
				message += ": " + record.detail.cause
			}
			fix := record.detail.fix
			if fix == "" {
				fix = check.fix(record.name)
			}
			findings = append(findings, rpc.DoctorCheckDiagnostic{
				Code:     check.decl.Codes[0],
				Severity: check.severity,
				Message:  message,
				Fix:      fix,
			})
		}
		return findings, nil
	}
	if check := storeDoctorCheckNamed(name); check != nil {
		return check.answer(s.store)
	}
	return nil, errors.New("container: the plugin declares no doctor check named " + strconv.Quote(name) +
		", so it cannot answer it; this is a Ze defect")
}

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
	{
		Code:        codeFailed,
		Title:       "A container the configuration enables does not run",
		Description: "The last start of a container failed, so the container holds the failed state and runs nothing. A start fails when the image store cannot answer, when a participant such as the network refuses the container, when the runtime library cannot create it, or when its entrypoint cannot be released; a failed start leaves nothing behind. Ze starts the container again at the next configuration commit and at a restart command. The finding names the container, the step that failed and its cause, and the command that fixes that cause. Fix the cause, then run ze request container restart name NAME.",
		Examples:    []string{exampleShowDoctor, "ze show diagnostic " + codeFailed},
	},
	{
		Code:         codeKernelCPULimitUnavailable,
		Title:        "The kernel cannot limit a container's CPU time",
		Description:  "The configuration holds a container to run, and the kernel was built without CPU bandwidth control (CONFIG_CFS_BANDWIDTH). Every container has a hard CPU limit, which Ze writes to the cpu.max file of the container's control group, and that file exists only with this option. This is an error: the daemon refuses to start, a reload is refused and ze config validate fails. Run a kernel built with CONFIG_CFS_BANDWIDTH, such as Ze's own runtime kernel, or disable the container.",
		Examples:     []string{exampleShowDoctor, "ze show diagnostic " + codeKernelCPULimitUnavailable},
		RelatedCodes: []string{codeKernelCPULimitUnknown},
	},
	{
		Code:         codeKernelCPULimitUnknown,
		Title:        "Cannot determine whether the kernel can limit a container's CPU time",
		Description:  "The configuration holds a container to run, and Ze could not read the kernel setting that shows CPU bandwidth control (CONFIG_CFS_BANDWIDTH): /proc/sys/kernel could not be read. This is a warning: the daemon starts, and a container fails to start if the kernel has no hard CPU limit. Make /proc readable to the process that judged the configuration, then read the doctor answer again.",
		Examples:     []string{exampleShowDoctor, "ze show diagnostic " + codeKernelCPULimitUnknown},
		RelatedCodes: []string{codeKernelCPULimitUnavailable},
	},
}
