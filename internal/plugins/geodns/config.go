// Design: plan/learned/992-geodns-1-config.md -- geodns config parse + validation
// RFC: rfc/short/rfc2181.md -- TTL bounds (section 8); rfc/short/rfc2782.md -- SRV records

package geodns

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// maxTTL is the largest TTL permitted by RFC 2181 section 8 (2^31 - 1): the
// RFC 1035 TTL field is 32 bits but RFC 2181 narrows it to a 31-bit unsigned
// range so the sign bit is never set on the wire.
const maxTTL = 2147483647

const (
	defaultListenPort = 5300
	defaultTTLSeconds = 300
	maxNameservers    = 9
)

// configValueTrue is the canonical boolean-true spelling in config leaf values.
const configValueTrue = "true"

// defaultClientIPSource and the valid set mirror the YANG enumeration.
const defaultClientIPSource = "edns0-then-packet"

var validClientIPSources = map[string]bool{
	"edns0":             true,
	"packet":            true,
	"edns0-then-packet": true,
}

var validSerialModes = map[string]bool{
	"auto-epoch":    true,
	"auto-datetime": true,
	"fixed":         true,
}

// soaConfig holds the configurable SOA fields. Serial generation per SerialMode
// happens at reload (see resolver state), not here.
type soaConfig struct {
	MName      string
	Contact    string
	SerialMode string
	Serial     uint32
	Refresh    uint32
	Retry      uint32
	Expire     uint32
	Minimum    uint32
}

// hostSet is a named, reusable set of records keyed by fully-qualified host name.
type hostSet struct {
	Name  string
	Hosts map[string][]dnsRecord
}

// sourceEntry maps a client-IP prefix to a host-set name (longest prefix wins).
type sourceEntry struct {
	Prefix  netip.Prefix
	HostSet string
}

// listenerEndpoint is one bound UDP+TCP endpoint (ze zt:listener model).
type listenerEndpoint struct {
	IP   netip.Addr
	Port uint16
}

// geodnsConfig is the parsed, validated configuration.
type geodnsConfig struct {
	Enabled        bool
	Listeners      []listenerEndpoint
	DefaultTTL     uint32
	ClientIPSource string
	Zones          []string
	Nameservers    []netip.Addr
	SOA            soaConfig
	HostSets       map[string]*hostSet
	Sources        []sourceEntry
	// Secure holds the optional DoT (RFC 7858) / DoH (RFC 8484) listener config;
	// both bind the configured listener IPs on their own ports and share the tls
	// cert material.
	Secure dnsserver.SecureConfig
}

// parseConfig unmarshals the JSON config section and validates every field. It
// is the single source of truth for both the offline verifier and the engine's
// OnConfigure, so a config that parses here is loadable. An empty/missing
// service.geodns container yields a zero (disabled) config.
func parseConfig(data string) (geodnsConfig, error) {
	var cfg geodnsConfig
	cfg.DefaultTTL = defaultTTLSeconds
	cfg.ClientIPSource = defaultClientIPSource
	cfg.HostSets = map[string]*hostSet{}
	cfg.SOA = soaConfig{Contact: "hostmaster", SerialMode: "auto-epoch", Refresh: 3600, Retry: 600, Expire: 300, Minimum: 300}
	cfg.Secure = dnsserver.DefaultSecureConfig()

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return cfg, fmt.Errorf("geodns config: unmarshal: %w", err)
	}
	svc, ok := asMap(root, "service")
	if !ok {
		return cfg, nil
	}
	g, ok := asMap(svc, "geodns")
	if !ok {
		return cfg, nil
	}

	if v, ok := asString(g, "enabled"); ok {
		cfg.Enabled = v == configValueTrue
	}

	listeners, err := parseListeners(g)
	if err != nil {
		return cfg, err
	}
	cfg.Listeners = listeners

	if v, ok := asString(g, "default-ttl"); ok {
		ttl, err := parseTTL(v)
		if err != nil {
			return cfg, fmt.Errorf("geodns: default-ttl: %w", err)
		}
		// A per-record TTL may be 0 (RFC 2181 section 8), but the daemon default
		// must be >= 1 so an unspecified host TTL is never zero (matches the
		// reference daemon).
		if ttl < 1 {
			return cfg, fmt.Errorf("geodns: default-ttl must be 1..%d, 0 is not allowed for the default", maxTTL)
		}
		cfg.DefaultTTL = ttl
	}

	if v, ok := asString(g, "client-ip-source"); ok {
		if !validClientIPSources[v] {
			return cfg, fmt.Errorf("geodns: client-ip-source %q invalid (edns0|packet|edns0-then-packet)", v)
		}
		cfg.ClientIPSource = v
	}

	for _, z := range asStringList(g, "zone") {
		cfg.Zones = append(cfg.Zones, fqdn(z))
	}

	ns, err := parseAddrList(g, "nameserver", true)
	if err != nil {
		return cfg, err
	}
	if len(ns) > maxNameservers {
		return cfg, fmt.Errorf("geodns: %d nameserver entries, max %d", len(ns), maxNameservers)
	}
	cfg.Nameservers = ns

	if soaMap, ok := asMap(g, "soa"); ok {
		if err := parseSOA(soaMap, &cfg.SOA); err != nil {
			return cfg, err
		}
	}

	if hsMap, ok := asMap(g, "host-set"); ok {
		names := sortedKeys(hsMap)
		for _, name := range names {
			hm, ok := hsMap[name].(map[string]any)
			if !ok {
				continue
			}
			hs, err := parseHostSet(name, hm, cfg.Zones, cfg.DefaultTTL)
			if err != nil {
				return cfg, err
			}
			cfg.HostSets[name] = hs
		}
	}

	if srcMap, ok := asMap(g, "source"); ok {
		prefixes := sortedKeys(srcMap)
		for _, pfx := range prefixes {
			sm, ok := srcMap[pfx].(map[string]any)
			if !ok {
				continue
			}
			prefix, err := netip.ParsePrefix(pfx)
			if err != nil {
				return cfg, fmt.Errorf("geodns: source prefix %q invalid: %w", pfx, err)
			}
			setName, _ := asString(sm, "host-set")
			if setName == "" {
				return cfg, fmt.Errorf("geodns: source %q has no host-set", pfx)
			}
			if _, ok := cfg.HostSets[setName]; !ok {
				return cfg, fmt.Errorf("geodns: source %q references unknown host-set %q", pfx, setName)
			}
			cfg.Sources = append(cfg.Sources, sourceEntry{Prefix: prefix.Masked(), HostSet: setName})
		}
	}

	// tls (DoT) + doh (DoH) listener config: shared parse, native-mirror port
	// validation. Defaults seeded above via DefaultSecureConfig.
	if err := dnsserver.ParseSecureLeaves(g, &cfg.Secure, "geodns"); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func parseSOA(m map[string]any, soa *soaConfig) error {
	if v, ok := asString(m, "mname"); ok {
		soa.MName = fqdn(v)
	}
	if v, ok := asString(m, "contact"); ok {
		soa.Contact = v
	}
	if v, ok := asString(m, "serial-mode"); ok {
		if !validSerialModes[v] {
			return fmt.Errorf("geodns: soa serial-mode %q invalid (auto-epoch|auto-datetime|fixed)", v)
		}
		soa.SerialMode = v
	}
	for _, f := range []struct {
		key string
		dst *uint32
	}{
		{"serial", &soa.Serial}, {"refresh", &soa.Refresh}, {"retry", &soa.Retry},
		{"expire", &soa.Expire}, {"minimum", &soa.Minimum},
	} {
		if v, ok := asString(m, f.key); ok {
			n, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return fmt.Errorf("geodns: soa %s %q invalid: %w", f.key, v, err)
			}
			*f.dst = uint32(n)
		}
	}
	return nil
}

func parseHostSet(name string, m map[string]any, zones []string, defaultTTL uint32) (*hostSet, error) {
	hs := &hostSet{Name: name, Hosts: map[string][]dnsRecord{}}
	hostMap, ok := asMap(m, "host")
	if !ok {
		return hs, nil
	}
	for _, hn := range sortedKeys(hostMap) {
		hm, ok := hostMap[hn].(map[string]any)
		if !ok {
			continue
		}
		host := fqdn(hn)
		if !hasZoneSuffix(host, zones) {
			return nil, fmt.Errorf("geodns: host %q (set %q) is not in any configured zone", host, name)
		}
		recs, err := parseHost(host, hm, defaultTTL)
		if err != nil {
			return nil, fmt.Errorf("geodns: host-set %q: %w", name, err)
		}
		hs.Hosts[host] = recs
	}
	return hs, nil
}

func parseHost(host string, m map[string]any, defaultTTL uint32) ([]dnsRecord, error) {
	ttl := defaultTTL
	if v, ok := asString(m, "ttl"); ok {
		t, err := parseTTL(v)
		if err != nil {
			return nil, fmt.Errorf("host %q ttl: %w", host, err)
		}
		ttl = t
	}

	rtype, _ := asString(m, "type")
	rtype = strings.ToUpper(rtype)

	if rtype == "SRV" {
		rec, err := parseSRV(host, m, ttl)
		if err != nil {
			return nil, err
		}
		return []dnsRecord{rec}, nil
	}

	var recs []dnsRecord
	for _, a := range asStringList(m, "address") {
		addr, err := netip.ParseAddr(a)
		if err != nil {
			return nil, fmt.Errorf("host %q address %q is not a valid IP", host, a)
		}
		switch rtype {
		case "A":
			if !addr.Is4() {
				return nil, fmt.Errorf("host %q type A but address %q is not IPv4", host, a)
			}
		case "AAAA":
			if addr.Is4() {
				return nil, fmt.Errorf("host %q type AAAA but address %q is not IPv6", host, a)
			}
		case "":
			// auto-detect per address
		default:
			return nil, fmt.Errorf("host %q has unknown record type %q", host, rtype)
		}
		recs = append(recs, addrRecord(ttl, addr))
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("host %q has no address records", host)
	}
	return recs, nil
}

func parseSRV(host string, m map[string]any, ttl uint32) (dnsRecord, error) {
	srv, ok := asMap(m, "srv")
	if !ok {
		return dnsRecord{}, fmt.Errorf("host %q type SRV but no srv block", host)
	}
	var fields [3]uint16
	for i, key := range []string{"priority", "weight", "port"} {
		v, ok := asString(srv, key)
		if !ok {
			return dnsRecord{}, fmt.Errorf("host %q srv missing %s (need priority weight port target)", host, key)
		}
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return dnsRecord{}, fmt.Errorf("host %q srv %s %q is not a number in 0..65535", host, key, v)
		}
		fields[i] = uint16(n)
	}
	target, ok := asString(srv, "target")
	if !ok || target == "" {
		return dnsRecord{}, fmt.Errorf("host %q srv missing target (need priority weight port target)", host)
	}
	return srvRecord(ttl, fields[0], fields[1], fields[2], fqdn(target)), nil
}

// parseTTL parses a decimal TTL and bounds it to RFC 2181 section 8 (0..maxTTL).
func parseTTL(v string) (uint32, error) {
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil || n > maxTTL {
		return 0, fmt.Errorf("ttl %q out of range, must be 0..%d", v, maxTTL)
	}
	return uint32(n), nil
}

// fqdn lower-cases and adds a trailing dot.
func fqdn(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s
	}
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

func hasZoneSuffix(host string, zones []string) bool {
	for _, z := range zones {
		if z != "" && strings.HasSuffix(host, z) {
			return true
		}
	}
	return false
}

func asMap(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key].(map[string]any)
	return v, ok
}

func asString(m map[string]any, key string) (string, bool) {
	v, ok := m[key].(string)
	return v, ok
}

// asStringList reads a leaf-list, which arrives as a JSON array of strings or a
// single string.
func asStringList(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

// parseAddrList reads a leaf-list of IP addresses; v4Only rejects IPv6.
func parseAddrList(m map[string]any, key string, v4Only bool) ([]netip.Addr, error) {
	var out []netip.Addr
	for _, s := range asStringList(m, key) {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("geodns: %s %q is not a valid IP", key, s)
		}
		if v4Only && !addr.Is4() {
			return nil, fmt.Errorf("geodns: %s %q must be IPv4", key, s)
		}
		out = append(out, addr)
	}
	return out, nil
}

// parseListeners reads the listener list (keyed by name, each with ip + port).
// When no entry is configured it defaults to 127.0.0.1:5300 and ::1:5300. A
// per-listener port of 0 is rejected (a DNS server needs a fixed port even
// though the zt:listener type permits 0 for OS-assigned).
func parseListeners(g map[string]any) ([]listenerEndpoint, error) {
	lm, ok := asMap(g, "listener")
	if !ok || len(lm) == 0 {
		return []listenerEndpoint{
			{IP: netip.MustParseAddr("127.0.0.1"), Port: defaultListenPort},
			{IP: netip.MustParseAddr("::1"), Port: defaultListenPort},
		}, nil
	}
	var out []listenerEndpoint
	for _, name := range sortedKeys(lm) {
		em, ok := lm[name].(map[string]any)
		if !ok {
			continue
		}
		ipStr, _ := asString(em, "ip")
		if ipStr == "" {
			ipStr = "127.0.0.1"
		}
		ip, err := netip.ParseAddr(ipStr)
		if err != nil {
			return nil, fmt.Errorf("geodns: listener %q ip %q is not a valid IP", name, ipStr)
		}
		port := uint16(defaultListenPort)
		if ps, ok := asString(em, "port"); ok {
			p, perr := strconv.ParseUint(ps, 10, 16)
			if perr != nil || p == 0 {
				return nil, fmt.Errorf("geodns: listener %q port %q invalid, must be 1..65535", name, ps)
			}
			port = uint16(p)
		}
		out = append(out, listenerEndpoint{IP: ip, Port: port})
	}
	return out, nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
