# Spec: home-cpe-1-logical-interface-name

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Accepted configuration does the wrong thing: `pppoe-client pppoe0` validates, yet a static route, a firewall match or a masquerade rule that names `pppoe0` never matches the session device, which the kernel calls `pppN`. The guide's own example (`docs/guide/static-routes.md`, `interface pppoe0`) does not work. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

All values in this spec are invented (RFC 5737 prefixes, `example.net`).

## Task

The `pppoe-client` list key (for example `pppoe0`) is a config-level name only.
The PPPoE dialer creates the PPP unit with PPPIOCNEWUNIT, so the kernel names
the device `ppp<unit>`, and the unit number can change on every reconnect. Every
consumer that names an interface (static routes, the FIB plugin, firewall and
NAT matches, DHCP server, TFTP, traffic shaping) either resolves the configured
name through `iface.Resolve`, which knows no mapping for a pppoe-client, or
takes it literally. None can name the session device by its configured name.

Goal: the configured name of a pppoe-client is the name of its kernel device,
on every session and every reconnect, so every consumer that names it reaches
the session; a static route through it survives the session's absence at boot
and across reconnects.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - the shared resolver
  → Decision: "`os-name` applies to matched kinds only ... Created kinds such as veth, bridge and tunnel are made by ze under the logical name, so aliasing them would break creation." A pppoe-client is a created kind
  → Constraint: no Ze code resolves a configured name straight against the kernel (`./le arch iface-resolution`, `internal/le/arch/ifaceresolution/`)
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md` - unit ownership: the unit stays open until `Cleanup` so `pppN` is not reused before the caller finishes
  → Constraint: a device named after the config key must be gone before the next session creates the same name
- [ ] `docs/architecture/static-routes.md` "An interface next hop is resolved TWICE"
  → Constraint: static resolves at config time and refuses an unresolvable name; the FIB plugin resolves again when it programs
- [ ] `docs/guide/firewall.md` (documents `internal/plugins/firewall/nft/lower_linux.go`)
- [ ] `docs/guide/static-routes.md` l.60-70 - `next { interface pppoe0 { } }` presented as working

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/pppoe_client.go` - `runSession` names the device `textbuf.StrInt("ppp", sess.UnitNum)` (l.254) for `SetMTU`, `SetAdminUp`, `AddAddressP2P`, `AddRoute`; `status` reports the same `pppN` (l.146)
- [ ] `internal/component/l2tp/pppoeclient/dialer.go` `Dial` - `ppp.DevPPPSetup(pppoxFD)` after PADS; the name is never passed down
- [ ] `internal/component/l2tp/ppp/devppp_linux.go` `DevPPPSetup` - PPPIOCGCHAN, opens /dev/ppp for the channel, PPPIOCATTCHAN, opens /dev/ppp for the unit, PPPIOCNEWUNIT with -1
- [ ] Linux `drivers/net/ppp/ppp_generic.c` (read from a local copy): `ppp_unit_register` names the device `ppp%i` only when `ifname_is_set` is false; `ppp_nl_newlink` (rtnetlink kind `ppp`, attribute `IFLA_PPP_DEV_FD` on an opened /dev/ppp file with no unit yet) sets `ifname_is_set` true and keeps the name the request carries; it returns EBUSY when `ppp_mutex` is contended (trylock to avoid lock inversion). `IFLA_PPP_DEV_FD` is in `vendor/golang.org/x/sys/unix/ztypes_linux.go`
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` l.1708 - `list pppoe-client`, description "The name is the list key and it never reaches the kernel"; `leaf name` is `type string` with no length or pattern
- [ ] `internal/component/iface/config.go` `parsePPPoEClientEntry` (l.977) - validates `source-interface` with `ValidateIfaceName`, not the key; `osNameMap` (l.68) and `permMACMap` cover ethernet only
- [ ] `internal/component/iface/resolve.go` - `osDeviceFor` maps a name through `osNames` (ethernet overrides) or uses it as is; `setResolverConfig` publishes `osNameMap` and `permMACMap`; cache invalidated by link events
- [ ] `internal/component/iface/config_apply.go` - dummy (l.914), veth (l.975), tunnel (l.1041, `e.Spec`), wireguard (l.1109), xfrm (l.1195, `e.Spec`), bridge (l.1225) are all created under the configured name: those kinds do not have this defect
- [ ] `internal/plugins/static/locrib.go` `checkNextHopResolvable` (l.204) - refuses the route when `iface.ResolveIndex` fails, so a static route through a PPPoE device that is not up (boot, reconnect) fails the commit
- [ ] `internal/plugins/static/backend_linux.go` `resolveNexthopIndex` and `internal/plugins/fib/kernel/nexthop_linux.go` (l.150, l.277) - resolve through `iface.ResolveIndex` when programming; neither the static plugin nor fib-kernel subscribes to link events (grep for `iface.Subscribe` and link events in both is empty), so a route the kernel removed with its device is not reinstalled when the device returns
- [ ] `internal/plugins/firewall/nft/lower_linux.go` `lowerIfaceMatch` (l.856), used for `MatchInputInterface` and `MatchOutputInterface` (l.493, l.500) - emits a Meta IIFNAME or OIFNAME compare of the configured name, literal or wildcard; no resolver call anywhere in `internal/component/firewall` or `internal/plugins/firewall`

**Behavior to preserve:**
- Every other interface kind's naming
- PPP unit ownership rules in `Dial` and `Cleanup`
- Static refuses at commit a name no configuration declares

**Behavior to change:**
- The pppoe-client device carries the configured name
- A static next-hop that names a declared pppoe-client is accepted while the device is absent, and is programmed whenever the device exists

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config `interface pppoe-client pppoe0 { ... }` plus `static ... next { interface pppoe0 }` and firewall `output-interface pppoe0`

### Transformation Path
1. Parse validates the key as a kernel interface name (`ValidateIfaceName`) and refuses a key equal to any other configured interface name
2. `reconcilePPPoEClients` passes `Name` in `PPPoEClientConfig` (already there)
3. `Dial` creates the unit through rtnetlink with that name and the unit descriptor, reads the unit number back for PPPIOCCONNECT; retries EBUSY, and EEXIST while the previous session's device is still being removed, bounded
4. `runSession` and `status` use the configured name for every backend call and the show output
5. iface publishes the declared pppoe-client names to the resolver as declared dynamic names
6. Static accepts a next-hop that names a declared dynamic interface while it is absent; the FIB path programs the route when a link event reports the device created or up, and the kernel's own removal on device loss stands
7. nft rules keep their literal name; a string match on `pppoe0` matches every session of that client and nothing else

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to kernel | RTM_NEWLINK kind `ppp` with `IFLA_PPP_DEV_FD` and `IFLA_IFNAME` | No |
| iface to static and fib plugins | `iface.Resolve` family plus link events through `iface.Subscribe` | No |

### Integration Points
- `iface.Resolve`, `iface.ResolveIndex`, `iface.Subscribe`
- `internal/plugins/static/locrib.go` `checkNextHopResolvable`
- `internal/plugins/fib/kernel` programming path
- Child 4 (`spec-home-cpe-4-ike-dynamic-wan.md`) binds IKE to the WAN by this name

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | consumers keep resolving through `iface.Resolve` |
| No unintended coupling | Yes | static and fib learn "declared but absent" from iface, not from the pppoe-client list |
| No duplicated functionality | Yes | no second mapping table; the name is the device |
| Zero-copy preserved where applicable | N-A | |
| Registration over hardcoding, outbound | N-A | |
| Registration over hardcoding, inbound | Yes | the declared-dynamic set is derived from the iface config the resolver already receives; searched `osNameMap`, `permMACMap`, `setResolverConfig`, `checkNextHopResolvable`, `lowerIfaceMatch` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze's kernel accepts RTM_NEWLINK kind `ppp` from Ze's user (CAP_NET_ADMIN) and the created unit behaves as a PPPIOCNEWUNIT one for PPPIOCCONNECT | `ppp_nl_newlink` in `ppp_generic.c` | fall back is not allowed (no layering); design returns to the owner | `TestDevPPPSetupNamesUnit` (Linux, user namespace) | unvalidated |
| A-2 | The netlink library in vendor can send a link of kind `ppp` with `IFLA_PPP_DEV_FD`, or a raw RTM_NEWLINK is encoded by Ze | vendor holds the constant only | small encoder in `internal/component/l2tp/ppp` | Phase 1 read of the vendored link types | unvalidated |
| A-3 | The iface monitor reports `created` and `up` for a ppp device | `resolve.go` invalidates on any link event | FIB never hears the device return | `TestFIBProgramsDeclaredInterfaceOnLinkUp` | unvalidated |
| A-4 | Closing the unit descriptor in `Cleanup` removes the device before or soon after the next `Dial` | ppp unit lifetime ends with its file | EEXIST on fast reconnect | bounded retry, integration test reconnecting at once | unvalidated |
| A-5 | The iface apply prune step leaves a ppp device named after a pppoe-client alone | prune works from previous config kinds | prune deletes the live session device on reload | `TestApplyKeepsPPPoEClientDevice` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A key longer than 15 octets or invalid as an ifname was accepted before and now fails | parse refusal on upgrade | message names the limit and suggests a shorter name; pre-release, no migration owed |
| R-2 | A configuration that matched `ppp*` or `ppp0` in firewall or static stops matching | traffic not NATed | documented; VyOS mapping guide (child 10) drops the `ppp*` rewrite |
| R-3 | The configured name collides with an existing kernel device not in config | EEXIST every dial | error names the device and the fix; `ze doctor` not needed |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the WAN session never comes up (creation fails), or routes and NAT miss the WAN |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | child 12 (same dialer), child 4 (IKE on the WAN name), child 9 and 21 (MSS on the WAN name), `spec-pppoe-virtual-interface-proof.md` (asserts `ppp<N>` exists on both sides: its AC changes to the configured name on the client), `spec-iface-route-arbitration.md` (learned default route) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `pppoe-client pppoe0` config, real kernel | → | `Dial` named unit creation | `TestDevPPPSetupNamesUnit` (Linux, user namespace) |
| `pppoe-client pppoe0` with fake dialer | → | `runSession` backend calls use `pppoe0` | `TestPPPoEClientUsesConfiguredDeviceName` |
| static route `interface pppoe0` while the session is down | → | `checkNextHopResolvable` accepts declared dynamic name | `test/static/static-interface-nexthop-pppoe-absent.ci` |
| device created after commit | → | FIB programs the route | `TestFIBProgramsDeclaredInterfaceOnLinkUp` |
| full CPE: session, static default, masquerade, reconnect | → | everything above | interop `pppoe-named-device-reconnect-ze-client` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Session up for `pppoe-client pppoe0` | kernel device `pppoe0` exists; no `pppN` created by the client; address, MTU and learned default route on `pppoe0` |
| AC-2 | Session lost and redialled | the new session's device is again `pppoe0`, whatever unit number the kernel gives it |
| AC-3 | `static route 0.0.0.0/0 next interface pppoe0` committed while no session is up | commit accepted; route installed when `pppoe0` appears; reinstalled after a reconnect |
| AC-4 | static next-hop naming an interface no configuration declares and the kernel lacks | refused at commit as today |
| AC-5 | firewall `output-interface pppoe0` masquerade | NATed traffic leaves through the session, counters increment, before and after a reconnect |
| AC-6 | pppoe-client key of 16 octets, or with a slash or space | refused by validation with the IFNAMSIZ limit in the message |
| AC-7 | pppoe-client key equal to an ethernet, bridge or other configured interface name | refused, message names both entries |
| AC-8 | `show interface` for the client | the OS name column shows `pppoe0` |
| AC-9 | iface reload while the session is up | `pppoe0` is not deleted |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `next { interface pppoe0 }` as the guide shows, with `no-default-route` on the client | config to static to FIB on device appearance | interop `pppoe-named-device-reconnect-ze-client` |
| 2 | Masquerades the LAN out `pppoe0` | firewall literal name equals device | same scenario, nft counter |
| 3 | Unplugs the fibre and plugs it back | reconnect, same name, route back | same scenario, accel-ppp terminates the session |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDevPPPSetupNamesUnit` | `internal/component/l2tp/ppp/devppp_integration_linux_test.go` | A-1, A-4: named creation, unit number read back, recreate after close | |
| `TestPPPoEClientUsesConfiguredDeviceName` | `internal/component/iface/pppoe_client_test.go` | AC-1, AC-8 with a fake backend | |
| `TestPPPoEClientNameValidated` | `internal/component/iface/config_test.go` | AC-6, AC-7 | |
| `TestApplyKeepsPPPoEClientDevice` | `internal/component/iface/config_apply_test.go` | AC-9, A-5 | |
| `TestResolverDeclaredDynamicName` | `internal/component/iface/resolve_test.go` | absent declared name reports "declared, absent", distinct from unknown | |
| `TestStaticAcceptsDeclaredAbsentInterface` | `internal/plugins/static/locrib_test.go` | AC-3, AC-4 | |
| `TestFIBProgramsDeclaredInterfaceOnLinkUp` | `internal/plugins/fib/kernel/` test file | AC-3, A-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pppoe-client name length | 1-15 | 15 | 0 | 16 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-client-name-too-long` | `test/parse/` | AC-6 | |
| `pppoe-client-name-collides` | `test/parse/` | AC-7 | |
| `static-interface-nexthop-pppoe-absent` | `test/static/` | AC-3 at commit | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-named-device-reconnect-ze-client` | `test/interop-pppoe/scenarios/` | accel-ppp | device named `pppoe0`, static default and masquerade through it, `accel-cmd` terminate, reconnect, route and NAT back | |

## Files to Modify
- `internal/component/l2tp/ppp/devppp_linux.go`, `devppp_other.go` - named unit creation through rtnetlink; PPPIOCNEWUNIT path removed for the client (the LNS and AC keep theirs: unnamed subscriber units)
- `internal/component/l2tp/pppoeclient/dialer.go` - pass the name, bounded retry
- `internal/component/iface/pppoe_client.go` - device name from config in `runSession` and `status`
- `internal/component/iface/config.go` - key validation and cross-kind uniqueness
- `internal/component/iface/resolve.go` - declared dynamic names published with the mapping
- `internal/component/iface/config_apply.go` - prune leaves the client device alone (if A-5 fails)
- `internal/plugins/static/locrib.go` - accept declared absent interface
- `internal/plugins/fib/kernel/` - program on device appearance
- `internal/component/iface/yang/ze-iface-conf.yang` - list and `name` descriptions; `length "1..15"` and the ifname pattern
- Docs: `docs/guide/static-routes.md` (example now true; say the route waits for the session), `docs/architecture/l2tp/cpe-1-pppoe-client.md`, `docs/architecture/iface/logical-name-resolution.md` (pppoe-client is a created kind; declared dynamic names), `docs/architecture/static-routes.md` (absent declared name), `docs/guide/firewall.md` (name the PPPoE client by its configured name)

## Files to Create
- `internal/component/l2tp/ppp/devppp_integration_linux_test.go`
- `test/parse/pppoe-client-name-too-long.ci`, `test/parse/pppoe-client-name-collides.ci`, `test/static/static-interface-nexthop-pppoe-absent.ci`
- `test/interop-pppoe/scenarios/pppoe-named-device-reconnect-ze-client/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | constraints on an existing key, descriptions |
| YANG validation constraints | Yes | `length`, ifname `pattern` |
| YANG custom validators | Yes | cross-kind uniqueness of interface names |
| CLI commands/flags | No | |
| CLI grammar | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | rtnetlink ppp kind uses /dev/ppp, already checked |
| Prometheus counters/metrics | No | |
| BGP family surface | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/` PPPoE client and static routes entries |
| 2 | Config syntax changed? | Yes | key constraints; `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | Yes | show output OS name; `docs/guide/command-reference.md` if it shows `pppN` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | static and fib behaviour; `docs/guide/plugins.md` if it describes resolution |
| 6 | Has a user guide page? | Yes | `docs/guide/static-routes.md`, `docs/guide/firewall.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/iface/logical-name-resolution.md`, `docs/architecture/static-routes.md`, `docs/architecture/l2tp/cpe-1-pppoe-client.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered plugin/event/command changed? | No | |
| 16 | Source anchors | Yes | derived at implementation |
| 17 | Existing examples for this area? | Yes | `docs/guide/static-routes.md` l.69, l.83, l.84, l.101 (`pppoe0`, `pppoe1`) |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- `Dial` receives the name; `runSession` and `status` use it; named creation stubbed to fail
   - Tests: `TestPPPoEClientUsesConfiguredDeviceName`, `TestDevPPPSetupNamesUnit`
   - Files: `dialer.go`, `pppoe_client.go`, `devppp_linux.go`
   - Verify: fake-dialer test passes on names; kernel test fails on the stub
2. **Phase: named unit** -- rtnetlink creation, unit number read back, EBUSY and EEXIST retry; AC-1, AC-2
3. **Phase: validation** -- key length, pattern, uniqueness; AC-6, AC-7; prune check AC-9
4. **Phase: static and FIB** -- declared dynamic names in the resolver; static accepts; FIB programs on appearance; AC-3, AC-4
5. **Phase: interop and docs** -- scenario with reconnect and nft counters; AC-5; every page above

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | no consumer still builds `ppp` plus unit number for the client |
| Data flow | static and fib learn absence from iface, never from the pppoe-client list |
| Rule: no-layering | PPPIOCNEWUNIT unnamed path not kept for the client beside the named one |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `ppp` plus unit name in the client | `grep -n 'StrInt("ppp"' internal/component/iface/pppoe_client.go` empty |
| Guide example true | interop scenario green |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | key validated as an ifname before it reaches netlink |
| Isolation | a client never creates or deletes a device it did not create (EEXIST is reported, never resolved by deleting) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 broken | STOP: return to the owner with the kernel evidence |
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Design Insights

The resolver exists to bind a logical name to hardware Ze did not create.
A PPPoE session device is created by Ze, like a bridge or a WireGuard device,
so it belongs on the other side of that line: created under its name, with no
mapping at all.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| none yet | - | grep of `plan/journal/` finds no row for the pppoe-client name or for static/FIB not reinstalling an interface route; the main thread adds one row for each and lists them here |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Create the PPP unit under the configured name (rtnetlink kind `ppp`, as pppd's `ifname` option does) | publish a logical-to-`pppN` mapping in the resolver and translate everywhere | nft `iifname`/`oifname` are strings compared per packet: a mapping means rewriting the ruleset on every reconnect, with a window where a rule names a `pppN` that is gone or, on a box that is also an LNS or AC, now belongs to a subscriber session. The firewall today translates nothing. Created kinds are already made under their logical name (`logical-name-resolution.md`), so this keeps one rule |
| Static accepts a declared but absent dynamic interface; FIB programs on appearance | keep refusing absent names | at boot and during a reconnect the session device does not exist; refusing makes the guide's route uncommittable |
| Absence learned from iface (declared names) | static reads the pppoe-client list | static must not learn interface kinds (registration over enumeration) |
| Keep IPCP-learned default route unchanged | move it to static | out of scope; `spec-iface-route-arbitration.md` owns route arbitration |

## Known Limitations
- Found while surveying, not in this child's scope as the umbrella states it ("any non-ethernet name"): an `ethernet` entry with `os-name` or `mac match` is named literally by firewall and NAT (`lowerIfaceMatch` takes the configured name), so `input-interface wan` with `ethernet wan { os-name eth2 }` matches nothing. Needs a journal row and an owner; the main thread decides whether this spec takes it.
- L2TP LNS and PPPoE AC subscriber devices stay `pppN`: they have no configured name.
- Other non-ethernet kinds checked: dummy, veth, tunnel, wireguard, xfrm, bridge are all created under their configured name (`config_apply.go`), so they need nothing.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass; `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
