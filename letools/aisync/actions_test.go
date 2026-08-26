package aisync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: The three verbs reach the checkout that the environment names.
// Each verb answers the exit code that its caller reads. Every unreadable or
// unwritable path is an error rather than a silent partial sync.
// PREVENTS: A swap from pointing `make ze-ai-skills-sync` and the session hook
// at a command whose verbs answer the wrong codes. It also prevents the shell
// failure class where a copy does not occur but the run reports success.

// useCheckout points every verb in this test at one checkout.
//
// env.Get answers from a CACHE built once from os.Environ(). Thus, t.Setenv
// alone changes nothing that lepath.Root() sees. Without a cache reset, the test
// would generate over the DEVELOPER's own tree. The reset makes the variable
// live. The cleanup stops this test's value from outliving the test.
func useCheckout(t *testing.T, root string) {
	t.Helper()
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// notRoot skips a case that proves a permission is honored. Root ignores the
// mode bits, so the case would pass for the wrong reason.
func notRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root, so a mode bit proves nothing")
	}
}

func TestTheSyncVerbWritesTheCheckoutTheEnvironmentNames(t *testing.T) {
	root := fixture(t)
	useCheckout(t, root)

	payload, code := Answer([]string{"skills-sync"})
	if code != 0 {
		t.Fatalf("the sync verb exited %d", code)
	}
	report, isReport := payload.(Report)
	if !isReport {
		t.Fatalf("the payload is %T, want a Report", payload)
	}
	if report.Mode != modeSync {
		t.Errorf("the report's mode is %q, want %q", report.Mode, modeSync)
	}
	if paths := generatedPaths(t, root); len(paths) != 9 {
		t.Errorf("%d files written, want exactly 9: %v", len(paths), paths)
	}
}

func TestTheCheckVerbAnswersZeroForAFreshTreeAndOneForAStaleOne(t *testing.T) {
	root := fixture(t)
	useCheckout(t, root)

	if _, code := Answer([]string{"skills-sync"}); code != 0 {
		t.Fatalf("the sync verb exited %d", code)
	}
	if _, code := Answer([]string{"sync-check"}); code != 0 {
		t.Fatalf("the check verb exited %d over a freshly synced tree", code)
	}

	write(t, root, filepath.Join(".claude", "skills", "ze-spec", "SKILL.md"), "edited\n")
	payload, code := Answer([]string{"sync-check"})
	if code != 1 {
		t.Fatalf("the check verb exited %d over a stale tree, want 1", code)
	}
	report, isReport := payload.(Report)
	if !isReport || report.Fresh() {
		t.Errorf("the stale check answered %v", payload)
	}
}

func TestThePreviewVerbWritesNothing(t *testing.T) {
	root := fixture(t)
	useCheckout(t, root)

	if _, code := Answer([]string{"sync-preview"}); code != 0 {
		t.Fatalf("the preview verb exited %d", code)
	}
	if paths := generatedPaths(t, root); len(paths) != 0 {
		t.Errorf("the preview wrote %d files: %v", len(paths), paths)
	}
}

// Each verb answers 1 for a checkout that holds no source, rather than
// reporting a run over nothing.
func TestEveryVerbRefusesACheckoutWithNoSource(t *testing.T) {
	useCheckout(t, t.TempDir())

	for _, verb := range []string{"skills-sync", "sync-check", "sync-preview"} {
		if payload, code := Answer([]string{verb}); code == 0 {
			t.Errorf("%q answered code 0 with payload %v over a tree holding no source", verb, payload)
		}
	}
}

// A source the process cannot read is an error, not a mirror left as it was.
func TestASourceThatCannotBeReadIsAnError(t *testing.T) {
	notRoot(t)
	root := fixture(t)
	locked := filepath.Join(root, skillSources, "ze-spec.md")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o600) }) //nolint:errcheck // best-effort cleanup

	if _, err := (Mirror{Root: root}).Sync(); err == nil {
		t.Error("a source that cannot be read synced successfully")
	}
}

// A target the process cannot write is an error, not a sync that reports the
// files it failed to write.
func TestATargetThatCannotBeWrittenIsAnError(t *testing.T) {
	notRoot(t)
	root := fixture(t)
	blocked := filepath.Join(root, claudeSkills)
	if err := os.MkdirAll(blocked, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Read and traverse, never write: the sync must fail creating the skill's
	// own directory under it.
	if err := os.Chmod(blocked, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0o750) }) //nolint:errcheck // best-effort cleanup

	if _, err := (Mirror{Root: root}).Sync(); err == nil {
		t.Error("a target that cannot be written synced successfully")
	}
}

// A source DIRECTORY that is a file is a broken checkout, and reading it must
// answer the error rather than an empty source list.
func TestASourceDirectoryThatIsAFileIsAnError(t *testing.T) {
	root := t.TempDir()
	write(t, root, skillSources, "not a directory\n")

	if _, err := (Mirror{Root: root}).Sync(); err == nil {
		t.Error("a checkout whose skills directory is a file synced successfully")
	}
}

// Only markdown FILES are sources. A directory whose name ends in .md is not
// one, and neither is a file with another extension.
func TestOnlyMarkdownFilesOfTheSourceDirectoryAreSources(t *testing.T) {
	root := fixture(t)
	if err := os.MkdirAll(filepath.Join(root, skillSources, "not-a-skill.md"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write(t, root, filepath.Join(skillSources, "notes.txt"), "not a skill\n")

	report, err := Mirror{Root: root}.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(report.Skills) != 2 {
		t.Fatalf("%d skills, want exactly 2: %v", len(report.Skills), report.Skills)
	}
	for _, name := range report.Skills {
		if strings.HasPrefix(name, "not-a-skill") || strings.HasPrefix(name, "notes") {
			t.Errorf("%q was taken as a skill", name)
		}
	}
}

// The check must detect drift in both instruction files. These generated files
// are the largest, and an agent reads them first.
func TestTheCheckSeesDriftInTheInstructionFiles(t *testing.T) {
	root := fixture(t)
	if _, err := (Mirror{Root: root}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	write(t, root, claudeInstructions, "edited by hand\n")

	report, err := Mirror{Root: root}.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(report.Stale) != 1 || report.Stale[0] != claudeInstructions {
		t.Errorf("stale is %v, want exactly [%s]", report.Stale, claudeInstructions)
	}
}

// A checkout can hold no agent definition and still be a checkout. Three
// definitions exist today. A tree with none must sync its skills rather than
// receive a refusal.
func TestACheckoutWithNoAgentDefinitionStillSyncs(t *testing.T) {
	root := fixture(t)
	if err := os.RemoveAll(filepath.Join(root, agentSources)); err != nil {
		t.Fatalf("remove: %v", err)
	}

	report, err := Mirror{Root: root}.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(report.Agents) != 0 {
		t.Errorf("%d agents reported, want 0", len(report.Agents))
	}
	if paths := generatedPaths(t, root); len(paths) != 8 {
		t.Errorf("%d files written, want exactly 8: %v", len(paths), paths)
	}
}
