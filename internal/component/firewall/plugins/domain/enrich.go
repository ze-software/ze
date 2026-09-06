// Design: docs/architecture/firewall/firewall-domain-group.md -- the DNS name beside the address
// Related: cache.go -- store.provenance, the address-to-name map this answers with
//
// An operator reading a ruleset must be able to see which name put an address
// there. The address alone does not say it, and a group holding several names
// makes the question real rather than rhetorical.
//
// The name does NOT reach the output through a field on firewall.SetElement.
// That type is shared by every table owner -- copp, policy-routes, flowspec,
// vrrp, firewall-irr -- and a provenance field would be a per-feature edit to a
// shared field list, carried unused by all of them, which is the shape
// ai/rules/principles.md refuses. The show enricher registry is the sanctioned
// route: this plugin declares an enricher at registration, the engine registers
// a proxy for it (internal/component/plugin/server/enricher.go), and the
// firewall's show handler calls show.Enrich. Remove the plugin and the column
// disappears with it, which is the property the registry exists for.

package domain

import "maps"

// nothingToAdd is what this enricher answers when it has no name to attach.
// It is an EMPTY map rather than nil: registerProxyEnrichers merges the answer
// with maps.Copy, so an empty map adds nothing, and returning a nil map beside
// a nil error would leave a caller unable to tell "no contribution" from "the
// call failed and invented a zero".
var nothingToAdd = map[string]any{}

const (
	// enrichCommand is the show command whose output this plugin adds to. It
	// MUST match the string handleShowFirewallRuleset passes to show.Enrich
	// (internal/plugins/firewall/nft/cmd_show.go); the two are checked against
	// each other by TestEnrichCommandMatchesTheShowHandler.
	enrichCommand = "show firewall ruleset"
	// enrichKey names this plugin's enricher within that command. Registration
	// refuses a duplicate, so the key is what keeps two plugins from silently
	// overwriting each other's contribution.
	enrichKey = "domain-group"

	// keySets is the response key carrying the ruleset's sets, and
	// keySourceName is the field this enricher adds to an element of one.
	keySets       = "sets"
	keyElements   = "elements"
	keyValue      = "value"
	keySourceName = "source-name"
	keyName       = "name"
)

// enrichShow answers a show-enrichment request with the ruleset's sets, each
// domain-group element carrying the DNS name that supplied its address.
//
// It returns the whole `sets` value rather than a side map, because
// registerProxyEnrichers merges the answer with maps.Copy at the TOP level: a
// side map would land beside the addresses and leave the reader joining two
// structures by hand.
//
// An element this plugin did not supply is copied through untouched, so the
// sets of every other table owner render exactly as they did.
func (plug *domainPlugin) enrichShow(command, key, _ string, base map[string]any) (map[string]any, error) {
	if command != enrichCommand || key != enrichKey {
		return nothingToAdd, nil
	}
	sets, ok := base[keySets].([]any)
	if !ok || len(sets) == 0 {
		return nothingToAdd, nil
	}

	names := plug.provenanceBySet()
	if len(names) == 0 {
		return nothingToAdd, nil
	}

	out := make([]any, 0, len(sets))
	touched := false
	for _, raw := range sets {
		set, ok := raw.(map[string]any)
		if !ok {
			out = append(out, raw)
			continue
		}
		setName, _ := set[keyName].(string)
		byAddress, mine := names[setName]
		if !mine {
			out = append(out, raw)
			continue
		}
		enriched, changed := enrichSet(set, byAddress)
		touched = touched || changed
		out = append(out, enriched)
	}
	if !touched {
		return nothingToAdd, nil
	}
	return map[string]any{keySets: out}, nil
}

// enrichSet copies one set, attaching the supplying name to each element whose
// address this plugin resolved. The copy is shallow apart from the elements,
// which are the only part that changes.
func enrichSet(set map[string]any, byAddress map[string]string) (map[string]any, bool) {
	elements, ok := set[keyElements].([]any)
	if !ok || len(elements) == 0 {
		return set, false
	}

	out := make([]any, 0, len(elements))
	changed := false
	for _, raw := range elements {
		element, ok := raw.(map[string]any)
		if !ok {
			out = append(out, raw)
			continue
		}
		value, _ := element[keyValue].(string)
		name, found := byAddress[value]
		if !found {
			out = append(out, raw)
			continue
		}
		copied := make(map[string]any, len(element)+1)
		maps.Copy(copied, element)
		copied[keySourceName] = name
		out = append(out, copied)
		changed = true
	}
	if !changed {
		return set, false
	}

	copied := make(map[string]any, len(set))
	maps.Copy(copied, set)
	copied[keyElements] = out
	return copied, true
}

// provenanceBySet maps each domain-group set name to its address-to-name map.
// Both families of a group share one provenance map, because a name supplies
// the addresses of both and the element's own family decides which set it is in.
func (plug *domainPlugin) provenanceBySet() map[string]map[string]string {
	plug.mu.RLock()
	cfg := plug.config
	plug.mu.RUnlock()
	if cfg == nil {
		return nil
	}

	out := make(map[string]map[string]string, len(cfg.groups)*2)
	for _, g := range cfg.groups {
		byAddress := plug.cache.provenance(g.Name, g.Names)
		if len(byAddress) == 0 {
			continue
		}
		v4Name, v6Name := setNames(g.Name)
		out[v4Name] = byAddress
		out[v6Name] = byAddress
	}
	return out
}
