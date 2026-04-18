// Design: docs/architecture/config/syntax.md — config parsing and loading
// Detail: environment_extract.go — extraction of environment values from config tree
// Detail: listener.go — listener conflict detection at config parse time
// Detail: apply_env.go — YANG environment block -> env var plumbing
//
// Package config provides configuration parsing for ze.
package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// Env var registrations. YANG `environment/<name>` leaves that drive runtime
// behavior live here; domain-specific registrations (BGP reactor tuning,
// L2TP auth, privilege drop) stay in their owning package.
var (
	// Hub infrastructure.
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ready.file", Type: "string", Description: "Write signal file when hub is ready (test infrastructure)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.ephemeral", Type: "string", Description: "Start SSH on port 0, write address to this file (config edit)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.pid.file", Type: "string", Description: "PID file path written at hub startup, removed at clean shutdown"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.pprof", Type: "string", Description: "pprof HTTP server address (e.g. :6060). Empty means disabled."})

	// Mirror of internal/core/privilege/drop.go registration so `environment
	// { daemon { user ...; } }` plumbing works even in builds that have not
	// imported the privilege package (e.g. unit tests for ApplyEnvConfig).
	_ = env.MustRegister(env.EnvEntry{Key: "ze.user", Type: "string", Description: "User to drop privileges to after port binding"})

	// BGP protocol knobs plumbed from YANG environment block.
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.openwait", Type: "int", Default: "120", Description: "Seconds to wait for peer OPEN after TCP connect (1-3600)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.announce.delay", Type: "duration", Default: "0s", Description: "Delay between reactor Ready and first UPDATE (staged announcement gate)"})

	// ExaBGP bridge subprocess uses os.Getenv on this key because the bridge
	// runs before ze's env registry is initialized; ze sets it via env.Set
	// here so the child inherits it via os.Environ().
	_ = env.MustRegister(env.EnvEntry{Key: "exabgp.api.ack", Type: "bool", Default: "true", Description: "ExaBGP bridge: emit done/error ack lines on plugin stdin after each dispatched command"})

	// Web server.
	_ = env.MustRegister(env.EnvEntry{Key: "ze.web.listen", Type: "string", Default: "0.0.0.0:3443", Description: "Web server listen address (ip:port[,ip:port])"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.web.enabled", Type: "bool", Description: "Enable web server"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.web.insecure", Type: "bool", Description: "Disable web authentication"})

	// MCP server.
	_ = env.MustRegister(env.EnvEntry{Key: "ze.mcp.listen", Type: "string", Default: "127.0.0.1:8080", Description: "MCP server listen address (ip:port)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.mcp.enabled", Type: "bool", Description: "Enable MCP server"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.mcp.token", Type: "string", Description: "MCP bearer token (Authorization header)", Secret: true})

	// API (REST + gRPC).
	_ = env.MustRegister(env.EnvEntry{Key: "ze.api-server.rest.enabled", Type: "bool", Description: "Enable REST API server"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.api-server.rest.listen", Type: "string", Default: "0.0.0.0:8081", Description: "REST API listen address (ip:port)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.api-server.grpc.enabled", Type: "bool", Description: "Enable gRPC API server"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.api-server.grpc.listen", Type: "string", Default: "0.0.0.0:50051", Description: "gRPC API listen address (ip:port)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.api-server.token", Type: "string", Description: "API bearer token (shared by REST and gRPC)", Secret: true})

	// Looking glass.
	_ = env.MustRegister(env.EnvEntry{Key: "ze.looking-glass.listen", Type: "string", Default: "0.0.0.0:8443", Description: "Looking glass listen address (ip:port)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.looking-glass.enabled", Type: "bool", Description: "Enable looking glass server"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.looking-glass.tls", Type: "bool", Description: "Enable TLS for looking glass"})

	// Gokrazy management proxy (mounted on ze web server at /gokrazy/).
	_ = env.MustRegister(env.EnvEntry{Key: "ze.gokrazy.enabled", Type: "bool", Description: "Enable gokrazy management proxy on web server at /gokrazy/"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.gokrazy.socket", Type: "string", Default: "/run/gokrazy-http.sock", Description: "Gokrazy management Unix socket path"})

	// DNS resolver.
	_ = env.MustRegister(env.EnvEntry{Key: "ze.dns.server", Type: "string", Default: "8.8.8.8:53", Description: "DNS server address (e.g., 8.8.8.8:53)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.dns.timeout", Type: "int", Default: "5", Description: "DNS query timeout in seconds (1-60)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.dns.cache-size", Type: "int", Default: "10000", Description: "DNS cache max entries (0 = disabled)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.dns.cache-ttl", Type: "int", Default: "86400", Description: "DNS cache max TTL in seconds (0 = response TTL only)"})

	// BGP reactor tuning (YANG augment under `environment/reactor/`).
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.reactor.speed", Type: "string", Default: "1.0", Description: "Reactor loop time multiplier (0.1-10.0)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.reactor.cache-ttl", Type: "int", Default: "60", Description: "Update cache TTL in seconds (0=immediate)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.reactor.cache-max", Type: "int", Default: "1000000", Description: "Update cache max entries (0=unlimited)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.reactor.update-groups", Type: "bool", Default: "true", Description: "Cross-peer UPDATE grouping"})

	// Chaos (hub YANG `environment/chaos`).
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.chaos.seed", Type: "int64", Description: "PRNG seed for chaos fault injection (0 = disabled)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.chaos.rate", Type: "string", Default: "0.1", Description: "Fault probability per operation (0.0-1.0)"})

	// Forward pool tuning (consumed in internal/component/bgp/reactor/).
	_ = env.MustRegister(env.EnvEntry{Key: "ze.fwd.chan.size", Type: "int", Default: "256", Description: "Per-destination forward worker channel capacity"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.fwd.write.deadline", Type: "duration", Default: "30s", Description: "TCP write deadline for forward pool batch writes"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.fwd.pool.size", Type: "int", Default: "0", Description: "Overflow MixedBufMux byte budget override (0 = auto-sized from peer prefix maximums)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.fwd.pool.maxbytes", Type: "int64", Default: "0", Description: "Combined byte budget for 4K+64K buffer pools (0 = unlimited)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.fwd.batch.limit", Type: "int", Default: "1024", Description: "Max items per forward batch, bounds writeMu hold time (0 = unlimited)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.fwd.teardown.grace", Type: "duration", Default: "5s", Description: "Grace period at >95% pool + >2x weight before forced teardown"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.fwd.pool.headroom", Type: "int64", Default: "0", Description: "Extra bytes beyond auto-sized pool baseline (0 = no extra)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.fwd.dest.cap", Type: "int", Default: "4096", Description: "Max destinations per Plugin.ForwardCached call (bounds per-call allocation)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.cache.safety.valve", Type: "duration", Default: "5m", Description: "Safety valve duration for UPDATE cache gap-based eviction"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.buf.read.size", Type: "int", Default: "65536", Description: "Per-session TCP read buffer size (bytes)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.buf.write.size", Type: "int", Default: "16384", Description: "Per-session TCP write buffer size (bytes)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.metrics.interval", Type: "duration", Default: "10s", Description: "Periodic metrics refresh interval"})

	// Route server (consumed in internal/component/bgp/plugins/rs/).
	_ = env.MustRegister(env.EnvEntry{Key: "ze.rs.chan.size", Type: "int", Default: "4096", Description: "Per-source-peer route server worker channel capacity"})

	// L2TP PPP authentication (consumed in internal/component/l2tp/).
	_ = env.MustRegister(env.EnvEntry{Key: "ze.l2tp.auth.timeout", Type: "duration", Default: "30s", Description: "PPP auth-phase timeout; session fails closed if no AuthResponse within this window"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.l2tp.auth.reauth-interval", Type: "duration", Default: "0s", Description: "PPP periodic re-authentication interval (CHAP/MS-CHAPv2 only); zero disables re-auth"})
)

// Plugin encoder type names (used by pc.Encoder and extracted from the
// per-plugin YANG `encoder` leaf).
const (
	EncoderJSON = "json"
	EncoderText = "text"
)

// ListenEndpoint represents a parsed ip:port pair from compound listen format.
type ListenEndpoint struct {
	IP   string
	Port int
}

// String returns the endpoint as "ip:port", using bracket notation for IPv6.
func (e ListenEndpoint) String() string {
	if strings.Contains(e.IP, ":") {
		return "[" + e.IP + "]:" + strconv.Itoa(e.Port)
	}
	return e.IP + ":" + strconv.Itoa(e.Port)
}

// ParseCompoundListen parses compound listen format "ip:port[,ip:port]..."
// into a list of endpoints. Supports IPv6 bracket notation: [::1]:3443.
// Port must be in range 1-65535.
func ParseCompoundListen(s string) ([]ListenEndpoint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty listen address")
	}

	var endpoints []ListenEndpoint
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty endpoint in listen address %q", s)
		}

		ep, err := parseOneEndpoint(part)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, ep)
	}

	return endpoints, nil
}

// parseOneEndpoint parses a single "ip:port" or "[ipv6]:port" endpoint.
func parseOneEndpoint(s string) (ListenEndpoint, error) {
	var ip, portStr string

	if strings.HasPrefix(s, "[") {
		// IPv6 bracket notation: [::1]:3443
		closeBracket := strings.Index(s, "]")
		if closeBracket < 0 {
			return ListenEndpoint{}, fmt.Errorf("invalid IPv6 address %q: missing closing bracket", s)
		}
		ip = s[1:closeBracket]
		if ip == "" {
			return ListenEndpoint{}, fmt.Errorf("invalid IPv6 address %q: empty address in brackets", s)
		}
		rest := s[closeBracket+1:]
		if !strings.HasPrefix(rest, ":") {
			return ListenEndpoint{}, fmt.Errorf("invalid endpoint %q: expected ':port' after ']'", s)
		}
		portStr = rest[1:]
	} else {
		// IPv4 or hostname: last colon separates host from port
		lastColon := strings.LastIndex(s, ":")
		if lastColon < 0 {
			return ListenEndpoint{}, fmt.Errorf("invalid endpoint %q: missing port (expected ip:port)", s)
		}
		ip = s[:lastColon]
		portStr = s[lastColon+1:]
	}

	if ip != "" && net.ParseIP(ip) == nil {
		return ListenEndpoint{}, fmt.Errorf("invalid IP address %q in endpoint %q", ip, s)
	}

	if portStr == "" {
		return ListenEndpoint{}, fmt.Errorf("invalid endpoint %q: empty port", s)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return ListenEndpoint{}, fmt.Errorf("invalid port in %q: %w", s, err)
	}
	if port < 1 || port > 65535 {
		return ListenEndpoint{}, fmt.Errorf("port %d in %q out of range: must be 1-65535", port, s)
	}

	return ListenEndpoint{IP: ip, Port: port}, nil
}

// ParseBoolStrict parses a boolean value strictly (case-insensitive).
// Accepts: true/false/yes/no/on/off/enable/disable/1/0.
// Returns error for unrecognized values rather than silently defaulting.
//
// Public because `ze config validate` (cmd/ze/config/cmd_validate.go) walks
// tree values whose boolean variant is any of the tokens above. Internal
// parsing paths use YANG schema type checking instead.
func ParseBoolStrict(value string) (bool, error) {
	v := strings.ToLower(value)
	if v == "1" || v == "true" || v == "yes" || v == "on" || v == "enable" {
		return true, nil
	}
	if v == "0" || v == configFalse || v == "no" || v == "off" || v == "disable" {
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean %q: must be true/false/yes/no/on/off/enable/disable/1/0", value)
}

// DefaultSocketPath returns the default socket path using XDG conventions.
// Resolution order:
//  1. $XDG_RUNTIME_DIR/ze.socket (per-user runtime dir)
//  2. /var/run/ze.socket (system runtime dir, when running as root)
//  3. /tmp/ze.socket (fallback, always writable)
func DefaultSocketPath() string {
	const socketName = "ze.socket"

	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir + "/" + socketName
	}
	if os.Getuid() == 0 {
		return "/var/run/" + socketName
	}
	return "/tmp/" + socketName
}

// ResolveConfigPath searches for a config file using XDG conventions.
// Search order:
//  1. Path as given (absolute or relative to cwd)
//  2. $XDG_CONFIG_HOME/ze/<name> (defaults to ~/.config/ze/)
//  3. Each dir in $XDG_CONFIG_DIRS/ze/<name> (defaults to /etc/xdg/ze/)
//
// Returns the original path unchanged if no XDG match is found,
// letting the caller produce the appropriate "file not found" error.
func ResolveConfigPath(path string) string {
	// Absolute paths and stdin are used as-is.
	if path == "-" || strings.HasPrefix(path, "/") {
		return path
	}

	// If it exists relative to cwd, use it.
	if _, err := os.Stat(path); err == nil {
		return path
	}

	name := path // treat as a filename to search for

	// $XDG_CONFIG_HOME/ze/<name> (default: ~/.config/ze/)
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		if home := os.Getenv("HOME"); home != "" {
			configHome = home + "/.config"
		}
	}
	if configHome != "" {
		base := filepath.Join(configHome, "ze")
		candidate := filepath.Join(base, name)
		if strings.HasPrefix(candidate, base+string(filepath.Separator)) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	// $XDG_CONFIG_DIRS/ze/<name> (default: /etc/xdg/ze/)
	configDirs := os.Getenv("XDG_CONFIG_DIRS")
	if configDirs == "" {
		configDirs = "/etc/xdg"
	}
	for dir := range strings.SplitSeq(configDirs, ":") {
		if dir == "" {
			continue
		}
		base := filepath.Join(dir, "ze")
		candidate := filepath.Join(base, name)
		if strings.HasPrefix(candidate, base+string(filepath.Separator)) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	// Nothing found — return original so caller gets a clear error.
	return path
}
