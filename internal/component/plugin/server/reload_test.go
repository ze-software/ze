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

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// mockReloadReactor implements the GetConfigTree/SetConfigTree subset of ReactorLifecycle.
// Embeds mockReactor (from handler_test.go) for all other interface methods.
type mockReloadReactor struct {
	mockReactor
	mu         sync.Mutex
	tree       map[string]any
	setTree    map[string]any
	verifyTree map[string]any
	applyTree  map[string]any
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

func (m *mockReloadReactor) VerifyConfig(tree map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifyTree = tree
	return nil
}

func (m *mockReloadReactor) ApplyConfigDiff(tree map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.applyTree = tree
	return nil
}

// mockPluginResponder runs a goroutine that reads RPCs from a PluginConn
// and responds with pre-configured verify/apply/rollback results. The
// TxCoordinator path drives the same RPCs through its engine-side RPC
// bridge, so these mocks exercise both the legacy and the coordinator
// path uniformly.
//
// The responder captures the Sections payload sent to config-verify and
// the DiffSections payload sent to config-apply so tests can assert the
// plugin receives the correct candidate data (not diff-shaped JSON as the
// earlier bridge implementation accidentally sent).
type mockPluginResponder struct {
	pluginConn       *ipc.PluginConn
	pluginName       string // label recorded by orderRecorder on apply
	verifyResp       *rpc.ConfigVerifyOutput
	applyResp        *rpc.ConfigApplyOutput
	opDecomposeResp  *rpc.ConfigOperationDecomposeOutput
	opVerifyResp     *rpc.ConfigOperationVerifyOutput
	opApplyResp      *rpc.ConfigOperationApplyOutput
	opRollbackResp   *rpc.ConfigOperationRollbackOutput
	opCommitResp     *rpc.ConfigOperationCommitOutput
	rollbackErr      error  // non-nil → SendError on rollback (CodeBroken path)
	beforeVerifyRsp  func() // Called BEFORE verify response is sent (blocks coordinator)
	verifyCalls      int
	applyCalls       int
	opDecomposeCalls int
	opVerifyCalls    int
	opApplyCalls     int
	opRollbackCalls  int
	opCommitCalls    int
	rollbackCalls    int
	verifySections   []rpc.ConfigSection     // last verify payload
	applySections    []rpc.ConfigDiffSection // last apply payload
	opDecomposeInput rpc.ConfigOperationDecomposeInput
	opVerifyInput    rpc.ConfigOperationVerifyInput
	opApplyInput     rpc.ConfigOperationApplyInput
	opRollbackInput  rpc.ConfigOperationRollbackInput
	opCommitInput    rpc.ConfigOperationCommitInput
	order            *orderRecorder // optional ordering trace
	mu               sync.Mutex
}

// orderRecorder tracks the sequence in which plugins receive config-apply
// RPCs. Multiple responders share one recorder via mockPluginResponder.order
// so tests can assert cross-plugin apply ordering.
type orderRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *orderRecorder) record(label string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, label)
}

func (r *orderRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
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
				var in rpc.ConfigVerifyInput
				if len(req.Params) > 0 {
					if uerr := json.Unmarshal(req.Params, &in); uerr == nil {
						m.verifySections = append([]rpc.ConfigSection(nil), in.Sections...)
					}
				}
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
				var in rpc.ConfigApplyInput
				if len(req.Params) > 0 {
					if uerr := json.Unmarshal(req.Params, &in); uerr == nil {
						m.applySections = append([]rpc.ConfigDiffSection(nil), in.Sections...)
					}
				}
				if m.order != nil {
					m.order.record(m.pluginName)
				}
				resp := m.applyResp
				if resp == nil {
					resp = &rpc.ConfigApplyOutput{Status: rpc.StatusOK}
				}
				_ = m.pluginConn.SendResult(ctx, req.ID, resp)

			case "ze-plugin-callback:config-operation-decompose":
				m.opDecomposeCalls++
				if len(req.Params) > 0 {
					_ = json.Unmarshal(req.Params, &m.opDecomposeInput)
				}
				resp := m.opDecomposeResp
				if resp == nil {
					resp = &rpc.ConfigOperationDecomposeOutput{Status: rpc.StatusOK}
				}
				_ = m.pluginConn.SendResult(ctx, req.ID, resp)

			case "ze-plugin-callback:config-operation-verify":
				m.opVerifyCalls++
				if len(req.Params) > 0 {
					_ = json.Unmarshal(req.Params, &m.opVerifyInput)
				}
				resp := m.opVerifyResp
				if resp == nil {
					resp = &rpc.ConfigOperationVerifyOutput{Status: rpc.StatusOK}
				}
				_ = m.pluginConn.SendResult(ctx, req.ID, resp)

			case "ze-plugin-callback:config-operation-apply":
				m.opApplyCalls++
				if len(req.Params) > 0 {
					_ = json.Unmarshal(req.Params, &m.opApplyInput)
				}
				resp := m.opApplyResp
				if resp == nil {
					resp = &rpc.ConfigOperationApplyOutput{Status: rpc.StatusOK}
				}
				_ = m.pluginConn.SendResult(ctx, req.ID, resp)

			case "ze-plugin-callback:config-operation-rollback":
				m.opRollbackCalls++
				if len(req.Params) > 0 {
					_ = json.Unmarshal(req.Params, &m.opRollbackInput)
				}
				resp := m.opRollbackResp
				if resp == nil {
					resp = &rpc.ConfigOperationRollbackOutput{Status: rpc.StatusOK}
				}
				_ = m.pluginConn.SendResult(ctx, req.ID, resp)

			case "ze-plugin-callback:config-operation-commit":
				m.opCommitCalls++
				if len(req.Params) > 0 {
					_ = json.Unmarshal(req.Params, &m.opCommitInput)
				}
				resp := m.opCommitResp
				if resp == nil {
					resp = &rpc.ConfigOperationCommitOutput{Status: rpc.StatusOK}
				}
				_ = m.pluginConn.SendResult(ctx, req.ID, resp)

			case "ze-plugin-callback:config-rollback":
				m.rollbackCalls++
				if m.rollbackErr != nil {
					_ = m.pluginConn.SendError(ctx, req.ID, m.rollbackErr.Error())
				} else {
					_ = m.pluginConn.SendResult(ctx, req.ID, nil)
				}

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

func (m *mockPluginResponder) getOperationDecomposeCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opDecomposeCalls
}

func (m *mockPluginResponder) getOperationVerifyCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opVerifyCalls
}

func (m *mockPluginResponder) getOperationApplyCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opApplyCalls
}

func (m *mockPluginResponder) getOperationRollbackCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opRollbackCalls
}

func (m *mockPluginResponder) getOperationCommitCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opCommitCalls
}

func (m *mockPluginResponder) getRollbackCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rollbackCalls
}

// newTestReloadServer creates a Server with a mock reactor and mock processes
// for testing the reload coordinator. Each pluginDef defines a mock plugin with
// its name, WantsConfigRoots, and verify/apply responses.
type pluginDef struct {
	name       string
	roots      []string
	configOps  []rpc.ConfigOperationDecl
	verifyResp *rpc.ConfigVerifyOutput
	applyResp  *rpc.ConfigApplyOutput
	order      *orderRecorder // optional: shared across plugins to capture cross-plugin apply order
	responder  *mockPluginResponder
}

func newTestReloadServer(t *testing.T, reactor *mockReloadReactor, plugins []pluginDef) *Server {
	t.Helper()

	s := &Server{
		reactor:           reactor,
		engineSubscribers: newEngineEventSubscribers(),
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
			ConfigOperations: pd.configOps,
		})
		proc.SetCapabilities(&plugin.PluginCapabilities{})
		proc.SetConn(engineConn)
		proc.SetRunning(true)

		pm.AddProcess(pd.name, proc)

		// Start mock plugin responder
		resp := &mockPluginResponder{
			pluginConn: pluginConn,
			pluginName: pd.name,
			verifyResp: pd.verifyResp,
			applyResp:  pd.applyResp,
			order:      pd.order,
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

// TestReloadConfigAutoLoadDependencyFailureFailsClosed verifies a reload that
// adds a config section owned by an auto-loaded plugin aborts when dependency
// resolution fails.
//
// VALIDATES: Reload-time config-path plugin auto-load is fail-closed.
// PREVENTS: New config becoming running config without the plugin that owns it.
func TestReloadConfigAutoLoadDependencyFailureFailsClosed(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	require.NoError(t, registry.Register(registry.Registration{
		Name:         "auto-load-owner",
		Description:  "test auto-load owner",
		ConfigRoots:  []string{"autopath"},
		Dependencies: []string{"missing-dep"},
		RunEngine:    func(net.Conn) int { return 0 },
		CLIHandler:   func([]string) int { return 0 },
	}))

	reactor := &mockReloadReactor{tree: map[string]any{}}
	s := newTestReloadServer(t, reactor, nil)
	newTree := map[string]any{"autopath": map[string]any{"enabled": "true"}}

	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config-path auto-load")
	assert.Contains(t, err.Error(), "missing-dep")
	assert.Nil(t, reactor.setTree, "failed auto-load must not update running config")
}

// TestReloadFailureStopsAutoLoadedPluginByName verifies failed reload cleanup
// uses plugin names returned by auto-load, not config roots.
//
// VALIDATES: AC-4, failed reload stops fib-kernel even though its config root is fib/kernel.
// PREVENTS: Rejected config reloads leaving auto-loaded plugin processes running.
func TestReloadFailureStopsAutoLoadedPluginByName(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "fib-kernel"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        pluginName,
		Description: "test fib kernel owner",
		ConfigRoots: []string{"fib/kernel"},
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(pluginName, conn)
			err := p.Run(context.Background(), sdk.Registration{WantsConfig: []string{"fib/kernel"}})
			if err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	reactor := &mockReloadReactor{tree: map[string]any{"other": map[string]any{"enabled": false}}}
	serverCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s, err := NewServer(&ServerConfig{}, reactor)
	require.NoError(t, err)
	s.ctx, s.cancel = context.WithCancel(serverCtx)
	t.Cleanup(s.cancel)

	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.StartWithContext(s.ctx))
	engineEnd, pluginEnd := net.Pipe()
	t.Cleanup(func() {
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	rejector := process.NewProcess(plugin.PluginConfig{Name: "reload-rejector"})
	rejector.SetRegistration(&plugin.PluginRegistration{WantsConfigRoots: []string{"other"}})
	rejector.SetConn(ipc.NewPluginConn(engineEnd, engineEnd))
	rejector.SetRunning(true)
	pm.AddProcess("reload-rejector", rejector)
	(&mockPluginResponder{
		pluginConn: ipc.NewPluginConn(pluginEnd, pluginEnd),
		pluginName: "reload-rejector",
		verifyResp: &rpc.ConfigVerifyOutput{Status: plugin.StatusError, Error: "reject other"},
	}).start(s.ctx)
	s.procManager.Store(pm)

	spawner := &lifecycleTestSpawner{ctx: s.ctx, pm: pm}
	s.SetProcessSpawner(spawner)

	newTree := map[string]any{
		"fib":   map[string]any{"kernel": map[string]any{"enabled": true}},
		"other": map[string]any{"enabled": true},
	}
	err = s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reject other")
	require.NotNil(t, spawner.pm)
	assert.Nil(t, spawner.pm.GetProcess(pluginName), "failed reload must stop auto-loaded plugin by plugin name")
	assert.Nil(t, reactor.setTree, "failed reload must not update running config")
}

// TestReloadFailureKeepsPreExistingDependency verifies failed reload cleanup
// restores only processes started by that reload.
//
// VALIDATES: AC-4, dependencies that predate a rejected reload remain running.
// PREVENTS: failed auto-load rollback stopping an unrelated pre-existing dependency.
func TestReloadFailureKeepsPreExistingDependency(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const depName = "reload-existing-dependency"
	const pluginName = "reload-dependent-owner"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        depName,
		Description: "pre-existing dependency",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(depName, conn)
			if err := p.Run(context.Background(), sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:         pluginName,
		Description:  "test config owner with dependency",
		ConfigRoots:  []string{"fib/kernel"},
		Dependencies: []string{depName},
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(pluginName, conn)
			err := p.Run(context.Background(), sdk.Registration{WantsConfig: []string{"fib/kernel"}})
			if err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	reactor := &mockReloadReactor{tree: map[string]any{"other": map[string]any{"enabled": false}}}
	serverCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s, err := NewServer(&ServerConfig{}, reactor)
	require.NoError(t, err)
	s.ctx, s.cancel = context.WithCancel(serverCtx)
	t.Cleanup(s.cancel)

	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.StartWithContext(s.ctx))
	depProc := process.NewProcess(plugin.PluginConfig{Name: depName})
	depProc.SetRunning(true)
	pm.AddProcess(depName, depProc)

	engineEnd, pluginEnd := net.Pipe()
	t.Cleanup(func() {
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	rejector := process.NewProcess(plugin.PluginConfig{Name: "reload-rejector"})
	rejector.SetRegistration(&plugin.PluginRegistration{WantsConfigRoots: []string{"other"}})
	rejector.SetConn(ipc.NewPluginConn(engineEnd, engineEnd))
	rejector.SetRunning(true)
	pm.AddProcess("reload-rejector", rejector)
	(&mockPluginResponder{
		pluginConn: ipc.NewPluginConn(pluginEnd, pluginEnd),
		pluginName: "reload-rejector",
		verifyResp: &rpc.ConfigVerifyOutput{Status: plugin.StatusError, Error: "reject other"},
	}).start(s.ctx)
	s.procManager.Store(pm)

	spawner := &lifecycleTestSpawner{ctx: s.ctx, pm: pm}
	s.SetProcessSpawner(spawner)

	newTree := map[string]any{
		"fib":   map[string]any{"kernel": map[string]any{"enabled": true}},
		"other": map[string]any{"enabled": true},
	}
	err = s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err)
	assert.Nil(t, spawner.pm.GetProcess(pluginName), "failed reload must stop newly auto-loaded plugin")
	assert.NotNil(t, spawner.pm.GetProcess(depName), "failed reload must keep pre-existing dependency")
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
	assert.Equal(t, newTree, reactor.verifyTree)
	assert.Equal(t, newTree, reactor.applyTree)
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
	assert.Equal(t, 0, plugins[0].responder.getApplyCalls(), "bgp rib should not get apply")
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

	// Acquire the transaction lock to simulate an in-progress reload.
	s.txLock.tryAcquire()

	// Attempt second reload — should fail immediately.
	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")

	// Release so cleanup works.
	s.txLock.release()
}

// TestTxLockSIGHUPQueuing verifies SIGHUP is queued when the lock is held and drained after release.
//
// VALIDATES: AC-8 - SIGHUP queued during active transaction, replayed after.
// PREVENTS: SIGHUP lost when received during config reload.
func TestTxLockSIGHUPQueuing(t *testing.T) {
	t.Parallel()

	s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{}}, nil)

	// Acquire lock.
	if !s.txLock.tryAcquire() {
		t.Fatal("first acquire failed")
	}

	// Second acquire should fail.
	if s.txLock.tryAcquire() {
		t.Fatal("second acquire should fail while locked")
	}

	// Queue a SIGHUP and verify it's queued.
	s.QueueSIGHUP()
	if !s.txLock.sighup {
		t.Fatal("SIGHUP not queued")
	}

	// Release lock.
	s.txLock.release()

	// Drain should return true and clear the flag.
	if !s.DrainSIGHUP() {
		t.Fatal("DrainSIGHUP returned false, expected true")
	}

	// Second drain should return false.
	if s.DrainSIGHUP() {
		t.Fatal("DrainSIGHUP returned true after already drained")
	}

	// Lock should be available again.
	if !s.txLock.tryAcquire() {
		t.Fatal("acquire after release failed")
	}
	s.txLock.release()
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

// TestDiffMapsLocal removed — the private server-local map-diff helper it
// exercised is deleted; the surviving canonical config.DiffMaps is covered by the 12
// tests in internal/component/config/diff_test.go (spec-unify-config-diff, R-2).

// TestRootHasChanges verifies root matching for diff filtering.
//
// VALIDATES: rootHasChanges correctly matches config paths to roots.
// PREVENTS: Plugins getting RPCs for roots they didn't register.
func TestRootHasChanges(t *testing.T) {
	t.Parallel()

	diff := &config.ConfigDiff{
		Added:   map[string]any{"bgp/peer/p2": "added"},
		Removed: map[string]any{"environment": "removed"},
		Changed: map[string]config.DiffPair{"bgp/router-id": {Old: "old", New: "new"}},
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
	require.Eventually(t, func() bool { return plugins[0].responder.getVerifyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "bgp rib should get verify for removed root")
	require.Eventually(t, func() bool { return plugins[0].responder.getApplyCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "bgp rib should get apply for removed root")

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

// TestDiffPairJSONKeys verifies that config.DiffPair marshals with kebab-case keys.
//
// VALIDATES: JSON output uses "old"/"new" not "Old"/"New".
// PREVENTS: PascalCase JSON keys violating ze JSON format standard.
func TestDiffPairJSONKeys(t *testing.T) {
	t.Parallel()

	dp := config.DiffPair{Old: "before", New: "after"}
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

	diff := &config.ConfigDiff{
		Added:   map[string]any{"bgp/peer/p2": "new-peer"},
		Removed: map[string]any{"environment/log": "info"},
		Changed: map[string]config.DiffPair{"bgp/router-id": {Old: "1.2.3.4", New: "5.6.7.8"}},
	}

	// No declared roots: grouping falls back to top-level roots, the behavior
	// this test has always pinned.
	sections := buildDiffSections(diff, nil)

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

// TestBuildDiffSectionsNestedDeclaredRoot pins that a diff key is filed under the
// NESTED root its plugin declared, not that root's top-level ancestor.
//
// VALIDATES: with a declared root "ddos/local", the key
// "ddos/local/forward-mitigation" lands in a section whose Root is exactly
// "ddos/local", so the orchestrator's exact-match filterDiffs
// (internal/component/config/transaction/orchestrator.go:487-503) finds it;
// sibling keys under other ddos subtrees stay out of that section.
// PREVENTS: the SIGHUP dead-end behind test/plugin/ddos-transit-forward-drop.ci
// Phase B. Filing every key under "ddos" while the participant declared
// "ddos/local" made the lookup miss, so runVerify (:340-344) and runApply
// (:397-401) both `continue` past the plugin and the reload reported success
// having delivered nothing. All eight nested-root plugins were affected:
// ddos/{detect,local,observe,flowspec,flowtriq}, anomaly/{detect,shape},
// traffic/usage.
func TestBuildDiffSectionsNestedDeclaredRoot(t *testing.T) {
	t.Parallel()

	diff := &config.ConfigDiff{
		Changed: map[string]config.DiffPair{
			"ddos/local/forward-mitigation": {Old: "true", New: "false"},
			"ddos/detect/absolute-floor":    {Old: "1000", New: "2000"},
		},
		Added: map[string]any{"ddos/observe/incident-ring-size": "100"},
	}
	declared := []string{"ddos/local", "ddos/detect"}

	sectionMap := make(map[string]rpc.ConfigDiffSection)
	for _, s := range buildDiffSections(diff, declared) {
		sectionMap[s.Root] = s
	}

	local, ok := sectionMap["ddos/local"]
	require.True(t, ok, "a plugin declaring ddos/local must get a section keyed ddos/local, or the orchestrator never drives it")
	var changed map[string]any
	require.NoError(t, json.Unmarshal([]byte(local.Changed), &changed))
	assert.Contains(t, changed, "ddos/local/forward-mitigation")
	assert.NotContains(t, changed, "ddos/detect/absolute-floor", "sibling subtree leaked into ddos/local")

	detect, ok := sectionMap["ddos/detect"]
	require.True(t, ok, "ddos/detect was declared too and must get its own section")
	require.NoError(t, json.Unmarshal([]byte(detect.Changed), &changed))
	assert.Contains(t, changed, "ddos/detect/absolute-floor")

	// ddos/observe was NOT declared by any participant in this reload, so its key
	// keeps the top-level fallback rather than vanishing.
	observe, ok := sectionMap["ddos"]
	require.True(t, ok, "an undeclared subtree must still be filed under its top-level root")
	var added map[string]any
	require.NoError(t, json.Unmarshal([]byte(observe.Added), &added))
	assert.Contains(t, added, "ddos/observe/incident-ring-size")
}

// TestGroupRootForSegmentBoundary pins that declared-root matching respects path
// segment boundaries and prefers the longest match.
//
// VALIDATES: "ddos/localhost/x" is not claimed by declared root "ddos/local";
// an exact key match wins; the longest declared prefix wins over a shorter one.
// PREVENTS: a plain strings.HasPrefix filing one plugin's keys into another
// plugin's section (ddos/local vs a hypothetical ddos/localhost), and a parent
// root swallowing a declared child's keys.
func TestGroupRootForSegmentBoundary(t *testing.T) {
	t.Parallel()

	declared := []string{"ddos/local", "ddos", "*"}
	cases := []struct {
		key  string
		want string
	}{
		{"ddos/local/forward-mitigation", "ddos/local"},
		{"ddos/local", "ddos/local"},
		{"ddos/localhost/x", "ddos"},
		{"ddos/detect/absolute-floor", "ddos"},
		{"traffic/usage/enabled", "traffic"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			if got := groupRootFor(tc.key, declared); got != tc.want {
				t.Errorf("groupRootFor(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
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
	proc.CloseConnForTest()

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
// VALIDATES: Plugin apply rejection → rollback fires, running config unchanged, error returned.
// PREVENTS: Partial apply leaving the runtime in a mixed old/new state.
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

	// With the TxCoordinator, an apply rejection triggers rollback, so the
	// running config tree is NOT updated: the runtime goes back to the old
	// state via the rollback callbacks. This differs from the legacy reload
	// path that used to partial-apply and still call SetConfigTree.
	reactor.mu.Lock()
	assert.Nil(t, reactor.setTree, "reactor config tree should not be updated after rollback")
	reactor.mu.Unlock()
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
	proc.CloseConnForTest()

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
		proc.ClearConnForTest()
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

// TestReloadTxCoordinatorRollback verifies that when one plugin fails apply
// the TxCoordinator publishes rollback to every participant, the RPC bridge
// dispatches config-rollback to each plugin process, and the reactor config
// tree stays at the old state because rollback reverts the runtime.
//
// VALIDATES: Apply failure -> broadcast rollback -> every participant
// receives config-rollback -> reactor.SetConfigTree NOT called.
// PREVENTS: Apply error leaving half the plugins applied without rollback,
// which would corrupt the runtime and regress the transaction protocol.
func TestReloadTxCoordinatorRollback(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}

	plugins := []pluginDef{
		{name: "rib", roots: []string{"bgp"}},
		{
			name:      "gr",
			roots:     []string{"bgp"},
			applyResp: &rpc.ConfigApplyOutput{Status: plugin.StatusError, Error: "journal full"},
		},
	}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err, "apply rejection should surface as reload error")
	assert.Contains(t, err.Error(), "journal full")
	assert.Contains(t, err.Error(), "gr")

	// Every participant gets a config-rollback RPC via the bridge because
	// the orchestrator broadcasts rollback to all participants when any
	// single apply fails. Even the plugin that never applied (gr failed
	// before succeeding) is invited to rollback so its journal stays
	// consistent.
	require.Eventually(t, func() bool { return plugins[0].responder.getRollbackCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "bgp rib should receive rollback RPC")
	require.Eventually(t, func() bool { return plugins[1].responder.getRollbackCalls() == 1 }, 2*time.Second, 10*time.Millisecond, "gr should receive rollback RPC")

	// Config tree stays at the old state because the runtime was rolled
	// back. The legacy reload path would have called SetConfigTree; the
	// transaction protocol deliberately does not.
	reactor.mu.Lock()
	assert.Nil(t, reactor.setTree, "reactor config tree should not be updated after rollback")
	reactor.mu.Unlock()
}

// TestReloadTxVerifyReceivesFullSubtree verifies that plugins see the
// post-change candidate subtree for their declared root when the
// TxCoordinator drives verify, matching the legacy reload.go contract.
//
// VALIDATES: ConfigSection.Data handed to OnConfigVerify is the full
// subtree JSON (e.g. {"router-id":"5.6.7.8"}) and not the diff shape
// ({"bgp/router-id":{"old":...,"new":...}}).
// PREVENTS: Plugins validating against a malformed diff payload instead
// of the candidate config -- a subtle regression that mock tests
// without payload inspection would miss.
func TestReloadTxVerifyReceivesFullSubtree(t *testing.T) {
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
	require.Eventually(t, func() bool { return plugins[0].responder.getVerifyCalls() == 1 }, 2*time.Second, 10*time.Millisecond)

	plugins[0].responder.mu.Lock()
	sections := append([]rpc.ConfigSection(nil), plugins[0].responder.verifySections...)
	plugins[0].responder.mu.Unlock()

	require.Len(t, sections, 1, "plugin should receive exactly one section for the bgp root")
	assert.Equal(t, "bgp", sections[0].Root)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(sections[0].Data), &data))
	bgp, ok := data["bgp"].(map[string]any)
	require.True(t, ok, "ExtractConfigSubtree wraps the leaf in its path, so the Data JSON starts with the root key; got %v", data)
	assert.Equal(t, "5.6.7.8", bgp["router-id"], "plugin should see the post-change candidate subtree")
	// Diff-shaped encoding would surface keys like "bgp/router-id" at the
	// top level (the mistake fixed here). Assert that neither shape leaks
	// through so a future regression is caught immediately.
	assert.NotContains(t, data, "bgp/router-id", "plugin must not see diff-shaped keys at top level")
	assert.NotContains(t, bgp, "old", "plugin must not see DiffPair fields")
	assert.NotContains(t, bgp, "new", "plugin must not see DiffPair fields")
}

// TestReloadTxApplyBGPLast verifies that the "bgp" participant receives
// config-apply after every non-bgp participant when BGP and other plugins
// share the bgp config root. This guards the legacy reload.go ordering
// semantic where BGP peer reconciliation saw sysrib/interface/gr/etc.
// committed state.
//
// VALIDATES: Participants with the "bgp" name are sorted to the tail of
// the apply emit order so their RPC runs last in the bridge's serial
// dispatch loop.
// PREVENTS: A future change that introduces a plugin literally named
// "bgp" silently reordering apply and breaking peer reconciliation.
func TestReloadTxApplyBGPLast(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}

	order := &orderRecorder{}
	plugins := []pluginDef{
		// Intentional registration order: "bgp" first, then a non-bgp
		// plugin. The sort must still place bgp last in apply order.
		{name: "bgp", roots: []string{"bgp"}, order: order},
		{name: "rib", roots: []string{"bgp"}, order: order},
		{name: "gr", roots: []string{"bgp"}, order: order},
	}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return len(order.snapshot()) == 3 }, 2*time.Second, 10*time.Millisecond)

	calls := order.snapshot()
	require.Len(t, calls, 3)
	assert.Equal(t, "bgp", calls[len(calls)-1], "bgp participant must receive config-apply last; got order %v", calls)
}

// TestReloadUsesRegisteredOperationDecomposer verifies that production reload
// wiring calls component-owned operation decomposers and uses operation RPCs
// instead of the legacy full-diff apply callback when operations are returned.
//
// VALIDATES: ReloadConfig -> registered decomposer -> operation verify/apply/commit callbacks.
// PREVENTS: Operation planning existing only behind test-only coordinator injection.
func TestReloadUsesRegisteredOperationDecomposer(t *testing.T) {
	root := fmt.Sprintf("oproot-reload-operation-path-%d", time.Now().UnixNano())
	owner := "opowner"
	require.NoError(t, transaction.RegisterOperationDecomposer(root, func(_ context.Context, req transaction.DecomposeRequest) ([]transaction.ConfigOperation, error) {
		assert.Equal(t, root, req.Root)
		assert.Contains(t, req.ActiveRoot, "old")
		assert.Contains(t, req.CandidateRoot, "new")
		return []transaction.ConfigOperation{{ID: "op-reload-1", Root: root, Owner: owner, Type: transaction.OperationSetProperty, Target: transaction.ResourceRef{Kind: transaction.ResourceSysctl, Name: "test"}}}, nil
	}))

	oldTree := map[string]any{root: map[string]any{"value": "old"}}
	newTree := map[string]any{root: map[string]any{"value": "new"}}
	reactor := &mockReloadReactor{tree: oldTree}
	plugins := []pluginDef{{
		name:  owner,
		roots: []string{root},
		configOps: []rpc.ConfigOperationDecl{{
			Root:       root,
			Decompose:  true,
			Operations: []rpc.ConfigOperationType{rpc.OperationSetProperty},
		}},
	}}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return plugins[0].responder.getOperationVerifyCalls() == 1 }, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return plugins[0].responder.getOperationApplyCalls() == 1 }, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return plugins[0].responder.getOperationCommitCalls() == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, plugins[0].responder.getApplyCalls(), "legacy config-apply must be skipped when operation path is used")
}

// TestReloadRejectsUndeclaredConfigOperation verifies production reload rejects
// operation-path commits when a plugin has not declared support for the
// operation callback it is about to receive.
//
// VALIDATES: Operation callbacks are exact-or-reject gated by Stage 1 declarations.
// PREVENTS: Undeclared operation callbacks being invoked by accident.
func TestReloadRejectsUndeclaredConfigOperation(t *testing.T) {
	root := fmt.Sprintf("oproot-undeclared-operation-%d", time.Now().UnixNano())
	owner := "opowner-undeclared"
	require.NoError(t, transaction.RegisterOperationDecomposer(root, func(context.Context, transaction.DecomposeRequest) ([]transaction.ConfigOperation, error) {
		return []transaction.ConfigOperation{{ID: "op-undeclared-1", Root: root, Owner: owner, Type: transaction.OperationSetProperty, Target: transaction.ResourceRef{Kind: transaction.ResourceSysctl, Name: "test"}}}, nil
	}))

	oldTree := map[string]any{root: map[string]any{"value": "old"}}
	newTree := map[string]any{root: map[string]any{"value": "new"}}
	reactor := &mockReloadReactor{tree: oldTree}
	plugins := []pluginDef{{name: owner, roots: []string{root}}}
	s := newTestReloadServer(t, reactor, plugins)

	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not declare config operation")
	assert.Equal(t, 0, plugins[0].responder.getOperationApplyCalls())
}

// TestReloadUsesExternalOperationDecompose verifies a plugin that declares a
// decompose callback can provide operations over the same event/RPC bridge as
// internal decomposers.
//
// VALIDATES: Stage 1 ConfigOperations declarations drive external operation decomposition.
// PREVENTS: External operation callback support being stored but never read.
func TestReloadUsesExternalOperationDecompose(t *testing.T) {
	root := fmt.Sprintf("oproot-external-decompose-%d", time.Now().UnixNano())
	owner := "opowner-external"
	op := transaction.ConfigOperation{ID: "op-external-1", Root: root, Owner: owner, Type: transaction.OperationSetProperty, Target: transaction.ResourceRef{Kind: transaction.ResourceSysctl, Name: "test"}}

	oldTree := map[string]any{root: map[string]any{"value": "old"}}
	newTree := map[string]any{root: map[string]any{"value": "new"}}
	reactor := &mockReloadReactor{tree: oldTree}
	plugins := []pluginDef{{
		name:  owner,
		roots: []string{root},
		configOps: []rpc.ConfigOperationDecl{{
			Root:       root,
			Decompose:  true,
			Operations: []rpc.ConfigOperationType{rpc.OperationSetProperty},
		}},
	}}
	s := newTestReloadServer(t, reactor, plugins)
	plugins[0].responder.mu.Lock()
	plugins[0].responder.opDecomposeResp = &rpc.ConfigOperationDecomposeOutput{Status: rpc.StatusOK, Operations: []rpc.ConfigOperation{op}}
	plugins[0].responder.mu.Unlock()

	err := s.ReloadConfig(context.Background(), newTree)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return plugins[0].responder.getOperationDecomposeCalls() == 1 }, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return plugins[0].responder.getOperationApplyCalls() == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, plugins[0].responder.getApplyCalls(), "legacy config-apply must be skipped when external operation decompose returns operations")
}

// TestConfigTxBridgeDispatchesOperationApply verifies that operation apply
// stream events use the same bridge path as full-diff apply events: the bridge
// receives the per-plugin config event, calls the SDK/RPC operation callback,
// and publishes an operation ack back to the config namespace.
//
// VALIDATES: config operation apply event -> config-operation-apply RPC -> operation apply ack.
// PREVENTS: Operation executor events being registered but never reaching external plugins.
func TestConfigTxBridgeDispatchesOperationApply(t *testing.T) {
	t.Parallel()

	reactor := &mockReloadReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}}
	plugins := []pluginDef{
		{name: "bgp", roots: []string{"bgp"}},
	}
	s := newTestReloadServer(t, reactor, plugins)
	gw := newConfigEventGateway(s)
	bridge := newConfigTxBridge(s, gw, []string{"bgp"}, map[string][]rpc.ConfigSection{})
	require.NoError(t, bridge.Subscribe(context.Background()))
	defer bridge.Close()

	ackCh := make(chan transaction.ConfigOperationApplyAck, 1)
	errCh := make(chan error, 1)
	unsub := gw.SubscribeConfigEvent(transaction.EventOperationApplyOK, func(payload []byte) {
		var ack transaction.ConfigOperationApplyAck
		if err := json.Unmarshal(payload, &ack); err != nil {
			errCh <- err
			return
		}
		ackCh <- ack
	})
	defer unsub()

	op := transaction.ConfigOperation{
		ID:    "op-1",
		Root:  "bgp",
		Owner: "bgp",
		Type:  transaction.OperationAddPeer,
		Target: transaction.ResourceRef{
			Kind: transaction.ResourcePeer,
			Peer: "192.0.2.1",
		},
		Params: transaction.ConfigOperationParams{Peer: "192.0.2.1"},
	}
	payload, err := json.Marshal(transaction.ConfigOperationApplyEvent{
		TransactionID: "tx-op",
		Operation:     op,
		DeadlineMS:    time.Now().Add(time.Second).UnixMilli(),
	})
	require.NoError(t, err)

	_, err = gw.EmitConfigEvent(transaction.EventOperationApplyFor("bgp"), payload)
	require.NoError(t, err)

	require.Eventually(t, func() bool { return plugins[0].responder.getOperationApplyCalls() == 1 }, 2*time.Second, 10*time.Millisecond)
	plugins[0].responder.mu.Lock()
	gotInput := plugins[0].responder.opApplyInput
	plugins[0].responder.mu.Unlock()
	assert.Equal(t, "tx-op", gotInput.TransactionID)
	assert.Equal(t, op.ID, gotInput.Operation.ID)
	assert.Equal(t, op.Type, gotInput.Operation.Type)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case ack := <-ackCh:
		assert.Equal(t, "tx-op", ack.TransactionID)
		assert.Equal(t, "bgp", ack.Plugin)
		assert.Equal(t, op.ID, ack.OperationID)
		assert.Equal(t, transaction.CodeOK, ack.Status)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for operation apply ack")
	}
}

// TestConfigTxBridgeDispatchesOperationRollback verifies that executor rollback
// events reach the plugin operation rollback callback and produce a config ack.
//
// VALIDATES: config operation rollback event -> config-operation-rollback RPC -> operation rollback ack.
// PREVENTS: Executor rollback events being emitted without reaching plugin journals.
func TestConfigTxBridgeDispatchesOperationRollback(t *testing.T) {
	t.Parallel()

	reactor := &mockReloadReactor{tree: map[string]any{"interface": map[string]any{"eth0": map[string]any{}}}}
	plugins := []pluginDef{
		{name: "iface", roots: []string{"interface"}},
	}
	s := newTestReloadServer(t, reactor, plugins)
	gw := newConfigEventGateway(s)
	bridge := newConfigTxBridge(s, gw, []string{"iface"}, map[string][]rpc.ConfigSection{})
	require.NoError(t, bridge.Subscribe(context.Background()))
	defer bridge.Close()

	ackCh := make(chan transaction.ConfigOperationRollbackAck, 1)
	errCh := make(chan error, 1)
	unsub := gw.SubscribeConfigEvent(transaction.EventOperationRollbackOK, func(payload []byte) {
		var ack transaction.ConfigOperationRollbackAck
		if err := json.Unmarshal(payload, &ack); err != nil {
			errCh <- err
			return
		}
		ackCh <- ack
	})
	defer unsub()

	op := transaction.ConfigOperation{
		ID:    "op-rollback-1",
		Root:  "interface",
		Owner: "iface",
		Type:  transaction.OperationAddAddress,
		Target: transaction.ResourceRef{
			Kind:      transaction.ResourceAddress,
			Interface: "eth0",
			Address:   "192.0.2.1/32",
		},
	}
	payload, err := json.Marshal(transaction.ConfigOperationRollbackEvent{
		TransactionID: "tx-op-rollback",
		Operations:    []transaction.ConfigOperation{op},
		DeadlineMS:    time.Now().Add(time.Second).UnixMilli(),
	})
	require.NoError(t, err)

	_, err = gw.EmitConfigEvent(transaction.EventOperationRollbackFor("iface"), payload)
	require.NoError(t, err)

	require.Eventually(t, func() bool { return plugins[0].responder.getOperationRollbackCalls() == 1 }, 2*time.Second, 10*time.Millisecond)
	plugins[0].responder.mu.Lock()
	gotInput := plugins[0].responder.opRollbackInput
	plugins[0].responder.mu.Unlock()
	assert.Equal(t, "tx-op-rollback", gotInput.TransactionID)
	require.Len(t, gotInput.Operations, 1)
	assert.Equal(t, op.ID, gotInput.Operations[0].ID)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case ack := <-ackCh:
		assert.Equal(t, "tx-op-rollback", ack.TransactionID)
		assert.Equal(t, "iface", ack.Plugin)
		assert.Equal(t, op.ID, ack.OperationID)
		assert.Equal(t, transaction.CodeOK, ack.Status)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for operation rollback ack")
	}
}

// TestConfigTxBridgeDispatchesOperationDecompose verifies operation decompose
// callbacks carry active root, candidate root, and diff context to the plugin.
//
// VALIDATES: config operation decompose event -> config-operation-decompose RPC -> operation decompose ack.
// PREVENTS: External plugins being unable to participate in component-owned decomposition.
func TestConfigTxBridgeDispatchesOperationDecompose(t *testing.T) {
	t.Parallel()

	reactor := &mockReloadReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}}
	plugins := []pluginDef{{name: "bgp", roots: []string{"bgp"}}}
	s := newTestReloadServer(t, reactor, plugins)
	gw := newConfigEventGateway(s)
	bridge := newConfigTxBridge(s, gw, []string{"bgp"}, map[string][]rpc.ConfigSection{})
	require.NoError(t, bridge.Subscribe(context.Background()))
	defer bridge.Close()

	op := rpc.ConfigOperation{ID: "decomposed-1", Root: "bgp", Owner: "bgp", Type: rpc.OperationAddPeer}
	plugins[0].responder.mu.Lock()
	plugins[0].responder.opDecomposeResp = &rpc.ConfigOperationDecomposeOutput{Status: rpc.StatusOK, Operations: []rpc.ConfigOperation{op}}
	plugins[0].responder.mu.Unlock()

	ackCh := make(chan transaction.ConfigOperationDecomposeAck, 1)
	unsub := gw.SubscribeConfigEvent(transaction.EventOperationDecomposeOK, func(payload []byte) {
		var ack transaction.ConfigOperationDecomposeAck
		require.NoError(t, json.Unmarshal(payload, &ack))
		ackCh <- ack
	})
	defer unsub()

	payload, err := json.Marshal(transaction.ConfigOperationDecomposeEvent{
		TransactionID: "tx-op-decompose",
		Root:          "bgp",
		ActiveRoot:    `{"bgp":{"router-id":"1.2.3.4"}}`,
		CandidateRoot: `{"bgp":{"router-id":"5.6.7.8"}}`,
		Diff:          transaction.DiffSection{Root: "bgp", Changed: `{"bgp/router-id":{"old":"1.2.3.4","new":"5.6.7.8"}}`},
		DeadlineMS:    time.Now().Add(time.Second).UnixMilli(),
	})
	require.NoError(t, err)

	_, err = gw.EmitConfigEvent(transaction.EventOperationDecomposeFor("bgp"), payload)
	require.NoError(t, err)

	require.Eventually(t, func() bool { return plugins[0].responder.getOperationDecomposeCalls() == 1 }, 2*time.Second, 10*time.Millisecond)
	plugins[0].responder.mu.Lock()
	gotInput := plugins[0].responder.opDecomposeInput
	plugins[0].responder.mu.Unlock()
	assert.Equal(t, "tx-op-decompose", gotInput.TransactionID)
	assert.Equal(t, "bgp", gotInput.Root)
	assert.Equal(t, "bgp", gotInput.Active.Root)
	assert.Contains(t, gotInput.Candidate.Data, "5.6.7.8")
	assert.Equal(t, "bgp", gotInput.Diff.Root)

	select {
	case ack := <-ackCh:
		assert.Equal(t, "tx-op-decompose", ack.TransactionID)
		assert.Equal(t, "bgp", ack.Plugin)
		require.Len(t, ack.Operations, 1)
		assert.Equal(t, op.ID, ack.Operations[0].ID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for operation decompose ack")
	}
}

// TestConfigTxBridgeDispatchesOperationVerifyAndCommit verifies the remaining
// operation lifecycle callbacks used before and after execution.
//
// VALIDATES: operation verify and commit events reach SDK/RPC callbacks and ack.
// PREVENTS: Operation journals being applied without verify or finalization callbacks.
func TestConfigTxBridgeDispatchesOperationVerifyAndCommit(t *testing.T) {
	t.Parallel()

	reactor := &mockReloadReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}}
	plugins := []pluginDef{{name: "bgp", roots: []string{"bgp"}}}
	s := newTestReloadServer(t, reactor, plugins)
	gw := newConfigEventGateway(s)
	bridge := newConfigTxBridge(s, gw, []string{"bgp"}, map[string][]rpc.ConfigSection{})
	require.NoError(t, bridge.Subscribe(context.Background()))
	defer bridge.Close()

	verifyAckCh := make(chan transaction.ConfigOperationVerifyAck, 1)
	verifyUnsub := gw.SubscribeConfigEvent(transaction.EventOperationVerifyOK, func(payload []byte) {
		var ack transaction.ConfigOperationVerifyAck
		require.NoError(t, json.Unmarshal(payload, &ack))
		verifyAckCh <- ack
	})
	defer verifyUnsub()
	commitAckCh := make(chan transaction.ConfigOperationCommitAck, 1)
	commitUnsub := gw.SubscribeConfigEvent(transaction.EventOperationCommitOK, func(payload []byte) {
		var ack transaction.ConfigOperationCommitAck
		require.NoError(t, json.Unmarshal(payload, &ack))
		commitAckCh <- ack
	})
	defer commitUnsub()

	op := transaction.ConfigOperation{ID: "verify-1", Root: "bgp", Owner: "bgp", Type: transaction.OperationAddPeer}
	verifyPayload, err := json.Marshal(transaction.ConfigOperationVerifyEvent{TransactionID: "tx-op-lifecycle", Operation: op, DeadlineMS: time.Now().Add(time.Second).UnixMilli()})
	require.NoError(t, err)
	_, err = gw.EmitConfigEvent(transaction.EventOperationVerifyFor("bgp"), verifyPayload)
	require.NoError(t, err)

	commitPayload, err := json.Marshal(transaction.ConfigOperationCommitEvent{TransactionID: "tx-op-lifecycle", DeadlineMS: time.Now().Add(time.Second).UnixMilli()})
	require.NoError(t, err)
	_, err = gw.EmitConfigEvent(transaction.EventOperationCommitFor("bgp"), commitPayload)
	require.NoError(t, err)

	require.Eventually(t, func() bool { return plugins[0].responder.getOperationVerifyCalls() == 1 }, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return plugins[0].responder.getOperationCommitCalls() == 1 }, 2*time.Second, 10*time.Millisecond)
	plugins[0].responder.mu.Lock()
	gotVerify := plugins[0].responder.opVerifyInput
	gotCommit := plugins[0].responder.opCommitInput
	plugins[0].responder.mu.Unlock()
	assert.Equal(t, op.ID, gotVerify.Operation.ID)
	assert.Equal(t, "tx-op-lifecycle", gotCommit.TransactionID)

	select {
	case ack := <-verifyAckCh:
		assert.Equal(t, "tx-op-lifecycle", ack.TransactionID)
		assert.Equal(t, op.ID, ack.OperationID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for operation verify ack")
	}
	select {
	case ack := <-commitAckCh:
		assert.Equal(t, "tx-op-lifecycle", ack.TransactionID)
		assert.Equal(t, "bgp", ack.Plugin)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for operation commit ack")
	}
}
