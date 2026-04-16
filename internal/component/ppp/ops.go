// Design: docs/research/l2tpv2-implementation-guide.md -- ioctl injection for /dev/ppp
// Related: mtu_linux.go -- real PPPIOCSMRU implementation
// Related: mtu_other.go -- non-Linux stub

package ppp

// pppOps carries injectable function fields for syscalls that ze
// performs on /dev/ppp file descriptors. Mirrors the kernelOps
// pattern in internal/component/l2tp/kernel_linux.go: production
// uses newPPPOps(); tests inject fakes by constructing a pppOps
// literal with custom func fields.
//
// 6a only needs PPPIOCSMRU. 6b will not add ioctls (auth runs
// over the chan fd via ordinary read/write). 6c may add address-
// related ioctls if iface.Backend cannot cover all P2P semantics.
type pppOps struct {
	// setMRU sets the maximum receive unit on a /dev/ppp unit fd
	// via PPPIOCSMRU. Called once per session after LCP-Opened.
	setMRU func(unitFD int, mru uint16) error
}

// newPPPOps returns a pppOps populated with real syscall
// implementations for the current platform.
func newPPPOps() pppOps {
	return pppOps{
		setMRU: realSetMRU,
	}
}
