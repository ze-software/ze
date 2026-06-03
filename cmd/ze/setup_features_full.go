// Design: docs/architecture/cli/plugin-modes.md — full-build feature wiring
//
//go:build !ze_stripped

package main

import (
	zeinstall "codeberg.org/thomas-mangin/ze/cmd/ze/install"
	zeservice "codeberg.org/thomas-mangin/ze/cmd/ze/service"
	zeuninstall "codeberg.org/thomas-mangin/ze/cmd/ze/uninstall"
)

func runInstall(args []string) int {
	return zeinstall.Run(args)
}

func runService(args []string) int {
	return zeservice.Run(args)
}

func runUninstall(args []string) int {
	return zeuninstall.Run(args)
}
