// Design: (none -- pprof build-tag gate for TinyGo compatibility)

//go:build !tinygo

package main

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // pprof server only starts when --pprof flag is set
	"os"
)

// startPprof validates the address and starts a pprof HTTP server.
// Uses DefaultServeMux which net/http/pprof registers handlers on.
func startPprof(addr string) {
	if !isLocalhostPprof(addr) {
		fmt.Fprintf(os.Stderr, "error: --pprof must bind to localhost (e.g. 127.0.0.1:6060 or [::1]:6060), got %q\n", addr)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "pprof server listening on %s\n", addr) //nolint:gosec // stderr, not HTTP response
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil { //nolint:gosec // pprof is bound to localhost only
			fmt.Fprintf(os.Stderr, "error: pprof server: %v\n", err)
		}
	}()
}

// isLocalhostPprof returns true if addr binds to a loopback address only.
// Rejects empty host (binds all interfaces), 0.0.0.0, and non-loopback addresses.
func isLocalhostPprof(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}
