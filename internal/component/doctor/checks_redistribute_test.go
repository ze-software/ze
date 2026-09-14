// Design: docs/guide/redistribution.md -- the redistribute readiness check, driven through the runner
//
// The check and its unit cases live with the config component
// (internal/component/config/doctor_redistribute.go and its test). What stays
// here is the one case that drives `ze doctor` itself, because it is the
// runner it exercises.

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

// TestRunChecksReachesTheRedistributeCheck drives the check from the entry
// point an operator reaches, rather than from the helper.
//
// VALIDATES: `ze doctor` on a config whose redistribution destination is a
// typo reports it, through the registration the config component makes.
// PREVENTS: the defect this test was written for. The check and its unit
// tests once landed in one commit that never added the runner call, so the
// check was dead from the day it was written and every helper-driven test
// stayed green (`ai/rules/evidence.md`). A registration that init() no longer
// makes is the same defect wearing the registry's name.
func TestRunChecksReachesTheRedistributeCheck(t *testing.T) {
	cfgPath := writeTestConfig(t, `redistribute {
	destination ospv3 {
		import connected
	}
}
`)

	assert.Contains(t, codes(runChecks(cfgPath)), "doctor-redistribute-unknown-destination")
}
