// Design: docs/architecture/core-design.md -- the policy filter chain and its final
// Overview: registry.go -- the namespace of named filter instances a chain names
// Related: fields.go -- the route field vocabulary a chain is checked against

package policy

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Ref is one entry of a filter chain: the filter instance's name and whether
// the operator deactivated it. A deactivated ref stays in the chain, so a
// consumer can tell "deactivated" from "never configured", and the name is
// always clean: no `inactive:` prefix is ever part of it.
type Ref struct {
	Name     string
	Inactive bool
}

// Final is the operator's decision after every filter of a chain passes.
// Zero denotes absence, never a configured chain.
type Final uint8

const (
	FinalAccept Final = iota + 1
	FinalReject
)

// Chain is an immutable filter chain: ordered refs and the final decision.
// Its zero value is an absent operator chain; refs alone do not make it
// configured, so a protocol can carry automatic refs and still decide its own
// default. Safe for concurrent use: callers MUST NOT mutate borrowed refs.
type Chain struct {
	refs  []Ref
	final Final
}

// NewChain constructs a configured chain and refuses a missing or unknown final.
// The caller MUST keep refs immutable for the lifetime of every derived chain.
func NewChain(refs []Ref, final Final) (Chain, error) {
	switch final {
	case FinalAccept, FinalReject:
		return Chain{refs: refs, final: final}, nil
	default: // Open input: callers can convert arbitrary numeric values.
		return Chain{}, fmt.Errorf("invalid policy final disposition %d: set final accept or final reject", final)
	}
}

// Refs returns borrowed refs. The caller MUST NOT mutate them.
func (c Chain) Refs() []Ref { return c.refs }

// Final returns the configured final, or zero for an absent operator chain.
func (c Chain) Final() Final { return c.final }

// Configured distinguishes an explicit empty chain from an absent chain.
func (c Chain) Configured() bool { return c.final != 0 }

// WithRefs replaces the refs while preserving the chain's final.
// The caller MUST keep refs immutable for the lifetime of every derived chain.
func (c Chain) WithRefs(refs []Ref) Chain {
	c.refs = refs
	return c
}

// ConcatChains joins the chains of an inheritance hierarchy, least specific
// first. Refs accumulate in order, while the most specific configured chain
// supplies the final: an absent child never erases its inherited final.
func ConcatChains(levels ...Chain) Chain {
	n := 0
	var effective Chain
	for _, level := range levels {
		n += len(level.refs)
		if level.Configured() {
			effective = level
		}
	}
	if n == 0 {
		return effective.WithRefs(nil)
	}
	refs := make([]Ref, 0, n)
	for _, level := range levels {
		refs = append(refs, level.refs...)
	}
	return effective.WithRefs(refs)
}

// ExtractChain reads `filter { <direction> { filters [ ... ]; final ...; } }`
// from one original configuration layer. An absent block is an absent chain;
// member deactivation is kept apart from the clean name. path names the layer
// in a refusal.
func ExtractChain(tree *config.Tree, direction, path string) (Chain, error) {
	final, err := ParseChainFinal(tree, direction, path)
	if err != nil {
		return Chain{}, err
	}
	if final == 0 {
		return Chain{}, nil
	}
	block := tree.GetContainer("filter").GetContainer(direction)
	return NewChain(RefsFromMembers(block.GetMultiValuesState("filters")), final)
}

// ParseChainFinal validates an original layer before inheritance can hide an
// omitted final, and answers zero for a layer with no chain in direction.
// Validation alone never allocates or copies refs.
func ParseChainFinal(tree *config.Tree, direction, path string) (Final, error) {
	fc := tree.GetContainer("filter")
	if fc == nil {
		return 0, nil
	}
	block := fc.GetContainer(direction)
	if block == nil {
		if len(fc.GetMultiValuesState(direction)) != 0 {
			return 0, fmt.Errorf("%s/filter/%s: list-only policy is no longer supported; write %s { filters [ NAME ]; final accept; }", path, direction, direction)
		}
		if _, exists := fc.Get(direction); exists {
			return 0, fmt.Errorf("%s/filter/%s: list-only policy is no longer supported; write %s { filters [ NAME ]; final accept; }", path, direction, direction)
		}
		return 0, nil
	}
	final, _ := block.Get("final")
	switch final {
	case "accept":
		return FinalAccept, nil
	case "reject":
		return FinalReject, nil
	default:
		return 0, fmt.Errorf("%s/filter/%s/final: missing or invalid final %q; every configured policy requires a final decision; write %s { filters [ NAME ]; final accept; } or final reject", path, direction, final, direction)
	}
}

// RefsFromMembers maps the config Tree's structural member view onto chain refs.
func RefsFromMembers(states []config.MemberState) []Ref {
	if len(states) == 0 {
		return nil
	}
	refs := make([]Ref, len(states))
	for i, s := range states {
		refs[i] = Ref{Name: s.Value, Inactive: s.Inactive}
	}
	return refs
}

// CanonicalName rewrites one chain ref name to the canonical
// `<plugin-process-name>:<filter-name>` form runtime dispatch consumes. It
// accepts three operator forms:
//
//  1. `<plugin-process-name>:<filter-name>`, kept unchanged when the first
//     token is not a filter type
//  2. `<filter-type>:<filter-name>`, the short form using the YANG list type
//     name (`prefix-list:CUSTOMERS`), resolved to the plugin that registered
//     the type
//  3. `<filter-name>`, looked up in reg to find its type, then resolved
//     through the type to the plugin
//
// filterTypes maps a filter type to its plugin (the plugin registry's
// `FilterTypesMap`). A name that cannot be resolved (a plain name reg does not
// hold, an unknown prefix) is returned untouched, so validation can still
// report the operator's own token instead of a synthetic one.
func CanonicalName(name string, reg *Registry, filterTypes map[string]string) string {
	if before, after, found := strings.Cut(name, ":"); found {
		if plugin, ok := filterTypes[before]; ok {
			var tb textbuf.Buffer
			return tb.Str(plugin).Byte(':').Str(after).String()
		}
		// Otherwise assume it is already a plugin process name (e.g.,
		// `bgp-filter-prefix:CUSTOMERS`) and pass through.
		return name
	}
	if reg == nil {
		return name
	}
	entry, ok := reg.Lookup(name)
	if !ok {
		return name
	}
	plugin, ok := filterTypes[entry.Type]
	if !ok {
		return name
	}
	var tb textbuf.Buffer
	return tb.Str(plugin).Byte(':').Str(name).String()
}
