// Design: docs/architecture/api/process-protocol.md -- outer mutating update settlement.
package server

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestUpdateRouteSocketCancellationJoinsCommandMutation(t *testing.T) {
	work, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
	t.Cleanup(stop)
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	releaseHandler := sync.OnceFunc(func() { close(release) })
	defer releaseHandler()
	dispatcher := NewDispatcher()
	require.NoError(t, dispatcher.Register("send bgp", func(ctx *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		close(entered)
		<-ctx.Context().Done()
		close(canceled)
		<-release
		return nil, ctx.Context().Err()
	}, "send bgp"))
	host := &Server{ctx: work, dispatcher: dispatcher, subscriptions: newSubscriptionManager()}
	proc := nativeOwnerProcess(t, host)
	client, engine := net.Pipe()
	peer := sdk.NewWithConn(proc.Name(), client)
	conn := plugipc.NewPluginConn(engine, engine)
	serverDone := make(chan struct{})
	go func() { host.serveRuntimeRequests(proc, conn); close(serverDone) }()
	t.Cleanup(func() {
		// Body defers/test cancellation must not expire the receiver join budget.
		join, cancelJoin := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelJoin()
		releaseHandler()
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-serverDone:
		case <-join.Done():
			t.Error("outer request receiver did not join")
		}
	})
	ctx, cancel := context.WithCancel(work)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := peer.UpdateRouteResult(ctx, "192.0.2.7", "update text nlri add 10.0.0.0/24")
		done <- err
	}()
	select {
	case <-entered:
	case <-work.Done():
		t.Fatal(work.Err())
	}
	cancel()
	select {
	case <-canceled:
	case <-work.Done():
		t.Fatal(work.Err())
	}
	select {
	case err := <-done:
		t.Fatalf("outer mutation returned before release: %v", err)
	default:
	}
	releaseHandler()
	require.ErrorIs(t, <-done, context.Canceled)
	require.NoError(t, host.Context().Err())
}

func TestUpdateRouteLegacyDeadlineKeepsOrdinaryReceiverContract(t *testing.T) {
	host := &Server{ctx: t.Context()}
	proc := nativeOwnerProcess(t, host)
	raw, err := json.Marshal(rpc.UpdateRouteInput{Command: "update text nlri add 10.0.0.0/24", PeerSelector: "192.0.2.7"})
	require.NoError(t, err)
	scope, err := updateRouteCancellationScope(host, proc, raw)
	require.NoError(t, err)
	require.Equal(t, requestScope{}, scope)
}

func TestOriginProducerContextCapturesOriginalProcess(t *testing.T) {
	host := &Server{ctx: t.Context()}
	proc := nativeOwnerProcess(t, host)
	command := &CommandContext{Server: host, Process: proc, RequestContext: t.Context(), Sender: plugin.ProcessSender(proc.Name())}
	owner, err := plugin.OriginProducerFromContext(command.OriginContext())
	require.NoError(t, err)
	original, err := proc.SelectedOwner()
	require.NoError(t, err)
	require.True(t, owner.SameIncarnation(original))
	command.Process = nil
	_, err = plugin.OriginProducerFromContext(command.OriginContext())
	require.Error(t, err, "a process name alone cannot issue origins")
}
