# 538 -- Report Bus

## Context

Ze had no central place for subsystems to report user-facing operational issues. BGP prefix warnings lived on a per-peer field queried via a noun-first `ze-bgp:warnings` RPC that was implemented but never YANG-registered (`validate-commands` flagged it as an orphan). There was no equivalent for errors, and other subsystems (interface, plugin, healthcheck, config) had no query path at all. Operators saw operational issues only in logs, login banners, or per-object status fields. The goal was a single cross-cutting push API every subsystem could use, backed by two verb-first user-facing RPCs (`ze show warnings`, `ze show errors`), with BGP prefix warnings and NOTIFICATIONs as the day-one producers and config commit failures added once the transaction protocol landed.

## Decisions

- **Push API over pull.** Errors are inherently event-style (there is nothing to "pull" after a NOTIFICATION is sent or a commit fails), so pulling from per-subsystem `Warnings()` implementations would have required a separate error path. A single push API (`report.RaiseWarning` / `RaiseError` / `ClearWarning` / `ClearSource`) handles both cleanly, enables lock-free reads from the show handler, and keeps the producer-side mental model uniform.
- **Concrete package functions over a `Reporter` interface.** YAGNI: there is one source today (BGP at ship, config transactions on day two). Introducing an interface for a single concrete implementation is speculative complexity. If external plugin authors need to push entries, a `pkg/ze/report/` re-export is the future spec (deferred).
- **State-based warnings, event-based errors** distinguished by severity with different storage: warnings go into an active-set map keyed by `(Source, Code, Subject)` with dedup + oldest-by-Updated eviction at cap; errors go into a bounded ring buffer with no dedup. Producers MUST pick the right severity; the bus does not auto-promote. The contract is "did something actually fail? error, otherwise warning."
- **Internal-only pkg path** (`internal/core/report/`) alongside the existing cross-cutting registries (`family`, `metrics`, `env`, `clock`). Avoids coupling to any subsystem.
- **Atomic pointer store** so `reset()` and `resetWithCaps()` remain race-safe with concurrent `Raise` / `Clear` / `snapshot` operations. Readers take a load-once, callers get a consistent view without serializing on a global mutex.
- **Length-bounded inputs** (Source/Code 64 bytes, Subject 256, Message 1024, Detail 16 keys) rejected at the boundary. Protects the bus from buggy or malicious producers pushing oversized entries. Env-configurable caps (`ze.report.warnings.max`, `ze.report.errors.max`) clamped to safe upper bounds to prevent operator typos from causing OOM.
- **Config commit error producers piggyback on the existing orchestrator publish sites.** `publishAbort`, `publishRollback`, and `writeConfigFile` already own the moments when verify fails, apply fails, and config save fails respectively. Adding a `report.RaiseError` call next to each stream-event emission kept all three paths in one file and avoided building a second event path.
- **Python SDK extension for .ci test coverage.** The existing `test/scripts/ze_api.py` handled `deliver-batch`/`deliver-event`/`filter-update` RPCs but not `config-verify`/`apply`/`rollback`. Adding `on_config_verify` / `on_config_apply` / `on_config_rollback` registration plus dispatcher support (about 40 lines) let Python test plugins exercise the engine-side RPC bridge (`config_tx_bridge.go`) end-to-end, unblocking `show-errors-config-abort.ci` without building a second Go test-helper plugin.
- **One .ci for the commit-aborted path, unit tests for the other two.** Writing a .ci test for `commit-rollback` needs a plugin that accepts verify and rejects apply mid-transaction while a second plugin accepts apply; the current single-plugin Python pattern cannot toggle per phase. Writing a .ci test for `commit-save-failed` needs read-only filesystem handling in the test runner. Unit tests `TestCommitRollbackRaisesReportError` and `TestCommitSaveFailedRaisesReportError` in `orchestrator_test.go` cover both producer paths deterministically via the testGateway fake and a `SetConfigWriter` stub; the end-to-end .ci tests are deferred with explicit blockers.

## Consequences

- Operators now have one command (`ze show warnings` / `ze show errors`) that queries every subsystem's operational issues in one place, instead of scattered logs, banners, and noun-first BGP-only RPCs.
- Adding a new producer costs one `import "internal/core/report"` plus one call per raise/clear point. There is no registration, no interface implementation, no plumbing. Future subsystems (ifacenetlink, plugin supervisor, healthcheck, RPKI cache) can plug in the same day.
- The bus is the single source of truth for the login banner too, so adding a new warning code automatically surfaces in the banner without touching `loader.go`.
- The transaction orchestrator and the report bus are still independent concerns. The orchestrator publishes stream events for engine-internal coordination (the state machine needs those); `report.RaiseError` is an additive call for operator visibility. A subsystem that does not care about `show errors` sees no overhead.
- The `validate-commands` tooling baseline is zero orphans (after the iface/monitor import fixes bundled into the same spec), so future mismatches surface immediately instead of hiding in a noisy backlog.
- `ze_api.py` now supports the config transaction protocol for Python test plugins. Any future `.ci` test that needs to exercise config-verify/apply/rollback callbacks can use the same pattern as `show-errors-config-abort.ci`.

## Gotchas

- `validate-commands` "orphans" are not all stale registrations. They can be missing YANG, missing imports in the tooling itself, or implemented-but-not-wired code. Investigating each orphan against its actual handler and the `ze-peer-cmd.yang` precedent (which has the `peer-detail` pattern of paired registrations) was what turned this spec from a "delete dead code" cleanup into a full bus design.
- `ExtractConfigSubtree` wraps the leaf data in its path structure. The plugin receiving `ConfigSection.Data` for root `"bgp"` sees `{"bgp":{"router-id":"..."}}` at the top level, not `{"router-id":"..."}`. The first `.ci` test I wrote for `show-errors-config-abort.ci` got this wrong and asserted on the flat shape. Related fix is in the test's JSON parsing.
- The Python ze_api `declare_wants_config` does not exist; the method is `declare_config(pattern)`. The first iteration of the Python test plugin called the wrong name, the plugin crashed at Stage 1, and the engine logged `rpc startup: read registration failed ... mux conn closed`. The error message was unhelpful. Adding a `declare_wants_config` alias would be a nice-to-have.
- The Python test plugin's `on_config_verify` handler is invoked on EVERY reload verify. For tests that want to accept some reloads and reject others, the handler needs internal state. The abort test works because it rejects every verify, and the daemon's first config load goes through `configure` (Stage 2) not `config-verify` (reload).
- Empty-bus `show errors` + `daemon shutdown` + `wait_for_shutdown` hangs in the .ci runner. Cause unknown. Affects any empty-bus functional test pattern and is tracked separately.
- The `ze-show:warnings` JSON returns `{"warnings": [...], "count": N}` and `ze-show:errors` returns `{"errors": [...], "count": N}` — symmetric shapes but different top-level keys. Clients dispatching both commands must switch on the verb, not rely on a single `entries` field.
- The spec file itself was created untracked in an earlier session and almost got deleted without commit (the exact scenario the `lg-overhaul` mistake log entry warns about). Rescued on 2026-04-08 by filling the audit tables first and then running the two-commit sequence. When a spec file is found untracked with significant code already committed, fill the audit before anything else.

## Files

### Core package
- `internal/core/report/report.go` -- Severity/Issue types, package-level store with atomic pointer, Raise/Clear/snapshot API, env-configurable caps
- `internal/core/report/report_test.go` -- 30 unit tests (concurrent, boundary, length-bounded input, JSON round-trip)

### BGP producers
- `internal/component/bgp/reactor/session_prefix.go` -- prefix-threshold and prefix-stale raise/clear, helpers for notification + session-dropped paths
- `internal/component/bgp/reactor/peer_stats.go` -- notification-sent/received raise via `raiseNotificationError`
- `internal/component/bgp/reactor/peer_run.go` -- session-dropped raise on non-NOTIFICATION Established->Idle transitions
- `internal/component/bgp/reactor/peer.go` -- old `PrefixWarnings` field, `SetPrefixWarned`, `clearPrefixWarned`, `PrefixWarnedFamilies`, `prefixWarnedMap` all deleted
- `internal/component/bgp/reactor/reactor_api.go` -- `PeerInfo.PrefixWarnings` population removed
- `internal/component/plugin/types_bgp.go` -- `PeerInfo.PrefixWarnings` field deleted

### Config transaction producers
- `internal/component/config/transaction/orchestrator.go` -- `publishAbort`, `publishRollback`, `writeConfigFile` push `commit-aborted`, `commit-rollback`, `commit-save-failed` alongside stream events
- `internal/component/config/transaction/orchestrator_test.go` -- `TestCommitAbortRaisesReportError`, `TestCommitRollbackRaisesReportError`, `TestCommitSaveFailedRaisesReportError` + `findReportError` helper

### Query handlers
- `internal/component/cmd/show/show.go` -- registers `ze-show:warnings` and `ze-show:errors`; removed `ze-show:bgp-warnings`
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` -- top-level `warnings` and `errors` containers
- `internal/component/cmd/show/show_test.go` -- handler unit tests covering empty/populated snapshots

### Banner migration
- `internal/component/bgp/config/loader.go` -- `collectPrefixWarnings` reads from `report.Warnings()` filtered by source/subject
- `internal/component/bgp/config/loader_test.go` -- banner tests seed the bus via `report.RaiseWarning` instead of peer fields

### Legacy deletion
- `internal/component/bgp/plugins/cmd/peer/peer_warnings.go` -- entire file removed
- `internal/component/bgp/plugins/cmd/peer/peer_warnings_test.go` -- entire file removed
- `internal/component/bgp/plugins/cmd/peer/peer_test.go` -- removal comment explaining the deleted `ze-bgp:warnings` registration

### Test infrastructure
- `test/scripts/ze_api.py` -- `on_config_verify`, `on_config_apply`, `on_config_rollback` handler registration + dispatcher wiring
- `test/plugin/show-warnings.ci` -- AC-8 prefix-stale via config
- `test/plugin/show-errors-sent.ci` -- AC-9 NOTIFICATION sent via teardown
- `test/plugin/show-errors-received.ci` -- AC-10 NOTIFICATION received
- `test/plugin/show-errors-config-abort.ci` -- AC-21 verify rejection via Python test plugin (new in final pass)

### Tooling
- `scripts/docvalid/commands.go` -- added missing imports for iface/monitor handlers so the orphan baseline is zero

### Documentation
- `docs/features.md` -- ze show warnings / ze show errors in operator commands
- `docs/guide/operational-reports.md` -- new operator guide page (severity semantics, day-one vocabulary, triage workflow)
- `docs/guide/command-reference.md` -- both new show commands
- `docs/architecture/api/commands.md` -- RPC contract + push API rationale
- `docs/architecture/core-design.md` -- `internal/core/report/` added to cross-cutting registries
- `docs/comparison.md` -- operator-visible warning/error query vs bird/frr
