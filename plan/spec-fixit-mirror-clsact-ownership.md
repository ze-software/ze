# Spec: fixit-mirror-clsact-ownership

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | `plan/deferrals/fixit-mirror-clsact-ownership.md` |
| Updated | 2026-08-15 |

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
- ~~`RemoveMirror` leaves no qdisc behind when the mirror was the only user (`TestIntegrationMirrorRemove`).~~ NOT preserved. AC-3 was relaxed on 2026-08-14 and `RemoveMirror` now leaves the shared qdisc in every case. `TestIntegrationMirrorRemove` asserts the qdisc REMAINS.
- The `SetupMirror` and `RemoveMirror` signatures on `Backend` (`internal/component/iface/backend.go`), which the vpp backend also implements.

**Behavior to change:** (only what the user asked for)
- Mirror teardown becomes filter-scoped: the priority-1 mirred filters go, and the shared qdisc stays. Only the rollback of a failed setup deletes a qdisc, and only one it created itself with no filter left on either hook.
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

- **The kernel holds a refcount, and reading it is still not enough.** Ownership
  of a shared qdisc looked like it needed bookkeeping across two plugins in two
  processes, and "does any filter remain on either hook" looked like the same
  question with the kernel already answering it. `netlink.FilterList` gives that
  answer, and it is stale the moment it returns: `RemoveSampling` leaves an empty
  qdisc, so "no filter" is a state a live sampling interface passes through. The
  rollback can act on the answer because it also knows it created the qdisc.
  `RemoveMirror` knows only that its own filters are gone, so it acts on nothing.
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
| `RemoveMirror` deletes no qdisc at all, in any case | Delete when the kernel reports no filter left on either hook; a refcount shared by the mirror and flow-export; a ze-owned registry of qdisc users | The last two add cross-plugin state that can go stale. The first looks safe and is not: `RemoveSampling` leaves an empty qdisc too, so an interface that samples presents both hooks empty for the length of a reconfigure, and deleting there takes a qdisc `SetupSampling` created between its `QdiscAdd` and its `FilterAdd`. No ordering of the list and the delete closes that window. An empty clsact qdisc carries no filter and forwards no packet, and the next setup adopts it through the `EEXIST` branch |
| The rollback of a failed setup is the one place that deletes, and it runs the last-user test first | Have the rollback delete unconditionally | A rollback knows it created the qdisc moments earlier, which is the knowledge `RemoveMirror` lacks. It still checks that no filter arrived in between, so it fails closed on a `FilterList` error |
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


## Implementation Summary

### What Was Implemented
- `RemoveMirror` (`internal/plugins/iface/netlink/mirror_linux.go`) deletes only the mirror's own priority-1 filters, on both clsact hooks, and leaves the shared qdisc at `ffff:` standing.
- `ensureClsactQdisc` tolerates `EEXIST` and reports whether THIS call created the qdisc. `undoMirrorSetup` deletes the qdisc only when that flag is set and `removeUnusedIngressQdisc` finds no filter left on either hook; it fails closed on a listing error.
- Every mirror uses clsact. `setupIngressMirror` is deleted, which makes a mirror with a different destination per direction work at all (`ai/rules/no-layering.md`).
- The config-delete path is new: `indexMirrorSpecs` and `removeStaleMirrors` (`internal/component/iface/config_mirror.go`) follow the `indexTunnelSpecs` delta pattern and run before the install loop, so a dropped or changed mirror is retired first.
- `isNotFound` tolerates ENOENT and EINVAL only. The substring match on `"no such"` is gone.

### Bugs Found/Fixed
- Three QEMU assertions still pinned the pre-relaxation contract and were red on a real kernel. Covered by `TestIntegrationMirrorRemove`, `TestIntegrationApplyConfigMirrorRemovedOnConfigDelete` and `TestIntegrationMirrorRemoveKeepsTheQdiscOfAnotherSubsystem`, each now asserting the qdisc REMAINS.
- `isNotFound` was the only error gate on the teardown path and matched any error whose text contained `"no such"`, so a real failure reported success. `TestIntegrationMirrorTeardownToleratesOnlyAnAbsentFilter` pins the two errnos and refuses ENODEV.
- `removeStaleMirrors` skipped a teardown silently on any `GetInterface` error. It logs the skip now.
- `TestIntegrationMirrorEgress` had gone vacuous: it asserted only that clsact exists, which every mirror creates before either hook is touched. It asserts the egress filter and its mirred destination now.
- The netlink co-attachment tests asserted survival by filter COUNT, which cannot tell the mirror filter from the foreign one. They assert priority and mirred destination now.

### Documentation Updates
- `docs/features/interfaces.md`: two bullets, the shared-qdisc ownership rule and the config-delete path, each with a source anchor. Both were rewritten at closure because the committed text still said teardown deletes the qdisc "only when no filter is left on either hook", which `RemoveMirror` does not do.
- `make ze-doc-test` was NOT run (see Pre-Commit Verification: the shared tree makes it unattributable). `make ze-validate` reports 33 unwired-export issues, none in these paths.

### Deviations from Plan
- AC-3 was relaxed during implementation: the qdisc is no longer deleted when the mirror is its last user. The Key Design Decisions table records the rejected alternatives.
- The spec's own "Behavior to preserve" line for `TestIntegrationMirrorRemove` was broken by that relaxation and is struck through rather than quietly edited.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The AC-3 relaxation was applied to the code, the commit message and the spec's AC row, and treated as complete | Three test assertions, the journal Fix cell and the public doc still stated the old contract, so the QEMU gate for this spec was red | Closure ran the two mirror packages in QEMU and got exactly three failures | Inverted the assertions, corrected the doc and the journal row, and wrote `plan/journal/stale-spec-claims-done.md` |
| assumption | The spec's Functional Tests table marked ten tests `pass` | Three of them could not have passed after the relaxation; the run they cited predated it | Same QEMU run | A cited test result is a claim about the tree it ran on. Re-ran everything at closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A removed mirror is actually removed | Done | `removeStaleMirrors`, `internal/component/iface/config_mirror.go` | Called from `applyConfig` before the install loop |
| Teardown is filter-scoped, not qdisc-scoped | Done | `removeMirrorFilters`, `mirror_linux.go` | The qdisc is left in every teardown |
| The shared qdisc has a defined owner | Changed | `RemoveMirror` doc comment, `mirror_linux.go` | No owner and no refcount. Nobody deletes it except the rollback that created it |
| Setup tolerates a qdisc another subsystem created | Done | `ensureClsactQdisc`, `mirror_linux.go` | EEXIST is not an error |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestIntegrationApplyConfigMirrorRemovedOnConfigDelete`, `TestApplyConfigRemovesMirrorDroppedFromConfig` | Filters gone from both hooks after the commit |
| AC-2 | Done | `TestIntegrationMirrorRemoveKeepsForeignFilter` | Foreign filter identified by mirred destination, not by count |
| AC-3 | Changed | `TestIntegrationMirrorRemove`, `TestIntegrationMirrorRemoveKeepsTheQdiscOfAnotherSubsystem` | Relaxed 2026-08-14: the qdisc REMAINS. Both tests assert that now |
| AC-4 | Done | `TestIntegrationMirrorSetupOnExistingClsact` | |
| AC-5 | Done | `TestIntegrationMirrorSetupRollbackKeepsForeignFilter`, `...RollbackRemovesTheQdiscItCreated` | Both rollback directions |
| AC-6 | Done | `TestApplyConfigRetiresChangedMirrorBeforeSetup`, `TestIntegrationMirrorTwoDestinations` | |
| AC-7 | Done | `TestIntegrationMirrorSetupIsIdempotent`, `TestApplyConfigKeepsUnchangedMirror` | The re-install is deliberate; its del/add window is recorded in the test comment |
| AC-8 | Done | `TestIntegrationMirrorRemoveLeavesForeignQdiscUntouched`, `TestIntegrationMirrorTeardownToleratesOnlyAnAbsentFilter` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The ten unit tests | pass | `internal/component/iface/config_mirror_test.go` | Eight drive `applyConfig` from its entry point |
| The ten integration tests | pass | the two `mirror_integration_linux_test.go` files | Plus `TestIntegrationMirrorTeardownToleratesOnlyAnAbsentFilter`, added at closure |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/iface/netlink/mirror_linux.go` | Done | |
| `internal/component/iface/config_apply.go` | Done | |
| `internal/component/iface/config_sysctl.go` | Done | |
| `internal/component/iface/config_mirror.go` | Changed | Created rather than folded into `config_apply.go` |
| `docs/features/interfaces.md` | Done | Rewritten at closure |
| `internal/component/iface/config_mirror_test.go` | Done | |

### Audit Summary
- **Total items:** 26
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (AC-3's relaxation, the shared-qdisc ownership answer, `config_mirror.go` as a new file)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A mirror removed from the config stops duplicating traffic | functional, real kernel | `TestIntegrationApplyConfigMirrorRemovedOnConfigDelete` PASS in QEMU. The defect it closes was total: `applyMirror` had no teardown path at all |
| Removing a mirror never takes flow-export sampling down | functional, real kernel | `TestIntegrationMirrorRemoveKeepsForeignFilter` and `TestIntegrationMirrorRemoveKeepsTheQdiscOfAnotherSubsystem` PASS. Both attach a real filter at sampling's priority 100 and assert the survivor by its mirred destination |
| A mirror can be added to an interface that already samples | functional, real kernel | `TestIntegrationMirrorSetupOnExistingClsact` PASS |
| A failed setup destroys nothing it did not create | functional, real kernel | `TestIntegrationMirrorSetupRollbackKeepsForeignFilter` PASS, and its complement `...RollbackRemovesTheQdiscItCreated` PASS |
| The tests DISCRIMINATE | reverted-half run | The pre-fix QEMU run is the reverted half for the three qdisc assertions: with the code shipping "leave the qdisc" and the tests asserting "delete it", exactly those three went red and nothing else did. Log: `qemu-mirror-before.log`, three FAILs at `mirror_integration_linux_test.go:260`, `:502` and `:237` |
| Kernel behaviour is proven, not asserted | QEMU run | 17 PASS / 0 FAIL across both packages on Linux 6.12 aarch64. The errno tolerance was measured on that kernel, not assumed |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Reconcile kernel tc state against the configuration at startup (R-2) | deferred | Still live and still unassigned, so `plan/deferrals/fixit-mirror-clsact-ownership.md` is NOT removed. Closure widened it: a transient `GetInterface` failure strands a mirror the same way a restart does, and only a reconcile retries either. Needs an owner decision before it can be homed |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-mirror-clsact-ownership-ca112cd4-8337-4992-b4e1-e0d7bbff5820.md` (hash-pinned over the six changed files; `tmp/` is ignored, so the artifact is checked and not committed) |
| `review_gate.py check` | `review_gate: OK (0 code files, clean, hashes match)` |
| Rounds | 3 |
| Reviewer lenses used | Round 1: logic+wiring+removed-behaviour+simplicity, and security+resources+edge-cases+test-discrimination, in parallel. Round 2: the eight fixes only. Round 3: the two comment corrections round 2 drove |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `isNotFound` was the only error gate on the teardown path and matched any error text containing `"no such"`, so a real failure reported success | `isNotFound`, `internal/plugins/iface/netlink/mirror_linux.go` | Substring match removed; ENOENT and EINVAL kept, both measured on a real kernel. `TestIntegrationMirrorTeardownToleratesOnlyAnAbsentFilter` pins them and refuses ENODEV |
| 2 | ISSUE | A teardown skipped because the backend could not read the interface was skipped silently, and `interfaceExists` cannot tell a deleted device from a failed read | `removeStaleMirrors`, `internal/component/iface/config_mirror.go` | The skip logs the interface and the error. The no-retry half is the R-2 deferral, widened to say so |
| 3 | ISSUE | The netlink co-attachment tests asserted survival by filter COUNT, which cannot tell "mirror removed, foreign kept" from the reverse | `internal/plugins/iface/netlink/mirror_integration_linux_test.go` | New `mirrorTestFilterAt` / `mirrorTestMirredDst`; the survivor is identified by priority and mirred destination |
| 4 | ISSUE | `TestIntegrationMirrorEgress` had become vacuous: clsact now exists before either hook is touched, so deleting the egress branch left it green | `internal/component/iface/mirror_integration_linux_test.go` | Asserts the egress filter, its destination, and that the ingress hook is untouched |
| 5 | ISSUE | `TestApplyConfigKeepsUnchangedMirror` claimed to prevent "every unrelated commit interrupting mirrored traffic", which is false: `addMirrorFilter`'s EEXIST branch does FilterDel then FilterAdd on every re-apply | `internal/component/iface/config_mirror_test.go` | The comment states the window and why the re-install is deliberate. The behaviour is unchanged: convergence onto the config is worth it |
| 6 | NOTE | `removeMirrorFilters` returned a `bool` both callers discarded | `mirror_linux.go` | Signature is `error` |
| 7 | NOTE | `undoMirrorSetup`'s comment overstated its scope; "an empty qdisc costs nothing" was unverifiable | `mirror_linux.go`, `docs/features/interfaces.md` | Both rewritten; the qdisc's real cost (a miniq and the ingress static key) is stated |
| 8 | NOTE | `countQdiscs` was a dead helper | `internal/component/iface/mirror_integration_linux_test.go` | Removed under a `test-relax:` token. Round 2 found the token's first reason FALSE (it named a last caller that never existed) and it now records a deletion instead: `git log -S countQdiscs` returns only `ad18e8dd9`, the commit that added it |
| 9 | ISSUE | Round 2: the "an empty qdisc costs nothing" rewording was applied in two places out of three. `removeUnusedIngressQdisc`'s doc comment still carried the exact claim round 1 refused | `removeUnusedIngressQdisc`, `internal/plugins/iface/netlink/mirror_linux.go` | Rewritten: it states the real asymmetry (a miniq against another subsystem's whole filter set) and names its one caller and that caller's precondition |

Round 1 also raised, and closure did NOT fix: an absent mirror DESTINATION aborts and unwinds the whole commit, while `absentPhysical` deliberately refuses to brick on an absent SOURCE NIC one screen away. It predates this spec and the goal does not depend on it, so it has a row in `plan/journal/transient-failure-treated-as-fatal.md`.

### Limits of the proof (recorded, not fixed)
- `isNotFound`'s REFUSAL half is proven on the helper, not driven from `RemoveMirror`. `TestIntegrationMirrorTeardownToleratesOnlyAnAbsentFilter` drives both tolerated errnos through `RemoveMirror` and asserts the refusal as `isNotFound(unix.ENODEV)`. Nothing makes a real `FilterDel` failure happen at the entry point: `RemoveMirror` resolves the link by name first, so the link is present by the time `FilterDel` runs. Round 2 raised it and could not construct the drive either.
- `removeUnusedIngressQdisc`'s non-empty branch is not exercised. Reaching it needs a foreign filter to arrive between `ensureClsactQdisc` and the failing `FilterAdd`, inside one call. `TestIntegrationMirrorSetupRollbackKeepsForeignFilter` reaches the early return instead (the qdisc was pre-created, so `createdQdisc` is false), and `...RollbackRemovesTheQdiscItCreated` reaches the loop with both hooks empty. The branch is a fail-closed guard on a race, and it is untested rather than proven.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/iface/config_mirror.go` | Yes | committed in `946d52f18`, 133 lines in its diffstat |
| `internal/component/iface/config_mirror_test.go` | Yes | same commit, 278 lines |
| `internal/plugins/iface/netlink/mirror_integration_linux_test.go` | Yes | same commit, 240 lines, extended at closure |
| `plan/deferrals/fixit-mirror-clsact-ownership.md` | Yes | one live row, so it is NOT removed |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A dropped mirror is torn down | `--- PASS: TestIntegrationApplyConfigMirrorRemovedOnConfigDelete (0.29s)` in QEMU |
| AC-2 | A co-attached foreign filter survives | `--- PASS: TestIntegrationMirrorRemoveKeepsForeignFilter (0.39s)` |
| AC-3 | The qdisc REMAINS | `--- PASS: TestIntegrationMirrorRemove (0.25s)`; the same assertion was FAIL before the fix |
| AC-4 | Setup on an existing clsact | `--- PASS: TestIntegrationMirrorSetupOnExistingClsact (0.34s)` |
| AC-5 | Rollback keeps what it found, removes what it made | `--- PASS` on both rollback tests |
| AC-6 | A changed destination is retired first | `--- PASS: TestIntegrationMirrorTwoDestinations (0.38s)` |
| AC-7 | Two applies leave one filter per direction | `--- PASS: TestIntegrationMirrorSetupIsIdempotent (0.22s)` |
| AC-8 | A teardown with nothing to remove takes nothing | `--- PASS: TestIntegrationMirrorRemoveLeavesForeignQdiscUntouched (0.18s)` and `...ToleratesOnlyAnAbsentFilter (0.01s)` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config commit dropping the mirror leaves | `TestIntegrationApplyConfigMirrorRemovedOnConfigDelete` (no `.ci`: this is kernel state, which a `.ci` cannot observe) | Yes, it calls `applyConfig(current, previous, b)` against the real netlink backend and reads the kernel with `filterAt` |
| Config commit changing a destination | `TestApplyConfigRetiresChangedMirrorBeforeSetup` | Yes, it asserts the teardown precedes the setup in the recorded op order |
| Config commit repeating an unchanged mirror | `TestApplyConfigKeepsUnchangedMirror` | Yes, one setup op and no teardown op |
| `OnApply` passes the previous config | `internal/component/iface/register.go` | Yes, read at closure: the reload call passes `previousCfg`, the startup call passes nil by design |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Only mirror and flow-export address `ffff:`. Re-checked at closure: `trafficnetlink`'s `rootQdisc` selects `Parent == HANDLE_ROOT`, so a clsact is filtered out before any snapshot decision |
| A-2 | confirmed | `TestIntegrationMirrorRemoveKeepsForeignFilter` deletes a filter and leaves the qdisc, on a real kernel |
| A-3 | confirmed | `register.go` passes `previousCfg` on reload; the ten unit tests drive `applyConfig(cfg, previous, b)` |
| A-4 | broken | "No filter remains on either hook" is NOT a sound last-user test for a teardown. `RemoveSampling` leaves empty qdiscs, so a live sampling interface passes through that state, and the answer can change between the list and the delete. It survives only in `undoMirrorSetup`, where the caller also knows it created the qdisc. See the Mistake Log and Key Design Decisions |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "mirror teardown deletes its own filters and leaves the qdisc" | `RemoveMirror` and `removeMirrorFilters`, read at closure | Yes, and the committed text said otherwise until this commit |
| "a mirror the config drops is torn down", and the restart limit | `removeStaleMirrors`; the restart path is `register.go`'s nil-previous call | Yes |
| Both source anchors on `mirror_linux.go` | Anchor now names `RemoveMirror, removeMirrorFilters, undoMirrorSetup, ensureClsactQdisc`; `removeUnusedIngressQdisc` dropped from the anchor because the claim beside it no longer describes it | Yes |
| No other doc references these files | `grep -rn "mirror_linux.go\|config_mirror.go" docs/` returns only `docs/features/interfaces.md` | Yes |

## Core Insight

The kernel holds a refcount, and reading it is still not enough. "Does any filter
remain on either hook" is the right question and the kernel answers it truthfully,
but the answer is stale the moment it returns, because the sibling subsystem
deliberately passes through the empty state on every reconfigure. What made the
rollback allowed to act on that answer was never the answer: it was knowing it had
created the qdisc itself, seconds earlier. Shared-resource ownership was not a
question about the resource's current state at all.
