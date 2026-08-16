# Spec: Plugin protocol versioning and a real SDK boundary (DESIGN-REVIEW finding 7)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-unify-startup (two Stage-1 handshake sites share the version check; closed, learned 1083) |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. ~~`DESIGN-REVIEW.md` finding 7 ("The public SDK boundary is not real") and its verification notes~~ (2026-07-22: ephemeral session artifact, never committed; the finding is restated inline in this spec. Both defects re-verified still open 2026-07-22: `*selector.Selector` leaks at `bridge.go,303` and `sdk_engine.go,31`; no `ProtocolVersion` anywhere in `pkg/plugin/`)
4. `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk.go`, `pkg/plugin/rpc/bridge.go`,
   `pkg/plugin/sdk/sdk_engine.go`, `internal/component/plugin/server/{startup,subsystem}.go`

## Task

Close DESIGN-REVIEW finding 7, using the corrected framing established during the finding
review (the original finding's build-failure claim was disproved; the real defects are
sharper). Two independent, verified problems:

1. **The wire protocol has no version field (silent cross-binary break).** No
   `ProtocolVersion` / `apiVersion` field exists anywhere in `pkg/plugin/` (broad non-test
   search empty; only Stage-5 transport "negotiation" exists, `pkg/plugin/rpc/enums.go`
   `EventKindNegotiated`, which is not a protocol version). `DeclareRegistrationInput` (Stage 1,
   `types.go`) carries no version. Any struct change in `pkg/plugin/rpc/types.go` is a
   silent mis-decode for a separately-compiled plugin process not rebuilt in lockstep. This
   bites any out-of-process plugin binary, in-tree or not.

2. **The exported "public" SDK surface leaks un-nameable internal types.** `pkg/plugin/sdk`
   transitively depends on 5 internal packages (`internal/core/stringsx`, `.../textbuf`,
   `.../selector`, `internal/component/plugin/ipc`, `internal/core/env`; `go list -deps
   ./pkg/plugin/sdk`), and `*internal/core/selector.Selector` appears in EXPORTED signatures:
   `pkg/plugin/rpc/bridge.go` (`UpdateRouteSelHandler`), `pkg/plugin/sdk/sdk_engine.go,32`
   (`Plugin.UpdateRouteSel` / `UpdateRouteSelWithMeta`). An out-of-tree module CAN blank-import
   the SDK (verified: it builds), but CANNOT import `internal/core/selector` ("use of internal
   package ... not allowed", verified), so it cannot construct the argument and those exported
   methods are uncallable out-of-tree. A public API with un-nameable parameter types is not a
   real boundary.

Goal: (1) add a negotiated protocol version at the Stage-1 handshake so a version mismatch is
rejected loudly instead of silently mis-decoding; (2) make the advertised public boundary
honest, so no exported `pkg/plugin/**` symbol names an `internal/` type, with a mechanical
guard preventing regression.

**Corrected-framing note (verified during review):** The original finding said an out-of-tree
author "cannot actually build against `pkg/` alone." That is FALSE: Go's internal rule checks
where the import statement lives, and every `sdk -> internal` edge is inside the ze module, so
an external module builds cleanly (empirically confirmed). The true defect is the exported
internal-typed API (above), not a build failure. This spec fixes the real defect.

**Explicitly out of scope (referenced, not duplicated):**
- Unifying the two daemon startup topologies (`server/startup.go` vs `server/subsystem.go`):
  owned by `spec-unify-startup.md`; this spec adds the version check at both Stage-1 sites (or
  the single unified site once that lands).
- The `pkg/plugin/rpc` command-envelope wire types: owned by `spec-unify-response-envelope.md`
  (its A-2 already documents `pkg/plugin/rpc` as the cross-process wire layer).

### Post-wave corrections (2026-07-10)

Two notes from the 2026-07 implementation wave (verified against current code):

1. **The ProtocolVersion handshake must coexist with new write-timeout wire behavior.**
   `pkg/plugin/rpc/conn.go` now applies a default 30s write deadline when the context carries
   none (`defaultWriteDeadline`, conn.go; applied in `writeAppended`, conn.go,
   :309) and, on transports without `SetWriteDeadline` (stdio, SSH channels), arms a
   fail-fast write watchdog that closes the connection on a stalled write (`fireWatchdog`,
   conn.go; transport selection at conn.go; hook `SetWriteWatchdogHook`,
   conn.go; counter `ze_plugin_write_watchdog_total` wired in
   `internal/component/plugin/server/server.go`). Consequences for this spec: the
   Stage-1 declare-registration write and the engine's version-rejection diagnostic write are
   both subject to this deadline/watchdog, and AC-2's "clear error, not a hang" now has an
   interacting mechanism, since the transport already converts some write stalls into
   fail-fast closes. The version design and its tests must not attribute a watchdog-triggered
   close to a version mismatch (or vice versa).

2. **`ze-plugin-boundary-check` does NOT satisfy AC-5, despite the name.** The verify gate
   `ze-plugin-boundary-check` (Makefile:287, :294, :319) runs
   `scripts/checks/plugin_process_boundary.go`, which guards same-process-effect direct calls
   that bypass DirectBridge/DispatchCommand (its header, plugin_process_boundary.go). It
   does not inspect exported `pkg/plugin/**` signatures for `internal/` types. AC-5 still
   requires its own new mechanical guard; name it distinctly to avoid collision with the
   existing gate.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/api/process-protocol.md` - the 5-stage plugin handshake and wire framing
  → Decision: Stage 1 is plugin-initiated declare-registration; it is the earliest structured message and the natural version-negotiation point.
  → Constraint: the wire framing must stay backward-parseable enough to emit a version-mismatch error rather than a raw decode failure.
- [ ] `ai/rules/architecture.md` - tier axes; where `pkg/` sits relative to `internal/`
  → Constraint: `pkg/` is the advertised external surface; a public package exposing internal types defeats the tier intent.
- [ ] `ai/rules/plugins.md` - the plugin SDK/protocol contract
  → Constraint: out-of-process plugins interact only via the wire protocol and the exported SDK; both must be self-describing.

**Key insights:** the wire boundary (separately-compiled binaries) and the source boundary
(what `pkg/` exports) are two different contracts; finding 7 conflates them. Version fixes the
first; API cleanup + a guard fixes the second.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `pkg/plugin/rpc/types.go` - `DeclareRegistrationInput` (Stage 1, :27-43) carries no protocol
  version; no version field anywhere in the file.
- [ ] `pkg/plugin/sdk/sdk.go` - Stage 1 declare-registration (:332), preceded by `SendAuth`
  (:212); Stages 2-5 follow. No version is sent.
- [ ] `pkg/plugin/rpc/bridge.go` - imports `internal/core/selector` (:14);
  `UpdateRouteSelHandler` (:294) and `DirectBridge.UpdateRouteSel` (:303) take `*selector.Selector`.
- [ ] `pkg/plugin/sdk/sdk_engine.go` - imports `internal/core/selector` (:14); `Plugin.UpdateRouteSel`
  (:27) / `UpdateRouteSelWithMeta` (:32) are the in-process-only typed fast path (comment :24-26)
  exposing `*selector.Selector`, with a `sel.String()` fallback (:36) external plugins already
  reach via the string `UpdateRoute` (:20).
- [ ] `internal/component/plugin/server/startup.go` - engine Stage-1 read + `registrationFromRPC`.
- [ ] `internal/component/plugin/server/subsystem.go` - the SECOND engine Stage-1 read site.

**Behavior to preserve:** (unless user explicitly said to change)
- In-tree plugins (rebuilt in lockstep) keep working across all 5 stages unchanged.
- The in-process DirectBridge fast path stays available to in-process plugins (it just stops
  being part of the advertised public surface if option (a) is chosen).
- External plugins keep the string `UpdateRoute` / `UpdateRouteWithMeta` API unchanged.
- The wire protocol JSON framing and existing message types stay compatible for a matching version.
- `SendAuth` / TLS handshake behavior unchanged.

**Behavior to change:** (only if user explicitly requested)
- Stage 1 carries a plugin protocol version; the engine rejects an incompatible version with a
  specific diagnostic instead of silently mis-decoding later structs.
- No exported `pkg/plugin/**` symbol names an `internal/` type; a mechanical guard enforces it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A plugin process connects, runs `SendAuth` (`sdk.go`), then sends Stage-1
  declare-registration (`DeclareRegistrationInput`, `sdk.go`) to the engine.
- Format at entry: a JSON `DeclareRegistrationInput` over the process transport.

### Transformation Path
1. Plugin authenticates via `SendAuth`, then emits Stage-1 `DeclareRegistrationInput`.
2. Engine reads Stage 1 at `server/startup.go` OR `server/subsystem.go` (two topologies) and
   converts via `registrationFromRPC`.
3. Today: no version is present, so a struct-shape drift decodes into wrong/zero fields silently.
4. Separately, at runtime an in-process plugin may call `Plugin.UpdateRouteSel(*selector.Selector)`
   (`sdk_engine.go`) into `DirectBridge.UpdateRouteSel` (`bridge.go`) - an exported path
   naming an internal type.
5. An out-of-tree importer builds against the SDK (allowed) but cannot name `*selector.Selector`
   (internal import rejected), so step 4's exported API is uncallable for it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin process ↔ engine (wire) | JSON `DeclareRegistrationInput`, no version | [ ] |
| Separately-compiled binaries | `pkg/plugin/rpc/types.go` structs exchanged, no version guard | [ ] |
| External importer ↔ exported SDK API | `*internal/core/selector.Selector` in exported signatures (un-nameable out-of-tree) | [ ] |
| pkg/ ↔ internal/ | `pkg/plugin/sdk` transitively imports 5 internal packages | [ ] |

### Integration Points
- `pkg/plugin/rpc.DeclareRegistrationInput` and a new `ProtocolVersion` constant - the version carrier.
- `pkg/plugin/sdk` Stage-1 send path - where the plugin declares its version.
- `server/startup.go` / `server/subsystem.go` `registrationFromRPC` - where the engine validates it.
- `pkg/plugin/rpc/bridge.go` / `pkg/plugin/sdk/sdk_engine.go` - the exported internal-typed surface.
- `make ze-precommit-verify` - where the new boundary guard runs.

### Architectural Verification
- [ ] No bypassed layers (version checked at the handshake, before typed struct exchange)
- [ ] No unintended coupling (exported public API names no internal type)
- [ ] No duplicated functionality (one version constant; guard reuses existing check tooling)
- [ ] Zero-copy preserved where applicable (in-process DirectBridge fast path retained)
- [ ] Registration over hardcoding — the version and the boundary guard are declared once and
  discovered by the handshake / verify pipeline, not special-cased per plugin
  (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All in-tree plugins rebuild in lockstep with the engine, so a mandatory version field does not break them | in-tree plugins compiled with the engine | A staged rollout needs a tolerant/optional version first | build + full plugin `.ci` suite after adding the field | unvalidated |
| A-2 | `*selector.Selector` is the only internal type in an exported `pkg/plugin/**` signature | `go list`/grep found selector; only reviewed selector + ipc | Other leaks exist; the guard must find them all | the AC-5 boundary guard run across all pkg/plugin exported signatures | unvalidated |
| A-3 | The in-process `UpdateRouteSel*` fast path has only in-tree callers, so moving it to an internal seam breaks nobody external | `sdk_engine.go` comment "in-process plugins use this"; external plugins use the string path | An external caller relied on it (it could not, being un-nameable) | grep callers of `UpdateRouteSel`/`UpdateRouteSelWithMeta` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Version check added at only one of the two Stage-1 sites (startup vs subsystem) | one topology rejects, the other silently accepts | Add to both, or gate on `spec-unify-startup`; a test drives both topologies |
| R-2 | Strict version match is too rigid for future minor-compatible changes | every minor bump breaks all plugins | Decide match policy (exact vs min-supported range) as a Key Design Decision |
| R-3 | Moving the exported internal-typed API breaks in-process plugin callers | compile break in in-tree plugins | Relocate to an internal in-process seam the in-tree callers import (allowed); update callers |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a plugin declares an incompatible protocol version at Stage 1 | → | engine rejects with a specific diagnostic, no silent decode | `test/plugin/plugin-version-mismatch.ci` |
| an out-of-tree module calls every advertised public SDK method | → | no exported signature names an internal type | `TestPublicPkgAPIHasNoInternalTypes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin and engine at the same protocol version | Stage-1 declare-registration carries a `ProtocolVersion`; handshake proceeds normally through Stage 5 |
| AC-2 | Plugin declares a protocol version the engine does not support | Engine rejects at Stage 1 with a specific, logged diagnostic BEFORE any later typed struct is trusted; the plugin sees a clear error, not a hang or silent mis-decode |
| AC-3 | Version check present at BOTH engine Stage-1 sites (or the unified site) | `startup.go` and `subsystem.go` topologies both enforce it; a test drives both |
| AC-4 | Inspect the advertised public `pkg/plugin/**` exported API | No exported func/method/type/handler signature names a type from any `internal/` package (`*selector.Selector` no longer appears in the public surface) |
| AC-5 | Run `make ze-precommit-verify` | A mechanical guard fails the build if any exported `pkg/plugin/**` signature references an `internal/` type, preventing regression |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs a plugin binary built against an older protocol | Stage-1 version check -> clean rejection | `test/plugin/plugin-version-mismatch.ci` |
| 2 | (external author) builds and calls the public SDK | import -> exported API (no internal types) | `TestPublicPkgAPIHasNoInternalTypes` |
| 3 | a matching-version plugin registers | Stage 1..5 unchanged | existing plugin handshake `.ci` still green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestProtocolVersionNegotiated` | `internal/component/plugin/server/startup_test.go` | matching version proceeds; mismatch rejected with diagnostic | |
| `TestProtocolVersionBothTopologies` | `internal/component/plugin/server/subsystem_test.go` | both Stage-1 sites enforce the version | |
| `TestPublicPkgAPIHasNoInternalTypes` | `pkg/plugin/boundary_test.go` | no exported pkg/plugin signature names an internal type | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ProtocolVersion | 1 .. current | current | 0 (unset, rejected) | future/unknown rejected with diagnostic |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-version-mismatch` | `test/plugin/plugin-version-mismatch.ci` | old-protocol plugin rejected cleanly | |
| `plugin-version-match` | `test/plugin/plugin-version-match.ci` | matching-version plugin registers and runs | |

### Interop Tests (MANDATORY for protocol features)
Not applicable to peer wire protocols (BGP/IPsec/L2TP): this is the ze plugin RPC protocol, not
a peer-facing protocol. The functional `.ci` tests above are the cross-binary protocol gate.

### Future (if deferring any tests)
- A true out-of-tree example plugin module under `test/` that exercises the public SDK end to
  end is a strong follow-on if external plugins become a supported product.

## Files to Modify
- `pkg/plugin/rpc/types.go` - add a `ProtocolVersion` constant and field on
  `DeclareRegistrationInput` (Stage 1).
- `pkg/plugin/sdk/sdk.go` - send `ProtocolVersion` at Stage 1; surface the engine's rejection.
- `internal/component/plugin/server/startup.go` - validate the version in `registrationFromRPC`
  (or before it); reject on mismatch.
- `internal/component/plugin/server/subsystem.go` - the second Stage-1 site; same check.
- `pkg/plugin/rpc/bridge.go` - move the `*selector.Selector` typed fast path out of the exported
  public surface (option a) so no exported signature names an internal type.
- `pkg/plugin/sdk/sdk_engine.go` - relocate `UpdateRouteSel`/`UpdateRouteSelWithMeta` to an
  internal in-process seam the in-tree callers import; keep the string `UpdateRoute` public.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Boundary guard in verify | [ ] | `scripts/checks/pkg-no-internal-in-api` or `pkg/plugin/boundary_test.go`, wired into `make ze-precommit-verify` |
| Functional test for handshake | [ ] | `test/plugin/plugin-version-mismatch.ci`, `plugin-version-match.ci` |
| Prometheus counter (version rejections) | [ ] | plugin server telemetry, if a rejection metric is wanted |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` (version handshake) |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` plugin boundary section |
| 16 | Changed source referenced by doc anchors? | [ ] | grep `docs/` for `source: .../pkg/plugin/rpc/types.go`, `sdk.go` |

## Files to Create
- `test/plugin/plugin-version-mismatch.ci` - proves an incompatible plugin is rejected cleanly.
- `test/plugin/plugin-version-match.ci` - proves a matching plugin registers and runs.
- `pkg/plugin/boundary_test.go` - the mechanical no-internal-types-in-public-API guard.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate |
| 6. Full verification | `make ze-precommit-verify` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — add the `ProtocolVersion` constant + Stage-1 field and
   the boundary guard skeleton; write `plugin-version-mismatch.ci` and
   `TestPublicPkgAPIHasNoInternalTypes` expecting the end state (both fail now).
   - Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/boundary_test.go`, the `.ci` files.
   - Verify: version test fails (no field), boundary test fails (selector leak present).
2. **Phase: Version negotiation** — plugin sends the version; both engine Stage-1 sites validate
   and reject with a diagnostic; matching version proceeds.
   - Tests: `TestProtocolVersionNegotiated`, `TestProtocolVersionBothTopologies`,
     `plugin-version-mismatch.ci`, `plugin-version-match.ci`.
3. **Phase: Boundary cleanup** — relocate the internal-typed `UpdateRouteSel*` fast path out of
   the public surface; update in-tree callers; boundary guard passes.
   - Tests: `TestPublicPkgAPIHasNoInternalTypes`; existing in-process plugin tests still green.
4. **Phase: Guard in verify** — wire the boundary guard into `make ze-precommit-verify`.
5. **Full verification** → `make ze-precommit-verify`.
6. **Complete spec** → learned summary `plan/learned/NNN-sdk-boundary-versioning.md`; two commits.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Version mismatch rejected before any typed struct is trusted; both topologies enforce it |
| Data flow | Version checked at the handshake, not after decode |
| Registration over hardcoding | One version constant + one guard, discovered by handshake/verify; no per-plugin special case |
| Rule: no-layering | Old un-versioned path removed; internal-typed public API fully relocated, not duplicated |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Version field + check | `plugin-version-mismatch.ci` rejects; `plugin-version-match.ci` passes |
| No internal types in public API | `TestPublicPkgAPIHasNoInternalTypes` passes; grep for `selector.` in exported pkg/plugin signatures empty |
| Guard in verify | `make ze-precommit-verify` fails on a deliberately-reintroduced leak |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | A malformed/absent version is rejected, not defaulted into "compatible" |
| Error leakage | The rejection diagnostic does not leak auth token or internal path detail |

### Failure Routing
| Failure | Route To |
|---------|----------|
| One topology accepts a bad version | Add the check to both sites (Phase 2, R-1) |
| In-process caller breaks after relocation | Point it at the internal seam (Phase 3, R-3) |
| Guard flags a leak beyond selector | Fix that signature too (A-2) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (original finding) out-of-tree author cannot build against pkg/ | External module builds cleanly; the leak is at the call site | Empirical external-module build during finding-7 review | Corrected the spec framing to the real defect |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->
- The wire boundary (separately-compiled binaries) and the source boundary (what `pkg/`
  exports) are distinct contracts; finding 7 conflated them. Versioning fixes the first; API
  cleanup fixes the second. Neither remedy alone suffices.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Relocate the in-process `UpdateRouteSel*` internal-typed API out of public `pkg/` (option a) | (b) move the whole SDK under `internal/` and drop the public claim; (c) relocate `selector` to `pkg/` | (a) keeps the in-process fast path while making the public surface honest; (c) has a large blast radius (selector is used across internal); (b) is the honest-retreat fallback if external plugins are never intended |
| Version at Stage-1 declare-registration | Version in `SendAuth`, or a separate Stage-0 hello | Stage 1 is the first structured message and matches the finding's proposal; avoids a new stage |
| Match policy decided in design | strict-equality vs min-supported-range | Range tolerates minor-compatible changes; strict is simpler; pick per rollout needs (R-2) |

## Known Limitations
- This spec does not make `pkg/plugin/sdk` free of ALL internal imports (it may still use
  `internal/component/plugin/ipc` for transport as implementation); it only guarantees no
  internal type appears in the EXPORTED signatures and that the wire protocol is versioned.
  A fully self-contained `pkg/` (zero internal imports) is a larger, separate effort.
- If option (b) is chosen (SDK under `internal/`), AC-4/AC-5 are satisfied trivially and the
  version work still applies to any separately-compiled in-tree plugin process.

## Implementation Summary

### What Was Implemented
- [filled during /implement]

### Bugs Found/Fixed
- [any silent-decode case surfaced by adding the version]

### Documentation Updates
- [process-protocol.md version handshake + plugins.md, or "None" with grep evidence]

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`pkg/*`, `internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (one version constant; reuse existing check tooling)
- [ ] No speculative features (two named defects only)
- [ ] Explicit > implicit behavior (loud version rejection; honest public surface)
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (ProtocolVersion)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests N/A (plugin RPC protocol, not a peer wire protocol; `.ci` is the gate)

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Write learned summary to `plan/learned/NNN-sdk-boundary-versioning.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-review-sdk-boundary-versioning.md`
