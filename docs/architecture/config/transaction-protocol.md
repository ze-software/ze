# Config Transaction Protocol

<!-- source: pkg/ze/bus.go -- Bus interface -->
<!-- source: internal/component/plugin/server/reload.go -- current verify/apply flow -->
<!-- source: internal/component/iface/iface.go -- bus topic conventions -->

Ze uses a bus-based transaction protocol for config changes. All phases (verify, apply,
rollback) are events on the bus. Plugins subscribe to the topics they care about and
react. The engine orchestrates deadlines but does not direct individual plugins.

---

## 1. Design Principles

| Principle | Rationale |
|-----------|-----------|
| Bus-native | Config transactions use the same pub/sub as all other cross-component coordination (interface events, RIB changes). No separate RPC path. |
| Plugin autonomy | Plugins decide what they need. A plugin that depends on an interface being created waits for both the apply event and the interface event. The engine does not manage dependency graphs. |
| Plugin-estimated timeouts | Plugins declare verify and apply budgets at registration, update them after each transaction. Engine enforces the max. No mid-transaction negotiation. |
| Rollback is an event | Any plugin can trigger rollback by publishing a failure. All plugins that already applied undo. No central orchestrator sends individual rollback commands. |
| Runtime is authoritative | Config file is written only after all plugins confirm. Disk failure is a warning, not a rollback trigger. |

---

## 2. Transaction Phases

A config transaction has three phases. Each phase is a bus event with a shared
transaction ID. Plugins subscribe to `config/` prefix to participate.

### Phase 1: Verify

The engine publishes a verify event. Every plugin that owns affected config roots
validates the candidate config against its constraints. Verification is non-destructive:
no state changes, no side effects.

| Step | Actor | Event |
|------|-------|-------|
| 1 | Engine | Publishes `config/verify` with transaction ID, affected roots, and candidate diffs |
| 2 | Plugin | Validates its portion. Responds with `config/verify/ok` including an estimated apply duration, or `config/verify/failed` |
| 3 | Engine | Collects responses. Every plugin must ack positively. A single `config/verify/failed` or a missing ack (timeout) fails the entire verify phase. Engine publishes `config/verify/abort`, transaction ends. |
| 4 | Engine | Computes the transaction deadline from the maximum estimated duration across all plugins. This becomes the deadline in the `config/apply` event. |

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
interest in. Plugins with affected roots validate and respond with an estimated
apply duration. Plugins whose watched roots are not affected by this transaction
respond `config/verify/ok` with zero duration. The engine expects exactly one
positive ack from every participating plugin. Apply is never sent unless all
are in.

The engine publishes per-plugin verify and apply events (filtered by declared
roots), not a single broadcast with the full config. Each plugin receives the
union of its `ConfigRoots` and `WantsConfig` roots. A DHCP plugin declaring
`ConfigRoots: ["dhcp"]` and `WantsConfig: ["interface"]` receives diffs for both
`dhcp` and `iface`, but never sees `bgp` or `telemetry` config.

The deadline is plugin-decided but engine-enforced. Plugins know their workload
after inspecting the diffs during verify. The engine takes the maximum across all
plugins and enforces it as the transaction deadline.

### Phase 2: Apply

After all verifications pass, the engine publishes the apply event. The config
file is NOT written yet -- it stays unchanged until all plugins confirm. Plugins
apply their changes from the candidate diffs.

| Step | Actor | Event |
|------|-------|-------|
| 1 | Engine | Publishes `config/apply` with transaction ID and diffs (config file unchanged) |
| 2 | Plugin | Applies changes. May produce side-effect events (interface creation, listener start, etc.) |
| 3 | Plugin | Publishes `config/apply/ok` when done |
| 4 | Plugin (failure) | Publishes `config/apply/failed` -- triggers rollback for all |

Plugins may depend on side-effect events from other plugins before completing their
apply. For example, a DHCP plugin that binds to an interface waits for both
`config/apply` and `interface/created` before it acts. The plugin manages this
dependency internally -- the engine does not track inter-plugin dependencies.

If a plugin receives `config/rollback` while still applying, it finishes the
in-progress apply and immediately undoes it.

### Phase 3: Rollback (conditional)

Triggered when any plugin publishes `config/apply/failed` or when the deadline
expires without all plugins completing.

| Step | Actor | Event |
|------|-------|-------|
| 1 | Engine | Publishes `config/rollback` with transaction ID (triggered by apply/failed or timeout) |
| 2 | Engine | Sends rollback in reverse tier order: highest-tier plugins first, then lower tiers |
| 3 | Plugins that applied | Undo changes via journal, publish `config/rollback/ok` |
| 4 | Plugins that had not started | Skip apply, publish `config/rollback/ok` |

Only the engine publishes `config/rollback`. A failing plugin publishes
`config/apply/failed`; the engine reacts by publishing rollback. This ensures
a single source of truth -- no duplicate rollback events from multiple sources.

Rollback follows reverse tier order (mirror of startup tiers). Plugins that
depend on side effects from other plugins roll back first, before the plugins
that produced those side effects. Within a tier, rollback is concurrent (same-tier
plugins don't depend on each other).

Rollback deadline is 3x the apply deadline. If a plugin exceeds this, it is
treated as `broken` (see Failure Codes).

### Completion

After all plugins respond (ok or rollback), the engine publishes a final notification:

| Outcome | Action | Event |
|---------|--------|-------|
| All plugins applied | Engine publishes `config/committed`, then writes config file | Runtime is authoritative |
| All applied, file write fails | `config/committed` already sent. Warning reported to caller. Runtime is live, file is stale. | Caller can retry save. |
| Rollback occurred | Config file untouched, publishes `config/rolled-back` | File still matches pre-transaction runtime |

Runtime is the authority, not the file. The transaction succeeds when all
plugins apply. The file write is a persistence step that happens after
`config/committed`. If the file write fails, the runtime is still live and
correct -- the caller gets a warning ("applied but not persisted") and can
retry.

`config/applied` and `config/rolled-back` are informational for observers
(monitoring, web UI refresh, logging). `config/applied` includes a
`saved` boolean indicating whether the file write succeeded.

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
Plugins estimate only their own work. The engine computes the critical path:

- Independent plugins: `max(A, B, C)`
- Dependency chain (iface -> DHCP): `sum(iface, DHCP)`
- Mixed: `max(sum(iface, DHCP), bgp, sysrib)`

The engine knows the dependency graph from `ConfigRoots` and `WantsConfig`
declarations. A plugin that declares `WantsConfig: ["interface"]` depends on
the iface plugin's side effects. The engine sums their budgets along the
chain and takes the max across independent chains.

This means DHCP estimates "2s for my work." Iface estimates "10s." The engine
computes the chain: 10s + 2s = 12s. Independent BGP estimates 5s. Deadline =
max(12s, 5s) = 12s. DHCP gets the full 12s without needing to guess iface's
duration.

### Self-Correcting Feedback

A plugin starts with a guess at registration. After seeing real diffs during
verify, it refines the apply estimate. After completing apply (or rollback),
it updates both estimates for next time.

If a plugin underestimates and times out, the engine publishes `config/rollback`.
The plugin's rollback response includes a reason (e.g., `timeout`). The engine
forwards this to the caller. On retry, the plugin provides a higher estimate
based on what it learned.

There is no mid-transaction extension mechanism. The feedback loop operates
between transactions, not within one.

---

## 4. Bus Events

All events use the `config/` topic prefix. Payloads are JSON. The transaction ID
ties all events in a transaction together.

### Topic Hierarchy

```
config/
  verify              Engine -> plugins: validate candidate
  verify/ok           Plugin -> engine: verification passed
  verify/failed       Plugin -> engine: verification rejected
  verify/abort        Engine -> plugins: verification phase failed, stop

  apply               Engine -> plugins: apply the changes
  apply/ok            Plugin -> engine: apply succeeded
  apply/failed        Plugin -> engine: apply failed, trigger rollback

  rollback            Engine -> plugins: undo applied changes
  rollback/ok         Plugin -> engine: rollback complete

  committed           Engine -> plugins: transaction finalized, discard journals
  applied             Engine -> observers: transaction committed (after committed)
  rolled-back         Engine -> observers: transaction rolled back
```

### Event Payloads

**config/verify**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID (unique per commit) |
| roots | list of string | Affected config roots (e.g., "bgp", "interface", "sysrib") |
| diffs | object | Per-root diffs: added, removed, changed sections as JSON |
| deadline | string | ISO 8601 timestamp for transaction timeout |

**config/verify/ok**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID |
| plugin | string | Plugin name |
| apply-budget | string | Estimated apply time for this transaction (e.g., "2s", "100s"). Zero means trivial. |

**config/verify/failed**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID |
| plugin | string | Plugin name |
| error | string | Failure reason |

**config/apply**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID |
| roots | list of string | Affected config roots |
| diffs | object | Per-root diffs |
| deadline | string | ISO 8601 timestamp |

**config/apply/ok**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID |
| plugin | string | Plugin name |
| next-verify-budget | string | Updated verify budget for next transaction |
| next-apply-budget | string | Updated apply budget for next transaction |

**config/apply/failed**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID |
| plugin | string | Plugin name |
| error | string | Failure reason |

**config/rollback**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID |
| reason | string | What triggered rollback (plugin failure or timeout) |

**config/rollback/ok**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID |
| plugin | string | Plugin name |
| code | string | Rollback result code (see Failure Codes below) |
| reason | string | Human-readable explanation |

**config/committed**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID |

**config/applied**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID |
| roots | list of string | Affected config roots |
| saved | boolean | Whether the config file was written to disk |

**config/rolled-back**

| Field | Type | Description |
|-------|------|-------------|
| tx | string | Transaction ID |
| roots | list of string | Affected config roots |

---

## 5. Plugin Load/Unload During Reload

When a config change adds or removes config roots, plugins must be loaded or
stopped. This happens outside the transaction, not inside it.

| Change | When | Why |
|--------|------|-----|
| New config root added (e.g., `sysrib {}`) | Plugin loaded via 5-stage protocol **before** transaction starts | Plugin must be running to participate in verify |
| Config root removed (e.g., `bgp {}` deleted) | Plugin participates in transaction (cleans up during apply), stopped **after** `config/committed` | Plugin needs to shut down resources cleanly via journal |

On rollback of a removal: the plugin rolled back its cleanup (resources restored).
It stays running. The config root was never actually removed (file not written).

On rollback of an addition: the newly loaded plugin rolled back its initial apply.
The engine stops and unloads it after `config/rolled-back`. It was loaded for
nothing, but no harm done.

---

## 6. Transaction Exclusion

Only one config transaction can be active at a time. While a transaction is in
progress (from `config/verify` until `config/applied` or `config/rolled-back`),
all other config operations are refused.

| Rejected operation | Response |
|--------------------|----------|
| CLI `commit` | Error: transaction in progress (tx ID, initiator) |
| API `ConfigCommit` | Error: transaction in progress |
| SIGHUP reload | Queued until current transaction completes |
| Editor `set`/`delete` | Allowed (candidate editing is independent of commit) |

The engine holds a transaction lock acquired at verify and released when the
final `config/applied` or `config/rolled-back` is published. The lock carries
the transaction ID and initiator for diagnostics.

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
journal replays the undo operations in reverse order. On `config/committed`, the
journal is discarded -- the changes are permanent.

### Lifecycle

| Event | Journal action |
|-------|----------------|
| `config/apply` received | Plugin creates a journal for this transaction ID |
| Plugin applies a change | Plugin records the change and its undo operation |
| `config/apply/ok` sent | Journal stays open, waiting for finalization |
| `config/committed` received | Journal discarded -- changes are permanent |
| `config/rollback` received | Journal replayed in reverse, then discarded |

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
and the error propagates -- the plugin publishes `config/apply/failed`.

### Example: Interface plugin

1. Journal created on `config/apply`
2. Create interface eth0: `Record(createEth0, deleteEth0)`
3. Assign IP 10.0.0.1: `Record(addAddr, removeAddr)`
4. Publish `config/apply/ok`
5a. `config/committed` arrives: journal discarded, eth0 + IP are permanent
5b. `config/rollback` arrives: journal replays `removeAddr` then `deleteEth0`

---

## 8. Transaction Finalization

After all plugins ack (apply/ok or rollback/ok), the engine publishes a
finalization event. This is the signal for plugins to discard their journals.

| Outcome | Event | Journal action |
|---------|-------|----------------|
| All applied | `config/committed` | Discard journal -- changes are permanent |
| Rollback completed | `config/rolled-back` | Journal already replayed during rollback |

`config/committed` is distinct from `config/applied`. The difference:

| Event | Audience | Purpose |
|-------|----------|---------|
| `config/committed` | Transaction participants (plugins) | Finalize journals, release transaction resources |
| `config/applied` | Observers (web UI, monitoring) | Informational notification |

The engine publishes `config/committed` first (participants finalize), then
`config/applied` (observers notified). A plugin must not discard its journal
until it receives `config/committed` -- without it, a late rollback could
arrive with no journal to replay.

---

## 9. Dependency Waiting

Plugins that depend on side effects from other plugins handle this internally.
The engine does not manage a dependency graph for the apply phase.

**Pattern:** A plugin subscribes to both `config/apply` and the dependency topic.
It applies only when both events have arrived for the same transaction.

### Example: DHCP binding to a new interface

1. Engine publishes `config/apply` -- iface plugin and DHCP plugin both receive it
2. Iface plugin creates the interface, publishes `interface/created`
3. DHCP plugin sees `config/apply` but waits for `interface/created` for its interface
4. `interface/created` arrives -- DHCP binds to the interface, publishes `config/apply/ok`

### Example: BGP binding to a new local address

1. Engine publishes `config/apply` -- iface plugin and BGP reactor both receive it
2. Iface plugin assigns the IP, publishes `interface/addr/added`
3. BGP reactor sees `config/apply` but waits for `interface/addr/added` for the local address
4. Address event arrives -- BGP starts the listener, publishes `config/apply/ok`

### Rollback during dependency wait

If `config/rollback` arrives while a plugin is waiting for a dependency event:

- The plugin cancels its wait
- Publishes `config/rollback/ok` (nothing was applied, nothing to undo)

---

## 10. Inter-System Event Flow

This table shows all events that cross system boundaries during a config transaction.
Systems that produce side-effect events during apply are listed with their outputs.

### Transaction Events (config/ prefix)

| Event | Producer | Consumers | Purpose |
|-------|----------|-----------|---------|
| `config/verify` | Engine | All plugins with affected config roots | Validate candidate |
| `config/verify/ok` | Plugin | Engine | Verification passed |
| `config/verify/failed` | Plugin | Engine | Verification rejected |
| `config/verify/abort` | Engine | All plugins | Stop verification |
| `config/apply` | Engine | All plugins with affected config roots | Apply changes |
| `config/apply/ok` | Plugin | Engine | Apply succeeded |
| `config/apply/failed` | Plugin | Engine | Apply failed |
| `config/rollback` | Engine | All plugins that received apply | Undo changes |
| `config/rollback/ok` | Plugin | Engine | Rollback complete |
| `config/committed` | Engine | All plugins | Finalize: discard journals |
| `config/applied` | Engine | Observers (web UI, monitoring, logging) | Transaction committed |
| `config/rolled-back` | Engine | Observers | Transaction rolled back |

### Side-Effect Events (produced during apply)

These are existing bus events that plugins publish as a consequence of applying
config changes. Other plugins may depend on them before completing their own apply.

| Event | Producer | Consumers | When |
|-------|----------|-----------|------|
| `interface/created` | iface | DHCP, BGP, telemetry | New interface configured |
| `interface/deleted` | iface | DHCP, BGP | Interface removed |
| `interface/addr/added` | iface | BGP (listener binding) | IP address assigned |
| `interface/addr/removed` | iface | BGP | IP address removed |
| `interface/dhcp/lease-acquired` | DHCP | BGP, DNS | DHCP lease obtained |
| `bgp/listener/ready` | BGP | (informational) | BGP listener bound to address |
| `bgp/state` | BGP | RIB, monitoring | Peer state change after config apply |

### Event Flow Diagram

```
Caller (Web UI / API / SIGHUP)
  |
  | commit(timeout)
  v
Engine ----config/verify-----> [iface] [bgp] [sysrib] [dhcp] ...
  |   <---config/verify/ok---- [iface]
  |   <---config/verify/ok---- [bgp]
  |   <---config/verify/ok---- [sysrib]
  |
Engine ----config/apply------> [iface] [bgp] [sysrib] [dhcp] ...
  |                              |
  |                              +--interface/created-----> [dhcp]
  |                              +--interface/addr/added--> [bgp]
  |                              |
  |   <---config/apply/ok------ [iface]
  |   <---config/apply/ok------ [dhcp]   (after interface/created)
  |   <---config/apply/ok------ [bgp]    (after interface/addr/added)
  |   <---config/apply/ok------ [sysrib]
  |
  |
Engine ----config/committed--> [iface] [bgp] [sysrib] [dhcp]   (discard journals)
  |
  | write config file (best effort -- failure is warning, not rollback)
  |
Engine ----config/applied----> [web-ui] [monitoring] [logging]  (saved=true/false)
  |
  v
Caller: commit succeeded
```

---

## 11. Relationship to Existing Systems

### Current Config Reload (replaced)

The current reload flow in `plugin/server/reload.go` uses direct RPC calls for
verify and apply. The transaction protocol replaces this with bus events:

| Current (RPC) | New (Bus) |
|---------------|-----------|
| `ConfigVerify(sections)` RPC per plugin | `config/verify` bus event, plugins subscribe |
| `ConfigApply(diffs)` RPC per plugin | `config/apply` bus event, plugins subscribe |
| No rollback | `config/rollback` bus event |
| Engine tracks affected plugins | Plugins self-select by subscribing to `config/` |

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

Plugins report a code in `config/rollback/ok` to tell the engine what happened.
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
| Plugin crashes during verify | Treat as verify/failed. Abort transaction. |
| Plugin crashes during apply | Treat as apply/failed. Publish rollback. |
| Plugin does not respond before deadline | Publish rollback. Log timeout. |
| Rollback callback fails | Log error, continue rollback for other plugins. |
| Multiple plugins fail simultaneously | First failure triggers rollback. Subsequent failures are logged. |
| Plugin receives rollback before starting apply | Skip apply, publish rollback/ok with code `ok`. |
| Config file write fails (after apply) | Warning to caller. Runtime is live. `config/applied` with `saved: false`. No rollback. |
| Concurrent commit attempted | Rejected with error. SIGHUP queued instead of rejected. |
| Engine crashes during transaction | Plugins hold journals. On restart, no `config/committed` arrives. Plugins detect stale journal (no matching active tx) and roll back on next startup. |
| Plugin exceeds rollback deadline (3x apply) | Treated as `broken`. Engine restarts plugin. |
| Plugin reports `broken` | Engine restarts plugin once via 5-stage protocol. Second `broken` stops the plugin. |
