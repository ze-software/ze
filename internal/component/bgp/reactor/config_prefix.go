// Design: docs/architecture/core-design.md — config tree parsing (PeersFromTree)
// Overview: config.go — parseFamiliesFromTree calls this once per family
// Related: peersettings.go — the per-family prefix maps this writes
// Related: session_prefix.go — reads the maps to enforce the limits

package reactor

import "fmt"

// parsePrefixLimitFromFamily extracts prefix maximum, warning, teardown,
// idle-timeout, reconnect, count and updated from a family entry's prefix block.
// RFC 4486 Section 4: Maximum Number of Prefixes Reached.
// Every non-disabled family MUST have a prefix maximum configured.
//
// Every leaf in the YANG `prefix` container is per family, so every value read
// here is stored under familyKey. The caller walks the families in sorted key
// order, so a value written to a peer-wide field would be overwritten by every
// later family, and the last family in key order would govern all of them.
// YANG defaults are materialized into every family entry
// (internal/component/config/schema_defaults.go), so a family that configures
// nothing still arrives carrying the default and would do the overwriting.
func parsePrefixLimitFromFamily(familyKey string, entryMap map[string]any, ps *PeerSettings) error {
	prefixMap, hasPrefixBlock := mapMap(entryMap, "prefix")
	if !hasPrefixBlock {
		return fmt.Errorf("family %s: prefix maximum is mandatory (add prefix { maximum N; })", familyKey)
	}

	maximum, ok := mapUint32(prefixMap, "maximum")
	if !ok || maximum == 0 {
		return fmt.Errorf("family %s: prefix maximum is mandatory and must be > 0", familyKey)
	}

	// Initialize maps lazily.
	if ps.PrefixMaximum == nil {
		ps.PrefixMaximum = make(map[string]uint32)
	}
	if ps.PrefixWarning == nil {
		ps.PrefixWarning = make(map[string]uint32)
	}

	ps.PrefixMaximum[familyKey] = maximum

	// Warning defaults to 90% of maximum.
	warning, hasWarning := mapUint32(prefixMap, "warning")
	if hasWarning {
		if warning >= maximum {
			return fmt.Errorf("family %s: prefix warning (%d) must be less than maximum (%d)", familyKey, warning, maximum)
		}
		ps.PrefixWarning[familyKey] = warning
	} else {
		ps.PrefixWarning[familyKey] = maximum * 9 / 10
	}

	if v, ok := mapString(prefixMap, "teardown"); ok {
		if ps.PrefixTeardown == nil {
			ps.PrefixTeardown = make(map[string]bool)
		}
		ps.PrefixTeardown[familyKey] = v != valFalse
	}

	// 0 when absent, which is also the YANG default.
	idle, hasIdle := mapUint16(prefixMap, "idle-timeout")
	if hasIdle {
		if ps.PrefixIdleTimeout == nil {
			ps.PrefixIdleTimeout = make(map[string]uint16)
		}
		ps.PrefixIdleTimeout[familyKey] = idle
	}

	if err := parsePrefixReconnect(familyKey, prefixMap, idle, ps); err != nil {
		return err
	}

	if err := parsePrefixCount(familyKey, prefixMap, ps); err != nil {
		return err
	}

	if v, ok := mapString(prefixMap, "updated"); ok {
		if ps.PrefixUpdated == nil {
			ps.PrefixUpdated = make(map[string]string)
		}
		ps.PrefixUpdated[familyKey] = v
	}

	return nil
}

// parsePrefixCount reads the per-family `count` leaf, which states which
// prefixes the count compared against `maximum` holds.
//
// An absent or empty value leaves the map alone, and PrefixCountFor
// (peersettings.go) reads that as `offered`. The YANG default is `offered` too,
// so a config that states nothing arrives here carrying it and gets the same
// answer either way.
func parsePrefixCount(familyKey string, prefixMap map[string]any, ps *PeerSettings) error {
	raw, ok := mapString(prefixMap, "count")
	if !ok || raw == "" {
		return nil
	}

	mode, valid := parsePrefixCountMode(raw)
	if !valid {
		return fmt.Errorf("family %s: prefix count %q is not one of offered, installed", familyKey, raw)
	}

	if ps.PrefixCount == nil {
		ps.PrefixCount = make(map[string]PrefixCountMode)
	}
	ps.PrefixCount[familyKey] = mode
	return nil
}

// parsePrefixReconnect reads the per-family `reconnect` leaf and rejects a
// value that disagrees with `idle-timeout` in the same block.
//
// The leaf carries no YANG default on purpose. An absent value means "derive it
// from idle-timeout" (PrefixReconnectFor), which is what every config written
// before the leaf existed says. A materialized default would make every family
// state an opinion and would make the two checks below fire on configs that
// disagree with nothing.
//
// The two rejections apply the exact-or-reject rule: a wait of zero seconds is
// not a timer, and a peer told to stay down has no use for a wait.
func parsePrefixReconnect(familyKey string, prefixMap map[string]any, idle uint16, ps *PeerSettings) error {
	raw, ok := mapString(prefixMap, "reconnect")
	if !ok || raw == "" {
		return nil
	}

	mode, valid := parsePrefixReconnectMode(raw)
	if !valid {
		return fmt.Errorf("family %s: prefix reconnect %q is not one of never, backoff, timer", familyKey, raw)
	}
	if mode == PrefixReconnectTimer && idle == 0 {
		return fmt.Errorf("family %s: prefix reconnect timer needs idle-timeout above 0", familyKey)
	}
	if mode != PrefixReconnectTimer && idle > 0 {
		return fmt.Errorf("family %s: prefix reconnect %s conflicts with idle-timeout %d; remove one of them", familyKey, mode, idle)
	}

	if ps.PrefixReconnect == nil {
		ps.PrefixReconnect = make(map[string]PrefixReconnectMode)
	}
	ps.PrefixReconnect[familyKey] = mode
	return nil
}
