//go:build linux

// Design: docs/research/vpp-deployment-reference.md -- ze doctor check for the
// DPDK bind sequence this component runs before it starts VPP.
// Overview: dpdk.go -- DPDKBinder, the apply path that loads the same modules and
// binds the same PCI devices
// Related: register_linux.go -- the init() that installs the registration below
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. BindAll loads vfioModules and binds each configured
// PCI address under sysfsDevDir, so the readiness question is this package's
// (ai/patterns/registration.md, "Doctor Check Registry"): it uses the binder's
// module declarations and original-state capture for those devices.

package vpp

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// These read-only boundaries let tests supply the module list, PCI presence
// and original state of one host. No binding or restoration runs here.
var (
	dpdkLoadedModules = kernelcap.LoadedModules
	dpdkStatPath      = os.Stat
	dpdkOriginalState = func(pci string) (dpdkDriverRestore, error) {
		return readDPDKOriginalState(pci, nativeDPDKBindOps())
	}
)

// vppDPDKDoctorCheck is the registration register_linux.go installs.
//
// Order 2220 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: after the BMP collector check (2210,
// bgp/plugins/bmp/doctor.go) and before the update-check probes that follow it
// on the 2100 scale internal/component/doctor/doctor_checks.go names.
func vppDPDKDoctorCheck() diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{
		Name:         "vpp-dpdk",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        2220,
		Component:    componentVPP,
		Dependencies: []string{dependencyKernel, "sysfs"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{doctorVPPDPDKCode},
		Check:        checkVPPDPDK,
	}
}

// checkVPPDPDK reports, for a config that binds DPDK interfaces under the vpp
// backend, missing VFIO modules, absent configured PCI devices and original
// state that the binder cannot capture. These are all binding prerequisites.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkVPPDPDK(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ifaceBlock := tree.GetContainer("interface")
	if ifaceBlock == nil {
		return nil
	}
	backend, _ := ifaceBlock.Get("backend")
	if backend != componentVPP {
		return nil
	}
	vppBlock := tree.GetContainer(componentVPP)
	if vppBlock == nil {
		return nil
	}
	dpdk := vppBlock.GetContainer("dpdk")
	if dpdk == nil {
		return nil
	}
	interfaces := dpdk.GetListOrdered("interface")
	if len(interfaces) == 0 {
		return nil
	}

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	loaded := dpdkLoadedModules()
	for _, mod := range vfioModules {
		if !loaded[mod] {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     doctorVPPDPDKCode,
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("VPP DPDK: VFIO kernel module not loaded: ").Str(mod).String(),
			})
		}
	}
	for _, iface := range interfaces {
		pci := iface.Key
		sysfsPath := filepath.Join(sysfsDevDir, pci)
		if _, err := dpdkStatPath(sysfsPath); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     doctorVPPDPDKCode,
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("VPP DPDK: PCI device not found: ").Str(pci).String(),
				Path:     sysfsPath,
			})
			continue
		}
		if _, err := dpdkOriginalState(pci); err != nil {
			diags = append(diags, dpdkOriginalStateDiagnostic(pci, err))
		}
	}
	return diags
}

func dpdkOriginalStateDiagnostic(pci string, cause error) diagnostic.Diagnostic {
	path := filepath.Join(sysfsDevDir, pci)
	if native, ok := errors.AsType[*os.PathError](cause); ok {
		path = native.Path
	}
	message := cause.Error()
	fix := "Restore read access to " + path + " before binding this NIC"
	if problem, ok := errors.AsType[*opererr.Error](cause); ok {
		message = problem.Problem() + ": " + problem.Why()
		if evidence := problem.Unwrap(); evidence != nil {
			message += ": " + evidence.Error()
		}
		fix = problem.Fix()
	}
	return diagnostic.Diagnostic{
		Code: doctorVPPDPDKCode, Severity: diagnostic.SeverityError,
		Message: "VPP DPDK: cannot capture original state for PCI device " + pci + ": " + message,
		Path:    path, Actual: cause.Error(), Fix: fix,
	}
}
