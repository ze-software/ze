// Design: docs/guide/l2tp.md -- registered original-session selection qualifiers
//go:build linux

package l2tp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestCapturedSessionRegisteredCommandRefusesWrongQualifiers proves a supplied
// selector is never ignored: the actual original resources must remain owned.
func TestCapturedSessionRegisteredCommandRefusesWrongQualifiers(t *testing.T) {
	for _, suffix := range []string{"tunnel-id 999", "listener 192.0.2.99:1701", "listener unknown"} {
		t.Run(suffix, func(t *testing.T) {
			subsystem, _, _, observe := l2tp.SessionCaptureDispatchFixtureForTest(t)
			dispatch := captureActionDispatcher(t, subsystem)
			answer := dispatch("clear l2tp session id 7 " + suffix)
			require.Equal(t, plugin.StatusError, answer.Status)
			resources, deletes, closes := observe()
			require.Equal(t, 1, resources)
			require.Zero(t, deletes)
			require.Zero(t, closes)
		})
	}
}

// TestCapturedSessionRegisteredCommandAcceptsOriginalEndpoint drives the real
// command with its actual bound socket and tunnel, requiring physical cleanup.
func TestCapturedSessionRegisteredCommandAcceptsOriginalEndpoint(t *testing.T) {
	subsystem, original, _, observe := l2tp.SessionCaptureDispatchFixtureForTest(t)
	listeners := subsystem.Listeners()
	require.Len(t, listeners, 1)
	dispatch := captureActionDispatcher(t, subsystem)
	answer := dispatch("clear l2tp session id 7 tunnel-id 100 listener " + listeners[0].Addr.String() + " expected-id " + original.Birth.String())
	require.Equal(t, plugin.StatusDone, answer.Status, answer.Error)
	resources, deletes, closes := observe()
	require.Zero(t, resources)
	require.Equal(t, 1, deletes)
	require.Equal(t, 3, closes)
}

// TestCapturedSessionRegisteredCommandRefusesInvalidDetailsBeforeMutation
// exercises malformed audit/keyword inputs through the real dispatcher/actor.
func TestCapturedSessionRegisteredCommandRefusesInvalidDetailsBeforeMutation(t *testing.T) {
	for _, suffix := range []string{"cause invalid", "cause 4294967296", "cause", "reason", "actor", "unknown value", "actor alice actor bob"} {
		t.Run(suffix, func(t *testing.T) {
			subsystem, _, _, observe := l2tp.SessionCaptureDispatchFixtureForTest(t)
			dispatch := captureActionDispatcher(t, subsystem)
			answer := dispatch("clear l2tp session id 7 " + suffix)
			require.Equal(t, plugin.StatusError, answer.Status)
			resources, deletes, closes := observe()
			require.Equal(t, 1, resources)
			require.Zero(t, deletes)
			require.Zero(t, closes)
		})
	}
}
