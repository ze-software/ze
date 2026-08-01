# 837 -- doctor-check-registry

## Context

`ze doctor` previously sequenced checks by directly appending every probe in `cmd/ze/doctor/doctor.go`. That made each new runtime dependency touch the central runner and left no mechanical link between a check's emitted `doctor-*` codes and `ze explain` metadata. The goal was to prove a registration-first path without changing `ze doctor --json` shape or early-return behavior for missing or invalid config. Plugin binary checks were the safe first migration because they are post-config, platform-neutral, and already covered by unit and UI functional tests.

Update: future ownership placement is superseded by [838](838-doctor-check-ownership.md). New runtime dependency checks should register from the owning package, not from `cmd/ze/doctor`, unless no narrower owner exists.

## Decisions

- Historical, now superseded for future checks by [838](838-doctor-check-ownership.md): chose an unexported registry in `cmd/ze/doctor` over a new internal package because the doctor command still owned readiness execution and no component needed to import it yet.
- Chose explicit phases over one flat list because missing-config and parse-failure behavior are semantic boundaries, not just ordering details.
- Chose plugin binary checks over listener or Linux checks because they avoid build-tag and schema-listener breadth while still proving a real user-visible diagnostic path.
- Chose central diagnostic metadata plus a consistency test over moving metadata into check registration because `diagnostic.Lookup` is already the source used by `ze explain`.
- Chose duplicate-code rejection within one check over global code uniqueness because future checks may intentionally share code families.

## Consequences

- Future runtime dependency checks should add `mustRegisterDoctorCheck(doctorCheck{...})` metadata and tests instead of appending to the `runChecks` list.
- Registered checks are sorted by phase, order, then name, so init order does not leak into doctor output.
- Platform constraints are declared in metadata and enforced by the registry runner, while build-tagged files still own OS-specific probes.
- The first slice does not make every legacy check registered; future migrations should preserve their old relative location when choosing registry hook order.
- Every registered `doctor-*` code is now tested against `diagnostic.Lookup`, which catches missing `ze explain` metadata at unit-test time.

## Gotchas

- `runChecks` needs a populated context with tree, config directory, plugin list, storage, and platform before registered post-config checks can replace direct calls.
- A test that only asserts `doctor-plugin-external-builtin` appears can pass through the old direct call, so the runner test temporarily installs a registry-backed plugin check and uses an internal plugin config the old check would not warn on.
- `make ze-lint-changed` flags classic byte-index loops with `intrange`; use `for i := range len(value)` for byte validation helpers.
- Do not add production registry introspection just for tests; same-package tests can inspect unexported registry state without creating unwired production helpers.
- Documentation for future agents lives in both `ai/rules/doctor-checks.md` and `ai/patterns/registration.md`; updating only one leaves discovery split.

## Files

- `internal/component/doctor/registry.go`
- `internal/component/doctor/check_plugins.go`
- `internal/component/doctor/doctor.go`
- `internal/component/doctor/registry_test.go`
- `internal/component/doctor/doctor_test.go`
- `ai/rules/doctor-checks.md`
- `ai/patterns/registration.md`
- `ai/CODE-TO-DOCS.md`
