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
	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	lescratch "github.com/ze-software/ze/internal/le/scratch"
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

// blankImports answers the registering packages the composition imports: every
// blank import, and a named import of a package that registers. The root
// handler calls the scratch package's store-trim trigger, so that one
// registering package is imported by name, and a second, blank import of it
// would be the duplicate gocritic's dupImport refuses.
func blankImports(t *testing.T, compositionPath string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), compositionPath, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", compositionPath, err)
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		if spec.Name == nil {
			continue
		}
		path, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			t.Fatalf("unquote import %s: %v", spec.Path.Value, unquoteErr)
		}
		if spec.Name.Name != "_" && !namedRegistering(t, filepath.Dir(compositionPath), path) {
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

// namedRegistering reports whether a named import under the le prefix is a
// package with a register.go, below leDir (internal/le).
func namedRegistering(t *testing.T, leDir, importPath string) bool {
	t.Helper()
	member, under := strings.CutPrefix(importPath, leImportPrefix)
	if !under {
		return false
	}
	_, statErr := os.Stat(filepath.Join(leDir, filepath.FromSlash(member), "register.go"))
	if statErr != nil && !os.IsNotExist(statErr) {
		t.Fatalf("stat %s/register.go: %v", member, statErr)
	}
	return statErr == nil
}

func firstDuplicate(values []string) string {
	for index := 1; index < len(values); index++ {
		if values[index] == values[index-1] {
			return values[index]
		}
	}
	return ""
}

// TestRunSpawnsStoreTrimWhenDue proves the root handler every le command and
// hook passes through starts the store trim when the hourly stamp is due (AC-1
// trigger), not when it is fresh (AC-2), and not under ze.le.store.trim=off
// (AC-15), and that the trigger changes neither the exit code nor the output of
// the command it rides on.
//
// Method: ZE_REPO_ROOT names a throwaway checkout and the spawn seam records
// the root it is handed, so no child starts. Each run is compared, exit code
// and captured stdout and stderr, with leroot.Dispatch of the same words, which
// is the handler without the trigger.
func TestRunSpawnsStoreTrimWhenDue(t *testing.T) {
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("ZE_LE_STORE_TRIM", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	var spawned []string
	original := storeTrimSpawn
	storeTrimSpawn = func() lescratch.TrimSpawn {
		return func(root string) error {
			spawned = append(spawned, root)
			return nil
		}
	}
	t.Cleanup(func() { storeTrimSpawn = original })

	words := []string{"scratch", "no-such-verb"}
	wantCode, wantOutput := captureOutput(t, func() int { return leroot.Dispatch(invocationName(), words) })

	runOnce := func(t *testing.T) {
		t.Helper()
		code, output := captureOutput(t, func() int { return run(nil, words) })
		if code != wantCode {
			t.Errorf("exit code = %d, want %d (the trigger changed it)", code, wantCode)
		}
		if output != wantOutput {
			t.Errorf("output = %q, want %q (the trigger printed)", output, wantOutput)
		}
	}

	runOnce(t)
	if len(spawned) != 1 || spawned[0] != resolved {
		t.Fatalf("due stamp: spawned %q, want one trim of %s", spawned, resolved)
	}
	if _, statErr := os.Stat(filepath.Join(resolved, "tmp", "store-trim", "stamp")); statErr != nil {
		t.Fatalf("the trigger wrote no stamp: %v", statErr)
	}

	runOnce(t)
	if len(spawned) != 1 {
		t.Fatalf("fresh stamp: spawned %d trims, want still 1", len(spawned))
	}

	if removeErr := os.Remove(filepath.Join(resolved, "tmp", "store-trim", "stamp")); removeErr != nil {
		t.Fatal(removeErr)
	}
	t.Setenv("ZE_LE_STORE_TRIM", "off")
	env.ResetCache()
	runOnce(t)
	if len(spawned) != 1 {
		t.Fatalf("ze.le.store.trim=off: spawned %d trims, want still 1", len(spawned))
	}
}

// captureOutput runs call with stdout and stderr sent to one file, and answers
// its exit code and everything it wrote.
func captureOutput(t *testing.T, call func() int) (int, string) {
	t.Helper()
	sink, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close() //nolint:errcheck // read back below
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = sink, sink
	code := call()
	os.Stdout, os.Stderr = stdout, stderr
	data, err := os.ReadFile(sink.Name())
	if err != nil {
		t.Fatal(err)
	}
	return code, string(data)
}
