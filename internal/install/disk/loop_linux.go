// Design: plan/spec-installer-initrd-pure-go.md -- loop device attach/detach via ioctls

//go:build linux

package disk

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const loopMaxDev = 8

func init() {
	ensureLoopDevices = sysEnsureLoopDevices
	loopAttach = sysLoopAttach
	loopDetach = sysLoopDetach
}

func sysEnsureLoopDevices() {
	for i := range loopMaxDev {
		var tb textbuf.Buffer
		dev := tb.Str("/dev/loop").Int(int64(i)).String()
		if _, err := os.Stat(dev); err == nil {
			continue
		}
		unix.Mknod(dev, unix.S_IFBLK|0o660, int(unix.Mkdev(7, uint32(i)))) //nolint:errcheck // best-effort node creation
	}
}

func sysLoopAttach(file string) (string, error) {
	backing, err := os.Open(file) //nolint:gosec // installer-controlled path
	if err != nil {
		return "", fmt.Errorf("open %s: %w", file, err)
	}
	defer backing.Close() //nolint:errcheck // read-only

	for i := range loopMaxDev {
		var tb textbuf.Buffer
		dev := tb.Str("/dev/loop").Int(int64(i)).String()
		loopFd, openErr := unix.Open(dev, unix.O_RDWR, 0)
		if openErr != nil {
			continue
		}
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(loopFd), unix.LOOP_SET_FD, uintptr(backing.Fd()))
		unix.Close(loopFd)
		if errno == 0 {
			return dev, nil
		}
	}
	return "", fmt.Errorf("no free loop device for %s", file)
}

func sysLoopDetach(dev string) {
	fd, err := unix.Open(dev, unix.O_RDWR, 0)
	if err != nil {
		return
	}
	unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.LOOP_CLR_FD, 0) //nolint:errcheck // best-effort cleanup
	unix.Close(fd)
}
