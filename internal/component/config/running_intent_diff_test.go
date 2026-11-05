// Design: docs/architecture/config/syntax.md -- semantic running-intent differences.
package config

import (
	"strings"
	"testing"
)

func allInactiveDiffTree(disabled string, nested bool) *Tree {
	root, owner := NewTree(), NewTree()
	if nested {
		root.SetContainer("bgp", owner)
	} else {
		owner = root
	}
	owner.SetMemberStates("filters", []MemberState{{Value: disabled, Inactive: true}})
	return root
}

// TestInactiveIntentDiffUsesRealLeafOwnership exercises metadata-only, whole
// added/removed and type-replaced ancestors. Private intent always selects its
// real leaf path through Changed; it never invents an Added/Removed config path.
func TestInactiveIntentDiffUsesRealLeafOwnership(t *testing.T) {
	rootBefore := allInactiveDiffTree("automatic:a", false).ToRunningMap()
	rootAfter := allInactiveDiffTree("automatic:b", false).ToRunningMap()
	nestedBefore := allInactiveDiffTree("automatic:a", true).ToRunningMap()
	nestedAfter := allInactiveDiffTree("automatic:b", true).ToRunningMap()
	cases := []struct {
		name                                            string
		before, after                                   map[string]any
		path                                            string
		ordinaryAdded, ordinaryRemoved, ordinaryChanged string
	}{
		{"root metadata only", rootBefore, rootAfter, "filters/!inactive-members", "", "", ""},
		{"nested metadata only", nestedBefore, nestedAfter, "bgp/filters/!inactive-members", "", "", ""},
		{"whole added", map[string]any{}, nestedAfter, "bgp/filters/!inactive-members", "bgp", "", ""},
		{"whole removed", nestedBefore, map[string]any{}, "bgp/filters/!inactive-members", "", "bgp", ""},
		{"map replaced", nestedBefore, map[string]any{"bgp": "disabled"}, "bgp/filters/!inactive-members", "", "", "bgp"},
		{"map replacement", map[string]any{"bgp": "disabled"}, nestedAfter, "bgp/filters/!inactive-members", "", "", "bgp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diff := DiffMaps(tc.before, tc.after)
			if _, present := diff.Changed[tc.path]; !present {
				t.Errorf("private intent did not select real leaf owner %s: %v", tc.path, diff)
			}
			for _, paths := range []map[string]any{diff.Added, diff.Removed} {
				for path := range paths {
					if strings.HasSuffix(path, "/!inactive-members") || path == "!inactive-members" {
						t.Errorf("private intent became a public config addition/removal: %s", path)
					}
				}
			}
			if tc.ordinaryAdded != "" {
				if _, present := diff.Added[tc.ordinaryAdded]; !present {
					t.Error("whole ordinary added ancestor was lost")
				}
			}
			if tc.ordinaryRemoved != "" {
				if _, present := diff.Removed[tc.ordinaryRemoved]; !present {
					t.Error("whole ordinary removed ancestor was lost")
				}
			}
			if tc.ordinaryChanged != "" {
				if _, present := diff.Changed[tc.ordinaryChanged]; !present {
					t.Error("ordinary type-replaced ancestor was lost")
				}
			}
		})
	}
}

// TestInactiveMetadataSpellingDoesNotEraseLegitimateListEntries pins the value
// distinction: the same reserved spelling with a map value is a list entry,
// not metadata. Addition, removal and descendant changes retain that data.
func TestInactiveMetadataSpellingDoesNotEraseLegitimateListEntries(t *testing.T) {
	entry := func(value string) map[string]any {
		return map[string]any{"entries": map[string]any{"!inactive-members": map[string]any{"value": value}}}
	}
	empty := map[string]any{"entries": map[string]any{}}
	before, after := entry("old"), entry("new")
	if _, present := DiffMaps(empty, after).Added["entries/!inactive-members"]; !present {
		t.Error("same-name legitimate added entry was erased")
	}
	if _, present := DiffMaps(before, empty).Removed["entries/!inactive-members"]; !present {
		t.Error("same-name legitimate removed entry was erased")
	}
	if _, present := DiffMaps(before, after).Changed["entries/!inactive-members/value"]; !present {
		t.Error("same-name legitimate descendant change was erased")
	}
}
