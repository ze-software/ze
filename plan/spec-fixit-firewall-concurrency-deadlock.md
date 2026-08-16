# spec-fixit-firewall-concurrency-deadlock

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/6 (phases 2-5 landed; phase 1 reproduction and its phase-6 `.ci` remain) |
| Updated | 2026-08-07 |

## Task

A command-dispatch deadlock was observed under QEMU when a `firewall { backend nft }`
config block (which drives `LoadBackend` + `RegisterTables` + `ApplyAll` in the firewall
engine) runs alongside the ddos-local responder's own `firewall.ApplyAll` under a
sustained flood: the daemon's `ze-plugin-engine:dispatch-command` stopped responding for
roughly 255 seconds. The root cause is UNVERIFIED: only the symptom was observed, and no
goroutine dump or lock trace was captured. This is a potential real concurrency bug in
shared firewall infrastructure (`internal/component/firewall`), NOT something introduced
by the spec that observed it. Goal: root-cause the hang and fix it at the owning layer,
or, if the contention is inherent, document a hard constraint and make the failure
observable instead of a silent hang.

Skeleton = captured intent with verified `file:line` evidence. Research happens via
`/ze-spec` when this is picked up; the spec moves to `design` then.

## Origin

`plan/deferrals.md` row dated 2026-07-12, "spec-ddos-direction-allowlist (QEMU
observation)", recorded as "none yet (future firewall-concurrency fixit)". The `.ci`
tests for that spec worked around the hang by asserting direction classification rather
than the on-host drop.

### Scope

- IN: root-cause the dispatch hang; fix it in shared firewall infra (or document the
  constraint with evidence); prove it with a reproduction.
- IN: the lock-discipline hazards found while writing this spec (see Problem / Evidence),
  on their own merits.
- OUT: new ddos features; the per-source drop-term narrowing and flowspec withdraw items
  from the same parent spec (separate deferral rows).

## Required Reading

### Source (read before designing)
<!-- NEVER tick [ ] to [x]. Capture insights as -> Decision: / -> Constraint:. -->
- [ ] `internal/component/firewall/registry.go` (:79-124 ApplyAll) - merges every owner's
      tables, then calls `backend.Apply` (:123) holding NO firewall-package lock. Prime
      suspect region.
- [ ] `internal/component/firewall/registry.go` (:64-72 RegisterTables) - takes
      `tableRegistry.mu`; nil tables delete the owner key.
- [ ] `internal/component/firewall/backend.go` (:130 LoadBackend) - takes `backendsMu`;
      `loadBackendLocked` (:137 comment) is the variant `ApplyAll` calls while already
      holding `backendsMu`.
- [ ] `internal/component/firewall/engine.go` (:316-324 OnConfigure) - `LoadBackend`, then
      `RegisterTables("firewall", ...)`, then `ApplyAll`.
- [ ] `internal/component/firewall/engine.go` (:372-399 OnConfigApply) - conditional
      `LoadBackend`, then journalled `RegisterTables` + `ApplyAll`, with a rollback
      closure that calls both again.
- [ ] `internal/plugins/ddos/local/responder.go` (:92-150 applyMitigation) - calls
      `registerTables` then `applyAll` (:135-136) while the caller holds `r.mu` (:88);
      on failure calls both again to roll back (:141-142), still under `r.mu`.
- [ ] `internal/plugins/ddos/local/responder.go` (:63-86 onDetected / onCharacterized) -
      event-bus handlers, both take `r.mu`; fire repeatedly under a sustained flood.
- [ ] `internal/plugins/firewall/nft/backend_linux.go` (:40 Apply) - the kernel reconcile.
      Need its cost and blocking behavior under load.
- [ ] `internal/component/plugin/server/dispatch_registry.go` (:258 opDispatchCommand) -
      shared handler for `dispatch-command`. Establish whether dispatch and event
      delivery share a goroutine or a lock.
  -> Decision: they share NEITHER. `Dispatcher` (`command.go`) carries no mutex,
     and engine event handlers are invoked OUTSIDE the subscriber lock
     (`engine_event.go`). Dispatch and event delivery share only whatever lock a
     handler itself takes. For ddos-local that lock is `r.mu`, which IS shared (below).

### Added during research (2026-07-16)
- [ ] `internal/plugins/ddos/local/show.go` (:14-19, :23-37) - ddos-local DOES publish a
      command handler: `RegisterRPCs{WireMethod: "ze-show:ddos-local"}`, handler
      `handleShowDdosLocal`, which calls `r.status()` (:31) and therefore takes `r.mu`
      (`responder.go`).
  -> Constraint: `r.mu` IS on the `dispatch-command` path. The skeleton's contrary
     claim was a false negative (it grepped for `OnCommand`, the wrong symbol).
- [ ] `internal/plugins/ddos/local/cmd/yang/ze-ddos-local-cmd.yang` (:18) -
      `ze:command "ze-show:ddos-local"` is what maps the wire method to a CLI path.
- [ ] `internal/component/plugin/server/command.go` (:63-76 LoadBuiltins, :82-108
      LoadBuiltinsWithAliases) - every `RegisterRPCs` entry with a YANG path is registered
      into the dispatcher (:69, :100), which is what `dispatch-command` resolves against.
  -> Constraint: `RegisterRPCs` + a `ze:command` YANG node IS a dispatch-command handler.
     Grepping only for `OnCommand` misses every show surface in the tree.
- [ ] `internal/component/plugin/server/engine_event.go` (:14-23) - `EngineEventHandler`
      doc: handlers "are called synchronously from deliverEvent; they MUST NOT block on
      external I/O".
  -> Constraint: ddos-local's `onDetected` violates this documented contract: it performs
     a full netlink reconcile inside the handler.
- [ ] `internal/plugins/firewall/nft/backend_linux.go` (:24-35) - the backend struct holds
      `conn` + `applied` and has NO mutex; `newBackend` calls `nftables.New()` (:31) with
      no `AsLasting()` and no `SockOptions`.
- [ ] `github.com/google/nftables@v0.3.0/conn.go` (:36-46 Conn, :242-283 Flush,
      :299-322 dialNetlink) - the per-Conn command batch and its flush semantics.
  -> Decision: this is the producer of both the concurrent-apply hazard and the
     unbounded-blocking hazard. See Problem / Evidence.

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - named by `internal/component/firewall/registry.go`
      as the design home for the firewall table registry
  -> Constraint: it promises NOTHING about concurrent apply. No concurrency contract for
     `ApplyAll` or `Backend.Apply` is stated anywhere: not in the doc, not on the
     `Backend` interface (`backend.go`), not on `ApplyAll` (`registry.go`).
     The absence IS the bug: six callers were written against an unstated contract.

**Key insights:**
1. There is NO lock cycle. `tableRegistry.mu` and `backendsMu` are never held at the
   same time (`registry.go` unlocks before `:97` locks), `Backend.Apply` runs under
   no firewall lock (`registry.go`), and no path takes a firewall lock and then
   `r.mu`. The skeleton's "deadlock" framing is not supported by the code.
2. What exists instead is (a) an unsynchronized concurrent `Apply` producing lost
   updates, and (b) an UNBOUNDED blocking chain from a wedged netlink round trip
   through `r.mu` to `show ddos local`. (b) is the dispatch-stall mechanism.
3. `Backend.Apply` can block forever: the netlink receive has no deadline anywhere.

## Current Behavior (MANDATORY)

**Source files read (2026-07-15, spec author):**
- [ ] `internal/component/firewall/registry.go` (:79-124 ApplyAll) - snapshots the merged
      table set under `tableRegistry.mu`, releases it (:94), takes `backendsMu` to read or
      autoload `activeBackend` (:97-111), releases it (:111), then calls `b.Apply(all)`
      (:123) holding no firewall-package lock
- [ ] `internal/component/firewall/registry.go` (:104 autoload) - `ApplyAll` autoloads
      `defaultBackendForAutoload` when no backend is loaded and `len(all) > 0`, so a
      plugin can trigger a backend load with no `firewall {}` block present
- [ ] `internal/component/firewall/registry.go` (:113-121 no-backend path) - returns nil
      when nothing to apply, `errFirewallBackendNotLoaded` when tables are pending
- [ ] `internal/plugins/ddos/local/responder.go` (:135-148) - registers the table and
      calls `applyAll` (plus a possible rollback pair) while holding `r.mu` throughout
- [ ] `internal/component/firewall/engine.go` (:379-399) - wraps `RegisterTables` +
      `ApplyAll` in an `sdk.Journal` record/rollback pair

**Behavior to preserve:**
- Table ownership semantics: owner keys, the `ze_*` prefix sweep, and the merge of
  same-name tables across owners (`mergeSameNameTables`, registry.go).
- The autoload path: a plugin registering tables with no `firewall {}` block still
  reaches the kernel (registry.go).
- The withdraw-as-no-op path: `RegisterTables(owner, nil)` + `ApplyAll` with no backend
  loaded stays a no-op (registry.go).
- Journalled rollback on a failed reload (engine.go).
- ddos-local mitigation still installs and withdraws promptly under an active flood.
- The three registry contract tests stay green: `TestApplyAllAutoLoadsDefaultBackend`,
  `TestApplyAllNoBackendNoTablesIsNoOp`, `TestApplyAllNoDefaultKeepsNotLoadedError`
  (`internal/component/firewall/registry_test.go,65,89`).

**Source files read (2026-07-16, design session) -- the lock inventory:**

Every mutex reachable from a firewall reconcile, its guarded state, and its producer:

| # | Mutex | Declared | Guards | Held across a kernel call? |
|---|-------|----------|--------|----------------------------|
| L1 | `tableRegistry.mu` | `registry.go` | `tableRegistry.owners` (owner -> []Table) | No: unlocked at `registry.go`, before `:97` |
| L2 | `backendsMu` | `backend.go` | `backends`, `verifiers`, `activeBackend` | Yes, but only over `factory()` (`backend.go`) and `prev.Close()` inside `loadBackendLocked` |
| L3 | `r.mu` (ddos-local) | `responder.go` | `cfg`, `active`, `target` | **Yes: held across the whole `applyAll`** (`responder.go` -> `:136`) |
| L4 | `r.mu` (anomaly-shape) | `internal/plugins/anomaly/shape/responder.go` (struct) | `armed`, `armedCount`, `killed`, `cfg` | **Yes: `revertAll` is documented "Caller holds mu" (`:190`) and calls `applyAll`** |
| L5 | `nftables.Conn.mu` | `google/nftables@v0.3.0/conn.go` | `messages` (the shared command batch), `err`, `nlconn` | **Yes: `Flush` holds it across dial + SendMessages + the ack-receive loop (`conn.go`)** |
| L6 | `engineEventSubscribers.mu` | `engine_event.go` | handler map | No: handlers copied under RLock, invoked after `RUnlock` (`engine_event.go`) |
| - | `Dispatcher` | `command.go` | (struct has NO mutex) | n/a |
| - | `lastApplied`, `activeBackendName` | `accessor.go`, `:148` | applied snapshot, backend name | n/a: `atomic.Pointer` / `atomic.Value`, no lock |

**Lock ORDER actually taken, per path (producer-verified):**

| Path | Order | Cite |
|------|-------|------|
| `ApplyAll` | L1 (acquire, release) -> L2 (acquire, release) -> L5 (inside `Apply`) | `registry.go,94,97,111,123` |
| `ApplyAll` autoload | L2 held over `loadBackendLocked` -> `factory()` -> `nftables.New()` (takes no lock; `conn.go`) | `registry.go`, `backend.go` |
| ddos-local event | L3 -> L1 -> L2 -> L5 | `responder.go,136`; `registry.go,97,123` |
| ddos-local show | L3 only | `show.go`, `responder.go` |
| anomaly-shape event | L4 -> L1 -> L2 -> L5 | `shape/responder.go,200` |
| anomaly-shape show | L4 only | `shape/show.go`, `shape/responder.go` |
| firewall engine configure/apply | L2 (`LoadBackend`) -> release -> L1 -> release -> L2 -> release -> L5 | `engine.go,321,322`; `engine.go,382,383` |
| `FlushAllTables` | L1 (acquire, release at `:43`) then `ApplyAll` | `registry.go` |

-> Decision: **there is no cycle.** The order is strictly L3/L4 -> L1 -> L2 -> L5 on
every path, and nothing acquires them in the reverse direction. A lock-ordering
deadlock does not exist in this code. The skeleton's central hypothesis is BROKEN.

**Behavior to change:**
1. `ApplyAll` (`registry.go`) must serialise the snapshot-plus-apply so two owners
   cannot be inside `Backend.Apply` at once. Today nothing prevents it (`:123` holds no
   lock), and the nft backend cannot survive it (see Problem / Evidence).
2. `Backend.Apply` must be bounded. Today it can block forever: `conn.Flush` waits on
   `nlconn.Receive()` (`conn.go receiveAckAware`) with no deadline, and the ze
   backend sets none (`backend_linux.go` passes no `SockOptions`).
3. ddos-local must not hold `r.mu` across `applyAll` (`responder.go,136`), because
   `show ddos local` takes the same lock on the `dispatch-command` path (`show.go`).
4. anomaly-shape has the identical defect (`shape/responder.go,200` vs `:235`) and is
   fixed in the same change: it is one bug class in two plugins, not two bugs.
5. The concurrency contract must be written down where the callers can see it
   (`ApplyAll` doc, `Backend` interface doc, `docs/architecture/core-design.md`).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `firewall { backend nft }` section -> firewall engine `OnConfigure`
  (engine.go) / `OnConfigApply` (engine.go)
- Events: `ddosevent.Detected` / `Characterized` -> ddos-local responder handlers
  (`internal/plugins/ddos/local/register.go`)
- Operator: a CLI command -> `ze-plugin-engine:dispatch-command`
  (dispatch_registry.go) -- the surface observed to hang

### Transformation Path
1. Firewall engine parses the section, calls `LoadBackend` (engine.go/373)
2. Engine calls `RegisterTables("firewall", tables)` then `ApplyAll` (engine.go)
3. Concurrently: flood -> detector emits AttackDetected -> responder `onDetected` takes
   `r.mu` (responder.go) -> `applyMitigation` -> `registerTables` + `applyAll`
   (responder.go)
4. Both `ApplyAll` calls take `tableRegistry.mu`, release it, take `backendsMu`, release
   it (registry.go)
5. Both call `b.Apply(all)` (registry.go) with no firewall lock held -> nft reconcile
6. Meanwhile an operator command enters `opDispatchCommand` and does not return (~255s)

**Step 5-6 refined by research (2026-07-16).** The two `Apply` calls do not merely
"reconcile in parallel": they stage into ONE shared `nftables.Conn` batch, so one
`Flush` drains both and the other returns nil having sent nothing
(`conn.go`, Finding 1). And step 6 is not a mystery: the operator's command is
`show ddos local`, whose handler takes the same `r.mu` the responder holds across its
reconcile (`show.go` -> `responder.go` vs `:64,136`, Finding 3). The reconcile
does not return because the netlink ack never arrives and no deadline is set
(`conn.go`, `backend_linux.go`, Finding 2).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config -> firewall engine | plugin SDK OnConfigure / OnConfigApply | [ ] read `engine.go,348`: `LoadBackend` -> `RegisterTables` -> `ApplyAll` at `:316-324`, journalled at `:379-401` |
| event bus -> ddos-local responder | `ddosevent.Detected` / `Characterized` subscribe | [ ] read `register.go` (subscribe), `dispatch.go` -> `engine_event.go` (synchronous fan-out, outside the subscriber lock) |
| plugin -> shared firewall registry | `RegisterTables` + `ApplyAll` (two owners) | [ ] read: 8 call sites, 6 owners. Only `registry.go` calls `Backend.Apply` (Finding 5) |
| firewall registry -> kernel | nft backend `Apply` (netlink / shell-out) | [ ] read `backend_linux.go`: netlink via `google/nftables`, NOT shell-out. Batch staged then `Flush`; no mutex, no deadline |
| operator -> plugin engine | `ze-plugin-engine:dispatch-command` RPC | [ ] read `dispatch_registry.go` -> `dispatch.go` -> `command.go` `Dispatch`. No dispatcher mutex; blocking is per-handler only |
| dispatch-command -> ddos-local state | `RegisterRPCs` + `ze:command` YANG -> dispatcher -> `handleShowDdosLocal` -> `r.status()` | [ ] read `show.go,23,31`, `cmd/yang/ze-ddos-local-cmd.yang`, `command.go`. **This boundary was missing from the skeleton and is the one that matters** |

### Integration Points
- `internal/component/firewall/` registry, backend, engine (the shared, owning layer)
- `internal/plugins/firewall/nft/` the Linux backend doing the kernel reconcile
- `internal/plugins/ddos/local/` the second `ApplyAll` caller under flood
- `internal/component/plugin/server/` dispatch path that stalled
- Other `ApplyAll` callers that share the hazard: `internal/plugins/copp/register.go`,
  `internal/plugins/policyroute/register.go`, `internal/plugins/flowspec-firewall/engine.go`,
  `internal/plugins/anomaly/shape/responder.go`, `internal/component/firewall/plugins/irr/irr.go`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - the fix must not special-case ddos inside the
      generic firewall package (`ai/rules/plugins.md`)

## Problem / Evidence

**CONFIRMED (read in source, 2026-07-15):**
- `ApplyAll` calls `b.Apply(all)` with no firewall lock held (registry.go). Two
  concurrent callers (firewall engine + ddos responder) can therefore be inside
  `backend.Apply` at the same time. Whether the nft backend tolerates that is unverified.
- The ddos responder holds `r.mu` across the whole `applyAll` call, and across a second
  `applyAll` on the rollback path (responder.go): a lock held across a
  potentially slow kernel reconcile.
- `onDetected` and `onCharacterized` contend on `r.mu` (responder.go,74), so a
  sustained flood serialises repeated handlers behind each nft reconcile.
- ~~ddos-local registers no command handler of its own (no `OnCommand` under
  `internal/plugins/ddos/local/`), so `r.mu` is not directly on the `dispatch-command`
  path. The link between `r.mu` and the observed dispatch hang is NOT established.~~
  **SUPERSEDED 2026-07-16: false.** ddos-local publishes a handler via
  `RegisterRPCs{WireMethod: "ze-show:ddos-local"}` (`show.go`), not `OnCommand`.
  `LoadBuiltinsWithAliases` (`command.go`) registers it into the dispatcher under
  the YANG path from `cmd/yang/ze-ddos-local-cmd.yang`, and `dispatch-command`
  resolves against that dispatcher (`dispatch_registry.go` -> `dispatch.go`).
  The handler calls `r.status()` (`show.go`) which takes `r.mu` (`responder.go`).
  `r.mu` IS on the dispatch-command path. See the Mistake Log.

**CONFIRMED by reading the producer (2026-07-16, design session):**

*Finding 1 -- concurrent `Backend.Apply` corrupts the reconcile (AC-4).* The nft backend
holds a single `*nftables.Conn` (`backend_linux.go`, no mutex). `Conn` buffers
every staged command in one shared slice `cc.messages` guarded by `cc.mu`
(`conn.go`). `Apply` stages deletes (`backend_linux.go`) and adds
into that shared batch, then calls `b.conn.Flush()`. `Flush`
(`conn.go`) sends **the whole batch, whoever staged it**, and clears it
(`cc.messages = nil`, `:246`). So with two owners inside `Apply`:
  - Owner A's `Flush` sends owner B's half-staged commands to the kernel.
  - Owner B's `Flush` then hits `if len(cc.messages) == 0 { return nil }`
    (`conn.go`) and **returns success having sent nothing**.
  - `b.applied` is written at `backend_linux.go` and read at `:88` with no
    synchronization: a genuine data race, and a Go `fatal error: concurrent map read and
    map write` is reachable.
This is not benign. It is silent lost-update plus a process-killing race.

*Finding 2 -- `Backend.Apply` can block forever (the stall mechanism).*
`Flush` holds `cc.mu` (`conn.go`) across dial, `SendMessages` and an
ack-receive loop. The receive is `nlconn.Receive()` inside
`receiveAckAware` (`conn.go`) with **no deadline**: ze constructs the Conn as
`nftables.New()` (`backend_linux.go`) passing no `AsLasting()` and no `SockOptions`,
so no socket deadline is ever set. If the kernel never acks (a crashed nft subsystem is
documented for this exact environment, `mk/test-integration.mk`), `Apply` never
returns.

*Finding 3 -- the blocking chain to `dispatch-command` (the observed symptom).* Fully
cited, every hop read:

| # | Hop | Cite |
|---|-----|------|
| 1 | detector goroutine emits `AttackDetected` | `dispatch.go` deliverEvent |
| 2 | engine fans out synchronously, outside the subscriber lock | `dispatch.go`, `engine_event.go` |
| 3 | `onDetected` takes `r.mu` | `responder.go` |
| 4 | `applyMitigation` -> `applyAll()` **still under `r.mu`** | `responder.go` |
| 5 | `firewall.ApplyAll` -> `b.Apply(all)` | `registry.go` |
| 6 | nft `Apply` -> `conn.Flush` -> `Receive` with no deadline | `backend_linux.go`, `conn.go` |
| 7 | operator `show ddos local` -> dispatch-command -> `handleShowDdosLocal` | `dispatch_registry.go`, `show.go` |
| 8 | -> `r.status()` -> `r.mu.Lock()` **blocks for as long as hop 6 blocks** | `show.go`, `responder.go` |

-> Decision: this is **head-of-line blocking on `r.mu`, not a deadlock**. Nothing is
waiting on a cycle; one goroutine is parked in an unbounded kernel read while holding a
lock the management plane needs. A goroutine dump would show hop 3-6 in `Receive` and
hop 8 in `sync.Mutex.Lock`, not two goroutines waiting on each other.

-> Decision: this also explains the ~255s. Go's mutex starvation mode hands `r.mu` to a
waiter after 1ms, so repeated flood events CANNOT starve `show` for 255s: the only way
to block that long is a single reconcile that does not return. 255s is consistent with a
harness timeout on top of an indefinite netlink wait, not with contention. That makes
A-3 (slow apply) too weak a framing: apply is not slow, it is **unbounded**.

*Finding 4 -- blast radius (AC-8).* Of the six other `ApplyAll` callers, the
"lock held across the reconcile" defect exists in exactly one more:

| Caller | Holds a lock across `ApplyAll`? | Cite |
|--------|-------------------------------|------|
| `internal/plugins/anomaly/shape/responder.go` | **Yes** (`revertAll`, "Caller holds mu") and `statusSnapshot` is a show handler taking the same lock | `:190,200`; `show.go`, `responder.go` |
| `internal/plugins/copp/register.go` | No: `mu.Lock` comes AFTER `ApplyAll` | `:185,197` vs `:203` |
| `internal/plugins/policyroute/register.go` | No: `mu.Lock` comes AFTER `ApplyAll` | `:200` vs `:211` |
| `internal/component/firewall/plugins/irr/irr.go` | No: `RUnlock` at `:190` precedes `ApplyAll` at `:203` | `:181-208` |
| `internal/plugins/flowspec-firewall/engine.go` | No lock involved | `:180` |
| `internal/component/firewall/engine.go` | No lock involved | `:322,383` |

All six DO share Finding 1 (concurrent apply), which is why that fix belongs in the
registry, not in any plugin.

*Finding 5 -- `Backend.Apply` has exactly one caller.* `registry.go` is the only
`b.Apply(...)` in the tree; every `firewall.GetBackend()` consumer
(`audit.go`, `nft/cmd_show.go`, `web/page_firewall.go`, `firewall/cli/main.go`)
uses read-only paths, and those read paths use their own transient netlink conn
(`nftables/table.go-...` via `cc.netlinkConn()`, `conn.go`) and never stage
into `cc.messages`. So serialising inside `ApplyAll` is sufficient; there is no bypass.

**OBSERVED (QEMU run, 2026-07-12, not reproduced since):**
- `ze-plugin-engine:dispatch-command` stopped responding for roughly 255 seconds with a
  `firewall { backend nft }` block configured while ddos-local mitigation was active
  under a sustained flood.

**UNVERIFIED:**
- The root cause. No goroutine dump, no lock-ordering trace, no nft timing captured.
- Whether this is a true deadlock (a cycle) or livelock / starvation (repeated reconciles
  under flood starving the dispatch path).
- Whether nft `Apply` blocks on a kernel or netlink lock held by another actor.
- Whether the 255s was bounded by a harness timeout rather than genuine recovery.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The hang reproduces under QEMU with the same config shape | observed once during spec-ddos-direction-allowlist QEMU work (deferrals.md) | no reproduction: the spec degrades to a lock-discipline audit with a weaker outcome | re-run flood + `firewall { backend nft }` under QEMU | unvalidated (needs a trustworthy kernel: see R-5/R-6) |
| A-2 | The contention lives in shared firewall infra, not the ddos plugin | deferrals.md calls it "potential real concurrency bug in shared firewall infra"; `ApplyAll` is the only shared mutable path both actors touch | fix belongs in ddos-local or the plugin engine; AC-5 must change | goroutine dump showing blocked stacks | **broken (partially), 2026-07-16**: BOTH are true. The concurrent-apply corruption is shared infra (`registry.go` + `backend_linux.go`, Finding 1); the dispatch-stall mechanism is ddos-local's own lock discipline (`responder.go,136` + `show.go`, Finding 3). AC-5 stands for the registry fix but must not be read as forbidding the plugin-side fix |
| A-3 | nft `Apply` can be slow enough under flood to matter | inference from the ~255s stall; the nft backend reaches the kernel (backend_linux.go) | the stall is a genuine lock cycle, not slow-call-under-lock; fix is lock ordering | time `backend.Apply` under load | **broken (too weak), 2026-07-16**: `Apply` is not merely slow, it is UNBOUNDED. `receiveAckAware` -> `nlconn.Receive()` (`conn.go`) has no deadline and `backend_linux.go` sets no `SockOptions`. Timing it under load would have measured the wrong property |
| A-4 | `dispatch-command` shares a goroutine or lock with the blocked path | symptom is a dispatch hang while firewall work is in flight | the dispatch hang has an unrelated cause and this spec is scoped wrong | read dispatch_registry.go and the engine dispatch model | **broken as stated / confirmed when refined, 2026-07-16**: no shared goroutine and no dispatcher lock (`command.go` has no mutex; handlers run outside `engineEventSubscribers.mu`, `engine_event.go`). But it DOES share `r.mu` via `handleShowDdosLocal` (`show.go`). Consequence: only ddos-local's own commands stall, not all dispatch |
| A-5 | Concurrent `b.Apply` from two owners is a real hazard, not benign | registry.go holds no lock across the call | the unlocked call is fine and only the dispatch path matters | nft backend concurrency review + concurrent-apply test | **confirmed, 2026-07-16**: `Conn.Flush` sends the shared batch and clears it (`conn.go`); the loser's `Flush` returns nil having sent nothing; `b.applied` map is raced (`backend_linux.go` vs `:88`) |
| A-6 | The command observed to stall was one that takes `r.mu` (`show ddos local`) | Finding 3 is the only chain from a flood to a blocked dispatch handler that the code supports | the observed stall has a different producer and Finding 3, though a real bug, is not THE bug; root cause stays open | the 2026-07-12 run's test log / `.ci` file: which command did it dispatch? Then a goroutine dump on the repro | unvalidated -- **the one keystone fact still missing** |
| A-7 | A per-operation netlink deadline is achievable without changing the `Backend` interface | `dialNetlink` applies `cc.sockOptions` on EVERY dial (`conn.go`), and ze's Conn is non-lasting (`backend_linux.go` omits `AsLasting()`), so each `Flush`/list dials fresh and a `WithSockOptions` closure can compute `SetDeadline(time.Now().Add(N))` per operation | fall back to a watchdog goroutine or a lasting conn with an explicit deadline per op; the interface stays unchanged either way | confirmed by reading `dialNetlink`; prove with a test that a stalled socket returns an error | **confirmed, 2026-07-16** |
| A-8 | No caller holds `tableRegistry.mu` or `backendsMu` when calling `ApplyAll` (prerequisite for making a new reconcile lock the OUTERMOST firewall lock) | all 8 call sites read: `engine.go,383,395`; `registry.go` (`FlushAllTables` unlocks at `:43` first); `copp:185,197,199`; `policyroute:200,207,222`; `flowspec-firewall:180`; `irr:203`; `ddos/local:136,142,189`; `anomaly/shape:200` | the proposed `reconcileMu` would self-deadlock or invert the order; serialisation must move elsewhere | re-grep at implement time; the lock-order comment on `ApplyAll` documents it thereafter | **confirmed, 2026-07-16** |
| A-9 | ddos-local's mitigation ordering does not require `r.mu` to span the reconcile | `applyMitigation` only needs the lock to (a) read `cfg`, (b) serialise concurrent mitigations, (c) publish `active`/`target`. Only (b) needs to span the reconcile; `status()` reads only (c) | splitting the lock reorders concurrent mitigations; keep one lock and instead make `status()` read an atomic snapshot | design review + `TestResponderStatusDuringSlowApply` | unvalidated (settled by design D-3, proven by the test) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Not reproducible: root cause never established | QEMU re-run does not stall | fall back to a lock-discipline audit; fix the evidenced hazards on their own merits; do NOT claim the deadlock is fixed |
| R-2 | Serialising reconciles trades a deadlock for a longer stall | dispatch latency rises after the fix | measure before and after; prefer a single reconcile actor / queue over a coarse lock |
| R-3 | Fix regresses the autoload or withdraw-no-op paths | the three registry_test.go contract tests fail | keep them green throughout; they encode the contract |
| R-4 | Fix serialises mitigation behind config reloads, delaying a drop under attack | mitigation install latency rises under flood | measure install latency under load; consider a non-blocking mitigation path |
| R-5 | The repro never runs: `make ze-qemu-all-test` SKIPS the `firewall` suite by default (`mk/test-integration.mk` `ZE_QEMU_SKIP_SUITES ?= web,firewall`, passed through at `:239`; the script default agrees at `scripts/evidence/qemu-all-tests.sh`). A session reproducing under QEMU with the default target exercises no firewall `.ci` at all and may read the silence as "cannot reproduce" | QEMU run reports the firewall suite skipped, or finishes suspiciously fast | override it (`make ze-qemu-all-test ZE_QEMU_SKIP_SUITES=web`), or use `ze-qemu-needs-linux-test`, which hardcodes `ZE_QEMU_SKIP_SUITES="web"` and so DOES run firewall (`:248-250` names firewall explicitly). Verified by reading the targets, 2026-07-16 |
| R-6 | **The observed "deadlock" may be entangled with a known kernel crash, not only a Go lock hazard.** `mk/test-integration.mk` states the firewall suite is skipped by default because "firewall crashes the Alpine QEMU kernel on nft set-element-timeout operations". The deferral's symptom (dispatch unresponsive ~255s under a sustained flood with `firewall { backend nft }`) was observed in exactly that environment, so an unresponsive daemon there is not automatically a Go-level deadlock | the stall reproduces only under Alpine QEMU and never on real Linux or with a non-nft backend | before concluding anything about `ApplyAll` locking, establish WHERE the repro runs: real Linux vs Alpine QEMU, nft vs another backend. Note the two QEMU targets disagree about whether firewall is safe to run, which is itself unresolved. `plan/spec-fixit-qemu-runtime-kernel.md` owns moving the QEMU targets onto ze's own 7.1.1 kernel and un-skipping firewall; if it lands first this spec inherits a trustworthy repro environment and R-5/R-6 both fall away. Prefer waiting for it over debugging `ApplyAll` on a kernel ze itself declares unsupported (`tools/kernel-builder/build.py` refuses < 7.0). **Re-verified 2026-07-16** (line numbers hold: comment at `mk/test-integration.mk`, default at `:220`, pass-through at `:239`, `ze-qemu-needs-linux-test` pins `ZE_QEMU_SKIP_SUITES="web"` at `:261`). **Design-session update:** R-6 is now MORE than a caveat, it is corroborating evidence. Finding 2 shows a wedged nft subsystem makes `Apply` block forever (no netlink deadline), and `mk/test-integration.mk` documents that this exact kernel crashes on nft ops. That is a coherent joint story: the Alpine kernel crash wedges the netlink ack, `Apply` never returns, `r.mu` is held forever, `show ddos local` hangs until the harness gives up at ~255s. If so the Go-side bugs (Findings 1-3) are REAL and worth fixing, but the observation was triggered by the kernel, and the same test on 7.1.1 may simply pass. Both fixes are still correct: a crashed kernel must not wedge ze's management plane | 
| R-7 | Fix covers ddos only and leaves the other five `ApplyAll` callers exposed (renumbered from a duplicate `R-5`, 2026-07-16) | review finds copp / policyroute / flowspec unchanged | fix at the registry layer so every owner benefits. Finding 4 settles the split: the CONCURRENT-APPLY fix must be in `ApplyAll` (all six callers); the LOCK-ACROSS-RECONCILE fix is needed in exactly two plugins (ddos-local, anomaly-shape) |
| R-8 | **The deadline fix converts a hang into a failed mitigation.** Bounding `Apply` (D-2) means a wedged kernel now returns an error instead of blocking. ddos-local's failure path then rolls back and calls `applyAll` a SECOND time (`responder.go`), which will also time out: an attack goes unmitigated in the time it takes to fail twice | mitigation install latency under a wedged kernel equals 2x the deadline | choose the deadline so 2x is still well inside the detector's re-fire interval; do not retry the rollback on a timeout (the registry state is already correct); log + metric so the operator sees the kernel is wedged rather than a silent no-drop |
| R-9 | **Root cause is not established and this design may fix the wrong thing.** Findings 1-3 are read from the producing code and are real bugs on their own merits, but the LINK to the observed 255s stall rests on A-6 (unvalidated): nobody knows which command was dispatched | the repro stalls in a path Finding 3 does not cover | do not claim the observed hang is fixed until a goroutine dump matches Finding 3. AC-1 permits the honest alternative: fix the evidenced hazards, state that the 2026-07-12 observation remains unreproduced, and keep the deferral row open |
| R-10 | The `nftables` library is the real owner of the batch semantics (Finding 1). A future `go get -u` could change `Flush` behavior under us | `conn.go` Flush semantics change on upgrade | the registry-level serialisation (D-1) does not depend on library internals: it holds regardless of whether `Conn` is batch-shared. Do NOT build the fix on "Conn is safe if we only stage under a lock" |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `firewall { backend nft }` configured while ddos-local mitigates under flood | -> | shared firewall apply path stays responsive | `test/plugin/ddos-firewall-concurrency.ci` (conditional on a repro: see Known Limitations) |
| Two owners call `ApplyAll` concurrently | -> | `internal/component/firewall/registry.go` ApplyAll serialisation (D-1) | `TestApplyAllConcurrentOwnersConverge`, `TestApplyAllSerialisesBackendApply` |
| Operator command during an active nft reconcile | -> | `ze-plugin-engine:dispatch-command` bounded latency | `test/plugin/ddos-firewall-concurrency.ci` |
| `show ddos local` while the responder is mid-reconcile | -> | `show.go` -> `responder.go:status()` no longer waits on the reconcile (D-3) | `TestResponderStatusDuringSlowApply` (the in-process proof; does not need QEMU) |
| `show anomaly-shape` while the shape responder is mid-reconcile | -> | `shape/show.go` -> `statusSnapshot()` (D-4) | `TestShapeStatusDuringSlowApply` |
| Kernel never acks a netlink batch | -> | `backend_linux.go` newBackend deadline (D-2) | `TestNftApplyDeadlineSurfacesError` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Research phase complete | Root cause identified and evidenced (goroutine dump or lock trace showing blocked stacks), or an evidenced statement that it cannot be reproduced. -> Constraint: "no cycle exists" is now evidenced (Current Behavior lock-order table); what remains unevidenced is which command stalled (A-6). Do not close AC-1 by pointing at Findings 1-4: they are code-read evidence of hazards, not evidence about the 2026-07-12 event |
| AC-9 | The lock-order contract | Documented at the owning layer: `reconcileMu` -> `tableRegistry.mu` -> `backendsMu` -> backend-internal, with the rule that no owner may hold a lock across `ApplyAll` that a command handler also takes (D-5) |
| AC-10 | A wedged kernel (netlink never acks) | `Backend.Apply` returns a timeout error within the deadline; the daemon's management plane stays responsive; the failure is logged and counted (D-2, AC-6's observable-failure requirement) |
| AC-2 | A reproduction exists | A deterministic test (Go or `.ci`) reproduces the stall before the fix and passes after it |
| AC-3 | `firewall { backend nft }` configured while ddos-local mitigates under a sustained flood | `ze-plugin-engine:dispatch-command` keeps responding within a bounded time; no multi-second stall |
| AC-4 | Concurrent `ApplyAll` from two owners (firewall engine + a plugin) | The kernel converges to the merged desired state; no lost update, no interleaved partial apply |
| AC-5 | The fix lands | It lands at the owning layer (shared firewall infra), not as a workaround in ddos-local or in the `.ci` tests (`ai/rules/completion.md`) |
| AC-6 | Contention proves inherent (fix not possible) | The constraint is documented at the owning layer with evidence, and the failure is observable (log / metric / bounded timeout) rather than a silent hang |
| AC-7 | ddos-direction `.ci` workaround | Re-evaluated: if the hang is fixed, the tests assert the on-host drop rather than only direction classification, or the spec records why not |
| AC-8 | Every other `ApplyAll` caller (copp, policyroute, flowspec-firewall, anomaly/shape, irr) | Benefits from the same fix; none regress |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator runs a CLI command while the box is mitigating a flood with a firewall block configured | command -> dispatch-command -> plugin engine -> response | `test/plugin/ddos-firewall-concurrency.ci` |
| 2 | Operator reloads firewall config while ddos-local holds a drop table | OnConfigApply -> RegisterTables + ApplyAll alongside the responder's | `test/plugin/ddos-firewall-concurrency.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyAllSerialisesBackendApply` | `internal/component/firewall/registry_test.go` | AC-4 / D-1: a fake backend whose `Apply` asserts a non-overlap counter (enter/exit) never sees 2. RED before D-1 | |
| `TestApplyAllConcurrentOwnersConverge` | `internal/component/firewall/registry_test.go` | AC-4 / D-1: N goroutines register different owners and `ApplyAll`; the LAST `Apply` the fake backend receives contains every owner's tables (no stale snapshot wins) | |
| `TestApplyAllStaleSnapshotNotApplied` | `internal/component/firewall/registry_test.go` | D-1 rationale: proves the "lock only around `b.Apply`" alternative is insufficient. Register A, start apply, register B mid-flight; assert the kernel never ends on the A-only set | |
| `TestApplyAllContractTestsStayGreen` (existing three) | `internal/component/firewall/registry_test.go,65,89` | R-3: autoload, no-backend no-op, not-loaded error unchanged under `reconcileMu` | |
| `TestResponderStatusDuringSlowApply` | `internal/plugins/ddos/local/responder_test.go` | AC-3 / D-3: with `applyAll` stubbed to block on a channel, `status()` returns promptly. RED today (blocks on `r.mu`) | |
| `TestShapeStatusDuringSlowApply` | `internal/plugins/anomaly/shape/responder_test.go` | D-4: same assertion for anomaly-shape | |
| `TestResponderRollbackDoesNotRetryOnTimeout` | `internal/plugins/ddos/local/responder_test.go` | R-8: a timed-out apply does not trigger a second blocking `applyAll` | |
| `TestNftApplyDeadlineSurfacesError` | `internal/plugins/firewall/nft/*_linux_test.go` (integration) | AC-6 / D-2: a socket that never acks yields a timeout error, not a block | |

-> Constraint: the fake-backend tests must drive `firewall.ApplyAll` through the real
registry (`RegisterBackend` + `defaultBackendForAutoload`, the seam `registry_test.go`
already uses). A test that calls `b.Apply` directly proves nothing: `registry.go` is
the only production caller (Finding 5), so bypassing it tests nothing that ships.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| nft netlink deadline (D-2), seconds | 1..60, default 10 | 60 | 0 (means "no deadline": reject, that is today's bug) | 61 |
| `show ddos local` response bound under a wedged kernel (AC-3) | must be independent of the deadline: D-3 removes the coupling entirely | - | - | - |

-> Decision: the deadline default (10s) must satisfy R-8: 2x deadline (apply + rollback)
must stay inside the detector's re-fire interval. Verify that interval at implement time
against `internal/plugins/ddos/detect/` before fixing the number. If it is under 20s,
either lower the default or drop the rollback retry (R-8's preferred mitigation).
-> Constraint: the deadline is a constant, not a config leaf. It is a safety backstop,
not a tuning knob, and `ai/rules/config.md` reserves YANG for operator intent.
Revisit only if a real deployment needs it.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-firewall-concurrency.ci` | `test/plugin/` | firewall block + ddos-local mitigation under flood; assert dispatch-command stays responsive and the drop is installed (needs-linux / QEMU) | |

### Race / Stress
| Check | Command | Status |
|-------|---------|--------|
| Race detector on the concurrent-apply tests | `go test -race ./internal/component/firewall/ ./internal/plugins/ddos/local/` | |

## Files to Modify

Settled by research (2026-07-16). Every "only if" from the skeleton is now resolved:

| File | Change | Why (decision) |
|------|--------|----------------|
| `internal/component/firewall/registry.go` | add `reconcileMu`, acquire at the top of `ApplyAll` and hold to return; document the lock order L0 -> L1 -> L2 | D-1. Fixes Finding 1 for all six callers |
| `internal/component/firewall/backend.go` | doc only: state the single-writer + must-not-block-indefinitely contract on `Backend.Apply`; record that `backendsMu` sits UNDER `reconcileMu` | D-5. No lock-ordering change is needed: A-8 confirms nothing holds L1/L2 into `ApplyAll` |
| `internal/plugins/firewall/nft/backend_linux.go` | `newBackend`: pass `nftables.WithSockOptions` setting a per-dial deadline; surface a timeout as a typed error | D-2. Fixes Finding 2 |
| `internal/plugins/ddos/local/responder.go` | publish `active`/`target` via an atomic snapshot; `status()` stops taking `r.mu`; do not re-`applyAll` on a timed-out rollback | D-3, R-8. Fixes Finding 3 |
| `internal/plugins/anomaly/shape/responder.go` | same shape as D-3 for `statusSnapshot` vs `revertAll` | D-4. Fixes Finding 4's second instance |
| `docs/architecture/core-design.md` | document the firewall reconcile concurrency contract (single reconcile at a time; `Apply` is bounded; owners must not hold a lock a command handler needs across `ApplyAll`) | D-5, AC-6 |
| ~~`internal/component/plugin/server/dispatch_registry.go`~~ | **NOT modified.** The dispatch path is not the blocking layer: no dispatcher mutex (`command.go`), handlers invoked outside the subscriber lock (`engine_event.go`) | A-4 broken as stated |

Metric/observability surface (AC-6), to settle at implement time: an apply-latency
histogram plus a reconcile-timeout counter in the firewall component. Prefer the
existing `internal/core/metrics` registry over a new mechanism (`ai/rules/architecture.md`).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | N/A | no new config surface expected |
| CLI commands/flags | N/A | no new command |
| Functional test for the fix | Yes | `test/plugin/ddos-firewall-concurrency.ci` |
| Doctor check for runtime dependencies | N/A | no new runtime dependency |
| Prometheus counters/metrics | [ ] | consider an apply-latency / contention metric during design (AC-6 observability) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | likely No (bug fix); confirm at design |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` (firewall registry concurrency contract) |
| 13 | Known constraint documented (AC-6 path)? | [ ] | `docs/architecture/core-design.md` + the owning package doc comment |

## Files to Create
- `test/plugin/ddos-firewall-concurrency.ci` (needs-linux; the QEMU reproduction)
- unit tests listed above, in the existing `_test.go` files

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Risks & Assumptions (validate A-1..A-5 first) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 13. /ze-review gate | Review Gate section |
| 14. Present + close | Executive Summary; two-commit closure |

### Implementation Phases

Revised 2026-07-16 after research. The skeleton's "no fix until the stacks are in hand"
gate is kept for the OBSERVED stall (phase 1) but must not block phases 2-4: Findings
1-4 are read from the producing code and stand on their own merits, which the Scope
section already admits as IN.

1. **Phase: Reproduce + capture (A-6, the one open keystone fact)** - find the 2026-07-12
   run's dispatched command first (cheap: read the `.ci` from `spec-ddos-direction-allowlist`
   and its log). Then drive flood + `firewall { backend nft }` under QEMU and capture a
   goroutine dump (SIGQUIT / pprof) during the stall. Expect hop 3-6 of Finding 3 parked
   in `Receive` and hop 8 in `sync.Mutex.Lock`. **Read R-5/R-6 before running anything**:
   with the default target the firewall suite does not run at all, and the stock Alpine
   kernel is itself a suspect. Prefer waiting for `plan/spec-fixit-qemu-runtime-kernel.md`.
   -> Decision: if the dump matches Finding 3, root cause is established (AC-1). If it
   does not, STOP and report: the design below fixes real bugs but not the observed one
   (R-9), and AC-1's "evidenced statement that it cannot be reproduced" is the honest exit.
2. **Phase: Registry serialisation (D-1)** - RED `TestApplyAllSerialisesBackendApply` +
   `TestApplyAllConcurrentOwnersConverge`, then add `reconcileMu`. The three existing
   contract tests (`registry_test.go,65,89`) stay green throughout (R-3).
3. **Phase: Bound the kernel call (D-2)** - deadline in `newBackend`; typed timeout error;
   integration test. This is the AC-6 observability branch, not a fallback: a wedged
   kernel becomes a logged, counted error instead of a silent hang.
4. **Phase: Unblock the management plane (D-3, D-4)** - atomic status snapshot in
   ddos-local and anomaly-shape; RED `TestResponderStatusDuringSlowApply` first.
5. **Phase: Contract + docs (D-5)** - lock-order comment, `Backend.Apply` doc,
   `core-design.md`. Report the `EngineEventHandler` "MUST NOT block" gap under
   `ai/rules/repo-maintenance.md`.
6. **Phase: Prove** - `.ci` reproduction red-before / green-after if phase 1 produced one;
   race detector on both packages; AC-8 re-grep of all six callers.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | No lost update across owners; kernel converges; no lock held across a kernel call without justification |
| Concurrency | Lock order documented; no cycle; race detector clean |
| Blast radius | Every `ApplyAll` caller considered, not just ddos |
| No workaround | The `.ci` tests assert real behavior; the fix is not a sleep or a test relaxation |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Dispatch stays responsive under flood + firewall block | `bin/ze-test plugin --pattern ddos-firewall-concurrency` |
| Concurrent apply converges | `go test -race -run TestApplyAllConcurrentOwnersConverge ./internal/component/firewall/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Availability | A flood must not be able to wedge the management plane: that is the whole point of this spec |
| Resource exhaustion | Repeated reconciles under flood must not grow unbounded queues |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Cannot reproduce the stall | R-1 fallback: lock-discipline audit; report to user before proceeding |
| Test fails behavior mismatch | Re-read Current Behavior; RESEARCH if the blocked path was wrong |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `internal/plugins/firewall/engine.go` was the config-driven apply site (deferrals.md) | No such file: `internal/plugins/firewall/` holds only `nft/` and `vpp/` backends. The engine is `internal/component/firewall/engine.go`, with the LoadBackend + RegisterTables + ApplyAll sequence at :316-324 and :372-395 | Spec authoring, 2026-07-15: grepped for the symbols after the path failed to resolve | Citation corrected here; the deferral row's path is wrong |
| "ddos-local registers no command handler (no `OnCommand`), so `r.mu` is not on the dispatch-command path" -- recorded as CONFIRMED in the skeleton's Problem / Evidence | ddos-local DOES publish a dispatch-command handler, via `RegisterRPCs{WireMethod: "ze-show:ddos-local"}` + a `ze:command` YANG node (`show.go`, `cmd/yang/ze-ddos-local-cmd.yang`), registered into the dispatcher at `command.go`. Its handler takes `r.mu` (`show.go` -> `responder.go`). `r.mu` is squarely on the dispatch-command path and is the most probable stall mechanism (Finding 3) | Design session, 2026-07-16: grepped `ze-show:ddos-local` across the tree while tracing the dispatch model, after noticing `status()`'s doc comment says "for the show handler" (`responder.go`) | **Severe.** A false CONFIRMED pointed the whole spec away from the one chain the code actually supports. It also inverted the fix location: the skeleton concluded the ddos-side lock was NOT the dispatch problem, so a session implementing from the skeleton alone would have serialised the registry, seen the stall persist, and had no lead left |
| Grepping `OnCommand` establishes whether a plugin has command handlers | `OnCommand` is the SDK-side (out-of-process plugin) callback. In-process plugins publish commands via `pluginserver.RegisterRPCs` + a `ze:command` YANG node. Both reach the same dispatcher (`command.go`) | Same trace, 2026-07-16 | Generalised into a Required Reading `-> Constraint:` so the next session does not repeat it. Candidate for `ai/rules/repo-maintenance.md` at closure: "to find a plugin's command surface, grep its `yang/*-cmd.yang` for `ze:command`, not `OnCommand`" |
| A dispatch stall under a flood implies contention/starvation on the flood path | Go's `sync.Mutex` enters starvation mode after 1ms and hands the lock to the longest waiter, so repeated flood events cannot starve a `status()` waiter for 255 seconds. A multi-second stall on a mutex implies the HOLDER is blocked, not that waiters are being out-raced. That reframes the search from "too much contention" to "what does the holder block on", which is what led to the missing netlink deadline (Finding 2) | Design session, 2026-07-16, reasoning about A-3's plausibility before accepting it | A-3 was framed as "slow apply". Timing `Apply` under load (its stated validation method) would have measured latency and found it acceptable, confirming a false negative |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- `ApplyAll` deliberately drops both locks before `b.Apply` (registry.go,111,123). That
  keeps the registry lock off the kernel path but permits two owners inside `Apply` at
  once. Whichever way the deadlock resolves, this is the design tension to settle.
- **The tension is settled by reading the backend: dropping the lock was wrong.** The
  comment logic ("do not hold a registry lock across a kernel call") is sound, but the
  conclusion "so hold NO lock" does not follow. The correct shape is a DEDICATED
  reconcile lock that is not the registry lock. `registry.go,111` may still release
  L1/L2 early; what is missing is L0 above them. (2026-07-16)
- The `ze_*` sweep makes concurrent apply worse than a plain lost update.
  `shouldDeleteTable` (`backend_linux.go`) deletes any `ze_*` table in
  `b.applied`, so an interleaved apply can DELETE the other owner's live table and never
  re-add it, because the re-add was staged into a batch that a foreign `Flush` already
  drained (`conn.go`). Two owners is not "both writes land, one wins": it is "the
  drop rule silently disappears from the kernel while the registry says it is there".
- The `Backend` interface doc (`backend.go`) is precise about ownership ("Non-ze_*
  tables MUST NOT be touched") and silent about concurrency. Every implementer therefore
  reasonably assumed single-threaded. The nft backend IS correct code against an
  unstated single-writer contract. The bug is that the contract was never stated or
  enforced. (2026-07-16)
- `EngineEventHandler` already documents "MUST NOT block on external I/O"
  (`engine_event.go`). ddos-local's `onDetected` does a netlink round trip inside
  the handler. The rule exists; nothing enforces it. Worth reporting under
  `ai/rules/repo-maintenance.md`: a documented contract with no mechanical gate was
  violated by two of two plugins that had the opportunity (ddos-local, anomaly-shape).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| **D-1: serialise the ENTIRE `ApplyAll` body under a new package-level `reconcileMu` in `registry.go`, acquired before L1** | (a) lock only around `b.Apply(all)`; (b) put a mutex in each backend; (c) a single reconcile goroutine + queue | (a) is insufficient: the snapshot is taken at `:80-94` and applied at `:123`, so two callers can apply STALE snapshots out of order and converge to a superseded state. Snapshot and apply must be atomic together. (b) makes every backend reimplement it, and fixes the race but not the stale-snapshot lost update, which lives in the registry, not the backend. (c) is the better long-term shape (it decouples the caller from the kernel latency entirely) but it changes `ApplyAll` from sync to async, so callers lose the error return: `engine.go` and the journal rollback at `:395` depend on it. Rejected as too large for a fixit. A-8 confirms `reconcileMu` can be the outermost firewall lock without inverting any existing order |
| **D-2: bound the kernel call in the nft backend with a per-operation netlink deadline** via `nftables.New(nftables.WithSockOptions(...))` setting `SetDeadline(time.Now().Add(N))` | (a) add `context.Context` to `Backend.Apply`; (b) watchdog goroutine that closes the conn; (c) leave it unbounded and rely on D-1 + D-3 | (a) is the ze-idiomatic answer (`ai/rules/go-standards.md` Context) and `traffic`'s backend already takes a ctx (`traffic/register.go`), but `google/nftables` has no ctx-aware API, so the ctx would only be checked between calls and could not interrupt the blocked `Receive`. It would look like a fix and not be one. (b) reinvents what `SetDeadline` does. (c) leaves a crashed kernel able to wedge every firewall owner forever, which is exactly AC-6's "silent hang". A-7 confirms (D-2) works per-operation because ze's Conn is non-lasting and `dialNetlink` re-applies sockOptions on every dial |
| **D-3: in ddos-local, keep ONE mutex serialising mitigations but move the `status()` snapshot off it** (publish `active`/`target` via an atomic snapshot pointer, as `register.go` already does for the responder itself) | (a) compute the table under `r.mu`, release, reconcile outside; (b) split into `reconcileMu` + `stateMu`; (c) leave it (D-1 + D-2 bound the wait anyway) | (a) breaks mitigation ordering: two concurrent narrows could reconcile out of order and install the older term (A-9). (b) is (a) with extra steps: two locks, and `status()` still needs the newer of the two. (c) still makes `show ddos local` latency equal to a full kernel reconcile even in the healthy case, and up to the D-2 deadline in the sick case: the management plane must not be coupled to kernel latency at all. The atomic-snapshot shape is already the house pattern here (`accessor.go`, `register.go`) |
| **D-4: apply D-3 to anomaly-shape in the same change** (`shape/responder.go,200` + `:235`) | fix ddos-local only; file a follow-up for shape | Same bug, same shape, found by the same read (Finding 4). A follow-up row for a two-line change is deferral theatre (`ai/rules/planning.md`), and AC-8 already requires the other callers not to regress |
| **D-5: state the concurrency contract in code, not only in docs** -- a lock-order comment on `reconcileMu`, a "single-writer, never called concurrently, must not block indefinitely" clause on `Backend.Apply` (`backend.go`), and a `docs/architecture/core-design.md` note | doc-only; comment-only | The interface doc is where the next backend author looks (vpp is already a second implementation). Docs alone did not prevent this: `EngineEventHandler` documented "MUST NOT block on external I/O" and both responders block anyway |

## Known Limitations
- The root cause is an observed symptom only. Until a goroutine dump exists, every causal
  story in this spec is a hypothesis.
- **Refined 2026-07-16.** Two distinct claims must not be conflated:
  - **Findings 1-4 are findings, not hypotheses.** Each is read from the producing
    function and cited (`conn.go`, `backend_linux.go,77,88`,
    `responder.go,136,199`, `show.go`). They are real defects that ship today and
    are fixable on their own merits, which Scope already admits as IN.
  - **The link from Finding 3 to the 2026-07-12 observation is a hypothesis** (A-6, R-9).
    It is the only chain the code supports, and it is consistent with the 255s (mutex
    starvation mode rules out contention; an unbounded `Receive` explains an indefinite
    hold). Consistency is not verification. Do not write "the deadlock is fixed" in the
    learned summary. It is not a deadlock, and it may not be what was observed.
- The title of this spec is now wrong: there is no deadlock. Renaming mid-flight would
  break the deferral row's reference (`plan/deferrals.md`), so the name stays and the
  learned summary at closure carries the correction. -> Decision, 2026-07-16.
- The `.ci` reproduction (`test/plugin/ddos-firewall-concurrency.ci`) is conditional on
  phase 1 producing a repro. If the stall is kernel-triggered (R-6), a `.ci` that only
  passes on a crashing kernel is worse than none: it would encode the kernel bug as the
  expected environment. The unit tests (D-1, D-3) do not have this problem and are the
  primary proof.

## Implementation Summary
### What Was Implemented
- D-1: `reconcileMu` in `internal/component/firewall/registry.go` serializes the whole
  `ApplyAll` body, snapshot and apply together. Lock order documented on the mutex.
- D-2: a per-dial netlink deadline in `internal/plugins/firewall/nft/deadline_linux.go`
  (`netlinkTimeout`, `withNetlinkDeadline`, `asKernelTimeout`), installed by `newBackend`.
  Default 10s, clamped to 1..60s, and zero clamps up rather than disabling the bound.
- D-2 observability (AC-10 "counted"): `internal/component/firewall/metrics.go`
  (`observeApply`), called by `ApplyAll` and bound through `Registration.ConfigureMetrics`.
  `ze_firewall_apply_duration_seconds` on every reconcile, labeled by `result`
  (`ok`, `timeout`, `error`, `panic`); `ze_firewall_apply_timeout_total` plus an
  error log on `ErrKernelTimeout`. The observation is deferred, so a backend that
  panics is recorded rather than lost, and is not filed as healthy. It is
  registered BEFORE the `reconcileMu` unlock so LIFO runs it after the release:
  the timeout log is a syscall, and it must not extend the hold this spec exists
  to shorten. Buckets run to 60s, the largest deadline either backend accepts.
- `ErrKernelTimeout` means the dataplane accepted the request and went quiet. A
  dataplane that is ABSENT is excluded: the vpp connect wait also times out, but
  "VPP is not running" needs a different fix, and the sentinel's two consumers
  (the timeout counter, and ddos-local skipping its rollback reconcile) would
  both read it wrongly.
- D-2 for the SECOND backend: `internal/plugins/firewall/vpp/timeout_linux.go` bounds
  every VPP binary-API reply (`ze.firewall.vpp.reply-timeout`, default 10s, clamped
  1..60s) and tags a timeout as `ErrKernelTimeout`. govpp's `DefaultReplyTimeout` is 0,
  which it documents as disabling the timeout, and nothing called `SetReplyTimeout`, so
  the vpp backend had D-2's exact defect: an unbounded call held under `reconcileMu`.
  The deadline is bound inside `newGovppOps`, so no construction path can skip it.
- D-3: ddos-local publishes `{active, target}` through an atomic snapshot
  (`setStatus`), and `status()` takes no lock. `applyMitigation` skips the rollback
  reconcile on `ErrKernelTimeout` (R-8).
- D-4: the same shape in anomaly-shape (`publishStatus`, `statusSnapshot`).
- D-5: the single-writer and bounded-`Apply` contract on the `Backend` interface,
  `ErrKernelTimeout` on the contract rather than in a backend package, and the
  "Firewall reconcile concurrency" section of `docs/architecture/core-design.md`.

### Bugs Found/Fixed
- Concurrent `Backend.Apply` (Finding 1): two owners staged into one shared
  `nftables.Conn` batch, so one `Flush` drained both and the other returned success
  having sent nothing. Fixed at the registry, so all six `ApplyAll` callers benefit.
- Unbounded `Backend.Apply` (Finding 2): no netlink deadline anywhere, so a wedged
  kernel held `reconcileMu` forever.
- Head-of-line blocking on the management plane (Finding 3): `show ddos local` and
  `show anomaly-shape` waited on the same mutex the responder held across the reconcile.
- A wedged kernel was returned to the caller but never counted: AC-10's second half.
  Found while auditing this deferral row on 2026-08-07 and fixed here.
- The vpp backend had NO reply deadline at all (govpp defaults to 0, meaning
  disabled), so `backend vpp` kept the unbounded-`Apply` defect D-2 removed from nft,
  and the timeout counter could never leave 0 there. Found on 2026-08-07 while
  closing a review finding that read as a documentation gap and was not one.

### Documentation Updates
- `docs/architecture/core-design.md`, "Firewall reconcile concurrency": the lock order,
  the bounded-`Apply` obligation, `ErrKernelTimeout`'s home and why re-applying after a
  timeout is wrong, and the two metrics.

### Deviations from Plan
- The D-1 unit tests live in `registry_concurrency_test.go`, not `registry_test.go` as
  the TDD plan named. Same package, no functional difference.
- The metric surface the plan left "to settle at implement time" is one histogram plus
  one counter in the firewall component, with no new config leaf. It follows the
  deadline's reasoning: this is a safety and observability backstop, not a tuning knob.
- Phase 1 (reproduce the 2026-07-12 stall and capture goroutine stacks) was never run,
  so AC-1, AC-2, AC-3 and AC-7 are open and `test/plugin/ddos-firewall-concurrency.ci`
  does not exist. R-6 recommends waiting for `plan/spec-fixit-qemu-runtime-kernel.md`.
  This is a scope question for the owner, not a claim of completion.

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Root-cause the dispatch hang | Partial | Findings 1-4 | The mechanism the code supports is read and cited. The link to the 2026-07-12 event stays a hypothesis (A-6) |
| Fix it at the owning layer | Done | `internal/component/firewall/registry.go`, `backend.go`, `metrics.go` | D-1, D-5, AC-10 counter all live in the shared component |
| The lock-discipline hazards on their own merits | Done | `ddos/local/responder.go`, `anomaly/shape/responder.go`, `nft/deadline_linux.go` | D-2, D-3, D-4 |
| Make the failure observable rather than a silent hang | Done | `firewall/metrics.go`, `nft/deadline_linux.go` | Bounded, logged, counted |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Open | - | Needs phase 1, or the evidenced "cannot reproduce" statement. Findings 1-4 do not close it and the spec says so |
| AC-2 | Open | - | No deterministic reproduction of the stall exists |
| AC-3 | Partial | `TestResponderStatusDuringSlowApply`, `TestShapeStatusDuringSlowApply` | The in-process proof holds: a show handler no longer waits on a reconcile. The under-flood `.ci` is not written |
| AC-4 | Done | `TestApplyAllSerialisesBackendApply`, `TestApplyAllConcurrentOwnersConverge`, `TestApplyAllStaleSnapshotNotApplied` | Max concurrent `Apply` is 1; the last apply carries every owner |
| AC-5 | Done | `internal/component/firewall/registry.go` | Serialization is in the registry, not in any plugin or test |
| AC-6 | Done | `firewall/metrics.go`, `docs/architecture/core-design.md` | Bounded timeout, log, counter, and the contract written where a backend author reads it |
| AC-7 | Open | - | Depends on AC-2: the ddos-direction `.ci` workaround cannot be re-evaluated without a reproduction |
| AC-8 | Done | `TestApplyAllSerialisesBackendApply` plus the caller re-grep | All six `ApplyAll` callers inherit D-1. Only ddos-local and anomaly-shape needed the D-3 shape, and both have it |
| AC-9 | Done | `reconcileMu` doc comment, `Backend.Apply` doc, `core-design.md` | Lock order and the no-lock-across-`ApplyAll` rule are stated in code and in the doc |
| AC-10 | Done | `TestNetlinkTimeoutBounds`, `TestAsKernelTimeoutTagsDeadlineOnly`, `TestNftApplyDeadlineSurfacesError`, `TestApplyAllCountsKernelTimeout` | Bounded within the deadline, typed, logged, counted |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestApplyAllSerialisesBackendApply` | Done | `internal/component/firewall/registry_concurrency_test.go` | File name differs from the plan |
| `TestApplyAllConcurrentOwnersConverge` | Done | same | |
| `TestApplyAllStaleSnapshotNotApplied` | Done | same | |
| The three existing registry contract tests | Done | `internal/component/firewall/registry_test.go` | Green under `reconcileMu` (R-3) |
| `TestResponderStatusDuringSlowApply` | Done | `internal/plugins/ddos/local/responder_test.go` | |
| `TestShapeStatusDuringSlowApply` | Done | `internal/plugins/anomaly/shape/responder_test.go` | |
| `TestResponderRollbackDoesNotRetryOnTimeout` | Changed | `internal/plugins/ddos/local/responder_test.go` | Landed as `TestKernelTimeoutSkipsRollbackReconcile`, table-driven so an ordinary failure still gets its rollback reconcile |
| `TestNftApplyDeadlineSurfacesError` | Done | `internal/plugins/firewall/nft/deadline_integration_linux_test.go` | `integration && linux`, runs under `make ze-qemu-integration-test` |
| `TestApplyAllCountsKernelTimeout` | Added | `internal/component/firewall/metrics_test.go` | Not in the plan: AC-10's "counted" half had no test because it had no counter |
| `TestApplyAllRecordsAPanickingBackend` | Added | `internal/component/firewall/metrics_test.go` | Pins the deferred observation |
| `TestApplyAllObservesOutsideTheReconcileLock` | Added | `internal/component/firewall/metrics_test.go` | Pins WHERE it runs: the observer probes `reconcileMu.TryLock` and fails if the lock is still held |
| `TestVppReplyTimeoutBounds`, `TestNewGovppOpsBindsReplyTimeout`, `TestApplyTagsDataplaneTimeout` | Added | `internal/plugins/firewall/vpp/timeout_linux_test.go` | D-2 for the vpp backend; linux-only, run under `make ze-qemu-integration-test` |
| `firewall-metrics-registered.ci` | Added | `test/plugin/` | Proves `Registration.ConfigureMetrics` fires for the firewall component; needs-linux, caps=net-admin |
| `ddos-firewall-concurrency.ci` | Open | - | Conditional on phase 1 (Known Limitations) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/firewall/registry.go` | Done | `reconcileMu` plus the `observeApply` call |
| `internal/component/firewall/backend.go` | Done | Single-writer and bounded-`Apply` contract, `ErrKernelTimeout` |
| `internal/plugins/firewall/nft/backend_linux.go` | Done | `newBackend` installs the deadline; `asKernelTimeout` tags the error |
| `internal/plugins/ddos/local/responder.go` | Done | Atomic snapshot, no lock in `status`, no rollback retry on a timeout |
| `internal/plugins/anomaly/shape/responder.go` | Done | Same shape |
| `docs/architecture/core-design.md` | Done | "Firewall reconcile concurrency" |
| `internal/component/firewall/metrics.go` | Added | The AC-6 metric surface the plan left to settle at implement time |
| `internal/plugins/firewall/vpp/timeout_linux.go` | Added | D-2 for the vpp backend, so the timeout count means the same under both |
| `test/plugin/firewall-metrics-registered.ci` | Added | The metrics wiring proof |
| `test/plugin/ddos-firewall-concurrency.ci` | Open | Conditional on phase 1 |

### Audit Summary
- **Total items:** 10 AC, 10 planned tests, 8 files
- **Done:** AC-4, AC-5, AC-6, AC-8, AC-9, AC-10; 8 of 10 tests; 7 of 8 files
- **Partial:** AC-3 (in-process proof only, no under-flood `.ci`)
- **Skipped:** none
- **Open, needs an owner decision:** AC-1, AC-2, AC-7 and the `.ci`, all gated on the
  phase-1 reproduction (A-6, R-6)
- **Changed:** the rollback test name, the D-1 test file name, and one added test
  (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Dispatch no longer hangs under flood + firewall block | functional (QEMU) | NOT MET. `test/plugin/ddos-firewall-concurrency.ci` does not exist; it is conditional on phase 1 (Known Limitations) |
| A management-plane read is no longer coupled to kernel latency | unit, discrimination-checked | `TestResponderStatusDuringSlowApply` and `TestShapeStatusDuringSlowApply`. Restoring the lock in `status()` / `statusSnapshot()` makes each report "blocked behind the in-flight reconcile" (observed 2026-08-07) |
| A wedged kernel cannot stall every firewall owner forever | unit + QEMU integration | `TestNetlinkTimeoutBounds` (zero clamps up, never disables the bound), `TestNftApplyDeadlineSurfacesError` under `make ze-qemu-integration-test` |
| The failure is observable rather than silent | unit + functional | `TestApplyAllCountsKernelTimeout` (with the increment removed: "timeout counter = 0, want 1"), and `test/plugin/firewall-metrics-registered.ci`, which scrapes the daemon's Prometheus endpoint and finds both series (738ms PASS; 11.6s FAIL with `ConfigureMetrics` emptied) |
| The timeout count means the same under either backend | unit (QEMU) | `TestApplyTagsDataplaneTimeout` and `TestNewGovppOpsBindsReplyTimeout` for vpp, beside the nft pair. Both fail with their fix reverted (observed 2026-08-07) |
| Concurrent apply converges, no lost update | unit, race | `TestApplyAllSerialisesBackendApply` (max concurrent `Apply` 1, observed 8 without `reconcileMu`), `TestApplyAllConcurrentOwnersConverge`, `TestApplyAllStaleSnapshotNotApplied`, all under `-race` |
| Root cause established | trace | NOT MET. No goroutine dump exists. A-6 stays unvalidated and R-9 stands |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

**Scope of this review:** the `internal/component/firewall/*` slice only (D-1 registry
serialization + in-scope D-5 interface/lock-order docs). D-2/D-3/D-4/core-design.md/.ci are
disjoint sibling-agent scopes and reviewed under their own gates.

### Run 1 (initial) — 2 independent reviewer subagents
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `reconcileMu` now held across `b.Apply`, so a hung Apply stalls every firewall owner, not just concurrent reconciles. Intended single-writer design; makes the nft deadline (D-2) load-bearing | `registry.go` ApplyAll, `backend.go` Apply doc | acknowledged (documented in the Backend.Apply contract; D-2 is a sibling scope) |
| 2 | NOTE | `FlushAllTables` releases `tableRegistry.mu` before `ApplyAll` acquires `reconcileMu`; a plugin could re-add an owner in that gap during shutdown | `registry.go` | acknowledged (pre-existing shutdown-ordering assumption; engine is the single ordered actor at clean stop; out of scope for this fix) |
| 3 | NOTE | `TestApplyAllSerialisesBackendApply` RED signal relies on a 1ms sleep making overlap probable; GREEN invariant (`maxSeen==1`) is deterministic | `registry_concurrency_test.go` | acknowledged (test's job is to lock the GREEN invariant; observed max=8 without the lock) |
| 4 | NOTE | D-1 tests live in `registry_concurrency_test.go`, spec TDD plan named `registry_test.go` | same package | acknowledged (filename divergence only, no functional impact) |

### Fixes applied
- None required: both reviewers returned 0 BLOCKER, 0 ISSUE on the first pass. All four
  NOTEs are non-blocking and acknowledged above.

### Run 2 (2026-08-07) — independent reviewer

**Scope, fixed before the round ran:** the 2026-08-07 additions only, none of which
existed at Run 1: `internal/component/firewall/metrics.go` and its wiring
(`ApplyAll`, `ConfigureMetrics` in `register.go`), and the D-2/D-3/D-4 slices as
committed (`nft/deadline_linux.go`, `ddos/local/responder.go`,
`anomaly/shape/responder.go`). The D-1 registry serialization reviewed at Run 1 was
out of scope except where the new code touches it.

Upheld without change: `observeApply` is genuinely wired (`ApplyAll`, 11 production
call sites), and the timeout classification is correct (`errors.Is` against a
`%w: %w` wrap).

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 4 | ISSUE | The metrics wiring itself is untested. `metrics_test.go` calls `bindMetrics` directly; nothing drives `Registration.ConfigureMetrics` and no `.ci` asserts either series, so if the hook stops firing both metrics vanish permanently and every test stays green. Not hypothetical: `test/plugin/plugin-metrics-registered.ci` exists because internal plugins once silently skipped that hook, and it covers only `ze_role_*` | `internal/component/firewall/register.go`, `metrics_test.go` | FIXED. Added `test/plugin/firewall-metrics-registered.ci` (needs-linux, caps=net-admin), which scrapes the daemon's Prometheus endpoint and requires both series. Not folded into `plugin-metrics-registered.ci`: that test carries no `needs-linux`, so adding a `firewall{}` block would have pulled its `ze_role_*` coverage out of the merge gate |
| 6 | ISSUE | `ze_firewall_apply_timeout_total` can never increment under the vpp backend: `ErrKernelTimeout` is produced only in `nft/deadline_linux.go`. Under `backend vpp` the dataplane wedges, the histogram tail fills, and the counter reads 0 forever, while the help string and `core-design.md` both read backend-agnostic | `internal/plugins/firewall/vpp/`, `metrics.go` help string | FIXED, by making the claim true rather than narrowing it. Reading the producer showed this was not a wording gap: govpp's `DefaultReplyTimeout` is 0 (documented as disabling the timeout) and nothing called `SetReplyTimeout`, so vpp carried D-2's unbounded-`Apply` defect under the process-wide `reconcileMu`. Added `vpp/timeout_linux.go`: a clamped per-request deadline bound inside `newGovppOps`, and `asDataplaneTimeout` tagging the timeout |
| 7 | NOTE | Two smaller ones in `ApplyAll`: it times without `defer`, so a panic in a backend skips both the observation and the timeout log; and the histogram carries no result dimension, so a 10s timeout and a 10s slow success are indistinguishable | `internal/component/firewall/registry.go`, `metrics.go` | FIXED both. Observation deferred with the result starting at `panic`; histogram became a `HistogramVec` labeled `result` (`ok`/`timeout`/`error`/`panic`), with the counter and the label derived from one value so they cannot drift |

**Discrimination observed for each fix:** `.ci` PASS 738ms, and with `ConfigureMetrics`
emptied it polls its whole budget and fails at 11.6s. vpp: "SetReplyTimeout was never
called" with the binding removed; `errors.Is(err, ErrKernelTimeout) = false, want true`
with the tagging removed. Panic path: `observations under result="panic" = 0, want 1`
without the defer. Label: `apply-duration observations under result="timeout" = 0, want 1`
with the label collapsed.

### Run 3 (2026-08-07) — independent reviewer

**Scope, fixed before the round ran:** the Run 2 fixes themselves, verified against
govpp's own source rather than against the implementer's account of it.

Upheld without change: the item-6 diagnosis (confirmed stronger than claimed, with
`(*Channel).receiveReplyInternal` substituting `maxInt64` for a non-positive timeout
and `ReceiveReply` having no context arm); the `newGovppOps` single-construction
claim; the deferred observation registering after every early return; the counter
and log gating on one `result` value; and both backends' bounds matching the doc.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | The new `.ci`'s control is a guard that can never deny: the `families` check runs only AFTER the `series_present` wait succeeds, and the missing-series branch exits first, so it cannot execute in the one case its own comment names | `test/plugin/firewall-metrics-registered.ci` | FIXED. Moved above the series wait, behind a `fetch()` wait that only requires the endpoint to answer. Observed on the failure path: `CONTROL: telemetry on, 71 non-firewall families exposed` prints, then the series diagnosis, so the failure is attributed to the hook rather than to telemetry |
| 2 | ISSUE | The same govpp defect exists in `internal/plugins/traffic/vpp/backend_linux.go`, which builds `&govppOps{ch: ch}` with no `SetReplyTimeout`. Narrower blast radius (its own `b.mu`, not the process-wide `reconcileMu`), so it needs a HOME rather than a fix in this round | `internal/plugins/traffic/vpp/` | HOMED, not fixed. Six-cell row in `plan/deferrals/fixit-firewall-concurrency-deadlock.md`, destination `plan/spec-finish-vpp-stub.md`, naming the producer and quoting the govpp mechanism |
| 3 | NOTE | `observeApply` runs INSIDE `reconcileMu`: the unlock defer is registered first, so LIFO runs the observation before it, putting a `slog` Error write and a syscall under the process-wide reconcile lock | `internal/component/firewall/registry.go` | FIXED. The observe defer is now registered before the unlock defer, so it runs after the release; `applyResult` stays empty until `Apply` is entered, which keeps the early returns out of the histogram. New test `TestApplyAllObservesOutsideTheReconcileLock` probes `reconcileMu.TryLock` at observation time |
| 4 | NOTE | One classification is a stretch: VPP being ABSENT (the connect wait running out) was tagged `ErrKernelTimeout`, which ticks the wedged-dataplane counter and makes ddos-local skip its rollback reconcile. "Not there" is not "wedged" | `internal/plugins/firewall/vpp/timeout_linux.go` | FIXED by separating them. `asDataplaneTimeout` now tags `core.ErrReplyTimeout` only; the connect phase returns `vpp not reachable`, untagged. Recorded on the `ErrKernelTimeout` contract and in `core-design.md`. Test row flipped to `an absent dataplane is not a wedged one` |
| 5 | NOTE | Stale clause: `bindMetrics`'s doc says "before the engine runs", but `InjectPluginMetrics` defers the hook when no registry exists yet | `internal/component/firewall/metrics.go` | FIXED. The doc now states the hook can fire late, and records the reviewer's finding that `startStandaloneTelemetry` sets the registry before the plugin phase, so the boot reconcile is still observed |
| 6 | NOTE | Bucket ceiling: the last finite bucket was 30s while the deadline may be 60s, so a max-deadline timeout lands only in `+Inf` | `internal/component/firewall/metrics.go` | FIXED, and the follow-up finding that the 60s tie was nominal is fixed with it: three literals became one exported `firewall.MaxBackendDeadline`, which both backend clamps and the bucket list derive from. `countingRegistry.HistogramVec` now records its buckets, and `TestApplyDurationBucketsReachTheMaxDeadline` asserts on the REGISTERED list |

**Discrimination observed for each fix:** control moved, `CONTROL:` line appears on the
failure path where it previously could not print at all; lock position, re-registering
the observe defer after the unlock reds exactly `TestApplyAllObservesOutsideTheReconcileLock`
("observeApply ran while reconcileMu was held"); bucket ceiling, hardcoding the old 30s
list reds with "last finite bucket = 30s, want >= 60s (a max-deadline timeout would land
in +Inf)".

**One Run 3 note was judged rather than applied, per its own instruction.** The control's
family count could never deny, because `RegisterRuntimeCollectors` puts `go_*` and
`process_*` into any answering exposition, so it restated the preceding `fetch()`
assertion. It is REPLACED, not deleted, by a named check that the serving registry
carries `go_goroutines` and `process_start_time_seconds`: that denies on an endpoint
answering with an empty body and on a build that stops registering the runtime
collectors, both of which the count accepted. What neither version can distinguish,
recorded in the test for the next reader, is `/metrics` being served by a different
registry than plugin hooks bind into; only another plugin-bound family in the same
exposition separates that from a firewall hook failure, and this peer-less config has
no equivalent of `plugin-metrics-registered.ci`'s `ze_role_*`. The stale-body defect
Run 3 noted alongside it is fixed: a failed poll now clears the body, so the failure
diagnostic never describes an earlier successful scrape.

### Not yet reviewed (2026-08-07)
- The Run 3 fixes themselves. Every finding from rounds 2 and 3 is fixed and each fix
  carries an observed red, but no reviewer has read the fixes for rounds 3's items.
- One finding was homed rather than fixed (Run 3 item 2), and one blind spot in the
  review tooling was homed with it: `scripts/dev/audit-test-relaxation.py` reads
  `git diff` against HEAD, so a `test-relax:` token in an UNTRACKED test file feeds
  nothing. Measured on this work: the audit reported `1 finding(s)`, another session's
  file, and never mentioned `test/plugin/firewall-metrics-registered.ci` despite its two
  tokens. Row destination `spec-fixit-test-harness-fail-open-guards`, closed
  2026-08-14; the row stays live in
  `plan/deferrals/fixit-firewall-concurrency-deadlock.md`.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE  (achieved on Run 1; scope = firewall slice)
- [ ] All NOTEs recorded above (or explicitly "none")  (4 NOTEs recorded)
- Artifact: `tmp/review/fixit-firewall-concurrency-deadlock-58c51aab-79d8-400d-b779-2c0cf322a274.md`

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)
- [ ] The parent deferral row (`plan/deferrals.md`) is resolved or updated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Race detector clean on the touched packages
- [ ] Goal Validation table filled with concrete evidence

## Open Questions (research before design)

**Answered during the 2026-07-16 design session:**

| Question | Answer | Cite |
|----------|--------|------|
| Is nft `Apply` safe to call concurrently? | **No.** The `Conn` command batch is shared; the loser's `Flush` silently returns nil, and `b.applied` is a raced map | `conn.go`, `backend_linux.go,88` (Finding 1) |
| Should `ApplyAll` serialise reconciles? | **Yes, over the whole body (snapshot + apply), not just `Apply`** | D-1 |
| Is holding `r.mu` across `applyAll` load-bearing? | **Partly.** It serialises mitigations (keep) AND blocks `status()` (fix by publishing the snapshot atomically) | D-3, A-9 |
| Does the engine deliver events and dispatch-command on the same goroutine? | **No.** Separate goroutines, no dispatcher mutex, handlers invoked outside the subscriber lock | `command.go`, `engine_event.go` |
| Is there a lock-order cycle? | **No.** Strictly L3/L4 -> L1 -> L2 -> L5 on every path, no reverse edge | Current Behavior lock-order table |
| Do the other `ApplyAll` callers share the hazard? | All six share Finding 1; only anomaly-shape shares Finding 3 | Finding 4 |

**Still open (carry into implementation):**

- **What were the actual blocked goroutine stacks?** Unanswered, and it remains the one
  keystone fact (A-6, R-9). Finding 3 predicts them; prediction is not observation.
- **Which command did the 2026-07-12 run dispatch?** Cheapest next step, no QEMU needed:
  read the `spec-ddos-direction-allowlist` `.ci` and its run log. If it never dispatched a
  ddos-local command, Finding 3 is not the observed stall and R-9 fires.
- **Was the 255s bounded by a timeout, a watchdog, or genuine recovery?** Still unknown.
  Finding 2 makes "genuine recovery" unlikely: with no netlink deadline there is nothing
  to recover it. A harness timeout is the leading hypothesis, unverified.
- **Does the vpp firewall backend share Finding 1?** `internal/plugins/firewall/vpp/` was
  not read this session. D-1 protects it regardless, but D-5's interface contract should
  be checked against it before claiming AC-8.
- **What is the detector's re-fire interval?** Needed to fix the D-2 deadline default
  (R-8). Read `internal/plugins/ddos/detect/` at implement time.

**Resolved for readiness (autonomous defaults, 2026-07-17 readiness pass).** None of the
items above is a DESIGN blocker; each is impl-time evidence-gathering. The design proceeds
on the recorded fixes (D-1 registry serialisation, D-2 bounded `Apply`, D-3/D-4 atomic
status snapshot). Every load-bearing `file:line` in Current Behavior / Problem / Evidence /
Findings 1-5 was re-verified against source during this pass and holds (a few trivial
off-by-ones against the vendored `google/nftables` copy noted below). Resolutions,
append-only:

→ AUTONOMOUS DEFAULT (2026-07-17): (blocked goroutine stacks) proceed WITHOUT them; capture
  them when the phase-1 reproduction test runs during implementation (SIGQUIT / pprof during
  the stall). Rationale: Findings 1-4 are read from the producing code (`conn.go`,
  `backend_linux.go,74,77,88`, `responder.go,136,199`, `show.go` — all re-verified
  this pass) and ship as real defects today, which Scope already admits as IN. The stacks
  only confirm WHICH of these produced the 2026-07-12 symptom; they do not gate the fixes.
  AC-1 already accepts "an evidenced statement it cannot be reproduced" as the honest exit
  (R-9). Thomas: override if wrong.

→ AUTONOMOUS DEFAULT (2026-07-17): (which command the 2026-07-12 run dispatched; whether
  255s was a harness timeout) treat both as phase-1 evidence, NOT gates. Default: read the
  `spec-ddos-direction-allowlist` `.ci` + its run log first (cheap, no QEMU); if it
  dispatched a ddos-local command, Finding 3 is corroborated; if not, R-9 fires and the
  design still fixes real bugs but not the observed one. Rationale: mutex starvation mode
  rules out contention as the 255s cause (Mistake Log), leaving an unbounded `Receive`
  (Finding 2) as the only code-supported explanation; a harness timeout on top of it is the
  leading, unverified hypothesis. Neither answer changes D-1/D-2/D-3/D-4. Thomas: override
  if wrong.

→ AUTONOMOUS DEFAULT (2026-07-17): (vpp backend vs Finding 1) D-1 protects vpp regardless —
  serialisation lives in `ApplyAll`, above every backend. At implement time, check D-5's
  single-writer / bounded-`Apply` interface contract against
  `internal/plugins/firewall/vpp/backend_linux.go` (dir confirmed to exist this pass) before
  claiming AC-8; do not block the registry fix on it. Rationale: the concurrent-apply
  corruption lives in the registry, not the backend (Finding 4); vpp inherits the D-1 fix.
  Thomas: override if wrong.

→ AUTONOMOUS DEFAULT (2026-07-17): (D-2 deadline default = 10s; R-8 accepted as a documented
  consequence) keep the recorded 10s default and the bounded-`Apply` fix. R-8 — bounding
  `Apply` converts a wedged-kernel HANG into a FAILED MITIGATION, and ddos-local's rollback
  would `applyAll` a second time — is an ACCEPTED, DOCUMENTED consequence, not an open
  question. Its mitigation is already recorded in D-3 / Files to Modify ("do not re-`applyAll`
  on a timed-out rollback", `responder.go`) plus a log + metric so the operator sees
  a wedged kernel rather than a silent no-drop (AC-6, AC-10). At implement time verify 2x10s
  stays inside the detector re-fire interval (`internal/plugins/ddos/detect/`, dir confirmed);
  if that interval is under 20s, lower the default or drop the rollback retry (R-8's preferred
  mitigation). Rationale: a crashed kernel wedging the management plane is the worse failure;
  a failed-then-surfaced mitigation is recoverable and observable. Thomas: override if wrong.

→ AUTONOMOUS DEFAULT (2026-07-17): (parent deferral-row citation) the parent row is now at
  `plan/deferrals.md`, NOT `:55`. Line drift in the mutable ledger since 2026-07-15
  authoring; the row content matches this spec verbatim (it even carries the
  `firewall/engine.go` -> `internal/component/firewall/engine.go` path
  correction). Read every `deferrals.md` reference in this spec (Origin, A-1, A-2, Known
  Limitations) as `deferrals.md`. Thomas: override if wrong.

→ AUTONOMOUS DEFAULT (2026-07-17): (citation-drift note for the implementer) the vendored
  `google/nftables@v0.3.0` copy differs from the module-cache copy the author cited by a
  line or two: `SendMessages` is `conn.go` (spec cites `:261`); the per-dial sockOptions
  loop is `conn.go` (spec cites `:314-318`); `nftables.New()` is
  `backend_linux.go` (spec cites `:31` in Finding 2 and A-3). All substance holds
  (shared batch, loser flush returns nil, no netlink deadline, sockOptions applied per dial).
  Use these corrected lines when writing the D-2 change. Thomas: override if wrong.

## Notes
- Authored 2026-07-15 as a skeleton from `plan/deferrals.md`. Every `file:line` here was
  read at authoring time. The deferral row's `internal/plugins/firewall/engine.go`
  citation did not resolve and is corrected in the Mistake Log.
