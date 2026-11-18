//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- fixture schema publication.
// Related: validate_corpus_test.go -- the native config corpus and shared module publisher.
package cli

import (
	"strings"
	"testing"
)

// TestSweepIncludesDeclaredCommandOutcomeSchema runs the seven actual web
// outcome configurations through the corpus's native validator. Their provider's
// module must retain its declared owner, while an undeclared root still refuses.
func TestSweepIncludesDeclaredCommandOutcomeSchema(t *testing.T) {
	modules := sweepExternalModules()
	matches := 0
	for _, module := range modules {
		if module.Module != "ze-answer-outcome-web-conf" {
			continue
		}
		matches++
		if module.Plugin != "answer-outcome-provider" {
			t.Errorf("outcome module owner = %q, want answer-outcome-provider", module.Plugin)
		}
		if module.Namespace != "urn:ze:test:answer-outcome-web:conf" {
			t.Errorf("outcome module namespace = %q", module.Namespace)
		}
	}
	if matches != 1 {
		t.Errorf("outcome config module declarations = %d, want 1", matches)
	}

	report := runSweep(t, ruleNone, sweepKeep{"test/plugin/command-outcome-web-"})
	if report.zeConfigs != 7 {
		t.Errorf("outcome configurations swept = %d, want all 7", report.zeConfigs)
	}
	for _, finding := range report.findings {
		t.Error(finding)
	}
	for _, root := range report.absentRoots {
		t.Errorf("outcome configuration has an unavailable root: %s", root)
	}

	refused := runValidation("answer-outcome-unregistered { }", "undeclared-outcome.conf")
	if refused.Valid {
		t.Fatal("the fixture module made an unrelated undeclared root valid")
	}
	for _, diagnostic := range refused.Diagnostics {
		if diagnostic.Code != "config-parse" {
			continue
		}
		if strings.Contains(diagnostic.Message, "answer-outcome-unregistered") {
			return
		}
	}
	t.Errorf("undeclared root has no matching parse refusal: %+v", refused.Diagnostics)
}
