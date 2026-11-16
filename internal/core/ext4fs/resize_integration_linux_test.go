// Design: docs/architecture/testing/qemu-integration.md -- the ext4 fixtures a guest test attaches
// Related: resize_linux.go -- the online resize under test
//
// The test needs `integration && linux`: it attaches a loop device and mounts
// ext4, which only root of the initial user namespace may do, so it runs in
// the QEMU guest on Ze's runtime kernel (`./le test qemu all`), over a fixture
// the host made with the persistent partition's format.

//go:build integration && linux

package ext4fs_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/ext4fs"
	"github.com/ze-software/ze/internal/test/ext4fixture"
)

// grownBytes is what the fixture's disk grows to: 1 GiB, four times the
// fixture, inside the reserved group-descriptor area of the format.
const grownBytes = 1 << 30

// TestOnlineResizeGrowsMountedFilesystem settles spec assumptions A-1 and A-5
// (inside the reserved area): the kernel grows a mounted ext4 filesystem of
// the persistent partition's format through EXT4_IOC_RESIZE_FS, for a root
// caller, and refuses to shrink it.
//
// Method: mount the format fixture through a loop device, read its block count
// from the superblock (which also proves the superblock offsets against what
// mkfs.ext4 was told), write a witness file, enlarge the loop device, resize
// while mounted, and read the superblock, the free space and the witness;
// then ask for one block less and require a refusal that changes nothing;
// then unmount and mount again, and read the size and the witness once more.
// The resize's duration is logged for A-10.
func TestOnlineResizeGrowsMountedFilesystem(t *testing.T) {
	m := ext4fixture.Mount(t, ext4fixture.PermFormat)

	before := readSuperblock(t, m.Device)
	if before.BlockSize != 4096 {
		t.Fatalf("block size %d, want 4096", before.BlockSize)
	}
	if want := uint64(ext4fixture.SizeBytes / 4096); before.BlockCount != want {
		t.Fatalf("block count before the resize %d, want %d (the fixture's size): the superblock read is wrong", before.BlockCount, want)
	}
	freeBefore := freeBytes(t, m.Dir)

	witness := bytes.Repeat([]byte("ze-resize-witness\n"), 4096)
	witnessPath := filepath.Join(m.Dir, "witness")
	if err := os.WriteFile(witnessPath, witness, 0o600); err != nil {
		t.Fatal(err)
	}

	m.Grow(t, grownBytes)
	target := uint64(grownBytes / 4096)
	start := time.Now()
	if err := ext4fs.Resize(m.Dir, target); err != nil {
		t.Fatalf("A-1 broken: the online resize of the mounted filesystem failed: %v", err)
	}
	t.Logf("A-10: the online resize from %d to %d blocks took %v", before.BlockCount, target, time.Since(start))

	syncFilesystem(t, m.Dir)
	after := readSuperblock(t, m.Device)
	if after.BlockCount != target {
		t.Fatalf("block count after the resize %d, want %d", after.BlockCount, target)
	}
	if gained := freeBytes(t, m.Dir) - freeBefore; gained < (grownBytes-ext4fixture.SizeBytes)*9/10 {
		t.Fatalf("the mounted filesystem gained %d free bytes, want at least nine tenths of %d", gained, grownBytes-ext4fixture.SizeBytes)
	}
	requireWitness(t, witnessPath, witness)

	err := ext4fs.Resize(m.Dir, target-1)
	if err == nil {
		t.Fatal("a resize to one block less than the filesystem succeeded: the filesystem shrank")
	}
	if !errors.Is(err, unix.EINVAL) {
		t.Logf("the shrink was refused with %v, not EINVAL", err)
	}
	if got := readSuperblock(t, m.Device).BlockCount; got != target {
		t.Fatalf("block count after the refused shrink %d, want %d", got, target)
	}

	m.Unmount(t)
	if got := readSuperblock(t, m.Device).BlockCount; got != target {
		t.Fatalf("block count read unmounted %d, want %d", got, target)
	}
	m.MountAgain(t)
	requireWitness(t, witnessPath, witness)
}

// TestResizeRefusesZeroBlocks holds the boundary the kernel is never asked
// about: a count of zero is refused before any call.
func TestResizeRefusesZeroBlocks(t *testing.T) {
	if err := ext4fs.Resize(t.TempDir(), 0); !errors.Is(err, ext4fs.ErrZeroBlockCount) {
		t.Fatalf("Resize(0) = %v, want ErrZeroBlockCount", err)
	}
}

func readSuperblock(t *testing.T, device string) ext4fs.Superblock {
	t.Helper()
	file, err := os.Open(device)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // read-only
	sb, err := ext4fs.ReadSuperblock(file)
	if err != nil {
		t.Fatal(err)
	}
	return sb
}

func freeBytes(t *testing.T, dir string) int64 {
	t.Helper()
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		t.Fatal(err)
	}
	return int64(st.Bfree) * st.Bsize //nolint:gosec // a fixture's block count fits
}

func syncFilesystem(t *testing.T, dir string) {
	t.Helper()
	file, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // read-only
	if err := unix.Syncfs(int(file.Fd())); err != nil {
		t.Fatal(err)
	}
}

func requireWitness(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s changed across the resize", path)
	}
}
