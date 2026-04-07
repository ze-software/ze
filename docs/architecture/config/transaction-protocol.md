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
| Runtime is authoritative | Config file is written only after all plugins confirm. Disk failure is a warning, not a rollback trigger. |

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
| All plugins applied | Engine emits `(config, committed)`, then writes the config file. | Runtime is authoritative. |
| All applied, file write fails | `committed` already emitted. Warning reported to caller. Runtime is live, file is stale. | Caller can retry save. |
| Rollback occurred | Config file untouched, engine emits `(config, rolled-back)`. | File still matches pre-transaction runtime. |

Runtime is the authority, not the file. The transaction succeeds when all
plugins apply. The file write is a persistence step that happens after
`(config, committed)`. If the file write fails, the runtime is still live
and correct -- the caller gets a warning (`saved=false` in the `applied`
event payload) and can retry.

`(config, applied)` and `(config, rolled-back)` are informational events for
observers (monitoring, web UI refresh, logging). `applied` includes a `saved`
boolean indicating whether the file write succeeded.

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

Example: bgp (tier 0, 10s) and sysrib (tier 0, 5s) run concurrently in tier 0;
fib-kernel (tier 1, 3s) depends on sysrib. Tier 0 max = max(10, 5) = 10s.
Tier 1 max = 3s. Total deadline = 10 + 3 = 13s. The pre-graph flat formula
would have returned 10s, missing the 3 seconds the fib plugin needs after
sysrib finishes.

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
| New config root added (e.g., `sysrib {}`) | Plugin loaded via 5-stage protocol **before** transaction starts | Plugin must be running to participate in verify |
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
Engine ----(config, verify-sysrib)----> [sysrib]
Engine ----(config, verify-dhcp)------> [dhcp]
  |   <---(config, verify-ok)---- [iface]
  |   <---(config, verify-ok)---- [bgp]
  |   <---(config, verify-ok)---- [sysrib]
  |   <---(config, verify-ok)---- [dhcp]
  |
Engine ----(config, apply-iface)------> [iface]
Engine ----(config, apply-bgp)--------> [bgp]
Engine ----(config, apply-sysrib)-----> [sysrib]
Engine ----(config, apply-dhcp)-------> [dhcp]
  |                              |
  |                              +--(interface, created)----> [dhcp]
  |                              +--(interface, addr-added)-> [bgp]
  |                              |
  |   <---(config, apply-ok)---- [iface]
  |   <---(config, apply-ok)---- [dhcp]   (after interface/created)
  |   <---(config, apply-ok)---- [bgp]    (after interface/addr-added)
  |   <---(config, apply-ok)---- [sysrib]
  |
  |
Engine ----(config, committed)--> [iface] [bgp] [sysrib] [dhcp]   (discard journals)
  |
  | write config file (best effort -- failure is warning, not rollback)
  |
Engine ----(config, applied)----> [web-ui] [monitoring] [logging]  (saved=true/false)
  |
  v
Caller: commit succeeded
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

The API engine's `ConfigCommit(sessionID, message)` calls `CommitSession`, which
triggers the transaction protocol. The API gets verify/apply/rollback for free.
The response includes the transaction outcome (`saved` flag indicates whether
the config file was persisted).

### Plugin SDK

The SDK exposes transaction participation through callbacks:

| Callback | Phase | Required |
|----------|-------|----------|
| `OnConfigVerify` | Verify | Yes for all participants. Returns estimated apply duration. |
| `OnConfigApply` | Apply | Yes for all participants. |
| `OnConfigRollback` | Rollback | Optional (plugins that can undo) |

Plugins that do not implement `OnConfigRollback` cannot undo. If such a plugin's
apply succeeds but another plugin triggers rollback, the engine logs a warning.
The config file is not reverted (it is the source of truth for intended state).

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

The config file always matches runtime (written only after successful apply).
A restarted plugin converges to the config file state by applying its config
roots from scratch during Stage 2.

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
| Config file write fails (after apply) | Warning to caller. Runtime is live. `(config, applied)` with `saved=false`. No rollback. |
| Concurrent commit attempted | Rejected with error. SIGHUP queued instead of rejected. |
| Engine crashes during transaction | Plugins hold journals. On restart, no `(config, committed)` arrives. Plugins detect stale journal (no matching active tx) and roll back on next startup. |
| Plugin exceeds rollback deadline (3x apply) | Treated as `broken`. Engine restarts plugin. |
| Plugin reports `broken` | Engine restarts plugin once via 5-stage protocol between rollback tiers. Second `broken` stops the plugin. |
