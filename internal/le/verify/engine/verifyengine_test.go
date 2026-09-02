// VALIDATES: the native pre-commit orchestrator preserves the complete ordered
// stage population, structured results, logs, status, failure propagation, and
// interruption semantics of the current producer.
// PREVENTS: a missing registration or empty injected answer certifying a commit.
package verifyengine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFullStagesMatchesNativeActionPopulation(t *testing.T) {
	want := []string{
		"verify lint/run", "tier/check", "rfc/check", "iface-resolution",
		"plugin boundary/check", "config coercion/check", "fs-persistence/check",
		"dash-stdio/check", "port-defaults/check", "config claims",
		"test-sensitivity/check", "test-weakened/check",
		"staticcheck-feature-matrix/check/part/1/of/6",
		"staticcheck-feature-matrix/check/part/2/of/6",
		"staticcheck-feature-matrix/check/part/3/of/6",
		"staticcheck-feature-matrix/check/part/4/of/6",
		"staticcheck-feature-matrix/check/part/5/of/6",
		"staticcheck-feature-matrix/check/part/6/of/6",
		"repository tracked-build/check", "platform-vet/darwin/freebsd", "doc wiring",
		"doc check/verify", "doc check/links", "repository/tree-check",
		"plugin imports/check", "yang glue/check", "feature-tags/check",
		"doc check/templ-output", "vendor-web/check", "web-assets/check",
		"docs-to-code/index-check", "rules/render-check", "rules/index-check",
		"rules/condensed-check", "rules/lint", "arch-map/check",
		"discovery-index/check", "test-health/check", "site facts/check",
		"htmx-upgrade/check", "verify deps/evidence-vet", "hook-check/unit",
		"verify deps/vulnerability", "verify deps/unit-cached",
		"verify deps/unit-race-changed", "verify deps/alloc", "functional/gating",
		"functional/exabgp-test",
	}
	stages := fullStages()
	if len(stages) != len(want) {
		t.Fatalf("full stage population = %d, want %d", len(stages), len(want))
	}
	seen := make(map[string]bool, len(stages))
	for index, current := range stages {
		if current.Identity.Name != want[index] {
			t.Errorf("stage %d = %q, want %q", index, current.Identity.Name, want[index])
		}
		if seen[current.Identity.Name] {
			t.Errorf("stage %q appears twice", current.Identity.Name)
		}
		seen[current.Identity.Name] = true
		derived := stage(current.Identity.Command, current.Identity.Args...).Identity
		if !reflect.DeepEqual(current.Identity, derived) {
			t.Errorf("%s identity = %#v, want %#v", current.Identity.Name, current.Identity, derived)
		}
	}
}

func TestEveryActionStageCarriesExplicitArgsOrIsBare(t *testing.T) {
	bare := map[string]bool{
		"iface-resolution": true,
		"config claims":    true,
		"doc wiring":       true,
	}
	for _, current := range fullStages() {
		if len(current.Identity.Args) != 0 || bare[current.Identity.Name] {
			continue
		}
		t.Errorf("%s names action root %q without explicit args",
			current.Identity.Name, current.Identity.Command)
	}
}

func TestRunCallsEveryRegisteredActionInOrderAndWritesStatus(t *testing.T) {
	root := t.TempDir()
	var called []Identity
	runner := func(_ context.Context, gotRoot string, identity Identity) ActionResult {
		if gotRoot != root {
			t.Fatalf("runner root = %q, want %q", gotRoot, root)
		}
		called = append(called, identity)
		return ActionResult{Identity: identity, Registered: true, Completed: true, Output: "native answer"}
	}

	report := run(context.Background(), root, "0123456789abcdef", runner, func() time.Time {
		return time.Date(2026, 8, 27, 10, 11, 12, 0, time.UTC)
	})

	if report.Code != 0 || !report.Completed || report.Failure != nil {
		t.Fatalf("report = %#v", report)
	}
	want := fullStages()
	if len(called) != len(want) {
		t.Fatalf("called %d stages, want %d", len(called), len(want))
	}
	for index := range want {
		if !reflect.DeepEqual(called[index], want[index].Identity) {
			t.Fatalf("call %d = %#v, want %#v", index, called[index], want[index].Identity)
		}
	}
	status, err := os.ReadFile(filepath.Join(root, "tmp", "ze-verify.status"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"exit=0", "mode=full", "git_sha=0123456789abcdef"} {
		if !strings.Contains(string(status), line) {
			t.Errorf("status lacks %q: %s", line, status)
		}
	}
}

func TestRunCertificateRecordsSkippedSuitesAsPartialEvidence(t *testing.T) {
	t.Setenv("ZE_SKIP_SUITES", "firewall,web")
	root := t.TempDir()
	runner := func(_ context.Context, _ string, identity Identity) ActionResult {
		return ActionResult{Identity: identity, Registered: true, Completed: true}
	}
	report := Run(context.Background(), root, "abc", runner)
	if report.Code != 0 || !report.Completed {
		t.Fatalf("run report = %#v", report)
	}
	certificate, err := ReadCertificate(root)
	if err != nil {
		t.Fatal(err)
	}
	if certificate.Skipped != "firewall,web" {
		t.Fatalf("certificate skipped = %q", certificate.Skipped)
	}
	freshness := CheckCertificate(root, nil)
	if freshness.Fresh || !strings.Contains(freshness.Reason, "skipped suites (firewall,web)") {
		t.Fatalf("partial certificate freshness = %#v", freshness)
	}
}

func TestAStageFailureIsStructuredLoggedAndDoesNotHideLaterStages(t *testing.T) {
	root := t.TempDir()
	var calls int
	runner := func(_ context.Context, _ string, identity Identity) ActionResult {
		calls++
		result := ActionResult{Identity: identity, Registered: true, Completed: true, Output: "output from " + identity.Name}
		if calls == 2 {
			result.Code = 37
		}
		return result
	}

	report := Run(context.Background(), root, "abc", runner)
	if report.Code != 1 || calls != len(fullStages()) {
		t.Fatalf("code=%d calls=%d, want code 1 and %d calls", report.Code, calls, len(fullStages()))
	}
	failed := report.Stages[1]
	if failed.Code != 37 || failed.Failure == nil || failed.Failure.Kind != "stage-failed" {
		t.Fatalf("failed stage = %#v", failed)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(failed.Log)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "output from "+failed.Identity.Name) {
		t.Fatalf("stage log did not preserve runner output: %s", content)
	}
	if !strings.Contains(string(content), "Command: tier check") {
		t.Fatalf("stage log omitted native action args: %s", content)
	}
}

func TestEmptyAndUnregisteredAnswersFailClosed(t *testing.T) {
	for _, test := range []struct {
		name string
		run  ActionRunner
		kind string
	}{
		{name: "nil runner", run: nil, kind: "unregistered"},
		{name: "zero result", run: func(context.Context, string, Identity) ActionResult { return ActionResult{} }, kind: "unregistered"},
		{name: "registered without result", run: func(_ context.Context, _ string, identity Identity) ActionResult {
			return ActionResult{Identity: identity, Registered: true}
		}, kind: "empty-result"},
		{name: "missing action args", run: func(_ context.Context, _ string, identity Identity) ActionResult {
			identity.Args = nil
			return ActionResult{Identity: identity, Registered: true, Completed: true}
		}, kind: "identity-mismatch"},
		{name: "wrong action args", run: func(_ context.Context, _ string, identity Identity) ActionResult {
			identity.Args = append([]string{}, identity.Args...)
			identity.Args = append(identity.Args, "wrong-action")
			return ActionResult{Identity: identity, Registered: true, Completed: true}
		}, kind: "identity-mismatch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := Run(context.Background(), t.TempDir(), "abc", test.run)
			if report.Code != 2 || report.Completed || len(report.Stages) != 1 {
				t.Fatalf("report = %#v", report)
			}
			if report.Failure == nil || report.Failure.Kind != test.kind {
				t.Fatalf("failure = %#v, want kind %q", report.Failure, test.kind)
			}
		})
	}
}

func TestInterruptionStopsBeforeTheNextStageAndRecordsStatus(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	runner := func(_ context.Context, _ string, identity Identity) ActionResult {
		calls++
		cancel()
		return ActionResult{Identity: identity, Registered: true, Completed: true}
	}

	report := Run(ctx, root, "abc", runner)
	if report.Code != Interrupted || report.Completed || calls != 1 {
		t.Fatalf("report=%#v calls=%d", report, calls)
	}
	if report.Failure == nil || report.Failure.Kind != "interrupted" {
		t.Fatalf("failure = %#v", report.Failure)
	}
	status, err := os.ReadFile(filepath.Join(root, "tmp", "ze-verify.status"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(status), "exit=130") {
		t.Fatalf("interrupted status = %s", status)
	}
}
