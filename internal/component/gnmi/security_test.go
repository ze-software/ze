// Design: docs/architecture/api/architecture.md -- gNMI transport security enforcement
// Related: server.go -- checkGNMIListenAddr, checkAuth, recordAuthFailure
// Related: set.go -- read-only loopback refusal and commit audit recording

package gnmi

import (
	"crypto/tls"
	"net"
	"strings"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ze-software/ze/internal/component/api"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/audit"
)

// VALIDATES: Serve refuses any non-loopback gNMI listener without both
// authentication and TLS, allows loopback without TLS, and allows non-loopback
// when both token and valid TLS material are configured.
// PREVENTS: gNMI credentials and configuration crossing the network in clear
// text when Serve is called directly or behind a misconfigured caller.
// Method: drives Serve directly across refused and permitted listener configs
// and verifies refused configs return immediately without binding a socket.
func TestServeNonLoopbackRequiresAuthAndTLS(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeTestKeyPair(t, dir, "gnmi-nonloopback")

	refusedCases := []struct {
		name    string
		cfg     Config
		wantSub string
	}{
		{
			name:    "non-loopback without token or TLS",
			cfg:     Config{ListenAddr: "0.0.0.0:0"},
			wantSub: "requires authentication",
		},
		{
			name:    "non-loopback with TLS but without token",
			cfg:     Config{ListenAddr: "0.0.0.0:0", CertFile: certFile, KeyFile: keyFile},
			wantSub: "requires authentication",
		},
		{
			name:    "non-loopback with token but without TLS",
			cfg:     Config{ListenAddr: "0.0.0.0:0", Token: "s3cret"},
			wantSub: "requires TLS",
		},
		{
			name:    "wildcard port-only without TLS",
			cfg:     Config{ListenAddr: ":0", Token: "s3cret"},
			wantSub: "requires TLS",
		},
		{
			name:    "hostname localhost without TLS fails closed",
			cfg:     Config{ListenAddr: "localhost:0", Token: "s3cret"},
			wantSub: "requires TLS",
		},
	}

	for _, tt := range refusedCases {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(tt.cfg, treeFunc(zeconfig.NewTree()), nil, yangloader.DefaultLoader, nil)
			errCh := make(chan error, 1)
			go func() { errCh <- srv.Serve(t.Context()) }()

			var err error
			select {
			case err = <-errCh:
			case <-time.After(2 * time.Second):
				srv.Stop()
				t.Fatalf("Serve bound %q instead of refusing", srv.Address())
			}

			if err == nil {
				t.Fatal("Serve returned nil, want refusal error")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("Serve error = %q, want substring %q", err.Error(), tt.wantSub)
			}
			if addr := srv.Address(); addr != "" {
				t.Errorf("Serve bound %q; refused listener must bind nothing", addr)
			}
		})
	}

	t.Run("loopback without TLS is allowed", func(t *testing.T) {
		srv := NewServer(Config{ListenAddr: "127.0.0.1:0"}, treeFunc(zeconfig.NewTree()), nil, yangloader.DefaultLoader, nil)
		startTestServer(t, srv)

		client := dialTestServer(t, srv.Address())
		if _, err := client.Capabilities(t.Context(), &gpb.CapabilityRequest{}); err != nil {
			t.Fatalf("Capabilities on loopback without TLS: %v", err)
		}
	})

	t.Run("non-loopback with token and TLS is allowed", func(t *testing.T) {
		srv := NewServer(Config{
			ListenAddr: "0.0.0.0:0",
			Token:      "s3cret",
			CertFile:   certFile,
			KeyFile:    keyFile,
		}, treeFunc(zeconfig.NewTree()), nil, yangloader.DefaultLoader, nil)
		startTestServer(t, srv)

		_, port, err := net.SplitHostPort(srv.Address())
		if err != nil {
			t.Fatalf("split bound address %q: %v", srv.Address(), err)
		}
		tlsCfg := &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true, //nolint:gosec // self-signed test certificate
		}
		conn, err := grpc.NewClient(net.JoinHostPort("127.0.0.1", port), grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
		if err != nil {
			t.Fatalf("dial TLS: %v", err)
		}
		t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

		ctx := metadata.AppendToOutgoingContext(t.Context(), "authorization", "Bearer s3cret")
		if _, err := gpb.NewGNMIClient(conn).Capabilities(ctx, &gpb.CapabilityRequest{}); err != nil {
			t.Fatalf("Capabilities on non-loopback TLS listener: %v", err)
		}
	})
}

// VALIDATES: an unauthenticated loopback gNMI listener is read-only:
// Capabilities and Get succeed while Set is refused with codes.PermissionDenied
// and leaves the config session untouched.
// PREVENTS: a local unauthenticated process mutating router configuration over
// loopback gNMI, matching REST and gRPC read-only loopback enforcement.
// Method: starts a loopback server with empty Token and a wired session editor,
// then sends Capabilities, Get, and Set over gRPC.
func TestUnauthenticatedLoopbackReadOnly(t *testing.T) {
	tree := zeconfig.NewTree()
	bgp := zeconfig.NewTree()
	bgp.Set("router-id", "192.0.2.1")
	tree.SetContainer("bgp", bgp)

	editor := &configSessionEditor{values: make(map[string]string)}
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return editor, nil
	})
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"}, treeFunc(tree), sessions, yangloader.DefaultLoader, nil)
	startTestServer(t, srv)

	client := dialTestServer(t, srv.Address())
	ctx := t.Context()

	if _, err := client.Capabilities(ctx, &gpb.CapabilityRequest{}); err != nil {
		t.Fatalf("Capabilities on unauthenticated loopback: %v", err)
	}

	getResp, err := client.Get(ctx, &gpb.GetRequest{
		Encoding: gpb.Encoding_JSON_IETF,
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{{Name: "bgp"}, {Name: "router-id"}},
		}},
	})
	if err != nil {
		t.Fatalf("Get on unauthenticated loopback: %v", err)
	}
	if got := getResp.GetNotification()[0].GetUpdate()[0].GetVal().GetStringVal(); got != "192.0.2.1" {
		t.Fatalf("Get router-id = %q, want 192.0.2.1", got)
	}

	_, setErr := client.Set(ctx, &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "bgp"}, {Name: "router-id"}}},
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "198.51.100.1"}},
		}},
	})
	if got := status.Code(setErr); got != codes.PermissionDenied {
		t.Fatalf("unauthenticated Set code = %v (%v), want PermissionDenied", got, setErr)
	}
	if editor.committedContent != "" {
		t.Fatalf("unauthenticated Set committed %q; expected no mutation", editor.committedContent)
	}
}

// VALIDATES: authenticated gNMI Set succeeds and emits a config-commit audit
// entry under surface gnmi, while rejected credentials emit auth-fail audit
// entries without leaking any rejected token bytes.
// PREVENTS: gNMI config mutations and brute-force auth attempts being invisible
// in Ze's unified audit log.
// Method: runs a loopback gNMI server wired with an in-memory audit.Log and
// exercises missing token, wrong token, and authenticated Set over gRPC.
func TestAuthenticatedSetAndAuthFailuresEmitAudit(t *testing.T) {
	recorder, err := audit.NewMemory(100)
	if err != nil {
		t.Fatalf("create audit log: %v", err)
	}
	editor := &configSessionEditor{values: make(map[string]string)}
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return editor, nil
	})
	srv := NewServer(Config{
		ListenAddr:    "127.0.0.1:0",
		Token:         "audit-s3cret",
		AuditRecorder: recorder,
	}, treeFunc(zeconfig.NewTree()), sessions, yangloader.DefaultLoader, nil)
	startTestServer(t, srv)

	client := dialTestServer(t, srv.Address())
	ctx := t.Context()

	// Missing authorization header -> Unauthenticated + auth-fail audit entry.
	_, err = client.Capabilities(ctx, &gpb.CapabilityRequest{})
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("missing-auth Capabilities code = %v, want Unauthenticated", got)
	}

	// Wrong bearer token -> Unauthenticated + auth-fail audit entry without token leak.
	const wrongSecret = "wrong-token-must-not-appear-in-audit"
	badCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+wrongSecret)
	_, err = client.Capabilities(badCtx, &gpb.CapabilityRequest{})
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("wrong-token Capabilities code = %v, want Unauthenticated", got)
	}

	// Authenticated Set -> succeeds + config-commit audit entry.
	goodCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer audit-s3cret")
	resp, err := client.Set(goodCtx, &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "bgp"}, {Name: "router-id"}}},
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "10.0.0.9"}},
		}},
	})
	if err != nil {
		t.Fatalf("authenticated Set: %v", err)
	}
	if len(resp.Response) != 1 {
		t.Fatalf("Set responses = %d, want 1", len(resp.Response))
	}
	if !strings.Contains(editor.committedContent, "bgp.router-id = 10.0.0.9") {
		t.Fatalf("committed content %q missing router-id update", editor.committedContent)
	}

	authFails := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
	if len(authFails) != 2 {
		t.Fatalf("auth-fail audit entries = %d, want 2", len(authFails))
	}
	for i, entry := range authFails {
		if entry.Surface != audit.GNMI {
			t.Errorf("auth-fail[%d] surface = %q, want %q", i, entry.Surface, audit.GNMI)
		}
		if entry.Outcome != audit.OutcomeDenied {
			t.Errorf("auth-fail[%d] outcome = %q, want %q", i, entry.Outcome, audit.OutcomeDenied)
		}
		if entry.RemoteAddr == "" {
			t.Errorf("auth-fail[%d] remote-addr is empty", i)
		}
		if strings.Contains(entry.Actor, wrongSecret) {
			t.Errorf("auth-fail[%d] actor leaked rejected token", i)
		}
		if strings.Contains(entry.Detail, wrongSecret) {
			t.Errorf("auth-fail[%d] detail leaked rejected token", i)
		}
	}

	commits := recorder.Query(audit.Filter{Action: audit.ActionConfigCommit})
	if len(commits) != 1 {
		t.Fatalf("config-commit audit entries = %d, want 1", len(commits))
	}
	commit := commits[0]
	if commit.Surface != audit.GNMI {
		t.Errorf("commit surface = %q, want %q", commit.Surface, audit.GNMI)
	}
	if commit.Actor != gnmiUsername {
		t.Errorf("commit actor = %q, want %q", commit.Actor, gnmiUsername)
	}
	if commit.Outcome != audit.OutcomeSuccess {
		t.Errorf("commit outcome = %q, want %q", commit.Outcome, audit.OutcomeSuccess)
	}
	if commit.RemoteAddr == "" {
		t.Error("commit remote-addr is empty")
	}
}

// VALIDATES: a nil AuditRecorder fails safely on both auth failures and
// authenticated Set commits.
// PREVENTS: nil-pointer panics in isolated unit tests or callers that do not
// wire an audit recorder.
// Method: starts an authenticated loopback server with AuditRecorder nil and
// exercises one auth failure followed by one authenticated Set.
func TestNilAuditRecorderFailsSafely(t *testing.T) {
	editor := &configSessionEditor{values: make(map[string]string)}
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return editor, nil
	})
	srv := NewServer(Config{
		ListenAddr: "127.0.0.1:0",
		Token:      "nil-audit-s3cret",
	}, treeFunc(zeconfig.NewTree()), sessions, yangloader.DefaultLoader, nil)
	startTestServer(t, srv)

	client := dialTestServer(t, srv.Address())
	ctx := t.Context()

	_, err := client.Capabilities(ctx, &gpb.CapabilityRequest{})
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("unauthenticated Capabilities code = %v, want Unauthenticated", got)
	}

	goodCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer nil-audit-s3cret")
	_, err = client.Set(goodCtx, &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "bgp"}, {Name: "router-id"}}},
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "10.0.0.10"}},
		}},
	})
	if err != nil {
		t.Fatalf("authenticated Set with nil AuditRecorder: %v", err)
	}
}
