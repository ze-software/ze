// Design: docs/guide/pppoe.md -- unsupported access backends fail before apply.
// Related: backend_gate.go -- the registered original-tree validation boundary.

package pppoe

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestPPPoEVerifyRefusesVPPBackend drives the shared validation entry point,
// so an unwired validator cannot pass by testing only its helper.
func TestPPPoEVerifyRefusesVPPBackend(t *testing.T) {
	tree := pppoeBackendTree(t, "interface { backend vpp } pppoe { interface access0 {} }")
	_, err := config.ValidateCustomSections(tree)
	if err == nil {
		t.Fatal("validation accepted a PPPoE access interface on VPP")
	}
	for _, want := range []string{
		"/pppoe/interface/access0", `backend "vpp"`, "supported: netlink",
		"no PPPoE session data plane", "set interface backend netlink",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("backend refusal lacks %q: %v", want, err)
		}
	}
	if !strings.HasSuffix(err.Error(), "set interface backend netlink") {
		t.Fatalf("backend refusal does not end with its repair command: %v", err)
	}
}

// TestPPPoEVerifyAcceptsDefaultBackend checks the schema's actual default and
// the public validator on a tree that has no interface backend leaf.
func TestPPPoEVerifyAcceptsDefaultBackend(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	node, err := schema.Lookup("interface/backend")
	if err != nil {
		t.Fatal(err)
	}
	leaf, ok := node.(*config.LeafNode)
	if !ok {
		t.Fatal("interface/backend is not a schema leaf")
	}
	if leaf.Default != "netlink" {
		t.Fatalf("interface/backend default = %q, want netlink", leaf.Default)
	}
	tree := pppoeBackendTree(t, "pppoe { interface access0 {} }")
	if _, err := config.ValidateCustomSections(tree); err != nil {
		t.Fatalf("default backend refused: %v", err)
	}
}

// TestPPPoEVerifyAllowsNoAccessInterface proves the declaration applies only
// to configured access interfaces, rather than any PPPoE global setting.
func TestPPPoEVerifyAllowsNoAccessInterface(t *testing.T) {
	tree := pppoeBackendTree(t, "interface { backend vpp } pppoe { ac-name proof }")
	if _, err := config.ValidateCustomSections(tree); err != nil {
		t.Fatalf("PPPoE without an access interface refused: %v", err)
	}
}

// TestPPPoEVerifyReadsTheBackendPath distinguishes the interface backend
// from the same spelling in another setting or a comment.
func TestPPPoEVerifyReadsTheBackendPath(t *testing.T) {
	tree := pppoeBackendTree(t, `
interface {
    backend netlink
}
# backend vpp
pppoe {
    ac-name vpp
    interface access0 {}
}`)
	if _, err := config.ValidateCustomSections(tree); err != nil {
		t.Fatalf("a supported backend was confused with another value: %v", err)
	}
}

// pppoeBackendTree builds the same parsed structure the original-tree gate
// receives; it intentionally does not run validation before the test does.
func pppoeBackendTree(t *testing.T, source string) *config.Tree {
	t.Helper()
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.NewParser(schema).Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}
