// Design: docs/guide/redistribution.md -- the refusal `ze doctor` reports for a redistribute rule
//
// `ze doctor` holds no redistribution check of its own. Every entry point (the
// command, `show doctor`, the support bundle) reaches runChecks, which loads
// the config through config.LoadConfig, and that load refuses an unknown
// source or destination (config.RefuseRedistributeRules). A post-config check
// could only ever meet a tree the load had already accepted, so it was a
// second declaration of the refusal and was removed. What stays here proves
// the refusal reaches the operator through the doctor runner.

package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// codes flattens a diagnostic list to the codes it carries, which is what the
// assertions are about. It lives in this untagged file so every platform's
// tests reach it: a second copy in a linux-tagged file collides on linux and
// costs the whole package its test build.
func codes(diags []diagnostic.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	// Indexed, not ranged by value: Diagnostic is 184 bytes (gocritic
	// rangeValCopy), and only Code is read here.
	for i := range diags {
		out = append(out, diags[i].Code)
	}
	return out
}

// TestRunChecksReportsTheRedistributeRefusal drives `ze doctor` from the entry
// point an operator reaches.
//
// VALIDATES: `ze doctor` on a config whose redistribution destination is a
// typo reports it as an error that names the token, the declared destinations
// and the fix. The config load refuses a destination no protocol declared
// (config.RefuseRedistributeRules), so the report arrives as the config-parse
// diagnostic, before any post-config check runs.
// PREVENTS: an operator who runs `ze doctor` on the typo and is told nothing.
// A redistribution check once landed with its unit tests and no runner call,
// so it was dead from the day it was written while every helper-driven test
// stayed green (`ai/rules/evidence.md`). Only the runner proves the report.
func TestRunChecksReportsTheRedistributeRefusal(t *testing.T) {
	cfgPath := writeTestConfig(t, `redistribute {
	destination ospv3 {
		import connected
	}
}
`)

	diags := runChecks(cfgPath)
	var report *diagnostic.Diagnostic
	for i := range diags {
		if diags[i].Code == "doctor-config-parse" {
			report = &diags[i]
		}
	}
	if !assert.NotNil(t, report, "ze doctor reports the refused destination: %v", codes(diags)) {
		return
	}
	assert.Equal(t, diagnostic.SeverityError, report.Severity)
	assert.Contains(t, report.Message, `"destination ospv3" names no protocol that receives redistributed routes`)
	assert.Contains(t, report.Message, "declared destinations: ")
	assert.Contains(t, report.Message, `fix: rename "ospv3" to one of the declared destinations`)
}
