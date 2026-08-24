// Design: plan/spec-plugin-declares-answer-shape.md -- the adj-rib-in half of Phase 4
// Related: rib.go -- commandDecls, the declaration under test
package adj_rib_in

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
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
	// show (rib_commands.go): "adj-rib-in" maps an address to an ARRAY, which is
	// no row set either, so the only candidate left is the envelope itself.
	"show bgp adj-rib-in": {shape: "doc"},
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
	for _, decl := range decls {
		require.NotContains(t, byName, decl.Name, "the plugin declares %q twice", decl.Name)
		byName[decl.Name] = decl
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

// TestShowHoldsNoRowSetTheEngineCanAddress proves the `doc` declaration on
// `show bgp adj-rib-in` describes the answer rather than hiding rows from an
// operator.
//
// VALIDATES: the `doc` declaration for `show bgp adj-rib-in`, and it is the
// evidence against AC-16 as that criterion is worded.
// PREVENTS: declaring a row shape over an answer whose rows the engine cannot
// reach. rowSet (internal/component/command/answer_shape.go) reads a map as
// rows only when every value is an OBJECT, and the peer map's values are
// arrays. So the one candidate row set left is the envelope itself: a single
// row named "adj-rib-in" carrying every peer. Over that, `| first 1` answers
// the whole table and `| count` answers 1.
func TestShowHoldsNoRowSetTheEngineCanAddress(t *testing.T) {
	r := populatedManager(t)

	envelope, isEnvelope := r.show("*").(map[string]any)
	require.True(t, isEnvelope, "`show bgp adj-rib-in` answers an envelope")
	require.Len(t, envelope, 1, "the envelope carries one key")

	peers, isPeerMap := envelope["adj-rib-in"].(map[string][]map[string]any)
	require.True(t, isPeerMap, "the envelope carries its peers under \"adj-rib-in\"")
	require.NotEmpty(t, peers, "the fixture produced no peer")

	for peer, routes := range peers {
		assert.NotEmpty(t, routes, "peer %s carries an array of routes, which rowSet does not read as a row", peer)
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
