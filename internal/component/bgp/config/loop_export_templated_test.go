// Design: docs/architecture/core-design.md -- named policy references retain stage ownership.
// Related: templated_chain.go -- templatedChain.compile, the per-peer judgement
// Related: peers.go -- validateExportPolicyReferences, the load-time twin

package bgpconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
)

// templatedLoopConfig is a dynamic group whose %DIRECTION% chain names a
// loop-detection instance through `$remote_as`.
const templatedLoopConfig = `
policy {
	loop-detection AUTO-65010 { }
}
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	group ix {
		connection {
			remote { ip dynamic; connect false; range 10.0.0.0/8; }
			local  { ip 10.0.0.1; accept true; }
		}
		session { family { ipv4/unicast { mode enable; prefix { maximum 100; } } } }
		filter {
			%DIRECTION% { filters [ loop:AUTO-$remote_as ]; final accept; }
		}
	}
}
`

// resolvedTemplate resolves p's `$remote_as` refs for remoteAS the way the
// reactor does at establishment (resolveDynamicPolicy), and compiles it.
func resolvedTemplate(p filterapi.Policy, remoteAS string) (filterapi.Policy, error) {
	refs := make([]policy.Ref, len(p.Filters()))
	for i, ref := range p.Filters() {
		refs[i] = ref
		refs[i].Name = strings.ReplaceAll(ref.Name, "$remote_as", remoteAS)
	}
	return p.WithBoundFilters(refs, p.Bindings()).CompileResolved(filterapi.PeerFilterInfo{})
}

// TestTemplatedLoopExportRefused proves that a dynamic group's export chain
// naming a loop-detection instance through a config variable is refused once
// a peer resolves it, as the load refuses a written one
// (validateExportPolicyReferences): loop detection is an ingress stage, and
// no export check runs. The import twin resolves and compiles, so the
// refusal is the direction's, not the variable's.
//
// The group's export also carries a member a plugin inserted under a session
// condition (the EBGP blackhole agreement), so the chain drops the members
// whose condition fails before it judges the rest (keepHolding). The
// variable-named member must still be found there once its name holds no
// variable: the refusal below is reached only if it is.
//
// VALIDATES: templatedChain.compile refuses an export loop-detection ref, and
// the reactor's resolveDynamicPolicy then runs the fail-closed chain.
// PREVENTS: the library compiler skipping the bound ref (compileLibraryChain,
// "a compiled-in stage filter runs in the ingress pass") and the peer's export
// running as if the loop check had passed.
func TestTemplatedLoopExportRefused(t *testing.T) {
	load := func(direction string) filterapi.Policy {
		t.Helper()
		tree, err := config.ParseTreeWithYANG(strings.ReplaceAll(templatedLoopConfig, "%DIRECTION%", direction), nil)
		require.NoError(t, err)
		_, groups, err := peersAndDynamicGroups(tree)
		require.NoError(t, err, "a templated name the load cannot resolve is judged per peer")
		require.Len(t, groups, 1)
		if direction == "export" {
			return groups[0].Settings.ExportPolicy
		}
		return groups[0].Settings.ImportPolicy
	}

	export := load("export")
	require.True(t, export.Templated(), "the export template names its member through a variable")
	compiled, err := resolvedTemplate(export, "65010")
	require.Error(t, err, "an export loop-detection ref resolved for a peer must be refused")
	assert.Contains(t, err.Error(), "export filter loop:AUTO-65010 is only supported on import")
	require.NotNil(t, compiled.Library(), "the refused chain is the fail-closed library chain")

	compiled, err = resolvedTemplate(load("import"), "65010")
	require.NoError(t, err, "the same ref on import resolves to the engine's loop stage")
	found := false
	for i, ref := range compiled.Filters() {
		if ref.Name != "loop:AUTO-65010" {
			continue
		}
		found = true
		assert.True(t, compiled.InProcess(i), "the resolved import ref is bound to the ingress loop stage")
	}
	assert.True(t, found, "the resolved import chain holds the loop ref: %+v", compiled.Filters())
}
