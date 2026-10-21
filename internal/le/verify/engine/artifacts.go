// Design: docs/functional-tests.md -- published verification failure protocol
package verifyengine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// CombinedLogPath is the stable reader-facing log of the latest run.
	CombinedLogPath = "tmp/ze-verify.log"
	// FailuresLogPath is the stable human failure index of the latest run.
	FailuresLogPath = "tmp/ze-verify-failures.log"
	// FailuresJSONPath is the stable machine failure index of the latest run.
	FailuresJSONPath = "tmp/ze-verify-failures.json"
	// FullJSONPath preserves the latest full-mode machine index when a changed
	// run publishes its cheaper result.
	FullJSONPath = "tmp/ze-verify-full.json"
	// failuresLockPath serializes the read-merge-write of the failure index
	// between sessions that publish into the same checkout.
	failuresLockPath = "tmp/ze-verify-failures.lock"
)

// Group is one failure a stage declared about itself: the paths it names, the
// command that reruns it alone, and the log that holds the detail. It is the
// unit both the published failure index and the in-flight query read, so the
// two surfaces cannot disagree about what a red is about.
type Group struct {
	Stage     string   `json:"stage"`
	GroupID   string   `json:"group-id"`
	Kind      string   `json:"kind"`
	Related   []string `json:"related"`
	Summary   string   `json:"summary"`
	Rerun     string   `json:"rerun"`
	DetailLog string   `json:"detail-log"`
	Parallel  string   `json:"parallel"`
	Excerpt   []string `json:"excerpt,omitempty"`
}

// artifactStage is one stage of the full population in the failure index.
//
// A stage this run held is reached, or, after a first-red stop, not reached
// with OutsideRun false: the run meant to judge it and did not, and
// structuralGateReds in internal/le/commit charges it (AC-11).
//
// A stage this run did not hold (a piece of a cut run, the fast population, a
// run that broke) has OutsideRun true and carries the earlier index's verdict
// for it, with CarriedFrom naming the run that judged it (carryForward). One
// no earlier run judged is not reached and carries nothing: it is unjudged,
// never red, and a reader MUST NOT read its zero exit as a green.
type artifactStage struct {
	Stage       string         `json:"stage"`
	Reached     bool           `json:"reached"`
	ExitCode    int            `json:"exit-code"`
	DetailLog   string         `json:"detail-log"`
	Start       string         `json:"start,omitempty"`
	Seconds     float64        `json:"seconds"`
	Groups      []Group        `json:"groups,omitempty"`
	OutsideRun  bool           `json:"outside-run,omitempty"`
	CarriedFrom *carriedOrigin `json:"carried-from,omitempty"`
}

// carriedOrigin names the run whose verdict a carried stage repeats.
type carriedOrigin struct {
	Commit      string `json:"commit"`
	RunDir      string `json:"run-dir"`
	GeneratedAt string `json:"generated-at"`
}

type artifactIndex struct {
	Mode        string `json:"mode"`
	ExitCode    int    `json:"exit-code"`
	RunDir      string `json:"run-dir"`
	CombinedLog string `json:"combined-log"`
	GeneratedAt string `json:"generated-at"`
	Commit      string `json:"commit,omitempty"`
	// CarryNote says what the earlier index at the checkout gave carryForward:
	// the run it came from, or that it was missing or malformed and so treated
	// as empty.
	CarryNote string `json:"carry-note,omitempty"`
	// StoppedAt names the stage a first-red run stopped after; empty for a
	// run that went to its end.
	StoppedAt string          `json:"stopped-at,omitempty"`
	Stages    []artifactStage `json:"stages"`
}

func writeRunArtifacts(root string, report Report, at time.Time) error {
	combinedLog := filepath.ToSlash(filepath.Join(report.LogDir, "ze-verify.log"))
	index := artifactIndex{
		Mode: report.Mode, ExitCode: report.Code, RunDir: report.LogDir,
		CombinedLog: combinedLog, GeneratedAt: at.UTC().Format(time.RFC3339),
		Commit: report.Commit, StoppedAt: report.StoppedAt,
		Stages: make([]artifactStage, 0, len(report.Stages)+len(report.Unreached)),
	}
	for _, stage := range report.Stages {
		result := artifactStage{
			Stage: stage.Identity.Name, Reached: true, ExitCode: stage.Code, DetailLog: stage.Log,
			Start: stage.Start.UTC().Format(time.RFC3339), Seconds: stage.Seconds,
		}
		if stage.Code != 0 {
			if declared, complete := DeclaredGroups(root, stage); complete {
				result.Groups = declared
			} else {
				summary := fmt.Sprintf("stage exited %d", stage.Code)
				if stage.Failure != nil && stage.Failure.Message != "" {
					summary = stage.Failure.Message
				}
				result.Groups = []Group{{
					Stage: stage.Identity.Name, GroupID: "stage:" + stage.Identity.Name,
					Kind: "generic", Related: []string{}, Summary: summary,
					Rerun: "le " + invocation(stage.Identity), DetailLog: stage.Log,
					Parallel: "independent; rerun this stage alone",
				}}
			}
		}
		index.Stages = append(index.Stages, result)
	}
	for _, unreached := range report.Unreached {
		index.Stages = append(index.Stages, artifactStage{Stage: unreached.Name})
	}
	for _, artifact := range []string{CombinedLogPath, filepath.ToSlash(filepath.Join("tmp", "verify", "ze-verify.log"))} {
		if err := atomicWrite(root, artifact, []byte(report.Console)); err != nil {
			return fmt.Errorf("write verify artifact %s: %w", artifact, err)
		}
	}
	return writeFailureIndex(root, index)
}

// writeFailureIndex publishes index into the checkout at root, merged with the
// index already there, as the JSON and text failure index (and the full-mode
// copy for a full run).
//
// The checkout's index is whichever run wrote last, and the commit gate
// charges structural reds from it (structuralGateReds in internal/le/commit).
// A piece of a cut run, a fast run, and a run that broke each hold only part
// of the full population, so the stages they did not hold take the verdict
// the earlier index recorded (carryForward): a red stays red, naming the run
// that found it, until a run reaches that stage again. The read, the merge and
// the write run under one advisory lock, because other sessions publish into
// the same tmp/; each file is still replaced atomically, so a reader that
// takes no lock never sees a torn index.
func writeFailureIndex(root string, index artifactIndex) error {
	unlock, err := lockFailureIndex(root)
	if err != nil {
		return err
	}
	defer unlock()

	prior, note := readEarlierIndex(root)
	index.CarryNote = note
	index.Stages = carryForward(index.Stages, prior)
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal verify failure index: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWrite(root, FailuresLogPath, []byte(failureIndexText(index))); err != nil {
		return fmt.Errorf("write verify artifact %s: %w", FailuresLogPath, err)
	}
	if err := atomicWrite(root, FailuresJSONPath, data); err != nil {
		return fmt.Errorf("write verify artifact %s: %w", FailuresJSONPath, err)
	}
	if index.Mode == Mode {
		if err := atomicWrite(root, FullJSONPath, data); err != nil {
			return fmt.Errorf("write verify artifact %s: %w", FullJSONPath, err)
		}
	}
	return nil
}

// lockFailureIndex takes the checkout's failure-index lock and answers its
// release, which the caller MUST call.
func lockFailureIndex(root string) (func(), error) {
	path := filepath.Join(root, filepath.FromSlash(failuresLockPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("lock verify failure index: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // the path is a verification artifact under the checkout root
	if err != nil {
		return nil, fmt.Errorf("lock verify failure index: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock verify failure index: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

// readEarlierIndex answers the failure index already at root and a note for
// the reader saying what it was. A missing, unreadable or malformed index is
// treated as empty, so nothing is carried from it, and the note says so: a
// torn file another session left MUST NOT fail this run, and MUST NOT pass in
// silence either.
func readEarlierIndex(root string) (artifactIndex, string) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FailuresJSONPath))) //nolint:gosec // the path is a verification artifact under the checkout root
	if errors.Is(err, os.ErrNotExist) {
		return artifactIndex{}, "no earlier failure index at " + FailuresJSONPath + ", so nothing was carried forward"
	}
	if err != nil {
		return artifactIndex{}, "the earlier failure index at " + FailuresJSONPath +
			" could not be read (" + err.Error() + "), so it was treated as empty and nothing was carried forward"
	}
	var prior artifactIndex
	if err := json.Unmarshal(content, &prior); err != nil {
		return artifactIndex{}, "the earlier failure index at " + FailuresJSONPath +
			" is malformed (" + err.Error() + "), so it was treated as empty and nothing was carried forward"
	}
	return prior, "stages this run did not hold carry the earlier index's verdict (commit " +
		prior.Commit + ", run " + prior.RunDir + ")"
}

// carryForward answers the stages this run held, followed by one entry for
// every other stage of the uncut full population: the earlier index's verdict
// for it when that index recorded one, and an unjudged entry otherwise.
//
// The full population is the reference because it is what a commit carrying
// Go owes; the fast population is a subset of it. An earlier entry that was
// itself carried keeps its origin, so a red names the run that found it, not
// the last run that repeated it. An earlier entry with a red exit and no
// reached key comes from an index written before first-red existed, where
// every stage ran, so it carries too.
func carryForward(stages []artifactStage, prior artifactIndex) []artifactStage {
	result := make([]artifactStage, 0, len(stages))
	held := make(map[string]bool, len(stages))
	for index := range stages {
		if stages[index].OutsideRun {
			continue
		}
		held[stages[index].Stage] = true
		result = append(result, stages[index])
	}
	earlier := make(map[string]artifactStage, len(prior.Stages))
	for index := range prior.Stages {
		earlier[prior.Stages[index].Stage] = prior.Stages[index]
	}
	origin := carriedOrigin{Commit: prior.Commit, RunDir: prior.RunDir, GeneratedAt: prior.GeneratedAt}
	for _, stage := range StagesForMode(Mode) {
		name := stage.Identity.Name
		if held[name] {
			continue
		}
		entry := artifactStage{Stage: name, OutsideRun: true}
		if was, found := earlier[name]; found {
			if judged(was) {
				entry = was
				entry.Reached = true
				entry.OutsideRun = true
				if entry.CarriedFrom == nil {
					entry.CarriedFrom = &origin
				}
			}
		}
		result = append(result, entry)
	}
	return result
}

// judged reports whether an earlier index entry holds a verdict to carry.
func judged(stage artifactStage) bool {
	if stage.Reached {
		return true
	}
	return stage.ExitCode != 0
}

// PublishFailureIndex publishes the failure index a run wrote under source into
// target, merged with target's earlier index (writeFailureIndex), so the
// checkout that asked for a worktree run reads that run's verdicts, and the
// earlier verdicts for the stages the run did not hold.
// The commit gate charges structural reds from the checkout's index
// (structuralGateReds in internal/le/commit), and before this copy existed the
// worktree's index was deleted with the worktree: the 2026-10-18 run went red
// in lint and the gate charged nothing.
//
// logs names, relative to target, where the run's tmp/verify tree now lives
// (the saved-logs copy of a red worktree run). Every log path in the index is
// moved from tmp/verify to it, so a reader opens a file that exists. An empty
// logs keeps the paths as written: a green worktree run saves no logs, and
// its index points at nothing red. A source with no index answers an error
// wrapping os.ErrNotExist, which the caller reports and does not charge.
func PublishFailureIndex(source, target, logs string) error {
	content, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(FailuresJSONPath))) //nolint:gosec // the path is a verification artifact under the source root
	if err != nil {
		return fmt.Errorf("read %s: %w", FailuresJSONPath, err)
	}
	var index artifactIndex
	if err := json.Unmarshal(content, &index); err != nil {
		return fmt.Errorf("parse %s: %w", FailuresJSONPath, err)
	}
	if logs != "" {
		move := func(path string) string { return movedLogPath(path, logs) }
		index.RunDir = move(index.RunDir)
		index.CombinedLog = move(index.CombinedLog)
		for stageIndex := range index.Stages {
			stage := &index.Stages[stageIndex]
			stage.DetailLog = move(stage.DetailLog)
			for groupIndex := range stage.Groups {
				stage.Groups[groupIndex].DetailLog = move(stage.Groups[groupIndex].DetailLog)
			}
		}
	}
	if err := writeFailureIndex(target, index); err != nil {
		return fmt.Errorf("publish %s: %w", FailuresJSONPath, err)
	}
	return nil
}

// movedLogPath answers path with its tmp/verify prefix replaced by logs, and
// path unchanged when it does not lie under tmp/verify (a stage that declared
// a detail log elsewhere).
func movedLogPath(path, logs string) string {
	const runLogs = "tmp/verify"
	rest, found := strings.CutPrefix(path, runLogs)
	if !found {
		return path
	}
	if rest == "" {
		return logs
	}
	if rest[0] != '/' {
		return path
	}
	return logs + rest
}

// DeclaredGroups reads back the failure groups one stage log declares, and
// reports whether the declaration is COMPLETE: the log carries exactly one
// count marker and that count matches the groups read.
//
// An incomplete answer MUST NOT be read as "the stage named no file". A log the
// reader met mid-write, or one whose producer died between the groups and the
// count, returns the groups it could read and false. The caller decides what an
// unattributed red means for it, and neither caller may treat it as absence
// (ai/rules/principles.md).
func DeclaredGroups(root string, stage StageReport) ([]Group, bool) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(stage.Log))) //nolint:gosec // the path is a verification artifact under the checkout root
	if err != nil {
		return nil, false
	}
	var groups []Group
	var counts []string
	for line := range strings.SplitSeq(string(content), "\n") {
		if _, payload, found := strings.Cut(line, "VERIFY FAILURE GROUP:"); found {
			var group Group
			if err := json.Unmarshal([]byte(strings.TrimSpace(payload)), &group); err != nil {
				groups = append(groups, Group{
					GroupID: "unparsed-group:" + strconv.Itoa(len(groups)), Kind: "unparsed",
					Related:   []string{"unparsed-group"},
					Summary:   "a declared failure group line did not parse: " + err.Error(),
					DetailLog: stage.Log, Parallel: "independent; rerun this group alone",
					Excerpt: []string{line},
				})
				continue
			}
			if group.DetailLog == "" {
				group.DetailLog = stage.Log
			}
			if group.Parallel == "" {
				group.Parallel = "independent; rerun this group alone"
			}
			groups = append(groups, group)
			continue
		}
		if _, count, found := strings.Cut(line, "VERIFY FAILURE GROUPS COMPLETE:"); found {
			counts = append(counts, strings.TrimSpace(count))
		}
	}
	if len(counts) != 1 {
		return groups, false
	}
	count, err := strconv.Atoi(counts[0])
	return groups, err == nil && count == len(groups)
}

func failureIndexText(index artifactIndex) string {
	var text textbuf.Buffer
	text.Reset().Str("# Ze verify failure index\n")
	text.Str("Generated: ").Str(index.GeneratedAt).Byte('\n')
	text.Str("Mode: ").Str(index.Mode).Byte('\n')
	text.Str("Run directory: ").Str(index.RunDir).Byte('\n')
	text.Str("Combined log: ").Str(index.CombinedLog).Str("\n\n")
	if index.StoppedAt != "" {
		text.Str("Stopped at first red: ").Str(index.StoppedAt).Byte('\n')
	}
	var stopped, carried, unjudged []string
	for position := range index.Stages {
		stage := &index.Stages[position]
		switch {
		case !stage.OutsideRun && !stage.Reached:
			stopped = append(stopped, stage.Stage)
		case stage.OutsideRun && stage.Reached:
			carried = append(carried, stage.Stage+" ("+carriedVerdict(stage)+")")
		case stage.OutsideRun:
			unjudged = append(unjudged, stage.Stage)
		}
	}
	listed := false
	for _, list := range []struct {
		heading string
		stages  []string
	}{
		{"Not reached, the run stopped at its first red: ", stopped},
		{"Not run by this run, carried from an earlier run: ", carried},
		{"Not run by this run and never judged: ", unjudged},
	} {
		if len(list.stages) != 0 {
			text.Str(list.heading).Join(list.stages, ", ").Byte('\n')
			listed = true
		}
	}
	if index.CarryNote != "" {
		text.Str("Earlier index: ").Str(index.CarryNote).Byte('\n')
		listed = true
	}
	if listed {
		text.Byte('\n')
	}
	failed := 0
	for _, stage := range index.Stages {
		if stage.ExitCode == 0 {
			continue
		}
		failed++
		text.Str("## Stage: ").Str(stage.Stage)
		if stage.CarriedFrom != nil {
			text.Str(" (carried: ").Str(carriedVerdict(&stage)).Byte(')')
		}
		text.Byte('\n')
		text.Str("Exit: ").Int(int64(stage.ExitCode)).Byte('\n')
		text.Str("Detail log: ").Str(stage.DetailLog).Str("\n\n")
		for index := range stage.Groups {
			group := &stage.Groups[index]
			text.Str("### Group: ").Str(group.GroupID).Byte('\n')
			text.Str("Stage: ").Str(group.Stage).Byte('\n')
			text.Str("Kind: ").Str(group.Kind).Byte('\n')
			text.Str("Related: ").Join(group.Related, ", ").Byte('\n')
			text.Str("Summary: ").Str(group.Summary).Byte('\n')
			text.Str("Rerun: ").Str(group.Rerun).Byte('\n')
			text.Str("Detail log: ").Str(group.DetailLog).Byte('\n')
			text.Str("Parallel: ").Str(group.Parallel).Str("\n\n")
		}
	}
	if failed == 0 {
		text.Str("No failures.\n")
	}
	return text.String()
}

// carriedVerdict answers the earlier verdict a carried stage repeats and the
// run that judged it, as "red at <commit> in <run directory>".
func carriedVerdict(stage *artifactStage) string {
	verdict := "green"
	if stage.ExitCode != 0 {
		verdict = "red"
	}
	if stage.CarriedFrom == nil {
		return verdict
	}
	return verdict + " at " + stage.CarriedFrom.Commit + " in " + stage.CarriedFrom.RunDir
}
