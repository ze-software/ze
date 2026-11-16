// Design: docs/architecture/testing/runner-architecture.md -- authenticated browser fixtures.

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/config/storage"
	webtesting "github.com/ze-software/ze/internal/component/web/testing"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestWebReadOnlyFixtureNeverSeedsPowerAdmin parses each real permission case
// and inspects the store the launching runner seeds. The declared reader MUST
// remain distinct from the unconditional zefs power administrator.
func TestWebReadOnlyFixtureNeverSeedsPowerAdmin(t *testing.T) {
	for _, name := range []string{"web-rbac-readonly.wb", "config-download-edit-gated.wb"} {
		t.Run(name, func(t *testing.T) {
			workflow := webRoleFixture(t, name)
			dir := t.TempDir()
			if err := zeTestSeedWebUsers(context.Background(), dir, !workflow.RequiresAuth(), workflow.Auth); err != nil {
				t.Fatal(err)
			}
			store := webRoleSeedStore(t, dir)
			username, err := store.ReadFile(zefs.KeyLocalAdminUsername.Pattern)
			if err != nil {
				t.Fatal(err)
			}
			for _, user := range workflow.Auth {
				if user.Role != "read-only" {
					continue
				}
				if string(username) == user.Name {
					t.Errorf("read-only browser identity %q was seeded as the unconditional zefs power administrator", user.Name)
				}
			}
		})
	}
}

// TestWebAllReaderFixtureKeepsSeparatePowerAdmin preserves the sole-reader
// privilege regression even when repository browser cases declare an admin.
// A reader occupying the first synthetic name must remain a reader too.
func TestWebAllReaderFixtureKeepsSeparatePowerAdmin(t *testing.T) {
	workflow, err := webtesting.ParseWBFile("option=auth:user=noc:password=reader-fixture:role=read-only\n" +
		"option=auth:user=wb-seeded-admin-0:password=second-reader:role=read-only\n")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := zeTestSeedWebUsers(context.Background(), dir, false, workflow.Auth); err != nil {
		t.Fatal(err)
	}
	store := webRoleSeedStore(t, dir)
	username, err := store.ReadFile(zefs.KeyLocalAdminUsername.Pattern)
	if err != nil {
		t.Fatal(err)
	}
	for _, reader := range workflow.Auth {
		if string(username) == reader.Name {
			t.Errorf("reader %q was promoted to power admin", reader.Name)
		}
	}
}

// TestWebAdminFixtureKeepsLoginCredential checks the existing admin and unset-
// role login cases still seed the authored password, through the actual parser
// and tree store rather than a second fixture credential producer.
func TestWebAdminFixtureKeepsLoginCredential(t *testing.T) {
	for _, name := range []string{"web-rbac-admin.wb", "web-page-assets.wb"} {
		t.Run(name, func(t *testing.T) {
			workflow := webRoleFixture(t, name)
			dir := t.TempDir()
			if err := zeTestSeedWebUsers(context.Background(), dir, !workflow.RequiresAuth(), workflow.Auth); err != nil {
				t.Fatal(err)
			}
			store := webRoleSeedStore(t, dir)
			username, err := store.ReadFile(zefs.KeyLocalAdminUsername.Pattern)
			if err != nil {
				t.Fatal(err)
			}
			if string(username) != workflow.Auth[0].Name {
				t.Fatalf("power admin = %q, want declared admin %q", username, workflow.Auth[0].Name)
			}
			hash, err := store.ReadFile(zefs.KeyLocalAdminPassword.Pattern)
			if err != nil {
				t.Fatal(err)
			}
			if err := bcrypt.CompareHashAndPassword(hash, []byte(workflow.Auth[0].Password)); err != nil {
				t.Fatalf("authored admin password cannot authenticate: %v", err)
			}
		})
	}
}

// webRoleFixture uses the repository carrier, preserving its real role and
// password declarations even while a browser case remains explicitly skipped.
func webRoleFixture(t *testing.T, name string) *webtesting.WBTestCase {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "web", name))
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := webtesting.ParseWBFile(string(body))
	if err != nil {
		t.Fatal(err)
	}
	if !workflow.RequiresAuth() {
		t.Fatal("permission fixture must declare authenticated identities")
	}
	return workflow
}

// webRoleSeedStore owns the opened seed until test cleanup, which MUST close
// the handle before TempDir removes the fixture's tree and lock.
func webRoleSeedStore(t *testing.T, dir string) storage.Storage {
	t.Helper()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return store
}
