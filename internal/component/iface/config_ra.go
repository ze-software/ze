// Design: docs/features/interfaces.md -- per-unit Router Advertisement configuration
// Related: config.go -- parseIPv6Settings calls parseRAConfig for the ipv6 container

package iface

import (
	"fmt"
	"net/netip"
	"sort"
	"strconv"
)

// Defaults for the router-advertisement container. Each one repeats the
// default its leaf declares in yang/ze-iface-conf.yang, because the config
// tree delivers only what the operator wrote: an absent leaf arrives as an
// absent key, never as its schema default.
const (
	raDefaultMaximumInterval   = 600     // seconds, RFC 4861 Section 6.2.1 MaxRtrAdvInterval
	raDefaultMinimumInterval   = 200     // seconds, RFC 4861 Section 6.2.1 MinRtrAdvInterval
	raDefaultRouterLifetime    = 1800    // seconds, RFC 4861 Section 6.2.1 AdvDefaultLifetime
	raDefaultHopLimit          = 64      // RFC 4861 Section 6.2.1 AdvCurHopLimit
	raDefaultValidLifetime     = 2592000 // seconds, 30 days
	raDefaultPreferredLifetime = 604800  // seconds, 7 days
)

// Bounds RFC 4861 Section 6.2.1 places on the router configuration variables.
// The YANG ranges state the same numbers for CLI completion and schema
// validation; these re-check them, so a config that reached the parser by any
// other route is still refused rather than advertised.
const (
	raMinMaximumInterval = 4
	raMaxMaximumInterval = 1800
	raMinMinimumInterval = 3
	raMaxMinimumInterval = 1350
	raMaxRouterLifetime  = 9000
	raMaxReachableTime   = 3600000
	// raMinimumIntervalRatioNumerator over raMinimumIntervalRatioDenominator is
	// the 0.75 ceiling RFC 4861 Section 6.2.1 puts on MinRtrAdvInterval,
	// expressed as integers so the check never rounds.
	raMinimumIntervalRatioNumerator   = 3
	raMinimumIntervalRatioDenominator = 4
	// raRDNSSLifetimeIntervalMultiple is the "at least 3 * MaxRtrAdvInterval"
	// default RFC 8106 Section 5.1 recommends for the RDNSS lifetime.
	raRDNSSLifetimeIntervalMultiple = 3
)

// raPrefixConfig is one advertised prefix from the router-advertisement
// prefix list.
type raPrefixConfig struct {
	Prefix            netip.Prefix
	OnLink            bool
	Autonomous        bool
	ValidLifetime     uint32 // seconds
	PreferredLifetime uint32 // seconds
}

// raUnitConfig holds the router-advertisement container of one unit.
type raUnitConfig struct {
	Enabled         bool
	MaximumInterval uint16 // seconds
	MinimumInterval uint16 // seconds
	RouterLifetime  uint16 // seconds
	HopLimit        uint8
	Managed         bool
	OtherConfig     bool
	ReachableTime   uint32 // milliseconds
	RetransmitTimer uint32 // milliseconds
	Prefixes        []raPrefixConfig
	RDNSS           []netip.Addr
	// RDNSSLifetime is nil when the operator set no lifetime, which is
	// different from an explicit 0. RFC 8106 Section 5.1 gives 0 the meaning
	// "stop using these resolvers", so the two cases cannot share a value.
	RDNSSLifetime *uint32 // seconds
}

// EffectiveRDNSSLifetime returns the lifetime to advertise in the RDNSS
// option: what the operator set, or the 3 x MaximumInterval that RFC 8106
// Section 5.1 recommends when they set nothing.
func (c *raUnitConfig) EffectiveRDNSSLifetime() uint32 {
	if c.RDNSSLifetime != nil {
		return *c.RDNSSLifetime
	}
	return uint32(c.MaximumInterval) * raRDNSSLifetimeIntervalMultiple
}

// parseRAConfig reads the router-advertisement container out of a unit's ipv6
// container. It returns nil when the container is absent, and an error for any
// value or combination RFC 4861 forbids, which OnConfigVerify turns into a
// rejected commit.
func parseRAConfig(v6 map[string]any) (*raUnitConfig, error) {
	rm, ok := v6["router-advertisement"].(map[string]any)
	if !ok {
		return nil, nil //nolint:nilnil // absent container means unconfigured, not an error
	}

	cfg := &raUnitConfig{
		MaximumInterval: raDefaultMaximumInterval,
		MinimumInterval: raDefaultMinimumInterval,
		RouterLifetime:  raDefaultRouterLifetime,
		HopLimit:        raDefaultHopLimit,
	}

	if v, ok := rm["enabled"].(string); ok {
		cfg.Enabled = v == yangTrue
	}
	if v, ok := rm["managed"].(string); ok {
		cfg.Managed = v == yangTrue
	}
	if v, ok := rm["other-config"].(string); ok {
		cfg.OtherConfig = v == yangTrue
	}

	if err := raParseUint16(rm, "maximum-interval", &cfg.MaximumInterval); err != nil {
		return nil, err
	}
	if err := raParseUint16(rm, "minimum-interval", &cfg.MinimumInterval); err != nil {
		return nil, err
	}
	if err := raParseUint16(rm, "router-lifetime", &cfg.RouterLifetime); err != nil {
		return nil, err
	}
	if err := raParseUint8(rm, "hop-limit", &cfg.HopLimit); err != nil {
		return nil, err
	}
	if err := raParseUint32(rm, "reachable-time", &cfg.ReachableTime); err != nil {
		return nil, err
	}
	if err := raParseUint32(rm, "retransmit-timer", &cfg.RetransmitTimer); err != nil {
		return nil, err
	}

	if err := raParsePrefixes(rm, cfg); err != nil {
		return nil, err
	}
	if err := raParseRDNSS(rm, cfg); err != nil {
		return nil, err
	}
	if err := raValidate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// raParsePrefixes fills cfg.Prefixes from the prefix list, ordered by prefix so
// the advertised option order does not depend on Go's map iteration.
func raParsePrefixes(rm map[string]any, cfg *raUnitConfig) error {
	pm, ok := rm["prefix"].(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(pm))
	for k := range pm {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		entry, _ := pm[key].(map[string]any)
		p, err := raParsePrefixEntry(key, entry)
		if err != nil {
			return err
		}
		cfg.Prefixes = append(cfg.Prefixes, p)
	}
	return nil
}

// raParsePrefixEntry parses one prefix list entry. The list key carries the
// prefix itself.
func raParsePrefixEntry(key string, entry map[string]any) (raPrefixConfig, error) {
	p := raPrefixConfig{
		OnLink:            true,
		Autonomous:        true,
		ValidLifetime:     raDefaultValidLifetime,
		PreferredLifetime: raDefaultPreferredLifetime,
	}

	parsed, err := netip.ParsePrefix(key)
	if err != nil {
		return p, fmt.Errorf("router-advertisement prefix %q: %w", key, err)
	}
	if !parsed.Addr().Is6() || parsed.Addr().Is4In6() {
		return p, fmt.Errorf("router-advertisement prefix %q: not an IPv6 prefix", key)
	}
	// RFC 4861 Section 4.6.2: the bits after the prefix length must be zero.
	// Masking them silently would advertise something the operator did not
	// write, so the config is refused instead.
	if parsed.Masked() != parsed {
		return p, fmt.Errorf("router-advertisement prefix %q: host bits set below the prefix length, write %s",
			key, parsed.Masked())
	}
	// RFC 4861 Section 4.6.2: a router should not send a prefix option for the
	// link-local prefix, and a host should ignore one.
	if parsed.Addr().IsLinkLocalUnicast() {
		return p, fmt.Errorf("router-advertisement prefix %q: the link-local prefix is not advertised (RFC 4861 Section 4.6.2)", key)
	}
	p.Prefix = parsed

	if entry == nil {
		return p, nil
	}
	if v, ok := entry["on-link"].(string); ok {
		p.OnLink = v == yangTrue
	}
	if v, ok := entry["autonomous"].(string); ok {
		p.Autonomous = v == yangTrue
	}
	if err := raParseUint32(entry, "valid-lifetime", &p.ValidLifetime); err != nil {
		return p, fmt.Errorf("router-advertisement prefix %q: %w", key, err)
	}
	if err := raParseUint32(entry, "preferred-lifetime", &p.PreferredLifetime); err != nil {
		return p, fmt.Errorf("router-advertisement prefix %q: %w", key, err)
	}
	// RFC 4861 Section 4.6.2: the Preferred Lifetime must not exceed the Valid
	// Lifetime, so a host never prefers an address that has stopped being valid
	// (RFC 4862 Section 5.5.3).
	if p.PreferredLifetime > p.ValidLifetime {
		return p, fmt.Errorf("router-advertisement prefix %q: preferred-lifetime %d is above valid-lifetime %d",
			key, p.PreferredLifetime, p.ValidLifetime)
	}
	return p, nil
}

// raParseRDNSS fills the resolver list and its lifetime from the rdnss
// container.
func raParseRDNSS(rm map[string]any, cfg *raUnitConfig) error {
	dm, ok := rm["rdnss"].(map[string]any)
	if !ok {
		return nil
	}
	for _, s := range parseStringList(dm, "server") {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return fmt.Errorf("router-advertisement rdnss server %q: %w", s, err)
		}
		if !addr.Is6() || addr.Is4In6() {
			return fmt.Errorf("router-advertisement rdnss server %q: not an IPv6 address", s)
		}
		cfg.RDNSS = append(cfg.RDNSS, addr)
	}
	if v, ok := dm["lifetime"].(string); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("router-advertisement rdnss lifetime %q: %w", v, err)
		}
		lifetime := uint32(n)
		cfg.RDNSSLifetime = &lifetime
	}
	return nil
}

// raValidate applies the bounds and the cross-leaf rules of RFC 4861
// Section 6.2.1, which no single-leaf YANG range can express.
func raValidate(cfg *raUnitConfig) error {
	if cfg.MaximumInterval < raMinMaximumInterval || cfg.MaximumInterval > raMaxMaximumInterval {
		return fmt.Errorf("router-advertisement maximum-interval %d: outside %d..%d seconds (RFC 4861 Section 6.2.1)",
			cfg.MaximumInterval, raMinMaximumInterval, raMaxMaximumInterval)
	}
	if cfg.MinimumInterval < raMinMinimumInterval || cfg.MinimumInterval > raMaxMinimumInterval {
		return fmt.Errorf("router-advertisement minimum-interval %d: outside %d..%d seconds (RFC 4861 Section 6.2.1)",
			cfg.MinimumInterval, raMinMinimumInterval, raMaxMinimumInterval)
	}
	// RFC 4861 Section 6.2.1: MinRtrAdvInterval must be no greater than
	// 0.75 * MaxRtrAdvInterval.
	if int(cfg.MinimumInterval)*raMinimumIntervalRatioDenominator > int(cfg.MaximumInterval)*raMinimumIntervalRatioNumerator {
		return fmt.Errorf("router-advertisement minimum-interval %d: above three quarters of maximum-interval %d (RFC 4861 Section 6.2.1)",
			cfg.MinimumInterval, cfg.MaximumInterval)
	}
	if cfg.RouterLifetime > raMaxRouterLifetime {
		return fmt.Errorf("router-advertisement router-lifetime %d: above %d seconds (RFC 4861 Section 6.2.1)",
			cfg.RouterLifetime, raMaxRouterLifetime)
	}
	// RFC 4861 Section 6.2.1: AdvDefaultLifetime is "either zero or between
	// MaxRtrAdvInterval and 9000 seconds". Zero is a value an operator sets on
	// purpose, to advertise prefixes without becoming a default router, so it
	// is accepted at any interval.
	if cfg.RouterLifetime != 0 && cfg.RouterLifetime < cfg.MaximumInterval {
		return fmt.Errorf("router-advertisement router-lifetime %d: below maximum-interval %d, and not 0 (RFC 4861 Section 6.2.1)",
			cfg.RouterLifetime, cfg.MaximumInterval)
	}
	if cfg.ReachableTime > raMaxReachableTime {
		return fmt.Errorf("router-advertisement reachable-time %d: above %d milliseconds (RFC 4861 Section 6.2.1)",
			cfg.ReachableTime, raMaxReachableTime)
	}
	return nil
}

// raParseUint16 reads one uint16 leaf, leaving dst untouched when the leaf is
// absent so the caller's default survives.
func raParseUint16(m map[string]any, leaf string, dst *uint16) error {
	v, ok := m[leaf].(string)
	if !ok {
		return nil
	}
	n, err := strconv.ParseUint(v, 10, 16)
	if err != nil {
		return fmt.Errorf("router-advertisement %s %q: %w", leaf, v, err)
	}
	*dst = uint16(n)
	return nil
}

// raParseUint8 reads one uint8 leaf, leaving dst untouched when the leaf is
// absent.
func raParseUint8(m map[string]any, leaf string, dst *uint8) error {
	v, ok := m[leaf].(string)
	if !ok {
		return nil
	}
	n, err := strconv.ParseUint(v, 10, 8)
	if err != nil {
		return fmt.Errorf("router-advertisement %s %q: %w", leaf, v, err)
	}
	*dst = uint8(n)
	return nil
}

// raParseUint32 reads one uint32 leaf, leaving dst untouched when the leaf is
// absent.
func raParseUint32(m map[string]any, leaf string, dst *uint32) error {
	v, ok := m[leaf].(string)
	if !ok {
		return nil
	}
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return fmt.Errorf("router-advertisement %s %q: %w", leaf, v, err)
	}
	*dst = uint32(n)
	return nil
}
