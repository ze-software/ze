// Design: docs/architecture/core-design.md -- bgp/policy/family-filter config parsing
// Related: handler.go -- family-filter runtime handler and export-chain guard

package filter_family

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/family"
)

// Action values for a family-filter instance (YANG enumeration).
const (
	actionRemove   = "remove"
	actionTearDown = "tear-down"
)

// familyFilter is a parsed bgp/policy/family-filter instance.
type familyFilter struct {
	name   string
	family family.Family
	action string // actionRemove | actionTearDown
}

// parseFamilyFilters reads bgp/policy/family-filter instances and enforces the
// import-only constraint on tear-down (AC-7): a tear-down instance referenced in
// any export chain is a configuration error (rejected at config-apply time).
func parseFamilyFilters(bgpCfg map[string]any) (map[string]*familyFilter, error) {
	instances := make(map[string]*familyFilter)

	policy, _ := bgpCfg["policy"].(map[string]any)
	ffBlock, _ := policy["family-filter"].(map[string]any)
	for name, v := range ffBlock {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		famStr, _ := m["family"].(string)
		if famStr == "" {
			return nil, fmt.Errorf("family-filter %q: missing family", name)
		}
		fam, ok := family.LookupFamily(famStr)
		if !ok {
			return nil, fmt.Errorf("family-filter %q: unknown address family %q", name, famStr)
		}
		actStr, _ := m["action"].(string)
		switch actStr {
		case actionRemove, actionTearDown:
		default:
			return nil, fmt.Errorf("family-filter %q: invalid action %q (want remove or tear-down)", name, actStr)
		}
		instances[name] = &familyFilter{name: name, family: fam, action: actStr}
	}

	if err := validateNoTearDownInExport(bgpCfg, instances); err != nil {
		return nil, err
	}
	return instances, nil
}

// validateNoTearDownInExport reports an error if any export chain (global, group,
// or peer level) references a tear-down family-filter instance. tear-down is
// import-only: tearing down a session on what we are about to SEND is illogical.
func validateNoTearDownInExport(bgpCfg map[string]any, instances map[string]*familyFilter) error {
	check := func(refs []string) error {
		for _, ref := range refs {
			name := exportRefInstanceName(ref)
			if name == "" {
				continue
			}
			if inst, ok := instances[name]; ok && inst.action == actionTearDown {
				return fmt.Errorf("family-filter %q: action tear-down cannot be used in an export chain (import only)", name)
			}
		}
		return nil
	}

	if err := check(exportChain(bgpCfg)); err != nil {
		return err
	}
	// This visitor validates and stores nothing, so it keys no template and does
	// not read the origin. A dynamic group's template visit still carries the
	// group's map, so the group's export chain is checked on that visit exactly
	// as it is for a group that lists peers.
	var visitErr error
	configjson.ForEachPeer(bgpCfg, func(_ string, peerMap, groupMap map[string]any, _ configjson.PeerOrigin) {
		if visitErr != nil {
			return
		}
		if groupMap != nil {
			if err := check(exportChain(groupMap)); err != nil {
				visitErr = err
				return
			}
		}
		if peerMap != nil {
			if err := check(exportChain(peerMap)); err != nil {
				visitErr = err
			}
		}
	})
	return visitErr
}

// exportChain returns the export filter chain leaf-list from a config level
// (bgp / group / peer), or nil if absent.
func exportChain(m map[string]any) []string {
	filterBlock, ok := m["filter"].(map[string]any)
	if !ok {
		return nil
	}
	return toStringList(filterBlock["export"])
}

// exportRefInstanceName returns the family-filter instance name a chain ref points
// to, or "" if the ref targets another plugin. Accepts bgp-filter-family:NAME,
// family-filter:NAME, and a bare NAME. Refs arrive clean via ToMap (per-member
// deactivation is out-of-band), so there is no inactive: prefix to strip.
func exportRefInstanceName(ref string) string {
	if before, after, found := strings.Cut(ref, ":"); found {
		if before == "bgp-filter-family" || before == "family-filter" {
			return after
		}
		return "" // a different plugin's filter
	}
	return ref // bare name; the caller checks it against our instance set
}

// toStringList normalises a leaf-list value to []string. The config loader may
// pass []any (JSON round-trip), []string (multi-value), or a bare string.
func toStringList(v any) []string {
	switch s := v.(type) {
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	case []string:
		return s
	case string:
		if s == "" {
			return nil
		}
		return []string{s}
	}
	return nil
}
