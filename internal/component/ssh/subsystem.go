// Design: docs/architecture/appliance/remote-operations.md -- registered SSH subsystem ownership.
// Related: ssh.go -- shared session admission and authentication identity.
// Upstream: charm.land/ssh v0.4.3, session.go subsystem dispatch bypasses Wish middleware;
// https://github.com/charmbracelet/ssh/blob/main/session.go.

package ssh

import (
	"context"
	"errors"
	"io"
	"time"

	"charm.land/ssh"
	"charm.land/wish/v2"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

// SubsystemHandler runs one channel synchronously. It must honor ctx, must not
// retain the channel or identity, and must finish its mutations before returning.
type SubsystemHandler func(ctx context.Context, channel io.ReadWriteCloser, caller plugin.CallerIdentity) error

// RegisterSubsystem registers a per-server handler before Start. The server owns
// session admission, panic containment, channel timeout, and the handler's join.
func (s *Server) RegisterSubsystem(name string, handler SubsystemHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wish != nil {
		return errors.New("SSH subsystems must register before the server starts")
	}
	if name == "" {
		return errors.New("SSH subsystem name is empty")
	}
	if handler == nil {
		return errors.New("SSH subsystem handler is missing")
	}
	if s.subsystems == nil {
		s.subsystems = make(map[string]SubsystemHandler)
	}
	if _, exists := s.subsystems[name]; exists {
		return errors.New("SSH subsystem name is already registered")
	}
	s.subsystems[name] = handler
	return nil
}

// subsystemOptions is called with s.mu held during Start. Each handler gets the
// same session counter and panic guard as shell/exec, explicitly: the upstream
// subsystem dispatcher does not invoke the Wish middleware chain.
func (s *Server) subsystemOptions() []ssh.Option {
	opts := make([]ssh.Option, 0, len(s.subsystems))
	for name, handler := range s.subsystems {
		serve := func(sess ssh.Session) { s.serveSubsystem(sess, handler) }
		wrapped := s.panicMiddleware()(s.maxSessionsMiddleware()(serve))
		opts = append(opts, wish.WithSubsystem(name, ssh.SubsystemHandler(wrapped)))
	}
	return opts
}

const subsystemTimeout = 45 * time.Second

func (s *Server) serveSubsystem(sess ssh.Session, handler SubsystemHandler) {
	transport, ok := sess.Context().Value(ssh.ContextKeyConn).(io.Closer)
	if !ok {
		io.WriteString(sess.Stderr(), "SSH subsystem has no transport owner; reconnect to the daemon\n") //nolint:errcheck // Best-effort refusal before invoking the handler.
		sess.Exit(1)                                                                                     //nolint:errcheck // Best-effort refusal.
		return
	}
	ctx, cancel := context.WithTimeout(sess.Context(), subsystemTimeout)
	defer cancel()
	owner := subsystemChannelOwner{ctx: ctx, transport: transport, channel: sess, finished: make(chan struct{}), joined: make(chan struct{})}
	go owner.run()
	defer owner.finish()
	err := handler(ctx, sess, plugin.CallerIdentity{
		Username: sess.User(), RemoteAddr: sess.RemoteAddr().String(),
		Authorizer: getSessionAuthorizer(sess), Surface: "ssh",
	})
	if err != nil {
		io.WriteString(sess.Stderr(), err.Error()+"\n") //nolint:errcheck // The peer may have disconnected.
		sess.Exit(1)                                    //nolint:errcheck // The peer may have disconnected.
		return
	}
	if ctx.Err() != nil {
		err = opererr.New("SSH operation ended before its receipt was delivered", "The result may already be applied; check the device before retrying", "show config")
		io.WriteString(sess.Stderr(), err.Error()+"\n") //nolint:errcheck // The peer may have disconnected.
		sess.Exit(1)                                    //nolint:errcheck // The peer may have disconnected.
		return
	}
	sess.Exit(0) //nolint:errcheck // The peer may have disconnected after receiving the result.
}

// subsystemChannelOwner interrupts blocked intake/output on cancellation. Its
// handler always joins it; it never owns or launches a configuration mutation.
type subsystemChannelOwner struct {
	ctx       context.Context
	transport io.Closer
	channel   io.Closer
	finished  chan struct{}
	joined    chan struct{}
}

func (o *subsystemChannelOwner) run() {
	defer close(o.joined)
	select {
	case <-o.ctx.Done():
		// Channel.Close writes an SSH packet and can block behind a peer that
		// stopped reading. Closing its transport first bounds that write too.
		o.transport.Close() //nolint:errcheck // Timeout terminates the stalled connection.
		o.channel.Close()   //nolint:errcheck // Release the channel after its transport closed.
	case <-o.finished:
	}
}

func (o *subsystemChannelOwner) finish() {
	close(o.finished)
	<-o.joined
}
