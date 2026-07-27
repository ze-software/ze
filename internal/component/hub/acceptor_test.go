package hub

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// writeProbePlugin writes a shell "plugin" that records the plugin connect-back
// environment the engine handed it and then exits. Exiting immediately makes the
// subsequent TLS connect-back wait terminate at once (the process monitor cancels
// the process context), so the test needs no sleep and no timeout budget.
func writeProbePlugin(t *testing.T, dir string) (script, envDump string) {
	t.Helper()

	envDump = filepath.Join(dir, "plugin-env.txt")
	script = filepath.Join(dir, "probe-plugin.sh")
	body := "#!/bin/sh\n" +
		"printf '%s\\n%s\\n%s\\n%s\\n' " +
		"\"$ZE_PLUGIN_HUB_HOST\" \"$ZE_PLUGIN_HUB_PORT\" " +
		"\"$ZE_PLUGIN_HUB_TOKEN\" \"$ZE_PLUGIN_CERT_FP\" > '" + envDump + "'\n"
	require.NoError(t, os.WriteFile(script, []byte(body), 0o700)) //nolint:gosec // test fixture must be executable
	return script, envDump
}

// TestOrchestratorStartWiresTLSAcceptor drives the hub orchestrator start path
// with a pure `plugin { external ... }` config -- the exact path
// cmd/ze/hub/main.go runOrchestratorWithData takes for a hub config with no
// bgp/ospf/... block.
//
// VALIDATES: Orchestrator.Start establishes a TLS acceptor before starting
// forked subsystems, so an external plugin is spawned with a reachable
// ZE_PLUGIN_HUB_HOST/PORT/TOKEN/CERT_FP.
// PREVENTS: regression to "no TLS acceptor configured (hub config required for
// external plugins)" -- the orchestrator/subsystem path never called
// Process.SetAcceptor, so every external plugin declared in a pure hub config
// died before exec.
func TestOrchestratorStartWiresTLSAcceptor(t *testing.T) {
	dir := t.TempDir()
	script, envDump := writeProbePlugin(t, dir)

	o := NewOrchestrator(&HubConfig{
		Plugins: []PluginDef{{Name: "probe", Run: script}},
	})
	t.Cleanup(o.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := o.Start(ctx)

	// The probe exits without performing the 5-stage handshake, so Start must
	// fail -- but it must fail at connect-back, i.e. AFTER the child was
	// spawned against a live acceptor, never at acceptor configuration.
	require.Error(t, err)
	require.NotContains(t, err.Error(), "no TLS acceptor configured",
		"external subsystem started without a TLS acceptor")
	require.Contains(t, err.Error(), "TLS connect-back")

	// Positive proof: the acceptor existed and its address plus a per-plugin
	// token actually reached the forked child.
	dump, readErr := os.ReadFile(envDump) //nolint:gosec // path built by this test
	require.NoError(t, readErr, "probe plugin was never executed")

	lines := strings.Split(strings.TrimRight(string(dump), "\n"), "\n")
	require.Len(t, lines, 4, "probe env dump: %q", string(dump))
	host, port, token, certFP := lines[0], lines[1], lines[2], lines[3]

	// Loopback, not merely non-empty: an empty Host in the auto-generated
	// server block would render as ":0" and bind every interface, exposing the
	// plugin listener off-box. Asserting the exact address is what makes that
	// regression visible.
	require.Equal(t, "127.0.0.1", host, "acceptor must bind loopback only")
	portNum, portErr := strconv.Atoi(port)
	require.NoError(t, portErr, "ZE_PLUGIN_HUB_PORT %q", port)
	require.Positive(t, portNum, "acceptor must bind a real port")
	require.NotEmpty(t, token, "ZE_PLUGIN_HUB_TOKEN")
	require.NotEmpty(t, certFP, "ZE_PLUGIN_CERT_FP")
}

// TestOrchestratorStartNoPluginsNeedsNoAcceptor pins the fail-closed guard's
// other side: a hub config that declares no plugin must start cleanly and must
// NOT open a listener nobody asked for.
//
// VALIDATES: acceptor creation is gated on external subsystems existing.
// PREVENTS: an always-on plugin listener on a config that declares no plugin.
func TestOrchestratorStartNoPluginsNeedsNoAcceptor(t *testing.T) {
	o := NewOrchestrator(&HubConfig{})
	t.Cleanup(o.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, o.Start(ctx))
	require.Nil(t, o.acceptor, "no subsystems declared, so no acceptor is warranted")
}

// TestReloadStartsAddedExternalPluginWithAcceptor covers the second entry point
// into the subsystem start path: SIGHUP reload adding an external plugin to a
// hub that started with none (so no acceptor existed yet).
//
// VALIDATES: Orchestrator.Reload creates the acceptor before forking a
// newly-declared external plugin.
// PREVENTS: the reload path reintroducing "no TLS acceptor configured" for
// plugins added after startup.
func TestReloadStartsAddedExternalPluginWithAcceptor(t *testing.T) {
	dir := t.TempDir()
	script, envDump := writeProbePlugin(t, dir)

	configPath := filepath.Join(dir, "hub.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("plugin {\n}\n"), 0o600))

	o := NewOrchestrator(&HubConfig{ConfigPath: configPath})
	t.Cleanup(o.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, o.Start(ctx))

	// Reload a config that adds an external plugin. The acceptor did not exist
	// at Start (no plugins), so reload must create one before forking.
	added := "plugin {\n\texternal probe {\n\t\trun \"" + script + "\";\n\t}\n}\n"
	require.NoError(t, os.WriteFile(configPath, []byte(added), 0o600))

	err := o.Reload(configPath)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "no TLS acceptor configured")
	// Positive anchor, not just "some other error": the failure must be the
	// connect-back the probe declined to perform, which only happens once the
	// child was spawned against a live acceptor.
	require.Contains(t, err.Error(), "TLS connect-back")

	_, readErr := os.ReadFile(envDump) //nolint:gosec // path built by this test
	require.NoError(t, readErr, "reload-added plugin was never executed")

	// The rollback emptied the registry, so the acceptor this reload created
	// must have been given back: a hub that forks nothing holds no listener.
	require.Nil(t, o.acceptor, "failed reload left a listener with no subsystem behind it")
}

// TestReloadRemovingLastPluginReleasesAcceptor pins the same invariant on the
// success path: a reload that removes the last plugin must not leave the
// connect-back listener bound.
//
// VALIDATES: releaseAcceptorIfIdleLocked runs after a successful removal.
// PREVENTS: a hub that forks nothing keeping a TLS listener open.
func TestReloadRemovingLastPluginReleasesAcceptor(t *testing.T) {
	dir := t.TempDir()
	script, _ := writeProbePlugin(t, dir)
	configPath := filepath.Join(dir, "hub.conf")

	// Start with one plugin already registered, and pretend the orchestrator is
	// running so Reload takes the started path. Registering without Start keeps
	// the probe unforked; the acceptor is created by the reload below.
	o := NewOrchestrator(&HubConfig{
		Plugins:    []PluginDef{{Name: "probe", Run: script}},
		Env:        map[string]string{},
		Blocks:     map[string]any{},
		ConfigPath: configPath,
	})
	t.Cleanup(o.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	o.ctx = ctx

	// First reload keeps the plugin but changes its run line, which forces a
	// replacement start and therefore an acceptor.
	changed := "plugin {\n\texternal probe {\n\t\trun \"" + script + " --v2\";\n\t}\n}\n"
	require.NoError(t, os.WriteFile(configPath, []byte(changed), 0o600))
	require.Error(t, o.Reload(configPath), "probe exits without handshaking")
	require.NotNil(t, o.acceptor, "a replacement start must have created an acceptor")

	// Now remove every plugin.
	require.NoError(t, os.WriteFile(configPath, []byte("plugin {\n}\n"), 0o600))
	require.NoError(t, o.Reload(configPath))
	require.Empty(t, o.subsystems.Names())
	require.Nil(t, o.acceptor, "removing the last plugin must release the listener")
}

// TestReloadAfterStopRefused pins the post-shutdown guard.
//
// VALIDATES: Reload after Stop returns ErrOrchestratorStopped and creates no
// acceptor.
// PREVENTS: a post-shutdown reload minting a TLS listener nothing will close
// (Stop cancels the context but leaves it non-nil, so the "was it started"
// checks alone do not catch this).
func TestReloadAfterStopRefused(t *testing.T) {
	dir := t.TempDir()
	script, _ := writeProbePlugin(t, dir)
	configPath := filepath.Join(dir, "hub.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("plugin {\n}\n"), 0o600))

	o := NewOrchestrator(&HubConfig{ConfigPath: configPath})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, o.Start(ctx))
	o.Stop()

	added := "plugin {\n\texternal probe {\n\t\trun \"" + script + "\";\n\t}\n}\n"
	require.NoError(t, os.WriteFile(configPath, []byte(added), 0o600))

	err := o.Reload(configPath)
	require.ErrorIs(t, err, ErrOrchestratorStopped)
	require.Nil(t, o.acceptor, "a stopped orchestrator must not open a listener")
}
