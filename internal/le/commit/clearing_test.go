// VALIDATES: the text naming what clears an unjudged stage hands the agent
// `./le verify changed` for a changed-population stage, and for any other stage
// hands Thomas the release-check command (AC-21).
// PREVENTS: a commit message telling an agent to start a full verification.

package commit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// TestUnjudgedStageNamesNoAgentFullRun proves the commit-time text never tells
// an agent to run a full verify: a stage the changed population holds clears
// through ./le verify changed, and any other names the release check only as
// the command to give Thomas; an unjudged stage is diagnosed by its own
// focused command (AC-21).
func TestUnjudgedStageNamesNoAgentFullRun(t *testing.T) {
	scoped := clearingRuns("verify deps/unit-changed", verifyengine.Mode)
	if !strings.Contains(scoped, "./le verify changed") || strings.Contains(scoped, "verify worktree") {
		t.Errorf("a changed-population stage clears with %q, want ./le verify changed only", scoped)
	}
	release := clearingRuns("verify deps/alloc", verifyengine.ChangedMode)
	if !strings.Contains(release, "give Thomas the command ./le verify worktree") {
		t.Errorf("a release-only stage clears with %q, want the command handed to Thomas", release)
	}
	for _, text := range []string{scoped, release, unreadableIndex("the index", errTestUnreadable)} {
		if strings.Contains(text, "verify current") || strings.Contains(text, "clears when a verify run reaches it: le verify worktree") {
			t.Errorf("%q tells the agent to run a full verify", text)
		}
	}
	if got := stageCommand("doc check/verify"); got != "./le doc check verify" {
		t.Errorf("the focused command of doc check/verify is %q", got)
	}
}

// TestAFullRedOnASharedStageNamesNoChangedClear proves the commit gate charges
// a Staticcheck part's red that a full run recorded and a changed run carried,
// and says only the release check clears it: a changed run judges that part
// over its scope's rows alone. A red the changed population judged on the same
// part clears through ./le verify changed.
// VALIDATES: AC-21. PREVENTS: an agent told a scoped run clears a full-run red.
func TestAFullRedOnASharedStageNamesNoChangedClear(t *testing.T) {
	part := ""
	for _, stage := range verifyengine.StagesForMode(verifyengine.ChangedMode) {
		if strings.HasPrefix(stage.Identity.Name, "go staticcheck/check/part/") {
			part = stage.Identity.Name
			break
		}
	}
	if part == "" {
		t.Fatal("the changed population holds no Staticcheck part")
	}
	for _, judge := range []struct{ mode, want, never string }{
		{verifyengine.Mode, "give Thomas the command ./le verify worktree", "clears when ./le verify changed"},
		{verifyengine.ChangedMode, "clears when ./le verify changed reaches it", "verify worktree"},
	} {
		root := t.TempDir()
		index := `{"format": ` + strconv.Itoa(verifyengine.FailureIndexFormat) + `, "mode": "` + verifyengine.ChangedMode +
			`", "stages": [{"stage": "` + part + `", "reached": true, "exit-code": 1, "outside-run": true,` +
			` "carried-from": {"commit": "aaa", "run-dir": "run-aaa", "mode": "` + judge.mode + `"}}]}` + "\n"
		path := filepath.Join(root, filepath.FromSlash(verifyengine.FailuresJSONPath))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(index), 0o600); err != nil {
			t.Fatal(err)
		}
		reds := structuralGateReds(root, nil)
		if len(reds.Charged) != 1 || reds.Charged[0] != part || len(reds.Causes) != 1 {
			t.Fatalf("judged by %s: charged %v causes %v, want %s charged", judge.mode, reds.Charged, reds.Causes, part)
		}
		if !strings.Contains(reds.Causes[0], judge.want) || strings.Contains(reds.Causes[0], judge.never) {
			t.Errorf("judged by %s, the cause is %q, want %q and never %q", judge.mode, reds.Causes[0], judge.want, judge.never)
		}
	}
}

var errTestUnreadable = stringError("unreadable")

type stringError string

func (e stringError) Error() string { return string(e) }
