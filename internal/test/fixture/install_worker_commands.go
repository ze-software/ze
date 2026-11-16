// Design: docs/functional-tests.md -- synthetic kernel worker command boundaries
// Related: install_fixture.go -- fixture executable dispatch and seeded worker

package fixture

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const installWorkerArchArg = "ARCH=x86_64"

// installWorkerMakeStub supplies make's command boundary without borrowing a
// host utility whose multicall dispatch can reject the make basename.
func installWorkerMakeStub(args []string) int {
	if err := installWorkerMake(args, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err) //nolint:errcheck // best-effort diagnostic before the fixture exits nonzero
		return 1
	}
	return 0
}

// installWorkerMake reports the accepted command without compiling a kernel or
// changing the seeded resolved config and artifacts.
func installWorkerMake(args []string, output io.Writer) error {
	if !installWorkerMakeArgs(args) {
		return fmt.Errorf("unexpected synthetic worker make arguments: %q", args)
	}
	_, err := fmt.Fprintln(output, strings.Join(args, " "))
	return err
}

// installWorkerMakeArgs accepts only the synthetic request's architecture,
// single build job, image target and optional module operations.
func installWorkerMakeArgs(args []string) bool {
	for _, want := range [][]string{
		{installWorkerArchArg, "defconfig"},
		{installWorkerArchArg, "olddefconfig"},
		{installWorkerArchArg, "-j1", "bzImage"},
		{installWorkerArchArg, "-j1", "bzImage", "modules"},
	} {
		if slices.Equal(args, want) {
			return true
		}
	}
	if len(args) != 3 {
		return false
	}
	if args[0] != installWorkerArchArg {
		return false
	}
	if args[2] != "modules_install" {
		return false
	}
	path, found := strings.CutPrefix(args[1], "INSTALL_MOD_PATH=")
	if !found {
		return false
	}
	return filepath.IsAbs(path)
}

// installWorkerMergeStub preserves the seeded resolved config so RunWorker's
// real required-symbol check can distinguish =m from =y after this boundary.
func installWorkerMergeStub(args []string) int {
	if !installWorkerMergeArgs(args) {
		fmt.Fprintf(os.Stderr, "unexpected synthetic worker merge arguments: %q\n", args) //nolint:errcheck // best-effort diagnostic before the fixture exits nonzero
		return 1
	}
	return 0
}

func installWorkerMergeArgs(args []string) bool {
	if len(args) != 3 {
		return false
	}
	if args[0] != "-m" {
		return false
	}
	if args[1] != ".config" {
		return false
	}
	return filepath.IsAbs(args[2])
}
