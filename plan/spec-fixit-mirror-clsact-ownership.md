# Spec: fixit-mirror-clsact-ownership

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | `plan/deferrals/fixit-mirror-clsact-ownership.md` |
| Updated | 2026-08-14 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/planning.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The clsact qdisc at handle `ffff:0` on an interface is a shared resource. Two
independent Ze features attach filters to it: interface traffic mirroring
(`setupClsactMirror`, `internal/plugins/iface/netlink/mirror_linux.go`, filter
priority 1) and flow-export sampling (`SetupSampling`,
`internal/plugins/flowexport/sampling/tc_linux.go`, filter priority
`SampleFilterPriority`). Ownership of that shared qdisc is not modelled. The
flow-export side states the coexistence contract in its own doc comments and
honors it. The mirror side does not.

Four defects follow. Each was read from the producing function.

1. **A mirror is never torn down when it is removed from config.** `applyMirror`
   (`internal/component/iface/config_sysctl.go`) returns `nil` when both
   `MirrorIngress` and `MirrorEgress` are empty, and it is the only consumer of
   those two fields, from a single call site in `config_apply.go`. No delta path
   exists. The qdisc and the mirred filter stay installed and keep duplicating
   traffic after the operator deletes the config.
2. **Mirror teardown destroys flow-export sampling.** `RemoveMirror`
   (`mirror_linux.go`) deletes the whole shared clsact qdisc instead of only its
   own priority-1 filters. An active sample filter on the same interface goes
   with it. `RemoveSampling` does the correct thing in the other direction and
   removes only its own filter.
3. **A mirror cannot be configured on an interface that already samples.**
   `setupClsactMirror` calls `netlink.QdiscAdd` and does not tolerate `EEXIST`,
   so the call fails when flow-export created the clsact first. `SetupSampling`
   tolerates `EEXIST` for the same call.
4. **The mirror rollback path destroys a foreign filter.** When
   `setupClsactMirror` fails to add its filter it calls `QdiscDel(qdisc)`,
   removing a pre-existing sample filter as a side effect of undoing its own
   work.

Goal: give the shared clsact one owner or an explicit refcount, make every
teardown filter-scoped rather than qdisc-scoped, and add the missing
config-delete path so a removed mirror is actually removed.

Ze's traffic/QoS backend is NOT implicated. `applyInterface`
(`internal/plugins/traffic/netlink/backend_linux.go`) only replaces the root
qdisc and never addresses handle `ffff:0`.

Provenance: found while comparing VyOS `vyos-1x` July 2026 work. VyOS fixed the
same class in T9080 (`qos: only delete ingress qdisc when interface has an
ingress policy`), where an unconditional ingress-qdisc delete in the QoS apply
path silently destroyed an unrelated mirror/redirect qdisc.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/rules/platform-linux.md` - the fix is `//go:build linux` netlink code
  → Constraint: Linux-only code ships with an integration test that runs in the QEMU Alpine VM. "Needs hardware" is never an exemption.
  → Constraint: `//go:build integration && linux` tests run under `make ze-qemu-integration-test`, whose package list is derived by grep, so a new test in an existing package needs no Makefile edit.
- [ ] `docs/features/interfaces.md` - the design anchor named at the head of `mirror_linux.go`
  → Decision: mirroring is expressed as a tc mirred filter, not as a device-level feature, so its teardown unit is a filter.

### RFC Summaries (Scope: protocol)

Not applicable. tc/clsact is a Linux kernel interface, not a wire protocol, and no
RFC governs qdisc ownership.

**Key insights:** (minimal context to resume after compaction)
- The clsact qdisc at handle `ffff:` is shared. Both hooks (`HANDLE_MIN_INGRESS`, `HANDLE_MIN_EGRESS`) hang off the one qdisc object, so deleting the qdisc deletes every filter on both hooks.
- `ingress` and `clsact` are alternatives at the same handle. A sample filter added at `HANDLE_MIN_INGRESS` attaches to whichever of the two exists.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/iface/netlink/mirror_linux.go` (186L) - `SetupMirror` routes to `setupClsactMirror` (egress asked) or `setupIngressMirror` (ingress only). Both call `netlink.QdiscAdd` with no `EEXIST` tolerance, both roll back with `netlink.QdiscDel(qdisc)`. `RemoveMirror` calls `QdiscDel` on the clsact qdisc, then on the ingress qdisc, and never touches a filter.
- [ ] `internal/plugins/flowexport/sampling/tc_linux.go` (103L) - `SetupSampling` tolerates `EEXIST` on the same `QdiscAdd`, and `RemoveSampling` removes only its own filter (priority 100) with `netlink.FilterDel`, leaving the qdisc. This is the contract the mirror side does not honor.
- [ ] `internal/plugins/traffic/netlink/translate_linux.go` - `translateQdisc` builds every qdisc with the root handle and `Parent: netlink.HANDLE_ROOT`, the `traffic.QdiscClsact` case included. The QoS backend never addresses handle `ffff:`, so it is not a user of the shared qdisc.
- [ ] `internal/component/iface/config_sysctl.go` - `applyMirror` returns `nil` when both `MirrorIngress` and `MirrorEgress` are empty. It is the only reader of those two fields.
- [ ] `internal/component/iface/config_apply.go` - `applyConfig` carries `previous *ifaceConfig` and already uses it for delta work (`rememberPreviousManaged`, `indexTunnelSpecs`, `indexWireguardSpecs`, `indexXFRMSpecs`). `OnApply` in `internal/component/iface/register.go` passes the live previous config on every commit.

**Behavior to preserve:** (unless the user explicitly said to change it)
- `RemoveMirror` on an interface with no mirror stays a no-op that returns nil (`TestIntegrationMirrorRemoveIdempotent`).
- `RemoveMirror` leaves no qdisc behind when the mirror was the only user (`TestIntegrationMirrorRemove`).
- The `SetupMirror` and `RemoveMirror` signatures on `Backend` (`internal/component/iface/backend.go`), which the vpp backend also implements.

**Behavior to change:** (only what the user asked for)
- Mirror teardown becomes filter-scoped: the priority-1 mirred filters go, and the qdisc goes only when no filter remains on either hook.
- Mirror setup tolerates a clsact or ingress qdisc another subsystem already created.
- Setup rollback removes the filters it added, not the qdisc it found.
- `applyConfig` tears down a mirror the new config no longer asks for, or asks for differently.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator commits an interface config. `OnApply` in `internal/component/iface/register.go` reads the previous config from `activeCfg` and calls `applyConfig(cfg, previousCfg, b)`.
- Format at entry: `*ifaceConfig`, whose `unitEntry` carries `MirrorIngress` and `MirrorEgress` as destination interface names, empty when unset.

### Transformation Path
1. `applyConfig` (`internal/component/iface/config_apply.go`) builds the desired and previous mirror sets with `indexMirrorSpecs`, and `removeStaleMirrors` calls `Backend.RemoveMirror` for every interface whose mirror was dropped or changed.
2. `applyMirror` (`internal/component/iface/config_sysctl.go`) installs the desired mirror for each unit that asks for one, through `Backend.SetupMirror`.
3. `SetupMirror` / `RemoveMirror` (`internal/plugins/iface/netlink/mirror_linux.go`) translate that into tc: a mirred filter at priority 1 on the ingress hook, the egress hook, or both, on the qdisc at handle `ffff:`.
4. The kernel duplicates matched packets to the destination ifindex.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Component ↔ backend | `Backend.SetupMirror` / `Backend.RemoveMirror` (`internal/component/iface/backend.go`), implemented by the netlink and vpp backends | Yes |
| Ze ↔ kernel | rtnetlink `RTM_NEWTFILTER` / `RTM_DELTFILTER` / `RTM_DELQDISC` through `github.com/vishvananda/netlink` | Yes |
| Mirror ↔ flow-export sampling | The shared clsact qdisc at handle `ffff:`. Priority 1 is the mirror's, `SampleFilterPriority` (100) is sampling's | Yes |

### Integration Points
- `applyBackendStep` (`internal/component/iface/config_apply.go`) - the teardown pass records its undo in the same journal, so a later failure re-installs the mirror it removed.
- `interfaceExists` (`internal/component/iface/config_apply.go`) - a mirror on a device that no longer exists needs no teardown, and asking the backend for it would error.
- `netlink.FilterDel`, `netlink.FilterList`, `netlink.QdiscList` - the vendored library already offers filter-scoped removal, which `RemoveSampling` uses.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Teardown goes through `Backend.RemoveMirror`, never a direct netlink call from the component |
| No unintended coupling (components stay isolated) | Yes | The mirror side learns nothing about flow-export. It reads kernel state (remaining filters), not another plugin's bookkeeping |
| No duplicated functionality (extends existing, does not recreate) | Yes | `removeStaleMirrors` follows the `indexTunnelSpecs` delta pattern already in `applyConfig`; `unitOSName` replaces four inline spellings of the VLAN os-name |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Config apply is not a hot path; the os-name helper uses `textbuf.Buffer` as the surrounding code does |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new registry, command, or core switch. The change is inside the existing backend method and the existing apply path |

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
| A-1 | Exactly two subsystems attach to the qdisc at handle `ffff:`: mirror and flow-export sampling | `grep` for `netlink.Clsact`, `HANDLE_CLSACT`, `netlink.Ingress{`, `QdiscAdd`, `QdiscDel` over `internal/`, then reading `translateQdisc` in the traffic backend | A third user could lose filters, or ze could delete a qdisc it does not own | The grep above plus reading `translateQdisc`, which pins every QoS qdisc to `HANDLE_ROOT` | confirmed |
| A-2 | A tc filter can be deleted without deleting its qdisc | `RemoveSampling` (`internal/plugins/flowexport/sampling/tc_linux.go`) does exactly this with `netlink.FilterDel` | The fix has no shape: teardown would have to keep destroying the qdisc | `TestIntegrationMirrorRemoveKeepsForeignFilter` in QEMU | confirmed |
| A-3 | `applyConfig` has the previous config at the point the mirror is applied | Its signature is `applyConfig(cfg, previous *ifaceConfig, b Backend)`, and `OnApply` in `register.go` passes `activeCfg.Load()` | Teardown would need kernel-state reconciliation instead of a config delta | Reading both, plus `TestApplyConfigRemovesMirrorDroppedFromConfig` | confirmed |
| A-4 | "No filter remains on either hook" is a sound last-user test for the shared qdisc | The kernel is the only authority both subsystems share; neither keeps cross-process bookkeeping | An empty qdisc could be deleted under a subsystem that is about to add a filter | `TestIntegrationMirrorRemoveKeepsForeignFilter` (qdisc kept) and `TestIntegrationMirrorRemove` (qdisc removed) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A mirror whose destination changes is torn down and re-installed, so mirrored traffic stops for the duration of the apply | A packet-loss report on a mirror destination during a commit | Accepted, and it matches the delete-then-create the tunnel path already uses for a changed spec. tc filters are additive: adding the new destination would otherwise leave the old one duplicating traffic |
| R-2 | A daemon restart loses the previous config, so a mirror removed from the config file while ze was down is not torn down | `tc filter show` reports a mirred filter no config asks for after a restart | Out of scope and recorded under Known Limitations. Every interface delta in `applyConfig` has this shape, and changing it is a reconciliation design, not this fix |
| R-3 | An operator-installed filter at priority 1 on the same hook is indistinguishable from ze's mirror filter and is removed with it | An operator reports a hand-made tc filter disappearing on commit | Accepted. Priority 1 is where ze installs the mirror, so removing priority 1 removes what ze installed. `RemoveSampling` identifies its own filter the same way |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Traffic duplication on an interface whose mirror was removed, or the loss of flow-export sampling on an interface that also mirrors. Both are data-plane visible, neither drops a session |
| How is it reverted? | Single commit revert. No config migration, no state on disk, no wire format |
| Who else touches this path? | `internal/plugins/flowexport/sampling` shares the qdisc and is read-only here. The vpp backend implements the same two methods over SPAN and is untouched |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config commit that drops the `mirror` leaves from a unit | → | `removeStaleMirrors` → `Backend.RemoveMirror` | `TestApplyConfigRemovesMirrorDroppedFromConfig` |
| Config commit that changes a mirror destination | → | `removeStaleMirrors` then `applyMirror` | `TestApplyConfigRetiresChangedMirrorBeforeSetup` |
| Config commit that repeats an unchanged mirror | → | `applyMirror` only, no teardown | `TestApplyConfigKeepsUnchangedMirror` |
| Config commit that drops the mirror, against a real kernel | → | `RemoveMirror` → `netlink.FilterDel` | `TestIntegrationApplyConfigMirrorRemovedOnConfigDelete` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config with a mirror is applied, then a config without it | The mirred filter is gone from the interface and no traffic is duplicated |
| AC-2 | A mirror is removed while a filter at another priority is attached to the same qdisc | The other filter is still there, and so is the qdisc |
| AC-3 | A mirror is removed and it was the only filter on the qdisc | The filter is gone. The qdisc REMAINS, empty. AC-3 was relaxed on 2026-08-14 and the reasoning is the implementer's, not a ruling, so it is open to challenge: a teardown cannot know who created a shared qdisc, and the answer can change between the check and the delete. `RemoveSampling` leaves an empty qdisc for the same reason, so an interface with sampling configured presents both hooks empty for the length of a reconfigure, and deleting there takes a qdisc another subsystem created. Creation is already idempotent through the EEXIST branch, so an empty qdisc is adopted rather than duplicated. Only `undoMirrorSetup` deletes, because a rollback knows it created the qdisc moments earlier. Pre-creating a qdisc on every managed interface was considered and rejected: it would have ze change kernel state where nothing was configured, which is the mirror image of the defect being fixed |
| AC-4 | A mirror is set up on an interface whose clsact qdisc already exists | Setup succeeds and both filters coexist |
| AC-5 | Mirror setup fails to add its filter while a foreign filter is attached | The foreign filter survives the rollback, and the qdisc survives with it |
| AC-6 | A config changes a mirror destination | Only the new destination receives mirrored traffic |
| AC-7 | The same config is applied twice | The second apply succeeds and leaves one mirror filter per direction |
| AC-8 | `RemoveMirror` runs on an interface that never had a mirror | It returns nil and removes nothing, the qdisc of another subsystem included |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Deletes the `mirror` block from an interface and commits | config commit -> `applyConfig` -> `removeStaleMirrors` -> `RemoveMirror` -> `netlink.FilterDel` | `TestIntegrationApplyConfigMirrorRemovedOnConfigDelete` |
| 2 | Removes a mirror from an interface that also exports flow samples | `RemoveMirror` -> `netlink.FilterDel` at priority 1 only | `TestIntegrationMirrorRemoveKeepsForeignFilter` |
| 3 | Adds a mirror to an interface that already exports flow samples | `SetupMirror` -> `QdiscAdd` returns `EEXIST`, tolerated -> `netlink.FilterAdd` | `TestIntegrationMirrorSetupOnExistingClsact` |
| 4 | Changes the mirror destination and commits | config commit -> `removeStaleMirrors` -> `applyMirror` | `TestApplyConfigRetiresChangedMirrorBeforeSetup` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyConfigRemovesMirrorDroppedFromConfig` | `internal/component/iface/config_mirror_test.go` | AC-1: dropping the leaves calls `RemoveMirror` for that interface | pass |
| `TestApplyConfigRetiresChangedMirrorBeforeSetup` | `internal/component/iface/config_mirror_test.go` | AC-6: a changed destination is removed before the new one is set up | pass |
| `TestApplyConfigKeepsUnchangedMirror` | `internal/component/iface/config_mirror_test.go` | AC-7: an unchanged mirror is not torn down | pass |
| `TestApplyConfigRemovesMirrorOnVLANUnit` | `internal/component/iface/config_mirror_test.go` | AC-1 on a VLAN unit, whose os name is `<parent>.<vlan>` | pass |
| `TestApplyConfigRetiresMirrorDirectionDropped` | `internal/component/iface/config_mirror_test.go` | AC-6: dropping one direction retires the mirror before the other is re-installed | pass |
| `TestApplyConfigRemovesMirrorWhenUnitIsDisabled` | `internal/component/iface/config_mirror_test.go` | AC-1: disabling the unit is a way of asking for no mirror | pass |
| `TestApplyConfigSkipsMirrorTeardownForAbsentInterface` | `internal/component/iface/config_mirror_test.go` | A removed device needs no teardown and reports no error | pass |
| `TestApplyConfigReportsMirrorTeardownFailure` | `internal/component/iface/config_mirror_test.go` | A teardown that fails is reported, never swallowed | pass |
| `TestIndexMirrorSpecsSkipsDisabledEntries` | `internal/component/iface/config_mirror_test.go` | A disabled entry or unit is not a desired mirror, so disabling one tears it down | pass |
| `TestIndexMirrorSpecsCoversEveryInterfaceFamily` | `internal/component/iface/config_mirror_test.go` | The delta pass sees a mirror on every interface family the apply loop installs one on | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| tc filter priority | 1-65535 (`uint16`) | 1 = mirror, 100 = flow-export sampling | N/A: both are compile-time constants, not operator input | N/A |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestIntegrationApplyConfigMirrorRemovedOnConfigDelete` | `internal/component/iface/mirror_integration_linux_test.go` | The operator deletes the mirror block and commits, and the kernel stops duplicating | pass |
| `TestIntegrationApplyConfigMirrorKeepsForeignFilterOnConfigDelete` | `internal/component/iface/mirror_integration_linux_test.go` | That same commit leaves an interface that also samples still sampling | pass |
| `TestIntegrationMirrorRemoveKeepsForeignFilter` | `internal/component/iface/mirror_integration_linux_test.go` | A mirror is removed on an interface that also samples, and sampling survives | pass |
| `TestIntegrationMirrorRemoveLeavesForeignQdiscUntouched` | `internal/component/iface/mirror_integration_linux_test.go` | A teardown with nothing of its own to remove takes nothing from anybody | pass |
| `TestIntegrationMirrorSetupOnExistingClsact` | `internal/component/iface/mirror_integration_linux_test.go` | A mirror is added to an interface that already samples | pass |
| `TestIntegrationMirrorSetupIsIdempotent` | `internal/component/iface/mirror_integration_linux_test.go` | The same config applied twice leaves one filter per direction | pass |
| `TestIntegrationMirrorTwoDestinations` | `internal/component/iface/mirror_integration_linux_test.go` | Ingress and egress traffic go to different capture interfaces | pass |
| `TestIntegrationMirrorSetupRollbackKeepsForeignFilter` | `internal/plugins/iface/netlink/mirror_integration_linux_test.go` | A failed mirror setup leaves the foreign filter and the qdisc alone | pass |
| `TestIntegrationMirrorSetupRollbackRemovesTheQdiscItCreated` | `internal/plugins/iface/netlink/mirror_integration_linux_test.go` | A failed setup that created the qdisc leaves no tc state behind | pass |
| `TestIntegrationMirrorRemoveKeepsTheQdiscOfAnotherSubsystem` | `internal/plugins/iface/netlink/mirror_integration_linux_test.go` | Both halves of the ownership rule: kept with a co-attached filter, removed as the last user | pass |

### Interop Tests (Scope: protocol)

Not applicable. No wire-visible protocol behavior changes. The peer here is the
Linux kernel, and the QEMU integration tests above exercise it directly.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/plugins/iface/netlink/mirror_linux.go` - filter-scoped teardown, `EEXIST` tolerance on qdisc add, filter-scoped rollback, qdisc removal only when no filter remains
- `internal/component/iface/config_apply.go` - `unitOSName`, `allIfaceEntries`, `mirrorSpec`, `indexMirrorSpecs`, `removeStaleMirrors`, and the call to it in `applyConfig`
- `internal/component/iface/config_sysctl.go` - `applyMirror` doc comment, now that teardown is a separate pass
- `internal/component/iface/mirror_integration_linux_test.go` - the QEMU tests above
- `docs/features/interfaces.md` - the mirror section states the shared-qdisc contract

## Files to Create
- `internal/component/iface/config_mirror_test.go` - the delta unit tests

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The `mirror` container with its `ingress` and `egress` leaves already exists in `internal/component/iface/yang/ze-iface-conf.yang`. This fix changes what deleting them does, not the schema |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | No | The config editor already reaches the mirror leaves. No verb added |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | Unchanged: the existing leaves keep their completion |
| Functional test for new RPC/API | N-A | No RPC or API added. The kernel-facing behavior is proven by the QEMU integration tests listed above |
| Pipe completeness | N-A | No command output added |
| Env var registration | N-A | No env var added |
| Doctor check for runtime dependencies | No | No new runtime dependency. tc/netlink is already a dependency of the netlink backend and `doctor_linux.go` in `internal/plugins/iface/netlink` already covers it |
| Prometheus counters/metrics | No | No new observable state. A mirror is either installed or not, and `tc filter show` is the source of truth |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Mirroring already ships. This is a defect fix |
| 2 | Config syntax changed? | No | The same leaves, with delete now honored |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | The netlink backend's behavior changed, not its surface |
| 6 | Has a user guide page? | No | Mirroring has no dedicated guide page |
| 7 | Wire format changed? | N-A | No wire format |
| 8 | Plugin SDK/protocol changed? | No | `Backend` keeps both signatures |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs tc |
| 10 | Test infrastructure changed? | No | New tests use the existing `withNetNS` helper and the derived QEMU package list |
| 11 | Affects daemon comparison? | No | Feature parity is unchanged |
| 12 | Internal architecture changed? | Yes | `docs/features/interfaces.md`: the shared-qdisc ownership rule, beside the existing mirror source anchor |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registered changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/interfaces.md` anchors `mirror_linux.go` twice. Both claims are checked against the new code |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No mirror config example in `docs/` |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- the config-delete path, which does not exist at all today
   - Tests: `TestApplyConfigRemovesMirrorDroppedFromConfig`, `TestApplyConfigRetiresChangedMirrorBeforeSetup`, `TestApplyConfigKeepsUnchangedMirror`, `TestApplyConfigRemovesMirrorOnVLANUnit`, `TestApplyConfigSkipsTeardownForAbsentInterface`, `TestIndexMirrorSpecsSkipsDisabled`
   - Files: `internal/component/iface/config_apply.go`, `internal/component/iface/config_mirror_test.go`
   - Verify: the tests fail against HEAD because `applyConfig` never calls `RemoveMirror`, then pass once `removeStaleMirrors` is wired in
2. **Phase: Filter-scoped teardown** -- stop deleting the shared qdisc
   - Tests: `TestIntegrationMirrorRemoveKeepsForeignFilter`, `TestIntegrationMirrorRemove` (unchanged, must stay green), `TestIntegrationMirrorRemoveIdempotent` (unchanged)
   - Files: `internal/plugins/iface/netlink/mirror_linux.go`
   - Verify: the co-attachment test fails against HEAD because `QdiscDel` takes the foreign filter with it, then passes
3. **Phase: Setup coexistence** -- tolerate a qdisc another subsystem created, and roll back by filter
   - Tests: `TestIntegrationMirrorSetupOnExistingClsact`, `TestIntegrationMirrorSetupRollbackKeepsForeignFilter`, `TestIntegrationMirrorSetupIsIdempotent`, `TestIntegrationApplyConfigMirrorRemovedOnConfigDelete`
   - Files: `internal/plugins/iface/netlink/mirror_linux.go`, `internal/component/iface/mirror_integration_linux_test.go`
   - Verify: setup on a pre-existing clsact fails against HEAD with `EEXIST`, then passes

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a named test |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness: fail closed | Every path that cannot establish "no filter remains" leaves the qdisc. A `FilterList` error must never fall through to `QdiscDel` |
| Correctness: both qdisc kinds | The ingress qdisc gets the same treatment as clsact. `setupIngressMirror` rollback and `RemoveMirror` must not delete it while a filter of another subsystem hangs off `HANDLE_MIN_INGRESS` |
| Correctness: no filter leak | A path that stops deleting the qdisc must delete the filters it added, or it trades one defect for another |
| Naming | The teardown pass reads as a delta pass beside `indexTunnelSpecs`, not as a second mirror API |
| Data flow | The component asks the backend to remove a mirror. It never speaks netlink, and it never learns about flow-export |
| Rule: `ai/rules/platform-linux.md` | Every changed linux-only branch is exercised by a test that runs in QEMU, and the QEMU run is pasted |
| Rule: `ai/rules/interop-and-goal-validation.md` | Each new test is shown red against the reverted half. A co-attachment test that never attaches a second filter proves nothing |
| Rule: `ai/rules/stale-comments.md` | `applyMirror`'s comment no longer claims it is the whole mirror story, and `RemoveMirror`'s comment states the ownership rule |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| Mirror teardown removes filters, not the shared qdisc | `grep -n QdiscDel internal/plugins/iface/netlink/mirror_linux.go` shows only the guarded last-user call |
| The config-delete path exists | `make ze-test-pkg PKG=./internal/component/iface` runs the six `config_mirror_test.go` tests |
| The kernel behavior is proven | `make ze-qemu-integration-test` runs the mirror integration tests in the Alpine VM |
| Each half is proven discriminating | The reverted-half runs are pasted in Goal Validation |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | Interface names still go through `iface.ValidateIfaceName` before any netlink call, on both the setup and the teardown path |
| Traffic exposure | A mirror copies every packet on an interface to another one. Failing to tear it down is a data-leak the operator asked to stop, which is what AC-1 closes |
| Denial of service on a neighbor feature | Deleting the shared qdisc silently disables flow-export sampling. AC-2 makes that impossible from the mirror side |
| Error paths | A teardown error is reported, never swallowed. A not-found error stays tolerated, because the desired state is reached |

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

- **The kernel already holds the refcount.** Ownership of a shared qdisc looked
  like it needed bookkeeping across two plugins in two processes. It does not:
  "does any filter remain on either hook" is the same question, and the kernel
  answers it. `netlink.FilterList` on both hooks after the mirror's own filters
  are gone is the whole mechanism.
- **A defect hid behind the one under investigation.** `applyMirror` calls
  `SetupMirror` twice when the two directions have different destinations. The
  first call installed the older `ingress` qdisc, which has no egress hook, so
  the second call could not work whatever `EEXIST` did. Using clsact for every
  mirror removes the qdisc-kind choice and the failure with it.
- **`RemoveMirror` had to become filter-scoped before the config-delete path
  could be added.** Wiring teardown to a `RemoveMirror` that deletes the shared
  qdisc would have turned "the operator removed a mirror" into "the operator
  lost flow-export sampling", so the second half is a precondition of the first.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The last-user test is "no filter remains on either hook", read from the kernel | A refcount shared by the mirror and flow-export; a ze-owned registry of qdisc users | Both alternatives add cross-plugin state that can go stale, and neither is more accurate than the kernel's own answer. The kernel is the one authority both subsystems already share |
| Delete the qdisc only when the mirror actually had a filter there | Always run the last-user test | A mirror that installed nothing owns nothing. Without this, `RemoveMirror` on an interface ze never mirrored could delete an empty qdisc another subsystem created and was about to use |
| Every mirror uses clsact, and `setupIngressMirror` is deleted | Keep both qdisc kinds and pick per direction | clsact carries both hooks. Keeping the ingress qdisc kept a case that cannot work (two destinations) and doubled every teardown path. `ai/rules/no-layering.md`: the old path is deleted, not left beside the new one |
| Teardown is a delta pass over the previous config, before the install loop | Give `applyMirror` the previous unit and let it decide | A unit removed from the config is never visited by the install loop, so the pass has to walk the previous config anyway. Once it does, splitting teardown between two places buys nothing |
| A changed mirror is removed and re-installed | Install the new destination over the old one | tc filters are additive. `FilterAdd` at the same priority does not retire the previous destination, it fails or stacks. Delete-then-create is what the tunnel path already does for a changed spec |
| The rollback of a failed setup leaves a qdisc it did not create | Always delete the qdisc on rollback, as the old code did | Deleting a qdisc the setup found rather than made destroys another subsystem's filters. The setup tracks whether it created the qdisc and undoes only its own work |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- **A restart forgets the previous config.** `OnConfigure` applies with a nil
  previous config, so a mirror deleted from the config file while ze was down is
  not reconciled away. Every interface delta in `applyConfig` has this shape:
  changing it means reconciling kernel state rather than config state, which is a
  design of its own and not this defect.
- **Priority 1 identifies the mirror.** An operator's hand-made filter at
  priority 1 on the same hook is removed with the mirror. Flow-export sampling
  identifies its own filter the same way, at priority 100.
- **The VPP backend is untouched.** It implements the same two methods over SPAN,
  which has no shared qdisc. The config-delete path reaches it through
  `Backend.RemoveMirror`, so VPP gains the teardown without a VPP-side change.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
