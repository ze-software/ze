// Design: docs/architecture/container/runtime.md -- the filter seam

//go:build linux

package seccomp

import (
	"errors"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	libseccomp "github.com/opencontainers/runc/libcontainer/seccomp"
)

// recordInstalls replaces installFilter with a function that records the
// profiles it is handed and answers descriptor, and restores the real one
// when the test ends. No filter is loaded into the test process.
func recordInstalls(t *testing.T, descriptor int) *[]*configs.Seccomp {
	t.Helper()
	saved := installFilter
	t.Cleanup(func() { installFilter = saved })
	var installed []*configs.Seccomp
	installFilter = func(profile *configs.Seccomp) (int, error) {
		installed = append(installed, profile)
		return descriptor, nil
	}
	return &installed
}

// TestSeccompSeamIsFilledByThePlugin proves the wiring of the filter: the
// library's InitSeccomp, which its init calls before the entrypoint, reaches
// the function this package registered, with the profile it was given, and
// the answer travels back unchanged.
//
// Method: replace the install step with a recorder and call the library's
// InitSeccomp with a profile.
//
// MUTATION: remove the libseccomp.RegisterInit call from init. InitSeccomp
// then answers the library's own ErrSeccompNotEnabled and the recorder sees
// no profile.
func TestSeccompSeamIsFilledByThePlugin(t *testing.T) {
	const notifyDescriptor = 7
	installed := recordInstalls(t, notifyDescriptor)
	profile := &configs.Seccomp{}

	descriptor, err := libseccomp.InitSeccomp(profile)

	if errors.Is(err, libseccomp.ErrSeccompNotEnabled) {
		t.Fatal("the library answered that no filter is registered: the seam is not filled")
	}
	if err != nil {
		t.Fatalf("InitSeccomp answered %v, want the install step's nil", err)
	}
	if descriptor != notifyDescriptor {
		t.Errorf("InitSeccomp answered descriptor %d, want the install step's %d", descriptor, notifyDescriptor)
	}
	if len(*installed) != 1 {
		t.Fatalf("the install step ran %d times, want 1", len(*installed))
	}
	if (*installed)[0] != profile {
		t.Error("the install step received another profile than the one InitSeccomp was given")
	}
}

// TestSeccompSeamIsNotCalledWithoutAProfile proves the other side: a
// container with no profile loads no filter, and the install step is not
// reached.
func TestSeccompSeamIsNotCalledWithoutAProfile(t *testing.T) {
	installed := recordInstalls(t, 7)

	descriptor, err := libseccomp.InitSeccomp(nil)

	if err != nil {
		t.Fatalf("InitSeccomp with no profile answered %v, want nil", err)
	}
	if descriptor != noNotifyDescriptor {
		t.Errorf("InitSeccomp with no profile answered descriptor %d, want %d", descriptor, noNotifyDescriptor)
	}
	if len(*installed) != 0 {
		t.Fatalf("the install step ran %d times with no profile", len(*installed))
	}
}
