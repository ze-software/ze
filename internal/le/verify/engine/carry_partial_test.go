// VALIDATES: a full-population red on a stage a partial population judges only
// over its scope is replaced only by a run that judges the stage in full, a
// partial run's own red stays charged, and a stage a partial run judges at its
// full identity takes that run's verdict; the status check's unreadable-record
// text names the scoped run an agent starts, never a full verify.
// PREVENTS: a partial red then a partial green erasing a full run's red over
// tags the partial runs never judged (review round 2, finding 1), a fast green
// on a full-identity stage being refused (finding 5), and an agent told to
// start a full verify (finding 2, AC-22).
package verifyengine

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/job"
)

// writeOneStage publishes an index holding the one stage at exit.
func writeOneStage(t *testing.T, root, stageName, commit, mode string, exit int) {
	t.Helper()
	index := artifactIndex{Format: FailureIndexFormat, Mode: mode, Commit: commit, RunDir: "run-" + commit,
		Stages: []artifactStage{{Stage: stageName, Reached: true, ExitCode: exit}}}
	if err := writeFailureIndex(root, index); err != nil {
		t.Fatal(err)
	}
}

// TestAPartialRedThenGreenKeepsAFullRedCharged proves, for the changed and the
// fast population, the sequence full red, partial red, partial green on a
// Staticcheck part: the full run's red is still the entry after both partial
// runs, carried from the full run with the full population as its judge, so
// the tags only the full run judged stay charged. It also proves a partial
// red over a full green is held as the partial run's own red.
//
// Method: publish the three indexes in order over one checkout and read the
// part's entry; then, over a fresh checkout, publish a full green and a
// partial red and require the partial red, not carried.
func TestAPartialRedThenGreenKeepsAFullRedCharged(t *testing.T) {
	part := staticcheckStages()[0].Identity.Name
	for _, mode := range []string{ChangedMode, FastMode} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			writeOneStage(t, root, part, "aaa", Mode, 1)
			writeOneStage(t, root, part, "bbb", mode, 1)
			writeOneStage(t, root, part, "ccc", mode, 0)
			entry := carriedEntry(t, readCarried(t, root), part)
			if entry.ExitCode != 1 || entry.CarriedFrom == nil || entry.CarriedFrom.Commit != "aaa" {
				t.Fatalf("%s after full red, %s red, %s green = %#v, want the full red carried from aaa",
					part, mode, mode, entry)
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FailuresJSONPath)))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), `"mode": "`+Mode+`"`) {
				t.Fatalf("the carried red does not name the full population as its judge:\n%s", data)
			}

			own := t.TempDir()
			writeOneStage(t, own, part, "ddd", Mode, 0)
			writeOneStage(t, own, part, "eee", mode, 1)
			entry = carriedEntry(t, readCarried(t, own), part)
			if entry.ExitCode != 1 || entry.OutsideRun || entry.CarriedFrom != nil {
				t.Fatalf("%s after full green then %s red = %#v, want the %s run's own red, not carried",
					part, mode, entry, mode)
			}
		})
	}
}

// TestAFullIdentityStageTakesAPartialRunsVerdict proves the shared set holds
// only the stages a partial population judges over its scope: in fast mode
// `go lint/run` and `go patch-stamps/check` run at their full identity, so a
// fast green replaces a full red there, while a Staticcheck part, the gating
// suite and arch enumeration read the published scope. In changed mode a race
// piece is a full judgement and a changed-only stage is not shared at all.
//
// Method: ask JudgesInPart for each stage, then publish a full red on
// `go lint/run` and a fast green over it, and require the green, not carried.
func TestAFullIdentityStageTakesAPartialRunsVerdict(t *testing.T) {
	staticcheck := staticcheckStages()[0].Identity.Name
	race := raceStages()[0].Identity.Name
	for _, check := range []struct {
		mode, name string
		want       bool
	}{
		{FastMode, "go lint/run", false},
		{FastMode, "go patch-stamps/check", false},
		{FastMode, race, false},
		{FastMode, staticcheck, true},
		{FastMode, FunctionalCommand + "/gating", true},
		{FastMode, "arch enumeration/check", true},
		{ChangedMode, race, false},
		{ChangedMode, "verify deps/unit-changed", false},
		{ChangedMode, staticcheck, true},
		{ChangedMode, FunctionalCommand + "/gating", true},
		{Mode, staticcheck, false},
	} {
		if got := JudgesInPart(check.mode, check.name); got != check.want {
			t.Errorf("JudgesInPart(%s, %s) = %v, want %v", check.mode, check.name, got, check.want)
		}
	}

	root := t.TempDir()
	writeOneStage(t, root, "go lint/run", "aaa", Mode, 1)
	writeOneStage(t, root, "go lint/run", "bbb", FastMode, 0)
	entry := carriedEntry(t, readCarried(t, root), "go lint/run")
	if entry.ExitCode != 0 || entry.OutsideRun || entry.CarriedFrom != nil {
		t.Fatalf("go lint/run after full red then fast green = %#v, want the fast green, not carried", entry)
	}
}

// TestACutFullRunPieceIsAFullJudgement pins JudgesInPart for the mode a cut
// full run writes (`full-part-i-of-n`, Part.Name). Each piece judges its
// stages over the whole tree, because the full population publishes the whole
// tree as its scope, so its verdict on a stage is a full judgement and replaces
// the entry, a Scoped stage included.
//
// Method: name every piece of a six-way cut of the full population and ask
// JudgesInPart about each stage that piece runs, the Scoped ones included.
func TestACutFullRunPieceIsAFullJudgement(t *testing.T) {
	const pieces = 6
	scoped := 0
	for index := 1; index <= pieces; index++ {
		part := Part{Index: index, Of: pieces}
		mode := part.Name(Mode)
		if mode != "full-part-"+strconv.Itoa(index)+"-of-"+strconv.Itoa(pieces) {
			t.Fatalf("Part{%d, %d}.Name(%q) = %q, want the cut full-run mode name", index, pieces, Mode, mode)
		}
		dealt, err := Dealt(Mode, part)
		if err != nil {
			t.Fatalf("Dealt(%s, %s): %v", Mode, mode, err)
		}
		for _, stage := range dealt {
			if stage.Scoped {
				scoped++
			}
			if JudgesInPart(mode, stage.Identity.Name) {
				t.Errorf("JudgesInPart(%s, %s) = true, want false: a cut full-run piece judges the whole tree",
					mode, stage.Identity.Name)
			}
		}
	}
	if scoped == 0 {
		t.Fatal("no piece of the cut full run holds a Scoped stage, so this test asks nothing of them")
	}
}

// TestAnUnreadableVerifyRecordNamesTheScopedRun proves both unreadable-record
// answers of the status check name the run an agent starts, and the release
// check only as Thomas's, never "run a full verify" (AC-22).
//
// Method: write a certificate, turn the manifest and then the status file into
// directories so each read fails with an error other than not-exist, and read
// the scoped and the whole-tree answer.
func TestAnUnreadableVerifyRecordNamesTheScopedRun(t *testing.T) {
	root := statusFixture(t)
	start := job.SnapshotTree(root)
	if _, err := WriteCertificate(root, WriteRequest{
		Exit: 0, Mode: Mode, GitSHA: job.Head(root), Start: start, At: time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{ManifestPath, StatusPath} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o750); err != nil {
			t.Fatal(err)
		}
		got := CheckCertificate(root, []string{"changed.txt"})
		if got.Fresh {
			t.Fatalf("%s unreadable reads fresh: %#v", rel, got)
		}
		if !strings.Contains(got.Reason, "unreadable") || !strings.Contains(got.Reason, "./le verify changed for an agent") ||
			!strings.Contains(got.Reason, "a release check is Thomas's to start") {
			t.Errorf("%s unreadable answers %q, want the scoped run and the release check as Thomas's", rel, got.Reason)
		}
		if strings.Contains(got.Reason, "full verify") {
			t.Errorf("%s unreadable answers %q, which tells an agent to run a full verify", rel, got.Reason)
		}
	}
}
