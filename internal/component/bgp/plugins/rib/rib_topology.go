// Design: docs/architecture/plugin/rib-storage-design.md -- graph terminal for AS path topology
// Overview: rib_pipeline.go -- pipeline terminal dispatch
// Related: rib_attr_format.go -- formatASPath for pool-based AS path extraction

package rib

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/graph"
)

// graphAnswerKey is the envelope key `show bgp rib ... graph` answers under.
// The text rendering finds the box-drawing renderer by it (register.go,
// command.MustRegisterTextRenderer), so the payload stays data for `| json`.
const graphAnswerKey = "graph"

// graphEmptyText is the text drawing of a graph no route reached.
const graphEmptyText = "graph: no AS path in the selected routes\n"

// graphAnswer is the AS topology a graph terminal answers: one node per AS,
// one edge per peering link seen in an AS path.
type graphAnswer struct {
	Nodes []graphNodeAnswer `json:"nodes"`
	Edges []graphEdgeAnswer `json:"edges"`
}

// graphNodeAnswer is one AS. Layer is its hop depth: 0 is the origin.
type graphNodeAnswer struct {
	ASN   uint32 `json:"asn"`
	Name  string `json:"name"`
	Layer int    `json:"layer"`
}

// graphEdgeAnswer is one peering link, from the AS nearer the source to the
// AS nearer the origin.
type graphEdgeAnswer struct {
	From uint32 `json:"from"`
	To   uint32 `json:"to"`
}

// errGraphTooManyNodes refuses a topology past graph.MaxNodes and names the
// narrowing that fixes it.
func errGraphTooManyNodes(nodes int) error {
	return fmt.Errorf("%w: narrow the routes with a filter before graph, for example "+
		"`show bgp rib received prefix 10.0.0.0/8 graph`", graph.TooManyNodesError(nodes))
}

// newGraphAnswer converts the built topology to its wire form. Empty lists
// stay empty lists, so an empty graph answers `[]`, never null.
func newGraphAnswer(g *graph.Graph) graphAnswer {
	answer := graphAnswer{
		Nodes: make([]graphNodeAnswer, 0, len(g.Nodes)),
		Edges: make([]graphEdgeAnswer, 0, len(g.Edges)),
	}
	for _, n := range g.Nodes {
		answer.Nodes = append(answer.Nodes, graphNodeAnswer{ASN: n.ASN, Name: n.Name, Layer: n.Layer})
	}
	for _, e := range g.Edges {
		answer.Edges = append(answer.Edges, graphEdgeAnswer{From: e.FromASN, To: e.ToASN})
	}
	return answer
}

// renderGraphText is the text rendering of a graph answer, registered under
// graphAnswerKey: the box diagram graph.RenderText draws. It refuses a payload
// that is not a graph, and a graph past graph.MaxNodes, rather than drawing
// nothing.
func renderGraphText(payload json.RawMessage) (string, error) {
	var answer graphAnswer
	if err := json.Unmarshal(payload, &answer); err != nil {
		return "", fmt.Errorf("answer is not an AS topology: %w", err)
	}
	if len(answer.Nodes) == 0 {
		return graphEmptyText, nil
	}
	g := graph.Graph{
		Nodes: make([]graph.Node, 0, len(answer.Nodes)),
		Edges: make([]graph.Edge, 0, len(answer.Edges)),
	}
	for _, n := range answer.Nodes {
		g.Nodes = append(g.Nodes, graph.Node{ASN: n.ASN, Name: n.Name, Layer: n.Layer})
	}
	for _, e := range answer.Edges {
		g.Edges = append(g.Edges, graph.Edge{FromASN: e.From, ToASN: e.To})
	}
	return graph.RenderText(&g)
}

// graphTerminal drains the upstream pipeline, extracts AS paths from RouteItems
// and builds the AS topology. Its answer is the graphAnswer document in
// PipelineMeta.JSON, or the refusal in PipelineMeta.Refusal when the topology
// passes graph.MaxNodes.
type graphTerminal struct {
	upstream pipelineIterator
	meta     PipelineMeta
	drained  bool
}

func newGraphTerminal(upstream pipelineIterator) *graphTerminal {
	return &graphTerminal{upstream: upstream}
}

func (gt *graphTerminal) Next() (RouteItem, bool) {
	if !gt.drained {
		gt.drain()
	}
	return RouteItem{}, false
}

func (gt *graphTerminal) drain() {
	gt.drained = true

	var paths [][]uint32
	count := 0

	for {
		item, ok := gt.upstream.Next()
		if !ok {
			break
		}
		count++

		asPath := extractASPathFromItem(item)
		if len(asPath) > 0 {
			paths = append(paths, asPath)
		}
	}

	gt.meta.Count = count

	g := graph.BuildGraphFromPaths(paths)
	if len(g.Nodes) > graph.MaxNodes {
		gt.meta.Refusal = errGraphTooManyNodes(len(g.Nodes))
		return
	}

	document, err := json.Marshal(map[string]graphAnswer{graphAnswerKey: newGraphAnswer(g)})
	if err != nil {
		gt.meta.Refusal = fmt.Errorf("graph answer could not be encoded: %w", err)
		return
	}
	gt.meta.JSON = string(document)
}

func (gt *graphTerminal) Meta() PipelineMeta {
	if !gt.drained {
		gt.drain()
	}
	return gt.meta
}

// pipelineRefusal is the answer of a show pipeline whose terminal could not
// build its document. commandAnswer turns it into an error status, so the
// refusal reaches the caller as an error and never as a done answer.
type pipelineRefusal struct {
	reason error
}

// errPipelineRefusalNoReason guards commandAnswer against a refusal built
// without its reason.
var errPipelineRefusalNoReason = errors.New("show pipeline refused with no reason")

// commandAnswer is the (status, data, error) a show pipeline's answer reaches
// the dispatcher as: a pipelineRefusal is an error, anything else is done.
func commandAnswer(answer any) (string, any, error) {
	refusal, refused := answer.(pipelineRefusal)
	if refused {
		if refusal.reason == nil {
			return statusError, "", errPipelineRefusalNoReason
		}
		return statusError, "", refusal.reason
	}
	return statusDone, answer, nil
}
