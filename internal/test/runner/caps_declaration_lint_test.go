package runner

// VALIDATES: each compiled fixture that mutates kernel networking through the
// iproute2 CLI has a tracked .ci caller declaring net-admin.
// PREVENTS: an unprivileged host running the fixture and reporting a product
// failure when the kernel refused the test setup.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type fixtureFunctionFacts struct {
	calls      map[string]bool
	privileged bool
}

var iprouteDomains = map[string]bool{
	"link": true, "addr": true, "address": true, "route": true,
	"rule": true, "neigh": true, "netns": true,
}

var iprouteMutations = map[string]bool{
	"add": true, "set": true, "del": true, "delete": true,
	"replace": true, "change": true, "append": true,
}

func TestNativeIPRouteFixturesDeclareNetAdmin(t *testing.T) {
	root := repoRootForTest(t)
	privileged, err := privilegedFixtureNames(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(privileged) == 0 {
		t.Fatal("no compiled fixture is recognized as an iproute2 mutator")
	}
	callers, err := nativeFixtureCallers(root)
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, name := range privileged {
		paths := callers[name]
		if len(paths) == 0 {
			missing = append(missing, fmt.Sprintf("%s has no tracked .ci caller", name))
			continue
		}
		for _, path := range paths {
			raw, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !declaresNetAdmin(string(raw)) {
				missing = append(missing, fmt.Sprintf("%s calls %s without option=needs-linux:caps=net-admin", path, name))
			}
		}
	}
	if len(missing) != 0 {
		t.Fatalf("compiled privileged fixtures are not capability-gated:\n  %s", strings.Join(missing, "\n  "))
	}
}

func privilegedFixtureNames(root string) ([]string, error) {
	dir := filepath.Join(root, "internal", "test", "fixture")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	functions := make(map[string]*fixtureFunctionFacts)
	registrations := make(map[string]map[string]bool)
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			facts := inspectFixtureExpression(function.Body)
			functions[function.Name.Name] = facts
			if function.Name.Name == "init" {
				collectFixtureRegistrations(function.Body, registrations)
			}
		}
	}

	changed := true
	for changed {
		changed = false
		for _, facts := range functions {
			if facts.privileged {
				continue
			}
			for called := range facts.calls {
				if target := functions[called]; target != nil && target.privileged {
					facts.privileged = true
					changed = true
					break
				}
			}
		}
	}

	var names []string
	for name, seeds := range registrations {
		for seed := range seeds {
			if facts := functions[seed]; facts != nil && facts.privileged {
				names = append(names, name)
				break
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

func inspectFixtureExpression(node ast.Node) *fixtureFunctionFacts {
	facts := &fixtureFunctionFacts{calls: make(map[string]bool)}
	ast.Inspect(node, func(current ast.Node) bool {
		call, ok := current.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch function := call.Fun.(type) {
		case *ast.Ident:
			facts.calls[function.Name] = true
		case *ast.SelectorExpr:
			facts.calls[function.Sel.Name] = true
		}
		words := stringLiterals(call)
		if hasWord(words, "ip") && hasAnyWord(words, iprouteDomains) && hasAnyWord(words, iprouteMutations) {
			facts.privileged = true
		}
		return true
	})
	return facts
}

func collectFixtureRegistrations(body *ast.BlockStmt, registrations map[string]map[string]bool) {
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok || name.Name != "Register" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		fixtureName, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		seeds := make(map[string]bool)
		for _, argument := range call.Args[1:] {
			ast.Inspect(argument, func(current ast.Node) bool {
				if identifier, held := current.(*ast.Ident); held {
					seeds[identifier.Name] = true
				}
				return true
			})
		}
		registrations[fixtureName] = seeds
		return true
	})
}

func stringLiterals(node ast.Node) map[string]bool {
	words := make(map[string]bool)
	ast.Inspect(node, func(current ast.Node) bool {
		literal, ok := current.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err == nil {
			words[value] = true
		}
		return true
	})
	return words
}

func hasWord(words map[string]bool, wanted string) bool { return words[wanted] }

func hasAnyWord(words, wanted map[string]bool) bool {
	for word := range wanted {
		if words[word] {
			return true
		}
	}
	return false
}

func nativeFixtureCallers(root string) (map[string][]string, error) {
	callers := make(map[string][]string)
	base := filepath.Join(root, "test")
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if isDraftPath(base, path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".ci") {
			return nil
		}
		raw, err := os.ReadFile(path) //nolint:gosec // repository test path
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for line := range strings.SplitSeq(string(raw), "\n") {
			at := strings.Index(line, "ze-test fixture ")
			if at < 0 {
				continue
			}
			fields := strings.Fields(line[at:])
			if len(fields) < 3 {
				continue
			}
			name, _, _ := strings.Cut(strings.Trim(fields[2], `"'`), ":")
			callers[name] = append(callers[name], filepath.ToSlash(rel))
		}
		return nil
	})
	return callers, err
}

func declaresNetAdmin(raw string) bool {
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "option=needs-linux") {
			continue
		}
		for field := range strings.SplitSeq(line, ":") {
			caps, ok := strings.CutPrefix(field, "caps=")
			if !ok {
				continue
			}
			for capability := range strings.SplitSeq(caps, ",") {
				if strings.TrimSpace(capability) == capsNetAdmin {
					return true
				}
			}
		}
	}
	return false
}

func TestNativeIPRouteDetectorDiscriminates(t *testing.T) {
	source := `package fixture
func driver(ctx context.Context) error {
    return rawCommand(ctx, "ip", "link", "add", "dummy0", "type", "dummy")
}`
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	facts := inspectFixtureExpression(file)
	if !facts.privileged {
		t.Fatal("native iproute2 mutation was not detected")
	}
}
