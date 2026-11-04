// Design: docs/architecture/appliance/builder.md -- explicit image injection execution.
package appliance

import (
	"context"
	"errors"
)

// runInjectionCommand preserves the existing per-command buildTimeout for every
// executor. The caller MUST supply its originating context; execute MUST use it
// and join its command before returning. The native proof supplies its checked
// process-group executor. Cleanup passes WithoutCancel explicitly.
func runInjectionCommand(
	ctx context.Context,
	execute func(context.Context, string, ...string) ([]byte, error),
	name string,
	args ...string,
) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, err := execute(ctx, name, args...)
	if cause := ctx.Err(); cause != nil {
		return output, errors.Join(cause, err)
	}
	return output, err
}
