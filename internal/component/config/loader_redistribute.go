// Design: docs/architecture/core-design.md -- redistribution config extraction
// Overview: loader.go -- config file loading

package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/config/redistribute"
)

// ExtractRedistributeRules extracts redistribution import rules from a config tree.
// Reads the top-level "redistribute" container, iterates its "destination" list,
// and collects the "import" list under each destination protocol.
//
// Returns nil with no error when the redistribute container is absent or empty.
// Returns an error when a source name is not in the registry, or when an
// import names filters with no final (importChain).
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
			// It carries no chain, so every route moves; use the list form to filter.
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

			rule, err := importChain(entry.Value, source, dest.Key)
			if err != nil {
				return nil, err
			}
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return nil, nil
	}
	return rules, nil
}

// RefuseRedistributeRules refuses a config whose `redistribute` block cannot
// act: every refusal ExtractRedistributeRules makes, a destination no protocol
// declared (a typo such as `destination ospv3`), and a rule that names a
// source or a destination whose protocol has no config root in this config.
//
// It needs the whole tree, which only the config doors hold: LoadConfig (boot
// and reload) and VerifyPluginConfigMapTransition (`ze config validate`, the
// editor and API commit). The redistribute orchestrator sees its own root only,
// so it cannot tell that a `bgp` block is missing, and BGP does not start at
// all without one. Without this check `destination bgp` on a router with no
// `bgp` block stopped the daemon at startup with "missing required bgp { }
// block", and `import bgp` moved nothing and said nothing. `destination isis`
// or `destination ospf` with no block of its own booted and moved nothing.
//
// The root a source or a destination needs is declared statically, from the
// protocol's own init() (RouteSource.ConfigRoot, Destination.ConfigRoot), so
// this check names no protocol and needs no plugin running. A destination's
// consumer is no help here: IS-IS and OSPF register theirs only when their
// plugin starts, after this check has run. Every rule that fails is reported,
// so the operator fixes them in one pass.
func RefuseRedistributeRules(tree *Tree) error {
	rules, err := ExtractRedistributeRules(tree)
	if err != nil {
		return err
	}
	var refusals []error
	// A destination is refused once, however many imports sit under it.
	refusedDestinations := make(map[string]bool)
	for i := range rules {
		rule := &rules[i]
		if src, ok := redistribute.LookupSource(rule.Source); ok {
			if refusal := refuseAbsentRoot(tree, src.ConfigRoot, `"import `+rule.Source+`" under "destination `+rule.Destination+`"`, "import "+rule.Source); refusal != nil {
				refusals = append(refusals, refusal)
			}
		}
		if refusedDestinations[rule.Destination] {
			continue
		}
		dest, ok := redistribute.LookupDestination(rule.Destination)
		if !ok {
			refusedDestinations[rule.Destination] = true
			refusals = append(refusals, refuseUndeclaredDestination(rule.Destination))
			continue
		}
		if refusal := refuseAbsentRoot(tree, dest.ConfigRoot, `"destination `+rule.Destination+`"`, "destination "+rule.Destination); refusal != nil {
			refusedDestinations[rule.Destination] = true
			refusals = append(refusals, refusal)
		}
	}
	return errors.Join(refusals...)
}

// refuseUndeclaredDestination answers the refusal for a destination name no
// protocol declared (redistribute.RegisterDestination). Nothing in this build
// can receive routes under that name, so the rules under it would move nothing.
// The declared names come from the registry, so the list is what this build
// holds, never a copy of it.
func refuseUndeclaredDestination(name string) error {
	declared := "none in this build"
	if names := redistribute.DestinationNames(); len(names) > 0 {
		declared = strings.Join(names, ", ")
	}
	return fmt.Errorf("redistribute: %q names no protocol that receives redistributed routes, so the rules under it would move no route\n"+
		"  declared destinations: %s\n"+
		"  fix: rename %q to one of the declared destinations, or remove %q from the redistribute block",
		"destination "+name, declared, name, "destination "+name)
}

// refuseAbsentRoot answers the refusal for a statement whose protocol needs
// root when the config has none, and nil when root is empty or present.
// statement names what the operator wrote and where, already quoted; remove is
// the statement the fix tells them to delete.
func refuseAbsentRoot(tree *Tree, root, statement, remove string) error {
	if root == "" {
		return nil
	}
	if tree.GetContainer(root) != nil {
		return nil
	}
	return fmt.Errorf("redistribute: %s needs %s configured, and this config has no %s { } block\n"+
		"  without it no %s runs, so the rule would move no route\n"+
		"  fix: configure %s in a %s { } block, or remove %q from the redistribute block",
		statement, root, root, root, root, root, remove)
}

// importChain reads one import entry's chain: `filters [ ... ]` and `final`.
// An entry with neither imports every route of the source. Filters with no
// final are refused by name rather than given a default decision, the rule
// every configured policy follows. The names are checked, and the chain
// compiled, by the redistribute orchestrator against the policy root, which
// this package cannot import.
func importChain(entry *Tree, source, destination string) (redistribute.ImportRule, error) {
	rule := redistribute.ImportRule{Source: source, Destination: destination, Filters: entry.GetMultiValues("filters")}
	final, ok := entry.Get("final")
	if !ok {
		if len(rule.Filters) > 0 {
			return redistribute.ImportRule{}, fmt.Errorf("redistribute: import %q under destination %q names filters and no final; "+
				"every configured policy requires a final decision; write import %s { filters [ NAME ]; final accept; } or final reject",
				source, destination, source)
		}
		return rule, nil
	}
	switch final {
	case "accept", "reject":
		rule.Final = final
		return rule, nil
	}
	return redistribute.ImportRule{}, fmt.Errorf("redistribute: import %q under destination %q has an invalid final %q; write final accept or final reject",
		source, destination, final)
}
