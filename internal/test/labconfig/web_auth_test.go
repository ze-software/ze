//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- browser credential composition.

package labconfig_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	webtesting "github.com/ze-software/ze/internal/component/web/testing"
	"github.com/ze-software/ze/internal/test/labconfig"
)

// TestWebDaemonAuthProfiles parses actual fixture declarations and authenticates
// the emitted config through the production credential/profile extractors.
func TestWebDaemonAuthProfiles(t *testing.T) {
	workflow, err := webtesting.ParseWBFile("option=server:kind=daemon\n" +
		"option=auth:user=admin:password=admin-fixture:role=admin\n" +
		"option=auth:user=noc:password=reader-fixture:role=read-only\n")
	if err != nil {
		t.Fatal(err)
	}
	body, err := labconfig.WebDaemonWithAuth(context.Background(), workflow.Plugins, workflow.Auth)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.ParseTreeWithYANG(string(body), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := infra.ValidateAuthzConfig(tree); err != nil {
		t.Fatal(err)
	}
	system := tree.GetContainer("system")
	if system == nil {
		t.Fatal("authenticated fixture emitted no system configuration")
	}
	local := authz.LocalAuthenticator{Users: infra.ExtractAuthUsers(system.ToMap())}
	policy, err := infra.ExtractAuthzStore(tree)
	if err != nil {
		t.Fatal(err)
	}
	if policy == nil {
		t.Fatal("fixture has no authorization store")
	}
	for _, declared := range workflow.Auth {
		t.Run(declared.Name, func(t *testing.T) {
			result, err := local.Authenticate(authz.AuthRequest{Username: declared.Name, Password: declared.Password})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Authenticated {
				t.Fatal("declared fixture user could not authenticate")
			}
			if got := policy.AuthorizeWithProfiles(declared.Name, result.Profiles, "show bgp", true); got != authz.Allow {
				t.Errorf("authenticated reader cannot read: %v", got)
			}
			wantEdit := authz.Allow
			if declared.Role == "read-only" {
				wantEdit = authz.Deny
			}
			if got := policy.AuthorizeWithProfiles(declared.Name, result.Profiles, "config commit", false); got != wantEdit {
				t.Errorf("edit decision = %v, want %v", got, wantEdit)
			}
			if _, err := local.Authenticate(authz.AuthRequest{Username: declared.Name, Password: "incorrect"}); err == nil {
				t.Fatal("incorrect fixture password authenticated")
			}
		})
	}
}

// TestWebDaemonAuthRejectsInvalidDeclarations refuses ambiguous identities and
// unknown roles instead of silently granting a guessed fixture privilege.
func TestWebDaemonAuthRejectsInvalidDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name  string
		users []webtesting.WBAuthUser
	}{
		{"empty name", []webtesting.WBAuthUser{{Password: "fixture", Role: "admin"}}},
		{"unknown role", []webtesting.WBAuthUser{{Name: "noc", Password: "fixture", Role: "unsupported"}}},
		{"duplicate identity", []webtesting.WBAuthUser{{Name: "noc", Password: "one"}, {Name: "noc", Password: "two", Role: "read-only"}}},
		{"bcrypt limit", []webtesting.WBAuthUser{{Name: "noc", Password: strings.Repeat("x", 73), Role: "read-only"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := labconfig.WebDaemonWithAuth(context.Background(), nil, tc.users); err == nil {
				t.Fatal("invalid fixture declaration produced a daemon configuration")
			}
		})
	}
}

// TestWebDaemonAuthCancellation refuses before synchronous bcrypt work and
// preserves the fixture caller's cancellation reason.
func TestWebDaemonAuthCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	users := []webtesting.WBAuthUser{{Name: "noc", Password: strings.Repeat("x", 73), Role: "read-only"}}
	if _, err := labconfig.WebDaemonWithAuth(ctx, nil, users); !errors.Is(err, context.Canceled) {
		t.Fatalf("fixture composition ignored canceled admission: %v", err)
	}
}
