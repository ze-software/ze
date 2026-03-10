package rib

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
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

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001, testWireCommunity)
	nlriBytes := []byte{24, 10, 0, 0}

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	// Use the source iterator
	src := newInboundSource(r, "*")
	item, ok := src.Next()
	require.True(t, ok, "expected at least one item")
	assert.Equal(t, "192.0.2.1", item.Peer)
	assert.Equal(t, "ipv4/unicast", item.Family)
	assert.Equal(t, "10.0.0.0/24", item.Prefix)
	assert.Equal(t, "received", item.Direction)
	assert.NotNil(t, item.InEntry)
}

// --- Phase 2: Filter stages ---

// TestFilterPath verifies the path filter stage.
//
// VALIDATES: path filter passes routes with matching AS path and rejects non-matching.
// PREVENTS: Path filter accepting routes without the specified AS in the path.
func TestFilterPath(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64501, 64502}}},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.2.0/24", OutRoute: &Route{ASPath: []uint32{64501}}},
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
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
		{Peer: "p1", Family: "ipv6/unicast", Prefix: "2001:db8::/32"},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.1.0/24"},
	}

	src := &sliceSource{items: items}
	f := newFamilyFilter(src, "ipv4/unicast")

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

// TestFilterCIDR verifies the CIDR/prefix filter stage.
//
// VALIDATES: cidr filter matches routes whose prefix starts with the given string.
// PREVENTS: Prefix filter matching unrelated prefixes.
func TestFilterCIDR(t *testing.T) {
	items := []RouteItem{
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "172.16.0.0/24"},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.1.0/24"},
	}

	src := &sliceSource{items: items}
	f := newCIDRFilter(src, "10.0")

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
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", OutRoute: &Route{Communities: []string{"65000:100", "65000:200"}}},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", OutRoute: &Route{Communities: []string{"65001:100"}}},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.2.0/24", OutRoute: &Route{Communities: []string{"65000:100"}}},
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
		{Peer: "192.0.2.1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
		{Peer: "192.0.2.2", Family: "ipv6/unicast", Prefix: "2001:db8::/32"},
		{Peer: "192.0.2.3", Family: "ipv4/unicast", Prefix: "10.0.1.0/24"},
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
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.1.0/24"},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.2.0/24"},
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
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", Direction: "received"},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", Direction: "received"},
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
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	tests := []struct {
		name     string
		args     []string
		wantJSON bool // false = count terminal
	}{
		{"count terminal", []string{"count"}, false},
		{"path then count", []string{"path", "64501", "count"}, false},
		{"family filter", []string{"family", "ipv4/unicast"}, true},
		{"no args = json default", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.showPipeline("*", tt.args)
			require.NoError(t, err)
			assert.NotEmpty(t, result)

			var parsed map[string]any
			require.NoError(t, json.Unmarshal([]byte(result), &parsed))
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

	_, err := r.showPipeline("*", []string{"bogus"})
	require.Error(t, err, "expected error for unknown keyword")
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
		{"cidr no value", []string{"cidr"}},
		{"match no value", []string{"match"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.showPipeline("*", tt.args)
			require.Error(t, err, "expected error for '%s'", tt.name)
		})
	}
}

// --- Phase 4: Unified rib show / rib best ---

// TestShowPipelineBothDirections verifies default scope returns both directions.
//
// VALIDATES: rib show (no scope) returns both adj-rib-in and adj-rib-out routes.
// PREVENTS: Default scope only returning one direction.
func TestShowPipelineBothDirections(t *testing.T) {
	r := newTestRIBManager(t)

	// Add inbound route
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	// Add outbound route
	r.ribOut["192.0.2.2"] = map[string]*Route{
		"ipv4/unicast:172.16.0.0/24": {
			Family: "ipv4/unicast", Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
		},
	}

	result, err := r.showPipeline("*", []string{"count"})
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	count, ok := parsed["count"]
	require.True(t, ok, "expected count key")
	assert.Equal(t, float64(2), count, "expected 2 routes (1 in + 1 out)")
}

// TestShowPipelineReceivedScope verifies received scope returns only inbound.
//
// VALIDATES: rib show received returns only adj-rib-in routes.
// PREVENTS: Outbound routes leaking into received scope.
func TestShowPipelineReceivedScope(t *testing.T) {
	r := newTestRIBManager(t)

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	r.ribOut["192.0.2.2"] = map[string]*Route{
		"ipv4/unicast:172.16.0.0/24": {
			Family: "ipv4/unicast", Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
		},
	}

	result, err := r.showPipeline("*", []string{"received", "count"})
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	count := parsed["count"]
	assert.Equal(t, float64(1), count, "expected 1 received route")
}

// TestShowPipelineSentScope verifies sent scope returns only outbound.
//
// VALIDATES: rib show sent returns only adj-rib-out routes.
// PREVENTS: Inbound routes leaking into sent scope.
func TestShowPipelineSentScope(t *testing.T) {
	r := newTestRIBManager(t)

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	r.ribOut["192.0.2.2"] = map[string]*Route{
		"ipv4/unicast:172.16.0.0/24": {
			Family: "ipv4/unicast", Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
		},
	}

	result, err := r.showPipeline("*", []string{"sent", "count"})
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	count := parsed["count"]
	assert.Equal(t, float64(1), count, "expected 1 sent route")
}

// TestShowPipelineComposed verifies composing multiple filters.
//
// VALIDATES: Multiple filters compose via pipeline (path + community + count).
// PREVENTS: Filters not chaining correctly.
func TestShowPipelineComposed(t *testing.T) {
	r := newTestRIBManager(t)

	med100 := uint32(100)
	r.ribOut["192.0.2.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {
			Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "10.0.0.1",
			ASPath: []uint32{64501, 64502}, Communities: []string{"65000:100"}, MED: &med100,
		},
		"ipv4/unicast:10.0.1.0/24": {
			Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "10.0.0.1",
			ASPath: []uint32{64501}, Communities: []string{"65001:200"}, MED: &med100,
		},
		"ipv4/unicast:10.0.2.0/24": {
			Family: "ipv4/unicast", Prefix: "10.0.2.0/24", NextHop: "10.0.0.1",
			ASPath: []uint32{64503}, Communities: []string{"65000:100"}, MED: &med100,
		},
	}

	// path 64501 community 65000:100 count -> should match only first route
	result, err := r.showPipeline("*", []string{"sent", "path", "64501", "community", "65000:100", "count"})
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	count := parsed["count"]
	assert.Equal(t, float64(1), count, "expected 1 route matching path 64501 AND community 65000:100")
}

// TestHandleCommandRibShow verifies unified rib show via handleCommand.
//
// VALIDATES: "rib show" is dispatched through handleCommand.
// PREVENTS: rib show not being wired into the command handler.
func TestHandleCommandRibShow(t *testing.T) {
	r := newTestRIBManager(t)

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	status, data, err := r.handleCommand("rib show", "*", nil)
	assert.Equal(t, statusDone, status)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &parsed))
}

// TestHandleCommandRibShowCount verifies rib show with count terminal.
//
// VALIDATES: "rib show" with count arg returns count without serializing routes.
// PREVENTS: count terminal still building full JSON output.
func TestHandleCommandRibShowCount(t *testing.T) {
	r := newTestRIBManager(t)

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	status, data, err := r.handleCommand("rib show", "*", []string{"count"})
	assert.Equal(t, statusDone, status)
	assert.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &parsed))
	count, ok := parsed["count"]
	require.True(t, ok, "expected count key")
	assert.Equal(t, float64(1), count)
}

// TestHandleCommandOldCommandsError verifies old commands return errors.
//
// VALIDATES: Old commands (rib show in, rib show out) return error.
// PREVENTS: Old commands silently working after migration.
func TestHandleCommandOldCommandsError(t *testing.T) {
	r := newTestRIBManager(t)

	for _, cmd := range []string{"rib show in", "rib show out", "rib show best", "rib adjacent inbound show", "rib adjacent outbound show"} {
		_, _, err := r.handleCommand(cmd, "*", nil)
		assert.Error(t, err, "expected error for old command %q", cmd)
	}
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
