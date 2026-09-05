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
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/changed"
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

// commitVerifyScopeFixture turns the fixture into a git checkout with a green
// baseline, which is what lets the selector answer about the working tree
// instead of widening for want of a proven commit.
func commitVerifyScopeFixture(t *testing.T, root string) {
	t.Helper()
	commands := [][]string{
		{"init", "-q", "."},
		{"config", "user.email", "tester@example.com"},
		{"config", "user.name", "Tester"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "-m", "base"},
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
	head := exec.CommandContext(t.Context(), "git", "rev-parse", "HEAD")
	head.Dir = root
	sha, err := head.Output()
	if err != nil {
		t.Fatalf("read the fixture HEAD: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		t.Fatalf("create the fixture status directory: %v", err)
	}
	status := "exit=0\nmode=full\ngit_sha=" + strings.TrimSpace(string(sha)) + "\n"
	if err := os.WriteFile(filepath.Join(root, "tmp", "ze-verify.status"), []byte(status), 0o600); err != nil {
		t.Fatalf("write the fixture verify status: %v", err)
	}
}

// runWatchingScope runs one verify over root and answers what each stage could
// read about the change set.
func runWatchingScope(t *testing.T, root string) (Report, []scopeSeenByStage) {
	t.Helper()
	var seen []scopeSeenByStage
	runner := func(_ context.Context, _ string, identity Identity) ActionResult {
		seen = append(seen, scopeSeenByStage{
			packagesPath: env.Get(changed.ScopeFileKey),
			tagsPath:     env.Get(changed.ScopeTagsKey),
		})
		return ActionResult{Identity: identity, Registered: true, Completed: true}
	}
	report := Run(context.Background(), root, "abc", runner)
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

	report, seen := runWatchingScope(t, root)

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

	_, firstRun := runWatchingScope(t, root)
	_, secondRun := runWatchingScope(t, root)

	if firstRun[0].packagesPath == secondRun[0].packagesPath {
		t.Errorf("both runs published their package answer at %s", firstRun[0].packagesPath)
	}
	if firstRun[0].tagsPath == secondRun[0].tagsPath {
		t.Errorf("both runs published their tag answer at %s", firstRun[0].tagsPath)
	}
}

func TestVerifyRunRestoresTheChangeScopeItNamed(t *testing.T) {
	root := writeVerifyScopeFixture(t)
	for _, key := range []string{changed.ScopeFileKey, changed.ScopeTagsKey} {
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

	runWatchingScope(t, root)

	for _, key := range []string{changed.ScopeFileKey, changed.ScopeTagsKey} {
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
	// empty string. This test runs inside `./le verify current mode full` as a
	// child of the unit stage, and that run publishes its own answer to every
	// stage it starts: an assertion of "" would judge the run that started the
	// test rather than the fixture the test drives, and it could only fail from
	// inside a verify.
	ambient := scopeSeenByStage{
		packagesPath: env.Get(changed.ScopeFileKey),
		tagsPath:     env.Get(changed.ScopeTagsKey),
	}

	_, seen := runWatchingScope(t, root)

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

	_, seen := runWatchingScope(t, root)

	tags := readAnswerLines(t, seen[0].tagsPath)
	if len(tags) != 1 || tags[0] != "ze_ssh" {
		t.Fatalf("the published tag answer = %v, want ze_ssh alone", tags)
	}
	packages := readAnswerLines(t, seen[0].packagesPath)
	if len(packages) != 1 || packages[0] != "./ssh" {
		t.Fatalf("the published package answer = %v, want ./ssh alone", packages)
	}
}
