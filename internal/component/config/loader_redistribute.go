// Design: docs/architecture/core-design.md -- redistribution config extraction
// Overview: loader.go -- config file loading

package config

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
)

// ExtractRedistributeRules extracts redistribution import rules from a config tree.
// Reads the top-level "redistribute" container and its "import" list.
// Each list entry's key is the source name; its "family" leaf-list is the
// optional family filter.
//
// Returns nil with no error when the redistribute container is absent or empty.
// Returns an error when a source name is not in the registry.
func ExtractRedistributeRules(tree *Tree) ([]redistribute.ImportRule, error) {
	redist := tree.GetContainer("redistribute")
	if redist == nil {
		return nil, nil
	}

	entries := redist.GetListOrdered("import")
	if len(entries) == 0 {
		return nil, nil
	}

	rules := make([]redistribute.ImportRule, 0, len(entries))
	for _, entry := range entries {
		source := entry.Key

		if _, ok := redistribute.LookupSource(source); !ok {
			return nil, fmt.Errorf("redistribute: unknown source %q", source)
		}

		families := entry.Value.GetMultiValues("family")

		rules = append(rules, redistribute.ImportRule{
			Source:   source,
			Families: families,
		})
	}

	return rules, nil
}
