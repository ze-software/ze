# Spec: On-Demand Route Origination (Gap D)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-cp-survival-0-umbrella |
| Phase | 1/11 |
| Updated | 2026-06-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/bgp/plugins/cmd/update/update_text.go` (existing runtime announce path)
4. `internal/component/bgp/reactor/reactor_api_batch.go` (`AnnounceNLRIBatch`)
5. `internal/plugins/flowspec-firewall/engine.go` (receive-only bridge — the reverse of D2)

## Task

Let an operator (or automation) originate a BGP route to selected peers **on demand** without
pre-staging static config or hand-writing the raw `peer * update text ... nlri ...` grammar.
Primary use cases are DDoS mitigation (FlowSpec, RTBH) but the verb is **family-agnostic**: any
supported BGP family (unicast, flowspec, ...) can be announced and withdrawn through the same
mechanism.

Every on-demand announcement is **tracked** in an in-memory announcement registry. The operator can
list active announcements, withdraw them individually (by ID or label), or withdraw in bulk (all, by
selector, by label pattern). Tracking ensures no orphaned announcements survive unnoticed.

Investigation found the runtime announce *path already exists* (`AnnounceNLRIBatch`, reached via the
`ze-bgp:peer-update` text RPC). What is missing is (D1) an **ergonomic, discoverable operator verb**
with a **tracked announcement registry**, and (D2) a **firewall→FlowSpec reverse bridge** so a local
mitigation rule can be lowered into an outbound announcement (`flowspec-firewall` only does the
receive direction today).

This is **not** a new BGP family: FlowSpec SAFI 133/134, the FlowSpec NLRI codec, and the BLACKHOLE
community all already exist. Origination reuses them. (BGP Family Gate evaluated → N/A, see below.)

Two phases: **D1 (operator verb + tracking)** is the must-have; **D2 (firewall bridge)** is a second phase.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/buffer-first.md` - the announce UPDATE must build via pooled `WriteTo`
  → Constraint: origination must work under memory pressure; reuse existing buffer-first encoders, no per-announce heap churn.
- [ ] `ai/rules/cli-grammar.md` - verb ordering
  → Constraint: action before identifier (`announce unicast ...`, `announce flowspec ...`, not `flowspec announce`).
- [ ] `ai/rules/plugin-self-containment.md` - command lives with BGP
  → Constraint: D1 verb registers under `internal/component/bgp/plugins/cmd/`; D2 bridge is its own plugin depending on firewall + the reactor announce API.
- [ ] `ai/patterns/bgp-family.md` - BGP Family Gate
  → Decision: N/A — no new SAFI/capability/attribute; reuses existing families. Documented in "BGP Family Gate" below.

### RFC Summaries
- [ ] `rfc/short/rfc7999.md` - BLACKHOLE community 0xFFFF029A (RTBH).
  → Constraint: BLACKHOLE rides on ordinary unicast NLRI; next-hop/scope per upstream policy.
- [ ] `rfc/short/rfc8955.md` - FlowSpec v4 (and rfc8956 v6).
  → Constraint: traffic-action extended communities (discard 0x8006, rate-limit) define the action.

**Key insights:**
- `AnnounceNLRIBatch(sel, batch)` (reactor_api_batch.go:28) is the single call both phases use.
  Non-established peers are handled (ShouldQueue branch); established peers get an immediate UPDATE.
  The API is already family-agnostic: it takes an `NLRIBatch` with any family's NLRI.
- Runtime announcements are **ephemeral** (not persisted to config; not re-sent after a daemon
  restart). For DDoS mitigation this is acceptable (re-trigger), but it must be documented, and the
  verb should support an optional auto-withdraw duration so stale rules don't linger (umbrella R-3).
- Today `AnnounceNLRIBatch` is fire-and-forget: there is no registry of what was announced at
  runtime. The operator has no way to list active on-demand announcements or withdraw them by
  reference. The tracking registry (D1) fills this gap.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` (lines 28-287) - `AnnounceNLRIBatch(sel, batch)` / `WithdrawNLRIBatch`; matches peers, queues non-established, builds + sends UPDATE.
  → Constraint: both phases call this; do not build a second announce path.
- [ ] `internal/component/bgp/plugins/cmd/update/update_text.go` (lines 488-819) - `ParseUpdateText` → `DispatchNLRIGroups` → `AnnounceNLRIBatch`/`WithdrawNLRIBatch`; RPC `ze-bgp:peer-update` registered in `init()` (656-660).
  → Constraint: D1 reuses this dispatch (or calls AnnounceNLRIBatch directly with a built batch); it is sugar, not a new mechanism.
- [ ] `internal/component/bgp/config/bgp_routes_flowspec.go` (lines 18-149) - static FlowSpec config → PluginRoutes.
  → Constraint: D1 is the runtime analogue; share the NLRI-building helpers, not the config path.
- [ ] `internal/component/bgp/plugins/nlri/flowspec/encode.go` + `register.go` - FlowSpec NLRI encoder; Families "ipv4/flow","ipv6/flow",...
  → Constraint: reuse this encoder to build the batch NLRI; no new codec.
- [ ] `internal/component/bgp/attribute/community.go` (line 99) - `CommunityBlackhole = 0xFFFF029A`, named "blackhole".
  → Constraint: RTBH = unicast NLRI + this community; reuse.
- [ ] `internal/plugins/flowspec-firewall/engine.go` - RECEIVE-only: BGP UPDATE → nftables. No reverse path.
  → Constraint: D2 is a new, separate plugin; do not bolt onto flowspec-firewall (keep receive/originate independent).

**Behavior to preserve:**
- Static config FlowSpec origination and the text-protocol RPC keep working unchanged.
- `flowspec-firewall` receive direction unchanged.
- Existing community parsing/JSON unchanged.

**Behavior to change:** Add a runtime announce verb with tracking (D1) and an export bridge plugin (D2). Both additive.

## Tag Registry

A **tag** is a key-value annotation on route operations. Routes carrying a tag are tracked in an
in-memory registry. The tag key+value pair is the handle for lifecycle management: withdraw all
routes matching a tag, list active tagged routes, or let them auto-expire.

Tags are generic: any key is valid. Conventions:
- `tag mitigation ddos-udp` -- operator-chosen grouping for on-demand routes
- `tag watchdog pool-1` -- watchdog routes (future: migrate watchdog plugin to use tags)
- `tag flowspec-egress rule-7` -- D2 bridge tagging

The registry is the **shared code path** for the CLI announce verb (D1), the plugin API, and the
future watchdog migration. The reactor (`AnnounceNLRIBatch`/`WithdrawNLRIBatch`) stays stateless;
the registry wraps it.

### Tag Entry Fields

| Field | Type | Description |
|-------|------|-------------|
| ID | uint64 | Auto-incrementing per entry, unique per daemon lifetime |
| TagKey | string | Tag key (e.g. "mitigation", "watchdog", "flowspec-egress") |
| TagValue | string | Tag value (e.g. "ddos-udp", "pool-1") |
| Family | family.Family | BGP family (e.g. ipv4/unicast, ipv4/flow) |
| Selector | *selector.Selector | Peer selector the announcement was sent to |
| Batch | NLRIBatch | The batch sent (needed for withdraw) |
| CreatedAt | time.Time | When the entry was created |
| ExpiresAt | *time.Time | nil if no duration; otherwise auto-withdraw deadline |
| Source | string | "cli" (D1), "plugin" (API), or "flowspec-egress" (D2) |

### Shared Code Path

```
CLI: announce ... tag <key> <value>       Plugin: UpdateRoute(sel, cmd, Meta:{"tag.<key>":"<val>"})
         │                                                    │
         ▼                                                    ▼
         └──────────────────┐    ┌────────────────────────────┘
                            ▼    ▼
                     ┌─ Tag Registry ─────────────────────┐
                     │  Announce(key, val, sel, batch, dur)│
                     │  WithdrawTag(key, val)              │
                     │  WithdrawTagKey(key)  (all values)  │
                     │  WithdrawAllTags()    (all keys)    │
                     │  WithdrawEntry(id)                  │
                     │  WithdrawAll([selector])             │
                     │  List([key, val, selector, family]) │
                     │  auto-withdraw timers               │
                     └──────────────┬─────────────────────┘
                                    ▼
                        AnnounceNLRIBatch / WithdrawNLRIBatch
                              (reactor, stateless)
```

### Registry Operations

- **Announce**: caller builds `NLRIBatch` → `registry.Announce(key, val, sel, batch, duration)` →
  creates entry → `AnnounceNLRIBatch(sel, batch)` → returns entry ID.
- **WithdrawTag**: `registry.WithdrawTag(key, val)` → withdraws all entries matching key+value →
  `WithdrawNLRIBatch` for each → removes entries.
- **WithdrawTagKey**: `registry.WithdrawTagKey(key)` → withdraws ALL entries under a tag key
  (equivalent to `withdraw tag <key> *`).
- **WithdrawAllTags**: `registry.WithdrawAllTags()` → withdraws ALL tagged entries across all keys
  (equivalent to `withdraw tag *`).
- **WithdrawEntry**: `registry.WithdrawEntry(id)` → withdraws one entry by ID.
- **WithdrawAll**: `registry.WithdrawAll(selector)` → withdraws all entries (optionally filtered by
  peer selector).
- **List**: `registry.List(filter)` → returns entries (filterable by key, value, selector, family).
- **Auto-withdraw**: duration timer fires → `WithdrawNLRIBatch` + remove entry.

### Plugin API Changes

No new RPC fields needed. The existing `Meta map[string]any` on `UpdateRouteInput` is the transport.
Meta keys with a `tag.` prefix opt into the registry:

- `Meta: {"tag.mitigation": "ddos-udp"}` → registry.Announce(key="mitigation", val="ddos-udp", ...)
- `Meta: {"tag.watchdog": "pool-1"}` → same mechanism, watchdog-flavored

`UpdateRouteOutput` gains an optional `RouteIDs` field (populated only when a tag was present).
New SDK methods:

- `WithdrawTag(ctx, key, value)` returns `(withdrawn, err)`
- `ListTag(ctx, key)` returns `([]TagEntry, err)`

These map to new RPCs: `ze-plugin-engine:withdraw-tag`, `ze-plugin-engine:list-tag`.

D2 entries are also tracked here (source="flowspec-egress") so the operator has a single view.
The registry lives in `internal/component/bgp/plugins/cmd/announce/`, exposed via the plugin
server's RPC dispatch.

### Watchdog Migration (future, separate spec)

The watchdog plugin currently manages its own route pools with per-peer state, MED overrides, and
reconnect resend logic. That logic stays. The migration would have the watchdog plugin tag its
routes via `Meta: {"tag.watchdog": "<pool-name>"}` when calling `UpdateRoute`, and
`request bgp watchdog withdraw <name>` would call `WithdrawTag("watchdog", name)` under the hood.
This is NOT in scope for this spec but the tag registry is designed to support it.

## Data Flow (MANDATORY)

### Entry Point
- D1 (CLI): `announce <family> <selector> <nlri> [attributes] [tag <key> <value>] [for <duration>]`
  - `announce unicast upstream 192.0.2.0/24 next-hop self community no-export tag mitigation maint-window for 3600s`
  - `announce blackhole upstream 192.0.2.1/32 tag mitigation ddos-victim for 600s` (sugar for unicast + blackhole community)
  - `announce flowspec upstream match {dest 198.51.100.0/24 proto udp} then discard tag mitigation ddos-udp for 300s`
  - `withdraw tag mitigation ddos-udp` (by key+value), `withdraw tag mitigation *` (all values under key)
  - `withdraw tag *` (all tagged entries), `withdraw id 17` (by entry ID), `withdraw all` (everything)
  - `show announcements [tag <key>] [selector <peer-pattern>]`
- D1 (plugin API): `UpdateRoute(sel, cmd, Meta: {"tag.mitigation": "ddos-udp"})` via existing
  `ze-plugin-engine:update-route`; `WithdrawTag(ctx, "mitigation", "ddos-udp")` via
  `ze-plugin-engine:withdraw-tag`.
- D2: a local firewall rule tagged for export changes → event.

### Transformation Path
1. **D1 (CLI):** verb parses args → builds `NLRIBatch` → `registry.Announce(key, val, sel, batch, dur)`
   → creates entry → `AnnounceNLRIBatch(sel, batch)` → UPDATE on wire. Duration schedules auto-withdraw.
2. **D1 (plugin):** `UpdateRoute` with `Meta: {"tag.<key>": "<val>"}` → dispatch →
   `DispatchNLRIGroups` → calls `registry.Announce(key, val, ...)` instead of bare
   `AnnounceNLRIBatch`. Returns entry IDs in output.
   Without a `tag.*` meta key: unchanged fire-and-forget path (no registry involvement).
3. **D2:** `flowspec-egress` subscribes to firewall export events → translates match to FlowSpec
   NLRI → `registry.Announce("flowspec-egress", ruleID, ...)` → `AnnounceNLRIBatch`. Rule removed →
   `registry.WithdrawTag("flowspec-egress", ruleID)` → `WithdrawNLRIBatch` + remove entries.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI verb ↔ tag registry (D1 CLI) | verb → `registry.Announce(key, val, sel, batch)` | [ ] |
| Plugin API ↔ tag registry (D1 plugin) | dispatch → `registry.Announce(...)` when `tag.*` meta key present | [ ] |
| Tag registry ↔ reactor | registry → `AnnounceNLRIBatch` / `WithdrawNLRIBatch` | [ ] |
| reactor ↔ wire | existing UPDATE builder (buffer-first) | [ ] |
| firewall events ↔ tag registry (D2) | `flowspec-egress` → `registry.Announce(...)` | [ ] |

### Integration Points
- `internal/component/bgp/reactor/reactor_api_batch.go` `AnnounceNLRIBatch`/`WithdrawNLRIBatch` — unchanged, stateless
- `internal/component/bgp/plugins/cmd/announce/registry.go` — new tag registry (both phases + plugin API)
- `internal/component/bgp/plugins/cmd/update/update_text.go` `DispatchNLRIGroups` — conditionally calls registry when `tag.*` meta key present
- `internal/component/bgp/plugins/nlri/flowspec/encode.go` — FlowSpec NLRI building (D1 flowspec, D2)
- `internal/component/bgp/attribute/community.go` `CommunityBlackhole` — RTBH shorthand (D1)
- `pkg/plugin/rpc/types.go` `UpdateRouteOutput` — new optional `RouteIDs` field (no input changes, uses existing Meta)
- `pkg/plugin/sdk/sdk_engine.go` — new `WithdrawTag`, `ListTag` methods
- `internal/plugins/flowspec-firewall` — sibling (receive); D2 is the originate counterpart

### Architectural Verification
- [ ] No bypassed layers (CLI verb / plugin API / D2 → tag registry → AnnounceNLRIBatch → wire)
- [ ] Single tagging path (CLI `tag` and plugin `Meta:{"tag.*"}` both go through registry.Announce; no parallel tracking)
- [ ] No unintended coupling (D2 talks to registry + reactor via public APIs, not internals)
- [ ] No duplicated functionality (reuses NLRI encoder, community, batch API; registry is only new state)
- [ ] Zero-copy preserved (buffer-first UPDATE encoding reused)
- [ ] Untagged path preserved (UpdateRoute without tag.* Meta: fire-and-forget, no registry, backward compat)
- [ ] ExaBGP compatibility preserved (migration code, YANG watchdog container, update text watchdog keyword all unchanged)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `AnnounceNLRIBatch` can be called from a new cmd plugin with a programmatically built `NLRIBatch` for any family | update_text.go does exactly this for all families | D1 needs a deeper hook | trace DispatchNLRIGroups → AnnounceNLRIBatch + a unit test | unvalidated |
| A-2 | Runtime announcements are ephemeral (lost on daemon restart); acceptable for mitigation | AnnounceNLRIBatch has no persist path | operators expect persistence | doc as designed behavior; functional test confirms re-announce needed | unvalidated |
| A-3 | No new BGP family work needed (existing families + BLACKHOLE exist) | flowspec/register.go, community.go:99 | hidden family wiring required | BGP Family Gate table below all N/A with file evidence | unvalidated |
| A-4 | An auto-withdraw timer can be scheduled in-reactor without blocking | reactor has timers (hold/keepalive) | duration feature unsafe | design uses the reactor clock; unit test the schedule/cancel | unvalidated |
| A-5 | An in-memory registry with mutex is sufficient (no persistence needed) | DDoS mitigation is re-triggerable; registry only for operator visibility | operators need persist across restart | document limitation; consider optional persist in future | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stale announcement stays active after the need ends | upstream keeps filtering / routing | explicit `withdraw` verb + `withdraw all` bulk + optional `for <duration>` auto-withdraw; `show announcements` lists all active |
| R-2 | Operator originates a FlowSpec that filters legitimate traffic (foot-gun) | collateral drop upstream | validate match spec; require explicit action; doc; consider a confirm for discard-all |
| R-3 | D2 firewall→FlowSpec creates a feedback loop with received FlowSpec→firewall (flowspec-firewall) | rule oscillation | D2 only exports rules explicitly tagged for export; never re-exports received ones |
| R-4 | Announce to the wrong peers (selector too broad) leaks routes to non-upstreams | route seen on transit/customer sessions | selector required + defaulted narrowly; doc; export policy on the upstream session |
| R-5 | Registry grows unbounded if announcements accumulate without withdrawal | memory growth over time | auto-withdraw durations encouraged; `show announcements` makes accumulation visible; warn on high count |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `announce unicast <peer> ... tag mitigation maint` | → | verb → NLRI + community → registry.Announce → `AnnounceNLRIBatch` → UPDATE | `unicast-announce.ci` |
| `announce blackhole <peer> ... tag mitigation ddos` | → | verb → NLRI + BLACKHOLE → registry.Announce → `AnnounceNLRIBatch` → UPDATE | `rtbh-announce.ci` |
| `announce flowspec <peer> ... tag mitigation ddos-udp` | → | verb → FlowSpec NLRI + ext-comm → registry.Announce → `AnnounceNLRIBatch` | `flowspec-announce.ci` |
| `withdraw tag <key> <value>` / `withdraw tag <key> *` / `withdraw tag *` / `withdraw id <N>` / `withdraw all` | → | registry.WithdrawTag/TagKey/AllTags/Entry/All → `WithdrawNLRIBatch` | `announce-withdraw.ci` |
| `show announcements [tag <key>]` | → | registry.List → formatted output | `announce-show.ci` |
| Plugin: `UpdateRoute(sel, cmd, Meta:{"tag.X":"Y"})` | → | dispatch → registry.Announce → `AnnounceNLRIBatch` | `tag-plugin-api.ci` |
| Plugin: `WithdrawTag("X", "Y")` | → | registry.WithdrawTag → `WithdrawNLRIBatch` | `tag-plugin-api.ci` |
| firewall rule tagged export added (D2) | → | `flowspec-egress` → registry.Announce → `AnnounceNLRIBatch` | `flowspec-egress.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `announce unicast <sel> 192.0.2.0/24 next-hop self community no-export tag mitigation maint` | Peer receives UPDATE for 192.0.2.0/24 with no-export; registry has entry under tag mitigation=maint |
| AC-2 | `announce blackhole <sel> 192.0.2.1/32 tag mitigation ddos-victim` | Peer receives UPDATE for 192.0.2.1/32 with BLACKHOLE community; entry tagged |
| AC-3 | `announce flowspec <sel> match {destination 198.51.100.0/24 protocol udp} then discard tag mitigation ddos-udp` | Peer receives FlowSpec NLRI with matching components and discard traffic-action ext-community |
| AC-4 | `withdraw tag mitigation ddos-udp` | All entries matching tag mitigation=ddos-udp withdrawn; peer receives withdrawals; entries removed |
| AC-5 | `withdraw tag mitigation *` | All entries under tag key "mitigation" withdrawn in bulk |
| AC-6 | `withdraw tag *` | All tagged entries across all keys withdrawn |
| AC-7 | `withdraw id 17` | Single entry withdrawn by ID; peer receives withdrawal |
| AC-8 | `withdraw all` or `withdraw all selector upstream` | All entries withdrawn (optionally filtered by selector) |
| AC-9 | `announce ... for 300s` | The announcement is auto-withdrawn after 300s (timer scheduled; cancellable by explicit withdraw) |
| AC-10 | `show announcements [tag <key>]` after one or more announces | Lists active entries with ID, tag key+value, family, NLRI, selector, age, expiry countdown |
| AC-11 | Plugin calls `UpdateRoute(sel, cmd, Meta: {"tag.myplugin": "group1"})` | Route is announced AND tagged; output includes route IDs |
| AC-12 | Plugin calls `WithdrawTag("myplugin", "group1")` | All routes under that tag withdrawn in bulk |
| AC-13 | invalid match (e.g. bad prefix, unknown family) | The verb rejects with a clear error; no partial UPDATE sent; no registry entry created |
| AC-14 (D2) | A firewall rule tagged `export-flowspec` is added | `flowspec-egress` announces the equivalent FlowSpec to configured upstream with tag; removing the rule withdraws it |
| AC-15 (D2) | A FlowSpec rule received from a peer (flowspec-firewall) | Is NOT re-exported by flowspec-egress (no loop) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | maintenance: `announce unicast upstream ... tag mitigation maint for 3600s` | verb → NLRI+community → registry(mitigation=maint) → AnnounceNLRIBatch → peer | `unicast-announce.ci` |
| 2 | under attack: `announce blackhole upstream 192.0.2.1/32 tag mitigation ddos-victim` | verb → NLRI+BLACKHOLE → registry → announce → peer | `rtbh-announce.ci` + interop |
| 3 | `announce flowspec upstream match {...} then rate-limit 1m for 600s tag mitigation ddos-udp` | verb → FlowSpec NLRI+action → registry → announce → auto-withdraw | `flowspec-announce.ci` + timer test |
| 4 | attack ends: `withdraw tag mitigation ddos-udp` | registry.WithdrawTag → WithdrawNLRIBatch → peer receives withdrawal | `announce-withdraw.ci` |
| 5 | full cleanup: `withdraw tag mitigation *` | registry.WithdrawTagKey → all mitigation entries withdrawn | `announce-withdraw.ci` |
| 6 | `show announcements tag mitigation` | registry.List → formatted table with ID, tag, family, age, expiry | `announce-show.ci` |
| 7 | plugin: `UpdateRoute(sel, cmd, Meta:{"tag.myplugin":"g1"})` | SDK → dispatch → registry(myplugin=g1) → AnnounceNLRIBatch | `tag-plugin-api.ci` |
| 8 | plugin: `WithdrawTag("myplugin", "g1")` | SDK → registry.WithdrawTag → WithdrawNLRIBatch → peers | `tag-plugin-api.ci` |
| 9 | tags a local firewall rule for export (D2) | firewall event → flowspec-egress → registry → announce | `flowspec-egress.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAnnounceBuildUnicastBatch` | `internal/component/bgp/plugins/cmd/announce/announce_test.go` | unicast NLRI + communities batch (AC-1) | |
| `TestAnnounceBuildBlackholeBatch` | same | unicast NLRI + BLACKHOLE community batch (AC-2) | |
| `TestAnnounceBuildFlowspecBatch` | same | FlowSpec NLRI + discard/rate-limit ext-comm (AC-3) | |
| `TestRegistryAnnounceCreatesEntry` | `internal/component/bgp/plugins/cmd/announce/registry_test.go` | Announce creates entry under tag key+value, calls AnnounceNLRIBatch, returns ID | |
| `TestRegistryWithdrawTag` | same | WithdrawTag(key, val) removes matching entries, calls WithdrawNLRIBatch (AC-4) | |
| `TestRegistryWithdrawTagKey` | same | WithdrawTagKey(key) removes ALL entries under key (AC-5) | |
| `TestRegistryWithdrawAllTags` | same | WithdrawAllTags removes all tagged entries across all keys (AC-6) | |
| `TestRegistryWithdrawEntry` | same | WithdrawEntry removes single entry by ID (AC-7) | |
| `TestRegistryWithdrawAll` | same | WithdrawAll removes all entries (AC-8) | |
| `TestRegistryWithdrawAllWithSelector` | same | WithdrawAll filtered by peer selector | |
| `TestRegistryList` | same | List returns entries filterable by tag key/selector/family (AC-10) | |
| `TestRegistryDurationAutoWithdraw` | same | `for <duration>` schedules + cancels auto-withdraw (AC-9) | |
| `TestRegistryMultipleEntriesSameTag` | same | multiple entries under same tag key+value (additive) | |
| `TestAnnounceInvalidInputRejected` | `internal/component/bgp/plugins/cmd/announce/announce_test.go` | bad prefix/family/port rejected, no entry (AC-13) | |
| `TestUpdateRouteWithTagMeta` | `internal/component/plugin/server/dispatch_test.go` | UpdateRoute with tag.* Meta key → registry entry created, RouteIDs returned (AC-11) | |
| `TestWithdrawTagRPC` | same | withdraw-tag RPC → registry.WithdrawTag called (AC-12) | |
| `TestFlowspecEgressNoReexport` | `internal/plugins/flowspec-egress/engine_test.go` | received FlowSpec is not re-announced (AC-15) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefix length v4 | 0-32 | 32 | N/A | 33 |
| prefix length v6 | 0-128 | 128 | N/A | 129 |
| flowspec port | 0-65535 | 65535 | N/A | 65536 |
| duration seconds | 1-… | n/a | 0 (reject / treat as no-timer) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `unicast-announce` | `test/plugin/unicast-announce.ci` | operator announces a unicast prefix with communities | |
| `rtbh-announce` | `test/plugin/rtbh-announce.ci` | operator announces a blackhole /32 mid-session | |
| `flowspec-announce` | `test/plugin/flowspec-announce.ci` | operator announces + withdraws a FlowSpec rule | |
| `flowspec-announce-duration` | `test/plugin/flowspec-announce-duration.ci` | `for <duration>` auto-withdraws | |
| `announce-withdraw` | `test/plugin/announce-withdraw.ci` | withdraw by tag key+value, by tag key *, by ID, and bulk withdraw all | |
| `announce-show` | `test/plugin/announce-show.ci` | show announcements lists active entries with detail | |
| `tag-plugin-api` | `test/plugin/tag-plugin-api.ci` | plugin UpdateRoute with tag.* Meta + WithdrawTag lifecycle | |
| `flowspec-egress` | `test/plugin/flowspec-egress.ci` | firewall-tagged rule announced (D2) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-unicast-originate-peer` | `test/interop/scenarios/` | ExaBGP / GoBGP / BIRD | a real peer accepts the originated unicast prefix + communities | |
| `NN-flowspec-originate-peer` | `test/interop/scenarios/` | ExaBGP / GoBGP / BIRD | a real peer accepts the originated FlowSpec NLRI + action | |
| `NN-rtbh-originate-peer` | `test/interop/scenarios/` | ExaBGP / GoBGP | a real peer receives the /32 with BLACKHOLE community | |

## BGP Family Gate
Evaluated per `ai/patterns/bgp-family.md`. **N/A** — no new SAFI, capability, or attribute.

| BGP Integration Point | Needed? | Evidence |
|----------------------|---------|----------|
| New SAFI / family register | No | SAFI 133/134 already registered; `flowspec/register.go` families exist |
| New NLRI struct/codec | No | reuse `plugins/nlri/flowspec/encode.go` |
| New capability | No | FlowSpec capability already negotiated |
| New attribute/community | No | BLACKHOLE `community.go:99`; traffic-action ext-comm already encoded |
| ExaBGP bridge family | No | FlowSpec bridge already exists |

## Files to Modify
- `pkg/plugin/rpc/types.go` - add `RouteIDs` field to `UpdateRouteOutput` (no input changes, uses existing Meta)
- `pkg/plugin/sdk/sdk_engine.go` - add `WithdrawTag`, `ListTag` methods
- `internal/component/plugin/server/dispatch.go` - recognize `tag.*` meta keys, route to registry; add withdraw-tag + list-tag RPCs
- `internal/component/bgp/plugins/cmd/update/update_text.go` - `DispatchNLRIGroups` conditionally uses registry when `tag.*` meta key present
- `docs/architecture/api/commands.md` - new announce/withdraw/show RPCs + plugin tag API
- `docs/guide/command-reference.md` - new operator verb
- `docs/features.md` - on-demand route origination row

## Files to Create
- `internal/component/bgp/plugins/cmd/announce/register.go` - RPC registration (mirror update_text.go:656)
- `internal/component/bgp/plugins/cmd/announce/announce.go` - parse args, build NLRIBatch per family
- `internal/component/bgp/plugins/cmd/announce/registry.go` - tag registry (in-memory, mutex-protected, tag key+value indexing, auto-withdraw timers)
- `internal/component/bgp/plugins/cmd/announce/registry_test.go` - registry unit tests
- `internal/component/bgp/plugins/cmd/announce/withdraw.go` - withdraw by tag key+value / tag key * / ID / all
- `internal/component/bgp/plugins/cmd/announce/show.go` - show announcements formatted output
- `internal/component/bgp/plugins/cmd/announce/yang/ze-announce-cmd.yang` - verb schema (family, selector, nlri, attributes, tag key value, duration)
- `internal/component/bgp/plugins/cmd/announce/announce_test.go` - batch building unit tests
- `test/plugin/unicast-announce.ci`, `test/plugin/rtbh-announce.ci`, `test/plugin/flowspec-announce.ci` - announce functional tests
- `test/plugin/flowspec-announce-duration.ci` - auto-withdraw functional test
- `test/plugin/announce-withdraw.ci` - withdraw by tag / id / all functional test
- `test/plugin/announce-show.ci` - show announcements functional test
- `test/plugin/tag-plugin-api.ci` - plugin API tag lifecycle functional test
- `test/interop/scenarios/NN-unicast-originate-peer/`, `NN-flowspec-originate-peer/`, `NN-rtbh-originate-peer/` - interop
- **(D2)** `internal/plugins/flowspec-egress/{register,engine,translate,model}.go` + `yang/` + `*_test.go` + `test/plugin/flowspec-egress.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | update_text.go + AnnounceNLRIBatch current state |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |

### Implementation Phases
1. **Phase: Tag registry (FIRST)** — registry.go with Announce/WithdrawTag/WithdrawTagKey/WithdrawEntry/WithdrawAll/List + unit tests; no CLI or API wiring yet.
2. **Phase: CLI wiring** — announce cmd plugin skeleton + YANG + RPC registration; registry integration; failing `unicast-announce.ci`.
3. **Phase: Unicast announce** — unicast NLRI + communities via registry with `tag <key> <value>`; AC-1.
4. **Phase: RTBH announce** — blackhole shorthand (unicast + BLACKHOLE community); AC-2.
5. **Phase: FlowSpec announce** — reuse encoder + action ext-comm; AC-3, AC-13.
6. **Phase: Withdraw + show** — `withdraw tag <key> <value|*>` / `withdraw tag *` / `withdraw id <N>` / `withdraw all` + `show announcements`; AC-4..AC-8, AC-10.
7. **Phase: Duration** — auto-withdraw timer; AC-9.
8. **Phase: Plugin API** — `tag.*` meta key recognition in dispatch, WithdrawTag/ListTag SDK methods, new RPCs; AC-11, AC-12.
9. **Phase: Interop** — originate to ExaBGP/GoBGP/BIRD; verify acceptance.
10. **Phase (D2): firewall→FlowSpec bridge** — new `flowspec-egress` plugin; export-tagged rules; loop guard; AC-14, AC-15.
11. **Full verification** → `make ze-verify-changed` + `make generate`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | each AC-N has file:line |
| Feature completeness | announce (unicast/blackhole/flowspec) + withdraw (tag key+val / tag key * / id / all) + duration + show + plugin API; D2 export + loop guard |
| Correctness | FlowSpec NLRI matches RFC 8955 components; BLACKHOLE 0xFFFF029A; unicast NLRI correct |
| Tagging | every tagged announce creates a registry entry; withdraw by tag removes matching entries; show lists all |
| Shared code path | CLI `tag <key> <value>` and plugin `Meta:{"tag.<key>":"<val>"}` both go through registry.Announce; no separate tracking |
| CLI grammar | action-before-identifier (`announce <family> ...`) |
| Buffer-first | UPDATE built via pooled WriteTo; no per-announce alloc |
| Rule: plugin-self-containment | D2 talks to reactor + registry via public API; no internal coupling |
| ExaBGP compat | ExaBGP migration (`internal/exabgp/migration/`) unchanged; YANG `watchdog` container unchanged; `tag` is a new keyword with no overlap |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Foot-gun | discard-all FlowSpec / over-broad selector; validate + warn |
| Loop | D2 never re-exports received FlowSpec (AC-15) |
| Input validation | prefixes, ports, durations, families, tag keys bounded; reject malformed before any UPDATE or registry entry |
| Scope leak | selector cannot leak routes to non-upstream sessions |
| Tag isolation | one plugin cannot withdraw another plugin's tag (source-scoped?) |

## Deliverables Checklist

| Deliverable | Verification Method | Status |
|-------------|-------------------|--------|
| Tag registry (Announce/WithdrawTag/WithdrawTagKey/WithdrawAllTags/WithdrawEntry/WithdrawAll/List) | `go test ./internal/component/bgp/plugins/cmd/announce/...` passes all registry tests | |
| CLI `announce` verb (unicast/blackhole/flowspec) | `unicast-announce.ci`, `rtbh-announce.ci`, `flowspec-announce.ci` pass | |
| CLI `withdraw tag/id/all` | `announce-withdraw.ci` passes | |
| CLI `show announcements` | `announce-show.ci` passes | |
| Auto-withdraw `for <duration>` | `flowspec-announce-duration.ci` + timer unit test pass | |
| Plugin API `tag.*` meta → registry | `tag-plugin-api.ci` passes | |
| Plugin SDK `WithdrawTag`/`ListTag` | unit test in `dispatch_test.go` | |
| D2 flowspec-egress plugin | `flowspec-egress.ci` passes | |
| Interop (unicast/flowspec/RTBH) | interop scenarios pass against ExaBGP/GoBGP/BIRD | |
| YANG schema | `ze-announce-cmd.yang` exists; `make generate` clean | |
| Documentation | docs updated per Documentation Update Checklist | |

## Documentation Update Checklist

| Category | Applies? | File | Update |
|----------|----------|------|--------|
| Feature list | | `docs/features.md` | |
| User guide | | `docs/guide/command-reference.md` | |
| Config syntax | | N/A (runtime only, no config) | |
| CLI reference | | `docs/guide/command-reference.md` | |
| API/RPC docs | | `docs/architecture/api/commands.md` | |
| Plugin SDK | | `docs/architecture/api/plugin-sdk.md` | |
| Wire format | | N/A (reuses existing UPDATE encoding) | |
| RFC compliance | | N/A (no new RFC implementation) | |
| Architecture design | | `docs/architecture/core-design.md` | |

## Goal Validation

| Goal (from Task) | Evidence | Status |
|------------------|----------|--------|
| Operator can originate any-family route on demand | AC-1..AC-3 demonstrated with .ci tests | |
| Routes are tracked via tags, withdrawable by tag/id/all | AC-4..AC-8 demonstrated with .ci tests | |
| Plugin API can opt into tagging via Meta | AC-11..AC-12 demonstrated with .ci test | |
| Auto-withdraw prevents stale announcements | AC-9 demonstrated with timer unit test + .ci test | |
| ExaBGP bridge compatibility preserved | ExaBGP migration code unchanged; grep shows no `tag` in exabgp paths | |
| D2 firewall→FlowSpec bridge works | AC-14..AC-15 demonstrated with .ci test | |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Audit
(Filled at closure.)

## Review Gate
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/bgp/plugins/cmd/announce`, `internal/plugins/flowspec-egress`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (ExaBGP/GoBGP/BIRD originate scenarios)

### Completion (BLOCKING — before ANY commit)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-4-flowspec-origination.md`
