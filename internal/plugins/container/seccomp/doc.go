// Design: docs/architecture/container/runtime.md -- the filter seam

// Package seccomp is the system-call filter of a container: Go code in Ze's
// own tree, where it is linted and tested, behind the registration seam the
// patched runtime library exposes. The library's init calls the registered
// function at its own point, immediately before it executes the entrypoint.
//
// The package holds the default profile (profile.go), derived from Moby's
// default profile, the call numbers of the two shipped architectures
// (table.go), the builder of the classic BPF program that enforces a
// profile (filter.go), and its load into the container's init
// (load_linux.go). Everything but this description builds on Linux only.
package seccomp
