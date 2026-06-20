# Spec: iface-resolve-0-umbrella — Interface logical-name resolution

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | 0 (umbrella) |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/523-iface-mac-discovery.md` - the model decision this spec completes
4. `internal/component/iface/yang/ze-iface-conf.yang`, `internal/component/iface/dispatch.go`

## Task

Decouple the Ze logical interface **name** from the OS/kernel device name at **runtime**.

The data model for this already exists (decided in learned 523): the interface `name` is a
human-meaningful logical key, the **MAC is the physical binding** to a device, and a hidden
`os-name` leaf preserves the original OS name. The gap is that **nothing at runtime honors that
model** — all interface consumers resolve the configured name straight against the kernel
(`netlink.LinkByName(name)` / `net.InterfaceByName(name)` / `SIOCGIFINDEX`), forcing
`name == os-name`. This spec introduces a shared **logical-name → device resolver** in the
`iface` component, reads the **permanent MAC** so a MAC override does not erase the binding, and
migrates every consumer onto the resolver.

This is the **umbrella**. Scope is delivered as numbered sub-specs (see Sub-Spec Decomposition).
Binding is **map-only**: Ze never renames the kernel device; it maps logical → OS internally and
shows both. OSPF (specced concurrently) consumes the new resolver via its own specs and is **not**
migrated here.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `plan/learned/523-iface-mac-discovery.md` - the model decision this spec completes
  → Decision: descriptive names are YANG keys; **MAC is the physical binding** chosen over MAC-as-key/IP-as-key, because names are human-meaningful and survive hardware replacement by updating the MAC.
  → Decision: `os-name` hidden leaf preserves the original OS name after the user renames the config entry, as a stable reference for internal tools mapping config → OS name.
  → Constraint: gotcha — linter hooks between sessions have **reverted** YANG `unique`/`ze:required` on the MAC binding; current YANG must be re-verified, the enforcement may have regressed.
  → Constraint: `link.Type()` returns `device` for both ethernet and loopback on Linux; loopback is detected by name (`lo`) before the ethernet case.
- [ ] `plan/learned/489-iface-0-umbrella.md` … `494-iface-5-vm-tests.md` - the original iface umbrella; structure and backend split precedent
  → Constraint: [fill in RESEARCH]
- [ ] `plan/learned/526-iface-backend-split.md`, `718-iface-3-unit-naming.md` - backend abstraction + unit (VLAN) naming model
  → Constraint: [fill in RESEARCH]
- [ ] `ai/rules/plugin-self-containment.md`, `ai/rules/config-naming.md` - resolver placement + leaf naming
  → Constraint: [fill in RESEARCH]

### RFC / Prior Art
- [ ] systemd.link / udev model - `[Match] PermanentMACAddress/Path/OriginalName` vs `[Link] Name/MACAddress`
  → Constraint: permanent MAC (kernel `IFLA_PERM_ADDRESS`) is the stable match key; the operational MAC is separately settable. Match-by-MAC applies to physical NICs only; virtual/created kinds are identified by assigned name.

**Key insights:** (minimal context to resume after compaction)
- The identity model is already decided (523); this spec is **runtime wiring + permaddr + enforcement restore**, not a new model.
- 54 direct kernel-resolution sites across 19 components (~11 non-trivial); no shared resolver; IS-IS and PPPoE duplicate the same ioctl wrapper.
- Map-only (no kernel rename). OSPF consumes the resolver via its own specs.

## Current Behavior (MANDATORY)

**Source files read:** (from this session's investigation)
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - all interface kinds keyed by `name`; `os-name` hidden leaf (L32-36, "Original OS interface name at discovery time"); `mac` container in `interface-l2` (L54-64) is an **optional override** ("when omitted, the kernel assigns a MAC") with **no `unique`/`ze:required`** — the binding enforcement 523 describes is absent in the live grouping.
  → Constraint: ethernet + loopback are **matched** against pre-existing kernel devices; dummy/veth/bridge/tunnel/wireguard/xfrm/VLAN-units are **created by Ze** (`zeManageable`, `config_apply.go:139-147`). Match-by-MAC/path only makes sense for matched kinds.
- [ ] `internal/component/iface/dispatch.go` - public package API; every op takes `name string` (`SetMTU`, `SetMACAddress`, `GetInterface`, `AddRoute`, …) and resolves it in the backend. `GetInterface(name)` (L233) / `ListInterfaces()` (L218) delegate to a **live `LinkByName`** — no registry, no ifindex cache, assumes `name == os-name`. A `Monitor` (L315) with an `eventBus` already exists — the link up/down event source the resolver can build on.
- [ ] `internal/plugins/iface/netlink/manage_linux.go` - `SetMACAddress` (L433-449, `LinkSetHardwareAddr`) and `GetMACAddress` (L451-464, reads only current `Attrs().HardwareAddr`). **No `IFLA_PERM_ADDRESS` / `ETHTOOL_GPERMADDR` anywhere in the tree.**
- [ ] `internal/component/isis/transport/backend_linux.go:248-278` + `internal/component/pppoe/kernel_linux.go:92-125` - **duplicated** `resolveInterface` ioctl wrappers (`SIOCGIFINDEX/HWADDR/MTU`).

**Behavior to preserve:**
- Existing configs where `name == os-name` MUST resolve unchanged (the default match is os-name == name).
- The `mac` override capability (set operational MAC) stays.
- The create-vs-match split per kind stays.

**Behavior to change:**
- Runtime resolution of a configured interface name goes through a shared resolver, not direct kernel lookups.
- Permanent MAC is read and stored as operational state.
- The MAC-as-binding enforcement is restored (subject to RESEARCH confirming it regressed).

## Data Flow (MANDATORY)

### Entry Point
- Config: `interface <logical-name> { os-name … | mac { address … } }` → Tree → `map[string]any` → iface model.
- Consumers (isis, routing, …) hold a logical interface name from their own config.

### Transformation Path
1. `ze init` / config discovery captures `os-name` + MAC for matched kinds (exists today, 523).
2. **[GAP today]** consumer takes the logical name and calls the kernel directly → forces name==os-name.
3. **[proposed]** consumer calls the iface **resolver**(logical-name) → `{ifindex, os-name, permaddr, current-mac, addresses, up/down}`.
4. Resolver maintains the mapping from netlink (match by os-name default, or permaddr/path), updates on link events.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Consumer ↔ iface | resolver API call (replaces direct kernel lookup) | [ ] |
| iface ↔ kernel | netlink (single owner of LinkByName/permaddr reads) | [ ] |

### Integration Points
- `iface.GetInterface` dispatch is the seam to extend into the resolver; `Monitor`/`eventBus` feeds link events.
- IS-IS `circuits.go` / `transport` and PPPoE `kernel_linux.go` are the first to drop their own wrappers.

### Architectural Verification
- [ ] No bypassed layers (consumers no longer touch the kernel for name→device)
- [ ] No unintended coupling (resolver is the single owner)
- [ ] No duplicated functionality (the two ioctl wrappers collapse into the resolver)

## Sub-Spec Decomposition

<!-- Umbrella core. Each sub-spec is a separate /ze-spec with its own ACs/tests. -->
| # | Sub-spec | Scope | Migrates |
|---|----------|-------|----------|
| 1 | `iface-resolve-1-model` | Restore/define the binding model: MAC binding enforcement (`unique`/required where right), promote `os-name` to a real match selector, optional `path` selector; read `IFLA_PERM_ADDRESS` + store as state; define match precedence + absent-at-boot (defer) / ambiguous (reject) semantics; `show interface` shows logical + os-name + permaddr | schema + state |
| 2 | `iface-resolve-2-resolver` | Shared resolver API in `iface` (name → ifindex/os-name/permaddr/current-mac/addresses/up-down); link up/down events + address query; collapse the two duplicated ioctl wrappers | iface dispatch |
| 3 | `iface-resolve-3-isis` | Migrate IS-IS (`transport/backend_linux.go` resolveInterface, `circuits.go` x4) — **proof consumer** | isis |
| 4 | `iface-resolve-4-routing` | static routes, policyroute, flowexport (next-hop / tc by name) | static, policyroute, flowexport |
| 5 | `iface-resolve-5-iface-internal` | iface plugin's own ops (manage/bridge/show/xfrm/tunnel/mirror ~22 sites), iface/dhcp v4+v6, provision | iface-netlink, dhcp, provision |
| 6 | `iface-resolve-6-protocols` | pppoe (drop duplicate wrapper), ldp (async discovery), ike, ppp | pppoe, ldp, ike, ppp |
| 7 | `iface-resolve-7-peripheral-guard` | doctor, imageserver, diag, traffic; add a checks guard that rejects new direct name→kernel resolution | peripheral + guard |

## Resolver Design (DESIGN)

### Alternatives Considered
| # | Approach | Gains | Costs |
|---|----------|-------|-------|
| A (chosen) | Extend `iface.dispatch` with resolver functions backed by a **cached registry fed by the existing `Monitor`/`eventBus`** | reuses the existing seam + event source; iface already owns interface knowledge; minimal new surface (3 calls); uniform with current dispatch | iface package grows; consumers depend on iface (acceptable — iface is infrastructure) |
| B | New standalone resolver service in a separate package that everything (incl. iface) depends on | clean separation; iface becomes a plain consumer | another component; duplicates ownership iface already has; more wiring |
| C | Per-consumer thin helper: translate logical→os-name then call existing kernel lookups | smallest change; no cache/events | does NOT solve deferred binding (LDP) or permaddr match; still N live lookups — fails the goal |

**Chosen: A.** Single owner (iface), single event source (Monitor), three-call API. Collapses the two
duplicated ioctl wrappers (isis, pppoe) into the uniform path.

### Resolver Consumer-Needs Matrix (locks the sub-spec 2 API)
| Consumer | sync resolve (ifindex) | mac / mtu | address list | async appear / up-down | permaddr match |
|----------|:---:|:---:|:---:|:---:|:---:|
| isis | yes (circuit open) | yes | yes (v4+v6) | yes (link → circuit lifecycle) | matched kind |
| ldp | yes | - | yes | **yes (wait for iface to appear)** | - |
| static / policyroute | yes (route apply) | - | - | - | - |
| flowexport | yes (tc attach) | - | - | - | - |
| iface-internal | yes (every op) | yes | yes | - | yes (discovery) |
| pppoe | yes | yes | - | - | matched kind |
| ike / ppp | yes | - | yes | - | - |
| dhcp v4/v6 | existence check | - | - | maybe | - |

→ Derived API surface (sub-spec 2): `Resolve(name) → {ifindex, osName, permaddr, mac, mtu}`,
`Addresses(name) → []addr`, `Subscribe(name) → <-chan LinkEvent`. Caching keyed by logical name,
invalidated by Monitor link events.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | MAC binding is `unique` but NOT `ze:required` — and that is the correct state (optional + unique preserves backward compat) | live YANG has `unique "mac/address"` on ethernet/veth/bridge (L505/536/556); no `ze:required` anywhere | sub-spec 1 keeps mac optional+unique; does NOT make it required; os-name (default name) is the default binding, mac is authoritative when present | grep confirmed unique present, ze:required absent | confirmed |
| A-2 | Kernel exposes a stable permanent MAC for matched physical NICs via `IFLA_PERM_ADDRESS` (netlink) | systemd.link uses it; standard since Linux 3.x | match-by-permaddr unimplementable; fall back to os-name/path only | netlink readback on a NIC in QEMU | unvalidated |
| A-3 | Default match (os-name == name) makes every existing config resolve unchanged | user requirement; 523 keeps os-name | backward-compat break across all consumers | run existing isis/iface .ci with unchanged configs | unvalidated |
| A-4 | OSPF can be written to consume the resolver via its own specs without this spec touching OSPF | user decision; OSPF specced not yet wired | OSPF/this effort conflict on iface seam | inspect spec-ospf-5 status | unvalidated |
| A-5 | The 54 sites all resolve a *config-sourced* name (not a kernel-enumerated one that should stay direct) | blast-radius agent: 48/54 from config | some sites must stay direct; migration list shrinks | per-site review in each sub-spec | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Resolver becomes a hot-path bottleneck (LDP/ISIS resolve frequently); 523 notes live netlink I/O per call | latency in adjacency/circuit setup | cache ifindex keyed by logical name, invalidate on link event |
| R-2 | Created kinds (veth/bridge/tunnel) have no permaddr; match logic mis-branches | create fails or wrong device matched | branch on `zeManageable`; MAC/path match for matched kinds only |
| R-3 | Big migration lands half-done, leaving mixed direct + resolver paths | grep guard finds residual direct lookups | sub-spec 7 guard check blocks new direct lookups; per-sub-spec completion |
| R-4 | Absent-at-boot device (deferred binding) leaves a consumer waiting forever | consumer never forms adjacency | resolver emits up/down events; consumer subscribes, no busy-wait |
| R-5 | Concurrent linter-hook YANG reversion (523 gotcha) silently drops new `unique`/`required` | YANG re-verify fails post-session | re-verify YANG after any concurrent activity; add to sub-spec 1 checklist |
| R-6 | **Cache staleness** — cached logical→ifindex goes stale on rename/hw-swap/missed event; stale ifindex routes to wrong device or closed fd | traffic to wrong iface; circuit on dead fd | ifindex is a hint, re-validate on use for matched kinds; Monitor handling authoritative (full resync on RTM_NEWLINK/DELLINK); this is sub-spec 2's correctness core |

## Wiring Test (MANDATORY)

<!-- Umbrella-level cross-cutting wiring; detailed per-consumer wiring lives in sub-specs. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `interface uplink { os-name eth0 }` + `isis { interface uplink {} }` | → | iface resolver → isis circuit open on eth0 | `test/isis/isis-logical-name.ci` (sub-spec 3) |
| `show interface` | → | resolver state (logical + os-name + permaddr) | `test/iface/iface-show-mapping.ci` (sub-spec 1) |

## Acceptance Criteria

<!-- Cross-cutting invariants. Detailed ACs live in sub-specs. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-U1 | grep the tree for config-sourced `LinkByName`/`InterfaceByName`/`SIOCGIFINDEX` outside the iface resolver | none remain (guard check passes) |
| AC-U2 | `interface uplink { os-name eth0 }`, IS-IS enabled on `uplink` | IS-IS opens its circuit on kernel `eth0`; adjacency forms |
| AC-U3 | NIC with `mac { address … }` override configured | permanent MAC still read + exposed; binding survives the override |
| AC-U4 | Existing config with `name == os-name`, no os-name/mac selector | resolves exactly as today (backward compat) |
| AC-U5 | `show interface` | displays logical name, os-name, and permanent MAC |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | names a NIC `uplink` bound to kernel `eth0`, runs IS-IS on it | config → iface resolver → isis circuit on eth0 | [sub-spec 3] |
| 2 | overrides a NIC's MAC but keeps it identifiable | config mac override → permaddr read → resolver binding stable | [sub-spec 1] |
| 3 | `show interface` to see logical↔OS mapping | resolver state → show | [sub-spec 1] |

## 🧪 TDD Test Plan
<!-- Detailed plans live in sub-specs; umbrella tracks the cross-cutting proofs. -->
### Unit Tests
<!-- Umbrella-level; exhaustive unit tables live in each sub-spec. -->
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveByOsName` | `internal/component/iface/resolve_test.go` (sub-spec 2) | logical name → ifindex via os-name default | |
| `TestResolveByPermaddr` | `internal/component/iface/resolve_test.go` (sub-spec 2) | physical NIC matched by permanent MAC | |
| `TestPermaddrSurvivesMacOverride` | `internal/plugins/iface/netlink/*_test.go` (sub-spec 1) | permaddr read stays stable after mac override | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| logical-name isis adjacency | `test/isis/*.ci` (sub-spec 3) | interface named != os-name forms adjacency | |
| show interface mapping | `test/iface/*.ci` (sub-spec 1) | logical + os-name + permaddr shown | |
| no-direct-resolution guard | `scripts/checks/` (sub-spec 7) | new direct kernel lookups rejected | |

## Files to Modify
<!-- Umbrella lists the seams; sub-specs enumerate exact files. -->
- `internal/component/iface/yang/ze-iface-conf.yang` - binding model (sub-spec 1)
- `internal/component/iface/dispatch.go` - resolver API (sub-spec 2)
- `internal/plugins/iface/netlink/*` - permaddr read + single kernel owner (sub-specs 1,2,5)
- consumer packages per the decomposition table (sub-specs 3-7)

## Implementation Steps

This umbrella is implemented by its sub-specs, in dependency order: **1 → 2 → 3 → (4,5,6 parallel) → 7**.
Sub-spec 1 (model) and 2 (resolver) are prerequisites; 3 (IS-IS) proves the pattern before the
bulk migration; 7 adds the guard last so it does not block in-flight migrations.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Resolver lives in `iface.dispatch`, cached, fed by existing `Monitor`/`eventBus` | standalone resolver service; per-consumer thin translation | iface already owns interface knowledge + an event source; minimal new surface; thin translation fails LDP async + permaddr |
| Binding = os-name (default name) default; mac (+ future path) authoritative when present; match only for matched kinds | mac-required (523 literal); name-only | keeps every `interface eth0 {}` config working; honors "MAC as physical binding" when the operator opts in |
| Map-only, never rename the kernel device | systemd-style kernel rename | avoids boot races with udev; Ze's own show layer presents the logical name |
| Umbrella + 7 sub-specs, IS-IS as proof consumer before bulk | one monolithic spec | reviewable units; matches iface/ospf/ipsec/fib/vpp convention; de-risks API via early proof |
| Read permanent MAC (`IFLA_PERM_ADDRESS`); keep operational mac override separate | infer identity from current mac | override must not erase the binding (the user's core point) |
| OSPF consumes the resolver via its own specs, not migrated here | migrate OSPF as 12th consumer | OSPF is being specced now; avoids two efforts editing the same files |

## Known Limitations
- Map-only: the kernel device is never renamed (deliberate; avoids boot races with udev).
- OSPF is not migrated here; it consumes the resolver via its own specs.
- Bonds/teams that share a permanent MAC across members are out of scope for permaddr match (use os-name/path).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-U1..AC-U5 all demonstrated (across the sub-specs)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Every sub-spec (1..7) closed
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
