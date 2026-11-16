// Design: docs/architecture/container/runtime.md -- the filter seam
// Related: filter.go -- the program built from a profile
// Related: load_linux.go -- the load of that program

package seccomp

import (
	"github.com/opencontainers/runc/libcontainer/configs"
	libseccomp "github.com/opencontainers/runc/libcontainer/seccomp"
)

// noNotifyDescriptor is what the library reads as "this profile needs no
// seccomp notify descriptor".
const noNotifyDescriptor = -1

// installFilter builds the filter of a profile and loads it into the calling
// process, and answers the seccomp notify descriptor or noNotifyDescriptor.
// It is a variable so the seam test replaces it: a loaded filter changes the
// process for good, which a test process must not do to itself.
var installFilter = buildAndLoad

//nolint:gochecknoinits // registration: the library calls what this package registers, and never imports it
func init() {
	// The library panics on a second registration, so nothing replaces this
	// loader after start-up.
	libseccomp.RegisterInit(initFilter)
}

// initFilter is the function the library's InitSeccomp calls for a profile.
// It runs in the container's init, after the root and the capabilities are
// set and before the entrypoint is executed.
func initFilter(profile *configs.Seccomp) (int, error) {
	return installFilter(profile)
}

// buildAndLoad builds the filter of profile for the native architecture and
// loads it. A refusal at any step starts nothing: the library's init fails,
// and the container is not run less confined than its description. A profile
// never needs a notify descriptor, because the filter refuses the notify
// action.
func buildAndLoad(profile *configs.Seccomp) (int, error) {
	table, err := nativeTable()
	if err != nil {
		return noNotifyDescriptor, err
	}
	program, err := assembleFilter(profile, table)
	if err != nil {
		return noNotifyDescriptor, err
	}
	if err := loadFilter(program); err != nil {
		return noNotifyDescriptor, err
	}
	return noNotifyDescriptor, nil
}
