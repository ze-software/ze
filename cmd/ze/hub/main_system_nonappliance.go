// Design: docs/guide/configuration.md -- System Host Name: a distro keeps its own host name
// Related: main_system_hostname.go -- reconcileKernelHostname, which reads this

//go:build !ze_appliance

package hub

// kernelHostnameOwned is false off the appliance: the operating system owns
// the host name, and Ze usually runs unprivileged, so it never calls
// sethostname.
const kernelHostnameOwned = false
