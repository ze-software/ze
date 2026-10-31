package reactor

import (
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/configorder"
)

// TestAddDynamicPeerReadsTheTreeItWasGiven holds the join between the command
// that creates a peer and the parser that reads one.
//
// GOAL: the tree `create bgp peer` builds is the tree parsePeerFromTree reads,
// so the command produces a peer with the settings the operator typed. The two
// halves name their leaves independently, and a path that disagrees fails at
// runtime with no compile error and no test red anywhere else.
// METHOD: hand AddDynamicPeer the tree handleBgpPeerCreate builds for
// `create bgp peer 192.0.2.7 asn 65002 local-as 65001 local-address 192.0.2.1
// router-id 1.2.3.4 receive-hold-time 90 connect-retry 5 group-updates false`,
// then read the PeerSettings it produced.
//
// VALIDATES: the peer is in the reactor under the address given, and its remote
// AS, local AS, local address, router id, hold time, connect retry and
// group-updates each carry the value the tree stated.
// PREVENTS: the failure this test was written for. AddDynamicPeer injected the
// remote address at the TOP of the tree while parsePeerFromTree reads
// `connection > remote > ip`, so every call failed with "missing required
// connection > remote > ip" and the only caller was a mock
// (ai/rules/principles.md).
func TestAddDynamicPeerReadsTheTreeItWasGiven(t *testing.T) {
	r := newDynamicPeerReactor(t, 0, 0)

	tree := map[string]any{
		"session": map[string]any{
			"asn":       map[string]any{"remote": "65002", "local": "65001"},
			"router-id": "1.2.3.4",
		},
		"connection": map[string]any{
			"local": map[string]any{"ip": "192.0.2.1"},
		},
		"timer":    map[string]any{"receive-hold-time": "90", "connect-retry": "5"},
		"behavior": map[string]any{"group-updates": "false"},
		"attach": map[string]any{
			"process": map[string]any{
				"peer-lifecycle": map[string]any{"receive": "*", "send": "*"},
			},
		},
	}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, r.AddDynamicPeer(addr, tree))

	r.mu.RLock()
	peer, exists := r.findPeerByAddr(addr)
	r.mu.RUnlock()
	require.True(t, exists, "the created peer is in the reactor")

	settings := peer.Settings()
	assert.Equal(t, addr, settings.Address)
	assert.Equal(t, uint32(65002), settings.PeerAS)
	assert.Equal(t, uint32(65001), settings.LocalAS)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), settings.LocalAddress)
	assert.Equal(t, uint32(0x01020304), settings.RouterID)
	assert.Equal(t, 90*time.Second, settings.ReceiveHoldTime)
	assert.Equal(t, 5*time.Second, settings.ConnectRetry)
	assert.False(t, settings.GroupUpdates)

	// The attach block is the peer's receive authorization and its send
	// permission both, so a binding that reaches the peer with neither grant
	// attaches a process and permits it nothing.
	require.Len(t, settings.ProcessBindings, 1, "the named process is bound to the peer")
	binding := settings.ProcessBindings[0]
	assert.Equal(t, "peer-lifecycle", binding.PluginName)
	assert.True(t, binding.ReceiveAll, "the process receives every message from this peer")
	assert.True(t, binding.SendAll, "the process may send every type toward this peer")
}

// TestAddDynamicPeerDefaultsTheLocalAddress holds the one leaf the caller may
// leave out.
//
// GOAL: a command that names no local address gets "auto", which is what
// applyLocalAddress requires and what leaves the kernel to choose the source.
// METHOD: call AddDynamicPeer with a tree stating no local ip.
//
// VALIDATES: the peer is created and its LocalAddress stays unset.
// PREVENTS: "local ip is required" reaching an operator who was never asked for
// one, which is what the parser answers when nothing fills the leaf.
func TestAddDynamicPeerDefaultsTheLocalAddress(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)

	tree := map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}

	addr := netip.MustParseAddr("192.0.2.8")
	require.NoError(t, r.AddDynamicPeer(addr, tree))

	r.mu.RLock()
	peer, exists := r.findPeerByAddr(addr)
	r.mu.RUnlock()
	require.True(t, exists)

	settings := peer.Settings()
	assert.False(t, settings.LocalAddress.IsValid(), "auto leaves the source address to the kernel")
	assert.Equal(t, uint32(65001), settings.LocalAS, "the reactor's own local AS is the default")
	assert.Equal(t, uint32(0x0A000001), settings.RouterID, "the reactor's own router id is the default")
}

// TestAddDynamicPeerRefusesAnIncompleteTree keeps the parser's refusal reaching
// the caller.
//
// GOAL: a tree with no remote AS creates no peer, and the error says which leaf
// is missing.
// METHOD: call AddDynamicPeer with an empty tree.
//
// VALIDATES: the call errors and the reactor holds no peer.
// PREVENTS: a half-built peer left in the reactor after a failed parse.
func TestAddDynamicPeerRefusesAnIncompleteTree(t *testing.T) {
	r := newDynamicPeerReactor(t, 0, 0)

	err := r.AddDynamicPeer(netip.MustParseAddr("192.0.2.9"), map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session > asn > remote")

	r.mu.RLock()
	_, exists := r.findPeerByAddr(netip.MustParseAddr("192.0.2.9"))
	r.mu.RUnlock()
	assert.False(t, exists, "a tree the parser refuses leaves no peer behind")
}

// TestAddDynamicPeerRefusesAPeerWithNoIdentity holds the refusal a peer created
// at runtime meets when nothing supplies its router id or its local AS.
//
// GOAL: a bgp section with no peer and
// no group loads with neither a global router-id nor a global local ASN, so
// `create bgp peer` over such a daemon is the first place either can be
// missing. The peer MUST be refused, naming the leaf, and never run with a
// zero BGP Identifier or a zero local AS (ai/rules/principles.md).
// METHOD: a reactor whose own local AS and router id are both zero; call
// AddDynamicPeer once with no router-id and once with no local AS.
//
// VALIDATES: each call errors naming the missing leaf and leaves no peer.
// PREVENTS: a created peer opening with BGP Identifier 0.0.0.0, which RFC 6286
// Section 2.1 rules out ("a 4-octet, unsigned, non-zero integer").
func TestAddDynamicPeerRefusesAPeerWithNoIdentity(t *testing.T) {
	r := newDynamicPeerReactor(t, 0, 0)

	cases := []struct {
		name    string
		session map[string]any
		want    string
	}{
		{
			name:    "no router id",
			session: map[string]any{"asn": map[string]any{"remote": "65002", "local": "65001"}},
			want:    "router-id",
		},
		{
			name: "no local AS",
			session: map[string]any{
				"asn":       map[string]any{"remote": "65002"},
				"router-id": "1.2.3.4",
			},
			want: "local as",
		},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr := netip.AddrFrom4([4]byte{192, 0, 2, byte(20 + i)})
			err := r.AddDynamicPeer(addr, map[string]any{"session": tc.session})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)

			r.mu.RLock()
			_, exists := r.findPeerByAddr(addr)
			r.mu.RUnlock()
			assert.False(t, exists, "a refused peer leaves nothing in the reactor")
		})
	}
}

// TestAddDynamicPeerAcceptsItsOwnIdentity is the positive control of the
// refusal above.
//
// GOAL: the router-id refusal fires only when nothing supplies one, so a peer
// that states its own router id and local AS is created over a reactor whose
// own are both zero.
// METHOD: AddDynamicPeer over a zero-identity reactor with a tree stating both.
//
// VALIDATES: the peer exists and carries the router id and local AS it stated.
// PREVENTS: the guard reading the reactor's router id instead of the resolved
// one, which would refuse every created peer on a daemon whose bgp section
// holds no session.
func TestAddDynamicPeerAcceptsItsOwnIdentity(t *testing.T) {
	r := newDynamicPeerReactor(t, 0, 0)

	addr := netip.MustParseAddr("192.0.2.30")
	require.NoError(t, r.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{
			"asn":       map[string]any{"remote": "65002", "local": "65001"},
			"router-id": "1.2.3.4",
		},
	}))

	r.mu.RLock()
	peer, exists := r.findPeerByAddr(addr)
	r.mu.RUnlock()
	require.True(t, exists, "the created peer is in the reactor")
	assert.Equal(t, uint32(0x01020304), peer.Settings().RouterID)
	assert.Equal(t, uint32(65001), peer.Settings().LocalAS)
}

// newDynamicPeerReactor builds the reactor these tests drive.
//
// It states the config explicitly rather than reusing newTestReactor: the
// router's own local AS and BGP Identifier are both in its config and in the
// bgp block of its running configuration, which is where the tree pipeline a
// created peer is built by reads them (createdPeerSettings). A zero leaves the
// leaf out of the running configuration, as a bgp section that states none.
// The reactor holds the tree pipeline every reactor CreateReactorFromTree
// builds holds, here the stand-in testTreeReload.
func newDynamicPeerReactor(t *testing.T, localAS, routerID uint32) *Reactor {
	t.Helper()
	r := newTestReactor(t)
	r.config = &Config{LocalAS: localAS, RouterID: routerID}
	bgp := map[string]any{}
	if localAS != 0 {
		bgp["session"] = map[string]any{"asn": map[string]any{"local": strconv.FormatUint(uint64(localAS), 10)}}
	}
	if routerID != 0 {
		bgp["router-id"] = netip.AddrFrom4([4]byte{byte(routerID >> 24), byte(routerID >> 16), byte(routerID >> 8), byte(routerID)}).String()
	}
	r.configTree = map[string]any{"bgp": bgp}
	r.SetTreeReloadFunc(testTreeReload(t))
	return r
}

// testTreeReload stands in for the config pipeline (createTreeReloadFunc,
// ../config/loader_create.go), which this package cannot import: the YANG
// defaults every configured peer gets, the globals of the bgp block, then
// each peer parsed with them, an incomplete one refused naming its leaf. It
// resolves no group and reads no dynamic group.
func testTreeReload(t *testing.T) TreeReloadFunc {
	t.Helper()
	return func(tree map[string]any) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
		schema, err := config.YANGSchema()
		if err != nil {
			return nil, Globals{}, nil, err
		}
		peerSchema, err := schema.Lookup("bgp/peer")
		if err != nil {
			return nil, Globals{}, nil, err
		}
		_, bgp, peers := copyToPeerList(tree)
		for name, entry := range peers {
			peer, isMap := cloneTestTree(entry).(map[string]any)
			if !isMap {
				return nil, Globals{}, nil, fmt.Errorf("peer %s is a %T", name, entry)
			}
			config.ApplyDefaults(peer, peerSchema)
			peers[name] = peer
		}
		if len(peers) > 0 {
			bgp["peer"] = peers
		}
		globals, err := GlobalsFromTree(bgp)
		if err != nil {
			return nil, Globals{}, nil, err
		}
		// The pipeline refuses an incomplete peer naming the leaf
		// (refuseIncompletePeers), where PeersFromTree skips it.
		names := slices.Sorted(maps.Keys(peers))
		built := make([]*PeerSettings, 0, len(names))
		for _, name := range names {
			peer, _ := peers[name].(map[string]any)
			settings, err := parsePeerFromTree(name, peer, globals.LocalAS, globals.RouterID)
			if err != nil {
				return nil, Globals{}, nil, fmt.Errorf("peer %s: %w", name, err)
			}
			settings.Name = name
			built = append(built, settings)
		}
		return built, globals, nil, nil
	}
}

// cloneTestTree answers a deep copy of a config tree value, so the defaults
// testTreeReload writes never reach the tree it was handed.
func cloneTestTree(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			out[key] = cloneTestTree(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = cloneTestTree(child)
		}
		return out
	case []string:
		return append([]string(nil), v...)
	default:
		return v
	}
}

// TestAddDynamicPeerRecordsTheRunningConfig holds the join between the command
// that creates a peer and the command that saves the running peer set.
//
// GOAL: a peer created at runtime is part of the running configuration, so
// `update bgp config` has the peer's own leaves to write into the file. The
// create command builds that tree once, and nothing else can rebuild it: a
// PeerInfo carries no families, no graceful restart time and no process
// binding.
// METHOD: create a peer through the API adapter, which is the object every
// command handler reaches, then read GetConfigTree.
//
// VALIDATES: bgp > peer > <address> carries the tree the command built, with
// the remote address AddDynamicPeer fills in.
// PREVENTS: `update bgp config` writing a peer the file cannot bring back up,
// which is what a save reconstructed from PeerInfo would write.
func TestAddDynamicPeerRecordsTheRunningConfig(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	api := &reactorAPIAdapter{r: r}

	tree := map[string]any{
		"session": map[string]any{
			"asn": map[string]any{"remote": "65002"},
			"family": map[string]any{
				"ipv4/unicast": map[string]any{"prefix": map[string]any{"maximum": "1000"}},
			},
		},
	}
	require.NoError(t, api.AddDynamicPeer(netip.MustParseAddr("192.0.2.7"), tree))

	peers := runningPeerList(t, api)
	require.Contains(t, peers, "peer-192.0.2.7",
		"the running configuration names the created peer, under a name its key leaf accepts")

	entry, ok := peers["peer-192.0.2.7"].(map[string]any)
	require.True(t, ok, "the entry is the peer's config subtree")
	session, ok := entry["session"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{
		"ipv4/unicast": map[string]any{"prefix": map[string]any{"maximum": "1000"}},
	}, session["family"], "every leaf the command stated is recorded, not the subset PeerInfo carries")

	connection, ok := entry["connection"].(map[string]any)
	require.True(t, ok, "AddDynamicPeer fills the remote address in")
	remote, ok := connection["remote"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.7", remote["ip"])
}

// TestRemovePeerDropsTheRunningConfig holds the absence half of the same join.
//
// GOAL: a peer deleted at runtime leaves the running configuration, so a later
// `update bgp config` takes it out of the file.
// METHOD: start from a configuration that declares one peer, remove it through
// the API adapter.
//
// VALIDATES: the peer list no longer names it.
// PREVENTS: a save that writes the deleted peer back, which is what a running
// configuration that still declared it would produce.
func TestRemovePeerDropsTheRunningConfig(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	api := &reactorAPIAdapter{r: r}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))
	require.Contains(t, runningPeerList(t, api), "peer-192.0.2.7")

	require.NoError(t, api.RemovePeer(addr))
	assert.NotContains(t, runningPeerList(t, api), "peer-192.0.2.7",
		"the running configuration drops the peer with the peer")
}

// TestRunningConfigKeepsTheEntryOrder holds the shape the peer list is
// delivered in.
//
// GOAL: the list stays readable by configorder.Entries, which refuses a list of
// two or more entries whose order does not name every one of them exactly once.
// METHOD: start from a configuration file that declares one peer, create a
// second at runtime, then delete the first.
//
// VALIDATES: two entries carry an order naming both, the configured one first;
// one entry carries no order key at all.
// PREVENTS: a peer list every reader refuses, which is what an entry added
// beside an order that does not name it produces.
func TestRunningConfigKeepsTheEntryOrder(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	r.configTree = map[string]any{
		"bgp": map[string]any{
			"peer": map[string]any{
				"peer1": map[string]any{"session": map[string]any{"asn": map[string]any{"remote": "65003"}}},
			},
		},
	}
	api := &reactorAPIAdapter{r: r}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))

	bgp := runningBGPBlock(t, api)
	entries, err := configorder.Entries(bgp, "peer", "name")
	require.NoError(t, err, "the delivered list and its order agree")
	require.Len(t, entries, 2)
	assert.Equal(t, "peer1", entries[0].Key, "the configured peer keeps its place")
	assert.Equal(t, "peer-192.0.2.7", entries[1].Key, "the created peer goes last")

	require.ErrorIs(t, api.RemovePeer(netip.MustParseAddr("127.0.0.1")), ErrPeerNotFound,
		"an address naming no peer is refused, and the peer list is left alone")
	assert.Len(t, runningPeerList(t, api), 2, "a refused removal drops nothing")
}

// TestReloadRemovalLeavesTheRunningConfig holds the boundary between the two
// removal paths.
//
// GOAL: only an API removal changes the running configuration. A reload removes
// peers through Reactor.RemovePeer while it applies a candidate, and it
// REPLACES the whole tree with SetConfigTree when it succeeds. A reload that
// fails partway must leave the running configuration as it was.
// METHOD: remove the peer through Reactor.RemovePeer, the way
// applyPeerOperation does.
//
// VALIDATES: the running configuration still names the peer.
// PREVENTS: a failed reload leaving a configured peer out of the running
// configuration, where the next `update bgp config` would delete it from the
// file.
func TestReloadRemovalLeavesTheRunningConfig(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	api := &reactorAPIAdapter{r: r}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))

	require.NoError(t, r.RemovePeer(addr))
	assert.Contains(t, runningPeerList(t, api), "peer-192.0.2.7",
		"the reload path leaves the running configuration to SetConfigTree")
}

// runningPeerList answers the peer list of the running configuration.
func runningPeerList(t *testing.T, api *reactorAPIAdapter) map[string]any {
	t.Helper()
	peers, ok := runningBGPBlock(t, api)["peer"].(map[string]any)
	require.True(t, ok, "the running configuration holds a peer list")
	return peers
}

// runningBGPBlock answers the bgp block of the running configuration.
func runningBGPBlock(t *testing.T, api *reactorAPIAdapter) map[string]any {
	t.Helper()
	bgp, ok := api.GetConfigTree()["bgp"].(map[string]any)
	require.True(t, ok, "the running configuration holds a bgp block")
	return bgp
}

// TestUnrelatedReloadKeepsACreatedPeer holds AC-9 of
// spec-yang-rpc-declarations-with-no-handler at the reactor.
//
// GOAL: a commit or a SIGHUP reload whose candidate never names a peer the
// operator created at runtime leaves that peer running, in the running
// configuration, and out of the reload's diff. The candidate comes from the
// configuration file, which a created peer is not in until `update bgp config`
// writes it, so its absence there says nothing about the operator's intent.
// METHOD: create a peer through the API adapter, then drive the three reactor
// steps a reload takes with a candidate that does not name it: ReloadRunning
// (what the diff and the decomposer compare against), ApplyConfigDiff, and
// SetConfigTree.
//
// VALIDATES: the peer carries the name the running configuration declares it
// under; ReloadRunning leaves it out, so no removal is planned; ApplyConfigDiff
// leaves it in the reactor; SetConfigTree keeps its entry in the running
// configuration.
// PREVENTS: "bgp operation remove-peer peer peer-127.0.0.2 is not running",
// which refused every commit while a created peer was up.
func TestUnrelatedReloadKeepsACreatedPeer(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	api := &reactorAPIAdapter{r: r}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))
	peer, held := r.findPeerByAddr(addr)
	require.True(t, held)
	assert.Equal(t, "peer-192.0.2.7", peer.Settings().Name,
		"a created peer carries the name the running configuration declares it under")

	candidate := map[string]any{"bgp": map[string]any{}}

	base := api.ReloadRunning(candidate)
	baseBGP, _ := base["bgp"].(map[string]any)
	basePeers, _ := baseBGP["peer"].(map[string]any)
	assert.NotContains(t, basePeers, "peer-192.0.2.7",
		"the reload compares the candidate against a running set without the unsaved created peer")
	assert.Contains(t, runningPeerList(t, api), "peer-192.0.2.7",
		"ReloadRunning reads the running configuration and changes nothing")

	require.NoError(t, api.ApplyConfigDiff(candidate))
	_, held = r.findPeerByAddr(addr)
	assert.True(t, held, "applying a candidate that never named the peer leaves it running")

	api.SetConfigTree(candidate)
	assert.Contains(t, runningPeerList(t, api), "peer-192.0.2.7",
		"the running configuration still names the peer after the reload")
	candidateBGP, _ := candidate["bgp"].(map[string]any)
	assert.NotContains(t, candidateBGP, "peer", "the caller's candidate is not written to")
}

// TestCandidateNamingACreatedPeerTakesItOver holds the other side of AC-9:
// once a candidate declares the created peer, the configuration owns it.
//
// GOAL: after `update bgp config` wrote the peer into the file, or after the
// operator declared its address under a name of their own, the reload treats
// the peer as configured, so a later candidate that drops it removes it.
// METHOD: create a peer; check ReloadRunning keeps it when the candidate names
// it by name and when it declares its address under another name; then set a
// candidate that names it, then one that does not.
//
// VALIDATES: ReloadRunning keeps the entry for both forms of declaration; after
// the declaring candidate is set, a candidate without it leaves no entry.
// PREVENTS: a created peer that no configuration change can ever remove, which
// is what carrying it unconditionally would produce.
func TestCandidateNamingACreatedPeerTakesItOver(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	api := &reactorAPIAdapter{r: r}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))
	entry := runningPeerList(t, api)["peer-192.0.2.7"]

	byName := map[string]any{"bgp": map[string]any{"peer": map[string]any{"peer-192.0.2.7": entry}}}
	assert.Contains(t, reloadRunningPeers(api, byName), "peer-192.0.2.7",
		"a candidate naming the peer compares against it")

	byAddress := map[string]any{"bgp": map[string]any{"peer": map[string]any{"edge": map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.7"}},
	}}}}
	assert.Contains(t, reloadRunningPeers(api, byAddress), "peer-192.0.2.7",
		"a candidate declaring the address under another name replaces the created peer")

	api.SetConfigTree(byName)
	api.SetConfigTree(map[string]any{"bgp": map[string]any{}})
	bgp, _ := api.GetConfigTree()["bgp"].(map[string]any)
	assert.NotContains(t, bgp, "peer", "a configured peer the candidate dropped leaves the running configuration")
}

// reloadRunningPeers answers the peer list of the running configuration a
// reload of candidate compares against.
func reloadRunningPeers(api *reactorAPIAdapter, candidate map[string]any) map[string]any {
	bgp, _ := api.ReloadRunning(candidate)["bgp"].(map[string]any)
	peers, _ := bgp["peer"].(map[string]any)
	return peers
}

// TestReloadRightAfterThePeerIsPublishedKeepsIt holds AC-9 against the window
// between a created peer entering the reactor and its createdPeers mark.
//
// GOAL: a reload that lands at any moment of `create bgp peer` keeps the peer.
// Nothing serializes the command against a reload (the transaction lock is the
// reload's own), so the earliest moment a reload can see the new peer is the
// moment Reactor.AddDynamicPeer publishes it in r.peers and releases r.mu. The
// mark and the running configuration entry MUST already be there then, because
// the reload's snapshot (reconcilePeersJournaled) reads the peers and the marks
// under one lock and removes a running peer that is neither declared nor marked.
// METHOD: deterministic interleaving, no sleep. Call Reactor.AddDynamicPeer,
// which returns exactly when the peer is published, then drive the three
// reactor steps of a reload whose candidate does not name it, before anything
// else of the create command runs. The adapter's AddDynamicPeer used to mark
// the peer only after that call returned, which is the window this test stands
// in.
//
// VALIDATES: after the reload the peer still runs, is still marked created,
// and still has its entry in the running configuration.
// PREVENTS: a reload deleting the peer an operator just created, then the
// create command recording a mark and an entry for a peer that no longer runs,
// which `update bgp config` would write to the file and `delete bgp peer`
// could never clear.
func TestReloadRightAfterThePeerIsPublishedKeepsIt(t *testing.T) {
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	api := &reactorAPIAdapter{r: r}

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, r.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))

	candidate := map[string]any{"bgp": map[string]any{}}
	assert.NotContains(t, reloadRunningPeers(api, candidate), "peer-192.0.2.7",
		"the reload compares against a running set without the unsaved created peer")
	require.NoError(t, api.ApplyConfigDiff(candidate))
	api.SetConfigTree(candidate)

	_, held := r.findPeerByAddr(addr)
	assert.True(t, held, "the reload leaves the peer running")
	r.mu.RLock()
	name, marked := r.createdPeers[addr]
	r.mu.RUnlock()
	assert.True(t, marked, "the peer is still marked created")
	assert.Equal(t, "peer-192.0.2.7", name)
	assert.Contains(t, runningPeerList(t, api), "peer-192.0.2.7",
		"the running configuration still names the peer")
}
