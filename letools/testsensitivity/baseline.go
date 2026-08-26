// Design: docs/architecture/testing/test-health.md -- the committed floors
//
// baseline.go reads the floors and applies the ratchet. The floors may only go
// DOWN: lower them in the same change that improves the number.

package testsensitivity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Baseline is the committed floor for each detector.
type Baseline struct {
	AssertNothing int `json:"assert-nothing"`
	TagOrphan     int `json:"tag-orphan"`
}

// ReadBaseline reads the floors from tree.
func ReadBaseline(tree string) (Baseline, error) {
	path := filepath.Join(tree, filepath.FromSlash(BaselinePath))
	var baseline Baseline
	raw, err := os.ReadFile(path) //nolint:gosec // fixed in-repo path
	if err != nil {
		return baseline, fmt.Errorf("read baseline %s: %w (run `make ze-test-health-update` to create it)", BaselinePath, err)
	}
	if err := json.Unmarshal(raw, &baseline); err != nil {
		return baseline, fmt.Errorf("parse baseline %s: %w", BaselinePath, err)
	}
	if baseline.AssertNothing < 0 || baseline.TagOrphan < 0 {
		return baseline, fmt.Errorf("baseline %s has a negative floor: %+v", BaselinePath, baseline)
	}
	return baseline, nil
}

// Judge applies the ratchet: a count at or below its floor passes, and a count
// above it fails.
//
// The verdict is set on the RESULT rather than answered alone, because the
// payload a caller pipes must carry it: the earlier version kept `Valid` a
// field the scan always set to true, so the JSON half of the check reported
// findings and exited 0 -- a guard whose only enforcement path could never
// deny.
func Judge(result Result, baseline Baseline) Verdict {
	result.Valid = len(result.AssertNothing) <= baseline.AssertNothing &&
		len(result.TagOrphan) <= baseline.TagOrphan
	return Verdict{Result: result, Baseline: baseline}
}

// marshalResult encodes a scan without re-entering Verdict's own marshaller.
func marshalResult(result Result) ([]byte, error) {
	type plain Result
	return json.Marshal(plain(result))
}
