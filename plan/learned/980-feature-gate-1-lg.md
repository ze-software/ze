# 980 - feature-gate child 1: per-feature compile-out (looking-glass pilot)

## Context

The feature-gate umbrella (`plan/spec-feature-gate-0-umbrella.md`) makes optional
services compile-out-able from the `ze` binary via per-feature `ze_<feature>` build
tags, for a smaller binary and attack surface. Child 1 proves the pattern end-to-end
on one service: a construction registry, tag-gated registration + schema, present/
absent build-tag tests, and a `dep_audit.py` no-direct-import gate.

The umbrella picked `ssh` as "the clean pilot (one ctor, daemon-only)". Reading the
hub falsified that: ssh is the *most*-coupled service (two construction paths in
`infra_setup.go` + `main.go`, ~10 setters, the whole interactive-CLI surface in
`session_factory.go`). The genuinely clean service is **looking-glass (lg)**: a
single self-contained `startLGServer`, one call site, and one already-generic
coupling (`ListenerMigrator.SetLG`). The user approved pivoting the pilot to lg;
ssh becomes child 2 (the real hardening target) on the proven pattern.

## Decisions

- **Construction registry in `package hub`, not a new package.** `service_registry.go`
  (always-on) holds `Service` (`Reconfigurable`+`Name`+`Shutdown`), `ServiceDeps` (generic
  deps), `registerService`/`buildServices`/`registerBuiltService`. The lg factory +
  adapter live in `service_lg.go` (`//go:build ze_lg`) with registration in
  `register_lg.go` (the only direct `internal/component/lg` import). The hub iterates
  `buildServices(deps)` instead of calling `lg.NewLGServer`; lg stays a pure component.
- **Service iface is NOT `ze.Subsystem`.** The umbrella assumed factories return a
  `ze.Subsystem`; only ssh is one. lg/web/gnmi are hub-managed HTTP servers
  (ListenAndServe/Shutdown/Addresses). The iface reuses the EXISTING `Reconfigurable`
  contract (`listener_migrate.go`) that `lg.LGServer` already satisfies; the only
  concrete coupling was the `SetLG(*lg.LGServer)` signature, widened to `SetLG(Reconfigurable)`.
- **Generator gates the YANG schema too (AC-6).** `plugin_imports.go` grew a
  `featureTags` map (`internal/component/lg/yang -> ze_lg`); tagged imports are emitted
  into a generated `all_ze_lg.go` (`//go:build ze_lg`) and removed from the flat
  `all.go`. So a no-`ze_lg` build also drops the config vocabulary: `looking-glass{}`
  config gets a clean "unknown field" validation error (not a panic).
- **`ZE_FEATURES` Makefile variable** centralizes default-on feature tags (one place,
  not inlined per target → mitigates "miss one of the duplicate build blocks"). `ze`/
  `ze-appliance` pass `$(ZE_FEATURES)`; `ze-stripped` (ze_core only) omits them.
- **`.golangci.yml` build-tags = `ze_core ze_lg`.** Lint the shipped feature set so the
  `//go:build ze_<feature>` files get real coverage and registry callers reached only
  from gated code (`registerService`) are not flagged unused. Add a feature's tag here
  when it becomes default-on.
- **`dep_audit.py` no-direct-import gate (AC-7).** A `DISABLEABLE` map (pkg -> tag);
  the `--check` gate fails if any always-on (untagged, non-test) file imports a
  disableable package. `--selftest` proves detection (flags an always-on importer,
  allows a build-tag-gated one and a test importer).

## Consequences

- `go tool nm`: ze_core build links **0** `internal/component/lg` server symbols (and 0
  `lg/yang`); ze_core+ze_lg links 183 (+3). Default `ze`/`ze-appliance` keep lg.
- The pattern generalizes: child specs add `ze_web`/`ze_ssh`/`ze_gnmi`/`ze_mcp` by
  adding a tag in FOUR places -- `ZE_FEATURES` (Makefile), `.golangci.yml` build-tags,
  `TestBuildTags()` (`internal/test/runner/runner.go`), and `featureTags` (generator,
  if it has a schema) -- plus a `service_<x>.go`/`register_<x>.go` and a present/absent
  build-tag test. ssh is child 2 and is bigger (decouple `session_factory.go` from
  `*zessh.Server`). Missing the `TestBuildTags` entry is the trap: the functional-test
  runner builds its OWN `ze` (not `bin/ze`) and every `.ci` test for the feature fails
  with "unknown field"/http-check until its tag is added there.
- Registration `init()` MUST live in a `register*.go` file (hook-enforced); the factory
  body goes in a sibling `service_<x>.go` under the same `//go:build` tag.

## Gotchas

- **`buildLGService` returns `(nil, nil)` only for the not-configured skip** (with
  `//nolint:nilnil`); failure paths return errors (logged by `buildServices` via slog).
  The original `startLGServer` printed to stderr and returned nil for every case;
  best-effort non-fatal behavior is preserved, the channel changed.
- **lg-only helpers must move into the gated file**, not stay always-on: `serveLG`,
  `serveLGBlocking`, `parseASNForDecorator` had no other caller after the move, so a
  no-`ze_lg` build would flag them U1000-unused. They live in `service_lg.go` now.
- **Generator stale-file glob must exclude `_test.go`**: `all_*.go` matches the large
  `all_test.go`. The generator only removes files carrying the "Code generated ... DO
  NOT EDIT" marker, never test files.
- **The `.ci` parse-harness runs one flavor**, so a no-lg `lg-absent-config.ci` is not
  expressible there. AC-4 (no-lg config safety) is guarded by `TestBuildTag_LG_Absent`
  + the generator `--check` + an empirical `ze-stripped config validate` run, not a .ci.
- **The no-sprintf-alloc hook applies even to `//go:build ignore` build tools**: in
  `plugin_imports.go` use `strings.Builder` + `w.WriteString` (no `+` concat, no
  `fmt.Fprintf(w, "%s")`); only `fmt.Fprintf(os.Stdout/os.Stderr, ...)` and `fmt.Errorf`
  are allowed.

## Files

- `cmd/ze/hub/service_registry.go` (new) - registry: Service/ServiceDeps/ServiceFactory,
  registerService/buildServices/registerBuiltService
- `cmd/ze/hub/service_lg.go`, `register_lg.go` (new, `//go:build ze_lg`) - lg factory +
  moved startLGServer/serveLG/parseASNForDecorator + registration
- `cmd/ze/hub/{service_registry,service_lg,build_tag_lg_present,build_tag_lg_absent}_test.go` (new)
- `cmd/ze/hub/main.go`, `main_servers.go`, `main_system.go`, `listener_migrate.go` - hub
  builds lg via the registry; `SetLG(Reconfigurable)`; lg import removed from always-on
- `scripts/codegen/plugin_imports.go` - featureTags + per-tag `all_<tag>.go` emission/check
- `internal/component/plugin/all/all_ze_lg.go` (generated), `all.go` (lg/yang removed)
- `scripts/dev/dep_audit.py` - DISABLEABLE map + disableable_gate + selftest fixture
- `Makefile` (`ZE_FEATURES`), `.golangci.yml` (ze_lg build-tag),
  `internal/test/runner/runner.go` (`TestBuildTags` += ze_lg),
  `ai/rules/architecture.md`, `docs/features.md`
