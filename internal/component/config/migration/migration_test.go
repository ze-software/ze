package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// VALIDATES: Config migration transformations produce correct output trees.
// PREVENTS: Regressions in ExaBGP → Ze config migration pipeline.

// --- Migrate() ---

// TestMigrate_NilTree verifies nil tree returns ErrNilTree.
func TestMigrate_NilTree(t *testing.T) {
	_, err := Migrate(nil)
	require.ErrorIs(t, err, ErrNilTree)
}

// TestMigrate_EmptyTree verifies empty tree passes through unchanged.
func TestMigrate_EmptyTree(t *testing.T) {
	tree := config.NewTree()
	result, err := Migrate(tree)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Applied)
	assert.Len(t, result.Skipped, len(transformations))
}

// TestMigrate_NeighborToPeer verifies neighbor blocks become peer blocks.
func TestMigrate_NeighborToPeer(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65002")
	tree.AddListEntry("neighbor", "10.0.0.1", peerTree)

	result, err := Migrate(tree)
	require.NoError(t, err)
	assert.Contains(t, result.Applied, "neighbor->peer")

	// "neighbor" should be gone; "peer" is now inside bgp {} (from wrap-bgp-block)
	assert.Empty(t, result.Tree.GetList("neighbor"))
	bgp := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers := bgp.GetList("peer")
	require.Contains(t, peers, "10.0.0.1")
	peerAS, ok := peers["10.0.0.1"].Get("peer-as")
	assert.True(t, ok)
	assert.Equal(t, "65002", peerAS)
}

// TestMigrate_PeerGlobToMatch verifies glob patterns move to template.match.
func TestMigrate_PeerGlobToMatch(t *testing.T) {
	tree := config.NewTree()

	// Regular peer — should stay
	regular := config.NewTree()
	regular.Set("peer-as", "65002")
	tree.AddListEntry("peer", "10.0.0.1", regular)

	// Glob peer — should move to template.match
	glob := config.NewTree()
	glob.Set("peer-as", "65000")
	tree.AddListEntry("peer", "10.0.0.0/8", glob)

	result, err := Migrate(tree)
	require.NoError(t, err)

	// Regular peer stays under "peer" (inside bgp {} after wrap)
	bgp := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgp, "bgp block should exist after wrap-bgp-block")
	peers := bgp.GetList("peer")
	assert.Contains(t, peers, "10.0.0.1")

	// Glob should be in template.bgp.peer (after template->new-format migration)
	tmpl := result.Tree.GetContainer("template")
	require.NotNil(t, tmpl)
	tmplBGP := tmpl.GetContainer("bgp")
	require.NotNil(t, tmplBGP)
	tmplPeers := tmplBGP.GetList("peer")
	assert.Contains(t, tmplPeers, "10.0.0.0/8")
}

// TestMigrate_TemplateNeighborToGroup verifies template.neighbor → template.group.
func TestMigrate_TemplateNeighborToGroup(t *testing.T) {
	tree := config.NewTree()
	tmpl := config.NewTree()
	tmplNeighbor := config.NewTree()
	tmplNeighbor.Set("hold-time", "30")
	tmpl.AddListEntry("neighbor", "my-group", tmplNeighbor)
	tree.SetContainer("template", tmpl)

	result, err := Migrate(tree)
	require.NoError(t, err)
	assert.Contains(t, result.Applied, "template.neighbor->group")

	// template.neighbor should be gone, template.group should exist
	// (then template->new-format converts group → bgp.peer)
	resultTmpl := result.Tree.GetContainer("template")
	require.NotNil(t, resultTmpl)
	assert.Empty(t, resultTmpl.GetList("neighbor"))
}

// --- Static route extraction ---

// TestExtractStaticRoutes_IPv4 verifies IPv4 static → announce.ipv4.unicast.
func TestExtractStaticRoutes_IPv4(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()

	static := config.NewTree()
	routeAttrs := config.NewTree()
	routeAttrs.Set("next-hop", "192.168.1.1")
	static.AddListEntry("route", "10.0.0.0/24", routeAttrs)
	peerTree.SetContainer("static", static)
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	peer := result.GetList("peer")["10.0.0.1"]
	require.NotNil(t, peer)

	// static should be removed
	assert.Nil(t, peer.GetContainer("static"))

	// announce.ipv4.unicast should have the route
	announce := peer.GetContainer("announce")
	require.NotNil(t, announce)
	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	unicast := ipv4.GetList("unicast")
	require.Contains(t, unicast, "10.0.0.0/24")
}

// TestExtractStaticRoutes_IPv6 verifies IPv6 static → announce.ipv6.unicast.
func TestExtractStaticRoutes_IPv6(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()

	static := config.NewTree()
	routeAttrs := config.NewTree()
	routeAttrs.Set("next-hop", "2001:db8::1")
	static.AddListEntry("route", "2001:db8::/32", routeAttrs)
	peerTree.SetContainer("static", static)
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	peer := result.GetList("peer")["10.0.0.1"]
	announce := peer.GetContainer("announce")
	require.NotNil(t, announce)
	ipv6 := announce.GetContainer("ipv6")
	require.NotNil(t, ipv6)
	unicast := ipv6.GetList("unicast")
	require.Contains(t, unicast, "2001:db8::/32")
}

// TestExtractStaticRoutes_VPN verifies routes with RD → announce.ipv4.mpls-vpn.
func TestExtractStaticRoutes_VPN(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()

	static := config.NewTree()
	routeAttrs := config.NewTree()
	routeAttrs.Set("rd", "100:100")
	routeAttrs.Set("label", "100")
	static.AddListEntry("route", "10.0.0.0/24", routeAttrs)
	peerTree.SetContainer("static", static)
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	peer := result.GetList("peer")["10.0.0.1"]
	announce := peer.GetContainer("announce")
	require.NotNil(t, announce)
	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	vpn := ipv4.GetList("mpls-vpn")
	require.Contains(t, vpn, "10.0.0.0/24")
}

// TestExtractStaticRoutes_Labeled verifies routes with label only → announce.ipv4.nlri-mpls.
func TestExtractStaticRoutes_Labeled(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()

	static := config.NewTree()
	routeAttrs := config.NewTree()
	routeAttrs.Set("label", "100")
	static.AddListEntry("route", "10.0.0.0/24", routeAttrs)
	peerTree.SetContainer("static", static)
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	peer := result.GetList("peer")["10.0.0.1"]
	announce := peer.GetContainer("announce")
	require.NotNil(t, announce)
	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	labeled := ipv4.GetList("nlri-mpls")
	require.Contains(t, labeled, "10.0.0.0/24")
}

// TestExtractStaticRoutes_Multicast verifies multicast range → announce.ipv4.multicast.
func TestExtractStaticRoutes_Multicast(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()

	static := config.NewTree()
	routeAttrs := config.NewTree()
	static.AddListEntry("route", "239.0.0.0/8", routeAttrs)
	peerTree.SetContainer("static", static)
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	peer := result.GetList("peer")["10.0.0.1"]
	announce := peer.GetContainer("announce")
	require.NotNil(t, announce)
	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	mcast := ipv4.GetList("multicast")
	require.Contains(t, mcast, "239.0.0.0/8")
}

// TestExtractStaticRoutes_NilTree verifies nil returns ErrNilTree.
func TestExtractStaticRoutes_NilTree(t *testing.T) {
	_, err := ExtractStaticRoutes(nil)
	require.ErrorIs(t, err, ErrNilTree)
}

// TestExtractStaticRoutes_NoStatic verifies no-op when no static blocks exist.
func TestExtractStaticRoutes_NoStatic(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65002")
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	peer := result.GetList("peer")["10.0.0.1"]
	assert.Nil(t, peer.GetContainer("announce"), "no announce block when no static")
}

// --- API migration ---

// TestMigrateAPIBlocks_NilTree verifies nil returns ErrNilTree.
func TestMigrateAPIBlocks_NilTree(t *testing.T) {
	_, err := MigrateAPIBlocks(nil)
	require.ErrorIs(t, err, ErrNilTree)
}

// TestMigrateAPIBlocks_AnonymousProcess verifies anonymous process block migration.
// api { processes [ foo ]; neighbor-changes; } becomes process foo { receive [ state ]; }.
func TestMigrateAPIBlocks_AnonymousProcess(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()

	apiTree := config.NewTree()
	apiTree.SetSlice("processes", []string{"foo"})
	apiTree.Set("neighbor-changes", "")
	peerTree.AddListEntry("process", config.KeyDefault, apiTree)
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := result.GetList("peer")["10.0.0.1"]
	processList := peer.GetList("process")

	// Old anonymous block should be gone
	_, hasDefault := processList[config.KeyDefault]
	assert.False(t, hasDefault, "anonymous block should be replaced")

	// New named block should exist
	require.Contains(t, processList, "foo")
	fooBlock := processList["foo"]

	// Should have receive [ state ]
	recv, ok := fooBlock.Get("receive")
	assert.True(t, ok)
	assert.Contains(t, recv, "state")
}

// TestMigrateAPIBlocks_FormatParsed verifies parsed flag → format parsed.
func TestMigrateAPIBlocks_FormatParsed(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()

	apiTree := config.NewTree()
	apiTree.SetSlice("processes", []string{"foo"})
	recv := config.NewTree()
	recv.Set("parsed", "")
	recv.Set("update", "")
	apiTree.SetContainer("receive", recv)
	peerTree.AddListEntry("process", config.KeyDefault, apiTree)
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := result.GetList("peer")["10.0.0.1"]
	fooBlock := peer.GetList("process")["foo"]
	require.NotNil(t, fooBlock)

	content := fooBlock.GetContainer("content")
	require.NotNil(t, content)
	format, ok := content.Get("format")
	assert.True(t, ok)
	assert.Equal(t, "parsed", format)
}

// TestMigrateAPIBlocks_FormatRaw verifies packets flag → format raw.
func TestMigrateAPIBlocks_FormatRaw(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()

	apiTree := config.NewTree()
	apiTree.SetSlice("processes", []string{"bar"})
	recv := config.NewTree()
	recv.Set("packets", "")
	recv.Set("update", "")
	apiTree.SetContainer("receive", recv)
	peerTree.AddListEntry("process", config.KeyDefault, apiTree)
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := result.GetList("peer")["10.0.0.1"]
	barBlock := peer.GetList("process")["bar"]
	content := barBlock.GetContainer("content")
	require.NotNil(t, content)
	format, ok := content.Get("format")
	assert.True(t, ok)
	assert.Equal(t, "raw", format)
}

// TestMigrateAPIBlocks_FormatFull verifies consolidate flag → format full.
func TestMigrateAPIBlocks_FormatFull(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()

	apiTree := config.NewTree()
	apiTree.SetSlice("processes", []string{"baz"})
	recv := config.NewTree()
	recv.Set("consolidate", "")
	apiTree.SetContainer("receive", recv)
	peerTree.AddListEntry("process", config.KeyDefault, apiTree)
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := MigrateAPIBlocks(tree)
	require.NoError(t, err)

	peer := result.GetList("peer")["10.0.0.1"]
	bazBlock := peer.GetList("process")["baz"]
	content := bazBlock.GetContainer("content")
	require.NotNil(t, content)
	format, ok := content.Get("format")
	assert.True(t, ok)
	assert.Equal(t, "full", format)
}

// TestMigrateAPIBlocks_DuplicateProcess verifies duplicate detection.
func TestMigrateAPIBlocks_DuplicateProcess(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()

	apiTree := config.NewTree()
	apiTree.SetSlice("processes", []string{"foo", "foo"})
	peerTree.AddListEntry("process", config.KeyDefault, apiTree)
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	_, err := MigrateAPIBlocks(tree)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDuplicateProcess)
}

// --- DryRun ---

// TestDryRun_NilTree verifies nil returns ErrNilTree.
func TestDryRun_NilTree(t *testing.T) {
	_, err := DryRun(nil)
	require.ErrorIs(t, err, ErrNilTree)
}

// TestDryRun_NoChanges verifies all-done case.
func TestDryRun_NoChanges(t *testing.T) {
	tree := config.NewTree()
	result, err := DryRun(tree)
	require.NoError(t, err)
	assert.True(t, result.WouldSucceed)
	assert.Empty(t, result.WouldApply)
	assert.Len(t, result.AlreadyDone, len(transformations))
}

// TestDryRun_WouldApply verifies detection of needed changes.
func TestDryRun_WouldApply(t *testing.T) {
	tree := config.NewTree()
	peerTree := config.NewTree()
	tree.AddListEntry("neighbor", "10.0.0.1", peerTree)

	result, err := DryRun(tree)
	require.NoError(t, err)
	assert.True(t, result.WouldSucceed)
	assert.Contains(t, result.WouldApply, "neighbor->peer")
}

// --- NeedsMigration ---

// TestNeedsMigration_True verifies detection when migration needed.
func TestNeedsMigration_True(t *testing.T) {
	tree := config.NewTree()
	tree.AddListEntry("neighbor", "10.0.0.1", config.NewTree())
	assert.True(t, NeedsMigration(tree))
}

// TestNeedsMigration_False verifies no detection when already migrated.
func TestNeedsMigration_False(t *testing.T) {
	tree := config.NewTree()
	assert.False(t, NeedsMigration(tree))
}

// TestNeedsMigration_Nil verifies nil returns false.
func TestNeedsMigration_Nil(t *testing.T) {
	assert.False(t, NeedsMigration(nil))
}

// --- ListTransformations ---

// TestListTransformations verifies it returns a copy with all entries.
func TestListTransformations(t *testing.T) {
	list := ListTransformations()
	assert.Len(t, list, len(transformations))

	// Verify it's a copy (modifying doesn't affect original)
	list[0].Name = "modified"
	assert.NotEqual(t, "modified", transformations[0].Name)
}

// --- Detect helpers ---

// TestIsGlobPattern verifies glob detection.
func TestIsGlobPattern(t *testing.T) {
	assert.True(t, isGlobPattern("10.0.0.0/8"))
	assert.True(t, isGlobPattern("*"))
	assert.True(t, isGlobPattern("10.0.0.*"))
	assert.False(t, isGlobPattern("10.0.0.1"))
	assert.False(t, isGlobPattern("my-peer"))
}

// TestIsIPv6Prefix verifies IPv6 detection.
func TestIsIPv6Prefix(t *testing.T) {
	assert.True(t, isIPv6Prefix("2001:db8::/32"))
	assert.True(t, isIPv6Prefix("::ffff:10.0.0.1"))
	assert.False(t, isIPv6Prefix("10.0.0.0/24"))
}

// TestIsMulticastPrefix verifies multicast range detection.
func TestIsMulticastPrefix(t *testing.T) {
	assert.True(t, isMulticastPrefix("224.0.0.0/4"))
	assert.True(t, isMulticastPrefix("239.255.255.255/32"))
	assert.True(t, isMulticastPrefix("ff00::/8"))
	assert.False(t, isMulticastPrefix("10.0.0.0/24"))
	assert.False(t, isMulticastPrefix("2001:db8::/32"))
	assert.False(t, isMulticastPrefix("invalid"))
}

// TestDetectSAFI verifies SAFI detection logic.
func TestDetectSAFI(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		hasRD    bool
		hasLabel bool
		expected string
	}{
		{"unicast", "10.0.0.0/24", false, false, "unicast"},
		{"vpn", "10.0.0.0/24", true, true, "mpls-vpn"},
		{"labeled", "10.0.0.0/24", false, true, "nlri-mpls"},
		{"multicast", "239.0.0.0/8", false, false, "multicast"},
		{"multicast beats rd", "239.0.0.0/8", true, false, "multicast"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, detectSAFI(tt.prefix, tt.hasRD, tt.hasLabel))
		})
	}
}

// --- Wrap BGP block ---

// TestWrapBGPBlock verifies BGP elements move into bgp {} container.
func TestWrapBGPBlock(t *testing.T) {
	tree := config.NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.Set("local-as", "65001")
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65002")
	tree.AddListEntry("peer", "10.0.0.1", peerTree)

	result, err := wrapBGPBlock(tree)
	require.NoError(t, err)

	// Root should not have BGP elements
	_, hasRID := result.Get("router-id")
	assert.False(t, hasRID, "router-id should be inside bgp block")
	_, hasAS := result.Get("local-as")
	assert.False(t, hasAS, "local-as should be inside bgp block")
	assert.Empty(t, result.GetList("peer"), "peers should be inside bgp block")

	// bgp container should have everything
	bgp := result.GetContainer("bgp")
	require.NotNil(t, bgp)
	rid, ok := bgp.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "1.2.3.4", rid)
	las, ok := bgp.Get("local-as")
	assert.True(t, ok)
	assert.Equal(t, "65001", las)
	assert.Contains(t, bgp.GetList("peer"), "10.0.0.1")
}

// --- Full pipeline ---

// TestMigrate_FullPipeline verifies all transformations in sequence.
func TestMigrate_FullPipeline(t *testing.T) {
	tree := config.NewTree()

	// Add a neighbor (should become peer)
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65002")
	peerTree.Set("router-id", "1.2.3.4")

	// Add static routes (should become announce)
	static := config.NewTree()
	routeAttrs := config.NewTree()
	routeAttrs.Set("next-hop", "192.168.1.1")
	static.AddListEntry("route", "10.0.0.0/24", routeAttrs)
	peerTree.SetContainer("static", static)

	tree.AddListEntry("neighbor", "10.0.0.1", peerTree)
	tree.Set("router-id", "1.2.3.4")
	tree.Set("local-as", "65001")

	result, err := Migrate(tree)
	require.NoError(t, err)
	require.NotNil(t, result.Tree)

	// Should have applied multiple transformations
	assert.Contains(t, result.Applied, "neighbor->peer")
	assert.Contains(t, result.Applied, "static->announce")
	assert.Contains(t, result.Applied, "wrap-bgp-block")

	// Final structure: bgp { peer 10.0.0.1 { announce { ipv4 { unicast ... } } } }
	bgp := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers := bgp.GetList("peer")
	require.Contains(t, peers, "10.0.0.1")

	peer := peers["10.0.0.1"]
	announce := peer.GetContainer("announce")
	require.NotNil(t, announce)
	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	assert.Contains(t, ipv4.GetList("unicast"), "10.0.0.0/24")
}
