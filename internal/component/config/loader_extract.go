// Design: docs/architecture/config/syntax.md -- environment service config extraction
// Related: constants.go -- configTrue used for boolean checks

package config

import (
	"fmt"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// loaderLogger is the config loader subsystem logger (lazy initialization).
var loaderLogger = slogutil.LazyLogger("config.loader")

const loopbackIP = "127.0.0.1"

// ServerEndpoint is one parsed "ip:port" pair from a YANG `list server {}`
// under any environment.<service> block. Shared by web, mcp, and looking-glass
// extraction helpers. Transport-level flags (auth, tls, cors) live on the
// surrounding config struct, not on the endpoint, because they apply to every
// listener of the same service.
type ServerEndpoint struct {
	Host string // Listen host (e.g. 0.0.0.0)
	Port string // Listen port
}

// Listen returns host:port.
func (e ServerEndpoint) Listen() string { return e.Host + ":" + e.Port }

// extractServerList reads the `server` list under a service container and
// returns every entry as a ServerEndpoint. When the list is empty, a single
// entry using the given YANG refine defaults is synthesized so callers always
// see at least one endpoint.
func extractServerList(svc *Tree, defaultHost, defaultPort string) []ServerEndpoint {
	entries := svc.GetListOrdered("server")
	if len(entries) == 0 {
		return []ServerEndpoint{{Host: defaultHost, Port: defaultPort}}
	}
	out := make([]ServerEndpoint, 0, len(entries))
	for _, entry := range entries {
		ep := ServerEndpoint{Host: defaultHost, Port: defaultPort}
		if v, ok := entry.Value.Get("ip"); ok && v != "" {
			ep.Host = v
		}
		if v, ok := entry.Value.Get("port"); ok && v != "" {
			ep.Port = v
		}
		out = append(out, ep)
	}
	return out
}

// WebListenConfig holds parsed environment.web settings.
// Servers is guaranteed non-empty when ExtractWebConfig returns ok=true.
type WebListenConfig struct {
	Servers  []ServerEndpoint // One entry per YANG `server <name> {}` block (defaults synthesized if empty)
	Insecure bool             // Disable authentication (forces every entry to 127.0.0.1)
}

// ExtractWebConfig returns the environment.web config if enabled.
// Every YANG list entry is returned in insertion order; callers that only
// want the first endpoint take `cfg.Servers[0]`.
func ExtractWebConfig(tree *Tree) (WebListenConfig, bool) {
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return WebListenConfig{}, false
	}
	web := envBlock.GetContainer("web")
	if web == nil {
		return WebListenConfig{}, false
	}

	// Service must be explicitly enabled (default false).
	enabled, _ := web.Get("enabled")
	if enabled != configTrue {
		return WebListenConfig{}, false
	}

	cfg := WebListenConfig{Servers: extractServerList(web, "0.0.0.0", "3443")}

	if v, ok := web.Get("insecure"); ok && v == configTrue {
		cfg.Insecure = true
	}

	// Insecure forces every entry to 127.0.0.1 binding.
	if cfg.Insecure {
		for i := range cfg.Servers {
			if cfg.Servers[i].Host != loopbackIP {
				loaderLogger().Error("environment.web: insecure forces host to 127.0.0.1", "host", cfg.Servers[i].Host)
				cfg.Servers[i].Host = loopbackIP
			}
		}
	}

	return cfg, true
}

// HasWebConfig returns true if the parsed config tree has an enabled environment.web block.
func HasWebConfig(tree *Tree) bool {
	_, ok := ExtractWebConfig(tree)
	return ok
}

// MCPListenConfig holds parsed environment.mcp settings.
// Servers is guaranteed non-empty when ExtractMCPConfig returns ok=true.
// Every entry is forced to 127.0.0.1 because MCP only accepts local clients.
type MCPListenConfig struct {
	Servers []ServerEndpoint
	Token   string // Bearer token (empty = no auth)
}

// ExtractMCPConfig returns the environment.mcp config if enabled.
func ExtractMCPConfig(tree *Tree) (MCPListenConfig, bool) {
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return MCPListenConfig{}, false
	}
	mcp := envBlock.GetContainer("mcp")
	if mcp == nil {
		return MCPListenConfig{}, false
	}

	// Service must be explicitly enabled (default false).
	enabled, _ := mcp.Get("enabled")
	if enabled != configTrue {
		return MCPListenConfig{}, false
	}

	cfg := MCPListenConfig{Servers: extractServerList(mcp, loopbackIP, "")}

	if token, ok := mcp.Get("token"); ok {
		cfg.Token = token
	}

	// Enforce loopbackIP binding on every entry.
	for i := range cfg.Servers {
		if cfg.Servers[i].Host != loopbackIP {
			loaderLogger().Error("environment.mcp: host must be 127.0.0.1", "host", cfg.Servers[i].Host)
			cfg.Servers[i].Host = loopbackIP
		}
	}

	// A port leaf is mandatory; if the user left it blank and the YANG default
	// is also empty, the block is effectively unusable.
	if len(cfg.Servers) == 0 || cfg.Servers[0].Port == "" {
		return MCPListenConfig{}, false
	}

	return cfg, true
}

// LGListenConfig holds parsed environment.looking-glass settings.
// Servers is guaranteed non-empty when ExtractLGConfig returns ok=true.
type LGListenConfig struct {
	Servers []ServerEndpoint
	TLS     bool // Enable TLS on every listener
}

// ExtractLGConfig returns the environment.looking-glass config if enabled.
func ExtractLGConfig(tree *Tree) (LGListenConfig, bool) {
	if tree == nil {
		return LGListenConfig{}, false
	}
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return LGListenConfig{}, false
	}
	lg := envBlock.GetContainer("looking-glass")
	if lg == nil {
		return LGListenConfig{}, false
	}

	// Service must be explicitly enabled (default false).
	enabled, _ := lg.Get("enabled")
	if enabled != configTrue {
		return LGListenConfig{}, false
	}

	cfg := LGListenConfig{Servers: extractServerList(lg, "0.0.0.0", "8443")}

	if v, ok := lg.Get("tls"); ok && v == configTrue {
		cfg.TLS = true
	}

	return cfg, true
}

// minTokenLength is the minimum length for hub auth tokens.
const minTokenLength = 32

// ExtractHubConfig extracts plugin hub transport config from a parsed config tree.
// Returns zero-value HubConfig with no servers/clients if no hub block is present.
func ExtractHubConfig(tree *Tree) (plugin.HubConfig, error) {
	pluginContainer := tree.GetContainer("plugin")
	if pluginContainer == nil {
		return plugin.HubConfig{}, nil
	}
	hubContainer := pluginContainer.GetContainer("hub")
	if hubContainer == nil {
		return plugin.HubConfig{}, nil
	}

	var hub plugin.HubConfig

	for _, entry := range hubContainer.GetListOrdered("server") {
		srv, err := extractHubServerConfig(entry.Key, entry.Value)
		if err != nil {
			return plugin.HubConfig{}, fmt.Errorf("hub server %q: %w", entry.Key, err)
		}
		hub.Servers = append(hub.Servers, srv)
	}

	for _, entry := range hubContainer.GetListOrdered("client") {
		cli, err := extractHubClientConfig(entry.Key, entry.Value)
		if err != nil {
			return plugin.HubConfig{}, fmt.Errorf("hub client %q: %w", entry.Key, err)
		}
		hub.Clients = append(hub.Clients, cli)
	}

	return hub, nil
}

func extractHubServerConfig(name string, tree *Tree) (plugin.HubServerConfig, error) {
	srv := plugin.HubServerConfig{Name: name}

	if ip, ok := tree.Get("ip"); ok {
		srv.Host = ip
	}

	if portStr, ok := tree.Get("port"); ok {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return srv, fmt.Errorf("invalid port %q: %w", portStr, err)
		}
		srv.Port = uint16(port)
	}

	if secret, ok := tree.Get("secret"); ok && secret != "" {
		if len(secret) < minTokenLength {
			return srv, fmt.Errorf("secret too short: minimum %d characters, got %d", minTokenLength, len(secret))
		}
		srv.Secret = secret
	}

	clients := tree.GetList("client")
	if len(clients) > 0 {
		srv.Clients = make(map[string]string, len(clients))
		for clientName, clientTree := range clients {
			clientSecret, ok := clientTree.Get("secret")
			if !ok || clientSecret == "" {
				return srv, fmt.Errorf("client %q: secret required", clientName)
			}
			if len(clientSecret) < minTokenLength {
				return srv, fmt.Errorf("client %q: secret too short: minimum %d characters, got %d", clientName, minTokenLength, len(clientSecret))
			}
			srv.Clients[clientName] = clientSecret
		}
	}

	return srv, nil
}

// APIListenConfig holds one parsed api-server listen endpoint.
// Transport-level fields (cors-origin, tls-cert, tls-key) live on APIConfig
// because they apply to every listener of the same transport.
type APIListenConfig struct {
	Host string // Listen host (e.g. 0.0.0.0)
	Port string // Listen port
}

// Listen returns host:port.
func (c APIListenConfig) Listen() string { return c.Host + ":" + c.Port }

// APIConfig holds parsed environment.api settings.
// REST and GRPC each carry a slice of listen endpoints (one entry per
// YANG `list server {}` block). When the transport is enabled but no
// server entries are present, extraction synthesizes a single default
// entry from the YANG refine defaults so downstream binders always see
// at least one endpoint.
type APIConfig struct {
	Token string // Shared bearer token for both transports

	RESTOn         bool
	REST           []APIListenConfig
	RESTCORSOrigin string // REST-only: allowed CORS origin

	GRPCOn      bool
	GRPC        []APIListenConfig
	GRPCTLSCert string // gRPC-only: TLS certificate path
	GRPCTLSKey  string // gRPC-only: TLS key path
}

// ExtractAPIConfig returns the environment.api config if either REST or gRPC is enabled.
// Each transport returns every YANG list entry; if the list is empty the
// YANG refine defaults are used to synthesize one entry so the transport
// always has at least one listener to bind.
func ExtractAPIConfig(tree *Tree) (APIConfig, bool) {
	if tree == nil {
		return APIConfig{}, false
	}
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return APIConfig{}, false
	}
	apiBlock := envBlock.GetContainer("api-server")
	if apiBlock == nil {
		return APIConfig{}, false
	}

	var cfg APIConfig

	if token, ok := apiBlock.Get("token"); ok {
		cfg.Token = token
	}

	// REST transport.
	if rest := apiBlock.GetContainer("rest"); rest != nil {
		enabled, _ := rest.Get("enabled")
		if enabled == configTrue {
			cfg.RESTOn = true
			cfg.REST = extractAPIServerList(rest, "0.0.0.0", "8081")
			if v, ok := rest.Get("cors-origin"); ok {
				cfg.RESTCORSOrigin = v
			}
		}
	}

	// gRPC transport.
	if grpcBlock := apiBlock.GetContainer("grpc"); grpcBlock != nil {
		enabled, _ := grpcBlock.Get("enabled")
		if enabled == configTrue {
			cfg.GRPCOn = true
			cfg.GRPC = extractAPIServerList(grpcBlock, "0.0.0.0", "50051")
			if v, ok := grpcBlock.Get("tls-cert"); ok {
				cfg.GRPCTLSCert = v
			}
			if v, ok := grpcBlock.Get("tls-key"); ok {
				cfg.GRPCTLSKey = v
			}
		}
	}

	if !cfg.RESTOn && !cfg.GRPCOn {
		return APIConfig{}, false
	}

	return cfg, true
}

// extractAPIServerList reads the `server` list under a transport container
// (rest or grpc) and returns every entry as an APIListenConfig. When the
// list is empty, a single entry using the given YANG refine defaults is
// synthesized so callers always see at least one endpoint.
func extractAPIServerList(transport *Tree, defaultHost, defaultPort string) []APIListenConfig {
	entries := transport.GetListOrdered("server")
	if len(entries) == 0 {
		return []APIListenConfig{{Host: defaultHost, Port: defaultPort}}
	}
	out := make([]APIListenConfig, 0, len(entries))
	for _, entry := range entries {
		ep := APIListenConfig{Host: defaultHost, Port: defaultPort}
		if v, ok := entry.Value.Get("ip"); ok && v != "" {
			ep.Host = v
		}
		if v, ok := entry.Value.Get("port"); ok && v != "" {
			ep.Port = v
		}
		out = append(out, ep)
	}
	return out
}

func extractHubClientConfig(name string, tree *Tree) (plugin.HubClientConfig, error) {
	cli := plugin.HubClientConfig{Name: name}

	if host, ok := tree.Get("host"); ok {
		cli.Host = host
	}

	if portStr, ok := tree.Get("port"); ok {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return cli, fmt.Errorf("invalid port %q: %w", portStr, err)
		}
		cli.Port = uint16(port)
	}

	if secret, ok := tree.Get("secret"); ok && secret != "" {
		if len(secret) < minTokenLength {
			return cli, fmt.Errorf("secret too short: minimum %d characters, got %d", minTokenLength, len(secret))
		}
		cli.Secret = secret
	}

	return cli, nil
}
