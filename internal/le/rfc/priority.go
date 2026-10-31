// Design: docs/contributing/rfc-conformance-gates.md -- the priority tree, `./le rfc tree` and `./le rfc next`
// Related: profilecoverage.go -- the two figures every node carries
// Related: profile.go -- the use-case profiles; meta.go -- the two use-case rows that declare their members
// Related: graph/graph.go -- the edges and levels the order reads
// Related: tree.go -- the tree answer
// Related: next.go -- the next-gap answer
//
// priority.go holds what `./le rfc tree` and `./le rfc next` share: the corpus
// they read, assembled once, the members of a profile in its two declared
// tiers, and the order the owner rule of 2026-10-25 sets: roots before leaves
// (graph level), release profiles before the rest, then RFC number. The graph
// orders and suggests candidates; it never decides membership (owner: "A
// normative citation is never an obligation").
package rfc

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc/graph"
)

// keyProfile is the keyword `tree` and `next` take a profile name under.
const keyProfile = "profile"

// priorityCorpus is everything the tree and the next gap are judged from.
// Built only by newPriorityCorpus; read-only after.
type priorityCorpus struct {
	graph        *graph.Graph
	profiles     Profiles
	metas        map[string]Meta
	requirements []Requirement
	figures      map[string]StemFigures
	proven       map[string]bool
	// buildsOn is each node's builds-on targets, and updaters each node's
	// corpus updaters: the two edge kinds a row shows and a candidate is
	// named by. Neither adds a member.
	buildsOn map[string][]string
	updaters map[string][]string
}

// newPriorityCorpus indexes every edge of the graph by the node it reads from:
// a builds-on edge under the node it leaves, an updates edge under the node it
// updates. Every node is indexed, member of a profile or not. Every edge list
// stays sorted, because Edges answers them sorted.
func newPriorityCorpus(g *graph.Graph, profiles Profiles, metas map[string]Meta,
	requirements []Requirement, figures map[string]StemFigures, proven map[string]bool) *priorityCorpus {
	c := &priorityCorpus{graph: g, profiles: profiles, metas: metas, requirements: requirements,
		figures: figures, proven: proven, buildsOn: map[string][]string{}, updaters: map[string][]string{}}
	for _, edge := range g.Edges() {
		//exhaustive:ignore // Rows and candidates read builds-on and updates only; an obsoletes edge names a text the builds-on redirect already replaced.
		switch edge.Kind {
		case graph.KindBuildsOn:
			c.buildsOn[edge.From] = append(c.buildsOn[edge.From], edge.To)
		case graph.KindUpdates:
			c.updaters[edge.To] = append(c.updaters[edge.To], edge.From)
		}
	}
	return c
}

// loadPriorityCorpus assembles the corpus of one checkout through the same
// RenderInput the published pages read, so `tree` and `next` count exactly
// what the pages count: the verified discrimination records, the valid
// extraction sign-offs and the parsed profiles.
func loadPriorityCorpus(tree string) (*priorityCorpus, error) {
	collected, err := Collect(tree)
	if err != nil {
		return nil, err
	}
	in, err := NewRenderInput(tree, collected, nil, nil)
	if err != nil {
		return nil, err
	}
	return priorityCorpusOf(&in)
}

// priorityCorpusOf assembles the corpus from an input already rendered, so a
// caller that holds one (the site's ledger) reads the same figures without a
// second pass over the tree.
func priorityCorpusOf(in *RenderInput) (*priorityCorpus, error) {
	figures, err := StemFiguresOf(in)
	if err != nil {
		return nil, err
	}
	proven := provenRequirements(in.Discrimination)
	return newPriorityCorpus(in.Graph, in.Profiles, in.Metas, in.Requirements, figures, proven), nil
}

// profileScope is one profile's members. For a use-case profile, members is
// the required tier, declared by each summary's `Use cases` row, and optional
// the optional tier, declared by `Optional use cases`; the two are disjoint
// because ParseMeta refuses a name in both rows. For an aggregate, members is
// the summaries naming it and optional those of them whose `Aggregate tier`
// is optional; the two are disjoint because one flag places each summary.
type profileScope struct {
	name     string
	kind     string
	release  bool
	members  map[string]bool
	optional map[string]bool
}

// scope answers one named profile: a use-case profile's two declared tiers,
// or a technology aggregate's members split by their `Aggregate tier`. A
// name with no rfc/profiles/ file is refused naming every valid one, and its
// example names verb, the action the user ran (`tree` or `next`).
func (c *priorityCorpus) scope(name, verb string) (profileScope, error) {
	if out, held := scopeOf(c.profiles, c.metas, name); held {
		return out, nil
	}
	var tb textbuf.Buffer
	tb.Str("no profile ").Str(pyRepr(name)).Str(" under ").Str(profileRel).
		Str("/, so there is nothing to order. ")
	profileNamesHint(&tb, c.profiles)
	tb.Str(". Example: ./le rfc ").Str(verb).Str(" profile route-server")
	return profileScope{}, parseErr(&tb)
}

// scopeOf answers the named profile's two tiers, read from the summaries'
// declarations alone: the one place profile membership is built. held is
// false when no profile file carries the name.
func scopeOf(profiles Profiles, metas map[string]Meta, name string) (out profileScope, held bool) {
	if useCase, isUseCase := profiles.UseCases[name]; isUseCase {
		return useCaseScopeOf(&useCase, metas), true
	}
	if _, isAggregate := profiles.Aggregates[name]; !isAggregate {
		return profileScope{}, false
	}
	out = profileScope{name: name, kind: ProfileKindTechnology,
		members: map[string]bool{}, optional: map[string]bool{}}
	for stem := range metas {
		meta := metas[stem]
		if meta.Aggregate != name {
			continue
		}
		out.tier(meta.AggregateOptional)[stem] = true
	}
	return out, true
}

// useCaseScopeOf answers one use-case profile's two tiers: `Use cases` names
// the required tier and `Optional use cases` the optional one.
func useCaseScopeOf(useCase *UseCase, metas map[string]Meta) profileScope {
	out := profileScope{name: useCase.Name, kind: ProfileKindUseCase, release: useCase.Release,
		members: map[string]bool{}, optional: map[string]bool{}}
	for stem := range metas {
		meta := metas[stem]
		if slices.Contains(meta.UseCases, useCase.Name) {
			out.members[stem] = true
		}
		if slices.Contains(meta.OptionalUseCases, useCase.Name) {
			out.optional[stem] = true
		}
	}
	return out
}

// profileNamesHint writes every profile name a refusal can offer, use cases
// then aggregates.
func profileNamesHint(tb *textbuf.Buffer, profiles Profiles) {
	tb.Str("Name one of: ").Str(strings.Join(sortedKeysOf(profiles.UseCases), ", ")).Str(" (use cases); ").
		Str(strings.Join(sortedKeysOf(profiles.Aggregates), ", ")).Str(" (aggregates)")
}

// ProfileTiers is one profile's members as the summaries declare them, each
// tier sorted by stem. Kind is ProfileKindUseCase or ProfileKindTechnology;
// Release is false for an aggregate, which declares no release.
type ProfileTiers struct {
	Name     string
	Kind     string
	Title    string
	Release  bool
	Required []string
	Optional []string
}

// ProfileTiersOf answers the named profile's two tiers, built by scopeOf, the
// same membership `./le rfc tree` and `./le rfc next` read. A name no profile
// file carries is refused naming the valid ones.
func ProfileTiersOf(profiles Profiles, metas map[string]Meta, name string) (ProfileTiers, error) {
	scope, held := scopeOf(profiles, metas, name)
	if !held {
		var tb textbuf.Buffer
		tb.Str("no profile ").Str(pyRepr(name)).Str(" under ").Str(profileRel).
			Str("/, so it has no members to publish. ")
		profileNamesHint(&tb, profiles)
		return ProfileTiers{}, parseErr(&tb)
	}
	out := ProfileTiers{Name: name, Kind: scope.kind, Release: scope.release,
		Required: sortedKeysOf(scope.members), Optional: sortedKeysOf(scope.optional)}
	if scope.kind == ProfileKindUseCase {
		out.Title = profiles.UseCases[name].Title
		return out, nil
	}
	out.Title = profiles.Aggregates[name].Title
	return out, nil
}

// useCaseScopes answers every use-case profile, release ones only when
// releaseOnly, sorted by name.
func (c *priorityCorpus) useCaseScopes(releaseOnly bool) []profileScope {
	var out []profileScope
	for _, name := range sortedKeysOf(c.profiles.UseCases) {
		useCase := c.profiles.UseCases[name]
		if releaseOnly && !useCase.Release {
			continue
		}
		out = append(out, useCaseScopeOf(&useCase, c.metas))
	}
	return out
}

// tier answers one of s's two tiers: the optional one when optional, else the
// required one (an aggregate's members).
func (s *profileScope) tier(optional bool) map[string]bool {
	if optional {
		return s.optional
	}
	return s.members
}

// path renders the declaration that puts stem in the tier optional selects,
// such as `route-server: required by rfc/short/rfc4271.md`. Membership is the
// caller's to establish, and both callers read stem out of that tier: nextGap
// tests it, and treeRows ranges over it.
func (s *profileScope) path(stem string, optional bool) string {
	var tb textbuf.Buffer
	tb.Str(s.name).Str(": ")
	switch {
	case optional:
		tb.Str("optional in ")
	case s.kind == ProfileKindTechnology:
		tb.Str("aggregate named by ")
	default:
		tb.Str("required by ")
	}
	return tb.Str(summaryRel).Byte('/').Str(stem).Str(".md").String()
}

// requiredUnion answers the members of the scopes' required tiers.
func requiredUnion(scopes []profileScope) map[string]bool {
	out := map[string]bool{}
	for index := range scopes {
		for stem := range scopes[index].members {
			out[stem] = true
		}
	}
	return out
}

// optionalUnion answers the members of the scopes' optional tiers that no
// scope requires: across several profiles, a stem one of them requires is
// counted there and is not also listed as optional.
func optionalUnion(scopes []profileScope) map[string]bool {
	required := requiredUnion(scopes)
	out := map[string]bool{}
	for index := range scopes {
		for stem := range scopes[index].optional {
			if !required[stem] {
				out[stem] = true
			}
		}
	}
	return out
}

// ScopeRollup is a rollup with its completeness stated, so a reader of the
// JSON never recomputes the rule that an unwalked member, or no member at
// all, keeps a set incomplete.
type ScopeRollup struct {
	Rollup
	MandatoryComplete bool `json:"mandatory-complete"`
}

// rollupOfScopes sums the union of the scopes' required members: the profile
// figure, which the optional tier never enters.
func (c *priorityCorpus) rollupOfScopes(scopes []profileScope) ScopeRollup {
	return c.rollupOfSet(requiredUnion(scopes))
}

// optionalRollupOfScopes sums the optional tier optionalUnion answers, as its
// own figure beside the profile's.
func (c *priorityCorpus) optionalRollupOfScopes(scopes []profileScope) ScopeRollup {
	return c.rollupOfSet(optionalUnion(scopes))
}

// rollupOfSet sums one set of members with its completeness stated.
func (c *priorityCorpus) rollupOfSet(members map[string]bool) ScopeRollup {
	rollup := rollupOf(c.figures, sortedSet(members))
	return ScopeRollup{Rollup: rollup, MandatoryComplete: rollup.mandatoryComplete()}
}

// level answers stem's graph level, nil for a stem with no text in the
// corpus: three summaries have none, and a level of 0 would claim it builds
// on nothing.
func (c *priorityCorpus) level(stem string) *int { return levelOf(c.graph, stem) }

// compareLevels orders two levels, roots first, an unknown level last.
func compareLevels(left, right *int) int {
	if left == nil {
		if right == nil {
			return 0
		}
		return 1
	}
	if right == nil {
		return -1
	}
	return *left - *right
}
