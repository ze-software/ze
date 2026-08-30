// Design: docs/architecture/appliance/build-artifacts.md -- doctor check functions for appliance build prerequisites

package appliance

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

const (
	// componentAppliance is the doctor component that owns the appliance build checks.
	componentAppliance = "appliance"
	// dependencyExternalBinary is the doctor dependency of a check that runs a program from PATH.
	dependencyExternalBinary = "external-binary"
	// grubStandalone and grubStandalone2 are the two names distributions give the
	// GRUB standalone EFI image builder. Debian keeps the original name, and the
	// distributions that packaged GRUB 2 beside GRUB Legacy prefix theirs.
	grubStandalone  = "grub-mkstandalone"
	grubStandalone2 = "grub2-mkstandalone"
)

var doctorLookPathFn = exec.LookPath

func applianceDoctorChecks() []diagnostic.DoctorCheck {
	return []diagnostic.DoctorCheck{
		{
			Name:         "appliance-kernel",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        800,
			Component:    componentAppliance,
			Dependencies: []string{dependencyExternalBinary},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-kernel"},
			Check:        checkKernelArtifact,
		},
		{
			Name:         "appliance-initrd",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        801,
			Component:    componentAppliance,
			Dependencies: []string{dependencyExternalBinary},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-initrd"},
			Check:        checkInitrdArtifact,
		},
		{
			Name:         "appliance-grub",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        802,
			Component:    componentAppliance,
			Dependencies: []string{dependencyExternalBinary},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-grub"},
			Check:        checkGrubBinary,
		},
		{
			Name:         "appliance-xorriso",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        803,
			Component:    componentAppliance,
			Dependencies: []string{dependencyExternalBinary},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-xorriso"},
			Check:        checkXorrisoBinary,
		},
		{
			Name:         "appliance-e2fsprogs",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        804,
			Component:    componentAppliance,
			Dependencies: []string{dependencyExternalBinary},
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
	for _, name := range []string{grubStandalone, grubStandalone2} {
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
	if e2fsMkfs != "" && e2fsDebugfs != "" {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-appliance-e2fsprogs",
		Severity: diagnostic.SeverityWarning,
		Message:  "e2fsprogs not found (mkfs.ext4 + debugfs); install e2fsprogs for appliance builds",
	}}
}
