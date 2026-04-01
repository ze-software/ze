package config

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateListenerConflicts_SamePort verifies exact duplicate ip:port is detected.
// VALIDATES: AC-1 -- two services on same ip:port produces error naming both.
// PREVENTS: Duplicate listener silently accepted.
func TestValidateListenerConflicts_SamePort(t *testing.T) {
	endpoints := []ListenerEndpoint{
		{Service: "web", IP: net.ParseIP("0.0.0.0"), Port: 8443},
		{Service: "looking-glass", IP: net.ParseIP("0.0.0.0"), Port: 8443},
	}
	err := ValidateListenerConflicts(endpoints)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web")
	assert.Contains(t, err.Error(), "looking-glass")
}

// TestValidateListenerConflicts_WildcardIPv4 verifies 0.0.0.0 conflicts with specific IPv4.
// VALIDATES: AC-2 -- wildcard 0.0.0.0 conflicts with 127.0.0.1 on same port.
// PREVENTS: Wildcard binding not detected as conflicting.
func TestValidateListenerConflicts_WildcardIPv4(t *testing.T) {
	endpoints := []ListenerEndpoint{
		{Service: "web", IP: net.ParseIP("0.0.0.0"), Port: 8443},
		{Service: "looking-glass", IP: net.ParseIP("127.0.0.1"), Port: 8443},
	}
	err := ValidateListenerConflicts(endpoints)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web")
	assert.Contains(t, err.Error(), "looking-glass")
}

// TestValidateListenerConflicts_WildcardIPv6 verifies :: conflicts with specific IPv6.
// VALIDATES: AC-6 -- IPv6 wildcard :: conflicts with ::1 on same port.
// PREVENTS: IPv6 wildcard not handled.
func TestValidateListenerConflicts_WildcardIPv6(t *testing.T) {
	endpoints := []ListenerEndpoint{
		{Service: "ssh", IP: net.ParseIP("::"), Port: 2222},
		{Service: "mcp", IP: net.ParseIP("::1"), Port: 2222},
	}
	err := ValidateListenerConflicts(endpoints)
	require.Error(t, err)
}

// TestValidateListenerConflicts_WildcardIPv6Dup verifies :: vs :: same port conflicts.
// VALIDATES: Wildcard logic row 6.
// PREVENTS: IPv6 duplicate wildcard not detected.
func TestValidateListenerConflicts_WildcardIPv6Dup(t *testing.T) {
	endpoints := []ListenerEndpoint{
		{Service: "ssh", IP: net.ParseIP("::"), Port: 2222},
		{Service: "mcp", IP: net.ParseIP("::"), Port: 2222},
	}
	err := ValidateListenerConflicts(endpoints)
	require.Error(t, err)
}

// TestValidateListenerConflicts_NoConflict verifies different ports or different specific IPs pass.
// VALIDATES: AC-3, AC-5 -- non-overlapping listeners accepted.
// PREVENTS: False positive conflicts.
func TestValidateListenerConflicts_NoConflict(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []ListenerEndpoint
	}{
		{
			name: "different ports",
			endpoints: []ListenerEndpoint{
				{Service: "web", IP: net.ParseIP("0.0.0.0"), Port: 3443},
				{Service: "looking-glass", IP: net.ParseIP("0.0.0.0"), Port: 8443},
			},
		},
		{
			name: "different specific IPs same port",
			endpoints: []ListenerEndpoint{
				{Service: "bgp peer 10.0.0.1", IP: net.ParseIP("10.0.0.1"), Port: 179},
				{Service: "bgp peer 10.0.0.2", IP: net.ParseIP("10.0.0.2"), Port: 179},
			},
		},
		{
			name: "cross-family ipv4-wildcard vs ipv6-specific",
			endpoints: []ListenerEndpoint{
				{Service: "web", IP: net.ParseIP("0.0.0.0"), Port: 8443},
				{Service: "ssh", IP: net.ParseIP("::1"), Port: 8443},
			},
		},
		{
			name: "cross-family ipv6-wildcard vs ipv4-specific",
			endpoints: []ListenerEndpoint{
				{Service: "web", IP: net.ParseIP("::"), Port: 8443},
				{Service: "ssh", IP: net.ParseIP("10.0.0.1"), Port: 8443},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateListenerConflicts(tt.endpoints)
			assert.NoError(t, err)
		})
	}
}

// TestValidateListenerConflicts_BGPPeer verifies BGP peer local endpoint participates.
// VALIDATES: AC-4 -- BGP peer local conflicts with web on same ip:port.
// PREVENTS: BGP peer endpoints excluded from conflict check.
func TestValidateListenerConflicts_BGPPeer(t *testing.T) {
	endpoints := []ListenerEndpoint{
		{Service: "bgp peer 10.0.0.1", IP: net.ParseIP("10.0.0.1"), Port: 179},
		{Service: "web", IP: net.ParseIP("10.0.0.1"), Port: 179},
	}
	err := ValidateListenerConflicts(endpoints)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bgp peer")
	assert.Contains(t, err.Error(), "web")
}

// TestValidateListenerConflicts_PluginHub verifies plugin hub server participates.
// VALIDATES: AC-7 -- plugin hub server entry conflicts with web detected.
// PREVENTS: Plugin hub endpoints excluded from conflict check.
func TestValidateListenerConflicts_PluginHub(t *testing.T) {
	endpoints := []ListenerEndpoint{
		{Service: "plugin hub local", IP: net.ParseIP("127.0.0.1"), Port: 12700},
		{Service: "web", IP: net.ParseIP("0.0.0.0"), Port: 12700},
	}
	err := ValidateListenerConflicts(endpoints)
	require.Error(t, err)
}

// TestValidateListenerConflicts_NoListeners verifies empty list produces no error.
// VALIDATES: AC-9 -- no listeners configured is valid.
// PREVENTS: Nil/empty slice panics.
func TestValidateListenerConflicts_NoListeners(t *testing.T) {
	assert.NoError(t, ValidateListenerConflicts(nil))
	assert.NoError(t, ValidateListenerConflicts([]ListenerEndpoint{}))
}

// TestCollectListeners verifies tree walking collects enabled services and skips disabled.
// VALIDATES: CollectListeners walks known service paths.
// PREVENTS: Enabled/disabled logic broken, endpoints silently missed.
func TestCollectListeners(t *testing.T) {
	tree := NewTree()

	// Web: enabled true, one server entry.
	env := NewTree()
	web := NewTree()
	web.Set("enabled", "true")
	srv := NewTree()
	srv.Set("ip", "0.0.0.0")
	srv.Set("port", "3443")
	web.AddListEntry("server", "main", srv)
	env.SetContainer("web", web)

	// SSH: enabled false -- should be skipped.
	ssh := NewTree()
	ssh.Set("enabled", "false")
	sshSrv := NewTree()
	sshSrv.Set("ip", "0.0.0.0")
	sshSrv.Set("port", "2222")
	ssh.AddListEntry("server", "main", sshSrv)
	env.SetContainer("ssh", ssh)

	// MCP: no enabled leaf -- should be skipped (YANG default false).
	mcp := NewTree()
	mcpSrv := NewTree()
	mcpSrv.Set("ip", "127.0.0.1")
	mcpSrv.Set("port", "9718")
	mcp.AddListEntry("server", "main", mcpSrv)
	env.SetContainer("mcp", mcp)

	tree.SetContainer("environment", env)

	// Plugin hub: no enabled leaf but alwaysEnabled.
	plug := NewTree()
	hub := NewTree()
	hubSrv := NewTree()
	hubSrv.Set("ip", "127.0.0.1")
	hubSrv.Set("port", "12700")
	hub.AddListEntry("server", "local", hubSrv)
	plug.SetContainer("hub", hub)
	tree.SetContainer("plugin", plug)

	endpoints := CollectListeners(tree)

	// Should have web + plugin-hub, NOT ssh or mcp.
	require.Len(t, endpoints, 2)
	assert.Equal(t, "web main", endpoints[0].Service)
	assert.Equal(t, uint16(3443), endpoints[0].Port)
	assert.Equal(t, "plugin-hub local", endpoints[1].Service)
	assert.Equal(t, uint16(12700), endpoints[1].Port)
}

// TestCollectListeners_EmptyTree verifies empty tree returns nil.
func TestCollectListeners_EmptyTree(t *testing.T) {
	tree := NewTree()
	assert.Nil(t, CollectListeners(tree))
}

// TestParseListenerEntry verifies edge cases in endpoint extraction.
// VALIDATES: parseListenerEntry handles empty IP, port 0, malformed, boundary.
// PREVENTS: Silent conflict bypass from bad input.
func TestParseListenerEntry(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		port    string
		wantNil bool
		wantIP  string
	}{
		{"valid ipv4", "10.0.0.1", "8443", false, "10.0.0.1"},
		{"valid ipv6", "::1", "8443", false, "::1"},
		{"empty IP defaults to wildcard", "", "8443", false, "0.0.0.0"},
		{"port 0 skipped", "10.0.0.1", "0", true, ""},
		{"port 65535 valid", "10.0.0.1", "65535", false, "10.0.0.1"},
		{"port 65536 invalid", "10.0.0.1", "65536", true, ""},
		{"port non-numeric", "10.0.0.1", "abc", true, ""},
		{"both empty", "", "", true, ""},
		{"malformed IP", "not-an-ip", "8443", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := NewTree()
			if tt.ip != "" {
				entry.Set("ip", tt.ip)
			}
			if tt.port != "" {
				entry.Set("port", tt.port)
			}
			ep := parseListenerEntry("test", "main", entry)
			if tt.wantNil {
				assert.Nil(t, ep)
			} else {
				require.NotNil(t, ep)
				assert.Equal(t, tt.wantIP, ep.IP.String())
				assert.Equal(t, "test main", ep.Service)
			}
		})
	}
}
