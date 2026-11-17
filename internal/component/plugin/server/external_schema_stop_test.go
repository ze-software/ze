package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestSchemaStopSurvivesTheRefusedStart proves a start refused for an external
// plugin's config schema leaves the cause where the next `ze doctor` reads it,
// and that a clean start removes it. Method: a refused module and a plugin
// that gave no schema each write an entry of the last-stop record, read back
// with reason, doctor code, plugin, the refusal's problem and why, and its
// fix; a refusal with no operator error keeps its text and no fix; a clean
// start of a server deletes the record.
//
// VALIDATES: AC-14 of spec-plugin-external-yang-at-startup:
// doctor-plugin-schema-refused and doctor-plugin-schema-undeclared outlive the
// process that refused to start.
// PREVENTS: the two codes living only on the report bus of a process that
// exits, so `ze doctor` run after the failed start says nothing of them; and a
// stale row after the plugin was corrected and ze started.
func TestSchemaStopSurvivesTheRefusedStart(t *testing.T) {
	useLastStopStore(t)

	refused := opererr.New(
		`plugin "broken-module" declared YANG module "acme-broken-conf", which does not compile`,
		"ze cannot parse a config against a module with no schema",
		`ze show plugin conformance run "./broken"`)
	require.True(t, RecordRefusedSchemaStop(refused), "the refused module wrote no record")
	cfg := plugin.PluginConfig{Name: "no-schema", Run: "./no-schema-plugin"}
	require.True(t, RecordUndeclaredSchemaStop(cfg.Name, SchemalessRefusal(cfg)), "the schemaless plugin wrote no record")
	require.True(t, RecordRefusedSchemaStop(errors.New("resolve failed")))

	record, found, err := ReadLastStop()
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, record.Entries, 3)

	first := record.Entries[0]
	assert.Equal(t, LastStopSchemaRefused, first.Reason)
	assert.Equal(t, diagnostic.CodeDoctorPluginSchemaRefused, first.Reason.Code())
	assert.Empty(t, first.Plugin, "the join's refusal names its plugins in its own text")
	assert.Equal(t, `plugin "broken-module" declared YANG module "acme-broken-conf", which does not compile. `+
		"ze cannot parse a config against a module with no schema", first.Detail)
	assert.Equal(t, `ze show plugin conformance run "./broken"`, first.Fix)
	assert.False(t, first.Time.IsZero())

	second := record.Entries[1]
	assert.Equal(t, LastStopSchemaUndeclared, second.Reason)
	assert.Equal(t, diagnostic.CodeDoctorPluginSchemaUndeclared, second.Reason.Code())
	assert.Equal(t, "no-schema", second.Plugin)
	assert.Contains(t, second.Detail, `external plugin "no-schema" declared no YANG module`)
	assert.Equal(t, `ze show plugin conformance run "./no-schema-plugin"`, second.Fix)

	plain := record.Entries[2]
	assert.Equal(t, "resolve failed", plain.Detail)
	assert.Empty(t, plain.Fix, "a refusal that names no command keeps none")

	// The stored spelling of each reason reads back as the same reason.
	for _, reason := range []LastStopReason{LastStopSchemaRefused, LastStopSchemaUndeclared} {
		text, marshalErr := reason.MarshalText()
		require.NoError(t, marshalErr)
		var back LastStopReason
		require.NoError(t, back.UnmarshalText(text))
		assert.Equal(t, reason, back)
	}

	clean, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	clean.settleLastStop()
	_, found, err = ReadLastStop()
	require.NoError(t, err)
	assert.False(t, found, "a clean start left the record of the refused start")
}

// TestSchemaStopWithNoStoreReportsNoRecord proves the writer says so when the
// daemon has no store, so no caller reads a record that does not exist.
//
// VALIDATES: the record's writer answers false, never a silent success.
// PREVENTS: a start with no store reported as recorded for the next doctor.
func TestSchemaStopWithNoStoreReportsNoRecord(t *testing.T) {
	assert.False(t, RecordRefusedSchemaStop(errors.New("resolve failed")))
	assert.False(t, RecordUndeclaredSchemaStop("absent", errors.New("no declaration")))
}
