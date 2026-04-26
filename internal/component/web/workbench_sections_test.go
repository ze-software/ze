package web

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkbenchSections_TwoLevel verifies that sections have Children populated.
func TestWorkbenchSections_TwoLevel(t *testing.T) {
	got := WorkbenchSections(nil)
	for _, s := range got {
		assert.NotEmpty(t, s.Children, "section %q must have children", s.Key)
	}
}

// TestWorkbenchSections_TwoLevel_ChildSelected navigates to /show/bgp/peer/
// and verifies the "Peers" child is selected and the "Routing" parent is expanded.
func TestWorkbenchSections_TwoLevel_ChildSelected(t *testing.T) {
	got := WorkbenchSections([]string{"bgp", "peer"})

	var routing *WorkbenchSection
	for i := range got {
		if got[i].Key == "routing" {
			routing = &got[i]
			break
		}
	}
	require.NotNil(t, routing, "Routing section must exist")
	assert.True(t, routing.Selected, "Routing must be selected")
	assert.True(t, routing.Expanded, "Routing must be expanded")

	var peers *WorkbenchSubPage
	for i := range routing.Children {
		if routing.Children[i].Key == "peers" {
			peers = &routing.Children[i]
			break
		}
	}
	require.NotNil(t, peers, "Peers sub-page must exist under Routing")
	assert.True(t, peers.Selected, "Peers child must be selected")
}

// TestWorkbenchSections_TwoLevel_SectionCollapse verifies that sections other
// than the active one have Expanded=false.
func TestWorkbenchSections_TwoLevel_SectionCollapse(t *testing.T) {
	got := WorkbenchSections([]string{"bgp", "peer"})

	for _, s := range got {
		if s.Key == "routing" {
			assert.True(t, s.Expanded, "active section must be expanded")
		} else {
			assert.False(t, s.Expanded, "inactive section %q must not be expanded", s.Key)
		}
	}
}

// TestWorkbenchSections_TwoLevel_DashboardDefault verifies that the root path
// selects Dashboard and its Overview child.
func TestWorkbenchSections_TwoLevel_DashboardDefault(t *testing.T) {
	got := WorkbenchSections(nil)

	var dash *WorkbenchSection
	for i := range got {
		if got[i].Key == "dashboard" {
			dash = &got[i]
			break
		}
	}
	require.NotNil(t, dash, "Dashboard section must exist")
	assert.True(t, dash.Selected, "Dashboard must be selected")
	assert.True(t, dash.Expanded, "Dashboard must be expanded")

	require.NotEmpty(t, dash.Children, "Dashboard must have children")
	assert.True(t, dash.Children[0].Selected, "Dashboard first child (Overview) must be selected")
}

// TestWorkbenchSections_TwoLevel_AllSectionsPresent verifies all 11 sections exist.
func TestWorkbenchSections_TwoLevel_AllSectionsPresent(t *testing.T) {
	got := WorkbenchSections(nil)

	wantKeys := []string{
		"dashboard", "interfaces", "ip", "routing", "policy",
		"firewall", "l2tp", "services", "system", "tools", "logs",
	}

	gotKeys := make([]string, len(got))
	for i, s := range got {
		gotKeys[i] = s.Key
	}

	assert.Equal(t, wantKeys, gotKeys, "section keys must match expected list")
}

// TestWorkbenchSections_InterfaceSubNav verifies that the Interfaces section
// has sub-entries for All, Ethernet, Bridge, VLAN, Tunnel, Traffic.
func TestWorkbenchSections_InterfaceSubNav(t *testing.T) {
	got := WorkbenchSections([]string{"iface"})

	var ifaces *WorkbenchSection
	for i := range got {
		if got[i].Key == "interfaces" {
			ifaces = &got[i]
			break
		}
	}
	require.NotNil(t, ifaces, "Interfaces section must exist")
	assert.True(t, ifaces.Selected, "Interfaces must be selected for /show/iface/")
	assert.True(t, ifaces.Expanded, "Interfaces must be expanded")

	wantKeys := []string{"all", "ethernet", "bridge", "vlan", "tunnel", "traffic"}
	gotKeys := make([]string, len(ifaces.Children))
	for i, c := range ifaces.Children {
		gotKeys[i] = c.Key
	}
	assert.Equal(t, wantKeys, gotKeys, "Interface sub-nav keys must match")
}

// TestWorkbenchSectionsToolsURL verifies that Tools section URLs point to
// /show/tools/ (not /admin/), and that the section is selected for tool paths.
func TestWorkbenchSectionsToolsURL(t *testing.T) {
	got := WorkbenchSections([]string{segTools, "ping"})

	var tools *WorkbenchSection
	for i := range got {
		if got[i].Key == segTools {
			tools = &got[i]
			break
		}
	}
	require.NotNil(t, tools, "Tools section must exist")
	assert.True(t, tools.Selected, "Tools must be selected for /show/tools/ping/")
	assert.True(t, tools.Expanded, "Tools must be expanded")

	// All tool URLs must be under /show/tools/.
	for _, child := range tools.Children {
		assert.True(t, strings.HasPrefix(child.URL, "/show/tools/"),
			"tool %q URL %q must start with /show/tools/", child.Key, child.URL)
	}
}

// TestWorkbenchSectionsLogsURL verifies that Logs section URLs point to
// /show/logs/ (not /admin/), and that the section is selected for log paths.
func TestWorkbenchSectionsLogsURL(t *testing.T) {
	got := WorkbenchSections([]string{segLogs, "live"})

	var logs *WorkbenchSection
	for i := range got {
		if got[i].Key == segLogs {
			logs = &got[i]
			break
		}
	}
	require.NotNil(t, logs, "Logs section must exist")
	assert.True(t, logs.Selected, "Logs must be selected for /show/logs/live/")
	assert.True(t, logs.Expanded, "Logs must be expanded")

	// All log URLs must be under /show/logs/.
	for _, child := range logs.Children {
		assert.True(t, strings.HasPrefix(child.URL, "/show/logs/"),
			"log %q URL %q must start with /show/logs/", child.Key, child.URL)
	}
}

// TestWorkbenchSections_IPSubNav verifies that the IP section has sub-entries
// for Addresses, Routes, DNS.
func TestWorkbenchSections_IPSubNav(t *testing.T) {
	got := WorkbenchSections([]string{"ip", "addresses"})

	var ip *WorkbenchSection
	for i := range got {
		if got[i].Key == "ip" {
			ip = &got[i]
			break
		}
	}
	require.NotNil(t, ip, "IP section must exist")
	assert.True(t, ip.Selected, "IP must be selected for /show/ip/addresses/")
	assert.True(t, ip.Expanded, "IP must be expanded")

	wantKeys := []string{"addresses", "routes", "dns"}
	gotKeys := make([]string, len(ip.Children))
	for i, c := range ip.Children {
		gotKeys[i] = c.Key
	}
	assert.Equal(t, wantKeys, gotKeys, "IP sub-nav keys must match")
}
