// Design: docs/architecture/l2tp/bng-5-pppoe.md -- PPPoE configuration
// Related: subsystem.go -- Parameters produced here, consumed at Start

package pppoe

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

const (
	DefaultACName        = "ze"
	DefaultCookieTimeout = 5 * time.Second
	DefaultMaxSessions   = 65535
	DefaultPADIRateLimit = 100
	// DefaultAuthMethod matches the L2TP LNS default: a BNG asks a subscriber
	// who it is. Both transports feed the same PPP driver and the same auth
	// handlers, so an operator who configures a credential for one gets the
	// same treatment on the other.
	DefaultAuthMethod = ppp.AuthMethodCHAPMD5
)

// ErrAuthMethodNoneRequiresAllow refuses the one combination that reads as a
// typo rather than a decision: no authentication method, and no statement that
// unauthenticated subscribers are wanted. Mirrors the L2TP subsystem's rule.
var ErrAuthMethodNoneRequiresAllow = errors.New("pppoe auth-method none requires allow-no-auth true")

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

	// AuthMethod is the PPP Auth-Protocol the AC advertises in its LCP
	// Configure-Request; AllowNoAuth admits a subscriber whose LCP ends with
	// no method negotiated. Both travel to ppp.StartSession (server.go).
	AuthMethod  ppp.AuthMethod
	AllowNoAuth bool
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
//
// The error is returned for a configuration that cannot be honored, so the hub
// refuses to start rather than running an access concentrator that admits
// everybody. Today that is auth-method none without allow-no-auth.
func ExtractParameters(tree map[string]any) (Parameters, error) {
	p := Parameters{
		ACName:        DefaultACName,
		CookieTimeout: DefaultCookieTimeout,
		MaxSessions:   DefaultMaxSessions,
		PADIRateLimit: DefaultPADIRateLimit,
		AuthMethod:    DefaultAuthMethod,
	}

	pppoe, ok := tree["pppoe"].(map[string]any)
	if !ok {
		return p, nil
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
	if method, ok := pppoe["auth-method"].(string); ok && method != "" {
		m, err := ppp.ParseAuthMethod(method)
		if err != nil {
			return Parameters{}, fmt.Errorf("pppoe auth-method: %w", err)
		}
		p.AuthMethod = m
	}
	if allow, ok := cfgBool(pppoe["allow-no-auth"]); ok {
		p.AllowNoAuth = allow
	}
	if p.AuthMethod == ppp.AuthMethodNone && !p.AllowNoAuth {
		return Parameters{}, ErrAuthMethodNoneRequiresAllow
	}

	entries, ok := pppoe["interface"].(map[string]any)
	if !ok {
		return p, nil
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

	return p, nil
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
