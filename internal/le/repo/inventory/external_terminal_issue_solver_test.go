// Design: docs/architecture/plugin/feature-gates.md -- current issue priority.
package repoinventory

import (
	"errors"
	"go/ast"
	"strings"
	"testing"
)

// TestExternalTerminalUnknownPreservesRecordedUnsupported analyzes an authored
// send before the ambient unknown arises. It must keep the recorded unsupported
// issue while stopping the later callback, not stop on that earlier failure.
// The nil-channel send exercises analysis policy; Go initialization never runs.
func TestExternalTerminalUnknownPreservesRecordedUnsupported(t *testing.T) {
	effects, file := externalTerminalSelectedSource(t, `
var channel chan int
type Payload struct {
	Labels []string
	Values map[string][]int
}
func delayed(value Payload) {}
`, `
channel <- 1
fmt.Print(make(chan int), delayed)
`)
	var send *ast.SendStmt
	ast.Inspect(file, func(node ast.Node) bool {
		statement, found := node.(*ast.SendStmt)
		if found {
			if send != nil {
				t.Fatal("selected source has more than its authored send")
			}
			send = statement
		}
		return true
	})
	if send == nil {
		t.Fatal("selected source lost the unsupported send")
	}
	err := effects.solve()
	if effects.amb.unknown != externalTerminalChannelUnknown {
		t.Errorf("analysis stopped at the earlier failure before reaching terminal ambient unknown: %q", effects.amb.unknown)
	}
	if len(effects.admitted) != 0 {
		t.Errorf("terminal refusal admitted external operations: %+v", effects.admitted)
	}
	if err == nil {
		t.Fatal("source with an observed unsupported send and ambient unknown was accepted")
	}
	var issue *externalIssue
	if !errors.As(err, &issue) {
		t.Fatalf("terminal refusal lost its private issue classification: %v", err)
	}
	if issue.kind != externalUnsupported {
		t.Errorf("ambient unknown replaced the currently recorded unsupported issue: kind %d, error %v", issue.kind, err)
	}
	if issue.position != send.Chan.Pos() {
		t.Errorf("recorded issue lost its exact selected send attribution: got %d, want %d", issue.position, send.Chan.Pos())
	}
	if !strings.Contains(err.Error(), "external channel capability transfer is unsupported") {
		t.Errorf("recorded unsupported issue lost its send cause: %v", err)
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
