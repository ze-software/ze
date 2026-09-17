package init

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// Store ownership protects replacement even when the selected SSH target is remote
// or unreachable. The owning process need not expose an SSH listener.
func TestInitForceRefusesOwnerRegardlessTarget(t *testing.T) {
	dir := t.TempDir()
	owner, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close() //nolint:errcheck // test cleanup
	if err := owner.WriteKey(zefs.KeySSHDefault.Pattern, []byte("192.0.2.1/9")); err != nil {
		t.Fatal(err)
	}
	if code := runInit(strings.NewReader("new-admin\nsecret\n\n\n\n"), nil, dir, false, "", "", false, true); code == 0 {
		t.Fatal("forced initialization replaced a store with a live owner")
	}
	got, err := owner.ReadKey(zefs.KeySSHDefault.Pattern)
	if err != nil || string(got) != "192.0.2.1/9" {
		t.Fatalf("owned credentials changed: %q, %v", got, err)
	}
}

// An unrelated TCP listener cannot prevent replacing an unowned store. The
// listener deliberately never accepts, so probing it would hang initialization.
func TestInitForceIgnoresListenersWithoutOwner(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close() //nolint:errcheck // test cleanup
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	owner, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.WriteKey(zefs.KeySSHDefault.Pattern, []byte(host+"/"+port)); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if code := runInit(strings.NewReader("new-admin\nsecret\n\n\n\n"), nil, dir, false, "", "", false, true); code != 0 {
		t.Fatalf("unowned store replacement = %d", code)
	}
}

// A certificate failure after credential writes leaves no published partial tree.
func TestInitAtomicTree(t *testing.T) {
	dir := t.TempDir()
	if code := runInit(strings.NewReader("admin\nsecret\n\n\n\n"), nil, dir, false, "", "\u2603", false, false); code == 0 {
		t.Fatal("invalid certificate name accepted")
	}
	if _, err := os.Lstat(filepath.Join(dir, "database")); !os.IsNotExist(err) {
		t.Fatalf("failed initialization published database: %v", err)
	}
}

// An existing seed is never silently converted or overwritten by regular init.
func TestInitRefusesExistingBlob(t *testing.T) {
	dir := t.TempDir()
	seed, err := storage.CreateBlob(filepath.Join(dir, "database.zefs"))
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.WriteKey(zefs.KeyInstanceName.Pattern, []byte("seed-owner")); err != nil {
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	if code := runInit(strings.NewReader("admin\nsecret\n\n\n\n"), nil, dir, false, "", "", false, false); code == 0 {
		t.Fatal("initialization replaced an existing seed")
	}
	if _, err := os.Lstat(filepath.Join(dir, "database")); !os.IsNotExist(err) {
		t.Fatalf("initialization beside blob published database: %v", err)
	}
}

// A failed replacement leaves the previous credentials available under the live name.
func TestInitForcePopulationFailurePreservesStore(t *testing.T) {
	dir := t.TempDir()
	owner, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.WriteKey(zefs.KeyLocalAdminUsername.Pattern, []byte("old-admin")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if code := runInit(strings.NewReader("new-admin\nsecret\n\n\n\n"), nil, dir, false, "", "\u2603", false, true); code == 0 {
		t.Fatal("invalid certificate name accepted")
	}
	remaining, err := storage.OpenReadOnly(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer remaining.Close() //nolint:errcheck // test cleanup
	got, err := remaining.ReadKey(zefs.KeyLocalAdminUsername.Pattern)
	if err != nil || string(got) != "old-admin" {
		t.Fatalf("failed replacement lost credentials: %q, %v", got, err)
	}
}
