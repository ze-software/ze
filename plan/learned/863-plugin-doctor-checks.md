# Plugin Doctor Check Registration

## Context

External plugins (Python, Go, any language) had no way to contribute doctor
checks. Go plugins register checks at compile time via `init()`, but the
plugin IPC protocol had no doctor check declaration or callback.

## Decision

Extend the existing Stage 1 registration + callback pattern to doctor checks:

- `DoctorCheckDecl` in `declare-registration` (Stage 1 wire type)
- `ze-plugin-callback:doctor-check` callback invoked at runtime
- `HandleShowDoctor` queries running plugins alongside Go-registered checks
- Offline `ze doctor` unchanged (plugins are runtime processes)

This follows the same pattern as commands, families, filters, and YANG schemas:
declare at registration, invoke via callback.

## Consequences

- Plugin doctor checks vanish when the plugin is removed (self-containment preserved)
- Two parallel paths for doctor checks: Go registry (offline) and plugin callback (runtime)
- `show doctor` output shape unchanged; plugin diagnostics appended to same list
- Diagnostic codes from plugins are validated at registration (kebab-case, `doctor-` prefix)
- No bridge fast path for `SendDoctorCheck` (infrequent invocation, pipe round-trip acceptable)

## Gotchas

- `DoctorCheckPhase.Valid()` must live in `pkg/plugin/rpc` (where the type is defined),
  not in the server package (cannot define methods on non-local types)
- `diagnostic.Diagnostic` has no `Detail` field; do not add one to the wire type
  without extending the core diagnostic struct first
- Offline `ze explain` cannot know plugin diagnostic codes (they are runtime-declared);
  AC-10 was scoped to runtime code validation only

## Files

None recorded.
