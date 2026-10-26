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

// newPriorityCorpus indexes the graph's edges by the member they leave from. Every edge list stays sorted, because Edges answers them sorted.
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
	signed, _, err := evaluateExtractions(in.Deriver, in.Requirements)
	if err != nil {
		return nil, err
	}
	walked := make(map[string]bool, len(signed))
	for stem := range signed {
		walked[stem] = true
	}
	proven := provenRequirements(in.Discrimination)
	figures := figuresByStem(in.Requirements, in.Stems, walked, proven)
	return newPriorityCorpus(in.Graph, in.Profiles, in.Metas, in.Requirements, figures, proven), nil
}

// profileScope is one profile's members. For a use-case profile, members is
// the required tier, declared by each summary's `Use cases` row, and optional
// the optional tier, declared by `Optional use cases`; the two are disjoint
// because ParseMeta refuses a name in both rows. For an aggregate, members is
// the summaries naming it and optional is empty.
type profileScope struct {
	name     string
	kind     string
	release  bool
	members  map[string]bool
	optional map[string]bool
}

// scope answers one named profile: a use-case profile's two declared tiers,
// or a technology aggregate's members. A name with no rfc/profiles/ file is
// refused naming every valid one, and its example names verb, the action the
// user ran (`tree` or `next`).
func (c *priorityCorpus) scope(name, verb string) (profileScope, error) {
	if useCase, held := c.profiles.UseCases[name]; held {
		return c.useCaseScope(&useCase), nil
	}
	if _, held := c.profiles.Aggregates[name]; held {
		members := map[string]bool{}
		for stem := range c.metas {
			if c.metas[stem].Aggregate == name {
				members[stem] = true
			}
		}
		return profileScope{name: name, kind: profileKindTechnology, members: members, optional: map[string]bool{}}, nil
	}
	var tb textbuf.Buffer
	tb.Str("no profile ").Str(pyRepr(name)).Str(" under ").Str(profileRel).
		Str("/, so there is nothing to order. Name one of: ").
		Str(strings.Join(sortedKeysOf(c.profiles.UseCases), ", ")).Str(" (use cases); ").
		Str(strings.Join(sortedKeysOf(c.profiles.Aggregates), ", ")).
		Str(" (aggregates). Example: ./le rfc ").Str(verb).Str(" profile route-server")
	return profileScope{}, parseErr(&tb)
}

// useCaseScope answers one use-case profile's two tiers, read from the
// summaries' declarations alone: the one place use-case membership is built.
func (c *priorityCorpus) useCaseScope(useCase *UseCase) profileScope {
	out := profileScope{name: useCase.Name, kind: profileKindUseCase, release: useCase.Release,
		members: map[string]bool{}, optional: map[string]bool{}}
	for stem := range c.metas {
		meta := c.metas[stem]
		if slices.Contains(meta.UseCases, useCase.Name) {
			out.members[stem] = true
		}
		if slices.Contains(meta.OptionalUseCases, useCase.Name) {
			out.optional[stem] = true
		}
	}
	return out
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
		out = append(out, c.useCaseScope(&useCase))
	}
	return out
}

// path renders the declaration that puts stem in the scope, such as
// `route-server: required by rfc/short/rfc4271.md`, and MUST be called only
// for a member of either tier.
func (s *profileScope) path(stem string) string {
	var tb textbuf.Buffer
	tb.Str(s.name).Str(": ")
	switch {
	case s.members[stem] && s.kind == profileKindTechnology:
		tb.Str("aggregate named by ")
	case s.members[stem]:
		tb.Str("required by ")
	case s.optional[stem]:
		tb.Str("optional in ")
	default:
		panic("BUG: profileScope.path asked for " + stem + ", which " + s.name + " does not list")
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
