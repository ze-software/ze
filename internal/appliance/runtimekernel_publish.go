// Design: docs/architecture/appliance/build-artifacts.md -- one build per key, published by one rename
// Related: cmd_kernel.go -- resolveRuntimeKernel, the cold path that builds through this file
// Related: runtimekernel_cached.go -- CachedRuntimeKernelTree, the reader a test uses

package appliance

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// A runtime kernel entry <entry> in the cache namespace has two siblings: the
// lock file ".<entry>.lock", which one build of that key holds from its check
// to its publication, and the staging directories ".<entry>.staging-<random>",
// which a build writes into. Both are dot-prefixed, so no reader or eviction
// takes either for an entry.
const (
	runtimeKernelLockSuffix   = ".lock"
	runtimeKernelStagingInfix = ".staging-"
)

// runtimeKernelBuild is the cold path's work for one cache entry: build writes
// the kernel tree into the staging directory it is given, and validate checks
// a staged tree before it is published. Both are called with the key's lock held.
type runtimeKernelBuild struct {
	artifact string
	build    func(staging string) error
	validate func(staging string) error
}

// runtimeKernelWaitNotice tells the operator that this run waits for the build
// of entry that the process holder runs, and will reuse it. A variable so a test
// observes the wait instead of timing it.
var runtimeKernelWaitNotice = func(entry, holder string) {
	var tb textbuf.Buffer
	tb.Str("runtime kernel ").Str(entry).Str(": another run (pid ").Str(holder).
		Str(") is building it. This run waits for that build and reuses it\n").StdErr() //nolint:errcheck // progress output
}

// publishRuntimeKernel builds the entry at cachedDir once, whatever the number of
// runs that ask for it, and publishes it by one rename.
//
// The key's lock is held from the check to the rename, so a second run waits
// and then finds the entry the first published. The tree is built into a
// staging directory in the same namespace, so the rename that publishes it
// cannot cross a filesystem: until it, the entry does not exist, and after it,
// the entry is whole. No timeout, kill or later failure can leave a partial
// entry. A staging directory a killed or failed run left behind is reclaimed by
// the next build of the key: published when its build finished (its provenance
// record is there and it validates), else removed.
func publishRuntimeKernel(cachedDir string, run runtimeKernelBuild) (string, error) {
	namespace, entry := filepath.Dir(cachedDir), filepath.Base(cachedDir)
	if err := os.MkdirAll(namespace, cacheDirPerm); err != nil { //nolint:gosec // G301: public kernel trees use 0755 parent directories; only the owner can write.
		return "", fmt.Errorf("create the runtime kernel cache %s: %w", namespace, err)
	}
	lock, err := lockRuntimeKernelEntry(namespace, entry)
	if err != nil {
		return "", err
	}
	defer lock.Close() //nolint:errcheck // closing drops the hold; process exit drops it as well
	// The run that held the lock before this one may have published the entry.
	if _, err := os.Stat(filepath.Join(cachedDir, run.artifact)); err == nil {
		return cachedDir, nil
	}
	adopted, err := reclaimRuntimeKernelStaging(namespace, entry, cachedDir, run)
	if err != nil {
		return "", err
	}
	if adopted {
		return cachedDir, nil
	}
	staging, err := os.MkdirTemp(namespace, "."+entry+runtimeKernelStagingInfix)
	if err != nil {
		return "", fmt.Errorf("create the runtime kernel staging directory in %s: %w", namespace, err)
	}
	if err := run.build(staging); err != nil {
		// An error does not prove Docker removed every writer, so this run keeps
		// its staging directory rather than race a worker or an ownership-repair
		// container. It is not an entry, and the next build of the key reclaims it.
		return "", fmt.Errorf("%w; kept this run's staging directory %s because the builder's teardown was not established; "+
			"the next build of this kernel removes it", err, staging)
	}
	if err := run.validate(staging); err != nil {
		if removeErr := os.RemoveAll(staging); removeErr != nil {
			return "", errors.Join(err, fmt.Errorf("remove the refused staging directory %s: %w", staging, removeErr))
		}
		return "", err
	}
	if err := installRuntimeKernel(staging, cachedDir); err != nil {
		if removeErr := os.RemoveAll(staging); removeErr != nil {
			return "", errors.Join(err, fmt.Errorf("remove the staging directory %s: %w", staging, removeErr))
		}
		return "", err
	}
	return cachedDir, nil
}

// reclaimRuntimeKernelStaging handles the staging directories earlier runs left
// for entry. One whose build finished, which its provenance record shows because
// the builder writes it last, and which validates, is published, and the answer
// is true. Every other one is removed. A removal that fails is reported and
// skipped: each build stages in a directory of its own, so it blocks nothing.
// MUST be called with the key's lock held, so no live run writes into them.
func reclaimRuntimeKernelStaging(namespace, entry, cachedDir string, run runtimeKernelBuild) (bool, error) {
	names, err := os.ReadDir(namespace)
	if err != nil {
		return false, fmt.Errorf("read the runtime kernel cache %s: %w", namespace, err)
	}
	prefix := "." + entry + runtimeKernelStagingInfix
	for _, name := range names {
		if !name.IsDir() {
			continue
		}
		if !strings.HasPrefix(name.Name(), prefix) {
			continue
		}
		staging := filepath.Join(namespace, name.Name())
		if runtimeKernelStagingFinished(staging, run) {
			if err := installRuntimeKernel(staging, cachedDir); err != nil {
				return false, err
			}
			return true, nil
		}
		if err := os.RemoveAll(staging); err != nil {
			var tb textbuf.Buffer
			tb.Str("warning: cannot remove the runtime kernel staging directory ").Str(staging).
				Str(" an earlier build left: ").Err(err).
				Str(". It is not a cache entry and blocks nothing; remove it by hand:\nrm -rf ").
				Str(shellArg(staging)).Byte('\n').StdErr() //nolint:errcheck // warning output
		}
	}
	return false, nil
}

// runtimeKernelStagingFinished answers whether a staging directory holds a
// build that finished: the provenance record the builder writes after the
// kernel, and a tree that validates.
func runtimeKernelStagingFinished(staging string, run runtimeKernelBuild) bool {
	if _, err := os.Stat(filepath.Join(staging, kernelbuilder.ProvenanceName)); err != nil {
		return false
	}
	return run.validate(staging) == nil
}

// installRuntimeKernel publishes staging as cachedDir by one rename. MUST be
// called with the key's lock held. An entry already at cachedDir holds no kernel
// (the caller checked under the lock): it is a partial tree from before
// publication was atomic, never complete, so it is removed first.
func installRuntimeKernel(staging, cachedDir string) error {
	if _, err := os.Lstat(cachedDir); err == nil {
		if err := os.RemoveAll(cachedDir); err != nil {
			return fmt.Errorf("remove the incomplete runtime kernel entry %s: %w", cachedDir, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect the runtime kernel entry %s: %w", cachedDir, err)
	}
	if err := os.Rename(staging, cachedDir); err != nil {
		return fmt.Errorf("publish the runtime kernel %s as %s: %w", staging, cachedDir, err)
	}
	return nil
}

// lockRuntimeKernelEntry takes the build lock of entry, waiting while another
// run holds it, and answers the open lock file: closing it drops the hold, and
// the kernel drops it when the process exits, however it exits, so a killed
// build never leaves the key locked. The holder's pid is written into the file
// so a waiting run can name it.
func lockRuntimeKernelEntry(namespace, entry string) (*os.File, error) {
	path := filepath.Join(namespace, "."+entry+runtimeKernelLockSuffix)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644) //nolint:gosec // the lock file beside the cache entry
	if err != nil {
		return nil, fmt.Errorf("open the runtime kernel build lock %s: %w", path, err)
	}
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		runtimeKernelWaitNotice(entry, runtimeKernelLockHolder(file))
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
	}
	if err != nil {
		_ = file.Close() //nolint:errcheck // the lock error below is the answer
		return nil, fmt.Errorf("lock the runtime kernel build %s: %w", path, err)
	}
	if err := file.Truncate(0); err != nil {
		_ = file.Close() //nolint:errcheck // closing drops the hold; the write error is the answer
		return nil, fmt.Errorf("record this run's pid in %s: %w", path, err)
	}
	if _, err := file.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0); err != nil {
		_ = file.Close() //nolint:errcheck // closing drops the hold; the write error is the answer
		return nil, fmt.Errorf("record this run's pid in %s: %w", path, err)
	}
	return file, nil
}

// runtimeKernelLockHolder answers the pid the lock's holder wrote, or says why
// there is none to name.
func runtimeKernelLockHolder(file *os.File) string {
	buffer := make([]byte, 32)
	count, err := file.ReadAt(buffer, 0)
	pid := strings.TrimSpace(string(buffer[:count]))
	if pid != "" {
		return pid
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return "unknown: " + err.Error()
	}
	return "unknown: not written yet"
}
