package mlockexe

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// mlockOnFault is MLOCK_ONFAULT from the kernel's asm-generic/mman-common.h,
// which x/sys/unix doesn't define. It has the same value on all architectures.
const mlockOnFault = 0x01

func lock(onFault bool) (int64, error) {
	exe, err := os.Stat("/proc/self/exe")
	if err != nil {
		return 0, err
	}
	stat, ok := exe.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("unexpected stat type for /proc/self/exe")
	}
	exeInode := strconv.FormatUint(stat.Ino, 10)
	exePath, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return 0, err
	}
	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return 0, err
	}
	var locked int64
	var errs []error
	for line := range strings.Lines(string(maps)) {
		// Each line starts with the format
		//
		//	00400000-0219f000 r-xp 00000000 09:02 226050    /usr/local/bin/example
		//
		// The pathname can contain whitespace, so only use the five fixed fields.
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		// PROT_NONE gap mappings can't be locked.
		if !strings.HasPrefix(fields[1], "r") {
			continue
		}
		if fields[4] != exeInode {
			continue
		}
		startHex, endHex, ok := strings.Cut(fields[0], "-")
		if !ok {
			continue
		}
		start, err1 := strconv.ParseUint(startHex, 16, 64)
		end, err2 := strconv.ParseUint(endHex, 16, 64)
		if err1 != nil || err2 != nil || end <= start {
			errs = append(errs, fmt.Errorf("malformed maps line: %q", line))
			continue
		}
		mapFile := fmt.Sprintf("/proc/self/map_files/%x-%x", start, end)
		target, err := os.Readlink(mapFile)
		if err != nil {
			errs = append(errs, fmt.Errorf("readlink %s: %w", mapFile, err))
			continue
		}
		if target != exePath {
			continue
		}
		var errno syscall.Errno
		if onFault {
			_, _, errno = unix.Syscall(unix.SYS_MLOCK2,
				uintptr(start), uintptr(end-start), mlockOnFault)
		} else {
			_, _, errno = unix.Syscall(unix.SYS_MLOCK,
				uintptr(start), uintptr(end-start), 0)
		}
		if errno != 0 {
			errs = append(errs, fmt.Errorf("mlock %s %s: %w", fields[0], fields[1], errno))
			continue
		}
		locked += int64(end - start)
	}
	if len(errs) > 0 {
		return locked, errors.Join(errs...)
	}
	if locked == 0 {
		return 0, errors.New("no executable mappings found in /proc/self/maps")
	}
	return locked, nil
}
