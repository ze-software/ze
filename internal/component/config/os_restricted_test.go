package config

import (
	"errors"
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// otherOSTestYANG holds one list that only plan9 carries, so every host Ze
// builds on prunes it. Its timer container declares a default.
const otherOSTestYANG = `module ze-test-otheros-conf {
    namespace "urn:ze:test:otheros";
    prefix to;
    import ze-extensions { prefix ze; }
    container otheros-test {
        list dialer {
            key "name";
            ze:os "plan9";
            leaf name { type string; }
            container timer {
                leaf interval { type uint16; default 10; }
            }
        }
    }
}`

// TestLookupDeclaredReachesANodeOfAnotherOS proves that a node the build
// prunes for its ze:os stays refused by Lookup and by the parser, and that
// LookupDeclared still answers its declared defaults.
// Method: a test module whose list is restricted to plan9, looked up both
// ways, its container's defaults applied, and a config naming it parsed.
func TestLookupDeclaredReachesANodeOfAnotherOS(t *testing.T) {
	restore := configyang.RegisterModuleForTest("ze-test-otheros-conf.yang", otherOSTestYANG)
	t.Cleanup(restore)
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	const path = "otheros-test/dialer/timer"
	if _, err := schema.Lookup(path); err == nil {
		t.Fatalf("Lookup(%s) answered a node the build pruned for plan9", path)
	}
	node, err := schema.LookupDeclared(path)
	if err != nil {
		t.Fatalf("LookupDeclared(%s): %v", path, err)
	}
	timer, ok := node.(*ContainerNode)
	if !ok {
		t.Fatalf("LookupDeclared(%s) is %T, want a container", path, node)
	}
	values := map[string]any{}
	ApplyDefaults(values, timer)
	if values["interval"] != "10" {
		t.Fatalf("defaults = %v, want interval 10 from the YANG default", values)
	}
	if _, err := schema.LookupDeclared("otheros-test/dialer/no-such-node"); err == nil ||
		!strings.Contains(err.Error(), "unknown path element: no-such-node") {
		t.Fatalf("unknown child err = %v, want unknown path element", err)
	}

	_, err = NewParser(schema).Parse("otheros-test {\n  dialer d0 {\n  }\n}\n")
	var otherOS *NodeOfAnotherOSError
	if !errors.As(err, &otherOS) {
		t.Fatalf("parse err = %v, want NodeOfAnotherOSError", err)
	}
	if otherOS.OS != "plan9" {
		t.Fatalf("refusal names OS %q, want plan9", otherOS.OS)
	}
}
