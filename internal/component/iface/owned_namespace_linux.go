//go:build linux

// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Related: owned_bridge_link.go -- CreateOwnedLink, the one caller

package iface

import (
	"os"
	"strconv"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// openOwnedLinkNamespace opens the network namespace of process pid and
// answers it only when its inode number is inode. The check reads the opened
// descriptor, not the path, and the caller creates the link through that same
// descriptor: a process id reused between the request and this call names
// another namespace, whose inode differs, so the link can never land in it.
// The caller MUST close the answer.
func openOwnedLinkNamespace(pid int, inode uint64) (*os.File, error) {
	path := "/proc/" + strconv.Itoa(pid) + "/ns/net"
	namespace, err := os.Open(path) // #nosec G304 -- the path is built from an integer process id, no caller text reaches it
	if err != nil {
		return nil, opererr.Wrap(err,
			"The network namespace of process "+strconv.Itoa(pid)+" cannot be opened",
			"The link's peer end is created inside that namespace, so without it no link was made",
			"Ask for the link while the process that holds the namespace runs; ps -p "+strconv.Itoa(pid)+" shows whether it does",
		)
	}
	var stat unix.Stat_t
	if err := unix.Fstat(int(namespace.Fd()), &stat); err != nil {
		namespace.Close() //nolint:errcheck // the fstat failure is the answer
		return nil, opererr.Wrap(err,
			"The network namespace of process "+strconv.Itoa(pid)+" cannot be read",
			"Its inode number is what proves the namespace is the one the request names, so no link was made",
			"Ask for the link again; ze show log names the kernel's answer",
		)
	}
	if stat.Ino != inode {
		namespace.Close() //nolint:errcheck // the mismatch is the answer
		var why textbuf.Buffer
		why.Str("The request carries inode ").Uint(inode).Str(" and the namespace has inode ").Uint(stat.Ino).
			Str(", so the process id names another process now and no link was made")
		return nil, opererr.New(
			"The network namespace of process "+strconv.Itoa(pid)+" is not the one the request names",
			why.String(),
			"Read the namespace inode of the process again and send the request with both values of the same moment",
		)
	}
	return namespace, nil
}
