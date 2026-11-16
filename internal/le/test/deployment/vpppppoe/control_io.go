// Design: docs/labs/pppoe-interop.md -- signal-safe scratch control I/O.
package testvpppppoe

import (
	"context"
	"errors"
	"syscall"
)

// controlIO retries only signal interruptions. Each blocking socket has a
// one-second timeout, and every retry checks the existing run's context.
func controlIO(ctx context.Context, operation func() (int, error)) (int, error) {
	for ctx.Err() == nil {
		length, err := operation()
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		return length, err
	}
	return 0, ctx.Err()
}
