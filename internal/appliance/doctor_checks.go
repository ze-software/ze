// Design: docs/architecture/appliance/build-artifacts.md -- doctor check functions for appliance build prerequisites

package appliance

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
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

// doctorRuntimeBuilderFn answers the kernel builder this host would use. It is
// a package var so the doctor tests run with no Docker and no QEMU.
var doctorRuntimeBuilderFn = kernelbuilder.UsableBuilder

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
		{
			Name:         "appliance-runtime-kernel",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        805,
			Component:    componentAppliance,
			Dependencies: []string{dependencyExternalBinary},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-runtime-kernel"},
			Check:        checkRuntimeKernel,
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

// checkRuntimeKernel reports a host that cannot produce the runtime kernel
// `ze appliance build` boots. The kernel builds only on a host of its own arch
// (AC-13 of the kernel spec), so the check asks about this host's arch: the
// cache entry the image build would serve, else a usable builder for the cold
// build. It mirrors resolveRuntimeKernel, which serves the entry when its
// vmlinuz is present and otherwise builds.
func checkRuntimeKernel(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	arch := runtime.GOARCH
	cached, err := kernelCachePathFor(defaultKernelVersion, arch, runtimeKernelProfile, kernelTargetRuntime)
	if err != nil {
		return runtimeKernelDiagnostic("cannot check the runtime kernel: " + err.Error() + "\n" +
			"ze appliance build reads the kernel config from " + runtimeKernelConfigDir + " in the Ze source tree.\n" +
			"run ze doctor from the top of the Ze source tree")
	}
	if _, err := os.Stat(filepath.Join(cached, runtimeKernelArtifact)); err == nil {
		return nil
	}
	if _, err := doctorRuntimeBuilderFn(arch); err == nil {
		return nil
	}
	return runtimeKernelDiagnostic("the runtime kernel " + defaultKernelVersion + " for " + arch + " is not in the cache, " +
		"and this host cannot build it: Docker is not installed, and QEMU with Go is not installed.\n" +
		"ze appliance build boots this kernel, so it cannot build an image on this host.\n" +
		"cache entry: " + cached + "\n" +
		"install Docker, or QEMU and Go, then build the kernel once, with network access:\n" +
		"ze appliance kernel --target runtime --arch " + arch)
}

func runtimeKernelDiagnostic(message string) []diagnostic.Diagnostic {
	return []diagnostic.Diagnostic{{
		Code:     "doctor-appliance-runtime-kernel",
		Severity: diagnostic.SeverityWarning,
		Message:  message,
	}}
}
