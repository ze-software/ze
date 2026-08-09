// Design: docs/architecture/l2tp/bng-5-pppoe.md -- PPPoE configuration
// Related: subsystem.go -- Parameters produced here, consumed at Start

package pppoe

import (
	"sort"
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

// ExtractParameters parses PPPoE configuration from the YANG config tree, in
// the shape Tree.ToMap produces (internal/component/config/tree.go), which is
// the only thing the hub ever passes it (cmd/ze/hub/register_l2tp.go). Missing
// values get sensible defaults.
//
// ToMap emits a keyed YANG list as a map of key to entry, and a leaf-list as one
// string or a []string. It never emits a []any. Reading `interface` as a []any
// therefore found nothing on every real config, so Interfaces was always empty,
// so registerBNGSubsystems never registered the subsystem, so a configured AC
// answered no PADI at all. Two unit tests passed throughout because both built
// the map by hand in a shape no producer emits.
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
	p.ServiceNames = cfgStrings(pppoe["service-name"])
	if timeout, ok := cfgFloat(pppoe["cookie-timeout"]); ok && timeout > 0 {
		p.CookieTimeout = time.Duration(timeout) * time.Second
	}
	if maxSess, ok := cfgFloat(pppoe["max-sessions"]); ok && maxSess > 0 {
		p.MaxSessions = int(maxSess)
	}
	if rateLimit, ok := cfgFloat(pppoe["padi-rate-limit"]); ok && rateLimit > 0 {
		p.PADIRateLimit = int(rateLimit)
	}

	entries, ok := pppoe["interface"].(map[string]any)
	if !ok {
		return p
	}
	// The list key IS the interface name: `interface veth-bng { }` yields the
	// entry "veth-bng" mapped to an empty body, so nothing inside carries the
	// name. Sorted so two runs of one config produce the same Interfaces order.
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if name == "" {
			continue
		}
		body, ok := entries[name].(map[string]any)
		if !ok {
			continue
		}
		ic := InterfaceConfig{
			Name:         name,
			ServiceNames: cfgStrings(body["service-name"]),
			MaxSessions:  p.MaxSessions,
		}
		if maxSess, ok := cfgFloat(body["max-sessions"]); ok && maxSess > 0 {
			ic.MaxSessions = int(maxSess)
		}
		p.Interfaces = append(p.Interfaces, ic)
	}

	return p
}

// cfgStrings coerces a YANG leaf-list to a slice. Tree.ToMap collapses a
// single-member leaf-list to a bare string and emits several members as a
// []string, so both shapes are the producer's and both are read here.
func cfgStrings(v any) []string {
	switch s := v.(type) {
	case string:
		if s == "" {
			return nil
		}
		return []string{s}
	case []string:
		return append([]string(nil), s...)
	default:
		return nil
	}
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
