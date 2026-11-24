// Design: docs/architecture/exabgp-bridge.md -- unit tests for the ExaBGP bus endpoint wrapper.
// Related: main.go, main_sdk.go, bridgerun/bus.go, bridgerun/forms.go, bridgerun/fleet.go.

package exabgp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/internal/plugins/exabgp/bridgerun"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestWrapperDeclaresWhatItsTranslatorEmits verifies that the Stage 1
// registration built by the ExaBGP wrapper declares the exact set of CLI send
// prefixes and BGP receive messages defined in the translator's single tables,
// along with FailurePolicy restart and family AFI/SAFI numbers.
//
// VALIDATES: AC-12, AC-33, AC-43 — wrapper declarations derive directly from
// bridge.DeclaredCommandPrefixes() and bridge.DeclaredEventMessages().
// PREVENTS: drift between what the translator emits/renders and what the
// endpoint declares on the bus at Stage 1.
func TestWrapperDeclaresWhatItsTranslatorEmits(t *testing.T) {
	reg, err := buildRegistration([]string{"ipv4/unicast", "ipv4/mpls-label"})
	require.NoError(t, err)

	assert.Equal(t, sdk.FailureRestart, reg.FailurePolicy)
	assert.Equal(t, []string{"bgp"}, reg.WantsConfig)

	require.Len(t, reg.Families, 2)
	assert.Equal(t, sdk.FamilyDecl{Name: "ipv4/unicast", Mode: "both", AFI: 1, SAFI: 1}, reg.Families[0])
	assert.Equal(t, sdk.FamilyDecl{Name: "ipv4/mpls-label", Mode: "both", AFI: 1, SAFI: 4}, reg.Families[1])

	wantSends := bridge.DeclaredCommandPrefixes()
	gotSends := make([]string, len(reg.Sends))
	for i, s := range reg.Sends {
		gotSends[i] = s.Message
	}
	assert.Equal(t, wantSends, gotSends)

	wantReceives := bridge.DeclaredEventMessages()
	gotReceives := make([]string, len(reg.Receives))
	for i, r := range reg.Receives {
		gotReceives[i] = r.Message
	}
	assert.Equal(t, wantReceives, gotReceives)
}

// TestWrapperOptions verifies CLI flag parsing for `ze exabgp plugin`:
// `--encoder` defaults to text and accepts json, `--form` parses global and
// per-neighbor forms (IPv4 and IPv6), and removed `--respawn` / `--on-exit`
// flags are rejected (E-6).
//
// VALIDATES: `--encoder`, `--form`, `--family`, `--route-refresh`, `--add-path`
// parsing and refusal of retired `--respawn` and `--on-exit` options.
// PREVENTS: silent default to JSON when `--encoder` is omitted, or accepting
// deleted bridge lifecycle flags.
func TestWrapperOptions(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		var errBuf bytes.Buffer
		opts, helped, err := parsePluginArgs([]string{"/usr/local/bin/watcher.py"}, &errBuf)
		require.NoError(t, err)
		assert.False(t, helped)
		assert.Equal(t, bridge.EncoderText, opts.Encoder)
		assert.Equal(t, []string{"ipv4/unicast"}, opts.Families)
		assert.False(t, opts.RouteRefresh)
		assert.Empty(t, opts.AddPath)
		assert.Equal(t, bridge.FormParsed, opts.Forms.For("192.0.2.1", "receive"))
		assert.False(t, opts.Forms.NeedsBytes())
		assert.Equal(t, []string{"/usr/local/bin/watcher.py"}, opts.Cmd)
	})

	t.Run("explicit-options", func(t *testing.T) {
		var errBuf bytes.Buffer
		opts, helped, err := parsePluginArgs([]string{
			"--encoder", "json",
			"--family", "ipv4/mpls-label",
			"--family", "ipv6/unicast",
			"--route-refresh",
			"--add-path", "both",
			"--form", "receive-packets",
			"--form", "192.0.2.1:send-packets",
			"--form", "2001:db8::1:receive-consolidate",
			"python3", "script.py", "--arg",
		}, &errBuf)
		require.NoError(t, err)
		assert.False(t, helped)
		assert.Equal(t, bridge.EncoderJSON, opts.Encoder)
		assert.Equal(t, []string{"ipv4/mpls-label", "ipv6/unicast"}, opts.Families)
		assert.True(t, opts.RouteRefresh)
		assert.Equal(t, "both", opts.AddPath)
		assert.True(t, opts.Forms.NeedsBytes())
		assert.Equal(t, bridge.FormPackets, opts.Forms.For("192.0.2.1", "receive"))
		assert.Equal(t, bridge.FormPackets, opts.Forms.For("192.0.2.1", "send"))
		assert.Equal(t, bridge.FormConsolidated, opts.Forms.For("2001:db8::1", "receive"))
		assert.Equal(t, []string{"python3", "script.py", "--arg"}, opts.Cmd)
	})

	t.Run("rejects-removed-and-invalid-flags", func(t *testing.T) {
		for _, args := range [][]string{
			{"--respawn", "script.py"},
			{"--respawn=false", "script.py"},
			{"--on-exit", "keep", "script.py"},
			{"--encoder", "xml", "script.py"},
			{"--form", "not-a-form", "script.py"},
			{"--form", "bad-ip:receive-packets", "script.py"},
		} {
			var errBuf bytes.Buffer
			_, _, err := parsePluginArgs(args, &errBuf)
			assert.Error(t, err, "args %v should be rejected", args)
		}
	})
}

// TestWrapperScopeFromFilter verifies that Fleet.ApplyFilter updates the
// translator's peer scope, NoPeers flag, capability peers, negotiated grant,
// and EventFormat both at Stage 2 and on live filter rebuilds.
//
// VALIDATES: Q-E3, Q-E9 — BusParticipantFilter drives Translator.Peers,
// Translator.NoPeers, CapabilityPeers, negotiated grant, and EventFormat.
// PREVENTS: stale peer scope on rebuild, wildcard announcements leaking past
// `send > to`, or capability collisions between scripts on disjoint peers.
func TestWrapperScopeFromFilter(t *testing.T) {
	fleet := bridgerun.New(slogutil.DiscardLogger(), []string{"ipv4/unicast"}, []bridgerun.Script{{
		Name:    "watcher",
		Argv:    []string{"echo"},
		Encoder: bridge.EncoderJSON,
	}})

	// Initial unscoped state.
	peers, noPeers := fleet.TranslatorScope()
	assert.Empty(t, peers)
	assert.False(t, noPeers)
	assert.Nil(t, fleet.CapabilityPeers())
	assert.True(t, fleet.GrantsNegotiated("192.0.2.1"))

	// Narrow to two peers for send and one peer for bgp:negotiated, no bgp:update.
	narrowed := &rpc.BusParticipantFilter{
		SendRules: []rpc.BusFilterRule{{
			Message: "cli:send bgp",
			Allowed: true,
			Targets: []rpc.BusFilterTarget{
				{Path: "/bgp/peer/p2", Address: "192.0.2.2"},
				{Path: "/bgp/peer/p1", Address: "192.0.2.1"},
			},
		}},
		ReceiveRules: []rpc.BusFilterRule{
			{Message: "bgp:state", Allowed: true, TargetAll: true},
			{Message: "bgp:negotiated", Allowed: true, Targets: []rpc.BusFilterTarget{{Path: "/bgp/peer/p1", Address: "192.0.2.1"}}},
			{Message: "bgp:update", Allowed: false},
		},
	}
	fleet.ApplyFilter(narrowed)

	peers, noPeers = fleet.TranslatorScope()
	assert.Equal(t, []string{"192.0.2.1", "192.0.2.2"}, peers)
	assert.False(t, noPeers)
	assert.Equal(t, []string{"192.0.2.1", "192.0.2.2"}, fleet.CapabilityPeers())
	assert.True(t, fleet.GrantsNegotiated("192.0.2.1"))
	assert.False(t, fleet.GrantsNegotiated("192.0.2.2"))
	assert.Equal(t, plugin.FormatParsed, bridgerun.EventFormat(narrowed, bridgerun.FormSpecs{}))

	caps := buildCapabilityDecls([]string{"ipv4/unicast"}, true, "receive", fleet.CapabilityPeers())
	require.Len(t, caps, 2)
	assert.Equal(t, []string{"192.0.2.1", "192.0.2.2"}, caps[0].Peers)
	assert.Equal(t, []string{"192.0.2.1", "192.0.2.2"}, caps[1].Peers)

	// Rebuild to send-to-none and allow bgp:update.
	rebuilt := &rpc.BusParticipantFilter{
		SendToNone: true,
		ReceiveRules: []rpc.BusFilterRule{
			{Message: "bgp:update", Allowed: true, TargetAll: true},
			{Message: "bgp:negotiated", Allowed: false},
		},
	}
	fleet.ApplyFilter(rebuilt)

	peers, noPeers = fleet.TranslatorScope()
	assert.Empty(t, peers)
	assert.True(t, noPeers)
	assert.Nil(t, fleet.CapabilityPeers())
	assert.False(t, fleet.GrantsNegotiated("192.0.2.1"))
	assert.Equal(t, plugin.FormatFull, bridgerun.EventFormat(rebuilt, bridgerun.FormSpecs{}))
}

// TestWrapperIdentityIsEndpointName verifies that the wrapper authenticates
// and names its script by ZE_PLUGIN_NAME (falling back to the script basename
// when unset), and declares FailurePolicy restart.
//
// VALIDATES: Endpoint identity comes from ZE_PLUGIN_NAME rather than the
// retired hardcoded "exabgp-bridge" constant.
// PREVENTS: TLS token mismatch when an endpoint is named after its process.
func TestWrapperIdentityIsEndpointName(t *testing.T) {
	t.Setenv("ZE_PLUGIN_NAME", "route-watcher")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	assert.Equal(t, "route-watcher", resolveEndpointName([]string{"/opt/scripts/watch.py"}))

	t.Setenv("ZE_PLUGIN_NAME", "")
	env.ResetCache()
	assert.Equal(t, "watch.py", resolveEndpointName([]string{"/opt/scripts/watch.py"}))

	reg, err := buildRegistration([]string{"ipv4/unicast"})
	require.NoError(t, err)
	assert.Equal(t, sdk.FailureRestart, reg.FailurePolicy)
}

// TestWrapperExitsWithItsScript verifies both AC-45 / E-6 (the wrapper returns
// its script's exit code without restarting it) and query mode
// (`ZE_PLUGIN_MODE=declare` emits the Stage 1 declaration JSON line and never
// starts the script).
//
// VALIDATES: AC-45 / S42 / E-6 and AC-18 / S16 query mode.
// PREVENTS: wrapper respawning an exited script, swallowing its exit code, or
// executing the script during offline `ZE_PLUGIN_MODE=declare` interrogation.
func TestWrapperExitsWithItsScript(t *testing.T) {
	t.Run("mirrors-script-exit-status", func(t *testing.T) {
		dir := t.TempDir()
		forksFile := filepath.Join(dir, "forks")
		scriptPath := filepath.Join(dir, "exit3.sh")
		require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho fork >> "+forksFile+"\nexit 3\n"), 0o700))

		opts, _, err := parsePluginArgs([]string{"--encoder", "json", "--route-refresh", scriptPath}, io.Discard)
		require.NoError(t, err)
		reg, err := buildRegistration(opts.Families)
		require.NoError(t, err)

		engineEnd, pluginEnd := net.Pipe()
		t.Cleanup(func() {
			_ = engineEnd.Close()
			_ = pluginEnd.Close()
		})

		p := sdk.NewWithConn("exit-watcher", pluginEnd)
		engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
		t.Cleanup(func() { _ = engineMux.Close() })

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		codeCh := make(chan int, 1)
		go func() {
			codeCh <- runSDKWithPlugin(ctx, p, "exit-watcher", opts, &reg)
		}()

		readReq := func() *rpc.Request {
			t.Helper()
			select {
			case req, ok := <-engineMux.Requests():
				require.True(t, ok, "engine mux closed before request arrived")
				return req
			case <-ctx.Done():
				t.Fatalf("timed out waiting for plugin request: %v", ctx.Err())
				return nil
			}
		}

		// Drive the 5-stage startup handshake and trigger OnAllPluginsReady.
		req := readReq()
		require.Equal(t, rpc.MethodDeclareRegistration, req.Method)
		require.NoError(t, engineMux.SendResult(ctx, req.ID, &rpc.DeclareRegistrationOutput{
			ProtocolVersion: rpc.ProtocolVersion,
		}))

		_, err = engineMux.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{
			Sections: []rpc.ConfigSection{{Root: "bgp", Data: `{}`}},
			ParticipantFilter: &rpc.BusParticipantFilter{
				ParticipantPath: "/bus/endpoint/exit-watcher",
				EndpointName:    "exit-watcher",
				SendRules: []rpc.BusFilterRule{{
					Message: "cli:send bgp",
					Allowed: true,
					Targets: []rpc.BusFilterTarget{{Path: "/bgp/peer/p1", Address: "192.0.2.1"}},
				}},
				ReceiveRules: []rpc.BusFilterRule{{
					Message:   "bgp:state",
					Allowed:   true,
					TargetAll: true,
				}},
			},
		})
		require.NoError(t, err)

		req = readReq()
		require.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
		var capIn rpc.DeclareCapabilitiesInput
		require.NoError(t, json.Unmarshal(req.Params, &capIn))
		require.Len(t, capIn.Capabilities, 1)
		assert.Equal(t, uint8(2), capIn.Capabilities[0].Code)
		assert.Equal(t, []string{"192.0.2.1"}, capIn.Capabilities[0].Peers)
		require.NoError(t, engineMux.SendOK(ctx, req.ID))

		_, err = engineMux.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{})
		require.NoError(t, err)

		req = readReq()
		require.Equal(t, "ze-plugin-engine:ready", req.Method)
		var readyIn rpc.ReadyInput
		require.NoError(t, json.Unmarshal(req.Params, &readyIn))
		assert.Equal(t, plugin.FormatParsed, readyIn.Format)
		require.NoError(t, engineMux.SendOK(ctx, req.ID))

		_, err = engineMux.CallRPC(ctx, "ze-plugin-callback:post-startup", struct{}{})
		require.NoError(t, err)

		select {
		case code := <-codeCh:
			assert.Equal(t, 3, code, "wrapper must exit with its script's exit code 3")
		case <-time.After(5 * time.Second):
			t.Fatal("wrapper did not exit after its script exited")
		}

		data, err := os.ReadFile(forksFile) //nolint:gosec // G304: test-owned temp file.
		require.NoError(t, err)
		assert.Equal(t, 1, strings.Count(string(data), "\n"), "wrapper must never respawn its script")
	})

	t.Run("query-mode-declares-without-starting-script", func(t *testing.T) {
		dir := t.TempDir()
		sentinel := filepath.Join(dir, "ran")
		scriptPath := filepath.Join(dir, "never.sh")
		require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\ntouch "+sentinel+"\n"), 0o700))

		t.Setenv("ZE_PLUGIN_MODE", "declare")
		t.Setenv("ZE_PLUGIN_HUB_TOKEN", "")
		env.ResetCache()
		t.Cleanup(env.ResetCache)

		// Capture stdout while running cmdPlugin in query mode.
		origStdout := os.Stdout
		r, w, err := os.Pipe()
		require.NoError(t, err)
		os.Stdout = w
		defer func() { os.Stdout = origStdout }()

		code := cmdPlugin([]string{"--family", "ipv4/mpls-label", scriptPath})
		require.NoError(t, w.Close())
		os.Stdout = origStdout

		out, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())

		assert.Equal(t, exitOK, code)
		line := strings.TrimSpace(string(out))
		assert.True(t, strings.HasPrefix(line, "#1 "+rpc.MethodDeclareRegistration+" "), "got stdout: %q", line)
		assert.Contains(t, line, `"ipv4/mpls-label"`)
		assert.Contains(t, line, `"failure-policy":"restart"`)
		assert.Contains(t, line, `"cli:send bgp"`)
		assert.Contains(t, line, `"bgp:update"`)

		_, statErr := os.Stat(sentinel)
		assert.True(t, os.IsNotExist(statErr), "query mode must never start the script")
	})
}
