// Design: docs/architecture/testing/qemu-integration.md -- the ext4 fixtures a guest test attaches
// Related: quota_linux.go -- the quota calls under test
//
// The tests need `integration && linux`: each attaches a loop device and
// mounts ext4, which only root of the initial user namespace may do, so they
// run in the QEMU guest on Ze's runtime kernel (`./le test qemu all`), over
// fixtures the host made with the persistent partition's format.

//go:build integration && linux

package projquota_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/ext4fs"
	"github.com/ze-software/ze/internal/core/projquota"
	"github.com/ze-software/ze/internal/test/ext4fixture"
)

const (
	limitBytes = 1 << 20
	writeBytes = 2 << 20
	blockBytes = 4096
	chunkBytes = 64 << 10
)

// TestProjectQuotaStoredOptionEnforcesAtMount settles spec assumption A-3 on
// the format fixture: the project-quota mount option stored in the superblock
// is applied at a mount that passes no option, so enforcement is on with no
// caller step.
//
// Method: mount the whole format with no option string, as gokrazy does, and
// probe it. StateNotEnforced here breaks A-3 and moves the design to E2.
func TestProjectQuotaStoredOptionEnforcesAtMount(t *testing.T) {
	m := ext4fixture.Mount(t, ext4fixture.PermFormat)
	if got := projquota.Probe(m.Dir); got.State != projquota.StateEnforced {
		t.Fatalf("A-3 broken: the stored mount option left project quota %s (%s); the design moves to E2", got.State, got.Reason)
	}
}

// TestProjectQuotaRefusesWritePastLimit proves spec AC-9 on the runtime
// kernel: a folder with a project id and a 1 MiB limit refuses a write past
// the limit with the quota error, and its usage reads within one block of the
// limit; a folder with no project takes the same write, so the refusal is the
// limit's and not the filesystem's.
func TestProjectQuotaRefusesWritePastLimit(t *testing.T) {
	m := ext4fixture.Mount(t, ext4fixture.PermFormat)
	if got := projquota.Probe(m.Dir); got.State != projquota.StateEnforced {
		t.Fatalf("project quota is %s (%s), want enforced: see TestProjectQuotaStoredOptionEnforcesAtMount", got.State, got.Reason)
	}
	const project = 11
	capped := mkdir(t, m.Dir, "capped")
	if err := projquota.AssignProject(capped, project); err != nil {
		t.Fatal(err)
	}
	if err := projquota.SetLimit(m.Dir, project, limitBytes); err != nil {
		t.Fatal(err)
	}
	written, err := writeUnprivileged(filepath.Join(capped, "payload"), writeBytes)
	if !errors.Is(err, unix.EDQUOT) {
		t.Fatalf("the write into the capped folder stopped at %d bytes with %v, want EDQUOT", written, err)
	}
	usage, err := projquota.ReadUsage(m.Dir, project)
	if err != nil {
		t.Fatal(err)
	}
	if usage.LimitBytes != limitBytes {
		t.Fatalf("the limit reads %d, want %d", usage.LimitBytes, limitBytes)
	}
	if usage.UsedBytes > limitBytes || limitBytes-usage.UsedBytes > blockBytes {
		t.Fatalf("project usage %d is not within one block of the limit %d", usage.UsedBytes, limitBytes)
	}

	control := mkdir(t, m.Dir, "control")
	if written, err := writeUnprivileged(filepath.Join(control, "payload"), writeBytes); err != nil {
		t.Fatalf("the folder with no project took %d bytes and failed with %v: the refusal above is not the limit's", written, err)
	}
}

// TestProjectQuotaEnableOnMountedFilesystem settles spec assumption A-4: with
// the quota feature and the project inode present, the kernel accounts
// project usage from mount, and the quota-on call turns enforcement on for the
// mounted filesystem.
//
// Method: mount the fixture without the stored option, require the state to
// read not enforced, give a folder a project, write into it and read the
// usage, call Enable, require enforced, then require a write past a limit to
// be refused.
func TestProjectQuotaEnableOnMountedFilesystem(t *testing.T) {
	m := ext4fixture.Mount(t, ext4fixture.PermFormatNoMountOption)
	if got := projquota.Probe(m.Dir); got.State != projquota.StateNotEnforced {
		t.Fatalf("project quota is %s (%s) before the quota-on call, want not-enforced", got.State, got.Reason)
	}
	const project = 21
	capped := mkdir(t, m.Dir, "capped")
	if err := projquota.AssignProject(capped, project); err != nil {
		t.Fatal(err)
	}
	if _, err := writeUnprivileged(filepath.Join(capped, "before"), chunkBytes); err != nil {
		t.Fatal(err)
	}
	usage, err := projquota.ReadUsage(m.Dir, project)
	if err != nil {
		t.Fatal(err)
	}
	if usage.UsedBytes < chunkBytes {
		t.Fatalf("A-4 broken: project usage reads %d after a %d-byte write with enforcement off: no accounting from mount", usage.UsedBytes, chunkBytes)
	}
	if err := projquota.Enable(m.Dir); err != nil {
		t.Fatalf("A-4 broken: %v", err)
	}
	if err := projquota.SetLimit(m.Dir, project, limitBytes); err != nil {
		t.Fatal(err)
	}
	written, err := writeUnprivileged(filepath.Join(capped, "after"), writeBytes)
	if !errors.Is(err, unix.EDQUOT) {
		t.Fatalf("after the quota-on call the write stopped at %d bytes with %v, want EDQUOT", written, err)
	}
}

// TestProjectQuotaStateWithoutFeature proves that a filesystem without the
// quota feature reads no-feature, never not-enforced, and that Enable refuses
// it.
//
// Method: mount the fixture that took the format step alone (the project
// feature, no quota feature), require its superblock to say so, and probe it.
// On Ze's 7.3 runtime kernel the quota state query answers ENOSYS on such a
// filesystem (guest run of 2026-11-16), so Probe answers from the superblock;
// the superblock check here is what makes that answer the right one, and the
// format fixture's superblock is read too, so a feature bit read from the
// wrong place fails here rather than reading every filesystem as no-feature.
func TestProjectQuotaStateWithoutFeature(t *testing.T) {
	withFeature := ext4fixture.Mount(t, ext4fixture.PermFormat)
	if sb := readSuperblock(t, withFeature.Device); !sb.QuotaFeature || !sb.ProjectFeature {
		t.Fatalf("the format fixture's superblock reads quota %v project %v, want both", sb.QuotaFeature, sb.ProjectFeature)
	}
	m := ext4fixture.Mount(t, ext4fixture.FormatStepOnly)
	if sb := readSuperblock(t, m.Device); sb.QuotaFeature || !sb.ProjectFeature {
		t.Fatalf("the format-step fixture's superblock reads quota %v project %v, want no quota and project", sb.QuotaFeature, sb.ProjectFeature)
	}
	if got := projquota.Probe(m.Dir); got.State != projquota.StateNoFeature {
		t.Fatalf("a filesystem without the quota feature reads %s (%s), want no-feature", got.State, got.Reason)
	}
	if err := projquota.Enable(m.Dir); !errors.Is(err, projquota.ErrNoFeature) {
		t.Fatalf("Enable on a filesystem without the quota feature = %v, want ErrNoFeature", err)
	}
}

// TestProjectQuotaRefusesBoundaries holds the refusals made before any kernel
// call: project 0, a limit under one block, a limit that is no whole number of
// quota blocks, a folder that is not empty.
func TestProjectQuotaRefusesBoundaries(t *testing.T) {
	dir := t.TempDir()
	if err := projquota.AssignProject(dir, 0); !errors.Is(err, projquota.ErrDefaultProject) {
		t.Errorf("AssignProject(0) = %v, want ErrDefaultProject", err)
	}
	if err := projquota.SetLimit(dir, 1, 0); !errors.Is(err, projquota.ErrLimitTooSmall) {
		t.Errorf("SetLimit(0 bytes) = %v, want ErrLimitTooSmall", err)
	}
	if err := projquota.SetLimit(dir, 1, projquota.LimitBytesMin-1); !errors.Is(err, projquota.ErrLimitTooSmall) {
		t.Errorf("SetLimit(4095 bytes) = %v, want ErrLimitTooSmall", err)
	}
	if err := projquota.SetLimit(dir, 1, projquota.LimitBytesMin+1); !errors.Is(err, projquota.ErrLimitNotWhole) {
		t.Errorf("SetLimit(4097 bytes) = %v, want ErrLimitNotWhole", err)
	}
	mkdir(t, dir, "entry")
	if err := projquota.AssignProject(dir, 1); !errors.Is(err, projquota.ErrFolderNotEmpty) {
		t.Errorf("AssignProject on a folder with an entry = %v, want ErrFolderNotEmpty", err)
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

func mkdir(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeUnprivileged writes size bytes into path from a thread without
// CAP_SYS_RESOURCE, and answers what writeFile answers.
//
// The test runs as root, and the quota code lets a writer that holds
// CAP_SYS_RESOURCE past every hard limit (recalled from the kernel's
// fs/quota/dquot.c, ignore_hardlimit; not read on the machine this was
// written on). A capability is a property of one thread, so the write runs on
// a goroutine locked to its thread that drops the capability from its
// effective set and never unlocks: the thread ends with the goroutine, and the
// test's other threads keep their capabilities. The goroutine is one worker
// for one call, which returns only after it has sent its one result.
func writeUnprivileged(path string, size int) (int, error) {
	type result struct {
		written int
		err     error
	}
	done := make(chan result, 1)
	go func() {
		runtime.LockOSThread() // never unlocked: the thread ends with this goroutine
		if err := dropResourceCapability(); err != nil {
			done <- result{err: err}
			return
		}
		written, err := writeFile(path, size)
		done <- result{written: written, err: err}
	}()
	r := <-done
	return r.written, r.err
}

// dropResourceCapability clears CAP_SYS_RESOURCE from the calling thread's
// effective set and checks that it is gone.
func dropResourceCapability() error {
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&header, &data[0]); err != nil {
		return err
	}
	data[0].Effective &^= 1 << unix.CAP_SYS_RESOURCE
	if err := unix.Capset(&header, &data[0]); err != nil {
		return err
	}
	if err := unix.Capget(&header, &data[0]); err != nil {
		return err
	}
	if data[0].Effective&(1<<unix.CAP_SYS_RESOURCE) != 0 {
		return errors.New("CAP_SYS_RESOURCE is still effective after capset")
	}
	return nil
}

// writeFile writes size bytes in chunks, stops at the first error, syncs, and
// answers how many bytes were written with the error.
func writeFile(path string, size int) (int, error) {
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	chunk := make([]byte, chunkBytes)
	for i := range chunk {
		chunk[i] = byte(i)
	}
	written := 0
	for written < size {
		n, err := file.Write(chunk[:min(chunkBytes, size-written)])
		written += n
		if err != nil {
			file.Close() //nolint:errcheck,gosec // the write error is the one reported
			return written, err
		}
	}
	if err := file.Sync(); err != nil {
		file.Close() //nolint:errcheck,gosec // the sync error is the one reported
		return written, err
	}
	return written, file.Close()
}
