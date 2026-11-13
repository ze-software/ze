// Design: docs/architecture/api/process-protocol.md -- Stage 1 identity schema.
package yang

import (
	"encoding/json"
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRegistrationReplyDeclaresConfiguredIdentity(t *testing.T) {
	statements, err := goyang.Parse(ZePluginEngineYANG, "ze-plugin-engine.yang")
	if err != nil {
		t.Fatal(err)
	}
	module := identitySchemaChild(t, statements, "module", "ze-plugin-engine")
	registration := identitySchemaChild(t, module.SubStatements(), "rpc", "declare-registration")
	output := identitySchemaChild(t, registration.SubStatements(), "output", "")
	leaf := identitySchemaChild(t, output.SubStatements(), "leaf", "plugin-name")
	nameType := identitySchemaChild(t, leaf.SubStatements(), "type", "string")
	identitySchemaChild(t, nameType.SubStatements(), "length", "1..64")
	data, err := json.Marshal(rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion, PluginName: "forwarding-west"})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if string(wire[leaf.Argument]) != `"forwarding-west"` {
		t.Fatalf("schema identity absent from actual DTO %s", data)
	}
}

func identitySchemaChild(t *testing.T, statements []*goyang.Statement, keyword, argument string) *goyang.Statement {
	t.Helper()
	for _, statement := range statements {
		if statement.Keyword != keyword {
			continue
		}
		if statement.Argument == argument {
			return statement
		}
	}
	t.Fatalf("schema has no %s %q", keyword, argument)
	return nil
}
