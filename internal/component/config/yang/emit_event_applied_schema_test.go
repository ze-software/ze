// Design: docs/architecture/api/process-protocol.md -- applied event RPC publication contract.
// Related: internal/core/ipc/yang/emit_event_applied_schema_test.go -- YANG constraint assertions.
//
// This test lives beside the loader rather than beside the IPC modules
// because internal/core is the leaf tier and does not import a component.
package yang_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	ipcyang "github.com/ze-software/ze/internal/core/ipc/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func emitEventJSONFields(wire reflect.Type) []string {
	fields := make([]string, 0, wire.NumField())
	for field := range wire.Fields() {
		fields = append(fields, strings.Split(field.Tag.Get("json"), ",")[0])
	}
	return fields
}

// TestEmitEventAppliedPublishedRPCContract proves the native schema loader
// publishes every public input and output field of emit-event from
// ze-plugin-engine.yang.
func TestEmitEventAppliedPublishedRPCContract(t *testing.T) {
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-plugin-engine.yang", ipcyang.ZePluginEngineYANG))
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
	require.ElementsMatch(t, emitEventJSONFields(reflect.TypeFor[rpc.EmitEventInput]()), inputNames)
	require.ElementsMatch(t, emitEventJSONFields(reflect.TypeFor[rpc.EmitEventOutput]()), outputNames)
}
