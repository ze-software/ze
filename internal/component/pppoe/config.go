// Design: plan/learned/669-bng-5-pppoe.md -- PPPoE configuration

package pppoe

import "time"

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

	if enabled, ok := pppoe["enabled"].(bool); ok {
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
	if timeout, ok := pppoe["cookie-timeout"].(float64); ok && timeout > 0 {
		p.CookieTimeout = time.Duration(timeout) * time.Second
	}
	if maxSess, ok := pppoe["max-sessions"].(float64); ok && maxSess > 0 {
		p.MaxSessions = int(maxSess)
	}
	if rateLimit, ok := pppoe["padi-rate-limit"].(float64); ok && rateLimit > 0 {
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
			if maxSess, ok := ifm["max-sessions"].(float64); ok && maxSess > 0 {
				ic.MaxSessions = int(maxSess)
			}
			if ic.Name != "" {
				p.Interfaces = append(p.Interfaces, ic)
			}
		}
	}

	return p
}
