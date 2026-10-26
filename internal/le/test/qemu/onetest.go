// Design: docs/architecture/testing/qemu-integration.md -- one capability-gated .ci in the guest
// Overview: alltests.go -- the in-guest `all-tests test <path>` narrowing this command drives
//
// onetest.go answers how a host process runs ONE .ci inside the guest: the
// packages it installs and the command the guest executes. Every caller that
// runs a .ci the host skips (`le feature record-run`, `le rfc
// record`) reads both from here, so the guest route has one
// declaration.
package testqemu

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// OneCIPackages are what a capability-gated test shells out to in the guest
// beyond its base image: `ip`, `nft`, `setcap` for the per-test namespace
// suites, `ping`, and the module loader (docs/architecture/testing/qemu-integration.md).
const OneCIPackages = "coreutils nftables iproute2 iputils-ping kmod iptables libcap"

// OneCICommand answers the guest command that runs the .ci at file, a
// repository path, through the suite `all-tests` gives its directory.
//
// binaries is the checkout-relative directory holding the cross-built `ze`,
// `ze-stripped` and `ze-stripped-flow` for goarch; the harness is the guest le that `le test qemu run`
// builds at GuestLeRel. Both are relative because the guest runs the command
// from the checkout it mounts at GuestWorkspace.
func OneCICommand(binaries, file, goarch string) string {
	var command textbuf.Buffer
	command.Str("ZE_BIN=").Str(filepath.ToSlash(filepath.Join(binaries, zeName))).
		Str(" ZE_STRIPPED_BIN=").Str(filepath.ToSlash(filepath.Join(binaries, zeStrippedName))).
		Byte(' ').Str(strippedFlowBinKey).Byte('=').Str(filepath.ToSlash(filepath.Join(binaries, zeStrippedFlowName))).
		Byte(' ').Str(filepath.ToSlash(GuestLeRel(goarch))).
		Str(" test qemu all-tests ").Str(testKeyword).Byte(' ').Str(file)
	return command.String()
}
