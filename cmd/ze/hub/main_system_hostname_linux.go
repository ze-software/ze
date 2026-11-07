// Design: docs/guide/appliance.md -- the kernel host name follows system host on the appliance
// Related: main_system_hostname.go -- realKernelHostnameOps, the caller

//go:build linux

package hub

import "golang.org/x/sys/unix"

// setKernelHostname sets the kernel host name of the UTS namespace ze runs in.
func setKernelHostname(name string) error {
	return unix.Sethostname([]byte(name))
}
