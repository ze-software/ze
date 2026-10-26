// VALIDATES: the one le composition imports every live registering package.
// PREVENTS: a new tool existing on disk but disappearing from both personalities.
package le

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

const leImportPrefix = "github.com/ze-software/ze/internal/le/"

var commandsAtStart = leroot.Commands()

func TestCompositionEqualsLiveRegisteringPackagePopulation(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}

	registering := registeringPackages(t, filepath.Join(root, "internal", "le"))
	imports := compositionImports(t, filepath.Join(root, "internal", "le", "register.go"), registering)
	if len(imports) == 0 {
		t.Fatal("internal/le/register.go blank-imports no registering package")
	}
	if !slices.IsSorted(imports) {
		t.Errorf("composition imports are not sorted: %v", imports)
	}
	if duplicate := firstDuplicate(imports); duplicate != "" {
		t.Errorf("composition imports %s more than once", duplicate)
	}
	if !slices.Equal(imports, registering) {
		t.Errorf("composition differs from live register.go population:\nimports: %v\nlive: %v", imports, registering)
	}
}

func TestLeRegistersOneRootAndNoToolRoots(t *testing.T) {
	if registry.LookupRoot("le") == nil {
		t.Fatal("internal/le registered no le root")
	}
	// The rule is that le registers one root and no tool of its own as a
	// second one. A product root ze registers from outside internal/le may
	// share a tool's name: `le completion` lives under `le`, and `ze
	// completion` is the product's own command (D-P6-root). So a same-named
	// root fails only when its handler is defined inside internal/le.
	for _, tool := range commandsAtStart {
		handler := registry.LookupRoot(tool.Name)
		if handler != nil {
			owner := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
			if owner == nil {
				t.Errorf("tool %q shares a top-level root whose handler has no symbol", tool.Name)
			} else if strings.HasPrefix(owner.Name(), leImportPrefix) {
				t.Errorf("tool %q is also a top-level root, registered by %s", tool.Name, owner.Name())
			}
		}
		if tool.Meta.ShortHelp == "" || tool.Meta.Mode == "" || tool.Meta.Section == "" {
			t.Errorf("tool %q has incomplete help metadata: %#v", tool.Name, tool.Meta)
		}
	}
}

func TestDuplicateLeRootIsRejected(t *testing.T) {
	handler := func(*registry.RuntimeContext, []string) int { return 0 }
	meta := registry.Meta{ShortHelp: "a test probe", Mode: "offline", Section: registry.SectionTest}
	err := registry.RegisterRootHandler("le", handler, meta)
	if !errors.Is(err, registry.ErrRootHandlerDuplicate) {
		t.Fatalf("duplicate le root error = %v, want ErrRootHandlerDuplicate", err)
	}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Error("MustRegisterRootHandler accepted a duplicate le root")
		}
	}()
	registry.MustRegisterRootHandler("le", handler, meta)
}

func TestEveryLeToolUsesFullPathAndParityAloneHasNoShape(t *testing.T) {
	if len(commandsAtStart) == 0 {
		t.Fatal("le registered no local-data command")
	}
	for _, tool := range commandsAtStart {
		path := leroot.CommandPath(tool.Name)
		if !command.HasLocal(path) {
			t.Errorf("tool %q is absent at %q", tool.Name, path)
		}
		_, declared := command.ShapeForCommand(path)
		if tool.Name == "parity" {
			if declared {
				t.Error("parity declares an answer shape; its runtime-derived shape is the explicit exception")
			}
			continue
		}
		if !declared {
			t.Errorf("tool %q has no full-path answer shape", tool.Name)
		}
	}
}

// compositionImports answers the registering packages register.go imports:
// every blank import, and a named or unnamed import of a package in
// registering. run calls scratch by name (ensureCacheLink), and a second,
// blank import of the same path would be a duplicate import; either form runs
// the package's init, which is what puts its tool in the composition.
func compositionImports(t *testing.T, path string, registering []string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		path, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			t.Fatalf("unquote import %s: %v", spec.Path.Value, unquoteErr)
		}
		blank := spec.Name != nil && spec.Name.Name == "_"
		if !blank && !slices.Contains(registering, path) {
			continue
		}
		if !strings.HasPrefix(path, leImportPrefix) {
			t.Errorf("composition blank-imports non-le package %s", path)
		}
		imports = append(imports, path)
	}
	return imports
}

// registeringPackages answers every package under dir that registers a command.
//
// It walks two levels, because a namespace member sits one below its object:
// `le go lint` registers from internal/le/go/lint. An object that is
// also a command of its own, as verify, site and repository are, registers from
// both levels and is counted at each.
func registeringPackages(t *testing.T, dir string) []string {
	t.Helper()

	registers := func(path string) bool {
		_, statErr := os.Stat(filepath.Join(path, "register.go"))
		if statErr != nil && !os.IsNotExist(statErr) {
			t.Fatalf("stat %s/register.go: %v", path, statErr)
		}
		return statErr == nil
	}

	// Each word of a command name is one directory level, so the scan reads
	// every directory down to the deepest name le dispatches
	// (registeringDepth): `setup docker kernel` registers from
	// internal/le/setup/docker/kernel.
	packages := make([]string, 0, len(commandsAtStart))
	var walk func(relative string, depth int)
	walk = func(relative string, depth int) {
		entries, err := os.ReadDir(filepath.Join(dir, relative))
		if err != nil {
			if relative == "" {
				t.Fatalf("read %s: %v", dir, err)
			}
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			child := filepath.Join(relative, entry.Name())
			if registers(filepath.Join(dir, child)) {
				packages = append(packages, leImportPrefix+filepath.ToSlash(child))
			}
			if depth < registeringDepth {
				walk(child, depth+1)
			}
		}
	}
	walk("", 1)
	slices.Sort(packages)
	return packages
}

func firstDuplicate(values []string) string {
	for index := 1; index < len(values); index++ {
		if values[index] == values[index-1] {
			return values[index]
		}
	}
	return ""
}
