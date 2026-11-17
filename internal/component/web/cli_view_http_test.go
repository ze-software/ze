// Design: docs/architecture/web-interface.md -- interactive browser monitor transport.

package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebCLIViewNegotiatesControls asks the actual terminal stream entry point
// for an interactive monitor and requires a framed answer with owner controls.
// MUTATION: ignore the rich Accept type; this sees the text stream MIME instead.
func TestWebCLIViewNegotiatesControls(t *testing.T) {
	client, base, _ := cliViewTestServer(t, cliViewTestAdmission, cliViewTestBinding)
	response, decoder := cliViewTestOpen(t, client, base, "testuser")
	t.Cleanup(func() { require.NoError(t, response.Body.Close()) })
	require.Equal(t, http.StatusOK, response.StatusCode)
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "application/x-ze-cli-view+ndjson", media)
	var frame cliViewFrame
	require.NoError(t, decoder.Decode(&frame))
	assert.NotEmpty(t, frame.Handle)
	assert.Positive(t, frame.Sequence)
	assert.NotEmpty(t, frame.Text)
	assert.Contains(t, frame.Controls, cli.BrowserViewControl{ID: "stop", Label: "Stop"})
	cliViewTestAction(t, client, base, "testuser", frame.Handle, "stop", http.StatusNoContent)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
}

// TestWebCLIViewDeniedBeforeHandle checks the negative half of negotiation:
// command refusal must not publish a handle, control frame or successful MIME.
func TestWebCLIViewDeniedBeforeHandle(t *testing.T) {
	views := NewCLIViewTransport(func(context.Context, plugin.CallerIdentity, string, func(context.Context, plugin.CallerIdentity) error) (bool, error) {
		return true, plugin.ErrStreamUnauthorized
	}, func(context.Context, plugin.CallerIdentity, *cli.Model) error {
		t.Error("denied monitor opened a factory")
		return nil
	})
	handler := views.Handler(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("denied monitor reached the finite terminal")
	}), func(context.Context, plugin.CallerIdentity, string, io.Writer) (bool, error) {
		t.Error("rich refusal reached the raw producer")
		return false, nil
	})
	request := authedRequest(t, http.MethodPost, "/cli/stream", url.Values{
		"command": {"monitor bgp"}, "mode": {terminalModeOperational},
	})
	request.Header.Set("Accept", "application/x-ze-cli-view+ndjson")
	response := httptest.NewRecorder()
	handler(response, request)
	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.NotContains(t, response.Header().Get("Content-Type"), "application/x-ze-cli-view+ndjson")
	assert.NotContains(t, response.Body.String(), `"handle"`)
	assert.NotContains(t, response.Body.String(), `"controls"`)
	assert.Empty(t, views.active)
}

// TestWebCLIViewActionsTrackRequestLifetime follows actual frames and HTTP
// controls through sort/selection/detail/back/Stop, including a second tab and
// wrong-user, undeclared and stale actions. The shared Model owns all peer state.
func TestWebCLIViewActionsTrackRequestLifetime(t *testing.T) {
	details := make(chan string, 2)
	bind := func(ctx context.Context, caller plugin.CallerIdentity, model *cli.Model) error {
		if err := cliViewTestBinding(ctx, caller, model); err != nil {
			return err
		}
		model.SetCommandExecutor(func(input string) (cli.CommandOutput, error) {
			details <- input
			return cli.CommandOutput{Text: `{"peers":{"10.0.0.10":{"description":"selected-peer"}}}`}, nil
		})
		return nil
	}
	client, base, views := cliViewTestServer(t, cliViewTestAdmission, bind)
	first, decoder := cliViewTestOpen(t, client, base, "alice")
	t.Cleanup(func() { require.NoError(t, first.Body.Close()) })
	frame := cliViewTestFrame(t, decoder, func(frame cliViewFrame) bool { return strings.Contains(frame.Text, "10.0.0.10") })
	assert.Less(t, strings.Index(frame.Text, "10.0.0.9"), strings.Index(frame.Text, "10.0.0.10"))
	second, otherDecoder := cliViewTestOpen(t, client, base, "alice")
	t.Cleanup(func() { require.NoError(t, second.Body.Close()) })
	other := cliViewTestFrame(t, otherDecoder, func(frame cliViewFrame) bool { return frame.Handle != "" })
	require.NotEqual(t, frame.Handle, other.Handle)
	cliViewTestAction(t, client, base, "bob", frame.Handle, "stop", http.StatusForbidden)
	cliViewTestAction(t, client, base, "alice", frame.Handle, "invented", http.StatusBadRequest)
	for _, action := range []string{"next", "sort", "reverse"} {
		sequence := frame.Sequence
		cliViewTestAction(t, client, base, "alice", frame.Handle, action, http.StatusNoContent)
		frame = cliViewTestFrame(t, decoder, func(frame cliViewFrame) bool { return frame.Sequence > sequence })
	}
	cliViewTestAction(t, client, base, "alice", frame.Handle, "detail", http.StatusNoContent)
	frame = cliViewTestFrame(t, decoder, func(frame cliViewFrame) bool {
		for _, control := range frame.Controls {
			if control.ID == "back" {
				return true
			}
		}
		return false
	})
	select {
	case input := <-details:
		assert.Equal(t, "show bgp peer 10.0.0.10 detail", input)
	case <-t.Context().Done():
		t.Fatal("detail never reached the selected peer")
	}
	cliViewTestAction(t, client, base, "alice", frame.Handle, "back", http.StatusNoContent)
	cliViewTestFrame(t, decoder, func(frame cliViewFrame) bool {
		for _, control := range frame.Controls {
			if control.ID == "next" {
				return true
			}
		}
		return false
	})
	cliViewTestAction(t, client, base, "alice", frame.Handle, "stop", http.StatusNoContent)
	_, err := io.Copy(io.Discard, first.Body)
	require.NoError(t, err)
	cliViewTestAction(t, client, base, "alice", frame.Handle, "stop", http.StatusGone)
	cliViewTestAction(t, client, base, "alice", other.Handle, "stop", http.StatusNoContent)
	_, err = io.Copy(io.Discard, second.Body)
	require.NoError(t, err)
	views.mu.Lock()
	defer views.mu.Unlock()
	assert.Empty(t, views.active)
}

// TestWebCLIViewStopInterruptsBusyDetail queues another action behind a blocked
// context-bound detail RPC, then proves HTTP Stop cancels and joins that work.
func TestWebCLIViewStopInterruptsBusyDetail(t *testing.T) {
	detailStarted := make(chan struct{})
	detailStopped := make(chan struct{})
	bind := func(ctx context.Context, caller plugin.CallerIdentity, model *cli.Model) error {
		if err := cliViewTestBinding(ctx, caller, model); err != nil {
			return err
		}
		model.SetCommandExecutor(func(string) (cli.CommandOutput, error) {
			close(detailStarted)
			<-ctx.Done()
			close(detailStopped)
			return cli.CommandOutput{}, ctx.Err()
		})
		return nil
	}
	client, base, views := cliViewTestServer(t, cliViewTestAdmission, bind)
	response, decoder := cliViewTestOpen(t, client, base, "alice")
	t.Cleanup(func() { require.NoError(t, response.Body.Close()) })
	frame := cliViewTestFrame(t, decoder, func(frame cliViewFrame) bool { return strings.Contains(frame.Text, "10.0.0.10") })
	cliViewTestAction(t, client, base, "alice", frame.Handle, "detail", http.StatusNoContent)
	select {
	case <-detailStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("detail did not start")
	}
	cliViewTestAction(t, client, base, "alice", frame.Handle, "reverse", http.StatusNoContent)
	cliViewTestAction(t, client, base, "alice", frame.Handle, "stop", http.StatusNoContent)
	_, err := io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	select {
	case <-detailStopped:
	default:
		t.Fatal("Stop returned without joining the detail RPC")
	}
	views.mu.Lock()
	defer views.mu.Unlock()
	assert.Empty(t, views.active)
}

// TestWebCLIViewAdmissionBounds refuses resource exhaustion and keeps Stop
// effective when all eight queued action slots are occupied.
func TestWebCLIViewAdmissionBounds(t *testing.T) {
	views := NewCLIViewTransport(cliViewTestAdmission, cliViewTestBinding)
	for range cliViewHandlesMax {
		_, request, err := views.add(t.Context(), plugin.CallerIdentity{Username: "testuser"})
		require.NoError(t, err)
		t.Cleanup(request.cancel)
	}
	_, _, err := views.add(t.Context(), plugin.CallerIdentity{Username: "testuser"})
	require.ErrorContains(t, err, "Too many")
	views = NewCLIViewTransport(cliViewTestAdmission, cliViewTestBinding)
	handle, request, err := views.add(t.Context(), plugin.CallerIdentity{Username: "testuser"})
	require.NoError(t, err)
	defer request.cancel()
	request.controls = []cli.BrowserViewControl{{ID: "next", Label: "Next"}, {ID: "stop", Label: "Stop"}}
	for range cli.BrowserViewActionQueueMax {
		request.actions <- cli.BrowserViewAction{ID: "next"}
	}
	action := func(id string) int {
		response := httptest.NewRecorder()
		views.Action(response, authedRequest(t, http.MethodPost, "/cli/view/action", url.Values{"handle": {handle}, "action": {id}}))
		return response.Code
	}
	assert.Equal(t, http.StatusTooManyRequests, action("next"))
	assert.Equal(t, http.StatusNoContent, action("stop"))
	assert.ErrorIs(t, request.ctx.Err(), context.Canceled)
	response := httptest.NewRecorder()
	started := false
	err = views.write(response, request, cliViewFrame{Handle: handle, Text: strings.Repeat("a", cli.BrowserViewFrameBytesMax)}, &started)
	require.ErrorContains(t, err, "64 KiB")
	assert.False(t, started)
	assert.Empty(t, response.Body.String())
}

// TestWebCLIViewWriteFailureJoinsRunner proves an HTTP write failure cancels
// the bound Model lifetime and removes its handle before the handler returns.
func TestWebCLIViewWriteFailureJoinsRunner(t *testing.T) {
	var bound context.Context
	views := NewCLIViewTransport(cliViewTestAdmission, func(ctx context.Context, caller plugin.CallerIdentity, model *cli.Model) error {
		bound = ctx
		return cliViewTestBinding(ctx, caller, model)
	})
	handler := views.Handler(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unexpected fallback") }), nil)
	request := authedRequest(t, http.MethodPost, "/cli/stream", url.Values{"command": {"monitor bgp"}, "mode": {terminalModeOperational}})
	request.Header.Set("Accept", CLIViewMediaType)
	writer := &cliViewFailedWriter{ResponseRecorder: httptest.NewRecorder()}
	handler(writer, request)
	require.NotNil(t, bound)
	assert.ErrorIs(t, bound.Err(), context.Canceled)
	assert.Empty(t, views.active)
}

// TestWebCLIViewMalformedActionDoesNotReachRunner pairs malformed and anonymous
// forms with a declared action that remains usable after every refusal.
func TestWebCLIViewMalformedActionDoesNotReachRunner(t *testing.T) {
	views := NewCLIViewTransport(cliViewTestAdmission, cliViewTestBinding)
	handle, request, err := views.add(t.Context(), plugin.CallerIdentity{Username: "testuser"})
	require.NoError(t, err)
	defer request.cancel()
	request.controls = []cli.BrowserViewControl{{ID: "next", Label: "Next"}}
	for _, values := range []url.Values{
		{"handle": {handle}, "action": {"next", "stop"}},
		{"handle": {handle}, "action": {"next"}, "command": {"request shutdown"}},
		{"handle": {handle}, "action": {strings.Repeat("x", 65)}},
		{"handle": {handle}, "action": {""}},
	} {
		response := httptest.NewRecorder()
		views.Action(response, authedRequest(t, http.MethodPost, "/cli/view/action", values))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Empty(t, request.actions)
	}
	anonymous := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/cli/view/action", strings.NewReader("handle="+handle+"&action=next"))
	anonymous.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	views.Action(response, anonymous)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Empty(t, request.actions)
	status, refusal := views.action("testuser", handle, "next")
	require.Nil(t, refusal)
	assert.Equal(t, http.StatusNoContent, status)
	assert.Equal(t, cli.BrowserViewAction{ID: "next"}, <-request.actions)
}

// TestWebCLIViewNegotiationPreservesTextConsumers checks registered selection
// and explicit MIME negotiation, including a zero quality preference.
func TestWebCLIViewNegotiationPreservesTextConsumers(t *testing.T) {
	views := NewCLIViewTransport(cliViewTestAdmission, cliViewTestBinding)
	for _, accepted := range []string{"", "text/plain", "*/*", CLIViewMediaType + ";q=0.0", CLIViewMediaType + ";q=invalid"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/cli/stream", nil)
		request.Header.Set("Accept", accepted)
		assert.False(t, views.accepts(request, "monitor bgp"), accepted)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/cli/stream", nil)
	request.Header.Set("Accept", CLIViewMediaType)
	assert.True(t, views.accepts(request, "monitor bgp"))
	assert.False(t, views.accepts(request, "show version"))
	assert.False(t, views.accepts(request, "monitor bgpx"))
}

// TestWebCLIViewWriteDeadlineBoundsSocket checks the actual response controller
// installs the declared write allowance before emitting and flushing a frame.
func TestWebCLIViewWriteDeadlineBoundsSocket(t *testing.T) {
	views := NewCLIViewTransport(cliViewTestAdmission, cliViewTestBinding)
	handle, request, err := views.add(t.Context(), plugin.CallerIdentity{Username: "testuser"})
	require.NoError(t, err)
	defer request.cancel()
	writer := &cliViewDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	started := false
	before := time.Now()
	err = views.write(writer, request, cliViewFrame{Handle: handle, Sequence: 1, Text: "bounded answer"}, &started)
	require.NoError(t, err)
	assert.True(t, started)
	assert.True(t, writer.Flushed)
	assert.False(t, writer.deadline.Before(before.Add(cliViewWriteTimeout)))
	assert.False(t, writer.deadline.After(time.Now().Add(cliViewWriteTimeout)))
}

type cliViewDeadlineWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *cliViewDeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

type cliViewFailedWriter struct{ *httptest.ResponseRecorder }

func (*cliViewFailedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func cliViewTestAdmission(ctx context.Context, caller plugin.CallerIdentity, _ string, run func(context.Context, plugin.CallerIdentity) error) (bool, error) {
	return true, run(ctx, caller)
}

func cliViewTestBinding(_ context.Context, _ plugin.CallerIdentity, model *cli.Model) error {
	model.SetViewFactory(cli.ViewKeyDashboard, cli.DashboardFactory(func() (func() (string, error), error) {
		return func() (string, error) {
			return `{"router-id":"192.0.2.1","peers":[{"address":"10.0.0.10","remote-as":10},{"address":"10.0.0.9","remote-as":9}]}`, nil
		}, nil
	}))
	return nil
}

func cliViewTestServer(t *testing.T, admit plugin.CommandStreamAdmission, bind CLIViewBinding) (*http.Client, string, *CLIViewTransport) {
	t.Helper()
	views := NewCLIViewTransport(admit, bind)
	mux := http.NewServeMux()
	mux.Handle("POST /cli/stream", views.Handler(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unexpected fallback") }), nil))
	mux.HandleFunc("POST /cli/view/action", views.Action)
	protected := RequireSameOrigin(mux)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			http.Error(w, "synthetic sign in required", http.StatusUnauthorized)
			return
		}
		if password != "synthetic" {
			http.Error(w, "synthetic sign in required", http.StatusUnauthorized)
			return
		}
		protected.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyUsername, username)))
	}))
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = 5 * time.Second
	t.Cleanup(client.CloseIdleConnections)
	return client, server.URL, views
}

func cliViewTestOpen(t *testing.T, client *http.Client, base, username string) (*http.Response, *json.Decoder) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/cli/stream", strings.NewReader(url.Values{"command": {"monitor bgp"}, "mode": {terminalModeOperational}}.Encode()))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", CLIViewMediaType)
	request.Header.Set("Origin", base)
	request.SetBasicAuth(username, "synthetic")
	response, err := client.Do(request)
	require.NoError(t, err)
	return response, json.NewDecoder(response.Body)
}

func cliViewTestAction(t *testing.T, client *http.Client, base, username, handle, action string, status int) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/cli/view/action", strings.NewReader(url.Values{"handle": {handle}, "action": {action}}.Encode()))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", base)
	request.SetBasicAuth(username, "synthetic")
	response, err := client.Do(request)
	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, status, response.StatusCode, string(body))
}

func cliViewTestFrame(t *testing.T, decoder *json.Decoder, matches func(cliViewFrame) bool) cliViewFrame {
	t.Helper()
	for range 32 { // Action/poll milestones must arrive within 32 latest frames.
		var frame cliViewFrame
		require.NoError(t, decoder.Decode(&frame))
		require.Empty(t, frame.Error)
		if matches(frame) {
			return frame
		}
	}
	t.Fatal(errors.New("monitor did not reach the requested frame"))
	return cliViewFrame{}
}
