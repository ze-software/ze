//go:build ze_web && ze_bgp

// Design: docs/architecture/api/commands.md -- shared remote stream pipes.
// Related: view_web.go -- caller binding; server_stream.go -- shared admission.

package hub

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

// TestWebMonitorPipesSharedViews reaches authenticated HTTP admission and the
// registered Model starts. Only the packet factories are synthetic: the owner
// parses parameters, formats/enriches real reply values and handles HTTP Stop.
func TestWebMonitorPipesSharedViews(t *testing.T) {
	command.SetPTRResolver(webMonitorPipeResolver{})
	command.SetOriginResolver(webMonitorPipeResolver{})
	t.Cleanup(func() { command.SetPTRResolver(nil); command.SetOriginResolver(nil) })
	for _, test := range []struct {
		name, subject, pipes, output string
	}{
		{"ping JSON", "monitor ping 192.0.2.1 interval 200ms timeout 2s count 7 size 512", "| json | log", `"seq": 4`},
		{"ping log", "monitor ping 192.0.2.1 interval 200ms timeout 2s count 7 size 512", "| log", "seq=4"},
		{"ping resolved log", "monitor ping 192.0.2.1 interval 200ms timeout 2s count 7 size 512", "| resolve | log", "192.0.2.1 pipe-target.test"},
		{"ping origin log", "monitor ping 192.0.2.1 interval 200ms timeout 2s count 7 size 512", "| origin | log", "192.0.2.1 PIPE-TEST-AS"},
		{"ping enriched log", "monitor ping 192.0.2.1 interval 200ms timeout 2s count 7 size 512", "| resolve | origin | log", "192.0.2.1 PIPE-TEST-AS"},
		{"traceroute JSON", "monitor traceroute 192.0.2.1 max-hops 8", "| json", `"addr": "192.0.2.1"`},
		{"traceroute log", "monitor traceroute 192.0.2.1 max-hops 8", "| log", "192.0.2.1"},
		{"traceroute resolved log", "monitor traceroute 192.0.2.1 max-hops 8", "| resolve | log", "192.0.2.1 pipe-target.test"},
		{"traceroute origin log", "monitor traceroute 192.0.2.1 max-hops 8", "| origin | log", "192.0.2.1 PIPE-TEST-AS"},
		{"traceroute enriched log", "monitor traceroute 192.0.2.1 max-hops 8", "| resolve | origin | log", "192.0.2.1 PIPE-TEST-AS"},
	} {
		t.Run(test.name, func(t *testing.T) {
			line := test.subject + " " + test.pipes
			probeEnded := make(chan struct{})
			var release sync.Once
			bind := func(ctx context.Context, caller plugin.CallerIdentity, model *cli.Model) error {
				assert.Equal(t, "operator", caller.Username)
				assert.IsType(t, webStreamPolicy{}, caller.Authorizer)
				assert.Equal(t, "original-request", ctx.Value(webViewContextKey{}))
				_, hasDeadline := ctx.Deadline()
				assert.True(t, hasDeadline)
				model.SetViewFactory(cli.ViewKeyPing, cli.PingFactory(func(bound context.Context, target string, interval, timeout time.Duration, count, size int) (<-chan map[string]any, context.CancelFunc, error) {
					assert.Equal(t, "original-request", bound.Value(webViewContextKey{}))
					assert.Equal(t, "192.0.2.1", target)
					assert.Equal(t, 200*time.Millisecond, interval)
					assert.Equal(t, 2*time.Second, timeout)
					assert.Equal(t, 7, count)
					assert.Equal(t, 512, size)
					replies := make(chan map[string]any, 1)
					replies <- map[string]any{"seq": 4, "status": "ok", "rtt-ms": 1.5}
					return replies, func() { release.Do(func() { close(probeEnded) }) }, nil
				}))
				rounds := 0
				model.SetViewFactory(cli.ViewKeyTraceroute, cli.TracerouteFactory(func(bound context.Context, target string, maxHops int) (<-chan map[string]any, context.CancelFunc, error) {
					assert.Equal(t, "original-request", bound.Value(webViewContextKey{}))
					assert.Equal(t, "192.0.2.1", target)
					if rounds == 0 {
						assert.Equal(t, 8, maxHops)
					}
					rounds++
					hops := make(chan map[string]any, 1)
					if rounds == 1 {
						hops <- map[string]any{"ttl": 1, "addr": "192.0.2.1", "status": "ok", "rtt-ms": 1.5}
						close(hops)
					}
					return hops, func() { release.Do(func() { close(probeEnded) }) }, nil
				}))
				return nil
			}
			server, accounting, client, base := webMonitorPipeHTTP(t, bind, webStreamPolicy{allowed: true})
			response := webMonitorPipeOpen(t, client, base, line, zeweb.CLIViewMediaType)
			defer func() { require.NoError(t, response.Body.Close()) }()
			require.Equal(t, http.StatusOK, response.StatusCode)
			decoder := json.NewDecoder(response.Body)
			var frame webMonitorPipeFrame
			for range 16 {
				require.NoError(t, decoder.Decode(&frame))
				if strings.Contains(frame.Text, test.output) {
					break
				}
			}
			require.Contains(t, frame.Text, test.output)
			switch test.pipes {
			case "| resolve | log":
				assert.NotContains(t, frame.Text, "PIPE-TEST-AS")
			case "| origin | log", "| resolve | origin | log":
				assert.NotContains(t, frame.Text, "pipe-target.test")
			}
			if strings.Contains(test.pipes, "origin") {
				assert.Contains(t, frame.Text, "PIPE-TEST-AS")
			}
			assert.Contains(t, frame.Controls, cli.BrowserViewControl{ID: "stop", Label: "Stop"})
			request := webMonitorPipeRequest(t, t.Context(), base+"/cli/view/action", url.Values{"handle": {frame.Handle}, "action": {"stop"}})
			stopped, err := client.Do(request)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, stopped.StatusCode)
			require.NoError(t, stopped.Body.Close())
			_, err = io.Copy(io.Discard, response.Body)
			require.NoError(t, err)
			select {
			case <-probeEnded:
			case <-time.After(time.Second):
				t.Fatal("HTTP Stop did not release the shared packet factory")
			}
			assert.Zero(t, server.Monitors().Count(), "rich admission must not also start a raw producer")
			assert.Equal(t, []string{line}, accounting.commands())
		})
	}
}

// TestWebMonitorPipesRefuseBeforeFactory pairs accepted chains above with
// malformed chains, remote pathname save and denied policy on both HTTP forms.
func TestWebMonitorPipesRefuseBeforeFactory(t *testing.T) {
	for _, accept := range []string{zeweb.CLIViewMediaType, "text/plain"} {
		for _, test := range []struct {
			name, suffix, want string
			allowed            bool
			status             int
		}{
			{"unknown", "| no-such-pipe", "no-such-pipe", true, http.StatusBadRequest},
			{"missing operand", "| match", "match", true, http.StatusBadRequest},
			{"save", "| save", "save", true, http.StatusBadRequest},
			{"denied", "| json", "account", false, http.StatusForbidden},
		} {
			t.Run(accept+"/"+test.name, func(t *testing.T) {
				bind := func(context.Context, plugin.CallerIdentity, *cli.Model) error {
					t.Error("refusal opened the view binding")
					return nil
				}
				server, accounting, client, base := webMonitorPipeHTTP(t, bind, webStreamPolicy{allowed: test.allowed})
				path := filepath.Join(t.TempDir(), "must-not-exist.json")
				suffix := test.suffix
				if test.name == "save" {
					suffix += " " + path
				}
				line := "monitor ping 192.0.2.1 " + suffix
				response := webMonitorPipeOpen(t, client, base, line, accept)
				defer func() { require.NoError(t, response.Body.Close()) }()
				body, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				assert.Equal(t, test.status, response.StatusCode, string(body))
				assert.Contains(t, string(body), test.want)
				assert.NotContains(t, string(body), `"handle"`)
				assert.Zero(t, server.Monitors().Count())
				assert.Equal(t, []string{line}, accounting.commands())
				_, err = os.Stat(path)
				assert.ErrorIs(t, err, os.ErrNotExist)
			})
		}
	}
}

// TestWebMonitorPipesRawFilters reaches the real event producer through HTTP.
// Its subscription barrier precedes delivery; a filtered-out event followed by
// a retained event proves useful filtering rather than unconditional refusal.
func TestWebMonitorPipesRawFilters(t *testing.T) {
	server, accounting, client, base := webMonitorPipeHTTP(t, nil, webStreamPolicy{allowed: true})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	line := "monitor event | raw | match kept"
	request := webMonitorPipeRequest(t, ctx, base+"/cli/stream", url.Values{"command": {line}, "mode": {"operational"}})
	request.Header.Set("Accept", "text/plain")
	type opened struct {
		response *http.Response
		err      error
	}
	result := make(chan opened, 1)
	go func() {
		response, err := client.Do(request)
		if response != nil {
			t.Cleanup(func() { require.NoError(t, response.Body.Close()) })
		}
		result <- opened{response, err}
	}()
	joined := false
	t.Cleanup(func() {
		cancel()
		if !joined {
			select {
			case result := <-result:
				if result.response != nil {
					require.NoError(t, result.response.Body.Close())
				}
			case <-time.After(time.Second):
				t.Error("HTTP open did not join")
			}
		}
	})
	require.Eventually(t, func() bool { return server.Monitors().Count() == 1 }, time.Second, time.Millisecond,
		"piped raw monitor never reached its registered producer")
	server.Monitors().Deliver("bgp", "update", "received", "192.0.2.1", "sample", "discarded-event")
	server.Monitors().Deliver("bgp", "update", "received", "192.0.2.1", "sample", "kept-event")
	var response *http.Response
	select {
	case answer := <-result:
		joined = true
		require.NoError(t, answer.err)
		response = answer.response
	case <-time.After(time.Second):
		t.Fatal("retained event never reached HTTP")
	}
	defer func() { require.NoError(t, response.Body.Close()) }()
	require.Equal(t, http.StatusOK, response.StatusCode)
	text, err := bufio.NewReader(response.Body).ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "kept-event\n", text)
	cancel()
	require.Eventually(t, func() bool { return server.Monitors().Count() == 0 }, time.Second, time.Millisecond)
	require.Eventually(t, func() bool { return len(accounting.commands()) == 1 }, time.Second, time.Millisecond)
	assert.Equal(t, []string{line}, accounting.commands())
}

// TestWebMonitorPipesCancellation opens the same piped producer before ending
// either owning lifetime. A disconnect and daemon shutdown must release it;
// a daemon shutdown also travels as an explicit failure after HTTP starts.
func TestWebMonitorPipesCancellation(t *testing.T) {
	for _, owner := range []string{"request", "daemon"} {
		t.Run(owner, func(t *testing.T) {
			ended := make(chan struct{})
			var once sync.Once
			bind := func(_ context.Context, _ plugin.CallerIdentity, model *cli.Model) error {
				model.SetViewFactory(cli.ViewKeyPing, cli.PingFactory(func(context.Context, string, time.Duration, time.Duration, int, int) (<-chan map[string]any, context.CancelFunc, error) {
					return make(chan map[string]any), func() { once.Do(func() { close(ended) }) }, nil
				}))
				return nil
			}
			server, accounting, client, base := webMonitorPipeHTTP(t, bind, webStreamPolicy{allowed: true})
			line := "monitor ping 192.0.2.1 | json | log"
			response := webMonitorPipeOpen(t, client, base, line, zeweb.CLIViewMediaType)
			require.Equal(t, http.StatusOK, response.StatusCode)
			var first webMonitorPipeFrame
			require.NoError(t, json.NewDecoder(response.Body).Decode(&first))
			require.NotEmpty(t, first.Handle)
			if owner == "request" {
				require.NoError(t, response.Body.Close())
			} else {
				server.Stop()
				body, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				assert.Contains(t, string(body), "daemon stopped")
			}
			select {
			case <-ended:
			case <-time.After(time.Second):
				t.Fatal("canceled piped monitor retained its producer")
			}
			require.Eventually(t, func() bool { return len(accounting.commands()) == 1 }, time.Second, time.Millisecond)
			assert.Equal(t, []string{line}, accounting.commands())
		})
	}
}

type webMonitorPipeFrame struct {
	Handle string `json:"handle"`
	cli.BrowserViewFrame
}

type webMonitorPipeResolver struct{}

func (webMonitorPipeResolver) ResolvePTR(string) ([]string, error) {
	return []string{"pipe-target.test."}, nil
}
func (webMonitorPipeResolver) LookupOrigin(context.Context, string) (command.OriginResult, error) {
	return command.OriginResult{ASN: 64512, Prefix: "192.0.2.0/24", Name: "PIPE-TEST-AS"}, nil
}

// webMonitorPipeAccounting retains completed commands under a mutex because
// cancellation and HTTP completion may be observed on different test workers.
type webMonitorPipeAccounting struct {
	mu            sync.Mutex
	starts, stops []string
}

func (a *webMonitorPipeAccounting) CommandStart(username, remote, input string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.starts = append(a.starts, username+"|"+remote+"|"+input)
	return input
}
func (a *webMonitorPipeAccounting) CommandStop(_, username, remote, input string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stops = append(a.stops, username+"|"+remote+"|"+input)
}
func (a *webMonitorPipeAccounting) commands() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var completed []string
	for i, stop := range a.stops {
		if i >= len(a.starts) {
			return nil
		}
		if stop != a.starts[i] {
			return nil
		}
		parts := strings.SplitN(stop, "|", 3)
		if parts[0] != "operator" {
			return nil
		}
		if !strings.HasPrefix(parts[1], "127.0.0.1:") {
			return nil
		}
		completed = append(completed, parts[2])
	}
	return completed
}

func webMonitorPipeHTTP(t *testing.T, bind zeweb.CLIViewBinding, policy plugin.Authorizer) (*pluginserver.Server, *webMonitorPipeAccounting, *http.Client, string) {
	t.Helper()
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	require.NoError(t, server.StartWithContext(t.Context()))
	accounting := &webMonitorPipeAccounting{}
	server.Dispatcher().SetAccountingHook(accounting)
	server.Dispatcher().SetAuthorizer(webStreamPolicy{allowed: false})
	views := zeweb.NewCLIViewTransport(serverStreamAdmission(server), bind)
	mux := http.NewServeMux()
	mux.Handle("POST /cli/stream", views.Handler(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("monitor reached finite command handler") }), serverStreamDispatcher(server)))
	mux.HandleFunc("POST /cli/view/action", views.Action)
	store := zeweb.NewSessionStore(func() ([]authz.UserConfig, error) { return nil, nil })
	handler := zeweb.AuthMiddlewareWithAudit(store, completionAuthenticator{policy}, func(http.ResponseWriter, *http.Request) {}, zeweb.RequireSameOrigin(mux), nil)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		ctx = context.WithValue(ctx, webViewContextKey{}, "original-request")
		handler.ServeHTTP(w, r.WithContext(ctx))
	}))
	t.Cleanup(httpServer.Close)
	client := httpServer.Client()
	client.Timeout = 6 * time.Second
	t.Cleanup(client.CloseIdleConnections)
	return server, accounting, client, httpServer.URL
}

func webMonitorPipeRequest(t *testing.T, ctx context.Context, endpoint string, form url.Values) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	require.NoError(t, err)
	request.SetBasicAuth("operator", "synthetic")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	parsed, err := url.Parse(endpoint)
	require.NoError(t, err)
	request.Header.Set("Origin", parsed.Scheme+"://"+parsed.Host)
	return request
}

func webMonitorPipeOpen(t *testing.T, client *http.Client, base, line, accept string) *http.Response {
	t.Helper()
	request := webMonitorPipeRequest(t, t.Context(), base+"/cli/stream", url.Values{"command": {line}, "mode": {"operational"}})
	request.Header.Set("Accept", accept)
	response, err := client.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, response.Body.Close()) })
	return response
}
