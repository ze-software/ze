package help_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/plugins/help"
)

// TestVerboseHelpNamesTheGlobalOperators holds `ze help command --verbose` to
// what it is actually reporting. It printed one hand-typed line of ten for
// every command, which was wrong in both directions: it omitted catalog
// operators and asserted row operators that an answer holding one value cannot
// support.
//
// It now reports the command's OWN operators. Answer availability and
// local-process restrictions remain independent dimensions.
func TestVerboseHelpNamesTheGlobalOperators(t *testing.T) {
	entry := help.CommandEntry{
		Path:        "show test",
		Description: "a command that reaches the pipe layer",
	}
	entry.Operators, entry.AnswerShape = help.OperatorsFor(entry.Path, command.DeclaredForCommand(entry.Path), wireToPathsForTest(t))

	var buf bytes.Buffer
	rw := helpfmt.NewRenderWriter(&buf)
	help.PrintCommandVerbose(rw, []help.CommandEntry{entry})
	out := buf.String()
	if !strings.Contains(out, "  pipes:\n") {
		t.Errorf("verbose help omits the pipes heading: %q", out)
	}
	if !strings.Contains(out, "    always: json, ndjson, table, text, yaml, raw, no-more, save\n") {
		t.Errorf("verbose help always line does not include local-only save: %q", out)
	}
	if !strings.Contains(out, "    local process only: save\n") {
		t.Errorf("verbose help local-process line does not contain exactly save: %q", out)
	}

	for _, op := range command.PipeOperatorCatalog() {
		named := namesOperator(out, op.Name)
		// An undeclared command publishes every global always, and every row
		// operator as depending on the answer. The two that need a declared
		// address field are published by neither, because nothing could honor
		// them.
		want := !op.NeedsAddressField
		if named != want {
			t.Errorf("verbose help names %q = %v, want %v", op.Name, named, want)
		}
	}
	if !strings.Contains(out, "always:") {
		t.Errorf("verbose help does not separate what always applies: %q", out)
	}
	if !strings.Contains(out, "when the answer has rows:") {
		t.Errorf("verbose help does not say which operators depend on the answer: %q", out)
	}
	if !strings.Contains(out, "while the command keeps answering:") {
		t.Errorf("verbose help drops the streaming qualifier: %q", out)
	}
	if !strings.Contains(out, "local process only:") {
		t.Errorf("verbose help presents save as available on remote surfaces: %q", out)
	}
}

// VALIDATES: A local printing shortcut does not suppress a separately
// registered daemon handler's reachable pipe layer.
// PREVENTS: `show version` publishing no operators because its local shortcut
// is inspected before its daemon RPC registration.
func TestDualRegisteredDaemonCommandPublishesOperators(t *testing.T) {
	// cmd/ze registers the real `show version` shortcut (registerLocalCommands),
	// and this package cannot link cmd/ze, so a plain local handler stands in
	// for it at the same path: the property under test is how the catalog reads
	// a path that has both a plain local handler and a daemon handler.
	if !command.HasLocal("show version") {
		command.MustRegisterLocalMeta("show version", func(command.ValidatedArgs) int { return 0 }, registry.Meta{})
	}
	if !command.HasLocal("show version") {
		t.Fatal("show version has no local shortcut; this test no longer covers the dual-registration case")
	}
	if command.HasLocalData("show version") {
		t.Fatal("show version now has a local data handler; this test no longer covers a plain local shortcut")
	}
	if !help.DaemonHandlesPath("show version", wireToPathsForTest(t)) {
		t.Fatal("show version has no daemon handler; this test no longer covers dual registration")
	}
	ops, _ := help.OperatorsFor("show version", command.DeclaredForCommand("show version"), wireToPathsForTest(t))
	if len(ops) == 0 {
		t.Fatal("show version publishes no operators although its daemon handler reaches the pipe layer")
	}
	var saveLocal, logStreaming bool
	for _, op := range ops {
		switch op.Name {
		case "save":
			saveLocal = op.LocalOnly
		case "log":
			logStreaming = op.Available == "when-streaming"
		}
	}
	if !saveLocal {
		t.Fatal("show version does not publish save's local-only surface restriction")
	}
	if !logStreaming {
		t.Fatal("show version does not publish log's streaming qualifier")
	}
}

// TestACommandServedWithoutDataPublishesNoOperators holds the catalog to what
// the product can honor. A command served only by a plain local handler reaches
// no pipe layer. A dual-registered path is covered separately because its
// daemon surface can still pipe.
//
// `show data cat` and `show yang doc` were the vehicles, local-only plain
// handlers that published fifteen operators each; both answer data now, so a
// fixture registered here stands in: a plain local handler with a declared
// shape, at a path no YANG module declares.
func TestACommandServedWithoutDataPublishesNoOperators(t *testing.T) {
	const fixture = "show help-fixture plain"
	if !command.HasLocal(fixture) {
		command.MustRegisterLocalMeta(fixture, func(command.ValidatedArgs) int { return 0 }, registry.Meta{})
		command.RegisterShape([]string{fixture}, command.ShapeBlob)
	}
	for _, path := range []string{fixture} {
		if !command.HasLocal(path) {
			t.Fatalf("%s is not served locally; this test no longer covers its case", path)
		}
		if command.HasLocalData(path) {
			t.Fatalf("%s answers with data now; move it out of this test rather than deleting the check", path)
		}
		if help.DaemonHandlesPath(path, wireToPathsForTest(t)) {
			t.Fatalf("%s has a daemon handler now; move it to the dual-registration test", path)
		}
		if _, declared := command.ShapeForCommand(path); !declared {
			t.Fatalf("%s declares no shape; the check below would pass on an undeclared command", path)
		}
		ops, shape := help.OperatorsFor(path, command.DeclaredForCommand(path), wireToPathsForTest(t))
		if len(ops) != 0 {
			t.Errorf("%s publishes %d operators and reaches no pipe layer", path, len(ops))
		}
		if shape != "" {
			t.Errorf("%s publishes shape %q and reaches no pipe layer", path, shape)
		}
	}

	// The converted sibling still publishes, so the check above is not passing
	// because help.OperatorsFor answers nothing for everything.
	if ops, _ := help.OperatorsFor("show data registered", command.DeclaredForCommand("show data registered"), wireToPathsForTest(t)); len(ops) == 0 {
		t.Error("show data registered publishes no operators; it answers with data")
	}
}
