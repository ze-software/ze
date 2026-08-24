// Design: plan/spec-plugin-declares-answer-shape.md -- the healthcheck half of Phase 4
// Related: healthcheck.go -- commandDecls, the declaration under test
package healthcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// wantShapes is the shape each healthcheck command owes, read from the function
// that PRODUCES its answer. The value is the wire spelling the engine parses at
// Stage 1, and an empty one means the command declares no shape.
//
// Neither command owes a column order or an address field, so this table
// carries the shape alone and the test below requires the other two lists to be
// empty for every command.
var wantShapes = map[string]string{
	// handleShow: both branches answer rows, and the two branches carry
	// different row fields, so no one column order can be read against both.
	"show bgp healthcheck": "map",
	// handleReset answers a report of what it did rather than a data set.
	"clear bgp healthcheck": "",
}

// declByName indexes a declaration list by command path, and fails when the
// list and the expectation table name different commands. A command added to
// one and not the other is the failure this catches.
func declByName(t *testing.T, decls []sdk.CommandDecl, want map[string]string) map[string]sdk.CommandDecl {
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

// TestCommandDeclsDeclareTheAnswerShape holds every healthcheck command to
// declaring what its answer holds.
//
// VALIDATES: AC-17 for `show bgp healthcheck`.
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
			assert.Equal(t, want, decl.Shape, "%s declares shape %q", name, decl.Shape)
			assert.Empty(t, decl.Columns, "%s declares columns %v", name, decl.Columns)
			assert.Empty(t, decl.AddressFields, "%s declares address fields %v", name, decl.AddressFields)
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
// running probes at all.
func TestDeclaredShapeSpellingIsOneTheEngineParses(t *testing.T) {
	for _, decl := range commandDecls() {
		switch decl.Shape {
		case "", "doc", "map", "tab":
		default:
			t.Errorf("%s declares shape %q, which is not doc, map or tab", decl.Name, decl.Shape)
		}
	}
}

// TestShowBranchesCarryDifferentRowFields proves `show bgp healthcheck` declares
// "map" and not "tab" for a reason its two producers still hold.
//
// VALIDATES: the shape choice for AC-17, against the branches themselves.
// PREVENTS: the declaration drifting to "tab" once the two branches agree on a
// field set, or staying "map" after they diverge further. ShapeTab means "rows
// read against the column names this command declares"
// (internal/component/command/pipe_catalog.go), and one column order cannot be
// read against two different field sets.
func TestShowBranchesCarryDifferentRowFields(t *testing.T) {
	mgr := newTestManager()
	mgr.probes["dns"] = &runningProbe{config: ProbeConfig{Name: "dns", Group: "hc"}}

	listFields := keysOf(showRows(t, mgr, nil)[0])
	namedFields := keysOf(showRows(t, mgr, []string{"dns"})[0])

	assert.NotEqual(t, listFields, namedFields,
		"the two branches carry different row fields, which is why the shape is map")
	require.Subset(t, namedFields, listFields,
		"the named-probe row carries every field the list row carries")
}
