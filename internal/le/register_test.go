// VALIDATES: the one le composition imports every live registering package.
// PREVENTS: a new tool existing on disk but disappearing from both personalities.
package le

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

const leImportPrefix = "github.com/ze-software/ze/internal/le/"

var commandsAtStart = leroot.Commands()

func TestCompositionEqualsLiveRegisteringPackagePopulation(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}

	imports := blankImports(t, filepath.Join(root, "internal", "le", "register.go"))
	registering := registeringPackages(t, filepath.Join(root, "internal", "le"))
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
	for _, tool := range commandsAtStart {
		if registry.LookupRoot(tool.Name) != nil {
			t.Errorf("tool %q is also a top-level root", tool.Name)
		}
		if tool.Meta.Description == "" || tool.Meta.Mode == "" || tool.Meta.Section == "" {
			t.Errorf("tool %q has incomplete help metadata: %#v", tool.Name, tool.Meta)
		}
	}
}

func TestDuplicateLeRootIsRejected(t *testing.T) {
	handler := func(*registry.RuntimeContext, []string) int { return 0 }
	meta := registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest}
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
		if !registry.HasLocal(path) {
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

func blankImports(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "_" {
			continue
		}
		path, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			t.Fatalf("unquote import %s: %v", spec.Path.Value, unquoteErr)
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
// `le verify lint` registers from internal/le/verify/lint. An object that is
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

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	packages := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if registers(filepath.Join(dir, entry.Name())) {
			packages = append(packages, leImportPrefix+entry.Name())
		}
		members, readErr := os.ReadDir(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			continue
		}
		for _, member := range members {
			if !member.IsDir() {
				continue
			}
			if registers(filepath.Join(dir, entry.Name(), member.Name())) {
				packages = append(packages, leImportPrefix+entry.Name()+"/"+member.Name())
			}
		}
	}
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
