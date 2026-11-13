// Design: docs/architecture/testing/interop.md -- actual system RIB process proof.
// Related: sysrib_execution_fixture.go -- the existing exact procfs observer.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// sysribProcessPlacementDriver checks the actual process identified by the
// daemon's authenticated session-ping response. It starts no SDK connection.
func sysribProcessPlacementDriver(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("sysrib process placement takes daemon PID, plugin name, and internal|forked")
	}
	pid, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("parse daemon PID: %w", err)
	}
	if pid < 1 {
		return errors.New("sysrib process placement needs a positive daemon PID")
	}
	want := 0
	switch args[2] {
	case sysribPlacementInternal:
	case sysribPlacementForked:
		want = 1
	default:
		return fmt.Errorf("unknown system RIB placement %q", args[2])
	}
	children, observed, err := sysribExecutionPluginChildren(ctx, pid, args[1])
	if err != nil {
		return fmt.Errorf("%w; process evidence: %s", err, observed)
	}
	if len(children) != want {
		return fmt.Errorf("daemon %d has plugin %s child PIDs %v, want %d; process evidence: %s", pid, args[1], children, want, observed)
	}
	var output textbuf.Buffer
	output.Str("OK: system RIB placement ").Str(args[2]).Str("; daemon PID ").Uint(uint64(pid))
	for _, child := range children {
		output.Str("; ze plugin ").Str(args[1]).Str(" PID ").Uint(uint64(child))
	}
	return output.Byte('\n').StdOut()
}
