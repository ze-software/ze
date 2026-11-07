// Design: docs/guide/appliance.md -- the kernel host name follows system host on the appliance
// Related: main_system_hostname.go -- realKernelHostnameOps, the caller

//go:build !linux

package hub

import "errors"

var errSetHostnameUnsupported = errors.New("setting the kernel host name is supported on Linux only")

// setKernelHostname refuses: the appliance is Linux, and no other platform
// build owns the host name.
func setKernelHostname(string) error {
	return errSetHostnameUnsupported
}
