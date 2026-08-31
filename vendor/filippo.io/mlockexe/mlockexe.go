// Package mlockexe locks the running executable's file-backed pages in memory,
// so they can't be evicted under memory pressure.
//
// When a process hits a memory cgroup limit or the system runs out of memory,
// the kernel can repeatedly evict and re-fault the executable's own text,
// causing reclaim thrashing. Locking the executable in memory prevents this
// form of thrashing.
//
// Locking requires procfs to be available at /proc and RLIMIT_MEMLOCK to
// accommodate the whole size of the executable, even with [OnFault]. Note that
// the systemd default (LimitMEMLOCK=, usually 8 MiB) is smaller than most Go
// binaries.
package mlockexe

// OnFault locks the mappings of the running executable in memory, preventing
// the eviction of their pages under memory pressure.
//
// Pages that are already resident are locked immediately, and the others are
// locked as they are faulted in, without consuming startup time and memory for
// parts of the executable that are never used.
//
// It returns the total size of the locked mappings, all of which is charged
// against RLIMIT_MEMLOCK, even if unfaulted pages don't consume memory.
//
// It requires mlock2(2), available since Linux 4.4. On platforms other than
// Linux, it returns [errors.ErrUnsupported].
func OnFault() (int64, error) {
	return lock(true)
}

// Now is like [OnFault], but it also immediately faults in all the mapped pages
// of the executable, like mlock(2).
func Now() (int64, error) {
	return lock(false)
}
