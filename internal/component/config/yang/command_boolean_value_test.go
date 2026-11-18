// Design: docs/architecture/api/commands.md -- booleans are valued command leaves.
package yang

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// TestBooleanCommandLeafKeepsFalseAsAValue resolves the real YANG type and
// checks useful false and true values, flag absence, and a malformed refusal.
// MUTATION: drop gyang.Ybool while extracting typed command definitions.
func TestBooleanCommandLeafKeepsFalseAsAValue(t *testing.T) {
	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatal(err)
	}
	if err := loader.AddModuleFromText("ze-boolean-command-fixture-cmd", `module ze-boolean-command-fixture-cmd {
 namespace "urn:ze:test:boolean-command:cmd";
 prefix boolcmd;
 import ze-extensions { prefix ze; }
 container show {
  config false;
  container boolean-fixture {
   config false;
   ze:command "boolcmd:read";
   ze:help "Read the boolean fixture.";
   leaf enabled { type boolean; mandatory true; ze:help "Ordinary boolean value."; }
  }
 }
}`); err != nil {
		t.Fatal(err)
	}
	schema, err := loader.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	definitions := PathToArgDefs(schema)["show boolean-fixture"]
	if len(definitions) != 1 {
		t.Fatalf("resolved boolean command has %d typed leaves, want one", len(definitions))
	}
	if definitions[0].TypeName() != "boolean" {
		t.Fatalf("source type=%q, want boolean", definitions[0].TypeName())
	}
	for _, value := range []string{"false", "true"} {
		t.Run(value, func(t *testing.T) {
			arguments, err := command.ValidateArgs([]string{"enabled", value}, definitions, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, present := arguments.Value("enabled")
			if !present {
				t.Fatal("a supplied boolean disappeared")
			}
			if got != value {
				t.Fatalf("boolean=%q, want %q", got, value)
			}
			if arguments.Flag("enabled") {
				t.Fatal("an ordinary boolean became a presence-only flag")
			}
		})
	}
	if _, err := command.ValidateArgs([]string{"enabled", "maybe"}, definitions, nil); err == nil {
		t.Fatal("a malformed boolean was accepted")
	}
}
