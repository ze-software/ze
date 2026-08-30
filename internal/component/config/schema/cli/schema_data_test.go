package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// captureStderr runs fn and answers whatever it wrote to os.Stderr.
//
// The refusals below are written for the operator, so the test reads the
// operator's channel rather than the exit code alone. A command that exits 1
// and names nothing refuses, and the operator cannot tell what it refused.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	previous := os.Stderr
	os.Stderr = writer
	fn()
	writer.Close() //nolint:errcheck,gosec // test cleanup
	os.Stderr = previous
	written, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(written)
}

// TestSchemaAnswersRenderThroughThePipeLayer proves each `show schema *`
// command is served in this process and its answer reaches `| json`. That is
// what a registered structured answer buys, and it is what a rendering flag
// cannot: one payload, rendered by whichever operator was typed.
//
// The rows come back as a bare list: the renderer unwraps the single-key
// envelope the handler answers with. The field each row must carry is
// therefore what pins the answer, rather than the envelope key.
func TestSchemaAnswersRenderThroughThePipeLayer(t *testing.T) {
	for _, one := range []struct{ path, field string }{
		{path: "show schema list", field: keyModule},
		{path: "show schema methods", field: keyMethod},
		{path: "show schema events", field: keyMethod},
		{path: "show schema handlers", field: "handler"},
	} {
		t.Run(one.path, func(t *testing.T) {
			answer, code, served := command.ServeLocal(one.path+" | json", "")
			if !served {
				t.Fatalf("%s was not served in this process", one.path)
			}
			if code != 0 {
				t.Fatalf("exit code = %d, want 0 (answer: %q)", code, answer)
			}
			var rows []map[string]any
			if err := json.Unmarshal([]byte(answer), &rows); err != nil {
				t.Fatalf("| json answered something no JSON decoder takes: %v (answer: %q)",
					err, answer)
			}
			if len(rows) == 0 {
				t.Fatalf("| json answered no rows: %q", answer)
			}
			for _, row := range rows {
				if value, _ := row[one.field].(string); value == "" {
					t.Fatalf("a row carries no %s: %v", one.field, row)
				}
			}
		})
	}
}

// TestShowSchemaModuleAnswersOneModule pins the document `show schema module`
// answers: the module's identity and its YANG source.
func TestShowSchemaModuleAnswersOneModule(t *testing.T) {
	payload, code := dataModule([]string{"ze-bgp-conf"})
	if code != 0 {
		t.Fatalf("dataModule exited %d", code)
	}
	document, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("dataModule answers %T, want one document", payload)
	}
	if document[keyModule] != "ze-bgp-conf" {
		t.Errorf("module = %v, want ze-bgp-conf", document[keyModule])
	}
	if document[keyNamespace] == "" || document[keyNamespace] == nil {
		t.Error("the document carries no namespace")
	}
	source, _ := document[keyYANG].(string)
	if !strings.Contains(source, "module ze-bgp-conf") {
		t.Errorf("the document carries no YANG source: %q", source)
	}
}

// TestShowSchemaModuleRendersAsJSON drives the registered path through the pipe
// layer, which is the surface an operator and a tool actually reach.
func TestShowSchemaModuleRendersAsJSON(t *testing.T) {
	answer, code, served := command.ServeLocal("show schema module ze-bgp-conf | json", "")
	if !served {
		t.Fatal("show schema module was not served in this process")
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (answer: %q)", code, answer)
	}
	var document struct {
		Module    string   `json:"module"`
		Namespace string   `json:"namespace"`
		Plugin    string   `json:"plugin"`
		Handlers  []string `json:"handlers"`
		YANG      string   `json:"yang"`
	}
	if err := json.Unmarshal([]byte(answer), &document); err != nil {
		t.Fatalf("| json answered something no JSON decoder takes: %v (answer: %q)", err, answer)
	}
	if document.Module != "ze-bgp-conf" || document.Namespace == "" {
		t.Errorf("the answer lost its identity through | json: %+v", document)
	}
	if !strings.Contains(document.YANG, "module ze-bgp-conf") {
		t.Errorf("the answer lost its YANG source through | json: %q", document.YANG)
	}
}

// TestShowSchemaModuleRefusesRowOperatorsByName proves the declared shape is
// load-bearing. The answer is ONE document, so a row operator is refused by
// name before the command runs.
func TestShowSchemaModuleRefusesRowOperatorsByName(t *testing.T) {
	answer, code, served := command.ServeLocal("show schema module ze-bgp-conf | first 1", "")
	if !served {
		t.Fatal("show schema module was not served in this process")
	}
	if code == 0 {
		t.Fatalf("| first was accepted over one document (answer: %q)", answer)
	}
	if !strings.Contains(answer, "first") {
		t.Errorf("the refusal does not name the operator: %q", answer)
	}
}

// TestShowSchemaModuleRefusesAnythingButOneModuleName covers the guard's own
// entry point at both edges: no module named, and more than one.
func TestShowSchemaModuleRefusesAnythingButOneModuleName(t *testing.T) {
	for _, args := range [][]string{nil, {"ze-bgp-conf", "ze-fib-conf"}, {"no-such-module"}} {
		payload, code := dataModule(args)
		if code != 1 {
			t.Errorf("dataModule(%v) exited %d, want 1", args, code)
		}
		if payload != nil {
			t.Errorf("dataModule(%v) answered %v beside its refusal", args, payload)
		}
	}
}

// TestPrintedSchemaFormsRefuseARenderingOption proves the deleted flag is
// REFUSED rather than dropped in silence. A dropped token would report success
// for a question nobody answered, and the operator would read a table where
// they asked for JSON.
func TestPrintedSchemaFormsRefuseARenderingOption(t *testing.T) {
	for _, one := range []struct {
		name string
		run  func() int
	}{
		{name: subList, run: func() int { return cmdList([]string{"--json"}, nil) }},
		{name: "show", run: func() int { return cmdShow([]string{"--json", "ze-bgp-conf"}, nil) }},
		{name: subHandlers, run: func() int { return cmdHandlers([]string{"--json"}, nil) }},
		{name: "methods", run: func() int { return cmdMethods([]string{"--json"}, nil) }},
		{name: "events", run: func() int { return cmdEvents([]string{"--json"}, nil) }},
	} {
		t.Run(one.name, func(t *testing.T) {
			var code int
			written := captureStderr(t, func() { code = one.run() })
			if code != 1 {
				t.Errorf("ze schema %s --json exited %d, want 1", one.name, code)
			}
			if !strings.Contains(written, "--json") {
				t.Errorf("the refusal does not name the token: %q", written)
			}
			if !strings.Contains(written, "| json") {
				t.Errorf("the refusal does not name the pipe that answers it: %q", written)
			}
		})
	}
}

// TestSchemaSubcommandsRefuseAnUnexpectedArgument keeps the second half of the
// same guard: a token this dispatch cannot use is named, never dropped.
func TestSchemaSubcommandsRefuseAnUnexpectedArgument(t *testing.T) {
	var code int
	written := captureStderr(t, func() { code = cmdList([]string{"ze-bgp-conf"}, nil) })
	if code != 1 {
		t.Errorf("ze schema list <module> exited %d, want 1", code)
	}
	if !strings.Contains(written, "ze-bgp-conf") {
		t.Errorf("the refusal does not name the argument: %q", written)
	}
}

// TestPrintedSchemaFormsStillRender proves the printer survived the option's
// deletion. What went is the branch that answered JSON. The human-readable
// default output of every `ze schema` subcommand is what stayed.
func TestPrintedSchemaFormsStillRender(t *testing.T) {
	for _, one := range []struct {
		name    string
		run     func() int
		carries []string
	}{
		{
			name:    subList,
			run:     func() int { return cmdList(nil, nil) },
			carries: []string{"Module", "Namespace", "ze-bgp-conf"},
		},
		{
			name:    "show",
			run:     func() int { return cmdShow([]string{"ze-bgp-conf"}, nil) },
			carries: []string{"module ze-bgp-conf"},
		},
		{
			name:    subHandlers,
			run:     func() int { return cmdHandlers(nil, nil) },
			carries: []string{"bgp/peer", "ze-bgp-conf"},
		},
		{
			name:    "methods",
			run:     func() int { return cmdMethods(nil, nil) },
			carries: []string{"Method", "Description", "ze-bgp:peer-list"},
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			var code int
			printed := captureStdout(t, func() { code = one.run() })
			if code != 0 {
				t.Fatalf("ze schema %s exited %d", one.name, code)
			}
			for _, want := range one.carries {
				if !strings.Contains(printed, want) {
					t.Errorf("the printed form lost %q: %q", want, printed)
				}
			}
		})
	}
}
