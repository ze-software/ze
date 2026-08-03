# 838 -- doctor-check-ownership

## Context

Doctor check registration was introduced in `cmd/ze/doctor` as a first migration from direct `runChecks` calls. That made the runner explicit, but it still allowed owner-specific runtime dependency checks to accumulate in the doctor package instead of the plugin, component, backend, or command that owns the dependency.

## Decisions

- Doctor check ownership follows the same proximity rule as plugin registration: the owning package owns the registration, check function, and unit test.
- `cmd/ze/doctor` owns doctor execution phases, output shape, user-entry functional coverage, and checks with no narrower owner.
- If a dependency has no owning plugin, component, backend, or command package, keep the check and unit test in `cmd/ze/doctor` and make that explicit in the test name or comment.
- Do not add owner-specific doctor registrations to `cmd/ze/doctor` just because the current runner lives there.
- If the unexported registry location blocks owner-owned registration, moving or exposing a leaf registry API is part of adding the next owner-owned check.

## Consequences

- Future runtime dependency checks should be discoverable by deleting or reading the owner package.
- Unit tests for owner-specific checks move with the owner package; functional tests through `ze doctor --json` stay with the user entry point or existing functional suite.
- Central doctor tests should validate runner behavior, output contracts, registry metadata consistency, and no-owner checks, not every owner package's dependency logic.

## Files

- `ai/rules/repo-maintenance.md`
- `ai/patterns/registration.md`
- `ai/rules/plugins.md`
- `ai/LEARNED-INDEX.md`
- `plan/learned/837-doctor-check-registry.md`
