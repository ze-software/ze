package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// buildTestBGPTree creates a config tree with BGP peers and groups for testing.
func buildTestBGPTree() *config.Tree {
	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)

	// Router ID
	bgp.Set("router-id", "10.0.0.1")

	// Session defaults
	sess := config.NewTree()
	bgp.SetContainer("session", sess)
	asn := config.NewTree()
	sess.SetContainer("asn", asn)
	asn.Set("local", "65000")

	// Standalone peer: peer-a
	peerA := config.NewTree()
	connA := config.NewTree()
	remoteA := config.NewTree()
	remoteA.Set("ip", "192.168.1.1")
	connA.SetContainer("remote", remoteA)
	peerA.SetContainer("connection", connA)
	sessA := config.NewTree()
	asnA := config.NewTree()
	asnA.Set("local", "65000")
	asnA.Set("remote", "65001")
	sessA.SetContainer("asn", asnA)
	// Add a family
	famTree := config.NewTree()
	famTree.Set("mode", "enable")
	sessA.AddListEntry("family", "ipv4/unicast", famTree)
	peerA.SetContainer("session", sessA)
	bgp.AddListEntry("peer", "peer-a", peerA)

	// Standalone peer: peer-b
	peerB := config.NewTree()
	connB := config.NewTree()
	remoteB := config.NewTree()
	remoteB.Set("ip", "192.168.1.2")
	connB.SetContainer("remote", remoteB)
	peerB.SetContainer("connection", connB)
	sessB := config.NewTree()
	asnB := config.NewTree()
	asnB.Set("local", "65000")
	asnB.Set("remote", "65002")
	sessB.SetContainer("asn", asnB)
	peerB.SetContainer("session", sessB)
	bgp.AddListEntry("peer", "peer-b", peerB)

	// Group: transit with one peer
	group := config.NewTree()
	groupSess := config.NewTree()
	groupASN := config.NewTree()
	groupASN.Set("remote", "65100")
	groupSess.SetContainer("asn", groupASN)
	famG := config.NewTree()
	famG.Set("mode", "enable")
	groupSess.AddListEntry("family", "ipv6/unicast", famG)
	group.SetContainer("session", groupSess)

	groupPeer := config.NewTree()
	connGP := config.NewTree()
	remoteGP := config.NewTree()
	remoteGP.Set("ip", "10.0.0.1")
	connGP.SetContainer("remote", remoteGP)
	groupPeer.SetContainer("connection", connGP)
	gpSess := config.NewTree()
	gpASN := config.NewTree()
	gpASN.Set("local", "65000")
	gpASN.Set("remote", "65100")
	gpSess.SetContainer("asn", gpASN)
	groupPeer.SetContainer("session", gpSess)
	group.AddListEntry("peer", "transit-peer", groupPeer)

	bgp.AddListEntry("group", "transit", group)

	return tree
}

// --- Peers page tests ---

func TestBGPPeersPageRendersTable(t *testing.T) {
	tree := buildTestBGPTree()
	peers := collectPeers(tree)

	require.Len(t, peers, 3, "should find 2 standalone + 1 grouped peer")

	data := BuildBGPPeersTableData(peers, "")
	assert.Equal(t, "BGP Peers", data.Title)
	assert.Len(t, data.Rows, 3)
	assert.Equal(t, "Add Peer", data.AddLabel)

	// Verify columns
	assert.Len(t, data.Columns, 7)
	assert.Equal(t, "Name", data.Columns[0].Label)
	assert.Equal(t, "Remote IP", data.Columns[1].Label)
	assert.Equal(t, "Remote AS", data.Columns[2].Label)
	assert.Equal(t, "Local AS", data.Columns[3].Label)
	assert.Equal(t, "Group", data.Columns[4].Label)
	assert.Equal(t, "State", data.Columns[5].Label)
	assert.Equal(t, "Families", data.Columns[6].Label)
}

func TestBGPPeersEmptyState(t *testing.T) {
	tree := config.NewTree()
	peers := collectPeers(tree)
	data := BuildBGPPeersTableData(peers, "")

	assert.Empty(t, data.Rows)
	assert.Equal(t, "No BGP peers configured.", data.EmptyMessage)
	assert.NotEmpty(t, data.AddURL)
}

func TestBGPPeersEmptyStateNilTree(t *testing.T) {
	peers := collectPeers(nil)
	data := BuildBGPPeersTableData(peers, "")
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No BGP peers configured.", data.EmptyMessage)
}

func TestBGPPeersStateColorCoding(t *testing.T) {
	tree := buildTestBGPTree()
	peers := collectPeers(tree)
	data := BuildBGPPeersTableData(peers, "")

	// All configured peers should have green flag (v1: no operational state).
	for _, row := range data.Rows {
		assert.Equal(t, flagClassGreen, row.FlagClass,
			"configured peer %q should have green flag", row.Key)
		assert.Equal(t, "C", row.Flags)
	}
}

func TestBGPPeersDisabledFlag(t *testing.T) {
	pe := peerEntry{Name: "test", Disabled: true}
	flags, class := peerFlag(pe)
	assert.Equal(t, "D", flags)
	assert.Equal(t, flagClassGrey, class)
}

func TestBGPPeersFilterByGroup(t *testing.T) {
	tree := buildTestBGPTree()
	peers := collectPeers(tree)

	data := BuildBGPPeersTableData(peers, "transit")
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "transit-peer", data.Rows[0].Key)
	assert.Contains(t, data.EmptyMessage, "") // not empty since we have rows
}

func TestBGPPeersFilterByGroupEmpty(t *testing.T) {
	tree := buildTestBGPTree()
	peers := collectPeers(tree)

	data := BuildBGPPeersTableData(peers, "nonexistent")
	assert.Empty(t, data.Rows)
	assert.Contains(t, data.EmptyMessage, "nonexistent")
}

func TestBGPPeersRowActions(t *testing.T) {
	tree := buildTestBGPTree()
	peers := collectPeers(tree)
	data := BuildBGPPeersTableData(peers, "")

	require.NotEmpty(t, data.Rows)
	// First peer (peer-a) has IP, so should have Edit + Detail + Teardown actions.
	row := data.Rows[0]
	require.Len(t, row.Actions, 3)
	assert.Equal(t, "Edit", row.Actions[0].Label)
	assert.Equal(t, "Detail", row.Actions[1].Label)
	assert.Equal(t, "Teardown", row.Actions[2].Label)
	assert.Equal(t, "danger", row.Actions[2].Class)
	assert.NotEmpty(t, row.Actions[2].Confirm)
}

func TestBGPPeersExtractFields(t *testing.T) {
	tree := buildTestBGPTree()
	peers := collectPeers(tree)

	// Find peer-a (standalone)
	var peerA peerEntry
	for _, p := range peers {
		if p.Name == "peer-a" {
			peerA = p
			break
		}
	}
	assert.Equal(t, "192.168.1.1", peerA.RemoteIP)
	assert.Equal(t, "65001", peerA.RemoteAS)
	assert.Equal(t, "65000", peerA.LocalAS)
	assert.Equal(t, "", peerA.Group)
	assert.Contains(t, peerA.Families, "ipv4/unicast")
	assert.Equal(t, "/show/bgp/peer/peer-a/", peerA.EditURL)

	// Find transit-peer (grouped)
	var tPeer peerEntry
	for _, p := range peers {
		if p.Name == "transit-peer" {
			tPeer = p
			break
		}
	}
	assert.Equal(t, "transit", tPeer.Group)
	assert.Equal(t, "/show/bgp/group/transit/peer/transit-peer/", tPeer.EditURL)
}

// --- Groups page tests ---

func TestBGPGroupsPageRendersTable(t *testing.T) {
	tree := buildTestBGPTree()
	groups := collectGroups(tree)

	require.Len(t, groups, 1)
	data := BuildBGPGroupsTableData(groups)
	assert.Equal(t, "BGP Groups", data.Title)
	require.Len(t, data.Rows, 1)

	row := data.Rows[0]
	assert.Equal(t, "transit", row.Key)
	// Cells: Name, Peer Count, Remote AS, Families
	require.Len(t, row.Cells, 4)
	assert.Equal(t, "transit", row.Cells[0])
	assert.Equal(t, "1", row.Cells[1])     // 1 peer in group
	assert.Equal(t, "65100", row.Cells[2]) // group remote AS
}

func TestBGPGroupsEmptyState(t *testing.T) {
	tree := config.NewTree()
	groups := collectGroups(tree)
	data := BuildBGPGroupsTableData(groups)
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No peer groups configured.", data.EmptyMessage)
}

func TestBGPGroupsViewPeersLink(t *testing.T) {
	tree := buildTestBGPTree()
	groups := collectGroups(tree)
	data := BuildBGPGroupsTableData(groups)

	require.NotEmpty(t, data.Rows)
	row := data.Rows[0]
	require.Len(t, row.Actions, 2)
	assert.Equal(t, "View Peers", row.Actions[0].Label)
	assert.Contains(t, row.Actions[0].URL, "group=transit")
}

// --- Summary page tests ---

func TestBGPSummaryPageRenders(t *testing.T) {
	tree := buildTestBGPTree()
	data := BuildBGPSummaryTableData(tree)

	assert.Equal(t, "BGP Summary", data.Title)
	assert.Len(t, data.Rows, 3)

	// Verify columns include operational placeholders.
	colLabels := make([]string, len(data.Columns))
	for i, c := range data.Columns {
		colLabels[i] = c.Label
	}
	assert.Contains(t, colLabels, "State")
	assert.Contains(t, colLabels, "Uptime")
	assert.Contains(t, colLabels, "Prefixes")
	assert.Contains(t, colLabels, "Last Error")
}

func TestBGPSummaryEmptyState(t *testing.T) {
	tree := config.NewTree()
	data := BuildBGPSummaryTableData(tree)
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No BGP peers configured.", data.EmptyMessage)
}

// --- Families page tests ---

func TestBGPFamiliesPageRenders(t *testing.T) {
	tree := buildTestBGPTree()
	entries := collectFamilies(tree)

	require.NotEmpty(t, entries)
	data := BuildBGPFamiliesTableData(entries)
	assert.Equal(t, "BGP Address Families", data.Title)
	assert.NotEmpty(t, data.Rows)

	// peer-a has ipv4/unicast, transit group has ipv6/unicast
	familyNames := make([]string, 0, len(data.Rows))
	for _, row := range data.Rows {
		familyNames = append(familyNames, row.Cells[0])
	}
	assert.Contains(t, familyNames, "ipv4/unicast")
	assert.Contains(t, familyNames, "ipv6/unicast")
}

func TestBGPFamiliesEmptyState(t *testing.T) {
	tree := config.NewTree()
	entries := collectFamilies(tree)
	data := BuildBGPFamiliesTableData(entries)
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No address families configured.", data.EmptyMessage)
}

// --- Policy page tests ---

func TestBGPPolicyEmptyState(t *testing.T) {
	tree := config.NewTree()
	entries := collectPolicies(tree)
	data := BuildBGPPolicyTableData(entries)
	assert.Empty(t, data.Rows)
	assert.Equal(t, "No filters configured.", data.EmptyMessage)
}

func TestBGPPolicyWithData(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)
	policy := config.NewTree()
	bgp.SetContainer("policy", policy)

	// Add a filter list entry
	filterEntry := config.NewTree()
	rule := config.NewTree()
	rule.Set("match", "any")
	filterEntry.AddListEntry("rule", "10", rule)
	policy.AddListEntry("filter", "reject-all", filterEntry)

	entries := collectPolicies(tree)
	require.Len(t, entries, 1)
	assert.Equal(t, "reject-all", entries[0].Name)
	assert.Equal(t, "filter", entries[0].Type)
	assert.Equal(t, 1, entries[0].RuleCount)

	data := BuildBGPPolicyTableData(entries)
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "reject-all", data.Rows[0].Key)
}

// --- Peer detail tests ---

func TestBGPPeerDetailTabs(t *testing.T) {
	pe := peerEntry{
		Name:     "peer-a",
		RemoteIP: "192.168.1.1",
		RemoteAS: "65001",
		LocalAS:  "65000",
		Families: "ipv4/unicast",
	}

	detail := BuildBGPPeerDetailData(pe)
	assert.Equal(t, "peer-a", detail.Title)
	assert.Equal(t, "/show/bgp/peer/", detail.CloseURL)

	require.Len(t, detail.Tabs, 3)
	assert.Equal(t, "Config", detail.Tabs[0].Label)
	assert.True(t, detail.Tabs[0].Active)
	assert.Equal(t, "Status", detail.Tabs[1].Label)
	assert.Equal(t, "Actions", detail.Tabs[2].Label)
}

func TestBGPPeerDetailConfigContent(t *testing.T) {
	pe := peerEntry{
		Name:     "peer-a",
		RemoteIP: "192.168.1.1",
		RemoteAS: "65001",
		LocalAS:  "65000",
		Families: "ipv4/unicast",
	}

	detail := BuildBGPPeerDetailData(pe)
	configContent := string(detail.Tabs[0].Content)
	assert.Contains(t, configContent, "peer-a")
	assert.Contains(t, configContent, "192.168.1.1")
	assert.Contains(t, configContent, "65001")
	assert.Contains(t, configContent, "65000")
	assert.Contains(t, configContent, "ipv4/unicast")
}

func TestBGPPeerDetailStatusPlaceholder(t *testing.T) {
	pe := peerEntry{Name: "peer-a", RemoteIP: "192.168.1.1"}
	detail := BuildBGPPeerDetailData(pe)
	statusContent := string(detail.Tabs[1].Content)
	assert.Contains(t, statusContent, peerStateConfigured)
	assert.Contains(t, statusContent, "--") // placeholder values
}

func TestBGPPeerDetailActionsTab(t *testing.T) {
	pe := peerEntry{Name: "peer-a", RemoteIP: "192.168.1.1"}
	detail := BuildBGPPeerDetailData(pe)
	actionsContent := string(detail.Tabs[2].Content)
	assert.Contains(t, actionsContent, "Flush")
	assert.Contains(t, actionsContent, "Teardown")
	assert.Contains(t, actionsContent, "peer-flush")
	assert.Contains(t, actionsContent, "peer-teardown")
}

func TestBGPPeerDetailGroupedCloseURL(t *testing.T) {
	pe := peerEntry{Name: "peer-a", Group: "transit"}
	detail := BuildBGPPeerDetailData(pe)
	assert.Equal(t, "/show/bgp/group/transit/peer/", detail.CloseURL)
}

// --- Workbench integration tests ---

func TestWorkbench_BGPPeersPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := buildTestBGPTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/bgp/peer/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "must render workbench table")
	assert.Contains(t, body, "Add Peer", "must contain add peer button")
	assert.Contains(t, body, `id="workbench-shell"`, "must be inside workbench shell")
}

func TestWorkbench_BGPPeersHTMXPartial(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := buildTestBGPTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/bgp/peer/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "HTMX partial must contain table")
	assert.NotContains(t, body, `id="workbench-shell"`, "HTMX partial must not contain shell")
}

func TestWorkbench_BGPGroupsPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := buildTestBGPTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/bgp/group/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "must render workbench table")
	assert.Contains(t, body, "Add Group", "must contain add group button")
}

func TestWorkbench_BGPSummaryPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := buildTestBGPTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/bgp/summary/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "must render workbench table")
	// Summary has no AddURL, so check for peer data in cells.
	assert.Contains(t, body, "wb-table-row", "must contain table rows")
}

func TestWorkbench_BGPFamiliesPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := buildTestBGPTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/bgp/family/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "must render workbench table")
	// Families table has rows since the test tree has configured families.
	assert.Contains(t, body, "wb-table-row", "must contain table rows")
}

func TestWorkbench_BGPPolicyPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := config.NewTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/bgp/policy/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "wb-table", "must render workbench table")
	assert.Contains(t, body, "No filters configured", "must show empty state")
}

func TestWorkbench_BGPRootStillYANG(t *testing.T) {
	// /show/bgp/ must still fall through to generic YANG detail view.
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := config.NewTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/bgp/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `id="workbench-shell"`, "must be in workbench shell")
	assert.Contains(t, body, "ze-field", "must render YANG detail fields")
}

func TestWorkbench_BGPPeersEmptyPageDispatch(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	tree := config.NewTree()
	handler := HandleWorkbench(renderer, schema, tree, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/show/bgp/peer/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "No BGP peers configured", "must show empty state")
	assert.Contains(t, body, "Add Peer", "must show add button")
}

// --- Sub-navigation tests ---

func TestBGPSubNavigation(t *testing.T) {
	// Verify that navigating to bgp/peer/ selects the Routing section
	// and the Peers sub-page.
	sections := WorkbenchSections([]string{"bgp", "peer"})

	var routingSection *WorkbenchSection
	for i := range sections {
		if sections[i].Key == segRouting {
			routingSection = &sections[i]
			break
		}
	}
	require.NotNil(t, routingSection, "Routing section must exist")
	assert.True(t, routingSection.Selected, "Routing section must be selected for bgp/peer path")
	assert.True(t, routingSection.Expanded, "Routing section must be expanded")

	// Find "peers" sub-page.
	var peersChild *WorkbenchSubPage
	for i := range routingSection.Children {
		if routingSection.Children[i].Key == "peers" {
			peersChild = &routingSection.Children[i]
			break
		}
	}
	require.NotNil(t, peersChild, "Peers sub-page must exist")
	assert.True(t, peersChild.Selected, "Peers sub-page must be selected")
}

func TestBGPGroupsSubNavigation(t *testing.T) {
	sections := WorkbenchSections([]string{"bgp", "group"})

	var routingSection *WorkbenchSection
	for i := range sections {
		if sections[i].Key == segRouting {
			routingSection = &sections[i]
			break
		}
	}
	require.NotNil(t, routingSection)
	assert.True(t, routingSection.Selected)

	var groupsChild *WorkbenchSubPage
	for i := range routingSection.Children {
		if routingSection.Children[i].Key == "groups" {
			groupsChild = &routingSection.Children[i]
			break
		}
	}
	require.NotNil(t, groupsChild, "Groups sub-page must exist")
	assert.True(t, groupsChild.Selected, "Groups sub-page must be selected")
}

// --- Helper function tests ---

func TestValueOrDash(t *testing.T) {
	assert.Equal(t, "hello", valueOrDash("hello"))
	assert.Equal(t, "-", valueOrDash(""))
}
