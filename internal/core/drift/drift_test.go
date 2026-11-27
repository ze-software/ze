package drift

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDriftRegisterUnregisterAndCheckAll verifies local registration, token-safe
// idempotent unregistration, sorted CheckAll merging with remote precedence,
// single-subsystem Check, Subsystems union, and the 64-finding truncation cap.
//
// VALIDATES: AC-1, AC-2 -- Register/Unregister token safety, CheckAll sorting
// and remote collision precedence, Check single vs unknown, Subsystems union,
// and MaxFindingsPerSubsystem (64) truncation with Truncated: true.
// PREVENTS: stale unregister closures removing a newer registration, nil
// Findings slices in JSON, or unbounded finding slices from misbehaving checks.
func TestDriftRegisterUnregisterAndCheckAll(t *testing.T) {
	t.Run("register panics on invalid inputs", func(t *testing.T) {
		assertPanics(t, "empty subsystem", func() {
			Register("", func(context.Context) Report { return Report{} })
		})
		assertPanics(t, "nil local check", func() {
			Register("sub", nil)
		})
		assertPanics(t, "nil remote collector", func() {
			RegisterRemoteCollector(nil)
		})
		assertPanics(t, "nil remote lister", func() {
			RegisterRemoteLister(nil)
		})
	})

	t.Run("token-safe idempotent unregister", func(t *testing.T) {
		unreg1 := Register("alpha", func(context.Context) Report {
			return Report{Subsystem: "alpha", Status: StatusClean, Checked: 1}
		})
		t.Cleanup(unreg1)

		// Overwrite with a second registration; calling unreg1 MUST NOT remove the second.
		unreg2 := Register("alpha", func(context.Context) Report {
			return Report{Subsystem: "alpha", Status: StatusClean, Checked: 2}
		})
		t.Cleanup(unreg2)

		unreg1()
		unreg1() // idempotent

		rep, ok := Check(t.Context(), "alpha")
		if !ok {
			t.Fatalf("Check(alpha) = _, false after stale unreg1; want true")
		}
		if rep.Checked != 2 {
			t.Fatalf("Check(alpha).Checked = %d, want 2", rep.Checked)
		}

		unreg2()
		unreg2() // idempotent

		if _, ok := Check(t.Context(), "alpha"); ok {
			t.Fatalf("Check(alpha) = _, true after unreg2; want false")
		}

		// Same token safety for remote collector and remote lister.
		unregRC1 := RegisterRemoteCollector(func(context.Context, string) []Report {
			return []Report{{Subsystem: "rc", Status: StatusClean, Checked: 1}}
		})
		t.Cleanup(unregRC1)
		unregRC2 := RegisterRemoteCollector(func(context.Context, string) []Report {
			return []Report{{Subsystem: "rc", Status: StatusClean, Checked: 2}}
		})
		t.Cleanup(unregRC2)

		unregRC1()
		unregRC1()
		rcRep, ok := Check(t.Context(), "rc")
		if !ok || rcRep.Checked != 2 {
			t.Fatalf("Check(rc) after stale unregRC1 = (%+v, %v), want Checked=2, true", rcRep, ok)
		}
		unregRC2()
		unregRC2()
		if _, ok := Check(t.Context(), "rc"); ok {
			t.Fatalf("Check(rc) after unregRC2 = _, true; want false")
		}

		unregRL1 := RegisterRemoteLister(func() []string { return []string{"rl1"} })
		t.Cleanup(unregRL1)
		unregRL2 := RegisterRemoteLister(func() []string { return []string{"rl2"} })
		t.Cleanup(unregRL2)

		unregRL1()
		unregRL1()
		gotRL := Subsystems()
		if len(gotRL) != 1 || gotRL[0] != "rl2" {
			t.Fatalf("Subsystems() after stale unregRL1 = %v, want [rl2]", gotRL)
		}
		unregRL2()
		unregRL2()
		if got := Subsystems(); len(got) != 0 {
			t.Fatalf("Subsystems() after unregRL2 = %v, want empty", got)
		}
	})

	t.Run("CheckAll sorting remote merge and Subsystems union", func(t *testing.T) {
		unregZ := Register("zeta", func(ctx context.Context) Report {
			if _, hasDeadline := ctx.Deadline(); !hasDeadline {
				t.Error("local checker ctx has no timeout deadline")
			}
			return Report{Subsystem: "zeta", Status: StatusClean, Checked: 3}
		})
		t.Cleanup(unregZ)

		unregShared := Register("shared", func(context.Context) Report {
			return Report{Subsystem: "shared", Status: StatusClean, Checked: 1}
		})
		t.Cleanup(unregShared)

		unregRemote := RegisterRemoteCollector(func(_ context.Context, subsystem string) []Report {
			if subsystem == "shared" {
				return []Report{{Subsystem: "shared", Status: StatusDrifted, Checked: 5, Drifted: 1, Findings: []Finding{{Subject: "r1", Kind: KindMissing}}}}
			}
			if subsystem == "beta" {
				return []Report{{Subsystem: "beta", Status: StatusUnavailable}}
			}
			if subsystem != "" {
				return nil
			}
			return []Report{
				{Subsystem: "shared", Status: StatusDrifted, Checked: 5, Drifted: 1, Findings: []Finding{{Subject: "r1", Kind: KindMissing}}},
				{Subsystem: "beta", Status: StatusUnavailable},
			}
		})
		t.Cleanup(unregRemote)

		unregLister := RegisterRemoteLister(func() []string {
			return []string{"beta", "shared", "beta", ""}
		})
		t.Cleanup(unregLister)

		gotSubs := Subsystems()
		wantSubs := []string{"beta", "shared", "zeta"}
		if len(gotSubs) != len(wantSubs) {
			t.Fatalf("Subsystems() = %v, want %v", gotSubs, wantSubs)
		}
		for i := range wantSubs {
			if gotSubs[i] != wantSubs[i] {
				t.Fatalf("Subsystems()[%d] = %q, want %q", i, gotSubs[i], wantSubs[i])
			}
		}

		reports := CheckAll(t.Context())
		if len(reports) != 3 {
			t.Fatalf("CheckAll returned %d reports, want 3: %+v", len(reports), reports)
		}
		if reports[0].Subsystem != "beta" || reports[1].Subsystem != "shared" || reports[2].Subsystem != "zeta" {
			t.Fatalf("CheckAll order = [%s, %s, %s], want [beta, shared, zeta]",
				reports[0].Subsystem, reports[1].Subsystem, reports[2].Subsystem)
		}
		// Remote wins on name collision for "shared".
		if reports[1].Status != StatusDrifted || reports[1].Checked != 5 {
			t.Errorf("shared report = %+v, want remote StatusDrifted Checked=5", reports[1])
		}
		// Findings must always be normalized to non-nil empty slice.
		if reports[0].Findings == nil || reports[2].Findings == nil {
			t.Errorf("nil Findings slice in normalized reports: beta=%v zeta=%v", reports[0].Findings, reports[2].Findings)
		}

		// Single Check prefers remote, falls back to local, returns false on unknown.
		repShared, ok := Check(t.Context(), "shared")
		if !ok || repShared.Status != StatusDrifted {
			t.Errorf("Check(shared) = (%+v, %v), want drifted, true", repShared, ok)
		}
		repZeta, ok := Check(t.Context(), "zeta")
		if !ok || repZeta.Checked != 3 || repZeta.Findings == nil {
			t.Errorf("Check(zeta) = (%+v, %v), want checked=3 non-nil findings, true", repZeta, ok)
		}
		if _, ok := Check(t.Context(), "nonexistent"); ok {
			t.Error("Check(nonexistent) = _, true; want false")
		}
		if _, ok := Check(t.Context(), ""); ok {
			t.Error("Check(\"\") = _, true; want false")
		}
	})

	t.Run("truncates findings above MaxFindingsPerSubsystem", func(t *testing.T) {
		exactFindings := make([]Finding, MaxFindingsPerSubsystem)
		for i := range exactFindings {
			exactFindings[i] = Finding{Subject: "obj", Kind: KindMismatch}
		}
		unregExact := Register("exact64", func(context.Context) Report {
			return Report{
				Subsystem: "exact64",
				Status:    StatusDrifted,
				Checked:   MaxFindingsPerSubsystem,
				Drifted:   MaxFindingsPerSubsystem,
				Findings:  exactFindings,
			}
		})
		t.Cleanup(unregExact)

		overFindings := make([]Finding, MaxFindingsPerSubsystem+1)
		for i := range overFindings {
			overFindings[i] = Finding{Subject: "obj", Kind: KindMissing}
		}
		unregOver := Register("over65", func(context.Context) Report {
			return Report{
				Subsystem: "over65",
				Status:    StatusDrifted,
				Checked:   MaxFindingsPerSubsystem + 1,
				Drifted:   MaxFindingsPerSubsystem + 1,
				Findings:  overFindings,
			}
		})
		t.Cleanup(unregOver)

		exactRep, ok := Check(t.Context(), "exact64")
		if !ok {
			t.Fatal("Check(exact64) returned false")
		}
		if exactRep.Truncated || len(exactRep.Findings) != MaxFindingsPerSubsystem {
			t.Errorf("exact64: len=%d truncated=%v, want %d false", len(exactRep.Findings), exactRep.Truncated, MaxFindingsPerSubsystem)
		}

		overRep, ok := Check(t.Context(), "over65")
		if !ok {
			t.Fatal("Check(over65) returned false")
		}
		if !overRep.Truncated || len(overRep.Findings) != MaxFindingsPerSubsystem {
			t.Errorf("over65: len=%d truncated=%v, want %d true", len(overRep.Findings), overRep.Truncated, MaxFindingsPerSubsystem)
		}
		if overRep.Drifted != MaxFindingsPerSubsystem+1 {
			t.Errorf("over65: Drifted=%d, want %d", overRep.Drifted, MaxFindingsPerSubsystem+1)
		}
	})

	t.Run("concurrent register and CheckAll are race-free", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(2)
		go runConcurrentDriftWriter(&wg)
		go runConcurrentDriftReader(t.Context(), &wg)
		wg.Wait()
	})
}

func runConcurrentDriftWriter(wg *sync.WaitGroup) {
	defer wg.Done()
	for range 50 {
		unreg := Register("concurrent-sub", func(context.Context) Report {
			return Report{Subsystem: "concurrent-sub", Status: StatusClean, Checked: 1}
		})
		unreg()
	}
}

func runConcurrentDriftReader(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for range 50 {
		_ = CheckAll(ctx)
		_ = Subsystems()
	}
}

// TestDriftDoctorCheck verifies init registration of CodeDoctorDataplaneDrift
// and the dataplane-drift DoctorPhaseRuntime check, plus clean/drifted/error
// diagnostic emission.
//
// VALIDATES: AC-3 -- CodeDoctorDataplaneDrift is registered via diagnostic.Register,
// dataplane-drift is registered in DoctorPhaseRuntime at order 860, clean and
// unavailable subsystems emit no diagnostics, and drifted/error subsystems emit
// SeverityWarning diagnostics with Fix "ze show system drift".
// PREVENTS: drifted or errored dataplane subsystems going unreported in show doctor.
func TestDriftDoctorCheck(t *testing.T) {
	if meta := diagnostic.Lookup(CodeDoctorDataplaneDrift); meta == nil {
		t.Fatalf("diagnostic.Lookup(%q) = nil, want registered CodeMeta", CodeDoctorDataplaneDrift)
	}

	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime)
	for i := range checks {
		if checks[i].Name == dataplaneDriftCheckName {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("DoctorPhaseRuntime does not contain %q", dataplaneDriftCheckName)
	}
	if found.Order != dataplaneDriftOrder {
		t.Errorf("check Order = %d, want %d", found.Order, dataplaneDriftOrder)
	}
	if found.Component != dataplaneDriftComponent {
		t.Errorf("check Component = %q, want %q", found.Component, dataplaneDriftComponent)
	}

	t.Run("clean and unavailable produce nil diagnostics", func(t *testing.T) {
		unregClean := Register("clean-sub", func(context.Context) Report {
			return Report{Subsystem: "clean-sub", Status: StatusClean, Checked: 4}
		})
		t.Cleanup(unregClean)
		unregUnavail := Register("unavail-sub", func(context.Context) Report {
			return Report{Subsystem: "unavail-sub", Status: StatusUnavailable}
		})
		t.Cleanup(unregUnavail)

		diags := checkDataplaneDrift(diagnostic.DoctorCheckContext{})
		if len(diags) != 0 {
			t.Fatalf("checkDataplaneDrift = %+v, want none", diags)
		}
	})

	t.Run("drifted and error produce warning diagnostics", func(t *testing.T) {
		unregDrifted := Register("fw-test", func(context.Context) Report {
			return Report{
				Subsystem: "fw-test",
				Status:    StatusDrifted,
				Checked:   10,
				Drifted:   2,
				Findings: []Finding{
					{Subject: "ze_filter", Kind: KindMissing},
					{Subject: "ze_nat", Kind: KindExtra},
				},
			}
		})
		t.Cleanup(unregDrifted)

		unregErr := Register("fib-test", func(context.Context) Report {
			return Report{
				Subsystem: "fib-test",
				Status:    StatusError,
				Error:     "netlink read timeout",
			}
		})
		t.Cleanup(unregErr)

		diags := checkDataplaneDrift(diagnostic.DoctorCheckContext{})
		if len(diags) != 2 {
			t.Fatalf("checkDataplaneDrift returned %d diagnostics, want 2: %+v", len(diags), diags)
		}

		// Reports are sorted by subsystem name: fib-test then fw-test.
		if diags[0].Code != CodeDoctorDataplaneDrift || diags[0].Severity != diagnostic.SeverityWarning || diags[0].Fix != dataplaneDriftFix {
			t.Errorf("diag[0] = %+v, want code=%s severity=%s fix=%q", diags[0], CodeDoctorDataplaneDrift, diagnostic.SeverityWarning, dataplaneDriftFix)
		}
		if !strings.Contains(diags[0].Message, "fib-test") || !strings.Contains(diags[0].Message, "netlink read timeout") {
			t.Errorf("diag[0].Message = %q, want fib-test and netlink read timeout", diags[0].Message)
		}

		if diags[1].Code != CodeDoctorDataplaneDrift || diags[1].Severity != diagnostic.SeverityWarning || diags[1].Fix != dataplaneDriftFix {
			t.Errorf("diag[1] = %+v, want code=%s severity=%s fix=%q", diags[1], CodeDoctorDataplaneDrift, diagnostic.SeverityWarning, dataplaneDriftFix)
		}
		if !strings.Contains(diags[1].Message, "fw-test") || !strings.Contains(diags[1].Message, "2 finding(s)") {
			t.Errorf("diag[1].Message = %q, want fw-test and 2 finding(s)", diags[1].Message)
		}
	})
}

func assertPanics(t *testing.T, label string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic, got none", label)
		}
	}()
	fn()
}
