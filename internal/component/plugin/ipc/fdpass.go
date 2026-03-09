// Design: docs/architecture/api/process-protocol.md — plugin process management
// Related: socketpair.go — socketpair creation for plugin IPC

package ipc

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// SendFD sends a file descriptor over a Unix domain socket connection using SCM_RIGHTS.
// The connection must be a *net.UnixConn (created via socketpair, not net.Pipe).
// The caller should close their copy of f after SendFD returns successfully.
func SendFD(conn net.Conn, f *os.File) error {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("SendFD requires *net.UnixConn, got %T (net.Pipe connections do not support fd passing)", conn)
	}

	fd := int(f.Fd())
	rights := unix.UnixRights(fd)

	// WriteMsgUnix requires at least 1 byte of data alongside the control message.
	// The receiver reads this framing byte to know an fd was sent.
	_, _, err := uc.WriteMsgUnix([]byte{0}, rights, nil)
	if err != nil {
		return fmt.Errorf("WriteMsgUnix: %w", err)
	}

	return nil
}

// ReceiveFD receives a file descriptor from a Unix domain socket connection using SCM_RIGHTS.
// The connection must be a *net.UnixConn. The returned *os.File is the caller's responsibility
// to close. The fd has CloseOnExec set to prevent leaking into child processes.
func ReceiveFD(conn net.Conn) (*os.File, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("ReceiveFD requires *net.UnixConn, got %T", conn)
	}

	// OOB buffer sized for one fd: unix.CmsgLen(4) covers the cmsghdr + one int32 fd.
	oobBuf := make([]byte, unix.CmsgSpace(4))
	dataBuf := make([]byte, 1)

	_, oobn, _, _, err := uc.ReadMsgUnix(dataBuf, oobBuf)
	if err != nil {
		return nil, fmt.Errorf("ReadMsgUnix: %w", err)
	}

	scms, err := unix.ParseSocketControlMessage(oobBuf[:oobn])
	if err != nil {
		return nil, fmt.Errorf("ParseSocketControlMessage: %w", err)
	}

	for _, scm := range scms {
		fds, err := unix.ParseUnixRights(&scm)
		if err != nil {
			continue
		}
		if len(fds) == 0 {
			continue
		}

		// Set CloseOnExec on all received fds — macOS lacks MSG_CMSG_CLOEXEC.
		for _, fd := range fds {
			unix.CloseOnExec(fd)
		}

		// Use the first fd, close any extras.
		for _, extra := range fds[1:] {
			unix.Close(extra) //nolint:errcheck // best-effort cleanup of unexpected extra fds
		}

		return os.NewFile(uintptr(fds[0]), "received-fd"), nil
	}

	return nil, fmt.Errorf("no file descriptor in control message")
}
