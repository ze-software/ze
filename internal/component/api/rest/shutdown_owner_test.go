// Design: docs/architecture/api/architecture.md -- REST shutdown joins active command requests.
package rest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestRESTShutdownJoinsActiveCommand holds an accepted command in a real HTTP
// request. Both ordinary and canceled-grace shutdown must join that request;
// the canceled case must still report the original cancellation afterward.
func TestRESTShutdownJoinsActiveCommand(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "clean grace"
		if canceled {
			name = "canceled grace"
		}
		t.Run(name, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			srv := testServer(t)
			srv.engine = api.NewAPIEngine(
				func(context.Context, api.CallerIdentity, string) (*plugin.Response, error) {
					close(entered)
					<-release
					return plugin.NewResponse(api.StatusDone, plugin.Map{"finished": true}), nil
				},
				func() ([]api.CommandMeta, error) {
					return []api.CommandMeta{{Name: "show version", ReadOnly: true}}, nil
				},
				func(_, _ string) bool { return true }, nil,
			)
			serveErrors, err := srv.Start(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, srv.Shutdown(context.Background())) })
			t.Cleanup(unblock)
			request, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
				"http://"+srv.Addresses()[0]+"/api/v1/execute", strings.NewReader(`{"command":"show version"}`))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 3 * time.Second}
			t.Cleanup(client.CloseIdleConnections)
			type requestResult struct {
				status int
				body   []byte
				err    error
			}
			completed := make(chan requestResult, 1)
			go func() {
				response, requestErr := client.Do(request)
				if requestErr != nil {
					completed <- requestResult{err: requestErr}
					return
				}
				body, readErr := io.ReadAll(response.Body)
				closeErr := response.Body.Close()
				if readErr == nil {
					readErr = closeErr
				}
				completed <- requestResult{status: response.StatusCode, body: body, err: readErr}
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("HTTP request did not reach its accepted command")
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if canceled {
				cancel()
			}
			stopped := make(chan error, 1)
			go func() { stopped <- srv.Shutdown(ctx) }()
			select {
			case stopErr := <-stopped:
				t.Fatalf("REST shutdown returned while its accepted command was held: %v", stopErr)
			case <-time.After(200 * time.Millisecond):
				// The command cannot finish until unblock; a canceled grace
				// must report its error only after that borrower has finished.
			}
			unblock()
			select {
			case stopErr := <-stopped:
				if canceled {
					require.ErrorIs(t, stopErr, context.Canceled)
				} else {
					require.NoError(t, stopErr)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("REST shutdown did not return after its command finished")
			}
			result := <-completed
			require.NoError(t, result.err)
			require.Equal(t, http.StatusOK, result.status)
			require.Contains(t, string(result.body), `"finished":true`)
			require.NoError(t, <-serveErrors)
		})
	}
}
