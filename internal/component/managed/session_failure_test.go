// VALIDATES: fleet-1 AC-43, AC-44, AC-45, AC-46, and A-13 -- real TLS handshakes
// classify every hub session failure by x509/tls error type, log the code and fix
// on reconnect, clear on auth, and surface through the hub-session runtime check.
// PREVENTS: a managed client silently looping on certificate refusal, handshake
// failure, or auth refusal with no doctor finding or reconnect code.

package managed

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log/slog"
	"math/big"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/pki"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func issueTestCAAndLeaf(t *testing.T, dnsNames []string, ips []net.IP, notBefore, notAfter time.Time) (*x509.Certificate, tls.Certificate) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-fleet-ca"},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(48 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-fleet-hub"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dnsNames,
		IPAddresses:  ips,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	require.NoError(t, err)
	leafCert, err := x509.ParseCertificate(leafDER)
	require.NoError(t, err)
	return caCert, tls.Certificate{
		Certificate: [][]byte{leafDER, caDER},
		PrivateKey:  leafKey,
		Leaf:        leafCert,
	}
}

func startTLSRejectServer(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var buf [1]byte
				_, _ = c.Read(buf[:])
			}(conn)
		}
	}()
	return ln.Addr().String()
}

// VALIDATES: AC-43, AC-44 and A-13 -- real TLS handshakes produce typed x509
// verification errors classified into unknown authority, hostname mismatch,
// validity dates, and undefined pki ca anchor (without dialing).
func TestRunConnectionClassesCertificateFailure(t *testing.T) {
	now := time.Now()
	anchorCA, _ := issueTestCAAndLeaf(t, nil, []net.IP{net.ParseIP("127.0.0.1")}, now.Add(-time.Hour), now.Add(time.Hour))

	// 1. Unknown issuer: leaf signed by a different CA than client's HubRoot.
	_, foreignLeaf := issueTestCAAndLeaf(t, nil, []net.IP{net.ParseIP("127.0.0.1")}, now.Add(-time.Hour), now.Add(time.Hour))
	foreignAddr := startTLSRejectServer(t, foreignLeaf)
	var got HubSessionFailure
	cfg := &ClientConfig{
		Name: "edge-01", Server: foreignAddr, Token: testHubToken, HubRoot: anchorCA,
		ConnectTimeout:   2 * time.Second,
		OnSessionFailure: func(f HubSessionFailure) { got = f },
	}
	require.Error(t, runConnection(t.Context(), cfg, newBackoff(time.Second, time.Minute, 0)))
	require.Equal(t, HubSessionStepHandshake, got.Step)
	require.Equal(t, HubSessionCauseUnknownAuthority, got.Cause)
	require.Equal(t, HubTrustAnchorStoredRoot, got.Anchor)
	require.Equal(t, foreignAddr, got.Server)

	// 2. Hostname mismatch: leaf signed by trusted CA but only for 127.0.0.2.
	hostCA, hostLeaf := issueTestCAAndLeaf(t, []string{"hub.example.net"}, []net.IP{net.ParseIP("127.0.0.2")}, now.Add(-time.Hour), now.Add(time.Hour))
	hostAddr := startTLSRejectServer(t, hostLeaf)
	got = HubSessionFailure{}
	cfg = &ClientConfig{
		Name: "edge-01", Server: hostAddr, Token: testHubToken, HubRoot: hostCA,
		ConnectTimeout:   2 * time.Second,
		OnSessionFailure: func(f HubSessionFailure) { got = f },
	}
	require.Error(t, runConnection(t.Context(), cfg, newBackoff(time.Second, time.Minute, 0)))
	require.Equal(t, HubSessionStepHandshake, got.Step)
	require.Equal(t, HubSessionCauseHostnameMismatch, got.Cause)
	require.Contains(t, got.CertNames, "hub.example.net")
	require.Contains(t, got.CertNames, "127.0.0.2")

	// 3. Expired leaf.
	expNB, expNA := now.Add(-48*time.Hour), now.Add(-time.Hour)
	expCA, expLeaf := issueTestCAAndLeaf(t, nil, []net.IP{net.ParseIP("127.0.0.1")}, expNB, expNA)
	expAddr := startTLSRejectServer(t, expLeaf)
	got = HubSessionFailure{}
	cfg = &ClientConfig{
		Name: "edge-01", Server: expAddr, Token: testHubToken, HubRoot: expCA,
		ConnectTimeout:   2 * time.Second,
		OnSessionFailure: func(f HubSessionFailure) { got = f },
	}
	require.Error(t, runConnection(t.Context(), cfg, newBackoff(time.Second, time.Minute, 0)))
	require.Equal(t, HubSessionStepHandshake, got.Step)
	require.Equal(t, HubSessionCauseValidity, got.Cause)
	require.Equal(t, expNB.UTC().Truncate(time.Second), got.CertNotBefore.UTC())
	require.Equal(t, expNA.UTC().Truncate(time.Second), got.CertNotAfter.UTC())

	// 4. Not-yet-valid leaf.
	futNB, futNA := now.Add(2*time.Hour), now.Add(24*time.Hour)
	futCA, futLeaf := issueTestCAAndLeaf(t, nil, []net.IP{net.ParseIP("127.0.0.1")}, futNB, futNA)
	futAddr := startTLSRejectServer(t, futLeaf)
	got = HubSessionFailure{}
	cfg = &ClientConfig{
		Name: "edge-01", Server: futAddr, Token: testHubToken, HubRoot: futCA,
		ConnectTimeout:   2 * time.Second,
		OnSessionFailure: func(f HubSessionFailure) { got = f },
	}
	require.Error(t, runConnection(t.Context(), cfg, newBackoff(time.Second, time.Minute, 0)))
	require.Equal(t, HubSessionStepHandshake, got.Step)
	require.Equal(t, HubSessionCauseValidity, got.Cause)
	require.Equal(t, futNB.UTC().Truncate(time.Second), got.CertNotBefore.UTC())
	require.Equal(t, futNA.UTC().Truncate(time.Second), got.CertNotAfter.UTC())

	// 5. Undefined pki ca reports anchor failure without dialing.
	require.NoError(t, pki.Load(nil))
	got = HubSessionFailure{}
	cfg = &ClientConfig{
		Name: "edge-01", Server: "127.0.0.1:1", Token: testHubToken, CA: "missing-ca",
		ConnectTimeout:   2 * time.Second,
		OnSessionFailure: func(f HubSessionFailure) { got = f },
	}
	require.Error(t, runConnection(t.Context(), cfg, newBackoff(time.Second, time.Minute, 0)))
	require.Equal(t, HubSessionStepAnchor, got.Step)
	require.Equal(t, HubSessionCauseAnchorUndefined, got.Cause)
	require.Equal(t, HubTrustAnchorPKICA, got.Anchor)
	require.Equal(t, "missing-ca", got.CAName)
}

// VALIDATES: AC-45 -- handshake failures without verification error, auth
// rejections, and TCP connect failures each report their own step and cause.
func TestRunConnectionClassesHandshakeAndAuthFailure(t *testing.T) {
	// 1. Listener that closes immediately during TLS handshake.
	closeLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = closeLn.Close() })
	go func() {
		for {
			c, acceptErr := closeLn.Accept()
			if acceptErr != nil {
				return
			}
			_ = c.Close()
		}
	}()
	var got HubSessionFailure
	cfg := &ClientConfig{
		Name: "edge-01", Server: closeLn.Addr().String(), Token: testHubToken, TLSInsecure: true,
		ConnectTimeout:   2 * time.Second,
		OnSessionFailure: func(f HubSessionFailure) { got = f },
	}
	require.Error(t, runConnection(t.Context(), cfg, newBackoff(time.Second, time.Minute, 0)))
	require.Equal(t, HubSessionStepHandshake, got.Step)
	require.Equal(t, HubSessionCauseHandshakeOther, got.Cause)

	// 2. Plaintext non-TLS listener.
	plainLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = plainLn.Close() })
	go func() {
		for {
			c, acceptErr := plainLn.Accept()
			if acceptErr != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
			}(c)
		}
	}()
	got = HubSessionFailure{}
	cfg.Server = plainLn.Addr().String()
	require.Error(t, runConnection(t.Context(), cfg, newBackoff(time.Second, time.Minute, 0)))
	require.Equal(t, HubSessionStepHandshake, got.Step)
	require.Equal(t, HubSessionCauseHandshakeOther, got.Cause)

	// 3. Hub completes TLS and answers auth with error.
	now := time.Now()
	ca, leaf := issueTestCAAndLeaf(t, nil, []net.IP{net.ParseIP("127.0.0.1")}, now.Add(-time.Hour), now.Add(time.Hour))
	authLn, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{leaf}, MinVersion: tls.VersionTLS13})
	require.NoError(t, err)
	t.Cleanup(func() { _ = authLn.Close() })
	go func() {
		for {
			c, acceptErr := authLn.Accept()
			if acceptErr != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = pluginipc.AuthenticateWithLookup(t.Context(), conn, "", func(string) (string, bool) {
					return "different-secret-at-least-32-chars", true
				})
			}(c)
		}
	}()
	got = HubSessionFailure{}
	cfg = &ClientConfig{
		Name: "edge-01", Server: authLn.Addr().String(), Token: testHubToken, HubRoot: ca,
		ConnectTimeout:   2 * time.Second,
		OnSessionFailure: func(f HubSessionFailure) { got = f },
	}
	require.Error(t, runConnection(t.Context(), cfg, newBackoff(time.Second, time.Minute, 0)))
	require.Equal(t, HubSessionStepAuth, got.Step)
	require.Equal(t, HubSessionCauseAuthRejected, got.Cause)

	// 4. Refused TCP connect reports connect step.
	deadLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	deadAddr := deadLn.Addr().String()
	require.NoError(t, deadLn.Close())
	got = HubSessionFailure{}
	cfg.Server = deadAddr
	require.Error(t, runConnection(t.Context(), cfg, newBackoff(time.Second, time.Minute, 0)))
	require.Equal(t, HubSessionStepConnect, got.Step)
}

// VALIDATES: AC-46 -- after a failed attempt, an attempt that authenticates
// reports connected=true so the owner clears the stored failure.
func TestRunConnectionAuthenticatedClearsFailure(t *testing.T) {
	now := time.Now()
	ca, leaf := issueTestCAAndLeaf(t, nil, []net.IP{net.ParseIP("127.0.0.1")}, now.Add(-time.Hour), now.Add(time.Hour))
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{leaf}, MinVersion: tls.VersionTLS13})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			c, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, authErr := pluginipc.AuthenticateWithLookup(t.Context(), conn, "", func(name string) (string, bool) {
					if name == "edge-01" {
						return testHubToken, true
					}
					return "", false
				})
				if authErr != nil {
					return
				}
				var buf [1]byte
				_, _ = conn.Read(buf[:])
			}(c)
		}
	}()

	var lastFailure HubSessionFailure
	connectedCh := make(chan bool, 4)
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	// First attempt fails auth.
	badCfg := &ClientConfig{
		Name: "edge-01", Server: ln.Addr().String(), Token: "wrong-token-wrong-token-wrong-32", HubRoot: ca,
		ConnectTimeout: 2 * time.Second,
		Baseline:       NewBaselineAccess(store),
		OnSessionFailure: func(f HubSessionFailure) {
			lastFailure = f
		},
		OnConnection: func(connected bool) {
			if connected {
				lastFailure = HubSessionFailure{}
			}
			connectedCh <- connected
		},
	}
	require.Error(t, runConnection(t.Context(), badCfg, newBackoff(time.Second, time.Minute, 0)))
	require.Equal(t, HubSessionCauseAuthRejected, lastFailure.Cause)

	// Second attempt authenticates and clears the failure.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	goodCfg := *badCfg
	goodCfg.Token = testHubToken
	goodCfg.OnConnection = func(connected bool) {
		if connected {
			lastFailure = HubSessionFailure{}
			cancel()
		}
		connectedCh <- connected
	}
	_ = runConnection(ctx, &goodCfg, newBackoff(time.Second, time.Minute, 0))
	require.Equal(t, HubSessionStepNone, lastFailure.Step)
	require.True(t, <-connectedCh)
}

type syncLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// VALIDATES: AC-43 -- the reconnect Warn line carries the failure's doctor code and fix.
func TestReconnectLineNamesCode(t *testing.T) {
	require.NoError(t, pki.Load(nil))
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	var buf syncLogBuffer
	capture := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	origLogger := logger
	logger = func() *slog.Logger { return capture }
	t.Cleanup(func() { logger = origLogger })

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	failed := make(chan struct{}, 1)
	go RunManagedClient(ctx, ClientConfig{
		Name:     "edge-01",
		Server:   "127.0.0.1:1791",
		Token:    testHubToken,
		CA:       "missing-ca",
		Baseline: NewBaselineAccess(store),
		OnSessionFailure: func(HubSessionFailure) {
			select {
			case failed <- struct{}{}:
			default:
			}
		},
	})
	select {
	case <-failed:
	case <-time.After(3 * time.Second):
		t.Fatal("client did not report session failure")
	}
	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "connection lost, reconnecting")
	}, 2*time.Second, 10*time.Millisecond)
	cancel()

	logged := buf.String()
	require.Contains(t, logged, "code="+CodeDoctorHubCertificateUnverified)
	require.Contains(t, logged, "fix=")
	require.Contains(t, logged, "missing-ca")
}

// VALIDATES: AC-43, AC-44 -- hub-session check emits doctor-hub-certificate-unverified
// with the right cause wording and anchor-specific fix for every verification failure.
func TestCheckHubSessionCertificateUnverified(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		failure     HubSessionFailure
		wantMessage []string
		wantFix     []string
	}{
		{
			name: "undefined anchor",
			failure: HubSessionFailure{
				Server: "10.0.0.1:1791", Step: HubSessionStepAnchor, Cause: HubSessionCauseAnchorUndefined,
				Anchor: HubTrustAnchorPKICA, CAName: "corp-root",
			},
			wantMessage: []string{"10.0.0.1:1791", "pki ca corp-root", "holds no hub session"},
			wantFix:     []string{"pki ca corp-root", "ze init --managed --force --hub-ca"},
		},
		{
			name: "unknown authority with stored root",
			failure: HubSessionFailure{
				Server: "10.0.0.1:1791", Step: HubSessionStepHandshake, Cause: HubSessionCauseUnknownAuthority,
				Anchor: HubTrustAnchorStoredRoot, Error: "x509: unknown authority",
			},
			wantMessage: []string{"10.0.0.1:1791", "the root ze init stored", "did not issue it"},
			wantFix:     []string{"ze init --managed --force --hub-ca", "ze show pki local-ca pem", "bus hub server <block> certificate"},
		},
		{
			name: "unknown authority with pki ca",
			failure: HubSessionFailure{
				Server: "10.0.0.1:1791", Step: HubSessionStepHandshake, Cause: HubSessionCauseUnknownAuthority,
				Anchor: HubTrustAnchorPKICA, CAName: "fleet-root", Error: "x509: unknown authority",
			},
			wantMessage: []string{"pki ca fleet-root", "did not issue it"},
			wantFix:     []string{"correct pki ca fleet-root", "ze init --managed --force --hub-ca", "bus hub server <block> certificate"},
		},
		{
			name: "unknown authority with system trust store",
			failure: HubSessionFailure{
				Server: "10.0.0.1:1791", Step: HubSessionStepHandshake, Cause: HubSessionCauseUnknownAuthority,
				Anchor: HubTrustAnchorSystem, Error: "x509: unknown authority",
			},
			wantMessage: []string{"system trust store", "did not issue it"},
			wantFix:     []string{"name a ca", "bus hub server <block> certificate"},
		},
		{
			name: "hostname mismatch",
			failure: HubSessionFailure{
				Server: "192.0.2.5:1791", Step: HubSessionStepHandshake, Cause: HubSessionCauseHostnameMismatch,
				Anchor: HubTrustAnchorStoredRoot, CertNames: []string{"127.0.0.1", "hub.local"},
			},
			wantMessage: []string{"192.0.2.5", "127.0.0.1", "hub.local"},
			wantFix:     []string{"0.0.0.0", "bus hub client"},
		},
		{
			name: "validity dates",
			failure: HubSessionFailure{
				Server: "10.0.0.1:1791", Step: HubSessionStepHandshake, Cause: HubSessionCauseValidity,
				Anchor:        HubTrustAnchorStoredRoot,
				CertNotBefore: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				CertNotAfter:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			wantMessage: []string{"2025-01-01T00:00:00Z", "2026-01-01T00:00:00Z", "2026-11-25T12:00:00Z"},
			wantFix:     []string{"renew pki certificate", "clock"},
		},
		{
			name: "other cert failure",
			failure: HubSessionFailure{
				Server: "10.0.0.1:1791", Step: HubSessionStepHandshake, Cause: HubSessionCauseCertOther,
				Anchor: HubTrustAnchorStoredRoot, Error: "x509: incompatible key usage",
			},
			wantMessage: []string{"incompatible key usage"},
			wantFix:     []string{"ze init --managed --force --hub-ca"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := diagnoseHubSessionState(ClientState{Running: true, LastFailure: tc.failure}, now)
			require.Len(t, rows, 1)
			require.Equal(t, CodeDoctorHubCertificateUnverified, rows[0].Code)
			require.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
			for _, sub := range tc.wantMessage {
				require.Contains(t, rows[0].Message, sub)
			}
			for _, sub := range tc.wantFix {
				require.Contains(t, rows[0].Fix, sub)
			}
		})
	}
}

// VALIDATES: AC-45 -- handshake failure and auth rejection each produce their
// own code and fix, while a TCP connect failure is silent here.
func TestCheckHubSessionHandshakeAndAuth(t *testing.T) {
	now := time.Now()
	hsRows := diagnoseHubSessionState(ClientState{
		Running: true,
		LastFailure: HubSessionFailure{
			Server: "10.0.0.1:1791", Step: HubSessionStepHandshake, Cause: HubSessionCauseHandshakeOther,
			Error: "tls handshake: EOF",
		},
	}, now)
	require.Len(t, hsRows, 1)
	require.Equal(t, CodeDoctorHubHandshakeFailed, hsRows[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, hsRows[0].Severity)
	require.Contains(t, hsRows[0].Message, "EOF")
	require.Contains(t, hsRows[0].Fix, "ze show doctor")

	authRows := diagnoseHubSessionState(ClientState{
		Running: true,
		LastFailure: HubSessionFailure{
			Server: "10.0.0.1:1791", Step: HubSessionStepAuth, Cause: HubSessionCauseAuthRejected,
			Error: "auth rejected",
		},
	}, now)
	require.Len(t, authRows, 1)
	require.Equal(t, CodeDoctorHubAuthRejected, authRows[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, authRows[0].Severity)
	require.Contains(t, authRows[0].Fix, "doctor-managed-client-rejected")

	connRows := diagnoseHubSessionState(ClientState{
		Running: true,
		LastFailure: HubSessionFailure{
			Server: "10.0.0.1:1791", Step: HubSessionStepConnect, Error: "connect: connection refused",
		},
	}, now)
	require.Empty(t, connRows)
}

// VALIDATES: AC-46 -- no target installed, a connected client, and a running
// client with no failure each answer no row.
func TestCheckHubSessionSilent(t *testing.T) {
	prev := fleetTarget.Swap(nil)
	t.Cleanup(func() { fleetTarget.Store(prev) })

	require.Empty(t, checkHubSession(diagnostic.DoctorCheckContext{}))

	var state ClientState
	InstallFleetTarget(&FleetTarget{
		StartClient: func(func() error) (bool, error) { return false, nil },
		StopClient:  func(func() error) (ClientState, error) { return ClientState{}, nil },
		ClientState: func() ClientState { return state },
	})

	state = ClientState{Running: true, Connected: true, LastFailure: HubSessionFailure{
		Step: HubSessionStepAuth, Cause: HubSessionCauseAuthRejected,
	}}
	require.Empty(t, checkHubSession(diagnostic.DoctorCheckContext{}))

	state = ClientState{Running: true}
	require.Empty(t, checkHubSession(diagnostic.DoctorCheckContext{}))

	state = ClientState{Running: false, LastFailure: HubSessionFailure{
		Step: HubSessionStepAuth, Cause: HubSessionCauseAuthRejected,
	}}
	require.Empty(t, checkHubSession(diagnostic.DoctorCheckContext{}))
}

// VALIDATES: hub-session is registered in the runtime phase declaring its three
// codes, each code is registered in diagnostic, and doctor-hub-unreachable
// describes a TCP probe of the first hub block.
func TestHubSessionCheckDeclared(t *testing.T) {
	var found bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name != "hub-session" {
			continue
		}
		found = true
		for _, code := range []string{CodeDoctorHubCertificateUnverified, CodeDoctorHubHandshakeFailed, CodeDoctorHubAuthRejected} {
			require.True(t, slices.Contains(check.Codes, code), "hub-session must declare %s", code)
			meta := diagnostic.Lookup(code)
			require.NotNil(t, meta, "code %s must be registered in diagnostic", code)
			require.NotEmpty(t, meta.Description)
		}
	}
	require.True(t, found, "hub-session runtime check must be registered")

	unreachable := diagnostic.Lookup(codeHubUnreachable)
	require.NotNil(t, unreachable)
	require.Contains(t, unreachable.Description, "TCP probe of the first hub block")
}
