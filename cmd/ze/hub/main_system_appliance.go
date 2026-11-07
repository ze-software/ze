// Design: docs/guide/appliance.md -- the kernel host name follows system host on the appliance
// Related: main_system_hostname.go -- reconcileKernelHostname, which reads this

//go:build ze_appliance

package hub

// kernelHostnameOwned is true on the appliance: no other program on the
// gokrazy image sets the host name after boot, so Ze sets it from system host.
const kernelHostnameOwned = true
