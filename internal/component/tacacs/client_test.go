// Design: docs/architecture/aaa-tacacs.md -- the TACACS+ client and its server failover
// Related: client.go -- sendToServers, allServersFailed, serverFailure: the error this judges.

package tacacs

import (
	"errors"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/tacacs/yang"
	"github.com/ze-software/ze/internal/core/opererr"
)

// refusingAddress answers a loopback address nothing listens on: a listener
// is bound to learn a free port, then closed, so a dial there is refused.
func refusingAddress(t *testing.T) string {
	t.Helper()
	ln := listenTCP(t)
	address := ln.Addr().String()
	require.NoError(t, ln.Close())
	return address
}

// silentServer accepts connections and never answers, so the client's read
// deadline expires. At cleanup the listener closes, the worker returns, and
// every held connection closes.
func silentServer(t *testing.T) string {
	t.Helper()
	ln := listenTCP(t)
	held := make(chan net.Conn, 4)
	done := make(chan struct{})
	go acceptAndHold(ln, held, done)
	t.Cleanup(func() {
		closeIgnore(ln)
		<-done
		close(held)
		for conn := range held {
			closeIgnore(conn)
		}
	})
	return ln.Addr().String()
}

// acceptAndHold is silentServer's worker: it accepts until the listener
// closes and parks each connection unanswered, then closes done. A
// connection past the channel's depth is closed at once.
func acceptAndHold(ln net.Listener, held chan<- net.Conn, done chan<- struct{}) {
	defer close(done)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed
		}
		select {
		case held <- conn:
		default:
			closeIgnore(conn)
		}
	}
}

// TestTacacsAllServersFailedNamesEachCauseAndFix drives three servers that
// each fail in a different way and reads the error the client returns.
//
// VALIDATES: AC-6 -- when every TACACS+ server fails, the error names each
// server with its cause (refused, timeout, a reply that does not decode with
// the configured key), in the order tried, and the fix for the first server;
// each fix is a config line the TACACS+ schema accepts; no secret is echoed;
// errors.Is still reaches each server's own error.
// PREVENTS: "all TACACS+ servers unreachable for authentication", which named
// neither the servers, their causes, nor a fix.
func TestTacacsAllServersFailedNamesEachCauseAndFix(t *testing.T) {
	refused := refusingAddress(t)
	silent := silentServer(t)
	wrongKey := newTestServer(t, []byte("the-servers-own-key"), func(_ PacketHeader, _ []byte) []byte {
		return []byte{AuthenStatusPass, 0, 0, 0, 0, 0}
	})
	t.Cleanup(wrongKey.close)

	const clientKey = "ze-configured-secret-value"
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{
			{Address: refused, Key: []byte(clientKey)},
			{Address: silent, Key: []byte(clientKey)},
			{Address: wrongKey.addr(), Key: []byte(clientKey)},
		},
		Timeout: 200 * time.Millisecond,
	})
	t.Cleanup(client.Close)

	reply, err := client.Authenticate("admin", "secret", "ssh", "10.0.0.1")
	require.Error(t, err)
	assert.Nil(t, reply)

	refusedHost, _, splitErr := net.SplitHostPort(refused)
	require.NoError(t, splitErr)
	want := "no TACACS+ server answered the authentication request\n" +
		"Ze tried each configured server in order, and each one failed: " +
		refused + " refused the connection; " +
		silent + " did not answer in time; " +
		wrongKey.addr() + " sent a reply that does not decode with the configured key\n" +
		"fix: set system authentication tacacs server " + refusedHost + " port <the port the server listens on>"
	assert.Equal(t, want, err.Error())

	var operr *opererr.Error
	require.True(t, errors.As(err, &operr), "the error is an operator error")
	assert.ErrorIs(t, err, syscall.ECONNREFUSED)
	assert.ErrorIs(t, err, ErrBadSecret)
	assert.NotContains(t, err.Error(), clientKey)
	assert.NotContains(t, err.Error(), "the-servers-own-key")
}

// TestTacacsServerFailureFixResolves reads the fix of each cause class and
// parses every config fix through the TACACS+ schema.
//
// VALIDATES: spec R-2 -- each fix names a config path that exists; the
// timeout fix is `show ping <host>` (ze-ping-cmd.yang, show ping, the
// destination as the first bare word) and any other error sends the operator
// to `ze doctor`, whose tacacs-servers check names the cause.
// PREVENTS: a fix line naming a command or config path Ze does not have.
func TestTacacsServerFailureFixResolves(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	timeoutErr := &net.OpError{Op: "read", Net: "tcp", Err: timeoutError{}}
	tests := []struct {
		name  string
		err   error
		fix   string
		value string // replaces the placeholder of a config fix; "" for a command fix
	}{
		{"refused", syscall.ECONNREFUSED, "set system authentication tacacs server 192.0.2.1 port <the port the server listens on>", "4949"},
		{"bad secret", ErrBadSecret, "set system authentication tacacs server 192.0.2.1 key <the server's shared secret>", `"server-key"`},
		{"timeout", timeoutErr, "show ping 192.0.2.1", ""},
		{"other", errors.New("session ID mismatch"), "ze doctor", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure := serverFailure{address: "192.0.2.1:49", err: tt.err}
			fix := failure.fix()
			assert.Equal(t, tt.fix, fix)
			if tt.value == "" {
				return
			}
			open := strings.IndexByte(fix, '<')
			require.Positive(t, open, "a config fix carries one placeholder")
			line := fix[:open] + tt.value
			_, parseErr := config.NewSetParser(schema).Parse(line + "\n")
			assert.NoError(t, parseErr, "the fix %q is a config line the schema accepts", line)
		})
	}
}

// timeoutError is a net.Error whose Timeout answers true, as a read past
// its deadline returns.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
