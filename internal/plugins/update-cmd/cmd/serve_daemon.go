// Design: docs/architecture/appliance/self-update.md -- the update server under the daemon
// Related: serve.go -- the content the server answers, and the foreground `ze update serve`

package cmd

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// updateServeShutdownMax bounds how long stop lets requests in flight finish
// before it closes their connections.
const updateServeShutdownMax = 5 * time.Second

// updateServer is the daemon's one update server. A daemon runs at most one,
// so a second `update serve start` is refused rather than given a port of
// its own.
var updateServer updateServeOwner

// updateServeOwner owns the update server a daemon runs, and through it the
// server's one goroutine. start opens the listener and starts the goroutine;
// stop shuts the server down and MUST return only after that goroutine ended,
// which it does by waiting on the listener's done channel. pause toggles the
// held pause the server reads on every request. Safe for concurrent use.
type updateServeOwner struct {
	mu      sync.Mutex
	running *updateServeListener
}

// updateServeListener is one running server. serve is its goroutine, and it
// closes done when Serve returns, so a closed done means no goroutine is left.
type updateServeListener struct {
	srv    *http.Server
	addr   string
	paused atomic.Bool
	done   chan struct{}
}

// serve runs the server until Shutdown or Close ends it, then closes done.
// It MUST be the only goroutine started for the listener (start does).
func (l *updateServeListener) serve(ln net.Listener) {
	defer close(l.done)
	l.srv.Serve(ln) //nolint:errcheck // Serve always answers an error; ErrServerClosed is stop's own, and any other ends the server, which stop then reports as stopped
}

// updateServeState is what each lifecycle command answers.
type updateServeState struct {
	plugin.DataMarker
	Listen  string `json:"listen"`
	Paused  bool   `json:"paused"`
	Stopped bool   `json:"stopped,omitempty"`
}

// start opens listen and serves the running binary on it. The listener is
// opened before start returns, so the address it answers already accepts
// connections, the bound port included when listen asked for port 0.
func (o *updateServeOwner) start(ctx context.Context, listen string) (updateServeState, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.running != nil {
		var tb textbuf.Buffer
		return updateServeState{}, errors.New(tb.Str("an update server already listens on ").Str(o.running.addr).
			Str(", and a daemon runs one at a time: stop it first with 'update serve stop'").String())
	}

	content, err := newUpdateServeContent()
	if err != nil {
		return updateServeState{}, err
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", listen)
	if err != nil {
		var tb textbuf.Buffer
		return updateServeState{}, errors.New(tb.Str("cannot listen on ").Str(listen).Str(": ").Str(err.Error()).
			Str(": name a free address with 'update serve start listen <host:port>'").String())
	}

	l := &updateServeListener{addr: ln.Addr().String(), done: make(chan struct{})}
	l.srv = &http.Server{
		Handler:           content.handler(l.paused.Load),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go l.serve(ln)
	o.running = l
	return updateServeState{Listen: l.addr}, nil
}

// stop ends the running server and returns once its goroutine has ended.
func (o *updateServeOwner) stop() (updateServeState, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	l := o.running
	if l == nil {
		return updateServeState{}, errNoUpdateServer
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateServeShutdownMax)
	defer cancel()
	if err := l.srv.Shutdown(ctx); err != nil {
		// Requests still in flight at the bound: their connections are closed.
		l.srv.Close() //nolint:errcheck // Close after a failed Shutdown has no further remedy; done below proves the end
	}
	<-l.done
	o.running = nil
	return updateServeState{Listen: l.addr, Paused: l.paused.Load(), Stopped: true}, nil
}

// pause toggles the held pause, as SIGUSR1 does for the foreground server.
func (o *updateServeOwner) pause() (updateServeState, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	l := o.running
	if l == nil {
		return updateServeState{}, errNoUpdateServer
	}
	state := !l.paused.Load()
	l.paused.Store(state)
	return updateServeState{Listen: l.addr, Paused: state}, nil
}

var errNoUpdateServer = errors.New("no update server is running: start one with 'update serve start'")

// handleUpdateServeStart answers `update serve start [listen <host:port>]`.
func handleUpdateServeStart(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
	listen, found := args.Value("listen")
	if !found {
		listen = updateServeListenDefault
	}
	return updateServeResponse(updateServer.start(reqCtx(ctx), listen))
}

// handleUpdateServeStop answers `update serve stop`.
func handleUpdateServeStop(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return updateServeResponse(updateServer.stop())
}

// handleUpdateServePause answers `update serve pause`.
func handleUpdateServePause(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return updateServeResponse(updateServer.pause())
}

// updateServeResponse answers a lifecycle command's state, or its refusal.
func updateServeResponse(state updateServeState, err error) (*plugin.Response, error) {
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: state}, nil
}
