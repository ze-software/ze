// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption

// TACACS+ configuration extraction from the YANG config tree.
package tacacs

import (
	"fmt"
	"strconv"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// ExtractedConfig holds TACACS+ configuration extracted from the config tree.
type ExtractedConfig struct {
	Servers       []TacacsServer
	Timeout       time.Duration
	SourceAddress string
	Authorization bool
	Accounting    bool
	PrivLvlMap    map[int][]string // priv-lvl -> ze profile names
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
			srv := TacacsServer{
				Address: fmt.Sprintf("%s:%d", addr, port),
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
		if _, ok := tacContainer.Get("authorization"); ok {
			cfg.Authorization = true
		}
		if _, ok := tacContainer.Get("accounting"); ok {
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
