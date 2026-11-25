package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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

func dialFromLoopback(t *testing.T, localIP, targetAddr string) net.Conn {
	t.Helper()
	d := &net.Dialer{
		LocalAddr: &net.TCPAddr{IP: net.ParseIP(localIP)},
		Timeout:   2 * time.Second,
	}
	conn, err := d.Dial("tcp", targetAddr)
	require.NoError(t, err)
	return conn
}

// VALIDATES: AC-47 and A-14 -- a real TLS client whose root pool lacks the hub's
// authority fails its handshake with a certificate-rejection TLS alert, recorded
// in CertRefused under the remote IP without port; a second failure increments
// count and updates last time.
func TestManagedCertificateRefusedByClientRecorded(t *testing.T) {
	srv, addr := startTestManagedServer(t)

	// Real TLS client with an empty root pool rejects the hub's leaf.
	for attempt := 1; attempt <= 2; attempt++ {
		raw := dialFromLoopback(t, "127.0.0.2", addr)
		tlsConn := tls.Client(raw, &tls.Config{
			ServerName: "127.0.0.1",
			RootCAs:    x509.NewCertPool(),
			MinVersion: tls.VersionTLS13,
		})
		require.Error(t, tlsConn.Handshake())
		_ = tlsConn.Close()

		require.Eventually(t, func() bool {
			snap := srv.doctor.snapshotAt(srv.now())
			return len(snap.CertRefused) == 1 && snap.CertRefused[0].Count == attempt
		}, 2*time.Second, 10*time.Millisecond)
	}

	snap := srv.doctor.snapshotAt(srv.now())
	require.Len(t, snap.CertRefused, 1)
	entry := snap.CertRefused[0]
	require.Equal(t, "127.0.0.2", entry.Addr)
	require.Equal(t, 2, entry.Count)
	require.False(t, entry.LastTime.Before(entry.FirstTime))
	require.Contains(t, entry.LastError, "bad certificate")
	require.Empty(t, snap.Unidentified)
}

// VALIDATES: AC-47 and A-14 -- handshakes that fail for reasons other than
// certificate rejection (non-certificate fatal TLS alerts producing *net.OpError
// with Op=="remote error", incompatible TLS version mismatch, non-TLS bytes, and
// plain TCP close) are recorded under Unidentified (doctor-managed-client-rejected)
// and NEVER under CertRefused (doctor-managed-certificate-refused).
func TestManagedNonCertificateHandshakeFailureReportedAsClientRejected(t *testing.T) {
	// Direct classification check: non-certificate alerts are false; the 6 RFC 8446 cert alerts are true.
	for _, code := range []uint8{42, 43, 44, 45, 46, 48} {
		err := &net.OpError{Op: "remote error", Err: tls.AlertError(code)}
		require.True(t, isCertRejectionAlert(err), "alert %d must classify as certificate rejection", code)
	}
	for _, code := range []uint8{10, 40, 47, 50, 51, 70, 71, 80, 110, 112} {
		err := &net.OpError{Op: "remote error", Err: tls.AlertError(code)}
		require.False(t, isCertRejectionAlert(err), "non-cert alert %d must NOT classify as certificate rejection", code)
	}

	srv, addr := startTestManagedServer(t)

	// 1. Peer sends a real non-certificate fatal TLS alert (handshake_failure = 40),
	// which crypto/tls reads as *net.OpError{Op: "remote error", Err: alertHandshakeFailure}.
	alertConn := dialFromLoopback(t, "127.0.0.3", addr)
	_, err := alertConn.Write([]byte{0x15, 0x03, 0x03, 0x00, 0x02, 0x02, 0x28})
	require.NoError(t, err)
	_ = alertConn.Close()

	// 2. Incompatible TLS version (TLS 1.2 against TLS 1.3-only managed listener).
	oldTLS := dialFromLoopback(t, "127.0.0.4", addr)
	tls12 := tls.Client(oldTLS, &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // test forces TLS 1.2 mismatch
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS12,
	})
	require.Error(t, tls12.Handshake())
	_ = tls12.Close()

	// 3. Non-TLS bytes and plain TCP close.
	plainConn := dialFromLoopback(t, "127.0.0.5", addr)
	_, err = plainConn.Write([]byte("GET / HTTP/1.1\r\n\r\n"))
	require.NoError(t, err)
	_ = plainConn.Close()

	eofConn := dialFromLoopback(t, "127.0.0.6", addr)
	_ = eofConn.Close()

	require.Eventually(t, func() bool {
		snap := srv.doctor.snapshotAt(srv.now())
		return len(snap.Unidentified) == 4
	}, 2*time.Second, 10*time.Millisecond)

	snap := srv.doctor.snapshotAt(srv.now())
	require.Empty(t, snap.CertRefused, "non-certificate handshake failures must never enter CertRefused")
	addrs := make([]string, 0, len(snap.Unidentified))
	for _, entry := range snap.Unidentified {
		addrs = append(addrs, entry.Addr)
		require.NotEmpty(t, entry.LastError)
	}
	require.Equal(t, []string{"127.0.0.3", "127.0.0.4", "127.0.0.5", "127.0.0.6"}, addrs)
}

// VALIDATES: AC-47, R-21, R-22 -- each unidentified class bounds at 16 addresses,
// forgets the oldest and counts forgotten, and the two classes do not displace each other.
func TestManagedUnidentifiedBoundPerClass(t *testing.T) {
	var d managedDoctorState
	base := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

	for i := range managedUnidentifiedMax + 1 {
		addr := "127.0.0." + strconv.Itoa(i+2)
		d.recordCertRefused(addr, "127.0.0.1:1791", "remote error: tls: bad certificate", base.Add(time.Duration(i)*time.Second))
	}
	for i := range managedUnidentifiedMax + 1 {
		addr := "127.0.1." + strconv.Itoa(i+2)
		d.recordUnidentified(addr, "127.0.0.1:1791", "read auth request: EOF", base.Add(time.Duration(i)*time.Second))
	}

	snap := d.snapshotAt(base.Add(time.Minute))
	require.Len(t, snap.CertRefused, managedUnidentifiedMax)
	require.Equal(t, 1, snap.CertRefusedForgotten)
	for _, e := range snap.CertRefused {
		require.NotEqual(t, "127.0.0.2", e.Addr, "oldest address must be evicted")
	}

	require.Len(t, snap.Unidentified, managedUnidentifiedMax)
	require.Equal(t, 1, snap.UnidentifiedForgotten)
	for _, e := range snap.Unidentified {
		require.NotEqual(t, "127.0.1.2", e.Addr, "oldest address must be evicted")
	}
}

// VALIDATES: AC-47 -- an authenticated session from an address clears both
// nameless-connection failure classes for that address.
func TestManagedUnidentifiedClearedByAuthentication(t *testing.T) {
	srv, addr := startTestManagedServer(t)

	// Record both classes for 127.0.0.1 before authenticating from 127.0.0.1.
	now := srv.now()
	srv.doctor.recordCertRefused("127.0.0.1", addr, "remote error: tls: bad certificate", now)
	srv.doctor.recordUnidentified("127.0.0.1", addr, "read auth request: EOF", now)
	require.Len(t, srv.doctor.snapshotAt(now).CertRefused, 1)
	require.Len(t, srv.doctor.snapshotAt(now).Unidentified, 1)

	_ = dialTestClient(t, addr, testClientName, testClientSecret)
	require.Eventually(t, func() bool {
		snap := srv.doctor.snapshotAt(srv.now())
		return len(snap.CertRefused) == 0 && len(snap.Unidentified) == 0
	}, 2*time.Second, 10*time.Millisecond)
}

// VALIDATES: AC-47, R-23 -- an entry is still visible one second before
// managedUnidentifiedKeep and forgotten at managedUnidentifiedKeep.
func TestManagedUnidentifiedForgottenAfterKeep(t *testing.T) {
	base := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	var d managedDoctorState
	d.recordCertRefused("192.0.2.10", "127.0.0.1:1791", "remote error: tls: bad certificate", base)
	d.recordUnidentified("192.0.2.11", "127.0.0.1:1791", "read auth request: EOF", base)

	snap := d.snapshotAt(base.Add(managedUnidentifiedKeep - time.Second))
	require.Len(t, snap.CertRefused, 1)
	require.Len(t, snap.Unidentified, 1)

	snap = d.snapshotAt(base.Add(managedUnidentifiedKeep))
	require.Empty(t, snap.CertRefused)
	require.Empty(t, snap.Unidentified)
}
