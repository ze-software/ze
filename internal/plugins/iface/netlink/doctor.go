// Design: ai/rules/doctor-checks.md -- self-contained doctor checks owned by
// the package that owns the runtime dependency.
// Related: doctor_linux.go -- the real macvlan create/delete capability probe
// Related: doctor_other.go -- non-Linux probe stub
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck
//
// The netlink interface backend depends on kernel macvlan support (CONFIG_MACVLAN)
// for the generic owned-device mechanism (plugin-owned macvlan devices). This
// doctor check surfaces a missing capability or a privilege gap via `ze doctor`
// before a plugin's device create fails at apply. The capability is iface-owned
// infrastructure with zero consumer-specific knowledge, so the check and its
// diagnostic code (doctor-iface-macvlan) live with the backend (doctor-checks.md
// ownership rule).

package ifacenetlink

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// macvlanProbeResult classifies the outcome of the kernel macvlan capability
// probe.
type macvlanProbeResult int

const (
	macvlanProbeOK          macvlanProbeResult = iota // a bridge-mode macvlan was created and removed
	macvlanProbeUnsupported                           // the kernel cannot create a macvlan (CONFIG_MACVLAN)
	macvlanProbeNoPrivilege                           // the probe lacked CAP_NET_ADMIN (EPERM)
)

// macvlanProbe is a test seam over the platform capability probe
// (probeMacvlanCapability, defined per build tag in doctor_linux.go /
// doctor_other.go).
var macvlanProbe = probeMacvlanCapability

// macvlanDoctorCheck is the doctor-check registration, installed from
// register.go's init() (registration belongs in register.go, not init() here).
var macvlanDoctorCheck = diagnostic.DoctorCheck{
	Name:         "iface-macvlan",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        742,
	Component:    "iface",
	Dependencies: []string{"kernel-macvlan"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{"doctor-iface-macvlan"},
	Check:        checkIfaceMacvlan,
}

// checkIfaceMacvlan probes kernel macvlan capability and reports a diagnostic
// when it is missing or the probe lacked privilege. It is a no-op when the
// interface component is unconfigured, or when the configured backend is vpp
// (VPP rejects macvlan explicitly, so the netlink capability is irrelevant).
func checkIfaceMacvlan(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ifaceTree := tree.GetContainer("interface")
	if ifaceTree == nil {
		return nil
	}
	if backend, _ := ifaceTree.Get("backend"); backend == "vpp" {
		return nil
	}
	switch macvlanProbe() {
	case macvlanProbeOK:
		return nil
	case macvlanProbeNoPrivilege:
		return []diagnostic.Diagnostic{{
			Code:     "doctor-iface-macvlan",
			Severity: diagnostic.SeverityWarning,
			Message:  "cannot create a probe macvlan device (requires CAP_NET_ADMIN); plugin-owned macvlan devices will fail at apply without it",
		}}
	case macvlanProbeUnsupported:
		return []diagnostic.Diagnostic{{
			Code:     "doctor-iface-macvlan",
			Severity: diagnostic.SeverityError,
			Message:  "kernel cannot create a bridge-mode macvlan device; enable CONFIG_MACVLAN or load the macvlan module",
		}}
	}
	return nil
}
