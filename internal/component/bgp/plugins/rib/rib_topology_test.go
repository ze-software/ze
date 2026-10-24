package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/graph"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// decodeGraphAnswer reads the document a graph terminal answers, failing the
// test unless it is exactly one `graph` key holding nodes and edges.
func decodeGraphAnswer(t *testing.T, document string) graphAnswer {
	t.Helper()
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(document), &envelope), "graph answer is not a JSON object: %s", document)
	require.Len(t, envelope, 1, "graph answer carries one envelope key: %s", document)
	inner, ok := envelope[graphAnswerKey]
	require.True(t, ok, "graph answer has no %q key: %s", graphAnswerKey, document)
	var answer graphAnswer
	require.NoError(t, json.Unmarshal(inner, &answer))
	return answer
}

// TestGraphTerminalAnswersNodesAndEdges: the graph terminal answers the AS
// topology as data, one node per AS with its layer and one edge per peering
// link, never as pre-rendered text.
//
// Method: two routes sharing the origin AS 64503 through 64502; the answer must
// hold four nodes and the three links, and no box-drawing character.
func TestGraphTerminalAnswersNodesAndEdges(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", Direction: rpc.DirectionReceived,
			OutRoute: &Route{ASPath: []uint32{64501, 64502, 64503}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", Direction: rpc.DirectionReceived,
			OutRoute: &Route{ASPath: []uint32{64504, 64502, 64503}}},
	}

	gt := newGraphTerminal(&sliceSource{items: items})
	_, ok := gt.Next()
	assert.False(t, ok, "graph terminal yields no items")

	meta := gt.Meta()
	require.NoError(t, meta.Refusal)
	assert.Equal(t, 2, meta.Count)
	assert.NotContains(t, meta.JSON, "┌", "the payload carries no rendering")

	answer := decodeGraphAnswer(t, meta.JSON)
	layers := map[uint32]int{}
	for _, n := range answer.Nodes {
		layers[n.ASN] = n.Layer
	}
	assert.Equal(t, map[uint32]int{64503: 0, 64502: 1, 64501: 2, 64504: 2}, layers)
	assert.ElementsMatch(t, []graphEdgeAnswer{
		{From: 64501, To: 64502}, {From: 64504, To: 64502}, {From: 64502, To: 64503},
	}, answer.Edges)
}

// TestGraphTerminalAnswersEmptyGraph: no route reaching the terminal is an
// empty graph, and it is answered as one rather than as a route count.
func TestGraphTerminalAnswersEmptyGraph(t *testing.T) {
	meta := newGraphTerminal(&sliceSource{}).Meta()
	require.NoError(t, meta.Refusal)
	answer := decodeGraphAnswer(t, meta.JSON)
	assert.NotNil(t, answer.Nodes, "an empty graph answers an empty node list, not null")
	assert.Empty(t, answer.Nodes)
	assert.NotNil(t, answer.Edges)
	assert.Empty(t, answer.Edges)
}

// TestGraphTerminalRefusesPastMaxNodes: a topology larger than graph.MaxNodes
// is refused with an error naming the count and the limit, and no document is
// answered in its place.
func TestGraphTerminalRefusesPastMaxNodes(t *testing.T) {
	items := make([]RouteItem, graph.MaxNodes+1)
	for i := range items {
		items[i] = RouteItem{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24",
			Direction: rpc.DirectionReceived, OutRoute: &Route{ASPath: []uint32{uint32(i + 1), 99999}}}
	}

	meta := newGraphTerminal(&sliceSource{items: items}).Meta()
	require.Error(t, meta.Refusal)
	assert.Contains(t, meta.Refusal.Error(), "102 nodes")
	assert.Contains(t, meta.Refusal.Error(), "limit 100")
	assert.Empty(t, meta.JSON)
}

// TestGraphTerminalAnswersAtExactlyMaxNodes: graph.MaxNodes nodes is the last
// size answered, so the refusal starts one past it.
//
// VALIDATES: the limit boundary, MaxNodes answered and MaxNodes+1 refused.
// PREVENTS: an off-by-one refusing the largest graph the limit allows.
func TestGraphTerminalAnswersAtExactlyMaxNodes(t *testing.T) {
	items := make([]RouteItem, graph.MaxNodes-1) // the shared origin is the last node
	for i := range items {
		items[i] = RouteItem{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24",
			Direction: rpc.DirectionReceived, OutRoute: &Route{ASPath: []uint32{uint32(i + 1), 99999}}}
	}

	meta := newGraphTerminal(&sliceSource{items: items}).Meta()
	require.NoError(t, meta.Refusal)
	assert.Len(t, decodeGraphAnswer(t, meta.JSON).Nodes, graph.MaxNodes)
}

// TestGraphCommandAnswerStatus:a pipeline refusal reaches the operator as an
// error status, and an answered graph as done.
func TestGraphCommandAnswerStatus(t *testing.T) {
	status, data, err := commandAnswer(pipelineRefusal{reason: errGraphTooManyNodes(graph.MaxNodes + 5)})
	assert.Equal(t, statusError, status)
	assert.Empty(t, data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "105 nodes")

	r := newTestRIBManager(t)
	status, _, err = commandAnswer(r.showPipeline("*", []string{"received", "graph"}))
	assert.Equal(t, statusDone, status)
	assert.NoError(t, err)
}

// insertGraphRoute stores one route with AS_PATH [65001] for the pipeline tests.
func insertGraphRoute(r *RIBManager) {
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family.IPv4Unicast, concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001), []byte{24, 10, 0, 0})
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB
}

// TestGraphTerminalViaPipeline: `show bgp rib received graph` answers the
// structured document through the show pipeline.
func TestGraphTerminalViaPipeline(t *testing.T) {
	r := newTestRIBManager(t)
	insertGraphRoute(r)

	raw, ok := r.showPipeline("*", []string{"received", "graph"}).(json.RawMessage)
	require.True(t, ok, "graph answers a JSON document")
	answer := decodeGraphAnswer(t, string(raw))
	require.Len(t, answer.Nodes, 1)
	assert.Equal(t, uint32(65001), answer.Nodes[0].ASN)
}

// TestGraphTerminalViaBestPipeline: `show bgp rib best graph` answers the same
// structured document from the best-path pipeline.
func TestGraphTerminalViaBestPipeline(t *testing.T) {
	r := newTestRIBManager(t)
	insertGraphRoute(r)

	raw, ok := r.bestPipeline("*", []string{"graph"}).(json.RawMessage)
	require.True(t, ok, "best graph answers a JSON document")
	answer := decodeGraphAnswer(t, string(raw))
	require.Len(t, answer.Nodes, 1)
	assert.Equal(t, uint32(65001), answer.Nodes[0].ASN)
}

// TestGraphTextRendererDrawsBox: the renderer bgp-rib registers draws the box
// diagram from the payload alone, so the drawing lives on the reader's side.
func TestGraphTextRendererDrawsBox(t *testing.T) {
	payload := `{"nodes":[{"asn":64503,"name":"","layer":0},{"asn":64502,"name":"","layer":1}],` +
		`"edges":[{"from":64502,"to":64503}]}`
	text, err := renderGraphText(json.RawMessage(payload))
	require.NoError(t, err)
	assert.Contains(t, text, "AS64502")
	assert.Contains(t, text, "AS64503")
	assert.Contains(t, text, "┌")
}

// TestGraphTextRendererRefusesMalformedPayload: a payload that is not a graph
// is refused, never drawn as an empty diagram.
func TestGraphTextRendererRefusesMalformedPayload(t *testing.T) {
	_, err := renderGraphText(json.RawMessage(`{"nodes":"many"}`))
	require.Error(t, err)
}

// TestGraphTextRendererNamesEmptyGraph: an empty graph is drawn as a line that
// says so, not as nothing.
func TestGraphTextRendererNamesEmptyGraph(t *testing.T) {
	text, err := renderGraphText(json.RawMessage(`{"nodes":[],"edges":[]}`))
	require.NoError(t, err)
	assert.Equal(t, graphEmptyText, text)
}

// TestOperatorSeesGraphBox: the operator's default text rendering of
// `show bgp rib received graph` is the box diagram, and `| json` is the
// structure.
//
// Method: the daemon's answer from showPipeline goes through the same pipe
// chain the CLI applies (command.ProcessPipesDefaultFormatChecked), with the
// renderer bgp-rib's init() registered.
func TestOperatorSeesGraphBox(t *testing.T) {
	r := newTestRIBManager(t)
	insertGraphRoute(r)
	raw, ok := r.showPipeline("*", []string{"received", "graph"}).(json.RawMessage)
	require.True(t, ok)

	_, format, msg := command.ProcessPipesDefaultFormatChecked("show bgp rib received graph", "text")
	require.Empty(t, msg)
	text := format(string(raw))
	assert.Contains(t, text, "AS65001")
	assert.Contains(t, text, "┌")

	_, format, msg = command.ProcessPipesDefaultFormatChecked("show bgp rib received graph | json", "")
	require.Empty(t, msg)
	structured := format(string(raw))
	assert.NotContains(t, structured, "┌")
	assert.Contains(t, structured, `"asn": 65001`)
}

// TestGraphTerminalViaProtocolPipeline: `show bgp rib protocol bmp graph` (and
// `show bmp rib`, which relays it) answers the same structured document, a
// JSON object an SDK marshal accepts, not box text passed off as JSON.
//
// VALIDATES: the third graph site answers data.
// PREVENTS: json.RawMessage holding text that is not JSON at all.
func TestGraphTerminalViaProtocolPipeline(t *testing.T) {
	r := newTestRIBManager(t)
	bmpPeers := r.ribInPool[bmpProtocolID]
	bmpPeers["router1:peer1"] = storage.NewPeerRIB("router1:peer1")
	bmpPeers["router1:peer1"].Insert(family.IPv4Unicast, concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001), []byte{24, 10, 0, 1})

	status, result, err := r.showProtocolPipeline("bmp", []string{"graph"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	raw, ok := result.(json.RawMessage)
	require.True(t, ok, "protocol graph answers a JSON document")
	require.True(t, json.Valid(raw), "protocol graph answer is valid JSON: %s", raw)
	answer := decodeGraphAnswer(t, string(raw))
	require.Len(t, answer.Nodes, 1)
	assert.Equal(t, uint32(65001), answer.Nodes[0].ASN)
}
