//go:build ze_stripped

package main

import (
	"fmt"
	"os"
)

func runDeprecatedAppliance(_ []string) int {
	fmt.Fprintln(os.Stderr, "error: appliance management is not included in ze-stripped")
	return 1
}

func runInstall(_ []string) int {
	fmt.Fprintln(os.Stderr, "error: install is not included in ze-stripped")
	return 1
}

func runService(_ []string) int {
	fmt.Fprintln(os.Stderr, "error: service management is not included in ze-stripped")
	return 1
}

func runUninstall(_ []string) int {
	fmt.Fprintln(os.Stderr, "error: uninstall is not included in ze-stripped")
	return 1
}
