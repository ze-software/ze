package registry

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFilterInsertionRespectsDeclaredDirection exercises the Stage 1 insertion
// guard with exact and pattern names, including an exact declaration shadowing
// a broader pattern. Matching and both-direction filters remain insertable.
func TestFilterInsertionRespectsDeclaredDirection(t *testing.T) {
	for _, tc := range []struct {
		name      string
		filters   []rpc.FilterDecl
		filter    string
		direction rpc.FilterDirection
		refused   bool
	}{
		{"import on export", []rpc.FilterDecl{{Name: "f", Direction: rpc.FilterImport}}, "f", rpc.FilterExport, true},
		{"export on import", []rpc.FilterDecl{{Name: "f", Direction: rpc.FilterExport}}, "f", rpc.FilterImport, true},
		{"pattern import on export", []rpc.FilterDecl{{NamePattern: "[0-9]+", Direction: rpc.FilterImport}}, "65010", rpc.FilterExport, true},
		{"exact wins", []rpc.FilterDecl{{NamePattern: ".+", Direction: rpc.FilterBoth}, {Name: "f", Direction: rpc.FilterImport}}, "f", rpc.FilterExport, true},
		{"matching import", []rpc.FilterDecl{{Name: "f", Direction: rpc.FilterImport}}, "f", rpc.FilterImport, false},
		{"matching export", []rpc.FilterDecl{{Name: "f", Direction: rpc.FilterExport}}, "f", rpc.FilterExport, false},
		{"both import", []rpc.FilterDecl{{NamePattern: "[0-9]+", Direction: rpc.FilterBoth}}, "65010", rpc.FilterImport, false},
		{"both export", []rpc.FilterDecl{{NamePattern: "[0-9]+", Direction: rpc.FilterBoth}}, "65010", rpc.FilterExport, false},
		{"unspecified unchanged", []rpc.FilterDecl{{Name: "f"}}, "f", rpc.FilterExport, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckFilterInsertions("ext", tc.filters, nil, []rpc.FilterInsertion{{
				Filter: "ext:" + tc.filter, Direction: tc.direction, Config: []string{"role"},
			}})
			if tc.refused {
				if err == nil {
					t.Fatal("an insertion opposite to its declared direction was accepted")
				}
				for _, want := range []string{"ext:" + tc.filter, "direction", tc.direction.String()} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal %q does not name %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("compatible insertion refused: %v", err)
			}
		})
	}
}
