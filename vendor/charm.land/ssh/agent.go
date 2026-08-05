package ssh

import (
	"io"
	"net"
	"os"
	"path"
	"sync"

	gossh "golang.org/x/crypto/ssh"
)

const (
	agentRequestType = "auth-agent-req@openssh.com"
	agentChannelType = "auth-agent@openssh.com"

	agentTempDir    = "auth-agent"
	agentListenFile = "listener.sock"
)

// contextKeyAgentRequest is an internal context key for storing if the
// client requested agent forwarding.
var contextKeyAgentRequest = &contextKey{"auth-agent-req"}

// SetAgentRequested sets up the session context so that AgentRequested
// returns true.
func SetAgentRequested(ctx Context) {
	ctx.SetValue(contextKeyAgentRequest, true)
}

// AgentRequested returns true if the client requested agent forwarding.
func AgentRequested(sess Session) bool {
	return sess.Context().Value(contextKeyAgentRequest) == true
}

// NewAgentListener sets up a temporary Unix socket that can be communicated
// to the session environment and used for forwarding connections.
func NewAgentListener() (net.Listener, error) {
	dir, err := os.MkdirTemp("", agentTempDir)
	if err != nil {
		return nil, err
	}
	l, err := net.Listen("unix", path.Join(dir, agentListenFile))
	if err != nil {
		return nil, err
	}
	return l, nil
}

// ForwardAgentConnections takes connections from a listener to proxy into the
// session on the OpenSSH channel for agent connections. It blocks and services
// connections until the listener stop accepting.
func ForwardAgentConnections(l net.Listener, s Session) {
	sshConn := s.Context().Value(ContextKeyConn).(gossh.Conn)
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			defer recoverAndLog("panic forwarding agent connection", nil, nil)
			defer func() { _ = conn.Close() }()
			channel, reqs, err := sshConn.OpenChannel(agentChannelType, nil)
			if err != nil {
				return
			}
			defer func() { _ = channel.Close() }()
			go gossh.DiscardRequests(reqs)
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				// Done is deferred ahead of the recover so that it runs even
				// if the copy panics. Otherwise the recover would turn a crash
				// into a permanently blocked Wait below, leaking this
				// goroutine along with the socket and the channel.
				defer wg.Done()
				defer recoverAndLog("panic proxying agent connection", nil, nil)
				_, _ = io.Copy(conn, channel)
				_ = conn.(*net.UnixConn).CloseWrite()
			}()
			go func() {
				defer wg.Done()
				defer recoverAndLog("panic proxying agent connection", nil, nil)
				_, _ = io.Copy(channel, conn)
				_ = channel.CloseWrite()
			}()
			wg.Wait()
		}(conn)
	}
}
