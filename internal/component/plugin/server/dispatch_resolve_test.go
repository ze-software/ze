package server

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPluginResolveRPCReachesHubResolver proves a plugin's resolve call is
// answered by the resolver the hub registered, and by nothing else.
//
// VALIDATES: wiring row 3, A-2 -- a plugin reaches the daemon's single resolver
// over the RPC rather than building one of its own.
// PREVENTS: the failure that killed the plugin placement argument. A plugin
// with its own Resolver would double the query load against the upstream
// server and split the TTL view between two caches, and nothing about the
// plugin working would say so.
func TestPluginResolveRPCReachesHubResolver(t *testing.T) {
	var (
		gotName  string
		gotQtype uint16
	)
	rpc.RegisterDNSResolver(func(name string, qtype uint16) ([]string, uint32, string, error) {
		gotName, gotQtype = name, qtype
		return []string{"192.0.2.1"}, 300, "NOERROR", nil
	})
	t.Cleanup(func() { rpc.RegisterDNSResolver(nil) })

	s := &Server{}
	params, err := json.Marshal(&rpc.ResolveDNSInput{Name: "a.invalid", Type: 1})
	require.NoError(t, err)

	result, err := s.opResolveDNS(nil, params)
	require.NoError(t, err)

	assert.Equal(t, "a.invalid", gotName, "the name reaches the registered resolver")
	assert.Equal(t, uint16(1), gotQtype, "and so does the record type")

	out, ok := result.(*rpc.ResolveDNSOutput)
	require.True(t, ok)
	assert.Equal(t, []string{"192.0.2.1"}, out.Records)
	assert.Equal(t, uint32(300), out.TTL)
	assert.Equal(t, "NOERROR", out.Status, "the response code crosses the boundary with the records")
}

// TestResolveRPCRefusesWithNoResolverRegistered proves an unwired hub refuses
// rather than answering with an empty record list.
//
// The two are different facts and a plugin cannot tell them apart. A firewall
// plugin reading an empty answer as "this name holds no address" would empty a
// live set because the hub was started without a resolver, which is the
// silently-wrong-value failure ai/rules/principles.md names first.
func TestResolveRPCRefusesWithNoResolverRegistered(t *testing.T) {
	rpc.RegisterDNSResolver(nil)
	t.Cleanup(func() { rpc.RegisterDNSResolver(nil) })

	s := &Server{}
	params, err := json.Marshal(&rpc.ResolveDNSInput{Name: "a.invalid", Type: 1})
	require.NoError(t, err)

	result, err := s.opResolveDNS(nil, params)
	require.Error(t, err, "no resolver is refused, never answered with an empty list")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no DNS resolver registered")
}

// TestResolveRPCRefusesAnEmptyName proves a call with no name is refused. An
// empty name would otherwise reach the resolver and query the root.
func TestResolveRPCRefusesAnEmptyName(t *testing.T) {
	rpc.RegisterDNSResolver(func(string, uint16) ([]string, uint32, string, error) {
		t.Fatal("the resolver must not be reached with an empty name")
		return nil, 0, "", nil
	})
	t.Cleanup(func() { rpc.RegisterDNSResolver(nil) })

	s := &Server{}
	params, err := json.Marshal(&rpc.ResolveDNSInput{Type: 1})
	require.NoError(t, err)

	_, err = s.opResolveDNS(nil, params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no name given")
}

// TestResolveRPCRelaysAResolverError proves a lookup failure reaches the
// plugin as an error rather than as an empty answer. The plugin's whole
// classification turns on telling a failure from an absence.
func TestResolveRPCRelaysAResolverError(t *testing.T) {
	rpc.RegisterDNSResolver(func(string, uint16) ([]string, uint32, string, error) {
		return nil, 0, "", errors.New("server unreachable")
	})
	t.Cleanup(func() { rpc.RegisterDNSResolver(nil) })

	s := &Server{}
	params, err := json.Marshal(&rpc.ResolveDNSInput{Name: "a.invalid", Type: 1})
	require.NoError(t, err)

	_, err = s.opResolveDNS(nil, params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server unreachable")
}

// TestResolveRPCRefusesMalformedParams proves a bad payload is reported rather
// than read as an empty request.
func TestResolveRPCRefusesMalformedParams(t *testing.T) {
	s := &Server{}
	_, err := s.opResolveDNS(nil, json.RawMessage(`{"name":`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid resolve-dns params")
}

// TestGetDNSResolverIsNilBeforeRegistration proves the accessor answers nil
// rather than a zero-value handler. A non-nil handler that returned nothing
// would be indistinguishable from a name holding no address.
func TestGetDNSResolverIsNilBeforeRegistration(t *testing.T) {
	rpc.RegisterDNSResolver(nil)
	t.Cleanup(func() { rpc.RegisterDNSResolver(nil) })
	assert.Nil(t, rpc.GetDNSResolver())
}
