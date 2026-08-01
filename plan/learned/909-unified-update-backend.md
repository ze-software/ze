# 909 -- Unified update backend

## Context

Ze had two independent global singletons for firmware updates: `UpdateChecker` (passive
version check) and `SelfUpdater` (download/verify/stage/restart). Neither carried backend
identity or platform awareness. On gokrazy appliances, where image updates are managed by
gokrazy, firmware commands returned the confusing "not configured" error instead of
explaining that updates are externally managed.

## Decisions

- Single `UpdateBackend` interface with factory registry. `NewBackend(platform, cfg, opts)`
  selects the backend at startup based on `host.DetectPlatform()`. Two implementations:
  `zeBackend` wraps the existing UpdateChecker/SelfUpdater, `gokrazyBackend` probes the
  gokrazy management socket and returns structured unsupported responses.

- Backend identity is a `BackendName` string type (`"ze-self-update"` or `"gokrazy-ab"`),
  not an enum, because the value appears directly in JSON output.

- Build-tagged split: `backend_ze_distro.go` (ze_distro) wraps UpdateChecker/SelfUpdater;
  `backend_ze_appliance.go` (!ze_distro) provides a stripped stub returning "unsupported
  in minimal build". This ensures minimal builds compile without the self-update dependency
  chain while still registering the `ze-self-update` backend name.

- Gokrazy backend probes via HTTP over Unix socket transport rather than importing the
  gokrazy management library. Shared helpers (socket path, auth header) factored into
  `internal/core/gokrazyutil/` to avoid coupling the backend to the gokrazy web proxy.

- Platform detection happens once at startup via `sync.OnceValues(host.DetectPlatform)`.
  Platform does not change at runtime; per-request detection would be wasteful.

- Doctor checks are platform-aware: skip writable-binary warning on gokrazy (AC-9), warn
  if update-check config is present on gokrazy since it is ignored (AC-10).

## Mistakes

- Show and firmware handlers were initially planned in `internal/component/cmd/show/` and
  `internal/component/cmd/update/`, but a concurrent plugin self-containment refactor
  relocated them to `internal/plugins/update-cmd/cmd/`. The spec was never updated with
  the new paths, causing the entire spec to reference files that no longer existed.

- The spec planned a single `backend_ze.go` and `backend_test.go`. The build-tag split
  (`ze_distro` / `!ze_distro`) required two implementation files and two test files.
  Not a mistake per se, but the spec was never updated with the actual file names.

## Patterns Reused

- Factory registration via `init()`: same pattern as traffic backends, firewall backends,
  and plugin registration. Factories map `BackendName` to constructor functions.
- Global accessor with RWMutex: same `Active*()` / `SetActive*()` pattern used elsewhere
  in the codebase for singleton lifecycle management.
- Doctor checks: `checks_platform.go` and `checks_storage.go` pattern of platform-conditional
  diagnostics, consistent with NTP, DNS, and archive destination checks.
- Build-tag conditional compilation: same `ze_distro` / `!ze_distro` split used by
  selfupdate.go itself.

## Files

None recorded.
