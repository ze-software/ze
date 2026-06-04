// Design: docs/architecture/cli/plugin-modes.md — full-build feature wiring
//
//go:build !ze_stripped

package main

import (
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/connect"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/install"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/install/local"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/install/remote"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/install/systemd"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/uninstall"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/uninstall/local"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/uninstall/systemd"
)
