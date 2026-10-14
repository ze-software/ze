// VALIDATES: AC-20, le's root handler selects the patched GOROOT before it
// dispatches, and a child started the way the inheritance sites start one
// compiles with it.
// PREVENTS: a Go command le starts from os.Environ() that runs the unpatched
// standard library.

package le

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// inheritChildMode marks the re-executed test binary that runs the root
// handler with a clean environment.
const inheritChildMode = "ZE_TEST_LE_INHERIT_CHILD"

// TestLeChildInheritsThePatchedGOROOT drives the root handler `run` with `go
// stdpatch goroot` in a child process whose environment carries no tree (no
// GOROOT, no GOTOOLCHAIN, PATH without a tree's bin): the activation is
// answered once per process, so only a fresh process proves that `run` is
// what sets it. After `run`, the child starts `go env GOROOT` from
// os.Environ() and resolves `go` with exec.LookPath, as the inheritance sites
// do; both must answer the tree, compared through their links because cache/
// is a link to ~/.cache/ze.
func TestLeChildInheritsThePatchedGOROOT(t *testing.T) {
	if os.Getenv(inheritChildMode) != "" {
		inheritChild(t)
		return
	}

	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		switch name {
		case "GOROOT", "GOTOOLCHAIN":
			continue
		case "PATH":
			entry = "PATH=" + withoutTreeBin(value)
		}
		environment = append(environment, entry)
	}
	child := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestLeChildInheritsThePatchedGOROOT$", "-test.v")
	environment = append(environment, inheritChildMode+"=1")
	child.Env = environment
	out, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("the child failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "INHERITED: ") {
		t.Fatalf("the child did not report its answer:\n%s", out)
	}
}

// withoutTreeBin answers a PATH with every patched GOROOT's bin removed.
func withoutTreeBin(path string) string {
	kept := make([]string, 0, 8)
	for _, entry := range filepath.SplitList(path) {
		if strings.Contains(entry, string(filepath.Separator)+"stdpatch"+string(filepath.Separator)) {
			continue
		}
		kept = append(kept, entry)
	}
	return strings.Join(kept, string(filepath.ListSeparator))
}

// inheritChild is the child half: run the root handler, then ask a child of
// its own which GOROOT and which go it gets.
func inheritChild(t *testing.T) {
	t.Helper()
	if _, set := os.LookupEnv("GOROOT"); set {
		t.Fatal("the child started with GOROOT set; the test proves nothing")
	}
	if code := run(nil, []string{"go", "stdpatch", "goroot"}); code != 0 {
		t.Fatalf("le go stdpatch goroot exited %d", code)
	}
	tree := os.Getenv("GOROOT")
	if tree == "" {
		t.Fatal("run left GOROOT unset")
	}
	if got := os.Getenv("GOTOOLCHAIN"); got != "local" {
		t.Errorf("GOTOOLCHAIN = %q after run, want local", got)
	}

	cmd := exec.CommandContext(context.Background(), "go", "env", "GOROOT")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go env GOROOT: %v", err)
	}
	if !sameFile(t, strings.TrimSpace(string(out)), tree) {
		t.Errorf("a child's go env GOROOT answered %s, want the tree %s", strings.TrimSpace(string(out)), tree)
	}
	goProgram, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	if !sameFile(t, filepath.Dir(goProgram), filepath.Join(tree, "bin")) {
		t.Errorf("exec.LookPath(go) answered %s, want %s", goProgram, filepath.Join(tree, "bin", "go"))
	}
	t.Log("INHERITED: " + tree)
}

// sameFile answers whether two paths name the same file once their links
// are resolved.
func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	resolvedA, err := filepath.EvalSymlinks(a)
	if err != nil {
		t.Fatal(err)
	}
	resolvedB, err := filepath.EvalSymlinks(b)
	if err != nil {
		t.Fatal(err)
	}
	return resolvedA == resolvedB
}
