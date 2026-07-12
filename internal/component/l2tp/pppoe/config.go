// Design: plan/learned/669-bng-5-pppoe.md -- PPPoE configuration
// Related: subsystem.go -- Parameters produced here, consumed at Start

package pppoe

import (
	"strconv"
	"strings"
	"time"
)

const (
	DefaultACName        = "ze"
	DefaultCookieTimeout = 5 * time.Second
	DefaultMaxSessions   = 65535
	DefaultPADIRateLimit = 100
)

// InterfaceConfig holds per-interface PPPoE settings.
type InterfaceConfig struct {
	Name         string
	ServiceNames []string
	MaxSessions  int
}

// Parameters holds the parsed PPPoE subsystem configuration.
type Parameters struct {
	Enabled       bool
	ACName        string
	ServiceNames  []string
	Interfaces    []InterfaceConfig
	CookieTimeout time.Duration
	MaxSessions   int
	PADIRateLimit int
}

// ExtractParameters parses PPPoE configuration from the YANG config
// tree. Missing values get sensible defaults.
func ExtractParameters(tree map[string]any) Parameters {
	p := Parameters{
		ACName:        DefaultACName,
		CookieTimeout: DefaultCookieTimeout,
		MaxSessions:   DefaultMaxSessions,
		PADIRateLimit: DefaultPADIRateLimit,
	}

	pppoe, ok := tree["pppoe"].(map[string]any)
	if !ok {
		return p
	}

	if enabled, ok := cfgBool(pppoe["enabled"]); ok {
		p.Enabled = enabled
	}
	if acName, ok := pppoe["ac-name"].(string); ok && acName != "" {
		p.ACName = acName
	}
	if svcNames, ok := pppoe["service-name"].([]any); ok {
		for _, v := range svcNames {
			if s, ok := v.(string); ok {
				p.ServiceNames = append(p.ServiceNames, s)
			}
		}
	}
	if timeout, ok := cfgFloat(pppoe["cookie-timeout"]); ok && timeout > 0 {
		p.CookieTimeout = time.Duration(timeout) * time.Second
	}
	if maxSess, ok := cfgFloat(pppoe["max-sessions"]); ok && maxSess > 0 {
		p.MaxSessions = int(maxSess)
	}
	if rateLimit, ok := cfgFloat(pppoe["padi-rate-limit"]); ok && rateLimit > 0 {
		p.PADIRateLimit = int(rateLimit)
	}

	if ifaces, ok := pppoe["interface"].([]any); ok {
		for _, v := range ifaces {
			ifm, ok := v.(map[string]any)
			if !ok {
				continue
			}
			ic := InterfaceConfig{
				MaxSessions: p.MaxSessions,
			}
			if name, ok := ifm["name"].(string); ok {
				ic.Name = name
			}
			if svcNames, ok := ifm["service-name"].([]any); ok {
				for _, s := range svcNames {
					if str, ok := s.(string); ok {
						ic.ServiceNames = append(ic.ServiceNames, str)
					}
				}
			}
			if maxSess, ok := cfgFloat(ifm["max-sessions"]); ok && maxSess > 0 {
				ic.MaxSessions = int(maxSess)
			}
			if ic.Name != "" {
				p.Interfaces = append(p.Interfaces, ic)
			}
		}
	}

	return p
}

// cfgBool coerces a config value (native JSON bool or the string form "true"/
// "false" the plugin config framework delivers) to bool. Without the string
// case, `enabled` arriving as "true" fails a v.(bool) assertion and the whole
// PPPoE subsystem stays disabled.
func cfgBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		pb, err := strconv.ParseBool(strings.TrimSpace(b))
		if err != nil {
			return false, false
		}
		return pb, true
	default:
		return false, false
	}
}

// cfgFloat coerces a config value (native JSON number or the string form the
// plugin config framework delivers, e.g. "1000") to float64.
func cfgFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
