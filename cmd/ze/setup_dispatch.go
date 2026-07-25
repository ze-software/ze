// Design: docs/architecture/cli/plugin-modes.md — ze_setup stdin dispatch for provision fork

//go:build ze_setup && !ze_core

package main

import (
	"github.com/ze-software/ze/internal/component/config/storage"

	"github.com/ze-software/ze/cmd/ze/hub"
)

func init() {
	binaryDispatch = setupDispatch
}

func setupDispatch(args []string) int {
	if len(args) > 0 && args[0] == "-" {
		return hub.Run(storage.NewFilesystem(), "-", nil, 0, -1, false, "", false, "", "")
	}
	return defaultDispatch(args)
}
