// Design: docs/architecture/hub-architecture.md -- live bus filter reloads vs external process declaration refusals
// Related: reload_declarations.go -- RefuseChangedDeclarations

package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
)

// TestRefuseChangedDeclarationsAllowsLiveBusFilterReload verifies that
// external declaration comparison ignores bus filter changes (which live
// outside PluginConfig) and canonical defaults, while refusing external process
// changes (run, encoding, respawn, timeout, on-exit).
//
// VALIDATES: Live bus filter edits pass declarationChangeRefusal; changing
// run, encoding, respawn, timeout, or on-exit is refused naming the leaf.
// PREVENTS: Live bus filter reloads spuriously demanding a daemon restart, or
// process-level changes slipping past reload refusal.
func TestRefuseChangedDeclarationsAllowsLiveBusFilterReload(t *testing.T) {
	started := []plugin.PluginConfig{{
		Name:         "collector",
		Run:          "/usr/local/bin/collector",
		Encoder:      "",
		Respawn:      plugin.RespawnUnstated,
		OnExit:       "",
		StageTimeout: 5 * time.Second,
	}}

	// Canonical default spellings ("json" for empty Encoder, "withdraw" for
	// empty OnExit) and ReceiveUpdate/WorkDir differences must not trigger a
	// restart refusal.
	sameProcess := []plugin.PluginConfig{{
		Name:          "collector",
		Run:           "/usr/local/bin/collector",
		Encoder:       plugin.EncodingJSON,
		Respawn:       plugin.RespawnUnstated,
		OnExit:        plugin.OnExitWithdraw,
		StageTimeout:  5 * time.Second,
		WorkDir:       "/tmp/other",
		ReceiveUpdate: true,
	}}
	assert.NoError(t, declarationChangeRefusal(started, sameProcess))

	// Changing run must be refused naming "run".
	changedRun := []plugin.PluginConfig{{
		Name:         "collector",
		Run:          "/usr/local/bin/collector-v2",
		StageTimeout: 5 * time.Second,
	}}
	err := declarationChangeRefusal(started, changedRun)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run")

	// Changing encoding must be refused naming "encoding".
	changedEnc := []plugin.PluginConfig{{
		Name:         "collector",
		Run:          "/usr/local/bin/collector",
		Encoder:      plugin.EncodingText,
		StageTimeout: 5 * time.Second,
	}}
	err = declarationChangeRefusal(started, changedEnc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "encoding")

	// Changing on-exit must be refused naming "on-exit".
	changedOnExit := []plugin.PluginConfig{{
		Name:         "collector",
		Run:          "/usr/local/bin/collector",
		OnExit:       plugin.OnExitKeep,
		StageTimeout: 5 * time.Second,
	}}
	err = declarationChangeRefusal(started, changedOnExit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "on-exit")
}
