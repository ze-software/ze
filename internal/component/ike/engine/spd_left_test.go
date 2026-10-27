// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: spd_left.go -- the ledger these tests fill and read
// Related: rfc4301_spd_lifecycle_test.go -- lifecycleDP, resetSPDLeft, lifecycleConfig

package engine

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
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
	// The ledger is named in the XFRM form, whose commands the assertion spells out.
	left := spdLeft.describe(&xfrmCommandDP{})
	for _, p := range dp.installed {
		want := "ip xfrm policy delete " + dirLabel(p.Dir) + " src " + p.Src.String() + " dst " + p.Dst.String()
		if !slices.ContainsFunc(left, func(entry string) bool { return strings.HasPrefix(entry, want) }) {
			t.Errorf("the inbound policy the rollback failed to remove (%s) is not in the SPD ledger: %v", want, left)
		}
	}
	if !warnSPDLeft(&xfrmCommandDP{}, log) {
		t.Error("warnSPDLeft reports nothing left, so releaseSPD would remove the catch-all beneath the stuck entry")
	}
	if !strings.Contains(logged.String(), "could not remove a failed tunnel's policy") {
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

	if left := spdLeft.describe(dp); len(left) != 0 {
		t.Errorf("the SPD ledger holds entries the backend never installed: %v", left)
	}
	if strings.Contains(logged.String(), "catch-all stays installed") {
		t.Errorf("the exit warned that entries were left installed when none was ever installed:\n%s", logged.String())
	}
}

// VALIDATES: two leftover entries that differ only by port are named by two different
// strings, and each string is the exact `ip xfrm policy delete` command that removes
// that entry and no other: direction, both prefixes, protocol, both ports and if_id.
// METHOD: record failed removals of two UDP bypass entries (ports 500 and 4500) and of
// one entry with no source prefix and an if_id, then compare describe and the warning
// text with the commands written out by hand.
// PREVENTS: describe printing only direction, prefixes and protocol, so the IKE bypass
// on port 500 and the one on port 4500 printed the same line and the operator could
// not tell which command removes which entry.
func TestSPDLeftoverNamesTheExactRemovalCommand(t *testing.T) {
	resetSPDLeft(t)
	refused := errors.New("netlink: operation not permitted")
	_, host1, _ := net.ParseCIDR("192.0.2.1/32")
	_, host2, _ := net.ParseCIDR("192.0.2.2/32")
	_, net6, _ := net.ParseCIDR("2001:db8::/64")

	bypass := func(port uint16) dataplane.SPParams {
		return dataplane.SPParams{
			Src: host1, Dst: host2, Dir: dataplane.SADirIn,
			UpperProto: 17,
			SrcPort:    dataplane.ExactPortMatch(port),
			DstPort:    dataplane.ExactPortMatch(port),
			Priority:   dataplane.PriorityIKEBypass,
			Action:     dataplane.SPActionBypass,
		}
	}
	ike, natt := bypass(500), bypass(4500)
	child := dataplane.SPParams{Dst: net6, Dir: dataplane.SADirOut, IfID: 7, Priority: dataplane.PriorityChildSA}
	for _, p := range []dataplane.SPParams{ike, natt, child} {
		spdLeft.record(&p, refused)
	}

	want := []string{
		"ip xfrm policy delete dir in src 192.0.2.1/32 dst 192.0.2.2/32 proto 17 sport 4500 dport 4500",
		"ip xfrm policy delete dir in src 192.0.2.1/32 dst 192.0.2.2/32 proto 17 sport 500 dport 500",
		"ip xfrm policy delete dir out src ::/0 dst 2001:db8::/64 if_id 7",
	}
	dp := &xfrmCommandDP{}
	left := spdLeft.describe(dp)
	if !slices.Equal(left, want) {
		t.Fatalf("describe named the leftover entries as\n%q\nwant\n%q", left, want)
	}

	var logged bytes.Buffer
	if !warnSPDLeft(dp, slog.New(slog.NewTextHandler(&logged, nil))) {
		t.Fatal("warnSPDLeft reports nothing left with three entries in the ledger")
	}
	for _, command := range want {
		if !strings.Contains(logged.String(), command) {
			t.Errorf("the warning does not carry the command %q:\n%s", command, logged.String())
		}
	}
}

// VALIDATES: the exit warning names each leftover entry in the removal command of the
// backend that holds it: the VPP backend's vppctl line on VPP, and, on a backend that
// names no command, a statement that it names none, with the selector.
// METHOD: record one failed removal, then read describe and warnSPDLeft through a
// dataplane naming VPP commands and through one naming none.
// PREVENTS: describe printing `ip xfrm policy delete` whatever the backend, a command
// that removes nothing from VPP's SPD.
func TestSPDLeftoverNamesTheBackendCommand(t *testing.T) {
	resetSPDLeft(t)
	_, dst, _ := net.ParseCIDR("10.1.2.3/32")
	p := dataplane.SPParams{Dst: dst, Dir: dataplane.SADirOut, Priority: dataplane.PriorityChildSA}
	spdLeft.record(&p, errors.New("vpp: ipsec spd entry del: retval -1"))

	vpp := &vppCommandDP{}
	if left := spdLeft.describe(vpp); !slices.Equal(left, []string{vppCommandFixture}) {
		t.Fatalf("describe on VPP named %q, want the VPP backend's command %q", left, vppCommandFixture)
	}
	var logged bytes.Buffer
	if !warnSPDLeft(vpp, slog.New(slog.NewTextHandler(&logged, nil))) {
		t.Fatal("warnSPDLeft reports nothing left with one entry in the ledger")
	}
	if strings.Contains(logged.String(), "ip xfrm") {
		t.Fatalf("the warning on VPP names an ip xfrm command:\n%s", logged.String())
	}

	left := spdLeft.describe(&failingRemoveDP{})
	if len(left) != 1 || !strings.HasPrefix(left[0], "(the dataplane names no command that removes the policy dir out") {
		t.Fatalf("describe on a backend naming no command named %q", left)
	}
}
