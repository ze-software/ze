// Design: docs/architecture/api/process-protocol.md -- complete origin callback schema.
package yang

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func originSchemaJSONFields(wire reflect.Type) []string {
	fields := make([]string, 0, wire.NumField())
	for field := range wire.Fields() {
		fields = append(fields, strings.Split(field.Tag.Get("json"), ",")[0])
	}
	return fields
}

func originSchemaFields(t *testing.T, statement *goyang.Statement) []string {
	t.Helper()
	var fields []string
	for _, child := range statement.SubStatements() {
		switch child.Keyword {
		case "leaf", "leaf-list", "container", "list":
			fields = append(fields, child.Argument)
		}
	}
	return fields
}

// This compiles against original B alone. Missing schema is a contract failure,
// not a missing-Go-symbol compilation proxy; nested values/output must stay exact.
func TestOriginIntakeSchemaMatchesPublicValues(t *testing.T) {
	callback, err := goyang.Parse(ZePluginCallbackYANG, "ze-plugin-callback.yang")
	require.NoError(t, err)
	callbackModule := identitySchemaChild(t, callback, "module", "ze-plugin-callback")
	operation := identitySchemaChild(t, callbackModule.SubStatements(), "rpc", "origin-intake")
	identitySchemaChild(t, operation.SubStatements(), "ze:method", rpc.MethodOriginIntake)
	input := identitySchemaChild(t, operation.SubStatements(), "input", "")
	require.ElementsMatch(t, originSchemaJSONFields(reflect.TypeFor[rpc.OriginIntakeInput]()), originSchemaFields(t, input))
	for _, contract := range []struct {
		name string
		wire reflect.Type
	}{
		{"producer", reflect.TypeFor[rpc.OriginProducer]()}, {"source", reflect.TypeFor[rpc.OriginSource]()}, {"route", reflect.TypeFor[rpc.OriginBuild]()},
	} {
		node := identitySchemaChild(t, input.SubStatements(), "container", contract.name)
		require.ElementsMatch(t, originSchemaJSONFields(contract.wire), originSchemaFields(t, node), contract.name)
	}
	for _, name := range []string{"peer-owner", "origin-receipt", "deadline-ms"} {
		leaf := identitySchemaChild(t, input.SubStatements(), "leaf", name)
		identitySchemaChild(t, leaf.SubStatements(), "mandatory", "true")
	}
	output := identitySchemaChild(t, operation.SubStatements(), "output", "")
	identitySchemaChild(t, output.SubStatements(), "uses", "engine:update-route-result")
	engine, err := goyang.Parse(ZePluginEngineYANG, "ze-plugin-engine.yang")
	require.NoError(t, err)
	module := identitySchemaChild(t, engine, "module", "ze-plugin-engine")
	result := identitySchemaChild(t, module.SubStatements(), "grouping", "update-route-result")
	require.ElementsMatch(t, originSchemaJSONFields(reflect.TypeFor[rpc.UpdateRouteOutput]()), originSchemaFields(t, result))
	admissions := identitySchemaChild(t, result.SubStatements(), "list", "admissions")
	require.ElementsMatch(t, originSchemaJSONFields(reflect.TypeFor[rpc.RouteAdmission]()), originSchemaFields(t, admissions))
}

func TestOriginIntakeDeclarationAndUpdateDeadlineAreInSchema(t *testing.T) {
	statements, err := goyang.Parse(ZePluginEngineYANG, "ze-plugin-engine.yang")
	require.NoError(t, err)
	module := identitySchemaChild(t, statements, "module", "ze-plugin-engine")
	registration := identitySchemaChild(t, module.SubStatements(), "rpc", "declare-registration")
	input := identitySchemaChild(t, registration.SubStatements(), "input", "")
	support := identitySchemaChild(t, input.SubStatements(), "leaf", "origin-intake")
	identitySchemaChild(t, support.SubStatements(), "type", "boolean")
	identitySchemaChild(t, support.SubStatements(), "default", "false")
	data, err := json.Marshal(rpc.DeclareRegistrationInput{OriginIntake: true})
	require.NoError(t, err)
	require.Contains(t, string(data), `"origin-intake":true`)
	update := identitySchemaChild(t, module.SubStatements(), "rpc", "update-route")
	updateInput := identitySchemaChild(t, update.SubStatements(), "input", "")
	deadline := identitySchemaChild(t, updateInput.SubStatements(), "leaf", "deadline-ms")
	identitySchemaChild(t, deadline.SubStatements(), "default", "0")
	identitySchemaChild(t, identitySchemaChild(t, deadline.SubStatements(), "type", "int64").SubStatements(), "range", "0..max")
	updateOutput := identitySchemaChild(t, update.SubStatements(), "output", "")
	identitySchemaChild(t, updateOutput.SubStatements(), "uses", "update-route-result")
}
