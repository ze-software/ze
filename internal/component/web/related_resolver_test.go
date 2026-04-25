package web

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// buildPeerTree returns a config tree containing a single bgp/peer/<name>
// entry. ip may be empty to exercise the fallback-to-key path.
//
//nolint:unparam // peerName parameter kept for clarity at call sites
func buildPeerTree(t *testing.T, peerName, ip string) *config.Tree {
	t.Helper()
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")

	peer := config.NewTree()
	if ip != "" {
		conn := config.NewTree()
		remote := config.NewTree()
		remote.Set("ip", ip)
		conn.SetContainer("remote", remote)
		peer.SetContainer("connection", conn)
	}
	bgp.AddListEntry("peer", peerName, peer)
	return tree
}

// buildGroupPeerTree returns a tree with a peer inside bgp/group/<group>/peer.
// groupIP is the group-level connection/remote/ip; peerIP is the peer-level
// override (empty triggers path-inherit walking up to the group).
func buildGroupPeerTree(t *testing.T, groupName, peerName, groupIP, peerIP string) *config.Tree {
	t.Helper()
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	group := config.NewTree()
	if groupIP != "" {
		conn := config.NewTree()
		remote := config.NewTree()
		remote.Set("ip", groupIP)
		conn.SetContainer("remote", remote)
		group.SetContainer("connection", conn)
	}

	peer := config.NewTree()
	if peerIP != "" {
		conn := config.NewTree()
		remote := config.NewTree()
		remote.Set("ip", peerIP)
		conn.SetContainer("remote", remote)
		peer.SetContainer("connection", conn)
	}
	group.AddListEntry("peer", peerName, peer)
	bgp.AddListEntry("group", groupName, group)
	return tree
}

func mustResolve(t *testing.T, schema *config.Schema, tree *config.Tree, tool *config.RelatedTool, contextPath []string) *Resolution {
	t.Helper()
	r := NewRelatedResolver(schema, tree)
	res, err := r.Resolve(tool, contextPath)
	require.NoError(t, err)
	return res
}

// TestRelatedToolResolve_PeerSelectorFromRemoteIP verifies the row-context
// `${path:connection/remote/ip|key}` placeholder substitutes the peer's
// configured remote IP into the command template. This is the canonical
// flow for the day-one BGP tools.
//
// VALIDATES: AC-3 (rows show tools, command resolves to the IP).
// PREVENTS: A regression that strips the placeholder or substitutes the
// list key when the IP is set, both of which would dispatch the wrong peer.
func TestRelatedToolResolve_PeerSelectorFromRemoteIP(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")

	tool := &config.RelatedTool{
		ID:      "peer-detail",
		Label:   "Peer Detail",
		Command: "peer ${path:connection/remote/ip|key} detail",
	}
	res := mustResolve(t, schema, tree, tool, []string{"bgp", "peer", "thomas"})
	assert.Equal(t, "peer 10.0.0.1 detail", res.Command)
	assert.False(t, res.Disabled)
}

// TestRelatedToolResolve_FallbackToKey verifies the `|key` fallback uses the
// peer's list key when the relative path resolves to an empty value. This
// is what the spec calls "fallback from configured field value to list key"
// and is required for peers that haven't been given a remote IP yet.
//
// VALIDATES: AC-4 (tools work or fall back when remote IP is unset).
// PREVENTS: Operators losing the action affordance entirely on freshly
// created peer rows.
func TestRelatedToolResolve_FallbackToKey(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "")

	tool := &config.RelatedTool{
		ID:      "peer-detail",
		Label:   "Peer Detail",
		Command: "peer ${path:connection/remote/ip|key} detail",
	}
	res := mustResolve(t, schema, tree, tool, []string{"bgp", "peer", "thomas"})
	assert.Equal(t, "peer thomas detail", res.Command)
}

// TestRelatedToolResolve_PathInheritFromGroup verifies the path-inherit
// placeholder walks one parent list entry when the row's own value is
// missing. For bgp/group/peer, the peer can omit connection/remote/ip and
// inherit the group's value. This is Spec D7.
//
// VALIDATES: D7 inheritance (peer -> group -> key).
// PREVENTS: A grouped peer with the IP defined on the group looking
// non-functional in the workbench.
func TestRelatedToolResolve_PathInheritFromGroup(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildGroupPeerTree(t, "g1", "p1", "192.0.2.1", "")

	tool := &config.RelatedTool{
		ID:      "peer-detail",
		Label:   "Peer Detail",
		Command: "peer ${path-inherit:connection/remote/ip|key} detail",
	}
	res := mustResolve(t, schema, tree, tool, []string{"bgp", "group", "g1", "peer", "p1"})
	assert.Equal(t, "peer 192.0.2.1 detail", res.Command)
}

// TestRelatedToolResolve_PathInheritFallsBackToKey verifies the
// path-inherit placeholder ends at the row key when neither the peer nor
// the group carries the field.
func TestRelatedToolResolve_PathInheritFallsBackToKey(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildGroupPeerTree(t, "g1", "p1", "", "")

	tool := &config.RelatedTool{
		ID:      "peer-detail",
		Label:   "Peer Detail",
		Command: "peer ${path-inherit:connection/remote/ip|key} detail",
	}
	res := mustResolve(t, schema, tree, tool, []string{"bgp", "group", "g1", "peer", "p1"})
	assert.Equal(t, "peer p1 detail", res.Command)
}

// TestRelatedToolResolve_RejectsUnsafeValue verifies the resolver rejects
// resolved placeholder values that contain whitespace or shell
// metacharacters. Command construction never reaches a shell, but rejecting
// these still blocks attempts at injection through downstream renderers
// (Spec Resolved-Value Validation).
//
// VALIDATES: Security gate -- resolved values are tokens, not free text.
// PREVENTS: A peer name like `evil; rm -rf` slipping through into the
// constructed command string.
func TestRelatedToolResolve_RejectsUnsafeValue(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "evil; rm -rf /")

	tool := &config.RelatedTool{
		ID:      "peer-detail",
		Label:   "Peer Detail",
		Command: "peer ${path:connection/remote/ip|key} detail",
	}
	r := NewRelatedResolver(schema, tree)
	_, err = r.Resolve(tool, []string{"bgp", "peer", "thomas"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe")
}

// TestRelatedToolResolve_PathDepthExceeded verifies that a relative path
// containing more than 16 segments is rejected at parse time. The boundary
// table caps depth to keep placeholder evaluation predictable.
func TestRelatedToolResolve_PathDepthExceeded(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")

	deepPath := strings.Repeat("a/", 17) // 17 segments
	deepPath = strings.TrimSuffix(deepPath, "/")
	tool := &config.RelatedTool{
		ID:      "peer-detail",
		Label:   "Peer Detail",
		Command: "peer ${path:" + deepPath + "} detail",
	}
	r := NewRelatedResolver(schema, tree)
	_, err = r.Resolve(tool, []string{"bgp", "peer", "thomas"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth")
}

// TestRelatedToolResolve_DisabledOnMissingValue verifies that when a
// placeholder fails to resolve and the descriptor's empty=disable default
// applies, the tool is marked Disabled instead of returning a partial
// command.
func TestRelatedToolResolve_DisabledOnMissingValue(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "")

	tool := &config.RelatedTool{
		ID:      "peer-detail",
		Label:   "Peer Detail",
		Command: "peer ${path:connection/remote/ip} detail", // no |key fallback
		Empty:   config.RelatedEmptyDisable,
	}
	r := NewRelatedResolver(schema, tree)
	res, err := r.Resolve(tool, []string{"bgp", "peer", "thomas"})
	require.NoError(t, err)
	assert.True(t, res.Disabled, "tool must be disabled when placeholder unresolvable")
	assert.NotEmpty(t, res.DisabledReason)
}

// TestRelatedToolResolve_ResolvedCommandLengthLimit verifies the total
// resolved command length cap (4096 chars, Boundary Tests row) is
// enforced. The resolver must reject a tool whose substituted command
// would exceed the cap.
func TestRelatedToolResolve_ResolvedCommandLengthLimit(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	// Build a tree where the peer key is 250 chars; the template uses ${key}
	// repeated enough to push the resolved command past 4096 chars while
	// the template itself stays under the 512-char limit.
	longKey := strings.Repeat("a", 250)
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", longKey, config.NewTree())

	cmd := "peer"
	for range 17 { // 17 * 250 = 4250 > 4096
		cmd += " ${key}"
	}
	require.LessOrEqual(t, len(cmd), 512, "test setup keeps template under cap")
	tool := &config.RelatedTool{
		ID:      "x",
		Label:   "X",
		Command: cmd,
	}
	r := NewRelatedResolver(schema, tree)
	_, err = r.Resolve(tool, []string{"bgp", "peer", longKey})
	require.Error(t, err, "resolver must reject when resolved command exceeds 4096 chars")
}

// TestRelatedToolResolve_KeyPlaceholder verifies the bare ${key}
// placeholder substitutes the trailing context-path segment (the row's
// list key). This is the simplest placeholder source and acts as a sanity
// gate for the substitution loop.
func TestRelatedToolResolve_KeyPlaceholder(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")

	tool := &config.RelatedTool{
		ID:      "x",
		Label:   "X",
		Command: "peer ${key} flush",
	}
	res := mustResolve(t, schema, tree, tool, []string{"bgp", "peer", "thomas"})
	assert.Equal(t, "peer thomas flush", res.Command)
}

// TestRelatedToolResolve_CurrentPathPlaceholder verifies ${current-path}
// expands to the slash-joined context path. This is the diagnostic-only
// placeholder used for command labeling.
func TestRelatedToolResolve_CurrentPathPlaceholder(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")

	tool := &config.RelatedTool{
		ID:      "x",
		Label:   "X",
		Command: "show context ${current-path}",
	}
	// `/` is in the allowed value-character set; the resolver must NOT
	// reject the substituted value.
	res := mustResolve(t, schema, tree, tool, []string{"bgp", "peer", "thomas"})
	assert.Equal(t, "show context bgp/peer/thomas", res.Command)
}

// TestRelatedToolResolve_LeafAndValuePlaceholders verifies the field-level
// placeholders ${leaf} and ${value} substitute the last context-path
// segment (the leaf name) and that leaf's tree value respectively. These
// placeholders are reserved for future field-level tools but the resolver
// supports them today.
func TestRelatedToolResolve_LeafAndValuePlaceholders(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	// Build a tree with a single bgp/router-id leaf so ${value} has
	// something to read at the field path.
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.Set("router-id", "1.2.3.4")

	tool := &config.RelatedTool{
		ID:      "x",
		Label:   "X",
		Command: "echo ${leaf}=${value}",
	}
	res := mustResolve(t, schema, tree, tool, []string{"bgp", "router-id"})
	assert.Equal(t, "echo router-id=1.2.3.4", res.Command)
}

// TestRelatedToolResolve_EmptyAllowCollapsesGap verifies that when a
// placeholder fails to resolve and the descriptor's Empty=allow lets the
// command continue, the substitution gap is collapsed to a single space
// so the dispatcher tokenizer sees `peer detail` rather than `peer  detail`.
func TestRelatedToolResolve_EmptyAllowCollapsesGap(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "")

	tool := &config.RelatedTool{
		ID:      "x",
		Label:   "X",
		Command: "peer ${path:connection/remote/ip} detail",
		Empty:   config.RelatedEmptyAllow,
	}
	r := NewRelatedResolver(schema, tree)
	res, err := r.Resolve(tool, []string{"bgp", "peer", "thomas"})
	require.NoError(t, err)
	require.False(t, res.Disabled)
	assert.Equal(t, "peer detail", res.Command, "missing placeholder must not leave a double space")
}
