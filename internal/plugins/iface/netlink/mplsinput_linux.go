// Design: docs/architecture/mpls/mpls-kernel.md -- MPLS label input on an interface
// Related: rpf_linux.go -- ReplaceIPv4RPF, the same accept-and-install-nothing contract

package ifacenetlink

// ReplaceMPLSInput accepts the set and installs nothing for it, because this
// backend's traffic is received by the Linux kernel, and the kernel already
// enforces each entry: the same leaf reaches net.mpls.conf.<dev>.input through
// the iface component's sysctl phase, which decides whether the kernel accepts
// a labeled frame on the device.
func (b *netlinkBackend) ReplaceMPLSInput(_ []string) error {
	return nil
}
