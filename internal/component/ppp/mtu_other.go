// Design: docs/research/l2tpv2-implementation-guide.md -- non-Linux PPPIOCSMRU stub
// Related: ops.go -- pppOps struct referencing realSetMRU, realConnect

//go:build !linux

package ppp

// realSetMRU is a stub on non-Linux platforms. /dev/ppp is Linux-only;
// reaching this in a real build path is a transport-layer bug.
func realSetMRU(unitFD int, mru uint16) error {
	return errNotLinux
}

func realConnect(chanFD, unitNum int) error {
	return errNotLinux
}
