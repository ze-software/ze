// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: apparmor.go -- the registry this file writes

package interoplab

import "github.com/ze-software/ze/internal/component/kernelcap"

func init() {
	RegisterAppArmorProfile(AppArmorProfile{
		Name:  kernelcap.ProbeAppArmorProfileName,
		Runs:  "the kernel probe",
		Needs: "the probe's mount and its /proc/sys write",
		Text:  kernelcap.ProbeAppArmorProfile,
		Next:  "./le setup docker-kernel check",
	})
}
