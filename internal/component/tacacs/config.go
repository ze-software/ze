// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption

// TACACS+ configuration extraction from the YANG config tree.
package tacacs

import (
	"encoding/json"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/redact"
)

const configValueTrue = "true"

// RFC 8907 Section 10.5.1: "TACACS+ servers and clients MUST treat shared
// secrets as sensitive data to be managed securely, as would be expected for
// other sensitive data such as identity credential information."
// Render the endpoint, never the credential, including when the server is
// nested in an extracted configuration or a client configuration.

// String describes a server without exposing its shared secret.
func (s TacacsServer) String() string {
	return s.Address + " key=" + redact.Placeholder
}

// GoString keeps Go-syntax diagnostics from dumping the key's bytes.
func (s TacacsServer) GoString() string {
	return s.String()
}

// MarshalJSON hides the shared secret in structured diagnostic output.
func (s TacacsServer) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Address string
		Key     string
	}{Address: s.Address, Key: redact.Placeholder})
}

// LogValue keeps structured logs from publishing the shared secret.
func (s TacacsServer) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("address", s.Address),
		slog.String("key", redact.Placeholder),
	)
}

// ExtractedConfig holds TACACS+ configuration extracted from the config tree.
type ExtractedConfig struct {
	Servers        []TacacsServer
	Timeout        time.Duration
	SourceAddress  string
	Authorization  bool
	StrictFallback bool
	Accounting     bool
	PrivLvlMap     map[int][]string // priv-lvl -> ze profile names
}

// HasServers returns true if at least one TACACS+ server is configured.
func (c *ExtractedConfig) HasServers() bool {
	return len(c.Servers) > 0
}

// ExtractConfig reads TACACS+ configuration from the parsed config tree.
// Reads from system.authentication.tacacs and system.authentication.tacacs-profile.
// Safe to call with nil tree (returns zero config).
func ExtractConfig(tree *config.Tree) ExtractedConfig {
	var cfg ExtractedConfig

	if tree == nil {
		return cfg
	}
	sys := tree.GetContainer("system")
	if sys == nil {
		return cfg
	}
	auth := sys.GetContainer("authentication")
	if auth == nil {
		return cfg
	}

	// TACACS+ servers (RFC 8907). Use GetListOrdered to preserve
	// configured failover order (YANG: ordered-by user).
	tacContainer := auth.GetContainer("tacacs")
	if tacContainer != nil {
		for _, item := range tacContainer.GetListOrdered("server") {
			addr := item.Key
			entry := item.Value
			port := uint16(49)
			if v, ok := entry.Get("port"); ok {
				if n, err := strconv.ParseUint(v, 10, 16); err == nil {
					port = uint16(n)
				}
			}
			// net.JoinHostPort brackets IPv6 addresses so the resulting
			// string round-trips through net.SplitHostPort and
			// net.Dialer.Dial. Plain Sprintf("%s:%d", ...) would produce
			// "2001:db8::1:49" which neither parser can unambiguously
			// split back into host and port.
			srv := TacacsServer{
				Address: net.JoinHostPort(addr, strconv.Itoa(int(port))),
			}
			if v, ok := entry.Get("key"); ok {
				srv.Key = []byte(v)
			}
			cfg.Servers = append(cfg.Servers, srv)
		}

		cfg.Timeout = 5 * time.Second
		if v, ok := tacContainer.Get("timeout"); ok {
			if n, err := strconv.ParseUint(v, 10, 16); err == nil {
				cfg.Timeout = time.Duration(n) * time.Second
			}
		}
		if v, ok := tacContainer.Get("source-address"); ok {
			cfg.SourceAddress = v
		}
		if v, ok := tacContainer.Get("authorization"); ok && v == configValueTrue {
			cfg.Authorization = true
		}
		if v, ok := tacContainer.Get("strict-fallback"); ok && v == configValueTrue {
			cfg.StrictFallback = true
		}
		if v, ok := tacContainer.Get("accounting"); ok && v == configValueTrue {
			cfg.Accounting = true
		}
	}

	// Privilege level to profile mapping.
	if profiles := auth.GetList("tacacs-profile"); len(profiles) > 0 {
		cfg.PrivLvlMap = make(map[int][]string, len(profiles))
		for levelStr, entry := range profiles {
			if lvl, err := strconv.Atoi(levelStr); err == nil {
				cfg.PrivLvlMap[lvl] = entry.GetSlice("profile")
			}
		}
	}

	return cfg
}
