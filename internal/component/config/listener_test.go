package config

import (
	"net"
	"strconv"
	"strings"
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
	byName := map[string]ListenerService{}
	for _, svc := range services {
		byName[svc.Name] = svc
	}

	// All 8 TCP services must be discovered.
	for _, name := range []string{"web", "ssh", "mcp", "looking-glass", "prometheus", "plugin-hub", "api-server-rest", "api-server-grpc"} {
		svc, ok := byName[name]
		require.True(t, ok, "service %q must be discovered", name)
		assert.Equal(t, []string{ProtocolTCP}, svc.Protocols, "service %q must be TCP", name)
		assert.True(t, svc.ServerList, "service %q must use server sub-list", name)
	}

	// Wireguard must be discovered as UDP with flat shape.
	wg, ok := byName["wireguard"]
	require.True(t, ok, "wireguard must be discovered")
	assert.Equal(t, []string{ProtocolUDP}, wg.Protocols)
	assert.False(t, wg.ServerList, "wireguard uses flat listen-port, not server sub-list")

	// Plugin-hub has no enabled leaf in YANG.
	assert.False(t, byName["plugin-hub"].HasEnabledLeaf, "plugin-hub has no enabled leaf")

	// Web has an enabled leaf in YANG.
	assert.True(t, byName["web"].HasEnabledLeaf, "web has an enabled leaf")
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
	byName := map[string]ListenerService{}
	for _, svc := range services {
		byName[svc.Name] = svc
	}

	svc, ok := byName["test-svc"]
	require.True(t, ok, "synthetic test-svc must be discovered without Go code change")
	assert.Equal(t, []string{ProtocolTCP}, svc.Protocols)
	assert.True(t, svc.ServerList)
	assert.True(t, svc.HasEnabledLeaf)
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

func TestCollectListenersWithDefaults_EmptyServerList(t *testing.T) {
	schema := listenerTestSchema(t)

	RegisterListenerDefault("ssh", "127.0.0.1", "2222")
	RegisterListenerDefault("web", "0.0.0.0", "3443")

	tree := NewTree()
	env := NewTree()
	ssh := NewTree()
	ssh.Set("enabled", "true")
	env.SetContainer("ssh", ssh)
	tree.SetContainer("environment", env)

	endpoints := CollectListenersWithDefaults(tree, schema)

	found := false
	for _, ep := range endpoints {
		if ep.Service == "ssh" && ep.Port == 2222 {
			found = true
			assert.Equal(t, "127.0.0.1", ep.IP.String())
			break
		}
	}
	assert.True(t, found, "expected ssh default endpoint when server list is empty")
}

// TestCollectListenersWithDefaults_EntryWithoutPort verifies the endpoint an
// entry that names an ip and OMITS the port binds: the service's default port.
//
// The daemon does exactly this -- extractAPIServerList (loader_extract.go) seeds
// each entry with the default host and port and overwrites only what the entry
// names -- while parseListenerEntry returns nil for a port of 0 and the
// empty-list fill is skipped whenever the list has entries. So `ze doctor`
// probed NOTHING for a config that binds a real port.
//
// VALIDATES: an ip-only entry yields a probe at the registered default port.
// PREVENTS: the one config shape between "all defaults" and "fully spelled out"
// falling through both fills and being probed by nothing.
func TestCollectListenersWithDefaults_EntryWithoutPort(t *testing.T) {
	schema := listenerTestSchema(t)

	RegisterListenerDefault("ssh", "127.0.0.1", "2222")

	tree := NewTree()
	env := NewTree()
	ssh := NewTree()
	ssh.Set("enabled", "true")
	srv := NewTree()
	srv.Set("ip", "10.0.0.1") // no port: the daemon uses the default
	ssh.AddListEntry("server", "s1", srv)
	env.SetContainer("ssh", ssh)
	tree.SetContainer("environment", env)

	endpoints := CollectListenersWithDefaults(tree, schema)

	found := false
	for _, ep := range endpoints {
		if ep.Service == "ssh s1" {
			found = true
			assert.Equal(t, uint16(2222), ep.Port, "entry without a port must use the default port")
			assert.Equal(t, "10.0.0.1", ep.IP.String(), "the entry's own ip must win over the default ip")
		}
	}
	assert.True(t, found, "expected an endpoint for the ip-only entry")
}

// TestCollectListenersWithDefaults_EntryDefaultNotUsedForEmptyList verifies the
// two registrations stay separate: a service registered with
// RegisterListenerEntryDefault gets its per-entry fallback and NO endpoint for
// an empty list, because it starts no listener then.
//
// VALIDATES: ListenerDefault.WhenListEmpty gates the empty-list fill alone.
// PREVENTS: doctor probing a port the daemon never binds, which is what a plain
// RegisterListenerDefault for l2tp or bmp would produce.
func TestCollectListenersWithDefaults_EntryDefaultNotUsedForEmptyList(t *testing.T) {
	schema := listenerTestSchema(t)

	RegisterListenerEntryDefault("ssh", "127.0.0.1", "2222")
	t.Cleanup(func() { RegisterListenerDefault("ssh", "127.0.0.1", "2222") })

	empty := NewTree()
	envEmpty := NewTree()
	sshEmpty := NewTree()
	sshEmpty.Set("enabled", "true")
	envEmpty.SetContainer("ssh", sshEmpty)
	empty.SetContainer("environment", envEmpty)

	for _, ep := range CollectListenersWithDefaults(empty, schema) {
		assert.NotEqual(t, "ssh", ep.Service, "an entry-only default must not fill an empty list")
	}

	withEntry := NewTree()
	envEntry := NewTree()
	sshEntry := NewTree()
	sshEntry.Set("enabled", "true")
	srv := NewTree()
	srv.Set("ip", "10.0.0.1")
	sshEntry.AddListEntry("server", "s1", srv)
	envEntry.SetContainer("ssh", sshEntry)
	withEntry.SetContainer("environment", envEntry)

	found := false
	for _, ep := range CollectListenersWithDefaults(withEntry, schema) {
		if ep.Service == "ssh s1" {
			found = true
			assert.Equal(t, uint16(2222), ep.Port)
		}
	}
	assert.True(t, found, "an entry-only default must still complete an entry")
}

// TestListenerDefaultsAgreeWithMCPExtraction ties the doctor default for mcp to
// the extraction that decides where an mcp listener binds.
//
// ze-mcp-conf.yang declares `refine port { default 8080; }` for
// environment/mcp/server and the Ze YANG compiler drops refine defaults, so
// extractMCPBlock applies that port itself and listener_defaults.go registers the
// same endpoint for the probe.
//
// Both halves are asserted together on purpose. Taking the default out of
// extractMCPBlock, and taking the registration out of listener_defaults.go, each
// break one half, and either leaves the daemon and its readiness check
// disagreeing about where MCP listens.
// VALIDATES: an mcp server entry that names no port binds 8080, and ze doctor
// probes that same endpoint.
// PREVENTS: the schema promising a port the daemon ignores, and a listener the
// daemon starts that no probe reaches.
func TestListenerDefaultsAgreeWithMCPExtraction(t *testing.T) {
	schema := listenerTestSchema(t)
	RegisterBuiltinListenerDefaults()

	// ports[i] is the port of the i-th server entry; "" means the entry omits it.
	mcpTree := func(ports ...string) *Tree {
		tree := NewTree()
		env := NewTree()
		mcp := NewTree()
		mcp.Set("enabled", "true")
		for i, port := range ports {
			srv := NewTree()
			srv.Set("ip", "127.0.0.1")
			if port != "" {
				srv.Set("port", port)
			}
			mcp.AddListEntry("server", string(rune('a'+i)), srv)
		}
		env.SetContainer("mcp", mcp)
		tree.SetContainer("environment", env)
		return tree
	}

	cases := []struct {
		name  string
		ports []string
		// want is the port of every endpoint the daemon starts, in config order.
		want []string
	}{
		{"empty server list", nil, []string{"8080"}},
		{"one entry, no port", []string{""}, []string{"8080"}},
		// One entry names a port and the other does not. The named port is the
		// operator's and is never overwritten; only the silent entry takes 8080.
		{"one entry names a port, the other does not", []string{"9090", ""}, []string{"9090", "8080"}},
	}
	for _, tc := range cases {
		tree := mcpTree(tc.ports...)
		cfg, ok := ExtractMCPConfig(tree)
		require.Truef(t, ok, "%s: an enabled mcp block must start a listener", tc.name)
		var got []string
		for _, s := range cfg.Servers {
			got = append(got, s.Port)
		}
		assert.Equalf(t, tc.want, got, "%s: every server entry must carry a port", tc.name)

		probed := map[uint16]bool{}
		for _, ep := range CollectListenersWithDefaults(tree, schema) {
			if strings.HasPrefix(ep.Service, "mcp") {
				probed[ep.Port] = true
			}
		}
		for _, port := range tc.want {
			n, convErr := strconv.ParseUint(port, 10, 16)
			require.NoError(t, convErr)
			assert.Truef(t, probed[uint16(n)],
				"%s: ze doctor probes no mcp listener on port %s, so the daemon binds a port the readiness check never reaches", tc.name, port)
		}
	}
}

// TestMCPPortDefaultFollowsTheEnabledGate verifies the default reaches only the
// blocks that asked for a listener.
//
// A synthesized endpoint is not an instruction to bind: an mcp block the
// operator switched off, and a tree with no mcp block at all, must start nothing
// and report nothing, exactly as they did before the default was applied.
//
// VALIDATES: ExtractMCPConfig keeps its `enabled` gate now that the port gate is
// gone, and a portless entry raises no diagnostic on either operator surface.
// PREVENTS: an upgrade starting MCP for a deployment that never enabled it, and
// `ze config validate` rejecting the config an operator writes from the schema.
func TestMCPPortDefaultFollowsTheEnabledGate(t *testing.T) {
	withEnabled := func(enabled string) *Tree {
		tree := NewTree()
		env := NewTree()
		mcp := NewTree()
		if enabled != "" {
			mcp.Set("enabled", enabled)
		}
		srv := NewTree()
		srv.Set("ip", "127.0.0.1") // no port: the default supplies it
		mcp.AddListEntry("server", "a", srv)
		env.SetContainer("mcp", mcp)
		tree.SetContainer("environment", env)
		return tree
	}

	for _, tc := range []struct {
		name    string
		enabled string
		want    bool
	}{
		{"enabled leaf absent (YANG default false)", "", false},
		{"enabled false", "false", false},
		{"enabled true", "true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, ok := ExtractMCPConfig(withEnabled(tc.enabled))
			assert.Equal(t, tc.want, ok, "only a switched-on mcp block starts a listener")
			if ok {
				require.Len(t, cfg.Servers, 1)
				assert.Equal(t, "8080", cfg.Servers[0].Port, "the entry names no port, so the schema default supplies it")
			}
			for _, d := range ValidateSemantics(withEnabled(tc.enabled)) {
				assert.NotEqual(t, "config-mcp-invalid", d.Code,
					"a server entry that omits the port is the shape the schema default exists for, not a mistake")
			}
		})
	}

	// No mcp block at all: nothing to start.
	_, ok := ExtractMCPConfig(NewTree())
	assert.False(t, ok, "an absent mcp block starts no listener")
}

// TestCollectListenersWithDefaults_DualStackEmptyList verifies a service whose
// empty list yields SEVERAL default endpoints gets a probe for each.
//
// geodns is the case: parseListeners (internal/plugins/geodns/config.go) returns
// 127.0.0.1:5300 AND ::1:5300, so probing only the first would pass a config
// whose other half cannot bind.
// VALIDATES: RegisterListenerDefaultIPs yields one endpoint per address.
// PREVENTS: a dual-stack default silently collapsing to its first address.
func TestCollectListenersWithDefaults_DualStackEmptyList(t *testing.T) {
	schema := listenerTestSchema(t)

	RegisterListenerDefaultIPs("ssh", []string{"127.0.0.1", "::1"}, "2222")
	t.Cleanup(func() { RegisterListenerDefault("ssh", "127.0.0.1", "2222") })

	tree := NewTree()
	env := NewTree()
	ssh := NewTree()
	ssh.Set("enabled", "true")
	env.SetContainer("ssh", ssh)
	tree.SetContainer("environment", env)

	var got []string
	for _, ep := range CollectListenersWithDefaults(tree, schema) {
		if ep.Service == "ssh" {
			got = append(got, ep.IP.String())
		}
	}
	assert.Contains(t, got, "127.0.0.1", "the first default address must be probed")
	assert.Contains(t, got, "::1", "the second default address must be probed too")
}

// TestRegisterListenerDefaultRefusesUnusable verifies a registration that cannot
// produce an endpoint is DROPPED rather than half-stored.
//
// Stored, it was worse than absent: the empty-list path yielded nothing while the
// per-entry path still produced an endpoint at a fallback address nobody
// registered, so the service looked probed from one direction and was not from
// the other, and neither said anything.
// VALIDATES: a default with no usable address or port registers nothing, so the
// covered-row assertion in TestDoctorProbesEveryCoveredListener fails for it.
// PREVENTS: a silent registration bug leaving a listener unprobed while its
// inventory row claims coverage.
func TestRegisterListenerDefaultRefusesUnusable(t *testing.T) {
	schema := listenerTestSchema(t)
	t.Cleanup(func() { RegisterListenerDefault("ssh", "127.0.0.1", "2222") })

	tree := NewTree()
	env := NewTree()
	ssh := NewTree()
	ssh.Set("enabled", "true")
	srv := NewTree()
	srv.Set("ip", "10.0.0.1") // no port: only a registered default can complete it
	ssh.AddListEntry("server", "s1", srv)
	env.SetContainer("ssh", ssh)
	tree.SetContainer("environment", env)

	for _, tc := range []struct {
		name string
		def  func()
	}{
		{"no addresses", func() { RegisterListenerDefaultIPs("ssh", nil, "2222") }},
		{"unparseable address", func() { RegisterListenerDefault("ssh", "not-an-ip", "2222") }},
		{"unparseable port", func() { RegisterListenerDefault("ssh", "127.0.0.1", "http") }},
		{"port zero", func() { RegisterListenerDefault("ssh", "127.0.0.1", "0") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Start from a GOOD registration, so this also proves a refused
			// RE-registration clears the previous default instead of leaving the
			// service probed at an endpoint its owner has just tried to replace.
			RegisterListenerDefault("ssh", "127.0.0.1", "2222")
			tc.def()
			for _, ep := range CollectListenersWithDefaults(tree, schema) {
				assert.NotContains(t, ep.Service, "ssh",
					"an unusable registration must yield NO endpoint, not one at an address nobody registered and not the previous one")
			}
		})
	}
}

// TestRegisterListenerProtocolsRefusesUnknownTransport verifies a transport ze
// cannot probe never reaches an endpoint.
//
// probeListener (internal/component/doctor/checks_listener.go) hands the network
// string to net.Listen for anything that is not "udp", so an unknown one errors
// and doctor reports doctor-listen-unavailable against a listener that is
// healthy. That is worse than the silent case storeListenerDefault guards: it
// fabricates a failure rather than hiding one, and the operator is sent after a
// port that was never in trouble.
//
// VALIDATES: only tcp and udp reach ListenerEndpoint.Protocol.
// PREVENTS: a typo in one registration turning into a permanent false positive
// in `ze doctor`.
func TestRegisterListenerProtocolsRefusesUnknownTransport(t *testing.T) {
	schema := listenerTestSchema(t)
	RegisterBuiltinListenerDefaults()
	t.Cleanup(func() {
		listenerProtocolsMu.Lock()
		delete(listenerProtocols, "ssh")
		listenerProtocolsMu.Unlock()
	})

	tree := NewTree()
	env := NewTree()
	ssh := NewTree()
	ssh.Set("enabled", "true")
	srv := NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", "2222")
	ssh.AddListEntry("server", "s1", srv)
	env.SetContainer("ssh", ssh)
	tree.SetContainer("environment", env)

	sshProtocols := func() []string {
		var got []string
		for _, ep := range CollectListeners(tree, schema) {
			if strings.HasPrefix(ep.Service, "ssh") {
				got = append(got, ep.Protocol)
			}
		}
		return got
	}

	// The registration is ONE claim, so an unknown transport refuses the whole
	// call rather than keeping the readable part. Keeping a subset would assert a
	// narrower claim than anyone wrote, and on a flat listen-port list -- whose
	// shape fallback is UDP -- keeping the tcp out of ("junk", tcp) would probe
	// the WRONG transport where refusing the lot probes the right one.
	t.Run("one unknown transport refuses the whole registration", func(t *testing.T) {
		RegisterListenerProtocols("ssh", ProtocolUDP, "sctp")
		got := sshProtocols()
		assert.Equal(t, []string{ProtocolTCP}, got,
			"a partly-unreadable registration falls back to the list shape, it does not keep its readable half")
		assert.NotContains(t, got, "sctp", "an unprobeable network string must never reach an endpoint")
	})

	t.Run("all unknown falls back to the list shape", func(t *testing.T) {
		RegisterListenerProtocols("ssh", "sctp")
		got := sshProtocols()
		assert.Equal(t, []string{ProtocolTCP}, got,
			"with nothing usable registered the service falls back to the shape, which is a real network name")
		assert.NotContains(t, got, "sctp", "an unprobeable network string must never reach an endpoint")
	})

	// A refused call also CLEARS a good earlier registration: the caller was
	// restating this service's transports, and keeping the old set would go on
	// probing a claim they were replacing.
	t.Run("refusal clears a previous good registration", func(t *testing.T) {
		RegisterListenerProtocols("ssh", ProtocolUDP)
		require.Equal(t, []string{ProtocolUDP}, sshProtocols(), "precondition: the good registration took effect")
		RegisterListenerProtocols("ssh", "sctp")
		assert.Equal(t, []string{ProtocolTCP}, sshProtocols(),
			"the stale udp claim must not survive a refused restatement")
	})
}

func TestCollectListenersWithDefaults_ExplicitOverridesDefault(t *testing.T) {
	schema := listenerTestSchema(t)

	RegisterListenerDefault("ssh", "127.0.0.1", "2222")

	tree := NewTree()
	env := NewTree()
	ssh := NewTree()
	ssh.Set("enabled", "true")
	srv := NewTree()
	srv.Set("ip", "10.0.0.1")
	srv.Set("port", "2223")
	ssh.AddListEntry("server", "s1", srv)
	env.SetContainer("ssh", ssh)
	tree.SetContainer("environment", env)

	endpoints := CollectListenersWithDefaults(tree, schema)

	for _, ep := range endpoints {
		if strings.Contains(ep.Service, "ssh") {
			assert.NotEqual(t, uint16(2222), ep.Port, "default port should not appear when explicit entry exists")
		}
	}
}
