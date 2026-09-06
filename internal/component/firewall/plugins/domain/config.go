// Design: docs/architecture/firewall/firewall-domain-group.md -- domain-group config parsing

package domain

import (
	"encoding/json"
	"sort"
	"strconv"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// ttlFloorDefault matches the YANG default of the ttl-floor leaf. A parse
	// that produced 0 here would schedule a refresh at once and spin against a
	// name answering TTL=0, so the default is applied by this package as well
	// as by the schema (schedule.go states why the floor exists).
	ttlFloorDefault = 60
	// ttlFloorMax matches the YANG range. A value past it is a config Ze never
	// accepts, so parsing clamps rather than carrying it into the schedule.
	ttlFloorMax = 86400
)

// group is one configured domain group: a set name, the DNS names whose
// addresses fill it, and how long Ze waits at least before asking again.
type group struct {
	Name     string
	Names    []string
	TTLFloor uint32
}

// termRef is a rule that names a domain group, and the table the rule sits in.
// The set this plugin registers must land in THAT table: ApplyAll merges by
// owner, and a set registered in the wrong table leaves the rule naming a set
// no owner supplied, which holds the operator's whole table back
// (dropTablesMissingAProvidedSet, internal/component/firewall/registry.go).
type termRef struct {
	Group     string
	TableName string
}

type domainConfig struct {
	groups []group
	refs   []termRef
}

// groupByName returns the configured group of that name. The second return
// separates "no such group" from "a group with no names", which take different
// answers at verify and in the update command.
func (c *domainConfig) groupByName(name string) (group, bool) {
	if c == nil {
		return group{}, false
	}
	for _, g := range c.groups {
		if g.Name == name {
			return g, true
		}
	}
	return group{}, false
}

// groupNames returns every configured group name, sorted, so an error message
// and a show listing are stable across runs. Go randomizes map iteration and
// the config arrives as a map.
func (c *domainConfig) groupNames() []string {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.groups))
	for _, g := range c.groups {
		names = append(names, g.Name)
	}
	sort.Strings(names)
	return names
}

// referencedGroups returns the groups a rule actually names, which is the set
// verify refuses on. A group defined and never referenced enforces nothing, so
// refusing a commit for it would refuse a config that harms nobody.
func (c *domainConfig) referencedGroups() []string {
	if c == nil {
		return nil
	}
	seen := make(map[string]bool, len(c.refs))
	names := make([]string, 0, len(c.refs))
	for _, r := range c.refs {
		if seen[r.Group] {
			continue
		}
		seen[r.Group] = true
		names = append(names, r.Group)
	}
	sort.Strings(names)
	return names
}

func parseDomainConfig(sections []sdk.ConfigSection) *domainConfig {
	cfg := &domainConfig{}
	for _, s := range sections {
		if s.Root != configRoot {
			continue
		}
		var root map[string]any
		if json.Unmarshal([]byte(s.Data), &root) != nil {
			continue
		}
		fw, ok := root[configRoot].(map[string]any)
		if !ok {
			continue
		}
		cfg.groups = parseGroups(fw)
		cfg.refs = extractRefsFromConfig(root)
	}
	return cfg
}

func parseGroups(fw map[string]any) []group {
	groupMap, ok := fw["domain-group"].(map[string]any)
	if !ok {
		return nil
	}
	groups := make([]group, 0, len(groupMap))
	for name, v := range groupMap {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		groups = append(groups, group{
			Name:     name,
			Names:    parseDomainNames(entry["domain-names"]),
			TTLFloor: parseTTLFloor(entry["ttl-floor"]),
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

// parseDomainNames reads the domain-names leaf-list. A delivered leaf-list can
// arrive as a list of values or, when it holds one value, as that value alone,
// so both shapes are read. Order is normalized: the schedule and the change log
// are keyed by name, and a reordered config must not read as a change.
func parseDomainNames(v any) []string {
	var names []string
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				names = append(names, s)
			}
		}
	case string:
		if t != "" {
			names = append(names, t)
		}
	}
	sort.Strings(names)
	return names
}

// parseTTLFloor reads the ttl-floor leaf. Every delivered config value arrives
// as a JSON string or a JSON number depending on the path it took, so both are
// read (ai/rules/config.md). An absent or unreadable value takes the YANG
// default rather than 0, which would ask the schedule for an immediate refresh.
func parseTTLFloor(v any) uint32 {
	if v == nil {
		return ttlFloorDefault
	}
	var n uint64
	switch t := v.(type) {
	case float64:
		if t < 0 {
			return ttlFloorDefault
		}
		n = uint64(t)
	case string:
		parsed, err := strconv.ParseUint(t, 10, 32)
		if err != nil {
			return ttlFloorDefault
		}
		n = parsed
	default:
		return ttlFloorDefault
	}
	if n == 0 {
		return ttlFloorDefault
	}
	if n > ttlFloorMax {
		return ttlFloorMax
	}
	return uint32(n)
}

// extractRefsFromConfig finds every rule naming a domain group, with the table
// the rule sits in. It walks the same firewall/table/chain/term/from shape the
// firewall's own parser walks, because the plugin receives the config tree
// rather than the parsed tables.
func extractRefsFromConfig(root map[string]any) []termRef {
	fw, ok := root[configRoot].(map[string]any)
	if !ok {
		return nil
	}
	tableMap, ok := fw["table"].(map[string]any)
	if !ok {
		return nil
	}
	type refKey struct{ group, table string }
	seen := make(map[refKey]bool)
	var refs []termRef
	for tblName, tblVal := range tableMap {
		tblMap, ok := tblVal.(map[string]any)
		if !ok {
			continue
		}
		fullName := tableNamePrefix + tblName
		chainMap, ok := tblMap["chain"].(map[string]any)
		if !ok {
			continue
		}
		for _, chainVal := range chainMap {
			chainData, ok := chainVal.(map[string]any)
			if !ok {
				continue
			}
			termMap, ok := chainData["term"].(map[string]any)
			if !ok {
				continue
			}
			for _, termVal := range termMap {
				termData, ok := termVal.(map[string]any)
				if !ok {
					continue
				}
				from, ok := termData["from"].(map[string]any)
				if !ok {
					continue
				}
				for _, leaf := range [2]string{"source-domain-group", "destination-domain-group"} {
					name, ok := from[leaf].(string)
					if !ok || name == "" {
						continue
					}
					k := refKey{group: name, table: fullName}
					if seen[k] {
						continue
					}
					seen[k] = true
					refs = append(refs, termRef{Group: name, TableName: fullName})
				}
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].TableName != refs[j].TableName {
			return refs[i].TableName < refs[j].TableName
		}
		return refs[i].Group < refs[j].Group
	})
	return refs
}
