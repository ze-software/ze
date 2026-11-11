package server

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"
	"time"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// authAs dials the managed server, sends one auth frame, and reports whether
// the server answered ok. A connection the server closed without a reply is a
// refusal.
func authAs(t *testing.T, addr, name, token string) bool {
	t.Helper()
	dialer := &tls.Dialer{Config: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // test-only; self-signed server cert
		MinVersion:         tls.VersionTLS13,
	}}
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer dialCancel()
	conn, err := dialer.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck // test cleanup
	if authErr := pluginipc.SendAuth(context.Background(), conn, token, name); authErr != nil {
		t.Fatalf("send auth: %v", authErr)
	}
	line, readErr := pluginipc.ReadLineRaw(conn, 512)
	if readErr != nil {
		return false
	}
	_, verb, _, _ := rpc.ParseLine(line)
	return verb == "ok"
}

// waitForRejection waits until the server's doctor record holds a rejection
// of client, or fails the test.
func waitForRejection(t *testing.T, srv *ManagedServer, client string) ManagedClientRejection {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, rejection := range srv.doctor.snapshot().Rejected {
			if rejection.Client == client {
				return rejection
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no rejection of %q recorded: %+v", client, srv.doctor.snapshot().Rejected)
	return ManagedClientRejection{}
}

// TestManagedUnknownNameEmptyTokenRefused is the managed half of the empty
// expected token guard. The managed server authenticates with no shared
// secret, so before AuthenticateWithLookup refused an empty expected token, a
// name with no client entry that sent an empty token authenticated. Method:
// dial the real listener as "intruder" with no token and expect no ok, then
// read the doctor record for the claimed name under ManagedRejectUnknownName;
// a configured client with a wrong token is recorded under
// ManagedRejectBadToken without the token; the configured client with its own
// secret still authenticates.
func TestManagedUnknownNameEmptyTokenRefused(t *testing.T) {
	srv, addr := startTestManagedServer(t)

	if authAs(t, addr, "intruder", "") {
		t.Fatal("an unknown name with an empty token authenticated")
	}
	if got := waitForRejection(t, srv, "intruder"); got.Cause != ManagedRejectUnknownName {
		t.Errorf("intruder rejection = %+v, want ManagedRejectUnknownName", got)
	}

	const wrongToken = "wrong-token-wrong-token-wrong-32"
	if authAs(t, addr, testClientName, wrongToken) {
		t.Fatal("a configured client with a wrong token authenticated")
	}
	got := waitForRejection(t, srv, testClientName)
	if got.Cause != ManagedRejectBadToken {
		t.Errorf("%s rejection = %+v, want ManagedRejectBadToken", testClientName, got)
	}
	if strings.Contains(got.Detail, wrongToken) {
		t.Errorf("rejection detail %q carries the token", got.Detail)
	}

	_ = dialTestClient(t, addr, testClientName, testClientSecret)
}

// TestManagedUnknownNamesBounded proves the unknown-name record keeps at most
// managedUnknownMax names: whoever connects chooses the name, so an unbounded
// map would let a client grow the hub's memory one claimed name at a time.
func TestManagedUnknownNamesBounded(t *testing.T) {
	var d managedDoctorState
	for i := range managedUnknownMax + 4 {
		d.recordUnknown("name-"+strings.Repeat("x", i), "refused")
	}
	record := d.snapshot()
	if len(record.Rejected) != managedUnknownMax {
		t.Fatalf("kept %d unknown names, bound is %d", len(record.Rejected), managedUnknownMax)
	}
	last := "name-" + strings.Repeat("x", managedUnknownMax+3)
	d.recordUnknown(last, "refused again")
	if got := len(d.snapshot().Rejected); got != managedUnknownMax {
		t.Errorf("re-recording a kept name changed the count to %d", got)
	}
}
