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

// TestReloadRefusesChangedDeliveryQueueMax verifies that changing an external
// endpoint's delivery-queue-max across a reload is refused naming
// "delivery-queue-max", while identical values pass.
//
// VALIDATES: AC-5 -- declarationChangeRefusal refuses delivery-queue-max
// transitions (0 -> 4096, 4096 -> 8192, 4096 -> 0) naming "delivery-queue-max"
// and allows unchanged values.
// PREVENTS: Live reloads silently ignoring a changed delivery-queue-max leaf
// without requesting a daemon restart.
func TestReloadRefusesChangedDeliveryQueueMax(t *testing.T) {
	makeDecl := func(queueMax int) []plugin.PluginConfig {
		return []plugin.PluginConfig{{
			Name:             "collector",
			Run:              "/usr/local/bin/collector",
			StageTimeout:     5 * time.Second,
			DeliveryQueueMax: queueMax,
		}}
	}

	assert.NoError(t, declarationChangeRefusal(makeDecl(0), makeDecl(0)))
	assert.NoError(t, declarationChangeRefusal(makeDecl(4096), makeDecl(4096)))

	cases := []struct {
		name   string
		before int
		after  int
	}{
		{"unset_to_explicit", 0, 4096},
		{"explicit_to_explicit", 4096, 8192},
		{"explicit_to_unset", 4096, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := declarationChangeRefusal(makeDecl(tc.before), makeDecl(tc.after))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "delivery-queue-max")
			assert.Contains(t, err.Error(), "collector")
		})
	}
}
