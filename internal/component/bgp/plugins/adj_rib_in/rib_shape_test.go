// Design: docs/architecture/api/commands.md -- a plugin declares its own answer shape
// Related: rib.go -- commandDecls, the declaration under test
package adj_rib_in

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// wantShape is what one command is expected to declare.
//
// The three fields are the three CommandDecl carries (pkg/plugin/rpc/types.go),
// spelled as they travel: the shape is the wire spelling the engine parses at
// Stage 1, and a name is a JSON key of the answer verbatim.
type wantShape struct {
	shape         string
	columns       []string
	addressFields []string
}

// wantShapes is the declaration each adj-rib-in command owes, read from the
// function that PRODUCES its answer. The producer is named on every row,
// because that function is the only thing that can settle what the answer holds.
var wantShapes = map[string]wantShape{
	// status (rib_commands.go): "peers" maps an address to a COUNT, not to an
	// object, so it is no row set.
	"show bgp adj-rib-in status": {shape: "doc"},
	// show (rib_commands.go): "adj-rib-in" maps a peer address to that peer's
	// routes, which is rows keyed by identity. A row is a LIST here, so no
	// column order goes with it: a column name orders the keys of a row, and
	// these sit one level below the row.
	"show bgp adj-rib-in": {shape: "map"},
	// The request verbs answer a report of what they did rather than a data set,
	// and they are outside the population this spec measured.
	"request bgp adj-rib-in replay":            {},
	"request bgp adj-rib-in claim-replay":      {},
	"request bgp adj-rib-in enable-validation": {},
	"request bgp adj-rib-in accept-routes":     {},
	"request bgp adj-rib-in reject-routes":     {},
	"request bgp adj-rib-in batch-validate":    {},
	"request bgp adj-rib-in revalidate":        {},
}

// declByName indexes a declaration list by command path, and fails when the
// list and the expectation table name different commands. A command added to
// one and not the other is the failure this catches.
func declByName(t *testing.T, decls []sdk.CommandDecl, want map[string]wantShape) map[string]sdk.CommandDecl {
	t.Helper()

	byName := make(map[string]sdk.CommandDecl, len(decls))
	// Range over the index: sdk.CommandDecl is large enough that a value
	// variable copies it on every iteration.
	for i := range decls {
		name := decls[i].Name
		require.NotContains(t, byName, name, "the plugin declares %q twice", name)
		byName[name] = decls[i]
	}
	for name := range byName {
		assert.Contains(t, want, name, "the plugin declares %q and this test states nothing about it", name)
	}
	for name := range want {
		assert.Contains(t, byName, name, "this test expects %q and the plugin declares no such command", name)
	}
	return byName
}

// TestCommandDeclsDeclareTheAnswerShape holds every adj-rib-in command to
// declaring what its answer holds.
//
// VALIDATES: AC-17 for the two `show bgp adj-rib-in` paths.
// PREVENTS: a command reaching no pre-dispatch check. validateDeclaredShape
// (internal/component/command/pipe.go) returns at `if !declared`, so an
// undeclared command accepts every operator until its answer is in hand, and
// the published catalog names none of them.
func TestCommandDeclsDeclareTheAnswerShape(t *testing.T) {
	byName := declByName(t, commandDecls(), wantShapes)

	for name, want := range wantShapes {
		decl, found := byName[name]
		if !found {
			continue
		}
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, want.shape, decl.Shape, "%s declares shape %q", name, decl.Shape)
			assert.Equal(t, want.columns, decl.Columns, "%s declares columns %v", name, decl.Columns)
			assert.Equal(t, want.addressFields, decl.AddressFields, "%s declares address fields %v", name, decl.AddressFields)

			// A doc answers one document, so a column order over it would order
			// nothing and an address-field list would admit an address operator
			// over rows that are not there.
			if decl.Shape == "doc" {
				assert.Empty(t, decl.Columns, "%s answers one document and declares columns", name)
				assert.Empty(t, decl.AddressFields, "%s answers one document and declares address fields", name)
			}
		})
	}
}

// TestDeclaredShapeSpellingIsOneTheEngineParses holds each declaration to a
// spelling ParseAnswerShape (internal/component/command/pipe_catalog.go) knows.
//
// VALIDATES: AC-2 from the declaring side.
// PREVENTS: the plugin failing Stage 1 and taking its commands down with it.
// validateShapeDecls refuses a fourth spelling and the refusal fails the WHOLE
// registration, so `"table"` for `"tab"` is a typo that stops the daemon
// storing received routes at all.
func TestDeclaredShapeSpellingIsOneTheEngineParses(t *testing.T) {
	for _, decl := range commandDecls() {
		switch decl.Shape {
		case "", "doc", "map", "tab":
		default:
			t.Errorf("%s declares shape %q, which is not doc, map or tab", decl.Name, decl.Shape)
		}
		if decl.Shape != "" {
			continue
		}
		assert.Empty(t, decl.Columns, "%s declares columns and no shape, which Stage 1 refuses", decl.Name)
		assert.Empty(t, decl.AddressFields, "%s declares address fields and no shape, which Stage 1 refuses", decl.Name)
	}
}

// populatedManager answers a manager holding one route from one peer, so the
// two producers below write a populated answer rather than an empty one.
//
// An empty answer would make the assertions vacuously true: rowsInKeyed
// (internal/component/command/answer_shape.go) reports an empty answer as
// having zero rows, which is a different fact from an answer whose shape cannot
// carry rows.
func populatedManager(t *testing.T) *AdjRIBInManager {
	t.Helper()

	r := newTestManager(t)
	r.handleReceived(&bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: "180a0000"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{NextHop: "10.0.0.1", Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	})
	require.NotEmpty(t, r.ribIn, "the fixture stored no route")
	return r
}

// peerRoute answers the event that stores one route from one peer, so a
// manager can be given two peers that an operator keeping ONE row can be told
// apart by.
func peerRoute(t *testing.T, peer, prefix, nlriHex string) *bgp.Event {
	t.Helper()

	return &bgp.Event{
		Message: &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 100},
		Peer: mustMarshal(t, map[string]any{
			"remote": map[string]any{"address": peer, "as": uint32(65001)},
			"local":  map[string]any{"address": "10.0.0.2", "as": uint32(65002)},
		}),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: nlriHex},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{NextHop: "10.0.0.2", Action: routeaction.Add, NLRIs: []any{prefix}},
			},
		},
	}
}

// twoPeerManager answers a manager holding one route from each of two peers.
//
// Two peers are what makes the row operators discriminating: over one peer,
// `| first 1` answers the whole table whether it selected a row or passed the
// payload through.
func twoPeerManager(t *testing.T) *AdjRIBInManager {
	t.Helper()

	r := newTestManager(t)
	r.handleReceived(peerRoute(t, firstPeer, firstPrefix, "180a0000"))
	r.handleReceived(peerRoute(t, secondPeer, secondPrefix, "180a0100"))
	require.Len(t, r.ribIn, 2, "the fixture stored routes for %d peers, want 2", len(r.ribIn))
	return r
}

const (
	firstPeer    = "10.0.0.1"
	secondPeer   = "10.0.0.3"
	firstPrefix  = "10.0.0.0/24"
	secondPrefix = "10.1.0.0/24"
)

// declareShapes registers this plugin's declarations the way Stage 1 does, so a
// chain is validated against what the plugin itself says about its answers.
func declareShapes(t *testing.T) {
	t.Helper()

	decls := commandDecls()
	declared := make([]command.PluginShape, 0, len(decls))
	for i := range decls {
		if decls[i].Shape == "" {
			continue
		}
		shape, known := command.ParseAnswerShape(decls[i].Shape)
		require.True(t, known, "%s declares shape %q, which Stage 1 refuses", decls[i].Name, decls[i].Shape)
		declared = append(declared, command.PluginShape{
			Command:       decls[i].Name,
			Shape:         shape,
			Columns:       command.ColumnOrder(decls[i].Columns),
			AddressFields: decls[i].AddressFields,
		})
	}
	require.NoError(t, command.RegisterPluginShapes(pluginShapeOwner, declared))
	t.Cleanup(func() { command.UnregisterPluginShapes(pluginShapeOwner) })
}

const pluginShapeOwner = "bgp-adj-rib-in-test"

// answerOf runs a chain the way the CLI runs it: the declaration decides
// whether the operator is admitted at all, and the operators then act on the
// JSON the producer wrote.
func answerOf(t *testing.T, r *AdjRIBInManager, chain string) string {
	t.Helper()

	_, format, errMsg := command.ProcessPipesDefaultFormatChecked(chain, "")
	require.Empty(t, errMsg, "%s was refused", chain)

	payload, err := json.Marshal(r.show("*"))
	require.NoError(t, err)
	return format(string(payload))
}

// TestShowAnswersRowsKeyedByPeer proves `show bgp adj-rib-in | first 1` answers
// ONE peer's routes, under the peer address the answer already carries.
//
// VALIDATES: spec-plugin-declares-answer-shape AC-16.
// PREVENTS: the row set being read as the envelope. rowSet
// (internal/component/command/answer_shape.go) used to read a map as rows only
// when every value was an OBJECT, so the peer map was refused and the one
// candidate left was the envelope: a single row named "adj-rib-in" holding
// every peer. Over that, `| first 1` answered the whole table and `| count`
// answered 1, which is a plausible number and the wrong question.
//
// The chain runs through ProcessPipesDefaultFormatChecked, so the `map`
// declaration is what admits the operator and a `doc` declaration fails this
// test before any payload is looked at.
func TestShowAnswersRowsKeyedByPeer(t *testing.T) {
	declareShapes(t)
	r := twoPeerManager(t)

	answer := answerOf(t, r, "show bgp adj-rib-in | first 1 | json")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(answer), &decoded), "the answer does not parse: %s", answer)
	peers, isMap := decoded["adj-rib-in"].(map[string]any)
	require.True(t, isMap, "the peers are no longer keyed by address: %s", answer)
	require.Len(t, peers, 1, "`| first 1` kept %d peers: %s", len(peers), answer)

	routes, isList := peers[firstPeer].([]any)
	require.True(t, isList, "`| first 1` kept the wrong peer, or changed what the row holds: %s", answer)
	assert.Len(t, routes, 1, "the kept peer holds %d routes, want its own 1: %s", len(routes), answer)
	assert.NotContains(t, answer, secondPeer, "`| first 1` kept the second peer too: %s", answer)
	assert.NotContains(t, answer, secondPrefix, "`| first 1` kept the second peer's routes: %s", answer)
}

// TestShowCountsThePeers proves `| count` answers the number of rows rather
// than the number 1 the envelope reading produced.
//
// VALIDATES: spec-plugin-declares-answer-shape AC-16, from the other side.
// PREVENTS: an operator passing the payload through instead of counting it. No
// route in the fixture holds the digit 2, so an answer that echoed the rows
// fails the first check and an answer that counted the envelope fails the
// second.
func TestShowCountsThePeers(t *testing.T) {
	declareShapes(t)
	r := twoPeerManager(t)

	answer := answerOf(t, r, "show bgp adj-rib-in | count | json")

	assert.Contains(t, answer, "2", "`| count` over two peers = %s", answer)
	assert.NotContains(t, answer, firstPrefix, "`| count` answered the rows rather than their number: %s", answer)
}

// TestShowRendersInEveryFormat proves the same payload reaches an operator
// through each of the three renderings, which is what `ai/rules/cli.md`
// requires of a command answering structured data.
//
// VALIDATES: `| json`, `| yaml` and `| table` each render `show bgp adj-rib-in`.
// PREVENTS: a shape declaration that admits an operator on one rendering and
// answers nothing on another.
func TestShowRendersInEveryFormat(t *testing.T) {
	declareShapes(t)
	r := twoPeerManager(t)

	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			answer := answerOf(t, r, "show bgp adj-rib-in | "+format)

			for _, held := range []string{firstPeer, secondPeer, firstPrefix, secondPrefix} {
				assert.Contains(t, answer, held, "`| %s` does not carry %s: %s", format, held, answer)
			}
		})
	}
}

// TestStatusHoldsNoRowSet proves the `doc` declaration on `show bgp adj-rib-in
// status` describes the answer rather than hiding rows from an operator.
//
// VALIDATES: A-4 for `show bgp adj-rib-in status`.
// PREVENTS: declaring doc over an answer that does hold one row set, which
// would refuse `| count` and `| first` on a command that can serve them. The
// "peers" key maps an address to a route COUNT, and rowSet reads a map as rows
// only when every value is an object, so a scalar leaves this answer one
// document.
func TestStatusHoldsNoRowSet(t *testing.T) {
	r := populatedManager(t)

	envelope, isEnvelope := r.status().(map[string]any)
	require.True(t, isEnvelope, "`show bgp adj-rib-in status` answers an envelope")

	peers, isPeerMap := envelope["peers"].(map[string]int)
	require.True(t, isPeerMap, "the envelope carries its peers under \"peers\"")
	require.NotEmpty(t, peers, "the fixture produced no peer")

	assert.Equal(t, true, envelope["running"], "\"running\" holds a scalar")
	assert.NotNil(t, envelope["total-routes"], "\"total-routes\" holds a scalar")
}

// TestShowMatchesInsideARow proves a row that is a LIST is still searchable:
// `| match` reads the values a row holds, however deep they sit.
//
// VALIDATES: `show bgp adj-rib-in | match <prefix>` answers the peer holding
// that prefix.
// PREVENTS: a row set whose rows no field operator can reach. appendValueText
// (internal/component/command/pipe.go) walks a list as it walks a record, and
// the identity key is matched beside the values, so both the peer address and
// a route inside the row select the row.
func TestShowMatchesInsideARow(t *testing.T) {
	declareShapes(t)
	r := twoPeerManager(t)

	answer := answerOf(t, r, "show bgp adj-rib-in | match "+secondPrefix+" | json")

	assert.Contains(t, answer, secondPeer, "`| match` dropped the peer holding the prefix: %s", answer)
	assert.NotContains(t, answer, firstPeer, "`| match` kept the peer that does not hold it: %s", answer)
}
