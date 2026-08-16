package cli

import (
	"os"
	"path/filepath"
	"testing"

	webtesting "github.com/ze-software/ze/internal/component/web/testing"
)

// TestZeTestWebAuth verifies the harness decides the web-server auth mode from a
// .wb file's option=auth directives: a plain test runs insecure, an auth test
// drops insecure and surfaces the users to seed, and a missing/unparseable file
// defaults safely to insecure.
//
// VALIDATES: the login/multi-user harness directive controls server startup.
// PREVENTS: an auth .wb test silently running under --insecure-web (where role
// gating never engages), which would make the RBAC assertions vacuously pass.
func TestZeTestWebAuth(t *testing.T) {
	dir := t.TempDir()

	plain := filepath.Join(dir, "plain.wb")
	if err := os.WriteFile(plain, []byte("action=open:path=/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if tc := zeTestWebCase(plain); tc.RequiresAuth() || len(tc.Auth) != 0 {
		t.Errorf("plain: requiresAuth=%v users=%v, want insecure with no users", tc.RequiresAuth(), tc.Auth)
	}

	authed := filepath.Join(dir, "auth.wb")
	body := "option=auth:user=noc:password=noc-pw:role=read-only\n" +
		"option=auth:user=root:password=root-pw:role=admin\n" +
		"action=login:user=root:password=root-pw\n"
	if err := os.WriteFile(authed, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	tc := zeTestWebCase(authed)
	if !tc.RequiresAuth() {
		t.Error("auth test must not run with --insecure-web")
	}
	if len(tc.Auth) != 2 {
		t.Fatalf("want 2 seeded users, got %d", len(tc.Auth))
	}

	if missing := zeTestWebCase(filepath.Join(dir, "missing.wb")); missing.RequiresAuth() {
		t.Error("missing file must default to insecure")
	}
}

// TestZeTestPickAdmin verifies the seeded admin credential prefers a user with
// the admin role over a read-only one.
func TestZeTestPickAdmin(t *testing.T) {
	users := []webtesting.WBAuthUser{
		{Name: "noc", Role: "read-only"},
		{Name: "root", Role: "admin"},
	}
	if got := zeTestPickAdmin(users).Name; got != "root" {
		t.Errorf("pickAdmin = %q, want root", got)
	}
}
