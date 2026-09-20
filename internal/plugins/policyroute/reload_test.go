// Design: docs/architecture/plugin/plugin-system.md -- verify-to-apply seam
// Related: register.go (pendingPolicies, applyStagedPolicies)

package policyroute

import (
	"errors"
	"sync"
	"testing"
)

// recordingInstaller stands in for applyPolicies: it records what the apply
// asked for, so the decision can be proven without a firewall backend and
// without root.
type recordingInstaller struct {
	installs [][]PolicyRoute
	undos    int
	err      error
}

func (r *recordingInstaller) install(policies []PolicyRoute) error {
	r.installs = append(r.installs, policies)
	return r.err
}

func (r *recordingInstaller) undo() error {
	r.undos++
	return nil
}

// TestApplyStagedPoliciesRemovesWhenEveryPolicyIsGone proves that deleting the
// `policy` block takes this plugin's nftables tables and ip rules out of the
// kernel.
//
// VALIDATES: pendingPolicies carries "a verify ran" beside "what it found", so
// applyStagedPolicies can tell an empty result from an empty stage and installs
// the teardown.
// PREVENTS: the nil-conflation recorded in
// plan/journal/zero-value-as-valid-answer.md (2026-08-18) and
// plan/journal/component-rebuilt-during-reload.md. parsePolicyConfig returns
// (nil, nil) both for the empty body a reload delivers for a removed root and
// for a `policy` block with no `route` child, and the apply guarded on `if
// newPolicies == nil { return nil }`. The mark rules and the `ip rule` table
// selections stayed installed, steering traffic by a policy the operator had
// deleted.
func TestApplyStagedPoliciesRemovesWhenEveryPolicyIsGone(t *testing.T) {
	var mu sync.Mutex
	var pending pendingPolicies

	// What OnConfigVerify does for the empty body a removal delivers.
	pending.stage(nil)

	rec := &recordingInstaller{}
	j, err := applyStagedPolicies(&pending, &mu, rec.install, rec.undo)
	if err != nil {
		t.Fatalf("applyStagedPolicies: %v", err)
	}
	if j == nil {
		t.Error("a removal must record an undo journal, got none")
	}
	if len(rec.installs) != 1 {
		t.Fatalf("a removed policy block must install exactly one change, got %d", len(rec.installs))
	}
	if len(rec.installs[0]) != 0 {
		t.Errorf("a removed policy block must install the empty set, got %+v", rec.installs[0])
	}
}

// TestApplyStagedPoliciesInstallsTheStagedSet proves the ordinary commit path
// still installs what the verify accepted.
//
// VALIDATES: applyStagedPolicies passes the staged policies to install.
// PREVENTS: the removal fix swallowing the normal apply.
func TestApplyStagedPoliciesInstallsTheStagedSet(t *testing.T) {
	var mu sync.Mutex
	var pending pendingPolicies
	policies := []PolicyRoute{{Name: "wan"}}
	pending.stage(policies)

	rec := &recordingInstaller{}
	if _, err := applyStagedPolicies(&pending, &mu, rec.install, rec.undo); err != nil {
		t.Fatalf("applyStagedPolicies: %v", err)
	}
	if len(rec.installs) != 1 || len(rec.installs[0]) != 1 || rec.installs[0][0].Name != "wan" {
		t.Errorf("the staged policies did not reach install: %+v", rec.installs)
	}
}

// TestApplyStagedPoliciesRefusesAnApplyWithoutAVerify proves the apply fails
// closed when no verify staged a candidate.
//
// VALIDATES: the `staged` fact, which the policy slice alone cannot carry.
// PREVENTS: an apply that reaches the plugin out of protocol reporting success
// over the previous policies, with nothing logged at any level to say so
// (ai/rules/principles.md -- a value that is silently wrong must not be
// reachable).
func TestApplyStagedPoliciesRefusesAnApplyWithoutAVerify(t *testing.T) {
	var mu sync.Mutex
	var pending pendingPolicies

	rec := &recordingInstaller{}
	_, err := applyStagedPolicies(&pending, &mu, rec.install, rec.undo)
	if !errors.Is(err, errApplyWithoutVerify) {
		t.Errorf("an apply with nothing staged must be refused, got %v", err)
	}
	if len(rec.installs) != 0 {
		t.Errorf("an apply with nothing staged must install nothing, installed %+v", rec.installs)
	}
}

// TestPendingPoliciesClearUnstagesARemoval proves a rolled-back removal does
// not arm the next apply.
//
// VALIDATES: pendingPolicies.clear, which the OnConfigRollback handler calls.
// PREVENTS: the (nil, true) pair surviving a rollback and tearing down live
// policy routes on the next transaction that reaches an apply.
func TestPendingPoliciesClearUnstagesARemoval(t *testing.T) {
	var pending pendingPolicies
	pending.stage(nil)
	pending.clear()

	if policies, staged := pending.take(); staged || policies != nil {
		t.Errorf("clear must unstage the candidate, got policies=%+v staged=%v", policies, staged)
	}
}
