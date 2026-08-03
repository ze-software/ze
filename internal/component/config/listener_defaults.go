// Design: plan/learned/788-doctor-improvements.md -- AC-1/AC-2 listener defaults

package config

// RegisterBuiltinListenerDefaults registers the default IP and port for all
// ze:listener services whose YANG refine defaults are not propagated by the
// Ze YANG compiler. Called explicitly by the binary entry point.
func RegisterBuiltinListenerDefaults() {
	RegisterListenerDefault("web", "0.0.0.0", "3443")
	RegisterListenerDefault("ssh", "127.0.0.1", "2222")
	RegisterListenerDefault("mcp", "0.0.0.0", "8080")
	RegisterListenerDefault("gnmi", "0.0.0.0", "9339")
	RegisterListenerDefault("looking-glass", "0.0.0.0", "8443")
	RegisterListenerDefault("api-server-rest", "0.0.0.0", "8081")
	RegisterListenerDefault("api-server-grpc", "0.0.0.0", "50051")
	RegisterListenerDefault("prometheus", "127.0.0.1", "9273")
}
