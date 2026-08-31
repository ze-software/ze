// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS plugin config
// RFC: rfc/short/rfc2869.md -- NAS-Port-Id (Section 5.17)
// RFC: rfc/short/rfc5176.md -- CoA/Disconnect listener port
// Related: register.go -- plugin lifecycle callbacks
// Related: nasportid.go -- nas-port-id-format validation

package l2tpauthradius

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// radiusConfig holds parsed RADIUS server configuration.
type radiusConfig struct {
	Servers       []radius.Server
	Timeout       time.Duration
	Retries       int
	NASIdentifier string

	// AcctInterval is the acct-interval leaf. Zero means the operator set
	// nothing. Neither the leaf nor this parser writes a default, so the absent
	// case reaches acctInterval (acct.go) intact. RFC 2869 Section 2.1 needs
	// that: it tells an absent leaf apart from a value an operator chose.
	AcctInterval time.Duration

	SourceAddress   net.IP // bind outbound RADIUS socket to this IP; nil = any
	CoAPort         int    // RFC 5176 CoA/DM listener port; 0 = disabled
	NASPortIDFormat string // RFC 2869 Section 5.17 template; "" = no attribute

	// RequireMessageAuthenticator discards a CoA/Disconnect-Request that
	// carries no Message-Authenticator attribute. RFC 5176 Section 3.4 makes
	// the attribute optional, so the default is false.
	RequireMessageAuthenticator bool
}

// errNoRADIUSConfig is returned when the config tree has no auth.radius block.
var errNoRADIUSConfig = fmt.Errorf("%s: no auth.radius block in config", Name)

func parseConfigFromTree(tree map[string]any) (*radiusConfig, error) {
	l2tpBlock, ok := tree[configRootL2TP].(map[string]any)
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

	// AcctInterval stays at zero here. A default written in makes every
	// deployment look locally configured, and RFC 2869 Section 2.1 then silences
	// the Access-Accept of every session. The leaf declares no default either,
	// so the schema and this parser say the same thing to an operator.
	cfg := &radiusConfig{
		Timeout: 3 * time.Second,
		Retries: 3,
	}

	if nasID, ok := radiusBlock["nas-identifier"].(string); ok {
		cfg.NASIdentifier = nasID
	}

	// RFC 2869 Section 5.17: NAS-Port-Id is free text, so ze can only refuse
	// what it cannot resolve. A format naming an unknown placeholder is
	// rejected here, at commit time, rather than sent to the RADIUS server
	// with its own syntax still in it.
	if format, ok := radiusBlock["nas-port-id-format"].(string); ok {
		if err := validateNASPortIDFormat(format); err != nil {
			return nil, fmt.Errorf("%s: nas-port-id-format: %w", Name, err)
		}
		// A template whose only content is a placeholder ze will always resolve
		// to nothing sends no attribute at all. Refuse it here rather than leave
		// an operator looking for a NAS-Port-Id that never arrives.
		if strings.Contains(format, "{nas-id}") && cfg.NASIdentifier == "" {
			return nil, fmt.Errorf("%s: nas-port-id-format uses {nas-id} but nas-identifier is not set", Name)
		}
		cfg.NASPortIDFormat = format
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

	requireMA, err := boolFromAny(radiusBlock["require-message-authenticator"])
	if err != nil {
		return nil, fmt.Errorf("%s: require-message-authenticator: %w", Name, err)
	}
	cfg.RequireMessageAuthenticator = requireMA

	// The `server` list is `ordered-by user` (yang/ze-l2tp-auth-radius-conf.yang)
	// because the configured order IS the failover order: Servers[0] is tried
	// first. configorder.Entries carries that order across the JSON boundary.
	// Sorting the keyed map, which is what this did until 2026-08-23, made the
	// operator's primary whichever server name sorted first, and said nothing.
	entries, err := configorder.Entries(radiusBlock, "server", "name")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", Name, err)
	}

	for _, entry := range entries {
		m := entry.Map
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

// boolFromAny coerces a config scalar to a bool. Tree.ToMap() emits scalars as
// strings; JSON-delivered config and unit tests use a real bool. An absent
// field reads as false, which is the YANG default of every boolean leaf this
// plugin declares. A value that is neither form is an error rather than a
// silent false, so an operator who writes one learns it at commit time.
func boolFromAny(raw any) (bool, error) {
	switch v := raw.(type) {
	case nil:
		return false, nil
	case bool:
		return v, nil
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("invalid boolean %q", v)
		}
		return b, nil
	default:
		return false, fmt.Errorf("unexpected type %T", raw)
	}
}
