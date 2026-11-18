// Design: docs/architecture/plugin/rib-storage-design.md -- covering read for `show bgp rib received covering <address>`
package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// coveringTestAttrs is the attribute set every route of these tests carries.
// The covering read selects on the prefix alone, so one set serves them all.
func coveringTestAttrs() []byte {
	return concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001)
}

// seedCoveringPeers builds the AC-1 table: two peers hold 10.0.0.0/8, the first
// also holds 10.1.0.0/16 and 0.0.0.0/0, and both hold 192.0.2.0/24, which does
// not contain 10.1.2.3. The routes go in least specific first, so a walk that
// kept insertion order could not pass the ordering assertions.
func seedCoveringPeers(r *RIBManager) {
	fam := family.IPv4Unicast
	first := storage.NewPeerRIB("192.0.2.1")
	first.Insert(fam, coveringTestAttrs(), []byte{0})
	first.Insert(fam, coveringTestAttrs(), []byte{8, 10})
	first.Insert(fam, coveringTestAttrs(), []byte{16, 10, 1})
	first.Insert(fam, coveringTestAttrs(), []byte{24, 192, 0, 2})
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = first

	second := storage.NewPeerRIB("192.0.2.2")
	second.Insert(fam, coveringTestAttrs(), []byte{24, 192, 0, 2})
	second.Insert(fam, coveringTestAttrs(), []byte{8, 10})
	r.bgpPeers[netip.MustParseAddr("192.0.2.2")] = second
}

// coveringRowKeys runs `show bgp rib` with args through the real handler and
// answers "peer prefix" for each streamed row, in the order they came.
func coveringRowKeys(t *testing.T, r *RIBManager, args ...string) []string {
	t.Helper()
	status, payload, err := r.handleCommand("show bgp rib", "*", args)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	records, streaming := payload.(sdk.Records)
	require.True(t, streaming, "show bgp rib with no terminal answers a walk, got %T", payload)

	rows, ok := drainShowRecords(t, records)["routes"].([]any)
	require.True(t, ok, "expected a routes list")
	keys := make([]string, 0, len(rows))
	for _, raw := range rows {
		row, isRow := raw.(map[string]any)
		require.True(t, isRow)
		peer, _ := row["peer"].(string)
		prefix, _ := row["prefix"].(string)
		keys = append(keys, peer+" "+prefix)
	}
	return keys
}

// drainSource pulls every item a source yields.
func drainSource(src pipelineIterator) []RouteItem {
	var items []RouteItem
	for {
		item, ok := src.Next()
		if !ok {
			return items
		}
		items = append(items, item)
	}
}

// TestPipelineCoveringKeyword proves `covering` parses into a covering walk
// from the CLI grammar, reaches the received source through `show bgp rib`,
// composes with a terminal, and refuses every value that is not a bare
// address with a message naming the value and a form that works.
//
// VALIDATES: AC-12, and the wiring row `show bgp rib received covering` from
// the CLI to parsePipelineArgs.
// PREVENTS: the keyword answered `unknown keyword`, a mask silently dropped so
// `covering 10.0.0.0/8` answers a different question, or a refusal that names
// neither the value nor what to type instead.
func TestPipelineCoveringKeyword(t *testing.T) {
	t.Run("parses", func(t *testing.T) {
		scope, _, walk, stages, errMsg := parsePipelineArgs([]string{"received", "covering", "10.1.2.3", "count"})
		require.Empty(t, errMsg)
		assert.Equal(t, scopeReceived, scope)
		covering, isCovering := walk.(coveringRead)
		require.True(t, isCovering, "covering builds a covering walk, got %T", walk)
		assert.Equal(t, netip.MustParseAddr("10.1.2.3"), covering.address)
		require.Len(t, stages, 1, "covering is a source selector, not a stage")
		assert.Equal(t, terminalCount, stages[0].kind)

		_, _, whole, _, errMsg := parsePipelineArgs([]string{"received", "count"})
		require.Empty(t, errMsg)
		assert.Equal(t, receivedWalk(wholeTableRead{}), whole, "no covering keyword is the whole-table walk")
	})

	t.Run("answers through the handler with a terminal", func(t *testing.T) {
		r := newTestRIBManager(t)
		seedCoveringPeers(r)
		status, payload, err := r.handleCommand("show bgp rib", "*", []string{"received", "covering", "10.1.2.3", "count"})
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(mustMarshal(t, payload), &parsed))
		assert.Equal(t, float64(4), parsed["count"], "four of the six routes contain 10.1.2.3")
	})

	for _, bad := range []struct {
		name  string
		args  []string
		wants []string
	}{
		// A value with a mask is pointed at exact, never at the prefix text
		// filter, which also lists longer prefixes sharing the same text.
		{"ipv4 mask", []string{"received", "covering", "10.0.0.0/8"},
			[]string{`"10.0.0.0/8"`, "no mask", "such as 10.0.0.0", "use exact 10.0.0.0/8"}},
		{"ipv4 host mask", []string{"received", "covering", "10.1.2.3/32"},
			[]string{`"10.1.2.3/32"`, "such as 10.1.2.3", "use exact 10.1.2.3/32"}},
		{"ipv4 mask with host bits", []string{"received", "covering", "10.1.2.3/8"},
			[]string{`"10.1.2.3/8"`, "such as 10.1.2.3", "use exact 10.0.0.0/8"}},
		{"ipv6 mask", []string{"received", "covering", "2001:db8::/32"},
			[]string{`"2001:db8::/32"`, "such as 2001:db8::", "use exact 2001:db8::/32"}},
		{"not an address", []string{"received", "covering", "not-an-address"},
			[]string{`"not-an-address"`, "is not an IP address", "such as 192.0.2.1"}},
		{"ipv6 typo", []string{"received", "covering", "2001:db8:::1"},
			[]string{`"2001:db8:::1"`, "such as 2001:db8::1"}},
		{"octet past 255", []string{"received", "covering", "10.1.2.256"},
			[]string{`"10.1.2.256"`, "is not an IP address"}},
		{"zone", []string{"received", "covering", "fe80::1%eth0"},
			[]string{`"fe80::1%eth0"`, "no zone", "such as fe80::1"}},
		{"dash-leading token", []string{"received", "covering", "-1"},
			[]string{`"-1"`, "is not an IP address"}},
		{"no value", []string{"received", "covering"},
			[]string{"covering requires a value", "192.0.2.1"}},
		{"twice", []string{"received", "covering", "10.1.2.3", "covering", "10.1.2.4"},
			[]string{"duplicate covering filter"}},
		{"after a terminal", []string{"received", "count", "covering", "10.1.2.3"},
			[]string{"filter after terminal: covering"}},
	} {
		t.Run("refuses "+bad.name, func(t *testing.T) {
			_, _, walk, stages, errMsg := parsePipelineArgs(bad.args)
			require.NotEmpty(t, errMsg, "%v must be refused", bad.args)
			assert.Nil(t, walk, "a refused parse carries no walk")
			assert.Nil(t, stages)
			for _, want := range bad.wants {
				assert.Contains(t, errMsg, want)
			}

			r := newTestRIBManager(t)
			status, _, err := r.handleCommand("show bgp rib", "*", bad.args)
			assert.Equal(t, statusError, status)
			require.Error(t, err)
			assert.Contains(t, err.Error(), errMsg, "the handler answers the parser's reason")
		})
	}
}

// TestCoveringRefusedOnSentScope proves a covering lookup on a scope that holds
// sent routes is refused, naming the received form with the operator's own
// address, rather than answered from the whole Adj-RIB-Out.
//
// VALIDATES: the scope decision: covering reads the received trie only.
// PREVENTS: `show bgp rib covering <address>` (both directions by default)
// silently answering received routes alone, or scanning every sent route.
func TestCoveringRefusedOnSentScope(t *testing.T) {
	for _, args := range [][]string{
		{"covering", "10.1.2.3"},
		{"sent", "covering", "10.1.2.3"},
		{"advertised", "covering", "10.1.2.3"},
		{"sent-received", "covering", "10.1.2.3"},
		{"covering", "10.1.2.3", "sent"},
	} {
		r := newTestRIBManager(t)
		seedCoveringPeers(r)
		status, data, err := r.handleCommand("show bgp rib", "*", args)
		assert.Equal(t, statusError, status, "%v", args)
		require.Error(t, err, "%v", args)
		assert.Contains(t, err.Error(), "covering reads received routes only", "%v", args)
		assert.Contains(t, err.Error(), "show bgp rib received covering 10.1.2.3", "%v: the fix names the received form", args)
		assert.Equal(t, "", data, "%v: a refusal carries no rows", args)
	}
}

// TestCoveringRefusedOnBestScope proves `show bgp rib best` refuses covering
// with a message naming the scope that takes it.
//
// VALIDATES: the best-scope decision recorded in the spec's census row.
// PREVENTS: covering accepted on best and answered from a whole-table walk, or
// refused as an unknown keyword that tells the operator nothing.
func TestCoveringRefusedOnBestScope(t *testing.T) {
	r := newTestRIBManager(t)
	seedCoveringPeers(r)
	status, _, err := r.handleCommand("show bgp rib best", "*", []string{"covering", "10.1.2.3"})
	assert.Equal(t, statusError, status)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "covering is not available on show bgp rib best")
	assert.Contains(t, err.Error(), "show bgp rib received covering")
}

// TestCoveringReadsOnlyThePath proves the covering read visits the trie entries
// on the path to the address and no others, whatever the table size.
//
// Method: one peer holds the four routes that contain 10.1.2.3 and 4096
// unrelated host routes. The source counts every entry the walk hands it
// (inboundSource.walked). The whole-table walk over the same peer is the
// control: it proves the counter sees every entry it is handed, so a covering
// read implemented as a scan with a containment test would count 4100.
//
// VALIDATES: AC-5.
// PREVENTS: R-1, a covering lookup that reads every route of every peer on a
// public endpoint.
func TestCoveringReadsOnlyThePath(t *testing.T) {
	r := newTestRIBManager(t)
	fam := family.IPv4Unicast
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	const unrelated = 4096
	for i := range unrelated {
		peerRIB.Insert(fam, coveringTestAttrs(), []byte{32, 172, 16, byte(i >> 8), byte(i)})
	}
	for _, nlri := range [][]byte{{0}, {8, 10}, {16, 10, 1}, {24, 10, 1, 2}} {
		peerRIB.Insert(fam, coveringTestAttrs(), nlri)
	}
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	covering := newInboundSource(r, "*", coveringRead{address: netip.MustParseAddr("10.1.2.3")})
	items := drainSource(covering)
	covering.release()
	assert.Len(t, items, 4, "the four covering routes are answered")
	assert.Equal(t, 4, covering.walked, "the covering read visits the path to the address only")

	whole := newInboundSource(r, "*", wholeTableRead{})
	all := drainSource(whole)
	whole.release()
	assert.Len(t, all, unrelated+4)
	assert.Equal(t, unrelated+4, whole.walked, "control: the counter sees every entry a walk hands over")
}

// TestCoveringOrdersMostSpecificFirst proves the AC-1 answer: every route that
// contains the address, grouped by peer in peer order, most specific prefix
// first within each peer, and nothing that does not contain it.
//
// VALIDATES: AC-1 through the real `show bgp rib` handler.
// PREVENTS: a covering answer in trie or insertion order, one that misses a
// less specific route, or one that lets 192.0.2.0/24 in.
func TestCoveringOrdersMostSpecificFirst(t *testing.T) {
	r := newTestRIBManager(t)
	seedCoveringPeers(r)

	got := coveringRowKeys(t, r, "received", "covering", "10.1.2.3")
	assert.Equal(t, []string{
		"192.0.2.1 10.1.0.0/16",
		"192.0.2.1 10.0.0.0/8",
		"192.0.2.1 0.0.0.0/0",
		"192.0.2.2 10.0.0.0/8",
	}, got)
}

// TestCoveringIPv6 proves an IPv6 address answers the covering IPv6 routes,
// most specific first, and never an IPv4 route, even a default one.
//
// VALIDATES: AC-6.
// PREVENTS: the two IP versions mixed in one covering answer, or IPv6 left
// out of the covering read.
func TestCoveringIPv6(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	v6 := family.IPv6Unicast
	peerRIB.Insert(v6, coveringTestAttrs(), []byte{0})
	peerRIB.Insert(v6, coveringTestAttrs(), []byte{32, 0x20, 0x01, 0x0d, 0xb8})
	peerRIB.Insert(v6, coveringTestAttrs(), []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})
	peerRIB.Insert(v6, coveringTestAttrs(), []byte{32, 0x20, 0x01, 0x0d, 0xb9})
	peerRIB.Insert(family.IPv4Unicast, coveringTestAttrs(), []byte{0})
	peerRIB.Insert(family.IPv4Unicast, coveringTestAttrs(), []byte{8, 10})
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	got := coveringRowKeys(t, r, "received", "covering", "2001:db8:1::5")
	assert.Equal(t, []string{
		"192.0.2.1 2001:db8:1::/48",
		"192.0.2.1 2001:db8::/32",
		"192.0.2.1 ::/0",
	}, got)

	got = coveringRowKeys(t, r, "received", "covering", "10.1.2.3")
	assert.Equal(t, []string{"192.0.2.1 10.0.0.0/8", "192.0.2.1 0.0.0.0/0"}, got,
		"an IPv4 address reads the IPv4 routes only")
}

// TestCoveringOrdersAcrossFamilies proves a peer holding covering routes in
// several families of one IP version answers them in one order, the most
// specific prefix first whatever its family, with two families holding one
// length answered in family order, and that the read still hands over the
// covering entries only.
//
// VALIDATES: AC-1's order on a peer whose covering routes are split between
// IPv4 unicast and IPv4 multicast, the order the looking glass help, guide and
// birdwatcher contract state for each peer.
// PREVENTS: the covering read running each family's trie walk in turn, which
// answers the unicast /8 and the unicast default route before the multicast
// /16 and /24, so a default route is no longer last.
func TestCoveringOrdersAcrossFamilies(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	unicast, multicast := family.IPv4Unicast, family.IPv4Multicast
	const unrelated = 1024
	for i := range unrelated {
		peerRIB.Insert(unicast, coveringTestAttrs(), []byte{32, 172, 16, byte(i >> 8), byte(i)})
		peerRIB.Insert(multicast, coveringTestAttrs(), []byte{32, 172, 17, byte(i >> 8), byte(i)})
	}
	for _, nlri := range [][]byte{{0}, {8, 10}, {16, 10, 1}} {
		peerRIB.Insert(unicast, coveringTestAttrs(), nlri)
	}
	for _, nlri := range [][]byte{{16, 10, 1}, {24, 10, 1, 2}} {
		peerRIB.Insert(multicast, coveringTestAttrs(), nlri)
	}
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	source := newInboundSource(r, "*", coveringRead{address: netip.MustParseAddr("10.1.2.3")})
	items := drainSource(source)
	source.release()
	got := make([]string, 0, len(items))
	for _, item := range items {
		got = append(got, item.Family.String()+" "+item.Prefix)
	}
	assert.Equal(t, []string{
		"ipv4/multicast 10.1.2.0/24",
		"ipv4/unicast 10.1.0.0/16",
		"ipv4/multicast 10.1.0.0/16",
		"ipv4/unicast 10.0.0.0/8",
		"ipv4/unicast 0.0.0.0/0",
	}, got)
	assert.Equal(t, 5, source.walked, "the read hands over the covering entries only, never the 2048 others")
}

// TestCoveringListsEveryAddPathPath proves every ADD-PATH path of a covering
// prefix is answered, not one per prefix.
//
// VALIDATES: AC-7.
// PREVENTS: R-6, a covering read over the ADD-PATH store that yields one path
// per prefix.
func TestCoveringListsEveryAddPathPath(t *testing.T) {
	r := newTestRIBManager(t)
	fam := family.IPv4Unicast
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.SetAddPath(fam, true)
	peerRIB.Insert(fam, coveringTestAttrs(), []byte{0, 0, 0, 1, 8, 10})
	peerRIB.Insert(fam, coveringTestAttrs(), []byte{0, 0, 0, 1, 16, 10, 1})
	peerRIB.Insert(fam, coveringTestAttrs(), []byte{0, 0, 0, 2, 16, 10, 1})
	peerRIB.Insert(fam, coveringTestAttrs(), []byte{0, 0, 0, 1, 16, 10, 2})
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	got := coveringRowKeys(t, r, "received", "covering", "10.1.2.3")
	require.Len(t, got, 3, "two paths of 10.1.0.0/16 and one of 10.0.0.0/8: %v", got)
	assert.ElementsMatch(t, []string{
		"192.0.2.1 10.1.0.0/16 [pathID=1]",
		"192.0.2.1 10.1.0.0/16 [pathID=2]",
	}, got[:2], "both paths of the most specific prefix come first")
	assert.Equal(t, "192.0.2.1 10.0.0.0/8 [pathID=1]", got[2])
}

// TestCoveringOnProtocolPipeline proves the protocol pipeline, which holds
// received routes only, answers covering from the same trie read.
//
// VALIDATES: `show bgp rib protocol <name> covering <address>` reaches
// protocolInboundSource with the covering walk.
// PREVENTS: the keyword parsed and then ignored on the one other caller of
// parsePipelineArgs, answering every route of the protocol.
func TestCoveringOnProtocolPipeline(t *testing.T) {
	r := newTestRIBManager(t)
	fam := family.IPv4Unicast
	bmpPeers := r.ribInPool[bmpProtocolID]
	bmpPeers["router1:peer1"] = storage.NewPeerRIB("router1:peer1")
	bmpPeers["router1:peer1"].Insert(fam, coveringTestAttrs(), []byte{8, 10})
	bmpPeers["router1:peer1"].Insert(fam, coveringTestAttrs(), []byte{24, 192, 0, 2})

	status, result, err := r.showProtocolPipeline("bmp", []string{"covering", "10.1.2.3", "count"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	assert.Equal(t, float64(1), parsed["count"], "only 10.0.0.0/8 contains 10.1.2.3")
}

// TestPrefixTextFilterUnchanged proves the `prefix` filter keeps its text
// meaning, partial form included, on both scopes.
//
// VALIDATES: AC-11, `show bgp rib sent prefix 10.0 count` and its received
// twin count what a text prefix match counts.
// PREVENTS: the covering work changing `prefix` into a containment test,
// which would count 10.10.0.0/16 and drop the documented partial form.
func TestPrefixTextFilterUnchanged(t *testing.T) {
	r := newTestRIBManager(t)
	fam := family.IPv4Unicast
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	for _, nlri := range [][]byte{{24, 10, 0, 0}, {24, 10, 0, 1}, {16, 10, 10}, {8, 10}, {24, 192, 0, 2}} {
		peerRIB.Insert(fam, coveringTestAttrs(), nlri)
	}
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	r.ribOut[netip.MustParseAddr("192.0.2.2")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		fam: {
			"10.0.0.0/24":  {Family: fam, Prefix: "10.0.0.0/24", NextHop: "10.0.0.1"},
			"10.0.1.0/24":  {Family: fam, Prefix: "10.0.1.0/24", NextHop: "10.0.0.1"},
			"10.10.0.0/16": {Family: fam, Prefix: "10.10.0.0/16", NextHop: "10.0.0.1"},
			"192.0.2.0/24": {Family: fam, Prefix: "192.0.2.0/24", NextHop: "10.0.0.1"},
		},
	})

	for _, scope := range []string{"sent", "received"} {
		status, payload, err := r.handleCommand("show bgp rib", "*", []string{scope, "prefix", "10.0", "count"})
		require.NoError(t, err, scope)
		require.Equal(t, statusDone, status, scope)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(mustMarshal(t, payload), &parsed), scope)
		// 10.0.0.0/24 and 10.0.1.0/24 start with the text "10.0", and
		// 10.10.0.0/16 does not. The received table also holds 10.0.0.0/8,
		// whose text starts with "10.0" too.
		want := float64(2)
		if scope == "received" {
			want = 3
		}
		assert.Equal(t, want, parsed["count"], "%s prefix 10.0 count", scope)
	}
}
