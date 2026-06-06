// Design: docs/architecture/chaos-web-dashboard.md — ze-chaos minimal entry point

//go:build ze_chaos

package main

import (
	"os"

	zeversion "codeberg.org/thomas-mangin/ze/internal/core/version"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	zeversion.Stamp(version, buildDate)
	os.Exit(zeChaosRun(os.Args[1:]))
}
