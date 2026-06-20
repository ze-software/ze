# Spec: iface-resolve-2-resolver — shared logical-name resolver API

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-iface-resolve-0-umbrella, spec-iface-resolve-1-model |
| Phase | 1/4 (wiring) |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-iface-resolve-0-umbrella.md` - parent (resolver design A, consumer matrix)
3. `plan/spec-iface-resolve-1-model.md` - permaddr + os-name foundation this builds on
4. `internal/component/iface/dispatch.go`, `internal/component/iface/iface.go`

## Task

Provide the shared logical-name → device **resolver** in the `iface` component that all external
consumers call instead of resolving against the kernel themselves. The resolver returns a **value
snapshot** (never a `netlink.Link`), serves an **address list**, and delivers **link events** so
async consumers stop polling. It caches by logical name and invalidates on link events.

This sub-spec **provides the API only**; consumers are migrated in sub-specs 3-7. It also does NOT
re-route `iface`'s own internal netlink mutation ops — those legitimately need the full
`netlink.Link` object and stay inside the backend.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `plan/spec-iface-resolve-0-umbrella.md` - resolver design + consumer-needs matrix
  → Decision: chose A — resolver in `iface.dispatch`, cached, fed by the existing `Monitor`/`eventBus`.
  → Constraint: map-only; binding = os-name (default name) default, mac authoritative when present, match for matched kinds only.
- [ ] `plan/spec-iface-resolve-1-model.md` - permaddr + os-name
  → Constraint: `InterfaceInfo.PermanentMAC` and the os-name selector are provided by sub-spec 1; the resolver consumes them, does not re-add them.
- [ ] `ai/rules/plugin-design.md` - cross-boundary value types
  → Constraint: the resolver's public return types MUST be iface value types (no `netlink.Link`, no `*net.Interface` leak) so consumers don't couple to vishvananda/netlink.
- [ ] `ai/rules/go-standards.md` - Context, goroutine lifecycle
  → Constraint: `Subscribe` returns a channel with a cancel/Context; no leaked goroutines on unsubscribe.

**Key insights:**
- iface already has `Monitor`/`eventBus` (`dispatch.go:315`) emitting `TopicUp/Down/Created/Deleted/AddrAdded/AddrRemoved` — the event source for Subscribe + cache invalidation.
- External consumers need only value data (ifindex/mac/mtu/addresses/events); iface-internal needs `netlink.Link` and stays as-is.
- Async consumers today **poll every 5s** (LDP `waitForInterface`, iface-DHCP v4/v6) — replace with events.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/dispatch.go` (L218-345) - `GetInterface(name)`/`ListInterfaces()` do a live backend `LinkByName`; `Monitor` (L315) wraps `eventBus`; `NewMonitor`/`Start`/`Stop` exist. No `Resolve`/`Subscribe` API.
  → Constraint: extend this file with the resolver functions; reuse Monitor as the event feed.
- [ ] `internal/component/iface/iface.go` (L25-119) - event topic constants (`TopicUp`=L32, `TopicDown`=L34, `TopicCreated`=L28, `TopicDeleted`=L30, `TopicAddrAdded`=L36), `LinkPayload`/`StatePayload`/`AddrPayload`, and `InterfaceInfo` (+ `PermanentMAC` from sub-spec 1).
  → Constraint: Subscribe maps these topics to a per-name event stream.
- [ ] `internal/component/isis/transport/backend_linux.go` (L248-278) + `internal/component/pppoe/kernel_linux.go` (L92-125) - duplicated `resolveInterface` returning `(ifindex, hwaddr[6], mtu)`.
  → Constraint: the resolver `Binding` MUST expose exactly `{ifindex, operMAC, mtu}` so both wrappers can be deleted (in sub-specs 3/6).
- [ ] `internal/component/isis/circuits.go` (L323-447) - four address helpers extracting primary IPv4, IPv6 link-local, IPv6 non-link-local from `net.InterfaceByName().Addrs()`.
  → Constraint: `Addresses(name)` must return family + link-local scope so consumers filter v4 / v6-LL / v6-global.
- [ ] `internal/component/ldp/register.go` (L490-510) + `internal/plugins/iface/dhcp/v4/dhcp_v4_linux.go` (L25-31) - 5-second poll loops waiting for an interface to appear.
  → Constraint: `Subscribe` must fire an "appeared/up" event so these stop polling.
- [ ] `internal/plugins/iface/netlink/manage_linux.go` (L395-475) - mutation ops need the full `netlink.Link` (LinkSetMTU/Up/Down/HardwareAddr/AddrAdd/Modify) plus `GetStats`/QoS maps.
  → Constraint: these are iface-internal; they do NOT use the value resolver and stay on `netlink.Link`.

**Behavior to preserve:**
- `GetInterface`/`ListInterfaces` keep working (the resolver complements, not replaces, them).
- iface-internal mutation ops keep using `netlink.Link`.
- Address filtering semantics that IS-IS/LDP rely on (which addresses count) stay identical.

**Behavior to change:**
- New `Resolve` / `Addresses` / `Subscribe` API.
- Cache keyed by logical name, invalidated by Monitor events.

## Data Flow (MANDATORY)

### Entry Point
- A consumer holds a logical interface name and calls `iface.Resolve(name)` / `iface.Addresses(name)` / `iface.Subscribe(name)`.

### Transformation Path
1. `Resolve(name)` → cache lookup keyed by logical name → on miss, backend `LinkByName` (+ os-name/permaddr from sub-spec 1) → populate `Binding` → cache.
2. `Addresses(name)` → backend address list → classify family + link-local scope → `[]AddrInfo`.
3. `Subscribe(name)` → register against Monitor's `eventBus` topics → per-name `<-chan LinkEvent`.
4. Monitor `TopicUp/Down/Created/Deleted/AddrAdded` → invalidate/refresh the cache entry AND fan out to subscribers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Consumer ↔ iface | value `Binding` / `[]AddrInfo` / `<-chan LinkEvent` (no netlink leak) | [ ] |
| iface ↔ kernel | single `LinkByName`/addr owner inside the resolver | [ ] |
| Monitor ↔ resolver | eventBus topics drive invalidation + subscriber fan-out | [ ] |

### Integration Points
- Reuses `Monitor`/`eventBus` (`dispatch.go:315`) and `InterfaceInfo.PermanentMAC` (sub-spec 1).
- Output consumed by sub-specs 3-7; iface-internal mutation ops untouched.

### Architectural Verification
- [ ] No bypassed layers (consumers stop touching the kernel for name→device)
- [ ] No unintended coupling (no `netlink.Link` in exported API)
- [ ] No duplicated functionality (single cache + single kernel-name owner)

## Locked Consumer-Needs Matrix (re-derived from call sites)

| Consumer | ifindex | oper-MAC | MTU | addresses | events (async) | notes |
|----------|:---:|:---:|:---:|:---:|:---:|-------|
| isis transport | yes | yes | yes | - | - | socket bind; was ioctl wrapper |
| isis circuits | - | - | - | v4 primary, v6-LL, v6-global | up/down → circuit lifecycle | scope filtering needed |
| pppoe | yes | yes | yes | - | - | was ioctl wrapper (duplicate) |
| ldp | - | - | - | v4+v6 connected | **appeared/up (replaces 5s poll)** | per-session + all-ifaces |
| static / policyroute | yes | - | - | - | - | route nexthop |
| flowexport | yes | - | - | - | - | tc attach |
| ike | - | - | - | v4 first | - | tunnel local-addr |
| ppp ra/dhcpv6 | yes | - | - | - | - | mcast group join |
| iface-dhcp v4/v6 | existence | - | - | - | **appeared (replaces 5s poll)** | preflight |
| iface-internal | (uses `netlink.Link`) | | | | up/down | NOT a value-resolver consumer |

→ **Resolver return struct** = `Binding{Ifindex int; OsName string; OperMAC string; PermMAC string; MTU int; State string}`.
→ **`Addresses(name)` → []AddrInfo** (existing type: Address, PrefixLength, Family) + a link-local/scope flag so consumers filter.
→ **`Subscribe(name) → <-chan LinkEvent`** covering appeared/up/down (+ a cancel).

## Resolver API Design (DESIGN)

### Alternatives Considered
| # | Approach | Gains | Costs |
|---|----------|-------|-------|
| A (chosen) | Value `Binding` + `Addresses` + `Subscribe`, cached, Monitor-fed | no netlink leak; covers all external needs; events kill the 5s polls | cache invalidation complexity (R-6) |
| B | Return the `netlink.Link` / `*net.Interface` directly | trivial to implement | leaks vishvananda/netlink to every consumer; violates cross-boundary value types |
| C | Per-call live lookup, no cache | simplest, always fresh | LDP all-ifaces + isis hot paths re-hit netlink each call (R-1); no async events |

**Chosen: A.**

### Cache + Invalidation (the correctness core — R-6)
| Concern | Rule |
|---------|------|
| Key | logical name → cached `Binding` + ifindex |
| Freshness | ifindex is a **hint**; matched kinds re-validate on use; Monitor `TopicDeleted`/`TopicDown` invalidates the entry |
| Authority | full resync on link add/del events; never trust a cached ifindex past a `TopicDeleted` |
| Absent device | `Resolve` returns not-found; `Subscribe` fires when it appears (no busy-wait) |
| Concurrency | cache guarded by mutex; Subscribe channels buffered + dropped-on-full with resync |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Monitor emits enough events (up/down/created/deleted/addr) to keep the cache correct | `iface.go` topic constants L28-38 | resolver cache goes stale; need direct netlink subscribe | read Monitor impl `monitor_netlink_linux.go` | unvalidated |
| A-2 | `[]AddrInfo` + a scope flag reproduces IS-IS's exact v4/v6-LL/v6-global filtering | circuits.go L323-447 reads net.InterfaceByName().Addrs() | IS-IS adjacency/TLV regressions | port IS-IS filters onto Addresses() and diff | unvalidated |
| A-3 | Replacing 5s polls with events does not lose the "interface came up late" recovery | ldp/dhcp poll loops recover on late up | LDP/DHCP never start on a late interface | functional test: configure iface after engine start | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Cache staleness → wrong ifindex after rename/hw-swap/missed event | traffic to wrong iface; dead fd | ifindex is a hint; re-validate on use; full resync on link add/del |
| R-2 | Subscribe goroutine/channel leaks on unsubscribe | growing goroutine count | Context-cancel + close on unsubscribe; test with leak check |
| R-3 | Event storms (flap) thrash the cache | CPU/log spikes | debounce invalidation; coalesce resync |
| R-4 | `Addresses` scope flag misclassifies (e.g. deprecated/tentative v6) | IS-IS advertises wrong prefixes | mirror current filter logic exactly; unit test per family/scope |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| test harness calls `iface.Resolve("lo")` | → | resolver returns Binding{ifindex,...} | `internal/component/iface/resolve_test.go` |
| link up event after `Subscribe("x")` on absent iface | → | Subscribe fires "appeared" | `test/iface/iface-resolver-events.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `iface.Resolve(name)` on an existing interface | returns `Binding{Ifindex, OsName, OperMAC, PermMAC, MTU, State}`; not-found error for absent |
| AC-2 | `iface.Resolve` return shape | exposes exactly `{ifindex, operMAC, mtu}` that the IS-IS/PPPoE ioctl wrappers produced (so they can be deleted in 3/6) |
| AC-3 | `iface.Addresses(name)` | returns v4 + v6 addresses with family + link-local scope, reproducing IS-IS's v4/v6-LL/v6-global split |
| AC-4 | `iface.Subscribe(name)` on an absent interface, then it appears/goes up | subscriber receives an appeared/up event (no polling) |
| AC-5 | device removed (Monitor `TopicDeleted`) after a cached Resolve | cache invalidated; subsequent Resolve returns not-found, never a stale ifindex |
| AC-6 | public resolver API signature | returns iface value types only — no `netlink.Link` / `*net.Interface` in any exported signature |
| AC-7 | `Subscribe` then cancel | the backing goroutine/channel is released (no leak) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | (internal) a consumer resolves an interface to ifindex+mac+mtu | Resolve → Binding | `resolve_test.go` |
| 2 | configures an interface AFTER the engine starts | Subscribe → appeared event → consumer starts | `test/iface/iface-resolver-events.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveExisting` | `internal/component/iface/resolve_test.go` | Binding fields populated | |
| `TestResolveAbsent` | `internal/component/iface/resolve_test.go` | not-found, no stale entry | |
| `TestAddressesScopeSplit` | `internal/component/iface/resolve_test.go` | v4 / v6-LL / v6-global classification | |
| `TestSubscribeAppeared` | `internal/component/iface/resolve_test.go` | event on late appear/up | |
| `TestCacheInvalidatedOnDelete` | `internal/component/iface/resolve_test.go` | TopicDeleted invalidates | |
| `TestSubscribeNoLeak` | `internal/component/iface/resolve_test.go` | goroutine released on cancel | |
| `TestNoNetlinkLeakInAPI` | `internal/component/iface/resolve_test.go` | exported signatures use value types | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-resolver-events` | `test/iface/iface-resolver-events.ci` | interface configured after start triggers appeared event | |

### Interop Tests
N/A — internal resolver API; no wire-protocol behavior. (Proven end-to-end by IS-IS in sub-spec 3.)

## Files to Modify
- `internal/component/iface/dispatch.go` - export `Resolve`, `Addresses`, `Subscribe`
- `internal/component/iface/iface.go` - `Binding`, `LinkEvent` types (value types)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Resolver wired to Monitor events | [ ] | `dispatch.go` + `monitor_netlink_linux.go` |
| Functional test for events | [ ] | `test/iface/iface-resolver-events.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 8 | Plugin SDK/protocol changed? | [ ] | resolver is an internal API; doc in subsystem arch doc |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/` iface resolver section |

## Files to Create
- `internal/component/iface/resolve.go` - resolver: `Resolve`/`Addresses`/`Subscribe` + cache
- `internal/component/iface/resolve_test.go` - unit tests
- `test/iface/iface-resolver-events.ci` - late-appear event functional test

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add `Binding`/`LinkEvent` value types + `Resolve` stub + failing `resolve_test.go`
   - Tests: `TestResolveExisting`
   - Files: `iface.go`, `resolve.go`, `dispatch.go`
   - Verify: API exists, test fails on stub
2. **Phase: sync Resolve + Addresses** — implement Binding population + scope-aware address list
   - Tests: `TestResolveExisting/Absent`, `TestAddressesScopeSplit`, `TestNoNetlinkLeakInAPI`
   - Files: `resolve.go`
   - Verify: matches IS-IS/PPPoE wrapper output + scope split
3. **Phase: cache + invalidation** — cache by name; invalidate on Monitor events
   - Tests: `TestCacheInvalidatedOnDelete`
   - Files: `resolve.go`, Monitor wiring
   - Verify: no stale ifindex after delete
4. **Phase: Subscribe** — per-name event stream; replaces poll pattern
   - Tests: `TestSubscribeAppeared`, `TestSubscribeNoLeak`, `iface-resolver-events`
   - Files: `resolve.go`
   - Verify: late-appear fires; no goroutine leak
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit + learned summary; two commits

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-7 each have impl + test at file:line |
| Correctness | cache never returns stale ifindex; address scope split matches IS-IS |
| Data flow | resolver is the only new kernel-name owner; iface-internal still uses netlink.Link |
| No netlink leak | exported signatures use iface value types only (AC-6) |
| Goroutine lifecycle | Subscribe cancel releases resources (R-2) |
| Rule: no consumer migrated | grep shows isis/pppoe/ldp still on old path (migration is 3-7) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `Resolve`/`Addresses`/`Subscribe` | `grep -n 'func Resolve\|func Addresses\|func Subscribe' internal/component/iface/` |
| value-only API | `grep -n 'netlink\.\|net.Interface' internal/component/iface/resolve.go` returns no exported leak |
| event functional test | run `test/iface/iface-resolver-events.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | unbounded Subscribe channels / goroutines (R-2, R-3) |
| Input validation | name validated before kernel calls |

### Failure Routing
| Failure | Route To |
|---------|----------|
| stale-ifindex test fails | cache invalidation phase (R-1) |
| address scope mismatch | re-read circuits.go filters (A-2) |
| goroutine leak | Subscribe cancel path (R-2) |
| 3 fix attempts fail | STOP, report, ask user |

## Scope Merge (user-approved)

This unit MERGED sub-spec 3 (IS-IS migration) into sub-spec 2, because the repo
wiring gate (`ze-verify-wiring-docs`) rejects exported symbols with no production
caller, so the resolver API cannot land alone. The user chose "Merge IS-IS
migration in" and "Migrate IS-IS event path too". Delivered here: `Resolve` /
`Addresses` / `Subscribe` + cache + os-name translation; the `os-name` selector
config (un-hidden leaf + `parseIfaceEntry` + `osNameMap`); and the full IS-IS
migration (transport `resolveInterface` -> `iface.Resolve`; 5 address helpers ->
`iface.Addresses`; the transport link-event path -> per-circuit `iface.Subscribe`).

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Monitor emits a `TopicDeleted` event (AC-5 / R-6 wording) | RTM_DELLINK maps to `EventDown`, not a distinct deleted event; events live under `ifaceevents.Namespace` (`created`/`up`/`down`), not the `iface.Topic*` constants | Reading `monitor_linux.go` `handleLinkUpdate` (L204-213) | Cache invalidation hooks `EventDown` (covers delete); the resolver decodes `EventCreated`/`EventUp`/`EventDown` |
| Monitor delivers the typed Go payload struct | Monitor marshals to JSON and emits the **string**; in-process subscribers get a JSON string | Reading `monitor_linux.go` `emit` | `decodeLinkEvent` json-unmarshals the string into `{name,index}` |
| The `os-name` leaf is parsed into the runtime config | The leaf was `ze:hidden` and parsed by **nothing** (dormant, written only by `ze init`); `ifaceConfig`/`ifaceEntry` had no `OSName` field | Grepping config.go for os-name | Added `ifaceEntry.OSName` + `parseIfaceEntry` read + un-hid the YANG leaf so the selector is real |
| IS-IS integration tests would pass unchanged | `OpenCircuit`/address helpers now call `iface.Resolve`/`Addresses`, which need the iface backend loaded; the integration tests never loaded it | Tracing `startRealEngine` / transport `withVethPair` | Load `iface.LoadBackend("netlink")` in the integration helpers (mirrors production) |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Keep IS-IS event path on the raw EventBus, translate kernel->logical | Leaves `iface.Subscribe` unwired (wiring gate BLOCKER) and a kernel-name-keyed handler misses os-name-remapped circuits | Per-circuit `iface.Subscribe` with a reader goroutine into the existing rescan-backed lifecycle |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Value `Binding` return, never `netlink.Link` | return the link object | cross-boundary value-type rule; avoids coupling consumers to vishvananda/netlink |
| iface-internal mutation ops keep `netlink.Link` | force everything through the resolver | those ops need the full link; the resolver is for external value consumers only |
| Subscribe replaces 5s poll loops (LDP, DHCP) | keep polling | event-driven recovery; less latency + CPU |
| ifindex is a hint, re-validated on use | trust cached ifindex | prevents wrong-device/dead-fd after rename/hw-swap (R-1) |

## Known Limitations
- No consumer is migrated here (sub-specs 3-7).
- iface-internal netlink ops are out of scope (they correctly use `netlink.Link`).

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| shared resolver covering all external needs (value Binding, no netlink leak) | host unit tests | `resolve_test.go` AC-1..AC-7 + `TestResolveByOsName` (all PASS on darwin host) |
| logical name resolves to a DIFFERENT real kernel device (the core decoupling) | QEMU integration | `TestResolveRemapsLogicalNameToOSDevice` PASS (logical "uplink" -> real dummy `zeosdev0`) |
| address list reproduces IS-IS v4 / v6-LL / v6-global split on a real iface | QEMU integration | `TestAddressesRemapAndClassify` PASS |
| IS-IS references the logical name end-to-end (proof consumer) | QEMU integration | `TestISISAdjacencyUpVeth` + `TestISISTransportRawSocketCap` PASS, routed through `iface.Resolve` |
| IS-IS event path is logical-name aware (circuit open/close) | host unit | `TestISISTransportEventOpensAndCloses` + `TestISISTransportLateEnableSubscribes` PASS |
| os-name selector config surface | host unit + `.ci` | `TestParseIfaceEntryOSName`, `TestOSNameMapSkipsIdentityAndAbsent`; `test/isis/isis-logical-name.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | Resolver API would land with no production caller (wiring gate) | new exported symbols | fixed -- merged IS-IS migration so `Resolve`/`Addresses`/`Subscribe` all have IS-IS callers; `ze-verify-wiring-docs` "all references valid" |
| 2 | ISSUE | IS-IS integration tests resolve via `iface.Resolve` but never load the iface backend | adjacency/transport integration helpers | fixed -- `iface.LoadBackend("netlink")` in the shared helpers; QEMU PASS |
| 3 | ISSUE | Spec assumed a `TopicDeleted` event and typed payloads | resolve.go event handling | fixed -- invalidate on `EventDown`, decode JSON-string payloads |
| 4 | NOTE | Pre-existing `docs/DESIGN.md` plugin/family drift blocks the gate (not from this diff) | docs/DESIGN.md | fixed incidentally to keep the gate green |

### Run 2 (/ze-review)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | os-name-remapped *event* path not covered end-to-end (pieces unit-tested) | resolve.go bindEvents | fixed -- added `TestBindEventsDeliversRemappedEvent` (bus -> JSON decode -> remap -> subscriber + cache invalidation) |
| 2 | NOTE | os-name silently ignored on non-ethernet kinds | osNameMap | fixed -- documented ethernet-only scope in configuration.md |
| 3 | NOTE | Pre-existing HandleLinkUp/Down disable-race; pre-existing `ze-validate` unwired transport/iface symbols surfaced by touching the files | transport | acknowledged -- pre-existing, not introduced by this diff |

### Final status
- [x] `/ze-review` pass: 0 BLOCKER, 0 ISSUE in this diff; 2 actionable NOTEs fixed, pre-existing items acknowledged
- [x] IS-IS filter semantics preserved (QEMU adjacency PASS), no goroutine leak (Close teardown + reader-goroutine channel-close), empty-MAC / absent-device / os-name-identity handled
- [x] test-relaxation audit: 1 documented relaxation (transport_test.go) confirmed valid (event source moved EventBus -> resolver; replaced coverage)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
