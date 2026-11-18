// Design: docs/research/l2tpv2-ze-integration.md -- same-file unit stream handoff

//go:build linux

package kernel

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// This function borrows the exact original file while constructing its wake
// lane. On any error the consuming session lease still owns unit.Close.
func newInterruptibleCreatedUnit(unit *CreatedUnit, file *os.File) (io.ReadWriteCloser, error) {
	if unit == nil {
		return nil, errors.New("ppp: no original unit file to wrap")
	}
	if file == nil {
		return nil, errors.New("ppp: no original unit file to wrap")
	}
	if unit.file != file {
		return nil, errors.New("ppp: no original unit file to wrap")
	}
	return newInterruptibleSessionFile(file, "ppp.unit", unit.Close)
}

func newInterruptibleSessionFile(file *os.File, name string, closeFile func() error) (io.ReadWriteCloser, error) {
	if file == nil {
		return nil, errors.New("ppp: original stream file has no owner")
	}
	if closeFile == nil {
		return nil, errors.New("ppp: original stream file has no owner")
	}
	connection, err := file.SyscallConn()
	if err != nil {
		return nil, err
	}
	var modeErr error
	if err := connection.Control(func(fd uintptr) { modeErr = unix.SetNonblock(int(fd), false) }); err != nil {
		return nil, err
	}
	if modeErr != nil {
		return nil, fmt.Errorf("ppp: original unit blocking mode: %w", modeErr)
	}
	wakeFD, err := unix.Eventfd(0, unix.EFD_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("ppp: original unit wake file: %w", err)
	}
	return &interruptibleFDFile{channel: file,
		wake:      os.NewFile(uintptr(wakeFD), name+".wake"),
		readFrame: pollThenRead, closeChannel: closeFile}, nil
}
