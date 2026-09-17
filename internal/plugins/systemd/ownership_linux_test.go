//go:build linux

package systemd

import (
	"errors"
	"os"
	"os/user"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestInstallRefusesOwnershipTransferOfRunningStore drives AC-20 through the
// real ownership path `ze install systemd` runs (realServiceOps.transferOwnership
// -> storage.TransferOwnership): a store a running owner holds is refused, so
// an install never chowns a live daemon's database out from under it. The
// store's owner lock is only reachable as root, because the producer refuses
// a non-root caller first; a non-root run proves that refusal instead.
func TestInstallRefusesOwnershipTransferOfRunningStore(t *testing.T) {
	dir := t.TempDir()
	running, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create the running owner's store: %v", err)
	}
	t.Cleanup(func() { _ = running.Close() })

	account, err := user.Current()
	if err != nil {
		t.Fatalf("current user: %v", err)
	}
	group, err := user.LookupGroupId(account.Gid)
	if err != nil {
		t.Fatalf("current group: %v", err)
	}

	err = realServiceOps{stdout: os.Stdout, stderr: os.Stderr}.transferOwnership(dir, account.Username, group.Name)
	if err == nil {
		t.Fatalf("ownership transfer of a store held by a running owner succeeded")
	}
	if os.Geteuid() == 0 {
		if !errors.Is(err, storage.ErrBusy) {
			t.Fatalf("root transfer of a held store: got %v, want %v", err, storage.ErrBusy)
		}
		return
	}
	if !errors.Is(err, storage.ErrPermissions) {
		t.Fatalf("non-root transfer: got %v, want %v", err, storage.ErrPermissions)
	}
	if got := err.Error(); !strings.Contains(got, "run ownership maintenance as root") {
		t.Fatalf("non-root refusal does not name the repair: %s", got)
	}
}
