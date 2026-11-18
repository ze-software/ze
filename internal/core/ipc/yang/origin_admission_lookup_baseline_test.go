// Design: docs/architecture/api/process-protocol.md -- original accepted-record callback schema.
package yang

import (
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"
)

// This uses only the existing YANG parser and schema helpers. Missing callback
// registration must be a behavioral failure before new public Go symbols exist.
func TestOriginAdmissionLookupCallbackIsDeclared(t *testing.T) {
	statements, err := goyang.Parse(ZePluginCallbackYANG, "ze-plugin-callback.yang")
	require.NoError(t, err)
	module := identitySchemaChild(t, statements, "module", "ze-plugin-callback")
	operation := identitySchemaChild(t, module.SubStatements(), "rpc", "origin-admission-lookup")
	identitySchemaChild(t, operation.SubStatements(), "ze:method", "ze-plugin-callback:origin-admission-lookup")
	input := identitySchemaChild(t, operation.SubStatements(), "input", "")
	identitySchemaChild(t, input.SubStatements(), "uses", "origin-admission-identity")
	identitySchemaChild(t, input.SubStatements(), "leaf", "deadline-ms")
	output := identitySchemaChild(t, operation.SubStatements(), "output", "")
	identitySchemaChild(t, output.SubStatements(), "leaf", "state")
	identitySchemaChild(t, output.SubStatements(), "container", "snapshot")
}
