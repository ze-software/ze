# Spec: irr-apply-policy

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Amendment 2026-10-08

Two owner decisions taken while designing `plan/spec-netbox-0-umbrella.md` (its D-15 and D-16)
were edited into the rows of this spec in place; nothing here needs to be "read as" anything else.
Every owner decision in "Owner Decisions" (Q-1 to Q-8) stands unchanged; Q-4's decision (held in
every mode for automatic applies, warning plus gauge, off by default) holds, and only the place the
threshold is configured moved.

| What changed | Rows edited |
|--------------|-------------|
| The policy decision, the YANG `apply` grouping and the window evaluator live in one shared package that the NetBox builder also uses: `internal/core/applypolicy` (Go) and the grouping-only module `internal/component/config/yang/modules/ze-apply-policy.yang`. The IRR store keeps the IRR-specific parts | Required Reading (`resolve.md`, `ze-system-conf.yang`), A-3, Data Flow step 2, Architectural Verification, Files to Modify, Files to Create, Implementation Steps 1 and 2, Unit Tests, Key Design Decisions, Deliverables |
| (Round-3 review of the NetBox umbrella, 2026-10-08) The `apply` grouping's cross-leaf refusals (`scheduled` needs both window leaves, `start` != `end`, `delay maximum` not below `minimum`) are ONE validation function in `internal/core/applypolicy`, called by each consumer's verify (both IRR consumers here, `netbox source <s> apply` in the NetBox set), never written in each consumer's `config.go`. `parseHHMM` lives in `internal/component/config/system/selfupdate_validate.go` (it also validates `restart-time`), not in `selfupdate.go`; it moves to `internal/core/applypolicy` and both self-update files call it | Required Reading (`ze-system-conf.yang`), Current Behavior, Architectural Verification, Unit Tests, Files to Modify, Files to Create, Integration Checklist, Implementation Steps 1, 2 and 5 |
| Thomas: "hold large removal should be a per system option value". The shrink threshold is ONE leaf, `system apply-policy shrink-threshold` (percent, 1..100, absent = off), added to `ze-system-conf.yang` by THIS spec and read by every consumer (`bgp-filter-irr`, `firewall-irr`, later the NetBox builder); it is not in the per-consumer `apply` container | Task, Key insights, Behavior to change, Data Flow entry point and step c, Wiring Test, AC-1, AC-17, AC-18, AC-21, AC-25, Boundary Tests, mock text, Files to Modify, Integration Checklist, Implementation step 1, Documentation rows 2 and 6, new A-10 |

## Task

Owner request (Thomas, 2026-10-08): "We should have an option for the IRR when we say we want
immediate update on pull, time-based, or only operator controlled."

Today a successful IRR fetch changes the enforced prefix list at once, in both consumers of the
shared IRR prefix store: the BGP import filter (`bgp-filter-irr`) and the firewall IRR plugin
(`firewall-irr`). `docs/architecture/core-design.md` section 22 "Sources and Read Views" states the
principle this spec implements: fetching new data and applying it are two separate decisions, and
when a fetched change takes effect is the operator's choice. That section currently says "Holding a
fetched change for a later commit or a schedule is not implemented."

Goal: a configurable apply policy per IRR consumer with three modes.

| Mode | Fetch | Apply |
|------|-------|-------|
| `immediate` (default) | background at `refresh-interval`, or on `update ... irr` | at once (today's behavior), or after a `delay`: either a fixed number of minutes, or a uniform random number of minutes between a minimum and a maximum |
| `scheduled` | same | only while a configured daily window is open |
| `operator` | same | only when an operator or an agent runs the apply command |

Two further owner decisions (2026-10-08) apply on top of the mode:

- **Apply delay (immediate mode).** Thomas: "immediate may mean between 0 and X minutes after successful fetch". An amendment the same day: "the waiting time could be one value (fixed) or range (delay)". `immediate` gets an optional `delay { minimum; maximum; }` container, both values in minutes.
  - With no container, there is no delay, which is today's behavior.
  - With `maximum` absent or equal to `minimum`, the delay is fixed: the change is applied exactly `minimum` minutes after the successful fetch.
  - With `maximum` above `minimum`, the delay is a range: the change is applied after a delay drawn uniformly at random between the two. The original "0 to X minutes" case is `minimum 0; maximum X`. A fleet then does not change all at once, and an alert or a hold has time to be seen.
  - Verify refuses `maximum` below `minimum`. While it waits, the change is a held change with reason `delay` and its due time, visible in `show <consumer> irr held`.
- **Shrink threshold (every mode).** Thomas adopted it: "it should raise an alert which can be caught via monitoring". A fetched change that removes more than `system apply-policy shrink-threshold` percent of a family's applied prefixes is held, not applied automatically, and it raises an alert on the existing report bus plus a Prometheus gauge and counter. Thomas placed the threshold at system level ("hold large removal should be a per system option value"): one leaf in `ze-system-conf.yang` that every consumer reads, not a leaf per consumer. The leaf is unset by default, which turns the check off.
- **`clear bgp irr asn|as-set`.** Thomas asked for it ("yes please") so that BGP matches the firewall. It has the same semantics as `clear firewall irr asn|as-set`. The firewall has no `clear ... all`, so BGP gets none.

A change that is fetched and not yet applied is called a **held change** in this spec and in every
operator-facing surface. The word `pending` is NOT used for it, because `show bgp irr` already
reports `status: pending` meaning "not resolved yet" (`internal/component/bgp/plugins/filter_irr/command.go`).

The operator must be able to see a held change (prefixes added and removed per family, age), apply
it per ASN, per AS-SET or all, and dismiss it. Every existing safety rule (empty or failed fetch never
replaces a list, `clear` is the deliberate removal, oversized refusal, stale marking) keeps holding in
every mode.

This spec is written for later implementation; it is not scheduled.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` section 22 - the principle: a source, a builder, a read view, a consumer that pulls when ready
  → Decision: the shared fetched list is the read view the builder (the store's fetch) produces; each consumer's applied snapshot is that consumer's pull. Holding is a consumer decision, so the policy is per consumer, never on the builder.
  → Constraint: a pull that fails keeps the last good read view; this spec must not let a held-mode apply bypass that (an apply copies only what the fetch guard already accepted).
  → Constraint: the sentence "Holding a fetched change for a later commit or a schedule is not implemented." must be replaced in the same piece of work that implements it (`ai/rules/documentation.md`).
- [ ] `docs/guide/irr-filtering.md` - operator contract for refresh, last-known-good, stale and clear
  → Constraint: "A refresh that returns prefixes atomically replaces the in-memory list and persists it in ZeFS" becomes mode-dependent and must be rewritten per mode.
  → Constraint: "At first enrollment, do not place the session into service until `show bgp irr` reports `status: ok`" stays true in every mode, which the bootstrap rule (Key Design Decisions) preserves.
- [ ] `docs/architecture/bgp/filter-irr.md` - BGP plugin decisions and locking constraints
  → Constraint: `handleConfigure` publishes `byASN`, `prefixStore`, `config`, `refreshStop` together under `plug.mu`; a worker that captured `st` must re-read `byASN[asn]` under the write lock before mutating. The new apply paths (command, window) mutate `st.list` and must obey the same re-read.
  → Constraint: "The shared prefix store holds entries this plugin never enrolled"; `loadFromStore` applies only enrolled ASNs. The applied snapshot load must keep that enrollment gate.
- [ ] `docs/architecture/firewall/firewall-irr.md` - firewall decisions: shared store, reload, oversized, interface binding
  → Constraint: the PrefixStore instance is created once and kept for the life of the plugin (`configureStore`); `Open` runs on every configure and re-reads the shared zefs file. With a per-consumer applied snapshot, `Open` must load fetched and applied separately, or a reload would leak another consumer's applied data into this consumer.
  → Constraint: an oversized entry refuses the whole apply (`refuseOversizedRefs`) and keeps the registered sets; a held-change apply of an oversized list must refuse the same way and leave the change held.
  → Constraint: `refresh-interval` defaults to 0 in the firewall ("Invalid IRR data causing a firewall outage is the risk that makes auto-refresh opt-in"); scheduled mode with interval 0 only applies manual fetches, which must be documented, not refused.
- [ ] `docs/architecture/resolve.md` - the resolution component and the shared store
  → Decision: the policy decision (mode, delay draw, window open, shrink check, given an injected clock and random source) lives once in `internal/core/applypolicy`, shared with the NetBox builder (`plan/spec-netbox-0-umbrella.md` D-16). The IRR-specific mechanism (fetched and applied prefix lists, their keys, held, apply, dismiss, the prefix-set diff, the held record's persistence) lives once in `internal/component/resolve/irr/store`, which calls the shared decision; both consumers share one implementation, and the mode value is passed in by each consumer.
- [ ] `ai/patterns/config-option.md` and `ai/rules/config.md` - leaf naming, `ze:help` plus `description`, native validation
  → Constraint: leaves are spelled in full; the env var mirror is required only under `environment/`, and neither IRR container is under it (`refresh-interval` has none), so no env var is added.
  → Constraint: every new node carries a one-line `ze:help` (96 chars, 25 words) and a different `description`.
- [ ] `ai/patterns/cli-command.md` and `ai/rules/cli.md` - verb choice, keyword before value, structured payload
  → Constraint: the verb follows the effect on live state: applying a held change is `update`, dismissing it is `clear`, viewing it is `show`. `commit` and `apply` are not verbs in the canonical registry.
  → Constraint: the response payload is structured data satisfying `ResponseData`, and JSON is built with `encoding/json`, never by string concatenation. The existing `show bgp irr` and `show firewall irr` handlers build JSON with `textbuf`; the new `held` answers must not copy that shape.
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` `maintenance-window` - the existing fetch-anytime, replace-in-window precedent
  → Decision: the scheduled window reuses that shape: `start` and `end` as `HH:MM` local time, a start later than end crosses midnight.
  → Constraint: the window evaluation is private to `config/system`: `inMaintenanceWindow` in `selfupdate.go`, and `parseHHMM` in `selfupdate_validate.go`, which also uses it to validate `restart-time` and the maintenance-window leaves. Reusing it means moving both to `internal/core/applypolicy` and making `selfupdate.go` and `selfupdate_validate.go` call them (`ai/rules/no-layering.md`), not copying them. The HH:MM parse then has one home, which the shared validation function also uses.
  → Decision: this spec adds the container `apply-policy` under `system` in this module, holding one leaf, `shrink-threshold` (uint8, `range 1..100`, `units percent`, no default). It is the one shrink threshold every consumer reads (owner, 2026-10-08: "hold large removal should be a per system option value"); A-10 covers how the two plugins read it.

- [ ] `docs/architecture/core-design.md` section 15 "Operational Report Bus" - the existing alert channel
  → Decision: the shrink alert is a WARNING, because it describes a state that is currently true and can resolve: `report.RaiseWarning(source, code, subject, message, detail)` when the hold starts, `report.ClearWarning(source, code, subject)` when it ends. It dedupes on `(Source, Code, Subject)`, so re-raising it on each fetch is safe. Operators see it in `show warnings`; monitoring scrapes the Prometheus gauge. No new channel.
  → Constraint: Source and Code are capped at 64 bytes and Subject at 256, Detail at 16 keys. Source is the consumer's existing subsystem (`bgp` for the filter, which also puts it in the BGP login banner that filters on source `bgp`; `firewall` for the firewall plugin). Code `irr-shrink-held`. Subject is the store entry name (`AS65001`, `AS-CUSTOMER`).
  → Constraint: the bus is a process-local package store. A plugin reaches it only when it runs inside the daemon process, which `internal/plugins/fib/kernel/fibkernel.go` already relies on (it calls `report.RaiseWarning` and `report.ClearWarning`). See A-8.
- [ ] `docs/plugin-development/metrics.md` - metric naming and registration through the plugin's `metrics.Registry`
  → Constraint: new gauges and counters register in the existing `SetMetricsRegistry`/`setMetricsRegistry` of each plugin, with the existing prefixes `ze_irr_` (BGP) and `ze_firewall_irr_` (firewall).

### RFC Summaries (Scope: protocol)
N-A: no protocol behavior changes. IRR whois (RPSL queries) is untouched; only when the result takes effect changes.

**Key insights:**
- Two plugins, two `PrefixStore` instances, one shared persisted key space `meta/irr/{name}`. Sharing is deliberate: one fetch warms both consumers.
- Because the persisted list is shared, a per-consumer policy cannot live on that one list: a consumer in `immediate` mode would write what another consumer in `operator` mode is holding, and that consumer's next `Open` (every firewall configure) would load it. The design splits fetched (shared) from applied (per consumer).
- A consumer "can hold" when its mode is `scheduled` or `operator`, OR its `delay` is above 0 minutes (fixed or range), OR `system apply-policy shrink-threshold` is set. Because that leaf is one system value, setting it (for example for NetBox, whose doctor check recommends it) makes BOTH IRR consumers holding consumers. A consumer that cannot hold (`immediate`, no delay, no threshold, the default) writes no applied snapshot and enforces the shared fetched list, exactly as today. The new storage is touched only by a consumer that can hold. Throughout this spec "held modes" means "a consumer that can hold".
- `show bgp irr` already owns `status: pending` (unresolved). The new concept is named `held`.
- BGP truncates an oversized list instead of refusing it (journal row added 2026-10-08 in `plan/journal/guard-addition-drops-what-it-refuses.md`); this spec does not depend on that defect being fixed but must not make it worse.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/resolve/irr/store/store.go` (569L) - `PrefixStore`: `Refresh` resolves (PeeringDB AS-SET discovery when no hint, `RefreshPrefixes` always hits the server), then either `markStale` (lookup error, or `ErrNoPrefixes` when both families are empty) or `commit`; `commit` keeps the cached prefixes of a family that answered nothing (`enforcedFamily`), carries `RefreshedAt`/`StaleSince`, writes memory and persists `meta/irr/{name}`. `Purge` removes memory and the key. `Open` migrates the legacy blob and loads every `meta/irr/*` key, trusting the key segment as identity. `UseClients` swaps resolvers and keeps entries. `Get` reads memory only.
- [ ] `internal/component/bgp/plugins/filter_irr/filter_irr.go` (629L) - `handleConfigure` builds a NEW `PrefixStore` per configure, `Open`s it, enrolls only peers that use IRR, carries old `asnState` lists over, calls `loadFromStore`, then starts `initialResolve` and `refreshLoop` (interval 60..86400, default 3600). `refreshASNCtx` calls `ps.Refresh(asnName(asn), asSet)`, re-reads `byASN[asn]` under the write lock, on error records `lastErr` and keeps `st.list`, on success rebuilds `st.list` via `prefixListFromIRR` (500000 cap, truncating), sets `lastOK`, counts, and signals `firstDone`. Metrics: `ze_irr_prefixes_cached`, `ze_irr_refresh_outcomes_total{result}`, `ze_irr_last_refresh_timestamp`.
- [ ] `internal/component/bgp/plugins/filter_irr/cache.go` - `loadFromStore` gives each enrolled ASN with no in-memory list the stored list as a fallback; `firstDone` stays open so the first UPDATE still waits for a fresh resolution.
- [ ] `internal/component/bgp/plugins/filter_irr/cmd_irr.go`, `command.go`, `yang/ze-filter-irr-cmd.yang` - `show bgp irr` (`status` ok/error/pending, `last-refresh`, `next-refresh`), `show bgp irr prefix`, `show bgp irr check`, `update bgp irr all|asn|as-set` (manual fetch). Each command node is a `ze:command` forwarded to the plugin through `pluginserver.RegisterRPCs`. There is no `clear bgp irr`.
- [ ] `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang` - `bgp policy irr { server; peeringdb-url; source-address; refresh-interval }`, per-peer `session irr { as-set; enable }` on three augment paths.
- [ ] `internal/component/firewall/plugins/irr/irr.go` (677L) - one `PrefixStore` for the plugin's life (`configureStore`, `UseClients`, `Open` each configure). `refreshAllNow` refreshes every ref then `applyTables`; `refreshName` (manual `update firewall irr asn|as-set`) refreshes one name then `applyTables`, returning `keptDataError` on an empty answer. `applyTables` refuses oversized refs, builds term and interface tables from `ps.Get`, registers and `ApplyAll`s. `OnConfigVerify` refuses a reference with no cached prefixes (`verifyRefs`). Metrics `ze_firewall_irr_prefixes_cached`, `ze_firewall_irr_refresh_outcomes_total{result}` (success, empty, error, panic, apply-failed), `ze_firewall_irr_last_refresh_timestamp`, `ze_firewall_irr_data_age_seconds`.
- [ ] `internal/component/firewall/plugins/irr/command.go`, `cmd_irr.go`, `yang/ze-firewall-irr-cmd.yang` - `show firewall irr` (per entry `status` ok/stale/oversized/missing, `last-refresh`, `data-age-seconds`, `stale-since`), `show firewall irr prefix`, `update firewall irr all|asn|as-set`, `clear firewall irr asn|as-set` (Purge then applyTables).
- [ ] `internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang` - `firewall irr { server; peeringdb-url; refresh-interval (0 or 60..86400, default 0); interface ... }` plus term `from` leaves `source-asn`, `source-as-set`, `destination-asn`, `destination-as-set`.
- [ ] `internal/component/config/system/selfupdate.go` - `inMaintenanceWindow` compares minutes-of-day against `start`/`end`, crossing midnight when start > end; it fails open on an unparsable value.
- [ ] `internal/component/config/system/selfupdate_validate.go` - `parseHHMM`, called here for `restart-time` and the maintenance-window leaves, and from `selfupdate.go`.
- [ ] `internal/test/mock/irr/irr.go` - `le test irr [--port N] [--empty-after-first]`, fixed answers for `AS-TEST` and `AS-V4ONLY`; no way to answer a different list on a later query.
- [ ] `pkg/zefs/keys.go` - `KeyIRRPrefixCache` `meta/irr/{name}`, `KeyIRRCache` legacy; both plugins grant access through `statestore.RegisterPluginKeys` in their `register.go`.
- [ ] `internal/core/clock/clock.go` - injectable `Clock` (`Now`, `NewTimer`, `NewTicker`) for deterministic tests.

**Behavior to preserve:**
- `immediate` mode, the default, is byte-for-byte today's behavior: same persisted keys, same `show` fields, same metrics values, same `.ci` expectations (every existing `test/plugin/filter-irr*.ci` and `test/plugin/firewall-irr*.ci` stays green unchanged).
- An empty answer or a failed fetch never replaces a list, per family, in every mode; stale marking and `StaleSince` semantics are unchanged.
- `clear firewall irr asn|as-set` removes the entry at once in every mode, and re-applies the firewall tables.
- The firewall refuses an oversized list for the whole apply and keeps the registered sets.
- Startup never blocks on IRR reachability (`initialResolve` stays detached); `firstDone` semantics for the first UPDATE stay.
- Command paths `update bgp irr all|asn|as-set` and `update firewall irr all|asn|as-set` keep meaning "fetch now".

**Behavior to change:**
- In `scheduled` and `operator` modes a successful fetch updates the shared fetched list but not what the consumer enforces; the difference is a held change.
- New commands to show, apply and dismiss held changes; new `show` fields; new metrics; a new config container in each consumer.
- `immediate` with a `delay` above 0 holds each change until its due time: exactly `minimum` minutes after the fetch for a fixed delay, or a uniform random time between `minimum` and `maximum` minutes for a range.
- With `system apply-policy shrink-threshold` set, an automatic apply that would remove more than that share of a family is held and raises the `irr-shrink-held` warning.
- New `clear bgp irr asn <n>` and `clear bgp irr as-set <name>`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `bgp policy irr apply { mode; delay { minimum; maximum; } window { start; end; } }` and `firewall irr apply { ... }`, delivered as JSON strings to each plugin's configure; `system apply-policy shrink-threshold`, delivered to both plugins through `ConfigReads` (A-10).
- Delay trigger (new): the earliest due time among delayed held changes, armed as one timer in the existing per-configure `refreshLoop`.
- Deliberate removal (new for BGP): `clear bgp irr asn|as-set`.
- Fetch triggers (unchanged): the refresh ticker, `initialResolve` (BGP), `update <consumer> irr all|asn|as-set`.
- Apply triggers (new): a fetch completing in `immediate` mode; the window opening, or a fetch completing while it is open, in `scheduled` mode; `update <consumer> irr apply all|asn|as-set` in `operator` and `scheduled` modes.
- Dismiss trigger (new): `clear <consumer> irr held all|asn|as-set`.
- View (new): `show <consumer> irr held [asn <n> | as-set <name>]`, plus held fields in `show <consumer> irr`.

### Transformation Path
1. Fetch: `PrefixStore.Refresh` resolves and runs the unchanged last-known-good guard; the accepted result becomes the shared **fetched** entry `meta/irr/{name}` (memory and zefs), exactly as today.
2. Decide: the store compares the consumer's **applied** snapshot with the new fetched entry and asks `internal/core/applypolicy` for the outcome. A consumer that cannot hold has no snapshot, and the fetched entry is enforced. A consumer that can hold applies these rules in order:

| Step | Condition | Outcome | Reason recorded |
|------|-----------|---------|-----------------|
| a | no snapshot and no prior fetched entry (first enrollment) | applied at once (bootstrap), whatever the threshold says | - |
| b | fetched equals applied (as prefix sets) | nothing held; any existing hold, delay timer and shrink alert are cleared | - |
| c | `system apply-policy shrink-threshold` set and, for either family, removed prefixes divided by that family's applied count exceed it | held; `irr-shrink-held` warning raised; a delay is not armed | `shrink` |
| d | mode `operator` | held | `operator` |
| e | mode `scheduled`, window closed | held | `window` |
| f | mode `scheduled`, window open | applied | - |
| g | mode `immediate`, `delay` configured and its `maximum` (or, when absent, its `minimum`) above 0 | held with a due time set the first time this name enters the hold: now + `minimum` for a fixed delay (`maximum` absent or equal to `minimum`), otherwise drawn uniformly in [now + `minimum`, now + `maximum`]; a newer fetch during the wait replaces the held list and keeps the due time (newest list wins, timer not restarted) | `delay` |
| h | mode `immediate`, no `delay` container, or a fixed delay of 0 | applied | - |

   The threshold only gates automatic applies (steps f, h and a delay or window coming due). An operator's `update ... irr apply` applies a shrink-held change on purpose. In step c a family with no applied prefixes cannot shrink.
3. Apply (held modes): the store copies the fetched prefixes into the consumer's applied snapshot `meta/irr-applied/<consumer>/{name}` and clears the hold. The trigger is the apply command, a window opening, or a delay coming due. A delay or window that comes due re-runs step c against the current lists first. The snapshot record persists the hold reason, `held-since`, the delay due time and the dismissal digest, so a restart resumes them. A due time already past at startup is applied after `Open`.
3b. Alert lifecycle: the `irr-shrink-held` warning is raised when step c holds a change. It is cleared when that hold ends for any reason: the change is applied by command, dismissed, removed by `clear`, or replaced by a later fetch that no longer exceeds the threshold.
4. Enforce: BGP rebuilds `st.list` from the entry the store says is enforced; the firewall runs `applyTables`, which reads the enforced entry through the same accessor. A refused enforcement (firewall oversized) rolls the applied snapshot back and leaves the change held.
5. View: the store computes the per-family diff between applied and fetched on demand (no stored diff), and the consumer renders it as structured data.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ plugin | `ze:command` YANG nodes forwarded by `pluginserver.RegisterRPCs` handlers to the plugin's `OnExecuteCommand` (existing pattern in both `cmd_irr.go`) | No |
| Plugin ↔ managed store | `store.KeyStore` (`ReadKey`, `WriteKey`, `RemoveKey`, `ListKeys`) over the plugin state RPC, keys granted by `statestore.RegisterPluginKeys` | No |
| Config ↔ plugin | plugin configure JSON (BGP: `bgp` root map; firewall: `ConfigSection` list), coerced with a `case string:` arm per `ai/rules/config.md` | No |
| Firewall plugin ↔ firewall registry | `firewall.RegisterTables` and `firewall.ApplyAll` (unchanged) | No |

### Integration Points
- `PrefixStore.Refresh` - gains the decide step; its existing guard and return contract (entry plus error, `ErrNoPrefixes`) stay.
- `PrefixStore.Get` - becomes "the entry this consumer enforces": the fetched entry in `immediate`, the applied snapshot in held modes. Every existing caller (`applyTables`, `buildIRRTables`, `buildIfaceTables`, `refuseOversizedRefs`, `verifyRefs`, `loadFromStore`, show handlers) keeps calling `Get` and gets the right answer with no edit.
- `PrefixStore.Open` - loads fetched keys and this consumer's applied keys into separate maps.
- `PrefixStore.Purge` - removes the fetched key and this consumer's applied snapshot for the name.
- BGP `refreshASNCtx` - installs `st.list` from what the store reports as enforced, never from the raw fetch result.
- Firewall `refreshName`, `refreshAllNow` - unchanged call shape; `applyTables` already reads `Get`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | fetch, decide and apply all go through `PrefixStore`; consumers only read `Get` and call the new apply and dismiss methods |
| No unintended coupling (components stay isolated) | Yes | neither plugin reads the other's config; each passes its own mode and its own applied key to its own store instance |
| No duplicated functionality (extends existing, does not recreate) | Yes | one held/apply implementation in the store; one policy decision, one window check and one validation of the `apply` grouping in `internal/core/applypolicy`, which self-update (window) and the NetBox builder (all three) also call |
| Zero-copy preserved where applicable (refs, not copies) | Yes | the applied snapshot shares the fetched prefix slices (both immutable after creation); the diff is computed on demand only for `show` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them | Yes | new `ze:command` nodes in each plugin's own `-cmd.yang` and `RegisterRPCs` entries in each plugin's `cmd_irr.go`; each consumer registers its applied zefs key in its own package |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name | Yes | searched: the store holds no consumer list (the consumer passes its key); the verb registry already has `show`, `update`, `clear`; the mode enum is YANG-native so completion derives from the schema; `pkg/zefs/keys.go` does not need to name the consumers if `MustRegister` is called from each plugin (A-2) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Both consumers fetch the same name into the same fetched entry, so a shared fetched list is correct even when their `server` leaves differ | today `meta/irr/{name}` is already shared regardless of server (`store.go` persist, `firewall-irr.md` "The PrefixStore is shared with BGP") | two servers would overwrite each other's fetched list; already true today, not introduced here | read of `Open`/`persist`; owner decision Q-5 (2026-10-08): keep sharing | validated |
| A-2 | `zefs.MustRegister` can be called from a plugin package, so each consumer declares its applied key without editing `pkg/zefs/keys.go` | `MustRegister` is an exported function (`pkg/zefs/registry.go`) | the two keys are added to `keys.go` instead, a central list with two entries | grep for `zefs.MustRegister(` outside `pkg/zefs`: `internal/plugins/fib/vpp/register.go` already registers `srv6OwnershipKey` that way (2026-10-08) | validated |
| A-3 | A YANG grouping can live in the grouping-only module `internal/component/config/yang/modules/ze-apply-policy.yang` (beside `ze-types.yang`, which holds no tree nodes) that both consumer modules and the NetBox module import, so the `apply` container is declared once | YANG `grouping`/`uses` across modules is used elsewhere (`ze-types`, `ze-extensions` imports) | the container is declared in each consumer with a comparison test pinning them equal | try `./le` YANG load of the grouping-only module; read how `ze-types.yang` is registered for loading | unvalidated |
| A-4 | A `.ci` can drive the same daemon store across a restart (two sequential `ze` runs over one store) | `test/plugin/firewall-irr-cold-cache-recovers.ci` exercises a cold cache; restart across runs not yet confirmed | the restart AC is proven at the store boundary by unit tests over a real `KeyStore` and a second `Open`, plus a `.ci` that kills and restarts the plugin (`failure-policy: restart`) | read `docs/functional-tests.md` and `ai/patterns/functional-test.md` for a restart directive | unvalidated |
| A-5 | An observer fixture plugin (`le test fixture plugin/...`) can read the wall clock and commit a `window` that excludes or includes now | existing firewall IRR `.ci` files drive commands through observer fixtures | the scheduled `.ci` proves only the always-open window; the closed-window path is unit-tested with an injected `clock.Clock` | read one fixture under `internal/test/fixture` | unvalidated |
| A-6 | Prefix-set equality is the right "changed" test, ignoring order and duplicates | `RefreshPrefixes` order is the server's; a reorder is not a change an operator should approve | spurious holds on every fetch | unit test with reordered identical answers | unvalidated |
| A-7 | Local time of the appliance is the right clock for the window, matching `maintenance-window` and the SMART `time` leaf | `ze-system-conf.yang` "Start time HH:MM in local time"; `ze-storage-conf.yang` "The clock is the local time of the appliance" | DST and timezone surprises; same as the precedents | owner decision 2026-10-08: local time | validated |
| A-8 | Both consumers run inside the daemon process, so `report.RaiseWarning` reaches the bus `show warnings` reads | `fibkernel.go` raises warnings from a plugin; the filter is a BGP plugin running in process (`docs/architecture/bgp/filter-irr.md`) | from a plugin running in a separate process, the warning never reaches the daemon's bus. The alert is then only the Prometheus gauge, which must be said in the guide, or the plugin must report over its RPC | confirm how `firewall-irr` is run (`register.go`, `docs/guide/plugins.md`); a `.ci` asserting `show warnings` carries the code (AC-21) | unvalidated |
| A-9 | `clear bgp irr` purging the shared fetched key `AS<n>` is acceptable even though a firewall `source-asn` ref of the same ASN reads that key | `clear firewall irr asn` already purges the same shared key for BGP today (`store.go` `Purge`) | an operator clearing on one side surprises the other; same as today, and both guides must say it | guide text reviewed; AC-24 | unvalidated |
| A-10 | Both IRR plugins read `system apply-policy shrink-threshold` by listing `system` in their registration's `ConfigReads`, and a commit that changes only that leaf re-delivers config to them | `registry.Registration.ConfigReads` (`internal/component/plugin/registry/registry.go`: "roots the plugin reads but does not own"; `bgp-rpki` reads `pki` that way, verified 2026-10-08) | the plugin keeps the old threshold until its own subtree changes; then the system component publishes the value to consumers (still one declaration) | read the reconfigure path for a `ConfigReads` root; a `.ci` that changes only the system leaf and asserts the new threshold in `show <consumer> irr` (AC-18) | unvalidated |
### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Cross-consumer leak: the firewall's `Open` on reload loads an applied state another consumer produced, bypassing `operator` mode | a held-mode firewall entry changes after a commit that touched only BGP | applied snapshots are keyed per consumer (`meta/irr-applied/<consumer>/{name}`); `Open` loads only this consumer's prefix; AC-12 tests it |
| R-2 | A first enrollment in a held mode has nothing applied, so BGP fails closed and firewall verify refuses the reference forever | `show` reports held with no applied list; verify refusal names an entry the operator already fetched | bootstrap rule: a fetch for a name with no applied snapshot and no prior fetched entry is applied at once in every mode, counted `bootstrap` (AC-6); owner question Q-1 |
| R-3 | Switching `immediate` to a held mode makes the current fetched list look like a held change, or applies nothing | `show ... held` lists every entry right after the mode change | on entering a held mode, an entry with a fetched list and no applied snapshot gets its snapshot seeded from the fetched list (what was being enforced); AC-8 |
| R-4 | Switching a held mode to `immediate` silently applies held changes | prefixes change at commit time with no operator action | this is the definition of `immediate` and is documented; the commit's log line names every held entry it applies; AC-9; owner question Q-6 |
| R-5 | An apply that the consumer refuses (firewall oversized) leaves the store saying "applied" while the kernel holds the old sets | `show firewall irr` status oversized and no held change | the apply is two-phase: the store applies, the consumer enforces, and a refusal rolls back the snapshot and keeps the hold, returning the refusal text (AC-14) |
| R-6 | Window evaluation drifts from self-update's when copied | two windows with the same config behave differently | one window check in `internal/core/applypolicy`, self-update moved onto it in the same change (`no-layering`); one table test covers both crossing-midnight cases |
| R-7 | The scheduled timer leaks a goroutine per configure | goroutine count grows across reloads | the window timer lives inside the existing per-configure `refreshLoop` select under the same `refreshStop` channel, no new goroutine (`ai/rules/goroutine-lifecycle.md`) |
| R-8 | A dismissed change resurfaces on every fetch, or a different later change is suppressed with it | `show ... held` flips between empty and non-empty with no new data | dismissal stores the digest of the dismissed fetched list; only a fetched list with the same digest stays dismissed (AC-11) |
| R-9 | Concurrent apply command and background fetch race on one name | an apply reports success for a list that is not the one shown | apply takes the store write lock and applies the fetched entry current at that instant; the reply carries the counts it applied; the BGP re-read under `plug.mu` rule holds |
| R-10 | BGP's oversized truncation (journal 2026-10-08) means a held BGP list over 500000 is applied truncated | WARN "prefix list exceeds cap" on apply | out of scope here; the journal row tracks it; this spec adds no new truncation path |
| R-11 | `show` handlers keep building JSON by hand, and the new fields copy that | review finds `textbuf` JSON in a new handler | new `held` answers are structured `ResponseData`; touching the existing `show <consumer> irr` adds fields through the same structured path, which converts that handler (Files to Modify) |
| R-12 | A delay that restarts on every fetch never comes due when the IRR keeps changing | a delayed hold older than the configured `maximum` (or `minimum` for a fixed delay) | the due time is drawn once per hold and never re-armed by a newer fetch (step g); AC-20 |
| R-13 | The shrink alert sticks after the hold ends, or clears while the change is still held | `show warnings` disagrees with `show ... held` | one function ends a hold, and it clears the warning on every path (apply, dismiss, clear, recovering fetch); AC-22 covers each path |
| R-14 | A delayed change is lost or applied early across a restart | due time missing after restart | the due time is persisted in the snapshot record; AC-23 |
| R-15 | `clear bgp irr` leaves the ASN with no list, so its peers' UPDATEs are rejected until the next fetch | rejected routes after a clear | this is the deliberate meaning of clear, the same as the firewall's; the reply says so and names `update bgp irr asn <n>` to fetch again; the next fetch is a bootstrap and applies |
## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | In `immediate` mode, nothing may change (AC-1 pins it). In held modes, a wrong decide step either applies a change the operator meant to hold (a filter changes without approval) or never applies (stale enforcement). A wrong snapshot key leaks across consumers. |
| How is it reverted? | Single commit revert. New zefs keys `meta/irr-applied/...` are left behind harmlessly; `meta/irr/{name}` keeps today's meaning and format. |
| Who else touches this path? | `plan/spec-blackhole-authorization-from-irr.md` and `plan/spec-irr-filtering-both-directions.md` read the same store and `prefixListFromIRR`; both must read `Get` (enforced), never the fetched entry. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config `bgp policy irr apply mode operator` + `update bgp irr asn 65001` | → | `PrefixStore.Refresh` decide step holds; `refreshASNCtx` keeps `st.list` | `test/plugin/filter-irr-apply-operator.ci` |
| `update bgp irr apply asn 65001` | → | `PrefixStore.Apply`, then `st.list` rebuild | `test/plugin/filter-irr-apply-operator.ci` |
| `show bgp irr held asn 65001` | → | `PrefixStore.Held` diff, structured payload | `test/plugin/filter-irr-apply-operator.ci` |
| config `firewall irr apply mode operator` + `update firewall irr as-set AS-TEST` | → | decide step holds; `applyTables` reads the applied snapshot | `test/plugin/firewall-irr-apply-operator.ci` |
| `update firewall irr apply as-set AS-TEST` | → | `PrefixStore.Apply` then `applyTables` | `test/plugin/firewall-irr-apply-operator.ci` |
| `clear firewall irr held as-set AS-TEST` | → | `PrefixStore.Dismiss` | `test/plugin/firewall-irr-apply-dismiss.ci` |
| config `bgp policy irr apply mode scheduled window { start; end; }` | → | window check in `refreshLoop` calls `PrefixStore.ApplyDue` | `test/plugin/filter-irr-apply-scheduled.ci` |
| config `apply mode scheduled` with no window | → | plugin verify refuses | `test/parse/irr-apply-scheduled-needs-window.ci` |
| config `bgp policy irr apply delay { minimum 0; maximum 1; }` + fetch | → | decide step g (range draw), delay timer in `refreshLoop`, `ApplyDue` | `test/plugin/filter-irr-apply-delay-range.ci` |
| config `firewall irr apply delay { minimum 1; }` + fetch | → | decide step g (fixed due time), delay timer in the firewall refresh loop, `ApplyDue`, `applyTables` | `test/plugin/firewall-irr-apply-delay-fixed.ci` |
| config `apply delay { minimum 5; maximum 2; }` | → | plugin verify refuses `maximum` below `minimum` | `test/parse/irr-apply-delay-maximum-below-minimum.ci` |
| config `system apply-policy shrink-threshold 20` + shrinking fetch | → | decide step c, `report.RaiseWarning`, gauge | `test/plugin/firewall-irr-apply-shrink-alert.ci` |
| `clear bgp irr asn 65001` | → | `PrefixStore.Purge`, `st.list` cleared under `plug.mu` | `test/plugin/filter-irr-clear.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No `apply` container configured, or `mode immediate` with no `delay` above 0, and `system apply-policy shrink-threshold` absent, either consumer | Behavior identical to today: every existing `filter-irr*.ci` and `firewall-irr*.ci` passes unchanged; `show` gains `apply-mode: immediate` and no held fields; no `meta/irr-applied/` key is written |
| AC-2 | `mode immediate` explicitly | Same as AC-1 |
| AC-3 | `mode operator`, an applied list exists, a fetch returns a different non-empty list | Enforced list unchanged (a route matching only the new prefix is still rejected by BGP; firewall sets unchanged); `show <consumer> irr` reports the entry `held: true`, `held-since`, held IPv4/IPv6 added and removed counts; `ze_irr_held_entries` (BGP) or `ze_firewall_irr_held_entries` is 1 |
| AC-4 | AC-3 state, then `show <consumer> irr held asn <n>` (or `as-set <name>`) | Payload lists per family the prefixes added and removed, `held-since`, `held-age-seconds`, the fetched and applied counts, and the AS-SET; `| json` renders kebab-case keys; a name with no held change answers an empty held object with `held: false`, never an error; an unknown name is refused naming it |
| AC-5 | AC-3 state, then `update <consumer> irr apply asn <n>` | The fetched list becomes enforced (BGP accepts a route in the new prefix; firewall sets carry it); held cleared; reply names the counts applied; `apply` outcome counter `committed` +1. `apply all` applies every held entry and reports each; `apply as-set <name>` applies every entry resolved to that AS-SET |
| AC-6 | `mode operator` or `scheduled`, a name with no applied snapshot and no fetched entry (first enrollment), first successful fetch | Applied at once (bootstrap), counted `bootstrap`, logged at INFO naming the entry and saying the mode would otherwise hold it; `show` reports `status: ok`, not held |
| AC-7 | `mode operator`, restart of the daemon (or plugin) with a held change | After restart the applied list is enforced (BGP `loadFromStore`, firewall `applyTables`), the held change is still reported with its original `held-since`, and the startup fetch does not apply it |
| AC-8 | Config change `immediate` to `operator` with existing fetched entries | No enforced list changes; each entry's applied snapshot is seeded from its fetched list; `show ... held` is empty |
| AC-9 | Config change `operator` to `immediate` with a held change | The held change is applied at commit time, every applied entry logged at INFO; applied snapshots for this consumer are removed |
| AC-10 | Held modes, a fetch returns no prefixes or fails | Same as today: fetched list unchanged, marked stale; no new held change is created by it; an existing held change stays held |
| AC-11 | AC-3 state, then `clear <consumer> irr held asn <n>` | Held change dismissed: `show` reports `held: false`, the enforced list unchanged; the next fetch returning the same list does not re-raise it; a later fetch returning a different list raises a new held change; dismiss counted `dismissed` |
| AC-12 | BGP in `immediate`, firewall in `operator`, both enrolled on the same name; BGP fetches a change; firewall reloads config | Firewall still enforces its applied snapshot and reports the change held; BGP enforces the new list |
| AC-13 | `mode scheduled`, window open (injected clock in unit test; always-open window in `.ci`), fetch returns a different list | Applied at once, counted `scheduled`. Window closed: held. At the window's start (timer, no fetch) every held entry is applied |
| AC-14 | Firewall held mode, apply of a held list whose family exceeds the set bound | Apply refused with the existing oversized message naming the entry and the count; enforced sets unchanged; change stays held; counted `refused` |
| AC-15 | `clear firewall irr asn|as-set` in any mode | Entry removed at once (fetched, applied snapshot, held state, dismissal) and tables re-applied, as today |
| AC-16 | `mode scheduled` without both `window start` and `window end`, or `start` equal to `end`, or a value not `HH:MM` | Commit refused at verify with a message naming the leaf and the expected form |
| AC-17 | `update <consumer> irr apply ...` for a consumer that cannot hold (AC-1 config) | Refused: "nothing is held: apply mode is immediate with no delay and no system shrink threshold", naming the configured mode; exit non-zero. For a consumer that can hold but a name with nothing held, the reply says nothing was held for that name, without an error |
| AC-18 | `show <consumer> irr` in any mode | Top level carries `apply-mode`, `delay-minimum-minutes`, `delay-maximum-minutes` and `shrink-threshold-percent` (the value of `system apply-policy shrink-threshold`; each absent when unset; a commit that changes only the system leaf changes this field, A-10; a fixed delay reports `delay-maximum-minutes` equal to `delay-minimum-minutes`); in `scheduled` also `window-start`, `window-end`, `window-open` (bool), `next-window` (RFC 3339). Every held entry carries `held-reason` (`operator`, `window`, `delay`, `shrink`) |
| AC-19 | Range delay: `mode immediate`, `delay { minimum 2; maximum 10; }`, a fetch returns a different list | Not applied at once; `show <consumer> irr held` lists it with `held-reason: delay` and `apply-due` (RFC 3339) between fetch time + 2 minutes and fetch time + 10 minutes; with a seeded random source, repeated holds draw due times spread over that interval, not one fixed value; applied when `apply-due` passes (unit test with injected clock; `.ci` with `minimum 0; maximum 1` and a 90 s timeout); counted `delayed` then `applied` |
| AC-19b | Fixed delay: `mode immediate`, `delay { minimum 5; }` (no `maximum`), and separately `delay { minimum 5; maximum 5; }`, a fetch returns a different list | Not applied at once; held with `held-reason: delay` and `apply-due` exactly `held-since` + 5 minutes in both configs, with no random draw; applied when `apply-due` passes (unit test with injected clock; `.ci` with `minimum 1`, asserting `apply-due` minus `held-since` is 60 s, applied within a 90 s timeout); counted `delayed` then `applied` |
| AC-19c | `delay { minimum 5; maximum 2; }` in either consumer | Commit refused at verify with a message naming `maximum`, `minimum` and both values, and saying `maximum` must not be below `minimum` |
| AC-20 | AC-19 state, a newer fetch returns a third list before `apply-due` | The held list becomes the newest fetched list; `apply-due` unchanged; at `apply-due` the newest list is applied. A newer fetch equal to the applied list ends the hold and cancels the due time |
| AC-21 | `system apply-policy shrink-threshold 20`, applied IPv4 list of 3 prefixes, a fetch removes 1 (33%) in any mode | Change held with `held-reason: shrink`; `show warnings` carries source `bgp` or `firewall`, code `irr-shrink-held`, subject the entry name, and detail with family, applied count, removed count and threshold; `ze_irr_shrink_held_entries` / `ze_firewall_irr_shrink_held_entries` is 1; counter `shrink-held` +1. A fetch removing 1 of 10 (10%) applies normally |
| AC-22 | AC-21 state, then each of: `update ... irr apply`, `clear ... irr held`, `clear ... irr asn|as-set`, a fetch whose change no longer exceeds the threshold | The warning is cleared and the gauge returns to 0 on every path. The apply command applies the shrinking list deliberately. A dismissal keeps the enforced list |
| AC-23 | AC-19 state, then a restart before `apply-due` | After restart the change is still held with the same `apply-due` and is applied when it passes. A restart after `apply-due` applies it right after load |
| AC-24 | `clear bgp irr asn 65001` (and `clear bgp irr as-set AS-TEST`, which clears every enrolled ASN resolved to that AS-SET) | Entry removed from memory and from ZeFS (fetched entry, BGP applied snapshot, hold, dismissal, shrink warning); `show bgp irr` reports the ASN with no list; the reply names the entries removed and says to run `update bgp irr asn <n>` to fetch again; an ASN or AS-SET no IRR-filtered peer uses is refused with the plugin's existing "no IRR-filtered peer with ASN" wording, and an enrolled name with nothing cached is refused "no cached data for <name>", as `purge` in `firewall/plugins/irr/command.go` does |
| AC-25 | `delay minimum` or `delay maximum` outside 0..1440, `system apply-policy shrink-threshold` outside 1..100 | Refused by YANG range at commit |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets BGP IRR to operator mode, sees an upstream change held, reviews it, applies it | config → `handleConfigure` → refresh → `Refresh` holds → `show bgp irr held` → `update bgp irr apply asn` → `Apply` → `st.list` → filter accepts new prefix | `test/plugin/filter-irr-apply-operator.ci` |
| 2 | Sets firewall IRR to operator mode and dismisses a suspicious change | config → `configure` → `update firewall irr as-set` → held → `clear firewall irr held as-set` → dismissed; sets unchanged | `test/plugin/firewall-irr-apply-dismiss.ci` |
| 3 | Sets a nightly window; changes fetched during the day apply in the window | config → `refreshLoop` → held → window timer → `ApplyDue` → enforcement | `test/plugin/filter-irr-apply-scheduled.ci` plus `TestApplyDueAtWindowStart` |
| 4 | An agent reads held changes as JSON and applies them | `show firewall irr held | json` → decides → `update firewall irr apply all` | `test/plugin/firewall-irr-apply-operator.ci` |
| 5 | Restarts the router with a change held | daemon restart → `Open` loads fetched and applied → enforce applied → held still reported | `test/plugin/filter-irr-apply-restart.ci` (A-4) |
| 6 | Sets a delay range of 0 to 30 minutes so a fleet spreads its IRR updates | config `delay { minimum 0; maximum 30; }` → fetch → held `delay` with a random `apply-due` → timer → apply | `test/plugin/filter-irr-apply-delay-range.ci` |
| 6b | Sets a fixed 15-minute delay so every change waits a known time before it takes effect | config `delay { minimum 15; }` → fetch → held `delay` with `apply-due` = `held-since` + 15 minutes → timer → apply | `test/plugin/firewall-irr-apply-delay-fixed.ci` |
| 7 | Monitoring catches an IRR change that would remove a large share of a customer's prefixes | fetch → step c → `show warnings` and `ze_firewall_irr_shrink_held_entries` → operator reviews `held` → applies or dismisses → alert clears | `test/plugin/firewall-irr-apply-shrink-alert.ci` |
| 8 | Removes the cached list of a deregistered customer AS from the BGP filter | `clear bgp irr asn` → `Purge` → `st.list` cleared | `test/plugin/filter-irr-clear.ci` |
## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestImmediateModeWritesNoSnapshot` | `internal/component/resolve/irr/store/store_test.go` | AC-1, AC-2 | |
| `TestHeldModeHoldsChangedFetch` | same | AC-3 | |
| `TestHeldModeEqualFetchIsNotHeld` (reordered answer) | same | A-6 | |
| `TestHeldDiffPerFamily` | same | AC-4 | |
| `TestApplyCopiesFetchedAndClearsHold` | same | AC-5 | |
| `TestBootstrapAppliesFirstFetch` | same | AC-6 | |
| `TestOpenLoadsFetchedAndOwnSnapshotOnly` | same | AC-7, AC-12 | |
| `TestEnteringHeldModeSeedsSnapshot` | same | AC-8 | |
| `TestLeavingHeldModeAppliesAndRemovesSnapshots` | same | AC-9 | |
| `TestEmptyOrFailedFetchCreatesNoHold` | same | AC-10 | |
| `TestDismissByDigest` | same | AC-11 | |
| `TestApplyRollbackOnConsumerRefusal` | same | AC-14 | |
| `TestPurgeRemovesSnapshotAndHold` | same | AC-15 | |
| `TestApplyDueAtWindowStart`, `TestApplyDueOutsideWindowHolds` | same, with injected `clock.Clock` | AC-13 | |
| `TestWindowContains` (table, crossing midnight, start==end refused) | `internal/core/applypolicy/window_test.go` | AC-13, AC-16, R-6 | |
| `TestDecideTable` (steps a to h of Data Flow step 2, injected clock and random source) | `internal/core/applypolicy/decide_test.go` | AC-3, AC-13, AC-19, AC-19b, AC-21 | |
| `TestValidateApplyConfig` (`scheduled` without both window leaves, `start` == `end`, a value not `HH:MM`, `maximum` below `minimum`; each message names the leaf) | `internal/core/applypolicy/validate_test.go` | AC-16, AC-19c | |
| `TestSelfUpdateUsesSharedWindow` (`selfupdate.go` and `selfupdate_validate.go` call the moved window check and HH:MM parse) | `internal/component/config/system/selfupdate_test.go` | R-6 (no copy left) | |
| `TestDelayDueTimeDrawnOnceWithinBound` (range: due time in [`minimum`, `maximum`], drawn once per hold) | `store_test.go`, injected clock and seeded random source | AC-19 | |
| `TestFixedDelayDueTimeExact` (`maximum` absent, and `maximum` equal to `minimum`: due time exactly `minimum` minutes, random source never consulted) | same | AC-19b | |
| `TestNewerFetchDuringDelayKeepsDueTime`, `TestFetchEqualToAppliedCancelsDelay` | same | AC-20 |
| `TestDelayedHoldSurvivesReopen`, `TestPastDueAppliedAfterOpen` | same | AC-23 |
| `TestShrinkThresholdHoldsPerFamily` (33% held, 10% applied, empty applied family never shrinks) | same | AC-21 |
| `TestShrinkWarningRaisedAndClearedOnEveryPath` (apply, dismiss, purge, recovering fetch; reads `report.Warnings()`) | same | AC-22, R-13 |
| `TestOperatorApplyIgnoresThreshold` | same | step c scope |
| `TestClearBGPIRRPurgesAndEmptiesList`, `TestClearBGPIRRRefusesUnknown` | `filter_irr/command_test.go` | AC-24 |
| `TestParseApplyConfig` (both consumers, `case string:` coercion, `delay` with and without `maximum`) | `filter_irr/config_test.go`, `firewall/plugins/irr/config_test.go` | AC-16, AC-19, AC-19b | |
| `TestParseDelayRefusesMaximumBelowMinimum` (each consumer's verify reaches the shared validation function) | same | AC-19c | |
| `TestRefreshASNKeepsListWhenHeld` | `filter_irr/filter_irr_test.go` | AC-3 BGP side | |
| `TestApplyRefusedInImmediateMode` | both `command_test.go` | AC-17 | |
| `TestShowHeldPayloadShape` | both `command_test.go` | AC-4, AC-18, kebab-case keys | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `window start` hour | 00..23 | 23:59 | N/A (pattern) | 24:00 |
| `window start` minute | 00..59 | 00:59 | N/A | 00:60 |
| `window end` | same as start | 23:59 | N/A | 24:00 |
| window length | start != end | 00:00 to 00:01 | start == end refused | N/A |
| `delay minimum` (minutes) | 0..1440 | 0 and 1440 | N/A (uint) | 1441 |
| `delay maximum` (minutes) | `minimum`..1440 | equal to `minimum`, and 1440 | `minimum` - 1 (refused at verify, AC-19c) | 1441 |
| `system apply-policy shrink-threshold` (percent) | 1..100 | 1 and 100 | 0 | 101 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-irr-apply-operator` | `test/plugin/filter-irr-apply-operator.ci` | BGP held change visible, route still rejected, applied by command, route accepted | |
| `firewall-irr-apply-operator` | `test/plugin/firewall-irr-apply-operator.ci` | firewall held change visible as JSON, sets unchanged, applied by `apply all` | |
| `firewall-irr-apply-dismiss` | `test/plugin/firewall-irr-apply-dismiss.ci` | dismissed change stays dismissed across a same-answer fetch | |
| `filter-irr-apply-scheduled` | `test/plugin/filter-irr-apply-scheduled.ci` | always-open window applies at fetch; `show` reports window fields | |
| `filter-irr-apply-restart` | `test/plugin/filter-irr-apply-restart.ci` | held change survives restart, applied list enforced | |
| `firewall-irr-apply-cross-consumer` | `test/plugin/firewall-irr-apply-cross-consumer.ci` | AC-12 | |
| `irr-apply-scheduled-needs-window` | `test/parse/irr-apply-scheduled-needs-window.ci` | AC-16 refusal text | |
| `filter-irr-apply-delay-range` | `test/plugin/filter-irr-apply-delay-range.ci` | `delay { minimum 0; maximum 1; }`: change shown held with `apply-due` within 60 s of the fetch, applied within the 90 s timeout | |
| `firewall-irr-apply-delay-fixed` | `test/plugin/firewall-irr-apply-delay-fixed.ci` | `delay { minimum 1; }`: change shown held with `apply-due` exactly 60 s after `held-since`, firewall sets unchanged until then, applied within the 90 s timeout | |
| `irr-apply-delay-maximum-below-minimum` | `test/parse/irr-apply-delay-maximum-below-minimum.ci` | AC-19c refusal text | |
| `firewall-irr-apply-shrink-alert` | `test/plugin/firewall-irr-apply-shrink-alert.ci` | shrinking fetch held, `show warnings` carries `irr-shrink-held`, gauge 1; dismiss clears both | |
| `filter-irr-shrink-alert` | `test/plugin/filter-irr-shrink-alert.ci` | same on the BGP side, warning under source `bgp` | |
| `filter-irr-clear` | `test/plugin/filter-irr-clear.ci` | `clear bgp irr asn` empties the list, a route is rejected, `update bgp irr asn` restores it | |
| `completion-words-irr-held` | `test/ui/completion-words-irr-held.ci` | completion offers `held`, `apply`, and the mode enum | |

The mock whois server needs one new option so a fetch can return a changed list:
`le test irr --change-after-first` answers each query once with the current data, then with a
second fixed answer for `AS-TEST` that adds one IPv4 prefix and removes another (and adds one IPv6
prefix). Removing one of the three `AS-TEST` IPv4 prefixes is a 33% shrink, so the same option drives
the shrink tests with `system apply-policy shrink-threshold 20`. The usage text in `internal/test/mock/irr/irr.go`
documents both answers.

### Interop Tests (Scope: protocol)
N-A: no wire-visible change. IRR queries are unchanged; BGP and nftables output differ only in when a list takes effect.

## Files to Modify
- `internal/component/resolve/irr/store/store.go` - fetched vs applied split, mode, decide step, `Apply`, `ApplyDue`, `Dismiss`, `Held` (diff), `Open` loading per-consumer snapshots, `Purge` extension, injected `clock.Clock`
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` - pass mode and applied key to the store, install `st.list` from the enforced entry, window branch in `refreshLoop`, new metrics
- `internal/component/bgp/plugins/filter_irr/config.go` - parse `apply { mode; delay { minimum; maximum; } window { start; end; } }` and `system apply-policy shrink-threshold`; verify calls the shared validation function of `internal/core/applypolicy` for AC-16 and AC-19c (no local copy of the rules)
- `internal/component/config/system/yang/ze-system-conf.yang` - `system apply-policy shrink-threshold` (uint8, `range 1..100`, `units percent`, no default, `ze:help` and `description`)
- `internal/component/bgp/plugins/filter_irr/command.go`, `cmd_irr.go` - `show bgp irr held`, `update bgp irr apply`, `clear bgp irr held`, `clear bgp irr asn|as-set`, held fields in `show bgp irr` (structured payload), shrink warning raise and clear (source `bgp`), new gauges and counter results
- `internal/component/bgp/plugins/filter_irr/cache.go` - `loadFromStore` reads the enforced entry (unchanged call, documented)
- `internal/component/bgp/plugins/filter_irr/register.go` - register the BGP applied key and grant it; add `system` to `ConfigReads` (A-10)
- `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang`, `ze-filter-irr-cmd.yang` - `apply` container, new command nodes
- `internal/component/firewall/plugins/irr/irr.go`, `config.go`, `command.go`, `cmd_irr.go`, `register.go` - same set for the firewall consumer (warning source `firewall`, `system` in `ConfigReads`), verify calling the shared validation function for AC-16 and AC-19c, rollback on refused apply
- the store's decide step calls `internal/core/applypolicy` for the delay due time (fixed: `minimum`; range: a draw from `math/rand/v2`, injectable for tests) and the threshold check; the warning raise and clear sit in the one store function that starts and ends a hold, called through a consumer-supplied hook so the store does not hard-code a report source
- `internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang`, `ze-firewall-irr-cmd.yang` - `apply` container, new command nodes
- `internal/component/config/system/selfupdate.go` - call the window check in `internal/core/applypolicy`; `inMaintenanceWindow` moves out
- `internal/component/config/system/selfupdate_validate.go` - `parseHHMM` moves to `internal/core/applypolicy`; the `restart-time` and maintenance-window validation calls the moved parse
- `internal/test/mock/irr/irr.go` - `--change-after-first`
- `internal/component/config/yang/loader.go` - `LoadEmbedded` lists the embedded bootstrap modules by path (today `ze-extensions.yang` and `ze-types.yang`); add `modules/ze-apply-policy.yang` so the grouping loads before the consumer modules that `uses` it (A-3); `docs/architecture/config/yang-config-design.md` ("`LoadEmbedded()` loads the two foundation modules", and its module table) gains the third module in the same change
- `docs/guide/irr-filtering.md`, `docs/architecture/core-design.md` (section 22), `docs/architecture/bgp/filter-irr.md`, `docs/architecture/firewall/firewall-irr.md`, `docs/architecture/resolve.md`, `features/irr-bgp-import-filtering.md`, `docs/guide/configuration.md` (IRR section), `docs/guide/command-reference.md`, `docs/plugin-development/metrics.md` or the IRR telemetry section, `ai/INDEX.md`

## Files to Create
- `internal/core/applypolicy/` - the shared policy package: the decision (mode, delay draw, window open, shrink check, given an injected `clock.Clock` and random source) the window check and HH:MM parse moved from `config/system`, and the one validation function of the `apply` grouping's cross-leaf rules that every consumer's verify calls, with `decide_test.go`, `window_test.go` and `validate_test.go`. The NetBox builder (`plan/spec-netbox-0-umbrella.md`) uses this package and names no other location
- `internal/component/config/yang/modules/ze-apply-policy.yang` - grouping-only module holding the `apply` grouping (mode, delay, window; no shrink threshold), A-3
- `test/plugin/filter-irr-apply-operator.ci`, `test/plugin/filter-irr-apply-scheduled.ci`, `test/plugin/filter-irr-apply-restart.ci`
- `test/plugin/firewall-irr-apply-operator.ci`, `test/plugin/firewall-irr-apply-dismiss.ci`, `test/plugin/firewall-irr-apply-cross-consumer.ci`
- `test/parse/irr-apply-scheduled-needs-window.ci`, `test/parse/irr-apply-delay-maximum-below-minimum.ci`, `test/ui/completion-words-irr-held.ci`
- `test/plugin/filter-irr-apply-delay-range.ci`, `test/plugin/firewall-irr-apply-delay-fixed.ci`, `test/plugin/filter-irr-shrink-alert.ci`, `test/plugin/firewall-irr-apply-shrink-alert.ci`, `test/plugin/filter-irr-clear.ci`
- observer fixtures under the existing fixture tree for the `.ci` files that assert through the engine

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-apply-policy.yang` (grouping), `ze-filter-irr.yang`, `ze-firewall-irr.yang` (`apply` container via the shared grouping), `ze-system-conf.yang` (`system apply-policy shrink-threshold`), both `-cmd.yang` (held, apply, clear held nodes) |
| YANG validation constraints | Yes | `mode` enumeration; `start`/`end` use the same HH:MM `pattern` as the SMART `time` leaf in `ze-storage-conf.yang`; `delay` container with `minimum` uint16 `range 0..1440`, `units minutes`, default 0, and `maximum` uint16 `range 0..1440`, `units minutes`, no default (absent means a fixed delay of `minimum`); `system apply-policy shrink-threshold` uint8 `range 1..100`, `units percent`, no default (absent turns the check off) |
| YANG custom validators | Yes | one validation function in `internal/core/applypolicy`, called by each consumer's plugin verify: `scheduled` needs both window leaves, `start` != `end`; `delay maximum` not below `delay minimum` (AC-19c). All cross-leaf, not expressible natively; the NetBox set's `netbox source <s> apply` calls the same function |
| CLI commands/flags | Yes | `show <consumer> irr held [asn <n> \| as-set <name>]`, `update <consumer> irr apply all \| asn <n> \| as-set <name>`, `clear <consumer> irr held all \| asn <n> \| as-set <name>`, and the new `clear bgp irr asn <n> \| as-set <name>` |
| Report bus alert | Yes | `report.RaiseWarning`/`report.ClearWarning`, code `irr-shrink-held`, sources `bgp` and `firewall`; read by `show warnings` (A-8) |
| CLI grammar (keyword before value) | Yes | `apply` and `held` are action keywords before the typed selector `asn`/`as-set`; run `./le cli grammar` |
| Editor autocomplete | Yes | mode enum automatic; `asn`/`as-set` values reuse the existing completion of the `update ... irr` nodes |
| Functional test for new RPC/API | Yes | the `.ci` files listed above |
| Pipe completeness | Yes | held answers are `ResponseData` routed through `ApplyPipes` (`| json`, `| yaml`, `| table`) |
| Env var registration | N-A | neither container is under `environment/`; `refresh-interval` has no env var either |
| Doctor check for runtime dependencies | No | no new file path, socket, port or binary; the new zefs keys live in the existing managed store. The firewall doctor (`doctor.go`) reads `Get`; it must report against the enforced entry, covered by its existing test |
| Prometheus counters/metrics | Yes | BGP: `ze_irr_held_entries` (gauge), `ze_irr_held_age_seconds` (gauge, oldest held), `ze_irr_shrink_held_entries` (gauge, the alert monitoring scrapes), `ze_irr_apply_outcomes_total{result}` with result `applied`, `held`, `bootstrap`, `scheduled`, `delayed`, `shrink-held`, `committed`, `dismissed`, `refused`. Firewall: the same four with prefix `ze_firewall_irr_` |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/irr-bgp-import-filtering.md` (apply modes); a firewall IRR feature file if one exists at implementation time |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` IRR section, including the `apply delay { minimum; maximum; }` container and its fixed and range forms, and its system section for `system apply-policy shrink-threshold`; `docs/architecture/config/syntax.md` N-A (no syntax rule changes) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` if it lists the IRR wire methods; check at implementation |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` rows for `bgp-filter-irr` and `firewall-irr` if they describe refresh behavior |
| 6 | Has a user guide page? | Yes | `docs/guide/irr-filtering.md`: new "Choose when a fetched change takes effect" section, covering the three modes, the fixed and range delay, and the shrink threshold; it states that `system apply-policy shrink-threshold` is one system value, so setting it (for example for NetBox) holds large removals for both IRR consumers as well; refresh and troubleshooting rows rewritten per mode |
| 7 | Wire format changed? | N-A | no wire change |
| 8 | Plugin SDK/protocol changed? | N-A | no SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if it documents `le test irr` flags; the mock's usage text |
| 11 | Affects daemon comparison? | No | checked at implementation: `docs/comparison.md` IRR row, if any, gains the apply modes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` section 22 (replace "not implemented" sentence), `docs/architecture/resolve.md`, `docs/architecture/bgp/filter-irr.md`, `docs/architecture/firewall/firewall-irr.md` |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` or the IRR telemetry section that lists `ze_irr_*`; the report code `irr-shrink-held` in `docs/guide/operational-reports.md`, in the report-bus section of `docs/architecture/api/commands.md`, and in the code list of `docs/architecture/core-design.md` section 15 |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` and `docs/features/plugins.md` if they list IRR commands |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation by `./le spec citation anchors spec plan/spec-irr-apply-policy.md`; known now: `irr-filtering.md` and `core-design.md` anchor `store.go` and `filter_irr.go`; `docs/architecture/appliance/self-update.md` (declared by `selfupdate.go`) is updated if it describes where the maintenance-window check lives; `docs/architecture/testing/ci-format.md` (declared by the mock) is updated only if it lists `le test irr` flags, otherwise named here as unaffected; `docs/guide/firewall.md` (mentions `store.go`, `irr.go`) gains the firewall `apply` container and commands; `docs/architecture/cli/command-verbs.md` (mentions `command.go`) is checked for the `update`/`clear` verb examples and updated if it lists the IRR commands |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/irr-filtering.md` and `docs/guide/configuration.md` examples stay valid (default mode); new examples added |

Discovery (`ai/rules/repo-maintenance.md`): `ai/INDEX.md` has no IRR row today. Add one: keywords "IRR, AS-SET, prefix-list from IRR, held change, apply mode, operator approval" pointing at `docs/guide/irr-filtering.md`, `docs/architecture/bgp/filter-irr.md`, `docs/architecture/firewall/firewall-irr.md`. Regression is prevented by AC-1 (existing `.ci` unchanged) and the store unit tests; drift between consumers by the shared grouping and the single store implementation.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG `apply` grouping in `ze-apply-policy.yang` and both containers, `system apply-policy shrink-threshold` in `ze-system-conf.yang`, `system` in both plugins' `ConfigReads`, the new command nodes and `RegisterRPCs` entries in both plugins returning a stub "not implemented" error, config parsing of `mode`/`delay`/`window` and of the system threshold, mock `--change-after-first`
   - Tests: every Wiring Test row written and failing for the right reason; `TestParseApplyConfig`, `TestParseDelayRefusesMaximumBelowMinimum`
   - Files: `ze-apply-policy.yang`, `ze-system-conf.yang`, both YANG pairs, both `cmd_irr.go`, both `config.go`, both `register.go`, `internal/test/mock/irr/irr.go`
   - Verify: commands reachable and refused by the stub; config accepted; `./le cli grammar` clean
2. **Phase: Shared policy package** -- create `internal/core/applypolicy` (decision, window check, validation of the `apply` grouping), move `parseHHMM`/`inMaintenanceWindow` logic into it, self-update calls it, both consumers' verify call the validation
   - Tests: `TestWindowContains`, `TestDecideTable`, `TestValidateApplyConfig`, `TestSelfUpdateUsesSharedWindow`, existing self-update tests
   - Files: `internal/core/applypolicy/`, `selfupdate.go`, `selfupdate_validate.go`
3. **Phase: Store** -- fetched/applied split, decide, bootstrap, seed on mode entry, apply, apply-due, dismiss, diff, `Open`, `Purge`, rollback hook
   - Tests: every store row of the Unit Tests table
   - Files: `store.go`, `store_test.go`
4. **Phase: BGP consumer** -- mode and key into the store, `st.list` from the enforced entry, window branch in `refreshLoop`, commands, show fields, metrics
   - Tests: `TestRefreshASNKeepsListWhenHeld`, BGP command tests, `filter-irr-apply-*.ci`
5. **Phase: Firewall consumer** -- same, plus verify calling the shared validation function (AC-16, AC-19c) and refused-apply rollback (AC-14)
   - Tests: firewall command tests, `firewall-irr-apply-*.ci`
6. **Phase: Docs** -- every row of the Documentation Update Checklist, in the phase that changes the behavior a page describes (`ai/rules/documentation.md`), `ai/INDEX.md` row

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | An apply never copies anything the fetch guard did not accept; a held mode never writes the shared fetched key as "applied"; immediate mode writes no applied key |
| Correctness | Every path that mutates BGP `st.list` re-reads `byASN[asn]` under `plug.mu` |
| Naming | `held`, never `pending`, in keys, fields, commands, logs and docs; JSON keys kebab-case (`held-since`, `held-age-seconds`, `apply-mode`, `window-open`) |
| Data flow | consumers read only `Get` for enforcement; only the store decides held vs applied |
| Rule: no-layering | no copy of the window logic left in `config/system` |
| Rule: goroutine-lifecycle | no new goroutine; the window timer lives in the existing per-configure loop |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Three modes configurable in both consumers | `grep -n "enum immediate\|enum scheduled\|enum operator"` in `ze-apply-policy.yang`; `TestParseApplyConfig` |
| Held, apply, dismiss commands in both consumers | `.ci` files listed, run through `./le job run` |
| Immediate mode unchanged | every pre-existing `filter-irr*.ci` and `firewall-irr*.ci` green |
| Section 22 sentence replaced | `grep -n "not implemented" docs/architecture/core-design.md` returns nothing for IRR holding |
| No window copy | `grep -rn "inMaintenanceWindow\|parseHHMM" internal/` returns no definition outside `internal/core/applypolicy`; no consumer's `config.go` restates the `apply` cross-leaf rules |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `asn`/`as-set` selectors go through the existing `ValidateASSetName` and ASN range checks before reaching a zefs key; `..` and `.` refused (`validateName`) |
| Authorization | `update ... irr apply` and `clear ... irr held` change enforcement, so they sit under the same authz class as `update ... irr` and `clear firewall irr`; a read-only user can run `show ... held` only |
| Fail-open | a held-mode decision error (snapshot unreadable) must keep enforcing the last applied list, never fall back to the fetched list; logged and counted `refused` |
| Resource exhaustion | the diff is computed per request for one name or streamed per entry for `all`, bounded by the 500000 prefix bound already enforced per family |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The shared persisted list is both today's fetched list and today's applied list, because in `immediate` mode they are the same thing. Separating them is only needed for a consumer that holds, which is why `immediate` can stay byte-identical.
- Holding is a property of the consumer, not of the data. Two consumers enforcing the same AS-SET can rightly disagree on when to take a change (a BGP import filter is reversible in seconds; an interface whitelist can black-hole a port).

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Policy per consumer (one `apply` container in `bgp policy irr` and one in `firewall irr`, from the grouping in `ze-apply-policy.yang`), decision once in `internal/core/applypolicy`, IRR mechanism once in the shared store; the shrink threshold is the one system leaf (owner, 2026-10-08) | (a) one global policy on the shared store; (b) policy and mechanism both per consumer | (a) needs a config home neither plugin reads and forces one answer on two consumers with different risk (the firewall already defaults auto-refresh off for that reason); (b) duplicates the held/apply/diff logic. Per-consumer policy over one mechanism follows section 22: the builder is shared, each consumer pulls when ready |
| Fetched list shared (`meta/irr/{name}`, unchanged), applied snapshot per consumer (`meta/irr-applied/<consumer>/{name}`), only in held modes | per-consumer copies of everything; a held key beside the shared applied key | keeps today's sharing (one fetch serves both) and today's key meaning; a held key beside one shared applied list cannot express two consumers in different modes (R-1) |
| Held change = fetched differs from applied (computed), not a stored third list | store a pending list per consumer | nothing to keep in sync; restart and mode changes reduce to "is there a snapshot" |
| Scheduled shape: daily window `start`/`end` HH:MM local time, applies at window start and on any fetch completing inside it | a single daily `apply-time`; a cron expression; an `apply-interval` separate from `refresh-interval` | matches the existing `system ... maintenance-window` (fetch anytime, replace in window) and its operator vocabulary; a window does not miss a day when the router is busy or down at one instant; cron is power nobody asked for (`ai/rules/simplicity.md`); an interval says how often, not when, which is the question "time-based" asks |
| Bootstrap: with no snapshot and no prior fetched entry, the first fetch applies in every mode | hold it and fail closed | there is no enforced list to protect; holding leaves BGP rejecting everything and firewall verify refusing the reference, which no operator wants on first enrollment; owner question Q-1 |
| Commands: `update <c> irr apply ...`, `clear <c> irr held ...`, `show <c> irr held ...` | a new `commit` verb; `request <c> irr apply` | `commit` is not in the verb registry and means config commit in Ze; `update` already owns "change IRR enforcement" in this namespace; `clear` already owns deliberate removal; owner question Q-2 |
| Immediate-mode delay: `delay { minimum; maximum; }` in minutes; fixed (exactly `minimum`) when `maximum` is absent or equal to `minimum`, uniform random in [`minimum`, `maximum`] otherwise, drawn once per hold; verify refuses `maximum` below `minimum` | (a) one leaf for the upper bound, random from 0 only; (b) one string leaf "N" or "N-M" | Thomas: "the waiting time could be one value (fixed) or range (delay)". A fixed delay gives every change a known wait before it takes effect; a range spreads a fleet so its routers do not move at the same instant. (a) was the first reading and cannot express a fixed wait, so this amendment replaces it; (b) is parsed text with no native YANG range check, and Ze has no existing YANG range idiom to reuse, while two uint16 leaves get `range 0..1440` from YANG and leave only the cross-leaf order to verify |
| Newer fetch during a delay: the newest list wins, the due time is not restarted | restart the timer on each fetch; keep the first list | restarting can postpone forever while the IRR keeps changing (R-12); keeping the first list applies data already known to be outdated |
| Delayed change is a held change (`held-reason: delay`, `apply-due`) | a separate "scheduled to apply" state | one concept, one view: `show ... held` answers "what will change and when" for every reason |
| Shrink threshold held in every mode, gating automatic applies only, alert on the report bus as a warning plus a Prometheus gauge | an error event; a new alert channel; a threshold only in `immediate` | the condition is a state that resolves, which the bus's severity contract calls a warning; section 15 is the existing operator-visible channel and Prometheus is what monitoring scrapes; a scheduled window can apply a 90% shrink as easily as immediate can. The deliberate operator apply is not gated, because that is the review the alert asks for |
| Shrink alert clears when its hold ends (apply, dismiss, clear, or a later fetch no longer over the threshold) | clear only on apply | a dismissed or superseded change is no longer a pending risk, and an alert nobody can act on trains people to ignore it |
| `clear bgp irr asn|as-set` with the firewall's semantics; no `all` | add `all` to both | the firewall has no `clear ... all` and Thomas asked for symmetry, not a new verb form |
| Dismiss by digest of the dismissed fetched list | dismiss until next fetch; no dismiss at all | a same-answer fetch must not re-raise it, a different answer must |
| Name `held` | `pending` (owner's word) | `status: pending` already means unresolved in `show bgp irr` |

## Known Limitations

- BGP oversized truncation is a separate defect (journal row 2026-10-08), not fixed here.
- A fetch made by one consumer for a name the other consumer also enrolls is visible to both, as today; with different `server` leaves the last fetch wins (A-1).

## Owner Decisions (2026-10-08)

| ID | Question | Decision |
|----|----------|----------|
| Q-1 | First enrollment in `scheduled` or `operator` mode: apply the first fetch at once or hold it? | Apply at once (bootstrap), as recommended |
| Q-2 | Command spelling `update <c> irr apply ...`, `clear <c> irr held ...` | As proposed |
| Q-3 | Window clock | Appliance local time, as `maintenance-window` |
| Q-4 | Shrink threshold | Adopted: "it should raise an alert which can be caught via monitoring". Held in every mode for automatic applies, report-bus warning plus Prometheus gauge, off by default (AC-21, AC-22) |
| Q-5 | A holding consumer accepts the other consumer's fetches | Yes, sharing kept |
| Q-6 | Held mode back to `immediate` | Applies every held change at commit and logs each one |
| Q-7 | `clear bgp irr asn|as-set` | "yes please": added with the firewall's semantics (AC-24) |
| Q-8 | "immediate may mean between 0 and X minutes after successful fetch", amended the same day: "the waiting time could be one value (fixed) or range (delay)" | `delay { minimum; maximum; }` in minutes, 0..1440; no container means no delay (default); `maximum` absent or equal to `minimum` is a fixed delay of `minimum`; `maximum` above `minimum` is a uniform random delay between the two ("0 to X" is `minimum 0; maximum X`); verify refuses `maximum` below `minimum`. This replaces the first reading, a single upper-bound leaf with a random spread from 0. Newest list wins during the wait, the due time is not restarted; the waiting change shows in `show <c> irr held` (AC-19, AC-19b, AC-19c, AC-20, AC-23, AC-25) |

## RFC Documentation (Scope: protocol)

N-A: Scope is plugin; no protocol code changes.

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
- [ ] AC-1..AC-25, AC-19b and AC-19c all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no wire change)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-irr-apply-policy.md` only, in the same `./le commit create` script
