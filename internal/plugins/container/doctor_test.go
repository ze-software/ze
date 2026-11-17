// Design: docs/architecture/container/runtime.md -- the doctor codes the container plugin owns

package container

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestContainerDiagnosticCodesRegisterFromThePlugin proves every code of the
// plugin's problem-class table resolves in the code registry once the plugin
// is linked, with a title and an explanation, and that each carries the
// prefix every container code shares.
//
// Method: the plugin's init() ran when this package was loaded, so the test
// looks each code up in the registry the doctor reads.
//
// MUTATION: remove the registerContainerDiagnosticCodes call from
// registerContainer. Every lookup then answers nil.
func TestContainerDiagnosticCodesRegisterFromThePlugin(t *testing.T) {
	owned := []string{
		codeNotPrivileged,
		codeCgroupUnavailable,
		codeSealCopiesBinary,
		codeOutOfMemory,
		codeProcessLimit,
		codeStopKilled,
		codeCleanupIncomplete,
		codeFailed,
		codeStopped,
		codeKernelCPULimitUnavailable,
		codeKernelCPULimitUnknown,
	}
	if len(containerDiagnosticCodes) != len(owned) {
		t.Fatalf("the code table holds %d codes, want the %d of the problem-class table", len(containerDiagnosticCodes), len(owned))
	}
	for _, code := range owned {
		meta := diagnostic.Lookup(code)
		if meta == nil {
			t.Errorf("code %s is not in the code registry", code)
			continue
		}
		if !strings.HasPrefix(meta.Code, "doctor-container-") {
			t.Errorf("code %s does not start with doctor-container-", meta.Code)
		}
		if meta.Title == "" {
			t.Errorf("code %s has no title", code)
		}
		if meta.Description == "" {
			t.Errorf("code %s has no explanation", code)
		}
	}
}

// TestContainerDoctorChecksAnswerThroughTheCallback proves the wiring of the
// runtime doctor checks: each is declared in the Stage 1 registration the
// engine entry sends, at the runtime phase, under one registered code, and
// the function the doctor callback is bound to answers it from the
// supervisor's records with a finding that names the container and its fix.
//
// Method: engineRegistration, the value runContainerEngine passes to p.Run,
// and supervisor.answerDoctorCheck, the function it hands p.OnDoctorCheck.
// One problem of each kind is recorded for one of two containers.
//
// MUTATION: drop DoctorChecks from engineRegistration and no check is
// declared; make answerDoctorCheck answer nil findings and no recorded
// problem is reported.
func TestContainerDoctorChecksAnswerThroughTheCallback(t *testing.T) {
	declared := map[string]rpc.DoctorCheckDecl{}
	for _, decl := range engineRegistration().DoctorChecks {
		declared[decl.Name] = decl
	}
	// The image store's checks are declared beside these, and proven by
	// TestStoreDoctorCheckAnswersTheStoresRefusal.
	if len(declared) != len(runtimeDoctorChecks)+len(storeDoctorChecks) {
		t.Fatalf("the Stage 1 registration declares %d doctor checks, want the %d runtime checks and the %d store checks",
			len(declared), len(runtimeDoctorChecks), len(storeDoctorChecks))
	}

	sup := newSupervisor(slogutil.DiscardLogger(), nil, nil, nil)
	both := configOf(instanceConfig{name: "db", enabled: true}, instanceConfig{name: "web", enabled: true})
	if err := sup.install(both); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	wantFix := map[problemKind]string{
		problemOutOfMemory:       "set container instance web memory MEBIBYTES",
		problemProcessLimit:      "set container instance web process-limit COUNT",
		problemStopKilled:        "set container instance web stop-timeout SECONDS",
		problemCleanupIncomplete: "ze request container restart name web",
		problemFailed:            "ze request container restart name web",
		problemStopped:           "ze request container restart name web",
	}
	if len(wantFix) != len(runtimeDoctorChecks) {
		t.Fatalf("the test names %d fixes for %d runtime checks", len(wantFix), len(runtimeDoctorChecks))
	}
	// A container that runs, or ran, is a warning; one the config enables
	// and that does not run is an error.
	wantSeverity := map[problemKind]string{
		problemOutOfMemory:       severityWarning,
		problemProcessLimit:      severityWarning,
		problemStopKilled:        severityWarning,
		problemCleanupIncomplete: severityWarning,
		problemFailed:            severityError,
		problemStopped:           severityWarning,
	}
	if len(wantSeverity) != len(runtimeDoctorChecks) {
		t.Fatalf("the test names %d severities for %d runtime checks", len(wantSeverity), len(runtimeDoctorChecks))
	}

	for i := range runtimeDoctorChecks {
		check := runtimeDoctorChecks[i]
		name := check.decl.Name

		decl, isDeclared := declared[name]
		if !isDeclared {
			t.Fatalf("check %s is not declared in the Stage 1 registration", name)
		}
		if decl.Phase != rpc.DoctorPhaseRuntime {
			t.Errorf("check %s is declared at phase %s, want %s", name, decl.Phase, rpc.DoctorPhaseRuntime)
		}
		if len(decl.Codes) != 1 {
			t.Fatalf("check %s declares codes %v, want one", name, decl.Codes)
		}
		code := decl.Codes[0]
		if diagnostic.Lookup(code) == nil {
			t.Errorf("check %s declares code %s, which is not in the code registry", name, code)
		}

		healthy, err := sup.answerDoctorCheck(name)
		if err != nil {
			t.Fatalf("check %s refused with no problem recorded: %v", name, err)
		}
		if len(healthy) != 0 {
			t.Fatalf("check %s answered %v with no problem recorded, want no finding", name, healthy)
		}

		if err := sup.recordProblem("web", check.kind); err != nil {
			t.Fatalf("recording a problem for a configured container failed: %v", err)
		}
		findings, err := sup.answerDoctorCheck(name)
		if err != nil {
			t.Fatalf("check %s refused: %v", name, err)
		}
		if len(findings) != 1 {
			t.Fatalf("check %s answered %d findings for one recorded problem: %v", name, len(findings), findings)
		}
		finding := findings[0]
		if finding.Code != code {
			t.Errorf("check %s answered code %s, want %s", name, finding.Code, code)
		}
		if finding.Severity != wantSeverity[check.kind] {
			t.Errorf("check %s answered severity %q, want %q", name, finding.Severity, wantSeverity[check.kind])
		}
		if !strings.HasPrefix(finding.Message, "container web ") {
			t.Errorf("check %s answered message %q, which does not open with the container's name", name, finding.Message)
		}
		if finding.Fix != wantFix[check.kind] {
			t.Errorf("check %s answered fix %q, want %q", name, finding.Fix, wantFix[check.kind])
		}
	}

	// Each check reports its own kind only: every kind is recorded for web
	// now, and each check still answers one finding.
	for i := range runtimeDoctorChecks {
		name := runtimeDoctorChecks[i].decl.Name
		findings, err := sup.answerDoctorCheck(name)
		if err != nil {
			t.Fatalf("check %s refused: %v", name, err)
		}
		if len(findings) != 1 {
			t.Errorf("check %s answered %d findings with one problem of each kind recorded, want 1", name, len(findings))
		}
	}

	if _, err := sup.answerDoctorCheck("container-undeclared"); err == nil {
		t.Error("a check name the plugin did not declare was answered, want a refusal")
	}
	if err := sup.recordProblem("absent", problemOutOfMemory); err == nil {
		t.Error("a problem was recorded for a container the config does not hold, want a refusal")
	}

	// A container that leaves the config takes its records with it.
	if err := sup.install(configOf(instanceConfig{name: "db", enabled: true})); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	for i := range runtimeDoctorChecks {
		name := runtimeDoctorChecks[i].decl.Name
		findings, err := sup.answerDoctorCheck(name)
		if err != nil {
			t.Fatalf("check %s refused: %v", name, err)
		}
		if len(findings) != 0 {
			t.Errorf("check %s still answers %v after its container left the config", name, findings)
		}
	}
}

// refusingLibrary is a runtime library that refuses every create with
// refusal while it is set, and creates through fakeLibrary once it is nil.
type refusingLibrary struct {
	mu      sync.Mutex
	refusal error
	fake    *fakeLibrary
}

func (l *refusingLibrary) setRefusal(refusal error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refusal = refusal
}

func (l *refusingLibrary) sweep() []leftover { return l.fake.sweep() }

func (l *refusingLibrary) create(ctx context.Context, request *startRequest) (createdContainer, error) {
	l.mu.Lock()
	refusal := l.refusal
	l.mu.Unlock()
	if refusal != nil {
		return nil, refusal
	}
	return l.fake.create(ctx, request)
}

// TestFailedStartIsReportedWhileTheContainerIsFailed proves the failed
// record: a start that fails logs its Warn line under the failed code, and
// the container-failed check reports the container, as an error, with the
// step that failed, its cause and the refusal's own fix, for exactly as long
// as the container holds the failed state. A later start that runs, a config
// that disables the container and a config that removes it each clear it.
//
// Method: the worker's own step, converge, drives the start, over a library
// that refuses the create while the test sets a refusal.
//
// MUTATION: drop the record from setFailed and the check answers no
// finding; drop its removal from setState and the finding outlives the
// running start; drop the kind test from install and it outlives the
// disable.
func TestFailedStartIsReportedWhileTheContainerIsFailed(t *testing.T) {
	const check = "container-failed"
	ctx := context.Background()
	refusal := opererr.Wrap(errors.New("clone3: operation not permitted"),
		"container web: its init did not start",
		"the kernel refused the new namespaces",
		"ze show doctor",
	)
	library := &refusingLibrary{refusal: refusal, fake: &fakeLibrary{events: &eventLog{}, initProcess: 4242}}
	store := &fakeStore{}
	store.setAnswer(readyImage())
	var logged bytes.Buffer
	sup := newSupervisor(slog.New(slog.NewTextHandler(&logged, nil)), library, storeFactoryOf(store), nil)
	both := configOf(instanceConfig{name: "db", enabled: true}, instanceConfig{name: "web", enabled: true})
	if err := sup.install(both); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	sup.converge("web")

	if !strings.Contains(logged.String(), "code="+codeFailed) {
		t.Errorf("the start failure logged %q, want its line under code=%s", logged.String(), codeFailed)
	}
	if state, known := sup.state("web"); !known || state != StateFailed {
		t.Fatalf("the container holds state %s (known=%v) after a refused create, want %s", state, known, StateFailed)
	}
	findings, err := sup.answerDoctorCheck(check)
	if err != nil {
		t.Fatalf("check %s refused: %v", check, err)
	}
	if len(findings) != 1 {
		t.Fatalf("check %s answered %d findings for one failed container: %v", check, len(findings), findings)
	}
	finding := findings[0]
	if finding.Code != codeFailed {
		t.Errorf("the finding has code %s, want %s", finding.Code, codeFailed)
	}
	if finding.Severity != severityError {
		t.Errorf("the finding has severity %s, want %s", finding.Severity, severityError)
	}
	for _, part := range []string{"container web cannot run", refusal.Problem(), refusal.Why(), "clone3: operation not permitted"} {
		if !strings.Contains(finding.Message, part) {
			t.Errorf("the finding %q does not name %q", finding.Message, part)
		}
	}
	if finding.Fix != refusal.Fix() {
		t.Errorf("the finding's fix is %q, want the refusal's own %q", finding.Fix, refusal.Fix())
	}

	library.setRefusal(nil)
	sup.converge("web")

	if state, _ := sup.state("web"); state != StateRunning {
		t.Fatalf("the container holds state %s after a start that ran, want %s", state, StateRunning)
	}
	if findings, _ := sup.answerDoctorCheck(check); len(findings) != 0 {
		t.Errorf("check %s still answers %v after a start that ran", check, findings)
	}
	if err := sup.stop(ctx, "web"); err != nil {
		t.Fatalf("the stop failed: %v", err)
	}

	library.setRefusal(refusal)
	sup.converge("db")
	dbFindings, _ := sup.answerDoctorCheck(check)
	if len(dbFindings) != 1 {
		t.Fatalf("check %s answered %v after db failed, want one finding", check, dbFindings)
	}
	if !strings.HasPrefix(dbFindings[0].Message, "container db ") {
		t.Errorf("check %s answered %q after db failed, want db's finding", check, dbFindings[0].Message)
	}
	disabled := configOf(instanceConfig{name: "db", enabled: false}, instanceConfig{name: "web", enabled: true})
	if err := sup.install(disabled); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if findings, _ := sup.answerDoctorCheck(check); len(findings) != 0 {
		t.Errorf("check %s still answers %v after the config disabled the container", check, findings)
	}

	if err := sup.install(both); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	sup.converge("db")
	if findings, _ := sup.answerDoctorCheck(check); len(findings) != 1 {
		t.Fatalf("check %s answered %v after db failed again, want one finding", check, findings)
	}
	if err := sup.install(configOf(instanceConfig{name: "web", enabled: true})); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if findings, _ := sup.answerDoctorCheck(check); len(findings) != 0 {
		t.Errorf("check %s still answers %v after the container left the config", check, findings)
	}
}

// TestFailureDetailFindsTheRefusalOrQuotesTheError proves the detail of a
// failed start reads the operator error through a join, as the library's
// create answers it, and that an error that is none is quoted whole with the
// restart command as its fix rather than answered empty.
func TestFailureDetailFindsTheRefusalOrQuotesTheError(t *testing.T) {
	refusal := opererr.New("container web: the runtime library refused its description", "the description is wrong", "ze show doctor")
	joined := errors.Join(refusal, errors.New("the root mount could not be removed"))

	detail := failureDetail("web", joined)

	if detail.cause != refusal.Problem()+"; "+refusal.Why() {
		t.Errorf("the cause is %q, want the refusal's problem and why", detail.cause)
	}
	if detail.fix != refusal.Fix() {
		t.Errorf("the fix is %q, want the refusal's %q", detail.fix, refusal.Fix())
	}

	plain := failureDetail("web", errors.New("an answer of no known form"))

	if plain.cause != "an answer of no known form" {
		t.Errorf("the cause of a plain error is %q, want the error's whole text", plain.cause)
	}
	if plain.fix != "ze request container restart name web" {
		t.Errorf("the fix of a plain error is %q, want the restart command", plain.fix)
	}
}

// TestHostCodesAreMarkedAsHostVerdicts proves the three codes that judge the
// host, never the configuration, say so: a tool that validates a config
// written for another machine reads the mark to tell the two apart.
func TestHostCodesAreMarkedAsHostVerdicts(t *testing.T) {
	host := map[string]bool{
		codeNotPrivileged:     true,
		codeCgroupUnavailable: true,
		codeSealCopiesBinary:  true,
	}
	for _, meta := range containerDiagnosticCodes {
		if meta.HostVerdict != host[meta.Code] {
			t.Errorf("code %s has HostVerdict %v, want %v", meta.Code, meta.HostVerdict, host[meta.Code])
		}
	}
}
