// Design: docs/architecture/plugin/plugin-system.md -- registration fields, config needs
// Detail: docs/architecture/plugin/feature-gates.md -- naming a plugin the build left out
// Related: loader.go -- LoadConfig, the boot and reload door that calls refuseAbsentOptionalProviders
// Related: plugin_verify.go -- VerifyPluginConfigMapTransition, the validate and commit door that calls it

package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// ErrAbsentOptionalProvider is what every refusal of a config need unwraps to:
// the config sets a value that needs a plugin this build does not include.
var ErrAbsentOptionalProvider = errors.New("config: a setting needs a plugin this build does not include")

// ErrConfigNeedUnevaluated is returned when a registration declares a config
// need and the door has no config to judge it against. It keeps the check
// fail-closed: a declared need is never answered as met without a config.
var ErrConfigNeedUnevaluated = errors.New("config: config need cannot be evaluated")

// refuseAbsentOptionalProviders is the one check behind every door a config
// passes before it is applied: LoadConfig (boot, SIGHUP reload, editor-commit
// reload) and VerifyPluginConfigMapTransition (ze config validate, editor
// pre-commit, API commit).
//
// lowerConfig answers the config in Tree.ToPluginMap form. It is called only
// when some compiled-in registration declares a registry.ConfigNeed, so a build
// with no need pays for no lowering.
//
// A need holds when the leaf at its Path carries its Value, in any entry of a
// list the path crosses (registry.AnyEntry). A held need is met
// when any of its providers is registered in this build. The verdict is about
// the build only: whether the provider is configured is not judged here, so an
// any-of need is met by one provider compiled in and left unconfigured. Every
// held, unmet need is refused, once for each list entry that holds it, and the
// refusals are joined so the operator sees all of them at once.
func refuseAbsentOptionalProviders(lowerConfig func() map[string]any) error {
	var configTree map[string]any
	var refusals []error
	for _, reg := range registry.All() {
		for _, need := range reg.ConfigNeeds {
			if configTree == nil {
				configTree = lowerConfig()
			}
			if configTree == nil {
				return fmt.Errorf("%w: plugin %q declares a need on %q and the config is missing",
					ErrConfigNeedUnevaluated, reg.Name, strings.Join(need.Path, " "))
			}
			for _, setting := range heldSettings(configTree, need) {
				if refusal := refusalOfUnmetNeed(reg.Name, setting, need); refusal != nil {
					refusals = append(refusals, refusal)
				}
			}
		}
	}
	return errors.Join(refusals...)
}

// ConfigNeedHolds answers, for configTree, whether a registry.ConfigNeed
// holds, in the form registry.ResolveDependencies takes. It is the one
// evaluator behind both jobs a need does: refusing the config when the
// provider is absent (refuseAbsentOptionalProviders) and pulling the provider
// when it is present. configTree is a lowered config, Tree.ToMap or
// Tree.ToPluginMap form: both carry the same containers and leaves.
//
// A nil configTree is no config at all, so it answers nil, which
// ResolveDependencies refuses with registry.ErrConfigNeedsUnanswered. An empty
// tree is a config that sets nothing, and no need holds for it.
func ConfigNeedHolds(configTree map[string]any) registry.NeedHolds {
	// refuse-by-default: guards registry.ResolveDependencies -- a nil predicate is refused there, never read as "no need holds"
	if configTree == nil {
		return nil
	}
	return func(_ string, need registry.ConfigNeed) bool {
		return configHoldsNeed(configTree, need)
	}
}

// configHoldsNeed reports whether configTree holds need: the leaf at
// need.Path carries need.Value, for any entry of every list the path crosses.
func configHoldsNeed(configTree map[string]any, need registry.ConfigNeed) bool {
	return len(heldSettings(configTree, need)) > 0
}

// heldSettings answers each concrete setting at which configTree holds need:
// need.Path with every registry.AnyEntry replaced by the key of a list entry
// that carries need.Value, in key order. A path through no list answers at
// most one setting, need.Path itself. Nil means the need does not hold.
func heldSettings(configTree map[string]any, need registry.ConfigNeed) [][]string {
	setting := make([]string, 0, len(need.Path))
	return appendHeldSettings(nil, configTree, need.Path, setting, need.Value)
}

// appendHeldSettings walks path from node and appends to held a copy of
// setting, extended by the keywords and entry keys it crossed, for each leaf
// at the end of path that carries value. A map in the lowered tree is either a
// container, keyed by keyword, or a list, keyed by entry: the path says which,
// because a list's keyword is followed by registry.AnyEntry. A list lowers to a
// map of entry key to entry map (Tree.toMap), and an entry order key, when
// ToPluginMap emits one, sits beside the list in its parent, never inside it.
//
// The recursion depth is bounded by len(path), which a registration declares,
// and its breadth by the entries of the operator's own lists, so neither is
// chosen by a peer. Any node that is not what the path expects (absent, a leaf
// where a container is due, an entry that is not a map) holds nothing.
func appendHeldSettings(held [][]string, node any, path, setting []string, value string) [][]string {
	if len(path) == 0 {
		if leafCarries(node, value) {
			return append(held, slices.Clone(setting))
		}
		return held
	}
	container, isContainer := node.(map[string]any)
	// refuse-by-default: guards refuseAbsentOptionalProviders -- a node that is not a container holds no leaf, so the need does not hold
	if !isContainer {
		return held
	}
	keyword := path[0]
	if keyword != registry.AnyEntry {
		return appendHeldSettings(held, container[keyword], path[1:], append(setting, keyword), value)
	}
	for _, entryKey := range slices.Sorted(maps.Keys(container)) {
		held = appendHeldSettings(held, container[entryKey], path[1:], append(setting, entryKey), value)
	}
	return held
}

// leafCarries reports whether leaf, as ToPluginMap lowers it, carries value: a
// leaf whose string is value, or a leaf-list with value among its members. Any
// other node (absent, a container, a list) does not carry it.
func leafCarries(leaf any, value string) bool {
	if text, isLeaf := leaf.(string); isLeaf {
		return text == value
	}
	if members, isLeafList := leaf.([]string); isLeafList {
		return slices.Contains(members, value)
	}
	return false
}

// absentProvider is one provider of a held need that this build does not
// register, with the build tags that would carry it. gated false means no
// feature gate names it, so no build tag adds it and none is printed.
type absentProvider struct {
	name  string
	tags  []string
	gated bool
}

// unmetConfigNeedError carries the operator message for one held, unmet need.
// It unwraps to ErrAbsentOptionalProvider.
type unmetConfigNeedError struct {
	message string
}

func (e *unmetConfigNeedError) Error() string { return e.message }

func (e *unmetConfigNeedError) Unwrap() error { return ErrAbsentOptionalProvider }

// refusalOfUnmetNeed answers nil when any provider of need is registered in
// this build, and otherwise the refusal naming the setting, the providers and
// the build tags that would carry them. setting is the concrete path at which
// the config holds need (heldSettings), so a need through a list names the
// entry the operator wrote.
func refusalOfUnmetNeed(owner string, setting []string, need registry.ConfigNeed) error {
	absent := make([]absentProvider, 0, len(need.Providers))
	for _, name := range need.Providers {
		if registry.Lookup(name) != nil {
			return nil
		}
		tags, gated := registry.GatedPluginTags(name)
		absent = append(absent, absentProvider{name: name, tags: tags, gated: gated})
	}
	written := strings.Join(setting, " ") + " " + need.Value
	return &unmetConfigNeedError{message: unmetNeedMessage(owner, written, absent)}
}

// unmetNeedMessage writes the problem, why it matters, and the fix on a line
// of its own (ai/rules/user-facing-errors.md). setting is the leaf and value
// as the operator writes them, which is also what the fix tells them to remove.
func unmetNeedMessage(owner, setting string, absent []absentProvider) string {
	var b strings.Builder
	b.WriteString(`config: "`)
	b.WriteString(setting)
	b.WriteString(`" needs `)
	if len(absent) == 1 {
		b.WriteString("plugin ")
		b.WriteString(absent[0].name)
		b.WriteString(", and this build does not include it\n  ")
		b.WriteString(owner)
		b.WriteString(" cannot act on this setting without it, so ze refuses the config rather than run a setting that does nothing\n")
	} else {
		b.WriteString("one of the plugins ")
		writeAlternatives(&b, absent)
		b.WriteString(", and this build includes none of them\n  ")
		b.WriteString(owner)
		b.WriteString(" cannot act on this setting without one of them, so ze refuses the config rather than run a setting that does nothing\n")
	}
	for _, provider := range absent {
		if provider.gated {
			continue
		}
		b.WriteString("  plugin ")
		b.WriteString(provider.name)
		b.WriteString(" is not part of any ze build, so no build tag adds it\n")
	}
	b.WriteString("  fix: ")
	rebuilds := 0
	for _, provider := range absent {
		if !provider.gated {
			continue
		}
		if rebuilds == 0 {
			b.WriteString("rebuild ze with ")
		} else {
			b.WriteString(", or with ")
		}
		writeBuildTags(&b, provider.tags)
		rebuilds++
	}
	if rebuilds == 0 {
		b.WriteString(`remove "`)
	} else {
		b.WriteString(` added to its -tags list, or remove "`)
	}
	b.WriteString(setting)
	b.WriteString(`" from the config`)
	return b.String()
}

// writeAlternatives writes the provider names as "a or b" or "a, b or c".
func writeAlternatives(b *strings.Builder, absent []absentProvider) {
	for i, provider := range absent {
		if i > 0 {
			if i == len(absent)-1 {
				b.WriteString(" or ")
			} else {
				b.WriteString(", ")
			}
		}
		b.WriteString(provider.name)
	}
}

// writeBuildTags writes "build tag ze_bgp" for one tag and "build tags ze_bgp
// and ze_bmp" for a nested gate, whose tags must all be set together.
func writeBuildTags(b *strings.Builder, tags []string) {
	if len(tags) == 1 {
		b.WriteString("build tag ")
	} else {
		b.WriteString("build tags ")
	}
	b.WriteString(strings.Join(tags, " and "))
}
