// Design: docs/architecture/config/syntax.md -- peer name rules
// Related: loader_test.go -- TestReservedPeerNamesSyncWithRPCs reads livePeerKeywords

package bgpconfig

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// livePeerKeywords builds the command tree the daemon builds, and reads the BGP
// peer keywords out of it. This package's tests import plugin/all, so the tree
// is the live merged one rather than a fixture: the children of one `peer`
// container come from several modules, and the selector one module declares
// binds every command another module hangs there.
//
// It REFUSES a partial registry. A derivation over a subset of the modules
// answers about a grammar no build has. The gap is too small for a count floor
// to see.
//
// ze-peer-cmd.yang declares the mandatory selector of `show bgp peer` and of
// `request peer`. ze-refresh-cmd.yang hangs borr, eorr, refresh and clear under
// that second container, and re-declares no selector. A binary holding the
// refresh module and not the peer module reads four guarded verbs as bare ones.
// That is the exact shape which reports a verb as colliding when it does not.
// So the modules are named from the source and each one is required.
func livePeerKeywords(t *testing.T) pluginserver.PeerKeywords {
	t.Helper()

	loader, err := yang.DefaultLoader()
	require.NoError(t, err)

	loaded := make(map[string]bool)
	for _, name := range loader.ModuleNames() {
		loaded[name] = true
	}
	declaring := peerContainerModules(t)
	for _, module := range declaring {
		if loaded[module] {
			continue
		}
		t.Fatalf("YANG module %s declares a `peer` container and is not linked into this test binary (the ze_bgp tag is off): the merged tree is a subset, so every assertion below would read a grammar no build has", module)
	}

	keywords := pluginserver.PeerSubcommandKeywords(yang.BuildCommandTree(loader))

	// The registry is whole, so a short vocabulary is the tree walk failing
	// rather than the modules missing. A floor is not a count: 16 keywords were
	// declared on 2026-09-13, and the floor only has to sit above what a broken
	// enumeration returns.
	const declaredFloor = 15
	if len(keywords.Declared) < declaredFloor {
		t.Fatalf("only %d bgp peer keywords enumerated from %d modules (floor %d): enumeration is broken and this gate is guarding almost nothing",
			len(keywords.Declared), len(declaring), declaredFloor)
	}
	return keywords
}

// peerContainerModules answers every YANG module in the BGP subtree that
// declares a `peer` container, read from the .yang sources rather than listed
// here. A list would be a second declaration of what the modules already say,
// and it would go stale the first time one is added.
//
// The BGP subtree is the scope because a BGP peer container is declared there:
// the peer containers of other subsystems (`show vpn ipsec peer`, `show ntp
// peer`) name no BGP peer and their modules gate on their own features.
func peerContainerModules(t *testing.T) []string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "internal", "component", "bgp"))
	require.NoError(t, err)

	var modules []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".yang") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		module := peerContainerModuleName(string(source))
		if module == "" {
			return nil
		}
		modules = append(modules, module)
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, modules, "no .yang source under internal/component/bgp declares a `peer` container: this walk reads the wrong tree and the refusal below refuses nothing")

	slices.Sort(modules)
	return modules
}

// peerContainerModuleName answers the module name a .yang source declares when
// that source declares a `peer` container, and "" when it declares none.
//
// `container peers` is a different node, so the keyword is matched with its
// opening brace rather than as a prefix.
func peerContainerModuleName(source string) string {
	var module string
	declares := false
	for line := range strings.SplitSeq(source, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case module == "" && strings.HasPrefix(line, "module "):
			module = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "module "), "{"))
		case strings.HasPrefix(line, "container peer {"):
			declares = true
		}
	}
	if !declares {
		return ""
	}
	return module
}

// TestPeerSubcommandKeywordsReadTheSelector holds the collision derivation to
// the grammar an operator types.
//
// A verb under a `peer` container collides with a peer NAME only when the
// operator can type it with nothing in between. The mandatory selector is what
// decides that, and it is declared as a YANG leaf on the container, which is a
// value rather than a node and so appears in no command path.
//
// VALIDATES: a word that follows the mandatory selector is not a collision, and
// a word that follows `peer` directly is.
// PREVENTS: the path-adjacency derivation this replaced, which read every verb
// as adjacent to `peer` and demanded reservations against an ambiguity the
// grammar cannot produce.
func TestPeerSubcommandKeywordsReadTheSelector(t *testing.T) {
	keywords := livePeerKeywords(t)

	// Guarded verbs. `show bgp peer` and `request peer` each declare a
	// mandatory `selector` leaf, and these verbs are their siblings in
	// ze-peer-cmd.yang. The operator types `show bgp peer <selector> detail`.
	// A peer named `detail` is read in the selector slot, with the verb still
	// to come.
	for _, keyword := range []string{"detail", "teardown"} {
		assert.True(t, keywords.Declared[keyword],
			"%q is declared under a bgp peer container, so the derivation must see it", keyword)
		assert.False(t, keywords.Colliding[keyword],
			"%q follows the mandatory peer selector, so no peer name can stand in its slot and it must not be reported as a collision", keyword)
	}

	// Bare verbs. `show bgp peer list` declares ze:inherit "none" and reads
	// every peer, so it takes no selector (ze-peer-cmd.yang). The operator
	// types it immediately after `peer`.
	//
	// `peer raw` and `peer update` stood beside it until 2026-09-05, when both
	// moved to `send bgp <selector> <form>`. No count of the set is pinned
	// here. TestReservedPeerNamesSyncWithRPCs (loader_test.go) reads the whole
	// of Colliding, so a bare verb that arrives later is caught there, against
	// reservedPeerNames rather than against a number.
	assert.True(t, keywords.Colliding["list"],
		"`show bgp peer list` takes no selector, so a peer named `list` stands in its slot and the derivation must report the collision")
}

// TestResolveRefusesCollidingPeerNames drives the guard from the entry point an
// operator's config reaches, rather than from the derivation alone.
//
// ResolveBGPTree is what the loader calls, validatePeerName (resolve.go) is the
// guard inside it, and reservedPeerNames is the list the guard reads. A test
// over the derivation says what the tree declares; only this one says what a
// config file gets back.
//
// VALIDATES: every colliding keyword is refused as a peer name, and a verb the
// selector guards is accepted.
// PREVENTS: reservedPeerNames drifting from the derivation in either direction,
// and the wrong repair for the red this replaced -- reserving `withdraw`, a
// name no dispatch can confuse.
func TestResolveRefusesCollidingPeerNames(t *testing.T) {
	keywords := livePeerKeywords(t)

	for keyword := range keywords.Colliding {
		assert.Error(t, resolveWithPeerNamed(keyword),
			"a peer named %q stands in the slot of a keyword typed immediately after `peer`, so the loader must refuse the config", keyword)
	}

	// The counter-cases this test exists for. `announce`, `withdraw`, `raw` and
	// `update` were keywords under a `peer` container until 2026-09-05. All
	// four then moved to `send bgp <selector> <form>`, and the root `peer`
	// container was deleted (ze-raw-cmd.yang, ze-cli-announce-cmd.yang,
	// ze-update-cmd.yang). No peer container declares any of them now. No
	// dispatch can read a peer of that name as a verb, so the loader must
	// accept every one.
	//
	// The premise is asserted before the acceptance. A word the tree declares
	// again is a collision candidate rather than a counter-case. This loop
	// would then assert the opposite of what it says.
	for _, name := range []string{"announce", "withdraw", "raw", "update"} {
		require.False(t, keywords.Declared[name],
			"a bgp peer container declares %q again, so this counter-case is stale: judge the name with the collision loop above", name)
		assert.NoError(t, resolveWithPeerNamed(name),
			"no bgp peer container declares %q, so a peer of that name stands in nobody's slot and the loader must accept it", name)
	}
}

// resolveWithPeerNamed resolves the smallest BGP config that names one peer,
// and answers what the loader said about the name. The name is the list KEY,
// which is what validatePeerName reads.
func resolveWithPeerNamed(name string) error {
	remote := config.NewTree()
	remote.Set("ip", "10.0.0.1")
	remote.Set("as", "65001")

	peer := config.NewTree()
	peer.SetContainer("remote", remote)

	group := config.NewTree()
	group.AddListEntry("peer", name, peer)

	local := config.NewTree()
	local.Set("as", "65000")

	bgp := config.NewTree()
	bgp.SetContainer("local", local)
	bgp.AddListEntry("group", "test-group", group)

	tree := config.NewTree()
	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	return err
}
