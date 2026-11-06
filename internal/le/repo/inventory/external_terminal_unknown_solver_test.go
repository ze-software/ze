// Design: docs/architecture/plugin/feature-gates.md -- permanent ambient refusal.
package repoinventory

import (
	"errors"
	"go/ast"
	"go/token"
	"strings"
	"testing"
)

const externalTerminalDirectory = "internal/terminalproof"
const externalTerminalFilename = externalTerminalDirectory + "/proof.go"
const externalTerminalChannelUnknown = "channel element capability transfer is unsupported"

// TestExternalTerminalUnknownProvisionalFailureSurvives keeps a returned
// callable's first empty summary from becoming a permanent refusal. The package
// initializer demands factory's result before any scheduled body has run.
func TestExternalTerminalUnknownProvisionalFailureSurvives(t *testing.T) {
	effects, file := externalTerminalSelectedSource(t, `
func factory() func() int { return func() int { return 1 } }
var result = factory()()
`, "fmt.Print(result)")
	if err := effects.solve(); err != nil {
		t.Fatalf("a provisional returned-callable failure became terminal: %v", err)
	}
	externalTerminalAccepted(t, effects, file)
	factory := effects.source.snapshot.packages[externalTerminalDirectory].functions["factory"].decl
	if factory == nil {
		t.Fatal("selected source lost the returned-callable factory")
	}
	if len(effects.returns[factory].callables) != 1 {
		t.Fatalf("accepted control lost the factory's returned callable: %+v", effects.returns[factory])
	}
}

// TestExternalTerminalUnknownLocalProjectionSurvives builds a holder with a
// channel capability, then exposes only its exact scalar member. A local type
// unknown must not cancel analysis before that projection removes the sibling.
func TestExternalTerminalUnknownLocalProjectionSurvives(t *testing.T) {
	effects, file := externalTerminalSelectedSource(t, "", `
fmt.Print((struct { Blocked chan int; Label string }{Label: "scalar copy"}).Label)
`)
	if err := effects.solve(); err != nil {
		t.Fatalf("a local unknown contaminated the exposed scalar projection: %v", err)
	}
	externalTerminalAccepted(t, effects, file)
}

// TestExternalTerminalUnknownStopsCallbackExpansion obtains terminal unknown
// from a selected channel construction in the same call that exposes delayed.
// The actual delayed declaration and formal must remain unscheduled; observing
// those identities proves skipped expansion without a timing threshold.
func TestExternalTerminalUnknownStopsCallbackExpansion(t *testing.T) {
	effects, _ := externalTerminalSelectedSource(t, `
type Payload struct {
	Labels []string
	Values map[string][]int
}
func delayed(value Payload) {}
`, "fmt.Print(make(chan int), delayed)")
	err := effects.solve()
	externalTerminalRefused(t, effects, err)
	delayed := effects.source.snapshot.packages[externalTerminalDirectory].functions["delayed"].decl
	if delayed == nil {
		t.Fatal("selected source lost the delayed callback")
	}
	if _, scheduled := effects.bodies[delayed]; scheduled {
		t.Error("terminal ambient unknown still scheduled the delayed callback body")
	}
	if effects.possible[delayed] {
		t.Error("terminal ambient unknown still marked the delayed callback possible")
	}
	if len(delayed.Type.Params.List) != 1 {
		t.Fatal("selected delayed callback lost its authored parameter")
	}
	field := delayed.Type.Params.List[0]
	if len(field.Names) != 1 {
		t.Fatal("selected delayed callback lost its named formal")
	}
	formal := field.Names[0].Obj
	if formal == nil {
		t.Fatal("selected delayed callback lost its formal declaration identity")
	}
	if effects.formalExternal[formal] {
		t.Error("terminal ambient unknown still granted external authority to the delayed formal")
	}
	if _, bound := effects.bindings[formal]; bound {
		t.Error("terminal ambient unknown still expanded the delayed parameter type into a binding")
	}
}

// TestExternalTerminalUnknownDiagnosticPrecedesUnvisitedUnsupported pins the
// proposed terminal diagnostic policy. The current exhaustive producer visits
// the closure and prefers its unsupported send. Terminal cancellation must
// instead retain the unresolved ambient refusal and never schedule that body.
// This is a new policy case, not permission to refresh existing expectations.
func TestExternalTerminalUnknownDiagnosticPrecedesUnvisitedUnsupported(t *testing.T) {
	effects, file := externalTerminalSelectedSource(t, "var channel chan int\n", `
fmt.Print(make(chan int), func() { channel <- 1 })
`)
	err := effects.solve()
	externalTerminalRefused(t, effects, err)
	var callback *ast.FuncLit
	ast.Inspect(file, func(node ast.Node) bool {
		literal, found := node.(*ast.FuncLit)
		if found {
			if callback != nil {
				t.Fatal("diagnostic source has more than its authored callback")
			}
			callback = literal
		}
		return true
	})
	if callback == nil {
		t.Fatal("selected diagnostic source lost its callback")
	}
	if _, scheduled := effects.bodies[callback]; scheduled {
		t.Error("terminal ambient refusal still scheduled the unsupported-send callback")
	}
}

// externalTerminalSelectedSource captures real selected syntax and both sink
// import-cycle witnesses through the existing helper and production constructor.
// It never seeds private ambient capabilities, summaries or admission state.
func externalTerminalSelectedSource(t *testing.T, declarations, statements string) (*externalEffects, *ast.File) {
	t.Helper()
	selected := typeIdentitySelectedSource(t, externalTerminalDirectory, map[string]string{
		"go.mod":                                     "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt":                          "",
		registrationPackage + "/register.go":         "package registry\nimport _ \"fmt\"\nfunc Register(label string) {}\n",
		"internal/component/config/yang/register.go": "package yang\nimport _ \"fmt\"\n",
		externalTerminalFilename: `package terminalproof
import (
	"fmt"
	registry "github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/component/config/yang"
)
var label = "retained registration metadata"
` + declarations + `
func init() {
	registry.Register(label)
` + statements + `
}
`,
	})
	source, err := newExternalSource(selected.source.snapshot)
	if err != nil {
		t.Fatal(err)
	}
	effects := newExternalEffects(source)
	file := typeIdentitySelectedFile(t, source.snapshot.packages[externalTerminalDirectory], externalTerminalFilename)
	return effects, file
}

// externalTerminalAccepted requires exact retained metadata and exact external
// site admission, so accepting everything or dropping demanded work cannot make
// either survivor control green.
func externalTerminalAccepted(t *testing.T, effects *externalEffects, file *ast.File) {
	t.Helper()
	if effects.amb.unknown != "" {
		t.Errorf("accepted source retained terminal ambient unknown: %q", effects.amb.unknown)
	}
	if effects.failure != nil {
		t.Errorf("accepted source retained an unresolved failure: %v", effects.failure)
	}
	metadata := file.Scope.Lookup("label")
	if metadata == nil {
		t.Fatal("selected source lost the retained metadata declaration")
	}
	if len(effects.metadata) != 1 {
		t.Errorf("successful control changed the metadata population: %+v", effects.metadata)
	}
	if !effects.metadata[metadata] {
		t.Error("successful control lost its exact retained metadata declaration")
	}
	var external *ast.CallExpr
	ast.Inspect(file, func(node ast.Node) bool {
		call, found := node.(*ast.CallExpr)
		if !found {
			return true
		}
		selector, selected := call.Fun.(*ast.SelectorExpr)
		if !selected {
			return true
		}
		owner, named := selector.X.(*ast.Ident)
		if !named {
			return true
		}
		if owner.Name != "fmt" {
			return true
		}
		if selector.Sel.Name != "Print" {
			return true
		}
		if external != nil {
			t.Fatal("survivor source has more than its authored external site")
		}
		external = call
		return true
	})
	if external == nil {
		t.Fatal("selected survivor source lost its external call")
	}
	site := externalSite(effects.source.snapshot, external)
	if len(effects.operations) != 1 {
		t.Errorf("successful control changed the external operation population: %+v", effects.operations)
	}
	if effects.operations[site].value != external {
		t.Error("successful control lost its exact selected external operation")
	}
	if len(effects.admitted) != 1 {
		t.Errorf("successful control changed the admitted external population: %+v", effects.admitted)
	}
	if !effects.admitted[site] {
		t.Error("successful control did not admit its exact selected external site")
	}
}

// externalTerminalRefused pins the permanent channel fact and the proposed
// source-free unresolved refusal, while requiring that no operation is admitted.
// Source-selected tests do not replace signed public-report publication checks.
func externalTerminalRefused(t *testing.T, effects *externalEffects, err error) {
	t.Helper()
	if effects.amb.unknown != externalTerminalChannelUnknown {
		t.Errorf("negative source did not reach its exact ambient unknown: %q", effects.amb.unknown)
	}
	if len(effects.admitted) != 0 {
		t.Errorf("terminal refusal admitted external operations: %+v", effects.admitted)
	}
	if err == nil {
		t.Error("terminal ambient unknown was accepted")
		return
	}
	var issue *externalIssue
	if !errors.As(err, &issue) {
		t.Errorf("terminal refusal lost its private issue classification: %v", err)
		return
	}
	if issue.kind != externalUnresolved {
		t.Errorf("unvisited effects replaced the terminal unresolved refusal: kind %d, error %v", issue.kind, err)
	}
	if issue.position != token.NoPos {
		t.Errorf("ambient terminal refusal borrowed an execution-source attribution: %d", issue.position)
	}
	if !strings.Contains(err.Error(), "registration capability is unknown: "+externalTerminalChannelUnknown) {
		t.Errorf("terminal refusal lost its exact ambient cause: %v", err)
	}
}
