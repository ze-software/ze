// Design: docs/architecture/core-design.md -- redistribution config extraction
// Overview: loader.go -- config file loading

package config

import (
	"fmt"

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

			rules = append(rules, redistribute.ImportRule{
				Source:      source,
				Destination: dest.Key,
				Families:    families,
			})
		}
	}

	if len(rules) == 0 {
		return nil, nil
	}
	return rules, nil
}
