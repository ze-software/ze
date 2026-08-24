# Spec: fixit-vrrp-parent-ignores-the-selector

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**An operator pins an interface to its hardware by permanent MAC, and VRRP builds
that interface's virtual-router device on whatever else is wearing the logical
name.**

Two producers were read on 2026-08-22 and both hold today.

`unitDevice` (`internal/plugins/vrrp/groups.go`) returns the logical interface
name from the configuration, or that name with the VLAN id appended. It consults
no selector: not `mac/match`, not `os-name`, and not `iface.Resolve`. Its answer
becomes `GroupSpec.ParentDevice` in `groupsForFamily` (same file).

`CreateMacvlanDevice` (`internal/plugins/iface/netlink/macvlan_linux.go`) resolves
that value with `netlink.LinkByName(spec.Parent)`. A kernel lookup by name over a
value that was never resolved to a device.

So for an interface whose hardware is selected rather than named, the virtual-MAC
device is created on the wrong parent, or the call fails because no kernel device
wears that name. Neither outcome is reported as a selector problem.

**The code contradicts its own recorded invariant.** `ParentDevice`'s doc comment
in `internal/plugins/vrrp/groups.go` states it plainly: "Sockets, the macvlan
parent, and the tie-break source address all belong to THIS device, not to the
logical interface name." The comment is right and the producer does not hold it,
which is the shape `ai/rules/evidence.md` warns about: a comment is its author's
belief, never evidence about the code beneath it.

**This is the sibling of a defect that closed the same day.**
`spec-fixit-iface-selector-ignored-by-apply` fixed the config apply path, where
roughly twenty-eight consumers reached the right device through `iface.Resolve`
and apply did not. The plugin-facing registries were outside that spec's
acceptance criteria and were left keyed by the logical name. VRRP is the
confirmed instance. The spec's first phase is to find out how many others there
are, because a defect fixed in one path and left in a sibling is half fixed
(`ai/rules/completion.md`).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - selector resolution and who owns it
  → Constraint: a selector is answered once, by `iface.Resolve`, and every consumer takes the resolved device. A consumer that re-derives a device from a name has left the contract.
- [ ] `docs/architecture/iface/offload.md` - the sibling spec's closing note
  → Constraint: bypassing `Backend` does not bypass the selector. The same sentence must become true of the plugin registries.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9568.md` - VRRP virtual MAC and the interface it lives on
  → Constraint: the virtual router owns a virtual MAC on the interface it protects. Building it on a different interface is not a degraded VRRP, it is VRRP for the wrong link.

**Key insights:**
- The logical name and the OS device are two different values, and every place that passes one where the other is meant is an instance of this defect.
- The invariant is already written down at the field that violates it. The gap is enforcement, not knowledge.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/vrrp/groups.go` - `unitDevice` returns the configured name or `<name>.<vlan>` and consults no selector; `groupsForFamily` writes it to `GroupSpec.ParentDevice`; the field's doc comment states the invariant the value breaks
- [ ] `internal/plugins/iface/netlink/macvlan_linux.go` - `CreateMacvlanDevice` calls `netlink.LinkByName(spec.Parent)`
- [ ] `internal/component/iface/resolve.go` - `iface.Resolve` and `matchByMAC`, the one answer to a selector
- [ ] `internal/component/iface/config_apply.go` - `bindDevices` and `deviceFor`, how the apply path takes the resolved device once and hands it to every consumer

**Behavior to preserve:**
- The VLAN composition rule. A unit carrying a `vlan-id` still names `<device>.<vid>`, with the resolved device on the left.
- An interface with no selector keeps resolving to its own name, so nothing changes for the common configuration.
- The macvlan rollback and adoption paths in `CreateMacvlanDevice` are untouched.

**Behavior to change:**
- `ParentDevice` carries the RESOLVED device, so it means what its doc comment says.
- A selector that answers no device, or more than one, is refused where the group is built, with an error naming the interface and the selector, rather than surfacing as a netlink lookup failure or silently binding elsewhere.
- Every other plugin-facing registry found in Phase 1 takes the same treatment.

**Behavior that MUST NOT change, found by review (B-1):** a binding outcome MAY
create a virtual router and MAY move one, and it MUST NOT destroy one. The first
implementation dropped an unbindable group from the desired set, so the
delete-first loop tore its instance down: a Priority-0 resignation and the VIP
removed. That is a worse failure than the defect being fixed, because
`ResolveDevice` refuses on ANY `Resolve` failure once the name carries a selector
-- `errIfaceNoBackendLoaded`, and a failed `ListInterfaces` inside `matchByMAC`,
included -- the resolver cache cannot absorb it (`setMapping` clears the cache on
every iface apply), and `apply` has no re-trigger, so one transient netlink read
during an unrelated commit failed a live master over permanently. AC-5 therefore
reads: a group that is NOT running is not started, and a group that IS running
keeps the device it is bound to. Only the config removing a group tears it down.
The device actually disappearing is handled by `parentReady` and `watchParent`,
which stop and restart the virtual router with no commit and no macvlan churn.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator configures a `vrrp group` under a unit of an interface whose hardware is selected by `mac/match` or aliased by `os-name`, and commits.
- Format at entry: the resolved config tree, walked by the vrrp plugin.

### Transformation Path
1. The vrrp plugin walks interfaces and units and calls `unitDevice` (`internal/plugins/vrrp/groups.go`) -- the defect is here.
2. `groupsForFamily` writes that value to `GroupSpec.ParentDevice`.
3. The group's reconcile builds a `iface.MacvlanSpec` whose `Parent` is that value.
4. `CreateMacvlanDevice` (`internal/plugins/iface/netlink/macvlan_linux.go`) resolves it with `netlink.LinkByName`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ vrrp plugin | the walked interface and unit containers | No |
| vrrp plugin ↔ iface backend | `iface.MacvlanSpec` | No |
| Backend ↔ kernel | `netlink.LinkByName` plus `LinkAdd` | No |

### Integration Points
- `iface.Resolve` (`internal/component/iface/resolve.go`) - the single answer to a selector, which this fix makes the vrrp plugin ask
- `GroupSpec.ParentDevice` - the field whose documented meaning the fix restores

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | VRRP now asks `iface.ResolveDevice`, the same translation the 25 by-name dispatch ops take. The netlink backend still receives a kernel device name, so no selector knowledge moved into it |
| No unintended coupling (components stay isolated) | Yes | `groups.go` keeps its no-`iface`-import property: the resolver arrives as a `deviceResolver` func through `enginePlatform`, the seam the transport and the macvlan calls already use. `register.go` and `engine.go` already imported `internal/component/iface` |
| No duplicated functionality (extends existing, does not recreate) | Yes | `resolveOS` was exported rather than reimplemented. Its rule (identity with no selector, refuse when a selector answers nothing or several) is the one this fix needed, and a second copy in vrrp is what the Key Design Decision rejected |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Config-time strings on a once-per-apply path. `unitDeviceName` uses `textbuf.Buffer`, as `unitDevice` did |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Nothing was added to a core package for VRRP. The one iface change is an EXPORT of a function that already existed and already served every by-name op; it names no plugin |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | VRRP is not the only plugin-facing registry keyed by the logical name | the closing review of `spec-fixit-iface-selector-ignored-by-apply` reported two, and named one | the spec is one file and Phase 1 is wasted | the Phase 1 enumeration, run with `gopls references` over `iface.Resolve` and over every consumer of a configured interface name | **broken, in the narrow reading; confirmed in the wide one.** VRRP is the ONLY plugin-facing REGISTRY holding an untranslated name. `RegisterOwnedAddresses` has exactly one caller (`internal/plugins/vrrp/register.go`) and it passes the macvlan device Ze itself composed, so that registry is correct. The enumeration found TWO further sites of the same shape outside the registries, and an independent review found the second after this row first claimed the walk was complete: `checkRAForwarding` (`internal/plugins/iface/ra/doctor.go`) reads `/proc/sys/net/ipv6/conf/<configured-name>/forwarding`, and `resolveOSName` (`internal/plugins/traffic/netlink/backend_linux.go`) falls back to the logical name on ANY resolution failure, which is the exact fallback `ResolveDevice` refuses. Neither is fixed here; both are recorded. See "Phase 1 enumeration" under Files to Modify |
| A-2 | The vrrp plugin can reach `iface.Resolve` without a tier violation | the resolver lives in `internal/component/iface`, and `internal/plugins/` may depend on components | the fix needs the resolved device passed in rather than looked up, which changes the plugin's input | `make ze-tier-check` after a trial import | **confirmed.** `internal/plugins/vrrp/register.go` and `engine.go` already import `internal/component/iface`, and `register.go` already calls `iface.Resolve` for `parentIfindex` and `parentReady`. The fix adds no import: the resolver reaches `groups.go` as a `deviceResolver` func field, so the pure config layer keeps its no-iface-import property |
| A-3 | A selector answering no device at group-build time is a configuration error rather than a transient absence | the sibling spec refuses ambiguity at apply and drops an unanswered selector from the applied set | refusing the group breaks a valid boot ordering where the hardware appears late | the sibling spec's `validateSelectors` behavior, read whole, and a boot-ordering test | **broken, and R-1's fallback shipped.** `validateSelectors` (`internal/component/iface/config_apply.go`) refuses ONLY ambiguity and returns nil for zero matches, saying so: "an absent device is a deferred binding, which the YANG promises, not an error". `bindDevices` maps an unanswerable selector to UNBOUND and every apply phase SKIPS it. Refusing at group-build time would also have made the vrrp verifier impure, so `ze config validate` on a box whose NIC has not enumerated would refuse the daemon's own saved config, and `ze doctor` would need the hardware present. The binding therefore moved to `engine.apply`. A group that is not running is left out of the desired set and never bound to a wrong device; a group that IS running is kept where it is, because a binding outcome may create and may move a virtual router and may never destroy one (review B-1, R2-1) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Resolving at group-build time races hardware that appears late in boot | a VRRP group is refused on a cold boot and works on a warm one | A-3 decides it. If the risk is real, the group is built unresolved and refused at the point of use, never bound to a wrong device |
| R-2 | The enumeration in Phase 1 is short, and a third registry keeps the defect | a later session finds another `LinkByName` over a configured name | the enumeration uses `gopls references` rather than grep, and its result is recorded in the spec so the next reader inherits it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A VRRP virtual MAC is created on the wrong physical link, so the protected address fails over to a router that cannot carry it |
| How is it reverted? | Single commit revert. No config migration, no persisted state |
| Who else touches this path? | `spec-fixit-iface-selector-ignored-by-apply` closed the apply half on 2026-08-22 and is the precedent this follows |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A VRRP group on an interface selected by `mac/match` | → | `GroupSpec.parentDevice` + `engine.apply`'s binding step (the split replaced `unitDevice` with `unitVLANID` and `unitDeviceName`) | `TestVRRPParentTakesTheResolvedDevice` |
| The same configuration on a live kernel | → | `CreateMacvlanDevice` | `test/vrrp/vrrp-macvlan-parent-selector.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file) --> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A VRRP group under an interface whose hardware is selected by `mac/match` | The macvlan parent is the device the selector answered, not the logical name |
| AC-2 | The same, with `os-name` aliasing a kernel device | Same |
| AC-3 | The same, on a unit carrying a `vlan-id` | The parent is `<resolved-device>.<vid>` |
| AC-4 | An interface with no selector | The parent is its own name, exactly as today |
| AC-5 | A selector answering no device, or more than one | **A group that is NOT running is not started**: an error names the interface and the selector, and no macvlan is created on any device. **A group that IS running keeps the device it is bound to.** Only the config removing a group tears it down, and a MOVE holds its predecessor until the replacement is built. The original wording said the group is "refused" without distinguishing the two, and taken literally it destroys a live virtual router on a transient netlink read (review B-1, R2-1) -- see "Behavior that MUST NOT change" |
| AC-6 | Every plugin-facing registry found in Phase 1 | Each takes the resolved device, and none resolves a configured name against the kernel itself |
| AC-7 | AC-1 on a live kernel | The created macvlan's parent index is the selected device's index |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Pins an interface to a NIC by permanent MAC, then configures VRRP on it | config tree → vrrp plugin → iface backend → kernel | `test/vrrp/vrrp-macvlan-parent-selector.ci` <!-- doc-links: ignore (this spec's AC-7 creates the file) --> |
| 2 | Moves the NIC to a different slot so the kernel renames it, and reboots | same path, same selector, new kernel name | the same test, with the kernel name changed between runs |

## 🧪 TDD Test Plan

### Unit Tests
Every one is driven through `engine.apply`, never through the helper alone: apply
is where `ParentDevice` is bound and where the kernel-facing values are handed
out, so a test on the helper would leave every sink free to take the logical name.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVRRPParentTakesTheResolvedDevice` | `internal/plugins/vrrp/groups_test.go` | AC-1 and AC-2: `ParentDevice`, the macvlan parent, the sysctls and the transport parent are all the resolved device. One resolver answers both selector forms, since the form is iface's business and never reaches vrrp | green; red under mutation |
| `TestVRRPParentComposesVLANOnTheResolvedDevice` | `internal/plugins/vrrp/groups_test.go` | AC-3 | green; red under mutation |
| `TestVRRPParentUnselectedInterfaceIsUnchanged` | `internal/plugins/vrrp/groups_test.go` | AC-4, so the common configuration is pinned against regression | green; correctly SURVIVES the mutation, which is what makes it the regression guard |
| `TestVRRPGroupRefusesAnUnansweredSelector` | `internal/plugins/vrrp/groups_test.go` | AC-5 in both the zero and the ambiguous case | green; red under mutation |
| `TestVRRPGroupSurvivesAnUnresolvedSelector` | `internal/plugins/vrrp/groups_test.go` | AC-5 for a group that is ALREADY RUNNING: it keeps its device through every way `ResolveDevice` can refuse, including the two that say nothing about the hardware, and heals by reconfiguring rather than rebuilding. Replaces `TestVRRPGroupStopsWhenItsSelectorStopsAnswering`, which asserted the reverse and was deleted inside this changeset (review B-1) | green; red under mutation |
| `TestVRRPGroupEditsLandWhileItsSelectorIsUnresolved` | `internal/plugins/vrrp/groups_test.go` | keeping a group must not mean freezing it: a priority edit still lands while the selector is unresolved. Counts instances BEFORE ranging, so it cannot pass vacuously on the empty map the defect produces | green; red under mutation |
| `TestVRRPGroupMoveKeepsTheOldRouterWhenTheRebuildFails` | `internal/plugins/vrrp/groups_test.go` | the MOVE path obeys the same rule: a replacement that cannot be built leaves the running router in place, and the move still happens once it can (review R2-1) | green; red under mutation |
| `TestVRRPGroupMovesWithItsSelector` | `internal/plugins/vrrp/groups_test.go` | AC-1 across an apply that moves the binding: reconfigure-in-place would leave the virtual MAC on the old device | green; red under both mutations |
| `TestVRRPGroupRenameIsNotAMove` | `internal/plugins/vrrp/groups_test.go` | a kernel RENAME preserves the ifindex, so it composes the SAME macvlan and must reconfigure rather than rebuild. Added in review round 3: rebuilding would register the device and immediately unregister it, and the next reconcile would orphan-delete it while the FSM still reported master | green; red under mutation |
| `TestVRRPGroupNeverAdoptsAParentItsMacvlanIsNotOn` | `internal/plugins/vrrp/groups_test.go` | AC-5 for the second, INDEPENDENT resolution: `ResolveDevice` answers the logical name and `parentIfindex` then fails on the device it named, so `spec.ParentDevice` names hardware this instance's macvlan is not on. Added in review round 4: `reconfigure` assigns the spec wholesale, so adopting it points `parentReady` at an absent device and the master resigns and drops the VIP. Drives the fake's `ifindexErr` arm, which no test reached before | green; red under mutation (the mutated run reports `ParentDevice = "eth7", want eth3` and a sysctl reassert on `eth7/`) |
| `TestNoRegistryResolvesAConfiguredNameAgainstTheKernel` | `internal/plugins/vrrp/groups_test.go` | AC-6 over every sink at once (macvlan parent, sysctls, reasserted sysctls, transport parent, instance spec), rather than over the one that was fixed | green; red under mutation |
| `TestUnitDeviceResolution` | `internal/plugins/vrrp/groups_test.go` | repointed: extraction records the unit's `VLANID` and binds NO device, which is the purity property `ze config validate` depends on | green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VLAN id | 0-4094 | 4094 | N/A (0 means no VLAN) | 4095 |
| Devices answering a selector | 0..N | 1 | 0 is refused | 2 is refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vrrp-macvlan-parent-selector` | `test/vrrp/vrrp-macvlan-parent-selector.ci` | a MAC-selected interface hosts its virtual router on the selected NIC | **green on a live kernel.** Re-run 2026-08-24 on linux/amd64 after the round-4 fix: `make ze-qemu-debug NOBUILD=1 RUN='<bin>/ze-test-linux-amd64 vrrp -a -v'` gives `pass 9/9 100.0%`, this test among them. Discriminates, re-proved in the same run shape: with the binding in `engine.apply` reverted to the logical name it FAILS in QEMU with `macvlan zv4-4-40 hangs off the DECOY zevrwan (ifindex 6): the parent was taken from the configured interface name instead of the selector's answer` <!-- doc-links: ignore (this spec's AC-7 creates the file) --> |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | The defect is local device selection, not wire behavior. RFC 9568 conformance is unchanged by this fix, and the existing VRRP interop coverage exercises the protocol | N-A |

## Files to Modify
- `internal/plugins/vrrp/groups.go` - `unitDevice` split into `unitVLANID` (the config fact) and `unitDeviceName` (the composition); `GroupSpec.parentDevice` asks the injected `deviceResolver` and refuses an unanswered selector
- `internal/plugins/vrrp/engine.go` - `apply` binds every group's `ParentDevice` before the diff; a group that cannot bind and is not running is not started, one that IS running keeps its device, and a group whose device moved is rebuilt by BUILDING the replacement before releasing the predecessor. `create` splits into `build` (makes everything, stores nothing) and `start`, which is what makes that ordering possible
- `internal/plugins/vrrp/register.go` - `livePlatform` wires `resolveDevice: iface.ResolveDevice`
- `internal/component/iface/dispatch.go` - `resolveOS` becomes exported `ResolveDevice`, so a plugin-facing consumer takes the ONE answer rather than composing a second from `Resolve` plus a selector check
- `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`, `docs/architecture/iface/logical-name-resolution.md`, `docs/guide/vrrp.md`, `docs/features/interfaces.md`, `ai/digests/iface.md`
- `test/vrrp/vrrp-instance-up.ci` - joins the `iface-owned-macvlan` exclusive group. The group is named for the RESOURCE, the `ze:owned:` alias namespace, rather than for vrrp: any plugin that registers an owned macvlan contends for it, and vrrp is only today's single caller
- `test/vrrp/vrrp-idle.ci`, `test/vrrp/vrrp-doctor-quiet.ci` - both asserted `ze doctor --json` exit 0, which is a verdict on the WHOLE machine: `checkInterfaces` (`internal/component/doctor/checks_linux.go`) emits `doctor-iface-missing` at ERROR severity for a configured `ethernet` entry with no `/sys/class/net` directory, and the fixtures name `eth0` and `eth1`, which this dev box and the QEMU VM do not both carry. Both now assert the doctor payload (`schema-version`) and keep the `doctor-vrrp-*` rejects, which is the property they exist for. This closes the open journal row in `plan/journal/parallel-copies-collide-on-a-deterministic-port.md`, whose mechanism was unestablished and whose class was wrong

### Phase 1 enumeration (A-1)

Every consumer outside `internal/component/iface` that passes an interface name
was read. Three populations and one registry are correct by construction, and FOUR sites carried the defect: two are fixed here, and two are recorded scope calls with their next step named.

| Population | Verdict |
|-----------|---------|
| The 25 by-name dispatch ops (`internal/component/iface/dispatch.go`) | correct: each translates through `ResolveDevice` |
| Explicit `iface.Resolve` / `Addresses` / `Subscribe` callers (isis, ldp, ospf, static, traffic, flowexport, dhcp, ra sender, tftpserver, trafficusage, diag, l2tp/pppoe, and vrrp's own `parentIfindex` and `parentReady`) | correct: `Resolve` takes a LOGICAL name and answers the selector |
| Raw by design (`GetInterface`, `ListInterfaces`, `Create*`, `ValidateIfaceName`, `ComposeOwnedDeviceName`) | correct: a created device's name IS its kernel name, and the resolver is built on the two listing calls |
| `iface.RegisterOwnedMacvlan`, whose `MacvlanSpec.Parent` is documented "OS device name of the parent interface" | **DEFECT, fixed here.** One caller: `internal/plugins/vrrp/register.go` |
| `parentSysctls` (`internal/plugins/vrrp/dataplane_linux.go`) | **DEFECT, fixed here.** Builds `/proc/sys/net/ipv4/conf/<parent>/` from the same `ParentDevice` value, so one binding fixes both |
| `iface.RegisterOwnedAddresses` | correct: one caller (vrrp), passing the macvlan device Ze composed |
| `resolveOSName` (`internal/plugins/traffic/netlink/backend_linux.go`) | **DEFECT, NOT fixed here.** Found by independent review after this table first claimed the walk complete. It calls `iface.Resolve` and returns the LOGICAL name on any failure, which is exactly the fallback `ResolveDevice` refuses for a name that has a selector, so a tc operation lands on whatever wears that name. Not small: the helper returns no error, its three call sites want different answers, and `restoreOriginalLocked`'s fallback is deliberate and documented (a missing snapshot makes restore a correct no-op). Making it fallible means threading an error through a tc snapshot/restore path whose correctness argument this spec has not read, in a commit about VRRP. Recorded in `plan/journal/identity-default-hides-a-mapping.md` |
| `checkRAForwarding` (`internal/plugins/iface/ra/doctor.go`) | **DEFECT, NOT fixed here.** Reads `/proc/sys/net/ipv6/conf/<configured-name>/forwarding`, so a selected interface reports "unknown" and the check goes silent. `ResolveDevice` cannot fix it: `ze doctor` is registered `Mode: "offline"` as a LOCAL command (`internal/component/doctor/register.go`), so it runs in the CLI process where `setResolverConfig` has never been called and every name resolves to itself. The `ze-show:doctor` RPC form runs in the daemon, where the maps ARE published, so one check would answer two ways. Recorded in `plan/journal/identity-default-hides-a-mapping.md` with the next step |

## Files to Create
- `test/vrrp/vrrp-macvlan-parent-selector.ci` - the AC-7 proof. `test/vrrp/`, not the `test/iface/` this spec first named: that directory does not exist and every VRRP `.ci` lives in `test/vrrp/` <!-- doc-links: ignore (this spec's AC-7 creates the file) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | the selector leaves already exist; they stop being ignored |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | the ambiguity refusal needs the interface listing, which validation has no view of. It lives where the group is built |
| CLI commands/flags | No | no command changes |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | no new env var |
| Doctor check for runtime dependencies | **OPEN -- needs the owner's decision, and it is the one item this spec does not close** | The intent stands: a VRRP group whose selector answers nothing should be visible in `ze doctor`. It cannot be written today, and the reason is a capability gap rather than an omission. `ze doctor` is registered `Mode: "offline"` (`internal/component/doctor/register.go`), so `Run` executes in the `ze` CLI process, where `setResolverConfig` has never run and every logical name resolves to itself; a check calling `iface.ResolveDevice` there would report "no selector configured" for every selected interface and be silently vacuous, while the `ze-show:doctor` RPC form running inside the daemon would answer correctly. The same gap is why `checkRAForwarding` goes silent (Phase 1 enumeration, above): two checks, one missing capability. Building it twice is what `ai/rules/simplicity.md` forbids, so this belongs in one spec that gives a CLI-process doctor check a selector answer -- read out of the config tree the check already holds, matched against a live listing, the way `bindDevices` does. No implementation adds a runtime dependency here: the selector leaves already existed and this fix only stops them being ignored |
| Prometheus counters/metrics | **No here, and the old reason was wrong** | The row read "a refused group is a config error, reported at commit". It is not: an absent selector is a DEFERRED binding and the commit succeeds (A-3, broken). A group that never starts is therefore visible only as one `Error` log line. A gauge for it needs a label set keyed without a device -- the existing series are `{device,group,vrid,family}` and an unbound group has no device -- so it is a new series with its own docs, not a relabel. Recorded with the retry gap in `plan/journal/unwired-feature.md` rather than added to this commit |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a defect is removed |
| 2 | Config syntax changed? | No | no leaf changes |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/vrrp.md`: a selected interface hosts its virtual router on the selected device |
| 6 | Has a user guide page? | Yes | the same page |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 9568 conformance is unchanged; the virtual router now lives on the interface the operator named |
| 10 | Test infrastructure changed? | No | existing runners |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`, the design doc `groups.go` declares in its `// Design:` header: it must say the virtual router lives on the RESOLVED device and what happens when the selector answers nothing. Also `docs/architecture/iface/logical-name-resolution.md`, the sentence that every consumer takes the resolved device, extended to the plugin registries |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | **No counter, but a label VALUE moves** | `telemetry.go` labels the state gauge, the transition counter and the state-change event with `spec.ParentDevice`, which now reads the kernel device instead of the logical name. For an interface with NO selector nothing moves (the two are the same string). For a `mac/match` or `os-name` interface the `device` label changes value, so a dashboard or alert keyed on the old value follows the rename. The label becomes TRUE rather than changing meaning -- `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` already promised the OS device -- and it is recorded in the guide. `instanceView.Device` is unaffected: it carries `in.dev`, the macvlan name |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on `vrrp/groups.go` and `iface/netlink/macvlan_linux.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify the VRRP examples against a selected interface |

## Implementation Steps

1. **Phase: Enumerate, and validate A-1 (MANDATORY FIRST)** -- find every plugin-facing site that passes a configured interface name where a resolved device is meant
   - Method: `gopls references` over `iface.Resolve` to find who already asks, then over the consumers of configured interface names to find who does not. Not grep: a text match cannot tell a name from a device
   - Files: this spec's Files to Modify, amended with what the enumeration returns
   - Verify: A-1 flips to `confirmed` or `broken`. If VRRP is genuinely the only site, say so and narrow the spec
2. **Phase: Wiring** -- the two tests from the Wiring Test table, red against today's code
   - Tests: `TestVRRPParentTakesTheResolvedDevice`, `vrrp-macvlan-parent-selector`
   - Verify: the unit test shows the logical name where the selected device belongs, and the `.ci` shows the macvlan on the wrong parent or absent
3. **Phase: Decide A-3 before the refusal lands** -- resolve at build time, or build unresolved and refuse at use
   - Files: the spec's Assumptions table
   - Verify: A-3 flips. A boot-ordering test decides it, not a preference
4. **Phase: The resolved device** -- VRRP takes what the selector answered
   - Tests: the five unit tests above
   - Files: `internal/plugins/vrrp/groups.go`
   - Verify: red before, green after, and AC-4 proves the unselected path did not move
5. **Phase: The siblings** -- every other site Phase 1 named
   - Tests: `TestNoRegistryResolvesAConfiguredNameAgainstTheKernel`
   - Verify: no plugin-facing registry resolves a configured name against the kernel

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol, and AC-6 names the enumerated set |
| Correctness | The VLAN composition puts the RESOLVED device on the left of the dot |
| Data flow | One resolution, at one place, handed to every consumer |
| Rule: `ai/rules/evidence.md` | The refusal fails closed: an unanswered selector never yields a device that happens to match a name |
| Rule: `ai/rules/completion.md` | The sibling sites are fixed, not recorded. A registry left keyed by the logical name is this defect surviving |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The parent is the resolved device | `make ze-unit-pkg-test PKG=./internal/plugins/vrrp` |
| The kernel agrees | `test/vrrp/vrrp-macvlan-parent-selector.ci` on a live kernel <!-- doc-links: ignore (AC-7 of this spec creates this file) --> |
| No registry re-derives a device from a name | `TestNoRegistryResolvesAConfiguredNameAgainstTheKernel` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The selector is operator-controlled. A selector that answers the wrong device puts a virtual MAC on a link the operator did not name, which is a traffic-redirection failure rather than a cosmetic one |
| Resource exhaustion | None: the resolution is one listing per apply, as the sibling spec established |

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

- A field whose doc comment states an invariant its producer does not hold is not a documentation defect. The comment is the design, and the code is the bug.
- Fixing a selector in the path that applies configuration does not fix it in the paths that consume configuration. The two are found by asking who reads the name, never by asking who was in the last diff.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The vrrp plugin takes the resolved device and refuses an unanswered selector | **B. Resolve inside `CreateMacvlanDevice`.** REJECTED: it puts selector knowledge in a netlink backend, and every other backend call would need the same. **C. Leave the name and document the limitation.** REJECTED: the operator asked for a device by MAC and got another one, which `plan/future/README.md` lists as a defect that never goes to future | `iface.Resolve` is already the one answer to a selector and roughly twenty-eight consumers take it. This makes VRRP the twenty-ninth rather than inventing a second route |

## Known Limitations

- The enumeration in Phase 1 bounds this spec. A site it misses keeps the defect, which is why the result is recorded in the spec rather than only acted on.

## RFC Documentation (Scope: protocol)

No RFC obligation changes. Add a comment above `unitDevice` naming RFC 9568's
virtual MAC as the reason the parent must be the selected device, so the next
reader sees why the value is not a display name.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `internal/plugins/vrrp/groups.go` -- `unitDevice` split into `unitVLANID` (the
  config fact) and `unitDeviceName` (the composition). `GroupSpec` carries
  `VLANID` and leaves `ParentDevice` empty at extraction, so `ze config validate`
  and `ze doctor` stay pure. `GroupSpec.parentDevice` asks the injected
  `deviceResolver` and refuses a selector that answers nothing or several.
- `internal/plugins/vrrp/engine.go` -- `apply` binds every group's
  `ParentDevice` before the diff. A group that cannot bind and is not running is
  not started; one that IS running keeps the device it is on; one whose netdev
  moved is rebuilt. `create` splits into `build` (makes everything, stores
  nothing) and `start`, so a move holds its predecessor until the replacement
  exists. The move keys on the MACVLAN name, which is composed from the parent's
  ifindex, so a kernel rename reconfigures rather than rebuilds.
- `internal/plugins/vrrp/register.go` -- `livePlatform` wires
  `resolveDevice: iface.ResolveDevice`.
- `internal/component/iface/dispatch.go` -- `resolveOS` exported as
  `ResolveDevice`, with the consumer contract in its doc comment: pass a
  configured name, never `""`, and never tear live state down on its error.
- Tests -- 14 unit tests in `internal/plugins/vrrp/groups_test.go`, and a fake
  platform that models a per-parent ifindex, the macvlan's parent, the
  transport's own instance map, and the two error arms (`macvlanErr`,
  `ifindexErr`). `TestWaitSettledActuallySettles`
  (`internal/plugins/vrrp/cmd_show_test.go`) proves the settle helper settles.
- Functional -- `test/vrrp/vrrp-macvlan-parent-selector.ci` is the AC-7 proof on
  a live kernel. `test/vrrp/vrrp-instance-up.ci` gains `caps=net-admin` and the
  `iface-owned-macvlan` exclusive group. `test/vrrp/vrrp-idle.ci` and
  `test/vrrp/vrrp-doctor-quiet.ci` stop asserting `ze doctor`'s exit code.

### Bugs Found/Fixed
- The spec's own defect: `unitDevice` consulted no selector, so
  `MacvlanSpec.Parent` reached `netlink.LinkByName` carrying a logical name.
  Covered by `TestVRRPParentTakesTheResolvedDevice` and
  `test/vrrp/vrrp-macvlan-parent-selector.ci`.
- `waitSettled` (`internal/plugins/vrrp/cmd_show_test.go`) compared the view's
  state against `fsm.StateInitialize.String()`. `viewState` spells the same
  concept in lower case, so the guard matched nothing and the helper returned on
  its first poll. Covered by `TestWaitSettledActuallySettles`.
- `test/vrrp/vrrp-idle.ci` and `test/vrrp/vrrp-doctor-quiet.ci` asserted
  `ze doctor --json` exit 0, which `checkInterfaces`
  (`internal/component/doctor/checks_linux.go`, `doctor-iface-missing`) makes a
  verdict on the host's NIC inventory. Both now assert the payload and keep the
  `doctor-vrrp-*` rejects.
- Two concurrent test daemons each swept the other's owned macvlan
  (`reconcileOwnedDevices`, `internal/component/iface/config_apply.go`). Both
  vrrp `.ci` that own one now carry `option=exclusive:group=iface-owned-macvlan`.
- Found by the closure review, in the move path this changeset added: a move
  whose parent keeps its NAME closed its own transport. See Review Gate, R5-1.

### Documentation Updates
- `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` -- a new section, "The
  virtual router lives on the RESOLVED device", with anchors on
  `groups.go -- parentDevice, unitDeviceName, unitVLANID` and
  `engine.go -- apply, build, start, teardown`. It states the
  create/move/never-destroy rule and, after the closure review, the one move
  that releases first.
- `docs/architecture/iface/logical-name-resolution.md` -- a new section, "The
  plugin-facing registries take the resolved device too", with anchors on
  `iface/macvlan.go -- MacvlanSpec.Parent`, `vrrp/engine.go -- apply` and
  `vrrp/groups.go -- parentDevice, deviceResolver`.
- `docs/guide/vrrp.md` -- what an operator sees: the virtual router follows the
  selector, the `device` metric label carries the kernel device, an unanswered
  selector starts no group, and a running group is never stopped by one.
- `ai/digests/iface.md` -- the two `resolveOS` references renamed.
- `docs/features/interfaces.md` -- the two `resolveOS` references renamed. This
  file also holds another session's uncommitted mirror-reconcile edits, so the
  rename does NOT ride on this commit (`ai/rules/git-safety.md`: a path carrying
  another session's hunks is dropped from the commit).
- `make ze-doc-verify`: 2 findings, both foreign and both named in Pre-Commit
  Verification below. No finding names a file this spec changes.

### Deviations from Plan
- The refusal moved from group-BUILD time to `engine.apply`. A-3 said a selector
  answering nothing is a configuration error; `validateSelectors`
  (`internal/component/iface/config_apply.go`) proves it is a deferred binding,
  and refusing at extraction would make `ze config validate` need the hardware.
- AC-5 was rewritten during review: a binding outcome may create and may move a
  virtual router, and may never destroy one. The literal reading tore a live
  master down on a transient netlink read.
- `iface.ResolveDevice` is an EXPORT of the function the by-name dispatch ops
  already used, not a new resolver. The spec's Files to Modify was amended.
- Two sibling sites the Phase 1 walk found are recorded rather than fixed:
  `resolveOSName` (`internal/plugins/traffic/netlink/backend_linux.go`) and
  `checkRAForwarding` (`internal/plugins/iface/ra/doctor.go`). Neither is a
  plugin-facing registry, which is what AC-6 names. Both are rows in
  `plan/journal/identity-default-hides-a-mapping.md` with the next step.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 assumed several plugin-facing registries held an untranslated name | VRRP is the only REGISTRY. `RegisterOwnedAddresses`'s single caller passes a device Ze composed | the Phase 1 walk with `gopls references` | the spec was narrowed to VRRP; the two non-registry sites found by the same walk are journal rows |
| assumption | A-3 assumed a selector answering no device is a configuration error | `validateSelectors` returns nil for zero matches and says so: an absent device is a deferred binding | reading the sibling spec's producer whole | the binding moved to `engine.apply`, and extraction stays pure |
| approach | The first implementation dropped an unbindable group from the desired set | the delete-first loop then tore its instance down: a Priority-0 resignation and the VIP removed | review finding B-1 | a running group keeps its device; only the config removing a group tears it down |
| approach | The move path released the predecessor before building the replacement | one unrelated owned-device error destroys a working virtual router, permanently | review finding R2-1 | `create` split into `build` and `start`; build first, release second |
| approach | Build-first was applied to every move | a move whose parent keeps its NAME opens over the running transport key, so the teardown closes the replacement | the closure review, R5-1 | the one colliding move releases first, keyed on `in.key.Interface` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The macvlan parent is the device the selector answered | Done | `internal/plugins/vrrp/engine.go` `apply`; `internal/plugins/vrrp/groups.go` `parentDevice` | one resolution per group per apply |
| `ParentDevice` means what its doc comment says | Done | `internal/plugins/vrrp/groups.go` `GroupSpec.ParentDevice` | the comment now describes the value the producer writes |
| An unanswered selector is refused where the group is bound | Done | `internal/plugins/vrrp/engine.go` `apply`, the `default` arm | named error, no macvlan, no socket, no sysctl |
| Every other plugin-facing registry found in Phase 1 takes the same treatment | Done | Phase 1 enumeration table | VRRP is the only registry; the two non-registry sites are journal rows |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestVRRPParentTakesTheResolvedDevice`, `test/vrrp/vrrp-macvlan-parent-selector.ci` | asserts `ParentDevice`, the macvlan parent, the sysctls and the transport parent |
| AC-2 | Done | `TestVRRPParentTakesTheResolvedDevice` | one resolver answers both selector forms; the form is iface's business |
| AC-3 | Done | `TestVRRPParentComposesVLANOnTheResolvedDevice` | `eth3.100`, the tag on the resolved device |
| AC-4 | Done | `TestVRRPParentUnselectedInterfaceIsUnchanged` | survives the mutation, which is what makes it the regression guard |
| AC-5 | Done | `TestVRRPGroupRefusesAnUnansweredSelector`, `TestVRRPGroupSurvivesAnUnresolvedSelector`, `TestVRRPGroupEditsLandWhileItsSelectorIsUnresolved`, `TestVRRPGroupMoveKeepsTheOldRouterWhenTheRebuildFails`, `TestVRRPGroupNeverAdoptsAParentItsMacvlanIsNotOn` | not started when not running; kept where it is when running |
| AC-6 | Done | `TestNoRegistryResolvesAConfiguredNameAgainstTheKernel` | asserts over every sink at once, not over the one that was fixed |
| AC-7 | Done | `test/vrrp/vrrp-macvlan-parent-selector.ci` on a live kernel | the created macvlan's `iflink` is the selected device's ifindex |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestVRRPParentTakesTheResolvedDevice` | Done | `internal/plugins/vrrp/groups_test.go` | green; red under mutation |
| `TestVRRPParentComposesVLANOnTheResolvedDevice` | Done | same | green; red under mutation |
| `TestVRRPParentUnselectedInterfaceIsUnchanged` | Done | same | green; survives the mutation by design |
| `TestVRRPGroupRefusesAnUnansweredSelector` | Done | same | both the zero and the ambiguous case |
| `TestVRRPGroupSurvivesAnUnresolvedSelector` | Done | same | every way `ResolveDevice` can refuse |
| `TestVRRPGroupEditsLandWhileItsSelectorIsUnresolved` | Done | same | counts instances before ranging |
| `TestVRRPGroupMoveKeepsTheOldRouterWhenTheRebuildFails` | Done | same | the move path obeys the same rule |
| `TestVRRPGroupMovesWithItsSelector` | Done | same | green; red under both mutations |
| `TestVRRPGroupRenameIsNotAMove` | Done | same | a rename preserves the ifindex |
| `TestVRRPGroupNeverAdoptsAParentItsMacvlanIsNotOn` | Done | same | drives the fake's `ifindexErr` arm |
| `TestVRRPGroupMoveUnderOneParentNameKeepsALiveTransport` | Changed | same | ADDED by the closure review (R5-1), not in the TDD plan |
| `TestVRRPGroupMoveBackToTheNameItsSocketsUseKeepsALiveTransport` | Changed | same | ADDED by the closure review; pins the collision test on `in.key.Interface` |
| `TestNoRegistryResolvesAConfiguredNameAgainstTheKernel` | Done | same | AC-6 over every sink |
| `TestUnitDeviceResolution` | Changed | same | repointed to `VLANID`; asserts extraction binds NO device |
| `vrrp-macvlan-parent-selector` | Done | `test/vrrp/vrrp-macvlan-parent-selector.ci` | 9/9 in QEMU, discriminates |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/vrrp/groups.go` | Done | `unitVLANID`, `unitDeviceName`, `deviceResolver`, `GroupSpec.parentDevice` |
| `internal/plugins/vrrp/engine.go` | Done | the binding step, `build`/`start`, the move path |
| `internal/plugins/vrrp/register.go` | Done | `resolveDevice: iface.ResolveDevice` |
| `internal/component/iface/dispatch.go` | Done | `resolveOS` -> `ResolveDevice` |
| `internal/plugins/vrrp/groups_test.go`, `engine_test.go`, `cmd_show_test.go` | Done | 14 tests, the fake's new arms |
| `test/vrrp/vrrp-macvlan-parent-selector.ci` | Done | created |
| `test/vrrp/vrrp-instance-up.ci`, `vrrp-idle.ci`, `vrrp-doctor-quiet.ci` | Done | caps, exclusive group, doctor assertions |
| `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`, `docs/architecture/iface/logical-name-resolution.md`, `docs/guide/vrrp.md`, `ai/digests/iface.md` | Done | see Documentation Updates |
| `docs/features/interfaces.md` | Changed | edited, but NOT carried by this commit: the file holds another session's uncommitted hunks |

### Audit Summary
- **Total items:** 26
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (`TestUnitDeviceResolution` repointed, two tests added by the closure review, `docs/features/interfaces.md` left for the session that owns the rest of that file)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A VRRP group on hardware selected by `mac/match` builds its virtual-MAC device on the SELECTED NIC | functional, on a live kernel | `test/vrrp/vrrp-macvlan-parent-selector.ci`: `vrrp 9/9` in the QEMU VM. The test reads the macvlan's `iflink` from sysfs and compares it to the selected device's ifindex, and a DECOY device wears the logical name so the pre-fix daemon builds successfully on the wrong device rather than failing on absence |
| The same holds for `os-name`, and for a VLAN-tagged unit | unit | `TestVRRPParentTakesTheResolvedDevice` (one resolver answers both forms), `TestVRRPParentComposesVLANOnTheResolvedDevice` (`eth3.100`) |
| No kernel-facing value VRRP hands out is a configured interface name | unit, over every sink | `TestNoRegistryResolvesAConfiguredNameAgainstTheKernel` asserts the macvlan parent, the applied sysctls, the reasserted sysctls, the transport parent and the instance spec in one pass, and fails if any sink recorded nothing |
| The fix changes nothing for an interface with no selector | unit, regression guard | `TestVRRPParentUnselectedInterfaceIsUnchanged`: `eth0` and `eth0.100`, unchanged. It SURVIVES the mutation that reds every other test, which is the property that makes it the guard |
| Fixing the binding never costs an operator a live VIP | unit, five paths | `TestVRRPGroupSurvivesAnUnresolvedSelector`, `TestVRRPGroupEditsLandWhileItsSelectorIsUnresolved`, `TestVRRPGroupMoveKeepsTheOldRouterWhenTheRebuildFails`, `TestVRRPGroupNeverAdoptsAParentItsMacvlanIsNotOn`, `TestVRRPGroupMoveUnderOneParentNameKeepsALiveTransport`. Each is mutation-proven |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | The spec declares `Deferral shard: -` and `ls plan/deferrals/` holds no `fixit-vrrp-parent-ignores-the-selector.md`. Nothing to account for and nothing to remove |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-vrrp-parent-ignores-the-selector-9ad8358c-695f-41be-8019-5d92ba08f8e6.md` |
| `review_gate.py check` | OK (11 code files, clean, hashes match) |
| Rounds | 3 |
| Reviewer lenses used | logic + wiring + removed-behavior; security + edge cases + allocation; style (`docs/contributing/ze-style.md`) + docs + RFC 9568 |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| B-1 | BLOCKER | An unbindable group was dropped from the desired set, so the delete-first loop tore its instance down: a Priority-0 resignation and the VIP removed on one transient netlink read | `internal/plugins/vrrp/engine.go` `apply` | a running group keeps its device; AC-5 rewritten; `TestVRRPGroupSurvivesAnUnresolvedSelector` |
| R2-1 | BLOCKER | The move path released the predecessor before building the replacement, so one unrelated owned-device error destroyed a working virtual router | same | `create` split into `build` and `start`; `TestVRRPGroupMoveKeepsTheOldRouterWhenTheRebuildFails` |
| R3-1 | BLOCKER | A kernel RENAME was read as a move, so the rebuild registered and immediately unregistered one device and the next reconcile orphan-deleted it while the FSM reported master | same | the move keys on the macvlan name, composed from the parent's ifindex; `TestVRRPGroupRenameIsNotAMove` |
| R4-1 | BLOCKER | A failed `parentIfindex` let `reconfigure` adopt a `ParentDevice` this instance's macvlan is not on, pointing `parentReady` at an absent device | same | the error arm keeps `in.spec.ParentDevice`; `TestVRRPGroupNeverAdoptsAParentItsMacvlanIsNotOn` |
| R5-1 | BLOCKER | A move whose parent keeps its NAME opens the replacement under the RUNNING instance's transport key. `Transport.OpenInstance` (`internal/plugins/vrrp/transport/transport.go`) writes the new instance over the live map entry without shutting its sockets down, and `teardown`'s `CloseInstance` then closes the REPLACEMENT. The engine holds a virtual router whose sockets are shut and reports it running. Reachable whenever a netdev is replaced under one name: a card re-enumerates, a driver reloads, an iface apply recreates a VLAN device | `internal/plugins/vrrp/engine.go` `apply`, the move branch | the colliding move releases first, keyed on `in.key.Interface` because a rename adopts a new name into the spec and leaves the key on the name the sockets were opened under. `TestVRRPGroupMoveUnderOneParentNameKeepsALiveTransport` and `TestVRRPGroupMoveBackToTheNameItsSocketsUseKeepsALiveTransport`, both mutation-proven; the fake now models the transport's instance map so an overwrite is visible |
| R5-2 | NOTE | `internal/plugins/vrrp/groups.go` exports `GroupSpec`, `EffectivePriority` and `EffectiveAcceptMode` with no cross-package non-test caller, so `make ze-repository-check` reports three ISSUEs whenever the file changes | `internal/plugins/vrrp/groups.go` | not fixed: pre-existing at HEAD, where `git show` carries all three, and no `vrrp.GroupSpec` reference exists anywhere. The check only sees them because this commit touches the file. Unexporting them is `plan/deferrals/fixit-unexport-package-private-symbols.md`'s subject |
| R5-3 | NOTE | After a kernel rename, the transport instance keeps the parent name it was opened with (`InstanceSpec.Parent`, its counter labels and its IPv4 source resolution), while the engine's spec and metric labels carry the new name | `internal/plugins/vrrp/transport/transport.go` `OpenInstance` | not fixed: correcting it means closing and re-opening the sockets, which is the churn the rename branch exists to avoid. The sockets are bound to the netdev, which is unchanged, so the effect is a label that disagrees with the engine's until the next rebuild |
| R5-4 | NOTE | `internal/test/runner/exclusive_group_test.go` enumerates the clusters that MUST declare an exclusive group, and the new `iface-owned-macvlan` cluster is not one of its rows, so a future vrrp `.ci` that owns a macvlan can omit the group silently | `internal/test/runner/exclusive_group_test.go` | not fixed here: the file carries another session's uncommitted rows, so editing it would put that session's work in this commit (`ai/rules/git-safety.md`). The row belongs to whoever lands that file next: `{"vrrp", "*.ci", "option=needs-linux:caps=net-admin", "option=exclusive:group=iface-owned-macvlan", "the ze:owned: alias namespace reconcileOwnedDevices sweeps", 2}` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/vrrp/vrrp-macvlan-parent-selector.ci` | yes | `ls -l`: 8340 bytes |
| `test/vrrp/vrrp-instance-up.ci` | yes | `ls -l`: 9907 bytes |
| `test/vrrp/vrrp-idle.ci` | yes | `ls -l`: 4496 bytes |
| `test/vrrp/vrrp-doctor-quiet.ci` | yes | `ls -l`: 2094 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | the parent is the resolved device | `make ze-unit-pkg-test PKG=./internal/plugins/vrrp` ok 1.291s, race-instrumented, `TestVRRPParentTakesTheResolvedDevice` among them |
| AC-3 | the tag hangs off the resolved device | same run, `TestVRRPParentComposesVLANOnTheResolvedDevice` |
| AC-4 | an unselected interface is unchanged | same run, `TestVRRPParentUnselectedInterfaceIsUnchanged` |
| AC-5 | not started when not running, kept where it is when running | same run, five tests |
| AC-6 | no sink carries a configured name | same run, `TestNoRegistryResolvesAConfiguredNameAgainstTheKernel` |
| AC-7 | the kernel agrees | `make ze-qemu-debug NOBUILD=1 RUN='<session-bin>/ze-test-linux-amd64 vrrp -a -v'`: `pass 9/9 100.0%`, `vrrp-macvlan-parent-selector` and `vrrp-instance-up` among them, run over the tree that carries the R5-1 fix |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A VRRP group on an interface selected by `mac/match`, through `engine.apply`'s binding step | `test/vrrp/vrrp-macvlan-parent-selector.ci` | yes: read whole. `setup.py` builds the selected veth and the decoy; the config selects by MAC; `driver.py` reads the macvlan's `iflink` from sysfs and never asks a ze command, so the assertion shares no resolver with the code under test |
| The same configuration on a live kernel, through `CreateMacvlanDevice` | same | yes: `MACVLAN-PARENT` and `DECOY-UNTOUCHED` are both required, so a run that built nothing fails |
| `iface.ResolveDevice` reaches production | `internal/plugins/vrrp/register.go` `livePlatform` | yes: `resolveDevice: iface.ResolveDevice`, and `livePlatform` is the platform the engine is built with at plugin start |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken (narrow), confirmed (wide) | VRRP is the ONLY plugin-facing registry holding an untranslated name. `RegisterOwnedAddresses` has one caller, passing a device Ze composed. Two further sites of the same shape are not registries and are journal rows |
| A-2 | confirmed | `register.go` and `engine.go` already import `internal/component/iface`; the fix adds no import, because the resolver reaches `groups.go` as a func field. `make ze-unit-pkg-test PKG=./internal/component/iface` ok |
| A-3 | broken | `validateSelectors` (`internal/component/iface/config_apply.go`) refuses ambiguity alone and returns nil for zero matches. The binding moved to `engine.apply` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`: the virtual router lives on the resolved device | read against `engine.go` `apply` and `groups.go` `parentDevice` | yes |
| `docs/architecture/iface/logical-name-resolution.md`: a plugin registry takes `ResolveDevice`'s answer | read against `dispatch.go` `ResolveDevice` and `register.go` `livePlatform` | yes |
| `docs/guide/vrrp.md`: the `device` metric label carries the kernel device | read against `telemetry.go`, which labels on `spec.ParentDevice` | yes |
| No doc anchor on a changed file went stale | `grep -rn "source: internal/plugins/vrrp/groups.go\|engine.go\|register.go\|iface/dispatch.go" docs/`: 4 anchors outside the edited files, each naming a symbol this change does not touch (`vipMaskBits`, `AddressIsLocal`, `ResetCounters`, "group extraction + cross-leaf verifier") | yes |
| `make ze-doc-verify` | 2 findings, both foreign: `docs/guide/web-interface.md` anchors `liveAAABundleAuthenticator.Authenticate` in another session's uncommitted `cmd/ze/hub/aaa_authenticator_web.go`, and `rules-points: config.md is stale` belongs to the session editing `ai/rules/points/config/`. Neither names a file this spec changes | yes |
| `make ze-repository-check` | 19 ISSUEs, 3 in a file this spec changes, all three pre-existing exported-but-package-local symbols (Review Gate R5-2) | yes |
| `python3 scripts/dev/audit-test-relaxation.py` | 7 findings, 2 in this spec's paths: `test/vrrp/vrrp-idle.ci` loses one expectation (the doctor exit code, which asserted the host's NIC inventory), and `internal/plugins/vrrp/groups_test.go` is flagged whole-file because two of its `RFC requirement:` tags sit outside any func span. The first needs a row in `test/weakened.md`; the second needs the OWNER's approval row in `test/rfc-changed.md` | yes |
| `make ze-lint-changed` | one finding, `internal/component/command/pipe_catalog.go` goconst, on an UNMODIFIED line: another session's edits to `alias.go`, `answer_shape.go` and `pipe_filter.go` raised the package's `"none"` count over the threshold. This spec's own packages are clean: `golangci-lint run ./internal/plugins/vrrp/... ./internal/component/iface/...` gives 0 issues on both the host pass and the `GOOS=linux --build-tags integration` pass | yes |

## Core Insight

A resolution and a KEY derived from it are two facts, and a rule about one does
not hold for the other. "Build the replacement before releasing the predecessor"
is right about devices, whose names differ because their ifindexes do, and wrong
about transports, whose key is the parent's NAME. The safe ordering flipped on a
fact the rule never mentioned, and the code that got it wrong was itself a fix
for an outage. When a change adds an ordering rule, enumerate every resource the
two objects hold in common, one at a time, and name the one that decides.
