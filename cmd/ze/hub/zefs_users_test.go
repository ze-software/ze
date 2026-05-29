package hub

import (
	"errors"
	"path/filepath"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// writeZefsCreds builds a zefs database the way ze init / the imageserver do:
// credentials at the concrete .Key(host,port) plus a meta/ssh/default pointer.
// When user is empty, no credential entries are written (empty database).
func writeZefsCreds(t *testing.T, host, port, user, hash string, writePointer bool) *zefs.BlobStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database.zefs")
	store, err := zefs.Create(path)
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	if user != "" {
		if err := store.WriteFile(zefs.KeySSHUsername.Key(host, port), []byte(user), 0); err != nil {
			t.Fatal(err)
		}
		// Write the password key even when hash is empty, to exercise the
		// empty-hash fail-closed guard (ReadFile succeeds with empty bytes).
		if err := store.WriteFile(zefs.KeySSHPassword.Key(host, port), []byte(hash), 0); err != nil {
			t.Fatal(err)
		}
	}
	if writePointer {
		if err := store.WriteFile(zefs.KeySSHDefault.Pattern, []byte(host+"/"+port), 0); err != nil {
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

// Pins the reader/writer key-shape bug: credentials are written at the concrete
// .Key(host,port) with a meta/ssh/default pointer; the reader must follow the
// pointer (as the outbound SSH client does), not read the {host}/{port}
// placeholder key literally. The previous implementation read the literal
// placeholder key and would fail this test.
func TestUsersFromZefsDBResolvesViaDefaultPointer(t *testing.T) {
	db := writeZefsCreds(t, "127.0.0.1", "2222", "admin", "$2y$10$hash", true)

	users, err := usersFromZefsDB(db)
	if err != nil {
		t.Fatalf("usersFromZefsDB: %v", err)
	}
	if len(users) != 1 || users[0].Name != "admin" || users[0].Hash != "$2y$10$hash" {
		t.Fatalf("got %+v, want one admin user with hash", users)
	}
}

// Proves resolution is not hardcoded to 127.0.0.1/2222: an assemble-style
// appliance stores credentials at 0.0.0.0/22 and the pointer must steer the
// reader there.
func TestUsersFromZefsDBResolvesNonDefaultCoords(t *testing.T) {
	db := writeZefsCreds(t, "0.0.0.0", "22", "operator", "$2y$10$other", true)

	users, err := usersFromZefsDB(db)
	if err != nil {
		t.Fatalf("usersFromZefsDB: %v", err)
	}
	if len(users) != 1 || users[0].Name != "operator" {
		t.Fatalf("got %+v, want one operator user", users)
	}
}

func TestUsersFromZefsDBFailsClosedOnMissingCreds(t *testing.T) {
	db := writeZefsCreds(t, "127.0.0.1", "2222", "", "", false)

	if _, err := usersFromZefsDB(db); err == nil {
		t.Fatal("expected error for missing credentials (fail closed)")
	}
}

func TestUsersFromZefsDBFailsClosedOnEmptyHash(t *testing.T) {
	db := writeZefsCreds(t, "127.0.0.1", "2222", "admin", "", true)

	if _, err := usersFromZefsDB(db); !errors.Is(err, errEmptyPasswordInZefs) {
		t.Fatalf("got %v, want errEmptyPasswordInZefs", err)
	}
}

// A corrupt meta/ssh/default pointer must not crash the daemon: zefs.KeyEntry.Key
// panics on a ".."-containing component, so zefsDefaultTarget must reject such a
// pointer and fall back to the default coordinates. Without the guard this test
// panics instead of returning the user found at the default coordinates.
func TestUsersFromZefsDBSurvivesMalformedDefaultPointer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.zefs")
	store, err := zefs.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeySSHUsername.Key("127.0.0.1", "2222"), []byte("admin"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeySSHPassword.Key("127.0.0.1", "2222"), []byte("$2y$hash"), 0); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(zefs.KeySSHDefault.Pattern, []byte("../evil/2222"), 0); err != nil {
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
		t.Fatalf("usersFromZefsDB: %v", err)
	}
	if len(users) != 1 || users[0].Name != "admin" {
		t.Fatalf("got %+v, want admin via default-coordinate fallback", users)
	}
}

// Confirms both credential sources are active: the always-on power user plus
// any config-file users (the basis for web/API admitting config users, not just
// the power user).
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
