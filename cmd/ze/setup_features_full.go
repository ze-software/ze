//go:build !ze_stripped

package main

import (
	"fmt"
	"os"

	zeinstall "codeberg.org/thomas-mangin/ze/cmd/ze/install"
	zeservice "codeberg.org/thomas-mangin/ze/cmd/ze/service"
	zeuninstall "codeberg.org/thomas-mangin/ze/cmd/ze/uninstall"
)

func runDeprecatedAppliance(args []string) int {
	fmt.Fprintln(os.Stderr, "warning: \"ze appliance\" is deprecated, use \"ze install appliance\"")
	return zeinstall.Run(append([]string{"appliance"}, args...))
}

func runInstall(args []string) int {
	return zeinstall.Run(args)
}

func runService(args []string) int {
	return zeservice.Run(args)
}

func runUninstall(args []string) int {
	return zeuninstall.Run(args)
}
