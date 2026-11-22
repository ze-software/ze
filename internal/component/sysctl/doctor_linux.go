// Design: docs/features/ai-first.md -- the writability probe the readiness check runs
// Overview: doctor.go -- checkSysctlProcfs, the one caller

//go:build linux

package sysctl

import (
	"errors"

	"golang.org/x/sys/unix"
)

// probeProcSysPath is the probe checkSysctlProcfs runs per configured sysctl
// key. It is a variable so a test can stand in any kernel verdict; nothing else
// assigns it.
var probeProcSysPath = probeProcSysAccess

// probeProcSysAccess asks the kernel whether this process, with its effective
// UID and effective capabilities, can write path.
//
// It uses faccessat2(AT_FDCWD, path, W_OK, AT_EACCESS) directly rather than
// access(2) or unix.Faccessat:
//   - access(2) checks real UID with cap_effective cleared, which ignores
//     CAP_NET_ADMIN in Linux's net_ctl_permissions (net/sysctl_net.c);
//   - unix.Faccessat falls back on ENOSYS/EPERM to userspace fstatat +
//     CAP_DAC_OVERRIDE, which also ignores net_ctl_permissions.
//
// With AT_EACCESS, do_faccessat keeps current_cred() intact and invokes
// proc_sys_permission -> net_ctl_permissions, matching what an actual open for
// write checks. Any answer other than nil, EACCES or EROFS (such as ENOENT for
// an absent interface or ENOSYS on a kernel without faccessat2) is reported as
// procSysUnknown so the check says it cannot determine writability instead of
// guessing.
func probeProcSysAccess(path string) (procSysVerdict, error) {
	err := unix.Faccessat2(unix.AT_FDCWD, path, unix.W_OK, unix.AT_EACCESS)
	if err == nil {
		return procSysWritable, nil
	}
	if errors.Is(err, unix.EROFS) {
		return procSysReadOnlyFS, err
	}
	if errors.Is(err, unix.EACCES) {
		return procSysRefused, err
	}
	return procSysUnknown, err
}
