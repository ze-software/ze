// Design: docs/architecture/api/process-protocol.md -- applied event contract.
// Related: ze-plugin-engine.yang -- emit-event remains one engine operation.
// Related: internal/component/config/yang/emit_event_applied_schema_test.go -- loader RPC publication check.
package yang

import (
	"reflect"
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The same operation must publish every public field and the conditional
// deadline requirement. Method: parse constraints directly from the embedded
// YANG text; the native schema loader check lives beside the loader in
// internal/component/config/yang so internal/core does not import a component.
func TestEmitEventAppliedSchemaContract(t *testing.T) {
	statements, err := goyang.Parse(ZePluginEngineYANG, "ze-plugin-engine.yang")
	require.NoError(t, err)
	module := identitySchemaChild(t, statements, "module", "ze-plugin-engine")
	operation := identitySchemaChild(t, module.SubStatements(), "rpc", "emit-event")
	identitySchemaChild(t, operation.SubStatements(), "ze:method", rpc.MethodEmitEvent)
	input := identitySchemaChild(t, operation.SubStatements(), "input", "")
	output := identitySchemaChild(t, operation.SubStatements(), "output", "")
	require.ElementsMatch(t, originSchemaJSONFields(reflect.TypeFor[rpc.EmitEventInput]()), originSchemaFields(t, input))
	require.ElementsMatch(t, originSchemaJSONFields(reflect.TypeFor[rpc.EmitEventOutput]()), originSchemaFields(t, output))
	for _, contract := range []struct {
		node *goyang.Statement
		mode string
	}{{input, "await-applied"}, {output, "applied"}} {
		mode := identitySchemaChild(t, contract.node.SubStatements(), "leaf", contract.mode)
		identitySchemaChild(t, mode.SubStatements(), "type", "boolean")
		identitySchemaChild(t, mode.SubStatements(), "default", "false")
		identitySchemaChild(t, mode.SubStatements(), "must", ". = 'false' or ../deadline-ms > 0")
		deadline := identitySchemaChild(t, contract.node.SubStatements(), "leaf", "deadline-ms")
		identitySchemaChild(t, deadline.SubStatements(), "type", "int64")
	}
	deadline := identitySchemaChild(t, output.SubStatements(), "leaf", "deadline-ms")
	identitySchemaChild(t, identitySchemaChild(t, deadline.SubStatements(), "type", "int64").SubStatements(), "range", "1..max")
}
