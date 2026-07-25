// Design: plan/learned/856-install-10-iso-prerequisites.md — doctor check functions for appliance build prerequisites

package appliance

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

var doctorLookPathFn = exec.LookPath

func applianceDoctorChecks() []diagnostic.DoctorCheck {
	return []diagnostic.DoctorCheck{
		{
			Name:         "appliance-kernel",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        800,
			Component:    "appliance",
			Dependencies: []string{"external-binary"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-kernel"},
			Check:        checkKernelArtifact,
		},
		{
			Name:         "appliance-initrd",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        801,
			Component:    "appliance",
			Dependencies: []string{"external-binary"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-initrd"},
			Check:        checkInitrdArtifact,
		},
		{
			Name:         "appliance-grub",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        802,
			Component:    "appliance",
			Dependencies: []string{"external-binary"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-grub"},
			Check:        checkGrubBinary,
		},
		{
			Name:         "appliance-xorriso",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        803,
			Component:    "appliance",
			Dependencies: []string{"external-binary"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-xorriso"},
			Check:        checkXorrisoBinary,
		},
		{
			Name:         "appliance-e2fsprogs",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        804,
			Component:    "appliance",
			Dependencies: []string{"external-binary"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-e2fsprogs"},
			Check:        checkE2fsprogs,
		},
	}
}

func checkKernelArtifact(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	profiles, err := registeredKernelProfiles(kernelInstallerConfigDir)
	if err == nil {
		for _, arch := range []string{archAMD64, archARM64} {
			for _, profile := range profiles {
				if p := isoKernelCachePath(arch, profile); p != "" {
					return nil
				}
			}
			if installerKernelFallbackPath(arch, profiles) != "" {
				return nil
			}
		}
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-appliance-kernel",
		Severity: diagnostic.SeverityWarning,
		Message:  "installer kernel not found; run: ze appliance kernel",
	}}
}

func checkInitrdArtifact(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	for _, path := range []string{
		initrdCachePath(defaultInitrdVersion, runtime.GOARCH),
		defaultISOInitrd,
	} {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-appliance-initrd",
		Severity: diagnostic.SeverityWarning,
		Message:  "installer initrd not found; run: ze appliance initrd",
	}}
}

func checkGrubBinary(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	for _, name := range []string{"grub-mkstandalone", "grub2-mkstandalone"} {
		if _, err := doctorLookPathFn(name); err == nil {
			return nil
		}
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-appliance-grub",
		Severity: diagnostic.SeverityWarning,
		Message:  "grub-mkstandalone not found; install GRUB EFI tooling for ISO builds",
	}}
}

func checkXorrisoBinary(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if _, err := doctorLookPathFn("xorriso"); err == nil {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-appliance-xorriso",
		Severity: diagnostic.SeverityWarning,
		Message:  "xorriso not found; install xorriso for ISO builds",
	}}
}

func checkE2fsprogs(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if e2fsDir != "" {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-appliance-e2fsprogs",
		Severity: diagnostic.SeverityWarning,
		Message:  "e2fsprogs not found (mkfs.ext4 + debugfs); install e2fsprogs for appliance builds",
	}}
}
