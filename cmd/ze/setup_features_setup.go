// Design: docs/architecture/cli/plugin-modes.md — ze_setup feature wiring
//
//go:build ze_setup

package main

import (
	_ "github.com/ze-software/ze/cmd/ze/install"
	_ "github.com/ze-software/ze/internal/appliance"
	_ "github.com/ze-software/ze/internal/install/disk"
	_ "github.com/ze-software/ze/internal/plugins/provision"
)
