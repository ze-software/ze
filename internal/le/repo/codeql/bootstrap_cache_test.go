// Design: docs/architecture/testing/ci-workflows.md -- CodeQL's patched-toolchain bootstrap.
package codeql

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
)

// TestCodeQLInitializationSeesSelectedToolchain keeps CodeQL's Go-path capture
// and optional static-binary tracing wrapper on the selected patched toolchain.
// Changing PATH after initialization can bypass the wrapper it installed.
func TestCodeQLInitializationSeesSelectedToolchain(t *testing.T) {
	selection, initialization := -1, -1
	for index, step := range workflowSteps(t) {
		if step.Name == "Select the patched Go standard library" {
			selection = index
		}
		if step.Name == "Initialize CodeQL" {
			initialization = index
		}
	}
	if selection < 0 {
		t.Fatal("workflow never selects its patched Go toolchain")
	}
	if initialization < 0 {
		t.Fatal("workflow never initializes CodeQL")
	}
	if selection >= initialization {
		t.Fatal("CodeQL initializes before the patched Go toolchain it must trace is selected")
	}
}

// TestWorkflowBootstrapKeepsSelectedGOROOT executes the workflow's bootstrap
// before another launcher acquires its cache. The synthetic generator uses the
// real cache resolver; both launcher acquisitions use the actual shell helper.
// A selected tree must retain its Go defaults after that second acquisition.
// MUTATION: Bootstrap with a direct go run before the launcher places cache/;
// its next acquisition moves the selected tree and loses the Go defaults.
func TestWorkflowBootstrapKeepsSelectedGOROOT(t *testing.T) {
	root := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	function := codeqlCacheLauncher(t)
	tools := filepath.Join(root, "tools")
	fixtureWrite(t, root, "tools/go", "#!/bin/sh\nexec \"$CODEQL_BOOTSTRAP_FIXTURE_BINARY\" -test.run '^TestCodeQLBootstrapGo$' -- \"$@\"\n")
	fixtureWrite(t, root, "le", "#!/bin/sh\nset -eu\nroot=$PWD\n"+function+"\nensure_cache_link\nexec \"$CODEQL_BOOTSTRAP_FIXTURE_GO\" run ./internal/le/go/stdpatch/goroot\n")
	for _, path := range []string{"tools/go", "le"} {
		if err := os.Chmod(filepath.Join(root, path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	environment := append(os.Environ(),
		"PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HOME="+filepath.Join(root, "home"), "XDG_CACHE_HOME="+filepath.Join(root, "user-cache"),
		"GITHUB_ENV="+filepath.Join(root, "github-env"), "GITHUB_PATH="+filepath.Join(root, "github-path"),
		"CODEQL_BOOTSTRAP_FIXTURE_BINARY="+binary, "CODEQL_BOOTSTRAP_FIXTURE_GO="+filepath.Join(tools, "go"),
		"CODEQL_BOOTSTRAP_FIXTURE_CHILD=1")
	bootstrap := exec.CommandContext(t.Context(), "bash", "-euo", "pipefail", "-c", workflowStepBody(t, "Select the patched Go standard library"))
	bootstrap.Dir, bootstrap.Env = root, environment
	if output, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("workflow bootstrap: %v\n%s", err, output)
	}
	settings, err := os.ReadFile(filepath.Join(root, "github-env"))
	if err != nil {
		t.Fatal(err)
	}
	tree := ""
	for line := range strings.SplitSeq(string(settings), "\n") {
		if value, found := strings.CutPrefix(line, "GOROOT="); found {
			tree = value
		}
	}
	if tree == "" {
		t.Fatalf("bootstrap selected no GOROOT: %s", settings)
	}
	if !strings.Contains(string(settings), "GOTOOLCHAIN=local\n") {
		t.Fatalf("bootstrap did not retain the selected toolchain: %s", settings)
	}
	path, err := os.ReadFile(filepath.Join(root, "github-path"))
	if err != nil {
		t.Fatal(err)
	}
	if string(path) != filepath.Join(tree, "bin")+"\n" {
		t.Fatalf("bootstrap PATH entry = %q, want selected GOROOT bin", path)
	}
	next := exec.CommandContext(t.Context(), "sh", "-eu", "-c", "root=$PWD\n"+function+"\nensure_cache_link\n")
	next.Dir, next.Env = root, environment
	if output, err := next.CombinedOutput(); err != nil {
		t.Fatalf("next launcher cache acquisition: %v\n%s", err, output)
	}
	defaults, err := os.ReadFile(filepath.Join(tree, "go.env"))
	if err != nil {
		t.Fatalf("selected GOROOT lost its Go defaults after launcher cache acquisition: %v", err)
	}
	if string(defaults) != "GOSUMDB=sum.golang.org\n" {
		t.Fatalf("selected GOROOT defaults changed: %q", defaults)
	}
}

// TestCodeQLBootstrapGo is the synthetic generator process, not a compiler.
// It writes only a tiny tree below this fixture's real cache resolution.
func TestCodeQLBootstrapGo(t *testing.T) {
	if os.Getenv("CODEQL_BOOTSTRAP_FIXTURE_CHILD") != "1" {
		return
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cache, err := gen.CacheDir(root)
	if err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(cache, "fixture", "goroot")
	if err := os.MkdirAll(filepath.Join(tree, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "go.env"), []byte("GOSUMDB=sum.golang.org\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stdout.WriteString(tree + "\n"); err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}

func codeqlCacheLauncher(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the CodeQL bootstrap fixture")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../..", "le"))
	if err != nil {
		t.Fatal(err)
	}
	_, body, found := strings.Cut(string(source), "ensure_cache_link() {\n")
	if !found {
		t.Fatal("native launcher has no cache acquisition helper")
	}
	body, _, found = strings.Cut(body, "\n}\n")
	if !found {
		t.Fatal("native launcher cache acquisition helper has no closing boundary")
	}
	return "ensure_cache_link() {\n" + body + "\n}\n"
}
