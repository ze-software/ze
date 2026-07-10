# 1104 -- startup-resilience

## Context
Motivated by the osvbng comparison refresh: an appliance must boot to a functional
state (CLI up, config committed, forwarding up) even when RADIUS/TACACS/RPKI/BMP/NTP/hub
are unreachable, and converge when they return. An audit of all eight external-service
touchpoints (producer-cited `file:line`) found the STARTUP half of the invariant already
held everywhere -- clients either resolve lazily per request (RADIUS `Exchange`, TACACS
`dial`) or dial inside detached background goroutines with bounded timeouts (RPKI, BMP,
managed hub, NTP worker). The real residual risk was in OnConfigApply callbacks, because
the transaction coordinator enforces a real deadline (`orchestrator.go` "apply timeout"):
a blocking apply is a FAILED COMMIT, worse than a slow boot. Two apply-path weaknesses
plus one missing health surface were fixed.

## Decisions
- NTP reload block: stop-checks BETWEEN serial `ntp.Query` calls in `doSync`, keeping the
  worker handoff synchronous (single clock-writer invariant), over (a) detaching the old
  worker async -- breaks single-writer -- and (b) a cancellable Query -- beevik/ntp v1.5.0
  `Query` takes no context, so wrapping it in a goroutine orphans the query. Raised
  `ApplyBudget` 5->10s to be honest about the residual one-query (~5s) wait.
- authradius latent DNS-on-apply: bound `serverIPs` with ONE shared `context.WithTimeout`
  across all servers (750ms < the plugin's 1s `ApplyBudget`), over a per-server timeout
  (N servers x timeout still overruns the budget) and over a background re-resolution loop
  (adds a goroutine + reload lifecycle for a branch that cannot even be enabled today).
- Managed hub health: a STATELESS config-tree reachability probe (read hub client from
  `config.ExtractHubConfig`, TCP-dial it) registered via `diagnostic.RegisterDoctorCheck`,
  over an in-process connection-state snapshot -- `ze doctor` runs in a SEPARATE process
  from the daemon and `DoctorCheckContext` carries only the parsed config, so a snapshot
  would always read "disconnected" under `ze doctor`. Mirrors `radius/doctor.go`.

## Consequences
- Established Ze pattern for external clients: "detached goroutine + bounded dial + backoff
  + non-blocking event path" (RPKI/BMP/managed) OR "resolve-per-request" (RADIUS). Never
  dial in a constructor or an apply callback. Copy one of the two; do not invent a central
  connection manager.
- Any worker whose reload path JOINS a goroutine doing blocking network I/O inside
  OnConfigApply needs either cancellable I/O or stop-checks between bounded calls, else it
  can exceed the apply deadline and fail an unrelated commit.
- Doctor checks that predict daemon behavior must mirror the daemon's actual selection: the
  managed check probes `Clients[0]` only, because the daemon connects to the first hub
  client block only (`ze_core_start.go` extractManagedClientConfig). "Any hub reachable"
  would false-heal a down primary.
- coa-port remains a wiring gap (parsed at authradius `config.go` but no YANG leaf; the
  parser rejects unknown fields), routed to a separate spec. This spec only bounds the
  latent lookup so the CoA branch is safe whenever it goes live.

## Gotchas
- The synchronous hub `fetchInitialConfig` runs ONLY on first boot with no cached config
  (`ze_core_start.go`); with any cached config the daemon starts immediately and connects
  to the hub in the background. A-4 was stronger than the audit assumed.
- NTP `setClock` is build-tagged (real Settimeofday on Linux, no-op elsewhere) and needs
  `CAP_SYS_TIME`, so no test can drive a successful `doSync` without seaming it. Added
  `ntpQueryFn`/`setClockFn` seams (radius `radiusAdminProbe` convention) to unit-test the
  stop-check and convergence without real network or a privileged clock set.
- A valid stub `*ntp.Response` (passes `Validate`) needs only `Stratum in 1..14`,
  `Time >= ReferenceTime`, `RootDelay/2+RootDispersion < 16s`, `Leap != NotInSync`:
  `{Stratum:2, Time:now, ReferenceTime:now-1min}`.
- Doctor `.ci` codes already existed for rpki/bmp/ntp/radius; the new one is
  `doctor-hub-unreachable`. `ze doctor --json <config>` is static (no daemon), so it proves
  AC-5 deterministically. Inline `server x { address y }` config needs a `;` before `}`
  (only multi-line is newline-terminated) -- a config-parse error there makes `ze doctor`
  exit 1.
- Bare `go test` on `internal/component/doctor` fails 4 SSH/web listener tests for want of
  the `ze_ssh`/`ze_web` build tags; run with the full `make` feature-gate tag set.

## Files
- internal/plugins/ntp/ntp.go (stop-check + ntpQueryFn/setClockFn seams)
- internal/plugins/ntp/register.go (ApplyBudget 5->10)
- internal/plugins/ntp/ntp_test.go (4 tests)
- internal/component/l2tp/plugins/authradius/register.go (shared-deadline serverIPs)
- internal/component/l2tp/plugins/authradius/register_test.go (4 tests, new)
- internal/component/managed/doctor.go + register.go (stateless hub probe, new)
- internal/component/managed/doctor_test.go (new)
- internal/core/diagnostic/codes.go (doctor-hub-unreachable)
- docs/guide/health-checks.md (doctor code table + anchor)
- test/plugin/startup-unreachable-services.ci (new; boot + doctor + reload)
- plan/spec-startup-resilience.md
