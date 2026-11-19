// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: spd_policy.go -- installSPDPolicies and installSPDPoliciesAtStart, the reconcilers under test
// Related: doctor_spd_install.go -- the ipsec-spd-install check the start-time refusal feeds
// RFC: rfc/short/rfc4301.md -- the SPD management interface (Section 4.4.1)

package engine

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// refusingSPDDP is lifecycleDP with an install refusal: refuse answers the error the
// backend returns for one policy, nil to install it.
type refusingSPDDP struct {
	lifecycleDP
	refuse func(p dataplane.SPParams) error
}

func (d *refusingSPDDP) InstallPolicy(p dataplane.SPParams) error {
	if d.refuse != nil {
		if err := d.refuse(p); err != nil {
			return err
		}
	}
	return d.lifecycleDP.InstallPolicy(p)
}

// errSPDRefusedByTest is the dataplane's answer to a refused install. It is none of
// the answers isXFRMUnsupported reads as a platform with no XFRM.
var errSPDRefusedByTest = errors.New("netlink: no buffer space available")

// spdDiscardEntry is the lifecycle bypass entry turned into a discard entry.
func spdDiscardEntry(t *testing.T, name, local string) ipsec.SPDPolicy {
	t.Helper()
	entry := failClosedBypassEntry(t, name, local)
	entry.Action = dataplane.SPActionDiscard
	return entry
}

// withPolicies is cfg with its operator entries replaced, as a new configuration the
// way a reload delivers one.
func withPolicies(cfg *ipsec.IPsecConfig, policies map[string]ipsec.SPDPolicy) *ipsec.IPsecConfig {
	next := *cfg
	next.Policies = policies
	return &next
}

// wantEntryHeld fails unless the backend holds every policy of entry exactly: its
// selector, its disposition and its order.
func wantEntryHeld(t *testing.T, dp *refusingSPDDP, entry ipsec.SPDPolicy) {
	t.Helper()
	policies := spdPolicyParams(entry)
	for i := range policies {
		sp := &policies[i]
		held, ok := dp.live[keyOf(*sp)]
		if !ok {
			t.Fatalf("the dataplane does not hold %s's %s policy %s -> %s", entry.Name, dirLabel(sp.Dir), sp.Src, sp.Dst)
		}
		if held.Action != sp.Action || held.Priority != sp.Priority {
			t.Fatalf("the dataplane holds %s's %s policy as action %d priority %d, want action %d priority %d",
				entry.Name, dirLabel(sp.Dir), held.Action, held.Priority, sp.Action, sp.Priority)
		}
	}
}

// VALIDATES: AC-14. An operator entry the dataplane refuses on a reload refuses the
// apply with an operator error that names the entry, carries the dataplane's answer
// as its cause and names the command that retries, and the dataplane still holds the
// entry's previous form, whether the edit kept its selector (replaced in place) or
// moved it; a first apply that arrives as a reload is refused the same way.
// METHOD: start with a discard entry on a backend that records the database, then
// reload an edit of it while the backend refuses the INBOUND half of every operator
// entry, so the outbound half of the edit was already written when the refusal came
// and the apply has to take it back; read the error and the database.
// PREVENTS: the D-1 defect, where installSPDPolicies logged the refusal at Warn and
// the apply succeeded, and removed the old form first, so the traffic the discard
// named crossed the boundary from the removal on.
//
// RFC requirement: RFC4301-4.4.1-7 negative -- an operator entry the dataplane refuses on a reload fails applyConfig with an operator error naming the entry, wrapping the dataplane's answer and naming `commit` as the retry, and the dataplane keeps the entry's previous selector, disposition and order.
func TestRFC4301FailedOperatorEntryInstallRefusesTheApply(t *testing.T) {
	refuseInbound := func(p dataplane.SPParams) error {
		if p.Role == dataplane.PolicyRoleOperator && p.Dir == dataplane.SADirIn {
			return errSPDRefusedByTest
		}
		return nil
	}
	for _, tc := range []struct {
		name string
		edit func(ipsec.SPDPolicy) ipsec.SPDPolicy
	}{
		{"same selector, new order", func(p ipsec.SPDPolicy) ipsec.SPDPolicy {
			p.Order = 1500
			return p
		}},
		{"new selector", func(p ipsec.SPDPolicy) ipsec.SPDPolicy {
			p.LocalPrefix = mustCIDR(t, "192.0.2.0/25")
			return p
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetSPDLeft(t)
			dp := &refusingSPDDP{}
			loadFailClosedBackend(t, dp)
			state := applyTestState(t)
			old := spdDiscardEntry(t, "drop", "192.0.2.0/24")
			cfg := withPolicies(lifecycleConfig(t), map[string]ipsec.SPDPolicy{"drop": old})
			if err := state.applyConfig(cfg, applyStartup); err != nil {
				t.Fatalf("start: %v", err)
			}
			wantEntryHeld(t, dp, old)

			dp.refuse = refuseInbound
			edited := tc.edit(old)
			err := state.applyConfig(withPolicies(cfg, map[string]ipsec.SPDPolicy{"drop": edited}), applyReload)
			if err == nil {
				t.Fatal("a reload whose operator entry the dataplane refused was applied")
			}
			refusal, ok := errors.AsType[*opererr.Error](err)
			if !ok {
				t.Fatalf("the refusal is not an operator error: %v", err)
			}
			if !strings.Contains(refusal.Problem(), "drop") {
				t.Fatalf("the refusal does not name the entry: %q", refusal.Problem())
			}
			if !errors.Is(err, errSPDRefusedByTest) {
				t.Fatalf("the refusal does not carry the dataplane's answer: %v", err)
			}
			if refusal.Fix() != spdInstallFixCommit {
				t.Fatalf("the refusal's fix is %q, want %q", refusal.Fix(), spdInstallFixCommit)
			}
			wantEntryHeld(t, dp, old)
			for _, sp := range spdPolicyParams(edited) {
				if _, moved := dp.live[keyOf(sp)]; moved && !spdPoliciesShareASelector([]dataplane.SPParams{sp}, spdPolicyParams(old)) {
					t.Fatalf("the refused edit's %s policy %s -> %s is still installed", dirLabel(sp.Dir), sp.Src, sp.Dst)
				}
			}
			if !spdPolicySame(state.installedSPD["drop"], old) {
				t.Fatal("the engine records the refused edit as installed")
			}
		})
	}

	t.Run("first apply as a reload", func(t *testing.T) {
		resetSPDLeft(t)
		dp := &refusingSPDDP{refuse: refuseInbound}
		loadFailClosedBackend(t, dp)
		state := applyTestState(t)
		entry := spdDiscardEntry(t, "drop", "192.0.2.0/24")
		err := state.applyConfig(withPolicies(lifecycleConfig(t), map[string]ipsec.SPDPolicy{"drop": entry}), applyReload)
		if _, ok := errors.AsType[*opererr.Error](err); !ok {
			t.Fatalf("a first apply whose operator entry the dataplane refused answered %v, want an operator error", err)
		}
		for _, sp := range spdPolicyParams(entry) {
			if _, held := dp.live[keyOf(sp)]; held {
				t.Fatalf("the refused entry's %s half stayed installed", dirLabel(sp.Dir))
			}
		}
	})
}

// VALIDATES: AC-14 at the start, owner decision 2026-11-19 ("Install, log, doctor").
// The start applies a configuration with an entry the dataplane refuses: it installs
// every other entry, leaves the refused one out whole, logs it at Error, and the
// ipsec-spd-install doctor check reports it with the restart as its fix. A later
// reload, with the cause gone, installs it and clears the finding.
// METHOD: refuse every policy of one entry, start, read the database, the log and the
// check; then lift the refusal, reload the same entries and read both again.
// PREVENTS: the start refusing the IPsec block (the access-security door trap "A
// refusal added for reload also ran at startup"), and an entry the start could not
// install going unreported, or never retried because the engine recorded it as in.
func TestSPDInstallRefusedAtStartIsReportedByDoctor(t *testing.T) {
	resetSPDLeft(t)
	refused := spdDiscardEntry(t, "drop", "192.0.2.0/24")
	kept := failClosedBypassEntry(t, "keep", "203.0.113.0/24")
	dp := &refusingSPDDP{refuse: func(p dataplane.SPParams) error {
		if p.Role != dataplane.PolicyRoleOperator {
			return nil
		}
		if p.Src.String() == "192.0.2.0/24" || p.Dst.String() == "192.0.2.0/24" {
			return errSPDRefusedByTest
		}
		return nil
	}}
	loadFailClosedBackend(t, dp)
	var logged bytes.Buffer
	state := applyTestState(t)
	state.log = slog.New(slog.NewTextHandler(&logged, nil))
	policies := map[string]ipsec.SPDPolicy{"drop": refused, "keep": kept}
	cfg := withPolicies(lifecycleConfig(t), policies)

	if err := state.applyConfig(cfg, applyStartup); err != nil {
		t.Fatalf("the start refused the IPsec block: %v", err)
	}
	wantEntryHeld(t, dp, kept)
	for _, sp := range spdPolicyParams(refused) {
		if _, held := dp.live[keyOf(sp)]; held {
			t.Fatalf("the refused entry's %s half is installed; the start installs an entry whole or not at all", dirLabel(sp.Dir))
		}
	}
	if out := logged.String(); !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "could not install operator SPD policy drop") {
		t.Fatalf("the start did not log the refused entry at Error:\n%s", out)
	}
	findings := answerSPDInstall(state)
	if len(findings) != 1 {
		t.Fatalf("the doctor check answered %d findings, want 1 for the refused entry: %+v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Code != diagnostic.CodeDoctorIPsecSPDInstallRefused || finding.Severity != severityError {
		t.Fatalf("the finding is %s/%s, want %s/%s", finding.Severity, finding.Code, severityError, diagnostic.CodeDoctorIPsecSPDInstallRefused)
	}
	if !strings.Contains(finding.Message, "drop") || !strings.Contains(finding.Message, errSPDRefusedByTest.Error()) {
		t.Fatalf("the finding does not name the entry and the dataplane's answer: %q", finding.Message)
	}
	if finding.Fix != spdInstallFixRestart {
		t.Fatalf("the finding's fix is %q, want %q", finding.Fix, spdInstallFixRestart)
	}

	dp.refuse = nil
	if err := state.applyConfig(withPolicies(cfg, policies), applyReload); err != nil {
		t.Fatalf("the reload with the cause gone was refused: %v", err)
	}
	wantEntryHeld(t, dp, refused)
	if findings := answerSPDInstall(state); len(findings) != 0 {
		t.Fatalf("the doctor still reports an entry the reload installed: %+v", findings)
	}
}

// VALIDATES: AC-15, "the dataplane receives the entries in order, never in map order".
// METHOD: reconcile entries whose orders are not their name order onto a recording
// backend, many times, and read the order of the operator writes.
// PREVENTS: installSPDPolicies ranging over the map, which handed the backend a new
// sequence on every apply.
func TestSPDEntriesReachTheDataplaneInOrder(t *testing.T) {
	entries := map[string]ipsec.SPDPolicy{}
	for name, spec := range map[string]struct {
		local string
		order uint32
	}{
		"a-last":   {"192.0.2.0/26", 3000},
		"b-first":  {"192.0.2.64/26", 1000},
		"c-middle": {"192.0.2.128/26", 2000},
		"d-middle": {"192.0.2.192/26", 2000},
	} {
		entry := failClosedBypassEntry(t, name, spec.local)
		entry.Order = spec.order
		entries[name] = entry
	}
	want := []int{1000, 1000, 2000, 2000, 2000, 2000, 3000, 3000}
	for range 20 {
		dp := &bypassDP{}
		if err := installSPDPolicies(dp, nil, entries, slogutil.DiscardLogger()); err != nil {
			t.Fatalf("install: %v", err)
		}
		if len(dp.installed) != len(want) {
			t.Fatalf("installed %d policies, want %d", len(dp.installed), len(want))
		}
		for i := range want {
			if dp.installed[i].Priority != want[i] {
				t.Fatalf("write %d is priority %d, want %d: the entries did not reach the dataplane in their order", i, dp.installed[i].Priority, want[i])
			}
		}
		if dp.installed[2].Src.String() != "192.0.2.128/26" {
			t.Fatalf("within order 2000 the first write is %s, want c-middle's 192.0.2.128/26 (name order)", dp.installed[2].Src)
		}
	}
}
