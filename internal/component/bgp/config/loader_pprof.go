// Design: (none -- pprof build-tag gate for TinyGo compatibility)
// Overview: loader.go -- pprof server startup from config

//go:build !tinygo

package bgpconfig

import (
	"net/http"
	_ "net/http/pprof" //nolint:gosec // pprof server only starts when configured
)

// startPprofServer starts a pprof HTTP server on the given address.
// Uses DefaultServeMux which net/http/pprof registers handlers on.
func startPprofServer(addr string) {
	configLogger().Info("pprof server starting (config)", "addr", addr)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil { //nolint:gosec // pprof is intentionally bound to configured address
			configLogger().Error("pprof server failed", "error", err)
		}
	}()
}
