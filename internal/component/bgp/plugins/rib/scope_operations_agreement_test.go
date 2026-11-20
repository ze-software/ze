// Design: docs/architecture/api/commands.md -- command-owned pipe operations
//
// Related: rib_pipeline.go -- scopeKeywords, the table held here
//
// scope_operations_agreement_test.go holds this package's scope table to the
// pipe operations the RIB commands declare.
//
// The scope was a YANG enumeration of `show bgp peer rib` and is a pipe
// operation now (D-9): the operations are declared once, by the RIB command
// plugin (registerPipeFilters, internal/component/bgp/plugins/cmd/rib/rib.go),
// and this package's table maps each scope word to the table it reads. The
// table cannot be derived from the declaration, because it carries what the
// declaration does not -- which scope a word means. So what the two owe each
// other is AGREEMENT, and that is what this test reads, from the registry
// rather than from a copy of the words.

package rib

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	// The RIB command plugin records its pipe operations from init().
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/rib"
	"github.com/ze-software/ze/internal/component/command"
)

// declaredOperationNames answers the names of the pipe operations a command
// path declares, sorted. It FAILS rather than answering an empty set: a test
// comparing a Go table against nothing passes over every drift there is.
func declaredOperationNames(t *testing.T, path string) []string {
	t.Helper()
	filters := command.PipeFiltersForCommand(path)
	require.NotEmpty(t, filters, "%s declares no pipe operation", path)
	names := make([]string, 0, len(filters))
	for _, filter := range filters {
		names = append(names, filter.Name)
	}
	slices.Sort(names)
	return names
}

// TestRIBScopeKeywordsMatchTheDeclaredOperations proves scopeKeywords resolves
// every scope operation the route readers declare, and holds no other word. It
// replaces TestRIBScopeKeywordsMatchTheYANGModel, whose YANG leaf is gone.
//
// The scope operations are derived, not listed: they are what `show bgp rib`
// declares and `show bgp rib protocol` does not, because the protocol reader
// declares the same set without a scope (D-3). `show bgp peer rib` must
// declare every one of them too, and `show bgp rib best` none.
//
// PREVENTS: a scope word an operator can type that the pipeline does not
// recognize, which silently falls back to sent-received and answers about
// routes the operator did not ask for; and a word the table knows that no
// command offers.
func TestRIBScopeKeywordsMatchTheDeclaredOperations(t *testing.T) {
	routes := declaredOperationNames(t, "show bgp rib")
	protocol := declaredOperationNames(t, "show bgp rib protocol")
	declared := slices.DeleteFunc(slices.Clone(routes), func(name string) bool {
		return slices.Contains(protocol, name)
	})
	require.NotEmpty(t, declared, "show bgp rib declares no operation beyond the protocol reader's")

	offered := make([]string, 0, len(scopeKeywords))
	for keyword := range scopeKeywords {
		offered = append(offered, keyword)
	}
	slices.Sort(offered)

	require.Equal(t, declared, offered)

	peerRib := declaredOperationNames(t, "show bgp peer rib")
	best := declaredOperationNames(t, "show bgp rib best")
	// Every declared word resolves to one of the three scopes the pipeline
	// reads, so the table maps rather than merely covering the set.
	resolved := []string{scopeSent, scopeReceived, scopeSentReceived}
	for _, value := range declared {
		require.Contains(t, resolved, scopeKeywords[value],
			"show bgp rib declares scope %q and scopeKeywords resolves it to something the pipeline does not read", value)
		require.Contains(t, peerRib, value, "show bgp peer rib does not declare the scope %q", value)
		require.NotContains(t, best, value, "show bgp rib best declares the scope %q", value)
	}
}
