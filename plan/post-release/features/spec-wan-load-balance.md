# Spec: wan-load-balance

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | Multi-uplink load balancing with health checks is a capability VyOS offers and Ze lacks; nothing shipped is wrong without it, so it is post-release feature work. |
| RFC gaps | - |
| Depends | spec-policyroute-reload-make-before-break, spec-nat-load-balance |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator with two or more internet uplinks (DSL, fibre, LTE) wants new
outbound connections spread over the healthy ones by weight, a dead uplink taken
out of rotation within seconds and put back when it recovers, each connection
kept on the uplink it started on, and its source NATed to that uplink's address.
VyOS calls this WAN load balancing. Ze has policy routing (mark a packet, route
it by an ip rule) but no weighted choice and no uplink health.

Ze gains WAN load-balancing rules on top of policy routing: per rule, the
uplinks and their weights, and per uplink a health test (ping, TTL-limited
probe, or a user script) with a target, interval, and failure and success
counts.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - one writer per data plane, nft table ownership, `ip rule` through `internal/core/iprule`
- [ ] `ai/doors/core.md` - a shared health FSM in `internal/core/` imports nothing upward
- [ ] `docs/architecture/policyroute/policy-routing.md` - marks 0x50000 to 0x5FFFF, `ze_pr` table, ip rules
- [ ] `docs/architecture/bgp/healthcheck-plugin.md` - the existing rise and fall health FSM
- [ ] `docs/guide/policy-routing.md` - the operator surface this extends

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/policyroute/translate.go` `translate` - one prerouting filter chain, priority -150, in table `ze_pr`; each rule's mark and ip rule.
- [ ] `internal/plugins/policyroute/marks.go` `allocator` - marks and tables allocated in sequence; `reset` restarts both counters.
- [ ] `internal/plugins/policyroute/register.go` `applyPolicies` - on every apply: `alloc.reset()`, translate, remove ALL old ip rules and auto routes, register the new tables, then install the new rules. Marks are renumbered from config order on every reload, and between the removal and the install no policy route exists.
- [ ] `internal/plugins/policyroute/rules_linux.go` `applyIPRules`, `applyAutoRoutes` - netlink ip rules and routes with `rtproto.PolicyRoute`.
- [ ] `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - `then` has accept, drop, table, next-hop, tcp-mss; no weighted choice, no health.

**Behavior to preserve:** every existing policy-routing rule.

**Behavior to change:** add weighted uplink selection with health, connection stickiness and per-uplink source NAT.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Policy routing | `internal/plugins/policyroute/` `translate`, `applyPolicies` | Marks at prerouting, routes by ip rule | Extends: a load-balance action whose mark is chosen per new connection by weight and stored in the conntrack mark |
| Mark and table allocator | `marks.go` `allocator` | Sequential marks, reset on every apply | Extends: uplink marks and tables become stable per uplink name for as long as it stays configured (AC-3) |
| NAT load-balance map | `plan/post-release/features/spec-nat-load-balance.md` | numgen or jhash map mod sum of weights | Extends: the same weighted map lowering, with marks as values instead of NAT targets |
| Health FSM | `docs/architecture/bgp/healthcheck-plugin.md` (bgp `healthcheck` plugin) | Rise and fall FSM over a probe, announces or withdraws a route | Extends: the FSM moves to a shared `internal/core/` package so the BGP plugin and this feature use one implementation, not two |
| Masquerade | firewall `masquerade` action | Source NAT to the outgoing interface address | Extends: per-uplink masquerade in a Ze-owned postrouting chain |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: load-balance rules (match plus uplinks and weights) and per-uplink health tests.

### Transformation Path
1. New connection at prerouting: if the conntrack mark is zero, choose an uplink by weight among the healthy ones, write the mark to the packet and the conntrack entry; later packets restore it.
2. ip rule on the mark selects the uplink's table, whose default route points at that uplink.
3. Postrouting masquerades by output interface.
4. A health worker probes each uplink; a state change rewrites only the weighted map and the affected routes, in one nft transaction.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → plugin | YANG, `OnConfigApply` | No |
| Plugin → firewall registry | `firewall.RegisterTables` (process-local; an RPC if the plugin can run forked) | No |
| Plugin → kernel routing | `internal/core/iprule`, netlink | No |

### Integration Points
- `policyroute` translate and apply; the firewall table registry; a shared health FSM

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| load-balance config with two uplinks | → | weighted mark, ip rules, health worker | `test/policy/wan-load-balance.ci` (proposed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Two healthy uplinks, weights 1 and 3 | New connections split about 25/75 in a namespace lab with one fake ISP per uplink, each ISP answering with the source address it sees; every connection keeps its uplink for its life | - |
| AC-2 | TTL health test against a target | Healthy when the hop where the TTL runs out answers, or when a closer target answers itself; failed when nothing answers within the response time (VyOS `bd8be7c14`: the test was inverted, a dead uplink stayed active) | - |
| AC-3 | An uplink added whose name sorts before the others | Existing uplinks keep their mark and table; the new one gets the lowest free ones; existing connections keep working (VyOS `2b97c4fec`: renumbering by sort order sent established flows into the wrong table) | - |
| AC-4 | Any commit touching load balancing or a firewall group | The health worker is reloaded, not restarted: health state survives, and rules, routes and ip rules are replaced make-before-break in one transaction, with no window where traffic is unbalanced or not NATed (VyOS `794320749`) | - |
| AC-5 | Per-packet balancing set on a rule | Every packet is balanced on its own, not only new connections (VyOS `e71bb8751`: the option was read from a key no config node produced and had no effect) | - |
| AC-6 | Rate limit with threshold above | The rule matches traffic exceeding the rate (VyOS `db494f512`: above was ignored) | - |
| AC-7 | Inbound interface any | Matches traffic entering on any interface except the uplinks themselves, so replies from an ISP are not re-marked back to it (VyOS `272b6e646`: rendered as an interface literally named any) | - |
| AC-8 | A rule naming an uplink with no health configuration | Refused at commit (VyOS `f1205b233`: silently left out of the rule) | - |
| AC-9 | A ping or TTL test with no target | Refused at commit (VyOS `6a4684ca9`: accepted, then the daemon crash-looped and balancing stopped with no error shown) | - |
| AC-10 | Protocol negation that matches nothing (VyOS `!all`) | Refused at commit (VyOS `ca5ab0a86`) | - |
| AC-11 | Uplink names with a dash | Used as written end to end (VyOS `a189b6b7e`: mangled to an underscore, never installed) | - |
| AC-12 | Health worker start | Every uplink's initial state is reported, not only the first (VyOS `f353f8533`) | - |
| AC-13 | Table removed from the kernel by someone else | The next apply rebuilds it regardless of what is loaded (VyOS `68efa5770`) | - |
| AC-14 | All uplinks down | Behavior defined at design (fall back to the main table, or drop) and reported by `ze doctor` with the failing targets | - |

## Risks & Assumptions

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `applyPolicies` today removes every ip rule before installing the new set, which conflicts with AC-4 | Traffic test during reload | The load-balance path is make-before-break from the start; the existing remove-then-add of ordinary policy routes is owned by `plan/release/fixes/spec-policyroute-reload-make-before-break.md`, which this spec depends on |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUplinkMarkStable` | `internal/plugins/policyroute/marks_test.go` | AC-3 | |
| `TestLoadBalanceVerify` | `internal/plugins/policyroute/config_test.go` | AC-8 to AC-10 | |
| `TestTTLProbeVerdict` | health package | AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `wan-load-balance` | `test/policy/wan-load-balance.ci` | AC-1, AC-4 in a namespace lab | |

## Implementation Steps

1. **Phase: Wiring** -- YANG, verify, failing `.ci`
2. **Phase: Shared health FSM** -- move, then reuse
3. **Phase: Weighted mark and stickiness** -- AC-1, AC-3, AC-5 to AC-7
4. **Phase: Reload and failure** -- AC-4, AC-12 to AC-14

## Files to Modify
- `internal/plugins/policyroute/translate.go`, `marks.go`, `register.go`, `rules_linux.go`
- `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`
- `docs/guide/policy-routing.md`

## Files to Create
- `test/policy/wan-load-balance.ci`

## VyOS reference

T9472 (merge `97c7054e9`, PR #5571, October 2026): `bd8be7c14` inverted TTL
test, `2b97c4fec` stable per-interface marks, `794320749` reload not restart,
`e71bb8751` per-packet balancing, `db494f512` limit threshold above, `272b6e646`
inbound any, `f1205b233` health required for rule interfaces, `6a4684ca9` target
required, `ca5ab0a86` protocol `!all`, `a189b6b7e` dashed names, `f353f8533`
initial state, `a2491b218` smoketest lab. T9474 `68efa5770` table rebuild.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Targeted tests pass: `./le test unit plugins`, `./le test policy`, `./le test integration firewall`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
