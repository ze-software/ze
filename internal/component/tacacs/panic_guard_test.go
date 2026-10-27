// Design: docs/architecture/aaa-tacacs.md -- panic containment at the request entry
// Related: panic_guard.go -- requestTask, the boundary under test
// Related: client.go -- trySend, whose deferred close runs on the unwind
// Related: rfc8907_connection_test.go -- sessionServer, the server under the test

package tacacs

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// TestTACACSClientPanicContained proves a TACACS+ request contains a Ze defect
// met mid-exchange: the request answers the *taskguard.Panic as an error, so
// the AAA chain treats the server as unreachable, the connection it used is
// closed, the next request on the same client completes, and the panic is
// counted and crash-recorded on the real guard.
// METHOD: the client's pool map is nil, a state only a defect produces since
// NewTacacsClient always makes it. Server A echoes TAC_PLUS_SINGLE_CONNECT_FLAG,
// so the fresh connection is promoted and storeConn writes into the nil map
// with the exchange complete and the connection open. Server B does not echo,
// so the next request closes its connection and never reaches the pool.
// PREVENTS: a panic in a login killing the daemon, and the unwind leaving the
// connection open: RFC 8907 Section 4.4 forbids new sessions on a connection
// the client cannot trust.
func TestTACACSClientPanicContained(t *testing.T) {
	before, declared := requestGuard.Panics(taskRequest)
	if !declared {
		t.Fatalf("task %q is not declared on the tacacs boundary", taskRequest)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	defective := newSessionServer(t, sessionKey, true)
	healthy := newSessionServer(t, sessionKey, false)
	client := newSessionClient(t, defective, sessionKey)
	client.pool = nil

	reply, err := client.Authenticate("alice", "hunter2", "ssh", "192.0.2.1")
	if _, contained := errors.AsType[*taskguard.Panic](err); !contained {
		t.Fatalf("Authenticate answered (%v, %v), want a *taskguard.Panic error", reply, err)
	}
	conns := defective.connections()
	if len(conns) != 1 {
		t.Fatalf("server A saw %d connections, want 1", len(conns))
	}
	waitClosed(t, conns[0])

	client.config.Servers = []TacacsServer{{Address: healthy.addr(), Key: sessionKey}}
	authenticatePass(t, client)

	if count, _ := requestGuard.Panics(taskRequest); count != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d", taskRequest, count, before+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	want := "tacacs " + taskRequest + ", unit authentication request"
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
}
