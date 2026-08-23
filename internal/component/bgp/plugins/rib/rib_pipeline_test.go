package rib

import (
	"encoding/json"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// --- Phase 1: Path matching ---

// TestPathMatchContiguous verifies contiguous AS path subsequence matching.
//
// VALIDATES: path filter with "64501,64502" matches contiguous subsequence in AS_PATH.
// PREVENTS: Non-contiguous matches being accepted.
func TestPathMatchContiguous(t *testing.T) {
	tests := []struct {
		name    string
		asPath  []uint32
		pattern string
		want    bool
	}{
		{"exact match single", []uint32{64501}, "64501", true},
		{"contiguous pair", []uint32{64500, 64501, 64502, 64503}, "64501,64502", true},
		{"non-contiguous fails", []uint32{64501, 64999, 64502}, "64501,64502", false},
		{"full path match", []uint32{64501, 64502}, "64501,64502", true},
		{"single not present", []uint32{64501, 64502}, "64503", false},
		{"empty path", []uint32{}, "64501", false},
		{"empty pattern", []uint32{64501}, "", true},
		{"anchored start match", []uint32{64501, 64502}, "^64501", true},
		{"anchored start no match", []uint32{64502, 64501}, "^64501", false},
		{"anchored with multiple", []uint32{64501, 64502, 64503}, "^64501,64502", true},
		{"anchored mismatch", []uint32{64500, 64501, 64502}, "^64501,64502", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchASPath(tt.asPath, tt.pattern)
			assert.Equal(t, tt.want, got, "matchASPath(%v, %q)", tt.asPath, tt.pattern)
		})
	}
}

// TestRouteItemFromInbound verifies RouteItem construction from Adj-RIB-In entries.
//
// VALIDATES: RouteItem correctly carries peer, family, prefix, direction from pool entries.
// PREVENTS: Missing fields in pipeline items from inbound RIB.
func TestRouteItemFromInbound(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001, testWireCommunity)
	nlriBytes := []byte{24, 10, 0, 0}

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	// Use the source iterator
	src := newInboundSource(r, "*")
	item, ok := src.Next()
	require.True(t, ok, "expected at least one item")
	assert.Equal(t, "192.0.2.1", item.Peer)
	assert.Equal(t, family.IPv4Unicast, item.Family)
	assert.Equal(t, "10.0.0.0/24", item.Prefix)
	assert.Equal(t, rpc.DirectionReceived, item.Direction)
	assert.NotNil(t, item.InEntry)
}

// --- Phase 2: Filter stages ---

// TestFilterPath verifies the path filter stage.
//
// VALIDATES: path filter passes routes with matching AS path and rejects non-matching.
// PREVENTS: Path filter accepting routes without the specified AS in the path.
func TestFilterPath(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64501, 64502}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", OutRoute: &Route{ASPath: []uint32{64501}}},
	}

	src := &sliceSource{items: items}
	f := newPathFilter(src, "64501")

	var results []RouteItem
	for {
		item, ok := f.Next()
		if !ok {
			break
		}
		results = append(results, item)
	}

	require.Len(t, results, 2, "expected 2 routes matching path 64501")
	assert.Equal(t, "10.0.0.0/24", results[0].Prefix)
	assert.Equal(t, "10.0.2.0/24", results[1].Prefix)
}

// TestFilterFamily verifies the family filter stage.
//
// VALIDATES: family filter only passes routes matching the specified address family.
// PREVENTS: Routes from non-matching families leaking through.
func TestFilterFamily(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24"},
		{Peer: "p1", Family: family.IPv6Unicast, Prefix: "2001:db8::/32"},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24"},
	}

	src := &sliceSource{items: items}
	f := newFamilyFilter(src, family.IPv4Unicast.String())

	var results []RouteItem
	for {
		item, ok := f.Next()
		if !ok {
			break
		}
		results = append(results, item)
	}

	require.Len(t, results, 2)
	assert.Equal(t, "10.0.0.0/24", results[0].Prefix)
	assert.Equal(t, "10.0.1.0/24", results[1].Prefix)
}

// TestFilterPrefix verifies the prefix filter stage.
//
// VALIDATES: prefix filter matches routes whose prefix starts with the given string.
// PREVENTS: Prefix filter matching unrelated prefixes.
func TestFilterPrefix(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24"},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "172.16.0.0/24"},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24"},
	}

	src := &sliceSource{items: items}
	f := newPrefixFilter(src, "10.0")

	var results []RouteItem
	for {
		item, ok := f.Next()
		if !ok {
			break
		}
		results = append(results, item)
	}

	require.Len(t, results, 2)
	assert.Equal(t, "10.0.0.0/24", results[0].Prefix)
	assert.Equal(t, "10.0.1.0/24", results[1].Prefix)
}

// TestFilterCommunity verifies the community filter stage.
//
// VALIDATES: community filter passes routes with matching community.
// PREVENTS: Routes without the specified community passing through.
func TestFilterCommunity(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{Communities: []attribute.Community{attribute.Community(65000<<16 | 100), attribute.Community(65000<<16 | 200)}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{Communities: []attribute.Community{attribute.Community(65001<<16 | 100)}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", OutRoute: &Route{Communities: []attribute.Community{attribute.Community(65000<<16 | 100)}}},
	}

	src := &sliceSource{items: items}
	f := newCommunityFilter(src, "65000:100")

	var results []RouteItem
	for {
		item, ok := f.Next()
		if !ok {
			break
		}
		results = append(results, item)
	}

	require.Len(t, results, 2)
	assert.Equal(t, "10.0.0.0/24", results[0].Prefix)
	assert.Equal(t, "10.0.2.0/24", results[1].Prefix)
}

// TestFilterMatch verifies the match filter stage (server-side text search).
//
// VALIDATES: match filter checks route field values (prefix, peer, family, next-hop).
// PREVENTS: match only working on serialized JSON text.
func TestFilterMatch(t *testing.T) {
	items := []RouteItem{
		{Peer: "192.0.2.1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24"},
		{Peer: "192.0.2.2", Family: family.IPv6Unicast, Prefix: "2001:db8::/32"},
		{Peer: "192.0.2.3", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24"},
	}

	src := &sliceSource{items: items}
	f := newMatchFilter(src, "10.0")

	var results []RouteItem
	for {
		item, ok := f.Next()
		if !ok {
			break
		}
		results = append(results, item)
	}

	// Should match prefixes containing "10.0" and peer "192.0.2.1" etc.
	// All items have "10.0" in peer or prefix
	require.Len(t, results, 2, "match '10.0' should find 2 routes with 10.0 in prefix")
	assert.Equal(t, "10.0.0.0/24", results[0].Prefix)
	assert.Equal(t, "10.0.1.0/24", results[1].Prefix)
}

// --- Phase 3: Terminal stages and pipeline builder ---

// TestTerminalCount verifies the count terminal.
//
// VALIDATES: count terminal drains iterator and returns count in metadata.
// PREVENTS: count terminal serializing routes instead of just counting.
func TestTerminalCount(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24"},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24"},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.2.0/24"},
	}

	src := &sliceSource{items: items}
	ct := newCountTerminal(src)

	// Count terminal produces no items
	_, ok := ct.Next()
	assert.False(t, ok, "count terminal should produce no items")

	meta := ct.Meta()
	assert.Equal(t, 3, meta.Count)
}

// TestTerminalJSON verifies the json terminal serializes routes.
//
// VALIDATES: json terminal serializes all route items to JSON.
// PREVENTS: Routes being dropped during JSON serialization.
func TestTerminalJSON(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", Direction: rpc.DirectionReceived},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", Direction: rpc.DirectionReceived},
	}

	src := &sliceSource{items: items}
	jt := newJSONTerminal(src)

	meta := jt.Meta()
	assert.Equal(t, 2, meta.Count)
	assert.NotEmpty(t, meta.JSON)

	// Verify JSON is valid
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(meta.JSON), &result))
}

// TestBuildPipeline verifies pipeline construction from args.
//
// VALIDATES: buildPipeline correctly parses scope and filter stages from args.
// PREVENTS: Misparse of scope keywords vs filter keywords.
func TestBuildPipeline(t *testing.T) {
	r := newTestRIBManager(t)

	// Insert a test route
	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	tests := []struct {
		name     string
		args     []string
		wantJSON bool // false = count terminal
	}{
		{"count terminal", []string{"count"}, false},
		{"path then count", []string{"path", "64501", "count"}, false},
		{"family filter", []string{"family", family.IPv4Unicast.String()}, true},
		{"no args = json default", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.showPipeline("*", tt.args)
			assert.NotEmpty(t, result)

			var parsed map[string]any
			require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
			if !tt.wantJSON {
				_, hasCount := parsed["count"]
				assert.True(t, hasCount, "expected count in result")
			}
		})
	}
}

// TestBuildPipelineUnknownKeyword verifies unknown keywords return error.
//
// VALIDATES: Unknown pipeline keywords produce an error response.
// PREVENTS: Silent ignore of typos in pipeline args.
func TestBuildPipelineUnknownKeyword(t *testing.T) {
	r := newTestRIBManager(t)

	result := r.showPipeline("*", []string{"bogus"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	_, hasError := parsed["error"]
	assert.True(t, hasError, "expected error for unknown keyword: %s", result)
}

// TestBuildPipelineFilterKeywordNoValue verifies filter keywords without values return error.
//
// VALIDATES: Filter keywords without values produce an error response.
// PREVENTS: Silent empty filter when user forgets the value.
func TestBuildPipelineFilterKeywordNoValue(t *testing.T) {
	r := newTestRIBManager(t)

	tests := []struct {
		name string
		args []string
	}{
		{"path no value", []string{"path"}},
		{"community no value", []string{"community"}},
		{"family no value", []string{"family"}},
		{"prefix no value", []string{"prefix"}},
		{"match no value", []string{"match"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.showPipeline("*", tt.args)
			var parsed map[string]any
			require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
			_, hasError := parsed["error"]
			assert.True(t, hasError, "expected error for '%s': %s", tt.name, result)
		})
	}
}

// --- Phase 4: Unified show bgp rib / show bgp rib best ---

// TestShowPipelineBothDirections verifies default scope returns both directions.
//
// VALIDATES: show bgp rib (no scope) returns both adj-rib-in and adj-rib-out routes.
// PREVENTS: Default scope only returning one direction.
func TestShowPipelineBothDirections(t *testing.T) {
	r := newTestRIBManager(t)

	// Add inbound route
	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	// Add outbound route
	r.ribOut[netip.MustParseAddr("192.0.2.2")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"172.16.0.0/24": {
				Family: family.IPv4Unicast, Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
			},
		},
	})

	result := r.showPipeline("*", []string{"count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	count, ok := parsed["count"]
	require.True(t, ok, "expected count key")
	assert.Equal(t, float64(2), count, "expected 2 routes (1 in + 1 out)")
}

// TestShowPipelineReceivedScope verifies received scope returns only inbound.
//
// VALIDATES: show bgp rib received returns only adj-rib-in routes.
// PREVENTS: Outbound routes leaking into received scope.
func TestShowPipelineReceivedScope(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	r.ribOut[netip.MustParseAddr("192.0.2.2")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"172.16.0.0/24": {
				Family: family.IPv4Unicast, Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
			},
		},
	})

	result := r.showPipeline("*", []string{"received", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	count := parsed["count"]
	assert.Equal(t, float64(1), count, "expected 1 received route")
}

// TestShowPipelineSentScope verifies sent scope returns only outbound.
//
// VALIDATES: show bgp rib sent returns only adj-rib-out routes.
// PREVENTS: Inbound routes leaking into sent scope.
func TestShowPipelineSentScope(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	r.ribOut[netip.MustParseAddr("192.0.2.2")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"172.16.0.0/24": {
				Family: family.IPv4Unicast, Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
			},
		},
	})

	result := r.showPipeline("*", []string{"sent", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	count := parsed["count"]
	assert.Equal(t, float64(1), count, "expected 1 sent route")
}

// TestShowPipelineAdvertisedScope verifies advertised is the user-facing Adj-RIB-Out scope.
//
// VALIDATES: show bgp rib | advertised selects adj-rib-out routes.
// PREVENTS: operators needing the internal "sent" name for advertised routes.
func TestShowPipelineAdvertisedScope(t *testing.T) {
	r := newTestRIBManager(t)

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family.IPv4Unicast, concatBytes(testWireOriginIGP, testWireNextHop), []byte{24, 10, 0, 0}, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	r.ribOut[netip.MustParseAddr("192.0.2.2")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"172.16.0.0/24": {Family: family.IPv4Unicast, Prefix: "172.16.0.0/24", NextHop: "10.0.0.1"},
		},
	})

	result := r.showPipeline("*", []string{"advertised", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	assert.Equal(t, float64(1), parsed["count"], "expected 1 advertised route")
}

// TestShowPipelineRejectsConflictingDirection verifies received and advertised are exclusive.
//
// VALIDATES: route direction filters reject conflicting combinations.
// PREVENTS: last-one-wins behavior hiding operator mistakes.
func TestShowPipelineRejectsConflictingDirection(t *testing.T) {
	r := newTestRIBManager(t)

	result := r.showPipeline("*", []string{"received", "advertised", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	assert.Contains(t, parsed["error"], "multiple route direction filters")
}

// TestShowPipelinePeerFilter verifies peer pipe filter constrains generation.
//
// VALIDATES: show bgp rib | received | peer <selector> filters at source selection.
// PREVENTS: peer pipe filter being treated as a generic text pipe.
func TestShowPipelinePeerFilter(t *testing.T) {
	r := newTestRIBManager(t)

	for peer, prefix := range map[string][]byte{
		"192.0.2.1": {24, 10, 0, 0},
		"192.0.2.2": {24, 10, 0, 1},
	} {
		peerRIB := storage.NewPeerRIB(peer)
		peerRIB.Insert(family.IPv4Unicast, concatBytes(testWireOriginIGP, testWireNextHop), prefix, true)
		r.bgpPeers[netip.MustParseAddr(peer)] = peerRIB
	}

	result := r.showPipeline("*", []string{"received", "peer", "192.0.2.2", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	assert.Equal(t, float64(1), parsed["count"], "expected routes from one peer")
}

// TestShowPipelineComposed verifies composing multiple filters.
//
// VALIDATES: Multiple filters compose via pipeline (path + community + count).
// PREVENTS: Filters not chaining correctly.
func TestShowPipelineComposed(t *testing.T) {
	r := newTestRIBManager(t)

	med100 := uint32(100)
	r.ribOut[netip.MustParseAddr("192.0.2.1")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"10.0.0.0/24": {
				Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", NextHop: "10.0.0.1",
				ASPath: []uint32{64501, 64502}, Communities: []attribute.Community{attribute.Community(65000<<16 | 100)}, MED: &med100,
			},
			"10.0.1.0/24": {
				Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", NextHop: "10.0.0.1",
				ASPath: []uint32{64501}, Communities: []attribute.Community{attribute.Community(65001<<16 | 200)}, MED: &med100,
			},
			"10.0.2.0/24": {
				Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", NextHop: "10.0.0.1",
				ASPath: []uint32{64503}, Communities: []attribute.Community{attribute.Community(65000<<16 | 100)}, MED: &med100,
			},
		},
	})

	// path 64501 community 65000:100 count -> should match only first route
	result := r.showPipeline("*", []string{"sent", "path", "64501", "community", "65000:100", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	count := parsed["count"]
	assert.Equal(t, float64(1), count, "expected 1 route matching path 64501 AND community 65000:100")
}

// TestHandleCommandRibShow verifies unified show bgp rib via handleCommand.
//
// VALIDATES: "show bgp rib" is dispatched through handleCommand.
// PREVENTS: show bgp rib not being wired into the command handler.
func TestHandleCommandRibShow(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	status, data, err := r.handleCommand("show bgp rib", "*", nil)
	assert.Equal(t, statusDone, status)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, data), &parsed))
}

// TestHandleCommandRibShowCount verifies show bgp rib with count terminal.
//
// VALIDATES: "show bgp rib" with count arg returns count without serializing routes.
// PREVENTS: count terminal still building full JSON output.
func TestHandleCommandRibShowCount(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	status, data, err := r.handleCommand("show bgp rib", "*", []string{"count"})
	assert.Equal(t, statusDone, status)
	assert.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, data), &parsed))
	count, ok := parsed["count"]
	require.True(t, ok, "expected count key")
	assert.Equal(t, float64(1), count)
}

// TestHandleCommandOldCommandsError verifies old commands return errors.
//
// VALIDATES: Old commands (bgp rib show in, bgp rib show out, bgp rib show best) return errors;
// truly unknown commands return Go errors.
// PREVENTS: Old commands silently working after migration.
func TestHandleCommandOldCommandsError(t *testing.T) {
	r := newTestRIBManager(t)

	// Pipeline-parsed old keywords: routed through "show bgp rib" with args,
	// parsePipelineArgs returns "unknown keyword" error in JSON data.
	for _, keyword := range []string{"in", "out", "best"} {
		status, data, err := r.handleCommand("show bgp rib", "*", []string{keyword})
		assert.NoError(t, err, "pipeline error for %q should not be a Go error", keyword)
		assert.Equal(t, statusDone, status, "pipeline error status for %q", keyword)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(mustMarshal(t, data), &parsed), "data should be valid JSON for %q", keyword)
		_, hasError := parsed["error"]
		assert.True(t, hasError, "data should contain error key for old keyword %q", keyword)
	}

	// Truly unknown commands: fall through to default case in handleCommand.
	for _, cmd := range []string{"bgp rib adjacent inbound show", "bgp rib adjacent outbound show"} {
		_, _, err := r.handleCommand(cmd, "*", nil)
		assert.Error(t, err, "expected error for old command %q", cmd)
	}
}

// --- Phase 4a: Match filter cross-field ---

// TestFilterMatchCrossField verifies match filter checks AS-path and community values.
//
// VALIDATES: match filter searches across origin, AS-path, communities, MED, local-pref fields.
// PREVENTS: match filter only checking prefix/peer/family/next-hop.
func TestFilterMatchCrossField(t *testing.T) {
	med100 := uint32(100)
	localPref200 := uint32(200)

	tests := []struct {
		name    string
		pattern string
		items   []RouteItem
		want    int
	}{
		{
			name:    "match AS path value",
			pattern: "64501",
			items: []RouteItem{
				{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64501, 64502}}},
				{Peer: "p2", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
			},
			want: 1,
		},
		{
			name:    "match community value",
			pattern: "65000:100",
			items: []RouteItem{
				{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{Communities: []attribute.Community{attribute.Community(65000<<16 | 100)}}},
				{Peer: "p2", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{Communities: []attribute.Community{attribute.Community(65001<<16 | 200)}}},
			},
			want: 1,
		},
		{
			name:    "match origin value",
			pattern: "igp",
			items: []RouteItem{
				{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{Origin: new(OriginIGP)}},
				{Peer: "p2", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{Origin: new(OriginEGP)}},
			},
			want: 1,
		},
		{
			name:    "match MED value",
			pattern: "100",
			items: []RouteItem{
				{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{MED: &med100}},
				{Peer: "p2", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{}},
			},
			want: 1,
		},
		{
			name:    "match local-pref value",
			pattern: "200",
			items: []RouteItem{
				{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{LocalPreference: &localPref200}},
				{Peer: "p2", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{}},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &sliceSource{items: tt.items}
			f := newMatchFilter(src, tt.pattern)

			var results []RouteItem
			for {
				item, ok := f.Next()
				if !ok {
					break
				}
				results = append(results, item)
			}

			assert.Len(t, results, tt.want, "expected %d results for pattern %q", tt.want, tt.pattern)
		})
	}
}

// TestParsePipelineInvalidASN verifies invalid ASN in path filter is rejected at parse time.
//
// VALIDATES: path filter with non-numeric ASN returns error from parsePipelineArgs.
// PREVENTS: invalid ASN silently passing through to matchASPath where it returns false.
func TestParsePipelineInvalidASN(t *testing.T) {
	_, _, _, errMsg := parsePipelineArgs([]string{"path", "abc"})
	assert.NotEmpty(t, errMsg, "expected error for invalid ASN")
	assert.Contains(t, errMsg, "invalid ASN", "error should mention invalid ASN")

	// Also verify via bestPipelineArgs
	_, _, errMsg = parseBestPipelineArgs([]string{"path", "abc"})
	assert.NotEmpty(t, errMsg, "expected error for invalid ASN in best pipeline")
	assert.Contains(t, errMsg, "invalid ASN", "error should mention invalid ASN")
}

// TestFilterMatchCrossFieldInEntry verifies match filter checks InEntry pool attributes.
//
// VALIDATES: match filter searches InEntry attributes (origin, AS-path, communities, MED, local-pref).
// PREVENTS: match filter only working for OutRoute but not InEntry.
func TestFilterMatchCrossFieldInEntry(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001, testWireCommunity, testWireMED100, testWireLocalPref100)
	nlriBytes := []byte{24, 10, 0, 0}

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	// Match on AS-path value "65001"
	result := r.showPipeline("*", []string{"received", "match", "65001", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	assert.Equal(t, float64(1), parsed["count"], "expected match on AS-path 65001")

	// Match on community "65000:100"
	result = r.showPipeline("*", []string{"received", "match", "65000:100", "count"})
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	assert.Equal(t, float64(1), parsed["count"], "expected match on community 65000:100")

	// Match on origin "igp"
	result = r.showPipeline("*", []string{"received", "match", "igp", "count"})
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	assert.Equal(t, float64(1), parsed["count"], "expected match on origin igp")
}

// --- Phase 4b: Terminal ordering validation ---

// TestParsePipelineTerminalBeforeFilter verifies filter after terminal returns error.
//
// VALIDATES: AC-10 — terminal before filter is invalid.
// PREVENTS: Silently ignoring filters placed after a terminal stage.
func TestParsePipelineTerminalBeforeFilter(t *testing.T) {
	_, _, _, errMsg := parsePipelineArgs([]string{"count", "path", "64501"})
	assert.Contains(t, errMsg, "filter after terminal")
}

// TestParsePipelineTwoTerminals verifies multiple terminals return error.
//
// VALIDATES: AC-10 — multiple terminals not allowed.
// PREVENTS: Ambiguous pipeline with two terminal stages.
func TestParsePipelineTwoTerminals(t *testing.T) {
	_, _, _, errMsg := parsePipelineArgs([]string{"count", "json"})
	assert.Contains(t, errMsg, "multiple terminals not allowed")
}

// --- Phase 4c: Zero-count and explicit scope ---

// TestTerminalCountZero verifies count terminal returns 0 when no routes match.
//
// VALIDATES: count terminal returns {"count":0} format for empty result.
// PREVENTS: count terminal returning empty string or omitting count key on zero.
func TestTerminalCountZero(t *testing.T) {
	src := &sliceSource{items: nil} // no items
	ct := newCountTerminal(src)

	_, ok := ct.Next()
	assert.False(t, ok, "count terminal should produce no items")

	meta := ct.Meta()
	assert.Equal(t, 0, meta.Count)
}

// TestShowPipelineCountZeroWithFilter verifies count=0 when filter excludes all routes.
//
// VALIDATES: Pipeline with filter that matches nothing returns {"count":0}.
// PREVENTS: Empty filter result producing invalid JSON or missing count key.
func TestShowPipelineCountZeroWithFilter(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	// Path filter for ASN 99999 — no routes have this ASN
	result := r.showPipeline("*", []string{"received", "path", "99999", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	count, ok := parsed["count"]
	require.True(t, ok, "expected count key in result")
	assert.Equal(t, float64(0), count, "expected count=0 for non-matching filter")
}

// TestShowPipelineExplicitSentReceived verifies explicit sent-received scope returns both directions.
//
// VALIDATES: "sent-received" keyword produces same result as default (no scope).
// PREVENTS: sent-received keyword being rejected as unknown.
func TestShowPipelineExplicitSentReceived(t *testing.T) {
	r := newTestRIBManager(t)

	// Add inbound route
	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	// Add outbound route
	r.ribOut[netip.MustParseAddr("192.0.2.2")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"172.16.0.0/24": {
				Family: family.IPv4Unicast, Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
			},
		},
	})

	// Explicit sent-received scope
	result := r.showPipeline("*", []string{"sent-received", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))
	count, ok := parsed["count"]
	require.True(t, ok, "expected count key")
	assert.Equal(t, float64(2), count, "expected 2 routes (1 in + 1 out) with explicit sent-received")
}

// --- Phase 5: Best-path pipeline ---

// TestBestPipeline_WithFilter verifies best-path pipeline with community filter.
//
// VALIDATES: bestPipeline applies filter stages to best-path results.
// PREVENTS: Filters being ignored on best-path output.
func TestBestPipeline_WithFilter(t *testing.T) {
	r := newTestRIBManager(t)

	// Two peers with routes to same prefix, different communities
	fam := family.IPv4Unicast
	nlri1 := []byte{24, 10, 0, 0}   // 10.0.0.0/24
	nlri2 := []byte{24, 172, 16, 0} // 172.16.0.0/24

	// Peer 1: 10.0.0.0/24 with community 65000:100
	attr1 := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001, testWireCommunity)
	peerRIB1 := storage.NewPeerRIB("192.0.2.1")
	peerRIB1.Insert(fam, attr1, nlri1, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB1

	// Peer 2: 172.16.0.0/24 with no community (just origin + nexthop)
	attr2 := concatBytes(testWireOriginIGP, testWireNextHop)
	peerRIB2 := storage.NewPeerRIB("192.0.2.2")
	peerRIB2.Insert(fam, attr2, nlri2, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.2")] = peerRIB2

	// Best pipeline with community filter: should only return the route with 65000:100
	result := r.bestPipeline("*", []string{"community", "65000:100"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))

	bestPath, ok := parsed["best-path"].([]any)
	require.True(t, ok, "expected best-path array")
	require.Len(t, bestPath, 1, "expected 1 best-path result matching community 65000:100")

	entry, ok := bestPath[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.0/24", entry["prefix"])
}

// TestBestPipeline_CountTerminal verifies count terminal on best-path results.
//
// VALIDATES: bestPipeline with count terminal returns count of best-path entries.
// PREVENTS: Count terminal not working with best-path source.
func TestBestPipeline_CountTerminal(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	nlri1 := []byte{24, 10, 0, 0}   // 10.0.0.0/24
	nlri2 := []byte{24, 172, 16, 0} // 172.16.0.0/24

	// Single peer with two prefixes
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlri1, true)
	peerRIB.Insert(fam, attrBytes, nlri2, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	result := r.bestPipeline("*", []string{"count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))

	count, ok := parsed["count"]
	require.True(t, ok, "expected count key")
	assert.Equal(t, float64(2), count, "expected 2 best-path entries")
}

// TestBestPipeline_Empty verifies best-path pipeline with empty RIB returns empty array.
//
// VALIDATES: bestPipeline with no routes returns {"best-path":[]} not {"best-path":null}.
// PREVENTS: nil slice marshaling to JSON null instead of empty array.
func TestBestPipeline_Empty(t *testing.T) {
	r := newTestRIBManager(t)

	// No routes in ribInPool — best pipeline should return empty array
	result := r.bestPipeline("*", nil)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))

	bestPath, ok := parsed["best-path"]
	require.True(t, ok, "expected best-path key")
	// Must be empty array, not null
	arr, ok := bestPath.([]any)
	require.True(t, ok, "best-path must be an array, not null; got %T", bestPath)
	assert.Empty(t, arr, "expected empty best-path array")
}

// --- Phase 6: Graph terminal ---

// TestGraphTerminal verifies the graph terminal produces box-drawing output.
//
// VALIDATES: AC-1 "Output contains box-drawing characters and both ASN labels."
// PREVENTS: Graph terminal producing empty or JSON output instead of text graph.
func TestGraphTerminal(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", Direction: rpc.DirectionReceived,
			OutRoute: &Route{ASPath: []uint32{64501, 64502, 64503}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", Direction: rpc.DirectionReceived,
			OutRoute: &Route{ASPath: []uint32{64504, 64502, 64503}}},
	}

	src := &sliceSource{items: items}
	gt := newGraphTerminal(src)

	// Graph terminal produces no items (drains upstream)
	_, ok := gt.Next()
	assert.False(t, ok, "graph terminal should produce no items")

	meta := gt.Meta()
	assert.Equal(t, 2, meta.Count)
	require.NotEmpty(t, meta.JSON, "graph terminal should produce text output in JSON field")

	// Output should contain ASN labels
	assert.Contains(t, meta.JSON, "AS64501")
	assert.Contains(t, meta.JSON, "AS64502")
	assert.Contains(t, meta.JSON, "AS64503")
	assert.Contains(t, meta.JSON, "AS64504")

	// Output should contain box-drawing characters
	assert.Contains(t, meta.JSON, "\u250C", "should contain box-drawing ┌")
}

// TestGraphTerminal_NoRoutes verifies the graph terminal handles empty input.
//
// VALIDATES: AC-7 "No routes match filters -- no crash."
// PREVENTS: Panic on empty pipeline input.
func TestGraphTerminal_NoRoutes(t *testing.T) {
	src := &sliceSource{items: nil}
	gt := newGraphTerminal(src)

	meta := gt.Meta()
	assert.Equal(t, 0, meta.Count)
	// Empty graph produces empty or minimal output
	assert.NotContains(t, meta.JSON, "panic")
}

// TestGraphTerminalViaPipeline verifies the graph terminal is wired into the pipeline.
//
// VALIDATES: AC-5 "Filters applied before graph construction."
// PREVENTS: Graph terminal not reachable through pipeline dispatch.
func TestGraphTerminalViaPipeline(t *testing.T) {
	r := newTestRIBManager(t)

	// Add routes with AS paths
	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	result := r.showPipeline("*", []string{"received", "graph"})
	require.NotEmpty(t, result)
	graphText, ok := result.(string)
	require.True(t, ok, "graph terminal should return a string so dispatch-command encodes it as JSON string data")

	assert.Contains(t, graphText, "AS65001")
	assert.Contains(t, graphText, "\u250C")
}

// TestGraphTerminalViaBestPipeline verifies graph terminal works with best-path pipeline.
//
// VALIDATES: AC-6 "show bgp rib best graph works."
// PREVENTS: Graph terminal only working with show, not best.
func TestGraphTerminalViaBestPipeline(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	result := r.bestPipeline("*", []string{"graph"})
	graphText, ok := result.(string)
	require.True(t, ok, "best graph terminal should return a string so dispatch-command encodes it as JSON string data")
	assert.Contains(t, graphText, "AS65001")
}

// --- Helpers ---

// sliceSource is a test helper that yields items from a slice.
type sliceSource struct {
	items []RouteItem
	idx   int
	meta  PipelineMeta
}

func (s *sliceSource) Next() (RouteItem, bool) {
	if s.idx >= len(s.items) {
		return RouteItem{}, false
	}
	item := s.items[s.idx]
	s.idx++
	return item, true
}

func (s *sliceSource) Meta() PipelineMeta {
	return s.meta
}

func TestShowRibFirstPipeline(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64501}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64502}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.3.0/24", OutRoute: &Route{ASPath: []uint32{64504}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.4.0/24", OutRoute: &Route{ASPath: []uint32{64505}}},
	}

	src := &sliceSource{items: items}
	f := newFirstFilter(src, "3")

	var results []RouteItem
	for {
		item, ok := f.Next()
		if !ok {
			break
		}
		results = append(results, item)
	}

	require.Len(t, results, 3)
	assert.Equal(t, "10.0.0.0/24", results[0].Prefix)
	assert.Equal(t, "10.0.2.0/24", results[2].Prefix)
}

func TestShowRibLastPipeline(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64501}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64502}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.3.0/24", OutRoute: &Route{ASPath: []uint32{64504}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.4.0/24", OutRoute: &Route{ASPath: []uint32{64505}}},
	}

	src := &sliceSource{items: items}
	f := newLastFilter(src, "2")

	var results []RouteItem
	for {
		item, ok := f.Next()
		if !ok {
			break
		}
		results = append(results, item)
	}

	require.Len(t, results, 2)
	assert.Equal(t, "10.0.3.0/24", results[0].Prefix)
	assert.Equal(t, "10.0.4.0/24", results[1].Prefix)
}

// TestShowRibLastHugeN verifies that a huge N does not preallocate: the lazy
// ring buffer is bounded by the actual item count. If `last` preallocated N
// RouteItems (~110 TB here) this test would OOM. With N >> items, every item
// passes through in order.
func TestShowRibLastHugeN(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64501}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64502}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
	}

	src := &sliceSource{items: items}
	f := newLastFilter(src, "1099511627776") // 2^40

	var results []RouteItem
	for {
		item, ok := f.Next()
		if !ok {
			break
		}
		results = append(results, item)
	}

	require.Len(t, results, 3)
	assert.Equal(t, "10.0.0.0/24", results[0].Prefix)
	assert.Equal(t, "10.0.2.0/24", results[2].Prefix)
}

// TestShowRibLastRingWraps verifies oldest-first ordering after the ring buffer
// wraps multiple times: last 3 over 7 items keeps the final 3 in order.
func TestShowRibLastRingWraps(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64500}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64501}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", OutRoute: &Route{ASPath: []uint32{64502}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.3.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.4.0/24", OutRoute: &Route{ASPath: []uint32{64504}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.5.0/24", OutRoute: &Route{ASPath: []uint32{64505}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.6.0/24", OutRoute: &Route{ASPath: []uint32{64506}}},
	}

	src := &sliceSource{items: items}
	f := newLastFilter(src, "3")

	var got []string
	for {
		item, ok := f.Next()
		if !ok {
			break
		}
		got = append(got, item.Prefix)
	}

	assert.Equal(t, []string{"10.0.4.0/24", "10.0.5.0/24", "10.0.6.0/24"}, got)
}

// TestParsePipelineArgsRejectsBadFirstLast verifies first/last reject
// non-positive / non-numeric counts server-side. This is the folded-command
// path that bypasses the client-side ValidatePipes numeric check, so without
// server-side validation a crafted `last 99999999999` would reach the filter.
func TestParsePipelineArgsRejectsBadFirstLast(t *testing.T) {
	for _, arg := range []string{"0", "-1", "abc", ""} {
		_, _, _, errMsg := parsePipelineArgs([]string{"last", arg})
		assert.NotEmpty(t, errMsg, "last %q should be rejected", arg)
		_, _, _, errMsg = parsePipelineArgs([]string{"first", arg})
		assert.NotEmpty(t, errMsg, "first %q should be rejected", arg)
	}
	if _, _, _, errMsg := parsePipelineArgs([]string{"last", "5"}); errMsg != "" {
		t.Errorf("last 5 should be accepted, got %q", errMsg)
	}
}

func TestShowRibFirstThenCount(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64501}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64502}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.3.0/24", OutRoute: &Route{ASPath: []uint32{64504}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.4.0/24", OutRoute: &Route{ASPath: []uint32{64505}}},
	}

	src := &sliceSource{items: items}
	first := newFirstFilter(src, "3")
	count := newCountTerminal(first)

	meta := count.Meta()
	assert.Equal(t, 3, meta.Count)
}

func TestApplyFirstPositionMatters(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64501}}},
		{Peer: "p1", Family: family.IPv6Unicast, Prefix: "2001:db8::/32", OutRoute: &Route{ASPath: []uint32{64502}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
		{Peer: "p1", Family: family.IPv6Unicast, Prefix: "2001:db8::1/128", OutRoute: &Route{ASPath: []uint32{64504}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", OutRoute: &Route{ASPath: []uint32{64505}}},
	}

	// | first 3 | family ipv4-unicast: from first 3 items, keep only ipv4
	src1 := &sliceSource{items: items}
	first := newFirstFilter(src1, "3")
	fam := newFamilyFilter(first, family.IPv4Unicast.String())

	var results []RouteItem
	for {
		item, ok := fam.Next()
		if !ok {
			break
		}
		results = append(results, item)
	}
	// First 3 items: ipv4, ipv6, ipv4. Keeping ipv4 only = 2.
	require.Len(t, results, 2)
}

func TestPipeMetadataServerSide(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64501}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64502}}},
		{Peer: "p1", Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
	}

	src := &sliceSource{items: items}
	first := newFirstFilter(src, "2")
	count := newCountTerminal(first)
	meta := count.Meta()

	assert.Equal(t, 2, meta.Count, "first 2 then count should yield 2")

	countJSON, _ := json.Marshal(map[string]any{"count": meta.Count})
	result := string(countJSON)
	assert.Contains(t, result, `"count":2`)
}

// --- Spec: bounded show bgp rib dump ---

// TestShowOutboundSourceLazy verifies that the outbound source does not
// materialize the entire table at construction. After newOutboundSource,
// no items have been buffered; they load lazily per peer on Next().
//
// VALIDATES: AC-1 lazy Adj-RIB-Out source.
// PREVENTS: Regression to eager full-table materialization.
func TestShowOutboundSourceLazy(t *testing.T) {
	r := newTestRIBManager(t)

	for i, peer := range []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"} {
		prefix := "10.0." + strconv.Itoa(i) + ".0/24"
		r.ribOut[netip.MustParseAddr(peer)] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
			family.IPv4Unicast: {
				prefix: {Family: family.IPv4Unicast, Prefix: prefix, NextHop: "10.0.0.1"},
			},
		})
	}

	src := newOutboundSource(r, "*")

	// After construction, no items should be buffered yet
	assert.Empty(t, src.items, "lazy source must not pre-materialize items at construction")
	assert.Equal(t, 3, len(src.peers), "should have snapshotted 3 peers")

	// Drain and verify all items arrive
	var count int
	for {
		_, ok := src.Next()
		if !ok {
			break
		}
		count++
	}
	assert.Equal(t, 3, count, "lazy source should yield all 3 routes")
}

// TestShowJSONContractIsFlatRows pins the shape `show bgp rib` answers with:
// ONE envelope, one row per route, each row carrying `peer` and `direction` as
// FIELDS, with kebab-case attribute keys.
//
// The shape CHANGED here, deliberately. It was a top-level `adj-rib-in` and
// `adj-rib-out` pair of maps keyed by peer, and this test asserted that was
// unchanged. Owner ruling, 2026-08-23: flat rows, taken knowing the payload
// changes, because `show bgp rib | peer 10.0.0.1 | direction in | summary`
// cannot be expressed against two levels of envelope. A row operator has
// nothing to act on until whose-and-which-way are fields.
//
// VALIDATES: spec-record-answers-3-zero-alloc AC-4, and AC-8 no longer holding
// for this command by that ruling.
// PREVENTS: the two-level envelope coming back, which would silently take the
// row operators away again.
func TestShowJSONContractIsFlatRows(t *testing.T) {
	r := newTestRIBManager(t)

	// Inbound route with full attributes
	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001, testWireCommunity, testWireMED100, testWireLocalPref100)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	// Outbound route
	med := uint32(100)
	r.ribOut[netip.MustParseAddr("192.0.2.2")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"172.16.0.0/24": {Family: family.IPv4Unicast, Prefix: "172.16.0.0/24", NextHop: "10.0.0.1", MED: &med},
		},
	})

	result := r.showPipeline("*", nil)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))

	// One envelope, one row per route. Each row says whose it is and which way
	// it went, as FIELDS, which is what a row operator can select on.
	rows, ok := parsed["routes"].([]any)
	require.True(t, ok, "expected a routes list, got %v", parsed)
	require.Len(t, rows, 2, "one received route and one sent route")

	byDirection := make(map[string]map[string]any, 2)
	for _, raw := range rows {
		row, isRow := raw.(map[string]any)
		require.True(t, isRow, "every element is a route row")
		direction, hasDirection := row["direction"].(string)
		require.True(t, hasDirection, "every row names its direction: %v", row)
		require.Contains(t, row, "peer", "every row names its peer: %v", row)
		byDirection[direction] = row
	}

	route := byDirection[rpc.DirectionReceived.String()]
	require.NotNil(t, route, "no received row among %v", rows)
	assert.Equal(t, "192.0.2.1", route["peer"])
	assert.Equal(t, "10.0.0.0/24", route["prefix"])
	assert.Contains(t, route, "next-hop")
	assert.Contains(t, route, "origin")
	assert.Contains(t, route, "as-path")
	assert.Contains(t, route, "community")
	assert.Contains(t, route, "med")
	assert.Contains(t, route, "local-preference")

	outRoute := byDirection[rpc.DirectionSent.String()]
	require.NotNil(t, outRoute, "no sent row among %v", rows)
	assert.Equal(t, "192.0.2.2", outRoute["peer"])
	assert.Equal(t, "172.16.0.0/24", outRoute["prefix"])

	// The two halves are no longer separated by an envelope key, so nothing
	// should be building one.
	assert.NotContains(t, parsed, "adj-rib-in")
	assert.NotContains(t, parsed, "adj-rib-out")
}

// TestShowPipesUnchanged verifies every pipe operator produces correct
// results after the lazy source and lock-scope changes.
//
// VALIDATES: AC-3 all existing pipes unchanged.
// PREVENTS: Filter or terminal regressions from source restructuring.
func TestShowPipesUnchanged(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001, testWireCommunity, testWireMED100, testWireLocalPref100)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, []byte{24, 10, 0, 0}, true)   // 10.0.0.0/24
	peerRIB.Insert(fam, attrBytes, []byte{24, 172, 16, 0}, true) // 172.16.0.0/24
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	tests := []struct {
		name      string
		args      []string
		checkJSON func(t *testing.T, parsed map[string]any)
	}{
		{
			name: "count",
			args: []string{"received", "count"},
			checkJSON: func(t *testing.T, parsed map[string]any) {
				assert.Equal(t, float64(2), parsed["count"])
			},
		},
		{
			name: "prefix filter + count",
			args: []string{"received", "prefix", "10.0", "count"},
			checkJSON: func(t *testing.T, parsed map[string]any) {
				assert.Equal(t, float64(1), parsed["count"])
			},
		},
		{
			name: "path filter + count",
			args: []string{"received", "path", "65001", "count"},
			checkJSON: func(t *testing.T, parsed map[string]any) {
				assert.Equal(t, float64(2), parsed["count"])
			},
		},
		{
			name: "family filter + count",
			args: []string{"received", "family", "ipv4/unicast", "count"},
			checkJSON: func(t *testing.T, parsed map[string]any) {
				assert.Equal(t, float64(2), parsed["count"])
			},
		},
		{
			name: "community filter + count",
			args: []string{"received", "community", "65000:100", "count"},
			checkJSON: func(t *testing.T, parsed map[string]any) {
				assert.Equal(t, float64(2), parsed["count"])
			},
		},
		{
			name: "match filter + count",
			args: []string{"received", "match", "10.0.0.0/24", "count"},
			checkJSON: func(t *testing.T, parsed map[string]any) {
				assert.Equal(t, float64(1), parsed["count"])
			},
		},
		{
			name: "histogram",
			args: []string{"received", "histogram"},
			checkJSON: func(t *testing.T, parsed map[string]any) {
				assert.Contains(t, parsed, "histogram")
				assert.Contains(t, parsed, "count")
			},
		},
		{
			name:      "graph",
			args:      []string{"received", "graph"},
			checkJSON: nil, // graph returns text, not JSON map
		},
		{
			name: "first 1 + count",
			args: []string{"received", "first", "1", "count"},
			checkJSON: func(t *testing.T, parsed map[string]any) {
				assert.Equal(t, float64(1), parsed["count"])
			},
		},
		{
			name: "last 1 + count",
			args: []string{"received", "last", "1", "count"},
			checkJSON: func(t *testing.T, parsed map[string]any) {
				assert.Equal(t, float64(1), parsed["count"])
			},
		},
		{
			name: "json (default terminal)",
			args: nil,
			checkJSON: func(t *testing.T, parsed map[string]any) {
				// Flat rows: the answer is one envelope of routes, each row
				// naming its peer and direction, not an `adj-rib-in` map.
				rows, ok := parsed["routes"].([]any)
				require.True(t, ok, "expected a routes list, got %v", parsed)
				require.NotEmpty(t, rows)
				for _, raw := range rows {
					row, isRow := raw.(map[string]any)
					require.True(t, isRow)
					assert.Contains(t, row, "peer")
					assert.Contains(t, row, "direction")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.showPipeline("*", tt.args)
			require.NotEmpty(t, result)

			if tt.checkJSON != nil {
				var parsed map[string]any
				require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed), "result: %s", result)
				tt.checkJSON(t, parsed)
			}
		})
	}
}

// BenchmarkShowLargeTable measures per-route allocations for a multi-peer table.
//
// VALIDATES: AC-1/AC-2 lazy source and formatting allocation baseline.
func BenchmarkShowLargeTable(b *testing.B) {
	registerBuiltinCommands()

	newRIB := func() *RIBManager {
		r := &RIBManager{
			bgpPeers:     make(map[netip.Addr]*storage.PeerRIB),
			ribOut:       make(map[netip.Addr]map[family.Family]map[ribOutKey]ribOutEntry),
			ribOutSource: make(map[family.Family]map[ribOutKey]ribOutSourceRef),
			ribInPool:    make(map[redistevents.ProtocolID]map[string]*storage.PeerRIB),
			peerUp:       make(map[netip.Addr]bool),
			peerMeta:     make(map[netip.Addr]*peerMetadata),
		}
		r.maximumPaths.Store(1)
		return r
	}

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001, testWireCommunity, testWireMED100, testWireLocalPref100)

	b.Run("inbound", func(b *testing.B) {
		r := newRIB()
		for p := range 5 {
			peer := netip.MustParseAddr("192.0.2." + strconv.Itoa(p+1))
			peerRIB := storage.NewPeerRIB(peer.String())
			for i := range 200 {
				nlri := []byte{24, byte(10 + p), byte(i), 0}
				peerRIB.Insert(fam, attrBytes, nlri, true)
			}
			r.bgpPeers[peer] = peerRIB
		}
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			_ = r.showPipeline("*", []string{"received", "count"})
		}
	})

	b.Run("outbound", func(b *testing.B) {
		r := newRIB()
		med := uint32(100)
		origin := attribute.OriginIGP
		for p := range 5 {
			peer := netip.MustParseAddr("192.0.2." + strconv.Itoa(p+1))
			routes := make(map[string]*Route, 200)
			for i := range 200 {
				prefix := "10." + strconv.Itoa(p) + "." + strconv.Itoa(i) + ".0/24"
				routes[prefix] = &Route{
					Family:  fam,
					Prefix:  prefix,
					NextHop: "10.0.0.1",
					Origin:  &origin,
					ASPath:  []uint32{65001},
					MED:     &med,
					Communities: []attribute.Community{
						attribute.Community(65000<<16 | 100),
					},
				}
			}
			r.ribOut[peer] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
				fam: routes,
			})
		}
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			_ = r.showPipeline("*", []string{"sent", "count"})
		}
	})
}

// TestShowPipelineConcurrentChurn exercises the I2 fix: show/best pipelines that
// dereference pool handles (via match/community filters and best-path lookups)
// must be mutually exclusive with the writers that release those handles. The
// readers hold peerMu.RLock for the whole drain; the writer churns routes under
// peerMu.Lock, freeing and re-interning bundle handles. Run under -race -- before
// the fix (deref outside peerMu) a concurrent withdraw could free a handle the
// reader was still dereferencing.
func TestShowPipelineConcurrentChurn(t *testing.T) {
	r := newTestRIBManager(t)
	r.maximumPaths.Store(1)
	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001, testWireCommunity, testWireMED100, testWireLocalPref100)

	const peerCount = 3
	const routeCount = 40
	for p := range peerCount {
		peer := netip.MustParseAddr("192.0.2." + strconv.Itoa(p+1))
		peerRIB := storage.NewPeerRIB(peer.String())
		for i := range routeCount {
			peerRIB.Insert(fam, attrBytes, []byte{24, byte(10 + p), byte(i), 0}, true)
		}
		r.bgpPeers[peer] = peerRIB
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = r.showPipeline("*", []string{"received", "match", "65001", "count"})
				_ = r.showPipeline("*", []string{"received", "community", "65000:100", "count"})
				_ = r.bestPipeline("*", []string{"count"})
			}
		})
	}

	wg.Go(func() {
		for iter := range 1000 {
			p := iter % peerCount
			peer := netip.MustParseAddr("192.0.2." + strconv.Itoa(p+1))
			nlri := []byte{24, byte(10 + p), byte(iter % routeCount), 0}
			r.peerMu.Lock()
			if peerRIB := r.bgpPeers[peer]; peerRIB != nil {
				peerRIB.Remove(fam, nlri)                  // frees the bundle pool handles
				peerRIB.Insert(fam, attrBytes, nlri, true) // re-interns
			}
			r.peerMu.Unlock()
		}
		close(stop)
	})

	wg.Wait()
}

// TestShowRowsAreDeterministic pins the row ORDER of `show bgp rib`.
//
// It is not cosmetic and it is not free. The sources walk `r.bgpPeers`, which
// is a map, so rows arrive in Go's map order and that differs between runs. The
// two-level envelope this replaced hid it: it keyed an object by peer, and
// encoding/json sorts object keys, so the answer came out sorted without
// anybody arranging it. A flat list has no such accident.
//
// VALIDATES: an answer a test can assert and a reader can diff.
// PREVENTS: `show bgp rib` reordering itself between identical runs, which
// would make every downstream assertion flaky for a reason nothing names.
func TestShowRowsAreDeterministic(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001)
	// Inserted in an order that is not the sorted one, from several peers, so a
	// walk that simply preserved arrival order would fail this.
	for _, peer := range []string{"198.51.100.7", "192.0.2.1", "203.0.113.9", "192.0.2.2"} {
		peerRIB := storage.NewPeerRIB(peer)
		peerRIB.Insert(fam, attrBytes, []byte{24, 10, 0, 0}, true)
		peerRIB.Insert(fam, attrBytes, []byte{24, 10, 0, 1}, true)
		r.bgpPeers[netip.MustParseAddr(peer)] = peerRIB
	}

	first := showRowKeys(t, r)
	require.Len(t, first, 8, "four peers with two routes each")

	// The rows are grouped by PEER, in peer order, and each peer's routes come
	// in the order the RIB iterates them. A streamed answer cannot sort its
	// rows after the fact without holding them all, which is what streaming
	// exists to avoid, so the peer LIST is sorted at construction instead.
	peers := make([]string, 0, len(first))
	for _, key := range first {
		peer := key[:strings.IndexByte(key, '|')]
		if len(peers) == 0 || peers[len(peers)-1] != peer {
			peers = append(peers, peer)
		}
	}
	sortedPeers := append([]string(nil), peers...)
	sort.Strings(sortedPeers)
	assert.Equal(t, sortedPeers, peers, "the peers are not walked in order")
	assert.Len(t, peers, 4, "each peer appears as one contiguous group")

	// Run it repeatedly: one pass can agree with sorted order by luck of the
	// map iteration, and a single comparison would not tell the difference.
	for range 8 {
		assert.Equal(t, first, showRowKeys(t, r), "the row order changed between runs")
	}
}

// showRowKeys answers one sortable key per row of `show bgp rib`.
func showRowKeys(t *testing.T, r *RIBManager) []string {
	t.Helper()
	// The walk STREAMS, so the rows are drained from the generator rather than
	// read out of a document (spec-record-answers-3-zero-alloc AC-4).
	records, streaming := r.showPipeline("*", nil).(sdk.Records)
	require.True(t, streaming, "show bgp rib must answer with a walk")

	rows, ok := drainShowRecords(t, records)["routes"].([]any)
	require.True(t, ok, "expected a routes list")

	keys := make([]string, 0, len(rows))
	for _, raw := range rows {
		row, isRow := raw.(map[string]any)
		require.True(t, isRow)
		peer, _ := row["peer"].(string)
		direction, _ := row["direction"].(string)
		famName, _ := row["family"].(string)
		prefix, _ := row["prefix"].(string)
		var key textbuf.Buffer
		keys = append(keys, key.Str(peer).Byte('|').Str(direction).Byte('|').
			Str(famName).Byte('|').Str(prefix).String())
	}
	return keys
}
