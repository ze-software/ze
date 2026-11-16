// Design: docs/architecture/host/inventory.md -- the persistent storage read

//go:build linux

package host

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// persistentRoot answers a Detector over a fixture root whose mount table is
// mountinfo. An empty mountinfo writes no mount table at all.
func persistentRoot(t *testing.T, mountinfo string) *Detector {
	t.Helper()
	root := t.TempDir()
	if mountinfo == "" {
		return &Detector{Root: root}
	}
	dir := filepath.Join(root, "proc", "self")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("make the fixture proc folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mountinfo"), []byte(mountinfo), 0o600); err != nil {
		t.Fatalf("write the fixture mount table: %v", err)
	}
	return &Detector{Root: root}
}

// TestDetectPersistentStorageReadsTheMountTable asks what
// DetectPersistentStorage answers for each mount table a system can show it.
// The method reads /proc/self/mountinfo under the Detector's root, so each
// case writes that one file.
//
// VALIDATES: a system with nothing mounted on /perm answers
// ErrNoPersistentStorage; a mounted persistent partition answers an error that
// names its device, because the sizes are not read yet; a mount table that is
// missing or malformed answers an error that is not ErrNoPersistentStorage.
// PREVENTS: a system whose storage could not be read passing for a system with
// no persistent storage, which `ze doctor` reports as healthy.
func TestDetectPersistentStorageReadsTheMountTable(t *testing.T) {
	const (
		rootLine  = "25 1 8:2 / / ro,relatime shared:1 - squashfs /dev/sda2 ro\n"
		permLine  = "36 25 8:4 / /perm rw,relatime shared:2 - ext4 /dev/sda4 rw\n"
		laterLine = "41 25 259:4 / /perm rw,relatime - ext4 /dev/nvme0n1p4 rw\n"
		underLine = "37 25 8:5 / /perm/data rw,relatime - ext4 /dev/sda5 rw\n"
		cutLine   = "36 25 8:4 / /perm rw,relatime shared:2 - ext4\n"
		bareLine  = "36 25 8:4 / /perm rw,relatime shared:2\n"
	)
	for _, tc := range []struct {
		name      string
		mountinfo string
		want      error
		device    string
	}{
		{"nothing on the mount point", rootLine, ErrNoPersistentStorage, ""},
		{"a mount below the mount point only", rootLine + underLine, ErrNoPersistentStorage, ""},
		{"mounted", rootLine + permLine, errPersistentSizesNotImplemented, "/dev/sda4"},
		{"mounted twice", rootLine + permLine + laterLine, errPersistentSizesNotImplemented, "/dev/nvme0n1p4"},
		{"line cut after the filesystem type", rootLine + cutLine, errPersistentMountSource, ""},
		{"line without the separator", rootLine + bareLine, errPersistentMountSource, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage, err := persistentRoot(t, tc.mountinfo).DetectPersistentStorage()
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if storage != (PersistentStorage{}) {
				t.Errorf("an error came with a value: %+v", storage)
			}
			if !strings.Contains(err.Error(), tc.device) {
				t.Errorf("error %q does not name the device %q", err, tc.device)
			}
		})
	}

	t.Run("no mount table", func(t *testing.T) {
		_, err := persistentRoot(t, "").DetectPersistentStorage()
		if err == nil {
			t.Fatal("a missing mount table answered no error")
		}
		if errors.Is(err, ErrNoPersistentStorage) {
			t.Errorf("a missing mount table reads as no persistent storage: %v", err)
		}
	})
}
