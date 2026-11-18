// Design: docs/architecture/plugin/rib-storage-design.md -- exact read for `show bgp rib received exact <prefix>`
// Related: rib_pipeline_covering_test.go -- the covering tests whose fixtures these reuse
package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// TestPipelineExactKeyword proves `exact` parses into an exact walk from the
// CLI grammar, reaches the received source through `show bgp rib`, composes
// with a terminal, and refuses every value that is not a masked prefix with a
// message naming the value and a form that works.
//
// VALIDATES: the `exact <prefix>` source selector and its parse-time refusals.
// PREVENTS: a bare address read as a host route without saying so, a prefix
// with host bits silently masked into one the operator did not write, a zone
// accepted, or a refusal that names neither the value nor what to type.
func TestPipelineExactKeyword(t *testing.T) {
	t.Run("parses", func(t *testing.T) {
		scope, _, walk, stages, errMsg := parsePipelineArgs([]string{"received", "exact", "10.0.0.0/8", "count"})
		require.Empty(t, errMsg)
		assert.Equal(t, scopeReceived, scope)
		require.True(t, walk.isExact(), "exact builds an exact walk")
		assert.False(t, walk.isCovering(), "an exact walk is not a covering walk")
		assert.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), walk.exact)
		require.Len(t, stages, 1, "exact is a source selector, not a stage")
		assert.Equal(t, terminalCount, stages[0].kind)

		_, _, v6, _, errMsg := parsePipelineArgs([]string{"received", "exact", "2001:db8::/32"})
		require.Empty(t, errMsg)
		assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), v6.exact)

		_, _, whole, _, errMsg := parsePipelineArgs([]string{"received", "count"})
		require.Empty(t, errMsg)
		assert.False(t, whole.isExact(), "no exact keyword is the whole-table walk")
	})

	t.Run("answers through the handler with a terminal", func(t *testing.T) {
		r := newTestRIBManager(t)
		seedCoveringPeers(r)
		status, payload, err := r.handleCommand("show bgp rib", "*", []string{"received", "exact", "10.0.0.0/8", "count"})
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(mustMarshal(t, payload), &parsed))
		assert.Equal(t, float64(2), parsed["count"], "both peers hold 10.0.0.0/8, and 10.1.0.0/16 is not it")
	})

	for _, bad := range []struct {
		name  string
		args  []string
		wants []string
	}{
		{"ipv4 bare address", []string{"received", "exact", "10.1.2.3"},
			[]string{`"10.1.2.3"`, "no mask", "such as 10.1.2.3/32", "use covering 10.1.2.3"}},
		{"ipv6 bare address", []string{"received", "exact", "2001:db8::1"},
			[]string{`"2001:db8::1"`, "such as 2001:db8::1/128", "use covering 2001:db8::1"}},
		{"not a prefix", []string{"received", "exact", "not-a-prefix"},
			[]string{`"not-a-prefix"`, "is not a prefix", "such as 10.0.0.0/8"}},
		{"mask past the address length", []string{"received", "exact", "10.0.0.0/33"},
			[]string{`"10.0.0.0/33"`, "is not a prefix"}},
		{"ipv6 typo", []string{"received", "exact", "2001:db8:::/32"},
			[]string{`"2001:db8:::/32"`, "is not a prefix", "such as 2001:db8::/32"}},
		{"zoned prefix", []string{"received", "exact", "fe80::%eth0/64"},
			[]string{`"fe80::%eth0/64"`, "no zone", "such as fe80::/64"}},
		{"zoned address", []string{"received", "exact", "fe80::1%eth0"},
			[]string{`"fe80::1%eth0"`, "no zone", "such as fe80::1/128"}},
		{"ipv4 host bits", []string{"received", "exact", "10.0.0.1/8"},
			[]string{`"10.0.0.1/8"`, "no host bits set", "such as 10.0.0.0/8"}},
		{"ipv6 host bits", []string{"received", "exact", "2001:db8::1/32"},
			[]string{`"2001:db8::1/32"`, "no host bits set", "such as 2001:db8::/32"}},
		{"dash-leading token", []string{"received", "exact", "-1"},
			[]string{`"-1"`, "is not a prefix"}},
		{"no value", []string{"received", "exact"},
			[]string{"exact requires a value", "10.0.0.0/8"}},
		{"twice", []string{"received", "exact", "10.0.0.0/8", "exact", "10.1.0.0/16"},
			[]string{"duplicate exact filter"}},
		{"after a terminal", []string{"received", "count", "exact", "10.0.0.0/8"},
			[]string{"filter after terminal: exact"}},
	} {
		t.Run("refuses "+bad.name, func(t *testing.T) {
			_, _, walk, stages, errMsg := parsePipelineArgs(bad.args)
			require.NotEmpty(t, errMsg, "%v must be refused", bad.args)
			assert.False(t, walk.isExact(), "a refused parse carries no walk")
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

// TestExactAnswersOnlyThatPrefix proves `exact` answers the one prefix asked
// for and no longer prefix that shares its text, while `prefix` over the same
// table keeps its text meaning and lists both.
//
// Method: one peer holds 192.0.0.0/2 and 192.0.0.0/24, whose text starts with
// "192.0.0.0/2", plus an unrelated 10.0.0.0/8. The `prefix` half is the defect
// the looking glass hit made visible, and the proof `prefix` is unchanged.
//
// VALIDATES: an exact lookup of a prefix whose length is one digit.
// PREVENTS: `exact` implemented as the text match, which answers two rows.
func TestExactAnswersOnlyThatPrefix(t *testing.T) {
	r := newTestRIBManager(t)
	fam := family.IPv4Unicast
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, coveringTestAttrs(), []byte{24, 192, 0, 0})
	peerRIB.Insert(fam, coveringTestAttrs(), []byte{2, 192})
	peerRIB.Insert(fam, coveringTestAttrs(), []byte{8, 10})
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	got := coveringRowKeys(t, r, "received", "exact", "192.0.0.0/2")
	assert.Equal(t, []string{"192.0.2.1 192.0.0.0/2"}, got)

	text := coveringRowKeys(t, r, "received", "prefix", "192.0.0.0/2")
	assert.ElementsMatch(t, []string{"192.0.2.1 192.0.0.0/2", "192.0.2.1 192.0.0.0/24"}, text,
		"prefix is a text filter: 192.0.0.0/24 starts with the text 192.0.0.0/2")
}

// TestExactReadsOneEntry proves the exact read visits the one entry keyed by
// the prefix and no other, whatever the table size.
//
// Method: one peer holds 10.0.0.0/8, 10.1.0.0/16 and 4096 unrelated host
// routes. The source counts every entry the walk hands it
// (inboundSource.walked). The whole-table walk over the same peer is the
// control: it proves the counter sees every entry it is handed, so an exact
// read implemented as a scan with an equality test would count 4098.
//
// VALIDATES: the exact lookup reads the trie by key.
// PREVENTS: an exact lookup that reads every route of every peer on a public
// looking glass endpoint.
func TestExactReadsOneEntry(t *testing.T) {
	r := newTestRIBManager(t)
	fam := family.IPv4Unicast
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	const unrelated = 4096
	for i := range unrelated {
		peerRIB.Insert(fam, coveringTestAttrs(), []byte{32, 172, 16, byte(i >> 8), byte(i)})
	}
	peerRIB.Insert(fam, coveringTestAttrs(), []byte{8, 10})
	peerRIB.Insert(fam, coveringTestAttrs(), []byte{16, 10, 1})
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	exact := newInboundSource(r, "*", exactWalk(netip.MustParsePrefix("10.1.0.0/16")))
	items := drainSource(exact)
	exact.release()
	require.Len(t, items, 1, "the one route of 10.1.0.0/16 is answered")
	assert.Equal(t, 1, exact.walked, "the exact read visits the entry of the prefix only")

	whole := newInboundSource(r, "*", receivedWalk{})
	all := drainSource(whole)
	whole.release()
	assert.Len(t, all, unrelated+2)
	assert.Equal(t, unrelated+2, whole.walked, "control: the counter sees every entry a walk hands over")
}

// TestExactListsEveryAddPathPath proves every ADD-PATH path of the prefix is
// answered, from every selected peer, whether the peer stores the family with
// ADD-PATH or without it.
//
// VALIDATES: the exact read over both the ADD-PATH (multi) and the plain
// (direct) table.
// PREVENTS: an exact read that yields one path per prefix, or that reads only
// one of the two table shapes.
func TestExactListsEveryAddPathPath(t *testing.T) {
	r := newTestRIBManager(t)
	fam := family.IPv4Unicast
	addPathPeer := storage.NewPeerRIB("192.0.2.1")
	addPathPeer.SetAddPath(fam, true)
	addPathPeer.Insert(fam, coveringTestAttrs(), []byte{0, 0, 0, 1, 16, 10, 1})
	addPathPeer.Insert(fam, coveringTestAttrs(), []byte{0, 0, 0, 2, 16, 10, 1})
	addPathPeer.Insert(fam, coveringTestAttrs(), []byte{0, 0, 0, 1, 8, 10})
	addPathPeer.Insert(fam, coveringTestAttrs(), []byte{0, 0, 0, 1, 24, 10, 1, 2})
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = addPathPeer

	plainPeer := storage.NewPeerRIB("192.0.2.2")
	plainPeer.Insert(fam, coveringTestAttrs(), []byte{16, 10, 1})
	plainPeer.Insert(fam, coveringTestAttrs(), []byte{8, 10})
	r.bgpPeers[netip.MustParseAddr("192.0.2.2")] = plainPeer

	got := coveringRowKeys(t, r, "received", "exact", "10.1.0.0/16")
	require.Len(t, got, 3, "two paths from the ADD-PATH peer and one from the plain peer: %v", got)
	assert.ElementsMatch(t, []string{
		"192.0.2.1 10.1.0.0/16 [pathID=1]",
		"192.0.2.1 10.1.0.0/16 [pathID=2]",
	}, got[:2], "both paths of the ADD-PATH peer, in peer order")
	assert.Equal(t, "192.0.2.2 10.1.0.0/16", got[2])
}

// TestExactIPv6 proves an IPv6 prefix answers that IPv6 prefix alone, never a
// longer one sharing its text and never a route of the other IP version, even
// for the default route.
//
// VALIDATES: the exact read over IPv6 families, and the IP-version split.
// PREVENTS: `exact 2000::/3` answering 2000::/32 (the text match), or
// `exact ::/0` answering 0.0.0.0/0.
func TestExactIPv6(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	v6 := family.IPv6Unicast
	peerRIB.Insert(v6, coveringTestAttrs(), []byte{0})
	peerRIB.Insert(v6, coveringTestAttrs(), []byte{3, 0x20})
	peerRIB.Insert(v6, coveringTestAttrs(), []byte{32, 0x20, 0x00, 0x00, 0x00})
	peerRIB.Insert(v6, coveringTestAttrs(), []byte{32, 0x20, 0x01, 0x0d, 0xb8})
	peerRIB.Insert(family.IPv4Unicast, coveringTestAttrs(), []byte{0})
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	assert.Equal(t, []string{"192.0.2.1 2000::/3"}, coveringRowKeys(t, r, "received", "exact", "2000::/3"))
	assert.ElementsMatch(t, []string{"192.0.2.1 2000::/3", "192.0.2.1 2000::/32"},
		coveringRowKeys(t, r, "received", "prefix", "2000::/3"),
		"prefix is a text filter: 2000::/32 starts with the text 2000::/3")
	assert.Equal(t, []string{"192.0.2.1 ::/0"}, coveringRowKeys(t, r, "received", "exact", "::/0"),
		"an IPv6 prefix reads the IPv6 routes only")
	assert.Equal(t, []string{"192.0.2.1 0.0.0.0/0"}, coveringRowKeys(t, r, "received", "exact", "0.0.0.0/0"),
		"an IPv4 prefix reads the IPv4 routes only")
}

// TestExactRefusedOnSentScope proves an exact lookup on a scope that holds sent
// routes is refused, naming the received form with the operator's own prefix,
// rather than answered from the whole Adj-RIB-Out or from the received half.
//
// VALIDATES: the scope decision: exact reads the received trie only.
// PREVENTS: `show bgp rib exact <prefix>` (both directions by default)
// silently answering received routes alone, or every sent route.
func TestExactRefusedOnSentScope(t *testing.T) {
	for _, args := range [][]string{
		{"exact", "10.0.0.0/8"},
		{"sent", "exact", "10.0.0.0/8"},
		{"advertised", "exact", "10.0.0.0/8"},
		{"sent-received", "exact", "10.0.0.0/8"},
		{"exact", "10.0.0.0/8", "sent"},
	} {
		r := newTestRIBManager(t)
		seedCoveringPeers(r)
		status, data, err := r.handleCommand("show bgp rib", "*", args)
		assert.Equal(t, statusError, status, "%v", args)
		require.Error(t, err, "%v", args)
		assert.Contains(t, err.Error(), "exact reads received routes only", "%v", args)
		assert.Contains(t, err.Error(), "show bgp rib received exact 10.0.0.0/8", "%v: the fix names the received form", args)
		assert.Equal(t, "", data, "%v: a refusal carries no rows", args)
	}
}

// TestExactRefusedOnBestScope proves `show bgp rib best` refuses exact with a
// message naming the scope that takes it.
//
// VALIDATES: the best-scope decision, the same as covering's.
// PREVENTS: exact accepted on best and answered from a whole-table walk, or
// refused as an unknown keyword that tells the operator nothing.
func TestExactRefusedOnBestScope(t *testing.T) {
	r := newTestRIBManager(t)
	seedCoveringPeers(r)
	status, _, err := r.handleCommand("show bgp rib best", "*", []string{"exact", "10.0.0.0/8"})
	assert.Equal(t, statusError, status)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exact is not available on show bgp rib best")
	assert.Contains(t, err.Error(), "show bgp rib received exact")
}

// TestExactAndCoveringTogetherRefused proves one command naming both exact and
// covering is refused, in either order, with a message saying what each alone
// answers so the operator knows which to drop.
//
// VALIDATES: the two source selectors exclude each other.
// PREVENTS: one of the two silently dropped, or both applied as an
// intersection nobody asked for.
func TestExactAndCoveringTogetherRefused(t *testing.T) {
	for _, args := range [][]string{
		{"received", "covering", "10.1.2.3", "exact", "10.0.0.0/8"},
		{"received", "exact", "10.0.0.0/8", "covering", "10.1.2.3"},
	} {
		_, _, walk, stages, errMsg := parsePipelineArgs(args)
		require.NotEmpty(t, errMsg, "%v must be refused", args)
		assert.False(t, walk.isCovering(), "%v: a refused parse carries no walk", args)
		assert.False(t, walk.isExact(), "%v: a refused parse carries no walk", args)
		assert.Nil(t, stages, "%v", args)
		assert.Contains(t, errMsg, "covering and exact select different routes", "%v", args)
		assert.Contains(t, errMsg, "drop exact to list the routes containing 10.1.2.3", "%v", args)
		assert.Contains(t, errMsg, "drop covering to list the routes of 10.0.0.0/8", "%v", args)

		r := newTestRIBManager(t)
		seedCoveringPeers(r)
		status, _, err := r.handleCommand("show bgp rib", "*", args)
		assert.Equal(t, statusError, status, "%v", args)
		require.Error(t, err, "%v", args)
		assert.Contains(t, err.Error(), errMsg, "%v: the handler answers the parser's reason", args)
	}
}

// TestExactOnProtocolPipeline proves the protocol pipeline, which holds
// received routes only, answers exact from the same trie read.
//
// VALIDATES: `show bgp rib protocol <name> exact <prefix>` reaches
// protocolInboundSource with the exact walk.
// PREVENTS: the keyword parsed and then ignored on the one other caller of
// parsePipelineArgs, answering every route of the protocol.
func TestExactOnProtocolPipeline(t *testing.T) {
	r := newTestRIBManager(t)
	fam := family.IPv4Unicast
	bmpPeers := r.ribInPool[bmpProtocolID]
	bmpPeers["router1:peer1"] = storage.NewPeerRIB("router1:peer1")
	bmpPeers["router1:peer1"].Insert(fam, coveringTestAttrs(), []byte{8, 10})
	bmpPeers["router1:peer1"].Insert(fam, coveringTestAttrs(), []byte{24, 10, 0, 0})

	status, result, err := r.showProtocolPipeline("bmp", []string{"exact", "10.0.0.0/8", "count"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	assert.Equal(t, float64(1), parsed["count"], "only 10.0.0.0/8 itself, not 10.0.0.0/24")
}
