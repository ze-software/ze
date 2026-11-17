//go:build linux

// Design: docs/architecture/container/network.md -- the namespace a link is made in
// Related: attach.go -- the created call that sends the inode

package network

import (
	"errors"
	"os"
	"strconv"
	"syscall"
)

// namespaceInode answers the inode number of the network namespace of pid.
// The link request carries it beside the process id, and the interface
// component refuses the request when the namespace of that process has
// another inode, so a reused process id cannot redirect the link.
func namespaceInode(pid int) (uint64, error) {
	path := "/proc/" + strconv.Itoa(pid) + "/ns/net"
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("read " + path + ": the file system answered no inode")
	}
	return stat.Ino, nil
}
