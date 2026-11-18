// Design: docs/architecture/api/process-protocol.md -- exact public callback cardinality.
package yang

import (
	"strconv"
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The shared Go validator and actual public YANG leaf-list carry one limit,
// rather than passing field-name parity with contradictory cardinality.
func TestOriginAdmissionLookupCommunitySchemaLimit(t *testing.T) {
	statements, err := goyang.Parse(ZePluginCallbackYANG, "ze-plugin-callback.yang")
	require.NoError(t, err)
	module := identitySchemaChild(t, statements, "module", "ze-plugin-callback")
	snapshot := identitySchemaChild(t, module.SubStatements(), "grouping", "origin-admission-snapshot")
	community := identitySchemaChild(t, snapshot.SubStatements(), "leaf-list", "community")
	limit := identitySchemaChild(t, community.SubStatements(), "max-elements", "65535")
	require.Equal(t, strconv.Itoa(rpc.OriginAdmissionCommunityMax), limit.Argument)
}
