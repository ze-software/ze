package hub

import (
	"context"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/managed"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOwnerRelaysHubAnswer proves the owner of the daemon's hub client keeps
// what the client last reported of a fetch answer and answers it in the
// client's state, the way it relays the connection and the hold.
//
// VALIDATES: spec-fleet-7 AC-21: `show fleet status` on the daemon route
// reads held-by-hub and revert-offered from the client through its owner;
// before the client reports one, the state carries the zero HubAnswer, which
// says no fetch was answered.
// PREVENTS: a client whose report reaches no owner, so the status says
// not-fetched while the hub holds a copy.
// MUTATION: drop `client.OnHubAnswer = run.storeHubAnswer` from startWith and
// this fails.
func TestOwnerRelaysHubAnswer(t *testing.T) {
	answer := managed.HubAnswer{HeldVersion: "0123456789abcdef", Revert: managed.RevertForOther}
	reported := make(chan struct{})
	owner := newManagedClientOwner(func() (*managed.ClientConfig, error) {
		return &managed.ClientConfig{Name: "edge-01"}, nil
	}, func(*managed.ClientConfig) {})
	owner.run = func(ctx context.Context, client managed.ClientConfig) {
		client.OnConnection(true)
		client.OnHubAnswer(answer)
		close(reported)
		<-ctx.Done()
	}
	t.Cleanup(owner.shutdown)

	started, err := owner.startWith(noFlagWrite)
	require.NoError(t, err)
	require.True(t, started)
	select {
	case <-reported:
	case <-time.After(10 * time.Second):
		t.Fatal("the stand-in client never reported a fetch answer")
	}

	assert.Equal(t, managed.ClientState{Running: true, Connected: true, HubAnswer: answer}, owner.state())
}
