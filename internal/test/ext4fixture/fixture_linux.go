// Design: docs/architecture/testing/qemu-integration.md -- the ext4 fixtures a guest test attaches
// Overview: fixture.go -- the fixture set and how the host makes it

//go:build linux

package ext4fixture

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

// Mounted is a guest-local copy of a fixture, attached to a loop device and
// mounted. Its cleanup unmounts it and detaches the loop device. Not safe for
// concurrent use.
type Mounted struct {
	// Backing is the guest-local copy the loop device reads.
	Backing string
	// Device is the loop device node.
	Device string
	// Dir is the mount point.
	Dir string

	loop    *os.File
	mounted bool
}

// loopConfigureAttempts bounds the retries when another process takes the
// free loop device between LOOP_CTL_GET_FREE and LOOP_CONFIGURE.
const loopConfigureAttempts = 8

// copyChunkBytes is the unit of the sparse copy: a chunk of zeros is skipped,
// so the copy on the guest's tmpfs holds only the fixture's metadata.
const copyChunkBytes = 1 << 20

// Mount copies fixture into the test's temporary directory, attaches the copy
// to a free loop device and mounts it as ext4 with no option string, the way
// gokrazy mounts the persistent partition.
//
// It skips the test when DirEnv is unset: the fixtures exist only in a guest
// run. It fails the test when DirEnv is set and the fixture is missing, or
// when the process is not root.
func Mount(t *testing.T, fixture Fixture) *Mounted {
	t.Helper()
	dir := os.Getenv(DirEnv)
	if dir == "" {
		t.Skip(DirEnv + " is unset: the ext4 fixtures exist only in a guest run of ./le test qemu all")
	}
	if os.Geteuid() != 0 {
		t.Fatal("ext4fixture: a loop device and an ext4 mount need root; run the test as root in the guest")
	}
	source := filepath.Join(dir, string(fixture))
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("ext4fixture: %v; make the fixtures with ./le test qemu guest-binaries", err)
	}
	work := t.TempDir()
	m := &Mounted{Backing: filepath.Join(work, "filesystem.img"), Dir: filepath.Join(work, "mnt")}
	if err := copySparse(source, m.Backing); err != nil {
		t.Fatalf("ext4fixture: copy %s: %v", fixture, err)
	}
	if err := os.Mkdir(m.Dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := m.attach(); err != nil {
		t.Fatalf("ext4fixture: attach %s: %v", fixture, err)
	}
	t.Cleanup(func() { m.release(t) })
	m.MountAgain(t)
	return m
}

// MountAgain mounts the device on Dir after Unmount.
func (m *Mounted) MountAgain(t *testing.T) {
	t.Helper()
	if err := unix.Mount(m.Device, m.Dir, "ext4", 0, ""); err != nil {
		t.Fatalf("ext4fixture: mount %s on %s: %v", m.Device, m.Dir, err)
	}
	m.mounted = true
}

// Unmount unmounts Dir and keeps the loop device attached.
func (m *Mounted) Unmount(t *testing.T) {
	t.Helper()
	if err := unix.Unmount(m.Dir, 0); err != nil {
		t.Fatalf("ext4fixture: unmount %s: %v", m.Dir, err)
	}
	m.mounted = false
}

// Grow enlarges the backing copy to sizeBytes and tells the loop device its
// new capacity, which is what a larger disk under a partition looks like to
// the filesystem.
func (m *Mounted) Grow(t *testing.T, sizeBytes int64) {
	t.Helper()
	if err := os.Truncate(m.Backing, sizeBytes); err != nil {
		t.Fatal(err)
	}
	if err := unix.IoctlSetInt(int(m.loop.Fd()), unix.LOOP_SET_CAPACITY, 0); err != nil {
		t.Fatalf("ext4fixture: LOOP_SET_CAPACITY on %s: %v", m.Device, err)
	}
}

// release unmounts and detaches. The loop device was configured to clear
// itself on its last close, so closing it after the unmount detaches it.
func (m *Mounted) release(t *testing.T) {
	if m.mounted {
		if err := unix.Unmount(m.Dir, 0); err != nil {
			t.Errorf("ext4fixture: unmount %s: %v", m.Dir, err)
		}
	}
	if err := m.loop.Close(); err != nil {
		t.Errorf("ext4fixture: close %s: %v", m.Device, err)
	}
}

// attach configures a free loop device over Backing.
func (m *Mounted) attach() error {
	control, err := os.OpenFile("/dev/loop-control", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer control.Close() //nolint:errcheck // a control node opened only for an ioctl
	backing, err := os.OpenFile(filepath.Clean(m.Backing), os.O_RDWR, 0)
	if err != nil {
		return err
	}
	// The loop device takes its own reference to the backing file.
	defer backing.Close() //nolint:errcheck // the loop device holds its own reference
	for range loopConfigureAttempts {
		number, err := unix.IoctlRetInt(int(control.Fd()), unix.LOOP_CTL_GET_FREE)
		if err != nil {
			return err
		}
		device := "/dev/loop" + strconv.Itoa(number)
		loop, err := os.OpenFile(filepath.Clean(device), os.O_RDWR, 0)
		if err != nil {
			return err
		}
		config := unix.LoopConfig{Fd: uint32(backing.Fd())} //nolint:gosec // a descriptor number is never negative
		config.Info.Flags = unix.LO_FLAGS_AUTOCLEAR
		err = unix.IoctlLoopConfigure(int(loop.Fd()), &config)
		if err == nil {
			m.Device = device
			m.loop = loop
			return nil
		}
		loop.Close() //nolint:errcheck,gosec // the configure error is the one that matters
		if !errors.Is(err, unix.EBUSY) {
			return err
		}
	}
	return errors.New("every free loop device was taken before it could be configured")
}

// copySparse copies source to target and skips each chunk of zeros, so the
// target is as sparse as the fixture's content allows. The source is read in
// full: the 9p share answers no hole query.
func copySparse(source, target string) error {
	in, err := os.Open(filepath.Clean(source))
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only
	out, err := os.Create(filepath.Clean(target))
	if err != nil {
		return err
	}
	buf := make([]byte, copyChunkBytes)
	var size int64
	for {
		n, readErr := io.ReadFull(in, buf)
		if n > 0 {
			if err := writeChunk(out, buf[:n]); err != nil {
				out.Close() //nolint:errcheck,gosec // the write error is the one reported
				return err
			}
			size += int64(n)
		}
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
		if readErr != nil {
			out.Close() //nolint:errcheck,gosec // the read error is the one reported
			return readErr
		}
	}
	if err := out.Truncate(size); err != nil {
		out.Close() //nolint:errcheck,gosec // the truncate error is the one reported
		return err
	}
	return out.Close()
}

// writeChunk writes chunk, or seeks past it when it holds only zeros.
func writeChunk(out *os.File, chunk []byte) error {
	for _, b := range chunk {
		if b != 0 {
			_, err := out.Write(chunk)
			return err
		}
	}
	_, err := out.Seek(int64(len(chunk)), io.SeekCurrent)
	return err
}
