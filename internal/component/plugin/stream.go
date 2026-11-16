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

// CommandStreamAdmission invokes run only after the stream registry, arguments,
// authorization and accounting admit the original command. The run context ends
// with either the request or daemon. The accepted caller policy stays bound to
// this run; controls MUST NOT replace it with a later action request's policy.
// A false result MUST NOT invoke run, so another command route may handle it.
type CommandStreamAdmission func(ctx context.Context, caller CallerIdentity, input string, run func(context.Context, CallerIdentity) error) (bool, error)

// CommandStreamHandlers shares admission between raw streaming and a registered
// interactive view. A nil member makes that transport unavailable.
type CommandStreamHandlers struct {
	Dispatch CommandStreamDispatcher
	Admit    CommandStreamAdmission
}
