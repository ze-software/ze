# Spec: firewall-domain-group

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Depends | `plan/spec-firewall-remote-group.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-02 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's firewall matches packets against nftables named sets whose members are
written literally in config (`list set` in `ze-firewall-conf.yang`, lowered by
`lowerSet` in `lower_linux.go`). Nothing in the firewall tree resolves a name.
`internal/component/firewall` and `internal/plugins/firewall` import
`resolve/irr`, `resolve/irr/store` and `resolve/peeringdb`, and never
`resolve/dns`, so an operator who wants to permit or deny a service that
publishes its addresses in DNS must transcribe those addresses into config and
re-transcribe them each time they move.

Goal: add a firewall group whose members are the addresses a DNS name resolves
to, re-resolved when the answer's TTL expires, with the resulting set
reprogrammed only when the addresses actually changed. Every change is recorded
so an operator can later answer what a name pointed at, and when it moved.

This spec owns four things:

1. **The group and its TTL schedule.** A named group holds one or more DNS
   names. Each name is re-resolved when its own answer expires rather than on a
   shared interval, because a TTL is a per-answer property.
   `Resolver.ResolveWithTTL` (`resolver.go`) already returns records plus the
   minimum answer TTL, and the remaining TTL on a cache hit, so the schedule
   reads its input from the resolver rather than computing it.
2. **Showing the name beside the address.** `SetElement` (`model.go`) carries
   `Value`, `Timeout` and `IntervalEnd` and has nowhere to record where an
   address came from. An operator reading a ruleset must be able to see both the
   address in the kernel and the name that put it there.

   There are TWO rendering surfaces and they must not be confused. The
   operator-facing one is `handleShowFirewallRuleset`
   (`internal/plugins/firewall/nft/cmd_show.go`), an in-hub RPC handler whose
   `Data` map today carries `table`, `family` and `chains` and NO set elements
   at all, so an address has nowhere to appear before provenance can be attached
   to it. That is the target. The `formatSet` and `formatInSet` text formatters
   (`internal/component/firewall/cmd/show.go`) serve only the offline
   `ze firewall show` debug binary (`internal/component/firewall/cli/main.go`),
   which reads `Backend.ListTables` directly. They are out of scope unless that
   binary is explicitly brought in.
3. **A DNS change record, split across two stores.** Every change in what a name
   resolves to is recorded: what the name resolved to before, what it resolves
   to now, and when it moved. The two halves go to different stores, because
   they have different shapes (owner, 2026-09-02, after research):

   - **Last-good resolved addresses go to zefs**, under a registered key. This
     is small, bounded, per-group state, and it is what makes the cold-start
     decision below work across a restart. zefs writes are durable before they
     return (`flushFull` calls `tmp.Sync`, `os.Rename`, then `dirFd.Sync`).
   - **The change log goes to raw JSON-lines OUTSIDE zefs**, at
     `<config-dir>/firewall-domain-group.dns.jsonl`, beside the operator audit
     log that `openAuditLog` (`cmd/ze/hub/audit.go`) builds as
     `<config-dir>/<name>.audit.jsonl`. It is written by the plugin process, is
     bounded by an entry count, and evicts oldest-first. zefs has no
     append: `BlobStore.WriteFile` replaces a whole value, and a value that
     outgrows its 10% capacity headroom forces a full-store rewrite via temp and
     rename. `docs/architecture/zefs-format.md` already names
     `internal/core/audit` as a raw-filesystem exception to "state goes through
     zefs", so Ze's one true append-only log was deliberately kept out of it.
     This log follows that same exception.

   This is deliberately NOT the operator audit log itself
   (`internal/core/audit`, `<config>.audit.jsonl`). That log records operator
   actions and is bounded; a name whose addresses rotate is neither an operator
   action nor bounded by operator behavior, and would evict commit history.
   It is also not `AuditTables` (`internal/component/firewall/audit.go`), which
   is a kernel-versus-config drift check and records no events at all.

4. **A resolver change to distinguish a deleted name from a broken server.**
   `query` (`resolver.go`) discards `resp.Rcode` and returns
   `(nil, 0, nil)` for every non-success rcode, so NXDOMAIN, SERVFAIL and
   REFUSED are indistinguishable to a caller, and in DNSSEC-permissive mode a
   broken chain joins them. A firewall cannot act on that: the same signal would
   have to mean both "empty the set" and "keep the last good answer". This spec
   surfaces the rcode so the two are separable (owner, 2026-09-02). Every other
   resolver caller benefits, and none is required to change.

Placement is decided (owner, 2026-09-02, after research): a PLUGIN under
`internal/component/firewall/plugins/`, following `firewall-irr`. That directory
is already a plugin discovery path scanned by `pluginimports.go`, and every
firewall feature that owns a timer today is a separate process, so the
registration machinery (`registry.Register` with `RunEngine`, YANG glue,
`pluginserver.RegisterRPCs`, `firewall.RegisterTables`, a doctor check) is
proven rather than invented.

A plugin does NOT fork the DNS cache. `pkg/plugin/sdk/sdk_engine.go` is an
established outbound RPC surface from plugin to hub (`RouteInstall`,
`BatchValidate`, `DispatchCommand` and others), so this spec adds ONE resolve
call in that same shape and the plugin uses the hub's single resolver and its
cache. A second `Resolver` instance in the plugin process is explicitly
rejected: it would double the query load and split the TTL view.

Naming follows `ai/patterns/config-option.md`, whose allowed-abbreviation list
is closed and contains neither `fqdn` nor `dns`. The config surface is therefore
`domain-group` holding a `domain-names` leaf-list, and the feature is named for
that.

Related, and deliberately out of scope:

- `plan/spec-firewall-remote-group.md` covers URL-sourced groups. It owns the
  substrate this spec depends on: the disk cache, keeping the last good answer
  when a source fails, programming the set, and the show/update/clear command
  shape. This spec adds DNS as a source and TTL as a schedule, and MUST NOT
  build a second copy of that substrate.
- `plan/immediate/spec-firewall-dynamic-address-group.md` covers packet-triggered set
  population through nftables `dynset`. It owns finishing the inert
  `flags-dynamic` and `flags-timeout` lowering in `applySet`, which this spec
  needs if group members are to carry a kernel-side timeout.

Provenance: this is the third mechanism `plan/spec-firewall-remote-group.md`
names and excludes, and the destination for the unhomed deferral row in
the retired deferral shard "firewall-remote-group".

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/patterns/config-option.md` - the group is a YANG config surface.
  → Constraint: a `leaf-list` is named in the plural, so the names a group holds
    are `names`, not `name`.
  → Constraint: `ttl` is on the short list of industry-standard abbreviations a
    leaf name may use, so a TTL leaf does not have to spell the word out.
  → Constraint: a dimensioned leaf states its unit through `units`, and the leaf
    name stays unit-free: `ttl-floor` with `units seconds;`, never
    `ttl-floor-seconds`.
  → Decision: this is YANG config, not an env var. It survives the decision
    table on the first question: an operator changes which names a firewall
    group holds during normal operation, and it must appear in
    `show configuration` and in a config backup.
- [ ] `ai/rules/planning.md` - deferral homing.
  → Constraint: the deferral row that names this spec stays live until the work
    LANDS, not until the destination exists, so closure must resolve the row in
    the retired deferral shard "firewall-remote-group", not only this spec's own shard.
- [ ] `ai/rules/writing.md` - spec prose.
  → Constraint: cite `file.go` `Symbol`, not a line number, because a line
    number rots at the next edit.

### Architecture Docs still owed (Feature Surface Gate NOT discharged)
- [ ] `ai/rules/architecture.md`, `docs/architecture/core-design.md` - required
  before the Data Flow section can be called complete.
- [ ] `ai/rules/config.md` - YANG structure and naming obligations.
- [ ] `ai/patterns/cli-command.md` - the show/update/clear commands this adds.
- [ ] `ai/patterns/registration.md` - registration over hardcoding.
- [ ] `docs/architecture/resolve.md` - the resolver this spec consumes.

### RFC Summaries (Scope: protocol)

N-A. Scope is `config`. This spec consumes an existing DNS resolver and adds no
wire-visible behavior of its own. The resolver's own RFC conformance is
`plan/spec-fixit-dns-rfc1035-conformance.md`.

**Key insights:** (minimal context to resume after compaction)
- The resolver already returns what the schedule needs. `Resolver.ResolveWithTTL`
  (`resolver.go`) returns records plus the minimum TTL across answers, and the
  remaining TTL on a cache hit. No new resolver capability is required.
- `extractRecords` (`resolver.go`) applies its 300s default ONLY when there were
  no answers at all. A server that answers with TTL=0 is explicitly saying "do
  not cache", and the value reaches the caller as 0. The schedule must decide
  what 0 means; it cannot treat it as "expired now" without spinning.
- A firewall table whose rule names a set no owner has registered is HELD BACK,
  not failed. `dropTablesMissingAProvidedSet` (`registry.go`) drops that one
  table, warns, and programs every other table. Its own comment states the
  policy: "an unfiltered port beats a blackholed one". For a domain group this
  means a name that has never resolved silently removes the filtering that
  depends on it.
- Ze already has two audit surfaces and they are different things.
  `AuditTables` (`internal/component/firewall/audit.go`) is a drift check that
  compares kernel state to config and raises warnings. `audit.Log`
  (`internal/core/audit`) is the append-only operator-action record written to
  `<config>.audit.jsonl`. A DNS change belongs to the second, not the first.
- `firewall-irr` is the working reference for every piece of this except the
  resolution itself: a refresh loop that survives every fetch error
  (`refreshAllNow`, `irr.go`), a disk cache that outlives a restart, and the
  cold-cache reasoning in `refreshName`'s comment.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - `list set` keyed
  by name, carrying `type`, four presence containers (`flags-interval`,
  `flags-timeout`, `flags-constant`, `flags-dynamic`) and a `list element` of
  static `value` plus optional `timeout`. Every member is written literally.
  There is no source, no URL and no name anywhere in the set.
- [ ] `internal/component/firewall/model.go` - `SetElement` carries exactly
  `Value`, `Timeout` and `IntervalEnd`. There is no field in which to record
  where an address came from, which is the gap requirement 2 has to close.
- [ ] `internal/component/firewall/cmd/show.go` - `formatSet` renders each
  member as a bare `element <value>;`, adding `{ timeout N; }` only when the
  element carries one. `formatInSet` renders a rule's set match as
  `source address @<name>`. Neither has access to anything but the set itself.
- [ ] `internal/component/firewall/registry.go` - `RegisterTables(owner, tables)`
  stores a desired snapshot per owner and `ApplyAll` merges every owner's tables
  and reconciles them. `dropTablesMissingAProvidedSet` holds back a table whose
  term names a set (`MatchInSet.ProvidedType`) no owner has registered, warns,
  and programs the rest. `StoreLastApplied` records the merged result, which is
  what `show firewall ruleset` reads.
- [ ] `internal/component/firewall/audit.go` - `AuditTables` compares
  `LastApplied` to `Backend.ListTables` and raises `firewall-stale-table` and
  `firewall-drift` warnings. Read-only, and unrelated to recording an event.
- [ ] `internal/core/audit/audit.go` - `Entry` carries Timestamp, Actor,
  RemoteAddr, Surface, Action, Detail and Outcome. `Action` is a fixed constant
  list (`config-commit`, `auth-fail`, and so on) and `Surface` includes `System`.
  `Log.Record` appends. `MaxMaxEntries` is 100000 and the log is bounded.
- [ ] `cmd/ze/hub/audit.go` - `openAuditLog` builds the path as
  `<config-dir>/<name>.audit.jsonl`, so the DNS record would land in the same
  file as operator actions.
- [ ] `cmd/ze/hub/main_system.go` - `newResolvers` builds ONE `resolve.Resolvers`
  at hub startup, holding the DNS resolver, Cymru, PeeringDB and IRR. This is
  the instance the owner directed this feature to use.
- [ ] `internal/component/resolve/dns/resolver.go` - `ResolveWithTTL`,
  `ResolveA`, `ResolveAAAA`, `CacheEntries`, and `extractRecords` with its
  TTL=0 handling described in Key Insights.
- [ ] `internal/component/firewall/plugins/irr/irr.go` - `refreshLoop` returns
  only on its stop channels; `refreshAllNow` logs every fetch failure, keeps the
  cached data and still applies. This is the error posture this spec copies.

**Behavior to preserve:**
- A `set` with statically written elements behaves exactly as it does today, and
  its `show firewall ruleset` output is unchanged.
- `dropTablesMissingAProvidedSet` keeps its current policy for every existing
  supplier. This spec must not change what an absent IRR set does.
- The audit log stays readable by whatever consumes `<config>.audit.jsonl`
  today: a new Action value must not change the shape of `Entry`.

**Behavior to change:**
- A firewall group may name DNS names instead of literal addresses.
- Each name is re-resolved on its own answer TTL, and the set is reprogrammed
  only when the resolved address set actually changed.
- `show firewall ruleset` shows the name that supplied each address.
- A change in what a name resolves to is recorded for audit.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Two, and they are different in kind:

- **Config.** An operator writes a domain group with a `domain-names` leaf-list.
  Format at entry is the YANG-parsed `map[string]any` the plugin receives in
  `OnConfigVerify` and `OnConfigure`, the same shape `parseIRRConfig` consumes.
- **A TTL expiry inside the plugin.** No external data arrives; the timer fires
  and the plugin asks for a name again. Format at entry is the plugin's own
  per-name schedule state.

### Transformation Path

1. Config parse. YANG validates each name against a `pattern` and `length`; the
   plugin decodes the group into its own config type.
2. Verify. For each name the plugin reads the zefs cache. A name with no cached
   answer refuses the commit, mirroring `verifyRefs` (`irr.go`). Verify performs
   NO network I/O: `ValidateFn` runs synchronously at commit, so resolving there
   would block a commit on DNS.
3. Resolution. The plugin calls the new SDK resolve RPC; the hub answers from
   `Resolver.ResolveWithTTL` per name and per family, returning records, TTL and
   rcode.
4. Classification. `err != nil` is transient, keep last good. `NXDOMAIN` is
   authoritative absence, empty the set. `SERVFAIL` and `REFUSED` are transient,
   keep last good. This step is why the resolver change is in scope.
5. Change detection. Compare the resolved address set to the last-good set. If
   equal, stop here: no write, no log record, no reprogram.
6. Persist. Write the new last-good set to zefs, then append one record to the
   JSON-lines change log.
7. Program. Build `[]firewall.Set`, call `firewall.RegisterTables("firewall-domain", tables)`
   then `firewall.ApplyAll()`.
8. Schedule. Arm the next refresh for this name and family at its TTL, clamped
   by the configured floor.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Hub (resolution) | New SDK call in `sdk_engine.go` shape, hub handler calling `Resolver.ResolveWithTTL` | No |
| Plugin → Firewall registry | `firewall.RegisterTables` + `ApplyAll`, owner `"firewall-domain"` | No |
| Plugin → zefs | Registered key holding the last-good addresses per group | No |
| Plugin → change log | JSON-lines file outside zefs, following the `internal/core/audit` exception | No |
| Hub → Plugin (show) | `enrich-show` carrying address-to-name provenance into `show firewall ruleset` | No |
| Operator → Plugin (commands) | `pluginserver.RegisterRPCs` plus `ForwardToPlugin`, as `cmd_irr.go` does | No |

### Integration Points
- `firewall.RegisterTables` / `ApplyAll` (`registry.go`) - programs the set. A
  rule naming a set this plugin has not registered holds its whole table back
  (`dropTablesMissingAProvidedSet`), which is what AC-6 and AC-7 exist to prevent.
- `show.Enrich` (`internal/core/show/show.go`) - the call site the firewall show
  handlers do not have yet, added by this spec.
- `Resolver.ResolveWithTTL` (`resolver.go`) - changed to surface the rcode.
- `ProcessManager.Respawn` (`internal/component/plugin/process/manager.go`) -
  restarts the plugin after a crash, with the zefs cache intact.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The remaining TTL `ResolveWithTTL` returns on a cache hit is accurate enough to schedule the next refresh on | `resolver.go` `ResolveWithTTL` doc comment, which states remaining TTL on hit and response TTL on miss | The schedule fires early (wasted queries) or late (the set serves stale addresses past their TTL) | Unit test driving a stub resolver across a hit and a miss, plus reading `cache.getWithTTL` | unvalidated |
| A-2 | A plugin can reach the hub's single resolver over a new SDK RPC, so no second `Resolver` instance and no second DNS cache is created | `pkg/plugin/sdk/sdk_engine.go` already carries plugin-to-hub calls of exactly this shape (`RouteInstall`, `RouteRemove`, `BatchValidate`, `DispatchCommand`), each paired with a hub-side handler. No resolve call exists yet, so one is added | The plugin falls back to its own `Resolver`, doubling query load against upstream servers and splitting the TTL view between two caches, which was the whole argument against a plugin | Wiring test proving a name configured in a group is resolved by the hub's resolver instance, plus a cache-hit assertion showing the plugin's lookup populates the same cache `show dns` reads | unvalidated |
| A-3 | zefs can hold a bounded, growing DNS change log at acceptable cost | BROKEN 2026-09-02 by the zefs research. zefs has NO append: `BlobStore.WriteFile` replaces the whole value, and a value outgrowing its 10% capacity headroom forces a full-store rewrite via temp+rename. There is no ring-buffer primitive anywhere in `pkg/zefs`. `docs/architecture/zefs-format.md` names `internal/core/audit` as an explicit raw-filesystem EXCEPTION to "state goes through zefs", so the one true append-only log in Ze was deliberately kept OUT of zefs | A bounded log is still possible, trimmed in memory before each write like `History.Save` (`internal/component/cli/history.go`), but every recorded change rewrites the blob, and a per-name key spends from the store-wide `maxEntryCount` of 100000 entries | Owner decision at the DESIGN gate, then a write-cost test at the chosen bound | broken |
| A-4 | Address provenance can be carried without adding a field to the shared `SetElement` model | CONFIRMED 2026-09-02 by the registration research. `ai/rules/principles.md`: "A new feature MUST register itself and be discovered; it MUST NOT require an edit to a switch, a case, a factory, a field list, or any other central enumeration." A provenance field on `SetElement` is that banned field-list edit, carried unused by every other owner (copp, policy-routes, flowspec, vrrp, firewall-irr). The sanctioned route is the show enricher registry, `show.MustRegister` / `show.Enrich` (`internal/core/show/show.go`) | The shared model grows a per-feature field in violation of the rule | Show-output functional test, plus a grep proving `SetElement` is unchanged | confirmed |
| A-6 | The resolver can tell a name that does not exist from a server that failed | BROKEN as found 2026-09-02, then TAKEN INTO SCOPE. `query` (`resolver.go`) returns `(nil, 0, nil)` for EVERY non-success rcode: NXDOMAIN, SERVFAIL and REFUSED are indistinguishable, and in DNSSEC-permissive mode a broken chain joins them. `resp.Rcode` is discarded. The owner decided 2026-09-02 that this spec changes the resolver to surface it | Without the change the set cannot be emptied safely: "no records" would mean both "this name legitimately has no addresses" and "the server refused", so either a failed server silently empties a firewall set, or a deleted name keeps enforcing stale addresses forever | Unit test per rcode (NXDOMAIN, SERVFAIL, REFUSED, NOERROR-empty) asserting the caller can separate them, plus a test that existing callers are unaffected | unvalidated |
| A-7 | This feature can be built with existing plugin registration machinery, inventing none | RESOLVED 2026-09-02: the owner chose the plugin placement precisely because that machinery is proven. `registry.Register` with `RunEngine`, YANG glue via `./le yang glue write`, `pluginserver.RegisterRPCs`, `firewall.RegisterTables`, and `diagnostic.RegisterDoctorCheck` are each demonstrated by `firewall-irr`. `pluginimports.go` already scans the directory, so `./le repository generate` discovers the package with no manual codegen step | Some part of the feature needs a mechanism `firewall-irr` does not demonstrate, and that part needs its own justification | Building the wiring phase first, per the Implementation Steps, and confirming each registration point has a live call | confirmed |
| A-8 | The show enricher can carry per-address provenance from a plugin into hub-rendered output | `OnEnrichShow` (`sdk_callbacks.go`) is delivered by `PluginConn.SendEnrichShow` (`internal/component/plugin/ipc/rpc.go`) from `internal/component/plugin/server/enricher.go`, and `show.Enrich` has production consumers in `handler_l2tp.go` and `l2tp/subscriber/cmd/subscriber.go` | Requirement 2 has no route that does not edit the shared `SetElement`, which `ai/rules/principles.md` forbids, so it would need its own design | A functional test asserting `show firewall ruleset` prints the name beside the address, with the enricher supplying it | unvalidated |
| A-5 | `spec-firewall-remote-group` will adopt whatever config shape this spec settles for a sourced group | That spec is `skeleton` and has chosen no shape; this spec would decide first | Two sourced-group mechanisms ship with divergent config syntax, which is the duplication the SCOPE gate set out to avoid | Owner confirmation, or writing the shared shape into the remote-group spec before this one implements | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Cold start with no cached answer holds back every table whose rules name a domain group, so the traffic those tables filter is not filtered | The WARN in `dropTablesMissingAProvidedSet`: "table held back, a rule names a set no owner has registered yet" | DECIDED (owner, 2026-09-02): persist resolved addresses in zefs so a restart keeps filtering, AND refuse the commit at verify when a group has never resolved, the way `verifyRefs` (`irr.go`) refuses an uncached IRR reference. The operator learns at commit time rather than from a WARN after traffic already passed |
| R-2 | A name answering with a very low TTL, or TTL=0, drives a refresh spin and continuous set churn | Refresh outcome counter rate; nftables set replacement rate | A `ttl-floor` leaf with `units seconds;` clamping the schedule, and an explicit rule for what TTL=0 means rather than treating it as "expired now" |
| R-3 | A name whose addresses rotate constantly grows the change log without bound, filling the disk | Change-log file size and growth rate | RESOLVED in shape by the owner decision of 2026-09-02: the log is JSON-lines outside zefs, not a zefs blob, so an append does not rewrite a growing value. Still record ONLY an actual change in the resolved address set, never a refresh that confirmed the same answer, and give the log a size bound and rotation. The `internal/core/audit` exception in `docs/architecture/zefs-format.md` is the precedent being followed |
| R-4 | The config shape decided here front-runs `spec-firewall-remote-group`, which has chosen none | The two specs describe different syntax for the same idea of a sourced group | Settle the shared sourced-group shape with the remote-group spec before this spec implements, per A-5 |
| R-5 | DNS is attacker-influenced input, so a hostile or hijacked answer can insert addresses into a group a permit rule trusts | None at runtime; the firewall does exactly what the config asked | Honor the hub's DNSSEC validation setting, cap the number of addresses accepted per name, and treat a name in a permit-list group as a deliberate operator decision documented as such |
| R-6 | A panic under resolution or answer handling kills the refresh loop | The plugin process exiting, and `ze_plugin_restarts_total` incrementing for this plugin | REDUCED by the plugin placement: the blast radius is one process, and `ProcessManager.Respawn` (`internal/component/plugin/process/manager.go`) restarts it with the zefs cache intact. Not eliminated: respawn is limit-bounded, so a DETERMINISTIC panic on a hostile answer crash-loops into the limit and leaves the plugin down, at which point every table naming a domain-group set is held back. Recover inside the loop so a bad answer costs one cycle, not the process. `ze-go-style.md` states "A peer MUST NOT be able to panic the daemon", and an upstream DNS answer is the same class of untrusted input |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / routes mis-encoded / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | [other plugins, components, or specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A committed config carrying a domain group | → | the plugin's `OnConfigure` handler, then `firewall.RegisterTables` | `TestDomainGroupConfigureRegistersSet` |
| A name's answer TTL expiring | → | the plugin's refresh loop, then the resolve RPC | `TestDomainGroupRefreshFiresAtTTL` |
| The new SDK resolve call from the plugin | → | the hub handler calling `Resolver.ResolveWithTTL` | `TestPluginResolveRPCReachesHubResolver` |
| `update firewall domain-group <name>` typed by an operator | → | the forwarded plugin command handler | `test/plugin/firewall-domain-group-update.ci` |
| `show firewall ruleset` typed by an operator | → | `show.Enrich` call site plus the plugin's `OnEnrichShow` | `test/firewall/firewall-cli-domain-group-show.ci` |
| A daemon restart with a populated cache | → | the plugin's zefs cache load on `OnConfigure` | `TestDomainGroupColdStartProgramsFromCache` |
| A commit naming a never-resolved group | → | the plugin's `OnConfigVerify` handler | `TestDomainGroupVerifyRefusesUncachedName` |
| A DNS answer carrying a non-success rcode | → | the changed `query` in `resolver.go` | `TestResolveWithTTLSurfacesRcode` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A domain group naming a resolvable name is committed, after the name has been fetched once | The nftables set for that group holds exactly the addresses the name resolves to, and a rule matching `@<group>` filters on them |
| AC-2 | A name's answer TTL expires and the re-resolution returns the SAME addresses | No set is reprogrammed, no zefs write occurs, and no change-log record is written |
| AC-3 | A name's answer TTL expires and the re-resolution returns DIFFERENT addresses | The set is reprogrammed to the new addresses, the zefs last-good value is replaced, and exactly ONE record is appended to the change log naming the old set, the new set and the time |
| AC-4 | Re-resolution returns a transport error (`err != nil`) | The set keeps the last-good addresses, the change log gets no record, and the failure is logged at WARN |
| AC-5 | Re-resolution returns NXDOMAIN | The set is emptied and a change-log record is written. SERVFAIL and REFUSED do NOT empty it: each keeps the last-good addresses, as in AC-4 |
| AC-6 | A commit names a domain group whose names have never been resolved | The commit is REFUSED at verify, naming the group and the command that would fetch it. No table is programmed and no table is silently held back |
| AC-7 | The daemon restarts with a populated zefs cache and DNS unreachable | Every domain-group set is programmed from the cached addresses before any resolution is attempted, so filtering resumes without waiting for DNS |
| AC-8 | `show firewall ruleset` is run on a ruleset using a domain group | Each address in the group's set is displayed with the DNS name that supplied it |
| AC-9 | A name answers with a TTL below the configured floor, or with TTL=0 | The next refresh is scheduled at the floor, never sooner. TTL=0 is treated as "do not cache", not as "expired now", so it does not spin |
| AC-10 | A name has both A and AAAA records with different TTLs | Each family is refreshed on its own TTL, and a change in one family does not reschedule or reprogram the other |
| AC-11 | The plugin process is killed while a refresh is in flight | It is respawned, programs the sets from the zefs cache, and loses no previously recorded change-log record |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Fetches a name, then commits a rule permitting it | `update firewall domain-group` -> resolve RPC -> hub resolver -> zefs cache -> commit -> verify passes -> `RegisterTables` -> `ApplyAll` -> nft | `test/plugin/firewall-domain-group-update.ci` |
| 2 | Commits a rule naming a group never fetched | commit -> `OnConfigVerify` -> zefs cache miss -> commit refused with the fetch command named | `TestDomainGroupVerifyRefusesUncachedName` |
| 3 | Reads the ruleset and sees which name supplied each address | `show firewall ruleset` -> `show.Enrich` -> `enrich-show` to the plugin -> provenance map -> rendered output | `test/firewall/firewall-cli-domain-group-show.ci` |
| 4 | Asks what a name pointed at last week | reads the JSON-lines change log | `TestDomainGroupChangeLogRecordsOldAndNew` |
| 5 | Reboots the box while its upstream DNS is down | restart -> `OnConfigure` -> zefs cache load -> `RegisterTables` -> filtering resumes | `TestDomainGroupColdStartProgramsFromCache` |
| 6 | Removes a group and expects its set gone | `clear firewall domain-group` -> withdraw from registry -> `ApplyAll` -> set removed | `test/plugin/firewall-domain-group-clear.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveWithTTLSurfacesRcode` | `internal/component/resolve/dns/resolver_test.go` | Wiring row 8; AC-5, that the rcode reaches the caller distinctly from `err` | |
| `TestQueryExistingCallersUnaffected` | `internal/component/resolve/dns/resolver_test.go` | `Resolve`, `ResolveA` and `ResolveAAAA` keep their signatures and behavior | |
| `TestDomainGroupConfigureRegistersSet` | `internal/component/firewall/plugins/domain/domain_test.go` | Wiring row 1; AC-1 | |
| `TestBuildDomainGroupTablesMatchesResolvedAddresses` | `internal/component/firewall/plugins/domain/sets_test.go` | AC-1, that the set holds exactly the resolved addresses | |
| `TestDomainGroupRefreshFiresAtTTL` | `internal/component/firewall/plugins/domain/schedule_test.go` | Wiring row 2 | |
| `TestRefreshSameAddressesSkipsReprogramAndLog` | `internal/component/firewall/plugins/domain/domain_test.go` | AC-2 | |
| `TestRefreshChangedAddressesReprogramsAndLogsOnce` | `internal/component/firewall/plugins/domain/domain_test.go` | AC-3 | |
| `TestDomainGroupChangeLogRecordsOldAndNew` | `internal/component/firewall/plugins/domain/changelog_test.go` | User story 4; AC-3 | |
| `TestRefreshTransportErrorKeepsLastGood` | `internal/component/firewall/plugins/domain/domain_test.go` | AC-4 | |
| `TestClassifyRcodeNXDOMAINEmptiesSet` | `internal/component/firewall/plugins/domain/domain_test.go` | AC-5, the NXDOMAIN branch | |
| `TestClassifyRcodeServfailKeepsLastGood` | `internal/component/firewall/plugins/domain/domain_test.go` | AC-5, the SERVFAIL branch | |
| `TestClassifyRcodeRefusedKeepsLastGood` | `internal/component/firewall/plugins/domain/domain_test.go` | AC-5, the REFUSED branch | |
| `TestDomainGroupVerifyRefusesUncachedName` | `internal/component/firewall/plugins/domain/verify_test.go` | Wiring row 7; AC-6; user story 2 | |
| `TestDomainGroupColdStartProgramsFromCache` | `internal/component/firewall/plugins/domain/domain_test.go` | Wiring row 6; AC-7; user story 5 | |
| `TestEnrichShowAttachesNameToAddress` | `internal/component/firewall/plugins/domain/enrich_test.go` | AC-8 | |
| `TestScheduleClampsBelowFloor` | `internal/component/firewall/plugins/domain/schedule_test.go` | AC-9 | |
| `TestScheduleTreatsTTLZeroAsDoNotCache` | `internal/component/firewall/plugins/domain/schedule_test.go` | AC-9 | |
| `TestScheduleFamiliesIndependentTTL` | `internal/component/firewall/plugins/domain/schedule_test.go` | AC-10 | |
| `TestDomainGroupOnConfigureAfterRespawnPreservesChangeLogAndCache` | `internal/component/firewall/plugins/domain/domain_test.go` | AC-11, mirroring `TestReconfigureKeepsFetchedPrefixes` in `irr_test.go`. Process supervision itself is `ProcessManager.Respawn` and is proven generically | |
| `TestPluginResolveRPCReachesHubResolver` | `internal/component/plugin/server/dispatch_resolve_test.go` | Wiring row 3; A-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ttl-floor`, units seconds | 1..86400 | 86400 | 0, refused by the YANG range | 86401, refused by the YANG range |
| `maxChangeLogEntries`, entries retained in the change log | 1..10000 | exactly 10000 retained, oldest first | N-A, an append never rejects a write | the 10001st append evicts the oldest and the count stays at 10000 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-domain-group-update` | `test/plugin/firewall-domain-group-update.ci` | An operator fetches a name, then commits a rule permitting it. Wiring row 4, user story 1 | |
| `firewall-domain-group-clear` | `test/plugin/firewall-domain-group-clear.ci` | An operator clears a group and its set leaves the kernel. User story 6 | |
| `firewall-cli-domain-group-show` | `test/firewall/firewall-cli-domain-group-show.ci` | An operator reads the ruleset over the real SSH `ze cli` path and sees the DNS name beside each address. Wiring row 5, user story 3 | |

### Interop Tests (Scope: protocol)

N-A. `ai/rules/interop-and-goal-validation.md` permits omission for "a
config-only feature with no protocol impact", which this is: Scope is `config`,
the feature adds YANG config, a command surface and an internal plugin-to-hub
RPC, and it introduces no wire protocol Ze speaks to a peer. DNS queries go
through the existing `internal/component/resolve/dns` client, whose own RFC
conformance is owned by `plan/spec-fixit-dns-rfc1035-conformance.md`.

## Files to Modify

- `internal/component/resolve/dns/resolver.go` - `query` discards `resp.Rcode`
  and returns `(nil, 0, nil)` for every non-success rcode. Add the rcode to
  `query` and to `ResolveWithTTL` as a third return. `Resolve`, `ResolveA`,
  `ResolveAAAA`, `ResolveTXT` and `ResolvePTR` keep their signatures: they call
  `query` directly, not `ResolveWithTTL`. `extractRecords` and its TTL=0
  handling are unchanged.
- `internal/component/resolve/cmd/show_dns.go` - the only existing caller of
  `ResolveWithTTL`. Update the call site for the new return.
- `pkg/plugin/rpc/bridge.go` - add a DNS resolver handler type with
  `RegisterDNSResolver` and `GetDNSResolver`, mirroring `BatchValidateHandler`
  with `RegisterBatchValidator` and `GetBatchValidator`. This package imports
  nothing beyond `internal/core/selector`, so the indirection keeps the resolver
  out of packages that may not import it.
- `pkg/plugin/rpc/types.go` - add the resolve method constant and its input and
  result types, beside `MethodBatchValidate` and `BatchValidateInput`.
- `pkg/plugin/sdk/sdk_engine.go` - add the plugin-side `ResolveDNS` call, shaped
  like `RouteInstall`. No in-process fast path: this plugin runs forked, as
  `firewall-irr` does.
- `internal/component/plugin/server/dispatch_registry.go` - add one `engineOp`
  entry for the resolve method, with no typed wire slot, matching
  `MethodRouteInstall`.
- `cmd/ze/hub/main.go` - beside the existing `resolvecmd.SetResolvers` call,
  register the hub's resolver with the RPC layer. This is the one place
  authorized to wire the single `resolve.Resolvers` instance in, because
  `core-design.md` section 19 gives the hub "everything (orchestrator)".
- `pkg/zefs/keys.go` - register the domain-group cache key, per name within a
  group, following `KeyIRRPrefixCache`'s granularity.
- `internal/plugins/firewall/nft/cmd_show.go` - `handleShowFirewallRuleset` is
  the operator-facing handler. Two changes: its `Data` map carries `table`,
  `family` and `chains` and NO set elements, so add the sets; then call
  `show.Enrich` before returning so a registered enricher can attach the name.
  `handler_l2tp.go` is the working precedent for the call shape.

Design documents DECLARED by the files above, each named here because
`./le spec citation anchors` blocks until they are:

- `docs/architecture/api/ipc_protocol.md` - declared by `pkg/plugin/rpc/types.go`.
  AFFECTED: this spec adds a method constant and its input and result types, so
  the IPC method table gains a row.
- `docs/architecture/firewall/backend-command-dispatch.md` - declared by
  `internal/plugins/firewall/nft/cmd_show.go`. AFFECTED: the ruleset handler's
  response gains set elements and an enrichment call, which changes what the
  dispatch surface returns.
- `docs/architecture/hub-architecture.md` - declared by `cmd/ze/hub/main.go`.
  AFFECTED: hub startup gains the resolver registration into the RPC layer.
- `docs/architecture/diagnostics/procfs-diagnostics.md` - declared by
  `internal/component/resolve/cmd/show_dns.go`. UNAFFECTED: the only change to
  that file is updating one call site for `ResolveWithTTL`'s new return. No
  behavior the document describes changes.

Confirmed as needing NO edit: `internal/le/plugin/imports/pluginimports.go`
already scans the plugin directory generically. `ze-firewall-cmd.yang` and its
`self_containment_test.go` own only the existing show nodes; the new commands
live in the new plugin's own YANG and MUST NOT be added centrally, the same rule
`firewall-irr` follows. `internal/component/firewall/cmd/show.go` serves only
the offline debug binary and is out of scope.

## Files to Create

Plugin package `internal/component/firewall/plugins/domain/`, templated on
`internal/component/firewall/plugins/irr/`, registered as `firewall-domain` and
owning firewall tables under the same owner string:

- `register.go` - registration and the doctor check registration.
- `domain.go` - the engine entry and the SDK config and command callbacks.
- `config.go` - config parsing.
- `schedule.go` - per-name and per-family TTL scheduling. No IRR analog: IRR has
  one shared interval, this feature needs one timer per name and family.
- `sets.go` - builds the sets and tables from resolved addresses.
- `cache.go` - zefs load and save of the last-good addresses.
- `changelog.go` - the JSON-lines change log outside zefs, with the size bound
  R-3 requires. No IRR analog.
- `cmd_domain.go` - the command forwarders.
- `command.go` - command dispatch.
- `doctor.go` - the stale-data and no-data check.
- `enrich.go` - the enricher declaration and `OnEnrichShow` handler answering
  with the plugin's own address-to-name map. This is the sanctioned route for
  A-4, and it adds no field to `firewall.SetElement`.
- One `_test.go` per source file above, except `register.go`, matching IRR.

YANG under `internal/component/firewall/plugins/domain/yang/`:

- `doc.go`, `embed.go`, `register.go` - generated glue, written by
  `./le yang glue write`.
- `ze-firewall-domain-group.yang` - the config augment: a `domain-group` list
  keyed by name, holding a `domain-names` leaf-list and a `ttl-floor` leaf with
  `units seconds`.
- `ze-firewall-domain-group-cmd.yang` - the show, update and clear command tree,
  owned here rather than centrally.

Hub RPC handler:

- `internal/component/plugin/server/dispatch_resolve.go` - the resolve handler,
  calling the registered resolver through `pkg/plugin/rpc` rather than holding a
  resolver reference. `internal/component/plugin/server` may import only `aaa`
  and `audit` per the `core-design.md` Component Boundaries table, so it MUST
  NOT hold a `*resolve.Resolvers` field.

Functional tests and fixtures:

- `test/plugin/firewall-domain-group-update.ci` and its Go fixture, following
  `test/plugin/firewall-irr-update.ci` and `plugin_fixture_07_irr.go`, with a
  background mock DNS server.
- `test/plugin/firewall-domain-group-clear.ci` and its fixture.
- `test/firewall/firewall-cli-domain-group-show.ci` and its fixture, following
  `test/firewall/firewall-cli-show.ci` and the `netfilter_fixture.go`
  registration pattern, driving the real SSH `ze cli` path.
- `docs/architecture/firewall/firewall-domain-group.md` - the owning design doc
  the new source files' `// Design:` headers declare.

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/firewall/plugins/domain/yang/ze-firewall-domain.yang` (new) -- config augment adding a `domain-group` list (`domain-names` leaf-list, `ttl-floor` leaf) to `/fw:firewall`, modeled on `internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang`'s `irr` augment. `internal/component/firewall/plugins/domain/yang/ze-firewall-domain-cmd.yang` (new) -- `show`/`update`/`clear firewall domain-group` command tree, modeled on `internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang`. Generated glue (`yang/embed.go`, `yang/register.go`, `yang/doc.go`, all new) via `./le yang glue write`, same as `internal/component/firewall/plugins/irr/yang/embed.go` and `register.go` |
| YANG validation constraints | Yes | `domain-names` leaf-list: `type string { length "1..255"; pattern ...; }` (no `hostname`/`fqdn`/`domain` typedef exists in `internal/component/config/yang/ze-types.yang`, verified by grep across its typedef list, so the pattern is written inline, same as every other DNS-name-carrying leaf in the repo: `irr`'s `server` leaf, geodns `host`). `ttl-floor` leaf: `type uint32 { range "60..86400"; } units "seconds";`, modeled on `internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang`'s `refresh-interval` leaf (`range "0 \| 60..86400"; units "seconds";`) |
| YANG custom validators | N-A | The "this name has never resolved" refusal (AC-6) is stateful (a zefs cache read) and plugin-process-local. `firewall-irr`'s own precedent for the identical shape of check, `verifyRefs` (`internal/component/firewall/plugins/irr/irr.go`), runs inside the plugin SDK's `OnConfigVerify` callback, not as a YANG `ze:validate` + `ValidateFn` custom validator -- confirmed by `grep -rn "ze:validate\|ValidateFn" internal/component/firewall/plugins/irr/` returning zero hits. `ze:validate` registers with the hub's in-process config validator registry (`yang.CheckAllValidatorsRegistered`, `ai/patterns/config-option.md`), which is the wrong process for a check that needs the plugin's own zefs-backed cache. Every leaf's syntax is covered by native constraints (row above) |
| CLI commands/flags | Yes | New `show firewall domain-group [<name>]`, `update firewall domain-group <name>`, `clear firewall domain-group <name>` command declarations in `internal/component/firewall/plugins/domain/yang/ze-firewall-domain-cmd.yang` (new), forwarded server-side from a new `internal/component/firewall/plugins/domain/cmd_domain.go` via `pluginserver.RegisterRPCs`, modeled verbatim on `internal/component/firewall/plugins/irr/cmd_irr.go` (`WireMethod: "ze-show:firewall-irr-status"` etc.) |
| CLI grammar (keyword before value) | Yes | `domain-group <name>` is a mandatory `leaf name` taking a bare trailing value with no subcommand children, the same shape `ai/rules/cli.md` and `internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang`'s `asn <asn>` / `as-set <name>` leaves already use; not an R5/R6 violation for the same reason those are not (terminal leaf, no children) |
| Editor autocomplete | No | `internal/component/firewall/plugins/irr/cmd_irr.go` and `command.go` wire zero `CompleteFn` for their `asn`/`as-set` leaves (verified: `grep -n "CompleteFn" internal/component/firewall/plugins/irr/*.go` returns nothing), even though those are also operator-defined, not fixed-enum, values. This spec's `domain-group <name>` leaf matches that same precedent and ships with no `CompleteFn`, so it introduces no new autocomplete gap beyond the one `firewall-irr` already carries |
| Functional test for new RPC/API | Yes | `test/plugin/firewall-domain-group-update.ci` (new), `test/plugin/firewall-domain-group-clear.ci` (new), `test/firewall/firewall-cli-domain-group-show.ci` (new) -- named in the spec's own Wiring Test and End-to-End User Stories tables, following the naming of `test/plugin/firewall-irr-update.ci` and `test/firewall/firewall-cli-show.ci` |
| Pipe completeness | Yes | The new forwarded handlers return `plugin.Response.Data` (a `ResponseData`-satisfying payload), the same shape `internal/component/firewall/plugins/irr/cmd_irr.go`'s forwarders return. `ApplyPipes` is invoked centrally by `internal/component/command/render_records.go` over every handler's structured output (confirmed: `grep -rln "ApplyPipes(" --include="*.go" internal/` finds callers only in `internal/component/command/`, never in a plugin), so pipe support is automatic and needs no plugin-side wiring, matching `firewall-irr` |
| Env var registration | N-A | This is YANG config, not an env var. The spec's own Required Reading section already makes this call explicitly: an operator changes which names a domain group holds during normal operation, so it must appear in `show configuration` and a config backup, which the YANG-vs-env-var decision table in `ai/rules/config.md` resolves to YANG on that fact alone |
| Doctor check for runtime dependencies | Yes | `internal/component/firewall/plugins/domain/doctor.go` (new) -- a `checkDomainGroupFreshness` function and `registerDomainDoctor()` call in `internal/component/firewall/plugins/domain/register.go` (new), modeled on `registerIRRDoctor` (`internal/component/firewall/plugins/irr/register.go`) and `checkIRRDataFreshness` (`internal/component/firewall/plugins/irr/doctor.go`). New diagnostic codes `doctor-firewall-domain-group-stale-data` and `doctor-firewall-domain-group-no-data` (parallel to `codeIRRStaleData`/`codeIRRNoData`), registered via `diagnostic.Register` (`internal/core/diagnostic/registry.go`, code metadata table `internal/core/diagnostic/codes.go` is the built-in-only table and is NOT where these go) and `diagnostic.RegisterDoctorCheck` (`internal/core/diagnostic/doctor_registry.go`). Unit test `doctor_test.go` (new) and `.ci` coverage per `ai/rules/repo-maintenance.md` |
| Prometheus counters/metrics | Yes | Following the `ze_firewall_irr_*` shape defined in `setMetricsRegistry` (`internal/component/firewall/plugins/irr/irr.go`): `ze_firewall_domain_group_addresses_cached` (Gauge) -- total resolved addresses cached across every domain group, parallel to `ze_firewall_irr_prefixes_cached`. `ze_firewall_domain_group_refresh_outcomes_total` (CounterVec, label `result`) -- parallel to `ze_firewall_irr_refresh_outcomes_total`, whose label values are `success`/`empty`/`error` (verified: `grep -n "incRefreshOutcome(" internal/component/firewall/plugins/irr/irr.go`); this spec's label values are `unchanged`, `changed`, `nxdomain`, `error`, matching the four outcomes AC-2 through AC-5 define. `ze_firewall_domain_group_last_refresh_timestamp` (Gauge) -- parallel to `ze_firewall_irr_last_refresh_timestamp`. `ze_firewall_domain_group_data_age_seconds` (Gauge) -- parallel to `ze_firewall_irr_data_age_seconds`. Defined in a new `internal/component/firewall/plugins/domain/domain.go` `setMetricsRegistry`, wired through `ConfigureMetrics` in `register.go`, exactly as `internal/component/firewall/plugins/irr/register.go` does |
| BGP family surface (new SAFI / capability / attribute) | N-A | No SAFI, capability or attribute is added or changed. This is a firewall/DNS feature with no BGP wire surface |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- the existing Firewall row (confirmed at line 98) already documents `firewall-irr` in the same paragraph ("IRR-based prefix-list filtering: ..."); add a parallel sentence for domain-group naming the `domain-group` config surface and the DNS re-resolution behavior |
| 2 | Config syntax changed? | No | No new YANG PATTERN is introduced (leaf-list, list, presence container, `units` are all existing patterns already documented in `docs/architecture/config/syntax.md`), only new leaves within existing pattern types. Precedent: `firewall-irr` added a comparable-shaped `list interface` plus several leaves and touched neither `docs/guide/configuration.md` nor `docs/architecture/config/syntax.md` (confirmed: both files grep for "firewall irr" / "firewall-irr" with zero hits) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- confirmed precedent at "### show, update and clear firewall irr" (line 976); add a parallel "### show, update and clear firewall domain-group" section documenting the three new commands |
| 4 | API/RPC added/changed? | No | `docs/architecture/api/commands.md` documents the generic verb taxonomy and dispatch architecture (show/set/delete/update/monitor), not a per-command catalog. `firewall-irr`'s own forwarded `ze-show:`/`ze-update:`/`ze-clear:` RPCs are not named there either (confirmed: zero grep hits), because they are new INSTANCES of an already-documented mechanism, not a new mechanism |
| 5 | Plugin added/changed? | No | `docs/guide/plugins.md` does not document `firewall-irr` either (confirmed: zero grep hits for "firewall-irr"/"firewall irr"); the plugin's user-facing content lives in `docs/guide/firewall.md` instead (row 6) |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md` -- confirmed precedent: a full `firewall-irr` section (from line ~364) covers commands, the stale-data doctor codes, and the `ze_firewall_irr_*` metrics. Add a parallel `firewall-domain-group` section covering the `domain-group` config, the `show`/`update`/`clear` commands, the change log, and the new metrics/doctor codes |
| 7 | Wire format changed? | N-A | No wire encoding (BGP, IPsec, L2TP, or any on-the-wire byte format) changes. The plugin-to-hub resolve call is an internal RPC (row 8), not a wire protocol |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` -- the plugin-engine RPC method table (from line ~604, sourced from `pkg/plugin/sdk/sdk_engine.go` -- confirmed: `route-install`, `batch-validate`, `dispatch-command` rows already there) needs a new row for the resolve RPC this spec adds. `ai/rules/plugins.md` is unaffected: the new RPC follows the existing `RPCRegistration`/self-contained-value-type pattern the rule already states, so no new rule text is owed |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is `config`; this spec consumes an existing DNS resolver and adds no wire-visible protocol behavior. Already stated by the spec's own "RFC Summaries" section |
| 10 | Test infrastructure changed? | No | New tests use existing `.ci` / `ze-test` / unit-test infrastructure (`test/plugin/`, `test/firewall/`); no new test tool, harness, or runner is added |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares BGP-daemon feature parity (address families, session features) against BIRD/FRR/GoBGP/etc.; a firewall DNS-sourced group is out of that table's scope, confirmed by its "Overview"/"Address Families" structure |
| 12 | Internal architecture changed? | Yes | `docs/architecture/firewall/firewall-domain-group.md` (new) -- modeled on `docs/architecture/firewall/firewall-irr.md`, needed both because the new source files' `// Design:` headers must declare an owning doc and because the plugin's own shape (refresh-loop ownership, the plugin-to-hub resolve RPC boundary, the zefs/JSON-lines split) is a structural decision worth recording, per the precedent `docs/architecture/firewall/firewall-irr.md` sets for `firewall-irr` |
| 13 | Route metadata keys added/changed? | N-A | No BGP route metadata (`internal/component/bgp` per-UPDATE `map[string]any`) is read or written. This feature only affects firewall set membership |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/firewall.md` -- confirmed this, not `docs/plugin-development/metrics.md`, is where `ze_firewall_irr_refresh_outcomes_total`, `ze_firewall_irr_data_age_seconds` and `ze_firewall_irr_last_refresh_timestamp` are actually documented (`docs/plugin-development/metrics.md` is a generic how-to page naming `rib`/`gr`/`fibkernel` as examples and carries no `firewall-irr` metric names at all, confirmed by grep). Document the four new `ze_firewall_domain_group_*` metrics in the same `docs/guide/firewall.md` section added for row 6 |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `docs/plugin-overview.md`, `docs/features/plugins.md` and `docs/guide/status.md` each grep to zero hits for "firewall-irr"/"firewall irr"; none of the three catalogs individual plugins (they describe the registration MECHANISM using `rib`/`gr` as worked examples), so a new firewall plugin does not get an entry there either |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED from `./le spec citation anchors spec plan/spec-firewall-domain-group.md`, rerun 2026-09-02 once Files to Modify carried real paths. It reports 4 DECLARED design docs, which block, and 23 advisory `<!-- source: -->` mentions, which do not. All four declared docs are named under `## Files to Modify`: `docs/architecture/api/ipc_protocol.md`, `docs/architecture/firewall/backend-command-dispatch.md` and `docs/architecture/hub-architecture.md` are affected and get edits; `docs/architecture/diagnostics/procfs-diagnostics.md` is recorded there as unaffected, because the only change to `show_dns.go` is a call-site update for the new return |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/firewall.md` and `docs/guide/configuration.md` show existing `set`/rule/`irr` config examples in the firewall area. Once the `domain-group` YANG and commands are implemented, verify those examples still match (no leaf renamed, no command syntax drifted) before closing |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- register the plugin and every entry point as stubs, and write the wiring tests so they fail for the right reason
   - Tests: every row of the Wiring Test table, all failing
   - Files: the new plugin package's `register.go` and engine entry, its YANG glue, the command registrations
   - Verify: `./le repository generate` discovers the package, the plugin starts, and each wiring test fails because the handler is a stub rather than because nothing is registered
2. **Phase: Resolver rcode** -- surface the DNS response rcode so a caller can separate absence from failure
   - Tests: `TestResolveWithTTLSurfacesRcode`, plus a test that existing callers are unaffected
   - Files: `internal/component/resolve/dns/resolver.go`
   - Verify: NXDOMAIN, SERVFAIL, REFUSED and NOERROR-empty are each distinguishable. This phase comes before resolution because AC-5 cannot be written without it
3. **Phase: Plugin-to-hub resolve RPC** -- one call in the shape of the existing `sdk_engine.go` calls, and its hub handler
   - Tests: `TestPluginResolveRPCReachesHubResolver`
   - Files: `pkg/plugin/sdk/sdk_engine.go`, the hub-side handler, the rpc types
   - Verify: a plugin lookup populates the same cache `show dns` reads, proving no second resolver exists (A-2)
4. **Phase: Config and verify refusal** -- the YANG surface, and the commit refusal for a never-resolved group
   - Tests: `TestDomainGroupVerifyRefusesUncachedName`
   - Files: the plugin's YANG config module, its config parse, its `OnConfigVerify`
   - Verify: AC-6. Verify performs no network I/O
5. **Phase: Resolution, change detection and the zefs cache** -- the refresh loop, the TTL schedule, and last-good persistence
   - Tests: `TestDomainGroupRefreshFiresAtTTL`, `TestDomainGroupColdStartProgramsFromCache`, and the AC-2, AC-4, AC-5, AC-9, AC-10 unit tests
   - Files: the plugin's refresh loop and store, `pkg/zefs/keys.go`
   - Verify: AC-2, AC-4, AC-5, AC-7, AC-9, AC-10. The loop is an owning type with start, Stop and Wait per `ai/rules/goroutine-lifecycle.md`, and it recovers rather than letting a bad answer kill the process (R-6)
6. **Phase: The change log** -- JSON-lines append, bounded and rotated
   - Tests: `TestDomainGroupChangeLogRecordsOldAndNew`, plus the size-bound boundary test
   - Files: the plugin's change-log writer
   - Verify: AC-3, and that AC-2 writes nothing
7. **Phase: Set programming** -- build the sets and reconcile
   - Tests: `TestDomainGroupConfigureRegistersSet`
   - Files: the plugin's set building and `RegisterTables` call
   - Verify: AC-1. No lock is held across `ApplyAll`, and `ErrKernelTimeout` does not trigger an immediate retry
8. **Phase: Commands and show enrichment** -- show, update and clear, and the name beside the address
   - Tests: the three `.ci` tests, and `test/firewall/firewall-cli-domain-group-show.ci`
   - Files: the plugin's command handlers, its YANG command module, the `show.Enrich` call site in the firewall show handler, the plugin's `OnEnrichShow`
   - Verify: AC-8 and user stories 1, 3 and 6
9. **Phase: Doctor check and metrics** -- the runtime-dependency check and the counters
   - Tests: the doctor check unit and functional tests
   - Files: the plugin's `register.go`, `internal/core/diagnostic/codes.go`
   - Verify: a group whose names cannot be resolved is reported by `ze doctor`

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific, for example "merge order correct", "error messages name the offending value"] |
| Naming | [feature-specific, for example "JSON keys kebab-case", "YANG leaf matches env var leaf"] |
| Data flow | [feature-specific, for example "resolution in X only, reactor unaware of Y"] |
| Rule: [relevant rule] | [what to check] |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

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
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Its own spec, with the sourced-group substrate named as a dependency on `spec-firewall-remote-group` | One spec covering both URL and DNS sources; a fully standalone spec accepting three copies of the machinery | Owner, 2026-09-02. Keeps this spec small and homes the unhomed deferral row in the retired deferral shard "firewall-remote-group", without rewriting scope that spec already agreed |
| A plugin under `internal/component/firewall/plugins/`, reaching the hub's resolver over a new SDK RPC | An in-hub firewall component owning the loop; extending the `firewall-irr` plugin | Owner, 2026-09-02, REVERSING an earlier in-hub decision once research showed it had no precedent. Every firewall feature owning a timer is a separate process, that directory is already a codegen discovery path, and `core-design.md` section 19 has no component-boundary row permitting a firewall-to-resolve import. The shared DNS cache was the reason for the hub, and it survives: `sdk_engine.go` already carries plugin-to-hub calls, so one resolve RPC keeps a single resolver and a single cache. Extending `firewall-irr` was rejected for conflating IRR prefix data with DNS name data in one config tree |
| The DNS name beside the address comes from the show enricher, never from a field on `SetElement` | A provenance field on the shared `SetElement`; a parallel map joined hub-side | `ai/rules/principles.md` bans the field-list edit outright, and every other table owner would carry it unused. `OnEnrichShow` (`sdk_callbacks.go`) plus `show.Enrich` (`internal/core/show/show.go`) is a working production path, used today by `handler_l2tp.go` and `l2tp/subscriber/cmd/subscriber.go`. The firewall show handlers do not call it yet, so this spec adds that call site |
| The resolver is changed to surface the response rcode | Never emptying a set from DNS; splitting the rcode work into its own spec that this one blocks on | Owner, 2026-09-02. Without it a SERVFAIL and a deleted name are the same signal, so either a failed server empties a live firewall set or a deleted name enforces stale addresses forever. Neither is acceptable in a firewall, and deferring it would leave this spec unable to state correct behavior |
| A group that has never resolved is refused at verify, and resolved addresses persist in zefs | Accept the commit and let `dropTablesMissingAProvidedSet` hold the table back, as IRR does today; register an empty set so the table is still programmed | Owner, 2026-09-02. The existing hold-back policy states "an unfiltered port beats a blackholed one", which silently removes a whole table when a name has never resolved. Refusing at commit tells the operator while they are still at the terminal. An empty set was rejected because it inverts meaning: an empty deny matches nothing, an empty permit blocks everything |
| Every DNS change is stored in a DNS log in zefs | The operator audit log (`internal/core/audit`, `<config>.audit.jsonl`); a separate flat file with its own rotation | Owner, 2026-09-02. The operator audit log is bounded and records operator actions; a rotating name is neither, and would evict commit history |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- **DNSSEC gives weaker assurance than its name suggests.** `dnssecDecision`
  (`resolver.go`) does no signature validation of its own: it sets the EDNS0 DO
  bit and trusts an upstream validating resolver's SERVFAIL, and it always
  accepts AD=0 from an unsigned zone. So a domain group cannot be described to
  an operator as cryptographically verified. Recorded as R-5; making Ze validate
  signatures itself is a resolver-sized piece of work and is not in this spec.
- **A permit rule keyed on a DNS name trusts whoever controls that name.** This
  spec caps addresses per name and honors the DNSSEC setting, and it does not
  otherwise constrain what a name may resolve to. That is an operator decision,
  and the documentation must say so plainly rather than implying safety.
- **Completion for configured group names is not registered, it is hand-wired.**
  Dynamic value completion lives in `internal/component/cli/client/main.go` with
  no registration path, which sits against `ai/rules/plugins.md` on plugin
  spelling in central packages. This spec follows the existing mechanism rather
  than inventing a registry for it; the tension is recorded here so a later spec
  can address it for every plugin at once.
- **The substrate this spec depends on is not built yet.**
  `plan/spec-firewall-remote-group.md` is still `skeleton`, so the shared
  sourced-group machinery it is meant to own does not exist. See the deferral
  shard and A-5: either that spec lands first, or the shape is settled jointly
  before this one implements.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `firewall-remote-group.md`, 2026-08-02

Deferred by spec-firewall-remote-group.

FQDN and domain groups
