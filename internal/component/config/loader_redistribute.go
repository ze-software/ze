// Design: docs/architecture/core-design.md -- redistribution config extraction
// Overview: loader.go -- config file loading

package config

import (
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
)

// ExtractRedistributeRules extracts redistribution import rules from a config tree.
// Reads the top-level "redistribute" container, iterates its "destination" list,
// and collects the "import" list under each destination protocol.
//
// Returns nil with no error when the redistribute container is absent or empty.
// Returns an error when a source name is not in the registry or when a family
// name is not registered (exact-or-reject: unknown families MUST NOT silently
// be dropped or translated).
func ExtractRedistributeRules(tree *Tree) ([]redistribute.ImportRule, error) {
	redist := tree.GetContainer("redistribute")
	if redist == nil {
		return nil, nil
	}

	destinations := redist.GetListOrdered("destination")
	if len(destinations) == 0 {
		return nil, nil
	}

	var rules []redistribute.ImportRule
	for _, dest := range destinations {
		entries := dest.Value.GetListOrdered("import")
		if len(entries) == 0 {
			// Scalar fallback: "import ipsec;" stores as key-value, not list entry.
			// Accepts all families (no family filter); use list form to restrict.
			if scalar, ok := dest.Value.Get("import"); ok && scalar != "" {
				if _, ok := redistribute.LookupSource(scalar); !ok {
					return nil, fmt.Errorf("redistribute: unknown source %q under destination %q", scalar, dest.Key)
				}
				rules = append(rules, redistribute.ImportRule{Source: scalar, Destination: dest.Key})
				continue
			}
			// A destination that imports nothing states an intention the
			// daemon cannot act on. Producing no rule for it looks exactly
			// like producing one that rejects everything, so name it instead
			// (ai/rules/principles.md).
			return nil, fmt.Errorf("redistribute: destination %q imports nothing; name at least one source or remove the destination", dest.Key)
		}
		for _, entry := range entries {
			source := entry.Key

			if _, ok := redistribute.LookupSource(source); !ok {
				return nil, fmt.Errorf("redistribute: unknown source %q under destination %q", source, dest.Key)
			}

			names := entry.Value.GetMultiValues("family")
			var families []family.Family
			if len(names) > 0 {
				families = make([]family.Family, 0, len(names))
				for _, name := range names {
					fam, ok := family.LookupFamily(name)
					if !ok {
						return nil, fmt.Errorf("redistribute: unknown family %q under source %q", name, source)
					}
					families = append(families, fam)
				}
			}

			tag, matchTag, err := importTag(entry.Value, source)
			if err != nil {
				return nil, err
			}

			rules = append(rules, redistribute.ImportRule{
				Source:      source,
				Destination: dest.Key,
				Families:    families,
				Tag:         tag,
				MatchTag:    matchTag,
			})
		}
	}

	if len(rules) == 0 {
		return nil, nil
	}
	return rules, nil
}

// importTag reads the optional `tag` leaf of one import entry. It reports the value
// and whether the leaf was written at all, because zero IS a selectable tag: a route
// with no `tag` carries zero, so `import <source> { tag 0 }` names the untagged routes
// and an entry with no `tag` leaf names every route in the source. Collapsing the two
// into one uint32 would make the first rule import everything (ai/rules/principles.md).
//
// An out-of-range value is refused by name rather than clamped: a tag the operator
// cannot express is a config error, and a clamped one silently imports the wrong set.
func importTag(entry *Tree, source string) (tag uint32, matchTag bool, err error) {
	raw, ok := entry.Get("tag")
	if !ok {
		return 0, false, nil
	}
	value, parseErr := strconv.ParseUint(raw, 10, 32)
	if parseErr != nil {
		return 0, false, fmt.Errorf("redistribute: import %q has an invalid tag %q: a route tag is a number from 0 to 4294967295", source, raw)
	}
	return uint32(value), true, nil
}
