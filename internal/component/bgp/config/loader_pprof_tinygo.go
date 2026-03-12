// Design: (none -- pprof build-tag gate for TinyGo compatibility)
// Overview: loader.go -- pprof server startup from config

//go:build tinygo

package bgpconfig

// startPprofServer is a no-op under TinyGo (net/http/pprof unsupported).
func startPprofServer(addr string) {
	configLogger().Warn("pprof not available (tinygo build)", "addr", addr)
}
