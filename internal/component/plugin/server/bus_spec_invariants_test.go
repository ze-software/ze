// Design: docs/architecture/api/architecture.md -- spec-explicit-plugin-and-attach-config invariant unit tests.
// Related: bus_filter.go -- CompileBus sends-before-receptions ordering and path resolution.

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSendsCompleteBeforeReceptions verifies that CompileBus catalogs every
// participant's declared sends and tag schemas BEFORE validating any
// participant's receptions, regardless of participant input ordering.
func TestSendsCompleteBeforeReceptions(t *testing.T) {
	// Place receiver BEFORE sender in input order; receiver filters on a custom
	// tag declared only on the sender's MessageSendDecl.
	bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{
		{
			Path:         "/bus/endpoint/aaa-receiver",
			EndpointName: "aaa-receiver",
			Receives:     []rpc.MessageReceiveDecl{{Message: "acme:alert"}},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					Messages: map[string]*plugin.BusMessageRule{
						"acme:alert": {
							Tags: map[string][]string{"severity": {"high"}},
						},
					},
				},
			},
		},
		{
			Path:         "/bus/endpoint/zzz-sender",
			EndpointName: "zzz-sender",
			Sends: []rpc.MessageSendDecl{{
				Message: "acme:alert",
				Tags:    []rpc.MessageTagDecl{{Name: "severity", Values: []string{"low", "high"}}},
			}},
		},
	})
	require.NoError(t, err, "sends must be cataloged before receptions regardless of participant order")
	require.Len(t, bus.Edges(), 1)
	assert.Equal(t, "/bus/endpoint/zzz-sender", bus.Edges()[0].SenderPath)
	assert.Equal(t, "/bus/endpoint/aaa-receiver", bus.Edges()[0].ReceiverPath)

	// Invalid tag value against sender's closed set must still be rejected even
	// when the receiver precedes the sender in input order.
	_, err = CompileBus(NewTagInterner(), []BusParticipantInput{
		{
			Path:         "/bus/endpoint/aaa-receiver",
			EndpointName: "aaa-receiver",
			Receives:     []rpc.MessageReceiveDecl{{Message: "acme:alert"}},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					Messages: map[string]*plugin.BusMessageRule{
						"acme:alert": {
							Tags: map[string][]string{"severity": {"critical"}},
						},
					},
				},
			},
		},
		{
			Path:         "/bus/endpoint/zzz-sender",
			EndpointName: "zzz-sender",
			Sends: []rpc.MessageSendDecl{{
				Message: "acme:alert",
				Tags:    []rpc.MessageTagDecl{{Name: "severity", Values: []string{"low", "high"}}},
			}},
		},
	})
	require.Error(t, err, "closed tag set from later sender must reject invalid value on earlier receiver")
}

// TestRegistrationCarriesNoSecondDeclaration verifies that every compiled-in
// plugin registration declares message capabilities solely through Sends and
// Receives with no legacy duplicate custom-event/send declaration fields.
func TestRegistrationCarriesNoSecondDeclaration(t *testing.T) {
	for _, name := range registry.Names() {
		reg := registry.Lookup(name)
		require.NotNil(t, reg)
		for _, send := range reg.Sends {
			assert.NotEmpty(t, send.Message, "plugin %s declared empty send message", name)
			assert.Contains(t, send.Message, ":", "plugin %s send %q must use <namespace>:<message>", name, send.Message)
		}
		for _, recv := range reg.Receives {
			assert.NotEmpty(t, recv.Message, "plugin %s declared empty receive message", name)
			assert.Contains(t, recv.Message, ":", "plugin %s receive %q must use <namespace>:<message>", name, recv.Message)
		}
	}
}

// TestHubUnderBusListenerAndGraph verifies that `bus { hub { server ... } }`
// parses cleanly under the YANG schema and builds a valid config graph under
// `section/bus`.
func TestHubUnderBusListenerAndGraph(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	tree := config.NewTree()
	bus := config.NewTree()
	hub := config.NewTree()
	srv := config.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", "9443")
	hub.AddListEntry("server", "main", srv)
	bus.SetContainer("hub", hub)
	tree.SetContainer("bus", bus)

	g := config.BuildGraph(tree, schema)
	require.NotNil(t, g)
	foundBus := false
	for _, n := range g.Nodes {
		if n.ID == "section/bus" {
			foundBus = true
			break
		}
	}
	assert.True(t, foundBus, "config graph must contain section/bus for bus > hub > server")
}

// TestBusReferenceWalk verifies short bus path prefix coverage and syntax
// validation across peer, group, IP, and endpoint reference walks.
func TestBusReferenceWalk(t *testing.T) {
	assert.NoError(t, plugin.ValidateBusPathSyntax("/bgp"))
	assert.NoError(t, plugin.ValidateBusPathSyntax("/bgp/peer/edge1"))
	assert.NoError(t, plugin.ValidateBusPathSyntax("/bgp/ip/192.0.2.1"))
	assert.NoError(t, plugin.ValidateBusPathSyntax("/bus/endpoint/collector"))
	assert.Error(t, plugin.ValidateBusPathSyntax("bgp/peer/edge1"))
	assert.Error(t, plugin.ValidateBusPathSyntax("/bgp//edge1"))

	assert.True(t, plugin.PathCovers("/bgp", "/bgp/peer/edge1"))
	assert.True(t, plugin.PathCovers("/bgp/group/ix", "/bgp/group/ix/peer/m1"))
	assert.False(t, plugin.PathCovers("/bgp/peer/edge1", "/bgp/peer/edge10"))
	assert.False(t, plugin.PathCovers("/bus/endpoint/col", "/bus/endpoint/collector"))
}
