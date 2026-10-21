// Design: docs/architecture/testing/ci-format.md -- option=open:value=inspect-open-message
// Related: peer.go -- doOpenHandshake, the code under test

package peer

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// ceaseConnectionRejected is NOTIFICATION Cease (6), Connection Rejected (5),
// with no data: what a held-down speaker answers before any OPEN.
const ceaseConnectionRejected = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0015030605"

// TestPreOpenNotificationIsAnExpectation proves that under inspect-open-message
// a NOTIFICATION sent in place of the OPEN is judged against the expect script,
// and that the peer answers it with nothing.
//
// The method is a socket pair on which the far side plays a daemon that refuses
// the connection. It writes the NOTIFICATION, then reads: any message there is
// an OPEN the peer should not have sent, and EOF (the test closing the peer's
// end after handleConnection returns) is the pass.
//
// VALIDATES: a .ci can assert the Cease a speaker sends before the OPEN
//
//	exchange (test/plugin/peer-admin-shutdown-inbound.ci).
//
// PREVENTS: "expected OPEN, got type 3" for a refusal the script expects.
func TestPreOpenNotificationIsAnExpectation(t *testing.T) {
	result, err := runRefusedConnection(t, true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("peer failed: %v", result.Error)
	}
}

// TestPreOpenNotificationWithoutInspectFails proves the default is unchanged:
// without inspect-open-message the first message must be an OPEN, so a script
// that does not ask for the inspection still fails on a refusal.
func TestPreOpenNotificationWithoutInspectFails(t *testing.T) {
	result, err := runRefusedConnection(t, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("result.Success = true, want false without inspect-open-message")
	}
	if !strings.Contains(result.Error.Error(), "expected OPEN") {
		t.Fatalf("error = %v, want it to name the missing OPEN", result.Error)
	}
}

// runRefusedConnection runs one check-mode connection against a daemon that
// refuses it, and returns the peer's result and the daemon side's verdict.
func runRefusedConnection(t *testing.T, inspect bool) (Result, error) {
	t.Helper()
	peer, err := New(&Config{
		Mode:               ModeCheck,
		Expect:             []string{"expect=bgp:conn=1:seq=1:hex=" + ceaseConnectionRejected},
		InspectOpenMessage: inspect,
		Output:             new(bytes.Buffer),
	})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}

	local, remote := net.Pipe()
	defer remote.Close() //nolint:errcheck // test socket

	answered := make(chan error, 1)
	go func() { answered <- refuseConnection(remote) }()

	result := peer.handleConnection(t.Context(), local)
	if err := local.Close(); err != nil {
		t.Fatalf("close peer end: %v", err)
	}
	return result, <-answered
}

// refuseConnection writes Cease, Connection Rejected, then reads once. A message
// read back is an error: a refused speaker gets no OPEN.
func refuseConnection(conn net.Conn) error {
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	notification, err := hex.DecodeString(ceaseConnectionRejected)
	if err != nil {
		return err
	}
	if _, err := conn.Write(notification); err != nil {
		return err
	}
	header, _, err := ReadMessage(conn)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if errors.Is(err, io.ErrClosedPipe) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("peer answered a refused connection with message type %d", header[18])
}
