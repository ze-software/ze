// Design: docs/architecture/appliance/build-artifacts.md -- doctor check functions for appliance build prerequisites

package appliance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// doctorRuntimeBuilderFn answers the kernel builder this host would use, with
// the same choice a build makes (kernelbuilder.SelectBuilder, no builder named):
// Docker, else QEMU and Go with a hardware accelerator this host can use. It is
// a package var so the doctor tests run with no Docker and no QEMU.
var doctorRuntimeBuilderFn = func(arch string) (string, error) {
	return kernelbuilder.SelectBuilder(context.Background(), "", arch)
}

func applianceDoctorChecks() []diagnostic.DoctorCheck {
	return []diagnostic.DoctorCheck{
		{
			Name:         "appliance-kernel",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        800,
			Component:    componentAppliance,
			Dependencies: []string{dependencyExternalBinary},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{doctorKernelCode},
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
		{
			Name:         "appliance-go-toolchain",
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        806,
			Component:    componentAppliance,
			Dependencies: []string{dependencyExternalBinary},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{"doctor-appliance-go-toolchain"},
			Check:        checkGoToolchain,
		},
	}
}

// doctorKernelCode is the code of the installer-kernel check and its reports.
const doctorKernelCode = "doctor-appliance-kernel"

func checkKernelArtifact(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	profiles, err := registeredKernelProfiles(kernelInstallerConfigDir)
	if err == nil {
		for _, arch := range []string{archAMD64, archARM64} {
			for _, profile := range profiles {
				p, err := isoKernelCachePath(arch, profile)
				if err != nil {
					return []diagnostic.Diagnostic{{
						Code:     doctorKernelCode,
						Severity: diagnostic.SeverityWarning,
						Message:  "cannot check the installer kernel: " + err.Error(),
					}}
				}
				if p != "" {
					return nil
				}
			}
			if installerKernelFallbackPath(arch, profiles) != "" {
				return nil
			}
		}
	}
	return []diagnostic.Diagnostic{{
		Code:     doctorKernelCode,
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
		if builder, err := doctorLookPathFn(name); err == nil {
			target, _, targetErr := isoGRUBTarget(runtime.GOARCH)
			if targetErr == nil {
				_, targetErr = isoModuleCheckFn(builder, target)
			}
			if targetErr == nil {
				return nil
			}
			failure := isoGRUBModuleError(targetErr, builder, target, "")
			var message textbuf.Buffer
			message.Str(failure.Problem())
			if cause := failure.Unwrap(); cause != nil {
				message.Str(": ").Err(cause)
			}
			return []diagnostic.Diagnostic{{
				Code:     "doctor-appliance-grub",
				Severity: diagnostic.SeverityWarning,
				Message:  message.String(),
				Help:     failure.Why(),
				Fix:      failure.Fix(),
			}}
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
	root, err := os.Getwd()
	if err != nil {
		return runtimeKernelDiagnostic("cannot resolve the runtime kernel source directory: " + err.Error())
	}
	arch := kernelbuilder.HostArch()
	cached, err := kernelCachePathFor(root, defaultKernelVersion, arch, runtimeKernelProfile, "", kernelTargetRuntime)
	if err != nil {
		return runtimeKernelDiagnostic("cannot check the runtime kernel: " + err.Error() + "\n" +
			"ze appliance build reads the kernel config from " + runtimeKernelConfigDir + " in the Ze source tree.\n" +
			"run ze doctor from the top of the Ze source tree")
	}
	if _, err := os.Stat(filepath.Join(cached, runtimeKernelArtifact)); err == nil {
		return nil
	}
	_, builderErr := doctorRuntimeBuilderFn(arch)
	if builderErr == nil {
		return nil
	}
	return runtimeKernelDiagnostic("the runtime kernel " + defaultKernelVersion + " for " + arch + " is not in the cache, " +
		"and this host cannot build it: " + builderErr.Error() + "\n" +
		"ze appliance build boots this kernel, so it cannot build an image on this host.\n" +
		"cache entry: " + cached + "\n" +
		"install Docker, or QEMU and Go with hvf or kvm usable, then build the kernel once, with network access:\n" +
		"ze appliance kernel --target runtime --arch " + arch)
}

// checkGoToolchain reports a host whose go cannot compile the image: the same
// question `ze appliance build` asks before it starts (CheckGoToolchain), so a
// host with Docker and an old go fails here rather than after the kernel.
func checkGoToolchain(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	goMod := "go.mod"
	if _, err := os.Stat(goMod); err != nil {
		return goToolchainDiagnostic("cannot check the go toolchain: no go.mod here (" + err.Error() + "), and ze appliance build compiles the image from the Ze source tree, with the go its go.mod requires.\n" +
			"run ze doctor from the top of the Ze source tree")
	}
	if err := CheckGoToolchain(goMod); err != nil {
		return goToolchainDiagnostic(err.Error())
	}
	return nil
}

func goToolchainDiagnostic(message string) []diagnostic.Diagnostic {
	return []diagnostic.Diagnostic{{
		Code:     "doctor-appliance-go-toolchain",
		Severity: diagnostic.SeverityWarning,
		Message:  message,
	}}
}

func runtimeKernelDiagnostic(message string) []diagnostic.Diagnostic {
	return []diagnostic.Diagnostic{{
		Code:     "doctor-appliance-runtime-kernel",
		Severity: diagnostic.SeverityWarning,
		Message:  message,
	}}
}
