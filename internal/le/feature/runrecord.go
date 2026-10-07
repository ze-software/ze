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
// a date: each run carries the git blob id the test file had when it ran, and a
// run is current exactly when that id equals the file's blob id now. A date
// comparison would need git history the shallow CI checkout does not hold, and
// would call a reverted edit stale.
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
	// runStateStale: a green run is recorded, and the test changed since.
	runStateStale
)

// loadRunRecord reads features/runs/<id>.json. A missing file is an empty
// record, which is the truthful answer for a feature nothing has run.
func loadRunRecord(tree, id string) (RunRecord, error) {
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
	}
	for _, run := range record.Interop {
		if run.Result != runResultPass {
			return RunRecord{}, errors.New(rel + ": run of " + run.Scenario + " has result '" + run.Result + "'; only a pass is recorded")
		}
	}
	return record, nil
}

func runRecordRel(id string) string {
	var tb textbuf.Buffer
	return tb.Str(runRecordDir).Byte('/').Str(id).Str(".json").String()
}

// stateOf answers whether item has a current green run in record. blob is the
// item's file's blob id now.
func (r RunRecord) stateOf(item, blob string) runState {
	state := runStateNotRun
	for _, run := range r.Runs {
		if run.Test != item {
			continue
		}
		if run.TestBlob == blob {
			return runStateCurrent
		}
		state = runStateStale
	}
	return state
}

// scenarioStateOf answers whether the Interop entry item has a current green
// run in record. tree is the scenario directory's git tree id now.
func (r RunRecord) scenarioStateOf(item, tree string) runState {
	state := runStateNotRun
	for _, run := range r.Interop {
		if run.Scenario != item {
			continue
		}
		if run.ScenarioTree == tree {
			return runStateCurrent
		}
		state = runStateStale
	}
	return state
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
