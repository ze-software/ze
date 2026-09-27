// Design: docs/contributing/rfc-conformance-gates.md -- a tagged unit that compiles only in the Linux guest
// Overview: discriminate_observe.go -- the runner this placement extends
// Related: ../test/qemu/run.go -- the guest `le test qemu run` boots
//
// discriminate_guest.go places ONE tagged Go unit where its build constraints
// hold. A test that touches the kernel carries `integration && linux`
// (ai/rules/platform-linux.md), so the host `go test` never compiles it, and a
// red that no run could produce was never observable. Such a unit is compiled
// for the QEMU guest on the host, where the overlay still reaches the compiler,
// and run there by name through `le test qemu run`, the guest runner the
// repository already has.
package rfc

import (
	"go/build"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// guestGOOS is the operating system the QEMU guest runs.
const guestGOOS = "linux"

// placementContexts answers the two build contexts a tagged unit's file is
// asked of: this host under the tag set the host run passes, and the guest.
//
// The guest adds the integration tag because a kernel-touching test is what it
// exists to run. CGO is off in both, because the toolchain sets CGO_ENABLED=0
// for a test run and the Alpine guest ships no C compiler.
func placementContexts(hostTags []string) (host, guest build.Context) {
	host = build.Default
	host.CgoEnabled = false
	host.BuildTags = hostTags
	guest = host
	guest.GOOS = guestGOOS
	guest.GOARCH = testqemu.GuestArch()
	guest.BuildTags = append(append(make([]string, 0, len(hostTags)+1), hostTags...), testqemu.IntegrationTag)
	return host, guest
}

// unitNeedsGuest answers whether one tagged Go file compiles only in the guest.
//
// go/build reads the constraints the way the go command does: the `//go:build`
// line and the _GOOS or _GOARCH file-name suffix. The host is asked first, so a
// unit that runs here keeps the host run. A file neither context compiles has
// nowhere to run, and that is an error rather than a skip: a unit nothing ran
// cannot have gone red.
func unitNeedsGuest(host, guest *build.Context, file string) (bool, error) {
	directory, name := filepath.Split(file)
	onHost, err := host.MatchFile(directory, name)
	if err != nil {
		var tb textbuf.Buffer
		return false, parseErr(tb.Str("cannot read the build constraints of ").Str(file).
			Str(": ").Err(err))
	}
	if onHost {
		return false, nil
	}
	inGuest, err := guest.MatchFile(directory, name)
	if err != nil {
		var tb textbuf.Buffer
		return false, parseErr(tb.Str("cannot read the build constraints of ").Str(file).
			Str(": ").Err(err))
	}
	if inGuest {
		return true, nil
	}
	var tb textbuf.Buffer
	return false, parseErr(tb.Str(file).Str(" compiles neither on this host nor in the ").
		Str(guest.GOOS).Byte('/').Str(guest.GOARCH).Str(" QEMU guest under tags ").
		Quoted(strings.Join(guest.BuildTags, " ")).
		Str(", so nothing can run it and no red can be observed there"))
}

// requireGuestKernel refuses a guest placement with no runtime kernel, and a
// kernel for a unit that runs on this host.
//
// A guest proof MUST boot Ze's runtime kernel, never the Alpine ISO's
// (ai/rules/platform-linux.md): `le test qemu run` checks the release only when
// it is given one. A kernel named for a host unit would be read by nothing, and
// an argument read by nothing is a claim the record never tested.
func requireGuestKernel(unit string, guest bool, kernel string) error {
	var tb textbuf.Buffer
	if guest {
		if kernel != "" {
			return nil
		}
		return parseErr(tb.Str(unit).Str(" compiles only in the Linux guest, and a guest ").
			Str("proof MUST boot Ze's runtime kernel: pass `").Str(keyKernel).
			Str(" <vmlinuz>`, which `ze appliance kernel` builds ").
			Str("(docs/architecture/testing/qemu-integration.md)"))
	}
	if kernel == "" {
		return nil
	}
	return parseErr(tb.Str(unit).Str(" runs on this host, so `").Str(keyKernel).
		Str("` names a guest nothing would boot"))
}

// runInGuest compiles the tagged unit's package for the guest on this host and
// runs the one unit inside the guest.
//
// The compile is on the host so the overlay reaches it, exactly as it does for
// a host run, and nothing in the checkout is modified. A compile that fails is
// a red that names no unit, which attribution refuses, the same answer a host
// `go test` gives for a break that does not build.
func (o *observationRunner) runInGuest(overlay, profile string) (bool, string, error) {
	scratch, err := observationScratch(o.tree)
	if err != nil {
		return false, "", err
	}
	// One name per process, as the cover profile has: several agents of one
	// session share this scratch directory.
	var tb textbuf.Buffer
	stem := tb.Str("guest-").Int(int64(os.Getpid())).String()
	binary := filepath.Join(scratch, tb.Reset().Str(stem).Str(".test").String())
	script := filepath.Join(scratch, tb.Reset().Str(stem).Str(".sh").String())

	defer os.Remove(binary) //nolint:errcheck // scratch; a leftover costs disk, never an answer
	defer os.Remove(script) //nolint:errcheck // scratch; a leftover costs disk, never an answer

	environ := o.toolchain.Environment(gotoolchain.EnvOptions{GOOS: guestGOOS, GOARCH: testqemu.GuestArch()})
	built, output, err := o.exec(unitRunDeadline, o.guestBuildArgv(binary, overlay, profile), environ, o.tree)
	if err != nil {
		return false, output, err
	}
	if !built {
		return false, output, nil
	}
	if err := os.WriteFile(script, []byte(o.guestScript(binary, profile)), 0o600); err != nil {
		return false, "", parseErr(tb.Reset().Str("cannot write the guest script: ").Err(err))
	}
	return o.exec(carrierRunDeadline, o.guestRunArgv(script), os.Environ(), o.tree)
}

// guestBuildArgv answers the host command that compiles the unit's package into
// one guest test binary.
func (o *observationRunner) guestBuildArgv(binary, overlay, profile string) []string {
	options := make([]string, 0, 9)
	options = append(options, "-c", "-o", binary)
	if overlay != "" {
		options = append(options, "-overlay", overlay)
	}
	if profile != "" {
		options = append(options, "-cover", "-coverpkg", o.coverPackages())
	}
	var tb textbuf.Buffer
	options = append(options, tb.Str("./").Str(filepath.ToSlash(filepath.Dir(o.tag.File))).String())
	return o.toolchain.GoTest(gotoolchain.TestOptions{Tags: []string{testqemu.IntegrationTag}}, options...)
}

// guestScript answers the shell the guest runs: the binary from the unit's
// package directory, selected to the one unit, with the flags a host run
// passes. `-test.v` is what prints `=== RUN   <Name>`, which killedByTheBreak
// reads. The profile is written through the shared checkout, so the host reads
// it where it asked for it.
func (o *observationRunner) guestScript(binary, profile string) string {
	var tb textbuf.Buffer
	tb.Str("cd ").Str(singleQuoted(o.guestPath(treePath(o.tree, filepath.Dir(o.tag.File))))).
		Str(" && exec ").Str(singleQuoted(o.guestPath(binary))).
		Str(" -test.run '^").Str(o.unitName).Str("$' -test.count=1 -test.v -test.timeout ").
		Str(o.toolchain.Timeout)
	if profile != "" {
		tb.Str(" -test.coverprofile ").Str(singleQuoted(o.guestPath(profile)))
	}
	return tb.Byte('\n').String()
}

// guestRunArgv answers the command that boots the guest on the runtime kernel
// and runs one script there. It re-executes this le binary on the qemu action,
// as carrierArgv does for interop, so the guest lifecycle has one owner.
func (o *observationRunner) guestRunArgv(script string) []string {
	var tb textbuf.Buffer
	return []string{o.self, leTestCommand, "qemu", "run", keyKernel, o.kernel,
		"command", tb.Str("sh ").Str(singleQuoted(o.guestPath(script))).String()}
}

// guestPath answers where the guest sees one path under this checkout, which it
// mounts at testqemu.GuestWorkspace.
func (o *observationRunner) guestPath(host string) string {
	return path.Join(testqemu.GuestWorkspace, filepath.ToSlash(relTo(o.tree, host)))
}

// singleQuoted quotes one value for the guest's shell.
func singleQuoted(value string) string {
	var tb textbuf.Buffer
	return tb.Byte('\'').Str(strings.ReplaceAll(value, "'", `'\''`)).Byte('\'').String()
}
