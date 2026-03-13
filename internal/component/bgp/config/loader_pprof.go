// Design: (none -- pprof build-tag gate for TinyGo compatibility)
// Overview: loader.go -- pprof server startup from config

//go:build !tinygo

package bgpconfig

import (
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // pprof server only starts when configured
)

// startPprofServer starts a pprof HTTP server on the given address.
// Uses DefaultServeMux which net/http/pprof registers handlers on.
// Rejects non-localhost addresses to prevent exposing heap dumps to the network.
func startPprofServer(addr string) {
	if !isLocalhostPprof(addr) {
		configLogger().Error("pprof must bind to localhost (e.g. 127.0.0.1:6060 or [::1]:6060)", "addr", addr)
		return
	}
	configLogger().Info("pprof server starting (config)", "addr", addr)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil { //nolint:gosec // pprof is bound to localhost only
			configLogger().Error("pprof server failed", "error", err)
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
