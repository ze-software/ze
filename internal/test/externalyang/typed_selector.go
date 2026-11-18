// Design: docs/architecture/api/commands.md -- typed command metadata from an external owner.
// Related: ../fixture/plugin_fixture_typed_selector.go -- the real declaring process.
package externalyang

const (
	TypedSelectorProvider = "typed-selector-provider"
	TypedSelectorCommand  = "request typed-selector id"
	TypedSelectorState    = "show typed-selector state"
)

const typedSelectorAPI = `module ze-test-typed-selector-api {
 namespace "urn:ze:test:typed-selector:api";
 prefix typedapi;
 import ze-extensions { prefix ze; }
 description "Typed selector and structured flag fixture metadata.";
 typedef original-number {
  type uint16 { range "1..65535"; }
  description "A nonzero fixture original number.";
 }
 grouping controls {
  leaf listener { type string { length "1..64"; } mandatory true; ze:help "Original listener."; }
  leaf count { type uint8 { range "1..9"; } mandatory true; ze:help "Ordinary count."; }
  leaf enabled { type boolean; mandatory true; ze:help "Ordinary boolean value."; }
  leaf force { type empty; ze:help "Count one harmless forced action."; }
 }
 rpc apply {
  ze:help "Apply to one fixture original.";
  input {
   leaf session-id { type original-number; mandatory true; ze:help "RPC original number."; }
   uses controls;
  }
 }
 rpc state { ze:help "Inspect fixture action counters."; }
}
`

const typedSelectorCommands = `module ze-test-typed-selector-cmd {
 namespace "urn:ze:test:typed-selector:cmd";
 prefix typedcmd;
 import ze-extensions { prefix ze; }
 import ze-test-typed-selector-api { prefix typedapi; }
 import ze-system-cmd { prefix syscmd; }
 import ze-cli-show-cmd { prefix clishowcmd; }
 description "Commands declared only by the typed selector fixture.";
 augment "/syscmd:request" {
  container typed-selector {
   config false; ze:help "Exercise typed fixture selection.";
   container id {
    config false;
    ze:command "typedcmd:apply";
    ze:rpc "ze-test-typed-selector-api:apply";
    ze:help "Apply to one fixture original.";
    leaf id { type typedapi:original-number; mandatory true; ze:help "Original number."; }
    uses typedapi:controls;
   }
  }
 }
 augment "/clishowcmd:show" {
  container typed-selector {
   config false; ze:help "Inspect typed fixture selection.";
   container state {
    config false;
    ze:command "typedcmd:state";
    ze:rpc "ze-test-typed-selector-api:state";
    ze:help "Inspect fixture action counters.";
   }
  }
 }
}
`

// TypedSelectorModules returns the actual modules sent at Stage 1. They add
// commands and RPC metadata, no config roots, so no corpus config registration
// is needed. The provider derives its validation from these same declarations.
func TypedSelectorModules() []Module {
	return []Module{
		{Name: "ze-test-typed-selector-api", Namespace: "urn:ze:test:typed-selector:api", Text: typedSelectorAPI},
		{Name: "ze-test-typed-selector-cmd", Namespace: "urn:ze:test:typed-selector:cmd", Text: typedSelectorCommands},
	}
}
