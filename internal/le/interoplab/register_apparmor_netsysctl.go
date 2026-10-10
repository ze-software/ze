// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: apparmor_netsysctl.go -- the profile this file registers

package interoplab

func init() {
	RegisterAppArmorProfile(AppArmorProfile{
		Name:  NetSysctlAppArmorProfileName,
		Runs:  "a lab peer that writes its network namespace's sysctls while it runs",
		Needs: "sysctl writes under /proc/sys/net",
		Text:  netSysctlAppArmorProfile,
		Next:  "re-run the lab that refused",
	})
}
