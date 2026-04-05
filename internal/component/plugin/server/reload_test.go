package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// mockReloadReactor implements the GetConfigTree/SetConfigTree subset of ReactorLifecycle.
// Embeds mockReactor (from handler_test.go) for all other interface methods.
type mockReloadReactor struct {
	mockReactor
	mu      sync.Mutex
	tree    map[string]any
	setTree map[string]any // Captures what was passed to SetConfigTree
}

func (m *mockReloadReactor) GetConfigTree() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tree
}

func (m *mockReloadReactor) SetConfigTree(tree map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setTree = tree
}

// mockPluginResponder runs a goroutine that reads RPCs from a PluginConn
// and responds with pre-configured verify/apply results.
type mockPluginResponder struct {
	pluginConn      *ipc.PluginConn
	verifyResp      *rpc.ConfigVerifyOutput
	applyResp       *rpc.ConfigApplyOutput
	beforeVerifyRsp func() // Called BEFORE verify response is sent (blocks coordinator)
	verifyCalls     int
	applyCalls      int
	mu              sync.Mutex
}

func (m *mockPluginResponder) start(ctx context.Context) {
	go func() {
		for {
			req, err := m.pluginConn.ReadRequest(ctx)
			if err != nil {
				return
			}

			m.mu.Lock()
			switch req.Method {
			case "ze-plugin-callback:config-verify":
				m.verifyCalls++
				resp := m.verifyResp
				if resp == nil {
					resp = &rpc.ConfigVerifyOutput{Status: rpc.StatusOK}
				}
				// Hook runs BEFORE sending the response — the coordinator's
				// SendConfigVerify blocks until it reads this response, so any
				// state change in the hook is visible before the pre-apply check.
				if m.beforeVerifyRsp != nil {
					fn := m.beforeVerifyRsp
					m.mu.Unlock()
					fn()
					m.mu.Lock()
				}
				_ = m.pluginConn.SendResult(ctx, req.ID, resp)

			case "ze-plugin-callback:config-apply":
				m.applyCalls++
				resp := m.applyResp
				if resp == nil {
					resp = &rpc.ConfigApplyOutput{Status: rpc.StatusOK}
				}
				_ = m.pluginConn.SendResult(ctx, req.ID, resp)

			default:
				_ = m.pluginConn.SendResult(ctx, req.ID, nil)
			}
			m.mu.Unlock()
		}
	}()
}

func (m *mockPluginResponder) getVerifyCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.verifyCalls
}

func (m *mockPluginResponder) getApplyCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.applyCalls
}

// newTestReloadServer creates a Server with a mock reactor and mock processes
// for testing the reload coordinator. Each pluginDef defines a mock plugin with
// its name, WantsConfigRoots, and verify/apply responses.
type pluginDef struct {
	name       string
	roots      []string
	verifyResp *rpc.ConfigVerifyOutput
	applyResp  *rpc.ConfigApplyOutput
	responder  *mockPluginResponder
}

func newTestReloadServer(t *testing.T, reactor *mockReloadReactor, plugins []pluginDef) *Server {
	t.Helper()

	s := &Server{
		reactor: reactor,
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(func() { s.cancel() })

	if len(plugins) == 0 {
		return s
	}

	pm := process.NewProcessManager(nil)

	for i := range plugins {
		pd := &plugins[i]

		engineEnd, pluginEnd := net.Pipe()
		t.Cleanup(func() {
			engineEnd.Close() //nolint:errcheck // test cleanup
			pluginEnd.Close() //nolint:errcheck // test cleanup
		})

		engineConn := ipc.NewPluginConn(engineEnd, engineEnd)
		pluginConn := ipc.NewPluginConn(pluginEnd, pluginEnd)

		proc := process.NewProcess(plugin.PluginConfig{Name: pd.name})
		proc.SetIndex(i)
		proc.SetRegistration(&plugin.PluginRegistration{
			WantsConfigRoots: pd.roots,
		})
		proc.SetCapabilities(&plugin.PluginCapabilities{})
		proc.SetConn(engineConn)
		proc.SetRunning(true)

		pm.AddProcess(pd.name, proc)

		// Start mock plugin responder
		resp := &mockPluginResponder{
			pluginConn: pluginConn,
			verifyResp: pd.verifyResp,
			applyResp:  pd.applyResp,
		}
		resp.start(s.ctx)
		pd.responder = resp
	}

	s.procManager.Store(pm)

	return s
}

// TestReloadConfigNoChange verifies that identical config produces no RPCs.
//
// VALIDATES: Empty diff → no verify/apply sent, no error returned.
// PREVENTS: Unnecessary plugin RPCs on no-op reload.
func TestReloadConfigNoChange(t *testing.T) {
	t.Parallel()

	tree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	reactor := &mockReloadReactor{tree: tree}

	plugins := []pluginDef{
		{name: "rib", roots: []string{"bgp"}},
	}
	s := newTestReloadServer(t, reactor, plugins)

	// Reload with same tree — should be a no-op.
	err := s.ReloadConfig(context.Background(), tree)
	require.NoError(t, err)

	// No RPCs should have been sent.
	require.Never(t, func() bool {
		return plugins[0].responder.getVerifyCalls() > 0 || plugins[0].responder.getApplyCalls() > 0
	}, 200*time.Millisecond, 10*time.Millisecond, "no RPCs should be sent for unchanged config")
	assert.Equal(t, 0, plugins[0].responder.getVerifyCalls())
	assert.Equal(t, 0, plugins[0].responder.getApplyCalls())

	// SetConfigTree should NOT have been called (no changes).
	reactor.mu.Lock()
	assert.Nil(t, reactor.setTree)
	reactor.mu.Unlock()
}

// TestReloadConfigVerifyFails verifies that verify error aborts apply.
//
// VALIDATES: Verify error → no apply sent, error returned, running config unchanged.
// PREVENTS: Partial config application when a plugin rejects.
func TestReloadConfigVerifyFails(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}

	plugins := []pluginDef{
		{
			name:       "rib",
			roots:      []string{"bgp"},
			verifyResp: &rpc.ConfigVerifyOutput{Status: plugin.StatusError, Error: "invalid router-id"},
		},
	}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config verify failed")
	assert.Contains(t, err.Error(), "invalid router-id")

	// Apply should NOT have been called.
	require.Eventually(t, func() bool { return plugins[0].responder.getVerifyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "verify should have been called once")
	assert.Equal(t, 0, plugins[0].responder.getApplyCalls())

	// Running config should NOT be updated.
	reactor.mu.Lock()
	assert.Nil(t, reactor.setTree)
	reactor.mu.Unlock()
}

// TestReloadConfigVerifyThenApply verifies the happy path: verify OK → apply.
//
// VALIDATES: All verify pass → apply sent to all, running config updated.
// PREVENTS: Apply not being sent after successful verify.
func TestReloadConfigVerifyThenApply(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}

	plugins := []pluginDef{
		{name: "rib", roots: []string{"bgp"}},
	}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.NoError(t, err)

	// Both verify and apply should have been called.
	require.Eventually(t, func() bool { return plugins[0].responder.getVerifyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "verify should have been called once")
	require.Eventually(t, func() bool { return plugins[0].responder.getApplyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "apply should have been called once")

	// Running config should be updated.
	reactor.mu.Lock()
	require.NotNil(t, reactor.setTree)
	bgpSection, ok := reactor.setTree["bgp"].(map[string]any)
	require.True(t, ok, "bgp section should be a map")
	assert.Equal(t, "5.6.7.8", bgpSection["router-id"])
	reactor.mu.Unlock()
}

// TestReloadConfigPerRootFiltering verifies that only plugins with matching roots get RPCs.
//
// VALIDATES: Plugin only gets verify/apply for roots it declared in WantsConfigRoots.
// PREVENTS: Sending config changes to plugins that don't care about those roots.
func TestReloadConfigPerRootFiltering(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{
		"bgp":         map[string]any{"router-id": "1.2.3.4"},
		"environment": map[string]any{"log": "info"},
	}
	newTree := map[string]any{
		"bgp":         map[string]any{"router-id": "1.2.3.4"}, // Unchanged
		"environment": map[string]any{"log": "debug"},         // Changed
	}
	reactor := &mockReloadReactor{tree: oldTree}

	plugins := []pluginDef{
		{name: "bgp-plugin", roots: []string{"bgp"}},         // Only cares about bgp
		{name: "env-plugin", roots: []string{"environment"}}, // Only cares about environment
	}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.NoError(t, err)

	// env-plugin SHOULD be called (environment changed).
	require.Eventually(t, func() bool { return plugins[1].responder.getVerifyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "env-plugin should get verify")
	require.Eventually(t, func() bool { return plugins[1].responder.getApplyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "env-plugin should get apply")

	// bgp-plugin should NOT be called (bgp unchanged).
	assert.Equal(t, 0, plugins[0].responder.getVerifyCalls(), "bgp-plugin should not get verify")
	assert.Equal(t, 0, plugins[0].responder.getApplyCalls(), "bgp-plugin should not get apply")
}

// TestReloadConfigMultiplePlugins verifies that one plugin rejecting aborts all.
//
// VALIDATES: Two plugins, one rejects verify → neither gets apply, error returned.
// PREVENTS: Apply reaching any plugin when one rejects during verify.
func TestReloadConfigMultiplePlugins(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}

	plugins := []pluginDef{
		{name: "rib", roots: []string{"bgp"}}, // Will accept
		{
			name:       "gr",
			roots:      []string{"bgp"},
			verifyResp: &rpc.ConfigVerifyOutput{Status: plugin.StatusError, Error: "GR in progress"},
		},
	}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config verify failed")
	assert.Contains(t, err.Error(), "GR in progress")

	// Neither should get apply (one rejected).
	require.Never(t, func() bool {
		return plugins[0].responder.getApplyCalls() > 0 || plugins[1].responder.getApplyCalls() > 0
	}, 200*time.Millisecond, 10*time.Millisecond, "no apply RPCs should be sent when verify fails")
	assert.Equal(t, 0, plugins[0].responder.getApplyCalls(), "rib should not get apply")
	assert.Equal(t, 0, plugins[1].responder.getApplyCalls(), "gr should not get apply")

	// Running config should NOT be updated.
	reactor.mu.Lock()
	assert.Nil(t, reactor.setTree)
	reactor.mu.Unlock()
}

// TestReloadConfigConcurrentRejected verifies that concurrent reloads are rejected.
//
// VALIDATES: Second reload while first in progress → error returned.
// PREVENTS: Race conditions from concurrent config modifications.
func TestReloadConfigConcurrentRejected(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}

	// No plugins — just test the mutex.
	s := newTestReloadServer(t, reactor, nil)

	// Lock the reload mutex manually to simulate an in-progress reload.
	s.reloadMu.Lock()

	// Attempt second reload — should fail immediately.
	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")

	// Unlock so cleanup works.
	s.reloadMu.Unlock()
}

// TestReloadFromDiskParseError verifies that config parse errors are propagated.
//
// VALIDATES: Config parse failure → error returned, running config unchanged.
// PREVENTS: Corrupt config being applied after parse failure.
func TestReloadFromDiskParseError(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	reactor := &mockReloadReactor{tree: oldTree}
	s := newTestReloadServer(t, reactor, nil)

	// Set a failing config loader.
	s.SetConfigLoader(func() (map[string]any, error) {
		return nil, fmt.Errorf("syntax error at line 42")
	})

	err := s.ReloadFromDisk(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config parse error")
	assert.Contains(t, err.Error(), "syntax error at line 42")

	// Running config should NOT be updated.
	reactor.mu.Lock()
	assert.Nil(t, reactor.setTree)
	reactor.mu.Unlock()
}

// TestReloadFromDiskNoLoader verifies error when no loader is configured.
//
// VALIDATES: Missing loader → clear error.
// PREVENTS: Nil pointer dereference on missing loader.
func TestReloadFromDiskNoLoader(t *testing.T) {
	t.Parallel()

	reactor := &mockReloadReactor{tree: map[string]any{}}
	s := newTestReloadServer(t, reactor, nil)

	err := s.ReloadFromDisk(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no config loader")
}

// TestHasConfigLoader verifies the HasConfigLoader predicate.
//
// VALIDATES: Returns false before SetConfigLoader, true after.
// PREVENTS: SIGHUP handler taking coordinator path when no loader is configured.
func TestHasConfigLoader(t *testing.T) {
	t.Parallel()

	reactor := &mockReloadReactor{tree: map[string]any{}}
	s := newTestReloadServer(t, reactor, nil)

	assert.False(t, s.HasConfigLoader(), "should be false before SetConfigLoader")

	s.SetConfigLoader(func() (map[string]any, error) {
		return map[string]any{}, nil
	})

	assert.True(t, s.HasConfigLoader(), "should be true after SetConfigLoader")
}

// TestDiffMapsLocal verifies the local diffMaps implementation.
//
// VALIDATES: Diff computation matches expected behavior for add/remove/change.
// PREVENTS: Diff logic bugs causing incorrect reload decisions.
func TestDiffMapsLocal(t *testing.T) {
	t.Parallel()

	old := map[string]any{
		"bgp": map[string]any{
			"router-id": "1.2.3.4",
			"peer": map[string]any{
				"p1": map[string]any{"address": "10.0.0.1"},
			},
		},
		"environment": map[string]any{"log": "info"},
	}

	newMap := map[string]any{
		"bgp": map[string]any{
			"router-id": "5.6.7.8", // Changed
			"peer": map[string]any{
				"p1": map[string]any{"address": "10.0.0.1"}, // Same
				"p2": map[string]any{"address": "10.0.0.2"}, // Added
			},
		},
		// environment removed
	}

	diff := diffMaps(old, newMap)

	assert.Contains(t, diff.changed, "bgp/router-id")
	assert.Contains(t, diff.added, "bgp/peer/p2")
	assert.Contains(t, diff.removed, "environment")
}

// TestRootHasChanges verifies root matching for diff filtering.
//
// VALIDATES: rootHasChanges correctly matches config paths to roots.
// PREVENTS: Plugins getting RPCs for roots they didn't register.
func TestRootHasChanges(t *testing.T) {
	t.Parallel()

	diff := &configDiff{
		added:   map[string]any{"bgp/peer/p2": "added"},
		removed: map[string]any{"environment": "removed"},
		changed: map[string]diffPair{"bgp/router-id": {Old: "old", New: "new"}},
	}

	assert.True(t, rootHasChanges(diff, "bgp"))
	assert.True(t, rootHasChanges(diff, "environment"))
	assert.False(t, rootHasChanges(diff, "plugin"))
	assert.True(t, rootHasChanges(diff, "*"))
}

// TestReloadConfigRootRemoved verifies that removing an entire config root
// still notifies the plugin (verify + apply).
//
// VALIDATES: Plugin gets verify with empty data when its root is removed from new config.
// PREVENTS: Silent skip of plugins when their config root disappears entirely.
func TestReloadConfigRootRemoved(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{
		"bgp":         map[string]any{"router-id": "1.2.3.4"},
		"environment": map[string]any{"log": "info"},
	}
	// New config removes "bgp" entirely.
	newTree := map[string]any{
		"environment": map[string]any{"log": "info"},
	}
	reactor := &mockReloadReactor{tree: oldTree}

	plugins := []pluginDef{
		{name: "rib", roots: []string{"bgp"}},
	}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.NoError(t, err)

	// Plugin MUST get both verify and apply even though root was removed.
	require.Eventually(t, func() bool { return plugins[0].responder.getVerifyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "rib should get verify for removed root")
	require.Eventually(t, func() bool { return plugins[0].responder.getApplyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "rib should get apply for removed root")

	// Running config should be updated.
	reactor.mu.Lock()
	require.NotNil(t, reactor.setTree)
	_, hasBGP := reactor.setTree["bgp"]
	assert.False(t, hasBGP, "bgp should not be in new tree")
	reactor.mu.Unlock()
}

// TestReloadConfigWildcardRoot verifies that plugins with WantsConfigRoots: ["*"]
// receive apply diff sections for all roots.
//
// VALIDATES: Wildcard root receives verify and apply for any changed root.
// PREVENTS: Apply-phase filter failing to match wildcard against concrete root names.
func TestReloadConfigWildcardRoot(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{
		"bgp":         map[string]any{"router-id": "1.2.3.4"},
		"environment": map[string]any{"log": "info"},
	}
	newTree := map[string]any{
		"bgp":         map[string]any{"router-id": "5.6.7.8"},
		"environment": map[string]any{"log": "debug"},
	}
	reactor := &mockReloadReactor{tree: oldTree}

	plugins := []pluginDef{
		{name: "monitor", roots: []string{"*"}},
	}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.NoError(t, err)

	// Wildcard plugin MUST get both verify and apply.
	require.Eventually(t, func() bool { return plugins[0].responder.getVerifyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "wildcard plugin should get verify")
	require.Eventually(t, func() bool { return plugins[0].responder.getApplyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "wildcard plugin should get apply")

	// Running config should be updated.
	reactor.mu.Lock()
	require.NotNil(t, reactor.setTree)
	reactor.mu.Unlock()
}

// TestDiffPairJSONKeys verifies that diffPair marshals with kebab-case keys.
//
// VALIDATES: JSON output uses "old"/"new" not "Old"/"New".
// PREVENTS: PascalCase JSON keys violating ze JSON format standard.
func TestDiffPairJSONKeys(t *testing.T) {
	t.Parallel()

	dp := diffPair{Old: "before", New: "after"}
	j, err := json.Marshal(dp)
	require.NoError(t, err)

	s := string(j)
	assert.Contains(t, s, `"old"`)
	assert.Contains(t, s, `"new"`)
	assert.NotContains(t, s, `"Old"`)
	assert.NotContains(t, s, `"New"`)
}

// TestBuildDiffSections verifies per-root grouping of diff entries.
//
// VALIDATES: Flat config keys are grouped by top-level root into ConfigDiffSections.
// PREVENTS: Diff data being sent to wrong root section.
func TestBuildDiffSections(t *testing.T) {
	t.Parallel()

	diff := &configDiff{
		added:   map[string]any{"bgp/peer/p2": "new-peer"},
		removed: map[string]any{"environment/log": "info"},
		changed: map[string]diffPair{"bgp/router-id": {Old: "1.2.3.4", New: "5.6.7.8"}},
	}

	sections := buildDiffSections(diff)

	// Should have 2 sections: bgp and environment.
	require.Len(t, sections, 2)

	sectionMap := make(map[string]rpc.ConfigDiffSection)
	for _, s := range sections {
		sectionMap[s.Root] = s
	}

	bgpSection, ok := sectionMap["bgp"]
	require.True(t, ok, "should have bgp section")
	assert.NotEmpty(t, bgpSection.Added)
	assert.NotEmpty(t, bgpSection.Changed)
	assert.Empty(t, bgpSection.Removed)

	// Verify JSON content.
	var addedData map[string]any
	require.NoError(t, json.Unmarshal([]byte(bgpSection.Added), &addedData))
	assert.Equal(t, "new-peer", addedData["bgp/peer/p2"])

	envSection, ok := sectionMap["environment"]
	require.True(t, ok, "should have environment section")
	assert.NotEmpty(t, envSection.Removed)
	assert.Empty(t, envSection.Added)
	assert.Empty(t, envSection.Changed)
}

// TestReloadVerifyCrashedPlugin verifies that a crashed plugin (conn==nil)
// during verify phase causes a verify error.
//
// VALIDATES: conn==nil during verify → verify error returned with plugin name.
// PREVENTS: Silent skip of crashed plugins during verify.
func TestReloadVerifyCrashedPlugin(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}

	plugins := []pluginDef{
		{name: "crashed-plugin", roots: []string{"bgp"}},
	}
	s := newTestReloadServer(t, reactor, plugins)

	// Simulate crash: close conn so Conn() returns nil.
	proc := s.procManager.Load().GetProcess("crashed-plugin")
	proc.CloseConn()

	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err, "should fail when plugin conn is nil during verify")
	assert.Contains(t, err.Error(), "crashed-plugin")
	assert.Contains(t, err.Error(), "verify")

	// Running config should NOT be updated.
	reactor.mu.Lock()
	assert.Nil(t, reactor.setTree)
	reactor.mu.Unlock()
}

// TestReloadApplyErrorReturned verifies that plugin apply rejection
// is returned as an error to the caller (not swallowed).
//
// VALIDATES: Plugin apply rejection → error returned to caller.
// PREVENTS: Apply errors being logged but silently returning nil.
func TestReloadApplyErrorReturned(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}

	plugins := []pluginDef{
		{
			name:      "rib",
			roots:     []string{"bgp"},
			applyResp: &rpc.ConfigApplyOutput{Status: plugin.StatusError, Error: "apply rejected"},
		},
	}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err, "should return error when plugin rejects apply")
	assert.Contains(t, err.Error(), "apply rejected")
	assert.Contains(t, err.Error(), "rib")

	// SetConfigTree should still be called (apply errors are non-fatal for config update).
	require.Eventually(t, func() bool {
		reactor.mu.Lock()
		defer reactor.mu.Unlock()
		return reactor.setTree != nil
	}, 2*time.Second, 10*time.Millisecond, "config should still be updated after apply error")
}

// TestReloadVerifyCrashedPluginMultiple verifies that when one of several plugins
// has conn==nil, the verify phase catches it and aborts reload.
//
// VALIDATES: One crashed plugin among many → verify error with crashed plugin name.
// PREVENTS: Partial verify when one plugin in a group has died.
func TestReloadVerifyCrashedPluginMultiple(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}

	// Two plugins: first responds normally, second has conn niled before reload.
	// Verify phase will see conn==nil on the second and fail.
	plugins := []pluginDef{
		{name: "healthy", roots: []string{"bgp"}},
		{name: "crashed", roots: []string{"bgp"}},
	}
	s := newTestReloadServer(t, reactor, plugins)

	// Nil conn for "crashed" before reload — verify phase catches this.
	proc := s.procManager.Load().GetProcess("crashed")
	proc.CloseConn()

	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err, "should fail when plugin conn is nil during verify")
	assert.Contains(t, err.Error(), "crashed")
	assert.Contains(t, err.Error(), "verify")
}

// TestReloadProcessDiedBetweenVerifyAndApply verifies that if a plugin's conn
// becomes nil after verify succeeds but before apply starts, the reload aborts.
//
// VALIDATES: Process death between verify and apply → reload aborted.
// PREVENTS: Sending apply to a subset of plugins when one has died.
func TestReloadProcessDiedBetweenVerifyAndApply(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}

	// Plugin responds to verify OK, then dies before apply.
	plugins := []pluginDef{
		{name: "dies-after-verify", roots: []string{"bgp"}},
	}
	s := newTestReloadServer(t, reactor, plugins)

	proc := s.procManager.Load().GetProcess("dies-after-verify")

	// Use beforeVerifyRsp to deterministically nil engineConn BEFORE the verify
	// response is sent. Only nil the pointer — do NOT close the underlying connection,
	// because the coordinator's in-flight SendConfigVerify still needs to read the
	// response through the old reference. The pre-apply alive check calls Conn()
	// which returns nil, triggering the abort.
	plugins[0].responder.mu.Lock()
	plugins[0].responder.beforeVerifyRsp = func() {
		proc.ClearConn()
	}
	plugins[0].responder.mu.Unlock()

	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err, "should fail when plugin dies between verify and apply")
	assert.Contains(t, err.Error(), "dies-after-verify")

	// Running config should NOT be updated when pre-apply check fails.
	reactor.mu.Lock()
	assert.Nil(t, reactor.setTree, "config should NOT be updated when process died between phases")
	reactor.mu.Unlock()
}
