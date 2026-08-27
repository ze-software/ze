package registry_test

import (
	"bufio"
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


def is_completion(statement):
    if not isinstance(statement, ast.Expr):
        return False
    if not is_named_call(statement.value, "print"):
        return False
    arguments = statement.value.args
    return (
        bool(arguments)
        and isinstance(arguments[0], ast.Constant)
        and isinstance(arguments[0].value, str)
        and arguments[0].value.startswith("OK:")
    )


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

    completion_indexes = [
        index for index, statement in enumerate(module.body)
        if is_completion(statement)
    ]
    if not completion_indexes:
        return {"error": "run.py has no top-level OK completion marker"}
    if len(completion_indexes) != 1:
        return {"error": "run.py has %d top-level OK completion markers" % len(completion_indexes)}

    invocations = []
    for statement in module.body[:completion_indexes[0]]:
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
    return {"invocations": invocations}


result = parse(sys.stdin.read())
sys.stdout.write(json.dumps(result, separators=(",", ":"), sort_keys=True))
`

type pythonLocalDataResult struct {
	Invocations *[]string `json:"invocations"`
	Error       *string   `json:"error"`
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
		"PY\n")
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
// VALIDATES: IR3-6, IR4-1 -- only executable top-level AST assignments before
// exactly one top-level success marker count.
// PREVENTS: comments, strings, nested blocks, and dynamic commands faking coverage.
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
			scenario: "payload = local_json('show before fake | json compact')\n" +
				"tmpfs=run.py:terminator=PY\n" +
				"print('OK: fixture complete')\n" +
				"PY\n" +
				"payload = local_json('show after fake | json compact')\n",
		},
		{
			name: "after successful completion",
			scenario: "tmpfs=run.py:terminator=PY\n" +
				"print('OK: fixture complete')\n" +
				"payload = local_json(command)\n" +
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
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
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
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			t.Errorf("%s has a non-literal MustRegisterLocalData path", path)
			return true
		}
		command, unquoteErr := strconv.Unquote(literal.Value)
		if unquoteErr != nil {
			t.Errorf("%s has an invalid MustRegisterLocalData path: %v", path, unquoteErr)
			return true
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			t.Fatalf("make %s relative: %v", path, relErr)
		}
		if previous, exists := commands[command]; exists {
			t.Errorf("%q is registered by both %s and %s", command, previous, relative)
			return true
		}
		commands[command] = relative
		return true
	})
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

func parseFunctionalLocalDataInvocations(content []byte) ([]string, error) {
	const payloadPrefix = "tmpfs=run.py:terminator="

	scanner := bufio.NewScanner(bytes.NewReader(content))
	var payload bytes.Buffer
	terminator := ""
	inPayload := false
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if !inPayload {
			if payloadTerminator, ok := strings.CutPrefix(line, payloadPrefix); ok {
				terminator = payloadTerminator
				if terminator == "" {
					return nil, fmt.Errorf("run.py payload has an empty terminator")
				}
				inPayload = true
			}
			continue
		}
		if line == terminator {
			return parsePythonLocalDataInvocations(payload.Bytes())
		}
		payload.WriteString(line)
		payload.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan functional scenario: %w", err)
	}
	if !inPayload {
		return nil, fmt.Errorf("functional scenario has no tmpfs=run.py payload")
	}
	return nil, fmt.Errorf("run.py payload has no %q terminator", terminator)
}

func parsePythonLocalDataInvocations(payload []byte) ([]string, error) {
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
		return nil, fmt.Errorf("run Python AST parser: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() != 0 {
		return nil, fmt.Errorf("Python AST parser wrote stderr: %s", strings.TrimSpace(stderr.String()))
	}
	return decodePythonLocalDataResult(stdout.Bytes())
}

func decodePythonLocalDataResult(output []byte) ([]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	var result pythonLocalDataResult
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Python AST parser output: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode Python AST parser output: unexpected trailing JSON")
		}
		return nil, fmt.Errorf("decode Python AST parser output: trailing data: %w", err)
	}
	if result.Invocations != nil && result.Error != nil {
		return nil, fmt.Errorf("decode Python AST parser output: both result and error are present")
	}
	if result.Error != nil {
		if *result.Error == "" {
			return nil, fmt.Errorf("decode Python AST parser output: empty error")
		}
		return nil, fmt.Errorf("parse run.py AST: %s", *result.Error)
	}
	if result.Invocations == nil {
		return nil, fmt.Errorf("decode Python AST parser output: result is missing")
	}
	return *result.Invocations, nil
}
