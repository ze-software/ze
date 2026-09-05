// Design: docs/contributing/running-commands.md -- asking a run in flight whether it reddened one file
// Related: engine/artifacts.go -- DeclaredGroups, the one reader of a stage's declaration
// Related: ../commit/verification.go -- structuralGateReds, which asks the same question of the FINISHED index
package verify

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
	"github.com/ze-software/ze/internal/le/verify/failuregroup"
)

// The verdicts this query answers. The set is closed, and it holds no word for
// "the tree is clean": a run that has not finished has judged nothing about the
// stages it has not reached, so no answer here may be read as a pass.
const (
	// VerdictNoRun says no run directory exists, so nothing has been judged.
	VerdictNoRun = "no-run"
	// VerdictNamed says a stage that has reported declared a red naming the path.
	VerdictNamed = "named"
	// VerdictUndetermined says nothing that has reported names the path, and
	// something is still unknown: stages that have not reported, a red that
	// named no file, or a run directory whose mode has no stage population.
	VerdictUndetermined = "undetermined"
	// VerdictNotNamed says every stage reported, every red among them named the
	// files it is about, and none of those files is the path.
	VerdictNotNamed = "not-named"
)

// resultMarker opens the line every stage log ends with. The engine writes a
// stage log once, after the stage returns (`runMode`, engine/run.go), and this
// line is the last thing in it. Its presence is therefore what proves a log is
// whole: a log met mid-write carries no marker and counts as not reported.
const resultMarker = "### Stage result: "

// Red is one stage that reported a non-zero status, named for a reader who has
// to decide whether to act on it.
type Red struct {
	Stage    string `json:"stage"`
	ExitCode int    `json:"exit-code"`
	Log      string `json:"log"`
	Summary  string `json:"summary,omitempty"`
}

// Reds answers whether a verification run has so far reddened one path.
//
// The run does not have to be finished. Every stage writes its own log when it
// ends, carrying the failure groups that say which files its red is about, so
// the answer for a path exists on disk long before the run's index does.
//
// Stages, Reported and Pending are part of the answer rather than decoration.
// A caller MUST read them: a verdict of not-named is worth what the count of
// stages behind it is worth, and this query is not a way to certify a tree.
type Reds struct {
	// Path is the file the question was asked about.
	Path string `json:"path"`
	// Run is the run directory read, relative to the checkout root.
	Run string `json:"run,omitempty"`
	// Mode is the stage population that run selected.
	Mode string `json:"mode,omitempty"`
	// Stages is how many stages the mode runs, Reported how many have written a
	// whole log, and Pending the remainder, which has judged nothing yet.
	Stages   int `json:"stages"`
	Reported int `json:"reported"`
	Pending  int `json:"pending"`
	// Finished says the run's stage loop is over, so Pending will not shrink
	// and waiting for the rest of this run will not answer anything more.
	Finished bool `json:"finished"`
	// Verdict is one of the four constants above.
	Verdict string `json:"verdict"`
	// Naming holds the reds whose declared files cover Path.
	Naming []Red `json:"naming,omitempty"`
	// Unattributed holds the reds that named no file at all. Any one of them
	// can be about Path, which is why their presence forbids not-named.
	Unattributed []Red `json:"unattributed,omitempty"`
}

// redsHere is the `le verify reds file <path>` action.
func redsHere(args leaction.Arguments) (any, int) {
	path, err := askedPath(args["file"])
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	answer := readReds(root, path)
	if answer.Verdict == VerdictNotNamed {
		return answer, 0
	}

	return answer, 1
}

// askedPath normalizes the path the caller typed. It does NOT require the file
// to exist: a caller asking about a file its own change deleted is asking a
// legitimate question, and refusing it would send the caller to the run's own
// logs, which is the cost this query exists to remove.
func askedPath(value string) (string, error) {
	path := strings.TrimSpace(filepath.ToSlash(value))
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return "", errors.New("verify reds: file needs a checkout-relative path, " +
			"as in: le verify reds file internal/le/verify/reds.go")
	}
	if strings.HasPrefix(path, "/") {
		return "", errors.New("verify reds: file takes a checkout-relative path, not " + value +
			"; drop the leading slash and name the path as git names it")
	}

	return path, nil
}

// readReds answers the query for one path against the newest run on disk.
func readReds(root, path string) Reds {
	answer := Reds{Path: path, Verdict: VerdictNoRun}
	run, found := latestRun(root)
	if !found {
		return answer
	}
	answer.Run = run
	answer.Mode = runMode(run)
	answer.Stages = len(verifyengine.StagesForMode(answer.Mode))
	answer.Finished = fileExists(filepath.Join(root, filepath.FromSlash(run), "ze-verify.log"))
	scanStages(root, run, path, &answer)
	// A run directory can hold more whole logs than the mode's population when
	// the population changed under a run in flight. Pending is a count of work
	// still to come, so it floors at zero rather than going negative.
	answer.Pending = max(answer.Stages-answer.Reported, 0)
	answer.Verdict = verdictOf(&answer)

	return answer
}

// verdictOf decides the answer from what was read. Only the last branch may
// answer that the path is clear, and it demands that every stage reported and
// that every red among them said which files it was about.
func verdictOf(answer *Reds) string {
	if len(answer.Naming) != 0 {
		return VerdictNamed
	}
	if answer.Stages == 0 || answer.Pending != 0 || len(answer.Unattributed) != 0 {
		return VerdictUndetermined
	}

	return VerdictNotNamed
}

// scanStages reads every stage log the run has written and sorts the reds into
// the ones that name the path and the ones that name no file at all.
//
// The loop is bounded by the run directory's own entries, which the engine
// writes one per stage plus the combined log.
func scanStages(root, run, path string, answer *Reds) {
	directory := filepath.Join(root, filepath.FromSlash(run))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		if entry.Name() == "ze-verify.log" {
			continue
		}
		log := filepath.ToSlash(filepath.Join(run, entry.Name()))
		content, readErr := os.ReadFile(filepath.Join(directory, entry.Name())) //nolint:gosec // the path is a verification artifact under the checkout root
		if readErr != nil {
			continue
		}
		stage, code, whole := stageResult(string(content))
		if !whole {
			continue
		}
		answer.Reported++
		if code == 0 {
			continue
		}
		red := Red{Stage: stage, ExitCode: code, Log: log}
		said, summary := attributionOf(root, path, verifyengine.StageReport{
			Identity: verifyengine.Identity{Name: stage}, Code: code, Log: log,
		})
		switch said {
		case attributionPath:
			red.Summary = summary
			answer.Naming = append(answer.Naming, red)
		case attributionNone:
			answer.Unattributed = append(answer.Unattributed, red)
		case attributionOther:
			// The stage said which files its red is about and this path is not
			// one of them, so the red belongs to another change.
		}
	}
	sortReds(answer.Naming)
	sortReds(answer.Unattributed)
}

// attribution is what one red stage's declaration says about a path. The zero
// value is the answer that keeps the red in play, so a branch that forgets to
// decide leaves the caller cautious rather than clear.
type attribution int

const (
	// attributionNone says the stage named no usable file at all, so its red
	// can be about any path, this one included.
	attributionNone attribution = iota
	// attributionOther says the stage named the files its red is about and this
	// path is not among them.
	attributionOther
	// attributionPath says the stage named this path.
	attributionPath
)

// attributionOf asks one red stage what its declaration says about the path,
// and answers the summary of the group that named it.
//
// An incomplete declaration answers attributionNone rather than
// attributionOther: a log read mid-write, or one whose producer died between
// its groups and its count, has told the reader nothing, and nothing is not
// "somebody else's" (ai/rules/principles.md).
func attributionOf(root, path string, stage verifyengine.StageReport) (attribution, string) {
	groups, complete := verifyengine.DeclaredGroups(root, stage)
	if !complete {
		return attributionNone, ""
	}
	said := attributionNone
	for index := range groups {
		group := &groups[index]
		if !failuregroup.CarriesPaths(group.Kind) {
			continue
		}
		for _, related := range group.Related {
			cleaned, usable := failuregroup.CleanPath(root, related)
			if !usable {
				continue
			}
			said = attributionOther
			if failuregroup.Covers(cleaned, []string{path}) {
				return attributionPath, group.Summary
			}
		}
	}

	return said, ""
}

// stageResult reads a stage log's closing line: the stage's own name, the
// status it exited with, and whether the line was there at all.
func stageResult(content string) (string, int, bool) {
	start := strings.LastIndex(content, resultMarker)
	if start < 0 {
		return "", 0, false
	}
	line := content[start+len(resultMarker):]
	if end := strings.IndexByte(line, '\n'); end >= 0 {
		line = line[:end]
	}
	stage, status, found := strings.Cut(line, " exit=")
	if !found {
		return "", 0, false
	}
	code, err := strconv.Atoi(strings.TrimSpace(status))
	if err != nil {
		return "", 0, false
	}

	return strings.TrimSpace(stage), code, true
}

// latestRun answers the newest run directory under tmp/verify, relative to
// root. A run in flight touches its own directory as each stage log lands, so
// the newest directory is the run a caller is most likely asking about.
func latestRun(root string) (string, bool) {
	parent := filepath.Join(root, "tmp", "verify")
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", false
	}
	newest := ""
	var newestAt int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		at := info.ModTime().UnixNano()
		if newest != "" && at <= newestAt {
			continue
		}
		newest, newestAt = entry.Name(), at
	}
	if newest == "" {
		return "", false
	}

	return filepath.ToSlash(filepath.Join("tmp", "verify", newest)), true
}

// runMode reads the mode out of a run directory's name. The engine names the
// directory `<mode>-<random>` (os.MkdirTemp in runMode, engine/run.go), and
// neither mode name holds a hyphen, so the first one closes the mode.
func runMode(run string) string {
	mode, _, found := strings.Cut(filepath.Base(run), "-")
	if !found {
		return ""
	}

	return mode
}

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// sortReds orders the reds by stage name, so two readings of one run directory
// answer in one order whatever the file system returns.
func sortReds(reds []Red) {
	sort.Slice(reds, func(first, second int) bool { return reds[first].Stage < reds[second].Stage })
}

// Text renders the answer for a person. Every branch prints the stage counts,
// because how much of the run is still unknown qualifies every verdict here.
func (r Reds) Text() string {
	var text textbuf.Buffer
	text.Str("path: ").Str(r.Path).Byte('\n')
	if r.Verdict == VerdictNoRun {
		text.Str("verdict: ").Str(r.Verdict).Byte('\n')
		text.Str("no verification run has written a stage log under tmp/verify, so nothing has judged this path\n")

		return text.String()
	}
	text.Str("run: ").Str(r.Run).Str(" (mode ").Str(r.Mode).Str(")\n")
	text.Str("verdict: ").Str(r.Verdict).Byte('\n')
	text.Str("stages: ").Int(int64(r.Reported)).Str(" of ").Int(int64(r.Stages)).
		Str(" reported, ").Int(int64(r.Pending)).Str(" not reported yet")
	if r.Finished {
		text.Str(" (the run has ended, so the rest never reported)")
	}
	text.Byte('\n')
	appendReds(&text, "reds naming this path:", r.Naming)
	appendReds(&text, "reds naming no file, so any of them can be about this path:", r.Unattributed)

	return text.String()
}

func appendReds(text *textbuf.Buffer, heading string, reds []Red) {
	if len(reds) == 0 {
		return
	}
	text.Str(heading).Byte('\n')
	for _, red := range reds {
		text.Str("  ").Str(red.Stage).Str(" exit=").Int(int64(red.ExitCode)).
			Str("  ").Str(red.Log)
		if red.Summary != "" {
			text.Str("  ").Str(red.Summary)
		}
		text.Byte('\n')
	}
}
