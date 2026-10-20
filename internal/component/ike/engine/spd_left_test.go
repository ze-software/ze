// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: spd_left.go -- the ledger these tests fill and read
// Related: rfc4301_spd_lifecycle_test.go -- lifecycleDP, resetSPDLeft, lifecycleConfig

package engine

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// rollbackDP refuses every OUTBOUND policy install and every policy removal, so a
// Child SA install reaches its rollback arm and the rollback removal fails.
type rollbackDP struct {
	bypassDP
	refused error
}

func (d *rollbackDP) InstallPolicy(p dataplane.SPParams) error {
	if p.Dir == dataplane.SADirOut {
		return d.refused
	}
	return d.bypassDP.InstallPolicy(p)
}

func (d *rollbackDP) RemovePolicyParams(_ dataplane.SPParams) error {
	return d.refused
}

// VALIDATES: a Child SA install whose outbound policy is refused, and whose rollback
// then fails to remove the inbound policy it had installed, leaves that inbound policy
// in the SPD ledger, so releaseSPD keeps the catch-all beneath it at exit.
// METHOD: drive createFirstChildSA against a backend that refuses the outbound install
// and every removal, then read the ledger and warnSPDLeft.
// PREVENTS: the rollback calling the backend directly and dropping its error, which
// left a Ze entry installed that the ledger did not know, so the catch-all was removed
// from beneath it at exit.
func TestChildInstallRollbackFailureIsRecordedInTheSPDLedger(t *testing.T) {
	resetSPDLeft(t)
	dp := &rollbackDP{refused: errors.New("netlink: operation not permitted")}

	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	if _, err := createFirstChildSA(wideSA("site-b", "10.0.0.3"), testESPGroup(), "10.0.0.1", "10.0.0.3",
		testChildIfID, dp, log); err == nil {
		t.Fatal("the Child SA installed with its outbound policy refused")
	}

	if len(dp.installed) == 0 {
		t.Fatal("no inbound policy was installed, so the rollback had nothing to remove; the test proves nothing")
	}
	left := spdLeft.describe()
	for _, p := range dp.installed {
		want := dirLabel(p.Dir) + " " + p.Src.String() + " -> " + p.Dst.String()
		if !slices.Contains(left, want) {
			t.Errorf("the inbound policy the rollback failed to remove (%s) is not in the SPD ledger: %v", want, left)
		}
	}
	if !warnSPDLeft(log) {
		t.Error("warnSPDLeft reports nothing left, so releaseSPD would remove the catch-all beneath the stuck entry")
	}
	if !strings.Contains(logged.String(), "could not roll back a policy") {
		t.Errorf("the failed rollback removal was not logged:\n%s", logged.String())
	}
}

// vppLikeDP refuses, install and removal alike, every SPD entry that names no
// interface, with the error the VPP backend returns (vppPolicyInterface,
// dataplane/vpp_policy.go), and declares it holds no node-wide SPD.
type vppLikeDP struct {
	bypassDP
}

func (vppLikeDP) CatchAllSupported() error { return dataplane.ErrNoNodeWideSPD }

func vppRefusal(p dataplane.SPParams) error {
	if p.IfIndex > 0 {
		return nil
	}
	return fmt.Errorf("%w: vpp: a policy applies to the interfaces its SPD is bound to, and SPParams.IfIndex is %d; VPP has no node-wide SPD",
		dataplane.ErrNotSupported, p.IfIndex)
}

func (d *vppLikeDP) InstallPolicy(p dataplane.SPParams) error {
	if err := vppRefusal(p); err != nil {
		return err
	}
	return d.bypassDP.InstallPolicy(p)
}

func (d *vppLikeDP) RemovePolicyParams(p dataplane.SPParams) error {
	if err := vppRefusal(p); err != nil {
		return err
	}
	return d.bypassDP.RemovePolicyParams(p)
}

// VALIDATES: on a backend that refuses the IKE bypass and the operator entries as
// entries it cannot hold (VPP), an exit leaves the SPD ledger empty and logs no
// warning that the catch-all stays.
// METHOD: apply a configuration with an operator entry and `unmatched bypass` on a
// VPP-like backend, run the shutdown, then read the ledger and the log.
// PREVENTS: removeIKEBypass asking to remove bypass entries that were never installed,
// the backend refusing them with ErrNotSupported, and the ledger recording each one, so
// warnSPDLeft named eight nonexistent entries on every exit.
func TestVPPExitLeavesNoFalseSPDLeftovers(t *testing.T) {
	resetSPDLeft(t)
	dp := &vppLikeDP{}
	loadFailClosedBackend(t, dp)

	var logged bytes.Buffer
	state := applyTestState(t)
	state.log = slog.New(slog.NewTextHandler(&logged, nil))
	cfg := lifecycleConfig(t)
	cfg.Unmatched = dataplane.SPActionBypass
	if err := state.applyConfig(cfg, applyStartup); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(dp.installed) != 0 {
		t.Fatalf("the VPP-like backend accepted %d entries naming no interface; the test refused nothing", len(dp.installed))
	}

	state.shutdown(state.log)

	if left := spdLeft.describe(); len(left) != 0 {
		t.Errorf("the SPD ledger holds entries the backend never installed: %v", left)
	}
	if strings.Contains(logged.String(), "catch-all stays installed") {
		t.Errorf("the exit warned that entries were left installed when none was ever installed:\n%s", logged.String())
	}
}
