// Design: docs/architecture/testing/qemu-integration.md -- one capability-gated .ci in the guest
// Overview: alltests.go -- the in-guest `test qemu all test <path>` narrowing this command drives
//
// onetest.go answers how a host process runs ONE .ci inside Ze's own guest: the
// command `le test qemu docker-lab harness` executes there. Every caller that
// runs a .ci the host skips (`le feature run`, `le rfc record`) reads it from
// here, so the guest route has one declaration.
package testqemu

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// OneCICommand answers the guest command that runs the .ci at file, a
// repository path, through the suite `test qemu all` gives its directory.
//
// binaries is the checkout-relative directory holding the cross-built `ze`,
// `ze-stripped` and `ze-stripped-flow` for goarch; the test harness is the guest le that `docker-lab
// harness` builds at GuestLeRel. Both are relative because that branch runs the
// command from the checkout it binds at GuestWorkspace.
func OneCICommand(binaries, file, goarch string) string {
	var command textbuf.Buffer
	command.Str(GuestBinaryEnvironment(binaries)).
		Byte(' ').Str(filepath.ToSlash(GuestLeRel(goarch))).
		Str(" test qemu all ").Str(testKeyword).Byte(' ').Str(file)
	return command.String()
}
