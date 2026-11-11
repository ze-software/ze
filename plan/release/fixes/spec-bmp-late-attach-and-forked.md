# Spec: bmp-late-attach-and-forked

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | A BMP collector must be told of every monitored peer that is Established, and a bgp-bmp that runs forked must work or refuse to start. |
| RFC gaps | RFC7854-x-16, RFC7854-x-17 |
| Depends | Related: `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` (the other BMP RFC 7854 defects, and the owner of `message-for-a-peer-never-announced` 2026-10-13, which shares `recordPeerUp`). No ordering between the two is established yet |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two defects in the BMP sender, found on 2026-11-11 by a design pass on
`plan/release/fixes/spec-single-owner-lifecycle.md` and read at the producer
the same day. Neither was run.

| ID | Defect | Goal |
|----|--------|------|
| D1 | A BGP peer that was Established before bgp-bmp observed its OPEN exchange is never announced to a collector: no Peer Up and no Adj-RIB-In dump | A collector is told of every monitored peer in the Established state and receives its Adj-RIB-In, whenever the plugin came to monitor it |
| D2 | bgp-bmp run as a forked process receives no event and sends no Peer Up and no Route Monitoring, with no refusal and no error | A forked bgp-bmp either monitors as the in-process one does, or refuses to start with a message naming the fix |

These two rows were first offered to `spec-bmp-sflow-export-rfc-defects`. The
spec-edit gate refused that edit because the older spec lacks
`## Existing Mechanisms` and needs source and doors unrelated to these rows, so
they are homed here.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the process boundary
  → Constraint: "Code that only works internal is broken." A forked plugin reaches another unit over the EventBus, DirectBridge's RPC form, or a text command.
- [ ] `ai/doors/bgp-plugins.md` - what a BGP plugin may rely on
  → Constraint: "Every one can run in another process."
  → Constraint: "A subscription delivers future events only. A late subscriber queries and reconciles. There is no replay."
  → Constraint: "Replay on reconnect is selective, never bulk".
- [ ] `ai/doors/plugin-infra.md` - the SDK and the two transports
  → Constraint: "Every feature works over both transports." A forked plugin "never gets `DirectBridge`".
- [ ] `docs/guide/bmp.md` - what the operator is told today
  → Constraint: the page states D1 as an instruction: "Attach BMP before establishing that BGP session, because replay depends on observing the OPEN exchange and subsequent updates. Adding the attachment to an already established peer requires that BGP peer to reconnect before it can be monitored." It is true today and it is a gap against RFC 7854 Sections 3.1 (Peer Up) and 3.3 (Peer Up and dump), never an intended limit. The fix edits this passage.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7854.md` - rows RFC7854-x-16 and RFC7854-x-17 quote the Section 3.3 sentences below, at level SHOULD, unticked. RFC7854-3.3-1 is the End-of-RIB MUST.
  → Constraint: the summary already carries the rows. This spec writes no new summary row.

### RFC text read (verbatim, `rfc/full/rfc7854.txt`)

Line breaks of the text file are joined. The hyphenated break "Adj-" / "RIBs-In" at line 320 is joined to "Adj-RIBs-In".

| Section | Lines | Quote |
|---------|-------|-------|
| 3.1 BMP Messages | 238-241 | "In addition to being sent whenever a peer transitions to the Established state, a Peer Up Notification is sent for each peer in the Established state when the BMP session itself comes up." |
| 3.3 Lifecycle of a BMP Session | 318-320 | "It subsequently sends a Peer Up message over the BMP session for each of its monitored BGP peers that is in the Established state." |
| 3.3 | 320-322 | "It follows by sending the contents of its Adj-RIBs-In (pre-policy, post-policy, or both, see Section 5) encapsulated in Route Monitoring messages." |
| 3.3 | 322-325 | "Once it has sent all the routes for a given peer, it MUST send an End-of-RIB message for that peer; when End-of-RIB has been sent for each monitored peer, the initial table dump has completed." |
| 3.3 | 333-334 | "If any new monitored BGP peer becomes Established, a corresponding Peer Up message is sent." |

The Peer Up and dump sentences carry no RFC 2119 keyword. Only the End-of-RIB sentence says MUST.

### Reference implementations read

| Implementation | Function | What it does |
|----------------|----------|--------------|
| FRR stable/10.8, `bgpd/bgp_bmp.c` | `bmp_send_peerup`, `bmp_send_peerup_per_instance` | Walks every peer of the BGP instance (`bgp->peer`) and writes a Peer Up for each from the peer's live state |
| FRR stable/10.8, `bgpd/bgp_bmp.c` | `bmp_wrsync` | Walks each address family marked as needing sync and dumps from the BGP table (`bgp_table_top`) |
| BIRD v3.2.1, `proto/bmp/bmp.c` | `bmp_startup` | On BMP session up, sends Initiation, subscribes to protocol state changes, then loads the current protocol states |
| BIRD v3.2.1, `proto/bmp/bmp.c` | `bmp_add_table` | Subscribes to the routing table's export (`rt_export_subscribe`), which feeds the table's current routes |

Both read the live peer and table state. Neither depends on having watched the session come up.

**Key insights:**
- D1 is a late subscriber that never reconciles: the door's rule is that such a subscriber queries.
- The engine already tells a newly attached process that the peer is up (reason `attached`). bgp-bmp drops that event.
- D2 is one callback registered for one transport only.

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-11, read not run)
- [ ] `internal/component/bgp/plugins/bmp/bmp_events.go` - `recordPeerUp` runs on every `SessionStateUp` event, reads the two OPEN PDUs from `openCache`, and returns with no log line when either is missing. `handleStructuredEvent` fills `openCache` only from OPEN events. `handleSenderUpdate` streams Route Monitoring without consulting `peerUps`. `primeSender` announces and replays only the peers in `peerUps`.
- [ ] `internal/component/bgp/plugins/bmp/bmp_replay.go` - `cacheAdjUpdate` returns nil when the peer has no `peerUps` entry, and nil when `se.RawMessage` is not a `*bgptypes.RawMessage`. The per-peer snapshot is fed by live UPDATE events only; nothing queries the RIB for a peer's Adj-RIB-In.
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - `runBMPPlugin` and `registerCallbacks`: the plugin registers `OnStructuredEvent` and no `OnEvent`, and holds no `IsInternal` check.
- [ ] `internal/component/bgp/plugins/bmp/bmp_locrib.go` - `startLocRIB` warns "loc-rib monitoring enabled but no event bus is available" when the bus is nil, and returns. `emitReplayRequest` asks bgp-rib for a full Loc-RIB replay over the bus. A comment in `handleBestChange` calls the dump token "an in-process claim".
- [ ] `internal/component/bgp/plugins/bmp/register.go` - `ConfigureEventBus` installs the bus. `RunEngine` is `runBMPPlugin`.
- [ ] `internal/component/bgp/server/events.go` - `onPeerBound` delivers a state up with reason `rpc.ReasonAttached` to each process a reload attached to a running session.
- [ ] `internal/component/bgp/reactor/peer_derived_bindings.go` - `bindProcesses` hands the attach to `OnPeerBound` with InitialReplay 0. The `onPeerBound` comment names `applyHotSwappableSettings` as the reload caller; that caller was not read.
- [ ] `internal/component/plugin/server/restart.go` - `restartHandshake` runs the five stages and the post-startup delivery for a respawned plugin. It delivers no state event for a peer that is already up.
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `initCallbackDefaults` answers `deliver-event` and `deliver-batch` with OK and no action. `OnEvent` replaces both. `OnStructuredEvent` sets only the handler the DirectBridge calls.

**D1 reach (which sequences leave a peer unobserved):**

| Sequence | Result | Evidence |
|----------|--------|----------|
| A reload attaches bgp-bmp to a peer that is Established | The attached up arrives, `openCache` holds nothing for the peer, `recordPeerUp` returns silently. No Peer Up, no dump. Later UPDATEs still stream as Route Monitoring | `onPeerBound`, `recordPeerUp`, `handleSenderUpdate` read |
| bgp-bmp is restarted by its failure policy while peers are up | The new process starts with empty `openCache` and `peerUps`, and `restartHandshake` replays no state. No Peer Up until each peer reconnects | `runBMPPlugin`, `restartHandshake` read; whether any other path delivers a state event after a restart was not traced |
| A reload first adds BMP config, so the plugin auto-loads while peers are up | Not traced | UNVERIFIED |
| A binding without `open` | Already recorded: `message-for-a-peer-never-announced` 2026-10-13, owned by `spec-bmp-sflow-export-rfc-defects` | journal row |

**D2 (bgp-bmp forked):** the handshake and config proceed, because nothing checks `IsInternal`. Events reach the SDK's default `deliver-event` and `deliver-batch` callbacks, which answer OK and drop them, so `handleStructuredEvent` never runs: no OPEN cache, no Peer Up, no Route Monitoring, no Route Mirroring. Loc-RIB monitoring logs one warning that names no fix. No functional test runs bgp-bmp through the external harness. Whether the receiver half works forked was not read.

**Behavior to preserve:**
- The order a collector receives on connect: Initiation, Peer Up for each recorded peer, the snapshot, End-of-RIB (`primeSender`).
- A Peer Up carries the two real OPEN PDUs (RFC 7854 Section 4.10); none is sent with a fabricated OPEN for a BGP peer.

**Behavior to change:**
- D1 and D2, as the Task table states.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Bus query for a table replay | `internal/component/bgp/plugins/bmp/bmp_locrib.go` `emitReplayRequest` | Asks bgp-rib over the EventBus to re-emit the Loc-RIB as replay batches, correlated by a token | Candidate to extend for a peer's Adj-RIB-In (open design question) |
| Late subscriber queries and reconciles | `ai/doors/bgp-plugins.md`, Traps | The stated rule for a subscriber that joins after the events it needs | The rule D1's fix follows, or the rule the design proposes to change, with its page |
| Attached up | `internal/component/bgp/server/events.go` `onPeerBound`, `rpc.ReasonAttached` | Tells a process a reload attached that the running session is up, with InitialReplay 0 | The event D1's fix reacts to. A grep shows `adj_rib_in`, `rib`, `rs`, `rr` and `watchdog` branch on the reason; their handlers were not read |
| Per-peer snapshot and replay | `internal/component/bgp/plugins/bmp/bmp_replay.go` `cacheAdjUpdate`, `replayPeerLocked` | Keeps each recorded peer's routes from live UPDATEs and replays them to a connecting collector | Extended: it needs a first fill for a peer whose UPDATEs it did not see |
| External-start refusal | `pkg/plugin/sdk/sdk.go` `IsInternal`; the door names `rib` and `adj_rib_in` as plugins that branch on it | A plugin that cannot work forked refuses or branches | One of D2's two candidate fixes |
| Text event rail | `pkg/plugin/sdk/sdk_callbacks.go` `OnEvent` | Delivers events to a forked plugin as text | The other candidate fix for D2 |

## Open design questions

Stated without a choice. The design round answers each from the RFC text, then FRR and BIRD.

| ID | Question | Options |
|----|----------|---------|
| DQ-1 | How bgp-bmp learns the routes of a peer whose UPDATEs it did not see | (a) bgp-bmp queries the RIB plugin for the peer's Adj-RIB-In, as `emitReplayRequest` does for the Loc-RIB; (b) the engine replays state-up and routes to a late subscriber, which changes the door's "There is no replay" |
| DQ-2 | How bgp-bmp learns the two OPEN PDUs of a peer whose OPEN exchange it did not see | Not designed. The attached up carries none that `recordPeerUp` reads |
| DQ-3 | D2: serve the text rail, or refuse external start | The door says every plugin can run in another process; a refusal needs the message and its fix command |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

Not designed (skeleton). The subsections state what is read today.

### Entry Point
- D1: a state up event with reason `attached`, delivered by `onPeerBound` when a reload attaches the process to a running session.
- D2: the events the engine delivers to a forked process as `deliver-event` and `deliver-batch`.

### Transformation Path
1. [design round]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | State and UPDATE events to bgp-bmp: structured over DirectBridge in process, text over RPC forked | No |
| Plugin ↔ Plugin | bgp-bmp to bgp-rib over the EventBus, if DQ-1 takes option (a) | No |

### Changed-Wire Census

Not designed (skeleton). DQ-1 and DQ-2 decide whether an event or an RPC changes.

### Integration Points
- `recordPeerUp`, `cacheAdjUpdate`, `primeSender` - the recorded-peer state every collector message is built from
- [design round]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

Draft rows; the design round fixes inputs, tests and survivors.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A reload attaches bgp-bmp to a peer that is Established, a collector connected | The collector receives a Peer Up for that peer carrying both OPEN PDUs, then its Adj-RIB-In as Route Monitoring, then End-of-RIB per family (RFC 7854 Section 3.1 for the Peer Up at BMP session start; Section 3.3 for the Peer Up, the dump and the End-of-RIB) | [design round] |
| AC-2 | bgp-bmp restarts while peers are Established | Each monitored Established peer is announced and dumped to the collector without the BGP session reconnecting | [design round] |
| AC-3 | bgp-bmp configured as a forked process | Per DQ-3: the collector receives the same messages as from the in-process plugin, or the daemon refuses the start with a message naming the fix | [design round] |

## 🧪 TDD Test Plan

Not designed (skeleton). The design round names each test; every row below is a placeholder.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [design round] | `internal/component/bgp/plugins/bmp/` | AC-1, AC-2: an attached up for a peer with no cached OPEN ends in a recorded Peer Up and a filled snapshot | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [design round] | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [design round] | [design round] | AC-1: a reload attaches bgp-bmp to an Established peer and the collector receives Peer Up, routes and End-of-RIB | |
| [design round] | [design round] | AC-3: bgp-bmp run as an external process | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| [design round] | [design round] | a BMP collector (the existing BMP scenarios use pmacct) | AC-1 as an independent collector decodes it | |

## Files to Modify
- `internal/component/bgp/plugins/bmp/bmp_events.go` - `recordPeerUp` and its callers (D1)
- `internal/component/bgp/plugins/bmp/bmp_replay.go` - the first fill of a peer's snapshot (D1)
- `internal/component/bgp/plugins/bmp/bmp.go` - `registerCallbacks`, `runBMPPlugin` (D2)
- `docs/guide/bmp.md` - the passage that tells the operator to reconnect the peer
- [design round: whatever DQ-1 and DQ-2 name on the RIB or engine side]

## Files to Create
- [design round]

## Implementation Steps

Not designed (skeleton).

1. **Phase: Wiring (MANDATORY FIRST)** -- [design round]

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `announced-state-never-replayed.md` | 2026-11-11 | bmp `recordPeerUp` on the attached up: a peer Established before bgp-bmp saw its OPENs is never announced |
| `silent-fall-through.md` | 2026-11-11 | bgp-bmp run forked receives no event and sends nothing, with no refusal |

## Known Limitations

- None agreed. `docs/guide/bmp.md` describes D1 as an operator instruction; it is owed work, not a limit.

## RFC Documentation (Scope: protocol)

Add the quoted sentence above the code that performs it, each under the section it comes from: `// RFC 7854 Section 3.1: "<quoted sentence>"` above the Peer Up for a peer already Established when the BMP session comes up, and `// RFC 7854 Section 3.3: "<quoted sentence>"` above the Peer Up, the Adj-RIBs-In dump and the End-of-RIB of the session lifecycle.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package test (`./le job run label bmp command go test -tags ze_bgp,ze_bmp ./internal/component/bgp/plugins/bmp/...`), then `./le test functional plugin` and `./le test functional reload` for the tests this spec adds. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded in this spec's Review Gate
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
