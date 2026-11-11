// Design: docs/architecture/plugin/feature-gates.md -- retained header evidence and exact report populations.
package repoinventory

import (
	"path"
	"slices"
	"strings"
	"testing"
)

const asmHeaderProbeDir = "internal/headerprobe"
const asmHeaderProbeFile = asmHeaderProbeDir + "/probe.go"
const asmHeaderUnit = asmHeaderProbeDir + "/probe_linux_amd64.s"
const asmHeaderReportFile = "internal/headerreportonly/report.go"
const asmHeaderNumericBody = "TEXT ·unentered(SB),4,$0-0\n RET\n"
const asmHeaderEmptyGo = "package headerprobe\nvar Ready bool\nfunc Dormant() {}\n"

func asmHeaderPaths(extra ...string) []string {
	paths := []string{
		"cmd/ze/main.go",
		"internal/component/config/yang/register.go",
		"internal/component/plugin/registry/registry.go",
		asmOrdinaryDir + "/register.go",
		asmOrdinaryDir + "/boundary.go",
		asmOrdinaryDir + "/header_probe_import.go",
		asmHeaderProbeFile,
	}
	paths = append(paths, extra...)
	slices.Sort(paths)
	return paths
}

// asmHeaderFixture includes an authored report-only file that is deliberately
// outside the selected import graph. Report statistics must retain it while
// selected Go paths must not acquire it or any header/assembly evidence.
func asmHeaderFixture(t *testing.T, declarations, unit string, witnesses ...string) *asmInventoryFixture {
	t.Helper()
	fixture := newASMInventoryFixture(t)
	fixture.addBoundary(t, "syscall")
	for _, origin := range witnesses {
		for _, sink := range []struct{ file, name string }{
			{"internal/component/plugin/registry/registry.go", "registry"},
			{"internal/component/config/yang/register.go", "yang"},
		} {
			declaration := "package " + sink.name + "\n"
			body := strings.Replace(fixture.goSources[sink.file], declaration, declaration+"import _ \""+origin+"\"\n", 1)
			fixture.goFile(t, sink.file, body)
		}
	}
	fixture.goFile(t, asmOrdinaryDir+"/header_probe_import.go", "package ordinary\nimport _ \"github.com/ze-software/ze/"+asmHeaderProbeDir+"\"\n")
	fixture.goFile(t, asmHeaderProbeFile, declarations)
	fixture.goFile(t, asmHeaderReportFile, "package reportonly\nconst Unselected = len(\"not selected\")\n")
	write(t, fixture.root, asmHeaderUnit, unit)
	commitInventoryFixture(t, fixture.root)
	asmHeaderPositive(t, fixture, asmHeaderPaths())
	return fixture
}

// asmHeaderCause prevents a prerequisite refusal from impersonating the named
// discriminator. In particular, old unknown includes are not collision proof.
func asmHeaderCause(t *testing.T, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected explicit header evidence refusal")
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("refusal is masked or wrong: %v; missing intended discriminator %q", err, fragment)
		}
	}
	t.Logf("exercised named header discriminator: %v", err)
}

func asmHeaderRefusal(t *testing.T, fixture *asmInventoryFixture, fragments ...string) {
	t.Helper()
	inv, err := Collect(fixture.root)
	if err == nil {
		t.Fatalf("header evidence falsely published successful report: %+v", inv)
	}
	if inv.Plugins != nil {
		t.Fatalf("header refusal leaked partial plugins: %+v: %v", inv.Plugins, err)
	}
	asmHeaderCause(t, err, fragments...)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatalf("source capture failed before header admission: %v", err)
	}
	plugins, err := snapshot.Plugins()
	if plugins != nil {
		t.Fatalf("retained header refusal leaked partial plugins: %+v", plugins)
	}
	asmHeaderCause(t, err, fragments...)
}

// asmHeaderCommitEntry creates only test-owned trees. Unlike the older frozen
// helper, its directory is the authored header-probe package, not ordinary.
func asmHeaderCommitEntry(t *testing.T, fixture *asmInventoryFixture, entry string) {
	t.Helper()
	head := strings.TrimSpace(committedGit(t, fixture.root, "rev-parse", "HEAD"))
	directory := asmHeaderProbeDir
	entries := committedGit(t, fixture.root, "ls-tree", head+":"+directory)
	childTree := asmGitInput(t, fixture.root, entries+entry+"\n", "mktree")
	for directory != "." {
		parent, child := path.Dir(directory), path.Base(directory)
		revision := head
		if parent != "." {
			revision += ":" + parent
		}
		rows := strings.Split(strings.TrimSuffix(committedGit(t, fixture.root, "ls-tree", revision), "\n"), "\n")
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
		childTree = asmGitInput(t, fixture.root, strings.Join(rows, "\n")+"\n", "mktree")
		directory = parent
	}
	commit := asmGitInput(t, fixture.root, "", "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit-tree", childTree, "-p", head, "-m", "header namespace object fixture")
	committedGit(t, fixture.root, "update-ref", "refs/heads/main", commit, head)
}

// The names below forward to ordinary_fixture_test.go for the proof-named
// tests that still use them; they go with this file.
func asmHeaderSelectedPaths(t *testing.T, snapshot *Snapshot, want []string) {
	t.Helper()
	checkSelectedPaths(t, snapshot, want)
}

func asmHeaderPositive(t *testing.T, fixture *asmInventoryFixture, paths []string) *Snapshot {
	t.Helper()
	return checkOrdinaryPositive(t, fixture, paths)
}

func asmHeaderExpected(fixture *asmInventoryFixture) *asmInventoryFixture {
	return frozenFixture(fixture)
}
