// Design: docs/research/vpp-deployment-reference.md -- DPDK NIC driver binding on Linux

//go:build linux

package vpp

import (
	"fmt"
	"os/exec"
)

func init() {
	loadModule = loadModuleLinux
}

// loadModuleLinux loads a kernel module via modprobe.
func loadModuleLinux(name string) error {
	cmd := exec.Command("modprobe", name) //nolint:gosec // name is from hardcoded vfioModules list
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("modprobe %s: %w: %s", name, err, out)
	}
	return nil
}
