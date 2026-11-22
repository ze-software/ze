// Design: docs/architecture/api/architecture.md -- gNMI transport security enforcement
// Related: server.go -- checkAuth, gnmiAuthRefusal
// Related: set.go -- the read-only refusal of a tokenless server

package gnmi

import (
	"context"
	"strings"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
)

// TestCheckAuthRefusalsNameReasonAndFix: every Unauthenticated answer says what
// was wrong with the request, why the server refuses it, and the fix, and a
// wrong token answer leaks neither the configured token nor the presented one.
//
// VALIDATES: ai/rules/user-facing-errors.md on the gNMI auth refusals.
// PREVENTS: a bare "invalid token" that tells a gNMI client operator nothing,
// and a refusal that echoes a secret.
func TestCheckAuthRefusalsNameReasonAndFix(t *testing.T) {
	const configured = "configured-s3cret"
	const presented = "presented-guess"
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0", Token: configured},
		treeFunc(zeconfig.NewTree()), nil, yangloader.DefaultLoader, nil)

	withAuth := func(value string) context.Context {
		return metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", value))
	}
	cases := []struct {
		name    string
		ctx     context.Context
		problem string
	}{
		{"no metadata", t.Context(), "missing metadata"},
		{"no authorization", metadata.NewIncomingContext(t.Context(), metadata.Pairs("other", "x")), "missing authorization"},
		{"bare prefix", withAuth("Bearer "), "invalid authorization format"},
		{"wrong scheme", withAuth("Basic " + presented), "invalid authorization format"},
		{"wrong token", withAuth("Bearer " + presented), "invalid token"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := srv.checkAuth(tt.ctx)
			st, ok := status.FromError(err)
			if !ok || st.Code() != codes.Unauthenticated {
				t.Fatalf("checkAuth = %v, want Unauthenticated", err)
			}
			msg := st.Message()
			if !strings.HasPrefix(msg, tt.problem) {
				t.Errorf("message = %q, want it to start with %q", msg, tt.problem)
			}
			if !strings.Contains(msg, gnmiAuthWhy) {
				t.Errorf("message = %q, want the reason %q", msg, gnmiAuthWhy)
			}
			if !strings.HasSuffix(msg, "fix: "+gnmiAuthFix) {
				t.Errorf("message = %q, want it to end with the fix %q", msg, gnmiAuthFix)
			}
			if strings.Contains(msg, configured) {
				t.Errorf("message = %q leaks the configured token", msg)
			}
			if strings.Contains(msg, presented) {
				t.Errorf("message = %q echoes the presented credential", msg)
			}
		})
	}

	if err := srv.checkAuth(withAuth("Bearer " + configured)); err != nil {
		t.Fatalf("the configured token must pass, got %v", err)
	}
}

// TestSetRefusalWithoutTokenNamesRestart: a tokenless server refuses Set, and
// its fix says the token takes effect only after a restart, because a reload
// does not rebuild gNMI.
//
// VALIDATES: the read-only refusal names the whole fix.
// PREVENTS: an operator setting the token, committing, and meeting the same
// refusal with no hint that gNMI still runs with the boot settings.
func TestSetRefusalWithoutTokenNamesRestart(t *testing.T) {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"},
		treeFunc(zeconfig.NewTree()), nil, yangloader.DefaultLoader, nil)
	_, err := srv.Set(t.Context(), &gpb.SetRequest{})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("Set = %v, want PermissionDenied", err)
	}
	if !strings.HasSuffix(st.Message(), "fix: set environment gnmi token <token>, commit, then run ze signal restart") {
		t.Errorf("message = %q, want the token-then-restart fix on its last line", st.Message())
	}
}
