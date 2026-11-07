// Design: docs/guide/config-editor.md -- Start admits listeners before Stop can join.
// Related: ssh.go -- serialized Start/Stop own the listener lifecycle.
// Upstream: charm.land/ssh v0.4.3 (go.mod and vendor/modules.txt).
// https://github.com/charmbracelet/ssh/blob/v0.4.3/server.go

package ssh

import (
	"errors"
	"net"

	"charm.land/ssh"
)

// registeredListener completes startup only after Serve has registered its
// listener: the vendor calls Accept strictly after trackListener's Add. Only
// the serving goroutine touches started, including its pre-Accept error path.
// This relies on the pinned server.go's Serve/trackListener/Shutdown ordering
// (vendor/charm.land/ssh/server.go), not an SSH protocol guarantee.
// Later Accept calls need no notification, allocation or synchronization.
type registeredListener struct {
	net.Listener
	started chan error
}

func (l *registeredListener) finishStart(err error) {
	if l.started != nil {
		l.started <- err
		l.started = nil
	}
}

func (l *registeredListener) Accept() (net.Conn, error) {
	l.finishStart(nil)
	return l.Listener.Accept()
}

// serveListener starts one lifecycle goroutine and waits for actual listener
// admission or Serve's actual early error. Start holds lifecycle until every
// successfully bound primary/extra listener has completed this handshake.
func (s *Server) serveListener(network *ssh.Server, listener net.Listener) error {
	started := make(chan error, 1)
	registered := &registeredListener{Listener: listener, started: started}
	go func() {
		s.logger.Info("SSH server listening", "address", listener.Addr().String())
		err := network.Serve(registered)
		registered.finishStart(err)
		if err == nil {
			return
		}
		if errors.Is(err, ssh.ErrServerClosed) {
			return
		}
		// Reconfigure closes a listener a reload removed, and a failed
		// Start or Reconfigure closes the set it bound: Serve then answers
		// the closed listener's Accept error, which is the stop, not a fault.
		if errors.Is(err, net.ErrClosed) {
			return
		}
		s.logger.Error("SSH server error", "address", listener.Addr().String(), "error", err)
	}()
	return <-started
}
