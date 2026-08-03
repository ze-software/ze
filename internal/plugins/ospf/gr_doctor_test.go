// VALIDATES: the Graceful Restart NVS readiness doctor check warns only when the restarter is
// enabled AND the non-volatile restart-fact store is unwritable, and stays silent otherwise
// (spec-ospf-ext-9, ai/rules/repo-maintenance.md).
// PREVENTS: a spurious GR NVS warning when GR is off or the store is fine, or a missed warning
// that a planned restart cannot persist its grace deadline.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOSPFGracefulRestartNVSDoctor(t *testing.T) {
	enabled := ospfConfig{present: true, GracefulRestart: gracefulRestartConfig{present: true, RestarterSupport: grSupportPlanned, RestartInterval: 120}}
	disabled := ospfConfig{present: true, GracefulRestart: gracefulRestartConfig{present: true, RestarterSupport: grSupportDisabled, RestartInterval: 120}}

	// Restarter enabled + store unwritable -> warn.
	diags := grNVSDiagnostics(enabled, false)
	if assert.Len(t, diags, 1, "restarter enabled + unwritable store must warn") {
		assert.Equal(t, codeOSPFGracefulRestartNVS, diags[0].Code)
	}

	// Restarter enabled + store writable -> silent.
	assert.Empty(t, grNVSDiagnostics(enabled, true), "a writable store is silent")

	// Restarter disabled -> silent regardless of the store.
	assert.Empty(t, grNVSDiagnostics(disabled, false), "GR restarter disabled is silent")
	assert.Empty(t, grNVSDiagnostics(ospfConfig{}, false), "absent OSPF is silent")
}
