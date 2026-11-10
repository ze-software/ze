package runner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeStaleSource answers the stale check from fixed lists and counts the
// calls, so a case can prove a question was never asked.
type fakeStaleSource struct {
	changed      []string
	changedErr   error
	linked       map[string]bool
	changedCalls int
	linkedCalls  int
	sha          string // the sha changedGoFiles was asked about
}

func (f *fakeStaleSource) changedGoFiles(_ context.Context, _, sha string) ([]string, error) {
	f.changedCalls++
	f.sha = sha
	return f.changed, f.changedErr
}

func (f *fakeStaleSource) linkedTestPackages(context.Context, string) (map[string]bool, error) {
	f.linkedCalls++
	return f.linked, nil
}

// staleLaunchBuild is a launcher build's le, as the dispatcher runs it.
const staleLaunchBuild = "/repo/bin/builds/20261110T042821Z-abc1234/le"

// TestRunnerNamesAStaleBuild proves the direct route's stale-runner notice
// (AC-9): a launcher build whose linked internal/test Go files differ from
// the working tree is named with its commit, at most ten paths and their
// count, and both fixes; every le that carries the tree's fixture code, and a
// change le does not link, says nothing. Method: findStaleBuild over a fake
// source, staleNotice rendered without color, Run over an empty selection,
// and the checkout source over a throwaway git repository.
func TestRunnerNamesAStaleBuild(t *testing.T) {
	plain := NewColorsWithOverride(false)
	invocation := []string{"test", "bgp", "plugin", "--pattern", "x"}

	t.Run("launcher build with a linked change", func(t *testing.T) {
		source := &fakeStaleSource{
			changed: []string{"internal/test/plugins/x/x.go", "internal/test/fixture/daemon_store.go"},
			linked:  map[string]bool{"internal/test/fixture": true},
		}
		check := findStaleBuild(t.Context(), staleLaunchBuild, "/repo", source)
		if check == nil {
			t.Fatal("no notice for a launcher build whose linked fixture file differs")
		}
		if source.sha != "abc1234" {
			t.Fatalf("diffed against %q, want the build's commit abc1234", source.sha)
		}
		notice := staleNotice(check, plain, "plugin", invocation)
		for _, want := range []string{
			"stale le: this le was built from commit abc1234, and 1 Go file(s) it links from internal/test/ differ",
			"  internal/test/fixture/daemon_store.go\n",
			"  ./le test functional plugin\n",
			"  ./le --name session test bgp plugin --pattern x",
		} {
			if !strings.Contains(notice, want) {
				t.Errorf("notice lacks %q:\n%s", want, notice)
			}
		}
		if strings.Contains(notice, "internal/test/plugins/x/x.go") {
			t.Errorf("notice names a file le does not link:\n%s", notice)
		}
	})

	t.Run("platform build", func(t *testing.T) {
		source := &fakeStaleSource{
			changed: []string{"internal/test/fixture/a.go"},
			linked:  map[string]bool{"internal/test/fixture": true},
		}
		check := findStaleBuild(t.Context(), "/repo/bin/builds/20261110T042821Z-abc1234-linux-arm64/le", "/repo", source)
		if check == nil {
			t.Fatal("no notice for another platform's launcher build")
		}
	})

	t.Run("eleven paths print ten and count the rest", func(t *testing.T) {
		changed := make([]string, 0, stalePathsMax+1)
		for i := range stalePathsMax + 1 {
			changed = append(changed, "internal/test/fixture/f"+strconv.Itoa(10+i)+".go")
		}
		source := &fakeStaleSource{changed: changed, linked: map[string]bool{"internal/test/fixture": true}}
		check := findStaleBuild(t.Context(), staleLaunchBuild, "/repo", source)
		if check == nil {
			t.Fatal("no notice for eleven linked changes")
		}
		notice := staleNotice(check, plain, "plugin", invocation)
		if !strings.Contains(notice, "and 11 Go file(s)") {
			t.Errorf("notice does not count eleven files:\n%s", notice)
		}
		if !strings.Contains(notice, "  internal/test/fixture/f19.go\n") {
			t.Errorf("notice lacks the tenth path:\n%s", notice)
		}
		if strings.Contains(notice, "f20.go") {
			t.Errorf("notice prints the eleventh path instead of counting it:\n%s", notice)
		}
		if !strings.Contains(notice, "  and 1 more\n") {
			t.Errorf("notice does not count the eleventh path:\n%s", notice)
		}
	})

	t.Run("ten paths print all and count none", func(t *testing.T) {
		changed := make([]string, 0, stalePathsMax)
		for i := range stalePathsMax {
			changed = append(changed, "internal/test/fixture/f"+strconv.Itoa(10+i)+".go")
		}
		source := &fakeStaleSource{changed: changed, linked: map[string]bool{"internal/test/fixture": true}}
		notice := staleNotice(findStaleBuild(t.Context(), staleLaunchBuild, "/repo", source), plain, "plugin", invocation)
		if strings.Contains(notice, "more") {
			t.Errorf("ten paths counted a remainder:\n%s", notice)
		}
	})

	for _, survivor := range []struct {
		name       string
		executable string
	}{
		{"a --name build", "/repo/bin/le-s-bfe0e27e/le"},
		{"the isolated set of ./le test functional", "/repo/tmp/session/x/scratch/ze-functional-bin-1/le"},
		{"a staging directory", "/repo/bin/builds/.staging-123/le"},
	} {
		t.Run(survivor.name+" is never stale", func(t *testing.T) {
			source := &fakeStaleSource{
				changed: []string{"internal/test/fixture/a.go"},
				linked:  map[string]bool{"internal/test/fixture": true},
			}
			if check := findStaleBuild(t.Context(), survivor.executable, "/repo", source); check != nil {
				t.Fatalf("notice for %s, which carries the working tree's code: %+v", survivor.executable, check)
			}
			if source.changedCalls != 0 {
				t.Fatal("ran git for an le that is not a launcher build")
			}
		})
	}

	t.Run("nothing changed asks go nothing", func(t *testing.T) {
		source := &fakeStaleSource{linked: map[string]bool{"internal/test/fixture": true}}
		if check := findStaleBuild(t.Context(), staleLaunchBuild, "/repo", source); check != nil {
			t.Fatalf("notice with no changed file: %+v", check)
		}
		if source.linkedCalls != 0 {
			t.Fatal("listed le's packages although no Go file changed")
		}
	})

	t.Run("a change le does not link", func(t *testing.T) {
		source := &fakeStaleSource{
			changed: []string{"internal/test/plugins/x/x.go"},
			linked:  map[string]bool{"internal/test/fixture": true},
		}
		if check := findStaleBuild(t.Context(), staleLaunchBuild, "/repo", source); check != nil {
			t.Fatalf("notice for a file le does not link: %+v", check)
		}
	})

	t.Run("a check that cannot answer says so", func(t *testing.T) {
		source := &fakeStaleSource{changedErr: errors.New("git diff failed: bad revision")}
		check := findStaleBuild(t.Context(), staleLaunchBuild, "/repo", source)
		if check == nil {
			t.Fatal("a failed check was read as a current build")
		}
		notice := staleNotice(check, plain, "plugin", invocation)
		for _, want := range []string{"le may be stale:", "bad revision", "./le test functional plugin"} {
			if !strings.Contains(notice, want) {
				t.Errorf("notice lacks %q:\n%s", want, notice)
			}
		}
	})

	t.Run("Run prints the notice before the tests and once on success", func(t *testing.T) {
		var output bytes.Buffer
		tests := NewEncodingTests(t.TempDir())
		r := &Runner{
			tests:      tests,
			colors:     plain,
			display:    NewDisplay(tests.Tests, plain),
			report:     newReport(plain),
			stale:      &staleCheck{sha: "abc1234", changed: []string{"internal/test/fixture/a.go"}},
			invocation: invocation,
		}
		r.display.SetOutput(&output)
		r.display.SetLabel("plugin")
		if !r.Run(t.Context(), &RunOptions{SkipTimings: true}) {
			t.Fatal("an empty selection failed")
		}
		if got := strings.Count(output.String(), "stale le:"); got != 1 {
			t.Fatalf("notice printed %d times on a passing run, want 1:\n%s", got, output.String())
		}
	})

	t.Run("the checkout source lists committed, uncommitted and untracked Go files", func(t *testing.T) {
		root := t.TempDir()
		staleWrite(t, root, "internal/test/fixture/committed.go", "package fixture\n")
		staleWrite(t, root, "internal/test/fixture/edited.go", "package fixture\n")
		staleWrite(t, root, "internal/test/fixture/edited_test.go", "package fixture\n")
		staleWrite(t, root, "internal/other/other.go", "package other\n")
		staleGitRun(t, root, "init", "--quiet")
		staleGitCommit(t, root, "base")
		sha := strings.TrimSpace(staleGitRun(t, root, "rev-parse", "--short", "HEAD"))
		staleWrite(t, root, "internal/test/fixture/committed.go", "package fixture\n\n// later\n")
		staleGitCommit(t, root, "later")
		staleWrite(t, root, "internal/test/fixture/edited.go", "package fixture\n\n// edit\n")
		staleWrite(t, root, "internal/test/fixture/edited_test.go", "package fixture\n\n// edit\n")
		staleWrite(t, root, "internal/test/fixture/new file.go", "package fixture\n")
		staleWrite(t, root, "internal/other/other.go", "package other\n\n// edit\n")

		files, err := checkoutStaleSource{}.changedGoFiles(t.Context(), root, sha)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			"internal/test/fixture/committed.go",
			"internal/test/fixture/edited.go",
			"internal/test/fixture/new file.go",
		}
		if strings.Join(files, "|") != strings.Join(want, "|") {
			t.Fatalf("changed Go files = %q, want %q", files, want)
		}
	})
}

// staleWrite writes one file of a throwaway checkout.
func staleWrite(t *testing.T, root, relative, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// staleGitCommit commits every file of a throwaway checkout, with no hook and
// no signing, under a fixed identity.
func staleGitCommit(t *testing.T, root, message string) {
	t.Helper()
	staleGitRun(t, root, "add", "--", ".")
	staleGitRun(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@invalid", "-c", "commit.gpgsign=false",
		"-c", "core.hooksPath="+os.DevNull, "commit", "--quiet", "-m", message)
}

// staleGitRun runs git in a throwaway checkout and answers its output. An
// inherited GIT_ variable (a hook's GIT_DIR, GIT_INDEX_FILE) would point git
// at the real checkout, so none is passed on.
func staleGitRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	environment := []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			environment = append(environment, entry)
		}
	}
	cmd.Env = environment
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}
