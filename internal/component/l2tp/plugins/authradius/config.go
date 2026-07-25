// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS plugin config
// Related: register.go -- plugin lifecycle callbacks

package l2tpauthradius

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// radiusConfig holds parsed RADIUS server configuration.
type radiusConfig struct {
	Servers       []radius.Server
	Timeout       time.Duration
	Retries       int
	AcctInterval  time.Duration
	NASIdentifier string
	SourceAddress net.IP // bind outbound RADIUS socket to this IP; nil = any
	CoAPort       int    // RFC 5176 CoA/DM listener port; 0 = disabled
}

// errNoRADIUSConfig is returned when the config tree has no auth.radius block.
var errNoRADIUSConfig = fmt.Errorf("%s: no auth.radius block in config", Name)

func parseConfigFromTree(tree map[string]any) (*radiusConfig, error) {
	l2tpBlock, ok := tree["l2tp"].(map[string]any)
	if !ok {
		return nil, errNoRADIUSConfig
	}
	authBlock, ok := l2tpBlock["auth"].(map[string]any)
	if !ok {
		return nil, errNoRADIUSConfig
	}
	radiusBlock, ok := authBlock["radius"].(map[string]any)
	if !ok {
		return nil, errNoRADIUSConfig
	}

	cfg := &radiusConfig{
		Timeout:      3 * time.Second,
		Retries:      3,
		AcctInterval: 300 * time.Second,
	}

	if nasID, ok := radiusBlock["nas-identifier"].(string); ok {
		cfg.NASIdentifier = nasID
	}

	if src, ok := radiusBlock["source-address"].(string); ok {
		ip := net.ParseIP(src)
		if ip == nil {
			return nil, fmt.Errorf("%s: invalid source-address %q", Name, src)
		}
		if ip.To4() == nil {
			return nil, fmt.Errorf("%s: source-address must be IPv4, got %q", Name, src)
		}
		cfg.SourceAddress = ip.To4()
	}

	if v, present, err := intFromAny(radiusBlock["timeout"]); err != nil {
		return nil, fmt.Errorf("%s: timeout: %w", Name, err)
	} else if present {
		if v < 1 || v > 30 {
			return nil, fmt.Errorf("%s: timeout must be 1-30, got %d", Name, v)
		}
		cfg.Timeout = time.Duration(v) * time.Second
	}

	if v, present, err := intFromAny(radiusBlock["retries"]); err != nil {
		return nil, fmt.Errorf("%s: retries: %w", Name, err)
	} else if present {
		if v < 1 || v > 10 {
			return nil, fmt.Errorf("%s: retries must be 1-10, got %d", Name, v)
		}
		cfg.Retries = v
	}

	if v, present, err := intFromAny(radiusBlock["acct-interval"]); err != nil {
		return nil, fmt.Errorf("%s: acct-interval: %w", Name, err)
	} else if present {
		if v < 60 || v > 3600 {
			return nil, fmt.Errorf("%s: acct-interval must be 60-3600, got %d", Name, v)
		}
		cfg.AcctInterval = time.Duration(v) * time.Second
	}

	if v, present, err := intFromAny(radiusBlock["coa-port"]); err != nil {
		return nil, fmt.Errorf("%s: coa-port: %w", Name, err)
	} else if present {
		if v < 1 || v > 65535 {
			return nil, fmt.Errorf("%s: coa-port must be 1-65535, got %d", Name, v)
		}
		cfg.CoAPort = v
	}

	entries, err := serverEntries(radiusBlock["server"])
	if err != nil {
		return nil, err
	}

	for _, m := range entries {
		address, _ := m["address"].(string)
		if address == "" {
			return nil, fmt.Errorf("%s: server entry missing address", Name)
		}
		port := 1812
		if v, present, err := intFromAny(m["port"]); err != nil {
			return nil, fmt.Errorf("%s: port: %w", Name, err)
		} else if present {
			port = v
			if port < 1 || port > 65535 {
				return nil, fmt.Errorf("%s: port must be 1-65535, got %d", Name, port)
			}
		}
		sharedKey, _ := m["shared-key"].(string)
		if sharedKey == "" {
			return nil, fmt.Errorf("%s: server %s missing shared-key", Name, address)
		}
		cfg.Servers = append(cfg.Servers, radius.Server{
			Address: func() string {
				var b textbuf.Buffer
				return b.Reset().Str(address).Byte(':').Int(int64(port)).String()
			}(),
			SharedKey: []byte(sharedKey),
		})
	}

	if len(cfg.Servers) == 0 {
		return nil, fmt.Errorf("%s: no valid servers configured", Name)
	}

	return cfg, nil
}

// serverEntries normalizes the "server" YANG list into a slice of entry maps.
// Tree.ToMap() (the production verify/configure path) emits a keyed list as a
// map keyed by the entry name: {"radius1": {"address": ...}}. JSON-delivered
// config and unit tests may instead use a flat []any of entry maps. Both shapes
// are accepted.
func serverEntries(raw any) ([]map[string]any, error) {
	if raw == nil {
		return nil, fmt.Errorf("%s: no servers configured", Name)
	}
	switch v := raw.(type) {
	case map[string]any:
		// Keyed list: values are the entry maps; keys are the entry names.
		// Map iteration is unordered, so sort by name for deterministic
		// server (failover) ordering.
		names := make([]string, 0, len(v))
		for name := range v {
			names = append(names, name)
		}
		sort.Strings(names)
		entries := make([]map[string]any, 0, len(v))
		for _, name := range names {
			if m, ok := v[name].(map[string]any); ok {
				entries = append(entries, m)
			}
		}
		return entries, nil
	case []any:
		entries := make([]map[string]any, 0, len(v))
		for _, entry := range v {
			if m, ok := entry.(map[string]any); ok {
				entries = append(entries, m)
			}
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("%s: invalid server list type %T", Name, raw)
	}
}

// intFromAny coerces a config scalar to an int. Tree.ToMap() emits scalars as
// strings; JSON-delivered config and unit tests use float64. Returns
// (value, present, error): present is false when the field is absent.
func intFromAny(raw any) (int, bool, error) {
	switch v := raw.(type) {
	case nil:
		return 0, false, nil
	case float64:
		return int(v), true, nil
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, true, fmt.Errorf("invalid integer %q", v)
		}
		return n, true, nil
	default:
		return 0, false, fmt.Errorf("unexpected type %T", raw)
	}
}
