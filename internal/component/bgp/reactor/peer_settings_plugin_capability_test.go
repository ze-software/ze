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

// TestReloadPluginCapabilityChange runs a whole reload whose ONLY change is a
// plugin's capability declaration.
//
// VALIDATES: AC-5 through the reload path. A reload whose PeerSettings are
// unchanged still restarts a session whose OPEN carried a plugin capability
// that changed for a code the peer advertised, and keeps a session whose peer
// never advertised the code (RFC 5492 Section 1).
// PREVENTS: the comparison living only in negotiationOutcomeUnchanged, which
// the reconcile loop never reached for a peer whose settings were equal: a
// Role, software-version or bridge change a plugin made left the session
// running on the OPEN it had sent.
// METHOD: adapter.Reload with the peer's own settings, after the test rewrote
// the store the session's OPEN was built from. A restart replaces the *Peer
// (reactor_peers.go AddPeer), so object identity is the assertion.
func TestReloadPluginCapabilityChange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		sent, next []plugin.InjectedCapability
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
			next := negotiationSettings(capIPv4())
			r, peer := newSwapTestReactor(t, settings, next)
			store := &pluginStore{}
			store.set(tc.sent...)
			r.pluginCapabilitySeam = store.get

			s := establishNegotiatedSession(t, peer, tc.peerCaps...)
			s.setPluginCapabilityGetter(peer.getPluginCapabilities)
			s.setPluginDeclarationGetter(peer.pluginCapabilityDeclarations)
			require.NoError(t, s.prepareOpen(), "the running OPEN carries the sent declaration")
			peer.state.Store(int32(PeerStateEstablished))

			store.set(tc.next...)
			require.NoError(t, (&reactorAPIAdapter{r: r}).Reload())

			r.mu.RLock()
			after := r.peers[settings.PeerKey()]
			r.mu.RUnlock()
			require.NotNil(t, after, "the peer must still be configured after reload")
			assert.Equal(t, tc.keepsGoing, after == peer,
				"keepsGoing=%v: a restart replaces the peer object", tc.keepsGoing)
		})
	}
}

// TestReloadCapabilityBlockEdit reloads a peer whose capability block changed,
// which reaches the reactor as a new RawCapabilityConfig beside the plugin
// declaration the edit produced.
//
// VALIDATES: a capability-block edit restarts the session only when the
// negotiation it reaches changes: a plugin capability changed for a code the
// peer advertised. An edit whose code the peer never advertised, or whose
// declaration did not change, keeps the session and delivers the new block
// (RFC 5492 Section 1; the negotiation-outcome ruling of 2026-08-07).
// PREVENTS: the block's parsed values restarting every session
// on any edit, which hid the plugin comparison behind a blanket restart: a
// software-version encoding edit flapped a peer that never advertised code 75.
// METHOD: adapter.Reload with settings that differ from the running ones in
// the block field (and, in one case, in a capability set that negotiates
// identically), after the test rewrote the store. Object identity tells a
// restart (reactor_peers.go AddPeer replaces the *Peer), and the running
// peer's settings show the block was delivered rather than dropped.
func TestReloadCapabilityBlockEdit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		sent, next []plugin.InjectedCapability
		nextCaps   []capability.Capability
		peerCaps   []capability.Capability
		keepsGoing bool
	}{
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
		{
			name: "capability set negotiating the same keeps the session",
			sent: []plugin.InjectedCapability{roleCapability(0)}, next: []plugin.InjectedCapability{roleCapability(3)},
			nextCaps: []capability.Capability{capIPv4(), capIPv6()},
			peerCaps: []capability.Capability{capIPv4()}, keepsGoing: true,
		},
		{
			name: "changed code the peer advertised restarts",
			sent: []plugin.InjectedCapability{roleCapability(0)}, next: []plugin.InjectedCapability{roleCapability(3)},
			peerCaps: []capability.Capability{capIPv4(), capability.NewPlugin(9, []byte{3})},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			settings := negotiationSettings(capIPv4())
			settings.RawCapabilityConfig = map[string]map[string]string{"hostname": {"host": "old"}}
			next := negotiationSettings(capIPv4())
			if tc.nextCaps != nil {
				next.Capabilities = tc.nextCaps
			}
			next.RawCapabilityConfig = map[string]map[string]string{"hostname": {"host": "new"}}
			r, peer := newSwapTestReactor(t, settings, next)
			store := &pluginStore{}
			store.set(tc.sent...)
			r.pluginCapabilitySeam = store.get

			s := establishNegotiatedSession(t, peer, tc.peerCaps...)
			s.setPluginCapabilityGetter(peer.getPluginCapabilities)
			s.setPluginDeclarationGetter(peer.pluginCapabilityDeclarations)
			require.NoError(t, s.prepareOpen(), "the running OPEN carries the sent declaration")
			peer.state.Store(int32(PeerStateEstablished))

			store.set(tc.next...)
			require.NoError(t, (&reactorAPIAdapter{r: r}).Reload())

			r.mu.RLock()
			after := r.peers[settings.PeerKey()]
			r.mu.RUnlock()
			require.NotNil(t, after, "the peer must still be configured after reload")
			require.Equal(t, tc.keepsGoing, after == peer,
				"keepsGoing=%v: a restart replaces the peer object", tc.keepsGoing)
			running := after.Settings()
			assert.Equal(t, next.RawCapabilityConfig, running.RawCapabilityConfig,
				"the running peer holds the edited capability values")
			assert.True(t, capabilitiesEqual(next.Capabilities, after.configuredCapabilities()),
				"the running peer holds the reloaded capability set")
		})
	}
}
