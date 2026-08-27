// VALIDATES: the native pre-commit orchestrator preserves the complete ordered
// stage population, structured results, logs, status, failure propagation, and
// interruption semantics of the current producer.
// PREVENTS: a missing registration or empty injected answer certifying a commit.
package verify

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFullStagesMatchesCurrentPrecommitPopulation(t *testing.T) {
	want := []string{
		"ze-lint", "ze-tier-check", "ze-rfc-check", "ze-iface-resolution-check",
		"ze-plugin-boundary-check", "ze-config-coercion-check", "ze-fs-persistence-check",
		"ze-dash-stdio-check", "ze-port-defaults-check", "ze-config-claims-check",
		"ze-test-sensitivity-check", "ze-test-weakened-check", "ze-staticcheck-feature-matrix-check",
		"ze-repository-tracked-build-check", "ze-platform-vet", "ze-doc-wiring-check",
		"ze-doc-verify", "ze-doc-links-check", "ze-repository-tree-check",
		"ze-plugin-imports-check", "ze-yang-glue-check", "ze-feature-tags-check",
		"ze-templ-output-check", "ze-vendor-web-check", "ze-web-assets-check",
		"ze-doc-index-check", "ze-rules-render-check", "ze-rules-index-check",
		"ze-rules-condensed-check", "ze-rules-lint", "ze-arch-map-check",
		"ze-discovery-index-check", "ze-test-health-check", "ze-site-facts-check",
		"ze-vendor-web-check", "ze-htmx-upgrade-check", "ze-evidence-vet",
		"ze-unit-hook-test", "ze-dependency-vulnerability-check", "ze-unit-test-cached",
		"ze-unit-test-race-changed", "ze-alloc-check", "ze-functional-test",
		"ze-functional-exabgp-test",
	}
	got := make([]string, 0, len(FullStages()))
	for _, stage := range FullStages() {
		got = append(got, stage.Identity.Gate)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("full stage order = %q, want %q", got, want)
	}
}

func TestRegisteredStageContractsNameExactRootAndActionArgs(t *testing.T) {
	registered := map[string]Identity{
		"ze-lint":                             stage("ze-lint", "verify-lint", "run").Identity,
		"ze-tier-check":                       stage("ze-tier-check", "tier", "check").Identity,
		"ze-rfc-check":                        stage("ze-rfc-check", "rfc", "check").Identity,
		"ze-iface-resolution-check":           stage("ze-iface-resolution-check", "iface-resolution").Identity,
		"ze-plugin-boundary-check":            stage("ze-plugin-boundary-check", "plugin-boundary", "check").Identity,
		"ze-config-coercion-check":            stage("ze-config-coercion-check", "config-coercion", "check").Identity,
		"ze-fs-persistence-check":             stage("ze-fs-persistence-check", "fs-persistence", "check").Identity,
		"ze-dash-stdio-check":                 stage("ze-dash-stdio-check", "dash-stdio", "check").Identity,
		"ze-port-defaults-check":              stage("ze-port-defaults-check", "port-defaults", "check").Identity,
		"ze-config-claims-check":              stage("ze-config-claims-check", "config-claims").Identity,
		"ze-test-sensitivity-check":           stage("ze-test-sensitivity-check", "test-sensitivity", "check").Identity,
		"ze-test-weakened-check":              stage("ze-test-weakened-check", "test-weakened", "check").Identity,
		"ze-staticcheck-feature-matrix-check": stage("ze-staticcheck-feature-matrix-check", "staticcheck-feature-matrix", "check").Identity,
		"ze-repository-tracked-build-check":   stage("ze-repository-tracked-build-check", "repository-tracked-build", "check").Identity,
		"ze-platform-vet":                     stage("ze-platform-vet", "platform-vet", "darwin", "freebsd").Identity,
		"ze-doc-wiring-check":                 stage("ze-doc-wiring-check", "doc-wiring").Identity,
		"ze-doc-verify":                       stage("ze-doc-verify", "doc-check", "verify").Identity,
		"ze-doc-links-check":                  stage("ze-doc-links-check", "doc-check", "links").Identity,
		"ze-repository-tree-check":            stage("ze-repository-tree-check", "repository", "tree-check").Identity,
		"ze-plugin-imports-check":             stage("ze-plugin-imports-check", "plugin-imports", "check").Identity,
		"ze-yang-glue-check":                  stage("ze-yang-glue-check", "yang-glue", "check").Identity,
		"ze-feature-tags-check":               stage("ze-feature-tags-check", "feature-tags", "check").Identity,
		"ze-templ-output-check":               stage("ze-templ-output-check", "doc-check", "templ-output").Identity,
		"ze-web-assets-check":                 stage("ze-web-assets-check", "web-assets", "check").Identity,
		"ze-doc-index-check":                  stage("ze-doc-index-check", "docs-to-code", "ze-doc-index-check").Identity,
		"ze-rules-render-check":               stage("ze-rules-render-check", "rules", "render-check").Identity,
		"ze-rules-index-check":                stage("ze-rules-index-check", "rules", "index-check").Identity,
		"ze-rules-condensed-check":            stage("ze-rules-condensed-check", "rules", "condensed-check").Identity,
		"ze-rules-lint":                       stage("ze-rules-lint", "rules", "lint").Identity,
		"ze-arch-map-check":                   stage("ze-arch-map-check", "arch-map", "check").Identity,
		"ze-discovery-index-check":            stage("ze-discovery-index-check", "discovery-index", "check").Identity,
		"ze-test-health-check":                stage("ze-test-health-check", "test-health", "check").Identity,
		"ze-site-facts-check":                 stage("ze-site-facts-check", "site-facts", "check").Identity,
		"ze-vendor-web-check":                 stage("ze-vendor-web-check", "vendor-web", "check").Identity,
		"ze-htmx-upgrade-check":               stage("ze-htmx-upgrade-check", "htmx-upgrade", "check").Identity,
		"ze-evidence-vet":                     stage("ze-evidence-vet", "verify-deps", "evidence-vet").Identity,
		"ze-unit-hook-test":                   stage("ze-unit-hook-test", "hook-check", "unit").Identity,
		"ze-dependency-vulnerability-check":   stage("ze-dependency-vulnerability-check", "verify-deps", "vulnerability").Identity,
		"ze-unit-test-cached":                 stage("ze-unit-test-cached", "verify-deps", "unit-cached").Identity,
		"ze-unit-test-race-changed":           stage("ze-unit-test-race-changed", "verify-deps", "unit-race-changed").Identity,
		"ze-alloc-check":                      stage("ze-alloc-check", "verify-deps", "alloc").Identity,
		"ze-functional-test":                  stage("ze-functional-test", "functional").Identity,
		"ze-functional-exabgp-test":           stage("ze-functional-exabgp-test", "functional", "exabgp-test").Identity,
	}
	for _, current := range FullStages() {
		want, ok := registered[current.Identity.Gate]
		if !ok {
			t.Fatalf("stage %q has no exact registered native action contract", current.Identity.Gate)
		}
		if !reflect.DeepEqual(current.Identity, want) {
			t.Errorf("%s identity = %#v, want %#v", current.Identity.Gate, current.Identity, want)
		}
	}

}

func TestEveryActionStageCarriesExplicitArgs(t *testing.T) {
	bare := map[string]bool{
		"ze-iface-resolution-check": true,
		"ze-config-claims-check":    true,
		"ze-doc-wiring-check":       true,
		"ze-functional-test":        true,
	}
	stages := FullStages()
	if len(stages) != 44 {
		t.Fatalf("full stage population = %d, want 44", len(stages))
	}
	for _, current := range stages {
		if len(current.Identity.Args) != 0 {
			continue
		}
		if bare[current.Identity.Gate] {
			continue
		}
		t.Errorf("%s names action root %q without explicit args",
			current.Identity.Gate, current.Identity.Command)
	}
}

func TestRunCallsEveryRegisteredGateInOrderAndWritesStatus(t *testing.T) {
	root := t.TempDir()
	var called []Identity
	runner := func(_ context.Context, gotRoot string, identity Identity) GateResult {
		if gotRoot != root {
			t.Fatalf("runner root = %q, want %q", gotRoot, root)
		}
		called = append(called, identity)
		return GateResult{Identity: identity, Registered: true, Completed: true, Output: "native answer"}
	}

	report := run(context.Background(), root, "0123456789abcdef", runner, func() time.Time {
		return time.Date(2026, 8, 27, 10, 11, 12, 0, time.UTC)
	})

	if report.Code != 0 || !report.Completed || report.Failure != nil {
		t.Fatalf("report = %#v", report)
	}
	want := FullStages()
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
	for _, line := range []string{"exit=0", "mode=ze-precommit-verify", "git_sha=0123456789abcdef"} {
		if !strings.Contains(string(status), line) {
			t.Errorf("status lacks %q: %s", line, status)
		}
	}
}

func TestAStageFailureIsStructuredLoggedAndDoesNotHideLaterStages(t *testing.T) {
	root := t.TempDir()
	var calls int
	runner := func(_ context.Context, _ string, identity Identity) GateResult {
		calls++
		result := GateResult{Identity: identity, Registered: true, Completed: true, Output: "output from " + identity.Gate}
		if calls == 2 {
			result.Code = 37
		}
		return result
	}

	report := Run(context.Background(), root, "abc", runner)
	if report.Code != 1 || calls != len(FullStages()) {
		t.Fatalf("code=%d calls=%d, want code 1 and %d calls", report.Code, calls, len(FullStages()))
	}
	failed := report.Stages[1]
	if failed.Code != 37 || failed.Failure == nil || failed.Failure.Kind != "stage-failed" {
		t.Fatalf("failed stage = %#v", failed)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(failed.Log)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "output from "+failed.Identity.Gate) {
		t.Fatalf("stage log did not preserve runner output: %s", content)
	}
	if !strings.Contains(string(content), "Command: tier check") {
		t.Fatalf("stage log omitted native action args: %s", content)
	}
}

func TestEmptyAndUnregisteredAnswersFailClosed(t *testing.T) {
	for _, test := range []struct {
		name string
		run  GateRunner
		kind string
	}{
		{name: "nil runner", run: nil, kind: "unregistered"},
		{name: "zero result", run: func(context.Context, string, Identity) GateResult { return GateResult{} }, kind: "unregistered"},
		{name: "registered without result", run: func(_ context.Context, _ string, identity Identity) GateResult {
			return GateResult{Identity: identity, Registered: true}
		}, kind: "empty-result"},
		{name: "missing action args", run: func(_ context.Context, _ string, identity Identity) GateResult {
			identity.Args = nil
			return GateResult{Identity: identity, Registered: true, Completed: true}
		}, kind: "identity-mismatch"},
		{name: "wrong action args", run: func(_ context.Context, _ string, identity Identity) GateResult {
			identity.Args = append([]string{}, identity.Args...)
			identity.Args = append(identity.Args, "wrong-action")
			return GateResult{Identity: identity, Registered: true, Completed: true}
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
	runner := func(_ context.Context, _ string, identity Identity) GateResult {
		calls++
		cancel()
		return GateResult{Identity: identity, Registered: true, Completed: true}
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
