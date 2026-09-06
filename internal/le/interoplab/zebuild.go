// Design: docs/architecture/testing/interop.md -- the lab daemon every interop image copies in
// Related: lab.go -- Suite.Preflight, the hook this producer is wired to.
// Related: docker.go -- ServerArchitecture, the daemon question that decides GOARCH.
//
// zebuild.go is the one place an interop lab binary is cross-compiled.
//
// Five labs used to answer that question in four ways. Two Dockerfiles derived
// their tags with an inline awk over feature-gates.txt, one took them as a
// build argument, and two Go copies of one function disagreed about both the
// tags and the environment. That drift is measured rather than feared: the
// untagged-build defect was fixed in test/interop first, in test/interop-l2tp
// on 2026-08-03, and the audit that found the second missed the third
// (test/interop-pppoe/Dockerfile.ze records it in its own comment).
//
// The image copies the result in rather than compiling it. A Go compiler and
// linker running inside the container is what the kernel killed three times on
// 2026-09-06 on an idle 31 GiB workstation, and it is what took 40m39s on a
// small VM on 2026-09-04. The build runs on the host, against the checkout's
// own GOCACHE, so a second run compiles what changed.
package interoplab

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/featuretags"
	"github.com/ze-software/ze/internal/le/gotoolchain"
)

// labBuildTimeout bounds one cross-compile. Five minutes is what the two
// hand-written copies of this producer used, and it covers a cold GOCACHE on
// the slowest machine this repository has been built on.
const labBuildTimeout = 5 * time.Minute

// labBuildPackage is the one Go package every lab binary is built from. The
// personality is a build tag rather than a second main (cmd/ze/main.go).
const labBuildPackage = "./cmd/ze"

// labBuildGOOS is the operating system every lab image runs. Every lab base is
// a Linux image, so no lab declares this.
const labBuildGOOS = "linux"

// LabBinary declares one binary a lab stages into its Docker build context.
//
// A lab declares its binaries beside the images it builds, so adding a lab
// touches that lab alone: this producer holds no list of labs and no per-lab
// branch.
type LabBinary struct {
	// Name is what the binary is called in a message: `ze`, `ze-test`.
	Name string
	// Base is the tag pair this personality carries before the feature gates,
	// such as featuretags.DaemonBase for a daemon and `ze_test` for the test
	// personality. The gates themselves are read from feature-gates.txt, so a
	// lab cannot declare a feature set that differs from the shipped one.
	Base string
	// Output is the staging path, relative to the checkout root. It sits inside
	// the lab's Docker build context, .dockerignore admits it by negation, and
	// .gitignore holds it because every run rewrites it.
	Output string
}

// StageBinaries answers the PreflightCheck that cross-compiles every declared
// binary into this lab's Docker build context.
//
// It is a PreflightCheck rather than a step inside Docker.Build because the
// binaries are an input to the build: Suite.Run calls Preflight before it
// builds the first image, so a cross-compile that fails costs no image build.
//
// noBuild is the suite's own NO_BUILD answer. A lab that reuses the images of a
// previous run reuses its binaries too, which is what makes NO_BUILD skip every
// build rather than only the Docker half.
func StageBinaries(root string, noBuild bool, binaries ...LabBinary) PreflightCheck {
	return func(ctx context.Context, docker *Docker) error {
		if noBuild {
			return nil
		}
		if docker == nil {
			return errors.New("staging a lab binary needs a Docker client to read the daemon architecture from")
		}
		if root == "" {
			return errors.New("staging a lab binary needs the checkout root")
		}
		if len(binaries) == 0 {
			return errors.New("a lab that stages no binary declares none, and an empty declaration is never a build")
		}

		architecture, err := docker.ServerArchitecture(ctx)
		if err != nil {
			return err
		}
		return stageBinaries(ctx, docker.runner, root, architecture, binaries)
	}
}

// stageBinaries cross-compiles each declared binary in declaration order and
// stops at the first failure.
//
// The toolchain comes from gotoolchain rather than from os.Environ, because
// that is what puts GOCACHE inside the checkout and pins GOTOOLCHAIN to the one
// go.mod names. An ambient environment resolves GOCACHE to the machine's
// default, which nothing in this repository manages: `./le scratch cache-clean`
// does not clear it, the full-disk discipline does not measure it, and another
// tool trimming it mid-build fails this cross-compile with a wave of "no such
// file or directory" against Go's own standard library
// (plan/journal/full-disk-false-red.md). Measured on 2026-09-04, twice.
func stageBinaries(ctx context.Context, runner processRunner, root, architecture string, binaries []LabBinary) error {
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		return err
	}
	// CGO stays off, which gotoolchain answers by default. A static Go binary
	// references no libc, which is what lets a binary built on a glibc host run
	// on the musl alpine:3.21 base every lab image uses.
	environment := toolchain.Environment(gotoolchain.EnvOptions{
		GOOS:   labBuildGOOS,
		GOARCH: architecture,
	})

	for _, binary := range binaries {
		if err := stageBinary(ctx, runner, root, environment, binary); err != nil {
			return err
		}
	}
	return nil
}

// stageBinary cross-compiles one declared binary.
func stageBinary(ctx context.Context, runner processRunner, root string, environment []string, binary LabBinary) error {
	if binary.Name == "" || binary.Output == "" || binary.Base == "" {
		var tb textbuf.Buffer
		return errors.New(tb.Str("a lab binary declares a name, a tag base and an output path; this one declares name ").
			Quoted(binary.Name).Str(", base ").Quoted(binary.Base).
			Str(" and output ").Quoted(binary.Output).String())
	}

	tags, err := featuretags.DaemonBuildTags(root, binary.Base)
	if err != nil {
		return err
	}
	command := processCommand{
		Arguments: []string{
			"go", "build",
			"-tags", tags,
			"-o", filepath.Join(root, binary.Output),
			labBuildPackage,
		},
		Timeout:     labBuildTimeout,
		Directory:   root,
		Environment: environment,
	}

	result, err := runner.Run(ctx, command)
	if err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("cross-compile ").Str(binary.Name).Str(" into ").
			Str(binary.Output).Str(": ").Err(err).Str(": ").Str(buildDiagnostic(result)).String())
	}
	if result.ExitCode != 0 {
		var tb textbuf.Buffer
		return errors.New(tb.Str("cross-compile ").Str(binary.Name).Str(" into ").
			Str(binary.Output).Str(" exited ").Int(int64(result.ExitCode)).Str(": ").
			Str(buildDiagnostic(result)).String())
	}
	return nil
}

// buildDiagnostic answers what the compiler said, preferring its diagnostics on
// stderr and falling back to stdout. An empty answer is named rather than left
// blank, so a failure never reads as a build that said nothing wrong.
func buildDiagnostic(result processResult) string {
	if trimmed := strings.TrimSpace(result.Stderr); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(result.Stdout); trimmed != "" {
		return trimmed
	}
	return "the compiler printed nothing"
}
