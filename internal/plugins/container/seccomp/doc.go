// Design: docs/architecture/container/runtime.md -- the filter seam

// Package seccomp is the system-call filter of a container: Go code in Ze's
// own tree, where it is linted and tested, behind the registration seam the
// patched runtime library exposes. The library's init calls the registered
// function at its own point, immediately before it executes the entrypoint.
//
// The package registers the seam. The filter builder, its system-call tables
// and the default profile are not built: a profile is refused until they are,
// as the library refuses one when nothing is registered.
package seccomp
