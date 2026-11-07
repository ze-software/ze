// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- non-Linux stub for the IPv6 mode
// Related: devppp_other.go -- SetIPv4Mode stub

//go:build !linux

package ppp

// SetIPv6Mode is unavailable on non-Linux platforms. The caller MUST retain
// descriptor ownership for the call, as required by the Linux implementation.
func SetIPv6Mode(_ int, mode NPMode) error {
	if _, err := mode.kernelValue(); err != nil {
		return err
	}
	return errPppDevPppNotAvailableOn
}
