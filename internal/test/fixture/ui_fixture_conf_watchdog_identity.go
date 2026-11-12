// Design: docs/functional-tests.md -- ExaBGP compatibility executable identity.

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/test/runner"
)

func init() {
	Register("ui/conf-watchdog-executable-identity", uiDriver(confWatchdogExecutableIdentity))
}

// confWatchdogExecutableIdentity drives the registered compatibility runner and
// its real watchdog wire assertions through a differently named executable.
// A wrong le on PATH must never run; the selected binary lives in a spaced path.
func confWatchdogExecutableIdentity(ctx context.Context) error {
	binary, err := nativeLEBinary()
	if err != nil {
		return err
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return fmt.Errorf("resolve selected le executable: %w", err)
	}
	zeBinary, err := uiZEBinary()
	if err != nil {
		return err
	}
	root, err := lepath.Root()
	if err != nil {
		return err
	}
	here, err := os.Getwd()
	if err != nil {
		return err
	}
	workspace, err := os.MkdirTemp(here, "conf-watchdog executable identity-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace) //nolint:errcheck // Only the fixture's own temporary links and captures are removed.
	const renamedBase = "renamed-watchdog-runner"
	renamed := filepath.Join(workspace, renamedBase)
	if err := os.Link(binary, renamed); err != nil {
		return fmt.Errorf("link the selected le under a different name without rebuilding it: %w", err)
	}
	wrongDir := filepath.Join(workspace, "wrong PATH executable")
	if err := os.Mkdir(wrongDir, 0o700); err != nil {
		return err
	}
	wrongMarker := filepath.Join(wrongDir, "wrong-le-ran")
	wrong := "#!/bin/sh\nprintf 'wrong executable ran\\n' > \"$WATCHDOG_WRONG_MARKER\"\nexit 99\n"
	if err := os.WriteFile(filepath.Join(wrongDir, "le"), []byte(wrong), 0o700); err != nil { //nolint:gosec // the PATH decoy is executed, so it needs its owner execute bit
		return err
	}
	base := childEnvironment(os.Environ(), map[string]string{
		"ZE_REPO_ROOT":          root,
		"ZE_BIN":                zeBinary,
		runner.EnvNoBuild:       "1",
		"PATH":                  wrongDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"WATCHDOG_WRONG_MARKER": wrongMarker,
	})
	refused := exec.CommandContext(ctx, renamed, "le", "help") //nolint:gosec // The fixture owns the hardlink to this run's selected binary.
	refused.Dir = here
	refused.Env = childEnvironment(base, map[string]string{"ZE_LE_BUILD_NAME": "foreign-watchdog-build"})
	output, err := refused.CombinedOutput()
	if err == nil {
		return fmt.Errorf("wrong build identity was not refused: le exited 0\n%s", output)
	}
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		return fmt.Errorf("wrong build identity was not refused: %w\n%s", err, output)
	}
	if exit.ExitCode() != 2 {
		return fmt.Errorf("wrong build identity exited %d instead of 2: %s", exit.ExitCode(), output)
	}
	if !bytes.Contains(output, []byte("foreign-watchdog-build")) {
		return fmt.Errorf("wrong build refusal omitted the requested identity: %s", output)
	}
	if !bytes.Contains(output, []byte(renamedBase)) {
		return fmt.Errorf("wrong build refusal omitted the actual executable identity: %s", output)
	}

	capture := filepath.Join(workspace, "captured watchdog frames")
	runCtx, cancel := context.WithTimeout(ctx, 65*time.Second)
	defer cancel()
	command := exec.CommandContext(runCtx, renamed, "le", "test", "exabgp", "encoding", //nolint:gosec // This is the registered runner, never a mock command.
		"--parallel", "1", "--timeout", "45s", "--verbose", "--save", capture, "conf-watchdog")
	command.Dir = root
	command.Env = childEnvironment(base, map[string]string{"ZE_LE_BUILD_NAME": renamedBase})
	// Let the compatibility runner cancel and reap its separately grouped
	// server and daemon before CommandContext's bounded wait forces an exit.
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
	command.WaitDelay = 5 * time.Second
	output, err = command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("registered conf-watchdog failed through %s: %w\n%s", renamed, err, output)
	}
	if !bytes.Contains(output, []byte("conf-watchdog")) {
		return fmt.Errorf("the runner did not identify the requested watchdog case: %s", output)
	}
	if !bytes.Contains(output, []byte("successful")) {
		return fmt.Errorf("the watchdog wire helper did not finish its assertions: %s", output)
	}
	frames, err := os.ReadFile(filepath.Join(capture, "conf-watchdog.1.frames")) //nolint:gosec // the path is under the fixture's own workspace
	if err != nil {
		return fmt.Errorf("read watchdog frames from the exact spaced --save argument: %w", err)
	}
	for _, wire := range []string{
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00310200000015400101004002004003040102030440050400000064204D4D4D4D",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001C020005204D4D4D4D0000",
	} {
		if !bytes.Contains(frames, []byte(wire)) {
			return fmt.Errorf("the real watchdog session did not record expected announcement/withdrawal %s", wire)
		}
	}
	if _, err := os.Stat(wrongMarker); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("the wrong PATH le was invoked or its sentinel cannot be checked: %v", err)
	}
	_, err = fmt.Fprintln(os.Stdout, "PASS: conf-watchdog wire assertions through selected renamed executable and spaced argv")
	return err
}
