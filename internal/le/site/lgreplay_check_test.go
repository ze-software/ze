package site

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

// lgReplayHistory makes a checkout with two commits on main and one on a
// branch that main does not hold. It answers the checkout and the three
// commits: main's tip, the commit before it, and the branch's.
func lgReplayHistory(t *testing.T) (repository, head, parent, side string) {
	t.Helper()
	repository = t.TempDir()
	commit := func(message string) string {
		t.Helper()
		roadmapGit(t, repository, "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", message)
		return roadmapGit(t, repository, "rev-parse", "HEAD")
	}
	roadmapGit(t, repository, "init", "-q", "-b", "main")
	parent = commit("first")
	head = commit("second")
	roadmapGit(t, repository, "checkout", "-q", "-b", "side")
	side = commit("on a branch main does not hold")
	roadmapGit(t, repository, "checkout", "-q", "main")
	return repository, head, parent, side
}

// lgReplayBuiltPage writes into output the page a build makes of recording:
// the block lgReplayBlockHTML renders, inside a document. The check is then
// judged on the marks the producer wrote, never on marks a test spelled.
func lgReplayBuiltPage(t *testing.T, output string, recording sitelgreplay.Recording) {
	t.Helper()
	source := lgReplayCheckout(t, false)
	lgReplayWriteRecording(t, source, recording)
	block, err := lgReplayBlockHTML(source, lgReplayPage)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(output, filepath.FromSlash(lgReplayPage)),
		"<!doctype html><html><body><main>\n"+block+"\n</main></body></html>\n")
}

// lgReplayCheckedRefusal requires exactly one refusal that is an operator
// error whose last line is the command fix, and answers it.
func lgReplayCheckedRefusal(t *testing.T, refusals []replayRefusal, fix string) *opererr.Error {
	t.Helper()
	if len(refusals) != 1 {
		t.Fatalf("the check answered %d refusals, want 1: %+v", len(refusals), refusals)
	}
	var refusal *opererr.Error
	if !errors.As(refusals[0].operatorError(), &refusal) {
		t.Fatalf("the refusal is no operator error: %+v", refusals[0])
	}
	if !strings.HasSuffix(refusal.Error(), "\nfix: "+fix) {
		t.Errorf("the fix %q is not the refusal's last line:\n%v", fix, refusal)
	}
	if exit := (checkReport{Replay: refusals}).exit(); exit != 1 {
		t.Errorf("a check with a replay refusal exits %d, want 1", exit)
	}
	return refusal
}

// TestSiteCheckRefusesATrialRecording proves ./le site check refuses an
// artifact whose replay comes from a recording of a checkout with uncommitted
// changes, with the command that records from a committed tree as its fix.
// The page is the one the producer builds, so the mark is read from where it
// is written. This is the refusal the build held until 2026-11-19.
func TestSiteCheckRefusesATrialRecording(t *testing.T) {
	repository, head, _, _ := lgReplayHistory(t)
	paths := Paths{Repository: repository, Output: t.TempDir()}
	trial := lgReplaySample()
	trial.Provenance.Commit = head
	trial.Provenance.TreeDirty = true
	lgReplayBuiltPage(t, paths.Output, trial)

	refusals, err := checkLGReplay(t.Context(), paths)
	if err != nil {
		t.Fatal(err)
	}
	refusal := lgReplayCheckedRefusal(t, refusals, sitelgreplay.RecorderCommand)
	if !strings.Contains(refusal.Problem(), "comes from a trial recording") {
		t.Errorf("the refusal does not name the trial recording: %s", refusal.Problem())
	}
	for _, want := range []string{
		sitelgreplay.File + " was recorded from a checkout with uncommitted changes",
		"did not alone produce the answers",
		"record again from the committed tree",
	} {
		if !strings.Contains(refusal.Why(), want) {
			t.Errorf("the refusal does not say %q: %s", want, refusal.Why())
		}
	}
}

// TestSiteCheckRefusesARecordingWhoseCommitIsNotInHistory proves the check
// refuses a replay whose recording names a commit the checkout's HEAD does
// not descend from: one on a branch main does not hold, and one the checkout
// does not hold at all, which is what a rebase leaves behind. Each refusal
// names the commit and the recorder as its fix.
func TestSiteCheckRefusesARecordingWhoseCommitIsNotInHistory(t *testing.T) {
	repository, _, _, side := lgReplayHistory(t)
	for name, commit := range map[string]string{
		"a commit on another branch":          side,
		"a commit the checkout does not hold": "0123456789abcdef0123456789abcdef01234567",
	} {
		t.Run(name, func(t *testing.T) {
			paths := Paths{Repository: repository, Output: t.TempDir()}
			recording := lgReplaySample()
			recording.Provenance.Commit = commit
			lgReplayBuiltPage(t, paths.Output, recording)

			refusals, err := checkLGReplay(t.Context(), paths)
			if err != nil {
				t.Fatal(err)
			}
			refusal := lgReplayCheckedRefusal(t, refusals, sitelgreplay.RecorderCommand)
			if !strings.Contains(refusal.Problem(), "names commit "+commit+", which is not in the history of this checkout") {
				t.Errorf("the refusal does not name the commit: %s", refusal.Problem())
			}
		})
	}
}

// TestSiteCheckRefusesAReplayItCannotJudge proves the check does not read a
// replay it cannot judge as one that passed: a commit that is no commit id is
// refused with the recorder as its fix, and a page that mounts a replay and
// states no commit, as an artifact built before the mark existed does, is
// refused with the build as its fix.
func TestSiteCheckRefusesAReplayItCannotJudge(t *testing.T) {
	repository, _, _, _ := lgReplayHistory(t)

	paths := Paths{Repository: repository, Output: t.TempDir()}
	recording := lgReplaySample()
	recording.Provenance.Commit = "--upload-pack=x"
	lgReplayBuiltPage(t, paths.Output, recording)
	refusals, err := checkLGReplay(t.Context(), paths)
	if err != nil {
		t.Fatal(err)
	}
	refusal := lgReplayCheckedRefusal(t, refusals, sitelgreplay.RecorderCommand)
	if !strings.Contains(refusal.Problem(), `"--upload-pack=x" as its commit, which is not a commit id`) {
		t.Errorf("the refusal does not name the text that is no commit: %s", refusal.Problem())
	}

	paths = Paths{Repository: repository, Output: t.TempDir()}
	writeFixtureFile(t, filepath.Join(paths.Output, filepath.FromSlash(lgReplayPage)),
		`<div class="lg-replay"><div data-lg-replay data-lg-replay-frame="replay/frame.html"></div></div>`)
	refusals, err = checkLGReplay(t.Context(), paths)
	if err != nil {
		t.Fatal(err)
	}
	refusal = lgReplayCheckedRefusal(t, refusals, "./le site build")
	if !strings.Contains(refusal.Problem(), "does not say which recording it comes from") {
		t.Errorf("the refusal does not say the page states no commit: %s", refusal.Problem())
	}
}

// TestSiteCheckPassesACommittedRecording proves the check refuses nothing it
// should publish: a recording of a committed tree whose commit is the
// checkout's HEAD or an ancestor of it, a page that says the looking glass is
// not recorded yet, and an artifact without the page.
func TestSiteCheckPassesACommittedRecording(t *testing.T) {
	repository, head, parent, _ := lgReplayHistory(t)
	for name, commit := range map[string]string{"the checkout's HEAD": head, "an ancestor of HEAD": parent} {
		t.Run(name, func(t *testing.T) {
			paths := Paths{Repository: repository, Output: t.TempDir()}
			recording := lgReplaySample()
			recording.Provenance.Commit = commit
			lgReplayBuiltPage(t, paths.Output, recording)
			refusals, err := checkLGReplay(t.Context(), paths)
			if err != nil {
				t.Fatal(err)
			}
			if len(refusals) != 0 {
				t.Fatalf("the check refused a committed recording of %s: %+v", name, refusals)
			}
			if exit := (checkReport{Replay: refusals}).exit(); exit != 0 {
				t.Fatalf("a check with no replay refusal and nothing else wrong exits %d", exit)
			}
		})
	}

	t.Run("a page with no recording", func(t *testing.T) {
		paths := Paths{Repository: repository, Output: t.TempDir()}
		block, err := lgReplayBlockHTML(lgReplayCheckout(t, false), lgReplayPage)
		if err != nil {
			t.Fatal(err)
		}
		writeFixtureFile(t, filepath.Join(paths.Output, filepath.FromSlash(lgReplayPage)), block)
		refusals, err := checkLGReplay(t.Context(), paths)
		if err != nil || len(refusals) != 0 {
			t.Fatalf("refusals=%+v err=%v, want none for a page that carries no replay", refusals, err)
		}
	})

	t.Run("an artifact without the page", func(t *testing.T) {
		paths := Paths{Repository: repository, Output: t.TempDir()}
		refusals, err := checkLGReplay(t.Context(), paths)
		if err != nil || len(refusals) != 0 {
			t.Fatalf("refusals=%+v err=%v, want none for an artifact without the page", refusals, err)
		}
	})
}

// TestSiteCheckActionReportsTheReplay proves the registered check action
// reaches the replay check: over a built artifact, ./le site check answers
// the trial refusal in its report and exits 1, and answers no replay refusal
// once the page comes from a recording of the checkout's own commit.
func TestSiteCheckActionReportsTheReplay(t *testing.T) {
	paths, source := publicationFixture(t)
	check := func() (checkReport, int) {
		t.Helper()
		payload, code := Answer([]string{"check", "output", paths.Output})
		report, ok := payload.(checkReport)
		if !ok {
			t.Fatalf("check exited %d and returned %T", code, payload)
		}
		return report, code
	}

	trial := lgReplaySample()
	trial.Provenance.Commit = source
	trial.Provenance.TreeDirty = true
	lgReplayBuiltPage(t, paths.Output, trial)
	report, code := check()
	if code != 1 {
		t.Fatalf("check of an artifact with a trial replay exited %d, want 1", code)
	}
	refusal := lgReplayCheckedRefusal(t, report.Replay, sitelgreplay.RecorderCommand)
	if !strings.Contains(refusal.Problem(), "comes from a trial recording") {
		t.Errorf("the action's refusal does not name the trial recording: %s", refusal.Problem())
	}

	committed := lgReplaySample()
	committed.Provenance.Commit = source
	lgReplayBuiltPage(t, paths.Output, committed)
	// The page is written beside the fixture's build, with no mirror and no
	// producer claim, so the other checks still refuse this artifact: what
	// this reads is that the replay check no longer does.
	report, _ = check()
	if len(report.Replay) != 0 {
		t.Fatalf("the action refused the replay of a committed recording: %+v", report.Replay)
	}
}
