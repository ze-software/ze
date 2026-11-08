// Design: docs/contributing/rfc-conformance-gates.md -- explicit or build-required tagged-unit guest placement
// Overview: discriminate_observe.go -- the runner this placement extends
// Related: ../test/qemu/dockerlab_command.go -- the own-system unit command carrier
//
// discriminate_guest.go places ONE tagged Go unit in an explicitly selected
// compatible guest, or otherwise where its build constraints hold, host first.
// A test that touches the kernel may carry `integration && linux`
// (ai/rules/platform-linux.md), so a host run cannot always compile it. Such a
// unit is compiled for the QEMU guest on the host, where the overlay still
// reaches the compiler, and run there by name through `le test qemu docker-lab`.
//
// A .ci the host's runner skips (`option=needs-linux:caps=...` on a host that
// lacks the capability) runs in the same own system through `le test qemu all
// test <path>`, the one-.ci narrowing `le feature run` drives, against daemons
// cross-built on the host under the overlay.
package rfc

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// guestGOOS is the operating system the QEMU guest runs.
const guestGOOS = "linux"

// exabgpCarrierName is the carrier of the ExaBGP compatibility files, whose
// runner is not the functional runner and whose files carry no needs-linux.
const exabgpCarrierName = "functional-exabgp"

// guestDaemonsRel is where a guest .ci observation cross-builds its daemons,
// inside the checkout because the harness binds it at GuestWorkspace.
const guestDaemonsRel = "tmp/qemu/rfc-discriminate"

// guestCICommandTimeout is what `le test qemu docker-lab` gives the guest command,
// inside carrierRunDeadline, which also bounds the boot and any image build.
const guestCICommandTimeout = "3000s"

// guestLocalDir is where the guest script copies a unit's test binary before
// running it: the guest's own writable /tmp. The ordinary guest owner creates
// the copy and counter directory; neither is executed from the shared checkout.
const guestLocalDir = "/tmp"

// guestRootDirective is the line a Go test's doc comment carries when the test
// can run only as root in the guest: loading a cgroup BPF program, or anything
// else no user namespace grants. It is a Go directive (`//tool:name`, no space),
// so godoc hides it and gofmt keeps it in the comment. The unit declares it,
// because the unit is what knows; no list here names a unit.
const guestRootDirective = "//ze:guest-root"

// unitDeclaresGuestRoot answers whether the test function unit in file carries
// guestRootDirective in its doc comment.
//
// A function the file does not declare is refused rather than answered false:
// the answer would then decide an unprivileged run for a unit nobody read.
func unitDeclaresGuestRoot(file, unit string) (bool, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ParseComments)
	if err != nil {
		var tb textbuf.Buffer
		return false, parseErr(tb.Str("cannot read ").Str(file).
			Str(" to learn whether ").Str(unit).Str(" needs root: ").Err(err))
	}
	for _, declaration := range parsed.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction {
			continue
		}
		if function.Recv != nil {
			continue
		}
		if function.Name.Name != unit {
			continue
		}
		return docCarriesGuestRoot(function.Doc), nil
	}
	var tb textbuf.Buffer
	return false, parseErr(tb.Str(file).Str(" declares no function ").Str(unit).
		Str(", so whether it needs root in the guest cannot be read"))
}

// docCarriesGuestRoot answers whether a doc comment holds guestRootDirective as
// one whole line.
func docCarriesGuestRoot(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, line := range doc.List {
		if strings.TrimSpace(line.Text) == guestRootDirective {
			return true
		}
	}
	return false
}

// requireRootInGuest refuses a root-declared unit placed on this host: the host
// run is unprivileged, so the unit would skip and observe nothing.
func requireRootInGuest(unit string, guest, root bool) error {
	if !root {
		return nil
	}
	if guest {
		return nil
	}
	var tb textbuf.Buffer
	return parseErr(tb.Str(unit).Str(" declares `").Str(guestRootDirective).
		Str("`, so it runs only as root in the Linux guest, and this host run is unprivileged: pass `").
		Str(keyKernel).Str(" <vmlinuz>`, which `ze appliance kernel` builds, to run it there"))
}

// placementContexts answers the two build contexts a tagged unit's file is
// asked of: this host under the tag set the host run passes, and the guest.
//
// The guest adds the integration tag because a kernel-touching test is what it
// exists to run. CGO is off in both, because the toolchain sets CGO_ENABLED=0
// for a test run and guest execution requires no C compiler.
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

// unitNeedsGuest places one tagged Go file under its build constraints.
//
// An explicit kernel selects the guest only when its context compiles the file.
// Without that selection the host comes first. A file neither context compiles
// is refused rather than skipped: a unit nothing ran cannot have gone red.
func unitNeedsGuest(host, guest *build.Context, file string, explicitGuest bool) (bool, error) {
	directory, name := filepath.Split(file)
	if !explicitGuest {
		onHost, err := host.MatchFile(directory, name)
		if err != nil {
			var tb textbuf.Buffer
			return false, parseErr(tb.Str("cannot read the build constraints of ").Str(file).
				Str(": ").Err(err))
		}
		if onHost {
			return false, nil
		}
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
	if explicitGuest {
		return false, parseErr(tb.Str(file).Str(" does not compile in the ").
			Str(guest.GOOS).Byte('/').Str(guest.GOARCH).
			Str(" QEMU guest selected by `kernel`; remove `kernel` to use host-first placement, ").
			Str("or select a unit compatible with the guest"))
	}
	return false, parseErr(tb.Str(file).Str(" compiles neither on this host nor in the ").
		Str(guest.GOOS).Byte('/').Str(guest.GOARCH).Str(" QEMU guest under tags ").
		Quoted(strings.Join(guest.BuildTags, " ")).
		Str(", so nothing can run it and no red can be observed there"))
}

// requireGuestKernel refuses a guest placement with no runtime kernel, and a
// kernel for a unit that runs on this host.
//
// A guest proof MUST boot Ze's runtime kernel, never the Alpine ISO's
// (ai/rules/platform-linux.md). Units and functional cases both use the
// own-system command carrier, which refuses any other kernel.
// A kernel named for a host unit would be read by nothing, and
// an argument read by nothing is a claim the record never tested.
func requireGuestKernel(unit string, guest bool, kernel string) error {
	var tb textbuf.Buffer
	if guest {
		if kernel != "" {
			return nil
		}
		return parseErr(tb.Str(unit).Str(" runs only in the Linux guest, and a guest ").
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
//
// The binary runs from a copy on the guest's own filesystem, never from the
// shared checkout. The checkout arrives over 9p with `security_model=none`, so
// its files keep the host owner's uid and modes, and the checkout root is
// commonly 0700. The own-system carrier mounts the checkout at its absolute
// host path and runs as that same owner's UID. The local copy is owned by this
// ordinary guest user, whose UID a re-executed user namespace maps. This keeps
// the executable and its counters on the guest filesystem while profiles are
// written through the mounted checkout. No privileged SSH session is needed.
//
// A coverage run also names its counter directory with `-test.gocoverdir=`, as
// `go test -coverprofile` does on the host. The binary builds its profile from
// every counter file in that directory, and a unit that re-executes itself
// passes the flag on to its child, so the code the child runs appears in the
// profile. Without it the binary picks a private directory the child never
// learns, and a producer only the child reaches reads as never executed. The
// directory is guest-local for the same ownership reason as the binary.
func (o *observationRunner) guestScript(binary, profile string) string {
	local := singleQuoted(path.Join(guestLocalDir, filepath.Base(binary)))
	counters := singleQuoted(path.Join(guestLocalDir, "cover-"+filepath.Base(binary)))
	var tb textbuf.Buffer
	tb.Str("cp ").Str(singleQuoted(o.guestPath(binary))).Byte(' ').Str(local)
	if profile != "" {
		tb.Str(" && mkdir -p ").Str(counters)
	}
	tb.Str(" && cd ").Str(singleQuoted(o.guestPath(treePath(o.tree, filepath.Dir(o.tag.File))))).
		Str(" && exec ").Str(local).
		Str(" -test.run '^").Str(o.unitName).Str("$' -test.count=1 -test.v -test.timeout ").
		Str(o.toolchain.Timeout)
	if profile != "" {
		tb.Str(" -test.coverprofile ").Str(singleQuoted(o.guestPath(profile))).
			Str(" -test.gocoverdir=").Str(counters)
	}
	return tb.Byte('\n').String()
}

// guestRunArgv answers the command that boots the guest on the runtime kernel
// and runs one script there. It re-executes this le binary on the qemu action,
// as carrierArgv does for interop, so the guest lifecycle has one owner.
//
// A unit whose test declares guestRootDirective runs under docker-lab's root
// switch, which elevates the command and its interrupt and does nothing else.
// Every other unit keeps the ordinary owner's UID, which a re-executed user
// namespace maps and which a test that means something only unprivileged needs.
func (o *observationRunner) guestRunArgv(script string) []string {
	argv := make([]string, 0, 9)
	argv = append(argv, o.self, leTestCommand, "qemu", "docker-lab", keyKernel, o.kernel)
	if o.root {
		argv = append(argv, testqemu.KeywordRoot)
	}
	var tb textbuf.Buffer
	return append(argv, "command", tb.Str("sh ").Str(singleQuoted(o.guestPath(script))).String())
}

// guestPath preserves a unit input's absolute main-checkout path. The own-system
// carrier mounts checkout, scratch and cache at their host paths.
func (o *observationRunner) guestPath(host string) string {
	return filepath.ToSlash(host)
}

// singleQuoted quotes one value for the guest's shell.
func singleQuoted(value string) string {
	var tb textbuf.Buffer
	return tb.Byte('\'').Str(strings.ReplaceAll(value, "'", `'\''`)).Byte('\'').String()
}

// functionalNeedsGuest answers whether one .ci runs only in the guest. The
// answer is the functional runner's own (needs is testrunner.NeedsGuest): a
// file marked needs-linux whose capability this host lacks is skipped here,
// and a skip observes nothing. A file this host runs keeps the host run.
func functionalNeedsGuest(carrier Carrier, file string, needs func(string) (bool, error)) (bool, error) {
	if carrier.Name == exabgpCarrierName {
		return false, nil
	}
	guest, err := needs(file)
	if err != nil {
		var tb textbuf.Buffer
		return false, parseErr(tb.Str("cannot read the options of ").Str(file).
			Str(" to decide where it runs: ").Err(err))
	}
	return guest, nil
}

// runFunctionalInGuest runs ONE .ci the host skips inside the guest, on the
// runtime kernel.
//
// The daemons are cross-built on the host, with the overlay in GOFLAGS for the
// broken run, into a directory of this process's own: a concurrent recorder
// must never boot this run's broken ze. The guest command then runs under the
// restored environment, so the guest le `docker-lab harness` rebuilds into the
// shared tmp/qemu path never carries the break.
func (o *observationRunner) runFunctionalInGuest(overlay string) (bool, string, error) {
	goarch := testqemu.GuestArch()
	var tb textbuf.Buffer
	root := filepath.Join(guestDaemonsRel, tb.Str("pid-").Int(int64(os.Getpid())).String())
	binaries := filepath.Join(root, "linux-"+goarch, "bin")

	defer os.RemoveAll(filepath.Join(o.tree, root)) //nolint:errcheck // scratch; a leftover costs disk, never an answer
	if err := o.buildGuestDaemons(filepath.Join(o.tree, binaries), goarch, overlay); err != nil {
		return false, "", err
	}
	return o.exec(carrierRunDeadline, o.guestCIArgv(binaries, goarch), os.Environ(), o.tree)
}

// buildGuestDaemons cross-builds the guest ze and ze-stripped, under the
// overlay when one is given, and restores GOFLAGS before it returns.
func (o *observationRunner) buildGuestDaemons(binaries, goarch, overlay string) error {
	if overlay != "" {
		restore, err := setGoFlagsOverlay(overlay)
		if err != nil {
			return err
		}
		defer restore()
	}
	if err := testfunctional.PrepareGuest(o.toolchain, binaries, goarch); err != nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str("cannot cross-build the guest ze for linux/").Str(goarch).
			Str(": ").Err(err))
	}
	return nil
}

// guestCIArgv answers the command that boots Ze's own system on the runtime kernel
// and runs the one .ci there, re-executing this le binary on the qemu action
// as guestRunArgv does for a Go unit. The harness gives `test qemu all` the root,
// the mount and the guest le it requires.
func (o *observationRunner) guestCIArgv(binaries, goarch string) []string {
	return []string{o.self, leTestCommand, "qemu", "docker-lab", keyKernel, o.kernel,
		testqemu.KeywordHarness, "timeout", guestCICommandTimeout,
		"command", testqemu.OneCICommand(binaries, filepath.ToSlash(o.tag.File), goarch)}
}
