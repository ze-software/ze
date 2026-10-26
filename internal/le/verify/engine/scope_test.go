// VALIDATES: a verify run selects the change set once, writes both answers
// inside its own log directory, names each one to every stage it starts, and
// restores whatever the process held before.
// PREVENTS: the regression recorded on 2026-09-02 in
// plan/journal/refactor-removes-feature.md, where the native port kept the
// selector and dropped its publication, so every run judged all 38 staticcheck
// matrix rows and no stage could read a package answer.

package verifyengine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// scopeSeenByStage is what one stage of a run could read about the change set.
type scopeSeenByStage struct {
	packagesPath string
	tagsPath     string
}

// writeVerifyScopeFixture builds a module the selector can answer about: a
// manifest naming two features, a gated package for each, and one always-on
// package importing neither.
func writeVerifyScopeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		// tmp/ is where a run writes its own logs, status and answers, so the
		// fixture ignores it exactly as this repository does. Without that, the
		// run's own artifacts arrive as untracked changed paths and widen the
		// answer the test is about.
		".gitignore":        "tmp/\n",
		"go.mod":            "module example.com/scopefixture\n\ngo 1.26\n",
		"feature-gates.txt": "# fixture manifest\nze_ssh  ssh\nze_bgp  bgp\n",
		"ssh/ssh.go":        "package ssh\n\n// Listen is the gated feature.\nfunc Listen() int { return 2 }\n",
		"bgp/bgp.go":        "package bgp\n\n// Speak is the other gated feature.\nfunc Speak() int { return 3 }\n",
		"hub/hub.go":        "package hub\n\n// Start runs the always-on hub.\nfunc Start() int { return 0 }\n",
	}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("create the fixture directory for %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write the fixture file %s: %v", name, err)
		}
	}
	return root
}

// commitVerifyScopeFixture turns the fixture into a git checkout whose
// origin/HEAD is its one commit, which is the anchor the selector needs to
// answer about the working tree (repochanged.ResolveAnchor) rather than refuse.
func commitVerifyScopeFixture(t *testing.T, root string) {
	t.Helper()
	commands := [][]string{
		{"init", "-q", "."},
		{"config", "user.email", "tester@example.com"},
		{"config", "user.name", "Tester"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "-m", "base"},
		{"update-ref", "refs/remotes/origin/main", "HEAD"},
		{"symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"},
	}
	for _, args := range commands {
		command := exec.CommandContext(t.Context(), "git", args...) //nolint:gosec // the argument lists are literals above
		command.Dir = root
		if out, err := command.CombinedOutput(); err != nil {
			// Not a skip. Every test in this repository runs inside a git
			// checkout, so a git that cannot build a fixture repository is a
			// broken machine rather than a case this test may decline to make.
			t.Fatalf("git %s in the fixture checkout: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

// runWatchingScope runs one verify of mode's population over root and answers
// what each stage could read about the change set.
//
// The tests of the change-set publication pass FastMode. The full population is
// the release check and publishes the whole tree whatever the change set holds
// (TestFullModePublishesWholeTree), so the change-set publication they judge is
// the one every other population reads.
func runWatchingScope(t *testing.T, root, mode string) (Report, []scopeSeenByStage) {
	t.Helper()
	var seen []scopeSeenByStage
	runner := func(_ context.Context, _ string, identity Identity) ActionResult {
		seen = append(seen, scopeSeenByStage{
			packagesPath: env.Get(repochanged.ScopeFileKey),
			tagsPath:     env.Get(repochanged.ScopeTagsKey),
		})
		return ActionResult{Identity: identity, Registered: true, Completed: true}
	}
	report := RunMode(context.Background(), root, "abc", mode, Plan{}, runner, Slot{})
	if len(seen) == 0 {
		t.Fatal("the run started no stage")
	}
	return report, seen
}

// readAnswerLines reads one published answer into its non-empty lines.
func readAnswerLines(t *testing.T, path string) []string {
	t.Helper()
	body, err := os.ReadFile(path) //nolint:gosec // the path is the answer this test's own run published
	if err != nil {
		t.Fatalf("read the published answer %s: %v", path, err)
	}
	var out []string
	for line := range strings.SplitSeq(string(body), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func TestVerifyRunNamesTheFeatureScopeToEveryStage(t *testing.T) {
	root := writeVerifyScopeFixture(t)
	commitVerifyScopeFixture(t, root)

	report, seen := runWatchingScope(t, root, FastMode)

	first := seen[0]
	if first.packagesPath == "" || first.tagsPath == "" {
		t.Fatalf("the first stage read no change set: %#v", first)
	}
	for index, stage := range seen {
		if stage != first {
			t.Fatalf("stage %d read %#v, want the one answer %#v", index, stage, first)
		}
	}
	inLogDir := filepath.Join(root, filepath.FromSlash(report.LogDir))
	if filepath.Dir(first.packagesPath) != inLogDir {
		t.Errorf("the package answer sits at %s, want it inside %s", first.packagesPath, inLogDir)
	}
	if filepath.Dir(first.tagsPath) != inLogDir {
		t.Errorf("the tag answer sits at %s, want it inside %s", first.tagsPath, inLogDir)
	}
}

func TestVerifyRunPublishesTheChangeSetPerRun(t *testing.T) {
	root := writeVerifyScopeFixture(t)
	commitVerifyScopeFixture(t, root)

	_, firstRun := runWatchingScope(t, root, FastMode)
	_, secondRun := runWatchingScope(t, root, FastMode)

	if firstRun[0].packagesPath == secondRun[0].packagesPath {
		t.Errorf("both runs published their package answer at %s", firstRun[0].packagesPath)
	}
	if firstRun[0].tagsPath == secondRun[0].tagsPath {
		t.Errorf("both runs published their tag answer at %s", firstRun[0].tagsPath)
	}
}

func TestVerifyRunRestoresTheChangeScopeItNamed(t *testing.T) {
	root := writeVerifyScopeFixture(t)
	for _, key := range []string{repochanged.ScopeFileKey, repochanged.ScopeTagsKey} {
		before := env.Get(key)
		t.Cleanup(func() {
			if err := env.Set(key, before); err != nil {
				t.Errorf("restore %s: %v", key, err)
			}
		})
		if err := env.Set(key, "/outer/"+filepath.Base(key)); err != nil {
			t.Fatalf("name %s before the run: %v", key, err)
		}
	}

	runWatchingScope(t, root, FastMode)

	for _, key := range []string{repochanged.ScopeFileKey, repochanged.ScopeTagsKey} {
		if got := env.Get(key); got != "/outer/"+filepath.Base(key) {
			t.Errorf("%s = %q after the run, want the value the run found", key, got)
		}
	}
}

func TestVerifyRunWidensWhenTheChangeSetCannotBeSelected(t *testing.T) {
	// A checkout with no feature manifest is the one input the selector refuses
	// rather than answering about, so the run has nothing to publish.
	root := t.TempDir()

	// The assertion is against what this process ALREADY held, never against the
	// empty string. This test runs inside `./le verify current` as a
	// child of the unit stage, and that run publishes its own answer to every
	// stage it starts: an assertion of "" would judge the run that started the
	// test rather than the fixture the test drives, and it could only fail from
	// inside a verify.
	ambient := scopeSeenByStage{
		packagesPath: env.Get(repochanged.ScopeFileKey),
		tagsPath:     env.Get(repochanged.ScopeTagsKey),
	}

	_, seen := runWatchingScope(t, root, FastMode)

	if seen[0] != ambient {
		t.Fatalf("a refused selection published %#v, want the %#v this process already held", seen[0], ambient)
	}
}

func TestVerifyRunPublishesTheScopedAnswerAGatedChangeProduces(t *testing.T) {
	root := writeVerifyScopeFixture(t)
	commitVerifyScopeFixture(t, root)
	gated := filepath.Join(root, "ssh", "ssh.go")
	if err := os.WriteFile(gated, []byte("package ssh\n\n// Listen is the gated feature.\nfunc Listen() int { return 7 }\n"), 0o600); err != nil {
		t.Fatalf("edit the gated fixture file: %v", err)
	}

	_, seen := runWatchingScope(t, root, FastMode)

	tags := readAnswerLines(t, seen[0].tagsPath)
	if len(tags) != 1 || tags[0] != "ze_ssh" {
		t.Fatalf("the published tag answer = %v, want ze_ssh alone", tags)
	}
	// The package answer is read back through the reader a stage uses, because
	// the file states the checkout it is about on its first line and that line
	// is part of the contract (repochanged.WriteScopePackages).
	answer, code := (repochanged.Scope{Root: root, File: seen[0].packagesPath}).Resolve(nil)
	if code != 0 {
		t.Fatalf("read the published package answer: exit %d", code)
	}
	if len(answer.Packages) != 1 || answer.Packages[0] != "./ssh" {
		t.Fatalf("the published package answer = %v, want ./ssh alone", answer.Packages)
	}
	// And it is about THIS checkout alone: a stage of another one selects its
	// own change set rather than taking this run's.
	elsewhere, code := (repochanged.Scope{Root: t.TempDir(), File: seen[0].packagesPath}).Resolve(nil)
	if code == 0 && len(elsewhere.Packages) == 1 && elsewhere.Packages[0] == "./ssh" {
		t.Error("a caller asking about another checkout was handed this run's answer")
	}
}

// TestFullModePublishesWholeTree proves the release check judges the whole
// tree: over a checkout whose change set is empty (HEAD at its upstream), the
// full population publishes `./...` and every manifest tag, while the fast and
// changed populations publish the empty change set (AC-12).
func TestFullModePublishesWholeTree(t *testing.T) {
	root := writeVerifyScopeFixture(t)
	commitVerifyScopeFixture(t, root)

	_, full := runWatchingScope(t, root, Mode)
	// The first line names the checkout the answer is about.
	if got := readAnswerLines(t, full[0].packagesPath); len(got) != 2 || got[1] != "./..." {
		t.Errorf("the full population published packages %q, want the whole tree ./...", got)
	}
	if got := readAnswerLines(t, full[0].tagsPath); !slices.Equal(got, []string{"ze_bgp", "ze_ssh"}) {
		t.Errorf("the full population published tags %q, want every manifest tag [ze_bgp ze_ssh]", got)
	}

	for _, mode := range []string{FastMode, ChangedMode} {
		_, scoped := runWatchingScope(t, root, mode)
		if got := readAnswerLines(t, scoped[0].packagesPath); len(got) != 1 {
			t.Errorf("the %s population published packages %q over an empty change set, want none", mode, got)
		}
		if got := readAnswerLines(t, scoped[0].tagsPath); len(got) != 0 {
			t.Errorf("the %s population published tags %q over an empty change set, want none", mode, got)
		}
	}
}

// TestChangedPopulationReadsScope proves the changed population holds only
// stages that read the published scope, and none of the release check's
// whole-tree stages (AC-13).
func TestChangedPopulationReadsScope(t *testing.T) {
	readsScope := func(name string) bool {
		switch {
		case name == "go lint/run/changed", name == "verify deps/unit-changed",
			name == FunctionalCommand+"/gating",
			strings.HasPrefix(name, "go staticcheck/check/part/"),
			strings.HasPrefix(name, "verify deps/unit-race-changed/part/"):
			return true
		default:
			return false
		}
	}
	population := StagesForMode(ChangedMode)
	if len(population) == 0 {
		t.Fatal("the changed mode names no population")
	}
	for _, current := range population {
		if !readsScope(current.Identity.Name) {
			t.Errorf("the changed population holds %s, which does not read the published scope", current.Identity.Name)
		}
	}
	for _, wholeTree := range []string{"go lint/run", "repo compiles/check", "verify deps/unit-cached", "verify deps/alloc"} {
		for _, current := range population {
			if current.Identity.Name == wholeTree {
				t.Errorf("the changed population holds the whole-tree stage %s", wholeTree)
			}
		}
	}
}
