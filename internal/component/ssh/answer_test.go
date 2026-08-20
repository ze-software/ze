package ssh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// answerServer starts an SSH server whose exec executor answers what respond
// returns, so a test drives the whole path: client, channel, execMiddleware,
// pipe chain, renderer and frame.
func answerServer(t *testing.T, respond func(input string) (*plugin.Response, error)) *Server {
	t.Helper()
	srv, err := NewServer(Config{
		Listen:        "127.0.0.1:0",
		HostKeyPath:   t.TempDir() + "/test_host_key",
		Authenticator: passwordProfilesAuthenticator{},
	})
	require.NoError(t, err)
	srv.SetExecutorFactory(func(_, _ string, _ plugin.Authorizer) CommandExecutor {
		return func(input string) (*plugin.RenderedResponse, error) {
			resp, respErr := respond(input)
			if respErr != nil {
				return &plugin.RenderedResponse{Response: resp}, respErr
			}
			if _, generated := plugin.RecordRows(resp); generated {
				// The exec channel walks the generator itself, exactly as the
				// daemon's own executor leaves it to (cmd/ze/hub).
				return &plugin.RenderedResponse{Response: resp}, nil
			}
			output, renderErr := plugin.ResponseJSON(resp, nil)
			return &plugin.RenderedResponse{Output: output, Response: resp}, renderErr
		}
	})
	require.NoError(t, srv.Start(t.Context(), nil, nil))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(ctx))
	})
	return srv
}

// answerCredentials are the credentials a test client connects with.
func answerCredentials(t *testing.T, srv *Server) sshclient.Credentials {
	t.Helper()
	host, port, err := net.SplitHostPort(srv.Address())
	require.NoError(t, err)
	return sshclient.Credentials{Host: host, Port: port, Username: "operator", Auth: "read-pass"}
}

// commandRecords answers a generator of count command-list rows, and reports
// how many rows the walk produced.
func commandRecords(count int, produced *int) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		for i := range count {
			var b textbuf.Buffer
			b.Str(`{"value":"show cmd-`).Int(int64(i)).Str(`"}`)
			if produced != nil {
				*produced++
			}
			if !yield(rpc.Record{Item: json.RawMessage(b.String())}) {
				return
			}
		}
	}
}

// TestARecordAnswerReachesTheOperatorOverTheExecChannel drives a row generator
// from a handler to a client over a real SSH connection.
//
// One command is driven at three row counts: one row, one under the threshold,
// and one over it. The reader is the same call each time and it branches on
// nothing, which is what AC-2 asks for. The rendering is compared against the
// document the same rows collapse to, so a bounded answer proves it did not
// change and a streamed one proves it says the same thing.
//
// VALIDATES: AC-1, AC-1b, AC-1c, AC-2, AC-6 of the streaming answer protocol.
// PREVENTS:  the exec channel building the whole answer before writing a byte,
//
//	which is the materialization this protocol exists to remove, and the
//	answer changing shape at the row count where it starts streaming.
func TestARecordAnswerReachesTheOperatorOverTheExecChannel(t *testing.T) {
	t.Setenv("ze.cli.format", "text")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Records{Key: "commands", Rows: commandRecords(recordRowsWanted, nil)},
		}, nil
	})
	creds := answerCredentials(t, srv)

	for _, rows := range []int{1, rpc.AnswerBufferThreshold - 1, rpc.AnswerBufferThreshold + 1} {
		t.Run(textbuf.StringInt(int64(rows))+" rows", func(t *testing.T) {
			recordRowsWanted = rows

			var body strings.Builder
			answer, err := sshclient.ExecCommandStream(creds, "system command list | ndjson", &body)
			require.NoError(t, err)

			assert.Equal(t, rpc.VerdictDone, answer.Verdict)
			assert.Equal(t, uint64(rows), answer.Count, "the terminator must count the records the operator received")

			lines := strings.Split(strings.TrimRight(body.String(), "\n"), "\n")
			require.Len(t, lines, rows, "one line for each record")
			for i, line := range lines {
				assert.Equal(t, `{"value":"show cmd-`+textbuf.StringInt(int64(i))+`"}`, line)
			}
			assert.NotContains(t, body.String(), `"commands"`,
				"ndjson renders the records, not the envelope they collapse under")
		})
	}
}

// recordRowsWanted is how many rows the server's handler answers with. The
// server is started once and the count changes per subtest, which is what makes
// every subtest read ONE command through ONE reader.
var recordRowsWanted = 1

// TestAnUnknownCommandAnswersTheErrorVerb drives a command text that names no
// command, over a real connection, and reads what the client is told.
//
// VALIDATES: AC-4 -- the verb is error, the answer carries message=, and it is
//
//	the only line, so a client can offer completion here rather than
//	reporting a command that ran and failed.
//
// PREVENTS:  a typo reaching the operator as an operational failure, which
//
//	sends them to debug a command that never existed.
func TestAnUnknownCommandAnswersTheErrorVerb(t *testing.T) {
	srv := answerServer(t, func(input string) (*plugin.Response, error) {
		// The dispatcher wraps the sentinel rather than restating its text
		// (dispatchPlugin, internal/component/plugin/server/command.go), and it
		// is the wrap that the exec channel reads.
		return nil, fmt.Errorf("unknown command: %s: %w", input, pluginserver.ErrUnknownCommand)
	})
	creds := answerCredentials(t, srv)

	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(creds, "shwo bgp peers", &body)

	require.Error(t, err)
	assert.Equal(t, rpc.VerdictError, answer.Verdict, "the error verb ends the answer without a terminator")
	assert.Contains(t, err.Error(), "unknown command: shwo bgp peers")
	assert.Empty(t, body.String(), "a command that names no command renders nothing")
	assert.Zero(t, answer.Count)
}

// TestAFailedCommandCarriesItsMessageOnTheTerminator covers the other failure a
// declared client must tell apart: a command that was understood, ran, and
// failed.
//
// VALIDATES: AC-5 -- the head states status=error and the terminator carries
//
//	the operational message with count=0.
//
// PREVENTS:  a client treating an operational failure as a typo, and offering
//
//	completion for a command that exists.
func TestAFailedCommandCarriesItsMessageOnTheTerminator(t *testing.T) {
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return nil, errors.New("peer 10.0.0.1 not configured")
	})
	creds := answerCredentials(t, srv)

	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(creds, "show bgp peer 10.0.0.1", &body)

	require.Error(t, err)
	assert.Equal(t, "peer 10.0.0.1 not configured", err.Error())
	assert.Equal(t, rpc.VerdictAborted, answer.Verdict, "a stated message is what makes the walk aborted")
	assert.Zero(t, answer.Count)
}

// TestAnUndeclaredClientReadsTodaysBytes is the negotiation control for the
// exec channel: a client that declares nothing must see the rendering and no
// frame at all.
//
// VALIDATES: AC-13 for the exec channel -- `ssh <host> <command>` is unchanged.
// PREVENTS:  every script that pipes `ssh host 'show ...'` into a parser
//
//	suddenly reading frame lines on its stderr.
func TestAnUndeclaredClientReadsTodaysBytes(t *testing.T) {
	t.Setenv("ze.cli.format", "text")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	recordRowsWanted = 3
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Records{Key: "commands", Rows: commandRecords(recordRowsWanted, nil)},
		}, nil
	})
	creds := answerCredentials(t, srv)

	// ExecCommand declares nothing and reads stdout and stderr together, so a
	// frame line would land in what it returns.
	combined, err := sshclient.ExecCommand(creds, "system command list | ndjson")
	require.NoError(t, err)

	assert.Equal(t, 3, strings.Count(combined, "\n")+1, "three records, three lines and nothing else")
	assert.NotContains(t, combined, "status=")
	assert.NotContains(t, combined, "count=")
}

// TestAnAnswerCutMidStreamReportsTruncation is AC-9, end to end.
//
// The method is a TCP relay between the client and the daemon that stops
// forwarding after a fixed number of bytes. The cut is by byte count and not by
// time, so the answer is always cut in the same place: after the head and part
// of the body, and before the terminator. The client must then report a short
// answer rather than the records it received.
//
// The cut point is measured in the same test, from a complete run through the
// same relay, so it cannot drift with the rendering.
//
// VALIDATES: AC-9 -- a connection that dies before the terminator is reported
//
//	as truncation, not as a complete answer.
//
// PREVENTS:  a consumer taking half a routing table for the whole of it, which
//
//	is the failure the mandatory terminator exists to make impossible.
func TestAnAnswerCutMidStreamReportsTruncation(t *testing.T) {
	t.Setenv("ze.cli.format", "text")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	const rows = rpc.AnswerBufferThreshold + 2000
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Records{Key: "commands", Rows: commandRecords(rows, nil)},
		}, nil
	})

	// The complete run: it measures how many bytes a whole answer costs on the
	// wire, and it proves the relay itself does not break the answer.
	whole := newCutRelay(t, srv.Address(), 0)
	var full strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, whole.server()), "system command list | ndjson", &full)
	require.NoError(t, err, "the relay must carry a complete answer")
	require.Equal(t, rpc.VerdictDone, answer.Verdict)
	require.Equal(t, uint64(rows), answer.Count)
	forwarded := whole.forwarded()
	require.Positive(t, forwarded)

	// The cut run: the same answer over a relay that stops half way.
	cut := newCutRelay(t, srv.Address(), forwarded/2)
	var partial strings.Builder
	short, cutErr := sshclient.ExecCommandStream(answerCredentials(t, cut.server()), "system command list | ndjson", &partial)

	require.Error(t, cutErr, "an answer with no terminator must be an error")
	assert.ErrorIs(t, cutErr, sshclient.ErrAnswerTruncated)
	assert.Equal(t, rpc.VerdictTruncated, short.Verdict)
	assert.Less(t, len(partial.String()), len(full.String()),
		"the cut answer must be short; a full one would mean the relay cut nothing")
}

// cutRelay is a TCP relay that forwards a connection and stops after limit
// bytes have traveled from the daemon to the client. A limit of zero forwards
// everything.
//
// It exists because truncation is a property of the TRANSPORT, and a test that
// killed the daemon at a chosen moment would be timing-dependent. Counting
// bytes makes the cut land in the same place on every run.
type cutRelay struct {
	listener net.Listener
	target   string
	limit    int
	dial     *net.Dialer
	mu       sync.Mutex
	count    int
}

// newCutRelay starts a relay in front of target and returns it. It stops when
// the test ends.
func newCutRelay(t *testing.T, target string, limit int) *cutRelay {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	relay := &cutRelay{listener: listener, target: target, limit: limit, dial: &net.Dialer{}}
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // the test is ending

	// One goroutine for the life of the relay, and one for the life of each
	// connection it accepts: the lifecycle shape ai/rules/goroutine-lifecycle.md
	// permits. Both end when the listener closes.
	go relay.accept()
	return relay
}

// server returns a Server-shaped view of the relay, so a test builds
// credentials for it the same way it does for a real one.
func (r *cutRelay) server() *Server {
	return &Server{listener: r.listener}
}

// forwarded reports how many bytes have traveled from the daemon to the
// client. Safe for concurrent use.
func (r *cutRelay) forwarded() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func (r *cutRelay) accept() {
	for {
		client, err := r.listener.Accept()
		if err != nil {
			return
		}
		go r.relay(client)
	}
}

func (r *cutRelay) relay(client net.Conn) {
	defer client.Close() //nolint:errcheck // the connection is ending
	daemon, err := r.dial.DialContext(context.Background(), "tcp", r.target)
	if err != nil {
		return
	}
	defer daemon.Close() //nolint:errcheck // the connection is ending

	// The client-to-daemon direction closes the daemon connection when it
	// ends. Without that the daemon never sees the client go away, keeps the
	// session open, and the server cannot stop.
	go func() {
		io.Copy(daemon, client) //nolint:errcheck // the connection is ending
		daemon.Close()          //nolint:errcheck // as above
	}()

	buf := make([]byte, 4096)
	for {
		n, readErr := daemon.Read(buf)
		if n > 0 {
			r.mu.Lock()
			r.count += n
			over := r.limit > 0 && r.count >= r.limit
			r.mu.Unlock()
			if _, writeErr := client.Write(buf[:n]); writeErr != nil {
				return
			}
			if over {
				return
			}
		}
		if readErr != nil {
			return
		}
	}
}
