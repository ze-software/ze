// Design: docs/architecture/api/process-protocol.md -- applied event contract.
// Related: ze-plugin-engine.yang -- emit-event remains one engine operation.
package yang

import (
	"reflect"
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The same operation must publish every public field and the conditional
// deadline requirement. Method: parse constraints and load the actual module
// through the native schema loader, then inspect its published RPC metadata.
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
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-plugin-engine.yang", ZePluginEngineYANG))
	schema, err := loader.Resolve()
	require.NoError(t, err)
	operations := configyang.ExtractRPCs(schema, "ze-plugin-engine")
	var published *configyang.RPCMeta
	for index := range operations {
		if operations[index].WireMethod == rpc.MethodEmitEvent {
			published = &operations[index]
		}
	}
	require.NotNil(t, published)
	inputNames := make([]string, 0, len(published.Input))
	for _, field := range published.Input {
		inputNames = append(inputNames, field.Name)
	}
	outputNames := make([]string, 0, len(published.Output))
	for _, field := range published.Output {
		outputNames = append(outputNames, field.Name)
	}
	require.ElementsMatch(t, originSchemaJSONFields(reflect.TypeFor[rpc.EmitEventInput]()), inputNames)
	require.ElementsMatch(t, originSchemaJSONFields(reflect.TypeFor[rpc.EmitEventOutput]()), outputNames)
}
