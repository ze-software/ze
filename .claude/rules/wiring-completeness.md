# Wiring Completeness

**BLOCKING.** Extends `no-partial-completion.md` with a mechanical check.

## The Rule

Every exported function, type, or constant created by a spec implementation
MUST have at least one caller in the running daemon. "Library code with tests"
is not done. "Tested but not wired" is not done.

## Mechanical Check (MANDATORY before claiming done)

For every new exported symbol `Foo` in the diff:

```
grep -rn 'Foo' internal/ cmd/ --include="*.go" | grep -v "_test.go" | grep -v "plan/"
```

If the only hits are the definition and test files, the symbol is dead code.
Dead code is a BLOCKER, not a NOTE.

## Common Violations

| Pattern | Why it's wrong |
|---------|----------------|
| "The caller will wire it later" | Later never comes. Other sessions see it as done. |
| "It's available for callers" | Available is not wired. No caller means no effect. |
| "The review said NOTE" | Reviews must flag unwired code as BLOCKER. |
| "The web UI doesn't need it" | If the feature produces data that a UI page renders, the UI must show it. |

## Where to Check

| New code in | Must be called from |
|-------------|---------------------|
| `internal/component/host/` | `cmd/ze/hub/main.go`, `loader_create.go`, `cmd/show/host.go`, or `web/page_system.go` |
| `internal/component/config/system/` | `cmd/ze/hub/main.go` (startup + reload) |
| Any new metrics registration | `loader_create.go` telemetry block |
| Any new report bus emission | Verified via `show warnings` / `show errors` |
