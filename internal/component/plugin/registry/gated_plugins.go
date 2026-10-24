// Design: docs/architecture/plugin/feature-gates.md -- naming a plugin this build was compiled without
// Related: gated_plugins_gen.go -- the generated table this file reads

package registry

import "slices"

// GatedPluginTags answers the build tags that must all be set for a binary to
// carry the plugin called name, and whether a feature gate covers that name at
// all.
//
// The tags come in the order feature-gates.txt nests them: the enclosing gates
// sorted, the plugin's own gate last, so bgp-bmp answers ze_bgp then ze_bmp.
// The table is generated from the manifest and compiled into every build, which
// is what lets a binary built without a plugin still name the tag that would
// carry it. The slice is the caller's own copy.
//
// gated false means no gate covers the name, and the answer carries no tag:
// either the plugin is ungated, so every build carries it and Lookup finds it,
// or no build of this tree registers the name at all (an external plugin, a
// typo). A caller MUST tell those apart with Lookup and MUST NOT invent a tag
// for either.
func GatedPluginTags(name string) (tags []string, gated bool) {
	tags, gated = gatedPluginTags[name]

	return slices.Clone(tags), gated
}

// GatedConfigRootRequirements answers, for a config root some gated plugin
// owns (Registration.ConfigRoots), each set of build tags that would carry an
// owner of that root, and whether any gated plugin owns it at all.
//
// A root several plugins own answers one set per distinct requirement, keeping
// only the smallest: `bgp` is owned by bgp ({ze_bgp}) and by bgp-bmp ({ze_bgp,
// ze_bmp}), and ze_bgp alone already carries the root, so the answer is
// [[ze_bgp]]. The sets are alternatives: any one of them carries the root.
// They come sorted by their first tag, so the answer is stable.
//
// gated false means no gated plugin owns the root: the keyword is unknown to
// every build of this tree (a typo), or an ungated owner provides it. A caller
// MUST NOT name a tag then.
func GatedConfigRootRequirements(root string) (requirements [][]string, gated bool) {
	owners, gated := gatedConfigRoots[root]
	if !gated {
		return nil, false
	}
	for _, owner := range owners {
		requirements = appendSmallestRequirement(requirements, gatedPluginTags[owner])
	}
	slices.SortFunc(requirements, slices.Compare[[]string])

	return requirements, true
}

// appendSmallestRequirement adds tags to requirements unless a set already
// held is a subset of it, and drops every held set tags is a subset of. The
// result is the minimal alternatives, each the caller's own copy.
func appendSmallestRequirement(requirements [][]string, tags []string) [][]string {
	for _, held := range requirements {
		if isSubset(held, tags) {
			return requirements
		}
	}
	requirements = slices.DeleteFunc(requirements, func(held []string) bool { return isSubset(tags, held) })

	return append(requirements, slices.Clone(tags))
}

// isSubset reports whether every tag in small is in large.
func isSubset(small, large []string) bool {
	for _, tag := range small {
		if !slices.Contains(large, tag) {
			return false
		}
	}

	return true
}
