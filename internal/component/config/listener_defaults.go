// Design: docs/architecture/doctor-and-health-checks.md -- AC-1/AC-2 listener defaults

package config

// RegisterBuiltinListenerDefaults registers the default IP and port for all
// ze:listener services whose YANG refine defaults are not propagated by the
// Ze YANG compiler.
//
// Its only non-test caller is registerListenerDefaultsOnce.Do in
// collectSchemaListeners (internal/component/doctor/checks_listener.go), so it
// runs when `ze doctor` collects listeners and NEVER for `ze config validate` or
// any other CollectListeners caller. That is why a fact every caller needs --
// which transport a service binds -- is seeded in a package var in listener.go
// instead of being registered here: registered here it would be absent exactly
// where conflict detection runs, and a listener with the wrong transport there
// misses a real port clash silently.
//
// Membership is decided by the SERVICE's own config extraction, not by the YANG
// refine: a service is registered here only when it listens on the default
// endpoint with an EMPTY server list, and in RegisterListenerEntryDefault when
// an empty list starts nothing.
func RegisterBuiltinListenerDefaults() {
	RegisterListenerDefault("web", "0.0.0.0", "3443")
	RegisterListenerDefault("ssh", "127.0.0.1", "2222")
	RegisterListenerDefault("gnmi", "0.0.0.0", "9339")
	RegisterListenerDefault("looking-glass", "0.0.0.0", "8443")
	RegisterListenerDefault("api-server-rest", "0.0.0.0", "8081")
	RegisterListenerDefault("api-server-grpc", "0.0.0.0", "50051")
	RegisterListenerDefault("prometheus", "127.0.0.1", "9273")

	// l2tp appends one listen address per server entry and applies
	// DefaultListenIP/DefaultListenPort to an entry that omits them, and appends
	// nothing for an empty list (internal/component/l2tp/config.go,
	// ParseParameters). So the entry fallback is real and the empty-list
	// endpoint does not exist.
	RegisterListenerEntryDefault("l2tp", "0.0.0.0", "1701")

	// mcp is registered in NEITHER, and that is the whole point of reading the
	// extraction rather than the YANG refine. The refine declares port 8080, but
	// extractMCPBlock (loader_extract.go) passes an EMPTY default port and
	// ExtractMCPConfig returns ok=false whenever the first server carries no
	// port, so an empty list and an ip-only entry both start no listener at all.
	// A default here would make ze-build doctor report a bind failure for a listener
	// that does not exist.
}
