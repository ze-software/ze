# Wiring Completeness

**When:** Wiring is not a verification step at the end
**Severity:** blocking

## Directives

Extends `no-partial-completion.md` with a mechanical check.

## The Principle: Wire First, Feature Second

Wiring is not a verification step at the end. It is the first implementation step.

1. **Design phase:** the spec's Wiring Test table names every entry point before implementation starts.
2. **Implementation phase:** `/ze-implement` step 4 creates the entry point skeleton and a failing wiring test before any feature code is written.
3. **Review phase:** `/ze-review` step 1 checks wiring before any other analysis.
4. **Completion phase:** the mechanical check below catches anything that slipped through.

If you find yourself checking wiring for the first time at completion, three earlier gates failed.

## The Rule

Every exported function, type, or constant created by a spec implementation
MUST have at least one caller in the running daemon. "Library code with tests"
is not done. "Tested but not wired" is not done.

## Mechanical Check (MANDATORY before claiming done)

`make ze-verify` runs `make ze-verify-wiring-docs`. That changed-file
gate is blocking and checks:

- new exported Go symbols under `internal/` or `cmd/` have a non-test
  production reference in `internal/` or `cmd/`;
- command declaration changes run `make ze-validate-commands`;
- source-anchored documentation changes run doc drift and stale-anchor
  checks;
- plugin registration and generated inventory source changes run
  registry-backed inventory checks.

For manual review of a specific new exported symbol `Foo`, confirm it
is not only a definition plus tests:

```
grep -rn 'Foo' internal/ cmd/ --include="*.go" | grep -v "_test.go" | grep -v "plan/"
```

If the only hits are the definition and test files, the symbol is dead
code. Dead code is a BLOCKER, not a NOTE.

For multi-consumer data (route attributes, config fields, bus events),
grep all consumers: UI templates, graph rendering, functional tests,
CLI formatters. Changing the producer without updating consumers is
incomplete, not done.

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
| `internal/component/host/` | `cmd/ze/hub/main.go`, `loader_create.go`, `internal/component/cmd/show/system.go`, or `web/page_system.go` |
| `internal/component/config/system/` | `cmd/ze/hub/main.go` (startup + reload) |
| Any new metrics registration | `loader_create.go` telemetry block |
| Any new report bus emission | Verified via `show warnings` / `show errors` |
