// Design: docs/architecture/config/yang-config-design.md -- publishing the modules external plugins declare
// Related: completer_validate.go -- valueSchema

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestCompleterValueSchemaIsOfTheSetTheCompleterHolds: a completer takes the
// module set once, at NewCompleter, and builds its value schema once from that
// same set. Method: build two completers before a publish, ask the first for
// its value schema before the publish and the second only after it, then build
// a third completer after the publish.
//
// VALIDATES: a completer built after the publish completes and judges under
// the external module's root; a completer built before it keeps the set it
// took, and its value schema is the schema of that set whenever it is first
// asked for.
// PREVENTS: a completer whose nodes come from one module set and whose value
// checks come from another, and a completer built after the publish that
// still misses the published root.
func TestCompleterValueSchemaIsOfTheSetTheCompleterHolds(t *testing.T) {
	const section = "acme-monitor"
	const module = "acme-monitor-conf"
	t.Cleanup(configyang.ResetPublishedModulesForTest())

	early := newTestCompleter(t)
	earlyValues, err := early.valueSchema()
	require.NoError(t, err)
	require.Nil(t, earlyValues.Get(section), "the compiled-in schema already holds the external root")
	late := newTestCompleter(t)

	require.NoError(t, config.PublishExternalModules([]configyang.ExternalModule{{
		Plugin: "acme", Module: module, Namespace: "urn:acme:monitor",
		Text: `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8 { range "1..10"; } }
    }
}`,
	}}))

	after := newTestCompleter(t)
	require.NotNil(t, after.schema.GetModule(module), "a completer built after the publish misses the module")
	afterValues, err := after.valueSchema()
	require.NoError(t, err)
	assert.NotNil(t, afterValues.Get(section), "a completer built after the publish judges without the published root")

	assert.Nil(t, late.schema.GetModule(module), "a completer built before the publish changed the set it holds")
	lateValues, err := late.valueSchema()
	require.NoError(t, err)
	assert.Nil(t, lateValues.Get(section), "a completer holding the earlier set judges with the published set's nodes")

	earlyAgain, err := early.valueSchema()
	require.NoError(t, err)
	assert.Same(t, earlyValues, earlyAgain)
}
