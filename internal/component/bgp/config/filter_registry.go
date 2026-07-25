// Design: docs/architecture/config/syntax.md -- named filter registry
// Overview: peers.go -- peer configuration extraction

package bgpconfig

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/config"
)

// FilterEntry describes a single named filter instance.
type FilterEntry struct {
	Name string // filter instance name (list key)
	Type string // filter type name (the list name, e.g., "loop-detection")
}

// FilterRegistry collects all named filter instances from the policy section.
type FilterRegistry struct {
	entries map[string]FilterEntry
}

// BuildFilterRegistry scans the policy container tree for filter type lists
// and collects all named filter instances. It returns an error if the same
// instance name appears under two different filter types.
//
// policyTree is the parsed config tree for the policy container (may be nil).
// policySchema is the YANG schema for the policy container (provides list names).
func BuildFilterRegistry(policyTree *config.Tree, policySchema *config.ContainerNode) (*FilterRegistry, error) {
	reg := &FilterRegistry{entries: make(map[string]FilterEntry)}

	if policyTree == nil || policySchema == nil {
		return reg, nil
	}

	for _, childName := range policySchema.Children() {
		child := policySchema.Get(childName)
		if _, ok := child.(*config.ListNode); !ok {
			continue
		}

		listEntries := policyTree.GetList(childName)
		for name := range listEntries {
			if existing, dup := reg.entries[name]; dup {
				return nil, fmt.Errorf(
					"duplicate filter name %q: defined in both %q and %q",
					name, existing.Type, childName,
				)
			}
			reg.entries[name] = FilterEntry{Name: name, Type: childName}
		}
	}

	return reg, nil
}

// Lookup returns the filter entry for the given name.
func (r *FilterRegistry) Lookup(name string) (FilterEntry, bool) {
	e, ok := r.entries[name]
	return e, ok
}

// Names returns all filter instance names in sorted order.
func (r *FilterRegistry) Names() []string {
	names := make([]string, 0, len(r.entries))
	for n := range r.entries {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Len returns the number of registered filter instances.
func (r *FilterRegistry) Len() int {
	return len(r.entries)
}

// ValidateFilterNames checks that policy filter names in the chain exist in the
// registry. Deactivated refs are validated the same as active ones (deactivation
// is a structural bool on FilterRef, not part of the name).
// Skips names containing ":" (external plugin filters validated at runtime, not
// parse time -- plugins register at stage 1, after config parsing).
func (r *FilterRegistry) ValidateFilterNames(refs []filterapi.FilterRef, context string) error {
	for _, ref := range refs {
		if strings.Contains(ref.Name, ":") {
			continue // external plugin filter (plugin:filter), validated at runtime
		}
		if _, ok := r.entries[ref.Name]; !ok {
			return fmt.Errorf("%s: unknown filter %q", context, ref.Name)
		}
	}
	return nil
}
