// Design: docs/architecture/api/commands.md -- terminal streaming dispatch.
// Related: dispatch.go -- trusted identity and single-response dispatch.

package plugin

import (
	"context"
	"errors"
	"io"
)

// ErrStreamUnauthorized identifies an authorization refusal before a streaming
// transport writes its response headers. Its message shares the command policy.
var ErrStreamUnauthorized = errors.New(UnauthorizedMessage)

// CommandStreamDispatcher runs a registered stream with a trusted identity.
// The bool reports whether the registry recognizes a stream: false MUST write
// nothing, so the caller can use its ordinary command dispatcher instead. A
// handled stream MUST stop when ctx ends and return any write error.
type CommandStreamDispatcher func(ctx context.Context, caller CallerIdentity, input string, output io.Writer) (bool, error)
