// Design: docs/architecture/behavior/peer-lifecycle.md -- immutable OPEN preparation.
// Related: peer_settings_negotiation.go -- pluginCapabilitiesKept.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestPluginCapabilitiesKeptBeforeOpen distinguishes an unprepared session from
// one that froze its OPEN but has not sent it. A plugin-store change is captured
// by the former's first preparation; the latter must restart even before a peer
// OPEN arrives. Check the reload decision and exact frozen declaration and bytes.
func TestPluginCapabilitiesKeptBeforeOpen(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		name := "unprepared reads the new store"
		if prepared {
			name = "prepared keeps its frozen offer and restarts"
		}
		t.Run(name, func(t *testing.T) {
			settings := negotiationSettings(capIPv4())
			r, peer := newSwapTestReactor(t, settings, settings)
			store := &pluginStore{}
			store.set(roleCapability(0))
			r.pluginCapabilitySeam = store.get
			session := peer.newSession()
			if prepared {
				require.NoError(t, session.prepareOpen())
			}
			store.set(roleCapability(3))
			assert.Equal(t, !prepared, session.pluginCapabilitiesKept())
			_, reason := peerSettingsSwapPlan(settings, settings, session)
			wantReason := ""
			wantValue := byte(3)
			if prepared {
				wantReason = "PluginCapabilities"
				wantValue = 0
			}
			assert.Equal(t, wantReason, reason)
			require.NoError(t, session.prepareOpen())
			require.NotNil(t, session.proposal)
			assert.Equal(t, pluginDeclarationIndex([]plugin.InjectedCapability{roleCapability(wantValue)}), session.proposal.pluginDeclarations)
			caps, err := capability.ParseFromOptionalParams(session.proposal.message.OptionalParams, session.proposal.message.ExtendedParams)
			require.NoError(t, err)
			var role []byte
			for _, offered := range caps {
				if uint8(offered.Code()) == 9 {
					role = make([]byte, offered.Len())
					offered.WriteTo(role, 0)
				}
			}
			assert.Equal(t, []byte{9, 1, wantValue}, role)
		})
	}
}

// TestPluginCapabilitiesKeptDuringPreparation probes from inside the actual
// store read, after it captured the old declaration and published a replacement.
// Preparation still owns openMu and has not published its proposal, so nil
// cannot authorize keeping the session on the assumption it will read again.
func TestPluginCapabilitiesKeptDuringPreparation(t *testing.T) {
	settings := negotiationSettings(capIPv4())
	r, peer := newSwapTestReactor(t, settings, settings)
	store := &pluginStore{}
	store.set(roleCapability(0))
	session := peer.newSession()
	r.pluginCapabilitySeam = func(selectors ...string) []plugin.InjectedCapability {
		declared := store.get(selectors...)
		r.pluginCapabilitySeam = store.get
		store.set(roleCapability(3))
		assert.False(t, session.pluginCapabilitiesKept(), "an in-flight old store snapshot cannot authorize keep")
		return declared
	}
	func() {
		session.openMu.Lock()
		defer session.unlockOpen()
		require.NoError(t, session.prepareOpen())
	}()
	require.NotNil(t, session.proposal)
	assert.Equal(t, pluginDeclarationIndex([]plugin.InjectedCapability{roleCapability(0)}), session.proposal.pluginDeclarations)
	assert.False(t, session.pluginCapabilitiesKept(), "the frozen old declaration still requires restart")
}

// TestPrepareOpenFailureReleasesProposalCapture uses the existing unencodable
// Paths-Limit boundary, then repairs that input and retries the same Session.
// A failed construction must leave neither a proposal nor an owned capture lock.
func TestPrepareOpenFailureReleasesProposalCapture(t *testing.T) {
	settings := negotiationSettings(capIPv4())
	entries := make([]capability.PathsLimitEntry, 52)
	for i := range entries {
		entries[i] = capability.PathsLimitEntry{
			AFI: capability.AFI(i + 1), SAFI: capability.SAFIUnicast, Limit: 1,
		}
	}
	settings.Capabilities = append(settings.Capabilities, &capability.PathsLimit{Entries: entries})
	session := NewSession(settings)
	prepare := func() error {
		session.openMu.Lock()
		defer session.unlockOpen()
		return session.prepareOpen()
	}
	require.Error(t, prepare())
	assert.Nil(t, session.proposal)
	assert.True(t, session.pluginCapabilitiesKept(), "failed preparation leaves a genuinely unprepared session")
	settings.Capabilities = []capability.Capability{capIPv4()}
	require.NoError(t, prepare())
	require.NotNil(t, session.proposal)
	assert.True(t, session.pluginCapabilitiesKept(), "successful retry owns the unchanged empty plugin declaration")
}
