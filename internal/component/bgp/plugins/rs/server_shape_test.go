// Design: docs/architecture/api/commands.md -- a plugin declares its own answer shape
// Related: server.go -- commandDecls, the declaration under test
package rs

import (
	"net/netip"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// wantShapes is the declaration each rs command owes, read from the function
// that PRODUCES its answer. The producer is named on every row, because that
// function is the only thing that can settle whether a column name exists.
var wantShapes = map[string]wantShape{
	// handleCommand (server_handlers.go): one key holding a scalar, so no rows.
	"show bgp rs status": {shape: "doc"},
	// peerStatus (server_handlers.go), rows under "peers".
	"show bgp rs peers": {
		shape:         "tab",
		columns:       []string{"address", "remote", "up"},
		addressFields: []string{"address"},
	},
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

// TestCommandDeclsDeclareTheAnswerShape holds every rs command to declaring what
// its answer holds.
//
// VALIDATES: AC-17 for the two `show bgp rs` paths.
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

			// ShapeTab means "rows read against the column names this command
			// declares", so a tab declaration with no order names nothing for
			// the renderer to read the rows against. A doc answers one
			// document, so a column order over it would order nothing.
			if decl.Shape == "tab" {
				assert.NotEmpty(t, decl.Columns, "%s declares tab and no column order", name)
			}
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
// serving the route server at all.
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

// TestDeclaredColumnsExistInPayload holds every column and every address field
// this plugin declares to being a key its producer writes.
//
// VALIDATES: AC-17, and the naming half of the Critical Review Checklist.
// PREVENTS: the one failure in this spec with no signal. A declared name the
// payload never carries orders nothing and publishes a field that does not
// exist to `| display <partial>` completion, and the engine cannot check it: it
// never sees the payload the plugin will write. Nothing fails when it is wrong.
//
// The case runs the REAL producer, so the keys compared against are the keys
// peerStatus writes rather than a fixture kept in step by hand.
func TestDeclaredColumnsExistInPayload(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.peers["192.0.2.1"] = &PeerState{Address: "192.0.2.1", ASN: 65001, Up: true}
	rs.peers["2001:db8::1"] = &PeerState{Address: "2001:db8::1", ASN: 65002}

	decl := declByName(t, commandDecls(), wantShapes)["show bgp rs peers"]
	require.NotEmpty(t, decl.Columns, "`show bgp rs peers` declares no column order")

	envelope, isEnvelope := rs.peerStatus().(map[string]any)
	require.True(t, isEnvelope, "peerStatus answers an envelope")
	rows, isRows := envelope["peers"].([]map[string]any)
	require.True(t, isRows, "the envelope carries its rows under \"peers\"")
	require.NotEmpty(t, rows, "the producer wrote no row; the fixture carries none")

	for _, row := range rows {
		for _, column := range decl.Columns {
			_, written := row[column]
			assert.True(t, written, "`show bgp rs peers` declares column %q; the producer writes %v",
				column, sortedRowKeys(row))
		}
		for _, field := range decl.AddressFields {
			assertHoldsAddress(t, field, row)
		}
	}
}

// assertHoldsAddress holds a declared address field to naming a value the
// address operators can act on.
//
// resolveJSON and originJSON (internal/component/command/pipe_resolve.go,
// pipe_origin.go) decorate a string that parses as an address or a prefix, so a
// field holding anything else admits the operators over a value they leave
// untouched.
func assertHoldsAddress(t *testing.T, field string, row map[string]any) {
	t.Helper()

	value, written := row[field]
	if !assert.True(t, written, "`show bgp rs peers` declares address field %q; the producer writes %v",
		field, sortedRowKeys(row)) {
		return
	}
	text, isText := value.(string)
	if !assert.True(t, isText, "`show bgp rs peers` declares address field %q and the producer writes %T",
		field, value) {
		return
	}
	_, err := netip.ParseAddr(text)
	assert.NoError(t, err, "`show bgp rs peers` declares address field %q and the producer writes %q",
		field, text)
}

// sortedRowKeys answers a row's keys in one order, so a failure reads the same
// on every run.
func sortedRowKeys(row map[string]any) string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, " ")
}

// TestStatusHoldsNoRowSet proves the `doc` declaration describes the answer
// rather than hiding rows from an operator.
//
// VALIDATES: A-4 for `show bgp rs status`.
// PREVENTS: declaring doc over an answer that does hold one row set, which
// would refuse `| count` and `| first` on a command that can serve them.
//
// rowSet (internal/component/command/answer_shape.go) reads an array as rows,
// and a map as rows only when every value is an object. This answer is one key
// holding a bool, so neither test can find rows in it, at either level.
func TestStatusHoldsNoRowSet(t *testing.T) {
	rs := newTestRouteServer(t)

	status, answer, err := rs.handleCommand("show bgp rs status")
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	envelope, isEnvelope := answer.(map[string]any)
	require.True(t, isEnvelope, "`show bgp rs status` answers an envelope")
	require.Equal(t, map[string]any{"running": true}, envelope,
		"one key holding a scalar, which rowSet reads as rows at no level")
}
