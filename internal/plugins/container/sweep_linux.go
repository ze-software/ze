// Design: docs/architecture/container/runtime.md -- the sweep of what a previous run left
// Related: library_linux.go -- patchedLibrary.sweep, which calls it
// Related: rootfs_linux.go -- the run-time directories the sweep reads
// Related: describe_linux.go -- cgroupParent, the control group the sweep reads
// Related: supervise.go -- supervisor.sweep, which logs and reports what it found

package container

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/opencontainers/cgroups"
	"golang.org/x/sys/unix"
)

// The bound of the wait for a killed control group to empty: 250 steps of 20
// milliseconds, 5 seconds.
const (
	cgroupEmptyWaitStep  = 20 * time.Millisecond
	cgroupEmptyWaitSteps = 250
)

// sweepLeftovers removes every container a previous run of the plugin left,
// a daemon that was killed or crashed among them: the plugin's own control
// groups under cgroupParent, its root mounts under rootMountDirectory and the
// library's state directories under libraryStateRoot. Every entry there is
// the plugin's: no other code writes those directories. A container is named
// by its entry, whichever of the three holds it. It answers one entry per
// container, sorted by name; a directory it cannot list is answered as an
// entry named by its path.
func sweepLeftovers() []leftover {
	found := map[string]struct{}{}
	var unreadable []leftover
	for _, directory := range []string{filepath.Join(cgroupMount, cgroupParent), rootMountDirectory, libraryStateRoot} {
		names, err := childDirectories(directory)
		if err != nil {
			unreadable = append(unreadable, leftover{name: directory, err: err})
			continue
		}
		for _, name := range names {
			found[name] = struct{}{}
		}
	}
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	slices.Sort(names)
	swept := make([]leftover, 0, len(names)+len(unreadable))
	for _, name := range names {
		swept = append(swept, leftover{name: name, err: sweepContainer(name)})
	}
	return append(swept, unreadable...)
}

// childDirectories answers the names of the directories directory holds. A
// directory that does not exist holds none: nothing was left there.
func childDirectories(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// sweepContainer removes what is left of one container: it kills the
// processes of its control group and removes the group, unmounts its root and
// removes the mount point, and removes the library's state directory, which
// would otherwise refuse a new container of the same name. It does each step
// whatever the one before answered, and joins what failed.
func sweepContainer(name string) error {
	return errors.Join(
		killControlGroup(filepath.Join(cgroupMount, cgroupParent, name)),
		removeRootMount(filepath.Join(rootMountDirectory, name)),
		removeStateDirectory(filepath.Join(libraryStateRoot, name)),
	)
}

// killControlGroup kills every process of group, waits a bounded time for the
// group to empty and removes it. A group that does not exist is no error.
func killControlGroup(group string) error {
	_, statErr := os.Stat(group)
	if errors.Is(statErr, fs.ErrNotExist) {
		return nil
	}
	if statErr != nil {
		return statErr
	}
	if err := cgroups.WriteFile(group, "cgroup.kill", "1"); err != nil {
		return fmt.Errorf("kill the processes of control group %s: %w", group, err)
	}
	for range cgroupEmptyWaitSteps {
		populated, err := controlGroupPopulated(group)
		if err != nil {
			return err
		}
		if !populated {
			return removeControlGroup(group)
		}
		time.Sleep(cgroupEmptyWaitStep)
	}
	return errors.New("control group " + group + " still holds processes 5s after they were killed")
}

// controlGroupPopulated answers whether group or a group below it holds a
// process, from the populated line of its cgroup.events file.
func controlGroupPopulated(group string) (bool, error) {
	events, err := cgroups.ReadFile(group, "cgroup.events")
	if err != nil {
		return false, fmt.Errorf("read the events of control group %s: %w", group, err)
	}
	for line := range strings.SplitSeq(events, "\n") {
		value, found := strings.CutPrefix(line, "populated ")
		if found {
			return value != "0", nil
		}
	}
	return false, errors.New("the events of control group " + group + " hold no populated line")
}

// removeControlGroup removes an empty control group. One already gone is no
// error.
func removeControlGroup(group string) error {
	if err := unix.Rmdir(group); err != nil && !errors.Is(err, unix.ENOENT) {
		return fmt.Errorf("remove control group %s: %w", group, err)
	}
	return nil
}

// removeRootMount unmounts a container's root and removes its mount point. A
// mount point that is no longer mounted is only removed.
func removeRootMount(target string) error {
	_, statErr := os.Lstat(target)
	if errors.Is(statErr, fs.ErrNotExist) {
		return nil
	}
	if statErr != nil {
		return statErr
	}
	// EINVAL is the kernel's answer for a directory that is not a mount point.
	if err := unix.Unmount(target, 0); err != nil && !errors.Is(err, unix.EINVAL) {
		return fmt.Errorf("unmount the root %s: %w", target, err)
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove the mount point %s: %w", target, err)
	}
	return nil
}

// removeStateDirectory removes the library's state directory of a container.
func removeStateDirectory(directory string) error {
	if err := os.RemoveAll(directory); err != nil {
		return fmt.Errorf("remove the runtime library's state directory %s: %w", directory, err)
	}
	return nil
}
