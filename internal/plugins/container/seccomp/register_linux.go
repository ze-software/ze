// Design: docs/architecture/container/runtime.md -- the filter seam

package seccomp

import (
	"errors"

	"github.com/opencontainers/runc/libcontainer/configs"
	libseccomp "github.com/opencontainers/runc/libcontainer/seccomp"
)

// noNotifyDescriptor is what the library reads as "this profile needs no
// seccomp notify descriptor".
const noNotifyDescriptor = -1

// errFilterNotBuilt refuses a profile: this package cannot build a filter
// yet, and a container started without the filter its description asks for
// would run less confined than configured.
var errFilterNotBuilt = errors.New("container system-call filter: ze cannot build a filter yet, so a profile is refused")

// installFilter builds the filter of a profile and loads it into the calling
// process, and answers the seccomp notify descriptor or noNotifyDescriptor.
// It is a variable so the seam test replaces it: a loaded filter changes the
// process for good, which a test process must not do to itself.
var installFilter = refuseProfile

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

// refuseProfile is installFilter until the builder exists.
func refuseProfile(*configs.Seccomp) (int, error) {
	return noNotifyDescriptor, errFilterNotBuilt
}
