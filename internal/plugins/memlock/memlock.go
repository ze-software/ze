// Design: docs/architecture/plugin/plugin-system.md -- plugin registration
// Detail: memlock_linux.go -- the init() that takes the lock
// Detail: register.go -- registration, the engine run, and the doctor check
//
// Package memlock locks the running executable's own file-backed pages in
// memory, so the kernel cannot evict them under memory pressure.
//
// A router that is short of memory reclaims page cache, and the text of the
// running daemon IS page cache. The kernel evicts a page of ze and faults it
// back in on the next instruction that needs it, which is reclaim thrashing:
// the daemon waits on the disk that holds its own binary, at the moment it
// must keep sessions alive. Locking the executable removes that failure mode.
//
// Every ze process that links this package takes the lock, in init(), before
// main(). The daemon is the process the feature exists for, and a one-shot CLI
// run pays only for reading /proc/self/maps and a few mlock2(2) calls, because
// MLOCK_ONFAULT faults in no page the process does not execute.
//
// Remove this plugin and nothing locks: the daemon keeps running and every
// page of it stays evictable.
package memlock
