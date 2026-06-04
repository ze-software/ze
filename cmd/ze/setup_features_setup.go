// Design: docs/architecture/cli/plugin-modes.md — ze_setup feature wiring
//
//go:build ze_setup

package main

import (
	_ "codeberg.org/thomas-mangin/ze/cmd/ze/install"
	_ "codeberg.org/thomas-mangin/ze/internal/appliance"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/provision"
)
