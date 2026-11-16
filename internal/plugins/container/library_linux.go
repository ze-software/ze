// Design: docs/architecture/container/runtime.md -- the runtime library behind the supervisor
// Related: describe_linux.go -- the description the library creates from
// Related: rootfs_linux.go -- the root it mounts first
// Related: supervise.go -- runtimeLibrary and createdContainer, which this file implements

package container

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer"

	"github.com/ze-software/ze/internal/core/opererr"
)

// cgroupMount is where the runtime library reads the cgroup v2 hierarchy. The
// library fixes it there.
const cgroupMount = "/sys/fs/cgroup"

// signalExitBase is added to the signal that killed init to make its exit
// code, as a shell reports it.
const signalExitBase = 128

// patchedLibrary drives the vendored runtime library, as patched, for the
// supervisor. It holds no state of its own: each container is a
// libraryContainer.
type patchedLibrary struct{}

// newRuntimeLibrary answers the runtime library of this platform. The
// library's own log lines go through its global logger, which the log bridge
// work points at Ze's.
func newRuntimeLibrary(*slog.Logger) runtimeLibrary {
	return patchedLibrary{}
}

// create mounts the container's root, describes it and starts its init,
// which blocks before the entrypoint. A failure removes what it made: the
// root, and the control group and the state directory the library made
// before init started (the patched parent creates the control group first,
// and the library's own cleanup is not in place yet when the start fails).
func (patchedLibrary) create(_ context.Context, request *startRequest) (createdContainer, error) {
	name := request.instance.name
	process, err := describeEntrypoint(name, request.ready)
	if err != nil {
		return nil, err
	}
	rootfs, err := mountRoot(name, request.ready)
	if err != nil {
		return nil, err
	}
	container, err := libcontainer.Create(libraryStateRoot, name, describeContainer(request.instance, rootfs))
	if err != nil {
		return nil, errors.Join(createFailed(name, "the runtime library refused its description", err), unmountRoot(rootfs))
	}
	if err := container.Start(process); err != nil {
		return nil, errors.Join(createFailed(name, "its init did not start", err), container.Destroy(), unmountRoot(rootfs))
	}
	pid, err := process.Pid()
	if err != nil {
		return nil, errors.Join(createFailed(name, "its init has no process id", err), container.Destroy(), unmountRoot(rootfs))
	}
	return &libraryContainer{name: name, container: container, process: process, rootfs: rootfs, pid: pid}, nil
}

// libraryContainer is one container the runtime library created. Its methods
// follow the order createdContainer states; it is used by one supervisor
// worker and its watcher, never by two callers of one method at once.
type libraryContainer struct {
	name      string
	container *libcontainer.Container
	process   *libcontainer.Process
	rootfs    string
	pid       int
}

func (c *libraryContainer) initProcessID() int { return c.pid }

// networkNamespacePath names the network namespace of init, which every
// container has of its own.
func (c *libraryContainer) networkNamespacePath() string {
	return "/proc/" + strconv.Itoa(c.pid) + "/ns/net"
}

// release opens the library's exec fifo, which lets init execute the
// entrypoint.
func (c *libraryContainer) release() error {
	return c.container.Exec()
}

// wait reaps init. An init that exited with a failure or was killed is an
// answer, never an error: the library reports both as an exit error that
// carries the process state.
func (c *libraryContainer) wait() (int, error) {
	state, err := c.process.Wait()
	if state == nil {
		return 0, err
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return 0, err
	}
	status, isWaitStatus := state.Sys().(syscall.WaitStatus)
	if !isWaitStatus {
		return state.ExitCode(), nil
	}
	if status.Signaled() {
		return signalExitBase + int(status.Signal()), nil
	}
	return status.ExitStatus(), nil
}

// kill ends every process of the container through its control group: the
// kernel kills each process the group holds, whatever session, process group
// or namespace it joined.
func (c *libraryContainer) kill() error {
	return cgroups.WriteFile(filepath.Join(cgroupMount, cgroupParent, c.name), "cgroup.kill", "1")
}

// destroy removes the container's control group and its state directory
// through the library, then unmounts its root.
func (c *libraryContainer) destroy() error {
	return errors.Join(c.container.Destroy(), unmountRoot(c.rootfs))
}

func createFailed(name, step string, err error) error {
	return opererr.Wrap(err,
		"container "+name+" was not created: "+step,
		"the runtime library could not build the container, so nothing of it was left running",
		"ze request container restart name "+name,
	)
}
