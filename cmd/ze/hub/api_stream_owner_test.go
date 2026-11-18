// Design: docs/architecture/api/architecture.md -- canceling an API stream joins its producer cleanup.
package hub

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestAPIStreamCancelJoinsProducerCleanup holds actual registered producer
// cleanup after cancellation. The cancel callback must not release a transport
// borrower until the producer returns and closes its event channel.
func TestAPIStreamCancelJoinsProducerCleanup(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	const name = "test api stream joined cleanup"
	observed, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(name) })
	pluginserver.RegisterStreamingHandler(name, pluginserver.OneShotSample,
		func(ctx context.Context, _ *pluginserver.Server, output io.Writer, _ string, _ command.ValidatedArgs) error {
			if _, writeErr := fmt.Fprintln(output, "started"); writeErr != nil {
				return writeErr
			}
			<-ctx.Done()
			close(observed)
			<-release
			return ctx.Err()
		})
	events, cancel, err := apiStreamSource(server)(t.Context(), api.CallerIdentity{Username: "alice"}, name)
	require.NoError(t, err)
	t.Cleanup(func() {
		unblock()
		cancel()
		for range events {
		}
	})
	require.Equal(t, "started", <-events)
	returned := make(chan struct{})
	go func() {
		cancel()
		close(returned)
	}()
	select {
	case <-observed:
	case <-time.After(3 * time.Second):
		t.Fatal("producer did not observe stream cancellation")
	}
	select {
	case <-returned:
		t.Fatal("cancel returned while its producer cleanup was still held")
	case <-time.After(200 * time.Millisecond):
		// The producer has observed cancellation but cannot return before
		// unblock; the transport must retain its borrower through this wait.
	}
	unblock()
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not join the released producer")
	}
	_, open := <-events
	require.False(t, open, "producer's event channel must be closed before cancel returns")
}
