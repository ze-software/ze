// Design: docs/architecture/plugin/feature-gates.md -- current issue priority.
package repoinventory

import (
	"errors"
	"go/ast"
	"strings"
	"testing"
)

// TestExternalTerminalUnknownPreservesRecordedUnsupported analyzes an authored
// linkname call before the ambient unknown arises. It must keep the recorded unsupported
// issue while stopping the later callback, not stop on that earlier failure.
// The bodyless linkname pull exercises analysis policy; Go initialization never runs.
func TestExternalTerminalUnknownPreservesRecordedUnsupported(t *testing.T) {
	effects, file := externalTerminalSelectedSource(t, externalTerminalLoop+externalTerminalPull+`
type Payload struct {
	Labels []string
	Values map[string][]int
}
func delayed(value Payload) {}
`, `
pulled()
fmt.Print(loop{}, delayed)
`)
	// The linkname issue is sited at the pulled declaration's name.
	var pulled *ast.FuncDecl
	for _, declaration := range file.Decls {
		function, found := declaration.(*ast.FuncDecl)
		if found {
			if function.Name.Name == "pulled" {
				if pulled != nil {
					t.Fatal("selected source has more than its authored linkname pull")
				}
				pulled = function
			}
		}
	}
	if pulled == nil {
		t.Fatal("selected source lost the unsupported linkname pull")
	}
	err := effects.solve()
	if effects.amb.unknown != externalTerminalDepthUnknown {
		t.Errorf("analysis stopped at the earlier failure before reaching terminal ambient unknown: %q", effects.amb.unknown)
	}
	if len(effects.admitted) != 0 {
		t.Errorf("terminal refusal admitted external operations: %+v", effects.admitted)
	}
	if err == nil {
		t.Fatal("source with an observed unsupported linkname call and ambient unknown was accepted")
	}
	var issue *externalIssue
	if !errors.As(err, &issue) {
		t.Fatalf("terminal refusal lost its private issue classification: %v", err)
	}
	if issue.kind != externalUnsupported {
		t.Errorf("ambient unknown replaced the currently recorded unsupported issue: kind %d, error %v", issue.kind, err)
	}
	if issue.position != pulled.Name.Pos() {
		t.Errorf("recorded issue lost its exact linkname attribution: got %d, want %d", issue.position, pulled.Name.Pos())
	}
	if !strings.Contains(err.Error(), "external linkname authority is unsupported") {
		t.Errorf("recorded unsupported issue lost its linkname cause: %v", err)
	}
	delayed := effects.source.snapshot.packages[externalTerminalDirectory].functions["delayed"].decl
	if delayed == nil {
		t.Fatal("selected source lost the later delayed callback")
	}
	if _, scheduled := effects.bodies[delayed]; scheduled {
		t.Error("terminal unknown still scheduled the later callback body")
	}
	if effects.possible[delayed] {
		t.Error("terminal unknown still marked the later callback possible")
	}
	if len(delayed.Type.Params.List) != 1 {
		t.Fatal("selected callback lost its authored formal")
	}
	field := delayed.Type.Params.List[0]
	if len(field.Names) != 1 {
		t.Fatal("selected callback lost its named formal")
	}
	formal := field.Names[0].Obj
	if formal == nil {
		t.Fatal("selected callback lost its exact formal declaration")
	}
	if effects.formalExternal[formal] {
		t.Error("terminal unknown still granted external authority to the later formal")
	}
	if _, bound := effects.bindings[formal]; bound {
		t.Error("terminal unknown still expanded the later callback's parameter binding")
	}
}
