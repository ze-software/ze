// Design: docs/architecture/api/commands.md — command argument definitions
// Related: command.go — the YANG lowering this test drives

package yang_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/plugin/all" // registers every compiled-in YANG module
)

// TestKeywordOnlyStatementReachesTheArgumentDefinition proves a leaf's
// ze:keyword-only statement is carried onto its argument definition, and that
// a leaf without the statement is not marked.
//
// GOAL: every content leaf the shipped model declares is keyword-only as the
// validator reads it, and the leaf beside it is not.
// METHOD: lower the registered modules with PathToArgDefs and read
// ArgDef.KeywordOnly for each command that carries a content payload.
// VALIDATES: argDefFor, hasKeywordOnlyExtension, and the statement on the
// two content leaves of the data commands.
// PREVENTS: a payload leaf that a stray positional word can fill, because the
// statement was dropped from the model or never lowered.
func TestKeywordOnlyStatementReachesTheArgumentDefinition(t *testing.T) {
	loader, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader: %v", err)
	}
	defs := yang.PathToArgDefs(loader)
	cases := []struct {
		path    string
		payload string
		plain   string
	}{
		{"update data key", "content", "key"},
		{"update data config", "content", "name"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			seen := 0
			for _, def := range defs[tc.path] {
				if def.Name() == tc.payload {
					seen++
					if !def.KeywordOnly() {
						t.Errorf("%s %s is not keyword-only", tc.path, tc.payload)
					}
				}
				if def.Name() == tc.plain {
					seen++
					if def.KeywordOnly() {
						t.Errorf("%s %s is keyword-only, and its model does not say so", tc.path, tc.plain)
					}
				}
			}
			if seen != 2 {
				t.Fatalf("%s declares %d of the leaves %s and %s, so this test proves nothing", tc.path, seen, tc.payload, tc.plain)
			}
		})
	}
}
