// Design: docs/architecture/config/syntax.md — config variable substitution for dynamic peers
// Related: templated_chain.go — templatedChain, checkTemplatedMember

package bgpconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
)

// templatedChainConfig is a dynamic group whose import chain names a match
// instance through `$remote_as`; %CHAIN% and %PEER% are replaced per test.
const templatedChainConfig = `
policy {
	prefix-list ALLOW-LIST-65010 {
		entry 10.0.0.0/8 { ge 16; le 24; action accept; }
	}
	match ALLOW-65010 {
		prefix-list ALLOW-LIST-65010
	}
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
			import { filters [ %CHAIN% ]; final accept; }
		}
	}
%PEER%
}
`

const templatedStaticPeer = `
	peer static1 {
		connection { remote { ip 192.0.2.1; } local { ip 192.0.2.2; } }
		session { asn { remote 65010; } family { ipv4/unicast { mode enable; prefix { maximum 100; } } } }
		filter { import { filters [ ALLOW-$remote_as ]; final accept; } }
	}
`

func loadTemplated(t *testing.T, chain, peer string) ([]*reactor.DynamicGroupConfig, error) {
	t.Helper()
	text := strings.ReplaceAll(templatedChainConfig, "%CHAIN%", chain)
	text = strings.ReplaceAll(text, "%PEER%", peer)
	tree, err := config.ParseTreeWithYANG(text, nil)
	require.NoError(t, err)
	_, groups, err := peersAndDynamicGroups(tree)
	return groups, err
}

// resolvedImport resolves the group template's import chain for remoteAS the
// way the reactor does at establishment, and compiles it.
func resolvedImport(t *testing.T, group *reactor.DynamicGroupConfig, remoteAS string) (string, error) {
	t.Helper()
	template := group.Settings.ImportPolicy
	refs := make([]policy.Ref, len(template.Filters()))
	for i, ref := range template.Filters() {
		refs[i] = ref
		refs[i].Name = strings.ReplaceAll(ref.Name, "$remote_as", remoteAS)
	}
	compiled, err := template.WithBoundFilters(refs, template.Bindings()).CompileResolved()
	require.NotNil(t, compiled.Library(), "a resolved chain always runs as a library chain, failed or not")
	names := make([]string, 0, len(compiled.Filters()))
	for _, ref := range compiled.Filters() {
		names = append(names, ref.Name)
	}
	return strings.Join(names, " "), err
}

// TestTemplatedMemberResolvesPerPeer proves that a dynamic group's chain may
// name a match instance through `$remote_as`: the template loads with no
// library, and each peer's resolved chain is judged and compiled from the
// instance it names.
//
// VALIDATES: checkTemplatedMember at load, templatedChain.CompileResolved per peer.
// PREVENTS: `unknown filter "ALLOW-$remote_as"` at load once the plugin-qualified
// list form is gone, and a peer running a library compiled from the template.
func TestTemplatedMemberResolvesPerPeer(t *testing.T) {
	groups, err := loadTemplated(t, "ALLOW-$remote_as", "")
	require.NoError(t, err)
	require.Len(t, groups, 1)
	template := groups[0].Settings.ImportPolicy
	assert.True(t, template.Templated(), "a template naming a member through a variable is compiled per peer")
	assert.Nil(t, template.Library(), "the template's names are not instances, so it has no library")

	names, err := resolvedImport(t, groups[0], "65010")
	require.NoError(t, err)
	assert.Contains(t, names, "ALLOW-65010")

	_, err = resolvedImport(t, groups[0], "65099")
	require.Error(t, err, "a resolved name no instance has fails closed")
	assert.Contains(t, err.Error(), "ALLOW-65099")
}

// TestTemplatedMemberRefusedAtLoad proves the load-time judgement of a
// templated member: the pattern must be able to name an instance, every
// variable must be one Ze resolves, and only a dynamic group resolves them.
func TestTemplatedMemberRefusedAtLoad(t *testing.T) {
	for _, tc := range []struct {
		name, chain, peer, want string
	}{
		{"no instance matches", "NOPE-$remote_as", "", `"NOPE-$remote_as"`},
		{"unknown variable", "ALLOW-$remote_asn", "", "$remote_asn"},
		{"static peer", "ALLOW-$remote_as", templatedStaticPeer, "dynamic group"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadTemplated(t, tc.chain, tc.peer)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
