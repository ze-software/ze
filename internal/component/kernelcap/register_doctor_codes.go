// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/kernelcap.

package kernelcap

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// CodeDoctorKernelCapabilityForced is the doctor diagnostic code "doctor-kernel-capability-forced".
	CodeDoctorKernelCapabilityForced = "doctor-kernel-capability-forced"
	codeDoctorKernelCapabilityDenied = "doctor-kernel-capability-denied"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        CodeDoctorKernelCapabilityForced,
		Title:       "Kernel capability answer forced by a test",
		Description: "A kernel capability probe did not run: its answer was forced through ze.test.kernelcap.force, which only a zetest build (the functional-test daemon) reads. Every forced answer says so, a forced present included, so a verdict injected by a test is never read as the host's kernel. This code carries a forced PRESENT, at WARNING severity, and does not refuse a start; a forced absent or unknown reports the capability's own code with the variable named in its message. A shipped ze never reads the variable, so seeing this code from a shipped binary is a build defect. Remedy: unset ze.test.kernelcap.force, or run the build you meant to test.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-kernel-capability-forced"},
	},
	{
		Code:        codeDoctorKernelCapabilityDenied,
		Title:       "Kernel capability probe denied by the host's security policy",
		Description: "A kernel capability probe could not ask the kernel because a security policy confining ze (an AppArmor profile such as Docker's docker-default, or an SELinux label) refused a step it needs: the probe's mount of /proc/sys in a private mount namespace, or its write to /proc/sys/net/mpls/platform_labels. The answer is neither present nor absent, and the message names the policy, the denied step and the grant. It is a WARNING and does not refuse a start, as for a probe that could not ask, but `ze doctor --json kernel-capabilities` reports the row denied and exits 1, and the Docker kernel check refuses it. Remedy: grant the probe those steps; Ze's AppArmor profile ze-kernel-probe grants exactly them, and on a Docker host `./le setup docker kernel apparmor` loads it.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-kernel-capability-denied"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: kernelcap: doctor code registration refused: " + err.Error())
		}
	}
}
