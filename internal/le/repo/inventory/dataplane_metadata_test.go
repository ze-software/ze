// Design: docs/architecture/plugin/feature-gates.md -- missing forwarding providers retain their declared identity.

package repoinventory

import (
	"encoding/json"
	"go/ast"
	"go/token"
	"testing"
)

// TestRegistrationDataPlaneMetadataIsRetained proves the inventory preserves
// the declaration needed to identify a writer after it is compiled out.
func TestRegistrationDataPlaneMetadataIsRetained(t *testing.T) {
	reader := registrations{eval: &expression{active: make(map[ast.Expr]bool)}}
	var registered registeredPlugin
	if err := reader.field(&registered, "DataPlane", sourceExpr{
		value: &ast.BasicLit{Kind: token.STRING, Value: `"fabric"`},
	}); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(registered.plugin)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["data-plane"] != "fabric" {
		t.Fatalf("inventory lost declared data plane: %s", data)
	}
}

// TestDataPlaneIsProtectedRegistrationMetadata proves mutation analysis treats
// the writer selector as metadata rather than an irrelevant registration field.
func TestDataPlaneIsProtectedRegistrationMetadata(t *testing.T) {
	if !metadataField("DataPlane") {
		t.Fatal("data-plane selector is outside the immutable metadata contract")
	}
}
