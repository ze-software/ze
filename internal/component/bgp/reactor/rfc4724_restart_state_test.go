// Design: docs/guide/graceful-restart.md -- what Ze advertises in the GR capability
// RFC: rfc/short/rfc4724.md -- Sections 4.1 and 4.2, the Restart State bit in the OPEN
// Overview: peer.go -- getPluginCapabilities, the plugin capabilities the OPEN carries
// Related: peer_gr_flags.go -- restartFlagsFor, the bit edit the restart window gates

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// rfc4724SentGR builds the OPEN a session sends with the production capability getter
// (Peer.getPluginCapabilities) wired as peer_run.go wires it, while the bgp-gr plugin
// declares Restart Time 120 for IPv4 unicast with every flag clear, and returns the
// Graceful Restart Capability that OPEN carries. restartUntil is the reactor's restart
// deadline; the zero time is a cold start.
func rfc4724SentGR(t *testing.T, restartUntil time.Time) *capability.GracefulRestart {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	r := &Reactor{
		config: &Config{},
		pluginCapabilitySeam: func(_ ...string) []plugin.InjectedCapability {
			// Restart flags 0, Restart Time 120; AFI 1, SAFI 1, Flags for Address Family 0.
			return []plugin.InjectedCapability{{Code: 64, Value: []byte{0x00, 0x78, 0x00, 0x01, 0x01, 0x00}, Plugin: "bgp-gr"}}
		},
	}
	r.SetRestartUntil(restartUntil)
	peer := NewPeer(settings)
	peer.SetReactor(r)
	session := NewSession(settings)
	session.setPluginCapabilityGetter(peer.getPluginCapabilities)

	open, err := session.buildOpen(settings, settings.Capabilities)
	require.NoError(t, err)
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	require.NoError(t, err)
	var sent []*capability.GracefulRestart
	for _, c := range caps {
		if gr, ok := c.(*capability.GracefulRestart); ok {
			sent = append(sent, gr)
		}
	}
	require.Len(t, sent, 1, "the OPEN carries the plugin's Graceful Restart Capability once")
	return sent[0]
}

// TestRFC4724OpenInsideTheRestartWindowSetsRestartState builds the OPEN a session sends
// while the reactor's restart deadline is an hour away.
//
// VALIDATES: RFC 4724 Section 4.1 -- "To re-establish the session with its peer, the
// Restarting Speaker MUST set the "Restart State" bit in the Graceful Restart Capability of
// the OPEN message." getPluginCapabilities sets R on the code 64 value a plugin declared
// with R clear, and leaves the Restart Time and the family tuple as declared.
// PREVENTS: an OPEN from a restarting Ze that tells the peer nothing restarted, so the peer
// keeps waiting for routes Ze will not re-advertise in the form it expects.
//
// RFC requirement: RFC4724-4.1-4 positive -- with the restart deadline an hour ahead, the OPEN built through Peer.getPluginCapabilities carries the plugin's Graceful Restart Capability with the Restart State bit set, Restart Time 120 and the IPv4 unicast tuple unchanged.
// RFC requirement: RFC4724-4.2-6 negative -- the exception the MUST NOT allows: inside the restart window the speaker has restarted, and the same OPEN carries the Restart State bit set.
func TestRFC4724OpenInsideTheRestartWindowSetsRestartState(t *testing.T) {
	gr := rfc4724SentGR(t, time.Now().Add(time.Hour))

	assert.True(t, gr.RestartState, "a restarting speaker sets the Restart State bit")
	assert.Equal(t, uint16(120), gr.RestartTime, "the Restart Time is the plugin's")
	assert.Equal(t, []capability.GracefulRestartFamily{{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}},
		gr.Families, "the family tuple is the plugin's, F clear: no forwarding plane preserved")
}

// TestRFC4724OpenOutsideTheRestartWindowClearsRestartState builds the OPEN on a cold start
// (no restart deadline) and after the deadline has passed.
//
// VALIDATES: RFC 4724 Section 4.2 -- "In re-establishing the session, the "Restart State"
// bit in the Graceful Restart Capability of the OPEN message sent by the Receiving Speaker
// MUST NOT be set unless the Receiving Speaker has restarted." Outside the window
// getPluginCapabilities leaves R clear, so the bit is set only while restarting.
// PREVENTS: a Restart State bit set on every OPEN, which makes the peer treat each
// re-establishment as a restart of Ze and defer its own End-of-RIB handling.
//
// RFC requirement: RFC4724-4.2-6 positive -- on a cold start and once the restart deadline has passed, the OPEN built through Peer.getPluginCapabilities carries the Graceful Restart Capability with the Restart State bit clear and Restart Time 120.
// RFC requirement: RFC4724-4.1-4 negative -- the Restart State bit is not set blanket: the same production path leaves it clear when the speaker is not restarting.
func TestRFC4724OpenOutsideTheRestartWindowClearsRestartState(t *testing.T) {
	cases := []struct {
		name         string
		restartUntil time.Time
	}{
		{"cold start", time.Time{}},
		{"deadline passed", time.Now().Add(-time.Second)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gr := rfc4724SentGR(t, tc.restartUntil)

			assert.False(t, gr.RestartState, "a speaker that has not restarted leaves the Restart State bit clear")
			assert.Equal(t, uint16(120), gr.RestartTime, "the Restart Time is the plugin's")
		})
	}
}
