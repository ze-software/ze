// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- a loaded ESP GRO module can consume bare ESP before either receive path
// Related: register_esp_form_offload_linux.go -- the runtime declaration
// Related: internal/component/kernelcap/modules_linux.go -- the shared loaded-module reader

//go:build linux

package engine

import (
	"errors"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// espFormOffloadFix removes Ze's states before unloading the module: each ESP state
// can hold its module reference (xfrm_set_type_offload). Ze's state serializer sends no
// XFRMA_OFFLOAD_DEV, so reconstructing those states does not request the module again.
// Another IPsec owner can still hold or load it; the finding names that condition.
const espFormOffloadFix = "sudo systemctl stop ze && sudo modprobe -r esp4_offload && sudo systemctl start ze"

// answerESPFormOffload reads the receiver and the shared module-list snapshot, changing
// neither. It reports only the loaded-module case: an ESP GRO implementation built
// into the kernel is absent from that list, so an empty list proves no loaded module,
// never absence of the kernel feature. Safe for concurrent use; both reads synchronize
// their own state and every finding is local to this call.
func answerESPFormOffload(_ *ikeEngineState) []rpc.DoctorCheckDiagnostic {
	dp := dataplane.Get()
	if dp == nil {
		// refuse-by-default: guards checkIPsecESPFormFastPath -- it reports the failed dataplane load, so no receiver exists to judge here
		return nil
	}
	reporter, ok := dp.(dataplane.ESPFormFastPathReporter)
	if !ok {
		// refuse-by-default: guards dataplane.ESPFormFastPathReporter -- a backend without this interface has no dual-form XFRM receiver whose packets ESP GRO could consume
		return nil
	}
	state, err := reporter.ESPFormFastPathState()
	if errors.Is(err, dataplane.ErrNotSupported) {
		return nil
	}
	if err != nil {
		var cause textbuf.Buffer
		return espFormOffloadUnknown(cause.Str("Ze cannot read its receiver's state (").Err(err).
			Str("), so it cannot tell whether a loaded ESP GRO module affects a dual-form Child SA. "+
				"The command below lists the dataplane's warnings.").String(), dataplane.ESPFormFixLog)
	}
	switch state.Status {
	case dataplane.ESPFormFastPathIdle:
		return nil
	case dataplane.ESPFormFastPathAttached, dataplane.ESPFormFastPathUnavailable, dataplane.ESPFormFastPathDetached:
		return espFormOffloadModuleFindings(kernelcap.LoadedModules())
	case dataplane.ESPFormFastPathUnspecified:
		return espFormOffloadStatusUnknown(state.Status)
	default: // Open backend result: an unknown status cannot establish that no SPI is watched.
		return espFormOffloadStatusUnknown(state.Status)
	}
}

// espFormOffloadStatusUnknown refuses to read an unknown receiver status as idle.
func espFormOffloadStatusUnknown(status dataplane.ESPFormFastPathStatus) []rpc.DoctorCheckDiagnostic {
	var cause textbuf.Buffer
	return espFormOffloadUnknown(cause.Str("Ze's receiver reported the unknown status ").Str(status.String()).
		Str(", so Ze cannot tell whether a loaded ESP GRO module affects a dual-form Child SA. "+
			"The command below lists the dataplane's warnings.").String(), dataplane.ESPFormFixLog)
}

// espFormOffloadModuleFindings distinguishes a loaded esp4_offload module, an empty
// readable snapshot and a snapshot the engine could not read. Only IPv4 is served by
// the dual-form receiver today, so esp6_offload alone is not this problem.
func espFormOffloadModuleFindings(modules map[string]bool) []rpc.DoctorCheckDiagnostic {
	if modules == nil {
		return espFormOffloadUnknown("Ze cannot read /proc/modules in the IKE engine's process, so it cannot tell "+
			"whether the esp4_offload module can consume bare ESP before Ze receives it. Make procfs and its module "+
			"list readable in that process's namespace; the command below mounts procfs if it is missing. If procfs "+
			"is already mounted, correct its visibility or read permissions instead. A kernel built without module "+
			"support has no module list, and an ESP GRO implementation built into the kernel never appears in one; "+
			"those kernel configurations need a separate check of the kernel's build configuration.",
			"sudo mount -t proc proc /proc")
	}
	if !modules["esp4_offload"] {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     diagnostic.CodeDoctorIPsecESPFormOffload,
		Severity: severityWarning,
		Message: "The loaded kernel module esp4_offload can consume bare ESP during GRO before tc ingress and raw sockets. " +
			"For an inbound Child SA on UDP port 4500, the kernel can then refuse the bare form before either Ze's " +
			"rewrite program or its own receiver sees it. The command below stops all Ze tunnels, removing their " +
			"SAs so they release their module references, unloads esp4_offload, then starts Ze again. If another " +
			"IPsec owner still holds an SA using the module, unloading refuses and the command leaves Ze stopped: " +
			"stop that owner and retry before starting Ze. Ze sends no hardware-offload request on its SAs, so " +
			"negotiating them again does not autoload this module. Another IPsec owner can load it again; if that " +
			"owner needs the module, disable GRO on the receiving links with their existing interface offload gro false " +
			"configuration instead. Without systemd, stop Ze with its supervisor, run sudo modprobe -r esp4_offload, " +
			"then start Ze with the same supervisor. This check detects a loaded module; it cannot detect ESP GRO " +
			"built into the kernel.",
		Fix: espFormOffloadFix,
	}}
}

// espFormOffloadUnknown keeps an unavailable observation distinct from module absence.
func espFormOffloadUnknown(message, fix string) []rpc.DoctorCheckDiagnostic {
	return []rpc.DoctorCheckDiagnostic{{
		Code:     diagnostic.CodeDoctorIPsecESPFormOffloadUnknown,
		Severity: severityWarning,
		Message:  message,
		Fix:      fix,
	}}
}
