// Design: docs/architecture/api/commands.md -- complete configuration inventory verdicts.
package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestConfigListRefusesClosedStore reaches the real store's fs.ErrClosed
// result through the root command rather than reporting an empty inventory.
func TestConfigListRefusesClosedStore(t *testing.T) {
	store := storage.CreateMemory()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdListWithStorage(store, nil)
	})
	if code == exitOK {
		t.Error("closed store listing reported success")
	}
	if stdout != "" {
		t.Errorf("closed store published inventory: %q", stdout)
	}
	if !strings.Contains(stderr, fs.ErrClosed.Error()) {
		t.Errorf("refusal lost the real closed-store cause: %q", stderr)
	}
	if strings.Contains(stderr, "No config files found") {
		t.Errorf("closed inventory was reported as absent: %q", stderr)
	}
}

type inventoryListFault struct {
	storage.Storage
	prefix string
}

func (s *inventoryListFault) List(prefix string) ([]string, error) {
	if prefix == s.prefix {
		return nil, fmt.Errorf("read inventory directory: %w", fs.ErrPermission)
	}
	return s.Storage.List(prefix)
}

// TestConfigListRefusesUnreadableInventory faults each native store listing
// after preparing both real kinds of record. A partial inventory must not be
// printed as complete, and the caller must receive the storage failure.
func TestConfigListRefusesUnreadableInventory(t *testing.T) {
	for _, prefix := range []string{zefs.KeyFileActive.Dir(), zefs.KeyFileDraft.Dir()} {
		t.Run(prefix, func(t *testing.T) {
			store := storage.CreateMemory()
			t.Cleanup(func() {
				if err := store.Close(); err != nil {
					t.Error(err)
				}
			})
			for _, key := range []string{zefs.KeyFileActive.Key("router.conf"), zefs.KeyFileDraft.Key("draft.conf")} {
				if err := store.WriteKey(key, []byte("content")); err != nil {
					t.Fatal(err)
				}
			}
			fault := &inventoryListFault{Storage: store, prefix: prefix}
			code, stdout, stderr := captureBlobStdio(t, func() int {
				return cmdListWithStorage(fault, nil)
			})
			if code == exitOK {
				t.Error("unreadable store listing reported success")
			}
			if stdout != "" {
				t.Errorf("unreadable store listing published a partial inventory: %q", stdout)
			}
			if !strings.Contains(stderr, fs.ErrPermission.Error()) {
				t.Errorf("refusal lost the storage cause: %q", stderr)
			}
			if strings.Contains(stderr, "No config files found") {
				t.Errorf("unreadable inventory was reported as absent: %q", stderr)
			}
		})
	}
}

// TestConfigListKeepsReadableInventory admits complete real active and draft
// records, distinguishing the failure refusal from a blanket inventory ban.
func TestConfigListKeepsReadableInventory(t *testing.T) {
	store := storage.CreateMemory()
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	keys := []string{zefs.KeyFileActive.Key("router.conf"), zefs.KeyFileDraft.Key("draft.conf")}
	for _, key := range keys {
		if err := store.WriteKey(key, []byte("content")); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdListWithStorage(store, nil)
	})
	if code != exitOK {
		t.Fatalf("readable inventory was refused: %d %q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("readable inventory reported an error: %q", stderr)
	}
	for _, key := range keys {
		if !strings.Contains(stdout, "[data] "+key+"\n") {
			t.Errorf("readable inventory omitted %s: %q", key, stdout)
		}
	}
}

// TestConfigListPermitsEmptyInventory keeps a genuinely empty store usable;
// absent prefixes and empty optional search directories are not read failures.
func TestConfigListPermitsEmptyInventory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	store := storage.CreateMemory()
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdListWithStorage(store, nil)
	})
	if code != exitOK {
		t.Fatalf("empty inventory was refused: %d %q", code, stderr)
	}
	if stdout != "" {
		t.Errorf("empty inventory invented records: %q", stdout)
	}
	if !strings.Contains(stderr, "No config files found") {
		t.Errorf("empty inventory lost its ordinary guidance: %q", stderr)
	}
}

// TestConfigListRefusesUnreadableSearchDirectory uses a regular file where
// the declared XDG search directory should be. The deterministic ENOTDIR must
// refuse the complete inventory rather than publish the already-read records.
func TestConfigListRefusesUnreadableSearchDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if err := os.WriteFile(filepath.Join(xdg, "ze"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := storage.CreateMemory()
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := store.WriteKey(zefs.KeyFileActive.Key("router.conf"), []byte("content")); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdListWithStorage(store, nil)
	})
	if code == exitOK {
		t.Error("unreadable search directory reported success")
	}
	if stdout != "" {
		t.Errorf("unreadable search directory published a partial inventory: %q", stdout)
	}
	if !strings.Contains(stderr, "not a directory") {
		t.Errorf("refusal lost the search-directory cause: %q", stderr)
	}
}
