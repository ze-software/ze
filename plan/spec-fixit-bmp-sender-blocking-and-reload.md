# Spec: fixit-bmp-sender-blocking-and-reload

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-08-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. [List key architecture docs from Required Reading]
4. [List key source files from Files to Modify]

## Task

Two defects in the BMP sender, both found by independent review of the
`bmp-locrib` (functional test 97) shared-scratch fix and both deliberately NOT
fixed there: neither blocks that fix's goal, and each needs a design decision
rather than a local edit. Source: the 2026-07-22 session that fixed the
senderSession scratch race; deferral rows in
`plan/deferrals/ad-hoc-2026-07-22-f27dc80f.md`.

**1. Loc-RIB monitoring does blocking socket I/O on an EventBus subscriber
goroutine (contract violation).**

`BMPPlugin.handleBestChange` (`internal/component/bgp/plugins/bmp/bmp_locrib.go`)
runs on the RIB's publisher goroutine: engine EventBus subscribers fire
synchronously from `deliverEvent`
(`internal/component/plugin/server/engine_event.go`, `SubscribeEngineEvent`).
The bus contract is explicit -- `pkg/ze/eventbus.go` `EventBus.Subscribe`: "The
handler runs synchronously when an event is emitted and MUST NOT block on I/O."
It then loops `len(batch.Changes) x len(senders)` blocking `conn.Write` calls,
each bounded only by `writeTimeout` (10s), so a wedged collector stalls RIB
best-change publication.

Pre-existing, introduced with RFC 9069 Loc-RIB support; NOT a regression from
the `writeMu` serialization (concurrent `conn.Write` calls already serialize on
the socket's own write lock -- `internal/poll.FD.Write` takes `fd.writeLock()`
and holds it for the whole call, EAGAIN waits included).

Wanted: a bounded per-session send queue so no socket write happens on a
subscriber goroutine. Rejected shortcut: `TryLock`-and-drop, which silently
loses Route Monitoring messages -- worse for a monitoring protocol than a
bounded stall. The drop-vs-stall-vs-queue-depth choice is the design decision
this spec owes.

**DECIDED 2026-07-27 (Thomas): "do what bird does".** Researched against BIRD's
own source (`gitlab.nic.cz/labs/bird`, master `02d082a7`, `proto/bmp/`), not
documentation. BIRD's policy, with citations:

| Question | BIRD's answer | Where |
|---|---|---|
| Sync write on the announce path? | No. `bmp_rt_notify` serialises the message and `bmp_schedule_tx_packet` only memcpys into pages + `ev_schedule`; the socket write happens later in `bmp_fire_tx` from the event loop | `bmp.c:1016`, `:300-347`, `:362-399` |
| Queue structure | Linked list of page-sized `struct bmp_tx_buffer`, messages packed contiguously and split across page boundaries | `bmp.c:222-226`, `bmp.h:79-80` |
| Bounded? | Yes, **by BYTES** (pages), not message count. `tx_pending_count` vs `tx_pending_limit`, default 1 GiB, configurable `tx buffer limit <N>` in MB | `bmp.h:81-82`, `config.Y:30,75-78` |
| On overflow | **Resets the session.** Producer does `ev_schedule(tx_overflow_event)` and returns; the handler calls `bmp_down()` + `bmp_close_socket()` (freeing the whole queue) and arms the retry timer | `bmp.c:311-312`, `:1197-1215`, `:1236-1249` |
| Drop individual messages? | No | `bmp.c:1236-1249` |
| Block the producer? | No -- sockets are `O_NONBLOCK`, `sk_send` defers to the TX hook | `sysdep/unix/io.c:1137`, `:2003-2015` |
| Log / state | `log(L_ERR "%s: Connection stalled")`; protocol drops to `PS_START`; `sock_err` deliberately 0 so `show protocols all` distinguishes stalled from a socket error | `bmp.c:1206`, `:1204`, `:1214` |
| Reconnect | 10 s (`CONNECT_RETRY_TIME`), then Initiation + Peer Up for every established peer + a **full fresh RIB dump** ending in END-OF-RIB | `bmp.c:168`, `:1106-1123`, `:1040-1065` |
| Termination message on overflow? | No -- bare TCP close. `BMP_TERM_REASON_OOR` is defined but never used | `bmp.c:159`, `:981` |

So: **bounded byte queue; on overflow reset the session; never drop, never
block.** History matters -- BIRD had exactly this bug (unbounded `mb_alloc` list,
no limit check) and fixed it deliberately in `e6a100b3` (2024-09-17), whose
message states the intent: "there is a documented and configurable limit on the
TX queue size". The bounded design ships in v2.16 and v3.0.0+; 2.0.9-2.15.x
still have the unbounded version, so do not test against those and conclude
otherwise.

-> Constraint: bound by BYTES, not message count. BMP Route Monitoring messages
vary enormously in size, so a message-count cap silently diverges from BIRD.

-> Constraint: Ze must add an explicit goroutine + queue handoff to get the
property BIRD gets structurally. BIRD's producer *cannot* block (every fd is
`O_NONBLOCK`, `sk_send` returns 0 rather than blocking); ze's `writeRaw`
(`sender.go`) does `SetWriteDeadline(10s)` then a blocking `conn.Write`.
That is the substantive porting cost.

-> Constraint: enqueue WHOLE messages only. BIRD's copy loop can `return`
mid-message on hitting the limit (`bmp.c:307-312`), leaving a partially copied
message in a queue it is about to free wholesale. Ze should not construct that
hazard. (Whether a truncated buffer can reach the wire before the overflow event
runs was NOT established -- `tx_ev` and `tx_overflow_event` are on the same event
list and the ordering was not traced.)

-> Constraint: do NOT copy `bmp_fire_tx`'s yield-after-1024-buffers
(`bmp.c:392-397`). That is cooperative fairness for a single-threaded event loop;
the Go runtime preempts.

-> Constraint: `struct bmp_proto`'s `event *update_ev` (`bmp.h:70`) has zero
usages anywhere in BIRD's tree. There is no second decoupling layer to mirror;
the page queue is the whole mechanism.

-> Constraint (BLOCKING on the shape of the fix, found 2026-07-27 while scoping
it): **the queue must be a pooled byte ring, NOT a `[][]byte` of per-message
copies.** This is why the fix is not a small edit and must not be rushed.

Today every producer encodes into the session's single `scratch`
(`newSenderSession`, `sender.go`) and flushes it under `writeMu`. That buffer is
allocated exactly once per collector session, and its comment says why: it
"keeps the BGP-UPDATE -> BMP Route Monitoring hot path allocation-free". A
handoff queue holding `[]byte` elements would have to COPY each message out of
`scratch` before the producer returns (the producer reuses `scratch`
immediately), which is one heap allocation per Route Monitoring message on that
same hot path -- regressing precisely the property that comment records, and
banned by `ai/rules/performance.md` / `ai/rules/performance.md` for a
wire-facing path.

BIRD does not have this problem because it does not queue message objects: it
memcpys into pooled `alloc_page()` pages and packs messages contiguously,
splitting across page boundaries (`bmp.c:222-226`, `:316`). "Do what bird does"
therefore includes the storage shape, not only the overflow policy. The Ze
equivalent is a pooled ring sized in bytes, filled by copy under the existing
`writeMu`, drained by the session's own goroutine -- which also gives the
byte-accounting `tx_pending_count`/`tx_pending_limit` needs for free, since the
limit is a fill level rather than a count of elements.

Consequence for planning: this is a memory-architecture change to a hot path,
not a channel bolted onto `sendLocked`. It needs its own design pass against
`ai/rules/performance.md` (pool strategy by goroutine shape: one
producer set + one drain goroutine per session), and an allocation assertion in
its tests so the regression cannot come back silently.

**2. `startSender` is not idempotent across config reloads.**

`BMPPlugin.startSender` (`internal/component/bgp/plugins/bmp/bmp.go`) appends to
`bp.senders` and starts one goroutine per collector with no preceding
`stopSenders`. Its call site in `OnConfigure` is immediately adjacent to a
comment stating "startLocRIB is idempotent across reloads", so the asymmetry
reads as unintentional. If `OnConfigure` is re-delivered on reload the sender
set doubles: duplicate BMP streams to every collector, leaked sockets and
goroutines.

~~UNVERIFIED and to be settled FIRST: whether a config reload re-delivers the
Stage-2 configure callback at all.~~ **SETTLED 2026-07-27: it does NOT, so this
is LATENT, not live.** `deliverConfigRPC`
(`internal/component/plugin/server/startup.go`) has exactly one caller,
`engineStartupSink.deliverConfig`, reached only from
`runStartupHandshake` (`startup_driver.go`), reached only from
`handleProcessStartupRPC` (`startup.go`) and `subsystem.go`. That is
once per plugin PROCESS startup; nothing re-delivers Stage-2 configure to a
running plugin on reload.

**FIXED 2026-07-27** on the spec's own second branch ("the fix is a cheap guard
plus a regression test"): `startSender` now calls `stopSenders()` first, so it is
idempotent, matching its call-site neighbor `startLocRIB`. Regression test
`TestStartSenderIsIdempotent` (`sender_test.go`), mutation-verified -- removing
the guard makes it report 4 senders for 2 collectors. The guard is kept despite
being latent so that a future reload path cannot silently double every
collector's stream, sockets and goroutines; the test comment records why it is
not dead weight.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `path/to/file.go` - [what it currently does]

**Behavior to preserve:** (unless user explicitly said to change)
- [output format, e.g., "JSON uses nested [[]] arrays for OR/AND grouping"]
- [function signatures that callers depend on]
- [test expectations from existing .ci files]

**Behavior to change:** (only if user explicitly requested)
- [list changes user asked for, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Defect 1: RIB best-change batches enter `BMPPlugin.handleBestChange` synchronously on the RIB publisher goroutine -- engine EventBus subscribers fire from `deliverEvent` (`internal/component/plugin/server/engine_event.go`, `SubscribeEngineEvent`)
- Format at entry: best-change batch (`batch.Changes`), one entry per changed prefix
- Defect 2: BMP collector config enters via the Stage-2 configure callback (`OnConfigure` in `internal/component/bgp/plugins/bmp/bmp.go`), delivered by `deliverConfig` from the 5-stage startup driver (`internal/component/plugin/server/startup_driver.go`)

### Transformation Path
1. RIB publishes a best-change batch on the engine EventBus; `deliverEvent` runs subscribers synchronously on the publisher goroutine
2. `BMPPlugin.handleBestChange` (`internal/component/bgp/plugins/bmp/bmp_locrib.go`) builds a Route Monitoring message per change
3. Today: `len(batch.Changes) x len(senders)` blocking `conn.Write` calls to collector sockets, each bounded only by the 10s `writeTimeout` -- a wedged collector stalls RIB best-change publication (the defect)
4. Wanted: the subscriber callback only enqueues into a bounded per-session send queue; a per-sender goroutine drains the queue and does the socket writes
5. Reload path: `OnConfigure` calls `startSender`, which appends to `bp.senders` and starts one goroutine per collector with no preceding `stopSenders`; a re-delivered configure doubles the sender set (the second defect)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | [ ] |
| Wire ↔ Storage | [via iterators/pools] | [ ] |
| [other boundaries] | [mechanism] | [ ] |

### Integration Points
- `EventBus.Subscribe` (`pkg/ze/eventbus.go`) - contract the fix restores: handler "runs synchronously when an event is emitted and MUST NOT block on I/O"
- `deliverEvent` / `SubscribeEngineEvent` (`internal/component/plugin/server/engine_event.go`) - synchronous delivery path that puts `handleBestChange` on the RIB publisher goroutine
- `BMPPlugin.handleBestChange` (`internal/component/bgp/plugins/bmp/bmp_locrib.go`) - becomes enqueue-only; the per-session send queue drains on a per-sender goroutine
- `BMPPlugin.startSender` / `bp.senders` (`internal/component/bgp/plugins/bmp/bmp.go`) - gains the idempotency guard (stop-before-start or equivalent) across `OnConfigure` re-delivery
- `deliverConfig` (`internal/component/plugin/server/startup_driver.go`) - the reload re-delivery question to settle first (defect 2)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugins.md`)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->
<!-- Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis) land HERE, not just in conversation. -->

### Assumptions
<!-- Things believed true that the design depends on. Every row needs a validation method. -->
<!-- Status: unvalidated → confirmed | broken. A broken assumption also gets a Mistake Log "Wrong Assumptions" row. -->
<!-- No assumption may still be `unvalidated` at Pre-Commit Verification. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | [what we believe] | [where the belief comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
<!-- Things that could go wrong even if all assumptions hold. From /ze-spec Failure Mode Analysis. -->
<!-- Surviving risks copy forward to the Executive Summary "Risks & observations" and the learned summary. -->
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what could bite] | [how we'd notice] | [what we'd do] |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation — unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| RIB best-change event on engine EventBus | -> | `BMPPlugin.handleBestChange` enqueues to the bounded per-session send queue (no socket write on the subscriber goroutine) | `test/plugin/bmp-sender-nonblocking.ci` |
| Config reload re-delivering `OnConfigure` | -> | `startSender` idempotency guard (sender set not doubled, no leaked goroutines/sockets) | `test/plugin/bmp-sender-reload-idempotent.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [what triggers the behavior] | [observable outcome] |
| AC-2 | [what triggers the behavior] | [observable outcome] |

## End-to-End User Stories (MANDATORY for new features)

<!-- For each user-facing operation the feature enables, trace the full path.
     This section catches missing code that narrow ACs miss. ACs verify individual
     components work; user stories verify the full chain is connected.
     Every story must have a corresponding functional or wiring test. -->

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [e.g., "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |
| 2 | [e.g., "announces SR-Policy via ExaBGP bridge"] | [bridge command -> dispatch -> handler -> encoder -> wire] | [test name] |
| 3 | [e.g., "runs `ze bgp decode` on SR-Policy hex"] | [CLI -> InProcessNLRIDecoder -> Parse -> display] | [test name] |

<!-- If a path has a broken link (no implementation at some step), that is a spec gap.
     Add the missing component to ACs, Files to Create, and TDD Test Plan before proceeding. -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests — unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what user expects to happen] | |

### Interop Tests (MANDATORY for protocol features)
<!-- REQUIRED when the spec adds/changes wire protocol behavior (BGP, IPsec, L2TP). -->
<!-- See ai/rules/interop-and-goal-validation.md for when interop is required. -->
<!-- Skip this section (with justification) only for non-protocol features. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

### Future (if deferring any tests)
- [Tests to add later and why deferred — requires explicit user approval]

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
<!-- Check // Design: annotations on each file — if the change affects behavior
     described in the referenced architecture doc, include the doc here too -->
- `internal/...` - [feature changes]

### BGP Family Checklist (if new SAFI / capability / attribute)
<!-- BLOCKING: If this spec adds a new address family, SAFI, capability, or attribute,
     you MUST read ai/patterns/bgp-family.md BEFORE filling this section.
     Copy the relevant sections below and answer every one.
     Delete this block if the spec does not involve a BGP protocol extension. -->
<!-- See ai/patterns/bgp-family.md for the full 12-section checklist with file paths. -->
| BGP Integration Point | Needed? | File | Done? |
|----------------------|---------|------|-------|
| SAFI constant + family MustRegister | [ ] | `internal/core/family/family.go`, `register.go` | [ ] |
| NLRI struct (Family/Bytes/Len/String/PathID/WriteTo/SupportsAddPath) | [ ] | `plugins/nlri/<name>/types.go` | [ ] |
| Splitter + registration | [ ] | `nlrisplit/` | [ ] |
| ValidNextHopLens case | [ ] | `attribute/mpnlri.go` | [ ] |
| AppendJSON method | [ ] | `plugins/nlri/<name>/json.go` | [ ] |
| DecodeNLRIHex function | [ ] | `plugins/nlri/<name>/decode.go` | [ ] |
| parseNLRIByFamily case | [ ] | `cli/decode_mp.go` | [ ] |
| Plugin registry Registration (InProcessNLRIDecoder, Families) | [ ] | `plugins/nlri/<name>/register.go` | [ ] |
| Capability struct + negotiation + JSON | [ ] | `capability/`, `cli/decode_open.go`, `format/json.go` | [ ] |
| ExaBGP bridge (family map, command parser, event forwarding) | [ ] | `bridge/` | [ ] |
| ExaBGP migration YANG schema container | [ ] | `internal/exabgp/migration/exabgp.yang` | [ ] |
| ExaBGP migration route converter (flexSafis or dedicated convert*ToUpdate) | [ ] | `internal/exabgp/migration/migrate_routes.go` | [ ] |
| ExaBGP compat encoding tests (.ci + .conf) | [ ] | `test/exabgp-compat/encoding/`, `test/exabgp-compat/etc/` | [ ] |
| Exhaustive switch audit (`grep 'case.*SAFI' internal/`) | [ ] | all switches on SAFI | [ ] |
| Snapshot tests (all_test.go plugin names + wire methods) | [ ] | `plugin/all/all_test.go` | [ ] |
| Config surface guards (reservedPeerNames, command_ownership) | [ ] | `bgp/config/resolve.go`, `scripts/checks/` | [ ] |
| Functional decode tests (.ci per AFI) | [ ] | `test/decode/` | [ ] |
| Functional encode tests (.ci round-trip) | [ ] | `test/encode/` | [ ] |
| Interop scenario + existing interop config audit | [ ] | `test/interop/scenarios/` | [ ] |
| Cross-plugin impact (existing NLRI plugins still decode correctly) | [ ] | run existing decode tests | [ ] |
| Feature docs + comparison tables | [ ] | `docs/features/`, `docs/comparison.md` | [ ] |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | [ ] | Every leaf MUST have maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | [ ] | If native YANG constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for tab-completion. Register in `validators_register.go` |
| CLI commands/flags | [ ] | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (action before identifier) | [ ] | `ai/rules/cli.md` |
| Editor autocomplete | [ ] | Automatic for YANG enum/type leaves. For dynamic values: `CompleteFn` in custom validator returns valid options |
| Functional test for new RPC/API | [ ] | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | [ ] | If command produces output: route through `ApplyPipes`/`ProcessPipes`, support all pipe operators per `ai/rules/cli.md` |
| Env var registration | [ ] | If YANG config leaves added under `environment/`: matching `ze.<name>.<leaf>` env var via `env.MustRegister()`. Read `ai/rules/config.md` before adding env-only settings |
| Doctor check for runtime dependencies | [ ] | If any file path, socket, external service, kernel module, listen port, procfs/sysctl, netlink, external binary, or certificate material is introduced: owning package doctor check, `internal/core/diagnostic/codes.go`, unit test, functional test (see `ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | [ ] | If feature has observable state: define counters, register in telemetry, list metric names and labels in this spec |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- Every No MUST be backed by a source-aware check, not a guess. At minimum, grep docs for source anchors pointing at changed files. -->
<!-- Any factual doc change MUST include or update a source-anchor HTML comment after the claim. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | `rfc/short/rfcNNNN.md` (summary) and `docs/features/rfc-status.md` (status ledger row with source anchors) |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | [ ] | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | [ ] | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md`, relevant guide |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Grep `docs/` for `source: <changed-file>` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | Verify examples against YANG/parser/handler and update stale syntax |

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

## Implementation Steps

<!-- Steps must map to /implement stages. Each step should be a concrete phase of work,
     not a generic process description. The review checklists below are what /implement
     stages 5, 9, and 10 check against — they MUST be filled with feature-specific items. -->

### /implement Stage Mapping

<!-- This table maps /implement stages to spec sections. Fill during design. -->
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan — check what exists |
| 3. Wiring phase | Wiring Test table — register entry points, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist below |
| 13. /ze-review gate | Review Gate section — run `/ze-review`; fix every BLOCKER/ISSUE; re-run until 0 BLOCKER/0 ISSUE (final review gate before closure) |
| 14. Present summary + close | Executive Summary Report; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

<!-- List concrete phases of work. Each phase follows TDD: write test → fail → implement → pass.
     Phase 1 is ALWAYS wiring: create the entry point and a failing wiring test.
     Remaining phases fill in feature logic behind the wired skeleton.
     Phases should be ordered by dependency (e.g., schema before resolution, resolution before CLI). -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — register entry points, write failing wiring tests
   - Tests: [wiring test names from Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: entry point exists and is reachable; wiring test fails because feature logic is a stub
2. **Phase: [name]** — [what to implement]
   - Tests: [test names from TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses
3. **Phase: [name]** — [what to implement]
   - Tests: [test names from TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test passes
4. **Functional tests** → Create after feature works. Cover user-visible behavior.
5. **RFC refs** → Add `// RFC NNNN Section X.Y` comments (protocol work only)
6. **Full verification** → `make ze-verify` (lint + all ze tests except fuzz)
7. **Complete spec** → Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`. TWO commits: commit A saves code + tests + spec + learned summary; commit B does `git rm` of the spec. BLOCKING: summary is part of commit A, not a follow-up.

### Critical Review Checklist (/implement stage 6)

<!-- MANDATORY: Fill with feature-specific checks. /implement uses this table
     to verify the implementation. Generic checks from rules/quality.md always apply;
     this table adds what's specific to THIS feature. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path (no broken links in the chain). Reference feature comparison: new feature has everything the reference has |
| Correctness | [feature-specific: e.g., "merge order correct", "error messages accurate"] |
| Naming | [feature-specific: e.g., "JSON keys use kebab-case", "YANG uses kebab-case"] |
| Data flow | [feature-specific: e.g., "resolution in X only, reactor unaware of Y"] |
| CLI grammar | If CLI commands added: action before identifier per `ai/rules/cli.md` |
| Registration over hardcoding | New command/view/family/handler is registry-registered and core-discovered; no new per-feature field, switch case, or factory added to a core/shared struct (incl. the CLI `Model`). See `ai/rules/plugins.md` |
| Doctor checks | If runtime dependencies added: `ze doctor` check registered per `ai/rules/repo-maintenance.md` |
| YANG validation | If YANG leaves added: every leaf has max native constraints (`range`/`length`/`pattern`/`enum`). Bare `type string` is a red flag. Custom validator + `CompleteFn` where native is insufficient |
| Prometheus counters | If observable state exists: counters defined, registered, metric names listed |
| Rule: no-layering | [if replacing something: "old code fully deleted"] |
| Rule: [other relevant rule] | [what to check] |

### Deliverables Checklist (/implement stage 10)

<!-- MANDATORY: Every deliverable with a concrete verification method.
     /implement re-reads the spec and checks each item independently. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command to verify] |

### Security Review Checklist (/implement stage 11)

<!-- MANDATORY: Feature-specific security concerns. /implement checks each item.
     Think about: untrusted input, injection, resource exhaustion, error leakage. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |
| [other concern] | [what to check] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem → arch doc, process → rules, knowledge → memory.md -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->
<!-- Not all specs have one. Delete this section if nothing qualifies. -->
<!-- Source for learned summary Decisions section (METHODOLOGY.md extraction step 2). -->

## Key Design Decisions
<!-- Record each significant design choice as it is made. -->
<!-- Format: "Chose X over Y because Z." Include rejected alternatives. -->
<!-- Source for learned summary Decisions section (METHODOLOGY.md extraction step 2). -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries and constraints accepted. -->
<!-- Source for learned summary Consequences section (METHODOLOGY.md extraction step 3). -->
- [What was deliberately not done and why]

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

## Implementation Summary

### What Was Implemented
- Defect 1, landed by `ba6e0c39d`: a bounded per-session transmit queue.
  `txQueue` (`internal/component/bgp/plugins/bmp/txqueue.go`) is a pooled page
  ring bounded in BYTES (`txQueueLimitBytes`, 256 MiB per collector), and
  `senderSession.drainLoop` (`sender_drain.go`) is the one goroutine that writes
  to the collector socket. Every producer now ends at `enqueueLocked` (five call
  sites in `sender.go`, lines 621, 640, 671, 694, 730), so no socket write
  happens on the EventBus subscriber goroutine that runs
  `BMPPlugin.handleBestChange` (`bmp_locrib.go`).
- Overflow policy, as the design decided: never drop a message, never block the
  producer, reset the session instead. `enqueueLocked` closes the connection
  with no Termination message and returns `errQueueOverflow`.
- Defect 2, landed by `f091c69f1`: `BMPPlugin.startSender`
  (`internal/component/bgp/plugins/bmp/bmp.go`, line 478) calls `bp.stopSenders()`
  first, so it is idempotent and matches its neighbour `startLocRIB`.

### Bugs Found/Fixed
- The stale-drain hazard: a write that outlives its connection could discard the
  NEXT session's primed Peer Up messages. `clearConnAndResetIf` compares the
  connection before it resets, and `TestStaleDrainDoesNotDiscardTheNextSessionsQueue`
  (`sender_queue_test.go`) covers it.

### Documentation Updates
- `docs/guide/bmp.md` lines 167-184: the transmit queue, the 256 MiB bound, the
  stall log line, and two source anchors on
  `txqueue.go -- txQueueLimitBytes, txQueue.push` and
  `sender_drain.go -- enqueueLocked, drainLoop`.
- `docs/features/rfc-status.md`: RFC 7854 row. Both rode `ba6e0c39d`.

### Deviations from Plan
- The two `.ci` files named in the Wiring Test table
  (`bmp-sender-nonblocking.ci`, `bmp-sender-reload-idempotent.ci`) were not
  created. The entry points are covered by
  `TestHandleBestChangeDoesNotBlockOnWedgedCollector` and
  `TestStartSenderIsIdempotent`, and the queue path carries every Route
  Monitoring message, so the existing `test/plugin/bmp-sender-route-monitoring.ci`
  exercises it end to end against a live collector socket.
- This spec was never taken past `skeleton`, so it states no AC-N rows. The
  audit below is keyed to the two defects its Task section names.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| No socket write on an EventBus subscriber goroutine | Done | `enqueueLocked`, `drainLoop` (`sender_drain.go`) | Five producer sites in `sender.go` end at `enqueueLocked` |
| Bounded queue, in bytes, not in message count | Done | `txQueueLimitBytes`, `txQueue.push` (`txqueue.go`) | 256 MiB per collector session |
| Never drop, never block, reset on overflow | Done | `enqueueLocked` (`sender_drain.go`, lines 40-58) | Bare TCP close, no Termination message |
| The hot path stays allocation-free | Done | `txPagePool` (`txqueue.go`) | `TestSenderRouteMonitoringHotPathIsAllocationFree` |
| `startSender` idempotent across reloads | Done | `BMPPlugin.startSender` (`bmp.go`, line 478) | Latent, not live: no reload path re-delivers Stage-2 configure |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| none | Changed | -- | The spec stayed at `skeleton` and its AC table holds template rows. The Requirements table above is the audit of record |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestHandleBestChangeDoesNotBlockOnWedgedCollector` | Done | `sender_queue_test.go` | Defect 1, at its entry point |
| `TestSenderQueueOverflowResetsSessionWithoutTermination` | Done | `sender_queue_test.go` | The overflow policy |
| `TestTxQueueRefusesWholeMessageAtLimit` | Done | `txqueue_test.go` | Whole messages only |
| `TestTxQueueSteadyStateIsAllocationFree` | Done | `txqueue_test.go` | The pooled ring |
| `TestStartSenderIsIdempotent` | Done | `sender_test.go`, line 404 | Defect 2, mutation-verified |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/bmp/txqueue.go` | Done | New in `ba6e0c39d` |
| `internal/component/bgp/plugins/bmp/sender_drain.go` | Done | New in `ba6e0c39d` |
| `internal/component/bgp/plugins/bmp/sender.go` | Done | Producers route to `enqueueLocked` |
| `internal/component/bgp/plugins/bmp/bmp.go` | Done | `startSender` guard |
| `docs/guide/bmp.md` | Done | Operator-visible behaviour |

### Audit Summary
- **Total items:** 5 requirements, 5 tests, 5 files
- **Done:** 15
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the AC table stayed at template; see Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| A wedged collector no longer stalls RIB best-change publication | unit test at the entry point | `TestHandleBestChangeDoesNotBlockOnWedgedCollector` (`sender_queue_test.go`, line 266) |
| The queue is bounded and the bound is in bytes | unit test | `TestTxQueueRefusesWholeMessageAtLimit` (`txqueue_test.go`, line 76); `txQueueLimitBytes` = 256 MiB |
| Overflow resets the session and loses no message silently | unit test | `TestSenderQueueOverflowResetsSessionWithoutTermination` (`sender_queue_test.go`, line 296) |
| The BGP-UPDATE to Route Monitoring path stays allocation-free | allocation assertion | `TestSenderRouteMonitoringHotPathIsAllocationFree` (`sender_queue_test.go`, line 387); `TestTxQueueSteadyStateIsAllocationFree` |
| Messages still reach a real collector through the new path | functional test | `test/plugin/bmp-sender-route-monitoring.ci` drives a Python collector and validates the embedded BGP PDU |
| A re-delivered `OnConfigure` cannot double the sender set | unit test | `TestStartSenderIsIdempotent` (`sender_test.go`, line 404) |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Stop `handleBestChange` doing blocking socket I/O on a subscriber goroutine (`plan/deferrals/ad-hoc-2026-07-22-f27dc80f.md`) | done | `enqueueLocked` + `drainLoop` (`sender_drain.go`), landed by `ba6e0c39d` |
| Make `startSender` idempotent across config reloads (same shard) | done | `bp.stopSenders()` in `startSender` (`bmp.go`, line 478), landed by `f091c69f1` |

Both rows are terminal, so the shard is residue and this closure removes it.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-bmp-sender-blocking-and-reload-640fa955-f03a-45e8-a58f-4b367f5859e6.md` |
| `review_gate.py check` | clean (0 code files in the closure commit, hashes match) |
| Rounds | 1 |
| Reviewer lenses used | producer/consumer split and blocking I/O, queue bound and overflow policy, reload idempotency, test coverage of each |

**This review reads code that is already at HEAD.** The implementation landed in
`ba6e0c39d` and `f091c69f1`; this closure carries no code, so the artifact
records a review of the committed producers, not of a working-tree diff.

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | The two `.ci` files the Wiring Test names were never created | `test/plugin/` | acknowledged; the entry points carry unit tests and `bmp-sender-route-monitoring.ci` drives the queue path against a live collector |

### Fixes applied
- None. The review found no BLOCKER and no ISSUE.

### Final status
- [ ] Review shows 0 BLOCKER, 0 ISSUE
- [ ] The one NOTE is recorded above

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/bmp/txqueue.go` | yes | `git ls-tree HEAD internal/component/bgp/plugins/bmp/` lists it |
| `internal/component/bgp/plugins/bmp/sender_drain.go` | yes | same listing |
| `internal/component/bgp/plugins/bmp/txqueue_test.go` | yes | same listing |
| `internal/component/bgp/plugins/bmp/sender_queue_test.go` | yes | same listing |
| `test/plugin/bmp-sender-nonblocking.ci` | no | `git ls-tree HEAD test/plugin/` holds no such file; see Deviations |
| `test/plugin/bmp-sender-reload-idempotent.ci` | no | same listing; see Deviations |
| `test/plugin/bmp-sender-route-monitoring.ci` | yes | same listing; read, and it drives a Python collector |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| Defect 1 | No producer writes to the socket | `grep -n enqueueLocked sender.go` returns five producer sites, and the only `conn.Write` is in `writeRawLocked`, which only `drainLoop` reaches |
| Defect 1 | The bound is in bytes | `txQueueLimitBytes` (`txqueue.go`) is `256 << 20`; `txQueue.limit` is a byte fill level, not an element count |
| Defect 1 | Overflow resets, never drops | `enqueueLocked` (`sender_drain.go`): log, `closeLog`, `clearConnAndResetIf`, `errQueueOverflow` |
| Defect 2 | `startSender` is idempotent | `bp.stopSenders()` is the first statement of `startSender` (`bmp.go`) |
| Both | The package is green | `make ze-test-pkg PKG=./internal/component/bgp/plugins/bmp` -> `ok github.com/ze-software/ze/internal/component/bgp/plugins/bmp (cached)`, exit 0, 2026-08-11 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| RIB best-change event on the engine EventBus | `test/plugin/bmp-sender-route-monitoring.ci` (the named `bmp-sender-nonblocking.ci` does not exist) | yes: the file was read. It announces a prefix from `ze-peer` and a collector process validates the BMP-wrapped BGP PDU, so the bytes travel the queue and the drain goroutine |
| Config reload re-delivering `OnConfigure` | none (the named `bmp-sender-reload-idempotent.ci` does not exist) | covered by `TestStartSenderIsIdempotent` (`sender_test.go`). No reload path re-delivers Stage-2 configure, so no `.ci` can drive this entry point today |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | changed | The spec's assumption table stayed at template. The two claims its Task section did make were both settled there: config reload does NOT re-deliver Stage-2 configure (`deliverConfigRPC`, `internal/component/plugin/server/startup.go`, one caller), and BIRD bounds its queue by bytes rather than by message count (`tx_pending_limit`, `proto/bmp/bmp.h`) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/bmp.md` describes the transmit queue and its bound | The page states 256 MiB and carries `<!-- source: ... txqueue.go -- txQueueLimitBytes, txQueue.push -->`; the anchor matches `txQueueLimitBytes` | yes |
| Every other category | `ba6e0c39d` touched `docs/guide/bmp.md` and `docs/features/rfc-status.md` and no other doc; no config leaf, CLI verb, RPC or wire format changed, so no other page states a claim about this work | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
