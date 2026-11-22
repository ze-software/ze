// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/vpp.

package vpp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorVPPProcess = "doctor-vpp-process"
	codeDoctorVPPStats   = "doctor-vpp-stats"
	exampleDoctorJSON    = "ze doctor --json"

	// doctorVPPDPDKCode names a missing VFIO module or PCI device, or original NIC
	// state that cannot be captured. doctorDiagnosticCodes below registers it, so
	// `ze show diagnostic doctor-vpp-dpdk` answers.
	doctorVPPDPDKCode = "doctor-vpp-dpdk"

	doctorVPPCPUIsolationCode = "doctor-vpp-cpu-isolation"

	doctorVPPHugepagesCode = "doctor-vpp-hugepages"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorVPPProcess,
		Title:       "VPP process ownership is unavailable",
		Description: "VPP API and statistics ownership requires a supported 64-bit Linux runtime kernel with SO_PEERPIDFD and pidfs. Install and boot Ze's runtime kernel when the capability probe fails. A retained process observation failure remains unknown and cannot release subscriber resources. Inspect ze show warnings for the exact process and descriptor failure; resolve the host or descriptor condition before reconnecting.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-vpp-process", "ze show warnings"},
	},
	{
		Code:        codeDoctorVPPStats,
		Title:       "VPP runtime statistics unavailable",
		Description: "The running VPP statistics connection or a statistics read failed. Interface counters used by subscriber accounting and idle detection cannot be treated as zero. Inspect show warnings for the retained socket or read failure, then check the configured VPP statistics socket and its permissions. A successful read clears only its own statistics category from the same live API owner; an unrelated read cannot hide the failure.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-vpp-stats"},
	},
	{
		Code:        doctorVPPDPDKCode,
		Title:       "VPP DPDK readiness failure",
		Description: "VPP DPDK is configured but a required VFIO module or PCI device is missing, or the device's original driver and override cannot be captured for restoration. An ambiguous legacy override requires a kernel providing driver_override_raw before binding.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-vpp-dpdk"},
	},
	{
		Code:        doctorVPPHugepagesCode,
		Title:       "VPP hugepage reservation problem",
		Description: "VPP is enabled but the host's boot-time hugepage reservation is missing, smaller than VPP needs, clamped by the kernel, or (for 1G pages) unsupported by the CPU. Reserve hugepages via image.hugepages or the boot cmdline.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-vpp-hugepages"},
	},
	{
		Code:        doctorVPPCPUIsolationCode,
		Title:       "VPP worker CPUs are not isolated",
		Description: "VPP pins worker threads to CPUs the Linux scheduler still owns, or the host does not report which CPUs are isolated, or every online CPU is isolated and the Linux control plane has none. A VPP worker busy-polls, so a CPU it shares with Linux costs both. Isolate the worker cores with isolcpus on the boot cmdline, or with image.isolated-cpus on an appliance.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-vpp-cpu-isolation"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: vpp: doctor code registration refused: " + err.Error())
		}
	}
}
