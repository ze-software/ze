// Design: docs/architecture/api/process-protocol.md -- complete accepted-record lookup schema.
package yang

import (
	"reflect"
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Groupings and nested fields must describe every public value exactly, rather
// than accepting a callback name whose raw accepted source facts are missing.
func TestOriginAdmissionLookupSchemaMatchesPublicValues(t *testing.T) {
	statements, err := goyang.Parse(ZePluginCallbackYANG, "ze-plugin-callback.yang")
	require.NoError(t, err)
	module := identitySchemaChild(t, statements, "module", "ze-plugin-callback")
	identity := identitySchemaChild(t, module.SubStatements(), "grouping", "origin-admission-identity")
	fields := originSchemaFields(t, identity)
	require.ElementsMatch(t, append(fields, "deadline-ms"), originSchemaJSONFields(reflect.TypeFor[rpc.OriginAdmissionLookupInput]()))
	require.ElementsMatch(t, append(fields, "state", "snapshot"), originSchemaJSONFields(reflect.TypeFor[rpc.OriginAdmissionLookupOutput]()))
	snapshot := identitySchemaChild(t, module.SubStatements(), "grouping", "origin-admission-snapshot")
	require.ElementsMatch(t, originSchemaFields(t, snapshot), originSchemaJSONFields(reflect.TypeFor[rpc.OriginAdmissionSnapshot]()))
	engine, err := goyang.Parse(ZePluginEngineYANG, "ze-plugin-engine.yang")
	require.NoError(t, err)
	engineModule := identitySchemaChild(t, engine, "module", "ze-plugin-engine")
	declaration := identitySchemaChild(t, engineModule.SubStatements(), "rpc", "declare-registration")
	input := identitySchemaChild(t, declaration.SubStatements(), "input", "")
	support := identitySchemaChild(t, input.SubStatements(), "leaf", "origin-admission-lookup")
	identitySchemaChild(t, support.SubStatements(), "type", "boolean")
	identitySchemaChild(t, support.SubStatements(), "default", "false")
}
