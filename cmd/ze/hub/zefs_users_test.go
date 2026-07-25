package hub

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/pkg/zefs"
)

// writeZefsCreds builds a zefs database with local power-user credentials and,
// optionally, outbound remote credentials plus meta/ssh/default. When user is
// empty, no local credential entries are written (empty database).
func writeZefsCreds(t *testing.T, user, hash string) *zefs.BlobStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database.zefs")
	store, err := zefs.Create(path)
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	if user != "" {
		if err := store.WriteFile(zefs.KeyLocalAdminUsername.Pattern, []byte(user), 0); err != nil {
			t.Fatal(err)
		}
		// Write the password key even when hash is empty, to exercise the
		// empty-hash fail-closed guard (ReadFile succeeds with empty bytes).
		if err := store.WriteFile(zefs.KeyLocalAdminPassword.Pattern, []byte(hash), 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	db, err := zefs.Open(path)
	if err != nil {
		t.Fatalf("zefs.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() }) //nolint:errcheck // test cleanup
	return db
}

func writeRemoteCreds(t *testing.T, store *zefs.BlobStore, host, port, user, hash string, writePointer bool) {
	t.Helper()
	if user != "" {
		if err := store.WriteFile(zefs.KeySSHUsername.Key(host, port), []byte(user), 0); err != nil {
			t.Fatal(err)
		}
		if err := store.WriteFile(zefs.KeySSHPassword.Key(host, port), []byte(hash), 0); err != nil {
			t.Fatal(err)
		}
	}
	if writePointer {
		if err := store.WriteFile(zefs.KeySSHDefault.Pattern, []byte(host+"/"+port), 0); err != nil {
			t.Fatal(err)
		}
	}
}

// VALIDATES: zefs local power-user auth reads dedicated local credentials.
// PREVENTS: local admin auth depending on outbound remote-client state.
func TestUsersFromZefsDBReadsLocalPowerCredentials(t *testing.T) {
	db := writeZefsCreds(t, "admin", "$2y$10$hash")

	users, err := usersFromZefsDB(db)
	if err != nil {
		t.Fatalf("usersFromZefsDB: %v", err)
	}
	if len(users) != 1 || users[0].Name != "admin" || users[0].Hash != "$2y$10$hash" {
		t.Fatalf("got %+v, want one admin user with hash", users)
	}
}

// VALIDATES: spec-fixit-authz-admin-fallthrough O-3'/AC-2 -- the ze init bootstrap
// admin carries the reserved break-glass recovery profile, delivered through
// login-resolved profiles (never a config assignment). This is what lets the
// recovery account still reach a box whose authorization config defines profiles
// but no config admin, after Store.Authorize was made to fail closed.
// PREVENTS: a strict authorization default bricking the recovery account.
func TestUsersFromZefsDBCarriesRecoveryProfile(t *testing.T) {
	db := writeZefsCreds(t, "admin", "$2y$10$hash")

	users, err := usersFromZefsDB(db)
	if err != nil {
		t.Fatalf("usersFromZefsDB: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1", len(users))
	}
	if !slices.Contains(users[0].Profiles, aaa.ReservedRecoveryProfile) {
		t.Fatalf("bootstrap admin profiles = %v, want to contain the reserved recovery profile", users[0].Profiles)
	}
}

// VALIDATES: meta/ssh/default does not select local login credentials.
// PREVENTS: changing the outbound default remote from changing local admin auth.
func TestUsersFromZefsDBIgnoresRemoteDefaultPointer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.zefs")
	store, err := zefs.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeyLocalAdminUsername.Pattern, []byte("admin"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeyLocalAdminPassword.Pattern, []byte("$2y$10$local"), 0); err != nil {
		t.Fatal(err)
	}
	writeRemoteCreds(t, store, "203.0.113.10", "2222", "remote", "$2y$10$remote", true)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := zefs.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() }) //nolint:errcheck // test cleanup

	users, err := usersFromZefsDB(db)
	if err != nil {
		t.Fatalf("usersFromZefsDB: %v", err)
	}
	if len(users) != 1 || users[0].Name != "admin" || users[0].Hash != "$2y$10$local" {
		t.Fatalf("got %+v, want local admin user", users)
	}
}

// VALIDATES: legacy meta/ssh/* records are not accepted for local admin auth.
// PREVENTS: outbound remote-client state from becoming an implicit login source.
func TestUsersFromZefsDBRejectsLegacySSHOnlyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.zefs")
	store, err := zefs.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeRemoteCreds(t, store, "0.0.0.0", "22", "admin", "$2y$10$legacy", true)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := zefs.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() }) //nolint:errcheck // test cleanup

	if _, err := usersFromZefsDB(db); err == nil {
		t.Fatal("expected legacy ssh-only database to be rejected")
	}
}

func TestUsersFromZefsDBFailsClosedOnMissingCreds(t *testing.T) {
	db := writeZefsCreds(t, "", "")

	if _, err := usersFromZefsDB(db); err == nil {
		t.Fatal("expected error for missing credentials (fail closed)")
	}
}

func TestUsersFromZefsDBFailsClosedOnEmptyHash(t *testing.T) {
	db := writeZefsCreds(t, "admin", "")

	if _, err := usersFromZefsDB(db); !errors.Is(err, errEmptyPasswordInZefs) {
		t.Fatalf("got %v, want errEmptyPasswordInZefs", err)
	}
}

// VALIDATES: admin-disabled flag in zefs blocks the power user from loading.
// PREVENTS: built-in admin remaining active after operator explicitly disables it.
func TestUsersFromZefsDBRespectsAdminDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.zefs")
	store, err := zefs.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeyLocalAdminUsername.Pattern, []byte("admin"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeyLocalAdminPassword.Pattern, []byte("$2y$10$hash"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeyInstanceAdminDisabled.Pattern, []byte("true"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := zefs.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() }) //nolint:errcheck // test cleanup

	if _, err := usersFromZefsDB(db); !errors.Is(err, errAdminDisabledInZefs) {
		t.Fatalf("got %v, want errAdminDisabledInZefs", err)
	}
}

// VALIDATES: admin-disabled="false" does not block power user loading.
func TestUsersFromZefsDBAllowsExplicitFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.zefs")
	store, err := zefs.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeyLocalAdminUsername.Pattern, []byte("admin"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeyLocalAdminPassword.Pattern, []byte("$2y$10$hash"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeyInstanceAdminDisabled.Pattern, []byte("false"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := zefs.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() }) //nolint:errcheck // test cleanup

	users, err := usersFromZefsDB(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 || users[0].Name != "admin" {
		t.Fatalf("got %+v, want admin user", users)
	}
}

// Confirms both credential sources are active: the zefs power user plus
// any config-file users. Config user with the same name overrides the
// zefs entry.
func TestMergeAuthUsers(t *testing.T) {
	power := []authz.UserConfig{{Name: "admin", Hash: "h1"}}
	cfg := []authz.UserConfig{{Name: "op1", Hash: "h2"}, {Name: "op2", Hash: "h3"}}

	got := mergeAuthUsers(power, cfg)
	if len(got) != 3 {
		t.Fatalf("got %d users, want 3 (power + 2 config)", len(got))
	}
	if got[0].Name != "admin" {
		t.Errorf("power user must come first, got %q", got[0].Name)
	}
	names := map[string]bool{}
	for _, u := range got {
		names[u.Name] = true
	}
	for _, want := range []string{"admin", "op1", "op2"} {
		if !names[want] {
			t.Errorf("merged set missing %q (both sources must be active)", want)
		}
	}

	if len(mergeAuthUsers(nil, nil)) != 0 {
		t.Error("mergeAuthUsers(nil, nil) should be empty")
	}
	if u := mergeAuthUsers(nil, cfg); len(u) != 2 {
		t.Errorf("config-only merge should yield the config users, got %d", len(u))
	}

	// Must not alias the input slices.
	got2 := mergeAuthUsers(power, nil)
	got2[0] = authz.UserConfig{Name: "mutated"}
	if power[0].Name != "admin" {
		t.Error("mergeAuthUsers must not alias its input slice")
	}
}

// VALIDATES: config user with same name as zefs power user overrides it.
// PREVENTS: dual entries allowing stale zefs password as a backdoor.
func TestMergeAuthUsersConfigOverridesZefs(t *testing.T) {
	power := []authz.UserConfig{{Name: "admin", Hash: "zefs-hash"}}
	cfg := []authz.UserConfig{{Name: "admin", Hash: "config-hash"}, {Name: "op1", Hash: "h2"}}

	got := mergeAuthUsers(power, cfg)
	if len(got) != 2 {
		t.Fatalf("got %d users, want 2 (config admin replaces zefs admin + op1)", len(got))
	}
	for _, u := range got {
		if u.Name == "admin" && u.Hash != "config-hash" {
			t.Errorf("admin hash = %q, want config-hash (config must override zefs)", u.Hash)
		}
	}
}

// VALIDATES: multiple zefs users where only one is overridden.
func TestMergeAuthUsersPartialOverride(t *testing.T) {
	zefs := []authz.UserConfig{{Name: "admin", Hash: "z1"}, {Name: "rescue", Hash: "z2"}}
	cfg := []authz.UserConfig{{Name: "admin", Hash: "c1"}}

	got := mergeAuthUsers(zefs, cfg)
	if len(got) != 2 {
		t.Fatalf("got %d users, want 2 (rescue from zefs + admin from config)", len(got))
	}
	hashes := map[string]string{}
	for _, u := range got {
		hashes[u.Name] = u.Hash
	}
	if hashes["admin"] != "c1" {
		t.Errorf("admin hash = %q, want c1", hashes["admin"])
	}
	if hashes["rescue"] != "z2" {
		t.Errorf("rescue hash = %q, want z2", hashes["rescue"])
	}
}
