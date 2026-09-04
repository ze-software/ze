# Config Transaction Protocol

<!-- source: pkg/ze/eventbus.go -- EventBus interface -->
<!-- source: internal/component/config/transaction/orchestrator.go -- TxCoordinator state machine -->
<!-- source: internal/component/config/transaction/topics.go -- config namespace event types -->
<!-- source: internal/component/plugin/server/engine_event_gateway.go -- ConfigEventGateway adapter -->

Ze uses a stream-based transaction protocol for config changes. All phases (verify,
apply, rollback) are namespaced stream events delivered through the engine's event
system. Plugins subscribe to the event types they care about in the `config` namespace
and react. The engine orchestrates deadlines and ack collection but does not direct
individual plugins beyond publishing events.

---

## 1. Design Principles

| Principle | Rationale |
|-----------|-----------|
| Stream-native | Config transactions use the same `(namespace, event-type)` pub/sub backbone as all other cross-component coordination (interface events, RIB changes, BGP events). No separate RPC path, no separate bus. |
| Plugin autonomy | Plugins decide what they need. A plugin that depends on an interface being created waits for both the apply event and the interface event. The engine does not manage per-plugin dependency graphs at runtime. |
| Plugin-estimated timeouts | Plugins declare verify and apply budgets at registration, update them after each transaction. Engine enforces the dependency-graph-aware critical path. No mid-transaction negotiation. |
| Rollback is an event | The engine emits a single rollback event when any plugin's apply fails or times out. All plugins that already applied undo via their journals. The engine drains rollback acks in reverse dependency-tier order. |
| Runtime is authoritative | A candidate config is promoted to the active pointer only after runtime reload succeeds. Persistence failure after apply is a warning, not a plugin rollback trigger. |

---

## 2. Transaction Phases

A config transaction has three runtime phases. Each phase emits stream events in the
`config` namespace with a shared transaction ID. Plugins subscribe to event types in
the `config` namespace to participate.

### Phase 1: Verify

The engine emits a per-plugin verify event. Every plugin that owns affected config
roots validates the candidate config against its constraints. Verification is
non-destructive: no state changes, no side effects.

| Step | Actor | Event |
|------|-------|-------|
| 1 | Engine | Emits `(config, verify-<plugin>)` per plugin with transaction ID and that plugin's filtered diffs. The event type is registered dynamically per plugin name. |
| 2 | Plugin | Validates its portion. Emits `(config, verify-ok)` including an estimated apply duration, or `(config, verify-failed)` with a reason. |
| 3 | Engine | Collects acks. Every plugin must ack positively. A single `verify-failed` or a missing ack (timeout) fails the entire verify phase. Engine emits `(config, verify-abort)` and the transaction ends. |
| 4 | Engine | Computes the transaction deadline from the dependency-graph critical path (sum per-tier max budgets). This becomes the deadline in each `(config, apply-<plugin>)` event. |

Participation in config transactions is opt-in via two separate declarations
in Stage 1 of the 5-stage startup protocol:

| Declaration | Meaning |
|-------------|---------|
| `ConfigRoots` | Plugin owns these config roots (schema authority, validation) |
| `WantsConfig` | Plugin wants to receive diffs for these config roots during transactions |

A plugin that owns config (`ConfigRoots`) implicitly receives diffs for its own
roots. `WantsConfig` lets a plugin request diffs for roots owned by other plugins.
The plugin receives the actual config data, not just a notification -- it can
read the other module's config to make decisions.

A plugin declares `WantsConfig` for any root it needs to read, regardless of
ownership. This is how cross-plugin dependencies are expressed: the DHCP plugin
reads interface config to know which interfaces to serve on; the route-reflector
reads BGP config to know the peer topology.

Examples:

| Plugin | `ConfigRoots` | `WantsConfig` | Reads | Role |
|--------|--------------|--------------|-------|------|
| iface | `["interface"]` | - | own config | Owns and applies interface config |
| BGP | `["bgp"]` | - | own config | Owns and applies BGP config |
| DHCP | `["dhcp"]` | `["interface"]` | own + iface | Reads interface config to bind DHCP to interfaces |
| route-reflector | - | `["bgp"]` | bgp | Recalculates when BGP peers change |
| metrics-exporter | - | `["bgp", "interface"]` | bgp + iface | Updates labels on config change |
| NLRI codec | - | - | nothing | Pure protocol, no config involvement |

Plugins with neither declaration do not receive transaction events.

Among participating plugins, every one must respond to verify. The engine sends
each plugin only the diffs for the roots it declared (`ConfigRoots` or
`WantsConfig`). A plugin never sees config for roots it did not declare
interest in. Plugins with affected roots validate and emit `verify-ok` with an
estimated apply duration. Plugins whose watched roots are not affected by this
transaction emit `verify-ok` with zero duration. The engine expects exactly one
positive ack from every participating plugin. Apply is never sent unless all
are in.

The engine emits per-plugin verify and apply event types (`verify-<plugin>`,
`apply-<plugin>`), filtered by declared roots, not a single broadcast with the
full config. Each plugin subscribes only to its own event type and receives the
union of its `ConfigRoots` and `WantsConfig` roots. A DHCP plugin declaring
`ConfigRoots: ["dhcp"]` and `WantsConfig: ["interface"]` receives diffs for both
`dhcp` and `iface`, but never sees `bgp` or `telemetry` config.

The deadline is plugin-decided but engine-enforced. Plugins know their workload
after inspecting the diffs during verify. The engine takes the maximum across all
plugins and enforces it as the transaction deadline.

### Phase 2: Apply

After all verifications pass, the engine emits the per-plugin apply event. The
config file is NOT written yet -- it stays unchanged until all plugins confirm.
Plugins apply their changes from the candidate diffs.

| Step | Actor | Event |
|------|-------|-------|
| 1 | Engine | Emits `(config, apply-<plugin>)` per plugin with transaction ID and diffs (config file unchanged). |
| 2 | Plugin | Applies changes. May produce side-effect events (interface creation, listener start, etc.) on other namespaces. |
| 3 | Plugin | Emits `(config, apply-ok)` when done, including updated verify and apply budgets. |
| 4 | Plugin (failure) | Emits `(config, apply-failed)` -- triggers rollback for all participants. |

Plugins may depend on side-effect events from other plugins before completing their
apply. For example, a DHCP plugin that binds to an interface waits for both its
`apply-dhcp` event and the `(interface, created)` event before it acts. The plugin
manages this dependency internally; the engine does not track inter-plugin
dependencies during apply.

If a plugin receives `(config, rollback)` while still applying, it finishes the
in-progress apply and immediately undoes it.

### Phase 3: Rollback (conditional)

Triggered when any plugin emits `apply-failed` or when the apply deadline
expires without all plugins completing.

| Step | Actor | Event |
|------|-------|-------|
| 1 | Engine | Emits `(config, rollback)` with transaction ID (triggered by `apply-failed` or timeout). |
| 2 | Engine | Drains rollback acks in reverse dependency-tier order: highest-tier plugins (dependents) first, then lower tiers (dependencies). |
| 3 | Plugins that applied | Undo changes via journal, emit `(config, rollback-ok)`. |
| 4 | Plugins that had not started | Skip apply, emit `(config, rollback-ok)` with code `ok`. |

Only the engine emits `(config, rollback)`. A failing plugin emits
`(config, apply-failed)`; the engine reacts by emitting `rollback`. This ensures
a single source of truth -- no duplicate rollback events from multiple sources.

Rollback ack collection follows reverse dependency-tier order, computed via
`registry.TopologicalTiers`. Plugins in the deepest tier (most dependents) ack
first; the engine waits for the entire tier to ack before moving to the previous
tier. CodeBroken restarts happen between tiers so a plugin marked broken is
restarted before its dependencies start tearing down state. Within a tier, acks
are processed in arrival order (same-tier plugins are independent).

Rollback deadline is 3x the apply deadline. If a plugin exceeds this, it is
treated as `broken` (see Failure Codes).

### Completion

After all plugins ack (apply-ok or rollback-ok), the engine emits the
finalization events:

| Outcome | Action | Event |
|---------|--------|-------|
| All plugins applied | Engine emits `(config, committed)`. The hub promotes the staged candidate to active after the full subsystem reload succeeds. | Runtime is authoritative. |
| All applied, pointer promotion fails | `committed` already emitted. Warning reported to caller. Runtime is live, active pointer may still reference the previous version. | Caller can retry commit/reload. |
| Rollback occurred | Config file untouched, engine emits `(config, rolled-back)`. | File still matches pre-transaction runtime. |

Runtime is the authority, not the file. The transaction succeeds when all
plugins apply. In production, CLI, web, API, SIGHUP, and managed pushes write
the proposed config as an immutable version and set `meta/config/candidate`.
The hub reload path reads that candidate, runs plugin verification/apply, then
promotes the candidate to `meta/config/active` only after the wider subsystem
reload succeeds. If pointer promotion fails after runtime apply, the runtime is
still live and correct, and the caller gets an error/warning to retry.

`(config, applied)` and `(config, rolled-back)` are informational events for
observers (monitoring, web UI refresh, logging). `applied` includes a `saved`
boolean indicating whether the file write succeeded.

### Persistence Pointers

The production commit path stores versioned configs and moves named pointers:

| Pointer | Meaning |
|---------|---------|
| `meta/config/active` | Timestamp of the config version that boot and runtime consider active |
| `meta/config/candidate` | Transient timestamp staged for the current commit attempt |
| `meta/config/rollback` | Previous active timestamp after a successful promotion |
| `meta/config/recovery` | Operator-selected known-good timestamp for future recovery commands |

On success, promotion sets `rollback` to the previous `active`, sets `active`
to `candidate`, then clears `candidate`. On failure, the hub clears
`candidate` and leaves `active` unchanged. On boot, stale `candidate` is ignored
and cleaned up; the daemon loads `active` when present.

---

## 3. Timeout

Both verify and apply deadlines are computed from plugin estimates.

### Estimate Lifecycle

Plugins provide timeout estimates at registration and update them after each
transaction phase. The engine always uses the latest values.

| When | Plugin provides | Engine uses for |
|------|----------------|-----------------|
| Stage 1 registration | Initial verify budget + initial apply budget | First transaction |
| After each verify response | Updated apply budget (based on actual diffs) | This transaction's apply deadline |
| After each apply/rollback response | Updated verify budget + updated apply budget | Next transaction |

The engine computes the deadline from the dependency graph, not a simple max.
Plugins estimate only their own work. The engine computes the critical path
through the dependency tiers returned by `registry.TopologicalTiers`:

- Within a tier (independent plugins): take the max budget
- Across tiers (serialized phases): sum the per-tier maxes
- Total deadline = `sum_k(max_{p in tier k}(budget(p)))`

The engine derives tiers from each plugin's `Dependencies` field in its
registration. Plugins in tier 0 have no dependencies in the participant set;
tier `k` plugins depend (transitively) on plugins in tiers `0..k-1`. Plugins
within a tier run concurrently so their cost is the max; tiers are serialized
because tier `k+1` can only start after tier `k` finishes.

Example: bgp (tier 0, 10s) and rib (tier 0, 5s) run concurrently in tier 0;
fib-kernel (tier 1, 3s) depends on rib. Tier 0 max = max(10, 5) = 10s.
Tier 1 max = 3s. Total deadline = 10 + 3 = 13s. The pre-graph flat formula
would have returned 10s, missing the 3 seconds the fib plugin needs after
rib finishes.

### Self-Correcting Feedback

A plugin starts with a guess at registration. After seeing real diffs during
verify, it refines the apply estimate. After completing apply (or rollback),
it updates both estimates for next time.

If a plugin underestimates and times out, the engine emits `(config, rollback)`.
The plugin's rollback ack includes a code (e.g., `timeout`). The engine forwards
this to the caller. On retry, the plugin provides a higher estimate based on
what it learned.

There is no mid-transaction extension mechanism. The feedback loop operates
between transactions, not within one.

---

## 4. Stream Events

All events live in the `config` namespace. Payloads are JSON. The transaction ID
ties all events in a transaction together.

### Event Types in the `config` Namespace

| Event type | Direction | Purpose |
|------------|-----------|---------|
| `verify-<plugin>` | Engine -> plugin | Validate candidate. Per-plugin variants registered dynamically when each plugin starts. |
| `verify-ok` | Plugin -> engine | Verification passed. Includes apply budget estimate. |
| `verify-failed` | Plugin -> engine | Verification rejected. Includes failure reason. |
| `verify-abort` | Engine -> plugins | Verification phase failed, all plugins stop. |
| `apply-<plugin>` | Engine -> plugin | Apply the changes. Per-plugin variants registered dynamically. |
| `apply-ok` | Plugin -> engine | Apply succeeded. Includes updated verify and apply budgets. |
| `apply-failed` | Plugin -> engine | Apply failed, triggers rollback. |
| `rollback` | Engine -> plugins | Undo applied changes. |
| `rollback-ok` | Plugin -> engine | Rollback complete with status code. |
| `committed` | Engine -> plugins | Transaction finalized, discard journals. |
| `applied` | Engine -> observers | Transaction committed (emitted after `committed`). Includes `saved` flag. |
| `rolled-back` | Engine -> observers | Transaction rolled back. |

The per-plugin event types `verify-<name>` and `apply-<name>` are registered in
the engine's event registry when each plugin starts. The orchestrator subscribes
to the broadcast ack types (`verify-ok`, `apply-ok`, etc.) and demultiplexes by
the `Plugin` field in each ack payload.

The constants for these event types live in
`internal/component/config/transaction/topics.go`, which re-exports the
`internal/component/plugin` constants `EventConfigVerify`, `EventConfigVerifyOK`,
etc. The helpers `EventVerifyFor(name)` and `EventApplyFor(name)` build the
per-plugin variants. Reserved plugin names (`ok`, `failed`, `abort`) are
rejected at registration to prevent collision with the broadcast event types.

### Event Payloads

The Go types for these payloads live in
`internal/component/config/transaction/types.go` (`VerifyEvent`, `ApplyEvent`,
`VerifyAck`, `ApplyAck`, `RollbackEvent`, `RollbackAck`, `CommittedEvent`,
`AppliedEvent`).

**`(config, verify-<plugin>)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID (unique per commit) |
| Diffs | `diffs` | array of `DiffSection` | Per-root diffs filtered to this plugin's declared roots |
| DeadlineMS | `deadline-ms` | int64 | Verify deadline as Unix milliseconds |

**`(config, verify-ok)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID |
| Plugin | `plugin` | string | Plugin name (used by the engine to demultiplex acks) |
| Status | `status` | string | `ok` |
| ApplyBudgetSecs | `apply-budget-secs` | int | Estimated apply time for this transaction in seconds (capped at 600) |

**`(config, verify-failed)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID |
| Plugin | `plugin` | string | Plugin name |
| Status | `status` | string | `error` |
| Error | `error` | string | Failure reason |

**`(config, apply-<plugin>)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID |
| Diffs | `diffs` | array of `DiffSection` | Per-root diffs |
| DeadlineMS | `deadline-ms` | int64 | Apply deadline as Unix milliseconds |

**`(config, apply-ok)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID |
| Plugin | `plugin` | string | Plugin name |
| Status | `status` | string | `ok` |
| VerifyBudgetSecs | `verify-budget-secs` | int | Updated verify budget for next transaction |
| ApplyBudgetSecs | `apply-budget-secs` | int | Updated apply budget for next transaction |

**`(config, apply-failed)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID |
| Plugin | `plugin` | string | Plugin name |
| Status | `status` | string | `error` or `broken` |
| Error | `error` | string | Failure reason |

**`(config, rollback)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID |
| Reason | `reason` | string | What triggered rollback (plugin failure or timeout) |

**`(config, rollback-ok)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID |
| Plugin | `plugin` | string | Plugin name |
| Code | `code` | string | Rollback result code (see Failure Codes below) |

**`(config, committed)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID |

**`(config, applied)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID |
| Saved | `saved` | boolean | Whether the config file was written to disk |

**`(config, rolled-back)`**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| TransactionID | `transaction-id` | string | Transaction ID |

---

## 5. Plugin Load/Unload During Reload

When a config change adds or removes config roots, plugins must be loaded or
stopped. This happens outside the transaction, not inside it.

| Change | When | Why |
|--------|------|-----|
| New config root added (e.g., `rib {}`) | Plugin loaded via 5-stage protocol **before** transaction starts | Plugin must be running to participate in verify |
| Config root removed (e.g., `bgp {}` deleted) | Plugin participates in transaction (cleans up during apply), stopped **after** `(config, committed)` | Plugin needs to shut down resources cleanly via journal |

On rollback of a removal: the plugin rolled back its cleanup (resources restored).
It stays running. The config root was never actually removed (file not written).

On rollback of an addition: the newly loaded plugin rolled back its initial apply.
The engine stops and unloads it after `(config, rolled-back)`. It was loaded for
nothing, but no harm done.

---

## 6. Transaction Exclusion

Only one config transaction can be active at a time. While a transaction is in
progress (from the first `(config, verify-<plugin>)` emit until `(config, applied)`
or `(config, rolled-back)`), all other config operations are refused.

| Rejected operation | Response |
|--------------------|----------|
| CLI `commit` | Error: transaction in progress (tx ID, initiator) |
| API `ConfigCommit` | Error: transaction in progress |
| SIGHUP reload | Queued until current transaction completes |
| Editor `set`/`delete` | Allowed (candidate editing is independent of commit) |

The engine holds a transaction lock acquired at the start of verify and released
when the final `applied` or `rolled-back` is emitted. The lock carries the
transaction ID and initiator for diagnostics.

SIGHUP is queued rather than rejected because the user expects reload to happen.
If the current transaction completes, the queued SIGHUP fires. If a second SIGHUP
arrives while one is already queued, it replaces the queued one (only the latest
config matters).

### Shutdown while a transaction runs

The transaction owns the plugin connections. The daemon MUST NOT close them
under a running transaction. A closed connection is indistinguishable from a
crashed plugin, so the orchestrator reads a shutdown as a wave of crashes. It
then elects a rollback and restarts plugins the same shutdown is about to kill.

Shutdown therefore cancels the transaction and waits for it. It also names the
reason: the cancellation CAUSE is `transaction.ErrShutdown`. A transaction
canceled with that cause emits no abort and no rollback, because there is no
running system left to restore.

The wait is bounded at 3 seconds. After that the connections close anyway: a
shutdown that hangs on a stuck reload is worse than the log noise the wait
removes.

Any OTHER cancellation still rolls back. A reload that exceeds its own
30-second deadline leaves participants half-applied inside a daemon that keeps
running, and that daemon must be told to undo the change.

---

## 7. Apply Journal

Plugins should not each reimplement change tracking for rollback. The SDK provides
a journal library that records applied changes and can replay them in reverse.

### Purpose

During apply, a plugin records each state change in the journal. On rollback, the
journal replays the undo operations in reverse order. On `(config, committed)`, the
journal is discarded -- the changes are permanent.

### Lifecycle

| Event | Journal action |
|-------|----------------|
| `(config, apply-<plugin>)` received | Plugin creates a journal for this transaction ID |
| Plugin applies a change | Plugin records the change and its undo operation |
| `(config, apply-ok)` emitted | Journal stays open, waiting for finalization |
| `(config, committed)` received | Journal discarded -- changes are permanent |
| `(config, rollback)` received | Journal replayed in reverse, then discarded |

### SDK Interface

The journal is a helper in the plugin SDK. Plugins that do not need rollback
support can ignore it.

| Method | Description |
|--------|-------------|
| `NewJournal(tx)` | Create a journal for a transaction |
| `journal.Record(apply, undo)` | Record an applied action and its reverse |
| `journal.Rollback()` | Execute all undo operations in reverse order |
| `journal.Discard()` | Drop the journal (changes are permanent) |

The `apply` and `undo` arguments are functions. `Record` calls `apply` immediately
and stores `undo` for potential rollback. If `apply` fails, `undo` is not stored
and the error propagates -- the plugin emits `(config, apply-failed)`.

### Example: Interface plugin

1. Journal created on `(config, apply-iface)`
2. Create interface eth0: `Record(createEth0, deleteEth0)`
3. Assign IP 10.0.0.1: `Record(addAddr, removeAddr)`
4. Emit `(config, apply-ok)`
5a. `(config, committed)` arrives: journal discarded, eth0 + IP are permanent
5b. `(config, rollback)` arrives: journal replays `removeAddr` then `deleteEth0`

---

## 8. Transaction Finalization

After all plugins ack (apply-ok or rollback-ok), the engine emits a finalization
event. This is the signal for plugins to discard their journals.

| Outcome | Event | Journal action |
|---------|-------|----------------|
| All applied | `(config, committed)` | Discard journal -- changes are permanent |
| Rollback completed | `(config, rolled-back)` | Journal already replayed during rollback |

`committed` is distinct from `applied`. The difference:

| Event | Audience | Purpose |
|-------|----------|---------|
| `(config, committed)` | Transaction participants (plugins) | Finalize journals, release transaction resources |
| `(config, applied)` | Observers (web UI, monitoring) | Informational notification |

The engine emits `committed` first (participants finalize), then `applied`
(observers notified). A plugin must not discard its journal until it receives
`committed` -- without it, a late rollback could arrive with no journal to replay.

---

## 9. Dependency Waiting

Plugins that depend on side effects from other plugins handle this internally
during the apply phase. The engine knows the dependency graph for deadline
computation, but it does not coordinate inter-plugin dependencies during apply
itself; plugins subscribe to whatever side-effect events they need.

**Pattern:** A plugin subscribes to both its own `(config, apply-<self>)` event
and the side-effect events it depends on. It only finishes its apply when both
have arrived for the same transaction.

### Example: DHCP binding to a new interface

1. Engine emits `(config, apply-iface)` and `(config, apply-dhcp)` -- iface plugin and DHCP plugin both receive their own apply event
2. Iface plugin creates the interface, emits `(interface, created)`
3. DHCP plugin sees its `apply-dhcp` but waits for `(interface, created)` for its target interface
4. `(interface, created)` arrives -- DHCP binds to the interface, emits `(config, apply-ok)`

### Example: BGP binding to a new local address

1. Engine emits `(config, apply-iface)` and `(config, apply-bgp)` -- iface plugin and BGP reactor both receive their own apply event
2. Iface plugin assigns the IP, emits `(interface, addr-added)`
3. BGP reactor sees its `apply-bgp` but waits for `(interface, addr-added)` for the local address
4. Address event arrives -- BGP starts the listener, emits `(config, apply-ok)`

### Rollback during dependency wait

If `(config, rollback)` arrives while a plugin is waiting for a dependency event:

- The plugin cancels its wait
- Emits `(config, rollback-ok)` with code `ok` (nothing was applied, nothing to undo)

---

## 10. Inter-System Event Flow

This table shows all events that cross system boundaries during a config transaction.
Systems that produce side-effect events during apply are listed with their outputs.

### Transaction Events (`config` namespace)

| Event | Producer | Consumers | Purpose |
|-------|----------|-----------|---------|
| `(config, verify-<plugin>)` | Engine | One specific plugin | Validate candidate |
| `(config, verify-ok)` | Plugin | Engine | Verification passed |
| `(config, verify-failed)` | Plugin | Engine | Verification rejected |
| `(config, verify-abort)` | Engine | All plugins | Stop verification |
| `(config, apply-<plugin>)` | Engine | One specific plugin | Apply changes |
| `(config, apply-ok)` | Plugin | Engine | Apply succeeded |
| `(config, apply-failed)` | Plugin | Engine | Apply failed |
| `(config, rollback)` | Engine | All plugins that received apply | Undo changes |
| `(config, rollback-ok)` | Plugin | Engine | Rollback complete |
| `(config, committed)` | Engine | All plugins | Finalize: discard journals |
| `(config, applied)` | Engine | Observers (web UI, monitoring, logging) | Transaction committed |
| `(config, rolled-back)` | Engine | Observers | Transaction rolled back |

### Side-Effect Events (produced during apply)

These are existing stream events that plugins emit as a consequence of applying
config changes. Other plugins may depend on them before completing their own apply.

| Event | Producer | Consumers | When |
|-------|----------|-----------|------|
| `(interface, created)` | iface | DHCP, BGP, telemetry | New interface configured |
| `(interface, down)` | iface | DHCP, BGP | Interface removed or brought down |
| `(interface, addr-added)` | iface | BGP (listener binding) | IP address assigned |
| `(interface, addr-removed)` | iface | BGP | IP address removed |
| `(interface, dhcp-acquired)` | ifacedhcp | BGP, DNS | DHCP lease obtained |
| `(bgp, listener-ready)` | BGP | iface migrate | BGP listener bound to address |
| `(bgp, state)` | BGP | RIB, monitoring | Peer state change after config apply |

### Event Flow Diagram

```
Caller (Web UI / API / SIGHUP)
  |
  | commit(timeout)
  v
Engine ----(config, verify-iface)-----> [iface]
Engine ----(config, verify-bgp)-------> [bgp]
Engine ----(config, verify-rib)----> [rib]
Engine ----(config, verify-dhcp)------> [dhcp]
  |   <---(config, verify-ok)---- [iface]
  |   <---(config, verify-ok)---- [bgp]
  |   <---(config, verify-ok)---- [rib]
  |   <---(config, verify-ok)---- [dhcp]
  |
Engine ----(config, apply-iface)------> [iface]
Engine ----(config, apply-bgp)--------> [bgp]
Engine ----(config, apply-rib)-----> [rib]
Engine ----(config, apply-dhcp)-------> [dhcp]
  |                              |
  |                              +--(interface, created)----> [dhcp]
  |                              +--(interface, addr-added)-> [bgp]
  |                              |
  |   <---(config, apply-ok)---- [iface]
  |   <---(config, apply-ok)---- [dhcp]   (after interface/created)
  |   <---(config, apply-ok)---- [bgp]    (after interface/addr-added)
  |   <---(config, apply-ok)---- [rib]
  |
  |
Engine ----(config, committed)--> [iface] [bgp] [rib] [dhcp]   (discard journals)
  |
  | promote candidate pointer (after subsystem reload succeeds)
  |
Engine ----(config, applied)----> [web-ui] [monitoring] [logging]
  |
  v
Caller: commit succeeded after active pointer promotion
```

---

## 11. Relationship to Existing Systems

### Current Config Reload (replaced)

The current reload flow in `plugin/server/reload.go` uses direct RPC calls for
verify and apply. The transaction protocol replaces this with namespaced stream
events delivered through the engine's existing pub/sub fan-out:

| Current (RPC) | New (Stream) |
|---------------|--------------|
| `ConfigVerify(sections)` RPC per plugin | `(config, verify-<plugin>)` per-plugin event |
| `ConfigApply(diffs)` RPC per plugin | `(config, apply-<plugin>)` per-plugin event |
| No rollback | `(config, rollback)` broadcast event with reverse-tier ack collection |
| Engine tracks affected plugins | Engine emits per-plugin event types; each plugin subscribes only to its own |

### API Engine (spec-api-0-umbrella)

REST config sessions stage a candidate version, call the hub reload hook, and
close the session only after the hook succeeds. The API gets
verify/apply/rollback through the same hub path as web, CLI, SIGHUP, and managed
pushes.

### Plugin SDK

The SDK exposes transaction participation through callbacks:

| Callback | Phase | Required |
|----------|-------|----------|
| `OnConfigVerify` | Verify | Yes for all participants. Returns estimated apply duration. |
| `OnConfigApply` | Apply | Yes for all participants. |
| `OnConfigRollback` | Rollback | Optional (plugins that can undo) |

Plugins that do not implement `OnConfigRollback` cannot undo. If such a plugin's
apply succeeds but another plugin triggers rollback, the engine logs a warning.
The active pointer is not promoted on rollback; it remains the source of truth
for the last accepted runtime state.

Participation is declared in Stage 1 of the 5-stage startup protocol:

| Stage 1 field | SDK method | Effect |
|---------------|-----------|--------|
| `ConfigRoots` | `sdk.DeclareConfigRoots(...)` | Plugin owns config + receives diffs for those roots |
| `WantsConfig` | `sdk.WantsConfig(...)` | Plugin receives diffs for these roots (no ownership, read-only) |
| `VerifyBudget` | `sdk.SetVerifyBudget(...)` | Initial estimate for verify phase timeout |
| `ApplyBudget` | `sdk.SetApplyBudget(...)` | Initial estimate for apply phase timeout |

---

## 12. Failure Codes

Plugins report a code in `(config, rollback-ok)` to tell the engine what happened.
The engine forwards this to the caller.

| Code | System state | Meaning | Caller action |
|------|-------------|---------|---------------|
| `ok` | Known good | Clean rollback, no issues | Retry if desired |
| `timeout` | Known good | Ran out of time, rollback was clean | Retry (plugin will estimate higher) |
| `transient` | Known good | Temporary condition (resource busy, dependency not ready) | Retry (may work without changes) |
| `error` | Known good | Real failure, rollback was clean | Investigate, fix config, retry |
| `broken` | Unknown | Rollback could not fully complete, plugin state is inconsistent | Engine restarts plugin (see below) |

Everything except `broken` means the system is in a known good state after rollback.
`broken` means the plugin couldn't undo cleanly.

---

## 13. Recovery

### Broken Plugin Recovery

When a plugin reports `broken`, the engine automatically restarts it once:

1. Engine kills the plugin process
2. Engine respawns the plugin
3. 5-stage startup protocol runs
4. Plugin receives full config in Stage 2, applies from clean slate
5. If the plugin comes up healthy: recovery complete

If the plugin reports `broken` again after restart, the engine stops it and logs
an error. No restart loop. An operator must investigate and use a command to
force restart after fixing the underlying issue.

The active pointer matches the last accepted runtime state. A restarted plugin
converges to that active config by applying its roots from scratch during Stage
2. A stale candidate left by a crash is ignored on boot.

---

## 14. Failure Modes

| Failure | Engine behavior |
|---------|-----------------|
| Plugin crashes during verify | Treat as `verify-failed`. Abort transaction. |
| Plugin crashes during apply | Treat as `apply-failed`. Emit `rollback`. |
| Plugin does not respond before deadline | Emit `rollback`. Log timeout. |
| Rollback callback fails | Log error, continue rollback ack drain for other plugins. |
| Multiple plugins fail simultaneously | First failure triggers rollback. Subsequent failures are logged. |
| Plugin receives rollback before starting apply | Skip apply, emit `(config, rollback-ok)` with code `ok`. |
| Candidate promotion fails (after apply) | Warning/error to caller. Runtime is live. Active pointer may still reference previous version. No plugin rollback. |
| Concurrent commit attempted | Rejected with error. SIGHUP queued instead of rejected. |
| Engine crashes during transaction | Plugins hold journals. On restart, no `(config, committed)` arrives. Stale candidate is ignored; active pointer is loaded. Plugins detect stale journal (no matching active tx) and roll back on next startup. |
| Plugin exceeds rollback deadline (3x apply) | Treated as `broken`. Engine restarts plugin. |
| Plugin reports `broken` | Engine restarts plugin once via 5-stage protocol between rollback tiers. Second `broken` stops the plugin. |

---

## 15. Operation Graph Extension

<!-- source: internal/component/config/transaction/operation.go -- operation types, registries -->
<!-- source: internal/component/config/transaction/depgraph.go -- graph construction from constraint rules -->
<!-- source: internal/component/config/transaction/solver.go -- topological sort with cycle relaxation -->
<!-- source: internal/component/config/transaction/executor.go -- ordered execution with settlement -->
<!-- source: internal/component/iface/operation.go -- iface decomposer and constraint/settlement rules -->
<!-- source: internal/component/bgp/plugin/operation.go -- BGP decomposer and constraint rules -->

The stream-based transaction protocol (sections 1-14) applies config diffs as a
single broadcast per plugin. This works when plugins can independently apply their
portion of the diff. It breaks when operations span plugins and have ordering
constraints: a BGP peer cannot bind to an address that the iface plugin has not
yet assigned, and an IP swap between two interfaces creates a circular dependency
between remove-old and add-new.

The operation graph extension adds a secondary apply path that decomposes
full-config diffs into typed atomic operations, orders them via a constraint
graph, and executes them one at a time with inter-operation settlement.

### When the operation path activates

The operation path is not the default. It activates only when all three conditions
hold:

1. **Full-config verify succeeds.** Phase 1 (section 2) runs exactly as before.
   Every plugin validates the candidate config. Verify-failed or timeout aborts
   the transaction before the operation path is considered.

2. **An operation planner is registered.** The orchestrator holds an optional
   `OperationPlanner` set via `TxCoordinator.SetOperationPlanner`. If no planner
   is set, the orchestrator always takes the existing full-diff apply path.

3. **The planner returns operations.** The planner calls each registered
   `OperationDecomposer` for the affected config roots. If all decomposers return
   empty slices (no ordering-sensitive changes), the planner returns nil and the
   orchestrator falls through to the existing Phase 2 broadcast apply.

When the planner returns a non-empty operation list, the orchestrator calls
`runOperationPath` instead of `runApply`. The existing full-diff apply, rollback,
and commit paths are skipped entirely for that transaction.

```
orchestrator.go: Execute(ctx, diffs)
  |
  | Phase 1: full-config verify (unchanged)
  |
  | operationPlanner(ctx, OperationPlanRequest{diffs})
  |   \-- calls OperationDecomposerFor(root) per affected root
  |
  +-- ops == nil  --> Phase 2: full-diff apply (existing path)
  |
  +-- ops != nil  --> runOperationPath(ctx, ops)
                        |
                        | BuildOperationGraph(ops, ConstraintRules())
                        | TopologicalSort(graph)
                        | executor.Verify(ctx, sorted)
                        | executor.Execute(ctx, sorted)   // per-op apply + settlement
                        | executor.Commit(ctx, sorted)
```

### Operation types and the constraint rule registry

An operation is a typed, self-describing value (`ConfigOperation` in
`pkg/plugin/rpc/types.go`). Every operation carries:

| Field | Purpose |
|-------|---------|
| `ID` | Unique within a transaction (e.g., `interface-add-address-eth0-10.0.0.1_24`) |
| `Root` | Config root that owns this operation (`interface`, `bgp`, ...) |
| `Owner` | Plugin name responsible for apply/rollback |
| `Type` | One of the registered `ConfigOperationType` constants |
| `Target` | `ResourceRef`: kind, name, interface, address, peer, port, prefix, next-hop |
| `Params` | `ConfigOperationParams`: operation-specific values, config payloads, AllowDual flag |

Operation types are kebab-case string constants registered in `pkg/plugin/rpc/types.go`:

| Type | Resource kind | Example |
|------|--------------|---------|
| `add-interface` | `interface` | Create dummy0 |
| `remove-interface` | `interface` | Delete dummy0 |
| `add-address` | `address` | Assign 10.0.0.1/24 to eth0 |
| `remove-address` | `address` | Remove 10.0.0.1/24 from eth0 |
| `add-peer` | `peer` | Start BGP peer "upstream" |
| `remove-peer` | `peer` | Stop BGP peer "upstream" |
| `modify-peer` | `peer` | Update peer config in place |
| `add-listener` | `listener` | Bind BGP listener on 10.0.0.1:179 |
| `remove-listener` | `listener` | Unbind BGP listener |
| `add-bridge-member` | `bridge-member` | Add eth1 to br0 |
| `remove-bridge-member` | `bridge-member` | Remove eth1 from br0 |
| `set-property` | (varies) | Set MTU, admin state, etc. |
| `add-static-route` | `static-route` | Install a static route |
| `remove-static-route` | `static-route` | Remove a static route |
| `set-distance` | (varies) | Change administrative distance |
| `set-sysctl` | `sysctl` | Set a kernel parameter |
| `start-dhcp` | `dhcp` | Start DHCP client on an interface |
| `stop-dhcp` | `dhcp` | Stop DHCP client |
| `add-tunnel` | `tunnel` | Create a tunnel interface |
| `remove-tunnel` | `tunnel` | Destroy a tunnel interface |

#### Constraint rules

Ordering edges are produced by `ConstraintRule` values registered as data in
`operation.go`. A rule matches a pair of operations via selectors (type +
resource kind) and a relation (how their resources are connected). When both
selectors match and the relation holds, the graph builder adds an edge
`before -> after`.

| Field | Purpose |
|-------|---------|
| `ID` | Unique rule identifier (e.g., `iface-add-interface-before-address`) |
| `Before` | `OperationSelector{Type, ResourceKind}` matching the operation that must run first |
| `After` | `OperationSelector{Type, ResourceKind}` matching the operation that must run second |
| `Relation` | How the two operations' resources relate (see below) |

Resource relations:

| Relation | Meaning |
|----------|---------|
| (empty) | Any pair of matching operations |
| `same-resource` | Both target the same resource (same kind + key) |
| `interface-address` | The "before" operation's interface matches the "after" operation's address interface |
| `address-used-by` | The address in one operation matches the address used by the other |
| `same-address` | Both operations target the same IP address |

Rules are registered via `RegisterConstraintRule` in component `init()` functions.
Examples from the codebase:

| Rule ID | Before | After | Relation | Component |
|---------|--------|-------|----------|-----------|
| `iface-add-interface-before-address` | `add-interface/interface` | `add-address/address` | `interface-address` | iface |
| `iface-remove-address-before-interface` | `remove-address/address` | `remove-interface/interface` | `interface-address` | iface |
| `iface-remove-address-before-add-same-address` | `remove-address/address` | `add-address/address` | `same-address` | iface |
| `bgp-add-address-before-peer` | `add-address/address` | `add-peer/peer` | `address-used-by` | bgp |
| `bgp-remove-peer-before-address` | `remove-peer/peer` | `remove-address/address` | `address-used-by` | bgp |
| `bgp-add-address-before-listener` | `add-address/address` | `add-listener/listener` | `address-used-by` | bgp |
| `bgp-remove-listener-before-address` | `remove-listener/listener` | `remove-address/address` | `address-used-by` | bgp |

Rules are sorted by ID before graph construction for deterministic edge ordering.

### Decomposition ownership

Decomposition is component-owned, not centralized. Each component that knows how
to break its config root into atomic operations registers a decomposer via
`RegisterOperationDecomposer(root, fn)` in its `init()`. The planner calls
`OperationDecomposerFor(root)` per affected root and collects the results.

| Component | Config root | Decomposer | What it produces |
|-----------|------------|------------|-----------------|
| iface | `interface` | `decomposeIfaceOperations` | `add-interface`, `remove-interface`, `add-address`, `remove-address` per managed interface and IP |
| bgp | `bgp` | `decomposeBGPOperations` | `add-peer`, `remove-peer` per changed peer (modify = remove + add) |

A decomposer receives a `DecomposeRequest` with the transaction ID, the config
root name, the active and candidate root data (full JSON for that root), and
the diff. It returns a slice of `ConfigOperation` values. Each operation carries
its `Owner` field so the executor knows which plugin to contact for apply and
rollback.

Decomposers are selective: `decomposeIfaceOperations` returns nil unless the diff
touches addresses or managed interface types. `decomposeBGPOperations` returns nil
unless the diff touches the peer section. When a decomposer returns nil, that
root's changes flow through the existing full-diff apply path in the next
transaction (the operation path only activates when at least one decomposer
produces operations).

Components that do not register a decomposer (DNS, telemetry, DHCP) always use
the full-diff apply path. No code change is needed in those components.

### Graph construction and topological sort

`BuildOperationGraph` (in `depgraph.go`) takes the flat operation list and the
sorted constraint rules, then matches every operation pair against every rule.
When both selectors match and the resource relation holds, it adds a directed
edge. Duplicate edges (same from/to pair) are suppressed.

`TopologicalSort` (in `solver.go`) runs Kahn's algorithm on the graph. If all
operations are emitted, the sort is complete. If some operations remain (a cycle
exists), the solver attempts cycle relaxation.

#### Cycle detection and dual-presence fallback

Address operations can form cycles. An IP swap (move 10.0.0.1 from eth0 to eth1)
creates:

- Rule: `remove-address(10.0.0.1/eth0)` before `add-address(10.0.0.1/eth1)` (same-address uniqueness)
- Rule: `add-address(*/eth0)` before `remove-address(*/eth0)` (make-before-break on same interface)

If both interfaces are involved symmetrically (e.g., three-way rotation), these
edges form a cycle.

The solver handles this with `tryRelaxCycle`:

1. **Verify all cycle members are address operations.** If any non-address
   operation is in the cycle, the solver returns `ErrOperationCycle` (the
   transaction is aborted).

2. **Remove cross-interface edges.** Edges between address operations on
   different interfaces are dropped. Same-interface edges (make-before-break
   ordering) are preserved.

3. **Re-run Kahn's algorithm** on the reduced edge set. If the sort completes,
   the cycle is resolved.

4. **Mark dual-presence.** `ADD_ADDRESS` operations that were part of the relaxed
   cycle get `Params.AllowDual = true`. This tells the iface plugin that the
   address may temporarily exist on two interfaces during the transition. The
   kernel allows this; the solver makes it explicit.

Non-address cycles and same-interface cycles are not relaxable and cause
`ErrOperationCycle`, which aborts the transaction.

### Per-operation execution with settlement

The `OperationExecutor` (in `executor.go`) applies sorted operations sequentially
through per-plugin event callbacks. For each operation the sequence is:

1. **Arm settlement waiters** before emitting the apply event. For each
   `SettlementRule` matching the operation, the executor subscribes to the
   readiness event (namespace + event type + resource filter).

2. **Emit apply event** to the operation's owner plugin via
   `(config, operation-apply-<owner>)`.

3. **Wait for apply ack.** The owner responds with `operation-apply-ok` or
   `operation-apply-failed`.

4. **Wait for settlement.** Each armed waiter blocks until the matching readiness
   event arrives or the settlement timeout expires.

5. **Proceed to next operation** only after all settlement waiters for the
   current operation are satisfied.

#### Settlement waiter protocol

Settlement rules are registered as data via `RegisterSettlementRule`. Each rule
declares:

| Field | Purpose |
|-------|---------|
| `ID` | Unique rule identifier |
| `Operation` | `OperationSelector` matching which operations require this settlement |
| `Readiness` | `ConfigOperationReadiness{Namespace, EventType, Resource}` -- the event that signals completion |
| `ResourceFrom` | Which operation field supplies the resource value to match (`address`, `interface`, `peer`) |
| `Timeout` | Maximum wait time (capped at 60 seconds) |

The executor arms waiters before emitting the apply event so that readiness
events arriving during the apply callback are not missed. Each waiter subscribes
to the readiness event's namespace and event type via `gateway.SubscribeEvent`.
The waiter's callback checks whether the event payload contains the expected
resource value (matched against known keys: `resource`, `address`, `name`,
`interface`, `peer` in the payload JSON).

Examples from the codebase:

| Rule ID | Operation | Readiness event | Resource from | Timeout |
|---------|-----------|----------------|---------------|---------|
| `iface-add-address-settles-addr-added` | `add-address/address` | `(interface, addr-added)` | `address` | 5s |
| `iface-add-interface-settles-created` | `add-interface/interface` | `(interface, created)` | `interface` | 5s |
| `bgp-add-peer-settles-listener-ready` | `add-peer/peer` | `(bgp, listener-ready)` | `address` | 10s |

If a settlement timeout fires, the executor treats the operation as failed and
begins rollback.

### Operation verify phase

Before applying any operations, the executor runs a per-operation verify pass.
For each operation in sorted order, it emits
`(config, operation-verify-<owner>)` and waits for `operation-verify-ok` or
`operation-verify-failed`. A single failure aborts the transaction. This is
separate from the full-config verify in Phase 1: Phase 1 validates the overall
candidate config, while operation verify validates each atomic operation in the
context of the execution order.

### Rollback ordering

When an operation apply or settlement fails, the executor rolls back all
previously applied operations in reverse order:

1. **Per-operation reverse rollback.** For each applied operation, starting from
   the most recent, the executor emits
   `(config, operation-rollback-<owner>)` with that single operation. The owner
   undoes it and responds with `operation-rollback-ok` or
   `operation-rollback-failed`.

2. **Broadcast full-config rollback.** After per-operation rollback completes,
   the orchestrator emits the standard `(config, rollback)` broadcast (section 2,
   Phase 3) so that all transaction participants (including plugins that used the
   full-diff path for their roots) can undo via their journals.

3. **Reverse-tier ack collection.** Rollback acks are collected in reverse
   dependency-tier order, exactly as described in section 2.

This two-layer rollback ensures that operation-level changes are undone in the
correct reverse dependency order before the broader journal-based rollback
handles any remaining state.

### Operation commit

After all operations execute and settle successfully, the executor sends a commit
event per unique owner via `(config, operation-commit-<owner>)`. This tells each
owner to finalize its per-operation journals. The orchestrator then emits the
standard `(config, committed)` broadcast for the full transaction.

### Operation event types

All operation events live in the `config` namespace alongside the existing
transaction events. They are defined in
`internal/component/config/transaction/events/events.go`.

| Event type | Direction | Purpose |
|------------|-----------|---------|
| `operation-decompose-<plugin>` | Engine -> plugin | Decompose a root diff into operations |
| `operation-decompose-ok` | Plugin -> engine | Decomposition succeeded, includes operations |
| `operation-decompose-failed` | Plugin -> engine | Decomposition rejected |
| `operation-verify-<plugin>` | Engine -> plugin | Verify one operation before mutation |
| `operation-verify-ok` | Plugin -> engine | Operation verification passed |
| `operation-verify-failed` | Plugin -> engine | Operation verification rejected |
| `operation-apply-<plugin>` | Engine -> plugin | Apply one operation |
| `operation-apply-ok` | Plugin -> engine | Operation applied successfully |
| `operation-apply-failed` | Plugin -> engine | Operation apply failed |
| `operation-rollback-<plugin>` | Engine -> plugin | Roll back one or more operations |
| `operation-rollback-ok` | Plugin -> engine | Operation rollback succeeded |
| `operation-rollback-failed` | Plugin -> engine | Operation rollback failed |
| `operation-commit-<plugin>` | Engine -> plugin | Finalize operation journals |
| `operation-commit-ok` | Plugin -> engine | Operation commit succeeded |
| `operation-commit-failed` | Plugin -> engine | Operation commit failed |

### Plugin SDK callbacks

The SDK exposes five operation callbacks alongside the existing three
transaction callbacks:

| SDK method | RPC name | Phase | Required |
|------------|----------|-------|----------|
| `OnConfigOperationDecompose` | `config-operation-decompose` | Planning | Only for decomposer plugins |
| `OnConfigOperationVerify` | `config-operation-verify` | Per-operation verify | Only for operation owners |
| `OnConfigOperationApply` | `config-operation-apply` | Per-operation apply | Only for operation owners |
| `OnConfigOperationRollback` | `config-operation-rollback` | Per-operation rollback | Only for operation owners |
| `OnConfigOperationCommit` | `config-operation-commit` | Per-operation commit | Only for operation owners |

Plugins declare operation support in Stage 1 of the 5-stage startup protocol
via `ConfigOperations` in their registration, which lists the config roots
and operation types they handle.

### Integration with existing phases

The operation graph extension does not replace the existing transaction protocol.
It is an optional secondary path within a single transaction:

| Phase | Without operations | With operations |
|-------|-------------------|-----------------|
| Verify (Phase 1) | Full-config verify, all plugins | Identical -- unchanged |
| Plan | Skipped | Planner calls decomposers, builds operation list |
| Operation verify | Skipped | Per-operation verify in sorted order |
| Apply (Phase 2) | Full-diff broadcast to all plugins | Per-operation apply in sorted order with settlement |
| Commit | `(config, committed)` broadcast | Per-owner `operation-commit`, then `(config, committed)` broadcast |
| Rollback (Phase 3) | `(config, rollback)` broadcast | Per-operation reverse rollback, then `(config, rollback)` broadcast |
| Finalize | `(config, applied)` or `(config, rolled-back)` | Identical -- unchanged |

Transaction exclusion (section 6), apply journals (section 7), persistence
pointers (section 2), failure codes (section 12), and recovery (section 13) all
apply unchanged. The operation path is internal to the apply phase; the
transaction boundary and external-facing events remain the same.
