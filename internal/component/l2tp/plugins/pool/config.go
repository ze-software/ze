// Design: docs/architecture/l2tp/subscriber-session-model.md -- declarative pool configuration
// Related: register.go -- verifies descriptions and constructs configured pools

package l2tppool

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/opererr"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// poolConfiguration holds immutable, validated descriptions, never live
// bitmaps. Its zero value means no pools. Only the parser creates nonzero
// descriptions, and callers MUST NOT mutate their maps after publication.
type poolConfiguration struct {
	defaultIPv4 *ipv4PoolDescription
	namedIPv4   map[string]ipv4PoolDescription
	defaultIPv6 *ipv6PoolDescription
	namedIPv6   map[string]ipv6PoolDescription
}

// A zero description is uninitialized. Only a successful parser hands one to
// build; optional pools use nil pointers rather than zero descriptions.
type ipv4PoolDescription struct {
	gateway      netip.Addr
	start        netip.Addr
	end          netip.Addr
	dnsPrimary   netip.Addr
	dnsSecondary netip.Addr
	sources      dnssource.List
}

type ipv6PoolDescription struct {
	block    netip.Prefix
	delegLen int
	size     uint32
}

func parsePoolSections(sections []sdk.ConfigSection) (poolConfiguration, bool, error) {
	for _, section := range sections {
		if section.Root != configRootL2TP {
			continue
		}
		cfg, err := parsePoolDescription(section.Data)
		if err != nil {
			if _, ok := errors.AsType[*opererr.Error](err); ok {
				return poolConfiguration{}, false, err
			}
			return poolConfiguration{}, false, opererr.Wrap(err,
				"L2TP pools could not be configured", "the candidate is refused and the running pools are unchanged", "ze show config")
		}
		return cfg, true, nil
	}
	return poolConfiguration{}, false, nil
}

func parsePoolDescription(data string) (poolConfiguration, error) {
	var cfg poolConfiguration
	if data == "" {
		return cfg, nil
	}
	var tree map[string]any
	if err := json.Unmarshal([]byte(data), &tree); err != nil {
		return cfg, fmt.Errorf("%s: invalid config JSON: %w", Name, err)
	}
	if tree == nil {
		return cfg, fmt.Errorf("%s: l2tp config section must be an object; use {} to remove the root", Name)
	}
	l2tpBlock, present, err := poolConfigObject(tree, configRootL2TP, "l2tp")
	if err != nil {
		return cfg, err
	}
	// refuse-by-default: guards parsePoolSections -- an absent root is a valid empty pool description
	if !present {
		return cfg, nil
	}
	poolBlock, present, err := poolConfigObject(l2tpBlock, "pool", "l2tp pool")
	if err != nil {
		return cfg, err
	}
	// refuse-by-default: guards parsePoolSections -- an absent pool block is a valid empty pool description
	if !present {
		return cfg, nil
	}
	block, present, err := poolConfigObject(poolBlock, "ipv4", "l2tp pool ipv4")
	if err != nil {
		return cfg, err
	}
	if present {
		description, err := parseIPv4Description(block, poolDNSSourcePath, poolDNSDefault, "l2tp pool ipv4")
		if err != nil {
			return poolConfiguration{}, err
		}
		cfg.defaultIPv4 = &description
	}
	entries, err := poolConfigList(poolBlock, "named-pool", "l2tp pool named-pool")
	if err != nil {
		return cfg, err
	}
	for _, entry := range entries {
		if entry.Key == "" {
			return poolConfiguration{}, fmt.Errorf("%s: named-pool requires a name", Name)
		}
		path := "l2tp pool named-pool " + configtext.QuoteCommandWord(entry.Key)
		description, err := parseIPv4Description(entry.Fields, path+" dns-source", namedPoolDNSDefault, path)
		if err != nil {
			return poolConfiguration{}, fmt.Errorf("%s: named-pool %q: %w", Name, entry.Key, err)
		}
		if cfg.namedIPv4 == nil {
			cfg.namedIPv4 = make(map[string]ipv4PoolDescription)
		}
		cfg.namedIPv4[entry.Key] = description
	}
	block, present, err = poolConfigObject(poolBlock, "ipv6-pd", "l2tp pool ipv6-pd")
	if err != nil {
		return cfg, err
	}
	if present {
		description, err := parseIPv6Description(block)
		if err != nil {
			return poolConfiguration{}, err
		}
		cfg.defaultIPv6 = &description
	}
	entries, err = poolConfigList(poolBlock, "named-ipv6-pool", "l2tp pool named-ipv6-pool")
	if err != nil {
		return cfg, err
	}
	for _, entry := range entries {
		if entry.Key == "" {
			return poolConfiguration{}, fmt.Errorf("%s: named-ipv6-pool requires a name", Name)
		}
		description, err := parseIPv6Description(entry.Fields)
		if err != nil {
			return poolConfiguration{}, fmt.Errorf("%s: named-ipv6-pool %q: %w", Name, entry.Key, err)
		}
		if cfg.namedIPv6 == nil {
			cfg.namedIPv6 = make(map[string]ipv6PoolDescription)
		}
		cfg.namedIPv6[entry.Key] = description
	}
	if err := validatePoolConfiguration(cfg); err != nil {
		return poolConfiguration{}, err
	}
	return cfg, nil
}

func parseIPv4Description(block map[string]any, sourcesAt string, builtin dnssource.List, path string) (ipv4PoolDescription, error) {
	var description ipv4PoolDescription
	for _, leaf := range []string{"gateway", "start", "end"} {
		value, _ := block[leaf].(string)
		if value == "" {
			return description, fmt.Errorf("%s: pool ipv4 requires %s", Name, leaf)
		}
		address, err := netip.ParseAddr(value)
		if err != nil {
			return description, fmt.Errorf("%s: invalid %s address %q: %w", Name, leaf, value, err)
		}
		if !address.Is4() {
			return description, fmt.Errorf("%s: %s must be IPv4", Name, leaf)
		}
		// The map is the raw config boundary; the leaf selects a field of
		// this one fixed pool shape, not a feature dispatch.
		switch leaf {
		case "gateway":
			description.gateway = address
		case "start":
			description.start = address
		case "end":
			description.end = address
		}
	}
	if description.end.Less(description.start) {
		return description, fmt.Errorf("%s: end %s is before start %s", Name, description.end, description.start)
	}
	for _, leaf := range []string{"dns-primary", "dns-secondary"} {
		value, _ := block[leaf].(string)
		if value == "" {
			continue
		}
		address, err := netip.ParseAddr(value)
		if err != nil {
			return description, fmt.Errorf("%s: invalid %s %q: %w", Name, leaf, value, err)
		}
		if leaf == "dns-primary" {
			description.dnsPrimary = address
		} else {
			description.dnsSecondary = address
		}
	}
	if err := validateIPv4Description(description, path); err != nil {
		return ipv4PoolDescription{}, err
	}
	if description.gateway.Compare(description.start) >= 0 {
		if description.gateway.Compare(description.end) <= 0 {
			return description, fmt.Errorf("%s: gateway %s must not overlap pool range %s-%s",
				Name, description.gateway, description.start, description.end)
		}
	}
	sources, err := parsePoolSources(block, sourcesAt, builtin)
	if err != nil {
		return ipv4PoolDescription{}, err
	}
	description.sources = sources
	return description, nil
}

func parseIPv6Description(block map[string]any) (ipv6PoolDescription, error) {
	value, _ := block["block"].(string)
	if value == "" {
		return ipv6PoolDescription{}, fmt.Errorf("%s: ipv6-pd requires block", Name)
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return ipv6PoolDescription{}, fmt.Errorf("%s: invalid ipv6-pd block %q: %w", Name, value, err)
	}
	length := 56
	if raw, present := block["delegation-length"]; present {
		parsed, ok := configvalue.Int(raw)
		if !ok {
			return ipv6PoolDescription{}, fmt.Errorf("%s: ipv6-pd delegation-length is %q, want a prefix length", Name, raw)
		}
		length = int(parsed)
	}
	return describeIPv6Pool(prefix, length)
}

func describeIPv6Pool(block netip.Prefix, length int) (ipv6PoolDescription, error) {
	if !block.IsValid() {
		return ipv6PoolDescription{}, fmt.Errorf("%s: ipv6-pd block is invalid", Name)
	}
	if !block.Addr().Is6() {
		return ipv6PoolDescription{}, fmt.Errorf("%s: ipv6-pd block must be native IPv6", Name)
	}
	if block.Addr().Is4In6() {
		return ipv6PoolDescription{}, fmt.Errorf("%s: ipv6-pd block must be native IPv6", Name)
	}
	if length < 48 {
		return ipv6PoolDescription{}, fmt.Errorf("%s: delegation length %d below minimum 48", Name, length)
	}
	if length > 64 {
		return ipv6PoolDescription{}, fmt.Errorf("%s: delegation length %d above maximum 64", Name, length)
	}
	if length < block.Bits() {
		return ipv6PoolDescription{}, fmt.Errorf("%s: delegation length %d shorter than block /%d", Name, length, block.Bits())
	}
	spread := length - block.Bits()
	if spread > 24 {
		return ipv6PoolDescription{}, fmt.Errorf("%s: delegation spread %d bits (/%d from /%d) exceeds 24-bit maximum (16M prefixes)", Name, spread, length, block.Bits())
	}
	return ipv6PoolDescription{block: block.Masked(), delegLen: length, size: uint32(1) << spread}, nil
}

func (description ipv4PoolDescription) build() *ipv4Pool {
	pool := newIPv4Pool(description.gateway, description.start, description.end, description.dnsPrimary, description.dnsSecondary)
	pool.sources = description.sources
	return pool
}

func (description ipv6PoolDescription) build() *ipv6PrefixPool {
	return &ipv6PrefixPool{block: description.block, delegLen: description.delegLen,
		size: description.size, bitmap: make([]uint64, (description.size+63)/64)}
}

func (cfg poolConfiguration) build() poolConfigResult {
	var result poolConfigResult
	if cfg.defaultIPv4 != nil {
		result.defaultPool = cfg.defaultIPv4.build()
		result.found = true
	}
	for name := range cfg.namedIPv4 {
		if result.namedPools == nil {
			result.namedPools = make(map[string]*ipv4Pool)
		}
		result.namedPools[name] = cfg.namedIPv4[name].build()
		result.found = true
	}
	if cfg.defaultIPv6 != nil {
		result.defaultV6Pool = cfg.defaultIPv6.build()
		result.found = true
	}
	for name, description := range cfg.namedIPv6 {
		if result.namedV6Pools == nil {
			result.namedV6Pools = make(map[string]*ipv6PrefixPool)
		}
		result.namedV6Pools[name] = description.build()
		result.found = true
	}
	return result
}
