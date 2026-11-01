// Design: docs/architecture/testing/qemu-integration.md -- the daemons a guest runs a test against
// Related: alltests.go -- the in-guest run that refuses when one of them is missing
// Related: onetest.go -- the one-.ci guest command that names them
// Related: ../functional/binaries.go -- PrepareGuest, the one list of what a guest needs
//
// guestbin.go answers the host half of a guest test run: it cross-builds the
// daemons `test qemu all` checks for, through the producer that lists them
// (testfunctional.PrepareGuest), and names them in the environment the guest
// command carries. A caller that builds them by hand keeps a second list, and
// the nightly's did: it built ze and ze-stripped only, so every needs-linux run
// refused at `bin/ze-stripped-flow is missing or not executable` once
// ze-stripped-flow joined the set.

package testqemu

import (
	"errors"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// guestBinariesVerb is the host verb that cross-builds the guest daemons.
const guestBinariesVerb = "guest-binaries"

// GuestBinariesRel answers where `le test qemu guest-binaries` writes the
// guest daemons for goarch, relative to the checkout the guest mounts at
// GuestWorkspace. It sits beside the guest le (GuestLeRel), under tmp/.
func GuestBinariesRel(goarch string) string {
	return filepath.Join("tmp", "qemu", "linux-"+goarch, "bin")
}

// GuestBinaryEnvironment answers the assignments that point `test qemu all`
// at the three daemons PrepareGuest writes into binaries, a checkout-relative
// directory: ZE_BIN, ZE_STRIPPED_BIN and ZE_STRIPPED_FLOW_BIN, space separated.
// ze-fakefilter needs no variable: the shim links it from beside ZE_BIN.
func GuestBinaryEnvironment(binaries string) string {
	var env textbuf.Buffer
	env.Str(zeBinKey).Byte('=').Str(filepath.ToSlash(filepath.Join(binaries, zeName))).
		Byte(' ').Str(strippedBinKey).Byte('=').Str(filepath.ToSlash(filepath.Join(binaries, zeStrippedName))).
		Byte(' ').Str(strippedFlowBinKey).Byte('=').Str(filepath.ToSlash(filepath.Join(binaries, zeStrippedFlowName)))
	return env.String()
}

// GuestBinariesReport is what `le test qemu guest-binaries` answers: the
// directory it built into and the environment a guest command passes.
type GuestBinariesReport struct {
	Arch        string `json:"arch"`
	Directory   string `json:"directory"`
	Environment string `json:"environment"`
}

// runGuestBinariesHere cross-builds the guest daemons for GuestArch into
// GuestBinariesRel of this checkout.
func runGuestBinariesHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	goarch := GuestArch()
	rel := GuestBinariesRel(goarch)
	if err := testfunctional.PrepareGuest(toolchain, filepath.Join(root, rel), goarch); err != nil {
		var tb textbuf.Buffer
		leaction.ReportError(errors.New(tb.Str("cannot cross-build the guest daemons for linux/").Str(goarch).
			Str(" into ").Str(rel).Str(": ").Err(err).String()))
		return nil, 1
	}
	return GuestBinariesReport{Arch: goarch, Directory: filepath.ToSlash(rel), Environment: GuestBinaryEnvironment(rel)}, 0
}
