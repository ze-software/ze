// Design: docs/architecture/core-design.md — config reload delivers changed peer settings
// Related: peer_settings_negotiation.go — pluginDeclarationsKept, the comparison under test
package reactor

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// pluginStore is a capability store a test rewrites between the OPEN and the
// reload probe, as a reload's first apply rewrites the engine's.
type pluginStore struct {
	mu       sync.Mutex
	injected []plugin.InjectedCapability
}

func (ps *pluginStore) set(injected ...plugin.InjectedCapability) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.injected = injected
}

func (ps *pluginStore) get(...string) []plugin.InjectedCapability {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return append([]plugin.InjectedCapability(nil), ps.injected...)
}

// roleCapability is a plugin declaration of capability 9 with one value byte.
func roleCapability(value byte) plugin.InjectedCapability {
	return plugin.InjectedCapability{Code: 9, Value: []byte{value}, Plugin: "plugin-under-test"}
}

// TestNegotiationPluginCapabilityChange runs the reload probe after a reload
// changed a plugin's declaration in the store.
//
// VALIDATES: AC-5. A running session restarts when a plugin capability its
// OPEN carried changes and the peer advertised that code, and keeps running
// when the peer did not (RFC 5492 Section 1: a capability is in use only once
// the peer supports it). An unchanged declaration keeps the session.
// PREVENTS: a session that keeps running under the old Role after a reload
// changed it, which the peer never learns until the session drops.
// METHOD: the session's OPEN is built through prepareOpen with the Peer's own
// getters reading a store the test then rewrites, and the probe is asked with
// the peer's unchanged settings, so only the plugin declaration differs.
func TestNegotiationPluginCapabilityChange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		sent       []plugin.InjectedCapability
		next       []plugin.InjectedCapability
		peerCaps   []capability.Capability
		keepsGoing bool
	}{
		{
			name: "changed code the peer advertised restarts",
			sent: []plugin.InjectedCapability{roleCapability(0)}, next: []plugin.InjectedCapability{roleCapability(3)},
			peerCaps: []capability.Capability{capIPv4(), capability.NewPlugin(9, []byte{3})},
		},
		{
			name:     "added code the peer advertised restarts",
			next:     []plugin.InjectedCapability{roleCapability(3)},
			peerCaps: []capability.Capability{capIPv4(), capability.NewPlugin(9, []byte{3})},
		},
		{
			name:     "removed code the peer advertised restarts",
			sent:     []plugin.InjectedCapability{roleCapability(0)},
			peerCaps: []capability.Capability{capIPv4(), capability.NewPlugin(9, []byte{3})},
		},
		{
			name: "changed code the peer never advertised keeps the session",
			sent: []plugin.InjectedCapability{roleCapability(0)}, next: []plugin.InjectedCapability{roleCapability(3)},
			peerCaps: []capability.Capability{capIPv4()}, keepsGoing: true,
		},
		{
			name: "unchanged declaration keeps the session",
			sent: []plugin.InjectedCapability{roleCapability(0)}, next: []plugin.InjectedCapability{roleCapability(0)},
			peerCaps: []capability.Capability{capIPv4(), capability.NewPlugin(9, []byte{3})}, keepsGoing: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			settings := negotiationSettings(capIPv4())
			r, peer := newSwapTestReactor(t, settings, settings)
			store := &pluginStore{}
			store.set(tc.sent...)
			r.pluginCapabilitySeam = store.get

			s := establishNegotiatedSession(t, peer, tc.peerCaps...)
			s.setPluginCapabilityGetter(peer.getPluginCapabilities)
			s.setPluginDeclarationGetter(peer.pluginCapabilityDeclarations)
			require.NoError(t, s.prepareOpen(), "the running OPEN carries the sent declaration")

			store.set(tc.next...)
			assert.Equal(t, tc.keepsGoing, s.negotiationOutcomeUnchanged(peer.settings))
		})
	}
}
