// Design: docs/architecture/testing/verify-freshness-scope.md -- what the certificate proves at commit preparation
package commit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
	"github.com/ze-software/ze/internal/le/verify/failuregroup"
)

// VerificationState records what the latest native verify status proves for
// the prospective commit's explicit paths.
type VerificationState struct {
	State     string `json:"state"`
	Detail    string `json:"detail"`
	Mode      string `json:"mode,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Commit    string `json:"commit,omitempty"`
}

// The two VerificationState values the commit flow branches on: the
// certificate covers the commit's paths, or no native gate judges this tree.
const (
	verifyFresh         = "fresh"
	verifyNotApplicable = "not-applicable"
)

const compilesStage = "repo compiles/check"

// structuralStages answers which recorded reds mean the tree is broken.
//
// The set is READ from the stage population rather than restated here. It used
// to be a second hand-written list of the same eight names, and
// ai/rules/precommit-verify.md claimed a test held the two in agreement. No
// such test existed. A stage renamed in the population silently left this set,
// its red filed as unattributable, and the commit went through.
func structuralStages() map[string]bool { return verifyengine.Structural(verifyengine.Mode) }

type structuralReds struct {
	Charged      []string
	Unattributed []string
	Foreign      []string
	// Unreached names the structural stages a first-red run held and stopped
	// before (AC-11). Each is also in Charged: the run set out to judge them
	// and did not finish. A stage the run never held is not here: the index
	// carries an earlier verdict for it, or marks it unjudged, and an unjudged
	// stage is not red (the derived verification debt already owes the full
	// verify that judges it).
	Unreached []string
	// Causes says, for each charged stage in Charged order, why it is charged,
	// the command that diagnoses it, and the verify runs that clear it.
	Causes []string
}

type failureIndex struct {
	StoppedAt string            `json:"stopped-at"`
	Stages    []json.RawMessage `json:"stages"`
}

// failureOrigin names the run whose verdict a carried stage repeats.
type failureOrigin struct {
	Commit string `json:"commit"`
	RunDir string `json:"run-dir"`
}

type failureStage struct {
	Stage string `json:"stage"`
	// Reached is absent from an index written before first-red existed, and
	// every stage of such an index ran, so absent reads as reached.
	Reached  *bool           `json:"reached"`
	ExitCode int             `json:"exit-code"`
	Groups   json.RawMessage `json:"groups"`
	// OutsideRun marks a stage the recording run did not hold; CarriedFrom
	// names the earlier run whose verdict it repeats, and is nil for a stage
	// no run judged.
	OutsideRun  bool           `json:"outside-run"`
	CarriedFrom *failureOrigin `json:"carried-from"`
}

type failureGroup struct {
	ID      any `json:"group-id"`
	Kind    any `json:"kind"`
	Related any `json:"related"`
	Rerun   any `json:"rerun"`
}

// rerunAllStages is the full verify run: it reaches every stage, so its index
// clears any charge, and it is the diagnosing command for a stage no recorded
// group names a narrower rerun for.
const rerunAllStages = "le verify worktree"

func structuralGateReds(root string, paths []string) structuralReds {
	content, err := os.ReadFile(filepath.Join(root, "tmp", "ze-verify-failures.json")) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		return structuralReds{}
	}
	var index failureIndex
	if json.Unmarshal(content, &index) != nil {
		return structuralReds{}
	}
	scope := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != "" {
			scope = append(scope, path)
		}
	}
	structural := structuralStages()
	var result structuralReds
	for _, rawStage := range index.Stages {
		var stage failureStage
		if json.Unmarshal(rawStage, &stage) != nil {
			continue
		}
		if !structural[stage.Stage] {
			continue
		}
		if stage.Reached != nil && !*stage.Reached {
			if stage.OutsideRun {
				continue
			}
			result.Charged = append(result.Charged, stage.Stage)
			result.Unreached = append(result.Unreached, stage.Stage)
			result.Causes = append(result.Causes, stage.Stage+
				": not reached, because the run stopped at its first red ("+index.StoppedAt+
				"); "+clearingRuns(stage.Stage))
			continue
		}
		if stage.ExitCode == 0 {
			continue
		}
		var groups []failureGroup
		groupsReadable := json.Unmarshal(stage.Groups, &groups) == nil
		blind := make([]string, 0)
		owned := false
		if !groupsReadable || len(groups) == 0 {
			blind = append(blind, stage.Stage)
		}
		if groupsReadable {
			for _, group := range groups {
				named := groupRelatedPaths(root, group)
				if len(named) == 0 {
					id, _ := group.ID.(string)
					if id == "" {
						id = stage.Stage
					}
					blind = append(blind, id)
					continue
				}
				for _, related := range named {
					if failuregroup.Covers(related, scope) {
						owned = true
						break
					}
				}
			}
		}
		if len(blind) == 0 && !owned {
			result.Foreign = append(result.Foreign, stage.Stage)
			continue
		}
		result.Charged = append(result.Charged, stage.Stage)
		result.Causes = append(result.Causes, redCause(stage, groups))
		if !owned {
			result.Unattributed = append(result.Unattributed,
				stage.Stage+" ("+strings.Join(blind, ", ")+")")
		}
	}
	return result
}

// redCause answers why a red structural stage is charged, the command that
// diagnoses it, and the runs that clear the charge.
//
// The two commands differ, and the refusal names both. A group's rerun (the
// first one its groups name) runs the stage directly and publishes no failure
// index, so a stage fixed and rerun green that way is still charged: only a
// verify run that reaches the stage rewrites its entry (clearingRuns).
func redCause(stage failureStage, groups []failureGroup) string {
	diagnose := rerunAllStages
	for _, group := range groups {
		if named, ok := group.Rerun.(string); ok && named != "" {
			diagnose = named
			break
		}
	}
	tail := "; diagnose with: " + diagnose + "; " + clearingRuns(stage.Stage)
	if stage.CarriedFrom == nil {
		return stage.Stage + ": red in the latest verify run" + tail
	}
	origin := "at " + stage.CarriedFrom.Commit + " in " + stage.CarriedFrom.RunDir
	if stage.CarriedFrom.Commit == "" {
		// An index an older le or another tool wrote can name no commit, and
		// "red at  in " read as a defect in the refusal, not a gap in the record.
		origin = "in an earlier run (commit not recorded)"
	}
	return stage.Stage + ": red " + origin + ", carried because the latest run did not run it" + tail
}

// clearingRuns names the verify runs whose index rewrites the named stage's
// entry: the full run, the piece of a cut run that holds it, and the fast run
// when the fast population holds the stage. The fast membership is read from
// the population, so a stage moved in or out of it needs no edit here. No
// default piece count exists (the caller picks `part <i> of <n>`), so the
// piece is named by what it holds rather than by its number.
func clearingRuns(name string) string {
	runs := "clears when a verify run reaches it: " + rerunAllStages +
		", or the le verify worktree part <i> of <n> piece that holds it"
	for _, fast := range verifyengine.StagesForMode(verifyengine.FastMode) {
		if fast.Identity.Name == name {
			return runs + ", or le verify current fast"
		}
	}
	return runs
}

// groupRelatedPaths answers the checkout paths one recorded group named. Which
// kinds carry paths, and what makes a value a usable path, are decided by
// failuregroup: the in-flight query (internal/le/verify/reds.go) asks the same
// question of the same data, and a second copy of the answer here would let the
// two surfaces disagree about whose red it is.
func groupRelatedPaths(root string, group failureGroup) []string {
	kind, _ := group.Kind.(string)
	if !failuregroup.CarriesPaths(kind) {
		return nil
	}
	values, ok := group.Related.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0)
	for _, value := range values {
		related, ok := value.(string)
		if !ok {
			continue
		}
		cleaned, usable := failuregroup.CleanPath(root, related)
		if !usable {
			continue
		}
		result = append(result, cleaned)
	}
	return result
}

func verificationState(root string, paths []string) VerificationState {
	freshness := verifyengine.CheckCertificate(root, paths)
	state := VerificationState{
		State:     "stale",
		Detail:    freshness.Reason,
		Mode:      freshness.Mode,
		Timestamp: freshness.Timestamp,
		Commit:    freshness.GitSHA,
	}
	if freshness.Fresh {
		state.State = verifyFresh
	}
	return state
}
