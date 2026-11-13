// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- namespace forward default.
// Related: unmatched.go -- optional backend strategy, apply rollback and final release.

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

type forwardDefaultTestDP struct {
	*spdDP
	enabled                                  bool
	action                                   dataplane.SPAction
	applyErr, undoErr, installErr, removeErr error
	applies, undos, releases                 int
}

func (d *forwardDefaultTestDP) UsesForwardDefault() bool { return d.enabled }

func (d *forwardDefaultTestDP) ApplyForwardDefault(action dataplane.SPAction) (func() error, error) {
	d.applies++
	if d.applyErr != nil {
		return nil, d.applyErr
	}
	previous := d.action
	d.action = action
	return func() error {
		d.undos++
		if d.undoErr != nil {
			return d.undoErr
		}
		d.action = previous
		return nil
	}, nil
}

func (d *forwardDefaultTestDP) ReleaseForwardDefault() error {
	d.releases++
	return nil
}

func (d *forwardDefaultTestDP) InstallPolicy(policy dataplane.SPParams) error {
	if d.installErr != nil && policy.Dir == dataplane.SADirOut {
		return d.installErr
	}
	return d.spdDP.InstallPolicy(policy)
}

func (d *forwardDefaultTestDP) RemovePolicyParams(policy dataplane.SPParams) error {
	if d.removeErr != nil && policy.Dir == dataplane.SADirIn {
		return d.removeErr
	}
	return d.spdDP.RemovePolicyParams(policy)
}

func TestUnmatchedForwardDefaultRefusalInstallsNoOtherFinalEntry(t *testing.T) {
	refusal := errors.New("default policy refused")
	dp := &forwardDefaultTestDP{spdDP: newSPDDP(), enabled: true, applyErr: refusal}
	if err := installUnmatched(dp, dataplane.SPActionDiscard, slogutil.DiscardLogger()); !errors.Is(err, refusal) {
		t.Fatalf("apply error=%v, want default refusal", err)
	}
	if len(dp.policies) != 0 {
		t.Fatalf("default refusal installed %d other final entries", len(dp.policies))
	}
}

func TestUnmatchedForwardDefaultRollsBackOnAnotherFinalEntryRefusal(t *testing.T) {
	refusal := errors.New("out final entry refused")
	dp := &forwardDefaultTestDP{spdDP: newSPDDP(), enabled: true,
		action: dataplane.SPActionDiscard, installErr: refusal}
	if err := installUnmatched(dp, dataplane.SPActionBypass, slogutil.DiscardLogger()); !errors.Is(err, refusal) {
		t.Fatalf("apply error=%v, want final-entry refusal", err)
	}
	if dp.action != dataplane.SPActionDiscard || dp.undos != 1 {
		t.Fatalf("failed reload did not restore the running default: action=%v undos=%d", dp.action, dp.undos)
	}
}

func TestUnmatchedForwardDefaultPreservesBothApplyAndRollbackRefusals(t *testing.T) {
	install := errors.New("out final entry refused")
	rollback := errors.New("default restoration refused")
	dp := &forwardDefaultTestDP{spdDP: newSPDDP(), enabled: true, installErr: install, undoErr: rollback}
	err := installUnmatched(dp, dataplane.SPActionDiscard, slogutil.DiscardLogger())
	if !errors.Is(err, install) || !errors.Is(err, rollback) {
		t.Fatalf("apply error=%v, want install and rollback refusals", err)
	}
}

func TestUnmatchedForwardDefaultStaysHeldWhileFinalRemovalFails(t *testing.T) {
	dp := &forwardDefaultTestDP{spdDP: newSPDDP(), enabled: true}
	log := slogutil.DiscardLogger()
	if err := installUnmatched(dp, dataplane.SPActionDiscard, log); err != nil {
		t.Fatal(err)
	}
	for _, policy := range dp.policies {
		if policy.Dir == dataplane.SADirFwd {
			t.Fatal("namespace-default backend received an explicit FWD final policy")
		}
	}
	dp.removeErr = errors.New("in final entry remains installed")
	removeUnmatched(dp, log)
	if dp.releases != 0 {
		t.Fatal("a failed final removal released the namespace's protective default")
	}
	dp.removeErr = nil
	removeUnmatched(dp, log)
	if dp.releases != 1 || len(dp.policies) != 0 {
		t.Fatalf("successful removal retry: releases=%d policies=%d", dp.releases, len(dp.policies))
	}
}

func TestUnmatchedForwardDefaultStrategyQueryPreservesUnsupportedWrappers(t *testing.T) {
	dp := &forwardDefaultTestDP{spdDP: newSPDDP(), enabled: false}
	log := slogutil.DiscardLogger()
	if err := installUnmatched(dp, dataplane.SPActionDiscard, log); err != nil {
		t.Fatal(err)
	}
	if len(dp.policies) != len(ikeBypassFamilies)*len(unmatchedDirections) {
		t.Fatalf("unsupported wrapper lost existing final directions: %d policies", len(dp.policies))
	}
	removeUnmatched(dp, log)
	if dp.applies != 0 || dp.releases != 0 || dp.undos != 0 {
		t.Fatalf("unsupported wrapper used namespace-default lifecycle: applies=%d releases=%d undos=%d", dp.applies, dp.releases, dp.undos)
	}
}

func TestInterfaceForwardDefaultOmitsOnlyItsFinalForwardEntry(t *testing.T) {
	dp := &forwardDefaultTestDP{spdDP: newSPDDP(), enabled: true}
	state := &ikeEngineState{log: slogutil.DiscardLogger()}
	plan := xfrmInterfacePlan{entries: make(map[spdSelector]*xfrmInterfaceEntry)}
	for _, policy := range interfaceSPDCatchAll(43) {
		plan.entries[selectorOf(&policy)] = &xfrmInterfaceEntry{
			iface: "test-xfrm", role: dataplane.PolicyRoleInterfaceCatchAll, params: policy,
		}
	}
	if err := state.installXFRMInterfaceEntries(dp, plan); err != nil {
		t.Fatal(err)
	}
	if len(dp.policies) != len(ikeBypassFamilies)*2 {
		t.Fatalf("interface final policies=%d, want IN and OUT for each family", len(dp.policies))
	}
	for _, policy := range dp.policies {
		if policy.Dir == dataplane.SADirFwd {
			t.Fatal("explicit interface FWD final entry suppresses the payload lookup")
		}
	}
	state.pruneXFRMInterfaceEntries(dp, xfrmInterfacePlan{})
	if len(dp.policies) != 0 {
		t.Fatal("interface teardown left its final entries installed")
	}
}
