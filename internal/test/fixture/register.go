// Design: docs/architecture/testing/ci-format.md -- foreground helper exit assertions.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
)

func init() {
	Register("runner/helper-command-exit", helperCommandExit)
}

// helperCommandExit supplies process output and status, not an observer verdict.
// A requested nonzero exit MUST end the process: returning an error would emit
// ZE-OBSERVER-FAIL and replace the requested status with 1 in Run.
func helperCommandExit(_ context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("runner/helper-command-exit requires <stdout-line> <exit-code>; supply a line and a status from 0 through 255")
	}
	code, err := strconv.ParseUint(args[1], 10, 8)
	if err != nil {
		return fmt.Errorf("parse helper exit code %q: %w; supply a status from 0 through 255", args[1], err)
	}
	if _, err := os.Stdout.WriteString(args[0] + "\n"); err != nil {
		return fmt.Errorf("write helper stdout: %w", err)
	}
	if code == 0 {
		return nil
	}
	os.Exit(int(code))
	return nil
}
