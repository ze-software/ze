package registry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/tmpfs"
)

const pythonLocalDataAST = `
import ast
import json
import sys


def is_named_call(value, name):
    return (
        isinstance(value, ast.Call)
        and isinstance(value.func, ast.Name)
        and value.func.id == name
    )


def completion_marker(statement):
    if not isinstance(statement, ast.Expr):
        return None
    if not is_named_call(statement.value, "print"):
        return None
    arguments = statement.value.args
    if len(arguments) != 1 or statement.value.keywords:
        return None
    if not isinstance(arguments[0], ast.Constant):
        return None
    marker = arguments[0].value
    if not isinstance(marker, str) or not marker.startswith("OK:"):
        return None
    return marker


def command_template(argument):
    if isinstance(argument, ast.Constant) and isinstance(argument.value, str):
        return argument.value
    if (
        isinstance(argument, ast.BinOp)
        and isinstance(argument.op, ast.Mod)
        and isinstance(argument.left, ast.Constant)
        and isinstance(argument.left.value, str)
    ):
        return argument.left.value
    return None


def parse(source):
    try:
        module = ast.parse(source, filename="run.py", mode="exec")
    except SyntaxError as error:
        line = error.lineno or 0
        return {"error": "run.py syntax error at line %d: %s" % (line, error.msg)}

    completions = [
        (index, completion_marker(statement))
        for index, statement in enumerate(module.body)
        if completion_marker(statement) is not None
    ]
    if not completions:
        return {"error": "run.py has no top-level OK completion marker"}
    if len(completions) != 1:
        return {"error": "run.py has %d top-level OK completion markers" % len(completions)}

    invocations = []
    for statement in module.body[:completions[0][0]]:
        if not isinstance(statement, (ast.Assign, ast.AnnAssign)):
            continue
        if not is_named_call(statement.value, "local_json"):
            continue
        call = statement.value
        if len(call.args) != 1 or call.keywords:
            return {"error": "local_json assignment at line %d must have one positional command" % statement.lineno}
        command = command_template(call.args[0])
        if command is None:
            return {"error": "local_json assignment at line %d has a dynamic command" % statement.lineno}
        invocations.append(command)
    return {"completion_marker": completions[0][1], "invocations": invocations}


result = parse(sys.stdin.read())
sys.stdout.write(json.dumps(result, separators=(",", ":"), sort_keys=True))
`

type pythonLocalDataResult struct {
	CompletionMarker *string   `json:"completion_marker"`
	Invocations      *[]string `json:"invocations"`
	Error            *string   `json:"error"`
}

// TestEveryLocalDataRegistrationHasAFunctionalCase derives both populations:
// production calls from Go syntax, and functional cases from the local_json
// calls the UI test executes.
//
// VALIDATES: AC-10 -- every MustRegisterLocalData command has entry-to-pipe proof.
// PREVENTS: a new local data handler landing without a ze cli -c functional case.
func TestEveryLocalDataRegistrationHasAFunctionalCase(t *testing.T) {
	root := repositoryRoot(t)
	registered := productionLocalDataCommands(t, root)
	invocations := functionalLocalDataInvocations(t, root)

	covered := make(map[string]bool, len(registered))
	for _, invocation := range invocations {
		matched := ""
		for command := range registered {
			if invocation == command || strings.HasPrefix(invocation, command+" ") {
				if len(command) > len(matched) {
					matched = command
				}
			}
		}
		if matched == "" {
			t.Errorf("functional local_json invocation has no production registration: %q", invocation)
			continue
		}
		covered[matched] = true
	}

	for command, source := range registered {
		if !covered[command] {
			t.Errorf("%s registers %q without a functional local_json case", source, command)
		}
	}
}

// TestFunctionalLocalDataInvocationsIgnoreDrafts proves the permanent ratchet
// reads only evidence that the normal UI suite executes.
//
// VALIDATES: IR2-2 -- an ignored draft invocation cannot satisfy live coverage.
// PREVENTS: test/draft being preferred over the gating test/ui population.
func TestFunctionalLocalDataInvocationsIgnoreDrafts(t *testing.T) {
	root := t.TempDir()
	liveDir := filepath.Join(root, "test", "ui")
	draftDir := filepath.Join(root, "test", "draft", "ui")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("create live UI test directory: %v", err)
	}
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		t.Fatalf("create draft UI test directory: %v", err)
	}

	liveInvocation := "show env list | json compact"
	live := []byte("tmpfs=run.py:terminator=PY\n" +
		"payload = local_json('show env list | json compact')\n" +
		"print('OK: live fixture')\n" +
		"PY\n" +
		canonicalFunctionalScenarioDirectives("OK: live fixture"))
	livePath := filepath.Join(liveDir, "pipe-local-command.ci")
	if err := os.WriteFile(livePath, live, 0o600); err != nil {
		t.Fatalf("write live functional population: %v", err)
	}

	draft := []byte("local_json('show draft-only fake | json compact')\n")
	draftPath := filepath.Join(draftDir, "pipe-local-command.ci")
	if err := os.WriteFile(draftPath, draft, 0o600); err != nil {
		t.Fatalf("write draft-only fake invocation: %v", err)
	}

	invocations := functionalLocalDataInvocations(t, root)
	if len(invocations) != 1 {
		t.Fatalf("live functional population has %d invocations, want 1: %q",
			len(invocations), invocations)
	}
	if invocations[0] != liveInvocation {
		t.Fatalf("functional population read %q, want live invocation %q",
			invocations[0], liveInvocation)
	}
}

// TestFunctionalLocalDataInvocationsRequireExecutedTopLevelAssignments proves
// the static live-scenario contract cannot be satisfied by inert Python text.
//
// VALIDATES: IR3-6, IR4-1, IR5-5 -- only executable top-level AST assignments
// before exactly one top-level success marker count.
// PREVENTS: comments, strings, nested blocks, dynamic commands, and late literals faking coverage.
func TestFunctionalLocalDataInvocationsRequireExecutedTopLevelAssignments(t *testing.T) {
	tests := []struct {
		name      string
		scenario  string
		want      []string
		wantError bool
	}{
		{
			name: "top-level assignment",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"payload = local_json('show live command | json compact')\n" +
				"print('OK: fixture complete')\n" +
				"PY\n",
			want: []string{"show live command | json compact"},
		},
		{
			name: "literal percent-format template",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"path = '/tmp/config'\n" +
				"payload = local_json('show config dump %s | json compact' % path)\n" +
				"print('OK: fixture complete')\n" +
				"PY\n",
			want: []string{"show config dump %s | json compact"},
		},
		{
			name: "comment",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"# payload = local_json('show comment fake | json compact')\n" +
				"print('OK: fixture complete')\n" +
				"PY\n",
		},
		{
			name: "nested dead code",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"if False:\n" +
				"    payload = local_json('show dead fake | json compact')\n" +
				"print('OK: fixture complete')\n" +
				"PY\n",
		},
		{
			name: "nested function",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"def decoy():\n" +
				"    payload = local_json('show nested fake | json compact')\n" +
				"    print('OK: nested fake')\n" +
				"print('OK: fixture complete')\n" +
				"PY\n",
		},
		{
			name: "triple-quoted fakes",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"decoy = \"\"\"\n" +
				"payload = local_json('show string fake | json compact')\n" +
				"print('OK: string fake')\n" +
				"\"\"\"\n" +
				"payload = local_json('show live command | json compact')\n" +
				"print('OK: fixture complete')\n" +
				"PY\n",
			want: []string{"show live command | json compact"},
		},
		{
			name: "outside Python payload",
			scenario: "tmpfs=before.txt:terminator=BEFORE\n" +
				"payload = local_json('show before fake | json compact')\n" +
				"BEFORE\n" +
				"tmpfs=run.py:terminator=PY\n" +
				"print('OK: fixture complete')\n" +
				"PY\n" +
				"tmpfs=after.txt:terminator=AFTER\n" +
				"payload = local_json('show after fake | json compact')\n" +
				"AFTER\n",
		},
		{
			name: "literal after successful completion",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"print('OK: fixture complete')\n" +
				"payload = local_json('show late fake | json compact')\n" +
				"PY\n",
		},
		{
			name: "syntax error",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"payload = local_json('show broken command | json compact'\n" +
				"print('OK: fixture complete')\n" +
				"PY\n",
			wantError: true,
		},
		{
			name: "missing completion marker",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"payload = local_json('show live command | json compact')\n" +
				"PY\n",
			wantError: true,
		},
		{
			name: "duplicate completion markers",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"print('OK: first marker')\n" +
				"print('OK: second marker')\n" +
				"PY\n",
			wantError: true,
		},
		{
			name: "completion marker with extra argument",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"payload = local_json('show live command | json compact')\n" +
				"print('OK: fixture complete', 'decoy')\n" +
				"PY\n",
			wantError: true,
		},
		{
			name: "completion marker with keyword",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"payload = local_json('show live command | json compact')\n" +
				"print('OK: fixture complete', flush=True)\n" +
				"PY\n",
			wantError: true,
		},
		{
			name: "dynamic command",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"command = 'show dynamic fake | json compact'\n" +
				"payload = local_json(command)\n" +
				"print('OK: fixture complete')\n" +
				"PY\n",
			wantError: true,
		},
		{
			name: "nonliteral percent-format template",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"template = 'show dynamic fake %s | json compact'\n" +
				"payload = local_json(template % 'argument')\n" +
				"print('OK: fixture complete')\n" +
				"PY\n",
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseFunctionalLocalDataInvocations([]byte(test.scenario +
				canonicalFunctionalScenarioDirectives("OK: fixture complete")))
			if test.wantError {
				if err == nil {
					t.Fatalf("parse fixture succeeded with invocations %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Fatalf("invocations = %q, want %q", got, test.want)
			}
		})
	}
}

// TestFunctionalLocalDataInvocationsRequireLaunchedRunPayload proves AC-10
// evidence comes from the one run.py payload the scenario actually executes.
//
// VALIDATES: IR5-2, IR6-4, IR6-5, IR6-6, IR6-7, IR6-17, IR6-18,
// IR7-2, IR7-3, IR7-4 -- production discovery accepts only the canonical
// foreground run.py launch, timeout, and observed AST completion marker.
// PREVENTS: malformed grammar, skips, weak assertions, or competing orchestration satisfying the ratchet.
func TestFunctionalLocalDataInvocationsRequireLaunchedRunPayload(t *testing.T) {
	const (
		runPayload = "tmpfs=run.py:terminator=PY\n" +
			"payload = local_json('show live command | json compact')\n" +
			"print('OK: fixture complete')\n" +
			"PY\n"
		canonicalExpectations = "expect=exit:code=0\n" +
			"expect=stdout:contains=OK: fixture complete\n"
		canonicalRuntime = functionalTimeoutOption + "\n" +
			functionalRunCommandDirective +
			canonicalExpectations
	)

	tests := []struct {
		name      string
		scenario  string
		want      []string
		wantError bool
	}{
		{
			name:     "live canonical scenario",
			scenario: runPayload + canonicalRuntime,
			want:     []string{"show live command | json compact"},
		},
		{
			name: "no expectations",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				functionalRunCommandDirective,
			wantError: true,
		},
		{
			name: "exit-only marker before calls",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"print('OK: fixture complete')\n" +
				"payload = local_json('show late fake | json compact')\n" +
				"PY\n" +
				functionalTimeoutOption + "\n" +
				functionalRunCommandDirective +
				"expect=exit:code=0\n",
			wantError: true,
		},
		{
			name: "wrong OK expectation",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				functionalRunCommandDirective +
				"expect=exit:code=0\n" +
				"expect=stdout:contains=OK: somebody else completed\n",
			wantError: true,
		},
		{
			name: "nonzero file exit",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				functionalRunCommandDirective +
				"expect=exit:code=1\n" +
				"expect=stdout:contains=OK: fixture complete\n",
			wantError: true,
		},
		{
			name: "duplicate file exit",
			scenario: runPayload + canonicalRuntime +
				"expect=exit:code=0\n",
			wantError: true,
		},
		{
			name: "command exit",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				"cmd=foreground:seq=1:exec=python3 run.py:exit=0\n" +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "command timeout",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				"cmd=foreground:seq=1:exec=python3 run.py:timeout=10s\n" +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "await stderr",
			scenario: runPayload + canonicalRuntime +
				"await=stderr:contains=decoy\n",
			wantError: true,
		},
		{
			name: "stdout negative",
			scenario: runPayload + canonicalRuntime +
				"expect=stdout:!contains=ERROR\n",
			wantError: true,
		},
		{
			name: "stdout regex expectation",
			scenario: runPayload + canonicalRuntime +
				"expect=stdout:pattern=OK:.*\n",
			wantError: true,
		},
		{
			name: "stdout regex rejection",
			scenario: runPayload + canonicalRuntime +
				"reject=stdout:pattern=ERROR:.*\n",
			wantError: true,
		},
		{
			name: "stderr contains",
			scenario: runPayload + canonicalRuntime +
				"expect=stderr:contains=decoy\n",
			wantError: true,
		},
		{
			name: "stderr regex expectation",
			scenario: runPayload + canonicalRuntime +
				"expect=stderr:pattern=decoy\n",
			wantError: true,
		},
		{
			name: "stderr regex rejection",
			scenario: runPayload + canonicalRuntime +
				"reject=stderr:pattern=decoy\n",
			wantError: true,
		},
		{
			name: "syslog expectation",
			scenario: runPayload + canonicalRuntime +
				"expect=syslog:pattern=decoy\n",
			wantError: true,
		},
		{
			name: "syslog rejection",
			scenario: runPayload + canonicalRuntime +
				"reject=syslog:pattern=decoy\n",
			wantError: true,
		},
		{
			name: "malformed runtime regex",
			scenario: runPayload + canonicalRuntime +
				"expect=stdout:pattern=[invalid\n",
			wantError: true,
		},
		{
			name: "needs-path candidate",
			scenario: runPayload + canonicalRuntime +
				"option=needs-path:value=candidate.ci\n",
			wantError: true,
		},
		{
			name: "skip option",
			scenario: runPayload + canonicalRuntime +
				"option=skip-os:value=linux\n",
			wantError: true,
		},
		{
			name: "capability option",
			scenario: runPayload + canonicalRuntime +
				"option=needs-linux:caps=net-admin\n",
			wantError: true,
		},
		{
			name: "duplicate timeout",
			scenario: runPayload + canonicalRuntime +
				functionalTimeoutOption + "\n",
			wantError: true,
		},
		{
			name: "altered timeout",
			scenario: runPayload +
				"option=timeout:value=44s\n" +
				functionalRunCommandDirective +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "missing timeout",
			scenario: runPayload +
				functionalRunCommandDirective +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "driver.py launch selects second payload",
			scenario: runPayload +
				"tmpfs=driver.py:terminator=DRIVER\n" +
				"print('OK: alternate driver')\n" +
				"DRIVER\n" +
				functionalTimeoutOption + "\n" +
				"cmd=foreground:seq=1:exec=python3 driver.py\n" +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "second run.py payload",
			scenario: runPayload +
				"tmpfs=run.py:terminator=SECOND\n" +
				"print('OK: duplicate payload')\n" +
				"SECOND\n" +
				canonicalRuntime,
			wantError: true,
		},
		{
			name: "missing run.py payload",
			scenario: "tmpfs=driver.py:terminator=DRIVER\n" +
				"print('OK: alternate driver')\n" +
				"DRIVER\n" +
				canonicalRuntime,
			wantError: true,
		},
		{
			name: "missing command",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "duplicate command",
			scenario: runPayload + canonicalRuntime +
				"cmd=foreground:seq=2:exec=python3 driver.py\n",
			wantError: true,
		},
		{
			name: "ambiguous relative-path spelling",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				"cmd=foreground:seq=1:exec=python3 ./run.py\n" +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "ambiguous whitespace spelling",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				"cmd=foreground:seq=1:exec=python3  run.py\n" +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "missing run.py terminator",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"print('OK: fixture complete')\n" +
				canonicalRuntime,
			wantError: true,
		},
		{
			name: "malformed stop",
			scenario: runPayload + canonicalRuntime +
				"cmd=stop:seq=2\n",
			wantError: true,
		},
		{
			name: "malformed API command",
			scenario: runPayload + canonicalRuntime +
				"cmd=api:seq=2:text=shutdown\n",
			wantError: true,
		},
		{
			name: "malformed per-command exit",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				"cmd=foreground:seq=1:exec=python3 run.py:exit=yes\n" +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "unresolved stdin binding",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				"cmd=foreground:seq=1:exec=python3 run.py:stdin=missing\n" +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "valid but preemptive stop",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				"cmd=stop:seq=1:name=runner\n" +
				"cmd=foreground:seq=2:exec=python3 run.py\n" +
				canonicalExpectations,
			wantError: true,
		},
		{
			name: "extra API command",
			scenario: runPayload + canonicalRuntime +
				"cmd=api:conn=1:seq=2:text=shutdown\n",
			wantError: true,
		},
		{
			name: "extra runner message",
			scenario: runPayload + canonicalRuntime +
				"expect=json:conn=1:seq=2:json={}\n",
			wantError: true,
		},
		{
			name: "extra HTTP check",
			scenario: runPayload + canonicalRuntime +
				"http=get:seq=2:url=http://127.0.0.1/:status=200\n",
			wantError: true,
		},
		{
			name: "extra HTTP wait",
			scenario: runPayload + canonicalRuntime +
				"http=wait:seq=2:url=http://127.0.0.1/:status=200\n",
			wantError: true,
		},
		{
			name: "extra engine step",
			scenario: runPayload + canonicalRuntime +
				"command=show version\n",
			wantError: true,
		},
		{
			name: "extra file check",
			scenario: runPayload + canonicalRuntime +
				"expect=file:path=result.txt:exists=true\n",
			wantError: true,
		},
		{
			name: "named foreground launch",
			scenario: runPayload +
				functionalTimeoutOption + "\n" +
				"cmd=foreground:seq=1:exec=python3 run.py:name=runner\n" +
				canonicalExpectations,
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseFunctionalLocalDataInvocations([]byte(test.scenario))
			if test.wantError {
				if err == nil {
					t.Fatalf("parse fixture succeeded with invocations %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Fatalf("invocations = %q, want %q", got, test.want)
			}
		})
	}
}

// TestFunctionalLocalDataInvocationsAcceptLiveCanonicalScenario proves the
// committed AC-10 evidence satisfies the same production-runner contract as
// the rejection fixtures above.
func TestFunctionalLocalDataInvocationsAcceptLiveCanonicalScenario(t *testing.T) {
	root := repositoryRoot(t)
	path := filepath.Join(root, "test", "ui", "pipe-local-command.ci")
	content, err := os.ReadFile(path) //nolint:gosec // Repository-owned functional test.
	if err != nil {
		t.Fatalf("read canonical functional scenario: %v", err)
	}
	invocations, err := parseFunctionalLocalDataInvocations(content)
	if err != nil {
		t.Fatalf("parse canonical functional scenario: %v", err)
	}
	if len(invocations) == 0 {
		t.Fatal("canonical functional scenario has no executable local_json invocation")
	}
}

func TestDecodePythonLocalDataResultRejectsInvalidOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{name: "not JSON", output: "not-json"},
		{name: "unknown field", output: `{"invocations":[],"unknown":true}`},
		{name: "missing result", output: `{}`},
		{name: "ambiguous result", output: `{"invocations":[],"error":"failed"}`},
		{name: "empty error", output: `{"error":""}`},
		{name: "trailing document", output: `{"invocations":[]}{"invocations":[]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodePythonLocalDataResult([]byte(test.output)); err == nil {
				t.Fatal("decode parser output succeeded, want error")
			}
		})
	}
}

func TestProductionLocalDataCommandsSkipTestdataAndLERootAdapter(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		filepath.Join(root, "cmd"),
		filepath.Join(root, "cmd", "testdata", "malformed"),
		filepath.Join(root, "internal", "le", "leroot"),
		filepath.Join(root, "pkg"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create fixture directory %s: %v", dir, err)
		}
	}
	livePath := filepath.Join(root, "cmd", "live.go")
	if err := os.WriteFile(livePath, []byte("package cmd\nfunc register() {\n"+
		"registry.MustRegisterLocalData(\"show live | json compact\")\n}\n"), 0o600); err != nil {
		t.Fatalf("write live registration fixture: %v", err)
	}
	malformedPath := filepath.Join(root, "cmd", "testdata", "malformed", "broken.go")
	if err := os.WriteFile(malformedPath, []byte("package malformed\nfunc {"), 0o600); err != nil {
		t.Fatalf("write malformed testdata fixture: %v", err)
	}
	adapterPath := filepath.Join(root, "internal", "le", "leroot", "leroot.go")
	if err := os.WriteFile(adapterPath, []byte("package leroot\nfunc Register(name string) {\n"+
		"registry.MustRegisterLocalData(CommandPath(name))\n}\n"), 0o600); err != nil {
		t.Fatalf("write leroot adapter fixture: %v", err)
	}

	commands := productionLocalDataCommands(t, root)
	if len(commands) != 1 {
		t.Fatalf("production registrations = %v, want only the live literal", commands)
	}
	if got := commands["show live | json compact"]; got != filepath.Join("cmd", "live.go") {
		t.Fatalf("live registration source = %q, want cmd/live.go", got)
	}
}

func TestLERootLocalDataAdapterExclusionIsExact(t *testing.T) {
	commandPath := func(argumentCount int) ast.Expr {
		arguments := make([]ast.Expr, argumentCount)
		for index := range arguments {
			arguments[index] = &ast.Ident{Name: "name"}
		}
		return &ast.CallExpr{
			Fun:  &ast.Ident{Name: "CommandPath"},
			Args: arguments,
		}
	}
	tests := []struct {
		name     string
		path     string
		argument ast.Expr
		want     bool
	}{
		{
			name:     "exact leroot adapter",
			path:     filepath.Join("internal", "le", "leroot", "leroot.go"),
			argument: commandPath(1),
			want:     true,
		},
		{
			name:     "same call elsewhere",
			path:     filepath.Join("internal", "component", "other.go"),
			argument: commandPath(1),
		},
		{
			name:     "other dynamic expression in leroot",
			path:     filepath.Join("internal", "le", "leroot", "leroot.go"),
			argument: &ast.Ident{Name: "path"},
		},
		{
			name: "different command path argument",
			path: filepath.Join("internal", "le", "leroot", "leroot.go"),
			argument: &ast.CallExpr{
				Fun:  &ast.Ident{Name: "CommandPath"},
				Args: []ast.Expr{&ast.Ident{Name: "other"}},
			},
		},
		{
			name:     "different command path arity",
			path:     filepath.Join("internal", "le", "leroot", "leroot.go"),
			argument: commandPath(2),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isLERootLocalDataAdapter(test.path, test.argument); got != test.want {
				t.Fatalf("isLERootLocalDataAdapter() = %t, want %t", got, test.want)
			}
		})
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller did not report this test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func productionLocalDataCommands(t *testing.T, root string) map[string]string {
	t.Helper()
	commands := make(map[string]string)
	for _, sourceRoot := range []string{"cmd", "internal", "pkg"} {
		path := filepath.Join(root, sourceRoot)
		err := filepath.WalkDir(path, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			collectLocalDataRegistrations(t, root, path, commands)
			return nil
		})
		if err != nil {
			t.Fatalf("walk production Go under %s: %v", sourceRoot, err)
		}
	}
	if len(commands) == 0 {
		t.Fatal("derived no production MustRegisterLocalData commands")
	}
	return commands
}

func collectLocalDataRegistrations(t *testing.T, root, path string, commands map[string]string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "MustRegisterLocalData" {
			return true
		}
		if len(call.Args) == 0 {
			t.Errorf("%s has MustRegisterLocalData without a path", path)
			return true
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			t.Fatalf("make %s relative: %v", path, relErr)
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			if isLERootLocalDataAdapter(relative, call.Args[0]) {
				return true
			}
			t.Errorf("%s has a non-literal MustRegisterLocalData path", path)
			return true
		}
		command, unquoteErr := strconv.Unquote(literal.Value)
		if unquoteErr != nil {
			t.Errorf("%s has an invalid MustRegisterLocalData path: %v", path, unquoteErr)
			return true
		}
		if previous, exists := commands[command]; exists {
			t.Errorf("%q is registered by both %s and %s", command, previous, relative)
			return true
		}
		commands[command] = relative
		return true
	})
}

func isLERootLocalDataAdapter(relative string, argument ast.Expr) bool {
	if filepath.ToSlash(relative) != "internal/le/leroot/leroot.go" {
		return false
	}
	call, ok := argument.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	function, ok := call.Fun.(*ast.Ident)
	if !ok || function.Name != "CommandPath" {
		return false
	}
	name, ok := call.Args[0].(*ast.Ident)
	return ok && name.Name == "name"
}

func functionalLocalDataInvocations(t *testing.T, root string) []string {
	t.Helper()
	path := filepath.Join(root, "test", "ui", "pipe-local-command.ci")
	content, err := os.ReadFile(path) //nolint:gosec // Repository-owned functional test.
	if err != nil {
		t.Fatalf("read functional local-data population %s: %v", path, err)
	}
	invocations, err := parseFunctionalLocalDataInvocations(content)
	if err != nil {
		t.Fatalf("parse functional local-data population %s: %v", path, err)
	}
	if len(invocations) == 0 {
		t.Fatalf("%s has no executable top-level local_json assignments", path)
	}
	return invocations
}

const (
	functionalRunCommand          = "python3 run.py"
	functionalRunCommandDirective = "cmd=foreground:seq=1:exec=" + functionalRunCommand + "\n"
	functionalTimeoutOption       = "option=timeout:value=45s"
	functionalTimeout             = "45s"
)

type parsedPythonLocalData struct {
	Invocations      []string
	CompletionMarker string
}

func canonicalFunctionalScenarioDirectives(completionMarker string) string {
	return functionalTimeoutOption + "\n" +
		functionalRunCommandDirective +
		"expect=exit:code=0\n" +
		"expect=stdout:contains=" + completionMarker + "\n"
}

func parseFunctionalLocalDataInvocations(content []byte) ([]string, error) {
	dir, err := os.MkdirTemp("", "ze-functional-local-data-")
	if err != nil {
		return nil, fmt.Errorf("create isolated functional scenario directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	// needs-path discovery searches upward from the candidate for go.mod.
	// A standalone module makes the isolated directory the lookup root.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module functional.coverage\n"), 0o600); err != nil {
		return nil, fmt.Errorf("materialize isolated functional module: %w", err)
	}
	candidate := filepath.Join(dir, "candidate.ci")
	if err := os.WriteFile(candidate, content, 0o600); err != nil {
		return nil, fmt.Errorf("materialize functional scenario: %w", err)
	}

	discovered := runner.NewEncodingTests(dir)
	if err := discovered.Discover(dir); err != nil {
		return nil, fmt.Errorf("discover functional scenario with production runner: %w", err)
	}
	records := discovered.Registered()
	if len(records) != 1 {
		return nil, fmt.Errorf("production runner discovered %d functional scenarios, want exactly 1", len(records))
	}
	record := records[0]
	if record.ParseFailed {
		if record.Error != nil {
			return nil, fmt.Errorf("production runner rejected functional scenario: %w", record.Error)
		}
		return nil, fmt.Errorf("production runner rejected functional scenario without an error")
	}
	if record.Error != nil {
		return nil, fmt.Errorf("production runner recorded a functional scenario error: %w", record.Error)
	}

	scenario, err := tmpfs.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("locate run.py in functional scenario: %w", err)
	}
	if err := validateFunctionalScenarioOptions(record, scenario.OtherLines); err != nil {
		return nil, err
	}
	if record.SkipReason != "" {
		return nil, fmt.Errorf("production runner skipped functional scenario: %s", record.SkipReason)
	}

	if len(record.RunCommands) != 1 {
		return nil, fmt.Errorf("functional scenario has %d run commands, want exactly 1", len(record.RunCommands))
	}
	runCommand := record.RunCommands[0]
	if runCommand.Mode != "foreground" {
		return nil, fmt.Errorf("functional scenario run command mode is %q, want foreground", runCommand.Mode)
	}
	if runCommand.Seq != 1 {
		return nil, fmt.Errorf("functional scenario run command sequence is %d, want 1", runCommand.Seq)
	}
	if runCommand.Exec != functionalRunCommand {
		return nil, fmt.Errorf("functional scenario launches %q, want %q", runCommand.Exec, functionalRunCommand)
	}
	if runCommand.Stdin != "" {
		return nil, fmt.Errorf("functional scenario run command reads stdin block %q, want none", runCommand.Stdin)
	}
	if runCommand.Name != "" {
		return nil, fmt.Errorf("functional scenario run command has background name %q, want none", runCommand.Name)
	}
	if runCommand.Signal != "" {
		return nil, fmt.Errorf("functional scenario run command has signal %q, want none", runCommand.Signal)
	}
	if runCommand.ExitCode != nil {
		return nil, fmt.Errorf("functional scenario run command has a per-command exit assertion")
	}
	if runCommand.Timeout != "" {
		return nil, fmt.Errorf("functional scenario run command has timeout %q, want none", runCommand.Timeout)
	}
	if len(record.StdinBlocks) != 0 {
		return nil, fmt.Errorf("functional scenario has stdin orchestration blocks")
	}
	if len(record.Messages) != 0 {
		return nil, fmt.Errorf("functional scenario has runner message steps")
	}
	if len(record.Expects) != 0 {
		return nil, fmt.Errorf("functional scenario has legacy API expectation steps")
	}
	if len(record.HTTPChecks) != 0 {
		return nil, fmt.Errorf("functional scenario has HTTP check steps")
	}
	if len(record.HTTPWaits) != 0 {
		return nil, fmt.Errorf("functional scenario has HTTP wait steps")
	}
	if len(record.EngineSteps) != 0 {
		return nil, fmt.Errorf("functional scenario has engine steps")
	}
	if len(record.FileChecks) != 0 {
		return nil, fmt.Errorf("functional scenario has file-check steps")
	}

	var payload []byte
	runPayloads := 0
	for _, file := range scenario.Files {
		if file.Path != "run.py" {
			continue
		}
		runPayloads++
		payload = file.Content
	}
	if runPayloads != 1 {
		return nil, fmt.Errorf("functional scenario has %d tmpfs=run.py payloads, want exactly 1", runPayloads)
	}
	parsed, err := parsePythonLocalDataInvocations(payload)
	if err != nil {
		return nil, err
	}
	if err := validateFunctionalScenarioAssertions(record, parsed.CompletionMarker, scenario.OtherLines); err != nil {
		return nil, err
	}
	return parsed.Invocations, nil
}

func validateFunctionalScenarioOptions(record *runner.Record, lines []string) error {
	options := make([]string, 0, 1)
	for _, line := range lines {
		if strings.HasPrefix(line, "option=") {
			options = append(options, line)
		}
	}
	if len(options) != 1 {
		return fmt.Errorf("functional scenario has %d raw options, want exactly %q", len(options), functionalTimeoutOption)
	}
	if options[0] != functionalTimeoutOption {
		return fmt.Errorf("functional scenario option is %q, want exactly %q", options[0], functionalTimeoutOption)
	}
	if got := record.Extra["timeout"]; got != functionalTimeout {
		return fmt.Errorf("production runner timeout is %q, want %q", got, functionalTimeout)
	}
	if len(record.Extra) != 1 {
		return fmt.Errorf("production runner recorded %d runtime options, want only timeout", len(record.Extra))
	}
	return nil
}

func validateFunctionalScenarioAssertions(record *runner.Record, completionMarker string, lines []string) error {
	exitAssertions := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "expect=exit:") {
			if line != "expect=exit:code=0" {
				return fmt.Errorf("functional scenario exit directive is %q, want exactly expect=exit:code=0", line)
			}
			exitAssertions++
		}
	}
	if exitAssertions != 1 {
		return fmt.Errorf("functional scenario has %d raw exit assertions, want exactly 1", exitAssertions)
	}
	if record.ExpectExitCode == nil {
		return fmt.Errorf("functional scenario has no file-level successful exit assertion")
	}
	if *record.ExpectExitCode != 0 {
		return fmt.Errorf("functional scenario exit assertion is %d, want 0", *record.ExpectExitCode)
	}
	if len(record.ExpectStdoutMatch) != 1 {
		return fmt.Errorf("functional scenario has %d stdout contains assertions, want exactly 1", len(record.ExpectStdoutMatch))
	}
	if record.ExpectStdoutMatch[0] != completionMarker {
		return fmt.Errorf("functional scenario stdout assertion is %q, want AST completion marker %q",
			record.ExpectStdoutMatch[0], completionMarker)
	}
	if len(record.ExpectStdoutNotMatch) != 0 {
		return fmt.Errorf("functional scenario has stdout negative assertions")
	}
	if len(record.ExpectStdoutRegex) != 0 {
		return fmt.Errorf("functional scenario has stdout regex expectations")
	}
	if len(record.RejectStdoutRegex) != 0 {
		return fmt.Errorf("functional scenario has stdout regex rejections")
	}
	if len(record.ExpectStderrMatch) != 0 {
		return fmt.Errorf("functional scenario has stderr contains expectations")
	}
	if len(record.ExpectStderr) != 0 {
		return fmt.Errorf("functional scenario has stderr regex expectations")
	}
	if len(record.RejectStderr) != 0 {
		return fmt.Errorf("functional scenario has stderr regex rejections")
	}
	if len(record.ExpectSyslog) != 0 {
		return fmt.Errorf("functional scenario has syslog expectations")
	}
	if len(record.RejectSyslog) != 0 {
		return fmt.Errorf("functional scenario has syslog rejections")
	}
	if record.AwaitStderr != "" {
		return fmt.Errorf("functional scenario has an await stderr fence")
	}
	if record.AwaitStderrTimeout != "" {
		return fmt.Errorf("functional scenario has an await stderr timeout")
	}
	return nil
}

func parsePythonLocalDataInvocations(payload []byte) (parsedPythonLocalData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, "python3", "-I", "-c", pythonLocalDataAST) //nolint:gosec // Fixed interpreter and helper, with the scenario on stdin.
	command.Env = []string{"LANG=C", "LC_ALL=C"}
	command.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return parsedPythonLocalData{}, fmt.Errorf("run Python AST parser: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() != 0 {
		return parsedPythonLocalData{}, fmt.Errorf("Python AST parser wrote stderr: %s", strings.TrimSpace(stderr.String()))
	}
	return decodePythonLocalDataResult(stdout.Bytes())
}

func decodePythonLocalDataResult(output []byte) (parsedPythonLocalData, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	var result pythonLocalDataResult
	if err := decoder.Decode(&result); err != nil {
		return parsedPythonLocalData{}, fmt.Errorf("decode Python AST parser output: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return parsedPythonLocalData{}, fmt.Errorf("decode Python AST parser output: unexpected trailing JSON")
		}
		return parsedPythonLocalData{}, fmt.Errorf("decode Python AST parser output: trailing data: %w", err)
	}
	if result.Error != nil && (result.Invocations != nil || result.CompletionMarker != nil) {
		return parsedPythonLocalData{}, fmt.Errorf("decode Python AST parser output: both result and error are present")
	}
	if result.Error != nil {
		if *result.Error == "" {
			return parsedPythonLocalData{}, fmt.Errorf("decode Python AST parser output: empty error")
		}
		return parsedPythonLocalData{}, fmt.Errorf("parse run.py AST: %s", *result.Error)
	}
	if result.Invocations == nil || result.CompletionMarker == nil {
		return parsedPythonLocalData{}, fmt.Errorf("decode Python AST parser output: result is missing")
	}
	if !strings.HasPrefix(*result.CompletionMarker, "OK:") {
		return parsedPythonLocalData{}, fmt.Errorf("decode Python AST parser output: invalid completion marker %q", *result.CompletionMarker)
	}
	return parsedPythonLocalData{
		Invocations:      *result.Invocations,
		CompletionMarker: *result.CompletionMarker,
	}, nil
}
