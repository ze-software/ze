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
// (ai/patterns/registration.md, "Doctor Check Registry"): it reads the same two
// declarations the binder does, where the doctor copy restated both.

package vpp

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorVPPDPDKCode names a VFIO module that is not loaded or a configured PCI
// device sysfs does not hold. internal/core/diagnostic/codes.go declares it, so
// `ze explain doctor-vpp-dpdk` answers.
const doctorVPPDPDKCode = "doctor-vpp-dpdk"

// dpdkLoadedModules is the module-list reader checkVPPDPDK runs, and
// dpdkStatPath its sysfs probe. Both are variables so a test stands in a host
// without the modules or the device; nothing else assigns them.
var (
	dpdkLoadedModules = kernelcap.LoadedModules
	dpdkStatPath      = os.Stat
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
// backend, each VFIO module the kernel has not loaded and each configured PCI
// address sysfs does not hold. Both are errors: BindAll fails on either.
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
		}
	}
	return diags
}
