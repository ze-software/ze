// Design: docs/architecture/testing/qemu-integration.md -- one capability-gated .ci in the guest
// Overview: alltests.go -- the in-guest `test qemu all test <path>` narrowing this command drives
// Related: guestbin.go -- the cross-built daemons the guest command names
// Related: ../../../test/runner/record_parse_option.go -- the needs-linux skip that names `test qemu ci`
//
// onetest.go answers how a host process runs ONE .ci inside Ze's own guest: the
// command `le test qemu docker-lab harness` executes there. Every caller that
// runs a .ci the host skips (`le test qemu ci`, `le feature run`, `le rfc
// record`) reads it from here, so the guest route has one declaration.
package testqemu

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// oneCIVerb is the host verb that runs one .ci in Ze's guest. The functional
// runner names it in the reason it skips a needs-linux test for (runner.GuestRoute),
// and that package cannot import this one, so TestRunnerGuestRouteIsTheOneCIAction
// parses the runner's line through this action's own parser.
const oneCIVerb = "ci"

// oneCICommandTimeout is what `le test qemu ci` gives the guest command: the
// shim, the namespace preparation and the one test, after the boot.
const oneCICommandTimeout = "3000s"

// OneCICommand answers the guest command that runs the .ci at file, a
// repository path, through the suite `test qemu all` gives its directory.
//
// binaries is the checkout-relative directory holding the cross-built `ze`,
// `ze-stripped`, `ze-stripped-flow` and `ze-upgrade` for goarch; the test harness is the guest le that `docker-lab
// harness` builds at GuestLeRel. Both are relative because that branch runs the
// command from the checkout it binds at GuestWorkspace.
func OneCICommand(binaries, file, goarch string) string {
	var command textbuf.Buffer
	command.Str(GuestBinaryEnvironment(binaries)).
		Byte(' ').Str(filepath.ToSlash(GuestLeRel(goarch))).
		Str(" test qemu all ").Str(testKeyword).Byte(' ').Str(file)
	return command.String()
}

// runOneCIHere is `le test qemu ci test <path>` over the checkout it was run
// in: the host route for a .ci this host skips. It refuses a path no guest
// suite runs before it builds anything, then cross-builds the guest daemons
// (`guest-binaries`), resolves the runtime kernel the image boots, and boots
// the own-system carrier with `harness`, which runs OneCICommand in the guest.
func runOneCIHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	file := filepath.ToSlash(args.One(testKeyword))
	if err := oneCIAdmits(root, file); err != nil {
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
			Str(" into ").Str(filepath.ToSlash(rel)).Str(", so the guest has no ze to test: ").Err(err).String()))
		return nil, 1
	}
	kernel, err := GuestRuntimeKernel(root, goarch)
	if err != nil {
		leaction.ReportError(errors.New("cannot resolve the runtime kernel Ze's guest boots: " + err.Error()))
		return nil, 1
	}
	ctx, cancel := guestContext()
	defer cancel()

	return runDockerLab(ctx, root, oneCIArguments(kernel, file, goarch), "")
}

// oneCIAdmits refuses a file the guest cannot run before any build or boot:
// a path that names no test/<dir>/<name>.ci or draft that a guest suite walks
// (vmSuiteFor, the same selection the guest applies), or a file the checkout
// does not hold.
func oneCIAdmits(root, file string) error {
	if _, err := vmSuiteFor(file); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("qemu: ci cannot run ").Str(file).Str(" in the guest: ").Err(err).
			Str("\nname one test under test/<dir>/ or test/draft/<dir>/:\n./le test qemu ci test test/<dir>/<name>.ci").String())
	}
	if _, err := os.Stat(filepath.Join(root, file)); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("qemu: ci cannot run ").Str(file).Str(": the checkout holds no such file, so the guest would boot for nothing: ").
			Err(err).Str("\nname a .ci this checkout holds:\n./le test qemu ci test test/<dir>/<name>.ci").String())
	}
	return nil
}

// oneCIArguments answers the docker-lab invocation that runs file in the guest:
// the own-system command branch with harness, the runtime kernel, and the
// guest command that names the daemons `guest-binaries` built for goarch.
func oneCIArguments(kernel, file, goarch string) leaction.Arguments {
	return leaction.Arguments{
		keywordKernel:  {kernel},
		KeywordHarness: nil,
		keywordTimeout: {oneCICommandTimeout},
		keywordCommand: {OneCICommand(filepath.ToSlash(GuestBinariesRel(goarch)), file, goarch)},
	}
}
