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
			result := r.showPipeline("*", tt.args)
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

	result := r.showPipeline("*", []string{"bogus"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
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
		{"cidr no value", []string{"cidr"}},
		{"match no value", []string{"match"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.showPipeline("*", tt.args)
			var parsed map[string]any
			require.NoError(t, json.Unmarshal([]byte(result), &parsed))
			_, hasError := parsed["error"]
			assert.True(t, hasError, "expected error for '%s': %s", tt.name, result)
		})
	}
}

// --- Phase 4: Unified rib show / rib show best ---

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
	r.ribOut["192.0.2.2"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"172.16.0.0/24": {
				Family: "ipv4/unicast", Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
			},
		},
	}

	result := r.showPipeline("*", []string{"count"})
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

	r.ribOut["192.0.2.2"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"172.16.0.0/24": {
				Family: "ipv4/unicast", Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
			},
		},
	}

	result := r.showPipeline("*", []string{"received", "count"})
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

	r.ribOut["192.0.2.2"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"172.16.0.0/24": {
				Family: "ipv4/unicast", Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
			},
		},
	}

	result := r.showPipeline("*", []string{"sent", "count"})
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
	r.ribOut["192.0.2.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {
				Family: "ipv4/unicast", Prefix: "10.0.0.0/24", NextHop: "10.0.0.1",
				ASPath: []uint32{64501, 64502}, Communities: []string{"65000:100"}, MED: &med100,
			},
			"10.0.1.0/24": {
				Family: "ipv4/unicast", Prefix: "10.0.1.0/24", NextHop: "10.0.0.1",
				ASPath: []uint32{64501}, Communities: []string{"65001:200"}, MED: &med100,
			},
			"10.0.2.0/24": {
				Family: "ipv4/unicast", Prefix: "10.0.2.0/24", NextHop: "10.0.0.1",
				ASPath: []uint32{64503}, Communities: []string{"65000:100"}, MED: &med100,
			},
		},
	}

	// path 64501 community 65000:100 count -> should match only first route
	result := r.showPipeline("*", []string{"sent", "path", "64501", "community", "65000:100", "count"})
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
// VALIDATES: Old commands (rib show in, rib show out, rib show best) return pipeline errors;
// truly unknown commands return Go errors.
// PREVENTS: Old commands silently working after migration.
func TestHandleCommandOldCommandsError(t *testing.T) {
	r := newTestRIBManager(t)

	// Pipeline-parsed old keywords: routed through "rib show" with args,
	// parsePipelineArgs returns "unknown keyword" error in JSON data.
	for _, keyword := range []string{"in", "out", "best"} {
		status, data, err := r.handleCommand("rib show", "*", []string{keyword})
		assert.NoError(t, err, "pipeline error for %q should not be a Go error", keyword)
		assert.Equal(t, statusDone, status, "pipeline error status for %q", keyword)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(data), &parsed), "data should be valid JSON for %q", keyword)
		_, hasError := parsed["error"]
		assert.True(t, hasError, "data should contain error key for old keyword %q", keyword)
	}

	// Truly unknown commands: fall through to default case in handleCommand.
	for _, cmd := range []string{"rib adjacent inbound show", "rib adjacent outbound show"} {
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
				{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", OutRoute: &Route{ASPath: []uint32{64501, 64502}}},
				{Peer: "p2", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", OutRoute: &Route{ASPath: []uint32{64503}}},
			},
			want: 1,
		},
		{
			name:    "match community value",
			pattern: "65000:100",
			items: []RouteItem{
				{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", OutRoute: &Route{Communities: []string{"65000:100"}}},
				{Peer: "p2", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", OutRoute: &Route{Communities: []string{"65001:200"}}},
			},
			want: 1,
		},
		{
			name:    "match origin value",
			pattern: "igp",
			items: []RouteItem{
				{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", OutRoute: &Route{Origin: "igp"}},
				{Peer: "p2", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", OutRoute: &Route{Origin: "egp"}},
			},
			want: 1,
		},
		{
			name:    "match MED value",
			pattern: "100",
			items: []RouteItem{
				{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", OutRoute: &Route{MED: &med100}},
				{Peer: "p2", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", OutRoute: &Route{}},
			},
			want: 1,
		},
		{
			name:    "match local-pref value",
			pattern: "200",
			items: []RouteItem{
				{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", OutRoute: &Route{LocalPreference: &localPref200}},
				{Peer: "p2", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", OutRoute: &Route{}},
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
	_, _, errMsg := parsePipelineArgs([]string{"path", "abc"})
	assert.NotEmpty(t, errMsg, "expected error for invalid ASN")
	assert.Contains(t, errMsg, "invalid ASN", "error should mention invalid ASN")

	// Also verify via bestPipelineArgs
	_, errMsg = parseBestPipelineArgs([]string{"path", "abc"})
	assert.NotEmpty(t, errMsg, "expected error for invalid ASN in best pipeline")
	assert.Contains(t, errMsg, "invalid ASN", "error should mention invalid ASN")
}

// TestFilterMatchCrossFieldInEntry verifies match filter checks InEntry pool attributes.
//
// VALIDATES: match filter searches InEntry attributes (origin, AS-path, communities, MED, local-pref).
// PREVENTS: match filter only working for OutRoute but not InEntry.
func TestFilterMatchCrossFieldInEntry(t *testing.T) {
	r := newTestRIBManager(t)

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001, testWireCommunity, testWireMED100, testWireLocalPref100)
	nlriBytes := []byte{24, 10, 0, 0}

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	// Match on AS-path value "65001"
	result := r.showPipeline("*", []string{"received", "match", "65001", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.Equal(t, float64(1), parsed["count"], "expected match on AS-path 65001")

	// Match on community "65000:100"
	result = r.showPipeline("*", []string{"received", "match", "65000:100", "count"})
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.Equal(t, float64(1), parsed["count"], "expected match on community 65000:100")

	// Match on origin "igp"
	result = r.showPipeline("*", []string{"received", "match", "igp", "count"})
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.Equal(t, float64(1), parsed["count"], "expected match on origin igp")
}

// --- Phase 4b: Terminal ordering validation ---

// TestParsePipelineTerminalBeforeFilter verifies filter after terminal returns error.
//
// VALIDATES: AC-10 — terminal before filter is invalid.
// PREVENTS: Silently ignoring filters placed after a terminal stage.
func TestParsePipelineTerminalBeforeFilter(t *testing.T) {
	_, _, errMsg := parsePipelineArgs([]string{"count", "path", "64501"})
	assert.Contains(t, errMsg, "filter after terminal")
}

// TestParsePipelineTwoTerminals verifies multiple terminals return error.
//
// VALIDATES: AC-10 — multiple terminals not allowed.
// PREVENTS: Ambiguous pipeline with two terminal stages.
func TestParsePipelineTwoTerminals(t *testing.T) {
	_, _, errMsg := parsePipelineArgs([]string{"count", "json"})
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

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	// Path filter for ASN 99999 — no routes have this ASN
	result := r.showPipeline("*", []string{"received", "path", "99999", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
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
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	// Add outbound route
	r.ribOut["192.0.2.2"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"172.16.0.0/24": {
				Family: "ipv4/unicast", Prefix: "172.16.0.0/24", NextHop: "10.0.0.1",
			},
		},
	}

	// Explicit sent-received scope
	result := r.showPipeline("*", []string{"sent-received", "count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
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
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	nlri1 := []byte{24, 10, 0, 0}   // 10.0.0.0/24
	nlri2 := []byte{24, 172, 16, 0} // 172.16.0.0/24

	// Peer 1: 10.0.0.0/24 with community 65000:100
	attr1 := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001, testWireCommunity)
	peerRIB1 := storage.NewPeerRIB("192.0.2.1")
	peerRIB1.Insert(family, attr1, nlri1)
	r.ribInPool["192.0.2.1"] = peerRIB1

	// Peer 2: 172.16.0.0/24 with no community (just origin + nexthop)
	attr2 := concatBytes(testWireOriginIGP, testWireNextHop)
	peerRIB2 := storage.NewPeerRIB("192.0.2.2")
	peerRIB2.Insert(family, attr2, nlri2)
	r.ribInPool["192.0.2.2"] = peerRIB2

	// Best pipeline with community filter: should only return the route with 65000:100
	result := r.bestPipeline("*", []string{"community", "65000:100"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))

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

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	nlri1 := []byte{24, 10, 0, 0}   // 10.0.0.0/24
	nlri2 := []byte{24, 172, 16, 0} // 172.16.0.0/24

	// Single peer with two prefixes
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlri1)
	peerRIB.Insert(family, attrBytes, nlri2)
	r.ribInPool["192.0.2.1"] = peerRIB

	result := r.bestPipeline("*", []string{"count"})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))

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
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))

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
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", Direction: "received",
			OutRoute: &Route{ASPath: []uint32{64501, 64502, 64503}}},
		{Peer: "p1", Family: "ipv4/unicast", Prefix: "10.0.1.0/24", Direction: "received",
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
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	result := r.showPipeline("*", []string{"received", "graph"})
	require.NotEmpty(t, result)

	// Should contain AS65001 from the injected route's AS path
	assert.Contains(t, result, "AS65001")
	// Should contain box-drawing characters
	assert.Contains(t, result, "\u250C")
}

// TestGraphTerminalViaBestPipeline verifies graph terminal works with best-path pipeline.
//
// VALIDATES: AC-6 "rib show best graph works."
// PREVENTS: Graph terminal only working with show, not best.
func TestGraphTerminalViaBestPipeline(t *testing.T) {
	r := newTestRIBManager(t)

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001)
	nlriBytes := []byte{24, 10, 0, 0}
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	result := r.bestPipeline("*", []string{"graph"})
	require.NotEmpty(t, result)

	// Should contain AS65001
	assert.Contains(t, result, "AS65001")
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
