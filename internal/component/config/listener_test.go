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

// testSchema returns the YANG schema for use in listener tests.
func listenerTestSchema(t *testing.T) *Schema {
	t.Helper()
	schema, err := YANGSchema()
	require.NoError(t, err, "YANGSchema must load for listener tests")
	return schema
}

// TestCollectListeners verifies tree walking collects enabled services and skips disabled.
// VALIDATES: CollectListeners walks ze:listener-marked service paths.
// PREVENTS: Enabled/disabled logic broken, endpoints silently missed.
func TestCollectListeners(t *testing.T) {
	schema := listenerTestSchema(t)
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

	// Plugin hub: no enabled leaf in YANG -- always collected.
	plug := NewTree()
	hub := NewTree()
	hubSrv := NewTree()
	hubSrv.Set("ip", "127.0.0.1")
	hubSrv.Set("port", "12700")
	hub.AddListEntry("server", "local", hubSrv)
	plug.SetContainer("hub", hub)
	tree.SetContainer("plugin", plug)

	endpoints := CollectListeners(tree, schema)

	// Should have web + plugin-hub, NOT ssh or mcp.
	require.Len(t, endpoints, 2)
	byName := map[string]ListenerEndpoint{}
	for _, ep := range endpoints {
		byName[ep.Service] = ep
	}
	require.Contains(t, byName, "web main")
	assert.Equal(t, uint16(3443), byName["web main"].Port)
	require.Contains(t, byName, "plugin-hub local")
	assert.Equal(t, uint16(12700), byName["plugin-hub local"].Port)
}

// TestCollectListeners_EmptyTree verifies empty tree returns nil.
func TestCollectListeners_EmptyTree(t *testing.T) {
	schema := listenerTestSchema(t)
	tree := NewTree()
	assert.Nil(t, CollectListeners(tree, schema))
}

// TestCollectListeners_EmptySchema verifies that a schema with no
// ze:listener lists produces no endpoints without panicking.
func TestCollectListeners_EmptySchema(t *testing.T) {
	schema := NewSchema()
	tree := NewTree()
	assert.Nil(t, CollectListeners(tree, schema))
}

// TestCollectListeners_APIServerRest verifies api-server.rest listeners are
// picked up by CollectListeners via the dynamic YANG schema walk.
//
// VALIDATES: spec-named-service-listeners AC-16 (CollectListeners covers the
// api-server transports so REST + gRPC mis-config is caught at parse time).
// PREVENTS: Regression where api-server entries sit outside the conflict
// inventory and collide silently.
func TestCollectListeners_APIServerRest(t *testing.T) {
	schema := listenerTestSchema(t)
	tree := NewTree()
	env := NewTree()
	apiServer := NewTree()
	rest := NewTree()
	rest.Set("enabled", "true")
	restSrv := NewTree()
	restSrv.Set("ip", "0.0.0.0")
	restSrv.Set("port", "8081")
	rest.AddListEntry("server", "main", restSrv)
	apiServer.SetContainer("rest", rest)
	env.SetContainer("api-server", apiServer)
	tree.SetContainer("environment", env)

	endpoints := CollectListeners(tree, schema)
	require.Len(t, endpoints, 1)
	assert.Equal(t, "api-server-rest main", endpoints[0].Service)
	assert.Equal(t, uint16(8081), endpoints[0].Port)
}

// TestCollectListeners_APIServerGrpc mirrors the REST case for the gRPC
// transport.
func TestCollectListeners_APIServerGrpc(t *testing.T) {
	schema := listenerTestSchema(t)
	tree := NewTree()
	env := NewTree()
	apiServer := NewTree()
	grpcC := NewTree()
	grpcC.Set("enabled", "true")
	grpcSrv := NewTree()
	grpcSrv.Set("ip", "0.0.0.0")
	grpcSrv.Set("port", "50051")
	grpcC.AddListEntry("server", "main", grpcSrv)
	apiServer.SetContainer("grpc", grpcC)
	env.SetContainer("api-server", apiServer)
	tree.SetContainer("environment", env)

	endpoints := CollectListeners(tree, schema)
	require.Len(t, endpoints, 1)
	assert.Equal(t, "api-server-grpc main", endpoints[0].Service)
	assert.Equal(t, uint16(50051), endpoints[0].Port)
}

// TestValidateListenerConflicts_APIRestGrpc verifies that REST and gRPC
// configured on the same ip:port are reported as a conflict.
//
// VALIDATES: spec-named-service-listeners AC-11 (overlapping api-server
// transports rejected at parse time).
func TestValidateListenerConflicts_APIRestGrpc(t *testing.T) {
	schema := listenerTestSchema(t)
	tree := NewTree()
	env := NewTree()
	apiServer := NewTree()

	rest := NewTree()
	rest.Set("enabled", "true")
	restSrv := NewTree()
	restSrv.Set("ip", "127.0.0.1")
	restSrv.Set("port", "9000")
	rest.AddListEntry("server", "main", restSrv)
	apiServer.SetContainer("rest", rest)

	grpcC := NewTree()
	grpcC.Set("enabled", "true")
	grpcSrv := NewTree()
	grpcSrv.Set("ip", "127.0.0.1")
	grpcSrv.Set("port", "9000")
	grpcC.AddListEntry("server", "main", grpcSrv)
	apiServer.SetContainer("grpc", grpcC)

	env.SetContainer("api-server", apiServer)
	tree.SetContainer("environment", env)

	endpoints := CollectListeners(tree, schema)
	require.Len(t, endpoints, 2, "both transports must appear in the inventory")

	err := ValidateListenerConflicts(endpoints)
	require.Error(t, err, "REST + gRPC on the same port must report a conflict")
	assert.Contains(t, err.Error(), "listener conflict")
	assert.Contains(t, err.Error(), "api-server-rest")
	assert.Contains(t, err.Error(), "api-server-grpc")
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
			ep := parseListenerEntry("test", ProtocolTCP, "main", entry)
			if tt.wantNil {
				assert.Nil(t, ep)
			} else {
				require.NotNil(t, ep)
				assert.Equal(t, tt.wantIP, ep.IP.String())
				assert.Equal(t, "test main", ep.Service)
				assert.Equal(t, ProtocolTCP, ep.Protocol)
			}
		})
	}
}

// TestListenerProtocolDistinction verifies that a TCP and a UDP listener on
// the same port do not clash. This is the minimum behavior change introduced
// by Phase 5 of spec-iface-wireguard and is the reason ListenerEndpoint
// gained a Protocol field.
//
// VALIDATES: AC-19 -- wireguard UDP + web TCP on the same port are both
// accepted because the kernel keeps TCP and UDP in separate namespaces.
// PREVENTS: false-positive conflict when ze adds UDP services alongside
// existing TCP services.
func TestListenerProtocolDistinction(t *testing.T) {
	endpoints := []ListenerEndpoint{
		{Service: "web", Protocol: ProtocolTCP, IP: net.IPv4zero, Port: 443},
		{Service: "wireguard wg0", Protocol: ProtocolUDP, IP: net.IPv4zero, Port: 443},
	}
	err := ValidateListenerConflicts(endpoints)
	assert.NoError(t, err, "TCP:443 and UDP:443 on the same IP must not clash")
}

// TestCollectListeners_Wireguard verifies that CollectListeners discovers
// wireguard entries via the dynamic schema walk and emits one UDP endpoint
// per entry with IP=0.0.0.0.
//
// VALIDATES: AC-2 (spec-listener-dynamic-walk) -- wireguard flat-leaf shape handled.
// PREVENTS: silent drop of wireguard listen-port in conflict detection.
func TestCollectListeners_Wireguard(t *testing.T) {
	schema := listenerTestSchema(t)
	tree := NewTree()
	ifaceC := NewTree()
	wg0 := NewTree()
	wg0.Set("listen-port", "51820")
	ifaceC.AddListEntry("wireguard", "wg0", wg0)
	wg1 := NewTree()
	wg1.Set("listen-port", "51821")
	ifaceC.AddListEntry("wireguard", "wg1", wg1)
	tree.SetContainer("interface", ifaceC)

	endpoints := CollectListeners(tree, schema)
	require.Len(t, endpoints, 2)

	byName := map[string]ListenerEndpoint{}
	for _, ep := range endpoints {
		byName[ep.Service] = ep
	}
	require.Contains(t, byName, "wireguard wg0")
	require.Contains(t, byName, "wireguard wg1")
	assert.Equal(t, ProtocolUDP, byName["wireguard wg0"].Protocol)
	assert.Equal(t, uint16(51820), byName["wireguard wg0"].Port)
	assert.True(t, byName["wireguard wg0"].IP.Equal(net.IPv4zero))
	assert.Equal(t, uint16(51821), byName["wireguard wg1"].Port)
}

// TestCollectListeners_WireguardNoPort verifies that a wireguard entry
// without a listen-port is skipped (kernel picks an ephemeral port, nothing
// to conflict with).
//
// VALIDATES: wireguards with auto-assigned ports do not produce spurious
// endpoints or errors in the conflict detector.
// PREVENTS: accidental conflict on port 0 or false positive "missing port".
func TestCollectListeners_WireguardNoPort(t *testing.T) {
	schema := listenerTestSchema(t)
	tree := NewTree()
	ifaceC := NewTree()
	wg0 := NewTree()
	// no listen-port set
	ifaceC.AddListEntry("wireguard", "wg0", wg0)
	tree.SetContainer("interface", ifaceC)

	endpoints := CollectListeners(tree, schema)
	assert.Empty(t, endpoints)
}

// TestDynamicListenerWalk verifies that DiscoverListenerServices finds all
// ze:listener-marked lists in the YANG schema, including all 8 TCP services
// and the wireguard UDP service.
//
// VALIDATES: AC-1 (spec-listener-dynamic-walk) -- all existing services discovered.
// VALIDATES: AC-4 (spec-listener-dynamic-walk) -- knownListenerServices deleted.
// PREVENTS: dynamic walker misses a service that the hardcoded list had.
func TestDynamicListenerWalk(t *testing.T) {
	schema := listenerTestSchema(t)
	services := DiscoverListenerServices(schema)

	// Build a name->service map for assertions.
	byName := map[string]listenerService{}
	for _, svc := range services {
		byName[svc.name] = svc
	}

	// All 8 TCP services must be discovered.
	for _, name := range []string{"web", "ssh", "mcp", "looking-glass", "prometheus", "plugin-hub", "api-server-rest", "api-server-grpc"} {
		svc, ok := byName[name]
		require.True(t, ok, "service %q must be discovered", name)
		assert.Equal(t, ProtocolTCP, svc.protocol, "service %q must be TCP", name)
		assert.True(t, svc.serverList, "service %q must use server sub-list", name)
	}

	// Wireguard must be discovered as UDP with flat shape.
	wg, ok := byName["wireguard"]
	require.True(t, ok, "wireguard must be discovered")
	assert.Equal(t, ProtocolUDP, wg.protocol)
	assert.False(t, wg.serverList, "wireguard uses flat listen-port, not server sub-list")

	// Plugin-hub has no enabled leaf in YANG.
	assert.False(t, byName["plugin-hub"].hasEnabledLeaf, "plugin-hub has no enabled leaf")

	// Web has an enabled leaf in YANG.
	assert.True(t, byName["web"].hasEnabledLeaf, "web has an enabled leaf")
}

// TestDynamicListenerNewService verifies that a synthetic ze:listener list
// added to the schema is auto-discovered without any Go code change.
//
// VALIDATES: AC-3 (spec-listener-dynamic-walk) -- new YANG ze:listener auto-discovered.
// PREVENTS: dynamic walker only finding hardcoded services.
func TestDynamicListenerNewService(t *testing.T) {
	schema := listenerTestSchema(t)

	// Inject a synthetic listener list into the schema under "test-service".
	testContainer := &ContainerNode{
		children: map[string]Node{
			"enabled": &LeafNode{Type: TypeBool, Default: "false"},
			"server": &ListNode{
				KeyType:  TypeString,
				KeyName:  "name",
				Listener: true,
				children: map[string]Node{
					"name": &LeafNode{Type: TypeString},
					"ip":   &LeafNode{Type: TypeIP},
					"port": &LeafNode{Type: TypeUint16},
				},
				order: []string{"name", "ip", "port"},
			},
		},
		order: []string{"enabled", "server"},
	}
	schema.Define("test-svc", testContainer)

	services := DiscoverListenerServices(schema)
	byName := map[string]listenerService{}
	for _, svc := range services {
		byName[svc.name] = svc
	}

	svc, ok := byName["test-svc"]
	require.True(t, ok, "synthetic test-svc must be discovered without Go code change")
	assert.Equal(t, ProtocolTCP, svc.protocol)
	assert.True(t, svc.serverList)
	assert.True(t, svc.hasEnabledLeaf)
}

// TestValidateListenerConflicts_WireguardDuplicatePort verifies AC-18: two
// wireguard interfaces with the same listen-port are rejected.
//
// VALIDATES: AC-18 -- duplicate wireguard listen-port across interfaces is
// caught by ValidateListenerConflicts.
// PREVENTS: two wireguards silently binding the same UDP port (one would
// fail at kernel bind time with a confusing error).
func TestValidateListenerConflicts_WireguardDuplicatePort(t *testing.T) {
	endpoints := []ListenerEndpoint{
		{Service: "wireguard wg0", Protocol: ProtocolUDP, IP: net.IPv4zero, Port: 51820},
		{Service: "wireguard wg1", Protocol: ProtocolUDP, IP: net.IPv4zero, Port: 51820},
	}
	err := ValidateListenerConflicts(endpoints)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wg0")
	assert.Contains(t, err.Error(), "wg1")
	assert.Contains(t, err.Error(), "udp")
}
