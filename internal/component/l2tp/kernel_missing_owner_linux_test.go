// Design: docs/architecture/l2tp/subscriber-session-model.md -- missing native authority refusal
//go:build linux

package l2tp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKernelSetupWithoutOriginalRowRefusesNativeAcquisition(t *testing.T) {
	worker, fake, event := workerCleanupBaseline(t)
	event.session = nil
	worker.setupSession(event)
	require.Empty(t, worker.sessions)
	require.Empty(t, fake.tunnelCreated)
	require.Empty(t, fake.sessionAdded)
	require.Empty(t, fake.pppSetups)
}

func TestKernelTeardownWithoutOriginalRowPreservesCurrentResources(t *testing.T) {
	worker, fake, event := workerCleanupBaseline(t)
	worker.setupSession(event)
	key := sessionKey{event.localTID, event.localSID}
	owner := worker.sessions[key]
	require.NotNil(t, owner)
	t.Cleanup(worker.teardownAll)
	worker.teardownSession(kernelTeardownEvent{localTID: event.localTID, localSID: event.localSID})
	require.Same(t, owner, worker.sessions[key])
	require.Empty(t, fake.closes)
	require.Empty(t, fake.sessionGone)
}
