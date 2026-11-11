// Design: docs/contributing/ze-go-style.md -- failures have explicit outcomes.
// Related: reactor_notify.go, forward_build.go -- the owned BGP producers.
package archrefusebydefault

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestOwnedBGPRefusalProducersReportExplicitOutcomes checks both populations:
// an annotated sentinel is still a shape in the tree report. The actual
// producer declarations must exist, so renaming or removing one cannot make
// the check pass. Callback behavior is covered in the reactor package.
func TestOwnedBGPRefusalProducersReportExplicitOutcomes(t *testing.T) {
	// These existing bad fixtures prove that both relevant rules still fire.
	for _, control := range []struct {
		file string
		rule Rule
	}{
		{file: "dropped.go", rule: RuleErrorDroppedBelowWarn},
		{file: "miss.go", rule: RuleMissAnswered},
	} {
		hits, _, err := ScanFile(token.NewFileSet(), filepath.Join("testdata", control.file))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, hit := range hits {
			if hit.Rule == control.rule {
				found = true
			}
		}
		if !found {
			t.Fatalf("control %s produced no %s finding", control.file, control.rule)
		}
	}
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	for _, producer := range []struct {
		file string
		name string
	}{
		{file: "internal/component/bgp/reactor/reactor_notify.go", name: "Reactor.notifyOneMessage"},
		{file: "internal/component/bgp/reactor/forward_build.go", name: "buildWithdrawalPayload"},
		{file: "internal/component/bgp/reactor/forward_build.go", name: "fwdWithdrawal.of"},
	} {
		t.Run(producer.name, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(producer.file))
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			declared := false
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if ok && function.Body != nil && funcName(function) == producer.name {
					declared = true
				}
			}
			if !declared {
				t.Fatalf("owned producer %s no longer exists in %s", producer.name, producer.file)
			}
			hits, _, err := ScanFile(token.NewFileSet(), path)
			if err != nil {
				t.Fatal(err)
			}
			for _, hit := range hits {
				if hit.Fn == producer.name {
					t.Errorf("owned producer still reports %s: %s", hit.Rule, hit.Detail)
				}
			}
		})
	}
}
