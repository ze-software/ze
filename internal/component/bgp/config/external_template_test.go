package bgpconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestExternalIRRTemplateStage1AndResolution uses the real IRR registration,
// not a second ASN pattern. The dynamic template waits for Stage 1, then valid
// OPEN substitution compiles while an out-of-range ASN still fails closed.
func TestExternalIRRTemplateStage1AndResolution(t *testing.T) {
	clearPending(t)
	reg := registry.Lookup("bgp-filter-irr")
	if reg == nil {
		t.Fatal("the compiled IRR registration is absent")
	}
	groups, err := loadTemplated(t, "bgp-filter-irr:$remote_as", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want one", len(groups))
	}
	if err := registry.CheckFilterDeclarations(reg.Name, reg.Filters); err != nil {
		t.Fatalf("the documented IRR template was refused at Stage 1: %v", err)
	}
	declareRuntime(t, reg.Name, reg.Filters)
	name, err := resolvedImport(t, groups[0], "65010")
	if err != nil {
		t.Fatalf("valid peer ASN: %v", err)
	}
	if !strings.Contains(name, "bgp-filter-irr:65010") {
		t.Fatalf("resolved members = %q", name)
	}
	_, err = resolvedImport(t, groups[0], "4294967296")
	requireRefusal(t, err, "bgp-filter-irr:4294967296", "declares no filter")
	if _, err := loadTemplated(t, "bgp-filter-irr:$remote_asn", ""); err == nil {
		t.Fatal("an unknown variable was accepted")
	}
}

// TestExternalTemplateStillJudgesDeclaration drives pending references and
// reloads with generic exact and patterned declarations. Exact names must fit
// the template; every candidate retains the field and direction guards.
func TestExternalTemplateStillJudgesDeclaration(t *testing.T) {
	for _, tc := range []struct {
		name, reference string
		filter          rpc.FilterDecl
		want            string
	}{
		{"numeric pattern", "ext:$remote_as", rpc.FilterDecl{NamePattern: "[1-9][0-9]*", Direction: rpc.FilterImport, Reads: []string{"prefix"}}, ""},
		{"exact name", "ext:$remote_as", rpc.FilterDecl{Name: "65010", Direction: rpc.FilterImport, Reads: []string{"prefix"}}, ""},
		{"prefixed pattern", "ext:asn-$remote_as", rpc.FilterDecl{NamePattern: "asn-65[0-9]+", Direction: rpc.FilterBoth, Reads: []string{"prefix"}}, ""},
		{"no exact overlap", "ext:$remote_as", rpc.FilterDecl{Name: "status", Direction: rpc.FilterImport, Reads: []string{"prefix"}}, "declares no filter"},
		{"wrong direction", "ext:$remote_as", rpc.FilterDecl{NamePattern: "[0-9]+", Direction: rpc.FilterExport, Reads: []string{"prefix"}}, "direction"},
		{"wrong field", "ext:$remote_as", rpc.FilterDecl{NamePattern: "[0-9]+", Direction: rpc.FilterImport, Reads: []string{"tag"}}, "tag"},
		{"wrong set", "ext:$remote_as", rpc.FilterDecl{NamePattern: "[0-9]+", Direction: rpc.FilterImport, Reads: []string{"prefix"}, Sets: []string{"prefix"}}, "sets prefix"},
	} {
		for _, runtime := range []bool{false, true} {
			name := tc.name
			if runtime {
				name += "/reload"
			}
			t.Run(name, func(t *testing.T) {
				clearPending(t)
				filters := []rpc.FilterDecl{tc.filter}
				if runtime {
					declareRuntime(t, "ext", filters)
				}
				_, err := loadTemplated(t, tc.reference, "")
				if !runtime {
					if err != nil {
						t.Fatalf("pending template: %v", err)
					}
					err = registry.CheckFilterDeclarations("ext", filters)
				}
				if tc.want == "" {
					if err != nil {
						t.Fatalf("compatible template refused: %v", err)
					}
					return
				}
				requireRefusal(t, err, tc.reference, tc.want)
				if !runtime && !errors.Is(err, registry.ErrLoadedChainRefused) {
					t.Fatalf("unmarked Stage 1 refusal: %v", err)
				}
			})
		}
	}
}

// TestExternalTemplateResolvedDeclarationGuards proves that deferring a name
// until OPEN never bypasses its actual declaration. A compatible template
// loads first; each runtime declaration then refuses the concrete peer name
// through the same checks a static chain receives.
func TestExternalTemplateResolvedDeclarationGuards(t *testing.T) {
	for _, tc := range []struct {
		name    string
		filters []rpc.FilterDecl
		want    string
	}{
		{"name", []rpc.FilterDecl{{NamePattern: "66[0-9]+", Direction: rpc.FilterImport, Reads: []string{"prefix"}}}, "declares no filter"},
		{"direction", []rpc.FilterDecl{{Name: "65010", Direction: rpc.FilterExport, Reads: []string{"prefix"}}}, "direction"},
		{"read", []rpc.FilterDecl{{Name: "65010", Direction: rpc.FilterImport, Reads: []string{"tag"}}}, "tag"},
		{"set", []rpc.FilterDecl{{Name: "65010", Direction: rpc.FilterImport, Reads: []string{"prefix"}, Sets: []string{"prefix"}}}, "sets prefix"},
		{"exact wins", []rpc.FilterDecl{{NamePattern: "[0-9]+", Direction: rpc.FilterBoth, Reads: []string{"prefix"}}, {Name: "65010", Direction: rpc.FilterExport, Reads: []string{"prefix"}}}, "direction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearPending(t)
			groups, err := loadTemplated(t, "ext:$remote_as", "")
			if err != nil {
				t.Fatal(err)
			}
			if err := registry.CheckFilterDeclarations("ext", []rpc.FilterDecl{{NamePattern: "[0-9]+", Direction: rpc.FilterImport, Reads: []string{"prefix"}}}); err != nil {
				t.Fatalf("compatible pending template: %v", err)
			}
			declareRuntime(t, "ext", tc.filters)
			_, err = resolvedImport(t, groups[0], "65010")
			requireRefusal(t, err, "ext:65010", tc.want)
		})
	}
}

// TestExternalTemplatePatternNameJudgedAfterResolution keeps the name-pattern
// mismatch at the boundary that has the concrete name. Stage 1 judges the
// declaration's fields and direction, not intersection of two regex languages.
func TestExternalTemplatePatternNameJudgedAfterResolution(t *testing.T) {
	for _, runtime := range []bool{false, true} {
		name := "pending"
		if runtime {
			name = "reload"
		}
		t.Run(name, func(t *testing.T) {
			clearPending(t)
			filters := []rpc.FilterDecl{{NamePattern: "[0-9]+", Direction: rpc.FilterImport, Reads: []string{"prefix"}}}
			if runtime {
				declareRuntime(t, "ext", filters)
			}
			groups, err := loadTemplated(t, "ext:asn-$remote_as", "")
			if err != nil {
				t.Fatalf("template load: %v", err)
			}
			if !runtime {
				if err := registry.CheckFilterDeclarations("ext", filters); err != nil {
					t.Fatalf("Stage 1 template: %v", err)
				}
				declareRuntime(t, "ext", filters)
			}
			_, err = resolvedImport(t, groups[0], "65010")
			requireRefusal(t, err, "ext:asn-65010", "declares no filter")
		})
	}
}
