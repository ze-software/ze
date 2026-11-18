// Design: docs/architecture/l2tp/subscriber-session-model.md -- registered command original-owner mutation
//go:build linux

package l2tp_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	_ "github.com/ze-software/ze/internal/component/l2tp/cmd"
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all" // shipped schema/RPC composition for the daemon server
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func captureActionDispatcher(t *testing.T, subsystem *l2tp.Subsystem) func(string) *plugin.Response {
	t.Helper()
	l2tp.PublishService(subsystem)
	t.Cleanup(func() { l2tp.PublishService(nil) })
	server, err := pluginserver.NewServer(nil, nil)
	require.NoError(t, err)
	dispatcher := server.Dispatcher()
	return func(line string) *plugin.Response {
		answer, err := dispatcher.Dispatch(&pluginserver.CommandContext{RequestContext: t.Context(), Sender: plugin.OperatorSender()}, line)
		require.NoError(t, err)
		require.NotNil(t, answer)
		return answer
	}
}

func TestCapturedSessionRegisteredCommandConfirmsCleanupAndRefusesOldBirth(t *testing.T) {
	subsystem, original, admit, observe := l2tp.SessionCaptureDispatchFixtureForTest(t)
	dispatch := captureActionDispatcher(t, subsystem)
	line := fmt.Sprintf("clear l2tp session id 7 expected-id %s", original.Birth.String())
	answer := dispatch(line)
	require.Equal(t, plugin.StatusDone, answer.Status, answer.Error)
	resources, deletes, closes := observe()
	require.Zero(t, resources, "registered command success must confirm its original native cleanup")
	require.Equal(t, 1, deletes)
	require.Equal(t, 3, closes)
	replacement := admit()
	require.NotEqual(t, original.Birth, replacement.Birth)
	answer = dispatch(line)
	require.Equal(t, plugin.StatusError, answer.Status, "the old browser guard cannot clear the reused row")
	resources, deletes, closes = observe()
	require.Equal(t, 1, resources)
	require.Equal(t, 1, deletes)
	require.Equal(t, 3, closes)
	answer = dispatch(fmt.Sprintf("clear l2tp session id 7 expected-id %s", replacement.Birth.String()))
	require.Equal(t, plugin.StatusDone, answer.Status, answer.Error)
	resources, deletes, closes = observe()
	require.Zero(t, resources)
	require.Equal(t, 2, deletes)
	require.Equal(t, 6, closes)
	admit()
	answer = dispatch("clear l2tp session id 7")
	require.Equal(t, plugin.StatusDone, answer.Status, "interactive guard remains optional: %s", answer.Error)
	resources, deletes, closes = observe()
	require.Zero(t, resources)
	require.Equal(t, 3, deletes)
	require.Equal(t, 9, closes)
}

func TestCapturedSessionRegisteredCommandRefusesMalformedBirthBeforeMutation(t *testing.T) {
	for _, input := range []string{"invalid", "l2tp-00000000000000000000000000000000", "L2TP-00000000000000000000000000000001"} {
		t.Run(input, func(t *testing.T) {
			subsystem, _, _, observe := l2tp.SessionCaptureDispatchFixtureForTest(t)
			dispatch := captureActionDispatcher(t, subsystem)
			answer := dispatch("clear l2tp session id 7 expected-id " + input)
			require.Equal(t, plugin.StatusError, answer.Status)
			resources, deletes, closes := observe()
			require.Equal(t, 1, resources)
			require.Zero(t, deletes)
			require.Zero(t, closes)
		})
	}
}
