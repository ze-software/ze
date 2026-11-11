// Design: docs/architecture/appliance/iso-installer.md -- native Debian package repair.

package appliance

import (
	"errors"
	"runtime"
	"testing"
)

func TestISOGRUBHostPackageRepairRespectsPackageArchitecture(t *testing.T) {
	for _, test := range []struct {
		name    string
		distro  string
		target  string
		host    string
		want    string
		foreign bool
	}{
		{"ubuntu-native-amd64", "ubuntu", isoGRUBTargetAMD64, archAMD64, "sudo apt-get install grub-efi-amd64-bin", false},
		{"debian-native-arm64", "debian", isoGRUBTargetARM64, archARM64, "sudo apt-get install grub-efi-arm64-bin", false},
		{"ubuntu-foreign-arm64", "ubuntu", isoGRUBTargetARM64, archAMD64, "", true},
		{"debian-foreign-amd64", "debian", isoGRUBTargetAMD64, archARM64, "", true},
		{"fedora-noarch-arm64", "fedora", isoGRUBTargetARM64, archAMD64, "sudo dnf install grub2-tools-extra grub2-efi-aa64-modules", false},
		{"rocky-noarch-amd64", "rocky", isoGRUBTargetAMD64, archARM64, "sudo dnf install grub2-tools-extra grub2-efi-x64-modules", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			command, err := isoGRUBHostInstallCommand(test.distro, test.target, test.host)
			if test.foreign {
				if err == nil {
					t.Fatalf("foreign native package was suggested: %q", command)
				}
				if command != "" {
					t.Errorf("foreign package refusal returned a package command: %q", command)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if command != test.want {
				t.Errorf("host-aware GRUB repair=%q, want %q", command, test.want)
			}
		})
	}
}

func TestISOGRUBModuleErrorUsesCompleteBuilderForForeignDebianTarget(t *testing.T) {
	for _, distro := range []string{"debian", "ubuntu"} {
		t.Run(distro, func(t *testing.T) {
			oldRead := isoOSReleaseReadFn
			isoOSReleaseReadFn = func(path string) ([]byte, error) {
				if path != "/etc/os-release" {
					t.Errorf("distribution probe read %q", path)
				}
				return []byte("ID=" + distro + "\n"), nil
			}
			t.Cleanup(func() { isoOSReleaseReadFn = oldRead })
			target := isoGRUBTargetARM64
			if runtime.GOARCH == archARM64 {
				target = isoGRUBTargetAMD64
			}
			cause := errors.New("fixture modules are missing")
			failure := isoGRUBModuleError(cause, "/fixture/bin/grub-mkstandalone", target, "")
			if failure.Fix() != isoGRUBOverride {
				t.Errorf("foreign Debian-family target received a native package repair: %q", failure.Fix())
			}
			if !errors.Is(failure, cause) {
				t.Errorf("foreign module failure lost its cause: %v", failure)
			}
		})
	}
}
