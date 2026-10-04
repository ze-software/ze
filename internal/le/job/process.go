// Design: docs/architecture/core-design.md -- child process exit and signal propagation
// Related: job.go -- admitted jobs use the same signal forwarding primitive
package job

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/gaterun"
)

// ProcessIO specifies the streams and environment for one child process.
type ProcessIO struct {
	Dir     string
	Environ []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

// RunProcess starts one child, forwards SIGINT and SIGTERM, waits, and returns
// the shell-compatible child status. A start error returns CannotStart and the
// error. A child failure is a verdict, not a start error.
func RunProcess(argv []string, processIO ProcessIO) (int, error) {
	if len(argv) == 0 {
		return gaterun.CannotStart, ErrNoCommand
	}
	environ, err := commandEnvironment(argv, processIO.Dir, processIO.Environ)
	if err != nil {
		return gaterun.CannotStart, err
	}
	//nolint:gosec // argv is the caller's command; le is a build-host tool
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Dir = processIO.Dir
	cmd.Env = environ
	cmd.Stdin = processIO.Stdin
	cmd.Stdout = processIO.Stdout
	cmd.Stderr = processIO.Stderr
	if err := cmd.Start(); err != nil {
		return gaterun.CannotStart, err
	}
	stop := forwardSignals(cmd)
	err = cmd.Wait()
	stop()
	if err != nil {
		return gaterun.ExitCode(err), nil
	}
	return 0, nil
}

// commandEnvironment isolates direct Go tests and selects CGO for their race
// flag. Other commands retain their launch context and CGO setting.
func commandEnvironment(argv []string, dir string, environ []string) ([]string, error) {
	if environ == nil {
		environ = os.Environ()
	}
	if len(argv) < 2 {
		return environ, nil
	}
	if filepath.Base(argv[0]) != "go" {
		return environ, nil
	}
	args := argv[1:]
	// -C is Go's global directory option, preceding the subcommand.
	if args[0] == "-C" {
		if len(args) < 3 {
			return environ, nil
		}
		args = args[2:]
	} else if strings.HasPrefix(args[0], "-C=") {
		args = args[1:]
	}
	if len(args) == 0 {
		return environ, nil
	}
	if args[0] != "test" {
		return environ, nil
	}
	race, explicit := goTestRace(args[1:], false)
	if !explicit {
		flags, err := goTestDefaults(argv[:len(argv)-len(args)], dir, environ)
		if err != nil {
			return nil, err
		}
		race = goFlagsRace(flags)
	}
	isolated := env.Without(environ, "ze.repo.root", "ze.le.build.name")
	if race {
		return append(isolated, "CGO_ENABLED=1"), nil
	}
	return append(isolated, "CGO_ENABLED=0"), nil
}

// goTestDefaults uses Go's effective configuration, including GOENV and its
// platform-specific default path. A nonempty OS override already determines
// the answer; an empty value does not suppress Go's persistent configuration.
func goTestDefaults(prefix []string, dir string, environ []string) (string, error) {
	for _, entry := range slices.Backward(environ) {
		if value, found := strings.CutPrefix(entry, "GOFLAGS="); found {
			if value != "" {
				return value, nil
			}
			break
		}
	}
	// A configuration query must not indefinitely delay starting the test.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	args := make([]string, 0, len(prefix)+1)
	args = append(args, prefix[1:]...)
	args = append(args, "env", "GOFLAGS")
	cmd := exec.CommandContext(ctx, prefix[0], args...) //nolint:gosec // the selected Go executable also runs the test
	cmd.Dir = dir
	cmd.Env = environ
	var output, diagnostic strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &diagnostic
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("resolve Go test GOFLAGS: %w", err)
	}
	stop := forwardSignals(cmd)
	err := cmd.Wait()
	stop()
	if err != nil {
		return "", fmt.Errorf("resolve Go test GOFLAGS: %w: %s", err, strings.TrimSpace(diagnostic.String()))
	}
	return strings.TrimSpace(output.String()), nil
}

// goTestPackagePhase distinguishes package operands from positional test args.
type goTestPackagePhase uint8

const (
	goTestPackageUnspecified goTestPackagePhase = iota
	goTestPackagePending
	goTestPackageCollecting
	goTestPackageComplete
)

// goTestRace follows Go's testFlags package-list and flag-value boundaries.
// The scan is bounded by argv. Go retains responsibility for flag validation.
func goTestRace(args []string, race bool) (bool, bool) {
	explicit := false
	packages := goTestPackagePending
	unknownValue := false
	for len(args) > 0 {
		arg := args[0]
		args = args[1:]
		afterUnknown := unknownValue
		unknownValue = false
		if arg == "-args" {
			break
		}
		if arg == "--args" {
			break
		}
		if arg == "--" {
			break
		}
		isFlag := strings.HasPrefix(arg, "-") && arg != "-"
		if !isFlag {
			switch packages {
			case goTestPackageUnspecified:
				panic("BUG: Go test package phase is unspecified")
			case goTestPackagePending, goTestPackageCollecting:
				packages = goTestPackageCollecting
			case goTestPackageComplete:
				if !afterUnknown {
					return race, explicit
				}
			}
			continue
		}
		if packages == goTestPackageCollecting {
			packages = goTestPackageComplete
		}
		name, value, assigned := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "=")
		if name == "race" {
			explicit = true
			if !assigned {
				race = true
				continue
			}
			enabled, err := strconv.ParseBool(value)
			if err != nil {
				// The unchanged argv lets Go report the invalid boolean.
				return false, explicit
			}
			race = enabled
			continue
		}
		if goTestFlagTakesValue(name) {
			if !assigned {
				if len(args) > 0 {
					args = args[1:]
				}
			}
			continue
		}
		if goTestFlagBoolean(name) {
			continue
		}
		// Go forwards unknown flags and allows their next positional operand
		// to be a value, but closes the package list even without any packages.
		packages = goTestPackageComplete
		unknownValue = !assigned
	}
	return race, explicit
}

// goFlagsRace reads effective GOFLAGS. Go quotes whole fields with single/double
// quotes and does no unescaping; a shell lexer would interpret a different
// language. Each loop consumes a field or stops, bounded by the setting's
// length. Go reports malformed input.
func goFlagsRace(flags string) bool {
	race := false
	for flags != "" {
		flags = strings.TrimLeft(flags, " \t\r\n")
		if flags == "" {
			break
		}
		var field string
		if flags[0] == '\'' || flags[0] == '"' {
			end := strings.IndexByte(flags[1:], flags[0])
			if end < 0 {
				// Go rejects an unterminated quoted field before compiling.
				return false
			}
			field = flags[1 : end+1]
			flags = flags[end+2:]
		} else {
			end := strings.IndexAny(flags, " \t\r\n")
			if end < 0 {
				end = len(flags)
			}
			field = flags[:end]
			flags = flags[end:]
		}
		race, _ = goTestRace([]string{field}, race)
	}
	return race
}

// goTestFlagTakesValue prevents a value such as "-run -race" from enabling
// instrumentation. These are Go's value-taking build and forwarded test flags;
// unknown flags stay Go's responsibility rather than becoming job options.
func goTestFlagTakesValue(name string) bool {
	switch name {
	case "C", "p", "asmflags", "compiler", "buildmode", "gcflags", "gccgoflags",
		"mod", "modfile", "overlay", "installsuffix", "ldflags", "pgo", "pkgdir",
		"tags", "toolexec", "debug-actiongraph", "debug-runtime-trace", "debug-trace",
		"covermode", "coverpkg", "o", "exec", "vet":
		return true
	}
	switch strings.TrimPrefix(name, "test.") {
	case "bench", "benchtime", "blockprofile", "blockprofilerate", "count", "cpu",
		"cpuprofile", "fuzz", "list", "memprofile", "memprofilerate", "mutexprofile",
		"mutexprofilefraction", "outputdir", "parallel", "run", "skip", "timeout",
		"fuzztime", "fuzzminimizetime", "trace", "shuffle", "coverprofile":
		return true
	}
	return false
}

// goTestFlagBoolean identifies recognized flags that do not take a following
// value. Unknown test flags have different positional-argument semantics.
func goTestFlagBoolean(name string) bool {
	switch name {
	case "a", "n", "x", "asan", "buildvcs", "linkshared", "msan", "trimpath",
		"work", "modcacherw", "cover", "c", "json":
		return true
	}
	switch strings.TrimPrefix(name, "test.") {
	case "artifacts", "benchmem", "failfast", "fullpath", "short", "v":
		return true
	}
	return false
}
