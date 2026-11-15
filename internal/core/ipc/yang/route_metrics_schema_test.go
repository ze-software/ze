// Design: docs/architecture/api/process-protocol.md -- prospective route metric schema
package yang

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The prospective query must expose the complete existing route wire value,
// including policy and forwarding fields that a later producer can supply.
func TestRouteMetricsProspectiveSchemaMatchesWireTypes(t *testing.T) {
	statements, err := goyang.Parse(ZePluginEngineYANG, "ze-plugin-engine.yang")
	require.NoError(t, err)
	module := identitySchemaChild(t, statements, "module", "ze-plugin-engine")
	operation := identitySchemaChild(t, module.SubStatements(), "rpc", "route-metrics")
	input := identitySchemaChild(t, operation.SubStatements(), "input", "")
	proposals := identitySchemaChild(t, input.SubStatements(), "list", "prospective-routes")
	identitySchemaChild(t, proposals.SubStatements(), "max-elements", strconv.Itoa(rpc.RouteMetricsAddressMax))
	identitySchemaChild(t, proposals.SubStatements(), "uses", "route-install-entry")

	for _, contract := range []struct {
		group string
		wire  reflect.Type
	}{
		{"route-install-entry", reflect.TypeOf(rpc.RouteInstallEntry{})},
		{"route-install-next-hop", reflect.TypeOf(rpc.RouteNextHop{})},
	} {
		group := identitySchemaChild(t, module.SubStatements(), "grouping", contract.group)
		fields := routeMetricsSchemaFields(t, module, group)
		var expected []string
		for index := range contract.wire.NumField() {
			name := strings.Split(contract.wire.Field(index).Tag.Get("json"), ",")[0]
			require.NotEmpty(t, name, "the wire field needs an explicit JSON name")
			expected = append(expected, name)
		}
		require.ElementsMatch(t, expected, fields, "%s must describe every field of %s", contract.group, contract.wire.Name())
	}
}

// These built-in groupings expand one level of uses into leaf-only identity and
// next-hop groups. Refuse another nested uses instead of silently omitting fields.
func routeMetricsSchemaFields(t *testing.T, module, group *goyang.Statement) []string {
	t.Helper()
	var fields []string
	appendField := func(statement *goyang.Statement) {
		if statement.Keyword == "leaf" {
			fields = append(fields, statement.Argument)
		}
		if statement.Keyword == "leaf-list" {
			fields = append(fields, statement.Argument)
		}
		if statement.Keyword == "list" {
			fields = append(fields, statement.Argument)
		}
	}
	for _, statement := range group.SubStatements() {
		if statement.Keyword != "uses" {
			appendField(statement)
			continue
		}
		used := identitySchemaChild(t, module.SubStatements(), "grouping", statement.Argument)
		for _, member := range used.SubStatements() {
			require.NotEqual(t, "uses", member.Keyword, "the contract helper must explicitly handle deeper expansion")
			appendField(member)
		}
	}
	return fields
}
