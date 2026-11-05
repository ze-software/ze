// Design: docs/architecture/l2tp/bng-5-pppoe.md -- PPPoE configuration
// Related: subsystem.go -- Parameters produced here, consumed at Start

package pppoe

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/configvalue"
)

const (
	DefaultACName        = "ze"
	DefaultCookieTimeout = 5 * time.Second
	DefaultMaxSessions   = 65535
	DefaultPADIRateLimit = 100
	// DefaultMaxSessionsPerMAC bounds how many sessions one subscriber MAC
	// address may hold at once (spec-pppoe-padr-replay-allocates-unbounded-
	// sessions). RFC 2516 places no per-peer session limit, and accel-ppp
	// permits several concurrent sessions from one MAC, so the default sits
	// well above one: high enough for a CPE that opens more than one session
	// (dual-stack, multi-WAN, a set-top box beside the Internet session), and
	// far below the descriptor exhaustion a replay flood is aiming for.
	DefaultMaxSessionsPerMAC = 8
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
	Name              string
	ServiceNames      []string
	MaxSessions       int
	MaxSessionsPerMAC int
}

// Parameters holds the parsed PPPoE subsystem configuration.
type Parameters struct {
	Enabled           bool
	ACName            string
	ServiceNames      []string
	Interfaces        []InterfaceConfig
	CookieTimeout     time.Duration
	MaxSessions       int
	MaxSessionsPerMAC int
	PADIRateLimit     int

	// AuthMethod is the PPP Auth-Protocol the AC advertises in its LCP
	// Configure-Request; AllowNoAuth admits a subscriber whose LCP ends with
	// no method negotiated. Both travel to ppp.StartSession (server.go).
	AuthMethod  ppp.AuthMethod
	AllowNoAuth bool
	// AuthAllow is the effective immutable policy passed to new PPP sessions.
	AuthAllow []ppp.AuthMethod
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
// everybody. Invalid authentication policy is refused rather than defaulted.
func ExtractParameters(tree map[string]any) (Parameters, error) {
	p := Parameters{
		ACName:            DefaultACName,
		CookieTimeout:     DefaultCookieTimeout,
		MaxSessions:       DefaultMaxSessions,
		MaxSessionsPerMAC: DefaultMaxSessionsPerMAC,
		PADIRateLimit:     DefaultPADIRateLimit,
		AuthMethod:        DefaultAuthMethod,
		AuthAllow:         []ppp.AuthMethod{DefaultAuthMethod},
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
	p.ServiceNames = configvalue.LeafList(pppoe["service-name"])
	if timeout, ok := cfgFloat(pppoe["cookie-timeout"]); ok && timeout > 0 {
		p.CookieTimeout = time.Duration(timeout) * time.Second
	}
	if maxSess, ok := cfgFloat(pppoe["max-sessions"]); ok && maxSess > 0 {
		p.MaxSessions = int(maxSess)
	}
	if perMAC, ok := cfgFloat(pppoe["max-sessions-per-mac"]); ok && perMAC > 0 {
		p.MaxSessionsPerMAC = int(perMAC)
	}
	if rateLimit, ok := cfgFloat(pppoe["padi-rate-limit"]); ok && rateLimit > 0 {
		p.PADIRateLimit = int(rateLimit)
	}
	if value, present := pppoe["auth-method"]; present {
		method, ok := value.(string)
		if !ok {
			return Parameters{}, fmt.Errorf("pppoe auth-method must be a method name; set pppoe auth-method chap-md5")
		}
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
	var authNames []string
	if value, present := pppoe["authentication-allow"]; present {
		var err error
		authNames, err = extractAuthAllow(value)
		if err != nil {
			return Parameters{}, err
		}
	}
	var err error
	p.AuthAllow, err = ppp.ResolveAuthAllow(p.AuthMethod, authNames)
	if err != nil {
		return Parameters{}, fmt.Errorf("pppoe authentication-allow: %w; run `delete pppoe authentication-allow` to restore the auth-method default, or run `set pppoe auth-method chap-md5`, `delete pppoe authentication-allow`, then `set pppoe authentication-allow chap-md5` to select CHAP-MD5 only", err)
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
	slices.Sort(names)

	for _, name := range names {
		if name == "" {
			continue
		}
		body, ok := entries[name].(map[string]any)
		if !ok {
			continue
		}
		ic := InterfaceConfig{
			Name:              name,
			ServiceNames:      configvalue.LeafList(body["service-name"]),
			MaxSessions:       p.MaxSessions,
			MaxSessionsPerMAC: p.MaxSessionsPerMAC,
		}
		if maxSess, ok := cfgFloat(body["max-sessions"]); ok && maxSess > 0 {
			ic.MaxSessions = int(maxSess)
		}
		if perMAC, ok := cfgFloat(body["max-sessions-per-mac"]); ok && perMAC > 0 {
			ic.MaxSessionsPerMAC = int(perMAC)
		}
		p.Interfaces = append(p.Interfaces, ic)
	}

	return p, nil
}

// extractAuthAllow accepts every provider leaf-list representation, but never
// silently discards a malformed member of a security policy.
func extractAuthAllow(value any) ([]string, error) {
	switch names := value.(type) {
	case string:
		return []string{names}, nil
	case []string:
		return names, nil
	case []any:
		result := make([]string, len(names))
		for i, member := range names {
			name, ok := member.(string)
			if !ok {
				return nil, fmt.Errorf("pppoe authentication-allow member %d must be a method name; delete pppoe authentication-allow to restore the auth-method default", i+1)
			}
			result[i] = name
		}
		return result, nil
	default:
		return nil, fmt.Errorf("pppoe authentication-allow must be a method name or ordered list; delete pppoe authentication-allow to restore the auth-method default")
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
