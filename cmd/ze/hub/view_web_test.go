//go:build ze_web && ze_bgp

// Design: docs/architecture/web-interface.md -- original monitor admission and caller binding.

package hub

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestWebViewAdmissionChecksOriginalMonitor exercises shared registered stream
// admission without invoking its raw producer. Refused commands never open a
// view; admitted runs keep the original caller and balanced accounting.
func TestWebViewAdmissionChecksOriginalMonitor(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	require.NoError(t, server.StartWithContext(t.Context()))
	accounting := &webStreamAccounting{}
	server.Dispatcher().SetAccountingHook(accounting)
	server.Dispatcher().SetAuthorizer(webStreamPolicy{allowed: false})
	admit := serverStreamAdmission(server)
	runs := 0
	caller := plugin.CallerIdentity{Username: "alice", RemoteAddr: "192.0.2.25:1234", Authorizer: webStreamPolicy{allowed: false}}
	run := func(ctx context.Context, accepted plugin.CallerIdentity) error {
		runs++
		assert.Equal(t, caller, accepted)
		assert.NoError(t, ctx.Err())
		assert.Zero(t, server.Monitors().Count(), "admission must never start the raw monitor")
		return nil
	}
	handled, err := admit(t.Context(), caller, "monitor bgp", run)
	assert.True(t, handled)
	assert.ErrorIs(t, err, plugin.ErrStreamUnauthorized)
	assert.Zero(t, runs)
	caller.Authorizer = webStreamPolicy{allowed: true}
	handled, err = admit(t.Context(), caller, "monitor bgp", run)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, 1, runs)
	handled, err = admit(t.Context(), caller, "monitor bgp | match", run)
	assert.True(t, handled)
	assert.ErrorContains(t, err, "pipe")
	assert.Equal(t, 1, runs)
	handled, err = admit(t.Context(), caller, "monitor ping", run)
	assert.True(t, handled)
	assert.Error(t, err, "a missing target must not open a factory")
	assert.Equal(t, 1, runs)
	handled, err = admit(t.Context(), caller, "show version", run)
	assert.False(t, handled)
	require.NoError(t, err)
	assert.Equal(t, 1, runs)
	assert.Len(t, accounting.starts, 4)
	assert.Equal(t, accounting.starts, accounting.stops)
}

// TestWebViewBindingRetainsCallerForPollAndDetail drives the actual shared
// registered Model through the hub binding. Both RPCs inherit the original
// request values, deadline and policy, and Stop joins the driver.
func TestWebViewBindingRetainsCallerForPollAndDetail(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, webViewContextKey{}, "original-request")
	caller := plugin.CallerIdentity{Username: "alice", RemoteAddr: "192.0.2.25:1234", Authorizer: webStreamPolicy{allowed: true}}
	inputs := make(chan string, 8)
	dispatch := plugin.CommandDispatcher(func(bound context.Context, accepted plugin.CallerIdentity, input string) (*plugin.Response, error) {
		assert.Equal(t, "original-request", bound.Value(webViewContextKey{}))
		assert.Equal(t, caller, accepted)
		deadline, found := bound.Deadline()
		original, originalFound := ctx.Deadline()
		assert.True(t, found)
		assert.True(t, originalFound)
		assert.Equal(t, original, deadline)
		inputs <- input
		if input == "show bgp" {
			return plugin.NewResponse(plugin.StatusDone, plugin.Map{"router-id": "192.0.2.1", "peers": []any{map[string]any{"address": "10.0.0.9", "remote-as": 9}}}), nil
		}
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"peers": map[string]any{"10.0.0.9": map[string]any{"description": "bound-peer"}}}), nil
	})
	actions := make(chan cli.BrowserViewAction, cli.BrowserViewActionQueueMax)
	frames := make(chan cli.BrowserViewFrame, 1)
	done := make(chan error, 1)
	joined := false
	go func() {
		done <- cli.RunBrowserView(ctx, "monitor bgp", func(bound context.Context, model *cli.Model) error {
			return webViewBinding(dispatch)(bound, caller, model)
		}, actions, frames)
	}()
	t.Cleanup(func() {
		cancel()
		if !joined {
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("browser driver did not join")
			}
		}
	})
	waitFrame := func(matches func(cli.BrowserViewFrame) bool) {
		for range 16 {
			select {
			case frame := <-frames:
				if matches(frame) {
					return
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
		t.Fatal("browser view did not reach requested state")
	}
	waitFrame(func(frame cli.BrowserViewFrame) bool { return strings.Contains(frame.Text, "10.0.0.9") })
	actions <- cli.BrowserViewAction{ID: "detail"}
	waitFrame(func(frame cli.BrowserViewFrame) bool {
		for _, control := range frame.Controls {
			if control.ID == "back" {
				return true
			}
		}
		return false
	})
	assert.Equal(t, "show bgp", <-inputs)
	assert.Equal(t, "show bgp peer 10.0.0.9 detail", <-inputs)
	actions <- cli.BrowserViewAction{ID: "stop"}
	select {
	case err := <-done:
		joined = true
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

// TestWebViewAdmissionCancellation follows both lifetime owners through shared
// admission and requires a useful daemon-stop refusal or the original request
// cancellation, with no raw monitor subscription opened.
func TestWebViewAdmissionCancellation(t *testing.T) {
	for _, lifetime := range []string{"request", "daemon"} {
		t.Run(lifetime, func(t *testing.T) {
			daemonContext, stopDaemon := context.WithCancel(t.Context())
			defer stopDaemon()
			server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
			require.NoError(t, err)
			t.Cleanup(server.Stop)
			require.NoError(t, server.StartWithContext(daemonContext))
			requestContext, stopRequest := context.WithCancel(t.Context())
			defer stopRequest()
			handled, err := serverStreamAdmission(server)(requestContext, plugin.CallerIdentity{Username: "alice", Authorizer: webStreamPolicy{allowed: true}}, "monitor bgp", func(ctx context.Context, _ plugin.CallerIdentity) error {
				if lifetime == "request" {
					stopRequest()
				} else {
					stopDaemon()
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second):
					t.Error("admitted lifetime was not canceled")
					return nil
				}
			})
			assert.True(t, handled)
			if lifetime == "request" {
				assert.ErrorIs(t, err, context.Canceled)
			} else {
				assert.ErrorContains(t, err, "daemon stopped")
			}
			assert.Zero(t, server.Monitors().Count())
		})
	}
}

// TestWebViewFactoriesCoverRegistry runs in web builds, including builds with
// SSH removed. Every offered browser view must have a concrete shared factory.
func TestWebViewFactoriesCoverRegistry(t *testing.T) {
	model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
	t.Cleanup(func() { require.NoError(t, model.Close()) })
	injectViewFactories(&model, nil)
	for _, view := range cli.RegisteredViews() {
		if !view.Browser {
			continue
		}
		factory, found := model.ViewFactory(view.Key)
		assert.True(t, found, view.Prefix)
		assert.NotNil(t, factory, view.Prefix)
	}
}

type webViewContextKey struct{}
