// Related: hidden.go -- the hidden set this test pins against what the tests spawn

package leroot_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// spawnedRun matches a daemon config's plugin `run` statement that starts an
// le mock, as a functional test declares it:
// `run "le mock engine-steps ./engine-steps.json"`.
var spawnedRun = regexp.MustCompile(`^\s*run\s+"(?:[^"]*\s)?le (mock [a-z][a-z-]*)`)

// VALIDATES: AC-23 over the real registry -- a mock started by a test daemon's
// config or by a descriptor-fed native test launcher is hidden, and every
// other mock is not. The population comes from config run statements and
// actual non-test Go launch sites, not from the hidden registrations.
// PREVENTS: a spawned helper whose register.go dropped leroot.RegisterHidden,
// so help, completion and the menu offer a person a command only Ze types,
// and a mock a person starts by hand hidden from them.
func TestSpawnedMocksAreExactlyTheHiddenOnes(t *testing.T) {
	spawned := spawnedMocks(t)
	if len(spawned) == 0 {
		t.Fatal("no functional test starts a private le mock, so the hidden set is unpinned")
	}
	registered := make(map[string]bool, 16)
	for _, command := range leroot.Commands() {
		if !strings.HasPrefix(command.Name, "mock ") {
			continue
		}
		registered[command.Name] = true
		if spawned[command.Name] && !leroot.Hidden(command.Name) {
			t.Errorf("`le %s` is started by a test daemon or descriptor-fed launcher and is not hidden: its register.go MUST call leroot.RegisterHidden", command.Name)
		}
		if !spawned[command.Name] && leroot.Hidden(command.Name) {
			t.Errorf("`le %s` is hidden and no test daemon or descriptor-fed launcher starts it, so a person who starts it finds no help", command.Name)
		}
	}
	for name := range spawned {
		if !registered[name] {
			t.Errorf("a functional test spawns `le %s`, which le does not register", name)
		}
	}
}

// spawnedMocks answers the config-owned and descriptor-fed native workers.
func spawnedMocks(t *testing.T) map[string]bool {
	t.Helper()
	return spawnedMocksAt(t, repositoryRoot(t))
}

// spawnedMocksAt reads both launcher populations independently of registration.
func spawnedMocksAt(t *testing.T, root string) map[string]bool {
	t.Helper()
	spawned := make(map[string]bool, 4)
	err := filepath.WalkDir(filepath.Join(root, "test"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".ci" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for line := range strings.SplitSeq(string(body), "\n") {
			if match := spawnedRun.FindStringSubmatch(line); match != nil {
				spawned[match[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read the functional tests: %v", err)
	}
	err = filepath.WalkDir(filepath.Join(root, "internal", "test"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for name := range descriptorSpawnedMocks(file) {
			spawned[name] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read the native test launchers: %v", err)
	}
	return spawned
}

// descriptorSpawnedMocks follows constructor instances in source order, so
// reassigning a local cannot transfer another process's descriptors or start.
func descriptorSpawnedMocks(file *ast.File) map[string]bool {
	imports := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = path
	}
	type launch struct {
		name        string
		descriptors bool
	}
	commands := make(map[*ast.Object]*launch)
	spawned := make(map[string]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		if assignment, ok := node.(*ast.AssignStmt); ok {
			for index, left := range assignment.Lhs {
				if local, ok := left.(*ast.Ident); ok {
					delete(commands, local.Obj)
					if len(assignment.Lhs) == len(assignment.Rhs) {
						if call, ok := assignment.Rhs[index].(*ast.CallExpr); ok {
							if name := invokedMock(call, imports); name != "" && local.Obj != nil {
								commands[local.Obj] = &launch{name: name}
							}
						}
					}
					continue
				}
				if len(assignment.Lhs) != len(assignment.Rhs) {
					continue
				}
				field, ok := left.(*ast.SelectorExpr)
				if !ok || field.Sel.Name != "ExtraFiles" {
					continue
				}
				local, ok := field.X.(*ast.Ident)
				if !ok {
					continue
				}
				if command := commands[local.Obj]; command != nil {
					command.descriptors = inheritedFiles(assignment.Rhs[index])
				}
			}
		}
		if call, ok := node.(*ast.CallExpr); ok {
			if local := startedMock(call, imports); local != nil {
				if command := commands[local.Obj]; command != nil {
					if command.descriptors {
						// Preserve this launch even when the variable is reused later.
						spawned[command.name] = true
					}
				}
			}
		}
		return true
	})
	return spawned
}

// invokedMock reads only an os/exec constructor's literal mock command words.
func invokedMock(call *ast.CallExpr, imports map[string]string) string {
	function, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	owner, ok := function.X.(*ast.Ident)
	if !ok || imports[owner.Name] != "os/exec" {
		return ""
	}
	first := 1
	switch function.Sel.Name {
	case "Command":
	case "CommandContext":
		first = 2
	default:
		return ""
	}
	if len(call.Args) < first+2 {
		return ""
	}
	if literalWord(call.Args[first]) != "mock" {
		return ""
	}
	name := literalWord(call.Args[first+1])
	if name == "" {
		return ""
	}
	return "mock " + name
}

// literalWord refuses comments, variables and string fragments as launch sites.
func literalWord(expression ast.Expr) string {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return ""
	}
	word, err := strconv.Unquote(literal.Value)
	if err != nil {
		return ""
	}
	return word
}

// inheritedFiles refuses an explicitly nil or empty descriptor assignment.
func inheritedFiles(expression ast.Expr) bool {
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name != "nil"
	}
	if files, ok := expression.(*ast.CompositeLit); ok {
		return len(files.Elts) != 0
	}
	return true
}

// startedMock recognizes actual exec methods and the repository's child starter.
func startedMock(call *ast.CallExpr, imports map[string]string) *ast.Ident {
	function, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	owner, ok := function.X.(*ast.Ident)
	if !ok {
		return nil
	}
	if imports[owner.Name] == "github.com/ze-software/ze/internal/component/plugin" {
		if function.Sel.Name == "StartChild" && len(call.Args) == 1 {
			local, _ := call.Args[0].(*ast.Ident)
			return local
		}
		return nil
	}
	switch function.Sel.Name {
	case "Start", "Run", "Output", "CombinedOutput":
		return owner
	default:
		return nil
	}
}

// TestDescriptorSpawnDiscoveryRequiresOneOwnedLaunch discriminates real native
// workers from public commands, declarations, text and another command's files.
func TestDescriptorSpawnDiscoveryRequiresOneOwnedLaunch(t *testing.T) {
	for _, row := range []struct {
		name string
		body string
		want bool
	}{
		{"command-context", `cmd := child.CommandContext(ctx, executable, "mock", "worker"); cmd.ExtraFiles = handles[:4]; process.StartChild(cmd)`, true},
		{"command", `cmd := child.Command(executable, "mock", "worker"); cmd.ExtraFiles = []*os.File{input}; cmd.Start()`, true},
		{"run", `cmd := child.Command(executable, "mock", "worker"); cmd.ExtraFiles = handles; cmd.Run()`, true},
		{"output", `cmd := child.Command(executable, "mock", "worker"); cmd.ExtraFiles = handles; cmd.Output()`, true},
		{"combined-output", `cmd := child.Command(executable, "mock", "worker"); cmd.ExtraFiles = handles; cmd.CombinedOutput()`, true},
		{"public-server", `cmd := child.CommandContext(ctx, executable, "mock", "worker", "--port", "0"); cmd.Start()`, false},
		{"not-started", `cmd := child.CommandContext(ctx, executable, "mock", "worker"); cmd.ExtraFiles = handles[:4]`, false},
		{"no-descriptors", `cmd := child.CommandContext(ctx, executable, "mock", "worker"); process.StartChild(cmd)`, false},
		{"nil-descriptors", `cmd := child.CommandContext(ctx, executable, "mock", "worker"); cmd.ExtraFiles = nil; cmd.Start()`, false},
		{"empty-descriptors", `cmd := child.CommandContext(ctx, executable, "mock", "worker"); cmd.ExtraFiles = []*os.File{}; cmd.Start()`, false},
		{"different-command", `cmd := child.CommandContext(ctx, executable, "mock", "worker"); other := child.Command("echo"); other.ExtraFiles = handles[:4]; cmd.Start(); other.Start()`, false},
		{"shadowed-command", `cmd := child.CommandContext(ctx, executable, "mock", "worker"); cmd.ExtraFiles = handles[:4]; { cmd := child.Command("echo"); cmd.Start() }`, false},
		{"variable-name", `cmd := child.CommandContext(ctx, executable, "mock", name); cmd.ExtraFiles = handles[:4]; cmd.Start()`, false},
		{"unrelated-constructor", `cmd := unrelated.CommandContext(ctx, executable, "mock", "worker"); cmd.ExtraFiles = handles[:4]; cmd.Start()`, false},
		{"unrelated-starter", `cmd := child.CommandContext(ctx, executable, "mock", "worker"); cmd.ExtraFiles = handles[:4]; unrelated.StartChild(cmd)`, false},
		{"string-decoy", "text := `child.CommandContext(ctx, executable, \"mock\", \"worker\")`; _ = text", false},
		{"comment-decoy", "// cmd := child.CommandContext(ctx, executable, \"mock\", \"worker\")\n// cmd.ExtraFiles = handles[:4]; cmd.Start()", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			source := `package launch
import child "os/exec"
import process "github.com/ze-software/ze/internal/component/plugin"
func run() {
` + row.body + "\n}\n"
			file, err := parser.ParseFile(token.NewFileSet(), row.name+".go", source, 0)
			if err != nil {
				t.Fatal(err)
			}
			spawned := descriptorSpawnedMocks(file)
			if spawned["mock worker"] != row.want {
				t.Errorf("discovered %v, want mock worker=%v", spawned, row.want)
			}
			wantCount := 0
			if row.want {
				wantCount = 1
			}
			if len(spawned) != wantCount {
				t.Errorf("discovered %v, want %d workers", spawned, wantCount)
			}
		})
	}
}

// TestDescriptorSpawnDiscoveryTracksReassignedLaunches keeps evidence with the
// constructor that was started, rather than the last value of its local.
func TestDescriptorSpawnDiscoveryTracksReassignedLaunches(t *testing.T) {
	for _, row := range []struct {
		name string
		body string
		want []string
	}{
		{
			name: "started-before-new-constructor",
			body: `cmd := exec.Command("le", "mock", "first"); cmd.ExtraFiles = handles; cmd.Start(); cmd = exec.Command("le", "mock", "second")`,
			want: []string{"mock first"},
		},
		{
			name: "both-instances-started",
			body: `cmd := exec.Command("le", "mock", "first"); cmd.ExtraFiles = handles; cmd.Start(); cmd = exec.Command("le", "mock", "second"); cmd.ExtraFiles = handles; cmd.Start()`,
			want: []string{"mock first", "mock second"},
		},
		{
			name: "new-instance-has-no-descriptors",
			body: `cmd := exec.Command("le", "mock", "first"); cmd.ExtraFiles = handles; cmd = exec.Command("le", "mock", "second"); cmd.Start()`,
		},
		{
			name: "nonmock-constructor-clears-binding",
			body: `cmd := exec.Command("le", "mock", "first"); cmd.ExtraFiles = handles; cmd = exec.Command("echo"); cmd.Start()`,
		},
		{
			name: "nonconstructor-clears-binding",
			body: `cmd := exec.Command("le", "mock", "first"); cmd.ExtraFiles = handles; cmd = other; cmd.Start()`,
		},
		{
			name: "completed-launch-survives-nonmock-reassignment",
			body: `cmd := exec.Command("le", "mock", "first"); cmd.ExtraFiles = handles; cmd.Start(); cmd = exec.Command("echo"); cmd.Start()`,
			want: []string{"mock first"},
		},
		{
			name: "completed-launch-survives-descriptor-clear",
			body: `cmd := exec.Command("le", "mock", "first"); cmd.ExtraFiles = handles; cmd.Start(); cmd.ExtraFiles = nil`,
			want: []string{"mock first"},
		},
		{
			name: "descriptors-after-start-do-not-qualify",
			body: `cmd := exec.Command("le", "mock", "first"); cmd.Start(); cmd.ExtraFiles = handles`,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			source := "package launch\nimport \"os/exec\"\nfunc run() {\n" + row.body + "\n}\n"
			file, err := parser.ParseFile(token.NewFileSet(), row.name+".go", source, 0)
			if err != nil {
				t.Fatal(err)
			}
			spawned := descriptorSpawnedMocks(file)
			for _, name := range row.want {
				if !spawned[name] {
					t.Errorf("%s was not discovered: %v", name, spawned)
				}
			}
			if len(spawned) != len(row.want) {
				t.Errorf("discovered %v, want %v", spawned, row.want)
			}
		})
	}
}

// TestSpawnedMockDiscoveryUsesBothSources drives the same census as the
// registry guard and refuses launch evidence found only in Go test fixtures.
func TestSpawnedMockDiscoveryUsesBothSources(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"test/native/config.ci": "run \"le mock config-worker\"\n",
		"internal/test/native/parent.go": `package native
import "os/exec"
func run() {
	cmd := exec.Command("le", "mock", "native-worker")
	cmd.ExtraFiles = handles[:4]
	cmd.Start()
}
`,
		"internal/test/native/fixture_test.go": `package native
import "os/exec"
func run() {
	cmd := exec.Command("le", "mock", "test-only")
	cmd.ExtraFiles = handles[:4]
	cmd.Start()
}
`,
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	spawned := spawnedMocksAt(t, root)
	for _, name := range []string{"mock config-worker", "mock native-worker"} {
		if !spawned[name] {
			t.Errorf("%s was not discovered: %v", name, spawned)
		}
	}
	if len(spawned) != 2 {
		t.Errorf("discovered %v, want only the config and native workers", spawned)
	}
}

// repositoryRoot answers the directory holding go.mod above the test's own.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's directory")
		}
		dir = parent
	}
}
