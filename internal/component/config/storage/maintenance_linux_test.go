//go:build linux

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMaintenanceLockExcludesDaemonWithoutOpeningData(t *testing.T) {
	dir := t.TempDir()
	store, err := Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	lock, err := AcquireMaintenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); !errors.Is(err, ErrBusy) {
		t.Fatalf("daemon entered maintenance: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(dir)
	if err != nil {
		t.Fatalf("maintenance did not release lock: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMaintenanceLockRefusesLiveDaemon(t *testing.T) {
	dir := t.TempDir()
	store, err := Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // Test cleanup.
	if _, err := AcquireMaintenance(dir); !errors.Is(err, ErrBusy) {
		t.Fatalf("live daemon admitted: %v", err)
	}
}

func TestMaintenanceLockRefusesSymlinkFolder(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if lock, err := AcquireMaintenance(link); err == nil {
		lock.Close() //nolint:errcheck // Unexpected admission is already the failure.
		t.Fatal("symlink folder admitted")
	}
}

func TestMaintenanceRootLocksServiceOwnedFolderWithoutDataAccess(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root is required to create a service-owned fixture")
	}
	dir := t.TempDir()
	store, err := Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := TransferOwnership(dir, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	lock, err := AcquireMaintenance(dir)
	if err != nil {
		t.Fatalf("root maintenance refused service ownership: %v", err)
	}
	defer lock.Close() //nolint:errcheck // Test cleanup.
	if opened, err := OpenReadOnly(dir, dir); err == nil {
		opened.Close() //nolint:errcheck // Unexpected data admission is the failure.
		t.Fatal("maintenance granted root a data opener")
	}
}
