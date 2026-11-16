// Design: docs/architecture/container/runtime.md -- the container init role, selected before command dispatch
// Related: initrole_linux.go -- runLibraryInit, the call into the runtime library
// Related: register.go -- the plugin registration

package container

import (
	"github.com/ze-software/ze/internal/core/earlyrole"

	// The filter package registers the loader the library's init calls for a
	// system-call profile. The init role is the process that loads a filter,
	// so the role links the package that builds it.
	_ "github.com/ze-software/ze/internal/plugins/container/seccomp"
)

// libraryInitKey is the environment variable the runtime library sets on the
// process it starts as a container's init: the descriptor of the pipe the
// init configuration arrives on. No other caller of ze sets it. It belongs to
// the library's protocol, never to Ze's settings, so it is no registered Ze
// environment key.
const libraryInitKey = "_LIBCONTAINER_INITPIPE"

// libraryInitExitCode is what the role answers if the library's init ever
// returns. The library exits the process itself, with the same status, when
// its init fails.
const libraryInitExitCode = 255

//nolint:gochecknoinits // registration: the always-on early-role leaf names no feature, so the role's owner registers it
func init() {
	earlyrole.Register(libraryInitKey, runInitRole)
}

// runInitRole is the whole run of a process the library started as a
// container's init. The library's init replaces the process with the
// container's entrypoint or exits, so the return below is not reached.
func runInitRole() int {
	runLibraryInit()
	return libraryInitExitCode
}
