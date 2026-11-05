// Design: docs/architecture/config/syntax.md -- internal inactive intent, public active data.
// Related: tree.go -- ToRunningMap; plugin_map.go -- reconstruction; diff.go -- ownership.
package config

import (
	"fmt"
	"maps"
	"slices"
)

const inactiveMembersKey = "!inactive-members"

// IsInactiveMemberMetadata distinguishes private array metadata from a real
// map-valued keyed-list entry with the same arbitrary name. All running-map
// readers MUST use this value-aware distinction, never the spelling alone.
func IsInactiveMemberMetadata(name string, value any) bool {
	if name != inactiveMembersKey {
		return false
	}
	_, metadata := value.([]any)
	return metadata
}

// runningMembersLocked snapshots inactive leaf-list intent. The caller MUST
// hold t.mu; every returned slice/map belongs to the immutable running map.
func (t *Tree) runningMembersLocked() []any {
	var records []any
	for _, leaf := range slices.Sorted(maps.Keys(t.inactiveMembers)) {
		pending := maps.Clone(t.inactiveMembers[leaf])
		if len(pending) == 0 {
			continue
		}
		inactive := make([]string, 0, len(pending))
		for _, member := range t.multiValues[leaf] {
			if pending[member] {
				inactive = append(inactive, member)
				delete(pending, member)
			}
		}
		records = append(records, map[string]any{
			"leaf": leaf, "members": slices.Clone(t.multiValues[leaf]), "inactive": inactive,
		})
	}
	return records
}

type inactiveMemberIntent struct {
	leaf    string
	members []MemberState
	record  map[string]any
}

// runningMemberIntents decodes only node metadata. A map-valued same-name entry
// is ordinary config. Missing metadata guards restoreRunningMembers and the
// semantic diff: neither may invent inactive intent from an absent record.
func runningMemberIntents(node map[string]any) ([]inactiveMemberIntent, error) {
	raw, present := node[inactiveMembersKey]
	if !present {
		return nil, nil
	}
	if _, entry := raw.(map[string]any); entry {
		return nil, nil
	}
	records, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected metadata array, got %T", inactiveMembersKey, raw)
	}
	intents := make([]inactiveMemberIntent, 0, len(records))
	seen := make(map[string]bool, len(records))
	for index, rawRecord := range records {
		record, ok := rawRecord.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d]: expected leaf record, got %T", inactiveMembersKey, index, rawRecord)
		}
		leaf, ok := record["leaf"].(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d]: leaf is not a string", inactiveMembersKey, index)
		}
		if leaf == "" {
			return nil, fmt.Errorf("%s[%d]: leaf is empty", inactiveMembersKey, index)
		}
		if seen[leaf] {
			return nil, fmt.Errorf("%s: duplicate leaf %q", inactiveMembersKey, leaf)
		}
		seen[leaf] = true
		members, err := runningMemberStrings(record["members"])
		if err != nil {
			return nil, fmt.Errorf("%s %s members: %w", inactiveMembersKey, leaf, err)
		}
		inactive, err := runningMemberStrings(record["inactive"])
		if err != nil {
			return nil, fmt.Errorf("%s %s inactive: %w", inactiveMembersKey, leaf, err)
		}
		if len(inactive) == 0 {
			return nil, fmt.Errorf("%s %s: no inactive member", inactiveMembersKey, leaf)
		}
		present := make(map[string]bool, len(members))
		for _, member := range members {
			present[member] = true
		}
		marks := make(map[string]bool, len(inactive))
		for _, member := range inactive {
			if !present[member] {
				return nil, fmt.Errorf("%s %s: inactive member %q is absent", inactiveMembersKey, leaf, member)
			}
			if marks[member] {
				return nil, fmt.Errorf("%s %s: duplicate inactive member %q", inactiveMembersKey, leaf, member)
			}
			marks[member] = true
		}
		states := make([]MemberState, len(members))
		for i, member := range members {
			states[i] = MemberState{Value: member, Inactive: marks[member]}
		}
		intents = append(intents, inactiveMemberIntent{leaf: leaf, members: states, record: record})
	}
	return intents, nil
}

func runningMemberStrings(raw any) ([]string, error) {
	switch members := raw.(type) {
	case []string:
		return members, nil
	case []any:
		values := make([]string, len(members))
		for i, member := range members {
			text, ok := member.(string)
			if !ok {
				return nil, fmt.Errorf("member %d is %T, not a string", i, member)
			}
			values[i] = text
		}
		return values, nil
	default:
		return nil, fmt.Errorf("expected member array, got %T", raw)
	}
}

// restoreRunningMembers MUST run after ordinary keys were reconstructed, since
// AppendValue would otherwise overwrite the restored member state. It refuses
// disagreement rather than letting private intent replace active configuration.
func restoreRunningMembers(source map[string]any, target *Tree) error {
	intents, err := runningMemberIntents(source)
	if err != nil {
		return err
	}
	for _, intent := range intents {
		var active []string
		for _, member := range intent.members {
			if !member.Inactive {
				active = append(active, member.Value)
			}
		}
		raw, present := source[intent.leaf]
		if len(active) == 0 {
			if present {
				return fmt.Errorf("%s %s: inactive intent disagrees with active value", inactiveMembersKey, intent.leaf)
			}
		} else {
			if !present {
				return fmt.Errorf("%s %s: active members are missing", inactiveMembersKey, intent.leaf)
			}
			var actual []string
			if single, ok := raw.(string); ok {
				actual = []string{single}
			} else {
				actual, err = runningMemberStrings(raw)
				if err != nil {
					return fmt.Errorf("%s %s active value: %w", inactiveMembersKey, intent.leaf, err)
				}
			}
			if !slices.Equal(active, actual) {
				return fmt.Errorf("%s %s: inactive intent disagrees with active members", inactiveMembersKey, intent.leaf)
			}
		}
		target.SetMemberStates(intent.leaf, intent.members)
	}
	return nil
}

// ActiveConfigMap removes private running intent before a map reaches a public
// config consumer. The input MUST remain immutable; unchanged branches are
// shared, while only paths containing private metadata are copied.
func ActiveConfigMap(source map[string]any) map[string]any {
	active, _ := activeConfigMap(source)
	return active
}

// ActiveConfigValue is ActiveConfigMap for an added/removed subtree or either
// side of a public DiffPair. The input MUST remain immutable.
func ActiveConfigValue(value any) any {
	active, _ := activeConfigValue(value)
	return active
}

// activeConfigMap walks the finite parsed config; its depth is schema-bounded.
func activeConfigMap(source map[string]any) (map[string]any, bool) {
	result := source
	changed := false
	for name, value := range source {
		if IsInactiveMemberMetadata(name, value) {
			if !changed {
				result = maps.Clone(source)
				changed = true
			}
			delete(result, name)
			continue
		}
		active, differs := activeConfigValue(value)
		if differs {
			if !changed {
				result = maps.Clone(source)
				changed = true
			}
			result[name] = active
		}
	}
	return result, changed
}

func activeConfigValue(value any) (any, bool) {
	switch value := value.(type) {
	case map[string]any:
		return activeConfigMap(value)
	case []any:
		result := value
		changed := false
		for i, member := range value {
			active, differs := activeConfigValue(member)
			if differs {
				if !changed {
					result = slices.Clone(value)
					changed = true
				}
				result[i] = active
			}
		}
		return result, changed
	default:
		// Scalars and string arrays cannot contain node metadata.
		return value, false
	}
}
