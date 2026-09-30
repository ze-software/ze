// VALIDATES: ze.l2tp.cqm.echo-interval is a registered env key with default
// "1s", and cqmEchoInterval reads it: the override applies, an invalid value
// falls back to 1s, and a disabled CQM asks for no override.
// PREVENTS: the subsystem start path reading an unregistered key. env.Get
// aborts the daemon with "FATAL: env.Get called with unregistered key" before
// any tunnel comes up, which is how test/l2tp/radius-acct-wire.ci failed with
// "no SCCRP received".

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
)

const cqmEchoIntervalKey = "ze.l2tp.cqm.echo-interval"

// TestCQMEchoIntervalRegistered checks the registry entry itself, so the key
// is known before any Get can reach mustBeRegistered and exit the process.
func TestCQMEchoIntervalRegistered(t *testing.T) {
	require.True(t, env.IsRegistered(cqmEchoIntervalKey), "%s must be registered", cqmEchoIntervalKey)

	var entry env.EnvEntry
	found := false
	for _, e := range env.AllEntries() {
		if e.Key == cqmEchoIntervalKey {
			entry = e
			found = true
			break
		}
	}
	require.True(t, found, "%s missing from env.AllEntries", cqmEchoIntervalKey)
	assert.Equal(t, "1s", entry.Default)
	assert.Equal(t, "duration", entry.Type)
	assert.NotEmpty(t, entry.Description)
}

// TestCQMEchoIntervalReadsEnv drives cqmEchoInterval through the env cache.
// The registration check comes first, so at a tree where the key is absent
// this test fails instead of calling env.Get and exiting the test binary.
func TestCQMEchoIntervalReadsEnv(t *testing.T) {
	require.True(t, env.IsRegistered(cqmEchoIntervalKey), "%s must be registered", cqmEchoIntervalKey)

	enabled := NewSubsystem(Parameters{CQMEnabled: true})
	disabled := NewSubsystem(Parameters{CQMEnabled: false})

	t.Run("unset uses 1s", func(t *testing.T) {
		useEnv(t, map[string]string{cqmEchoIntervalKey: ""})
		assert.Equal(t, time.Second, enabled.cqmEchoInterval())
	})
	t.Run("override applies", func(t *testing.T) {
		useEnv(t, map[string]string{cqmEchoIntervalKey: "250ms"})
		assert.Equal(t, 250*time.Millisecond, enabled.cqmEchoInterval())
	})
	t.Run("invalid falls back to 1s", func(t *testing.T) {
		useEnv(t, map[string]string{cqmEchoIntervalKey: "fast"})
		assert.Equal(t, time.Second, enabled.cqmEchoInterval())
	})
	t.Run("non-positive falls back to 1s", func(t *testing.T) {
		useEnv(t, map[string]string{cqmEchoIntervalKey: "-2s"})
		assert.Equal(t, time.Second, enabled.cqmEchoInterval())
	})
	t.Run("cqm disabled asks for no override", func(t *testing.T) {
		useEnv(t, map[string]string{cqmEchoIntervalKey: "250ms"})
		assert.Equal(t, time.Duration(0), disabled.cqmEchoInterval())
	})
}
