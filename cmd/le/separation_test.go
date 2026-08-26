// The separation between the two binaries is the premise the shared engine
// rests on, and nothing about Go stops a developer adding one blank import
// across the line. So it is CHECKED rather than documented.
//
// The rule is DIRECTIONAL, and only the direction that carries risk is
// enforced. ze is what ships, so ze must never link an le package:
// TestZeLinksNoLePlugin reads `go list -deps` over every ze flavor. le is a
// build-host binary nobody deploys and several le tools exist to enumerate what
// ze registers, so le linking the product is required rather than forbidden;
// what le must not do is own a product COMMAND, which is
// TestLeRegistersNoProductCommand.
//
// The dependency check discriminates. Measured 2026-08-26, internal/perf/cli is
// absent from ze's 630-package dependency list with ze_perf off and present
// with it on, so a dependency list can see the difference it exists to see.

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
)

// leTree is the import-path prefix of every package that belongs to le and to
// no other program.
const leTree = "github.com/ze-software/ze/letools/"

// engineRegistry is the one registry both binaries dispatch through.
const engineRegistry = "github.com/ze-software/ze/internal/component/command/registry"

// zeFlavors is every build tag set cmd/ze is compiled under. CLAUDE.md names
// them, and a flavor missing here is a build this test does not judge.
var zeFlavors = []string{
	"ze_core",
	"ze_test",
	"ze_chaos",
	"ze_perf",
	"ze_analyze",
	"ze_setup",
	"ze_distro",
	"ze_appliance",
}

// depsTimeout bounds one `go list -deps`. Measured at 0.5 s over cmd/ze with
// ze_core on 2026-08-26; a run past this bound is a stuck toolchain.
const depsTimeout = 5 * time.Minute

// deps answers every package the named main package links under the given
// tags. It is the whole evidence base for this file: a claim about what a
// binary carries, made by the toolchain rather than by a reader.
func deps(t *testing.T, tags, pkg string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), depsTimeout)
	defer cancel()

	args := []string{"list", "-deps"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, pkg)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = repoRoot(t)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps -tags %q %s: %v", tags, pkg, err)
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

// repoRoot answers the checkout, walking up from the test's working directory,
// which `go test` sets to this package's own source directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "feature-gates.txt")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no Ze checkout above the test's working directory")
		}
		dir = parent
	}
}

// TestZeLinksNoLePlugin is AC-2. A dev tool that reaches the product is a
// dependency an appliance ships and nobody asked for.
func TestZeLinksNoLePlugin(t *testing.T) {
	for _, tags := range zeFlavors {
		t.Run(tags, func(t *testing.T) {
			for _, pkg := range deps(t, tags, "./cmd/ze") {
				if strings.HasPrefix(pkg, leTree) {
					t.Errorf("ze built with %q links %s: le's plugins are not ze's", tags, pkg)
				}
			}
		})
	}
}

// TestLeRegistersNoProductCommand is AC-3, and the rule it checks is
// DIRECTIONAL. le MAY link the product, including its composition root:
// twelve tool files under scripts/ blank-import
// internal/component/plugin/all today, because a command inventory, a
// CLI-grammar check and a YANG-leaf check cannot be computed without loading
// the registry they judge. le is a build-host binary nobody deploys, so that
// link costs nothing.
//
// What le must not do is OWN a product command. Two things would make it: an
// import in the composition root that names a product command package, and a
// root handler le registered whose code lives outside le's own trees.
//
// The second half is asked of leRoots rather than of the whole registry,
// because the transitive product packages DO register roots of their own --
// env, interface, plugin, schema and sysctl arrive with
// internal/component/plugin/all, measured 2026-08-26. Owning a name and
// sharing a process with one are different facts, and the one that matters is
// whether le RUNS it: TestLeDispatchesNoProductCommand asks that.
func TestLeRegistersNoProductCommand(t *testing.T) {
	root := repoRoot(t)

	for _, path := range toolImports(t) {
		if !strings.HasPrefix(path, leTree) {
			t.Errorf("cmd/le/register.go imports %s: the composition root names le packages only", path)
		}
	}

	for _, rc := range rootsAtStart {
		handler := registry.LookupRoot(rc.Name)
		if handler == nil {
			t.Errorf("%q is listed and cannot be looked up", rc.Name)
			continue
		}
		file := handlerFile(handler)
		rel, err := filepath.Rel(root, file)
		if err != nil {
			t.Errorf("%q is registered from %s, which is outside the checkout", rc.Name, file)
			continue
		}
		if !ownedByLe(rel) {
			t.Errorf("le carries the root command %q, registered from %s: that command belongs to another program", rc.Name, rel)
		}
	}
}

// TestLeDispatchesNoProductCommand is the half of AC-3 that the import list
// cannot cover. le links the product so its tools can read ze's registry, and
// linking it means ze's root commands register in this process: `le interface`
// would otherwise reach ze's interface editor, from a binary that has no
// business editing an interface.
//
// The refusal is dispatch's, so this drives dispatch. A product handler that
// ran would answer its own exit code, and every one of them exits 0 or 1 on a
// bare invocation, so the refusal is read from the usage page as well as the
// status.
func TestLeDispatchesNoProductCommand(t *testing.T) {
	product := 0
	for _, rc := range registry.ListRoot() {
		if leroot.Owns(rc.Name) {
			continue
		}
		product++
		t.Run(rc.Name, func(t *testing.T) {
			if code := dispatch([]string{rc.Name}); code != 1 {
				t.Errorf("`le %s` answered %d: le ran a command that belongs to ze", rc.Name, code)
			}
		})
	}

	if product == 0 {
		t.Fatal("no product command is registered in this process, so this test proves nothing: " +
			"le is meant to link ze so its tools can introspect it")
	}
}

// TestLeOwnsWhatItRegisters pins the ownership list itself against the
// registry: every name le claims resolves to a handler, and le claims no name
// the registry does not hold.
func TestLeOwnsWhatItRegisters(t *testing.T) {
	for _, name := range leroot.Owned() {
		if !registry.HasRootHandler(name) {
			t.Errorf("le claims %q and the registry holds no handler for it", name)
		}
	}
	if len(leroot.Owned()) < len(rootsAtStart) {
		t.Errorf("le owns %d names and %d root commands were listed as le's",
			len(leroot.Owned()), len(rootsAtStart))
	}
}

// handlerFile answers the source file a registered handler was defined in.
// The registry stores a function value, and the runtime knows where the code
// behind it lives, so this reads the truth rather than an import list.
func handlerFile(handler registry.RootHandler) string {
	pc := reflect.ValueOf(handler).Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}
	file, _ := fn.FileLine(pc)
	return file
}

// ownedByLe answers whether a repository-relative path belongs to one of le's
// own trees.
func ownedByLe(rel string) bool {
	rel = filepath.ToSlash(rel)
	return strings.HasPrefix(rel, "letools/") || strings.HasPrefix(rel, "cmd/le/")
}

// TestBothBinariesShareOneRegistry is AC-3b. Two registries kept in agreement
// by hand is the failure this whole design exists to prevent, so the check is
// that there is exactly one: both binaries link it, and neither tree declares
// a second.
func TestBothBinariesShareOneRegistry(t *testing.T) {
	leDeps := deps(t, "", "./cmd/le")
	zeDeps := deps(t, "ze_core", "./cmd/ze")

	if !slices.Contains(leDeps, engineRegistry) {
		t.Errorf("le does not link %s: its commands are registered somewhere else", engineRegistry)
	}
	if !slices.Contains(zeDeps, engineRegistry) {
		t.Errorf("ze does not link %s: its commands are registered somewhere else", engineRegistry)
	}

	// A second registry would announce itself by declaring the registration
	// entry point. Only the engine may declare it.
	root := repoRoot(t)
	for _, tree := range []string{"cmd/le", "letools"} {
		declaresRegistration(t, filepath.Join(root, tree))
	}
}

// declaresRegistration fails when any .go file under dir declares a root
// handler registry of its own rather than calling the engine's.
func declaresRegistration(t *testing.T, dir string) {
	t.Helper()
	// Assembled so this file stays inside its own check.
	declaration := "func Register" + "RootHandler("
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		body, readErr := os.ReadFile(path) //nolint:gosec // path comes from a walk of this repository
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(body), declaration) {
			t.Errorf("%s declares its own %s: the engine's registry is the only one", path, declaration)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
}
