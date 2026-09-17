// VALIDATES: the Graceful Restart NVS readiness doctor check warns only when the restarter is
// enabled AND the supplied durable state is unavailable or unreadable, and stays silent otherwise
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

	// Restarter enabled + state unreadable -> warn.
	diags := grNVSDiagnostics(enabled, false)
	if assert.Len(t, diags, 1, "restarter enabled + unreadable state must warn") {
		assert.Equal(t, codeOSPFGracefulRestartNVS, diags[0].Code)
	}

	// Readable state is silent; this check never probes by writing.
	assert.Empty(t, grNVSDiagnostics(enabled, true), "readable state is silent")

	// Restarter disabled -> silent regardless of the store.
	assert.Empty(t, grNVSDiagnostics(disabled, false), "GR restarter disabled is silent")
	assert.Empty(t, grNVSDiagnostics(ospfConfig{}, false), "absent OSPF is silent")
	v6Only := ospfConfig{present: true, V6: &enabled}
	diags = grNVSDiagnostics(v6Only, false)
	if assert.Len(t, diags, 1, "an IPv6-only restarter still needs durable state") {
		assert.Equal(t, codeOSPFGracefulRestartNVS, diags[0].Code)
	}
}
