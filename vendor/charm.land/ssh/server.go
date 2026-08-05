package ssh

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pires/go-proxyproto"
	gossh "golang.org/x/crypto/ssh"
)

// ErrServerClosed is returned by the Server's Serve, ListenAndServe,
// and ListenAndServeTLS methods after a call to Shutdown or Close.
var ErrServerClosed = errors.New("ssh: Server closed")

// SubsystemHandler is a handler for a given SSH subsystem.
type SubsystemHandler func(s Session)

// DefaultSubsystemHandlers is the default set of subsystem handlers.
var DefaultSubsystemHandlers = map[string]SubsystemHandler{}

// RequestHandler is a callback for custom global SSH requests.
type RequestHandler func(ctx Context, srv *Server, req *gossh.Request) (ok bool, payload []byte)

// DefaultRequestHandlers is the default set of request handlers.
var DefaultRequestHandlers = map[string]RequestHandler{}

// ChannelHandler is a callback for custom channel types.
type ChannelHandler func(srv *Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx Context)

// DefaultChannelHandlers is the default set of channel handlers.
var DefaultChannelHandlers = map[string]ChannelHandler{
	"session": DefaultSessionHandler,
}

var permissionsPublicKeyExt = "gliderlabs/ssh.PublicKey"

// ErrPermissionDenied is returned when authentication fails.
var ErrPermissionDenied = errors.New("permission denied")

func ensureNoPKInPermissions(ctx Context) error {
	if _, ok := ctx.Permissions().Extensions[permissionsPublicKeyExt]; ok {
		return errors.New("misconfigured server: public key incorrectly set")
	}

	return nil
}

// Server defines parameters for running an SSH server. The zero value for
// Server is a valid configuration. When both PasswordHandler and
// PublicKeyHandler are nil, no client authentication is performed.
type Server struct {
	Addr        string   // TCP address to listen on, ":22" if empty
	Handler     Handler  // handler to invoke, ssh.DefaultHandler if nil
	HostSigners []Signer // private keys for the host key, must have at least one
	Version     string   // server version to be sent before the initial handshake
	Banner      string   // server banner

	BannerHandler                 BannerHandler                 // server banner handler, overrides Banner
	KeyboardInteractiveHandler    KeyboardInteractiveHandler    // keyboard-interactive authentication handler
	PasswordHandler               PasswordHandler               // password authentication handler
	PublicKeyHandler              PublicKeyHandler              // public key authentication handler
	PtyCallback                   PtyCallback                   // callback for allocating and allowing PTY sessions, ssh.EmulatePtyCallback if nil
	PtyHandler                    PtyHandler                    // pty allocation handler, ssh.emulatePtyHandler if nil
	ConnCallback                  ConnCallback                  // optional callback for wrapping net.Conn before handling
	LocalPortForwardingCallback   LocalPortForwardingCallback   // callback for allowing local port forwarding, denies all if nil
	ReversePortForwardingCallback ReversePortForwardingCallback // callback for allowing reverse port forwarding, denies all if nil
	ServerConfigCallback          ServerConfigCallback          // callback for configuring detailed SSH options
	SessionRequestCallback        SessionRequestCallback        // callback for allowing or denying SSH sessions

	ConnectionFailedCallback ConnectionFailedCallback // callback to report connection failures
	ConnectionCloseCallback  ConnectionCloseCallback  // callback to report connection close

	HandshakeTimeout time.Duration // connection timeout until successful handshake, none if empty
	IdleTimeout      time.Duration // connection timeout when no activity, none if empty
	MaxTimeout       time.Duration // absolute connection timeout, none if empty

	EnableProxyProtocol bool // Enable support for HA Proxy's and NGinx's PROXY protocol

	// ChannelHandlers allow overriding the built-in session handlers or provide
	// extensions to the protocol, such as tcpip forwarding. By default only the
	// "session" handler is enabled.
	ChannelHandlers map[string]ChannelHandler

	// RequestHandlers allow overriding the server-level request handlers or
	// provide extensions to the protocol, such as tcpip forwarding. By default
	// no handlers are enabled.
	RequestHandlers map[string]RequestHandler

	// SubsystemHandlers are handlers which are similar to the usual SSH command
	// handlers, but handle named subsystems.
	SubsystemHandlers map[string]SubsystemHandler

	listenerWg sync.WaitGroup
	mu         sync.RWMutex
	started    atomic.Bool
	listeners  map[net.Listener]struct{}
	conns      map[*gossh.ServerConn]struct{}
	connWg     sync.WaitGroup
	doneChan   chan struct{}
}

func (srv *Server) ensureHostSigner() error {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	if len(srv.HostSigners) == 0 {
		signer, err := generateSigner()
		if err != nil {
			return err
		}
		srv.HostSigners = append(srv.HostSigners, signer)
	}
	return nil
}

func (srv *Server) ensureHandlers() {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	if srv.RequestHandlers == nil {
		srv.RequestHandlers = map[string]RequestHandler{}
		for k, v := range DefaultRequestHandlers {
			srv.RequestHandlers[k] = v
		}
	}
	if srv.ChannelHandlers == nil {
		srv.ChannelHandlers = map[string]ChannelHandler{}
		for k, v := range DefaultChannelHandlers {
			srv.ChannelHandlers[k] = v
		}
	}
	if srv.SubsystemHandlers == nil {
		srv.SubsystemHandlers = map[string]SubsystemHandler{}
		for k, v := range DefaultSubsystemHandlers {
			srv.SubsystemHandlers[k] = v
		}
	}
}

func (srv *Server) config(ctx Context) *gossh.ServerConfig {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	var config *gossh.ServerConfig
	if srv.ServerConfigCallback == nil {
		config = &gossh.ServerConfig{}
	} else {
		config = srv.ServerConfigCallback(ctx)
	}
	for _, signer := range srv.HostSigners {
		config.AddHostKey(signer)
	}
	if srv.PasswordHandler == nil && srv.PublicKeyHandler == nil && srv.KeyboardInteractiveHandler == nil &&
		config.PasswordCallback == nil && config.PublicKeyCallback == nil && config.KeyboardInteractiveCallback == nil {
		config.NoClientAuth = true
	}
	if srv.PtyHandler == nil {
		srv.PtyHandler = emulatePtyHandler
	}
	if srv.Version != "" {
		config.ServerVersion = "SSH-2.0-" + srv.Version
	}
	if srv.Banner != "" {
		config.BannerCallback = func(_ gossh.ConnMetadata) string {
			return srv.Banner
		}
	}
	if srv.BannerHandler != nil {
		config.BannerCallback = func(conn gossh.ConnMetadata) string {
			applyConnMetadata(ctx, conn)
			return srv.BannerHandler(ctx)
		}
	}
	if srv.PasswordHandler != nil {
		config.PasswordCallback = func(conn gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
			resetPermissions(ctx)
			applyConnMetadata(ctx, conn)
			err := ensureNoPKInPermissions(ctx)
			if err != nil {
				return ctx.Permissions().Permissions, err
			}
			ok := srv.PasswordHandler(ctx, string(password))
			if !ok {
				return ctx.Permissions().Permissions, ErrPermissionDenied
			}
			return ctx.Permissions().Permissions, nil
		}
	}
	if srv.PublicKeyHandler != nil {
		config.PublicKeyCallback = func(conn gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
			resetPermissions(ctx)
			applyConnMetadata(ctx, conn)
			err := ensureNoPKInPermissions(ctx)
			if err != nil {
				return ctx.Permissions().Permissions, err
			}
			ok := srv.PublicKeyHandler(ctx, key)
			if !ok {
				return ctx.Permissions().Permissions, ErrPermissionDenied
			}

			pkStr := base64.StdEncoding.EncodeToString(key.Marshal())
			if ctx.Permissions().Extensions == nil {
				ctx.Permissions().Extensions = map[string]string{}
			}
			ctx.Permissions().Extensions[permissionsPublicKeyExt] = pkStr

			return ctx.Permissions().Permissions, nil
		}
	}
	if srv.KeyboardInteractiveHandler != nil {
		config.KeyboardInteractiveCallback = func(conn gossh.ConnMetadata, challenger gossh.KeyboardInteractiveChallenge) (*gossh.Permissions, error) {
			resetPermissions(ctx)
			applyConnMetadata(ctx, conn)
			ok := srv.KeyboardInteractiveHandler(ctx, challenger)
			err := ensureNoPKInPermissions(ctx)
			if err != nil {
				return ctx.Permissions().Permissions, err
			}
			if !ok {
				return ctx.Permissions().Permissions, ErrPermissionDenied
			}
			return ctx.Permissions().Permissions, nil
		}
	}
	return config
}

// Handle sets the Handler for the server.
func (srv *Server) Handle(fn Handler) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	srv.Handler = fn
}

// Close immediately closes all active listeners and all active
// connections.
//
// Close returns any error returned from closing the Server's
// underlying Listener(s).
func (srv *Server) Close() error {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	srv.closeDoneChanLocked()
	err := srv.closeListenersLocked()
	for c := range srv.conns {
		_ = c.Close()
		delete(srv.conns, c)
	}
	return err
}

// Shutdown gracefully shuts down the server without interrupting any
// active connections. Shutdown works by first closing all open
// listeners, and then waiting indefinitely for connections to close.
// If the provided context expires before the shutdown is complete,
// then the context's error is returned.
func (srv *Server) Shutdown(ctx context.Context) error {
	srv.mu.Lock()
	lnerr := srv.closeListenersLocked()
	srv.closeDoneChanLocked()
	srv.mu.Unlock()

	finished := make(chan struct{}, 1)
	go func() {
		srv.listenerWg.Wait()
		srv.connWg.Wait()
		finished <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-finished:
		return lnerr
	}
}

// Serve accepts incoming connections on the Listener l, creating a new
// connection goroutine for each. The connection goroutines read requests and then
// calls srv.Handler to handle sessions.
//
// Serve always returns a non-nil error.
func (srv *Server) Serve(l net.Listener) error {
	if srv.EnableProxyProtocol {
		_, ok := l.(*proxyproto.Listener)
		if !ok {
			l = &proxyproto.Listener{Listener: l}
		}
	}

	srv.ensureHandlers()
	defer func() { _ = l.Close() }()
	if err := srv.ensureHostSigner(); err != nil {
		return err
	}
	if srv.Handler == nil {
		srv.Handler = DefaultHandler
	}
	var tempDelay time.Duration

	srv.trackListener(l, true)
	srv.started.Store(true)
	defer srv.trackListener(l, false)
	for {
		conn, e := l.Accept()
		if e != nil {
			select {
			case <-srv.getDoneChan():
				return ErrServerClosed
			default:
			}
			if ne, ok := e.(net.Error); ok && ne.Temporary() { //nolint:staticcheck // SA1019: same pattern as net/http
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if limit := 1 * time.Second; tempDelay > limit {
					tempDelay = limit
				}
				time.Sleep(tempDelay)
				continue
			}
			return e
		}
		go srv.HandleConn(conn)
	}
}

// HandleConn handles a new SSH connection.
//
// A panic while handling the connection is recovered and logged rather than
// allowed to escape. Connections are served on their own goroutine, and Go has
// no process-wide panic handler, so an unrecovered panic here would terminate
// the whole server process along with every other connection. That includes
// panics raised inside the SSH handshake, before authentication has happened,
// which would otherwise let an unauthenticated client take the server down.
func (srv *Server) HandleConn(newConn net.Conn) {
	defer recoverAndLog("panic serving connection", newConn, func() {
		// The deferred close inside handleConn did not get to run.
		_ = newConn.Close()
	})

	srv.handleConn(newConn)
}

// recoverAndLog recovers a panic on the current goroutine, logs it with a
// stack trace, and runs cleanup if one is given.
//
// This must be deferred directly on the goroutine to be protected: a recover
// only sees panics unwinding its own stack, so every goroutine the server
// starts needs its own call.
func recoverAndLog(msg string, conn net.Conn, cleanup func()) {
	r := recover()
	if r == nil {
		return
	}
	slog.Error("ssh: "+msg,
		"remote", remoteAddrString(conn),
		"panic", r,
		"stack", string(debug.Stack()),
	)
	if cleanup != nil {
		// Cleanup runs while already unwinding a panic, so a second panic
		// raised here would escape this recover and kill the process, undoing
		// the containment. Contain it separately.
		defer func() {
			if r := recover(); r != nil {
				slog.Error("ssh: panic during "+msg+" cleanup", "panic", r)
			}
		}()
		cleanup()
	}
}

// remoteAddrString returns conn's remote address for logging, tolerating a nil
// conn or address so that the panic handler cannot itself panic.
func remoteAddrString(conn net.Conn) string {
	if conn == nil {
		return "unknown"
	}
	if addr := conn.RemoteAddr(); addr != nil {
		return addr.String()
	}
	return "unknown"
}

func (srv *Server) handleConn(newConn net.Conn) {
	ctx, cancel := newContext(srv)
	if srv.ConnCallback != nil {
		cbConn := srv.ConnCallback(ctx, newConn)
		if cbConn == nil {
			_ = newConn.Close()
			return
		}
		newConn = cbConn
	}
	conn := &serverConn{
		Conn:          newConn,
		idleTimeout:   srv.IdleTimeout,
		closeCanceler: cancel,
	}
	if srv.MaxTimeout > 0 {
		conn.maxDeadline = time.Now().Add(srv.MaxTimeout)
	}
	if srv.HandshakeTimeout > 0 {
		conn.handshakeDeadline = time.Now().Add(srv.HandshakeTimeout)
	}
	conn.updateDeadline()
	defer func() { _ = conn.Close() }()
	defer func() {
		if srv.ConnectionCloseCallback != nil {
			srv.ConnectionCloseCallback(conn)
		}
	}()
	sshConn, chans, reqs, err := gossh.NewServerConn(conn, srv.config(ctx))
	if err != nil {
		if srv.ConnectionFailedCallback != nil {
			srv.ConnectionFailedCallback(conn, err)
		}
		return
	}
	conn.handshakeDeadline = time.Time{}
	conn.updateDeadline()

	if err := extractPublicKeyFromPermissions(ctx, sshConn); err != nil {
		if srv.ConnectionFailedCallback != nil {
			srv.ConnectionFailedCallback(conn, err)
		}
		return
	}

	// Additionally, now that the connection was authed, we can take the
	// permissions off of the gossh.Conn and re-attach them to the Permissions
	// object stored in the Context.
	ctx.Permissions().Permissions = sshConn.Permissions

	srv.trackConn(sshConn, true)
	defer srv.trackConn(sshConn, false)

	ctx.SetValue(ContextKeyConn, sshConn)
	applyConnMetadata(ctx, sshConn)
	// go gossh.DiscardRequests(reqs)
	go func() {
		defer recoverAndLog("panic handling requests", conn, nil)
		srv.handleRequests(ctx, reqs)
	}()
	for ch := range chans {
		handler := srv.ChannelHandlers[ch.ChannelType()]
		if handler == nil {
			handler = srv.ChannelHandlers["default"]
		}
		if handler == nil {
			_ = ch.Reject(gossh.UnknownChannelType, "unsupported channel type")
			continue
		}
		go func(ch gossh.NewChannel) {
			defer recoverAndLog("panic handling channel", conn, nil)
			handler(srv, sshConn, ch, ctx)
		}(ch)
	}
}

func (srv *Server) handleRequests(ctx Context, in <-chan *gossh.Request) {
	for req := range in {
		handler := srv.RequestHandlers[req.Type]
		if handler == nil {
			handler = srv.RequestHandlers["default"]
		}
		if handler == nil {
			_ = req.Reply(false, nil)
			continue
		}
		/*reqCtx, cancel := context.WithCancel(ctx)
		defer cancel() */
		ret, payload := handler(ctx, srv, req)
		_ = req.Reply(ret, payload)
	}
}

// ListenAndServe listens on the TCP network address srv.Addr and then calls
// Serve to handle incoming connections. If srv.Addr is blank, ":22" is used.
// ListenAndServe always returns a non-nil error.
func (srv *Server) ListenAndServe() error {
	addr := srv.Addr
	if addr == "" {
		addr = ":22"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}

// AddHostKey adds a private key as a host key. If an existing host key exists
// with the same algorithm, it is overwritten. Each server config must have at
// least one host key.
func (srv *Server) AddHostKey(key Signer) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	// these are later added via AddHostKey on ServerConfig, which performs the
	// check for one of every algorithm.

	// This check is based on the AddHostKey method from the x/crypto/ssh
	// library. This allows us to only keep one active key for each type on a
	// server at once. So, if you're dynamically updating keys at runtime, this
	// list will not keep growing.
	for i, k := range srv.HostSigners {
		if k.PublicKey().Type() == key.PublicKey().Type() {
			srv.HostSigners[i] = key
			return
		}
	}

	srv.HostSigners = append(srv.HostSigners, key)
}

// SetOption runs a functional option against the server.
// It returns an error if the server has already been started.
func (srv *Server) SetOption(option Option) error {
	if srv.started.Load() {
		return fmt.Errorf("ssh: cannot set option after server has started")
	}

	// NOTE: there is a potential race here for any option that doesn't call an
	// internal method. We can't actually lock here because if something calls
	// (as an example) AddHostKey, it will deadlock.

	// srv.mu.Lock()
	// defer srv.mu.Unlock()

	return option(srv)
}

func (srv *Server) getDoneChan() <-chan struct{} {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	return srv.getDoneChanLocked()
}

func (srv *Server) getDoneChanLocked() chan struct{} {
	if srv.doneChan == nil {
		srv.doneChan = make(chan struct{})
	}
	return srv.doneChan
}

func (srv *Server) closeDoneChanLocked() {
	ch := srv.getDoneChanLocked()
	select {
	case <-ch:
		// Already closed. Don't close again.
	default:
		// Safe to close here. We're the only closer, guarded
		// by srv.mu.
		close(ch)
	}
}

func (srv *Server) closeListenersLocked() error {
	var err error
	for ln := range srv.listeners {
		if cerr := ln.Close(); cerr != nil && err == nil {
			err = cerr
		}
		delete(srv.listeners, ln)
	}
	return err
}

func (srv *Server) trackListener(ln net.Listener, add bool) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	if srv.listeners == nil {
		srv.listeners = make(map[net.Listener]struct{})
	}
	if add {
		// If the *Server is being reused after a previous
		// Close or Shutdown, reset its doneChan:
		if len(srv.listeners) == 0 && len(srv.conns) == 0 {
			srv.doneChan = nil
		}
		srv.listeners[ln] = struct{}{}
		srv.listenerWg.Add(1)
	} else {
		delete(srv.listeners, ln)
		srv.listenerWg.Done()
	}
}

func (srv *Server) trackConn(c *gossh.ServerConn, add bool) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	if srv.conns == nil {
		srv.conns = make(map[*gossh.ServerConn]struct{})
	}
	if add {
		srv.conns[c] = struct{}{}
		srv.connWg.Add(1)
	} else {
		delete(srv.conns, c)
		srv.connWg.Done()
	}
}

// extractPublicKeyFromPermissions re-parses the public key from the
// permissions extensions and stores it in the context.
func extractPublicKeyFromPermissions(ctx Context, sshConn *gossh.ServerConn) error {
	if sshConn.Permissions == nil {
		return nil
	}
	keyData, ok := sshConn.Permissions.Extensions[permissionsPublicKeyExt]
	if !ok {
		return nil
	}
	decodedData, err := base64.StdEncoding.DecodeString(keyData)
	if err != nil {
		return err
	}
	key, err := gossh.ParsePublicKey(decodedData)
	if err != nil {
		return err
	}
	ctx.SetValue(ContextKeyPublicKey, key)
	return nil
}
