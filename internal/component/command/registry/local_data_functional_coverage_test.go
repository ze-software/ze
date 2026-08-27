package registry_test

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var (
	localDataAssignment = regexp.MustCompile(
		`^[A-Za-z_][A-Za-z0-9_]*[ \t]*=[ \t]*local_json\([ \t]*(?:'([^']*)'|"([^"]*)")`,
	)
	localDataSuccessMarker = regexp.MustCompile(`^print\([ \t]*['"]OK:`)
)

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
// the static live-scenario contract cannot be satisfied by inert text.
//
// VALIDATES: IR3-6 -- only top-level assignments executed before success count.
// PREVENTS: comments, dead blocks, and unrelated scenario text faking coverage.
func TestFunctionalLocalDataInvocationsRequireExecutedTopLevelAssignments(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		want     []string
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
				"payload = local_json('show late fake | json compact')\n" +
				"PY\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseFunctionalLocalDataInvocations([]byte(test.scenario))
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Fatalf("invocations = %q, want %q", got, test.want)
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
	var invocations []string
	terminator := ""
	inPayload := false
	completed := false
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
			if !completed {
				return nil, fmt.Errorf("run.py payload ends without a top-level OK completion marker")
			}
			return invocations, nil
		}
		if completed {
			continue
		}
		if localDataSuccessMarker.MatchString(line) {
			completed = true
			continue
		}
		match := localDataAssignment.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		command := match[1]
		if command == "" {
			command = match[2]
		}
		invocations = append(invocations, command)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan functional scenario: %w", err)
	}
	if !inPayload {
		return nil, fmt.Errorf("functional scenario has no tmpfs=run.py payload")
	}
	return nil, fmt.Errorf("run.py payload has no %q terminator", terminator)
}
