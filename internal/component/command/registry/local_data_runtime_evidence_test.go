package registry_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/tmpfs"
)

const (
	runtimeEvidenceScenarioRelativePath = "test/draft/ui/pipe-local-command-runtime.ci"
	runtimeEvidenceRunCommand           = "python3 run.py"
	runtimeEvidenceRunDirective         = "cmd=foreground:seq=1:exec=" + runtimeEvidenceRunCommand + "\n"
	runtimeEvidenceTimeoutOption        = "option=timeout:value=45s"
	runtimeEvidenceTimeout              = "45s"
	runtimeEvidenceCompletionMarker     = "OK: 15/15 local-data commands and local one-shot save"
	runtimeEvidenceCanonicalHelper      = "" +
		"def local_json(command, evidence):\n" +
		"    require(evidence and command.startswith(evidence), 'invalid evidence')\n" +
		"    require('| ' in command, 'local command has no real pipe')\n" +
		"    code, out, err = run(['ze', 'cli', '-c', command])\n" +
		"    require(code == 0, 'local command failed')\n" +
		"    value = json.loads(out)\n" +
		"    print('COVERED: ' + evidence + ' [done]')\n" +
		"    return value\n\n"
)

type runtimeEvidenceInvocation struct {
	Command  string `json:"command"`
	Evidence string `json:"evidence"`
}

type runtimeEvidenceParsedPython struct {
	Invocations      []runtimeEvidenceInvocation `json:"invocations"`
	CompletionMarker string                      `json:"completion_marker"`
}

func TestRuntimeEvidenceEveryProductionRegistrationHasInterpretedCase(t *testing.T) {
	root := runtimeEvidenceRepositoryRoot(t)
	registered := productionLocalDataCommands(t, root)
	parsed := runtimeEvidenceCommittedScenario(t, root)
	covered := make(map[string]bool, len(registered))
	commands := make(map[string]bool, len(parsed.Invocations))
	for _, invocation := range parsed.Invocations {
		if invocation.Evidence == "" || (invocation.Command != invocation.Evidence &&
			!strings.HasPrefix(invocation.Command, invocation.Evidence+" ")) {
			t.Errorf("invalid runtime command/evidence pair: %+v", invocation)
			continue
		}
		if !strings.Contains(invocation.Command, "| ") {
			t.Errorf("runtime command has no real pipe: %q", invocation.Command)
		}
		if commands[invocation.Command] {
			t.Errorf("runtime command is executed twice: %q", invocation.Command)
		}
		commands[invocation.Command] = true
		resolved := runtimeEvidenceLongestProductionPrefix(invocation.Command, registered)
		if resolved != invocation.Evidence {
			t.Errorf("runtime command %q resolves production registration %q, want evidence %q",
				invocation.Command, resolved, invocation.Evidence)
			continue
		}
		covered[resolved] = true
	}
	for command, source := range registered {
		if !covered[command] {
			t.Errorf("%s registers %q without interpreted runtime evidence", source, command)
		}
	}
}

func TestRuntimeEvidencePopulationAndCompletionAreExact(t *testing.T) {
	parsed := runtimeEvidenceCommittedScenario(t, runtimeEvidenceRepositoryRoot(t))
	if len(parsed.Invocations) != 18 {
		t.Fatalf("interpreted local-data calls = %d, want 18", len(parsed.Invocations))
	}
	distinct := make(map[string]bool, len(parsed.Invocations))
	for _, invocation := range parsed.Invocations {
		distinct[invocation.Evidence] = true
	}
	if len(distinct) != 15 {
		t.Fatalf("distinct interpreted evidence = %d, want 15: %v", len(distinct), distinct)
	}
	if parsed.CompletionMarker != runtimeEvidenceCompletionMarker {
		t.Fatalf("completion marker = %q, want %q", parsed.CompletionMarker, runtimeEvidenceCompletionMarker)
	}
}

func TestRuntimeEvidencePythonContractRejectsSpoofs(t *testing.T) {
	const (
		call = "payload = local_json('show live command | json compact', 'show live command')\n"
		done = "print('OK: fixture complete')\n"
	)
	tests := []struct {
		name      string
		payload   string
		want      []runtimeEvidenceInvocation
		wantError bool
	}{
		{name: "canonical helper and literal evidence", payload: runtimeEvidenceCanonicalHelper + call + done,
			want: []runtimeEvidenceInvocation{{Command: "show live command | json compact", Evidence: "show live command"}}},
		{name: "literal-left percent template", payload: runtimeEvidenceCanonicalHelper +
			"path = '/tmp/config'\n" +
			"payload = local_json('show config dump %s | json compact' % path, 'show config dump')\n" + done,
			want: []runtimeEvidenceInvocation{{Command: "show config dump %s | json compact", Evidence: "show config dump"}}},
		{name: "canonical draft shape", payload: "import json\nimport os\nimport stat\nimport subprocess\n\n" +
			"def require(condition, message):\n    if not condition:\n        raise SystemExit(message)\n\n" +
			"def run(argv):\n    return 0, '{}', ''\n\n" + runtimeEvidenceCanonicalHelper +
			"def rows(payload, key):\n    return payload[key]\n\n" +
			"path = '/tmp/config'\n" +
			"payload = local_json('show config dump %s | json compact' % path, 'show config dump')\n" +
			"command_tree = local_json('show yang tree --commands | json compact', 'show yang tree')\n" +
			"config_tree = local_json('show yang tree --config | json compact', 'show yang tree')\n" + done,
			want: []runtimeEvidenceInvocation{
				{Command: "show config dump %s | json compact", Evidence: "show config dump"},
				{Command: "show yang tree --commands | json compact", Evidence: "show yang tree"},
				{Command: "show yang tree --config | json compact", Evidence: "show yang tree"},
			}},
		{name: "no-op helper", payload: "def local_json(command, evidence):\n    return {}\n" + call + done, wantError: true},
		{name: "rebound helper", payload: runtimeEvidenceCanonicalHelper + "local_json = lambda command, evidence: {}\n" + call + done, wantError: true},
		{name: "import alias helper", payload: runtimeEvidenceCanonicalHelper + "import json as local_json\n" + call + done, wantError: true},
		{name: "nested function named local_json", payload: runtimeEvidenceCanonicalHelper +
			"def wrapper():\n    def local_json(command, evidence):\n        return {}\n" + call + done, wantError: true},
		{name: "class named local_json", payload: runtimeEvidenceCanonicalHelper + "class local_json:\n    pass\n" + call + done, wantError: true},
		{name: "argument named local_json", payload: runtimeEvidenceCanonicalHelper + "def wrapper(local_json):\n    return local_json\n" + call + done, wantError: true},
		{name: "deleted helper", payload: runtimeEvidenceCanonicalHelper + "del local_json\n" + call + done, wantError: true},
		{name: "global helper declaration", payload: runtimeEvidenceCanonicalHelper + "def wrapper():\n    global local_json\n" + call + done, wantError: true},
		{name: "except binding named local_json", payload: runtimeEvidenceCanonicalHelper +
			"try:\n    pass\nexcept Exception as local_json:\n    pass\n" + call + done, wantError: true},
		{name: "marker spoof via print", payload: runtimeEvidenceCanonicalHelper + "print('COVERED: show live command [done]')\n" + call + done, wantError: true},
		{name: "marker spoof via stdout write", payload: runtimeEvidenceCanonicalHelper + "sys.stdout.write('COVERED: show live command [done]\\n')\n" + call + done, wantError: true},
		{name: "unrelated print outside helper", payload: runtimeEvidenceCanonicalHelper + "print('diagnostic')\n" + call + done, wantError: true},
		{name: "unrelated stdout write outside helper", payload: runtimeEvidenceCanonicalHelper + "sys.stdout.write('diagnostic\\n')\n" + call + done, wantError: true},
		{name: "covered literal spoof", payload: runtimeEvidenceCanonicalHelper + "decoy = 'COVERED: show live command [done]'\n" + call + done, wantError: true},
		{name: "marker before decode", payload: strings.Replace(runtimeEvidenceCanonicalHelper,
			"    value = json.loads(out)\n    print('COVERED: ' + evidence + ' [done]')\n",
			"    print('COVERED: ' + evidence + ' [done]')\n    value = json.loads(out)\n", 1) + call + done, wantError: true},
		{name: "caught decode failure", payload: strings.Replace(runtimeEvidenceCanonicalHelper,
			"    value = json.loads(out)\n",
			"    try:\n        value = json.loads(out)\n    except json.JSONDecodeError:\n        value = {}\n", 1) + call + done, wantError: true},
		{name: "early return before command", payload: strings.Replace(runtimeEvidenceCanonicalHelper,
			"    code, out, err = run(['ze', 'cli', '-c', command])\n",
			"    if not evidence:\n        return {}\n    code, out, err = run(['ze', 'cli', '-c', command])\n", 1) + call + done, wantError: true},
		{name: "wrong marker suffix", payload: strings.Replace(runtimeEvidenceCanonicalHelper, " + ' [done]'", "", 1) + call + done, wantError: true},
		{name: "wrong evidence", payload: runtimeEvidenceCanonicalHelper + "payload = local_json('show live command | json compact', 'show wrong command')\n" + done, wantError: true},
		{name: "dynamic evidence", payload: runtimeEvidenceCanonicalHelper + "evidence = 'show live command'\npayload = local_json('show live command | json compact', evidence)\n" + done, wantError: true},
		{name: "nested source-only call", payload: runtimeEvidenceCanonicalHelper + "if False:\n    payload = local_json('show live command | json compact', 'show live command')\n" + done, wantError: true},
		{name: "nested assignment expression call", payload: runtimeEvidenceCanonicalHelper + "payloads = [local_json('show live command | json compact', 'show live command')]\n" + done, wantError: true},
		{name: "aliased helper call", payload: runtimeEvidenceCanonicalHelper + "invoke = local_json\n" + call + "extra = invoke('show extra | json compact', 'show extra')\n" + done, wantError: true},
		{name: "completion literal spoof", payload: runtimeEvidenceCanonicalHelper + "decoy = 'OK: fixture complete'\n" + call + done, wantError: true},
		{name: "missing completion marker", payload: runtimeEvidenceCanonicalHelper + call, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runtimeEvidenceParsePython([]byte(test.payload))
			if test.wantError {
				if err == nil {
					t.Fatalf("parse spoofed fixture succeeded with %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if !slices.Equal(got.Invocations, test.want) {
				t.Fatalf("invocations = %+v, want %+v", got.Invocations, test.want)
			}
			if got.CompletionMarker != "OK: fixture complete" {
				t.Fatalf("completion marker = %q, want fixture marker", got.CompletionMarker)
			}
		})
	}
}

func TestRuntimeEvidenceRunnerContractRejectsWeakenedMarkers(t *testing.T) {
	invocations := []runtimeEvidenceInvocation{
		{Command: "show zebra | json compact", Evidence: "show zebra"},
		{Command: "show alpha | json compact", Evidence: "show alpha"},
	}
	body := "zebra = local_json('show zebra | json compact', 'show zebra')\n" +
		"alpha = local_json('show alpha | json compact', 'show alpha')\n"
	valid := string(runtimeEvidenceCanonicalScenario(body, invocations, "OK: fixture complete"))
	alpha := "expect=stdout:contains=" + runtimeEvidenceMarker("show alpha") + "\n"
	zebra := "expect=stdout:contains=" + runtimeEvidenceMarker("show zebra") + "\n"
	completion := "expect=stdout:contains=OK: fixture complete\n"
	tests := []struct {
		name      string
		scenario  string
		wantError bool
	}{
		{name: "canonical", scenario: valid},
		{name: "missing marker", scenario: strings.Replace(valid, alpha, "", 1), wantError: true},
		{name: "extra marker", scenario: valid + "expect=stdout:contains=COVERED: show invented [done]\n", wantError: true},
		{name: "markers not sorted", scenario: strings.Replace(valid, alpha+zebra, zebra+alpha, 1), wantError: true},
		{name: "completion before markers", scenario: strings.Replace(valid, zebra+completion, completion+zebra, 1), wantError: true},
		{name: "missing successful exit", scenario: strings.Replace(valid, "expect=exit:code=0\n", "", 1), wantError: true},
		{name: "wrong launch", scenario: strings.Replace(valid, runtimeEvidenceRunCommand, "python3 alternate.py", 1), wantError: true},
		{name: "missing timeout", scenario: strings.Replace(valid, runtimeEvidenceTimeoutOption+"\n", "", 1), wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runtimeEvidenceParseFunctionalScenario([]byte(test.scenario))
			if test.wantError {
				if err == nil {
					t.Fatalf("weakened scenario succeeded with %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse canonical scenario: %v", err)
			}
		})
	}
}

func TestRuntimeEvidenceLongestPrefixRejectsBorrowedChildMarker(t *testing.T) {
	registered := map[string]string{"show": "show.go", "show config": "config.go", "show config history": "history.go"}
	command := "show config history pipe-local.conf | json compact"
	if got := runtimeEvidenceLongestProductionPrefix(command, registered); got != "show config history" {
		t.Fatalf("longest registration = %q, want show config history", got)
	}
	if marker := runtimeEvidenceMarker("show config history"); marker != "COVERED: show config history [done]" {
		t.Fatalf("terminal evidence marker = %q", marker)
	}
}

func runtimeEvidenceCommittedScenario(t *testing.T, root string) runtimeEvidenceParsedPython {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(runtimeEvidenceScenarioRelativePath))
	content, err := os.ReadFile(path) //nolint:gosec // Repository-owned functional scenario.
	if err != nil {
		t.Fatalf("read interpreted runtime-evidence scenario: %v", err)
	}
	parsed, err := runtimeEvidenceParseFunctionalScenario(content)
	if err != nil {
		t.Fatalf("parse interpreted runtime-evidence scenario: %v", err)
	}
	return parsed
}

func runtimeEvidenceRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve runtime-evidence test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", ".."))
}

func runtimeEvidenceLongestProductionPrefix(command string, registered map[string]string) string {
	longest := ""
	for path := range registered {
		if command != path && !strings.HasPrefix(command, path+" ") {
			continue
		}
		if len(path) > len(longest) {
			longest = path
		}
	}
	return longest
}

func runtimeEvidenceMarker(evidence string) string { return "COVERED: " + evidence + " [done]" }

func runtimeEvidenceDistinctMarkers(invocations []runtimeEvidenceInvocation) []string {
	distinct := make(map[string]struct{}, len(invocations))
	for _, invocation := range invocations {
		distinct[runtimeEvidenceMarker(invocation.Evidence)] = struct{}{}
	}
	markers := make([]string, 0, len(distinct))
	for marker := range distinct {
		markers = append(markers, marker)
	}
	sort.Strings(markers)
	return markers
}

func runtimeEvidenceCanonicalScenario(body string, invocations []runtimeEvidenceInvocation, completion string) []byte {
	return []byte("tmpfs=run.py:terminator=PY\n" + runtimeEvidenceCanonicalHelper + body +
		"print(" + strconv.Quote(completion) + ")\nPY\n" + runtimeEvidenceCanonicalDirectives(invocations, completion))
}

func runtimeEvidenceCanonicalDirectives(invocations []runtimeEvidenceInvocation, completion string) string {
	var directives strings.Builder
	directives.WriteString(runtimeEvidenceTimeoutOption + "\n")
	directives.WriteString(runtimeEvidenceRunDirective)
	directives.WriteString("expect=exit:code=0\n")
	for _, marker := range runtimeEvidenceDistinctMarkers(invocations) {
		directives.WriteString("expect=stdout:contains=" + marker + "\n")
	}
	directives.WriteString("expect=stdout:contains=" + completion + "\n")
	return directives.String()
}

func runtimeEvidenceParseFunctionalScenario(content []byte) (runtimeEvidenceParsedPython, error) {
	dir, err := os.MkdirTemp("", "ze-runtime-evidence-")
	if err != nil {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("create isolated scenario directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module runtime.evidence\n"), 0o600); err != nil {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("materialize isolated module: %w", err)
	}
	candidate := filepath.Join(dir, "candidate.ci")
	if err := os.WriteFile(candidate, content, 0o600); err != nil {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("materialize scenario: %w", err)
	}
	discovered := runner.NewEncodingTests(dir)
	if err := discovered.Discover(dir); err != nil {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("discover scenario with production runner: %w", err)
	}
	records := discovered.Registered()
	if len(records) != 1 {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("production runner discovered %d scenarios, want 1", len(records))
	}
	record := records[0]
	if record.ParseFailed || record.Error != nil {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("production runner rejected scenario: %v", record.Error)
	}
	scenario, err := tmpfs.Parse(bytes.NewReader(content))
	if err != nil {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("locate run.py in scenario: %w", err)
	}
	if err := runtimeEvidenceValidateOptions(record, scenario.OtherLines); err != nil {
		return runtimeEvidenceParsedPython{}, err
	}
	if record.SkipReason != "" {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("production runner skipped scenario: %s", record.SkipReason)
	}
	if err := runtimeEvidenceValidateOrchestration(record); err != nil {
		return runtimeEvidenceParsedPython{}, err
	}
	var payload []byte
	runPayloads := 0
	for _, file := range scenario.Files {
		if file.Path == "run.py" {
			runPayloads++
			payload = file.Content
		}
	}
	if runPayloads != 1 {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("scenario has %d run.py payloads, want 1", runPayloads)
	}
	parsed, err := runtimeEvidenceParsePython(payload)
	if err != nil {
		return runtimeEvidenceParsedPython{}, err
	}
	if err := runtimeEvidenceValidateAssertions(record, parsed, scenario.OtherLines); err != nil {
		return runtimeEvidenceParsedPython{}, err
	}
	return parsed, nil
}

func runtimeEvidenceValidateOptions(record *runner.Record, lines []string) error {
	options := make([]string, 0, 1)
	for _, line := range lines {
		if strings.HasPrefix(line, "option=") {
			options = append(options, line)
		}
	}
	if len(options) != 1 || options[0] != runtimeEvidenceTimeoutOption {
		return fmt.Errorf("scenario options are %q, want exactly %q", options, runtimeEvidenceTimeoutOption)
	}
	if record.Extra["timeout"] != runtimeEvidenceTimeout || len(record.Extra) != 1 {
		return fmt.Errorf("production runner options are %v, want only timeout %s", record.Extra, runtimeEvidenceTimeout)
	}
	return nil
}

func runtimeEvidenceValidateOrchestration(record *runner.Record) error {
	if len(record.RunCommands) != 1 {
		return fmt.Errorf("scenario has %d run commands, want 1", len(record.RunCommands))
	}
	command := record.RunCommands[0]
	if command.Mode != "foreground" || command.Seq != 1 || command.Exec != runtimeEvidenceRunCommand ||
		command.Stdin != "" || command.Name != "" || command.Signal != "" || command.ExitCode != nil || command.Timeout != "" {
		return fmt.Errorf("scenario has noncanonical run command: %+v", command)
	}
	if len(record.StdinBlocks) != 0 || len(record.Messages) != 0 || len(record.Expects) != 0 ||
		len(record.HTTPChecks) != 0 || len(record.HTTPWaits) != 0 || len(record.EngineSteps) != 0 || len(record.FileChecks) != 0 {
		return fmt.Errorf("scenario has competing runner orchestration")
	}
	return nil
}

func runtimeEvidenceValidateAssertions(record *runner.Record, parsed runtimeEvidenceParsedPython, lines []string) error {
	exits := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "expect=exit:") {
			if line != "expect=exit:code=0" {
				return fmt.Errorf("scenario exit directive is %q", line)
			}
			exits++
		}
	}
	if exits != 1 || record.ExpectExitCode == nil || *record.ExpectExitCode != 0 {
		return fmt.Errorf("scenario must have exactly one successful file exit assertion")
	}
	want := append(runtimeEvidenceDistinctMarkers(parsed.Invocations), parsed.CompletionMarker)
	if !slices.Equal(record.ExpectStdoutMatch, want) {
		return fmt.Errorf("stdout assertions are %q, want ordered runtime evidence %q", record.ExpectStdoutMatch, want)
	}
	if len(record.ExpectStdoutNotMatch) != 0 || len(record.ExpectStdoutRegex) != 0 || len(record.RejectStdoutRegex) != 0 ||
		len(record.ExpectStderrMatch) != 0 || len(record.ExpectStderr) != 0 || len(record.RejectStderr) != 0 ||
		len(record.ExpectSyslog) != 0 || len(record.RejectSyslog) != 0 || record.AwaitStderr != "" || record.AwaitStderrTimeout != "" {
		return fmt.Errorf("scenario has competing output assertions")
	}
	return nil
}

const runtimeEvidencePythonASTParser = `
import ast
import json
import sys

class ContractError(Exception):
    pass

def reject(message):
    raise ContractError(message)

def named_call(node, name):
    return isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id == name

def local_json_call(node):
    return named_call(node, "local_json")

def print_call(node):
    return isinstance(node, ast.Call) and ((isinstance(node.func, ast.Name) and node.func.id == "print") or (isinstance(node.func, ast.Attribute) and node.func.attr == "print"))

def stdout_write_call(node):
    if not isinstance(node, ast.Call) or not isinstance(node.func, ast.Attribute) or node.func.attr != "write":
        return False
    receiver = node.func.value
    while isinstance(receiver, ast.Attribute):
        if receiver.attr in ("stdout", "__stdout__"):
            return True
        receiver = receiver.value
    return isinstance(receiver, ast.Name) and receiver.id in ("stdout", "__stdout__")

def command_template(node):
    if isinstance(node, ast.Constant) and isinstance(node.value, str):
        return node.value
    if isinstance(node, ast.BinOp) and isinstance(node.op, ast.Mod) and isinstance(node.left, ast.Constant) and isinstance(node.left.value, str):
        return node.left.value
    reject("local_json command must be a literal or literal-left percent template")

def import_binding(node, alias):
    if alias.asname is not None:
        return alias.asname
    if isinstance(node, ast.Import):
        return alias.name.split(".", 1)[0]
    return alias.name

def parse_payload(source):
    tree = ast.parse(source, filename="run.py", mode="exec")
    helpers = [statement for statement in tree.body if isinstance(statement, ast.FunctionDef) and statement.name == "local_json"]
    if len(helpers) != 1:
        reject("run.py must define exactly one top-level local_json helper")
    helper = helpers[0]
    expected_helper = ast.parse("""def local_json(command, evidence):
    require(evidence and command.startswith(evidence), 'invalid evidence')
    require('| ' in command, 'local command has no real pipe')
    code, out, err = run(['ze', 'cli', '-c', command])
    require(code == 0, 'local command failed')
    value = json.loads(out)
    print('COVERED: ' + evidence + ' [done]')
    return value
""").body[0]
    if ast.dump(helper, include_attributes=False) != ast.dump(expected_helper, include_attributes=False):
        reject("local_json helper does not match the runtime-evidence contract")
    for node in ast.walk(tree):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)) and node.name == "local_json" and node is not helper:
            reject("run.py defines another local_json binding")
        if isinstance(node, (ast.Import, ast.ImportFrom)):
            for alias in node.names:
                if import_binding(node, alias) == "local_json":
                    reject("run.py imports a local_json binding")
        if isinstance(node, ast.Name) and node.id == "local_json" and isinstance(node.ctx, (ast.Store, ast.Del)):
            reject("run.py stores or deletes a local_json binding")
        if isinstance(node, ast.arg) and node.arg == "local_json":
            reject("run.py declares a local_json argument")
        if isinstance(node, (ast.Global, ast.Nonlocal)) and "local_json" in node.names:
            reject("run.py declares local_json global or nonlocal")
        if isinstance(node, ast.ExceptHandler) and node.name == "local_json":
            reject("run.py binds local_json in an except handler")
        if isinstance(node, (ast.MatchAs, ast.MatchStar)) and node.name == "local_json":
            reject("run.py binds local_json in a match pattern")
    if not tree.body:
        reject("run.py is empty")
    final_statement = tree.body[-1]
    if not isinstance(final_statement, ast.Expr) or not named_call(final_statement.value, "print"):
        reject("run.py must end with one top-level literal OK print")
    completion_call = final_statement.value
    if len(completion_call.args) != 1 or completion_call.keywords or not isinstance(completion_call.args[0], ast.Constant) or not isinstance(completion_call.args[0].value, str) or not completion_call.args[0].value.startswith("OK:"):
        reject("run.py must end with one top-level literal OK print")
    completion_literal = completion_call.args[0]
    helper_nodes = {id(node) for node in ast.walk(helper)}
    helper_marker_call = helper.body[5].value
    helper_marker_literal = helper_marker_call.args[0].left.left
    for node in ast.walk(tree):
        if print_call(node) and node not in (helper_marker_call, completion_call):
            reject("run.py has a print call outside the authorized sites")
        if stdout_write_call(node) and id(node) not in helper_nodes:
            reject("run.py has a stdout.write call outside local_json")
        if isinstance(node, ast.Constant) and isinstance(node.value, str):
            if "COVERED:" in node.value and node is not helper_marker_literal:
                reject("run.py has a COVERED literal outside local_json")
            if "OK:" in node.value and node is not completion_literal:
                reject("run.py has an OK literal outside the final print")
    helper_index = tree.body.index(helper)
    allowed_calls = []
    invocations = []
    for index, statement in enumerate(tree.body[:-1]):
        if not isinstance(statement, ast.Assign) or not local_json_call(statement.value):
            continue
        if index <= helper_index:
            reject("local_json assignment appears before its helper definition")
        call = statement.value
        if len(call.args) != 2 or call.keywords:
            reject("local_json assignment must have command and evidence arguments")
        command = command_template(call.args[0])
        evidence_node = call.args[1]
        if not isinstance(evidence_node, ast.Constant) or not isinstance(evidence_node.value, str) or not evidence_node.value:
            reject("local_json evidence must be a nonempty literal")
        evidence = evidence_node.value
        if not command.startswith(evidence):
            reject("local_json command must start with its evidence")
        allowed_calls.append(call)
        invocations.append({"command": command, "evidence": evidence})
    if not invocations:
        reject("run.py has no executable local_json assignment")
    allowed_call_ids = {id(call) for call in allowed_calls}
    allowed_name_ids = {id(call.func) for call in allowed_calls}
    for node in ast.walk(tree):
        if local_json_call(node) and id(node) not in allowed_call_ids:
            reject("local_json call is not a direct top-level assignment")
        if isinstance(node, ast.Name) and node.id == "local_json" and isinstance(node.ctx, ast.Load) and id(node) not in allowed_name_ids:
            reject("local_json helper is referenced outside an authorized call")
    return {"invocations": invocations, "completion_marker": completion_literal.value}

try:
    parsed = parse_payload(sys.stdin.read())
except (SyntaxError, ContractError) as error:
    print(error, file=sys.stderr)
    raise SystemExit(1)
json.dump(parsed, sys.stdout, ensure_ascii=False, separators=(",", ":"))
`

func runtimeEvidenceParsePython(payload []byte) (runtimeEvidenceParsedPython, error) {
	command := exec.Command("python3", "-c", runtimeEvidencePythonASTParser)
	command.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return runtimeEvidenceParsedPython{}, fmt.Errorf("parse run.py AST: %s", detail)
	}
	var parsed runtimeEvidenceParsedPython
	decoder := json.NewDecoder(&stdout)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("decode run.py AST result: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("decode run.py AST result: trailing output")
	}
	if len(parsed.Invocations) == 0 || parsed.CompletionMarker == "" {
		return runtimeEvidenceParsedPython{}, fmt.Errorf("decode run.py AST result: incomplete result")
	}
	return parsed, nil
}
