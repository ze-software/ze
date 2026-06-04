// Design: docs/architecture/cli/plugin-modes.md — full-build feature wiring
//
//go:build !ze_stripped

package main

import (
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/install"
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/uninstall"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/connect"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/local"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/provision"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/systemd"
)
