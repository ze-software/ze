// Design: docs/functional-tests.md -- running `le spec design` against a fixture checkout
// Related: register_spec_design.go -- registration of this fixture

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// specDesignInDir invokes `le spec design <args...>` against the synthetic
// checkout rooted at <dir>, pinning ZE_REPO_ROOT and the child's working
// directory to <dir>, and asserting that the exit code matches <want-exit>.
//
// Arguments: <dir> <want-exit> <args...>.
func specDesignInDir(ctx context.Context, args []string) error {
	if len(args) < 3 {
		return errors.New("usage: spec/design-in-dir <dir> <want-exit> <args...>")
	}
	absDir, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("spec/design-in-dir: resolve %q: %w", args[0], err)
	}
	wantExit, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("spec/design-in-dir: parse want-exit %q: %w", args[1], err)
	}
	leBin, err := nativeLEBinary()
	if err != nil {
		return fmt.Errorf("spec/design-in-dir: %w", err)
	}

	argv := append([]string{"spec", "design"}, args[2:]...)
	cmd := exec.CommandContext(ctx, leBin, argv...) //nolint:gosec // fixture invokes the checkout's native le binary
	cmd.Dir = absDir
	cmd.Env = withRepoRootEnv(os.Environ(), absDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	runErr := cmd.Run()
	code := 0
	if runErr != nil {
		exitErr, ok := errors.AsType[*exec.ExitError](runErr)
		if !ok {
			return fmt.Errorf("spec/design-in-dir: run le spec design: %w", runErr)
		}
		code = exitErr.ExitCode()
	}
	if code != wantExit {
		return fmt.Errorf("spec/design-in-dir: le spec design exited %d, want %d\nstdout:\n%s\nstderr:\n%s",
			code, wantExit, stdout.String(), stderr.String())
	}

	os.Stdout.Write(stdout.Bytes()) //nolint:errcheck // forwarding fixture command stdout to test runner
	os.Stderr.Write(stderr.Bytes()) //nolint:errcheck // forwarding fixture command stderr to test runner
	if code != 0 {
		return fmt.Errorf("le spec design exited %d", code)
	}
	return nil
}

// withRepoRootEnv replaces any ZE_REPO_ROOT / ze.repo.root spelling with root.
func withRepoRootEnv(base []string, root string) []string {
	out := make([]string, 0, len(base)+1)
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		lower := strings.ToLower(name)
		if lower == "ze_repo_root" {
			continue
		}
		if lower == "ze.repo.root" {
			continue
		}
		out = append(out, entry)
	}
	return append(out, "ZE_REPO_ROOT="+root)
}
