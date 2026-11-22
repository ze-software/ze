# Ze Go Style: every error is handled

The detail behind "Every error is handled" in
[the Go style core](../ze-go-style.md), which states the rule. The paragraphs
below follow the order of the core's paragraphs.

## An error that is not returned

Most catastrophic failures in distributed systems come from the handling of an
error the software had already detected. (The core states the rule that follows
from it.)

The code goes on as if the operation had happened, so the log line is the only
record that it did not, and debug is off on a running system.
`./le arch refuse-by-default check` refuses the debug or info form, the "Error
dropped below WARN" shape of [zero-values.md](zero-values.md).

## A wrap

When the config loader returns `parse config: %w` and the reload that called it
wraps that in `reload: parse config: %w`, the operator reads `reload: parse
config: parse config: line 3`. `./le arch error-wrap check`,
`./le go style check` and `./le commit create` refuse a newly added
`fmt.Errorf` when one of the `: `-separated segments before
its `%w` is already in the error it wraps. A segment's verbs compare as one
mark, so `line %d: ` written by the parser and again by the renderer is a
repeat, while a part that holds a verb and no word, a bare `%s: `, is no
segment. The check traces the error back through the syntax: the assignment
that bound it, the callee that returned it, and every return that passes it up
unwrapped. A called name that holds a function value is followed to the values
it can hold: a literal, a returned result, and for a function parameter the
argument every call in the same package passes for it, as far as the trace's
depth bound of 16 steps. The refusal names the wrap and the constructor that
first wrote the segment. A producer the syntax cannot name in full (an
interface method, an error-valued parameter, a parameter of an exported
function, a method or a literal, whose callers may sit outside the package)
makes the check count that wrap as untraced rather than pass it in silence,
while it still reads every producer it did name. `./le arch error-wrap report`
lists every such wrap in the tree.
<!-- source: internal/le/arch/errorwrap/errorwrap.go -- Check, judge, trace -->
<!-- source: internal/le/arch/errorwrap/format.go -- parseFormat, literalSegments -->
<!-- source: internal/le/arch/errorwrap/funcvalue.go -- funcValues, argumentValues -->

## The operator error

The handler can be a command handler, an RPC handler or a config verifier.
`opererr.New` and `opererr.Wrap` refuse an empty part, which only a Ze defect
produces. Context is added once, by the producer that holds the fact. A layer
above passes the error through unwrapped, or wraps it only to add a fact the
producer did not hold. A second `parse config:` on the same chain is the
defect this prevents. A handler that refuses by answering a
`plugin.StatusError` response builds it with `plugin.OperatorErrorResponse`,
the one route an operator error takes into `Response.Error`. How each surface
renders the three parts, and what `./le arch operator-error check` refuses:
`docs/architecture/cli/error-surface.md`, "The operator error".
<!-- source: internal/core/opererr/opererr.go -- New, Wrap, Error -->
