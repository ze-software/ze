# Spec: fixit-iface-selector-ignored-by-apply

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An interface entry can select its hardware by permanent MAC (`mac/match`) or
alias a kernel device (`os-name`). The resolver honors both, and roughly
twenty-eight consumers reach the right device through `iface.Resolve`. The config
apply path does not.

`desiredState` keys everything by `unitOSName`, which is the logical entry name,
and `reconcileOnReadyWithJournal` calls the backend directly rather than through
the by-name dispatch wrappers that translate. Nothing renames the kernel device,
and the architecture doc states that it never will. So the selector steers every
consumer except the one that assigns the addresses.

Two outcomes follow, and both are wrong:

- The kernel has no device with the logical name. Phase 2 skips the entry with a
  warning, then Phase 3 still tries to add the addresses, the backend answers not
  found, and the commit fails and rolls back. A `mac/match` entry that carries
  addresses cannot be committed at all.
- The kernel does have a device with that name, and it is not the selected one.
  MTU, MAC override, offloads, sysctl, admin state and every address are applied
  to the wrong physical port, with no error.

An ordering defect sits underneath both: `setResolverConfig` publishes the
mapping AFTER `applyConfig` runs, so an apply reads the previous commit's
mapping.

The selector surface fails open throughout. `resolveOS` returns the name
unchanged when resolution errors, the absent-device path treats a selected entry
exactly like an unselected one, and validation checks no selector at all.

The reason this survived: `ze init` writes `os-name` equal to the kernel name on
every entry, and the identity mapping is skipped, so the generated config hides
the defect until an operator aliases a device or a NIC moves.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - the resolver design
  → Decision: the kernel device is never renamed, so any fix that makes apply correct must translate rather than rename
  → Constraint: the resolver is documented as a map only; it holds no apply-path responsibility today, so the fix decides where translation belongs
- [ ] `docs/features/interfaces.md` - which dispatch operations translate and which are raw
  → Constraint: `Create*`, `GetInterface` and `ListInterfaces` are deliberately raw because the resolver is built on them; a fix must not route those through translation and recurse
- [ ] `docs/guide/configuration.md` - the operator-facing promise of `mac/match` and `os-name`
  → Constraint: the YANG says a binding defers until the matching device appears, so a fix must not turn a deferred binding into a commit failure
- [ ] `ai/rules/simplicity.md` - the fix must be the simplest fully correct answer
  → Decision: correctness here means the addresses land on the selected hardware; a simpler fix that only stops the wrong-port case is not sufficient

**Key insights:**
- `unitOSName` is the single naming funnel for `desiredState`, the VLAN device name, the mirror specs and the managed-VLAN recreation, which makes it the one insertion point that duplicates no resolver logic.
- Resolving inside `unitOSName` changes the VLAN device name for any entry carrying a selector, and Phase 4 prunes managed VLANs by the old name. That is a behavior change needing an explicit decision, recorded as A-2.
- The existing apply-path fake backend has no notion of a MAC-selected or aliased device, so no unit test can express this defect until the fake grows one.
- Every integration test that proves resolution works is built with the integration tag and does not run on the developer platform, which is why the apply-path gap survived.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/resolve.go` - `osDeviceFor` gives permanent-MAC selectors precedence over `os-name`, then falls back to the name; `matchByMAC` scans the interface list and takes the lowest index; `deviceMatchMAC` prefers the permanent MAC; `recordBinding` keeps the reverse map in memory only; `setResolverConfig` publishes the maps; `onLinkEvent` calls the backend inline
- [ ] `internal/component/iface/dispatch.go` - `resolveOS` translates and returns the name unchanged on any error; the by-name wrappers translate; `Create*`, `GetInterface` and `ListInterfaces` stay raw by design
- [ ] `internal/component/iface/config_apply.go` - `desiredState` keys addresses and managed devices by `unitOSName`; `unitOSName` returns the entry name or the name with the VLAN suffix; `applyConfig` runs the phases and calls `setResolverConfig` afterwards; Phase 2 drops an entry whose device is absent after a warning; Phase 3 still adds addresses for it and fails the commit; `addDesiredAddresses` logs failures at debug and returns no error
- [ ] `internal/component/iface/config.go` - `osNameMap` skips the identity mapping; `permMACMap` covers ethernet entries only
- [ ] `internal/component/iface/config_mirror.go` - mirror specs are built from the logical name
- [ ] `internal/component/iface/config_sysctl.go` - sysctl keys are built from the logical name
- [ ] `internal/component/iface/register.go` - `applyConfig` runs before `setResolverConfig`, so apply reads the previous mapping
- [ ] `internal/component/iface/validate.go` - no selector validation exists
- [ ] `internal/component/iface/resolve_test.go` - the resolver's own coverage, including the deferred-binding case that must be preserved
- [ ] `internal/component/iface/config_test.go` - the apply-path fake backend, which cannot represent a selected device
- [ ] `test/parse/iface-mac-match.ci` - schema acceptance only; its header records that live resolution is proven elsewhere

**Behavior to preserve:**
- A binding whose device is absent defers until the device appears; this is promised by the YANG and pinned by a resolver test.
- `Create*`, `GetInterface` and `ListInterfaces` stay untranslated.
- Consumers that call `iface.Resolve` keep getting the same answers.
- An entry with no selector, whose logical name is its kernel name, behaves exactly as today.

**Behavior to change:**
- Addresses, MTU, MAC override, offloads, sysctl, admin state and mirrors are applied to the device the selector names.
- The resolver mapping is published before the apply that depends on it.
- A selector that resolves to a device other than the one named no longer applies silently to the wrong port.
- A selected entry whose device is absent is distinguishable from an unselected entry whose device is absent.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A configuration commit carrying an `ethernet` entry with `mac/match` or `os-name`.
- Format at entry: the resolved config tree handed to the iface component's configure callback.

### Transformation Path
1. `parseIfaceConfig` in `internal/component/iface/config.go` records the selector on the entry and builds the selector maps
2. `applyConfig` in `internal/component/iface/config_apply.go` loads the backend and runs the phases
3. Phase 2 queries the backend by the logical name and drops the entry when it is absent
4. `desiredState` builds the address map keyed by `unitOSName`
5. `reconcileOnReadyWithJournal` diffs against the live addresses and calls the backend by that same key
6. `setResolverConfig` publishes the mapping, after every step above has already run
7. Later consumers call `iface.Resolve` and get the correct device

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ iface component | resolved tree in the configure callback | Yes -- `test/plugin/iface-osname-alias-apply.ci` drives a real commit through it |
| Component ↔ resolver | selector maps published by `setResolverConfig`, now BEFORE the apply through `applyAndPublish` | Yes -- `TestResolverMappingPublishedBeforeApply` |
| Component ↔ backend | the apply path resolves once per apply (`bindDevices`) and calls the backend by the resolved device; the dispatch wrappers keep translating for everyone else | Yes -- `TestApplyAddressLandsOnSelectedDevice`, `TestApplyNonAddressSettingsFollowSelector` |
| Backend ↔ kernel | netlink address, link and ethtool operations | Yes -- `TestApplyKeysByMACSelectedDeviceOnKernel` and the two `.ci` files run against a live kernel |

### Integration Points
- `unitOSName` - the single naming funnel every apply-path key passes through
- `resolveOS` - the existing translation the apply path bypasses
- `setResolverConfig` - the publication step whose ordering must change

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the apply path no longer keys by a name the kernel does not carry. It resolves from ONE `b.ListInterfaces()` per apply and every phase reads the same map through `deviceFor` |
| No unintended coupling (components stay isolated) | Yes | no new import edge. `doctor` reads sysfs rather than importing `iface`; `config` states the kernel name form inline, as `ISISNETValidator` already does, so `config` still imports no component |
| No duplicated functionality (extends existing, does not recreate) | Yes | `devicesWithMAC` and `deviceMatchMAC` are shared by the apply path and `resolver.matchByMAC`, which lost its own scan. The `absentPhysical` map was DELETED rather than left beside the new map (`ai/rules/no-layering.md`). The new integration test reuses `requireAddress` / `requireNoAddress` instead of adding its own |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no wire encoding here. The one listing is walked by index (`infos[i]`), and `devicesWithMAC` returns indices rather than copied `InterfaceInfo` values |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | the completion registers through the existing `yang.RegisterCompleteFn`; the validator through the existing `reg.Register`; the diagnostic codes through the existing `codes.go` catalogue. The doctor check extends `checkInterfaces`, which was already called from the post-config phase |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `mac/match` entry carrying an address cannot be committed today when the kernel name differs | Phase 3 adds addresses by the logical name and the failure path fails the commit | The defect is only the wrong-port case, and the severity ordering in this spec changes | A unit test with the selector-aware fake backend, asserting the commit outcome before any fix | confirmed -- `TestApplyAddressLandsOnSelectedDevice` fails RED before the fix with `wan add address 10.0.0.1/24: iface: add address on "wan": not found` |
| A-2 | A VLAN on a selected port should be named after the kernel device, not the logical name | the backends compose the netdev name themselves: `netlinkBackend.CreateVLAN` builds `<spec.Parent>.<VLANID>` and `vppBackendImpl.CreateVLAN` builds the same, and neither accepts a name | The managed-VLAN prune path renames and recreates VLANs on upgrade for any entry with a selector | `TestVLANOnSelectedParent` | confirmed -- not an open decision. `VLANSpec` carries no name, so the created device IS `<kernel device>.<vid>`; any other answer names a device that does not exist. The upgrade risk (R-1) is empty: before this fix a VLAN on a selected parent could not be created at all, because `netlink.LinkByName(spec.Parent)` was given the logical name |
| A-3 | Publishing the mapping before apply has no other consumer that depends on the current ordering | `setResolverConfig` is called once, from the configure callback, after apply | Moving it changes what a concurrent consumer sees mid-apply | Read every `setResolverConfig` caller in Phase 1 and record the answer | confirmed, and the defect is worse than stated: `setResolverConfig` has exactly ONE caller, `register.go` `OnConfigure`. `OnConfigApply` never calls it, so after any commit the resolver still serves the mapping the daemon booted with |
| A-4 | Failing closed on an unresolvable selector does not break the deferred-binding promise | the deferred case is a device that is absent, not a selector that resolves elsewhere | Fail-closed validation rejects a config the YANG says is valid | `TestDeferredBindingStillDefers`, and `resolve_test.go` stays green | confirmed. "Fail closed" is refusing the FALLBACK to the logical name, not refusing the commit: an unbound selector skips the entry (deferred), an AMBIGUOUS one refuses the commit |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Resolving inside `unitOSName` renames managed VLAN devices, and Phase 4 prunes the old name | A VLAN netdev disappears and reappears after an upgrade on a box using selectors | Settle A-2 before implementing; if the logical name wins for VLANs, resolve only the parent and keep the VLAN suffix on the logical name |
| R-2 | Routing more calls through translation slows the apply path with a resolver lookup per call | Apply duration grows on a box with many interfaces | Resolve once per entry in `desiredState` and pass the resolved name down, rather than translating at every call site |
| R-3 | Sysctl keys and ethtool calls take a device string, not a backend call, so they need separate handling and are easy to miss | A selected entry gets its addresses right and its sysctl wrong | Enumerate every string-taking site in Phase 2 and cover each with an assertion in the selector-aware fake |
| R-4 | The change is invisible to the default generated config, so a regression will not show up in normal use | Tests pass, operators with selectors break | The functional test must use a selector whose logical name differs from the kernel name, and the fake backend must refuse to answer by the wrong name |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every address, MTU and admin state on a box using selectors. A wrong landing puts the management address on the wrong port, which is a lockout |
| How is it reverted? | Single commit revert. No persisted state changes unless A-2 is answered in favor of kernel-named VLANs, in which case a revert renames those devices back |
| Who else touches this path? | `plan/journal/guard-added-to-one-half-of-a-pair.md` records the same file's kind-blind create steps and explicitly leaves the sibling steps open; the resolver's link-event handling is touched by the link-event spec |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A commit with a `mac/match` entry whose kernel name differs | → | `bindDevices` and `desiredState` in `internal/component/iface/config_apply.go` | `TestDesiredStateKeysBySelectedDevice` |
| The reconcile adds that entry's address | → | `reconcileOnReadyWithJournal` in `internal/component/iface/config_apply.go` | `TestApplyAddressLandsOnSelectedDevice` |
| A commit with an `os-name` alias | → | the same apply path | `TestApplyAddressFollowsOsNameAlias` |
| A `mac/match` several devices answer to | → | `validateSelectors` in `internal/component/iface/config_apply.go` | `TestApplyRefusesWrongDeviceForSelector` |
| A running daemon commits a selector-bound interface | → | the whole chain to the kernel | `test/plugin/iface-mac-match-address-apply.ci` |
| An operator runs `ze doctor` on a box whose selector names nothing | → | `checkInterfaces` and `selectedNetDevice` in `internal/component/doctor/checks_linux.go` | `TestCheckInterfacesFollowsMACMatch` |
| An operator types `os-name` in the editor | → | `osDeviceNameCompleteFn` in `internal/component/iface/validators.go` | `TestOSDeviceNameCompletionOffersPresentDevices` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An entry named differently from its kernel device, selected by `mac/match`, carrying an address | The address is present on the selected kernel device and absent everywhere else |
| AC-2 | The same entry with an MTU, a MAC override, offloads, sysctl settings, admin state and a mirror | Each lands on the selected device |
| AC-3 | An entry aliased with `os-name` | Same outcome as AC-1 and AC-2 |
| AC-4 | An entry with a selector whose device is absent | The binding defers with a warning, exactly as the YANG promises, and the commit does not fail because of it |
| AC-5 | A device exists carrying the entry's logical name while the selector points elsewhere | Nothing is applied to the coincidentally named device |
| AC-6 | A commit changes a selector | The apply uses the new mapping, not the previous commit's |
| AC-7 | A VLAN unit on a selected parent | The VLAN device is created on the selected parent, and its name follows the answer recorded in A-2 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Names an uplink `wan` and binds it to a NIC by MAC, then commits an address | config → parse → bindDevices → apply → netlink | `test/plugin/iface-mac-match-address-apply.ci` |
| 2 | Replaces the NIC and updates the MAC in config | config → the new commit's mapping → apply on the new device | `TestResolverMappingPublishedBeforeApply`, `TestApplyAddressLandsOnSelectedDevice` |
| 3 | Commits a config whose MAC selector names two devices | config → `validateSelectors` → refused with a message naming the entry and both devices | `TestApplyRefusesWrongDeviceForSelector` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDesiredStateKeysBySelectedDevice` | `internal/component/iface/config_apply_selector_test.go` | AC-1: the desired map is keyed by the resolved device | RED then GREEN |
| `TestApplyAddressLandsOnSelectedDevice` | `internal/component/iface/config_apply_selector_test.go` | AC-1 and AC-5 with a selector-aware fake backend | RED then GREEN |
| `TestApplyAddressFollowsOsNameAlias` | `internal/component/iface/config_apply_selector_test.go` | AC-3 | RED then GREEN |
| `TestApplyNonAddressSettingsFollowSelector` | `internal/component/iface/config_apply_selector_test.go` | AC-2: MTU, MAC, sysctl, admin state, mirror. Offloads take the same resolved name at `applyOffloads`, the one call site, and are proven on a live kernel because they are ethtool ioctls rather than backend calls | RED then GREEN |
| `TestDeferredBindingStillDefers` | `internal/component/iface/config_apply_selector_test.go` | AC-4, validates A-4. Two shapes: a device sharing the logical name, and no device at all (which validates A-1) | RED then GREEN |
| `TestResolverMappingPublishedBeforeApply` | `internal/component/iface/config_apply_selector_test.go` | AC-6, validates A-3. It lives beside the other selector tests rather than in a new `register_test.go`, because `applyAndPublish` is what carries the ordering contract and it lives in `config_apply.go` | RED then GREEN |
| `TestApplyRefusesWrongDeviceForSelector` | `internal/component/iface/config_apply_selector_test.go` | AC-5 fail-closed behavior | RED then GREEN |
| `TestVLANOnSelectedParent` | `internal/component/iface/config_apply_selector_test.go` | AC-7, pins the A-2 answer | RED then GREEN |
| `TestVLANOnSelectedParentBoundaryIDs` | `internal/component/iface/config_apply_selector_test.go` | the VLAN-id boundary row (1 and 4094) on a selected parent | RED then GREEN |
| `TestSelectorSkipsTheDeviceWearingAMemberAddress` | `internal/component/iface/config_apply_bridge_selector_test.go` | AC-5: a bridge wears its port's address, and the port is still the one answer | RED then GREEN |
| `TestSelectorStillRefusesTwoDevicesOwningOneAddress` | `internal/component/iface/config_apply_bridge_selector_test.go` | the exclusion above does not weaken the ambiguity refusal | GREEN (regression guard) |
| `TestBridgeMemberIsEnslavedByItsSelectedDevice` | `internal/component/iface/config_apply_bridge_selector_test.go` | AC-5 at the bridge-member naming site | RED then GREEN |
| `TestBridgeMemberDefersWhenItsHardwareIsAbsent` | `internal/component/iface/config_apply_bridge_selector_test.go` | AC-4 at the same site | RED then GREEN |
| `TestSecondApplySurvivesTheBridgeZeJustBuilt` | `internal/component/iface/config_apply_bridge_selector_test.go` | AC-5 across two applies: the bridge ze built in the first one must not make the second ambiguous | RED then GREEN |
| `TestListingFailureIsReportedAtTheLevelOfWhatItCosts` | `internal/component/iface/config_apply_bridge_selector_test.go` | a listing failure warns, and the not-ready sentinel stays at debug | GREEN (new surface) |
| `TestBridgeMemberListSurvivesOneMember` | `internal/component/iface/config_apply_bridge_selector_test.go` | the single-member leaf-list parse. Not an acceptance criterion of this spec: it pins the on-the-spot fix in `config.go` | RED then GREEN |
| `TestMirrorDestinationFollowsItsSelector`, `TestMirrorDefersWhenItsDestinationIsAbsent` | `internal/component/iface/config_mirror_test.go` | AC-5 and AC-4 at the mirror-destination naming site | RED then GREEN |
| `TestCheckInterfacesFollowsOSNameAlias`, `TestCheckInterfacesFollowsMACMatch`, `TestCheckInterfacesUnselectedEntryUnchanged` | `internal/component/doctor/checks_linux_test.go` | the doctor surface of the same defect. `TestCheckInterfacesFollowsMACMatch` carries the bridge and vlan cases: a device wearing another device's address is not a second answer | RED then GREEN (the unselected pin is a regression guard and stays green) |
| `TestOSDeviceNameValidatorScreensForm`, `TestOSDeviceNameCompletionOffersPresentDevices` | `internal/component/iface/config_osname_test.go` | the editor surface: form screening and live completion for `os-name` | GREEN (new surface, nothing to be red against) |
| `TestApplyKeysByMACSelectedDeviceOnKernel`, `TestApplyKeysByOSNameAliasOnKernel`, `TestApplyDefersAbsentSelectorOnKernel` | `internal/component/iface/config_apply_resolve_integration_linux_test.go` | the same ACs against netlink, where the composed VLAN name and the not-found errors are real | RED then GREEN |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VLAN id on a selected parent | 1 to 4094 | 4094 | 0 | 4095 |
| candidate devices matching one selector | 0 to the interface count | 1 is the only unambiguous count | N/A | 2 or more must not be resolved silently |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-mac-match-address-apply` | `test/plugin/iface-mac-match-address-apply.ci` | an interface bound by MAC, named differently from the kernel device, gets its address on the right port | RED then GREEN. Pre-fix the daemon failed its own startup config: `iface: set up "zemacwan": not found: Link not found` |
| `iface-osname-alias-apply` | `test/plugin/iface-osname-alias-apply.ci` | the same for an `os-name` alias | RED then GREEN, same pre-fix failure |
| `iface-bridge-mac-match-apply` | `test/plugin/iface-bridge-mac-match-apply.ci` | a selected port keeps its address after ze puts it in a bridge, which is the config `ze` builds itself | RED then GREEN. Pre-fix the second apply refused the selector as ambiguous |
| `iface-bridge-member-selector-apply` | `test/plugin/iface-bridge-member-selector-apply.ci` | a bridge enslaves the selected device, not the device carrying the logical name | RED then GREEN |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No protocol behavior changes; this is local interface configuration | |

## Files to Modify
- `internal/component/iface/config_apply.go` - `bindDevices` resolves once per apply from ONE listing; `deviceFor` answers every phase; `validateSelectors` refuses ambiguity; `devicesWithMAC`, `isStackedDevice` and `aggregatingDevices` do the matching, so a device wearing another device's address is never a candidate; `unitOSName` takes the kernel device; `applyAndPublish` fixes the ordering; the `absentPhysical` map is deleted
- `internal/component/iface/register.go` - all three apply sites (`OnConfigure`, `OnConfigApply`, its rollback) go through `applyAndPublish`
- `internal/component/iface/config.go` - `parseIfaceConfig` reads a bridge `member` leaf-list with `parseStringList`. A one-element leaf-list arrives as a bare string, and the `[]any` assertion read nothing, so a bridge with one member enslaved nothing and reported no error. This is a leaf-list parse defect, not a naming site: it is an on-the-spot fix with its journal row (`plan/journal/helper-bypassed-by-an-open-coded-copy.md`) and it is NOT an acceptance criterion of this spec
- `internal/component/iface/iface.go` - `InterfaceInfo.MasterIndex` carries IFLA_MASTER, the membership fact `aggregatingDevices` reads
- `internal/plugins/iface/netlink/show_linux.go` - `linkToInfo` sets `MasterIndex`, and it is the only producer of that field
- `internal/component/iface/config_mirror.go` - `mirrorSpecFor` and `mirrorDestination` are new: a mirror destination is as selectable as its source, so both sides are resolved. `indexMirrorSpecs`, `removeStaleMirrors`, `setupMirrorSpec` and `applyMirror` take the binding map
- `internal/component/iface/config_sysctl.go` - UNCHANGED. `applySysctl` already takes an `osName` string, and the caller now passes the resolved device
- `internal/component/iface/dispatch.go` - `resolveOS` returns `(string, error)` and fails closed for a name that HAS a selector; 21 call sites propagate
- `internal/component/iface/resolve.go` - `matchByMAC` shares `devicesWithMAC` and refuses ambiguity; `hasSelector` is what `resolveOS` asks
- `internal/component/iface/operation.go` - `decomposeIfaceOperations` resolves both sides against one listing, so an operation names the device its executor will configure
- `internal/component/iface/validate.go` - UNCHANGED. Selector validation cannot live here: `ValidateIfaceName` runs with no view of the hardware, and the ambiguity refusal needs the interface listing. It lives in `validateSelectors` (apply) and `checkInterfaces` (doctor)
- `internal/component/iface/validators.go` - `osDeviceNameCompleteFn` registers `os-name` completion
- `internal/component/iface/yang/ze-iface-conf.yang` - the `os-name` leaf gains `ze:validate "os-device-name"`
- `internal/component/config/validators.go`, `validators_register.go` - `OSDeviceNameValidator` screens the form of a kernel device name
- `internal/component/doctor/checks_linux.go` - `checkInterfaces` judges an entry by its selector; `selectedNetDevice`, `netDevicesWithAddress` and `hasLowerDevice` are new. `netDevicesWithAddress` counts a device only when the address it reports is its own, which is the population `devicesWithMAC` counts
- `internal/core/diagnostic/codes.go` - `doctor-iface-selector-unmatched`, `doctor-iface-selector-ambiguous`
- `internal/plugins/iface/vpp/query.go` - `detailsToInfo` sets `OsName`, which it never did (journal row: `plan/journal/zero-value-as-valid-answer.md`)
- `internal/component/iface/config_test.go` - the fake backend refuses an absent device, projects the full `InterfaceInfo`, gives a VLAN its parent's MAC, and models what a live kernel does on `BridgeAddPort`: the port names the bridge as its master, and the bridge takes the port's address
- `docs/architecture/iface/logical-name-resolution.md`, `docs/guide/configuration.md`, `docs/features/interfaces.md` - the apply-path behavior, the three selector outcomes, and the publication order

## Files to Create
- `internal/component/iface/config_apply_selector_test.go` - the AC-1..AC-7 unit coverage, on a selector-aware fake backend
- `internal/component/iface/config_apply_bridge_selector_test.go` - AC-5 over the two naming sites the first enumeration missed: a bridge member and the device wearing a member's address
- `internal/component/iface/config_apply_resolve_integration_linux_test.go` - integration coverage beside the existing resolver pair
- `test/plugin/iface-mac-match-address-apply.ci` - functional proof for AC-1
- `test/plugin/iface-osname-alias-apply.ci` - functional proof for AC-3
- `test/plugin/iface-bridge-mac-match-apply.ci` - functional proof that a selected port stays bound after ze puts it in a bridge
- `test/plugin/iface-bridge-member-selector-apply.ci` - functional proof that a bridge enslaves the selected device, not the coincidentally named one

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `mac/match` and `os-name` already exist; no new node |
| YANG validation constraints | N-A | the selector values already carry their native constraints |
| YANG custom validators | Yes | `os-device-name` (`internal/component/config/validators.go`), on the `os-name` leaf. It screens the FORM of a kernel device name only: an unresolvable selector MUST stay valid, because the YANG promises a binding that defers, and a config is validated on machines that will never run it. The ambiguity refusal lives in the apply (`validateSelectors`), which is the only place that can see the running hardware, and in `ze doctor` |
| CLI commands/flags | N-A | no new command |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | Yes | the MACs half already existed (`macAddressCompleteFn` behind `ze:validate "mac-address"`, which `mac/match` already carried). The names half is new: `osDeviceNameCompleteFn` (`internal/component/iface/validators.go`) behind the `os-device-name` validator |
| Functional test for new RPC/API | Yes | `test/plugin/iface-mac-match-address-apply.ci`, `test/plugin/iface-osname-alias-apply.ci`, `test/plugin/iface-bridge-mac-match-apply.ci`, `test/plugin/iface-bridge-member-selector-apply.ci` |
| Pipe completeness | N-A | no new command output |
| Env var registration | N-A | no `environment/` leaf |
| Doctor check for runtime dependencies | Yes | `checkInterfaces` (`internal/component/doctor/checks_linux.go`) judges an ethernet entry by its SELECTOR rather than its name. New codes `doctor-iface-selector-unmatched` (warning, the binding defers) and `doctor-iface-selector-ambiguous` (error, the apply refuses), both registered in `internal/core/diagnostic/codes.go`. It also stops calling a correct `mac/match` config a missing interface |
| Prometheus counters/metrics | No | the failure is a commit-time and boot-time condition, better served by the doctor check than a counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | the selectors already exist; this makes them work |
| 2 | Config syntax changed? | No | no syntax change; behavior and validation change |
| 3 | CLI command added/changed? | No | no command surface |
| 4 | API/RPC added/changed? | No | no API surface |
| 5 | Plugin added/changed? | No | the netlink backend is unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md`, the `mac/match` and MAC binding sections |
| 7 | Wire format changed? | No | no wire format |
| 8 | Plugin SDK/protocol changed? | No | no SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC requirement involved |
| 10 | Test infrastructure changed? | N-A | the harness gained nothing. Both `.ci` files use `interface { dummy ... }` to make the device and an `ethernet` entry to select it, which the existing format already expresses. The apply-path FAKE backend gained fidelity (see `plan/journal/fixture-encodes-an-impossible-state.md`), and that is package-internal |
| 11 | Affects daemon comparison? | No | no comparison claim |
| 12 | Internal architecture changed? | Yes | done: `docs/architecture/iface/logical-name-resolution.md` gains "The config apply path translates" and "The mapping is published before the apply"; `docs/features/interfaces.md` gains the apply-path paragraph and the `os-device-name` validator |
| 13 | Route metadata keys added/changed? | No | no metadata key |
| 14 | Prometheus counters added/changed? | No | no counter added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | every anchor still names a live symbol. `matchByMAC` and `deviceMatchMAC` survive (`matchByMAC` now delegates its scan to `devicesWithMAC`), `resolveOS` survives with a changed signature, `desiredState` survives with a new parameter. New anchors added for `bindDevices`, `deviceFor`, `validateSelectors`, `applyAndPublish`, `isStackedDevice`, `OSDeviceNameValidator` and `selectedNetDevice` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | both examples in `docs/guide/configuration.md` already used `uplink` bound to `eth0` and to a MAC, so a logical name that differs from the kernel name was already shown. What was missing is what the apply does with it, and the three-row table of what a selector naming zero, one or several devices produces |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the defect expressible in a test
   - Tests: `TestApplyAddressLandsOnSelectedDevice` failing for the right reason
   - Files: `internal/component/iface/config_test.go`, adding selector awareness to the fake backend
   - Verify: the fake refuses to answer by the wrong name, and the new test fails today, validating A-1
2. **Phase: enumerate every naming site** -- addresses, MTU, MAC, offloads, sysctl, admin state, mirrors, VLAN parents, bridge members, mirror destinations
   - Tests: `TestApplyNonAddressSettingsFollowSelector`, `TestBridgeMemberIsEnslavedByItsSelectedDevice`, `TestMirrorDestinationFollowsItsSelector`
   - Files: `internal/component/iface/config_apply.go`, `config_mirror.go`, `config_sysctl.go`
   - Verify: every site is covered by an assertion, and Files to Modify names every file those sites live in. R-3 is closed by enumeration, not by sampling. The first enumeration was short by two sites, bridge members and mirror destinations, and both were found later against a live kernel. So the check on this line is a comparison against the code, and Files to Modify is amended when it disagrees
3. **Phase: resolve once, key by the selected device** -- and settle A-2 first
   - Tests: `TestDesiredStateKeysBySelectedDevice`, `TestVLANOnSelectedParent`
   - Files: `internal/component/iface/config_apply.go`
   - Verify: the VLAN naming answer is recorded here before the code lands
4. **Phase: fix the ordering** -- publish the mapping before apply
   - Tests: `TestResolverMappingPublishedBeforeApply`
   - Files: `internal/component/iface/register.go`
   - Verify: an apply reads the mapping from its own commit
5. **Phase: fail closed** -- validation and the absent-device distinction
   - Tests: `TestApplyRefusesWrongDeviceForSelector`, `TestDeferredBindingStillDefers`
   - Files: `internal/component/iface/validate.go`, `internal/component/iface/config_apply.go`, the doctor check
   - Verify: a deferred binding still defers, and an ambiguous or misdirected selector is refused
6. **Phase: functional and integration proof**
   - Tests: the two `.ci` files and the new integration test
   - Files: `test/plugin/`, `internal/component/iface/config_apply_resolve_integration_linux_test.go`
   - Verify: reverting Phase 3 makes the address land on the wrong device and the test fails

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | Every naming site from Phase 2's enumeration is covered, not only addresses |
| Correctness | An entry with no selector behaves exactly as before, byte for byte |
| Naming | The resolved device name is computed once per entry and passed down; no site recomputes it differently |
| Data flow | Translation happens in the apply path, not by renaming the kernel device |
| Rule: `ai/rules/evidence.md` | The fail-open sites are enumerated and each is either justified or closed |
| Rule: `ai/rules/completion.md` | The sibling paths with the same defect, sysctl and offloads included, are in scope and not deferred |
| Registration over hardcoding | The doctor check and any completion function register through the existing registries |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Apply path translates | grep the apply path for direct backend calls taking an entry name |
| Ordering fixed | `TestResolverMappingPublishedBeforeApply` passes |
| Every naming site covered | the Phase 2 enumeration list matches the assertions in `TestApplyNonAddressSettingsFollowSelector` |
| Functional proof | `make ze-qemu-needs-linux-test` runs both new `.ci` files green |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail closed | An unresolvable or ambiguous selector must refuse rather than fall back to the logical name; a silent fallback is how an address reaches the wrong port |
| Lockout | The change must not make a management address unreachable on upgrade; the deferred-binding case stays permissive |
| Input validation | A MAC selector value is operator input and must be validated for form as well as for resolution |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-2 unanswered when Phase 3 starts | STOP. The VLAN naming decision is the owner's, and implementing either answer silently is the failure this row exists to prevent |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

The apply path resolves from ONE interface listing per apply rather than through
the resolver, and the reason is not performance. It is that the apply must read
the config it is applying: the rollback re-apply carries the previous config, and
the prune step must name the devices the PREVIOUS config made, under the previous
config's own selectors. Routing through the shared resolver would have given both
of those the mapping that happens to be published.

The listing is taken at the start of Phase 2, not at the top of `applyConfig`.
Phase 1 has just created devices by then, and the reconcile takes its own listing
at the same point in its own pass, so the two agree on what exists.

The doctor cannot share `devicesWithMAC`, and derives the same exclusions from
sysfs instead. The predicate takes `[]InterfaceInfo`, which only a running
backend produces, and `ze doctor` runs on a box whose daemon may be down. sysfs
states the same relation on the upper device: the kernel writes a `lower_<name>`
link for a stacked device and for an aggregator that has a member, and writes
none for a port or a veth. So `hasLowerDevice` reads one directory and answers
the question `isStackedDevice` and `aggregatingDevices` answer together.

A VLAN sub-interface had to be excluded from MAC matching. It inherits its
parent's hardware address, so a `mac/match` parent matched its own child and read
as ambiguous the moment Ze created a VLAN on it. Every host test missed this; the
live-kernel integration test caught it on the first run.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Resolve once per entry in the apply path | Route every backend call through the translating dispatch wrapper; rename the kernel device to the logical name | Per-call translation touches around twenty sites in three files and still misses the string-taking sysctl and ethtool paths. Renaming contradicts a documented decision, invalidates sockets bound to the device, and cannot be done safely on a live uplink |
| Fail closed on a misdirected selector, stay permissive on an absent one | Fail closed on both; keep failing open on both | The YANG promises the deferred case, so refusing it breaks a documented contract. A selector that resolves to the wrong device promises nothing and must refuse |
| Fix the publication ordering in the same spec | Leave it as a separate defect | The apply reading the previous commit's mapping makes every other fix in this spec conditional on the config not having changed, which is not a property anyone can rely on |

## Known Limitations
- Nothing here makes hardware identity survive a NIC replacement with no configuration change; the operator still updates the selector. This spec makes the selector work, not guess.
- Ambiguity is refused rather than resolved. When two candidates match, no rule can tell which is the operator's intended hardware, and choosing one is how a configured address ends up on a stranger's port.
- A device left behind by the DEFECT is not cleaned up. On a box where a device happened to carry an entry's logical name, the old apply made a VLAN on it (`wan.100`). After this fix the VLAN is made on the selected parent and the old one is left alone, because Ze cannot prove it made it: `previousManaged` is runtime-only and empty at boot, and deleting a manageable device Ze cannot claim is how an operator loses a device they made. The operator removes it.
- A deferred binding does not bind by itself when its device appears. `onLinkEvent` invalidates the resolver cache, and nothing re-runs the apply, so the entry stays unconfigured until the next commit or restart. That was true before this spec and is unchanged by it; what changed is that the commit now succeeds instead of failing. `ze doctor` reports the state (`doctor-iface-selector-unmatched`).
- The doctor check compares the CURRENT hardware address, which is all sysfs exposes; the daemon prefers the permanent one. They differ only on a box where something outside Ze overrode a NIC address, and there the check warns where the daemon binds. That is why the unmatched verdict is a warning naming the address it compared.

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

The code landed in three commits before this closure ran. This spec's diff is
exactly those three, plus two files the closure commit carries:

| Commit | Carries |
|--------|---------|
| `30ce55a41` | `bindDevices`, `deviceFor`, `validateSelectors`, `applyAndPublish`, `unitOSName` taking the kernel device, `resolveOS` failing closed, `matchByMAC` refusing ambiguity, `decomposeIfaceOperations` resolving both sides, `checkInterfaces` judging by selector, `detailsToInfo` setting `OsName`, the selector unit and integration tests, two `.ci` files, the architecture and feature docs |
| `be3394fba` | `devicesWithMAC`, `isStackedDevice` and `aggregatingDevices` excluding a device that wears another device's address, `MasterIndex` and its `linkToInfo` producer, the bridge-member loop moved to Phase 2a, `mirrorSpecFor` and `mirrorDestination`, the bridge selector tests, two more `.ci` files, the `parseStringList` fix for a one-element `member` leaf-list |
| `eea3b5550` | `netDevicesWithAddress` and `hasLowerDevice`, so the doctor counts a MAC only on the device that owns it |
| the closure commit | `OSDeviceNameValidator` and its `os-device-name` registration, and the two `doctor-iface-selector-*` codes in the diagnostic catalogue |

`internal/core/diagnostic/codes.go` is the one gap HEAD carried. `checkInterfaces`
emits `doctor-iface-selector-unmatched` and `doctor-iface-selector-ambiguous`
(`internal/component/doctor/checks_linux.go`, `selectedNetDevice`), and neither
code was in `builtinCodes`, so `ze explain` could not describe a code `ze doctor`
prints.

### Bugs Found/Fixed
- `detailsToInfo` (`internal/plugins/iface/vpp/query.go`) never set `OsName`, so every selector steered nothing on the VPP backend. Row in `plan/journal/zero-value-as-valid-answer.md`.
- `parseIfaceConfig` (`internal/component/iface/config.go`) read the bridge `member` leaf-list with a `[]any` assertion, so a bridge with one member enslaved nothing and reported no error. `TestBridgeMemberListSurvivesOneMember`. Row in `plan/journal/helper-bypassed-by-an-open-coded-copy.md`.
- The apply-path fake backend accepted five operations on a device it did not hold, and gave a VLAN no MAC. Row in `plan/journal/fixture-encodes-an-impossible-state.md`.
- A bridge or a bond made a `mac/match` selector ambiguous and failed the whole apply, because `devicesWithMAC` excluded only stacked devices. `aggregatingDevices` and `MasterIndex` close it.

### Documentation Updates
- `docs/architecture/iface/logical-name-resolution.md`: the apply-path translation, the bridge-member and mirror-destination sites, the ambiguity refusal, and the publication order. Anchors on `bindDevices, deviceFor, unitOSName`, on `mirrorSpecFor, mirrorDestination`, on `validateSelectors, devicesWithMAC, isStackedDevice, aggregatingDevices`, on `netDevicesWithAddress, hasLowerDevice`, and on `applyAndPublish`.
- `docs/features/interfaces.md`: the apply-path paragraph and the `os-device-name` validator, anchored on `internal/component/config/validators.go -- MACAddressValidator, OSDeviceNameValidator`.
- `docs/guide/configuration.md`: the `os-name` and `mac/match` sections, the VLAN naming rule, and the three selector outcomes, anchored on `bindDevices, deviceFor, unitOSName` and on `checkInterfaces, selectedNetDevice`. Those edits reached git in `8f0d02e81`, a shared-file commit made by another session. The content is preserved and only the attribution is off (`ai/rules/git-safety.md`).

### Deviations from Plan
- Files to Modify named `internal/component/iface/validate.go` and `config_sysctl.go`. Both are UNCHANGED, and the spec records why: selector validation cannot live in `validate.go`, which runs with no view of the hardware.
- Files to Modify was short by two naming sites, bridge members and mirror destinations. Both were found against a live kernel and the spec was amended in `eea3b5550`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first enumeration of naming sites was treated as complete by construction | Two sites were missing: the bridge-member loop and the mirror destination | The live-kernel `.ci` runs, not any host test | Implementation Step 2 now says the list is compared against the code and amended when the two disagree |
| assumption | The apply-path fake backend was treated as a faithful model of a kernel | It accepted five operations on a device it did not hold and gave a VLAN no MAC, so six tests passed on states no kernel produces | A VLAN inheriting its parent MAC made the parent match its own selector twice, caught by the integration test | The fake refuses an absent device and inherits the parent MAC. Six tests had their SETUP corrected and no assertion changed |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The apply path acts on the device the selector names | Done | `bindDevices` and `deviceFor`, `internal/component/iface/config_apply.go` | One listing per apply feeds every phase |
| The resolver mapping is published before the apply reads it | Done | `applyAndPublish`, `internal/component/iface/config_apply.go` | All three apply sites in `register.go` go through it |
| The selector surface stops failing open | Done | `resolveOS` (`dispatch.go`), `matchByMAC` (`resolve.go`), `validateSelectors` (`config_apply.go`) | A name that HAS a selector is refused rather than falling back to itself |
| A selected entry whose device is absent is distinguishable | Done | `bindDevices` maps it to the empty string; `checkInterfaces` reports `doctor-iface-selector-unmatched` | The deferred binding the YANG promises is preserved |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `bindDevices` resolves `mac/match` through `devicesWithMAC`; `desiredState(devices)` keys `addrs` by `unitOSName(device, u)`; `reconcileOnReadyWithJournal` calls `b.AddAddress(osName, addr)` on that key | `TestApplyAddressLandsOnSelectedDevice`, `TestDesiredStateKeysBySelectedDevice`, `test/plugin/iface-mac-match-address-apply.ci` |
| AC-2 | Done | The Phase 2 entry loop in `applyConfig` takes `device, _ := deviceFor(devices, e.Name)` once and hands it to `SetMTU`, `SetMACAddress`, `applySysctl`, `applyOffloads` and `applyMirror`; Phase 2c hands the same device to `SetAdminUp` | `TestApplyNonAddressSettingsFollowSelector`, `TestMirrorDestinationFollowsItsSelector` |
| AC-3 | Done | `bindDevices` case `e.OSName != "" && e.OSName != e.Name` | `TestApplyAddressFollowsOsNameAlias`, `test/plugin/iface-osname-alias-apply.ci` |
| AC-4 | Done | `bindDevices` maps an unanswered selector to the empty string, `deviceFor` returns `bound=false`, and the entry is dropped from `allEntries` before any phase touches it | `TestDeferredBindingStillDefers`, `TestApplyDefersAbsentSelectorOnKernel`, `TestBridgeMemberDefersWhenItsHardwareIsAbsent` |
| AC-5 | Done | `deviceFor` never falls back to the logical name; `devicesWithMAC` skips `isStackedDevice` and `aggregatingDevices`; `validateSelectors` refuses a MAC that several devices answer | `TestApplyRefusesWrongDeviceForSelector`, `TestSelectorSkipsTheDeviceWearingAMemberAddress`, `TestBridgeMemberIsEnslavedByItsSelectedDevice`, `TestSecondApplySurvivesTheBridgeZeJustBuilt` |
| AC-6 | Done | `applyAndPublish` calls `setResolverConfig(cfg)` and then `applyConfig(cfg, previous, b)`; `register.go` reaches it at `OnConfigure`, at `OnConfigApply` and at the rollback re-apply | `TestResolverMappingPublishedBeforeApply` |
| AC-7 | Done | `unitOSName(osName, u)` composes `<osName>.<VLANID>` and its caller passes the resolved device, so the VLAN is named after the kernel device, which is A-2's answer | `TestVLANOnSelectedParent`, `TestVLANOnSelectedParentBoundaryIDs` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The nine selector unit tests | Done | `internal/component/iface/config_apply_selector_test.go` | Present, plus `TestDecomposedOperationNamesTheSelectedDevice` |
| The seven bridge selector tests | Done | `internal/component/iface/config_apply_bridge_selector_test.go` | Present |
| The two mirror tests | Done | `internal/component/iface/config_mirror_test.go` | Present |
| The three doctor tests | Done | `internal/component/doctor/checks_linux_test.go` | Present |
| The two editor-surface tests | Done | `internal/component/iface/config_osname_test.go` | Present |
| The three integration tests | Done | `internal/component/iface/config_apply_resolve_integration_linux_test.go` | Build tag `integration && linux` |
| The four `.ci` tests | Done | `test/plugin/` | Each carries `needs-linux:caps=net-admin` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify | Done | Except `validate.go` and `config_sysctl.go`, which the spec records as deliberately UNCHANGED |
| Every file in Files to Create | Done | Verified by symbol listing in Pre-Commit Verification |

### Audit Summary
- **Total items:** 7 acceptance criteria, 4 task requirements, 26 named tests, 2 file lists
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** 2, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A `mac/match` entry carrying an address can be committed, and its address lands on the selected hardware | functional | `test/plugin/iface-mac-match-address-apply.ci`. Before the fix the daemon failed its own startup config with `iface: set up "zemacwan": not found: Link not found` |
| An `os-name` alias reaches the aliased kernel device | functional | `test/plugin/iface-osname-alias-apply.ci`, with the same pre-fix failure |
| Nothing is applied to a device that merely shares the entry's logical name | unit and functional | `TestApplyRefusesWrongDeviceForSelector`; `test/plugin/iface-bridge-member-selector-apply.ci` proves the bridge enslaves the selected device |
| A selector stays usable on the config ze builds itself | functional | `test/plugin/iface-bridge-mac-match-apply.ci`. Before the fix the second apply refused the selector as ambiguous |
| The apply reads its own commit's mapping | unit | `TestResolverMappingPublishedBeforeApply`, which is red without `applyAndPublish` |
| The same defect is answered on the operator's pre-flight surface | unit | `TestCheckInterfacesFollowsMACMatch`, `TestCheckInterfacesFollowsOSNameAlias`, `TestCheckInterfacesUnselectedEntryUnchanged` |
| Interop | N-A | No protocol behavior changes. This is local interface configuration, so `ai/rules/interop-and-goal-validation.md` requires no scenario |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| None | done | The spec metadata records `Deferral shard: -`, and no file under `plan/deferrals/` carries this spec's stem |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-iface-selector-ignored-by-apply-fa011c6d-dddd-408b-a46c-3ee25189f6a1.md`, 28 files pinned by hash |
| `review_gate.py check` | clean, exit 0: `review_gate: OK (3 code files, clean, hashes match ...)` over the three code files the closure commit carries |
| Rounds | 2. Round 1 covered the complete diff and found 0 BLOCKER, 0 ISSUE and 3 NOTEs. Round 2 covered the one file that changed after it, `internal/component/config/validators.go`, whose `OSDeviceNameValidator` doc comment was rewritten for `ai/rules/writing.md` and corrected to name the whitespace and `..` refusals the code makes. Comment only, no behavior change, 0 BLOCKER and 0 ISSUE |
| Reviewer lenses used | wiring and dead code, functional-test coverage, documentation drift, removed-behavior audit, logic correctness and fail-open guards, allocation and hot path, simplicity and altitude, ze-style, security |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No BLOCKER and no ISSUE. Three NOTEs are recorded below, and none blocks | - | - |

Three NOTEs:

1. `make ze-repository-check` reports `OSDeviceNameValidator` as having no cross-package non-test caller, together with the seventeen sibling validators in the same file. `RegisterValidators` (`internal/component/config/validators_register.go`) calls each one from the same package, and `check_cross_package_wiring` counts only cross-package callers. The Makefile documents this class and keeps the check out of `ze-precommit-verify` for exactly this reason. The finding disappears once the file leaves `git diff HEAD`.
2. `make ze-lint-changed` widened to `./...` and reported 8 findings, all in `scripts/evidence/l2tp-pppox-diag/main.go` and `scripts/evidence/l2tp-tunnel-diag/main.go`. Both are committed and neither is touched here. The class is already counted in `plan/journal/lint-contract-not-applied.md`.
3. `desiredState` merges the owned-address registry by the name a plugin registered, with no selector translation. Both producers are safe today: as112 registers on `lo`, and VRRP registers on the macvlan device it created itself (`installVIPs`, `internal/plugins/vrrp/instance.go`). A future plugin registering on a selector-bound ethernet name would key by the logical name.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/iface/config_apply_selector_test.go` | Yes | nine `Test*` functions listed by `grep -hn "^func Test"` |
| `internal/component/iface/config_apply_bridge_selector_test.go` | Yes | seven `Test*` functions |
| `internal/component/iface/config_apply_resolve_integration_linux_test.go` | Yes | three `Test*` functions |
| `test/plugin/iface-mac-match-address-apply.ci` | Yes | header read; `needs-linux:caps=net-admin` present |
| `test/plugin/iface-osname-alias-apply.ci` | Yes | committed in `30ce55a41` |
| `test/plugin/iface-bridge-mac-match-apply.ci` | Yes | committed in `be3394fba` |
| `test/plugin/iface-bridge-member-selector-apply.ci` | Yes | committed in `be3394fba` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-3, AC-5, AC-7 | The apply keys by the resolved device | `make ze-unit-pkg-test PKG=./internal/component/iface/... RACE=0` exit 0, `ok github.com/ze-software/ze/internal/component/iface 0.293s` |
| AC-2 | Every non-address setting takes the same device | the Phase 2 loop reads `device, _ := deviceFor(devices, e.Name)` once and passes it to every backend call. `allEntries` is filtered by boundness before that loop, so the discarded second return cannot be an unbound entry |
| AC-4 | A deferred binding still defers | `bindDevices` returns the empty string, `deviceFor` reports `bound=false`, and the reconcile's remove-extra loop iterates `desiredAddrs` keys only, so an unbound entry loses no address |
| AC-6 | Publication precedes apply | `grep applyAndPublish` finds exactly three production call sites, all in `register.go`, and `setResolverConfig` has one caller, which is `applyAndPublish` |
| Doctor surface | The two diagnostic codes resolve | `make ze-unit-pkg-test PKG="./internal/component/doctor/... ./internal/component/config/... ./internal/core/diagnostic/... ./internal/plugins/iface/..." RACE=0` exit 0 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A commit with a `mac/match` entry whose kernel name differs | `test/plugin/iface-mac-match-address-apply.ci` | Yes. The header states the device, the MAC and the address block it owns, and names the deferred-then-bound path it exercises |
| An `os-name` alias | `test/plugin/iface-osname-alias-apply.ci` | Yes |
| A selected port ze puts in a bridge | `test/plugin/iface-bridge-mac-match-apply.ci` | Yes |
| A bridge enslaving the selected device | `test/plugin/iface-bridge-member-selector-apply.ci` | Yes |
| `ze doctor` on a box whose selector names nothing | unit, `TestCheckInterfacesFollowsMACMatch` | Yes. `sysClassNetDir` is a variable so the test points the check at a fixture tree |
| The editor completing `os-name` | unit, `TestOSDeviceNameCompletionOffersPresentDevices` | Yes, registered through `yang.RegisterCompleteFn("os-device-name", osDeviceNameCompleteFn)` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestApplyAddressLandsOnSelectedDevice` was red before the fix with `wan add address 10.0.0.1/24: iface: add address on "wan": not found` |
| A-2 | confirmed | `VLANSpec` carries no name and both backends compose `<Parent>.<VLANID>`, so the VLAN IS named after the kernel device |
| A-3 | confirmed | `setResolverConfig` has one caller, `applyAndPublish` |
| A-4 | confirmed | Failing closed refuses the FALLBACK, never the commit. An unbound selector defers, and an ambiguous one is refused |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| The apply path translates | `docs/architecture/iface/logical-name-resolution.md` anchors `bindDevices, deviceFor, unitOSName`; all three are declared in `config_apply.go` | Yes |
| The ambiguity refusal | the same file anchors `validateSelectors, devicesWithMAC, isStackedDevice, aggregatingDevices`; all four are declared | Yes |
| The publication order | the same file anchors `applyAndPublish` and the three `register.go` sites | Yes |
| The doctor derives the same exclusions from sysfs | the same file anchors `netDevicesWithAddress, hasLowerDevice`; both are declared in `checks_linux.go` | Yes |
| The `os-device-name` validator | `docs/features/interfaces.md` and its anchor on `MACAddressValidator, OSDeviceNameValidator`; both are declared in `internal/component/config/validators.go` | Yes |
| The three selector outcomes for an operator | `docs/guide/configuration.md` names both diagnostic codes, and both are in `builtinCodes` after the closure commit | Yes |
| Categories answered No | `grep -rn "os-device-name" docs/` returns one hit, and no CLI, API, wire-format or RFC surface changed | Yes |

## Core Insight

The apply path must read the config it is applying, never the mapping that
happens to be published. The rollback re-apply carries the PREVIOUS config, and
the prune step must name the devices that config made under its OWN selectors.
That is why the fix resolves from one interface listing per apply rather than
through the shared resolver, and it is why publication had to move ahead of the
apply rather than simply be called a second time.
