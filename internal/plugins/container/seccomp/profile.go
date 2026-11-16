// Design: docs/architecture/container/runtime.md -- the system-call filter
// Related: filter.go -- the program built from the profile
// Related: table.go -- the names a profile may use

//go:build linux

package seccomp

import (
	"runtime"
	"slices"

	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

// DefaultProfile is the system-call profile of a container whose bounding
// capability set is bounding, on the architecture this binary runs on: every
// call the source profile allows, the calls of a capability the set holds,
// and EPERM for every other call. It MUST be given the container's final
// bounding set: a capability rule is selected by it.
//
// The rules are those of Moby's default profile (github.com/moby/profiles,
// seccomp/default.json; source, commit and digest in
// third_party/moby-seccomp-profile/README.md), selected as the source selects
// them, with three differences. A name neither table holds is left out: the
// 32-bit calls (chown32, fcntl64, the *_time64 calls and the rest), umount
// and stime. The rules for an architecture Ze ships no table for (ppc64le,
// arm, s390, riscv64) are left out. And the rule the source applies from
// kernel 4.8 (ptrace and process_vm_readv/writev) is applied always: a
// container starts only through a clone into its control group, which needs
// kernel 5.7.
func DefaultProfile(bounding []string) *configs.Seccomp {
	return defaultProfileFor(runtime.GOARCH, bounding)
}

// defaultProfileFor is DefaultProfile for the architecture goarch.
func defaultProfileFor(goarch string, bounding []string) *configs.Seccomp {
	errnoDefault := uint(unix.EPERM)
	profile := &configs.Seccomp{
		DefaultAction:   configs.Errno,
		DefaultErrnoRet: &errnoDefault,
	}
	for _, rule := range sourceRules {
		if !rule.selected(goarch, bounding) {
			continue
		}
		for _, name := range rule.names {
			profile.Syscalls = append(profile.Syscalls, &configs.Syscall{
				Name:     name,
				Action:   rule.action,
				ErrnoRet: rule.errnoRet,
				Args:     rule.args,
			})
		}
	}
	return profile
}

// sourceRule is one rule of the source profile and the condition that
// selects it. An empty condition field is the guard "no such condition": the
// source rule names none.
type sourceRule struct {
	names  []string
	action configs.Action
	// errnoRet is the error number of an Errno rule; nil answers EPERM.
	errnoRet *uint
	args     []*configs.Arg
	// arch selects the rule only on this GOARCH ("" for every one).
	arch string
	// capability selects the rule only when the bounding set holds it.
	capability string
	// withoutCapability selects the rule only when the bounding set lacks it.
	withoutCapability string
}

// selected tells whether the rule applies to a container of the bounding set
// on goarch, as the source profile's includes and excludes decide.
func (rule sourceRule) selected(goarch string, bounding []string) bool {
	if rule.arch != "" {
		if rule.arch != goarch {
			return false
		}
	}
	if rule.capability != "" {
		if !slices.Contains(bounding, rule.capability) {
			return false
		}
	}
	if rule.withoutCapability != "" {
		if slices.Contains(bounding, rule.withoutCapability) {
			return false
		}
	}
	return true
}

// allow is a rule that allows names unconditionally.
func allow(names ...string) sourceRule {
	return sourceRule{names: names, action: configs.Allow}
}

// allowWith is a rule that allows names when the bounding set holds
// capability.
func allowWith(capability string, names ...string) sourceRule {
	return sourceRule{names: names, action: configs.Allow, capability: capability}
}

// allowFirstArgument is a rule that allows name when its first argument
// compares to value under op: every argument rule of the source Ze keeps
// reads the first argument (the s390 clone rule, which reads the second, is left out).
func allowFirstArgument(name string, op configs.Operator, value uint64) sourceRule {
	return sourceRule{
		names:  []string{name},
		action: configs.Allow,
		args:   []*configs.Arg{{Index: 0, Value: value, Op: op}},
	}
}

// The calls the clone rules name besides the capability lists.
const (
	callClone  = "clone"
	callClone3 = "clone3"
)

// errnoNotImplemented is the error number of the clone3 rule: a C library
// that reads it falls back to clone, which the filter checks.
var errnoNotImplemented = uint(unix.ENOSYS)

// cloneNamespaceFlags is the source profile's mask over clone's flags: every
// CLONE_NEW* flag. Without CAP_SYS_ADMIN, clone is allowed only when none is
// set.
const cloneNamespaceFlags = 2114060288

// sourceRules are the rules of the source profile, in its order: the first
// rule that matches a call decides it.
var sourceRules = buildSourceRules()

// buildSourceRules writes the source profile's rules. The socket, socketpair
// and personality rules list the argument values the source allows.
func buildSourceRules() []sourceRule {
	rules := []sourceRule{
		allow(
			"accept", "accept4", "access", "adjtimex", "alarm", "bind", "brk", "cachestat",
			"capget", "capset", "chdir", "chmod", "chown", "clock_adjtime", "clock_getres",
			"clock_gettime", "clock_nanosleep", "close", "close_range", "connect",
			"copy_file_range", "creat", "dup", "dup2", "dup3", "epoll_create", "epoll_create1",
			"epoll_ctl", "epoll_ctl_old", "epoll_pwait", "epoll_pwait2", "epoll_wait",
			"epoll_wait_old", "eventfd", "eventfd2", "execve", "execveat", "exit", "exit_group",
			"faccessat", "faccessat2", "fadvise64", "fallocate", "fanotify_mark", "fchdir",
			"fchmod", "fchmodat", "fchmodat2", "fchown", "fchownat", "fcntl", "fdatasync",
			"fgetxattr", "flistxattr", "flock", "fork", "fremovexattr", "fsetxattr", "fstat",
			"fstatfs", "fsync", "ftruncate", "futex", "futex_requeue", "futex_wait",
			"futex_waitv", "futex_wake", "futimesat", "getcpu", "getcwd", "getdents",
			"getdents64", "getegid", "geteuid", "getgid", "getgroups", "getitimer",
			"getpeername", "getpgid", "getpgrp", "getpid", "getppid", "getpriority",
			"getrandom", "getresgid", "getresuid", "getrlimit", "get_robust_list", "getrusage",
			"getsid", "getsockname", "getsockopt", "get_thread_area", "gettid", "gettimeofday",
			"getuid", "getxattr", "getxattrat", "inotify_add_watch", "inotify_init",
			"inotify_init1", "inotify_rm_watch", "io_cancel", "ioctl", "io_destroy",
			"io_getevents", "io_pgetevents", "ioprio_get", "ioprio_set", "io_setup",
			"io_submit", "kill", "landlock_add_rule", "landlock_create_ruleset",
			"landlock_restrict_self", "lchown", "lgetxattr", "link", "linkat", "listen",
			"listmount", "listxattr", "listxattrat", "llistxattr", "lremovexattr", "lseek",
			"lsetxattr", "lstat", "madvise", "map_shadow_stack", "membarrier", "memfd_create",
			"memfd_secret", "mincore", "mkdir", "mkdirat", "mknod", "mknodat", "mlock",
			"mlock2", "mlockall", "mmap", "mprotect", "mq_getsetattr", "mq_notify", "mq_open",
			"mq_timedreceive", "mq_timedsend", "mq_unlink", "mremap", "mseal", "msgctl",
			"msgget", "msgrcv", "msgsnd", "msync", "munlock", "munlockall", "munmap",
			"name_to_handle_at", "nanosleep", "newfstatat", "open", "openat", "openat2",
			"pause", "pidfd_open", "pidfd_send_signal", "pipe", "pipe2", "pkey_alloc",
			"pkey_free", "pkey_mprotect", "poll", "ppoll", "prctl", "pread64", "preadv",
			"preadv2", "prlimit64", "process_mrelease", "pselect6", "pwrite64", "pwritev",
			"pwritev2", "read", "readahead", "readlink", "readlinkat", "readv", "recvfrom",
			"recvmmsg", "recvmsg", "remap_file_pages", "removexattr", "removexattrat",
			"rename", "renameat", "renameat2", "restart_syscall", "rmdir", "rseq",
			"rt_sigaction", "rt_sigpending", "rt_sigprocmask", "rt_sigqueueinfo",
			"rt_sigreturn", "rt_sigsuspend", "rt_sigtimedwait", "rt_tgsigqueueinfo",
			"sched_getaffinity", "sched_getattr", "sched_getparam", "sched_get_priority_max",
			"sched_get_priority_min", "sched_getscheduler", "sched_rr_get_interval",
			"sched_setaffinity", "sched_setattr", "sched_setparam", "sched_setscheduler",
			"sched_yield", "seccomp", "select", "semctl", "semget", "semop", "semtimedop",
			"sendfile", "sendmmsg", "sendmsg", "sendto", "setfsgid", "setfsuid", "setgid",
			"setgroups", "setitimer", "setpgid", "setpriority", "setregid", "setresgid",
			"setresuid", "setreuid", "setrlimit", "set_robust_list", "setsid", "setsockopt",
			"set_thread_area", "set_tid_address", "setuid", "setxattr", "setxattrat", "shmat",
			"shmctl", "shmdt", "shmget", "shutdown", "sigaltstack", "signalfd", "signalfd4",
			"splice", "stat", "statfs", "statmount", "statx", "symlink", "symlinkat", "sync",
			"sync_file_range", "syncfs", "sysinfo", "tee", "tgkill", "time", "timer_create",
			"timer_delete", "timer_getoverrun", "timer_gettime", "timer_settime",
			"timerfd_create", "timerfd_gettime", "timerfd_settime", "times", "tkill",
			"truncate", "umask", "uname", "unlink", "unlinkat", "uretprobe", "utime",
			"utimensat", "utimes", "vfork", "vmsplice", "wait4", "waitid", "write", "writev",
		),
		// The source applies this rule from kernel 4.8; Ze applies it always.
		allow("process_vm_readv", "process_vm_writev", "ptrace"),
		allowFirstArgument("socket", configs.LessThan, 3),
	}
	// The address families socket is allowed for, beyond the first three.
	for _, family := range []uint64{7, 8, 9, 10, 13, 14, 15, 16, 17, 18, 21, 22, 24, 26, 27, 28, 29, 30, 31, 32, 33, 35, 36, 39, 41, 42, 43, 44, 45} {
		rules = append(rules, allowFirstArgument("socket", configs.EqualTo, family))
	}
	for _, family := range []uint64{1, 30} {
		rules = append(rules, allowFirstArgument("socketpair", configs.EqualTo, family))
	}
	for _, persona := range []uint64{0, 8, 131072, 131080, 4294967295} {
		rules = append(rules, allowFirstArgument("personality", configs.EqualTo, persona))
	}
	return append(rules,
		sourceRule{names: []string{"arch_prctl"}, action: configs.Allow, arch: goarchAMD64},
		sourceRule{names: []string{"modify_ldt"}, action: configs.Allow, arch: goarchAMD64},
		allowWith("CAP_DAC_READ_SEARCH", "open_by_handle_at"),
		allowWith("CAP_SYS_ADMIN",
			"bpf", callClone, callClone3, "fanotify_init", "fsconfig", "fsmount", "fsopen",
			"fspick", "lookup_dcookie", "lsm_get_self_attr", "lsm_list_modules",
			"lsm_set_self_attr", "mount", "mount_setattr", "move_mount", "open_tree",
			"perf_event_open", "quotactl", "quotactl_fd", "setdomainname", "sethostname",
			"setns", "syslog", "umount2", "unshare",
		),
		sourceRule{
			names:             []string{callClone},
			action:            configs.Allow,
			args:              []*configs.Arg{{Index: 0, Value: cloneNamespaceFlags, ValueTwo: 0, Op: configs.MaskEqualTo}},
			withoutCapability: "CAP_SYS_ADMIN",
		},
		sourceRule{
			names:             []string{callClone3},
			action:            configs.Errno,
			errnoRet:          &errnoNotImplemented,
			withoutCapability: "CAP_SYS_ADMIN",
		},
		allowWith("CAP_SYS_BOOT", "reboot"),
		allowWith("CAP_SYS_CHROOT", "chroot"),
		allowWith("CAP_SYS_MODULE", "delete_module", "init_module", "finit_module"),
		allowWith("CAP_SYS_PACCT", "acct"),
		allowWith("CAP_SYS_PTRACE", "kcmp", "pidfd_getfd", "process_madvise", "process_vm_readv", "process_vm_writev", "ptrace"),
		allowWith("CAP_SYS_RAWIO", "iopl", "ioperm"),
		allowWith("CAP_SYS_TIME", "settimeofday", "clock_settime"),
		allowWith("CAP_SYS_TTY_CONFIG", "vhangup"),
		allowWith("CAP_SYS_NICE", "get_mempolicy", "mbind", "set_mempolicy", "set_mempolicy_home_node"),
		allowWith("CAP_SYSLOG", "syslog"),
		allowWith("CAP_BPF", "bpf"),
		allowWith("CAP_PERFMON", "perf_event_open"),
	)
}
