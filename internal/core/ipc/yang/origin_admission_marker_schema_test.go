// Design: docs/architecture/api/process-protocol.md -- accepted-origin route-only schema.
package yang

import (
	goyang "github.com/openconfig/goyang/pkg/yang"
	"testing"
)

// MUTATION: deleting the accepted-origin leaf's marker constraint removes the
// public route/marker separation while both scalar values remain declared.
func TestAcceptedOriginSchemaExcludesReplayMarker(t *testing.T) {
	statements, err := goyang.Parse(ZePluginEngineYANG, "ze-plugin-engine.yang")
	if err != nil {
		t.Fatal(err)
	}
	module := identitySchemaChild(t, statements, "module", "ze-plugin-engine")
	operation := identitySchemaChild(t, module.SubStatements(), "rpc", "update-route")
	input := identitySchemaChild(t, operation.SubStatements(), "input", "")
	leaf := identitySchemaChild(t, input.SubStatements(), "leaf", "origin-admission-id")
	identitySchemaChild(t, leaf.SubStatements(), "type", "uint64")
	identitySchemaChild(t, leaf.SubStatements(), "default", "0")
	identitySchemaChild(t, leaf.SubStatements(), "must", "not(../peer-marker) or . = 0")
	identitySchemaChild(t, input.SubStatements(), "leaf", "peer-marker")
}
