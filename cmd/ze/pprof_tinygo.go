// Design: (none -- pprof build-tag gate for TinyGo compatibility)

//go:build tinygo

package main

import (
	"fmt"
	"os"
)

// startPprof is a no-op under TinyGo (net/http/pprof unsupported).
func startPprof(addr string) {
	fmt.Fprintf(os.Stderr, "warning: --pprof not supported in TinyGo build\n")
	os.Exit(1)
}
