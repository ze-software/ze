// Design: docs/features/interfaces.md -- Bus Topics: the address table watch BGP runs with or without an interface block
// Related: address_watch_linux.go -- watchKernelAddresses, the watcher installed here

//go:build linux

package ifacenetlink

import "github.com/ze-software/ze/internal/core/network"

func init() {
	// RFC 2545 Section 3: BGP hears that the host's addresses moved, and so
	// that the connected subnets it judges the condition against moved,
	// through internal/core/network, never by importing this package. The
	// watch needs no interface config, so it runs where the interface monitor
	// does not.
	network.RegisterAddressWatcher(watchKernelAddresses)
}
