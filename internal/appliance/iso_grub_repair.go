// Design: docs/architecture/appliance/iso-installer.md -- GRUB prerequisite repair.

package appliance

import (
	"errors"
	"os"
	"runtime"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// A complete builder includes the matching modules beside its executable.
// The shell requires an explicit path instead of repeating the failed probe.
const isoGRUBOverride = `./ze-host appliance iso --check --builder "${GRUB_MKSTANDALONE:?set GRUB_MKSTANDALONE to the absolute path of a complete GRUB installation}"`

var isoOSReleaseReadFn = os.ReadFile

// isoGRUBSelectionError adds the invocation's appliance selection to this
// module's complete-builder override. A producer's different repair stays
// unchanged. An unnamed check, including doctor's host check, stays unnamed.
func isoGRUBSelectionError(err error, name, directory string) error {
	if name == "" {
		return err
	}
	var failure *opererr.Error
	if !errors.As(err, &failure) {
		return err
	}
	if failure.Fix() != isoGRUBOverride {
		return err
	}
	var repair textbuf.Buffer
	repair.Str("./ze-host appliance")
	if directory != "" {
		repair.Str(" --dir ").Str(shellArg(directory))
	}
	repair.Byte(' ').Str(strings.TrimPrefix(isoGRUBOverride, "./ze-host appliance ")).
		Byte(' ').Str(shellArg(name))
	return opererr.Wrap(failure.Unwrap(), failure.Problem(), failure.Why(), repair.String())
}

// isoGRUBInstallCommand names packages only for distributions whose package
// names are known. Unknown hosts MUST use a complete builder, not a guessed apt
// command. target is the GRUB target, not the host's CPU architecture.
func isoGRUBInstallCommand(distro, target string) (string, error) {
	aptPackage, rpmPackage := "", ""
	switch target {
	case isoGRUBTargetAMD64:
		aptPackage, rpmPackage = "grub-efi-amd64-bin", "grub2-efi-x64-modules"
	case isoGRUBTargetARM64:
		aptPackage, rpmPackage = "grub-efi-arm64-bin", "grub2-efi-aa64-modules"
	default:
		return "", opererr.New("no GRUB package repair is known for target "+target,
			"Ze ISO images use x86_64-efi or arm64-efi; select a supported appliance architecture",
			"./ze-host appliance iso --help")
	}
	switch distro {
	case "debian", "ubuntu":
		return "sudo apt-get install " + aptPackage, nil
	case "fedora", "rhel", "centos", "rocky", "almalinux":
		return "sudo dnf install grub2-tools-extra " + rpmPackage, nil
	default:
		return "", opererr.New("no GRUB package repair is known for distribution "+distro,
			"use a supported Linux build host or set GRUB_MKSTANDALONE to a complete, readable GRUB installation with the selected EFI modules",
			isoGRUBOverride)
	}
}

// isoGRUBHostInstallCommand refuses a foreign native-architecture Debian
// package. The RPM module packages are noarch, so their target-only mapping
// remains valid when the supplied image differs from the host architecture.
func isoGRUBHostInstallCommand(distro, target, host string) (string, error) {
	switch distro {
	case "debian", "ubuntu":
		nativeTarget, _, err := isoGRUBTarget(host)
		if err != nil {
			return "", opererr.Wrap(err, "cannot select native GRUB modules for build host "+host,
				"the host architecture has no supported native EFI module package; provide a complete matching GRUB installation",
				isoGRUBOverride)
		}
		if target != nativeTarget {
			return "", opererr.New("GRUB target "+target+" differs from build host "+host,
				"Debian and Ubuntu package these EFI modules for their native CPU architectures; this host needs a complete GRUB installation for the selected foreign target",
				isoGRUBOverride)
		}
	}
	return isoGRUBInstallCommand(distro, target)
}

// isoGRUBModuleError keeps the module failure and the repair together, so ISO
// builds and doctor cannot disagree about how to restore the prerequisite.
func isoGRUBModuleError(cause error, builder, target, path string) *opererr.Error {
	var problem textbuf.Buffer
	problem.Str("GRUB builder ").Str(builder).Str(" has no usable ").Str(target).Str("/modinfo.sh module set")
	repair := isoGRUBOverride
	why := "the selected EFI modules are required to make this installer ISO bootable"
	if path != "" {
		problem.Reset()
		problem.Str("cannot inspect GRUB ").Str(target).Str(" modules at ").Str(path)
		why += "; select a complete GRUB installation whose executable and module directories are readable"
	} else if release, err := isoOSReleaseReadFn("/etc/os-release"); err == nil {
		for line := range strings.SplitSeq(string(release), "\n") {
			key, value, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			if key != "ID" {
				continue
			}
			if command, err := isoGRUBHostInstallCommand(strings.Trim(value, "\"'"), target, runtime.GOARCH); err == nil {
				repair = command
			} else {
				var unavailable *opererr.Error
				if errors.As(err, &unavailable) {
					why += "; " + unavailable.Why()
				}
			}
			break
		}
	}
	var existing *opererr.Error
	if errors.As(cause, &existing) {
		return existing
	}
	return opererr.Wrap(cause, problem.String(), why, repair)
}
