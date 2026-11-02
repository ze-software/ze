package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// capabilityReloadPeer is the peer every declaration in these tests names.
const capabilityReloadPeer = "127.0.0.2"

// roleDeclaration declares capability 9 with payload for capabilityReloadPeer.
func roleDeclaration(payload string) *rpc.DeclareCapabilitiesInput {
	return &rpc.DeclareCapabilitiesInput{Capabilities: []rpc.CapabilityDecl{{
		Code: 9, Encoding: rpc.CapEncodingHex, Payload: payload, Peers: []string{capabilityReloadPeer},
	}}}
}

// seedCapabilities installs declared as plugin name's running declaration,
// in the store and in its process record, as Stage 3 does.
func seedCapabilities(t *testing.T, s *Server, name string, declared *rpc.DeclareCapabilitiesInput) {
	t.Helper()
	caps := capabilitiesFromRPC(declared)
	caps.PluginName = name
	require.NoError(t, s.capInjector.AddPluginCapabilities(caps))
	proc := s.procManager.Load().GetProcess(name)
	require.NotNil(t, proc)
	proc.SetCapabilities(caps)
}

// peerRoleValues lists the values of capability 9 the store holds for
// capabilityReloadPeer.
func peerRoleValues(s *Server) []string {
	var values []string
	for _, injected := range s.capInjector.GetCapabilitiesForSelectors(capabilityReloadPeer) {
		if injected.Code == 9 {
			values = append(values, string(injected.Value))
		}
	}
	return values
}

// recordedPayloads lists the payloads the "caps" plugin's process record holds.
func recordedPayloads(t *testing.T, s *Server) []string {
	t.Helper()
	proc := s.procManager.Load().GetProcess("caps")
	require.NotNil(t, proc)
	var payloads []string
	if caps := proc.Capabilities(); caps != nil {
		for _, capability := range caps.Capabilities {
			payloads = append(payloads, capability.Payload)
		}
	}
	return payloads
}

// TestConfigTxBridgeInstallsAndRestoresCapabilities follows the capability
// declaration a verify answer carries through a reload transaction.
//
// VALIDATES: AC-4. A committed reload installs the candidate declaration in
// the store and the process record; a rolled-back one leaves the previous
// declaration in both; a verify answer carrying none changes nothing; a
// candidate that conflicts with another plugin's declaration is refused at
// verify, before any apply.
// PREVENTS: a rolled-back reload leaving its candidate capabilities in every
// later OPEN, or a conflict reaching the store.
// METHOD: real bridge and coordinator over net.Pipe plugins, with the
// injector the engine reads at every OPEN build.
func TestConfigTxBridgeInstallsAndRestoresCapabilities(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	running := roleDeclaration("00")
	candidate := roleDeclaration("03")

	t.Run("committed installs the candidate", func(t *testing.T) {
		t.Parallel()
		plugins := []pluginDef{{
			name: "caps", roots: []string{"bgp"},
			verifyResp: &rpc.ConfigVerifyOutput{Status: rpc.StatusOK, Capabilities: candidate},
		}}
		s := newTestReloadServer(t, &mockReloadReactor{tree: oldTree}, plugins)
		s.capInjector = plugin.NewCapabilityInjector()
		seedCapabilities(t, s, "caps", running)

		require.NoError(t, s.ReloadConfig(context.Background(), newTree))
		assert.Equal(t, []string{"\x03"}, peerRoleValues(s), "the store holds the candidate declaration")
		assert.Equal(t, []string{"03"}, recordedPayloads(t, s), "the process record holds the candidate declaration")
	})

	t.Run("rolled back restores the previous declaration", func(t *testing.T) {
		t.Parallel()
		plugins := []pluginDef{
			{
				name: "caps", roots: []string{"bgp"},
				verifyResp: &rpc.ConfigVerifyOutput{Status: rpc.StatusOK, Capabilities: candidate},
			},
			{
				name: "failing", roots: []string{"bgp"},
				applyResp: &rpc.ConfigApplyOutput{Status: plugin.StatusError, Error: "journal full"},
			},
		}
		s := newTestReloadServer(t, &mockReloadReactor{tree: oldTree}, plugins)
		s.capInjector = plugin.NewCapabilityInjector()
		seedCapabilities(t, s, "caps", running)

		err := s.ReloadConfig(context.Background(), newTree)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "journal full")
		require.Positive(t, plugins[0].responder.getApplyCalls(), "the apply that installs the candidate ran")
		assert.Equal(t, []string{"\x00"}, peerRoleValues(s), "the store holds the previous declaration")
		assert.Equal(t, []string{"00"}, recordedPayloads(t, s), "the process record holds the previous declaration")
	})

	t.Run("an answer carrying no declaration keeps the current one", func(t *testing.T) {
		t.Parallel()
		plugins := []pluginDef{{name: "caps", roots: []string{"bgp"}}}
		s := newTestReloadServer(t, &mockReloadReactor{tree: oldTree}, plugins)
		s.capInjector = plugin.NewCapabilityInjector()
		seedCapabilities(t, s, "caps", running)

		require.NoError(t, s.ReloadConfig(context.Background(), newTree))
		assert.Equal(t, []string{"\x00"}, peerRoleValues(s))
		assert.Equal(t, []string{"00"}, recordedPayloads(t, s))
	})

	t.Run("a conflict with another plugin is refused at verify", func(t *testing.T) {
		t.Parallel()
		plugins := []pluginDef{
			{
				name: "caps", roots: []string{"bgp"},
				verifyResp: &rpc.ConfigVerifyOutput{Status: rpc.StatusOK, Capabilities: candidate},
			},
			{name: "holder", roots: []string{"bgp"}},
		}
		s := newTestReloadServer(t, &mockReloadReactor{tree: oldTree}, plugins)
		s.capInjector = plugin.NewCapabilityInjector()
		seedCapabilities(t, s, "holder", running)

		err := s.ReloadConfig(context.Background(), newTree)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "capability conflict: code 9 for peer 127.0.0.2 already registered by holder")
		assert.Zero(t, plugins[0].responder.getApplyCalls(), "a refused verify reaches no apply")
		assert.Equal(t, []string{"\x00"}, peerRoleValues(s), "the store keeps the holder's declaration")
		assert.Empty(t, recordedPayloads(t, s))
	})
}
