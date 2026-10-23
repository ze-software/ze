// Design: docs/architecture/behavior/fsm.md — negotiation publishes before Established.
// Related: peer_bind_dispatch_test.go — process event capture over the real IPC transport.
package reactor

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// TestNegotiatedEventUsesCurrentOpenIdentity resolves the remote identity from
// OPEN capabilities while dynamic settings still hold AS 0, then observes the
// negotiated JSON delivered to a subscribed process. Static controls preserve
// configured identity; four-octet cases detect AS_TRANS or uint16 truncation.
// No Established transition or dynamic settings resolution occurs in this test.
func TestNegotiatedEventUsesCurrentOpenIdentity(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configured uint32
		remote     uint32
	}{
		{"dynamic two-octet", 0, 65002},
		{"dynamic four-octet", 0, 4200000002},
		{"static two-octet", 65002, 65002},
		{"static four-octet", 4200000002, 4200000002},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := negotiationSettings(capIPv4())
			settings.LocalAS = 4200000001
			settings.LocalAddress = mustParseAddr("10.0.0.1")
			settings.PeerAS = tc.configured
			settings.IsDynamic = tc.configured == 0
			settings.ProcessBindings = []ProcessBinding{operatorBinding(t, "monitor", "negotiated", "")}
			peer := NewPeer(settings)
			// The fixture packs the peer's ASN4 capability into OPEN optional
			// parameters. negotiateWith resolves that OPEN, not settings.PeerAS.
			session := establishNegotiatedSession(t, peer, capIPv4(), &capability.ASN4{ASN: tc.remote})
			require.Equal(t, tc.remote, session.Negotiated().PeerASN)

			r := New(&Config{ListenAddr: "127.0.0.1:0"})
			srv := flowForwardPluginServer(t, r)
			r.api = srv
			r.eventDispatcher = bgpserver.NewEventDispatcher(srv)
			r.peers[settings.PeerKey()] = peer
			peer.reactor = r
			r.mu.Lock()
			r.publishDeliveryGraphLocked()
			r.mu.Unlock()

			engineSide, pluginSide := net.Pipe()
			proc := process.NewProcess(plugin.PluginConfig{Name: "monitor"})
			proc.SetConn(plugipc.NewPluginConn(engineSide, engineSide))
			proc.StartDelivery(t.Context())
			t.Cleanup(func() {
				proc.Stop()
				if err := pluginSide.Close(); err != nil {
					t.Logf("close plugin side: %v", err)
				}
			})
			srv.Subscriptions().Add(proc, &pluginserver.Subscription{
				Namespace: events.LookupNamespaceID(bgpevents.Namespace),
				EventType: events.LookupEventTypeID(bgpevents.EventNegotiated),
				Direction: events.DirUnspecified,
			})
			log := &stateLog{}
			go answerStateEvents(t.Context(), plugipc.NewPluginConn(pluginSide, pluginSide), log, nil)

			peer.announceNegotiated(r, session, session.Negotiated())

			got := log.recorded()
			require.Len(t, got, 1, "negotiation must reach the subscribed process before Established")
			assert.Contains(t, got[0], `\"type\":\"negotiated\"`)
			assert.Contains(t, got[0], `\"remote\":{\"address\":\"10.0.0.2\",\"as\":`+strconv.FormatUint(uint64(tc.remote), 10)+`}`)
			assert.Contains(t, got[0], `\"local\":{\"address\":\"10.0.0.1\",\"as\":4200000001}`)
			assert.Equal(t, tc.configured, peer.PeerAS(), "event publication must not resolve dynamic settings")
		})
	}
}
