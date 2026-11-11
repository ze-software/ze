// Design: docs/architecture/plugin/feature-gates.md -- authored committed inputs for external-body certificates.
package repoinventory

import (
	"os/exec"
	"path"
	"slices"
	"strings"
	"testing"
)

// The names below forward to ordinary_fixture_test.go for the proof-named
// tests that still use them; they go with this file.
const asmOrdinaryDir = ordinaryDir
const asmTargetFile = asmOrdinaryDir + "/boundary_linux_amd64.s"
const asmTargetBody = "//go:build linux && amd64 && gc && ze_le && ze_fixture\n\n#include \"textflag.h\"\nTEXT ·inventoryGate(SB),NOSPLIT,$0-56\n JMP syscall·Syscall(SB)\n"
const asmSchema = ordinarySchema
const asmExtraSchema = ordinaryExtraSchema

type asmInventoryFixture = ordinaryFixture

func newASMInventoryFixture(t *testing.T) *asmInventoryFixture {
	t.Helper()
	return newOrdinaryFixture(t)
}

func (fixture *asmInventoryFixture) addBoundary(t *testing.T, alias string) {
	t.Helper()
	// The assembly certificate and external initializer certificate have
	// separate obligations. Keep the original assembly stimulus unchanged.
	for _, sink := range []struct{ file, name string }{
		{"internal/component/plugin/registry/registry.go", "registry"},
		{"internal/component/config/yang/register.go", "yang"},
	} {
		body := strings.Replace(fixture.goSources[sink.file], "package "+sink.name+"\n", "package "+sink.name+"\nimport _ \"syscall\"\n", 1)
		fixture.goFile(t, sink.file, body)
	}
	imported := "\"syscall\""
	if alias != "syscall" {
		imported = alias + " \"syscall\""
	}
	fixture.goFile(t, asmOrdinaryDir+"/boundary.go", "package ordinary\nimport "+imported+"\nfunc inventoryGate(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err "+alias+".Errno)\nfunc init() { _, _, _ = inventoryGate(0, 0, 0, 0) }\n")
	write(t, fixture.root, asmTargetFile, asmTargetBody)
}

// asmGitInput writes only test-owned Git objects; no shared index or checkout is
// involved. It supports explicit empty-tree and gitlink shadow fixtures.
func asmGitInput(t *testing.T, root, input string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	command.Stdin = strings.NewReader(input)
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v: %s", args, err, raw)
	}
	return strings.TrimSpace(string(raw))
}

// asmCommitHeaderEntry rebuilds only ancestor tree objects so the header can be
// a tree (including empty) or gitlink, which a normal flat index cannot express.
func asmCommitHeaderEntry(t *testing.T, root, entry string) {
	t.Helper()
	head := strings.TrimSpace(committedGit(t, root, "rev-parse", "HEAD"))
	directory := asmOrdinaryDir
	entries := committedGit(t, root, "ls-tree", head+":"+directory)
	childTree := asmGitInput(t, root, entries+entry+"\n", "mktree")
	for directory != "." {
		parent, child := path.Dir(directory), path.Base(directory)
		revision := head
		if parent != "." {
			revision += ":" + parent
		}
		rows := strings.Split(strings.TrimSuffix(committedGit(t, root, "ls-tree", revision), "\n"), "\n")
		found := false
		for index, row := range rows {
			_, name, ok := strings.Cut(row, "\t")
			if ok && name == child {
				rows[index] = "040000 tree " + childTree + "\t" + child
				found = true
			}
		}
		if !found {
			t.Fatalf("fixture parent tree lacks %s", directory)
		}
		childTree = asmGitInput(t, root, strings.Join(rows, "\n")+"\n", "mktree")
		directory = parent
	}
	commit := asmGitInput(t, root, "", "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit-tree", childTree, "-p", head, "-m", "header object fixture")
	committedGit(t, root, "update-ref", "refs/heads/main", commit, head)
}

// asmExpectRefusal logs the actual boundary so Main can distinguish an old
// unavailable-body prerequisite from an exercised linkage/header discriminator.
func asmExpectRefusal(t *testing.T, fixture *asmInventoryFixture) {
	t.Helper()
	inv, err := Collect(fixture.root)
	if err == nil {
		t.Fatalf("unproved body/linkage published a partial successful report: %+v", inv.Plugins)
	}
	if inv.Plugins != nil {
		t.Fatalf("refusal leaked partial plugins: %+v: %v", inv.Plugins, err)
	}
	t.Logf("actual committed boundary refusal (classify prerequisite masking separately): %v", err)
}

func asmCheckSourcesStayGo(t *testing.T, snapshot *Snapshot, want int) {
	t.Helper()
	sources, _ := snapshot.Sources()
	if len(sources) != want {
		t.Fatalf("Go AST population = %d, want %d", len(sources), want)
	}
	if slices.ContainsFunc(sources, func(source SourceFile) bool { return !strings.HasSuffix(source.Path, ".go") }) {
		t.Fatal("assembly evidence entered Go Sources")
	}
}
