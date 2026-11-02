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
// server's one goroutine. start opens the listener, registers the server's end
// with the daemon's stop, and starts the goroutine; stop shuts the server down
// and MUST return only after that goroutine ended, which it does by waiting on
// the listener's done channel. A daemon stop ends the server the same way, so
// the goroutine never outlives the daemon that started it. pause toggles the
// held pause the server reads on every request. Safe for concurrent use.
type updateServeOwner struct {
	mu      sync.Mutex
	running *updateServeListener
}

// updateServeListener is one running server. serve is its goroutine, and it
// closes done when Serve returns, so a closed done means no goroutine is left.
type updateServeListener struct {
	srv      *http.Server
	addr     string
	paused   atomic.Bool
	done     chan struct{}
	withdraw func() // removes the daemon-stop hook start registered
}

// daemonStop registers fn to run when the daemon stops, and answers the
// function that withdraws it. The daemon's is (*pluginserver.Server).OnStop,
// which runs fn inside the daemon's ordered shutdown and waits for it.
type daemonStop func(fn func()) (func(), error)

// serve runs the server until Shutdown or Close ends it, then closes done.
// It MUST be the only goroutine started for the listener (start does).
func (l *updateServeListener) serve(ln net.Listener) {
	defer close(l.done)
	l.srv.Serve(ln) //nolint:errcheck // Serve always answers an error; ErrServerClosed is stop's own, and any other ends the server, which stop then reports as stopped
}

// ended reports whether serve has returned, read from done, so an answer that
// says stopped states what the goroutine did rather than what stop intended.
func (l *updateServeListener) ended() bool {
	select {
	case <-l.done:
		return true
	default:
		return false
	}
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
// connections, the bound port included when listen asked for port 0. The
// server's end is registered with onStop before its goroutine starts, so a
// daemon that refuses the hook (it is already stopping) starts no server.
func (o *updateServeOwner) start(ctx context.Context, listen string, onStop daemonStop) (updateServeState, error) {
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
	withdraw, err := onStop(func() { o.endWithDaemon(l) })
	if err != nil {
		ln.Close() //nolint:errcheck // the listener never served; the refusal below is the answer
		return updateServeState{}, err
	}
	l.withdraw = withdraw
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
	o.shutdown(l)
	return updateServeState{Listen: l.addr, Paused: l.paused.Load(), Stopped: l.ended()}, nil
}

// endWithDaemon is the daemon-stop hook start registers: it ends l when l is
// still the running server, and returns once l's goroutine has ended. An
// operator's stop that ran first already ended l and withdrew this hook, so
// finding another server (or none) means there is nothing of l's left to end.
func (o *updateServeOwner) endWithDaemon(l *updateServeListener) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.running != l {
		return
	}
	o.shutdown(l)
}

// shutdown ends l, waits for its goroutine, withdraws its daemon-stop hook
// and clears the running slot. The caller MUST hold o.mu.
func (o *updateServeOwner) shutdown(l *updateServeListener) {
	ctx, cancel := context.WithTimeout(context.Background(), updateServeShutdownMax)
	defer cancel()
	if err := l.srv.Shutdown(ctx); err != nil {
		// Requests still in flight at the bound: their connections are closed.
		l.srv.Close() //nolint:errcheck // Close after a failed Shutdown has no further remedy; done below proves the end
	}
	<-l.done
	l.withdraw()
	o.running = nil
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
	if ctx == nil {
		return updateServeResponse(updateServeState{}, errNoDaemonOwner)
	}
	if ctx.Server == nil {
		return updateServeResponse(updateServeState{}, errNoDaemonOwner)
	}
	return updateServeResponse(updateServer.start(reqCtx(ctx), listen, ctx.Server.OnStop))
}

// errNoDaemonOwner refuses a start no daemon server dispatched: nothing would
// end the update server when the daemon stops.
var errNoDaemonOwner = errors.New("update serve start reached no daemon, so nothing would stop the server it starts: " +
	"run it through the daemon ('ze cli -c \"update serve start\"'), or run 'ze update serve --listen <host:port>' in the foreground")

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
