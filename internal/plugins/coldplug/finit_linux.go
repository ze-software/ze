// Design: docs/architecture/appliance/kernel-profiles.md -- Loading driver modules on the device: the load
// Overview: coldplug.go -- the pass this load is handed to

//go:build linux

package coldplug

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// finitModule loads one module file with finit_module(2), the call gokrazy's
// own loader makes (gokrazy modules.go, loadModule). It passes no module
// parameters and no flags, so a compressed file is refused by the kernel; the
// pass names that case before it calls here.
//
// When the call returns, the module's init has run, and with it the probe of
// every device its driver binds synchronously, which is where a PCI NIC
// driver registers its interface. A driver that defers its probe
// (EPROBE_DEFER, a firmware load that completes later) registers its
// interface after this returns (kernel-profiles.md, "A NIC that appears after
// the pass", says which part of this is not yet verified).
func finitModule(file string) error {
	module, err := os.Open(file) //nolint:gosec // a module file the running kernel's own index names
	if err != nil {
		return err
	}
	defer module.Close() //nolint:errcheck // read-only module file; the kernel has read it or refused it
	err = unix.FinitModule(int(module.Fd()), "", 0)
	if errors.Is(err, unix.EEXIST) {
		return errAlreadyLoaded
	}
	return err
}
