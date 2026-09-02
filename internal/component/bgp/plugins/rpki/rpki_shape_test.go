// Design: docs/architecture/api/commands.md -- a plugin declares its own answer shape
package rpki

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

// wantShapes is the declaration each rpki command owes, read from the function
// that PRODUCES its answer. The producer is named on every row, because that
// function is the only thing that can settle whether a column name exists.
var wantShapes = map[string]wantShape{
	// overviewCommand, through appendCacheServers.
	"show bgp rpki": {
		shape:         "tab",
		columns:       []string{"address", "port", "state", "synced", "version"},
		addressFields: []string{"address"},
	},
	// statusCommand: two candidate row sets, so no single one.
	"show bgp rpki status": {shape: "doc"},
	// cacheCommand.
	"show bgp rpki cache": {
		shape: "tab",
		columns: []string{
			"address", "port", "preference", "state", "synced", "version",
			"session-id", "serial", "refresh-interval", "retry-interval",
			"expire-interval",
		},
		addressFields: []string{"address"},
	},
	// roaCommand and roaLookupCommand.
	"show bgp rpki roa": {
		shape:         "tab",
		columns:       []string{"prefix", "max-length", "asn"},
		addressFields: []string{"prefix"},
	},
	// summaryCommand: the aggregate keys alone.
	"show bgp rpki summary": {shape: "doc"},
	// aspaCommand, both branches.
	"show bgp rpki aspa": {
		shape:   "tab",
		columns: []string{"customer-asn", "providers"},
	},
	// A verdict for one prefix, outside the population this spec measured.
	"request bgp rpki validate": {},
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

// TestCommandDeclsDeclareTheAnswerShape holds every rpki command to declaring
// what its answer holds.
//
// VALIDATES: AC-17 for the six `show bgp rpki` paths.
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
// PREVENTS: the plugin failing Stage 1 and taking its commands and its
// validation gate down with it. validateShapeDecls refuses a fourth spelling
// and the refusal fails the WHOLE registration, so `"table"` for `"tab"` is a
// typo that stops the daemon serving RPKI at all.
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

// populatedPlugin answers a plugin whose every declared column has a value to
// carry: two cache servers, VRPs in both families, and an ASPA record.
//
// The lookup branches need data they can find, so the VRP and the ASPA record
// below are what the lookup cases in TestDeclaredColumnsExistInPayload ask for.
func populatedPlugin() *rPKIPlugin {
	rp := newTestPlugin()
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65001))
	rp.cache.Add(makeVRP("2001:db8::/32", 48, 65002))
	rp.aspaCache.Set(65001, []uint32{65000, 65010})
	rp.aspaEnabled.Store(true)
	rp.sessions = append(rp.sessions,
		newRTRSession("192.0.2.1", 323, 100, "", rp.cache, rp.aspaCache, rp.stopCh),
		newRTRSession("2001:db8::1", 324, 50, "", rp.cache, rp.aspaCache, rp.stopCh),
	)
	return rp
}

// rowsUnder answers the rows a producer wrote under an envelope key, and fails
// when the key holds no row.
//
// An empty row set would make every column assertion below vacuously true, so
// the emptiness is the failure rather than a skip.
func rowsUnder(t *testing.T, data any, key string) []map[string]any {
	t.Helper()

	entries := jsonArray(t, parseJSON(t, data), key)
	require.NotEmpty(t, entries, "the producer wrote no row under %q; the fixture carries none", key)

	rows := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, jsonObject(t, entry))
	}
	return rows
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
// Each case runs the REAL producer, so the keys compared against are the keys
// the producing function writes rather than a fixture kept in step by hand. A
// command whose argument selects a branch is driven through BOTH branches,
// because one declaration has to describe each of them.
func TestDeclaredColumnsExistInPayload(t *testing.T) {
	cases := []struct {
		name    string
		command string
		rows    func(*testing.T, *rPKIPlugin) []map[string]any
	}{
		{"show bgp rpki", "show bgp rpki", func(t *testing.T, rp *rPKIPlugin) []map[string]any {
			_, data, err := rp.overviewCommand()
			require.NoError(t, err)
			return rowsUnder(t, data, "cache-servers")
		}},
		{"show bgp rpki cache", "show bgp rpki cache", func(t *testing.T, rp *rPKIPlugin) []map[string]any {
			_, data, err := rp.cacheCommand()
			require.NoError(t, err)
			return rowsUnder(t, data, "cache-servers")
		}},
		{"show bgp rpki roa", "show bgp rpki roa", func(t *testing.T, rp *rPKIPlugin) []map[string]any {
			_, data, err := rp.roaCommand(nil)
			require.NoError(t, err)
			return rowsUnder(t, data, "entries")
		}},
		{"show bgp rpki roa <prefix>", "show bgp rpki roa", func(t *testing.T, rp *rPKIPlugin) []map[string]any {
			_, data, err := rp.roaCommand([]string{"10.0.0.0/8"})
			require.NoError(t, err)
			return rowsUnder(t, data, "entries")
		}},
		{"show bgp rpki aspa", "show bgp rpki aspa", func(t *testing.T, rp *rPKIPlugin) []map[string]any {
			_, data, err := rp.aspaCommand(nil)
			require.NoError(t, err)
			return rowsUnder(t, data, "entries")
		}},
		{"show bgp rpki aspa <customer-asn>", "show bgp rpki aspa", func(t *testing.T, rp *rPKIPlugin) []map[string]any {
			_, data, err := rp.aspaCommand([]string{"65001"})
			require.NoError(t, err)
			return rowsUnder(t, data, "entries")
		}},
	}

	byName := declByName(t, commandDecls(), wantShapes)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decl := byName[tc.command]
			require.NotEmpty(t, decl.Columns, "%s declares no column order", tc.command)

			rows := tc.rows(t, populatedPlugin())
			for _, row := range rows {
				for _, column := range decl.Columns {
					_, written := row[column]
					assert.True(t, written, "%s declares column %q; the producer writes %v",
						tc.command, column, sortedRowKeys(row))
				}
				for _, field := range decl.AddressFields {
					assertHoldsAddress(t, tc.command, field, row)
				}
			}
		})
	}
}

// assertHoldsAddress holds a declared address field to naming a value the
// address operators can act on.
//
// resolveJSON and originJSON (internal/component/command/pipe_resolve.go,
// pipe_origin.go) decorate a string that parses as an address or a prefix, so a
// field holding anything else admits the operators over a value they leave
// untouched.
func assertHoldsAddress(t *testing.T, command, field string, row map[string]any) {
	t.Helper()

	value, written := row[field]
	if !assert.True(t, written, "%s declares address field %q; the producer writes %v",
		command, field, sortedRowKeys(row)) {
		return
	}
	text, isText := value.(string)
	if !assert.True(t, isText, "%s declares address field %q and the producer writes %T",
		command, field, value) {
		return
	}
	_, addrErr := netip.ParseAddr(text)
	_, prefixErr := netip.ParsePrefix(text)
	assert.True(t, addrErr == nil || prefixErr == nil,
		"%s declares address field %q and the producer writes %q, which is neither an address nor a prefix",
		command, field, text)
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

// TestDocCommandsHoldNoSingleRowSet proves the two `doc` declarations describe
// the answers rather than hiding rows from an operator.
//
// VALIDATES: A-4 for `show bgp rpki status`.
// PREVENTS: declaring doc over an answer that does hold one row set, which
// would refuse `| count` and `| first` on a command that can serve them.
//
// candidateRowSets mirrors the test rowsInKeyed
// (internal/component/command/answer_shape.go) applies: an envelope key whose
// value is an array, or a map whose every value is an object, is a candidate
// row set. Exactly one candidate is what makes an answer rows; two candidates
// and zero candidates are both one document, for opposite reasons.
func TestDocCommandsHoldNoSingleRowSet(t *testing.T) {
	rp := populatedPlugin()

	_, status, err := rp.statusCommand()
	require.NoError(t, err)
	assert.Greater(t, len(candidateRowSets(parseJSON(t, status))), 1,
		"`show bgp rpki status` declares doc because it holds more than one candidate row set")

	_, summary, err := rp.summaryCommand()
	require.NoError(t, err)
	assert.Empty(t, candidateRowSets(parseJSON(t, summary)),
		"`show bgp rpki summary` declares doc because it holds no row set")
}

// candidateRowSets answers the envelope keys holding something rowsInKeyed
// would read as a row set.
func candidateRowSets(answer map[string]any) []string {
	var keys []string
	for name, value := range answer {
		if isRowSet(value) {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	return keys
}

// isRowSet mirrors rowSet (internal/component/command/answer_shape.go): an
// array carries rows, and so does a non-empty map whose every value is an
// object.
func isRowSet(value any) bool {
	switch typed := value.(type) {
	case []any:
		return true
	case map[string]any:
		if len(typed) == 0 {
			return false
		}
		for _, entry := range typed {
			if _, isObject := entry.(map[string]any); !isObject {
				return false
			}
		}
		return true
	}
	return false
}
