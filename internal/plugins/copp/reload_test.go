// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- verify-to-apply seam
// Related: register.go (pendingPolicy, applyStagedPolicy)

package copp

import (
	"errors"
	"sync"
	"testing"
)

// recordingInstaller stands in for applyCoppPolicy: it records what the apply
// asked for, so the decision can be proven without a firewall backend.
type recordingInstaller struct {
	calls []*coppPolicy
	err   error
}

func (r *recordingInstaller) install(policy *coppPolicy) error {
	r.calls = append(r.calls, policy)
	return r.err
}

func samplePolicy() *coppPolicy {
	return &coppPolicy{Rate: 100, RateUnit: "second", ProtectedPorts: []uint16{defaultPort}, OverPolicy: overPolicyAccept}
}

// TestApplyStagedPolicyWithdrawsWhenTheSectionIsRemoved proves that deleting
// the `control-plane-protection` block takes copp's table out of the kernel.
//
// VALIDATES: pendingPolicy carries "a verify ran" beside "what it found", so
// applyStagedPolicy can tell a removal from an empty stage and installs the
// withdrawal.
// PREVENTS: the defect recorded in
// plan/journal/component-rebuilt-during-reload.md (2026-09-09). parseCoppConfig
// reports found false for the empty body a reload delivers for a removed root,
// the verify staged nil, and the apply guarded on `if newPolicy == nil { return
// nil }` and withdrew nothing. The nftables table stayed in the kernel, rate
// limiting the control plane against a policy the operator had deleted, and the
// comment at the tail of runCoppPlugin claimed the opposite.
func TestApplyStagedPolicyWithdrawsWhenTheSectionIsRemoved(t *testing.T) {
	var mu sync.Mutex
	var pending pendingPolicy
	old := samplePolicy()

	// What OnConfigVerify does for the empty body a removal delivers.
	pending.stage(nil)

	rec := &recordingInstaller{}
	j, err := applyStagedPolicy(&pending, &mu, old, rec.install)
	if err != nil {
		t.Fatalf("applyStagedPolicy: %v", err)
	}
	if j == nil {
		t.Error("a removal must record an undo journal, got none")
	}
	if len(rec.calls) != 1 {
		t.Fatalf("a removed section must install exactly one change, got %d", len(rec.calls))
	}
	if rec.calls[0] != nil {
		t.Errorf("a removed section must install the withdrawal (nil policy), got %+v", rec.calls[0])
	}
}

// TestApplyStagedPolicyInstallsTheStagedPolicy proves the ordinary commit path
// still installs what the verify accepted.
//
// VALIDATES: applyStagedPolicy passes the staged policy to install.
// PREVENTS: the removal fix silently swallowing the normal apply.
func TestApplyStagedPolicyInstallsTheStagedPolicy(t *testing.T) {
	var mu sync.Mutex
	var pending pendingPolicy
	policy := samplePolicy()
	pending.stage(policy)

	rec := &recordingInstaller{}
	if _, err := applyStagedPolicy(&pending, &mu, nil, rec.install); err != nil {
		t.Fatalf("applyStagedPolicy: %v", err)
	}
	if len(rec.calls) != 1 || rec.calls[0] != policy {
		t.Errorf("the staged policy did not reach install: %+v", rec.calls)
	}
}

// TestApplyStagedPolicyRefusesAnApplyWithoutAVerify proves the apply fails
// closed when no verify staged a candidate.
//
// VALIDATES: the `staged` fact, which the policy pointer alone cannot carry.
// PREVENTS: an apply that reaches the plugin out of protocol reporting success
// over the previous policy, with nothing logged at any level to say so
// (ai/rules/principles.md -- a value that is silently wrong must not be
// reachable).
func TestApplyStagedPolicyRefusesAnApplyWithoutAVerify(t *testing.T) {
	var mu sync.Mutex
	var pending pendingPolicy

	rec := &recordingInstaller{}
	_, err := applyStagedPolicy(&pending, &mu, samplePolicy(), rec.install)
	if !errors.Is(err, errApplyWithoutVerify) {
		t.Errorf("an apply with nothing staged must be refused, got %v", err)
	}
	if len(rec.calls) != 0 {
		t.Errorf("an apply with nothing staged must install nothing, installed %+v", rec.calls)
	}
}

// TestApplyStagedPolicyUnstagesTheCandidate proves one verify arms exactly one
// apply.
//
// VALIDATES: take unstages, so a second apply behind one verify reaches the
// fail-closed branch instead of re-installing.
// PREVENTS: a candidate left staged being applied by the NEXT transaction that
// reaches an apply without a verify. The pair a rolled-back removal leaves
// behind is (nil, true), so that later apply would withdraw a table nobody
// asked it to withdraw.
func TestApplyStagedPolicyUnstagesTheCandidate(t *testing.T) {
	var mu sync.Mutex
	var pending pendingPolicy
	pending.stage(samplePolicy())

	rec := &recordingInstaller{}
	if _, err := applyStagedPolicy(&pending, &mu, nil, rec.install); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if _, err := applyStagedPolicy(&pending, &mu, nil, rec.install); !errors.Is(err, errApplyWithoutVerify) {
		t.Errorf("a second apply behind one verify must be refused, got %v", err)
	}
	if len(rec.calls) != 1 {
		t.Errorf("one verify must arm exactly one install, got %d", len(rec.calls))
	}
}

// TestPendingPolicyClearUnstagesARemoval proves a rolled-back removal does not
// arm the next apply.
//
// VALIDATES: pendingPolicy.clear, which the OnConfigRollback handler calls.
// PREVENTS: the (nil, true) pair surviving a rollback and withdrawing copp's
// table on the next transaction that reaches an apply.
func TestPendingPolicyClearUnstagesARemoval(t *testing.T) {
	var pending pendingPolicy
	pending.stage(nil)
	pending.clear()

	if policy, staged := pending.take(); staged || policy != nil {
		t.Errorf("clear must unstage the candidate, got policy=%+v staged=%v", policy, staged)
	}
}
