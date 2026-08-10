package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestResourcesUptime verifies the System Resources page reports a real uptime
// and current time instead of the "-" placeholders (F12/AC-13).
func TestResourcesUptime(t *testing.T) {
	data := buildResourcesData()
	assert.NotEqual(t, "-", data.Uptime, "uptime must be populated")
	assert.NotEmpty(t, data.Uptime)
	assert.NotEqual(t, "-", data.CurrentTime, "current time must be populated")
	assert.NotEmpty(t, data.CurrentTime)
}

func TestBuildSystemIdentityFormData_WithValues(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.Set("host", "router01")
	sys.Set("domain", "example.com")
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.Set("router-id", "10.0.0.1")

	form := buildSystemIdentityFormData(tree)
	assert.Equal(t, "System Identity", form.Title)
	require.Len(t, form.Fields, 3)
	assert.Equal(t, "router01", form.Fields[0].Value)
	assert.Equal(t, "example.com", form.Fields[1].Value)
	assert.Equal(t, "10.0.0.1", form.Fields[2].Value)
	assert.Equal(t, "/config/form/", form.SaveURL)
}

func TestBuildSystemIdentityFormData_NilTree(t *testing.T) {
	form := buildSystemIdentityFormData(nil)
	assert.Equal(t, "System Identity", form.Title)
	require.Len(t, form.Fields, 3)
	assert.Empty(t, form.Fields[0].Value)
	assert.Empty(t, form.Fields[1].Value)
	assert.Empty(t, form.Fields[2].Value)
}

func TestBuildSystemIdentityFormData_EmptyTree(t *testing.T) {
	tree := config.NewTree()
	form := buildSystemIdentityFormData(tree)
	require.Len(t, form.Fields, 3)
	assert.Empty(t, form.Fields[0].Value)
}

func TestCollectUsers_WithUsers(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	auth := sys.GetOrCreateContainer("authentication")

	u1 := config.NewTree()
	u1.AddListEntry("public-keys", "key1", config.NewTree())
	u1.AddListEntry("public-keys", "key2", config.NewTree())
	u1.AddListEntry("profile", "admin-profile", config.NewTree())
	u1.AddListEntry("profile", "network-admin", config.NewTree())
	auth.AddListEntry("user", "admin", u1)

	auth.AddListEntry("user", "operator", config.NewTree())

	users := collectUsers(tree)
	require.Len(t, users, 2)
	assert.Equal(t, "admin", users[0].Name)
	assert.Equal(t, 2, users[0].KeyCount)
	require.Len(t, users[0].Profiles, 2)
	assert.Contains(t, users[0].Profiles, "admin-profile")
	assert.Contains(t, users[0].Profiles, "network-admin")
	assert.Equal(t, "operator", users[1].Name)
	assert.Equal(t, 0, users[1].KeyCount)
	assert.Empty(t, users[1].Profiles)
}

func TestCollectUsers_NilTree(t *testing.T) {
	users := collectUsers(nil)
	assert.Nil(t, users)
}

func TestCollectUsers_NoAuth(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("system")
	users := collectUsers(tree)
	assert.Nil(t, users)
}

func TestBuildUsersTableData_WithUsers(t *testing.T) {
	users := []userEntry{
		{Name: "admin", KeyCount: 2},
		{Name: "operator", Profiles: []string{"read-only"}, KeyCount: 0},
	}
	table := buildUsersTableData(users)
	assert.Equal(t, "Users", table.Title)
	require.Len(t, table.Rows, 2)
	assert.Equal(t, "admin", table.Rows[0].Key)
	assert.Equal(t, "-", table.Rows[0].Cells[1])
	assert.Equal(t, "read-only", table.Rows[1].Cells[1])
}

func TestBuildUsersTableData_Empty(t *testing.T) {
	table := buildUsersTableData(nil)
	assert.Empty(t, table.Rows)
	assert.Equal(t, "No users configured.", table.EmptyMessage)
}

func TestUsersIncludesPowerUser(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	auth := sys.GetOrCreateContainer("authentication")
	auth.AddListEntry("user", "operator", config.NewTree())

	renderer, err := NewRenderer()
	require.NoError(t, err)

	html := string(handleUsersPage(renderer, tree, []string{"admin"}))

	assert.Contains(t, html, "admin (system)", "power user must appear with system marker")
	assert.Contains(t, html, "operator", "config user must still appear")
}

func TestBuildUsersTableData_SystemUserNoEditAction(t *testing.T) {
	users := []userEntry{
		{Name: "admin", System: true},
		{Name: "operator"},
	}
	table := buildUsersTableData(users)
	require.Len(t, table.Rows, 2)
	assert.Empty(t, table.Rows[0].Actions, "system user should have no edit action")
	assert.NotEmpty(t, table.Rows[1].Actions, "config user should have edit action")
	assert.Contains(t, table.Rows[0].Cells[0], "(system)")
}

func TestBuildResourcesData(t *testing.T) {
	data := buildResourcesData()
	assert.NotEmpty(t, data.Version)
	assert.True(t, data.CPUCount > 0)
	assert.True(t, data.GOMAXPROCS > 0)
	assert.True(t, data.Goroutines > 0)
	assert.NotEmpty(t, data.MemAlloc)
	assert.NotEmpty(t, data.MemSys)
}

func TestBuildResourcesHTML(t *testing.T) {
	data := resourcesData{
		Version:    "1.0.0",
		CPUCount:   4,
		GOMAXPROCS: 4,
		Goroutines: 10,
		MemAlloc:   "10 MB",
		MemSys:     "20 MB",
		GCRuns:     5,
	}
	html := buildResourcesHTML(data)
	s := string(html)
	assert.Contains(t, s, "System Resources")
	assert.Contains(t, s, "1.0.0")
	assert.Contains(t, s, "10 MB")
	assert.Contains(t, s, "hx-trigger")
}

func TestBuildHostHardwareData(t *testing.T) {
	sections := buildHostHardwareData()
	assert.NotEmpty(t, sections)
	for _, sec := range sections {
		assert.NotEmpty(t, sec.Title)
		assert.NotEmpty(t, sec.Items)
	}
}

func TestBuildHostHardwareHTML_WithSections(t *testing.T) {
	sections := []hardwareSection{
		{Title: "CPU", Items: []HardwareItem{{Key: "Cores", Value: "4"}}},
	}
	html := buildHostHardwareHTML(sections)
	s := string(html)
	assert.Contains(t, s, "Host Hardware")
	assert.Contains(t, s, "CPU")
	assert.Contains(t, s, "Cores")
	assert.Contains(t, s, "4")
	assert.Contains(t, s, "hx-trigger")
	assert.Contains(t, s, "every 10s")
}

func TestBuildHostHardwareHTML_Empty(t *testing.T) {
	html := buildHostHardwareHTML(nil)
	s := string(html)
	assert.Contains(t, s, "No hardware information available")
}

func TestBuildHostHardwareHTML_AlarmIndicator(t *testing.T) {
	sections := []hardwareSection{
		{Title: "Thermal", Items: []HardwareItem{
			{Key: "coretemp0", Value: "85.0°C [ALARM]", CSSClass: "alarm"},
			{Key: "coretemp1", Value: "42.0°C"},
		}},
	}
	html := buildHostHardwareHTML(sections)
	s := string(html)
	assert.Contains(t, s, `wb-hardware-alarm`)
	assert.Contains(t, s, "85.0°C [ALARM]")
	assert.Contains(t, s, "42.0°C")
}

func TestBuildHostHardwareHTML_NICCarrierClass(t *testing.T) {
	sections := []hardwareSection{
		{Title: "NIC", Items: []HardwareItem{
			{Key: "eth0", Value: "igb, aa:bb:cc:dd:ee:ff, 1000 Mbps, up", CSSClass: "up"},
			{Key: "eth1", Value: "igb, 11:22:33:44:55:66, -, down", CSSClass: "down"},
		}},
	}
	html := buildHostHardwareHTML(sections)
	s := string(html)
	assert.Contains(t, s, `wb-hardware-up`)
	assert.Contains(t, s, `wb-hardware-down`)
}

func TestCollectSysctlProfiles_WithProfiles(t *testing.T) {
	tree := config.NewTree()
	sysctl := tree.GetOrCreateContainer("sysctl")
	p1 := config.NewTree()
	p1.AddListEntry("setting", "net.ipv4.ip_forward", config.NewTree())
	p1.AddListEntry("setting", "net.ipv6.conf.all.forwarding", config.NewTree())
	sysctl.AddListEntry("profile", "forwarding", p1)

	sysctl.AddListEntry("profile", "performance", config.NewTree())

	profiles := collectSysctlProfiles(tree)
	require.Len(t, profiles, 2)
	assert.Equal(t, "forwarding", profiles[0].Name)
	assert.Equal(t, 2, profiles[0].SettingCount)
	assert.Equal(t, "performance", profiles[1].Name)
	assert.Equal(t, 0, profiles[1].SettingCount)
}

func TestCollectSysctlProfiles_NilTree(t *testing.T) {
	profiles := collectSysctlProfiles(nil)
	assert.Nil(t, profiles)
}

func TestBuildSysctlProfilesTableData_WithProfiles(t *testing.T) {
	profiles := []sysctlProfileEntry{
		{Name: "forwarding", SettingCount: 2},
		{Name: "performance", SettingCount: 5},
	}
	table := buildSysctlProfilesTableData(profiles)
	assert.Equal(t, "Sysctl Profiles", table.Title)
	require.Len(t, table.Rows, 2)
	assert.Equal(t, "forwarding", table.Rows[0].Key)
	assert.Equal(t, "2", table.Rows[0].Cells[1])
}

func TestBuildSysctlProfilesTableData_Empty(t *testing.T) {
	table := buildSysctlProfilesTableData(nil)
	assert.Empty(t, table.Rows)
	assert.Equal(t, "No sysctl profiles configured.", table.EmptyMessage)
}

func TestRenderSystemPageContent_Resources(t *testing.T) {
	// Resources handler does not use the renderer, so it works with nil.
	_, ok := renderSystemPageContent(nil, []string{"resources"}, nil)
	assert.True(t, ok)
}

func TestRenderSystemPageContent_Hardware(t *testing.T) {
	// Hardware handler does not use the renderer, so it works with nil.
	_, ok := renderSystemPageContent(nil, []string{"hardware"}, nil)
	assert.True(t, ok)
}

func TestRenderSystemPageContent_Unknown(t *testing.T) {
	_, ok := renderSystemPageContent(nil, []string{"nonexistent"}, nil)
	assert.False(t, ok)
}

func TestResolveRouterIdentity_Hostname(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.Set("host", "core-01")

	assert.Equal(t, "core-01", resolveRouterIdentity(tree))
}

func TestResolveRouterIdentity_RouterID(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.Set("router-id", "10.0.0.1")

	assert.Equal(t, "10.0.0.1", resolveRouterIdentity(tree))
}

func TestResolveRouterIdentity_Fallback(t *testing.T) {
	assert.Equal(t, "ze", resolveRouterIdentity(nil))
	assert.Equal(t, "ze", resolveRouterIdentity(config.NewTree()))
}

func TestResolveRouterIdentity_HostnameOverridesRouterID(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.Set("host", "edge-rtr-42")
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.Set("router-id", "10.0.0.1")

	assert.Equal(t, "edge-rtr-42", resolveRouterIdentity(tree))
}

func TestCollectFleetPeers_WithPeers(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	fleet := sys.GetOrCreateContainer("fleet")
	p1 := config.NewTree()
	p1.Set("url", "https://core-01.example.com")
	fleet.AddListEntry("peer", "core-01", p1)
	p2 := config.NewTree()
	p2.Set("url", "https://edge-01.example.com")
	fleet.AddListEntry("peer", "edge-01", p2)

	peers := CollectFleetPeers(tree, "core-01")
	require.Len(t, peers, 2)
	assert.Equal(t, "core-01", peers[0].Name)
	assert.Equal(t, "https://core-01.example.com", peers[0].URL)
	assert.True(t, peers[0].Active, "current identity should be marked Active")
	assert.Equal(t, "edge-01", peers[1].Name)
	assert.Equal(t, "https://edge-01.example.com", peers[1].URL)
	assert.False(t, peers[1].Active, "non-current peer should not be Active")
}

func TestCollectFleetPeers_NilTree(t *testing.T) {
	peers := CollectFleetPeers(nil, "ze")
	assert.Nil(t, peers)
}

func TestCollectFleetPeers_NoFleet(t *testing.T) {
	tree := config.NewTree()
	peers := CollectFleetPeers(tree, "ze")
	assert.Nil(t, peers)
}
