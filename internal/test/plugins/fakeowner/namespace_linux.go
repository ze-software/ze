//go:build linux

// Design: docs/architecture/api/process-protocol.md -- owned interface devices
// Related: fakeowner.go -- the link request that targets the namespace

package fakeowner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// holdNamespaceArg is the CLI word that runs the namespace holder.
const holdNamespaceArg = "hold-namespace"

// namespaceHolder is a child process in a network namespace of its own,
// created by cloning it with CLONE_NEWNET. It blocks on its stdin, so closing
// the pipe ends it and the namespace with it. Not safe for concurrent use:
// one goroutine starts it and one closes it. close MUST be called once.
type namespaceHolder struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	pid     int
	inode   uint64
}

// startNamespaceHolder runs the ze binary as `ze plugin fakeowner
// hold-namespace` in a new network namespace, and reads the inode number of
// that namespace. ctx bounds the holder's life. Creating a network namespace
// needs CAP_SYS_ADMIN.
func startNamespaceHolder(ctx context.Context) (*namespaceHolder, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate the ze binary: %w", err)
	}
	command := exec.CommandContext(ctx, self, "plugin", Name, holdNamespaceArg) // #nosec G204 G702 -- self is os.Executable, this running ze binary's own path, and every argument is a constant, so no request, config or peer value reaches the command line.
	command.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWNET}
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open the holder's stdin: %w", err)
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start the namespace holder: %w", err)
	}
	holder := &namespaceHolder{command: command, stdin: stdin, pid: command.Process.Pid}
	inode, err := namespaceInode(holder.pid)
	if err != nil {
		holder.close()
		return nil, err
	}
	holder.inode = inode
	return holder, nil
}

// namespaceInode answers the inode number of the network namespace of pid.
func namespaceInode(pid int) (uint64, error) {
	path := "/proc/" + strconv.Itoa(pid) + "/ns/net"
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("read " + path + ": the file system answered no inode")
	}
	return stat.Ino, nil
}

// close ends the holder by closing its stdin, and waits for it.
func (h *namespaceHolder) close() {
	h.stdin.Close()  //nolint:errcheck // the holder ends on EOF, or on the run context's kill, either way
	h.command.Wait() //nolint:errcheck // the holder's exit status carries nothing the test reads
}

// holdNamespace is the holder's own process: it keeps its namespace alive
// until its stdin closes.
func holdNamespace() int {
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		return 1
	}
	return 0
}
