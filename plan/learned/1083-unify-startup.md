# 1083 -- unify-startup

## Context
The plugin startup 5-stage handshake (`declare-registration` → `configure` → `declare-capabilities` → `share-registry` → `ready`) was driven by TWO independent implementations that shared no code: the engine's `Server.handleProcessStartupRPC` (barrier-coordinated) and the hub's `SubsystemHandler.completeProtocol` (inline, synchronous, no barrier). They hard-coded the same wire choreography and the same magic method strings in two places; a protocol change had to be applied to both by hand with nothing forcing them to stay in step (DESIGN-REVIEW.md finding 2). The goal was to unify the wire choreography behind one shared stage-driver while preserving every externally observable behavior (internal refactor, not a protocol change).

## Decisions
- Extracted `runStartupHandshake(ctx, startupSink)` as the single wire choreography (read → validate method → respond → send callbacks); both callers delegate to it. Chose generalizing the engine path (a strict superset: it already had the harder barrier feature and a nil-coordinator single-conn mode via ad-hoc sessions) over growing the hub subset or keeping both.
- Injected caller-specific effects through an unexported `startupSink` interface (`onRegistration`, `deliverConfig`, `onCapabilities`, `deliverRegistry`, `onReady`, `onRunning`, `postReady`, `transition`, `conn`) rather than an `if hub {...} else {...}` switch in the driver — keeps the driver caller-agnostic (ai/rules/plugins.md). `engineStartupSink` does the full engine registration set + barrier; `hubStartupSink` harvests commands/schema and delivers nil.
- Modeled config/registry delivery as sink methods (`deliverConfig`/`deliverRegistry`) rather than a payload-getter, because the two callers have genuinely divergent delivery-error policy: engine config-send failure = barrier abort + `startupErr`; engine registry-send failure = log-only/non-fatal; hub both = returned wrapped error. Chose this over forcing one error model on both.
- Dropped the immediate `handlePluginConflict` (coordinator `PluginFailed(realMsg)` + `proc.Stop()`) at the 3 conflict sites; the uniform driver abort (`SendError` + return) plus the existing deferred `rollbackStartupProcess` + defer `PluginFailed("startup incomplete")` produce identical registry/family/proc state and identical plugin-facing error strings. Chose this over having the sink call `handlePluginConflict` before the driver's `SendError`, which would have reversed the mandated SendError-before-Stop ordering (Stop can close the conn and drop the error to the plugin).

## Consequences
- The three `ze-plugin-engine:*` request method strings now live once, as constants in `startup_driver.go`; the two `ze-plugin-callback:*` strings were already single-located in `ipc/rpc.go`. A protocol change touches one place.
- New startup callers implement `startupSink` rather than copying the wire sequence. The barrier is an injected concern (`transition` returns true unconditionally for barrier-less callers), generalizing the engine's pre-existing nil-coordinator mode.
- The hub sink keeps delivering nil config and nil registry by design (the hub `Orchestrator` owns no reactor/config-tree/dispatcher registry). If the hub ever needs real config, that is a new feature behind the same seam, not a driver change.
- Out of scope: whether the hub `Orchestrator` and engine `Server` should be one process model — this unifies only the handshake driver they both call.

## Gotchas
- `subsystem_test.go` calls `handler.completeProtocol(ctx)` directly, so the method name/signature had to stay; it is now a thin delegator to `runStartupHandshake`.
- Sink methods must be UNEXPORTED. The `startupSink` interface is package-private, so exported (capitalized) methods trip `ze-validate`'s "no cross-package non-test caller" wiring check. Lowercase them (idiomatic for a package-private interface). A test sink field named `conn` also collides with the required `conn()` method — rename the field.
- The repo's `no-sprintf-alloc` hook blocks string `+` concatenation and `fmt.Sprintf("%v")` even on the cold startup path; use `textbuf.Buffer` (`tb.Str(...).Err(err).String()`). The pre-existing inline error strings predated the hook.
- `ze-verify` runs `scripts/status/verify_run.go` (`stagesForMode`), NOT `_ze-verify-impl` in the Makefile; and `ze-validate` (validate.py) is a `/ze-review` auxiliary, NOT part of the commit gate (`ze-verify-wiring-docs` = verify_wiring_docs.py is). `SubsystemManager.FindHandler`/`AllCommands`/`AllSchemas` are pre-existing `ze-validate` NOTEs surfaced only because subsystem.go became a changed file; they do not block the commit gate.

## Files
- `internal/component/plugin/server/startup_driver.go` (created) — `runStartupHandshake`, `startupSink`, method-string constants.
- `internal/component/plugin/server/startup_driver_test.go` (created) — hook-order + method-mismatch + abort characterization tests.
- `internal/component/plugin/server/startup.go` — thin `handleProcessStartupRPC` + `engineStartupSink`; deleted `progressThroughStages`/`stageProgression`/`handlePluginConflict`; `deliverConfigRPC`/`deliverRegistryRPC` take ctx.
- `internal/component/plugin/server/subsystem.go` — thin `completeProtocol` + `hubStartupSink`; deleted the inline five-stage sequence.
- `docs/architecture/api/process-protocol.md` — "Shared stage-driver" note + source anchors.
- `ai/DOCS-TO-CODE.md` — regenerated (registers `startup_driver.go`).
