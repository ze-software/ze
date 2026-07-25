// Design: docs/architecture/cli/plugin-modes.md — ze_distro feature wiring
//
//go:build ze_distro

package main

import (
	_ "github.com/ze-software/ze/cmd/ze/install"
	_ "github.com/ze-software/ze/cmd/ze/uninstall"
	_ "github.com/ze-software/ze/internal/plugins/connect"
	_ "github.com/ze-software/ze/internal/plugins/local"
	_ "github.com/ze-software/ze/internal/plugins/systemd"
)
