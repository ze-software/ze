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
