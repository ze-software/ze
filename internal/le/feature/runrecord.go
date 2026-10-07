// Design: docs/contributing/feature-maturity.md -- the recorded green run Supported requires
// Related: check.go -- the S1 criterion that reads these records
//
// Owner decision D-6 (2026-10-07): Supported requires real-path tests that
// exist AND have a recorded green run newer than the last change to the test.
// "Exists, not run" never reaches Supported.
//
// The record is committed beside the declarations, features/runs/<id>.json, for
// the reason rfc/discrimination/<stem>.json is: the published page is derived
// on any checkout, CI included, and a record under tmp/ exists on one machine
// only. "Newer than the last change to the test" is decided by CONTENT, not by
// the git history of the file: each run carries the git blob id the test file
// had when it ran, and a run matches its test exactly when that id equals the
// file's blob id now. A history comparison would need git history the shallow
// CI checkout does not hold, and would call a reverted edit stale.
//
// Owner decision 2026-10-07, run staleness option (c): a run matching its
// content still counts for runAgeDaysMax days only. The blob id covers the test
// file and nothing it reads, so a run outlived the fixture change of
// f02d58da88; keying on every package the test imports would stale most runs on
// every commit. The age bound re-proves on a schedule what the id cannot see.
//
// The owner's reading of D-6 covers interop scenarios too: an Interop entry
// counted toward a level needs a recorded green run of the scenario as it is
// now. A scenario is a directory, so its identity is the git TREE id of that
// directory, computed over the working tree the way blobID computes a file's.

package feature

import (
	"crypto/sha1" //nolint:gosec // the git blob id is SHA-1 by definition; it identifies content, it guards nothing
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// runRecordDir holds one run record per feature, relative to the checkout.
const runRecordDir = declarationDir + "/runs"

// runResultPass is the only result a record stores: a red run is never
// recorded, because it proves nothing a missing record does not.
const runResultPass = "pass"

// RunRecord is the green runs recorded for one feature's real-path tests.
type RunRecord struct {
	Feature string    `json:"feature"`
	Runs    []TestRun `json:"runs"`
	// Interop is absent from a record written before interop runs were
	// recorded, which reads as no scenario run: the truthful answer.
	Interop []ScenarioRun `json:"interop,omitempty"`
}

// TestRun is one recorded green run of one real-path test item.
type TestRun struct {
	// Test is the item exactly as the declaration lists it.
	Test string `json:"test"`
	// TestBlob is the git blob id of the test's file when it ran.
	TestBlob string `json:"test-blob"`
	// Commit is HEAD when it ran, for a reader; freshness never reads it.
	Commit string `json:"commit"`
	Date   string `json:"date"`
	Result string `json:"result"`
}

// ScenarioRun is one recorded green run of one interop scenario.
type ScenarioRun struct {
	// Scenario is the Interop entry exactly as the declaration lists it,
	// `<suite>/<scenario>`.
	Scenario string `json:"scenario"`
	// ScenarioTree is the git tree id of the scenario directory when it ran.
	ScenarioTree string `json:"scenario-tree"`
	// Commit is HEAD when it ran, for a reader; freshness never reads it.
	Commit string `json:"commit"`
	Date   string `json:"date"`
	Result string `json:"result"`
}

// runState is the answer for one real-path test item.
type runState uint8

const (
	runStateUnspecified runState = iota
	// runStateCurrent: a green run is recorded for the present content.
	runStateCurrent
	// runStateNotRun: the test exists and no green run is recorded for it.
	runStateNotRun
	// runStateChanged: a green run is recorded, and the test or scenario
	// changed since.
	runStateChanged
	// runStateAged: a green run is recorded for the present content, and it is
	// older than runAgeDaysMax days.
	runStateAged
)

// runAgeDaysMax is how many days a recorded green run counts for: a run is
// current on the day it was recorded and for the runAgeDaysMax days after, so
// day 30 counts and day 31 does not.
const runAgeDaysMax = 30

// runAnswer is one item's run state, with the date of the run it was read from
// ("" for runStateNotRun).
type runAnswer struct {
	state runState
	date  string
}

// loadRunRecord reads features/runs/<id>.json. A missing file is an empty
// record, which is the truthful answer for a feature nothing has run. A run
// whose date does not parse, or falls after today, refuses the record: it
// would never age out.
func loadRunRecord(tree, id string, today time.Time) (RunRecord, error) {
	rel := runRecordRel(id)
	raw, err := os.ReadFile(filepath.Join(tree, rel)) //nolint:gosec // rel is built from a declaration id, itself a directory entry
	if errors.Is(err, os.ErrNotExist) {
		return RunRecord{Feature: id}, nil
	}
	if err != nil {
		return RunRecord{}, err
	}
	var record RunRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return RunRecord{}, errors.New(rel + ": malformed run record: " + err.Error())
	}
	if record.Feature != id {
		return RunRecord{}, errors.New(rel + ": records feature '" + record.Feature + "', not '" + id + "'")
	}
	for _, run := range record.Runs {
		if run.Result != runResultPass {
			return RunRecord{}, errors.New(rel + ": run of " + run.Test + " has result '" + run.Result + "'; only a pass is recorded")
		}
		if problem := runDateProblem(run.Date, today); problem != "" {
			return RunRecord{}, errors.New(rel + ": run of " + run.Test + " " + problem)
		}
	}
	for _, run := range record.Interop {
		if run.Result != runResultPass {
			return RunRecord{}, errors.New(rel + ": run of " + run.Scenario + " has result '" + run.Result + "'; only a pass is recorded")
		}
		if problem := runDateProblem(run.Date, today); problem != "" {
			return RunRecord{}, errors.New(rel + ": run of " + run.Scenario + " " + problem)
		}
	}
	return record, nil
}

// runDateProblem answers why date cannot date a run judged on today, or "".
func runDateProblem(date string, today time.Time) string {
	recorded, err := time.Parse(attestationLayout, date)
	if err != nil {
		return "has date '" + date + "', not YYYY-MM-DD"
	}
	if recorded.After(today) {
		return "is dated " + date + ", after today " + today.Format(attestationLayout) +
			": a run is never recorded in the future"
	}
	return ""
}

// runAgeDays answers how many whole days before today date is. date passed
// runDateProblem in loadRunRecord, the only source of a judged run.
func runAgeDays(date string, today time.Time) int {
	recorded, err := time.Parse(attestationLayout, date)
	if err != nil {
		panic("BUG: a run date reached the age check unvalidated: " + err.Error())
	}
	return int(today.Sub(recorded).Hours()) / 24
}

// calendarDay answers the UTC calendar day of now at midnight, the day a run's
// Date names and the day the check judges ages on.
func calendarDay(now time.Time) time.Time {
	year, month, day := now.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func runRecordRel(id string) string {
	var tb textbuf.Buffer
	return tb.Str(runRecordDir).Byte('/').Str(id).Str(".json").String()
}

// stateOf answers whether item has a current green run in record on today.
// blob is the item's file's blob id now.
func (r RunRecord) stateOf(item, blob string, today time.Time) runAnswer {
	answer := runAnswer{state: runStateNotRun}
	for _, run := range r.Runs {
		if run.Test != item {
			continue
		}
		answer = preferredRun(answer, judgeRun(run.TestBlob == blob, run.Date, today))
	}
	return answer
}

// scenarioStateOf answers whether the Interop entry item has a current green
// run in record on today. tree is the scenario directory's git tree id now.
func (r RunRecord) scenarioStateOf(item, tree string, today time.Time) runAnswer {
	answer := runAnswer{state: runStateNotRun}
	for _, run := range r.Interop {
		if run.Scenario != item {
			continue
		}
		answer = preferredRun(answer, judgeRun(run.ScenarioTree == tree, run.Date, today))
	}
	return answer
}

// judgeRun answers the state of one recorded run of an item: changed when its
// content id no longer matches, aged when it matches and is older than
// runAgeDaysMax days on today, current otherwise.
func judgeRun(sameContent bool, date string, today time.Time) runAnswer {
	if !sameContent {
		return runAnswer{state: runStateChanged, date: date}
	}
	if runAgeDays(date, today) > runAgeDaysMax {
		return runAnswer{state: runStateAged, date: date}
	}
	return runAnswer{state: runStateCurrent, date: date}
}

// preferredRun answers which of two runs of one item speaks for it: a current
// run over any other, then a run of the present content that aged, then a run
// of changed content, then no run.
func preferredRun(held, next runAnswer) runAnswer {
	if runRank(next.state) > runRank(held.state) {
		return next
	}
	return held
}

func runRank(state runState) int {
	switch state {
	case runStateNotRun:
		return 1
	case runStateChanged:
		return 2
	case runStateAged:
		return 3
	case runStateCurrent:
		return 4
	case runStateUnspecified:
		panic("BUG: a run answer holds no run state")
	}
	panic("BUG: a run answer holds an unknown run state")
}

// problem answers why item has no current green run, or "" when it has one.
// content names what a changed run no longer matches ("test", "scenario");
// rel is the record the run would be in.
func (a runAnswer) problem(item, content, rel string) string {
	switch a.state {
	case runStateCurrent:
		return ""
	case runStateNotRun:
		return item + " exists, not run (no green run recorded in " + rel + ")"
	case runStateChanged:
		return item + " stale: " + content + " changed since its recorded green run"
	case runStateAged:
		return item + " stale: older than " + strconv.Itoa(runAgeDaysMax) + " days (recorded " + a.date + ")"
	case runStateUnspecified:
		panic("BUG: a run answer holds no run state")
	}
	panic("BUG: a run answer holds an unknown run state")
}

// blobID answers the git blob id of the file at rel: SHA-1 over
// "blob <size>\x00<content>", the id `git hash-object` prints.
func blobID(tree, rel string) (string, error) {
	content, err := os.ReadFile(filepath.Join(tree, rel)) //nolint:gosec // rel passed repoPath, which refuses absolute and escaping paths
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(gitObjectID("blob", content)), nil
}

// gitObjectID answers the raw git object id of content stored as kind: SHA-1
// over "<kind> <size>\x00<content>".
func gitObjectID(kind string, content []byte) []byte {
	hash := sha1.New() //nolint:gosec // the git object id is SHA-1 by definition
	hash.Write([]byte(kind + " " + strconv.Itoa(len(content)) + "\x00"))
	hash.Write(content)
	return hash.Sum(nil)
}
