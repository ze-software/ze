// Design: docs/architecture/api/process-protocol.md -- accepted redistribution input.
package yang

import (
	goyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"
	"testing"
)

// The existing schema parser observes the public field before any new Go
// producer exists; absence is a behavioral schema failure, not a compile red.
func TestUpdateRouteDeclaresAcceptedOriginIdentity(t *testing.T) {
	statements, err := goyang.Parse(ZePluginEngineYANG, "ze-plugin-engine.yang")
	require.NoError(t, err)
	module := identitySchemaChild(t, statements, "module", "ze-plugin-engine")
	operation := identitySchemaChild(t, module.SubStatements(), "rpc", "update-route")
	input := identitySchemaChild(t, operation.SubStatements(), "input", "")
	identitySchemaChild(t, input.SubStatements(), "leaf", "origin-admission-id")
}
