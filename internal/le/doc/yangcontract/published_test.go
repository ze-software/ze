// The prefix half of the contract gate: every wire method carries the prefix
// derived from the package that registered it
// (spec-rpc-published-name-does-not-reach-its-handler, AC-10).

package docyangcontract

import (
	"strings"
	"testing"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// VALIDATES: AC-10. Every registered wire method in the linked product carries
// the prefix OwnerPrefix derives from the package that called RegisterRPCs, so
// no subsystem declares under a prefix it does not own.
// PREVENTS: a shared verb prefix such as ze-show:, where 43 owner directories
// sat in one namespace and a clash between two of them was possible by
// construction, and a prefix copied from a module file name (ze-l2tp-api:).
// MUTATION: register a handler under ze-show: from internal/plugins/ospf and
// this test names it.
func TestEverySubsystemDeclaresUnderItsOwnPrefix(t *testing.T) {
	rpcs := pluginserver.AllBuiltinRPCs()
	if len(rpcs) == 0 {
		t.Fatal("no builtin rpc is registered, so this test proves nothing")
	}
	for _, violation := range foreignPrefixes(rpcs) {
		t.Errorf("%s", violation)
	}
}

// VALIDATES: a registration whose prefix is not its package's, and one whose
// package lives under no subsystem root, are each named.
// PREVENTS: the check passing a registration it could not judge, which would
// let a package outside internal/component and internal/plugins spell any
// prefix.
func TestForeignPrefixesNamesEachGround(t *testing.T) {
	rpcs := []pluginserver.RPCRegistration{
		{WireMethod: "ze-ospf:show-neighbors", Registrar: "github.com/ze-software/ze/internal/plugins/ospf"},
		{WireMethod: "ze-show:ospf-neighbors", Registrar: "github.com/ze-software/ze/internal/plugins/ospf"},
		{WireMethod: "ze-hub:status", Registrar: "github.com/ze-software/ze/cmd/ze/hub"},
		{WireMethod: "no-colon", Registrar: "github.com/ze-software/ze/internal/plugins/ospf"},
	}
	got := foreignPrefixes(rpcs)
	if len(got) != 3 {
		t.Fatalf("named %d violations, want 3: %v", len(got), got)
	}
	for i, want := range []string{"ze-show:ospf-neighbors", "ze-hub:status", "no-colon"} {
		if !strings.HasPrefix(got[i], want+" ") {
			t.Errorf("violation %d is %q, want it to name %s", i, got[i], want)
		}
	}
	if !strings.Contains(got[0], "owns ze-ospf") {
		t.Errorf("the violation does not name the owned prefix: %q", got[0])
	}
}

// VALIDATES: a foreign prefix fails the verdict and the report names it.
// PREVENTS: a prefix violation printed above "All commands validated.".
// MUTATION: drop the ForeignPrefixes clause from contractSatisfied and this
// test goes red.
func TestAForeignPrefixFailsTheVerdict(t *testing.T) {
	result := ValidationResult{ForeignPrefixes: []string{"ze-show:x (registered by p, which owns ze-p)"}}
	if contractSatisfied(&result) {
		t.Fatal("a wire method under a foreign prefix satisfied the contract")
	}
	result.Valid = contractSatisfied(&result)
	text := result.Text()
	if !strings.Contains(text, "FAILED: 1 problem(s)") {
		t.Fatalf("the report does not fail on the foreign prefix:\n%s", text)
	}
	if !strings.Contains(text, "  ze-show:x (registered by p, which owns ze-p)\n") {
		t.Fatalf("the report does not name the foreign prefix:\n%s", text)
	}
}
