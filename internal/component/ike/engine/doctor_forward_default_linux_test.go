//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- namespace FWD default dependency.

package engine

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: the engine declares ipsec-forward-default as a runtime doctor check with an
// answer, under the one code doctor-ipsec-forward-default, and that code has an operator
// description.
// METHOD: find the check in engineDoctorChecks and read its declaration and the code's
// registered description.
// PREVENTS: a namespace FWD-default dependency `ze doctor` never asks about, or reports
// under a code with no description.
func TestDoctorIPsecForwardDefaultRegistered(t *testing.T) {
	// zeSetup populates the diagnostic descriptions after package init.
	diagnostic.RegisterBuiltinCodes()
	const code = "doctor-ipsec-forward-default"
	for _, check := range engineDoctorChecks {
		if check.decl.Name != "ipsec-forward-default" {
			continue
		}
		if check.decl.Phase != rpc.DoctorPhaseRuntime || check.answer == nil {
			t.Fatal("forward-default dependency has no runtime answer")
		}
		if len(check.decl.Codes) != 1 || check.decl.Codes[0] != code {
			t.Fatalf("forward-default dependency codes=%v, want [%s]", check.decl.Codes, code)
		}
		if diagnostic.Lookup(code) == nil {
			t.Fatal("forward-default dependency has no operator description")
		}
		return
	}
	t.Fatal("the namespace FWD-default dependency has no registered runtime doctor check")
}

type forwardDefaultDoctorDP struct {
	forwardDefaultTestDP
	failure error
	checks  int
}

func (d *forwardDefaultDoctorDP) CheckForwardDefault() error { d.checks++; return d.failure }

// VALIDATES: the check reads the loaded backend's live forwarding default once when the
// backend uses one, and never when it does not; a healthy read answers nothing, and a
// failed read answers one error that keeps the backend's problem, reason and fix.
// METHOD: answer the registered check over a backend that is healthy, one whose live
// check fails, and one that does not use the forwarding default, counting the reads.
// PREVENTS: a check that answers from a cached or assumed state, one that invents a
// finding for a backend without the strategy, and a finding that drops the fix.
func TestDoctorIPsecForwardDefaultReadsTheLiveChecker(t *testing.T) {
	var answer func(*ikeEngineState) []rpc.DoctorCheckDiagnostic
	for _, check := range engineDoctorChecks {
		if check.decl.Name == "ipsec-forward-default" {
			answer = check.answer
		}
	}
	if answer == nil {
		t.Fatal("the live forwarding-default check is not registered")
	}
	for _, tc := range []struct {
		name            string
		enabled, failed bool
	}{
		{"healthy claimed backend", true, false}, {"unreadable or conflicting claim", true, true}, {"unsupported wrapped backend", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dp := &forwardDefaultDoctorDP{spdDP: newSPDDP(), enabled: tc.enabled}
			if tc.failed {
				dp.failure = opererr.New("test: the namespace cannot be read", "test: its final forwarding disposition is unknown", "test-inspect-forward-default")
			}
			loadESPFormBackend(t, dp)
			findings := answer(nil)
			if dp.checks != map[bool]int{false: 0, true: 1}[tc.enabled] {
				t.Fatalf("live reads=%d, strategy=%v", dp.checks, tc.enabled)
			}
			if !tc.enabled || !tc.failed {
				if len(findings) != 0 {
					t.Fatalf("invented finding: %+v", findings)
				}
				return
			}
			if len(findings) != 1 || findings[0].Code != diagnostic.CodeDoctorIPsecForwardDefault ||
				findings[0].Severity != severityError || findings[0].Fix != "test-inspect-forward-default" ||
				!strings.Contains(findings[0].Message, "namespace cannot be read") || !strings.Contains(findings[0].Message, "disposition is unknown") {
				t.Fatalf("live failure lost its operator problem/why/fix: %+v", findings)
			}
		})
	}
}

// VALIDATES: one forwarding-default failure carries one severity on every surface: the
// doctor finding for a backend error that is not an operator error is an error whose fix
// is the restart that reads and claims the default again, and the coded log line of a
// failed removal is at Error, as its report-bus event is (rpc.ReportError).
// METHOD: answer the check over a backend whose live check fails with a plain error;
// then fail a final-entry removal and read the log.
// PREVENTS: a doctor warning, a log warning and a report-bus error for one failure, and a
// fallback fix (dmesg) that only diagnoses.
func TestForwardDefaultFailureIsAnErrorOnEverySurface(t *testing.T) {
	var answer func(*ikeEngineState) []rpc.DoctorCheckDiagnostic
	for _, check := range engineDoctorChecks {
		if check.decl.Name == "ipsec-forward-default" {
			answer = check.answer
		}
	}
	if answer == nil {
		t.Fatal("the live forwarding-default check is not registered")
	}
	dp := &forwardDefaultDoctorDP{spdDP: newSPDDP(), enabled: true, failure: errors.New("test: netlink dump refused")}
	loadESPFormBackend(t, dp)
	findings := answer(nil)
	if len(findings) != 1 || findings[0].Severity != severityError || findings[0].Fix != spdInstallFixRestart() ||
		!strings.Contains(findings[0].Message, "netlink dump refused") {
		t.Fatalf("a plain backend failure is not an error finding with the restart fix: %+v", findings)
	}

	removal := &forwardDefaultTestDP{spdDP: newSPDDP(), enabled: true}
	if err := installUnmatched(removal, dataplane.SPActionDiscard, slogutil.DiscardLogger()); err != nil {
		t.Fatal(err)
	}
	removal.removeErr = fmt.Errorf("test: final entry is busy")
	var logged bytes.Buffer
	removeUnmatched(removal, slog.New(slog.NewTextHandler(&logged, nil)), nil)
	out := logged.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "code="+diagnostic.CodeDoctorIPsecForwardDefault) {
		t.Fatalf("the failed removal is not logged at Error under its doctor code:\n%s", out)
	}
	if strings.Contains(out, "level=WARN") {
		t.Fatalf("the failed removal is logged at Warn as well:\n%s", out)
	}
}

// VALIDATES: a final-entry removal that fails keeps the namespace FWD default claimed,
// raises one report-bus error naming the subject, the cause and a sudo fix, and the next
// removal that succeeds releases the default without raising another.
// METHOD: install the discard catch-all, fail its removal once and read the raised
// issues and the release count, then remove it again with the failure cleared.
// PREVENTS: a default released while the catch-all still stands, a failure raised once
// per retry, and a removal never retried after a failure.
func TestForwardDefaultRemovalFailureRaisesOneEventAndRetries(t *testing.T) {
	dp := &forwardDefaultTestDP{spdDP: newSPDDP(), enabled: true}
	if err := installUnmatched(dp, dataplane.SPActionDiscard, slogutil.DiscardLogger()); err != nil {
		t.Fatal(err)
	}
	raiser := &espFormRecordedIssues{}
	dp.removeErr = fmt.Errorf("test: final entry is busy")
	removeUnmatched(dp, slogutil.DiscardLogger(), raiser)
	if len(raiser.issues) != 1 || dp.releases != 0 {
		t.Fatalf("failed removal: issues=%d releases=%d", len(raiser.issues), dp.releases)
	}
	issue := raiser.issues[0]
	if issue.Code != diagnostic.CodeDoctorIPsecForwardDefault || issue.Severity != rpc.ReportError ||
		issue.Subject != "namespace-forward-default" || !strings.HasPrefix(issue.Fix, "sudo ") ||
		!strings.Contains(issue.Message, "retains the namespace") {
		t.Fatalf("restoration event lost its cause or fix: %+v", issue)
	}
	dp.removeErr = nil
	removeUnmatched(dp, slogutil.DiscardLogger(), raiser)
	if dp.releases != 1 || len(raiser.issues) != 1 {
		t.Fatalf("retry: releases=%d issues=%d", dp.releases, len(raiser.issues))
	}
}

type forwardDefaultCloseConn struct {
	net.Conn
	closed atomic.Bool
}

func (c *forwardDefaultCloseConn) Close() error { c.closed.Store(true); return c.Conn.Close() }

type forwardDefaultCloseDP struct {
	*spdDP
	close func() error
}

func (d *forwardDefaultCloseDP) Close() error { return d.close() }

// VALIDATES: the actual runEngine error exit releases dataplane state before it closes
// its SDK connection. A broken peer cannot receive a report, but must not change this
// ownership order; the connected clean exit then retains its reporting opportunity.
// METHOD: run the engine over a registered backend whose Close records whether the SDK
// connection was already closed, with the SDK's far end closed so the run exits.
// PREVENTS: an SDK closed before the dataplane release, so a release failure can no
// longer be reported.
func TestForwardDefaultEngineCleanupKeepsSDKUntilDataplaneRelease(t *testing.T) {
	ours, theirs := net.Pipe()
	if err := theirs.Close(); err != nil {
		t.Fatal(err)
	}
	conn := &forwardDefaultCloseConn{Conn: ours}
	backendClosed, earlySDKClose := false, false
	dp := &forwardDefaultCloseDP{spdDP: newSPDDP(), close: func() error { backendClosed = true; earlySDKClose = conn.closed.Load(); return nil }}
	name := fmt.Sprintf("test-forward-default-sdk-close-%d", failClosedBackendSeq.Add(1))
	if err := dataplane.Register(name, func() (dataplane.Dataplane, error) { return dp, nil }); err != nil {
		t.Fatal(err)
	}
	origName, origLog := ikeDataplaneFn, loggerPtr.Load()
	ikeDataplaneFn = func() string { return name }
	setLogger(slogutil.DiscardLogger())
	t.Cleanup(func() {
		ikeDataplaneFn = origName
		loggerPtr.Store(origLog)
		activeTablePtr.Store(nil)
		setActivePeers(nil)
		reEstablishFn.Store(nil)
		_ = dataplane.CloseBackend()
	})
	if code := runEngine(conn); code != 1 {
		t.Fatalf("broken SDK startup exit=%d, want1", code)
	}
	if !backendClosed || earlySDKClose || !conn.closed.Load() {
		t.Fatalf("cleanup order: backendClosed=%v SDKClosedBeforeBackend=%v SDKClosedAtEnd=%v", backendClosed, earlySDKClose, conn.closed.Load())
	}
}
