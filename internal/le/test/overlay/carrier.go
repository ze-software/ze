// Design: docs/contributing/rfc-conformance-gates.md -- every carrier applies the break through a Go overlay
// Overview: overlay.go -- the overlay these runners carry
//
// carrier.go builds what each carrier's OWN runner takes an overlay through:
// `go test` over one Go unit, the functional runner over one `.ci`, and a
// native le action over one interop scenario. Nothing here re-implements a
// runner the repository already has.
package overlay

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// LETestCommand is the le command every harness and integration run is under.
const LETestCommand = "test"

// UnitArgv answers the `go test` command that runs one Go unit, the function
// name in the package directory pkg (slash-separated, relative to the tree).
//
// -v prints `=== RUN   <Name>` as the unit starts, which is what tells a crash
// inside the unit from one in package initialization. cover holds any coverage
// options, placed after the overlay and before the package.
func UnitArgv(toolchain gotoolchain.Toolchain, pkg, name, overlay string, cover ...string) []string {
	var tb textbuf.Buffer
	options := make([]string, 0, 7+len(cover))
	options = append(options, "-run", tb.Byte('^').Str(name).Byte('$').String(), "-count=1", "-v")
	if overlay != "" {
		options = append(options, "-overlay", overlay)
	}
	options = append(options, cover...)
	options = append(options, tb.Reset().Str("./").Str(pkg).String())
	return toolchain.GoTest(gotoolchain.TestOptions{}, options...)
}

// UnitEnvironment answers the environment a Go unit run executes under. The
// overlay travels on the command line (UnitArgv), never in GOFLAGS.
func UnitEnvironment(toolchain gotoolchain.Toolchain) []string {
	return toolchain.Environment(gotoolchain.EnvOptions{Test: true, Procs: true})
}

// ActionArgv re-executes the le binary self on the native action
// `<command> <area>/<verb>`, as an interop catalog declares it.
//
// The already-built binary rather than the `./le` script, because the script
// rebuilds itself from the tree and the tree is about to be compiled under an
// overlay. Re-running the action the suite DECLARES is what keeps this from
// being a second interop runner.
func ActionArgv(self, action string) []string {
	command, verb, _ := strings.Cut(action, "/")
	return append(append([]string{self}, strings.Fields(command)...), verb)
}

// ProcessEnvironment answers the environment of a carrier that is a second
// process compiling for itself: the overlay travels in GOFLAGS, which every
// `go build` and `go test` inside it reads, and a scenario, when one is named,
// in the suite's selector variable. No file on disk is modified, which is what
// lets this run in a checkout several sessions share.
func ProcessEnvironment(overlay, selectorVariable, scenario string) []string {
	environ := os.Environ()
	if overlay != "" {
		var tb textbuf.Buffer
		environ = append(environ, tb.Str("GOFLAGS=-overlay=").Str(overlay).String())
	}
	if scenario != "" {
		var tb textbuf.Buffer
		environ = append(environ, tb.Str(selectorVariable).Byte('=').Str(scenario).String())
	}
	return environ
}

// Functional is one `.ci` run's isolated binaries, built under an overlay, and
// the command that runs it. Release MUST be called once the run has finished.
type Functional struct {
	// Argv runs the functional runner over the selected `.ci`.
	Argv []string
	// Environ is the environment the isolated binaries run under.
	Environ []string
	set     testfunctional.BinarySet
	restore func()
}

// PrepareFunctional builds the isolated set the suite runner builds, under the
// overlay when one is given, and answers the command that runs ONE `.ci` of it.
// selector is the runner's arguments after `test`. label names the set's
// directory.
//
// The overlay reaches the compiler through GOFLAGS, which every Go build
// command reads. It is set on THIS process because testfunctional.Prepare
// compiles in-process, and restored by Release: a caller runs one build at a
// time and starts no goroutine of its own, so the window belongs to nobody else.
func PrepareFunctional(toolchain gotoolchain.Toolchain, label, overlay string,
	selector []string) (*Functional, error) {
	restore := func() {}
	if overlay != "" {
		var err error
		restore, err = SetGoFlags(overlay)
		if err != nil {
			return nil, err
		}
	}
	set, err := testfunctional.Prepare(toolchain, label)
	if err != nil {
		restore()
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("cannot build the isolated binaries one .ci runs ").
			Str("against: ").Err(err).String())
	}
	argv := append([]string{filepath.Join(set.Dir, testfunctional.LE), LETestCommand}, selector...)
	return &Functional{Argv: argv, Environ: set.Environment(toolchain), set: set, restore: restore}, nil
}

// Release removes the isolated set and restores GOFLAGS. It MUST be called
// after PrepareFunctional succeeded, once the run has finished.
func (f *Functional) Release() {
	testfunctional.Release(f.set)
	f.restore()
}

// Exec runs one command under a deadline and answers whether it passed, with
// its combined output. subject names what the run observes, for the error a
// deadline produces.
//
// A deadline that expires is an ERROR rather than a red, because "the break
// reddens this unit" and "I never found out" must never be the same answer
// (ai/rules/principles.md).
func Exec(deadline time.Duration, argv, environ []string, dir, subject string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // every argv value is derived by the caller from the tree and its carrier tables
	cmd.Dir = dir
	cmd.Env = environ
	var out bytes.Buffer
	// Keep result stdout clean and share one writer so os/exec serializes
	// both streams into the attribution buffer while reporting real progress.
	output := io.MultiWriter(&out, os.Stderr)
	cmd.Stdout = output
	cmd.Stderr = output
	runErr := cmd.Run()
	if ctx.Err() != nil {
		var tb textbuf.Buffer
		return false, out.String(), errors.New(tb.Str("the observation of ").Str(subject).
			Str(" did not finish inside ").Str(deadline.String()).
			Str(", so whether the break reddens it is unknown, and an unmeasured answer is ").
			Str("never recorded as a proof").String())
	}
	return runErr == nil, out.String(), nil
}
