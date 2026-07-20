# Registration-Based Dispatch

**When:** the registration pattern: register handlers into a dispatcher (or sub-dispatcher),
**Severity:** advisory

## Rule

**Never use switch/case to dispatch subcommands.** All command dispatch must use
the registration pattern: register handlers into a dispatcher (or sub-dispatcher),
then call `Dispatch(args)`. This applies at every level of nesting.

## Why

Switch-based dispatch:
- Hides available commands from help/completion systems
- Requires editing the dispatcher when adding a command (violates open/closed)
- Cannot provide "did you mean?" suggestions
- Cannot list available subcommands programmatically

Registration-based dispatch:
- Self-documenting (descriptions visible to help formatters)
- Open for extension without modifying existing code
- Supports typo suggestions via `suggest.Command`
- Discoverable by tooling

## How to Apply

Use `subdispatch.New(name, summary)` for any command group that has sub-actions.
Register each sub-action with its handler and description. The dispatcher handles
help, unknown-command errors, and suggestions automatically.

```go
var fooDispatcher = newFooDispatcher()

func newFooDispatcher() *subdispatch.Dispatcher {
    d := subdispatch.New("foo", "Foo operations")
    d.Register("bar", runBar, subdispatch.SubMeta{Desc: "Do bar"})
    d.Register("baz", runBaz, subdispatch.SubMeta{Desc: "Do baz"})
    return d
}

func runFoo(args []string) int {
    return fooDispatcher.Dispatch(args)
}
```

## Banned Patterns

- `switch args[0] { case "x": ... }` for command dispatch
- Manual "unknown command" error messages (the dispatcher handles this)
- Hand-written help listing subcommands (derive from registration)
