package doctor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestSchemaStopRowsNameTheCodeTheCauseAndTheFix proves the last-stop check
// reports a start refused for a YANG module an external plugin declared under the
// schema codes: the refused module under doctor-plugin-schema-refused and the
// plugin that gave no schema under doctor-plugin-schema-undeclared, each with
// the time, the refusal's own text and the command the refusal named. An entry
// that kept no command points at ze's log up to the refusal.
//
// VALIDATES: AC-14 of spec-plugin-external-yang-at-startup: `ze doctor`, run
// with no daemon after the refused start, names the plugin, the cause and the
// fix.
// PREVENTS: the entries falling to the default arm and reading as a fatal
// plugin stop, under a code that says nothing of the config schema.
func TestSchemaStopRowsNameTheCodeTheCauseAndTheFix(t *testing.T) {
	at := time.Date(2026, 11, 17, 9, 30, 0, 0, time.UTC)
	record := server.LastStop{Entries: []server.LastStopEntry{
		{
			Reason: server.LastStopSchemaRefused, Time: at,
			Detail: `plugin "broken-module" declared YANG module "acme-broken-conf", which does not compile. ze cannot parse a config against it`,
			Fix:    `ze show plugin conformance run "./broken"`,
		},
		{
			Reason: server.LastStopSchemaUndeclared, Plugin: "no-schema", Time: at,
			Detail: `external plugin "no-schema" declared no YANG module, so it defines no config section`,
			Fix:    `ze show plugin conformance run "./no-schema-plugin"`,
		},
		{Reason: server.LastStopSchemaRefused, Time: at, Detail: "resolve failed"},
	}}

	rows := fatalStopRows(&record)
	require.Len(t, rows, 3)

	refused := rows[0]
	assert.Equal(t, server.CodeDoctorPluginSchemaRefused, refused.Code)
	assert.Equal(t, diagnostic.SeverityError, refused.Severity)
	assert.Equal(t, "ze refused to start at 2026-11-17T09:30:00Z because it refused a YANG module an external plugin declared: "+
		`plugin "broken-module" declared YANG module "acme-broken-conf", which does not compile. ze cannot parse a config against it`,
		refused.Message)
	assert.Equal(t, `ze show plugin conformance run "./broken"`, refused.Fix)

	undeclared := rows[1]
	assert.Equal(t, server.CodeDoctorPluginSchemaUndeclared, undeclared.Code)
	assert.Equal(t, diagnostic.SeverityError, undeclared.Severity)
	assert.Equal(t, "ze refused to start at 2026-11-17T09:30:00Z on a configuration name no schema defines, "+
		`and it held no config schema of external plugin "no-schema", so a section written for that plugin is such a name: `+
		`external plugin "no-schema" declared no YANG module, so it defines no config section`, undeclared.Message)
	assert.Equal(t, `ze show plugin conformance run "./no-schema-plugin"`, undeclared.Fix)

	assert.Equal(t, server.CodeDoctorPluginSchemaRefused, rows[2].Code)
	assert.Equal(t, "journalctl -u ze --until 2026-11-17T09:30:00Z", rows[2].Fix)
}
