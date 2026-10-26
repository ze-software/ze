// Design: docs/contributing/rfc-conformance-gates.md -- the priority tree, `./le rfc tree` and `./le rfc next`
// Related: profilecoverage.go -- the two figures every node carries
// Related: profile.go -- the use-case profiles whose closures these walk
// Related: graph/graph.go -- the edges and levels the order reads
// Related: tree.go -- the tree answer
// Related: next.go -- the next-gap answer
//
// priority.go holds what `./le rfc tree` and `./le rfc next` share: the corpus
// they read, assembled once, the closure of a profile over the dependency
// graph, and the order the owner rule of 2026-10-25 sets: roots before leaves
// (graph level), release profiles before the rest, then RFC number.
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
	// corpus updaters: the two edge kinds a closure follows.
	buildsOn map[string][]string
	updaters map[string][]string
}

// newPriorityCorpus indexes the graph's edges by the node a closure leaves
// from. Every edge list stays sorted, because Edges answers them sorted.
func newPriorityCorpus(g *graph.Graph, profiles Profiles, metas map[string]Meta,
	requirements []Requirement, figures map[string]StemFigures, proven map[string]bool) *priorityCorpus {
	c := &priorityCorpus{graph: g, profiles: profiles, metas: metas, requirements: requirements,
		figures: figures, proven: proven, buildsOn: map[string][]string{}, updaters: map[string][]string{}}
	for _, edge := range g.Edges() {
		//exhaustive:ignore // A closure follows builds-on and updates only; an obsoletes edge names a text the builds-on redirect already replaced.
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

// closureStep is how a node first entered a closure: a root, or reached from
// the node before it over one edge kind.
type closureStep struct {
	root bool
	from string
	kind graph.Kind
}

// profileScope is one profile's members and how each was reached.
type profileScope struct {
	name    string
	kind    string
	release bool
	reached map[string]closureStep
	absent  []int
}

// closure answers roots and every node reachable from them by builds-on
// edges and by the updates edges pointing at them, each with how it was first
// reached. Breadth first from the roots in their declared order, so every
// path is a shortest one. Bounded by the corpus: a node enters once.
func (c *priorityCorpus) closure(roots []string) map[string]closureStep {
	reached := make(map[string]closureStep, len(roots))
	queue := make([]string, 0, len(roots))
	for _, root := range roots {
		if _, held := reached[root]; held {
			continue
		}
		reached[root] = closureStep{root: true}
		queue = append(queue, root)
	}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, target := range c.buildsOn[node] {
			if _, held := reached[target]; !held {
				reached[target] = closureStep{from: node, kind: graph.KindBuildsOn}
				queue = append(queue, target)
			}
		}
		for _, updater := range c.updaters[node] {
			if _, held := reached[updater]; !held {
				reached[updater] = closureStep{from: node, kind: graph.KindUpdates}
				queue = append(queue, updater)
			}
		}
	}
	return reached
}

// scope answers one named profile: a use-case profile's closure over the
// graph, or a technology aggregate's members, each a root of its own. A name
// with no rfc/profiles/ file is refused naming every valid one.
func (c *priorityCorpus) scope(name string) (profileScope, error) {
	if useCase, held := c.profiles.UseCases[name]; held {
		return c.useCaseScope(&useCase), nil
	}
	if _, held := c.profiles.Aggregates[name]; held {
		var members []string
		for _, stem := range sortedKeysOf(c.metas) {
			if c.metas[stem].Aggregate == name {
				members = append(members, stem)
			}
		}
		return profileScope{name: name, kind: profileKindTechnology, reached: c.closureOfMembers(members)}, nil
	}
	var tb textbuf.Buffer
	tb.Str("no profile ").Str(pyRepr(name)).Str(" under ").Str(profileRel).
		Str("/, so there is nothing to order. Name one of: ").
		Str(strings.Join(sortedKeysOf(c.profiles.UseCases), ", ")).Str(" (use cases); ").
		Str(strings.Join(sortedKeysOf(c.profiles.Aggregates), ", ")).
		Str(" (aggregates). Example: ./le rfc tree profile route-server")
	return profileScope{}, parseErr(&tb)
}

// closureOfMembers makes every member a root, following no edge: an
// aggregate is the RFCs that name it, not what they build on.
func (*priorityCorpus) closureOfMembers(members []string) map[string]closureStep {
	out := make(map[string]closureStep, len(members))
	for _, stem := range members {
		out[stem] = closureStep{root: true}
	}
	return out
}

// useCaseScope answers one use-case profile's closure.
func (c *priorityCorpus) useCaseScope(useCase *UseCase) profileScope {
	return profileScope{name: useCase.Name, kind: profileKindUseCase, release: useCase.Release,
		reached: c.closure(useCase.Roots), absent: slices.Clone(useCase.Absent)}
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

// path renders why stem is in scope, and MUST be called only for a member: the edges from a root to it, such as
// `rfc7947 builds-on rfc4271 updated-by rfc7606`. Bounded by the closure:
// every step leads to a node reached earlier, and a root ends it.
func (s *profileScope) path(stem string) string {
	var hops []string
	node := stem
	for {
		step, held := s.reached[node]
		if !held {
			panic("BUG: profileScope.path asked for " + node + ", which the closure of " + s.name + " does not hold")
		}
		if step.root {
			break
		}
		label := "builds-on"
		if step.kind == graph.KindUpdates {
			label = "updated-by"
		}
		hops = append(hops, node, label)
		node = step.from
	}
	var tb textbuf.Buffer
	tb.Str(s.name).Str(": ").Str(node)
	for index := len(hops) - 1; index > 0; index -= 2 {
		tb.Byte(' ').Str(hops[index]).Byte(' ').Str(hops[index-1])
	}
	if node == stem {
		tb.Str(" (root)")
	}
	return tb.String()
}

// absentStems answers the stems of the absent RFCs of every scope,
// deduplicated and in RFC number order.
func absentStems(scopes []profileScope) []string {
	numbers := absentNumbers(scopes)
	out := make([]string, 0, len(numbers))
	for _, number := range numbers {
		out = append(out, absentFigures(number).Stem)
	}
	return out
}

// absentNumbers answers the deduplicated absent RFC numbers of every scope.
func absentNumbers(scopes []profileScope) []int {
	seen := map[int]bool{}
	var out []int
	for index := range scopes {
		for _, number := range scopes[index].absent {
			if !seen[number] {
				seen[number] = true
				out = append(out, number)
			}
		}
	}
	slices.Sort(out)
	return out
}

// ScopeRollup is a rollup with its completeness stated, so a reader of the
// JSON never recomputes the rule that absent and unwalked members keep a set
// incomplete.
type ScopeRollup struct {
	Rollup
	MandatoryComplete bool `json:"mandatory-complete"`
}

// rollupOfScopes sums the union of the scopes' members and absent RFCs.
func (c *priorityCorpus) rollupOfScopes(scopes []profileScope) ScopeRollup {
	union := map[string]bool{}
	for index := range scopes {
		for stem := range scopes[index].reached {
			union[stem] = true
		}
	}
	rollup := rollupOf(c.figures, sortedSet(union), absentNumbers(scopes))
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
