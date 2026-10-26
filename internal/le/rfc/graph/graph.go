// Design: docs/contributing/rfc-conformance-gates.md -- the RFC dependency graph
// Related: parse.go -- what one text declares
//
// graph.go builds the RFC dependency graph of the corpus: one node per text
// under rfc/full/ and rfc/drafts/, an edge for each normative citation of
// another corpus document (builds-on) and for each header lineage line
// (updates, obsoletes). Citation cycles collapse into components, and each
// component gets a level: 0 when it builds on nothing in the corpus, else one
// above the highest component it builds on.
package graph

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Kind is what an edge says about its two documents.
type Kind uint8

// The edge kinds. Zero is no kind, so an edge built without one is visible.
const (
	KindUnspecified Kind = iota
	// KindBuildsOn is a normative citation: From needs To.
	KindBuildsOn
	// KindUpdates is a header `Updates:` line of From naming To.
	KindUpdates
	// KindObsoletes is a header `Obsoletes:` line of From naming To.
	KindObsoletes
)

// String is the kind's TSV spelling.
func (k Kind) String() string {
	switch k {
	case KindBuildsOn:
		return "builds-on"
	case KindUpdates:
		return "updates"
	case KindObsoletes:
		return "obsoletes"
	case KindUnspecified:
		return "unspecified"
	}
	return "unspecified"
}

// Edge is one in-corpus relation between two documents.
type Edge struct {
	From string
	Kind Kind
	To   string
}

// Graph is the corpus's documents, their edges, and the cycle components and
// levels derived from the builds-on edges. Built only by Build; read-only
// after, so safe for concurrent reads.
type Graph struct {
	nodes []string
	edges []Edge
	// texts is what each document's own header said, kept whole: the header
	// names RFCs outside the corpus too, and the Meta link check compares
	// against all of them.
	texts map[string]Text
	// component is each node's component index, and level each component's.
	component map[string]int
	levels    []int
	members   [][]string
}

// excludedTargets are the two RFCs every modern text cites for its
// requirement keywords. They say how to read a document, not what it builds
// on, so an edge to either would make every RFC a leaf of BCP 14.
var excludedTargets = map[string]bool{"rfc2119": true, "rfc8174": true}

// Load parses every text under rfc/full/ and rfc/drafts/ of tree and builds
// the graph. A text that cannot be read is an error naming it: a missing node
// silently shrinks every profile that reaches it. An absent directory holds no
// texts, so a fixture tree builds an empty graph rather than failing.
func Load(tree string) (*Graph, error) {
	texts := map[string]Text{}
	for _, dir := range []string{"rfc/full", "rfc/drafts"} {
		entries, err := os.ReadDir(filepath.Join(tree, filepath.FromSlash(dir)))
		// refuse-by-default: guards Has -- a tree that holds no texts (a fixture, a scratch tree) is a graph of no nodes, and every reader asks Has before it trusts a header
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(dir).Str(": cannot read the RFC texts: ").Str(err.Error()).
				Str(". Run from the repository root, where rfc/full/ holds the texts").String())
		}
		for _, entry := range entries {
			name := entry.Name()
			stem, isText := strings.CutSuffix(name, ".txt")
			if !isText {
				continue
			}
			if entry.IsDir() {
				continue
			}
			body, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(dir), name)) //nolint:gosec // A text the directory listing of the repository's own rfc tree just named.
			if err != nil {
				var tb textbuf.Buffer
				return nil, parseErr(tb.Str(dir).Byte('/').Str(name).Str(": cannot read: ").Str(err.Error()).String())
			}
			text, err := Parse(string(body))
			if err != nil {
				var tb textbuf.Buffer
				return nil, parseErr(tb.Str(dir).Byte('/').Str(name).Str(": ").Str(err.Error()).String())
			}
			texts[stem] = text
		}
	}
	return Build(texts), nil
}

// Build makes the graph over texts, keyed by stem.
//
// A builds-on edge to an obsoleted document points at its successor, the
// corpus document whose header obsoletes it, followed down the chain; the
// header edges keep the document the header names, because they ARE that
// header. Edges to documents outside the corpus, to RFC 2119 and 8174, and
// to the document itself are dropped.
func Build(texts map[string]Text) *Graph {
	g := &Graph{texts: texts, component: map[string]int{}}
	byDraft := map[string]string{}
	for stem := range texts {
		g.nodes = append(g.nodes, stem)
		if strings.HasPrefix(stem, "draft-") {
			byDraft[DraftStem(stem)] = stem
		}
	}
	slices.Sort(g.nodes)
	successor := successors(g.nodes, texts)
	seen := map[Edge]bool{}
	add := func(edge Edge) {
		if _, inCorpus := texts[edge.To]; !inCorpus {
			return
		}
		if edge.To == edge.From {
			return
		}
		if excludedTargets[edge.To] {
			return
		}
		if seen[edge] {
			return
		}
		seen[edge] = true
		g.edges = append(g.edges, edge)
	}
	for _, from := range g.nodes {
		text := texts[from]
		for _, cited := range text.Cites {
			if node, isDraft := byDraft[cited]; isDraft {
				cited = node
			}
			add(Edge{From: from, Kind: KindBuildsOn, To: redirect(cited, successor)})
		}
		for _, to := range text.Updates {
			add(Edge{From: from, Kind: KindUpdates, To: to})
		}
		for _, to := range text.Obsoletes {
			add(Edge{From: from, Kind: KindObsoletes, To: to})
		}
	}
	slices.SortFunc(g.edges, compareEdges)
	g.collapse()
	return g
}

// successors maps each obsoleted stem to the corpus document whose header
// obsoletes it. Two documents obsoleting one stem resolve to the lower stem,
// so two runs agree.
func successors(nodes []string, texts map[string]Text) map[string]string {
	out := map[string]string{}
	for _, node := range nodes {
		for _, old := range texts[node].Obsoletes {
			if _, taken := out[old]; taken {
				continue
			}
			out[old] = node
		}
	}
	return out
}

// redirect follows the successor chain from stem. The chain is bounded by the
// number of successors, so a header cycle (two texts obsoleting each other)
// stops rather than spins.
func redirect(stem string, successor map[string]string) string {
	for range len(successor) {
		next, obsoleted := successor[stem]
		if !obsoleted {
			return stem
		}
		stem = next
	}
	return stem
}

func compareEdges(a, b Edge) int {
	if c := strings.Compare(a.From, b.From); c != 0 {
		return c
	}
	if a.Kind != b.Kind {
		return int(a.Kind) - int(b.Kind)
	}
	return strings.Compare(a.To, b.To)
}

// collapse finds the strongly connected components of the builds-on edges
// (Tarjan) and levels them. Tarjan emits a component only after every
// component it reaches, so one pass in emission order sees each dependency's
// level before it needs it.
//
// The recursion is over the corpus graph, bounded by its node count (a few
// hundred texts the repository holds), never by input a peer controls.
func (g *Graph) collapse() {
	deps := map[string][]string{}
	for _, edge := range g.edges {
		if edge.Kind == KindBuildsOn {
			deps[edge.From] = append(deps[edge.From], edge.To)
		}
	}
	index := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var visit func(node string)
	visit = func(node string) {
		index[node] = len(index)
		low[node] = index[node]
		stack = append(stack, node)
		onStack[node] = true
		for _, dep := range deps[node] {
			if _, visited := index[dep]; !visited {
				visit(dep)
				low[node] = min(low[node], low[dep])
				continue
			}
			if onStack[dep] {
				low[node] = min(low[node], index[dep])
			}
		}
		if low[node] != index[node] {
			return
		}
		g.emit(&stack, onStack, node, deps)
	}
	for _, node := range g.nodes {
		if _, visited := index[node]; !visited {
			visit(node)
		}
	}
}

// emit pops one component off the Tarjan stack and gives it its level.
func (g *Graph) emit(stack *[]string, onStack map[string]bool, root string, deps map[string][]string) {
	id := len(g.members)
	var members []string
	for {
		top := (*stack)[len(*stack)-1]
		*stack = (*stack)[:len(*stack)-1]
		onStack[top] = false
		g.component[top] = id
		members = append(members, top)
		if top == root {
			break
		}
	}
	slices.Sort(members)
	level := 0
	for _, member := range members {
		for _, dep := range deps[member] {
			other := g.component[dep]
			if other == id {
				continue
			}
			level = max(level, g.levels[other]+1)
		}
	}
	g.members = append(g.members, members)
	g.levels = append(g.levels, level)
}

// Nodes answers every document stem in the corpus, sorted.
func (g *Graph) Nodes() []string { return slices.Clone(g.nodes) }

// Edges answers every in-corpus edge, sorted by From, kind and To.
func (g *Graph) Edges() []Edge { return slices.Clone(g.edges) }

// Has answers whether stem is a document of the corpus.
func (g *Graph) Has(stem string) bool {
	_, ok := g.texts[stem]
	return ok
}

// Level answers stem's level, and false for a stem the corpus does not hold.
func (g *Graph) Level(stem string) (int, bool) {
	id, ok := g.component[stem]
	if !ok {
		return 0, false
	}
	return g.levels[id], true
}

// Component answers the members of stem's citation cycle, sorted, stem
// included; a document in no cycle is a component of one. False for a stem
// the corpus does not hold.
func (g *Graph) Component(stem string) ([]string, bool) {
	id, ok := g.component[stem]
	if !ok {
		return nil, false
	}
	return slices.Clone(g.members[id]), true
}

// HeaderUpdates answers what stem's own header says it updates, RFCs outside
// the corpus included, and false for a stem the corpus does not hold.
func (g *Graph) HeaderUpdates(stem string) ([]string, bool) {
	text, ok := g.texts[stem]
	return slices.Clone(text.Updates), ok
}

// HeaderObsoletes answers what stem's own header says it obsoletes, RFCs
// outside the corpus included, and false for a stem the corpus does not hold.
func (g *Graph) HeaderObsoletes(stem string) ([]string, bool) {
	text, ok := g.texts[stem]
	return slices.Clone(text.Obsoletes), ok
}

// Updaters answers every corpus document whose header updates stem, sorted.
func (g *Graph) Updaters(stem string) []string { return g.sources(KindUpdates, stem) }

func (g *Graph) sources(kind Kind, stem string) []string {
	var out []string
	for _, edge := range g.edges {
		if edge.Kind == kind && edge.To == stem {
			out = append(out, edge.From)
		}
	}
	return out
}

// TSV renders the edges as rfc/graph.tsv: a comment naming the generator, a
// column header, then one `from<TAB>kind<TAB>to` row per edge.
func (g *Graph) TSV() string {
	var tb textbuf.Buffer
	tb.Str("# Generated by `./le rfc index-update` from rfc/full/ and rfc/drafts/. Do not edit.\n")
	tb.Str("from\tkind\tto\n")
	for _, edge := range g.edges {
		tb.Str(edge.From).Byte('\t').Str(edge.Kind.String()).Byte('\t').Str(edge.To).Byte('\n')
	}
	return tb.String()
}
