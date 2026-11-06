# Spec: le-session-clean-preserves-active-harnesses

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Development session cleanup can expose active harness evidence to deletion. No shipped daemon behavior changes |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Preserve active sessions across the harnesses supported in this shared checkout
when automatic trim or explicit session cleanup judges directory ownership.
Keep cleanup useful for owners that are proven gone. Record this discovered
defect separately from `spec-tooling-one-off-defects.md`, whose implementation
and closure remain the current session's priority. This skeleton records the
repair owner. It does not approve a cleanup design or start implementation.

The [November 6 recovery record](tooling-one-off-evidence-recovery-2026-11-06.md)
holds the incident observations and retained-artifact location. An automatic
trim log starts at `2026-11-06T07:55:23Z`, counts 63 removed session directories,
and names a permission error for `tmp/session/2026-11-02-121815/`. The active
Codex session's November 5 proof directory and older state became unavailable.
The log does not enumerate all removed directories. The exact remover of each
missing path remains unconfirmed. No evidence establishes that closing another
Codex session triggered the trim.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` and `ai/doors/le-tooling.md` - shared-checkout boundaries.
  → Constraint: protect other sessions' files and preserve the shared checkout.
- [ ] `docs/contributing/running-commands.md` - Scratch files, Session identity,
  and the automatic store trim.
  → Constraint: a dated directory's owner is its `.session-id`, not its name.
- [ ] `docs/architecture/core-design.md` - command engine and before-dispatch hooks.
  → Constraint: preserve registered dispatch and its caller's output and exit code.
- [ ] `ai/rules/evidence.md` - retained verification evidence.
  → Constraint: volatile session paths cannot serve as retained proof.

**Key insights:** allocation, active-owner protection, and durable evidence
retention are separate contracts. The external archive mitigates the incident's
evidence risk without repairing cleanup's owner classification.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/procscan/procscan.go` - CLI detection recognizes Claude names
  and argv path components. It has no Codex-specific detection.
- [ ] `internal/le/le/path/known_session.go` `sessionIDFromContext` and
  `internal/le/le/path/session.go` `resolveSessionID`, `mintSessionID` - an
  explicit Claude session ID returns before the fallback PID cache is created.
  Codex environment IDs are not identity inputs.
- [ ] `internal/le/session/reap.go` `judge`, `pinnedSessions`, `Reap` - protects
  the caller's own ID, IDs present in argv, valid PID pins, and recent Claude
  transcripts. Explicit cleanup removes directories judged dead without the
  trim's additional age floor.
- [ ] `internal/le/scratch/livetrim.go` `sessionProcessVisible`, `trimSessions` -
  automatic trim requires a recognized CLI, then applies the shared judgment
  and a 24-hour floor to the parent directory's mtime. Nested writes do not
  refresh that timestamp.
- [ ] `internal/le/scratch/storetrim.go` `storeTrimBeforeDispatch`,
  `detachedTrim` - an ordinary command or hook can start a due automatic trim.
  The child inherits the spawning command's session environment.
- [ ] `internal/le/hookruntime/lifecycle.go` `hookEndSummary` - SessionEnd writes
  a recovery summary. It does not directly remove session directories.
- [ ] `internal/le/session/scratch.go` `cleanScratch` - explicit scratch cleanup
  removes the current identity's entire directory.

**Behavior to preserve:** keep the caller's own directory, unknown owners, valid
pins, existing Claude protection, and the trim's conservative no-visible-CLI
skip. Preserve cleanup of provably dead owners and explicit own-session clean.

**Behavior to change:** an active supported peer must not become dead solely
because its harness has no Claude process or transcript. Explicit IDs and old
parent mtimes must not defeat active-owner protection.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Shared session judgment | `internal/le/session/reap.go` `Judge`, `judge` | Provides dead-owner decisions to explicit cleanup and trim | extends: both consumers need the same active-harness protection |
| PID identity cache | `internal/le/le/path/known_session.go` `sessionIdentityCache`, `internal/le/session/reap.go` `pinnedSessions` | Protects owners through matching PID/start records | evaluate during design: explicit identity currently bypasses cache creation |
| Liveness process scan | `internal/le/procscan/procscan.go` | Produces process identity, start and CLI facts | evaluate during design: Codex coverage is absent |
| Age and visibility guards | `internal/le/scratch/livetrim.go` `trimLiveness`, `trimSessions` | Limits automatic removal after the shared judgment | preserve: age alone must not establish that an owner is dead |
| Evidence archive | November 6 recovery record | Retains fresh proof outside managed scratch and caches | does not replace the repair: archive durability is separate from session liveness |

## Data Flow (MANDATORY)

### Entry Point
- Automatic before-dispatch trim and explicit `le session clean`.

### Transformation Path
1. Resolve the invoking identity and collect process facts.
2. Read each dated directory's ownership marker and shared liveness evidence.
3. Judge ownership through `session.judge`.
4. Apply each existing consumer's removal guards before removing a directory.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness to le | Session environment, process arguments, PID/start records and transcripts | Source-read. Cross-harness policy remains to be designed |
| Shared judgment to cleanup | `Judge` and `Reap` consumers | Source-read. No corrective implementation or runtime proof |

### Changed-Wire Census

No format change is selected at skeleton stage. Design must census every writer
and reader if it changes ownership markers, PID pins, transcript interpretation,
or a retention marker. Do not create a second owner format without that census.

### Integration Points
- `session.judge` - one ownership judgment for trim and explicit cleanup.
- `scratch.trimSessions` - retain its visibility and age safeguards.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| Shared owner policy | Unvalidated | A design and corrective tests are still owed |
| Registered entry points preserved | Unvalidated | Preserve the current actions and before-dispatch registration |
| Shipped daemon unaffected | Required | Targeted producers are development tooling |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A supported harness exposes evidence sufficient to bind a live process to its session | Existing identity and process producers | A guessed owner can permit deletion or prevent reclamation indefinitely | Inspect actual harness producers before selecting policy | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A guessed Codex process identity protects the wrong owner | Several sessions share one application process | Prove the session-to-process binding and handle unknown ownership conservatively |
| R-2 | A change stops all useful cleanup | Dead directories remain in every control | Require positive dead-owner reclamation controls |
| R-3 | The incident narrative exceeds available evidence | Claim that every missing path or the closing action is identified | Keep log counts, named paths, source exposure and attribution uncertainty separate |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Active sessions lose files, or dead directories fill the disk |
| How is it reverted? | Design must describe restoration through normal reviewed changes. Never delete evidence to undo a failed repair |
| Who else touches this path? | Concurrent sessions, `spec-le-build-cache-cap.md`, and `spec-le-lab-storage-trim.md` share trim producers |

## Wiring Test (MANDATORY -- NOT deferrable)

Proposed test names identify the required paths. Design must finalize their
fixtures before implementation. No cleanup command or test was run for this record.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le session clean` | → | `Reap`, `judge` | `TestSessionCleanupPreservesActiveCodexPeer` |
| Automatic trim before an ordinary command | → | `storeTrimBeforeDispatch`, `trimSessions` | `TestAutomaticTrimKeepsActiveCodexProof` |

## Acceptance Criteria

These proposed criteria need design review before the spec advances.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Active supported peer, explicit session ID, no ID in argv, old parent mtime | Both cleanup paths preserve the peer's directory and bytes | The active owner and unknown owners remain protected |
| AC-2 | Peer proven dead under the selected policy, with trim age floor met | Explicit cleanup and trim reclaim the dead directory through their normal paths | Active or unjudged owners remain protected |
| AC-3 | Process scan or supported owner binding cannot be established | Cleanup reports uncertainty and preserves affected directories | Uncertain owners remain because death is not proved |
| AC-4 | Existing Claude owner, valid PID pin, or unknown ownership marker | Existing protection remains observable | These existing protected owners remain |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionCleanupPreservesActiveCodexPeer` | `internal/le/session/reap_test.go` | AC-1, AC-4 through explicit cleanup | Planned |
| `TestAutomaticTrimKeepsActiveCodexProof` | `internal/le/scratch/livetrim_test.go` | AC-1 with an old parent mtime | Planned |
| `TestSessionCleanupReclaimsProvenDeadPeer` | `internal/le/session/reap_test.go` | AC-2 and useful cleanup | Planned |
| `TestSessionCleanupKeepsUncertainHarnessOwner` | `internal/le/session/reap_test.go` | AC-3 | Planned |

### Boundary Tests (numeric inputs)
Age-floor equality and PID reuse need controls when the design selects its
binding mechanism. Preserve the current floor until an approved design changes it.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-session-clean-live-harnesses` | `test/ui/le-session-clean-live-harnesses.ci` | Built le preserves a live peer's sentinel and reclaims a proven-dead control | Planned. Read `ze-test` before authoring |

## Files to Modify
- `internal/le/session/reap.go` and `reap_test.go` - shared liveness policy and controls.
- `internal/le/scratch/livetrim.go` and `livetrim_test.go` - automatic-path integration and controls.
- `internal/le/le/path/known_session.go`, `session.go`, and `internal/le/procscan/procscan.go` - design must select which identity producers require changes.
- `docs/contributing/running-commands.md` - Scratch files and Session identity contracts after the repair is proven.

## Files to Create
- `test/ui/le-session-clean-live-harnesses.ci` and its native fixture, if the approved functional design uses this proposed carrier.

### Documentation Update Checklist (BLOCKING)
| Area | Applies? | File to update |
|------|----------|----------------|
| Session ownership and cleanup instructions | Yes | `docs/contributing/running-commands.md`, Scratch files and Session identity |
| CLI syntax, marker formats, new dependencies | Unvalidated | Decide during design and census affected consumers |
| Shipped feature, config, API, plugin, wire, RFC, metrics or comparison claims | No | This skeleton selects no shipped behavior changes |
| Existing source anchors | Unvalidated | Derive affected anchors from the final edit set before implementation |

## Implementation Steps

1. **Phase: Wiring** - finalize and write the cleanup-entry controls before any repair.
2. **Phase: Shared owner policy** - inspect supported harness bindings, select one conservative policy, and prove live, dead, unknown and reused-PID outcomes.
3. **Phase: Automatic path and documentation** - prove the registered automatic trigger preserves active peers, then update the documented contract.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `lifetime-ends-before-its-last-reader.md` | 2026-11-06 | `session.judge`, `scratch.trimSessions` expose active Codex owners |

## Known Limitations

This skeleton supplies no cleanup fix or recovered historical bytes. The
existing running-build cache-entry defect remains owned by
`spec-le-cache-trim-spares-running-builds.md`.

## Checklist

- [ ] Inspect supported harness identity producers and select the owner-binding policy.
- [ ] Complete template design checks, integration and documentation tables before advancing from skeleton.
- [ ] Prove each proposed acceptance criterion with the approved wiring and discrimination controls.
- [ ] Run testing in the main checkout, serialized in the main thread.
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` only when closure starts.
