// Design: docs/architecture/dns/server-harness.md -- the resolver's listener and policy config
// Related: config_tx.go -- the transaction that verifies and applies a parsed config

package dnsresolver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/internal/core/configvalue"
)

const (
	configRootService = "service"

	defaultPort               = 53
	defaultCacheSize          = 100000
	defaultCacheMaxTTL        = 86400
	defaultCacheMaxNegTTL     = 3600
	defaultTimeoutSeconds     = 10
	defaultLimitOutstanding   = 2048
	defaultLimitPerClientRate = 0

	// forwardServersMax mirrors the YANG max-elements on forward server.
	forwardServersMax = 8
)

// resolveMode is how the resolver finds an answer it has not cached. Zero is
// unspecified, so an unparsed config never reads as a mode.
type resolveMode uint8

const (
	modeUnspecified resolveMode = iota
	modeForward
)

// listenerEndpoint is one configured listener.
type listenerEndpoint struct {
	Name string
	IP   netip.Addr
	Port uint16
}

// upstream is one forward server, tried in configured order.
type upstream struct {
	IP   netip.Addr
	Port uint16
}

// resolverConfig is one parsed service dns-resolver section. Every field is
// a typed value converted once from the YANG leaf.
type resolverConfig struct {
	Enabled            bool
	Listeners          []listenerEndpoint
	AllowFrom          []netip.Prefix
	Mode               resolveMode
	Forward            []upstream
	CacheSize          uint32
	CacheMaxTTL        uint32
	CacheMaxNegTTL     uint32
	Timeout            time.Duration
	LimitOutstanding   uint32
	LimitPerClientRate uint32
}

// defaultConfig is what an absent section runs: disabled, with the YANG
// defaults.
func defaultConfig() resolverConfig {
	return resolverConfig{
		Mode:               modeForward,
		CacheSize:          defaultCacheSize,
		CacheMaxTTL:        defaultCacheMaxTTL,
		CacheMaxNegTTL:     defaultCacheMaxNegTTL,
		Timeout:            defaultTimeoutSeconds * time.Second,
		LimitOutstanding:   defaultLimitOutstanding,
		LimitPerClientRate: defaultLimitPerClientRate,
	}
}

// parseConfig reads the JSON of the service root and validates it. It is the
// one parse both the offline verifier and the reload's verify run, so the two
// cannot disagree.
func parseConfig(data string) (resolverConfig, error) {
	cfg := defaultConfig()

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return cfg, fmt.Errorf("config: unmarshal: %w", err)
	}
	svc, ok := asMap(root, configRootService)
	if !ok {
		return cfg, nil
	}
	r, ok := asMap(svc, "dns-resolver")
	if !ok {
		return cfg, nil
	}

	if v, ok := configvalue.Bool(r["enabled"]); ok {
		cfg.Enabled = v
	}
	listeners, err := parseListeners(r)
	if err != nil {
		return cfg, err
	}
	cfg.Listeners = listeners
	allow, err := parseAllowFrom(r)
	if err != nil {
		return cfg, err
	}
	cfg.AllowFrom = allow
	if v, ok := asString(r, "mode"); ok {
		if v != "forward" {
			return cfg, fmt.Errorf("mode %q is not a resolution mode Ze has; set `mode forward`", v)
		}
		cfg.Mode = modeForward
	}
	forward, err := parseForward(r)
	if err != nil {
		return cfg, err
	}
	cfg.Forward = forward
	if err := parseBounds(r, &cfg); err != nil {
		return cfg, err
	}
	if err := checkEnabled(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// parseListeners reads the listener list in name order. A wildcard address
// is refused whether or not the resolver is enabled: it would answer on
// every address of the host, the WAN side included.
func parseListeners(r map[string]any) ([]listenerEndpoint, error) {
	entries := configvalue.ListEntries(r["listener"])
	out := make([]listenerEndpoint, 0, len(entries))
	for _, entry := range entries {
		ep, err := parseListener(entry.Key, entry.Fields)
		if err != nil {
			return nil, err
		}
		out = append(out, ep)
	}
	return out, nil
}

func parseListener(name string, em map[string]any) (listenerEndpoint, error) {
	ipText, ok := asString(em, "ip")
	if !ok {
		return listenerEndpoint{}, fmt.Errorf("listener %q has no ip; set `listener %s ip <LAN or loopback address>`", name, name)
	}
	ip, err := netip.ParseAddr(ipText)
	if err != nil {
		return listenerEndpoint{}, fmt.Errorf("listener %q ip %q is not an IP address", name, ipText)
	}
	if ip.IsUnspecified() {
		return listenerEndpoint{}, fmt.Errorf("listener %q ip %s is a wildcard address: the resolver would also answer "+
			"on the WAN side and serve the whole internet; set `listener %s ip` to a LAN or loopback address", name, ip, name)
	}
	port := uint16(defaultPort)
	if text, ok := asString(em, "port"); ok {
		p, perr := strconv.ParseUint(text, 10, 16)
		if perr != nil {
			return listenerEndpoint{}, fmt.Errorf("listener %q port %q is not a port number 1-65535", name, text)
		}
		if p == 0 {
			return listenerEndpoint{}, fmt.Errorf("listener %q port 0 asks the system for any port, and no client could find "+
				"it; set `listener %s port 53` or another fixed port", name, name)
		}
		port = uint16(p)
	}
	return listenerEndpoint{Name: name, IP: ip, Port: port}, nil
}

func parseAllowFrom(r map[string]any) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, text := range configvalue.LeafList(r["allow-from"]) {
		p, err := netip.ParsePrefix(text)
		if err != nil {
			return nil, fmt.Errorf("allow-from %q is not an IP prefix", text)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// parseForward reads the forward server list in the order the operator wrote
// it, which is the failover order: the first entry is asked first. The list is
// `ordered-by user` (yang/ze-dns-resolver-conf.yang), as Unbound's
// forward-addr and dnsmasq's server lines are tried in the order written.
// configorder.Entries carries that order across the JSON boundary, and
// refuses a multi-entry list delivered without one rather than inventing an
// order nobody chose.
func parseForward(r map[string]any) ([]upstream, error) {
	fm, ok := asMap(r, "forward")
	if !ok {
		return nil, nil
	}
	entries, err := configorder.Entries(fm, "server", "ip")
	if err != nil {
		return nil, fmt.Errorf("forward: %w", err)
	}
	if len(entries) > forwardServersMax {
		return nil, fmt.Errorf("forward has %d servers, at most %d", len(entries), forwardServersMax)
	}
	out := make([]upstream, 0, len(entries))
	for _, entry := range entries {
		u, uerr := parseUpstream(entry)
		if uerr != nil {
			return nil, uerr
		}
		out = append(out, u)
	}
	return out, nil
}

// parseUpstream reads one forward server entry: its key is the address, and
// its port defaults to 53.
func parseUpstream(entry configorder.Entry) (upstream, error) {
	ip, err := netip.ParseAddr(entry.Key)
	if err != nil {
		return upstream{}, fmt.Errorf("forward server %q is not an IP address", entry.Key)
	}
	u := upstream{IP: ip, Port: defaultPort}
	text, ok := asString(entry.Map, "port")
	if !ok {
		return u, nil
	}
	p, err := strconv.ParseUint(text, 10, 16)
	if err != nil {
		return upstream{}, fmt.Errorf("forward server %s port %q is not a port number 1-65535", entry.Key, text)
	}
	if p == 0 {
		return upstream{}, fmt.Errorf("forward server %s port 0 is not a port a resolver answers on; set `port 53`", entry.Key)
	}
	u.Port = uint16(p)
	return u, nil
}

// parseBounds reads the cache, timeout and limit leaves. YANG holds each to
// its range; the parse refuses a value that does not fit its Go type rather
// than truncating it.
func parseBounds(r map[string]any, cfg *resolverConfig) error {
	if cm, ok := asMap(r, "cache"); ok {
		if err := parseUint32(cm, "cache size", "size", &cfg.CacheSize); err != nil {
			return err
		}
		if err := parseUint32(cm, "cache max-ttl", "max-ttl", &cfg.CacheMaxTTL); err != nil {
			return err
		}
		if err := parseUint32(cm, "cache max-negative-ttl", "max-negative-ttl", &cfg.CacheMaxNegTTL); err != nil {
			return err
		}
	}
	if text, ok := asString(r, "timeout"); ok {
		s, err := strconv.ParseUint(text, 10, 8)
		if err != nil {
			return fmt.Errorf("timeout %q is not a number of seconds 1-60", text)
		}
		if s == 0 {
			return errors.New("timeout 0 would answer every query SERVFAIL; set `timeout` to 1-60 seconds")
		}
		cfg.Timeout = time.Duration(s) * time.Second
	}
	if lm, ok := asMap(r, "limit"); ok {
		if err := parseUint32(lm, "limit outstanding", "outstanding", &cfg.LimitOutstanding); err != nil {
			return err
		}
		if cfg.LimitOutstanding == 0 {
			return errors.New("limit outstanding 0 would drop every query; set it to 1-65535")
		}
		if err := parseUint32(lm, "limit per-client-rate", "per-client-rate", &cfg.LimitPerClientRate); err != nil {
			return err
		}
	}
	return nil
}

func parseUint32(m map[string]any, leaf, key string, out *uint32) error {
	text, ok := asString(m, key)
	if !ok {
		return nil
	}
	v, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return fmt.Errorf("%s %q is not a number 0-4294967295", leaf, text)
	}
	*out = uint32(v)
	return nil
}

// checkEnabled refuses an enabled resolver that could answer nobody, or
// everybody. A disabled one may be staged incomplete.
func checkEnabled(cfg resolverConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if len(cfg.Listeners) == 0 {
		return errors.New("enabled with no listener, so it would answer on no address; " +
			"add `listener <name> ip <LAN or loopback address>`")
	}
	if len(cfg.AllowFrom) == 0 {
		return errors.New("enabled with no allow-from prefix, so the resolver would refuse every client; " +
			"add `allow-from <LAN or subscriber prefix>`")
	}
	if cfg.Mode == modeForward && len(cfg.Forward) == 0 {
		return errors.New("mode forward with no forward server, so the resolver has nowhere to resolve; " +
			"add `forward server <upstream resolver address>`")
	}
	return nil
}

func asMap(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key].(map[string]any)
	return v, ok
}

func asString(m map[string]any, key string) (string, bool) {
	v, ok := m[key].(string)
	return v, ok
}
