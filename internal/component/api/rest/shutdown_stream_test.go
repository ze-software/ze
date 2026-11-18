// Design: docs/architecture/api/architecture.md -- shutdown cancels an open REST stream before joining requests.
package rest

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestRESTShutdownCancelsOpenStream reaches the real SSE loop with a provider
// that remains open. Shutdown must end it before the client's safety deadline,
// without a client cancellation being needed to release the server's join.
func TestRESTShutdownCancelsOpenStream(t *testing.T) {
	srv := testServer(t)
	srv.engine = api.NewAPIEngine(
		func(context.Context, api.CallerIdentity, string) (*plugin.Response, error) {
			return plugin.NewResponse(api.StatusDone, nil), nil
		},
		func() ([]api.CommandMeta, error) {
			return []api.CommandMeta{{Name: "monitor event", ReadOnly: true}}, nil
		}, nil,
		func(context.Context, api.CallerIdentity, string) (<-chan string, func(), error) {
			events := make(chan string, 1)
			events <- "started"
			return events, func() { close(events) }, nil
		},
	)
	serveErrors, err := srv.Start(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Shutdown(context.Background())) })
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+srv.Addresses()[0]+"/api/v1/execute/stream?command=monitor+event", http.NoBody)
	require.NoError(t, err)
	client := &http.Client{}
	t.Cleanup(client.CloseIdleConnections)
	response, err := client.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, response.Body.Close()) })
	require.Equal(t, http.StatusOK, response.StatusCode)
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "data: started\n", line)
	finished := make(chan error, 1)
	go func() {
		_, readErr := io.Copy(io.Discard, reader)
		finished <- readErr
	}()
	require.NoError(t, srv.Shutdown(t.Context()))
	require.NoError(t, <-finished, "shutdown must end its SSE request before the client deadline")
	require.NoError(t, <-serveErrors)
}
