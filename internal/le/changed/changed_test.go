package changed

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: the selection that sizes a test run fails CLOSED. A git query that
// cannot answer, and a package directory that cannot be resolved, are errors --
// never an empty selection.
// PREVENTS: the regression where scripts/dev/changed-groups.sh answers nothing
// and exits 0 for a git that fails or a `go list` that fails, and
// mk/test-unit.mk:123 reads that as "No changed .go files -- skipping
// changed-group pass". The race-instrumented pass then runs no test and the
// target reports success.

// recorder is a command runner that answers from a table and remembers what it
// was asked. Every case that asserts a COUNT reads it, because "the two halves
// agree" is not the same claim as "three queries ran".
type recorder struct {
	answers map[string]string
	fail    map[string]error
	calls   [][]string
}

func (r *recorder) run(_ string, argv []string) (string, error) {
	r.calls = append(r.calls, argv)
	key := strings.Join(argv, " ")
	if err, bad := r.fail[key]; bad {
		return "", err
	}
	return r.answers[key], nil
}

const (
	unstagedQuery  = "git diff --name-only -- *.go"
	stagedQuery    = "git diff --cached --name-only -- *.go"
	untrackedQuery = "git ls-files --others --exclude-standard -- *.go"
)

func TestAPppFileIsClaimedByTheL2tpGroupThatCoversIt(t *testing.T) {
	group, mapped := GroupOf("internal/component/l2tp/ppp/session.go")
	if !mapped {
		t.Fatal("a ppp file was claimed by no group")
	}
	if group.Name != "l2tp" {
		t.Errorf("ppp file claimed by group %q, want l2tp: the l2tp pattern runs ppp's tests too", group.Name)
	}
	if group.Pattern != "./internal/component/l2tp/..." {
		t.Errorf("l2tp pattern is %q, which does not cover ppp", group.Pattern)
	}
}

// A prefix inside another prefix can only narrow the run: the parent's pattern
// already compiles and tests the subtree. The shell half carries one such row,
// unreachable because bash's hash order hides it, and this is what keeps it out
// of the port.
func TestNoGroupPrefixSitsInsideAnother(t *testing.T) {
	for i, outer := range groups {
		for j, inner := range groups {
			if i == j {
				continue
			}
			if strings.HasPrefix(inner.Prefix, outer.Prefix) {
				t.Errorf("group %q (%s) sits inside group %q (%s), so it can only narrow the run",
					inner.Group.Name, inner.Prefix, outer.Group.Name, outer.Prefix)
			}
		}
	}
}

func TestAPathNoPrefixClaimsIsNotGrouped(t *testing.T) {
	if group, mapped := GroupOf("internal/le/changed/changed.go"); mapped {
		t.Errorf("internal/le path claimed by group %q, want no group", group.Name)
	}
}

func TestAGitFailureAnswersAnErrorRatherThanAnEmptyFileList(t *testing.T) {
	boom := errors.New("git: not a repository")
	rec := &recorder{fail: map[string]error{unstagedQuery: boom}}

	files, err := Selector{Root: t.TempDir(), Run: rec.run}.ChangedFiles()
	if err == nil {
		t.Fatalf("a failing git answered %v and no error", files)
	}
	if !errors.Is(err, boom) {
		t.Errorf("error %v does not carry the git failure", err)
	}
}

// The third query is the one that fails, so a selection that checked only the
// first would pass this and must not.
func TestEveryGitQueryMustSucceedForASelection(t *testing.T) {
	boom := errors.New("git: ls-files failed")
	rec := &recorder{
		answers: map[string]string{unstagedQuery: "cmd/ze/main.go\n", stagedQuery: ""},
		fail:    map[string]error{untrackedQuery: boom},
	}

	if _, err := (Selector{Root: t.TempDir(), Run: rec.run}).ChangedFiles(); err == nil {
		t.Fatal("a failing third query answered no error")
	}
	if len(rec.calls) != 3 {
		t.Errorf("%d git queries ran, want exactly 3", len(rec.calls))
	}
}

func TestTheFileListIsTheUnionOfTheThreeGitQueries(t *testing.T) {
	rec := &recorder{answers: map[string]string{
		unstagedQuery:  "cmd/ze/main.go\ninternal/core/env/env.go\n",
		stagedQuery:    "cmd/ze/main.go\n",
		untrackedQuery: "internal/plugins/ntp/new.go\n\n",
	}}

	files, err := Selector{Root: t.TempDir(), Run: rec.run}.ChangedFiles()
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	want := []string{"cmd/ze/main.go", "internal/core/env/env.go", "internal/plugins/ntp/new.go"}
	if len(files) != len(want) {
		t.Fatalf("%d files, want exactly %d: %v", len(files), len(want), files)
	}
	for i, file := range want {
		if files[i] != file {
			t.Errorf("file %d is %q, want %q", i, files[i], file)
		}
	}
}

func TestAGroupIsNamedOnceHoweverManyFilesHitIt(t *testing.T) {
	selection := groupFiles([]string{
		"internal/component/bgp/reactor/peer.go",
		"internal/component/bgp/wire/update.go",
		"internal/component/bgp/fsm/fsm.go",
	})
	if len(selection.Groups) != 1 {
		t.Fatalf("%d groups, want exactly 1: %v", len(selection.Groups), selection.Groups)
	}
	if selection.Groups[0].Name != "bgp" {
		t.Errorf("group is %q, want bgp", selection.Groups[0].Name)
	}
	if len(selection.unmapped) != 0 {
		t.Errorf("%d unmapped directories, want 0: %v", len(selection.unmapped), selection.unmapped)
	}
}

// The files arrive in the reverse of the table's order, so a listing built in
// discovery order answers cmd, core, bgp and fails here.
func TestGroupsAreOrderedByTheTableRatherThanByDiscovery(t *testing.T) {
	selection := groupFiles([]string{
		"cmd/ze/main.go",
		"internal/core/env/env.go",
		"internal/component/bgp/reactor/peer.go",
	})
	want := []string{"bgp", "core", "cmd"}
	if len(selection.Groups) != len(want) {
		t.Fatalf("%d groups, want %d: %v", len(selection.Groups), len(want), selection.Groups)
	}
	for i, name := range want {
		if selection.Groups[i].Name != name {
			t.Errorf("group %d is %q, want %q", i, selection.Groups[i].Name, name)
		}
	}
}

func TestAnUnmappedFileBecomesItsOwnPackageDirectory(t *testing.T) {
	selection := groupFiles([]string{"internal/le/changed/changed.go", "internal/le/changed/scope.go"})
	if len(selection.Groups) != 0 {
		t.Errorf("%d groups, want 0: %v", len(selection.Groups), selection.Groups)
	}
	if len(selection.unmapped) != 1 || selection.unmapped[0] != "./internal/le/changed" {
		t.Fatalf("unmapped is %v, want exactly [./internal/le/changed]", selection.unmapped)
	}
}

func TestAPackageResolutionFailureAnswersAnErrorRatherThanDroppingThePackage(t *testing.T) {
	boom := errors.New("go: cannot load module")
	rec := &recorder{fail: map[string]error{"go list -e -f {{if not .Error}}{{.Dir}}{{end}}" +
		" ./internal/le/changed": boom}}

	if _, err := (Selector{Root: t.TempDir(), Run: rec.run}).Packages([]string{"./internal/le/changed"}); err == nil {
		t.Fatal("a failing go list answered no error")
	}
}

// This test covers the complete path against a real toolchain. A broken module
// makes `go list` fail. The selection must return an error instead of the empty
// answer that the shell returns (measured 2026-08-26).
func TestABrokenModuleAnswersAnErrorRatherThanAnEmptySelection(t *testing.T) {
	goToolchain(t)
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module probe\n\ngo 1.25\nrequire nosuch v9.9.9\n")
	writeFile(t, root, "internal/le/x/x.go", "package x\n")

	_, err := (Selector{Root: root}).Packages([]string{"./internal/le/x"})
	if err == nil {
		t.Fatal("a broken module resolved its packages without error")
	}
}

func TestADirectoryWhoseGoFilesAreAllBuildIgnoredIsDropped(t *testing.T) {
	goToolchain(t)
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module probe\n\ngo 1.25\n")
	writeFile(t, root, "real/real.go", "package real\n")
	writeFile(t, root, "tool/tool.go", "//go:build zzz_never_defined\n\npackage main\n")

	packages, err := (Selector{Root: root}).Packages([]string{"./real", "./tool"})
	if err != nil {
		t.Fatalf("Packages: %v", err)
	}
	if len(packages) != 1 || packages[0] != "./real" {
		t.Fatalf("packages are %v, want exactly [./real]", packages)
	}
}

func TestTheGroupListingAndThePackageListingRenderOneSelection(t *testing.T) {
	selection := Selection{
		Groups: []Group{{"bgp", "./internal/component/bgp/..."}},
		Rest:   []string{"./internal/le/changed"},
	}

	names := GroupNames{Selection: selection}.Text()
	if names != "bgp\nrest\n" {
		t.Errorf("group listing is %q, want \"bgp\\nrest\\n\"", names)
	}

	packages := GroupPackages{Selection: selection}.Text()
	if packages != "./internal/component/bgp/...\n./internal/le/changed\n" {
		t.Errorf("package listing is %q", packages)
	}
}

func TestAnEmptySelectionRendersNothing(t *testing.T) {
	if text := (GroupNames{}).Text(); text != "" {
		t.Errorf("empty group listing is %q, want the empty string", text)
	}
	if text := (GroupPackages{}).Text(); text != "" {
		t.Errorf("empty package listing is %q, want the empty string", text)
	}
	if !(Selection{}).Empty() {
		t.Error("the zero selection does not report itself empty")
	}
}

// The area must reach every verb it declares, or a developer types a word the
// listing printed and gets a refusal.
func TestEveryVerbOfTheAreaIsReachable(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 3 {
		t.Fatalf("%d actions, want exactly 3: %v", len(list.Actions), list.Actions)
	}
	want := map[string]bool{"groups": false, "group-packages": false, "packages": false}
	for _, row := range list.Actions {
		if _, known := want[row.Verb]; !known {
			t.Errorf("unexpected verb %q", row.Verb)
			continue
		}
		want[row.Verb] = true
	}
	for verb, seen := range want {
		if !seen {
			t.Errorf("verb %q is not in the listing", verb)
		}
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

// goToolchain skips a case that needs a real `go` on PATH.
func goToolchain(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
}
