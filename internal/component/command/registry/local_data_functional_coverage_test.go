package registry_test

import (
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

var localDataInvocation = regexp.MustCompile(`local_json\('([^']+)'`)

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
	live := []byte("local_json('show env list | json compact')\n")
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
	matches := localDataInvocation.FindAllSubmatch(content, -1)
	if len(matches) == 0 {
		t.Fatalf("%s has no executable local_json cases", path)
	}
	invocations := make([]string, 0, len(matches))
	for _, match := range matches {
		invocations = append(invocations, string(match[1]))
	}
	return invocations
}
