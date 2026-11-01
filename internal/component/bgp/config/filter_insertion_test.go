package bgpconfig

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// insertionConfig puts peer A in group G, whose session carries the leaf the
// test insertions read (insertionTestYANG), and peer B outside it with no
// such leaf.
const insertionConfig = `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	group G {
		session { asn { remote 65000; } }
		insertion-test { mode on; }
		peer A {
			connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
			%A%
		}
	}
	peer B {
		connection { remote { ip 192.0.2.2; } local { ip 192.0.2.254; } }
		session { asn { remote 65002; } }
	}
}
`

// insertionTestYANG is the inserting plugin's own module: it adds the
// container the test conditions read to a peer and a group, so a condition
// names config the plugin owns, as the loader requires of a compiled-in
// plugin (config.CheckFilterInsertionPaths).
const insertionTestYANG = `module ze-test-inserter-conf {
    namespace "urn:ze:test:inserter";
    prefix ti;
    import ze-bgp-conf { prefix bgp; }
    grouping inserter { container insertion-test { leaf mode { type string; } } }
    augment "/bgp:bgp/bgp:peer" { uses inserter; }
    augment "/bgp:bgp/bgp:group" { uses inserter; }
    augment "/bgp:bgp/bgp:group/bgp:peer" { uses inserter; }
}`

// registerInserter registers a compiled-in plugin, with insertionTestYANG as
// its module, that declares the given insertions, restoring the registry and
// the module set when the test ends.
func registerInserter(t *testing.T, insertions ...rpc.FilterInsertion) {
	t.Helper()
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-inserter-conf.yang", insertionTestYANG))
	require.NoError(t, registry.Register(registry.Registration{
		YANG:        insertionTestYANG,
		Name:        "fake-inserter",
		Description: "test plugin declaring filter insertions",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		Filters: []rpc.FilterDecl{
			{Name: "late", Direction: rpc.FilterBoth},
			{Name: "early", Direction: rpc.FilterBoth},
		},
		FilterInsertions: insertions,
	}))
}

func insertionPeers(t *testing.T, peerA string) (map[string]*reactor.PeerSettings, error) {
	t.Helper()
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(strings.Replace(insertionConfig, "%A%", peerA, 1))
	require.NoError(t, err)
	peers, _, err := peersAndDynamicGroups(tree)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*reactor.PeerSettings, len(peers))
	for _, ps := range peers {
		byName[ps.Name] = ps
	}
	return byName, nil
}

// refNames lists a chain's reference names, in chain order.
func refNames(refs []policy.Ref) []string {
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	return names
}

// rrClient is the condition the test insertions read: the group's leaf the
// plugin's own module adds, so peer A meets it by inheritance and peer B does
// not.
var rrClient = []string{"insertion-test", "mode"}

// TestInsertionFollowsPeerConfig checks spec-filter-wire-6-stage-filters
// AC-1 at the loader: a declared insertion lands in the named list of exactly
// the peers whose resolved config (group inheritance included) meets the
// condition, and in no other list. Method: one import insertion conditioned on
// a group leaf; A inherits it, B does not carry it.
func TestInsertionFollowsPeerConfig(t *testing.T) {
	registerInserter(t, rpc.FilterInsertion{Filter: "fake-inserter:late", Direction: rpc.FilterImport,
		Priority: 5, Config: rrClient, Values: []string{"on"}})

	peers, err := insertionPeers(t, "")
	require.NoError(t, err)
	require.Contains(t, refNames(peers["A"].ImportPolicy.Filters()), "fake-inserter:late",
		"peer A inherits the condition from group G, so its import list carries the member")
	require.NotContains(t, refNames(peers["A"].ExportPolicy.Filters()), "fake-inserter:late",
		"an import insertion never reaches the export list")
	require.False(t, peers["A"].ImportPolicy.Configured(),
		"an inserted member leaves the operator's policy unconfigured, so RFC 8212 Section 3 still decides the end")
	require.NotNil(t, peers["A"].ImportPolicy.Library(), "the inserted member runs in the peer's library chain")
	require.NotContains(t, refNames(peers["B"].ImportPolicy.Filters()), "fake-inserter:late",
		"peer B does not meet the condition, so nothing is inserted")
}

// TestInsertionsSortByPriority checks that inserted members precede in
// ascending priority whatever order the plugin declared them in, and that a
// value condition that does not match inserts nothing.
func TestInsertionsSortByPriority(t *testing.T) {
	registerInserter(t,
		rpc.FilterInsertion{Filter: "fake-inserter:late", Direction: rpc.FilterExport, Priority: 20, Config: rrClient},
		rpc.FilterInsertion{Filter: "fake-inserter:early", Direction: rpc.FilterExport, Priority: 10, Config: rrClient},
		rpc.FilterInsertion{Filter: "fake-inserter:early", Direction: rpc.FilterImport, Priority: 1,
			Config: rrClient, Values: []string{"off"}},
	)

	peers, err := insertionPeers(t, "")
	require.NoError(t, err)
	export := refNames(peers["A"].ExportPolicy.Filters())
	require.GreaterOrEqual(t, len(export), 2)
	require.Equal(t, []string{"fake-inserter:early", "fake-inserter:late"}, export[:2],
		"priority 10 runs before priority 20")
	require.NotContains(t, refNames(peers["A"].ImportPolicy.Filters()), "fake-inserter:early",
		"the leaf is on, so a condition asking for off inserts nothing")
}

// TestRequiredInsertionRefusesOperatorReference checks spec AC-7 at the
// loader: an operator who names a required member, plainly or `inactive:`, is
// refused with the declared reason, while a non-required member the operator
// names is not inserted a second time.
func TestRequiredInsertionRefusesOperatorReference(t *testing.T) {
	const reason = `RFC 9234 Section 5: "The operator MUST NOT have the ability to modify the procedures defined in this section."`
	registerInserter(t,
		rpc.FilterInsertion{Filter: "fake-inserter:late", Direction: rpc.FilterImport, Priority: 5,
			Config: rrClient, Required: true, Reason: reason},
		rpc.FilterInsertion{Filter: "fake-inserter:early", Direction: rpc.FilterExport, Priority: 5, Config: rrClient},
	)

	for _, form := range []string{"fake-inserter:late", "inactive:fake-inserter:late"} {
		_, err := insertionPeers(t, "filter { import { filters [ "+form+" ]; final accept; } }")
		require.Error(t, err, "naming the required member as %q must be refused", form)
		require.Contains(t, err.Error(), "peer A")
		require.Contains(t, err.Error(), reason)
	}

	peers, err := insertionPeers(t, "filter { export { filters [ fake-inserter:early ]; final accept; } }")
	require.NoError(t, err)
	count := 0
	for _, name := range refNames(peers["A"].ExportPolicy.Filters()) {
		if name == "fake-inserter:early" {
			count++
		}
	}
	require.Equal(t, 1, count, "the operator placed the non-required member; it is not inserted again")
}

// TestCompiledInsertionMustConditionOnOwnConfig checks AC-2 at the loader
// for a compiled-in plugin: a condition on config another module defines is
// refused, naming that module, because only the plugin's own config both
// means the peer needs the member and loads the plugin that serves it; a
// path the schema does not hold is refused too (A-10).
func TestCompiledInsertionMustConditionOnOwnConfig(t *testing.T) {
	for _, tc := range []struct {
		path []string
		want string
	}{
		{[]string{"session", "route-reflector-client"}, "ze-bgp-conf defines"},
		{[]string{"insertion-test", "nope"}, "bgp peer insertion-test nope"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			registerInserter(t, rpc.FilterInsertion{Filter: "fake-inserter:late", Direction: rpc.FilterImport,
				Priority: 5, Config: tc.path})
			_, err := insertionPeers(t, "")
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestThirdPartyInsertionAppliesFromStageOne checks
// spec-filter-wire-6-stage-filters A-12 at the loader: the insertion a
// third-party plugin declared at Stage 1, which no compiled registration
// carries, lands on the peers whose config meets its condition, and only
// there. Method: no compiled inserter; the runtime seam answers one process
// whose insertion conditions on a compiled path another module adds (a
// third party may name any compiled path); A inherits the condition, B does
// not carry it.
func TestThirdPartyInsertionAppliesFromStageOne(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-inserter-conf.yang", insertionTestYANG))
	registry.SetRuntimeFilterInsertions(func() []registry.ProcessFilterInsertions {
		return []registry.ProcessFilterInsertions{{
			Process: "third-party",
			Insertions: []rpc.FilterInsertion{{Filter: "third-party:judge", Direction: rpc.FilterImport,
				Priority: 100, Config: rrClient, Values: []string{"on"}}},
		}}
	})
	t.Cleanup(func() { registry.SetRuntimeFilterInsertions(nil) })

	peers, err := insertionPeers(t, "")
	require.NoError(t, err)
	require.Equal(t, []string{"third-party:judge"}, refNames(peers["A"].ImportPolicy.Filters()),
		"peer A meets the third party's condition, so its import list carries the member the plugin declared at Stage 1")
	require.NotContains(t, refNames(peers["B"].ImportPolicy.Filters()), "third-party:judge",
		"peer B does not meet the condition, so nothing is inserted")
}

// TestReloadVerifyAppliesRuntimeInsertions checks that the tree route a
// reload's verify and apply take (createTreeReloadFunc, which
// reactorAPIAdapter.VerifyConfig reaches through loadPeersFullOrTree) judges
// a third party's Stage 1 insertions, not only the compiled ones.
//
// Method: a running third-party process declares a required import insertion
// conditioned on peer A's config (registry.SetRuntimeFilterInsertions). The
// candidate tree, lowered to the plugin map form the reactor holds, goes
// through the tree reload: naming the required member is refused with the
// declared reason, and leaving it out inserts it on peer A only.
//
// VALIDATES: declaredInsertions appends registry.RuntimeFilterInsertions on
// the reload route (spec-filter-wire-6-stage-filters A-12).
// PREVENTS: a reload whose verify passes an operator edit of a third party's
// required member, or whose apply drops the member the boot placed.
func TestReloadVerifyAppliesRuntimeInsertions(t *testing.T) {
	const reason = "the third party's procedure is not the operator's to edit"
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-inserter-conf.yang", insertionTestYANG))
	registry.SetRuntimeFilterInsertions(func() []registry.ProcessFilterInsertions {
		return []registry.ProcessFilterInsertions{{
			Process: "third-party",
			Insertions: []rpc.FilterInsertion{{Filter: "third-party:judge", Direction: rpc.FilterImport,
				Priority: 100, Config: rrClient, Values: []string{"on"}, Required: true, Reason: reason}},
		}}
	})
	t.Cleanup(func() { registry.SetRuntimeFilterInsertions(nil) })

	reload := createTreeReloadFunc()
	candidate := func(peerA string) map[string]any {
		schema, err := config.YANGSchema()
		require.NoError(t, err)
		tree, err := config.NewParser(schema).Parse(strings.Replace(insertionConfig, "%A%", peerA, 1))
		require.NoError(t, err)
		return tree.ToPluginMap()
	}

	_, _, _, err := reload(candidate("filter { import { filters [ third-party:judge ]; final accept; } }"))
	require.Error(t, err, "the reload route must refuse an operator reference to a third party's required member")
	require.Contains(t, err.Error(), "peer A")
	require.Contains(t, err.Error(), reason)

	peers, _, _, err := reload(candidate(""))
	require.NoError(t, err)
	byName := make(map[string][]string, len(peers))
	for _, ps := range peers {
		byName[ps.Name] = refNames(ps.ImportPolicy.Filters())
	}
	require.Equal(t, []string{"third-party:judge"}, byName["A"],
		"the reload route must insert the third party's member where its condition holds")
	require.NotContains(t, byName["B"], "third-party:judge")
}
