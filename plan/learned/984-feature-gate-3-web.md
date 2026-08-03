# 984 - feature-gate-3-web

## Context

The web UI used to be always linked into `ze`, including stripped builds, because
always-on hub, installer, init, and looking-glass code imported
`internal/component/web` for both lifecycle and borrowed certificate helpers.
The goal was to make web compile-out-able behind `ze_web` while preserving default
full builds and install-time TLS bootstrap behavior.

## Decisions

- **Extract before gating.** The self-signed cert/TLS helpers moved from
  `internal/component/web` to the always-on leaf package `internal/core/selfcert`.
  Appliance init, `ze init`, looking-glass, and web now share that leaf instead of
  borrowing from the UI package.
- **Use the construction registry.** Web fits the same listener-service registry
  shape as looking-glass: a gated `service_web.go` builds a `Service`, and
  `register_web.go` registers it under `ze_web`. Always-on hub code sees only
  `Reconfigurable` and generic `ServiceDeps`.
- **Gate the second construction path.** `RunWebOnly` is routed through a nil-able
  web-only seam so a no-web build returns a clean "web not compiled in" failure
  instead of importing the web package.
- **Keep cert storage always-on.** The zefs-backed `blobCertStore` lives in hub
  because looking-glass still needs certificate storage when `ze_lg` is on and
  `ze_web` is off.
- **Manifest owns build-time feature facts.** `feature-gates.txt` declares
  `ze_web internal/component/web`; Makefile, test runner, generator, and dep audit
  derive from it. The static `.golangci.yml` tag list is drift-checked.

## Consequences

- `go build -tags ze_core` links zero `internal/component/web` symbols.
- Default `ze` and `ze-appliance` still include web because `ZE_FEATURES` derives
  `ze_web` from the manifest.
- `ze-stripped` keeps `ze_ssh` as the management plane and drops web.
- A no-web build rejects `web {}` config as an unknown field without panic.
- Certificate generation is no longer owned by the UI component, which keeps future
  compile-out gates from pinning web through utility imports.

## Gotchas

- Web was easy to registry-ize because its direct call sites concentrated in
  `startWebServer`; the real blocker was non-lifecycle exports used by always-on
  code.
- Functional tests build their own `ze`; missing the manifest-derived
  `TestBuildTags` path makes web `.ci` tests fail even when normal builds work.
- `RunWebOnly` is a separate construction path. A service can be registry-based
  and still need a small standalone seam for no-BGP modes.
- Do not leave aliases in `internal/component/web` for moved cert helpers. Any
  always-on caller of those aliases pins web back into stripped binaries.

## Files

- `feature-gates.txt`
- `ai/rules/plugins.md`
- `cmd/ze/hub/service_registry.go`
- `cmd/ze/hub/service_web.go`
- `cmd/ze/hub/register_web.go`
- `cmd/ze/hub/web_infra.go`
- `cmd/ze/hub/cert_store.go`
- `cmd/ze/hub/listener_migrate.go`
- `internal/core/selfcert/selfcert.go`
- `internal/core/selfcert/selfcert_test.go`
- `internal/component/web/server.go`
- `internal/component/plugin/all/all.go`
- `internal/component/plugin/all/all_ze_web.go`
- `scripts/codegen/plugin_imports.go`
- `scripts/dev/dep_audit.py`
- `internal/test/runner/runner.go`
- `internal/test/runner/manifest_test.go`
- `Makefile`

## Verification

- `make ze-lint-changed`: 0 issues before commit `a46cdc1b6`.
- `scripts/dev/dep_audit.py --check`, golangci drift gate, generator check: clean
  before commit `a46cdc1b6`.
- Hub web-on and web-off build-tag suites, web component tests, selfcert tests, and
  init cert tests passed before commit `a46cdc1b6`.
- Symbol evidence before commit `a46cdc1b6`: 0 web symbols in `ze_core` and
  `ze-stripped`; 716 web symbols with `ze_web` enabled.
- Full `make ze-verify` after `a46cdc1b6` was blocked by known pre-existing and
  other-session failures, not by feature-gate-3-web code.
