# Ze Go Style: assertions

The detail behind "Assertions, in a language that has none" in
[the Go style core](../ze-go-style.md), which states the rule. The paragraphs
below follow the order of the core's paragraphs.

| The failure | The tool |
|-------------|----------|
| A programmer error that MUST NOT happen at runtime, including a startup registration failure in `init()` | `panic("BUG: <what>")` (never `os.Exit()`) |
| An operating error that a running system produces | An error return, wrapped with `fmt.Errorf("context: %w", err)` |
| A design property that holds before the program runs | A type that cannot hold the bad state, `var _ Iface = (*T)(nil)`, or a test |

A startup registration failure in `init()` (such as `registry.Register`,
`diagnostic.RegisterDoctorCheck`, backend or validator registration) can only
result from a compile-time Ze defect. Panic with
`panic("BUG: <pkg>: ...: " + err.Error())` so Go prints the stack trace and
aborts startup cleanly; never call `os.Exit()`.

A check that runs can be forgotten, but a state that cannot be written needs no
check. "Types that cannot lie" shows the forms.

Validate an attribute when it is parsed from the wire, and again when it is
written back. A bug that survives one check rarely survives both.

A fuzz target proves that defects are present, never that they are absent.
Build the mental model first, write the checks that encode it, then let the
fuzzer find the gap.
<!-- source: ai/rules/testing.md -- Boundary Testing, Fuzz -->
