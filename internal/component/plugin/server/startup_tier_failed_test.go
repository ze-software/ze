// Design: docs/architecture/api/process-protocol.md -- a failure during a reload means the plugin failed.
// Related: reload_refused_untouched_test.go -- the same rule met at the tier barrier's wait.

package server

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// registerRefusedSibling registers a plugin whose declaration names no protocol
// version, which the engine refuses at Stage 1, failing its tier. refused closes
// once the plugin has read the refusal; the plugin then stays connected until the
// engine closes its end. Declaring nothing, its own end stops nobody.
func registerRefusedSibling(t *testing.T, name, root string, refused chan<- struct{}) {
	t.Helper()
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "declares no protocol version, so its tier fails",
		ConfigRoots: []string{root},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			mux := rpc.NewMuxConn(rpc.NewConn(conn, conn))
			defer mux.Close() //nolint:errcheck // The engine already closed its end.
			_, err := mux.CallRPC(context.Background(), rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{})
			close(refused)
			if err == nil {
				return 1
			}
			<-mux.Done()
			return 0
		},
	}))
}

// awaitTierFailure blocks until the tier's coordinator holds the refused
// sibling's failure, and answers that failure. The caller has not completed
// Registration, so the tier cannot advance and only a failure ends the wait. A
// missing coordinator answers nil, which every caller refuses as no failure seen.
func awaitTierFailure(s *Server, siblingRefused <-chan struct{}) error {
	<-siblingRefused
	s.coordinatorMu.Lock()
	coord := s.coordinator
	s.coordinatorMu.Unlock()
	if coord == nil {
		return nil
	}
	return coord.WaitForStage(context.Background(), plugin.StageConfig)
}

// TestFatalPluginDeclaringAfterASiblingFailedKeepsTheDaemon drives the interleaving
// a loaded host hit at random: a sibling fails the tier after the fatal plugin's
// registration was sent and before the engine records its configure allowance.
//
// GOAL: a sibling's failure met at the allowance declaration is the tier's, not the
// fatal plugin's, so the reload is refused and the daemon keeps running.
// METHOD: real plugins over the real reload entry point. The fatal plugin sends its
// registration only after the coordinator has recorded the sibling's failure, which
// it observes through WaitForStage, so the order is forced with no timer and no
// production seam.
// VALIDATES: the coordinator held the sibling's failure first; the reload names the
// sibling's refusal; ze did not stop; both plugins are gone.
// PREVENTS: the sibling's error recorded as the fatal plugin's startup cause, which
// hid the barrier abort and stopped a running daemon over a config typo.
func TestFatalPluginDeclaringAfterASiblingFailedKeepsTheDaemon(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	s, spawner := newLifecycleStartupServer(t)
	s.reactor = &mockReloadReactor{tree: map[string]any{}}
	_, stopped := watchDaemonStop(s)

	const fatalName, siblingName = "fatal-declares-after-tier-failed", "tier-failing-sibling"
	siblingRefused := make(chan struct{})
	observed := make(chan error, 1)
	registerRefusedSibling(t, siblingName, "tierfailedsibling", siblingRefused)
	require.NoError(t, registry.Register(registry.Registration{
		Name:        fatalName,
		Description: "declares fatal once its tier has failed, and stays connected",
		ConfigRoots: []string{"tierfailedfatal"},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			mux := rpc.NewMuxConn(rpc.NewConn(conn, conn))
			defer mux.Close() //nolint:errcheck // The engine already closed its end.
			observed <- awaitTierFailure(s, siblingRefused)
			if _, err := mux.CallRPC(context.Background(), rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				FailurePolicy:   rpc.FailureFatal,
			}); err != nil {
				return 1
			}
			<-mux.Done()
			return 0
		},
	}))

	err := s.ReloadConfig(context.Background(), map[string]any{
		"tierfailedfatal":   map[string]any{"enabled": "true"},
		"tierfailedsibling": map[string]any{"enabled": "true"},
	})

	tierFailure := <-observed
	require.Error(t, tierFailure, "the coordinator held the sibling's failure before the fatal plugin declared")
	assert.Contains(t, tierFailure.Error(), "failed: startup incomplete", "the failure recorded first is the sibling's")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "declared no protocol version", "the reload names the sibling's refusal")
	assert.NotErrorIs(t, err, errStartupPluginEnded, "the fatal plugin was connected when it met the failed tier")
	assert.False(t, stopped(), "a sibling's failure met at the allowance declaration must not stop the daemon")
	assert.Nil(t, spawner.pm.GetProcess(fatalName), "the failed tier is rolled back")
	assert.Nil(t, spawner.pm.GetProcess(siblingName), "the failed tier is rolled back")
}

// TestFatalPluginEndedBeforeItsDeclarationMetTheFailedTierStopsTheDaemon is the
// polarity of the test above: the same failed tier, met by a fatal plugin whose
// engine had already returned.
//
// GOAL: the tier-failure route forgives only a plugin that was still connected; a
// fatal plugin that ended before its allowance met the failed tier stops ze.
// METHOD: as above, and the plugin holds the server's coordinator lock across its
// registration, so the engine's stageTransition parks between the registration's
// answer and the allowance declaration. The test releases the lock only once the
// plugin's engine has returned, so the end precedes the declaration with no timer.
// VALIDATES: the reload names the plugin's end, and the daemon stops.
// PREVENTS: a fix that reads every tier-failure answer as a forgivable abort.
func TestFatalPluginEndedBeforeItsDeclarationMetTheFailedTierStopsTheDaemon(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	s, spawner := newLifecycleStartupServer(t)
	s.reactor = &mockReloadReactor{tree: map[string]any{}}
	_, stopped := watchDaemonStop(s)

	// The fatal plugin's name sorts first: a phase reports the first diagnosis in
	// name order, and the sibling's own is its refusal.
	const fatalName = "a-fatal-ends-before-declaring"
	siblingRefused := make(chan struct{})
	observed := make(chan error, 1)
	answered := make(chan struct{})
	registerRefusedSibling(t, "ended-tier-failing-sibling", "tierendedsibling", siblingRefused)
	require.NoError(t, registry.Register(registry.Registration{
		Name:        fatalName,
		Description: "declares fatal once its tier has failed, and exits once answered",
		ConfigRoots: []string{"tierendedfatal"},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			mux := rpc.NewMuxConn(rpc.NewConn(conn, conn))
			defer mux.Close() //nolint:errcheck // The engine already closed its end.
			observed <- awaitTierFailure(s, siblingRefused)
			s.coordinatorMu.Lock() // Released by the test once this engine has returned.
			defer close(answered)
			if _, err := mux.CallRPC(context.Background(), rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				FailurePolicy:   rpc.FailureFatal,
			}); err != nil {
				return 1
			}
			return 0
		},
	}))

	go func() {
		<-answered
		if proc := spawner.pm.GetProcess(fatalName); proc != nil {
			<-proc.EngineDone()
		}
		s.coordinatorMu.Unlock()
	}()

	err := s.ReloadConfig(context.Background(), map[string]any{
		"tierendedfatal":   map[string]any{"enabled": "true"},
		"tierendedsibling": map[string]any{"enabled": "true"},
	})

	tierFailure := <-observed
	require.Error(t, tierFailure, "the coordinator held the sibling's failure before the fatal plugin declared")
	assert.Contains(t, tierFailure.Error(), "failed: startup incomplete", "the failure recorded first is the sibling's")
	require.ErrorIs(t, err, errStartupPluginEnded, "the reload names the plugin's end, not the sibling's failure")
	assert.True(t, stopped(), "a fatal plugin that ended before meeting the failed tier must stop the daemon")
}
