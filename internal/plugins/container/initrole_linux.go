// Design: docs/architecture/container/runtime.md -- the container init role

package container

import "github.com/opencontainers/runc/libcontainer"

// runLibraryInit runs the upstream runtime's init sequence in this process.
// It never returns: on success the library replaces the process with the
// container's entrypoint, and on failure it reports the error to its parent
// over the inherited sync socket and exits with status 255.
//
// It MUST be called only in a process the patched library started as a
// container's init, which is the process that carries the library's
// environment key, and before anything else of ze starts: the library reads
// its descriptors from the environment, locks the thread and then clears the
// environment.
func runLibraryInit() {
	libcontainer.Init()
}
