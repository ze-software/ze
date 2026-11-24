// Design: docs/plugin-development/testing.md -- configured-target conformance through daemon-owned admission.
// Related: main_reload.go -- complete accepted configuration publication.
// Related: main_reload_auth_test.go -- reloadDriver's no-diff orchestration pattern.

package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/conformance"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const configuredConformanceCommand = "request plugin conformance"
const configuredConformanceBase = "system { dns { resolv-conf-path \"\"; } }\n"

type configuredConformanceFixture struct {
	dir         string
	path        string
	store       storage.Storage
	server      *pluginserver.Server
	provider    *zeconfig.Provider
	coordinator *plugin.Coordinator
	policy      *authz.Store
}

// newConfiguredConformanceFixture drives the actual whole-reload acceptance
// boundary. As reloadDriver does, it hands the coordinator the same runtime
// tree as the reload: no live plugin is needed to test accepted authority.
// Query programs start only when the tested conformance action invokes them.
func newConfiguredConformanceFixture(t *testing.T) *configuredConformanceFixture {
	t.Helper()
	resetAAABundleForTest(t)
	isolateBaseLogLevel(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "router.conf")
	store := newTestStore(t, dir)
	require.NoError(t, store.WriteFile(path, []byte(configuredConformanceBase), 0o600))
	require.NoError(t, initializeConfigSource(store, path, []byte(configuredConformanceBase)))
	parsed, err := zeconfig.ReadConfig(configuredConformanceBase, path, nil)
	require.NoError(t, err)
	coordinator := plugin.NewCoordinator(parsed.Tree.ToRunningMap())
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, coordinator)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	t.Cleanup(server.Stop)
	policy := authz.NewStore()
	policy.AddProfile(authz.Profile{Name: "conformance-admin", Run: authz.Section{Default: authz.Allow}, Edit: authz.Section{Default: authz.Allow}})
	policy.AddProfile(authz.Profile{Name: "conformance-reader", Run: authz.Section{Default: authz.Allow}, Edit: authz.Section{Default: authz.Deny}})
	policy.AssignProfiles("conformance-admin", []string{"conformance-admin"})
	policy.AssignProfiles("conformance-reader", []string{"conformance-reader"})
	f := &configuredConformanceFixture{dir: dir, path: path, store: store, server: server, provider: zeconfig.NewProvider(), coordinator: coordinator, policy: policy}
	f.program(t, "accepted")
	f.program(t, "candidate-only")
	f.accept(t, configuredConformanceConfig("accepted"))
	// Reset after startup and acceptance, so no admission control can pass by
	// counting the daemon's own earlier declaration/start work as this action.
	f.resetStarts(t)
	return f
}

func configuredConformanceConfig(names ...string) string {
	if len(names) == 0 {
		return configuredConformanceBase
	}
	var entries []string
	for _, name := range names {
		entries = append(entries, fmt.Sprintf("endpoint %s { external { run %q; } encoding json; }", name, "sh ./"+name+".sh"))
	}
	return configuredConformanceBase + "bus {\n" + strings.Join(entries, "\n") + "\n}\n"
}

// program records the selected identity in the daemon-resolved work directory,
// then emits an actual declaration using the protocol writer's exact bytes.
func (f *configuredConformanceFixture) program(t *testing.T, name string) {
	t.Helper()
	var declaration bytes.Buffer
	require.NoError(t, rpc.WriteDeclaration(&declaration, &rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show configured proof", ShortHelp: "Show the configured proof."}},
	}))
	require.NotContains(t, declaration.String(), "'")
	script := "#!/bin/sh\nprintf '%s\\n' '" + name + "' >> starts\nprintf '%s' '" + declaration.String() + "'\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, name+".sh"), []byte(script), 0o600))
}

func (f *configuredConformanceFixture) accept(t *testing.T, content string) {
	t.Helper()
	parsed, err := zeconfig.ReadConfig(content, f.path, nil)
	require.NoError(t, err)
	_, err = storage.WriteCandidateVersion(f.store, f.path, []byte(content), time.Now())
	require.NoError(t, err)
	// Only the domain diff is empty. Parsing, provider refresh, source promotion
	// and the complete reload's accepted-generation publication run for real.
	f.coordinator.SetConfigTree(parsed.Tree.ToRunningMap())
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return parsed.Tree.ToRunningMap(), parsed.Tree, nil
	}
	require.NoError(t, doReload(f.server, f.provider, f.store, f.path, load, nil))
	active, err := storage.ReadActiveConfig(f.store, f.path)
	require.NoError(t, err)
	require.Equal(t, content, string(active), "the assertion must follow actual source promotion")
}

func (f *configuredConformanceFixture) request(ctx context.Context, username, name string) (*plugin.Response, error) {
	return serverDispatcher(f.server, audit.Web)(ctx, plugin.CallerIdentity{
		Username: username, RemoteAddr: "192.0.2.20:43000", Authorizer: authz.StoreAuthorizer{Store: f.policy},
	}, configuredConformanceCommand+" name "+name)
}

func (f *configuredConformanceFixture) resetStarts(t *testing.T) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, "starts"), nil, 0o600))
}

func (f *configuredConformanceFixture) requireStarts(t *testing.T, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(f.dir, "starts"))
	require.NoError(t, err)
	require.Equal(t, want, string(got), "only this conformance action may start the selected accepted program")
}

func (f *configuredConformanceFixture) requireAccepted(t *testing.T) {
	t.Helper()
	response, err := f.request(t.Context(), "conformance-admin", "accepted")
	require.NoError(t, err, "the accepted target must be supported before judging a refusal control")
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusDone, response.Status)
	f.requireStarts(t, "accepted\n")
	f.resetStarts(t)
}

// TestConfiguredPluginConformanceAcceptedTargetMatchesRun pairs the daemon
// operation with the existing Run producer over the exact configured target.
// A relative script and its marker prove the accepted work directory is used.
func TestConfiguredPluginConformanceAcceptedTargetMatchesRun(t *testing.T) {
	f := newConfiguredConformanceFixture(t)
	budget, err := plugin.QueryBudget()
	require.NoError(t, err)
	local, err := conformance.Run(t.Context(), conformance.Target{Name: "accepted", Run: "sh ./accepted.sh", WorkDir: f.dir}, budget)
	require.NoError(t, err)
	require.Equal(t, conformance.VerdictPass, local.Verdict)
	f.resetStarts(t)
	response, err := f.request(t.Context(), "conformance-admin", "accepted")
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusDone, response.Status)
	encoded, err := json.Marshal(response.Data)
	require.NoError(t, err)
	var daemon struct {
		Checks []conformance.Row `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(encoded, &daemon))
	require.Equal(t, local.Rows, daemon.Checks)
	f.requireStarts(t, "accepted\n")
}

// TestConfiguredPluginConformanceUnconfiguredAndCandidateTargetsStartNothing
// admits an accepted target first, then proves neither an unknown name nor a
// candidate-only plugin or an edited source file changes execution authority.
func TestConfiguredPluginConformanceUnconfiguredAndCandidateTargetsStartNothing(t *testing.T) {
	f := newConfiguredConformanceFixture(t)
	f.requireAccepted(t)
	candidate := configuredConformanceConfig("accepted", "candidate-only")
	_, err := storage.WriteCandidateVersion(f.store, f.path, []byte(candidate), time.Now())
	require.NoError(t, err)
	require.NoError(t, f.store.WriteFile(f.path, []byte(candidate), 0o600), "a mirror/file edit is not an accepted generation")
	for _, name := range []string{"unconfigured", "candidate-only"} {
		t.Run(name, func(t *testing.T) {
			_, err := f.request(t.Context(), "conformance-admin", name)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "unknown command")
			assert.Contains(t, err.Error(), name)
			f.requireStarts(t, "")
		})
	}
}

// TestConfiguredPluginConformanceRemovedTargetStartsNothing promotes a config
// removing a previously accepted target, then refuses that formerly valid name.
func TestConfiguredPluginConformanceRemovedTargetStartsNothing(t *testing.T) {
	f := newConfiguredConformanceFixture(t)
	f.requireAccepted(t)
	f.accept(t, configuredConformanceConfig())
	f.resetStarts(t)
	_, err := f.request(t.Context(), "conformance-admin", "accepted")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "unknown command")
	assert.Contains(t, err.Error(), "accepted")
	f.requireStarts(t, "")
}

// TestConfiguredPluginConformanceReaderStartsNothing uses the real shared
// authorizer with a distinct read-only identity after an allowed control.
func TestConfiguredPluginConformanceReaderStartsNothing(t *testing.T) {
	f := newConfiguredConformanceFixture(t)
	f.requireAccepted(t)
	_, err := f.request(t.Context(), "conformance-reader", "accepted")
	require.ErrorIs(t, err, pluginserver.ErrUnauthorized)
	f.requireStarts(t, "")
}

// TestConfiguredPluginConformanceCanceledRequestStartsNothing cancels before
// dispatch after an allowed control, so cancellation cannot masquerade as a
// missing command and the program must not start under the daemon identity.
func TestConfiguredPluginConformanceCanceledRequestStartsNothing(t *testing.T) {
	f := newConfiguredConformanceFixture(t)
	f.requireAccepted(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := f.request(ctx, "conformance-admin", "accepted")
	require.ErrorIs(t, err, context.Canceled)
	f.requireStarts(t, "")
}

// TestConfiguredPluginConformanceStoppedGenerationStartsNothing stops the
// owning server after an allowed control, then refuses the stale accepted
// target even when the new request context itself has not been canceled.
func TestConfiguredPluginConformanceStoppedGenerationStartsNothing(t *testing.T) {
	f := newConfiguredConformanceFixture(t)
	f.requireAccepted(t)
	f.server.Stop()
	_, err := f.request(t.Context(), "conformance-admin", "accepted")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "unknown command")
	f.requireStarts(t, "")
}

// TestConfiguredPluginConformanceKeepsServerOwnership publishes two live
// daemon generations. Each dispatch must use its own accepted target, and
// stopping the older server must neither erase nor lend the newer target.
func TestConfiguredPluginConformanceKeepsServerOwnership(t *testing.T) {
	older := newConfiguredConformanceFixture(t)
	newer := newConfiguredConformanceFixture(t)
	older.requireAccepted(t)
	newer.requireStarts(t, "")
	newer.requireAccepted(t)
	older.requireStarts(t, "")

	older.server.Stop()
	newer.requireAccepted(t)
	older.requireStarts(t, "")
	_, err := older.request(t.Context(), "conformance-admin", "accepted")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "unknown command")
	older.requireStarts(t, "")
	newer.requireStarts(t, "")
}

// TestConfiguredPluginConformanceCompletedFailureRetainsRows removes a program
// after its accepted positive control. A completed failed check must keep the
// exact report, while a name never accepted must refuse without a report.
func TestConfiguredPluginConformanceCompletedFailureRetainsRows(t *testing.T) {
	f := newConfiguredConformanceFixture(t)
	f.requireAccepted(t)
	require.NoError(t, os.Remove(filepath.Join(f.dir, "accepted.sh")))
	budget, err := plugin.QueryBudget()
	require.NoError(t, err)
	local, err := conformance.Run(t.Context(), conformance.Target{Name: "accepted", Run: "sh ./accepted.sh", WorkDir: f.dir}, budget)
	require.NoError(t, err)
	require.Equal(t, conformance.VerdictFail, local.Verdict)
	response, err := f.request(t.Context(), "conformance-admin", "accepted")
	require.Error(t, err, "a completed failed conformance report keeps its failure status")
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusError, response.Status)
	require.NotNil(t, response.Data, "completed failed checks must not be suppressed by the failure channel")
	encoded, err := json.Marshal(response.Data)
	require.NoError(t, err)
	var daemon struct {
		Checks []conformance.Row `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(encoded, &daemon))
	require.Equal(t, local.Rows, daemon.Checks)
	f.requireStarts(t, "")

	response, err = f.request(t.Context(), "conformance-admin", "never-configured")
	require.Error(t, err)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusError, response.Status)
	assert.Nil(t, response.Data, "a run that was not admitted must not invent a conformance report")
	f.requireStarts(t, "")
}
