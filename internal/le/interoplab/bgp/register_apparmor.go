// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: prepare.go -- the VRRP scenario's ze container, which runs under this profile

package bgp

import (
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// vrrpLabAppArmorProfileName is the profile a VRRP scenario's ze runs under.
const vrrpLabAppArmorProfileName = "ze-lab-vrrp"

// vrrpLabProcSysKeep is what VRRP writes under /proc/sys: the per-device
// trees ipv4Conf and ipv6Conf name (internal/plugins/vrrp/dataplane_linux.go).
const vrrpLabProcSysKeep = "net/ipv[46]/conf/"

// vrrpLabAppArmorProfile is docker-default with its /proc/sys write rules
// narrowed to vrrpLabProcSysKeep, and docker-default's `deny mount,` kept.
func vrrpLabAppArmorProfile() string {
	return kernelcap.DockerDefaultProfile(vrrpLabAppArmorProfileName,
		kernelcap.ProcSysWriteDenies(vrrpLabProcSysKeep), "  deny mount,\n")
}

func init() {
	interoplab.RegisterAppArmorProfile(interoplab.AppArmorProfile{
		Name:  vrrpLabAppArmorProfileName,
		Runs:  "the VRRP scenario's ze",
		Needs: "VRRP's per-device sysctl writes under /proc/sys/net",
		Text:  vrrpLabAppArmorProfile,
		Next:  "./le test integration interop",
	})
}
