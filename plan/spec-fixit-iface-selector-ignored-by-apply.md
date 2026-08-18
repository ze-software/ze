# Spec: fixit-iface-selector-ignored-by-apply

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-18 |

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
| Config tree ↔ iface component | resolved tree in the configure callback | No |
| Component ↔ resolver | selector maps published by `setResolverConfig` | No |
| Component ↔ backend | direct backend calls in the apply path, dispatch wrappers everywhere else | No |
| Backend ↔ kernel | netlink address, link and ethtool operations | No |

### Integration Points
- `unitOSName` - the single naming funnel every apply-path key passes through
- `resolveOS` - the existing translation the apply path bypasses
- `setResolverConfig` - the publication step whose ordering must change

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `mac/match` entry carrying an address cannot be committed today when the kernel name differs | Phase 3 adds addresses by the logical name and the failure path fails the commit | The defect is only the wrong-port case, and the severity ordering in this spec changes | A unit test with the selector-aware fake backend, asserting the commit outcome before any fix | unvalidated |
| A-2 | A VLAN on a selected port should be named after the kernel device, not the logical name | nothing in the YANG or the docs says which; every existing test uses a logical name equal to the kernel name | The managed-VLAN prune path renames and recreates VLANs on upgrade for any entry with a selector | Owner decision, recorded here before Phase 3 starts; the chosen answer is pinned by a test | unvalidated |
| A-3 | Publishing the mapping before apply has no other consumer that depends on the current ordering | `setResolverConfig` is called once, from the configure callback, after apply | Moving it changes what a concurrent consumer sees mid-apply | Read every `setResolverConfig` caller in Phase 1 and record the answer | unvalidated |
| A-4 | Failing closed on an unresolvable selector does not break the deferred-binding promise | the deferred case is a device that is absent, not a selector that resolves elsewhere | Fail-closed validation rejects a config the YANG says is valid | The existing deferred-binding resolver test must stay green with the new validation in place | unvalidated |

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
| A commit with a `mac/match` entry whose kernel name differs | → | `desiredState` in `internal/component/iface/config_apply.go` | `TestDesiredStateKeysBySelectedDevice` |
| The reconcile adds that entry's address | → | `reconcileOnReadyWithJournal` in `internal/component/iface/config_apply.go` | `TestApplyAddressLandsOnSelectedDevice` |
| A commit with an `os-name` alias | → | the same apply path | `TestApplyAddressFollowsOsNameAlias` |
| A selector resolves to a device other than the one named | → | the fail-closed validation | `TestApplyRefusesWrongDeviceForSelector` |
| A running daemon commits a selector-bound interface | → | the whole chain to the kernel | `test/plugin/iface-mac-match-address-apply.ci` |

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
| 1 | Names an uplink `wan` and binds it to a NIC by MAC, then commits an address | config → parse → resolver → apply → netlink | `test/plugin/iface-mac-match-address-apply.ci` |
| 2 | Replaces the NIC and updates the MAC in config | config → resolver mapping → apply on the new device | `TestApplyAddressLandsOnSelectedDevice` |
| 3 | Commits a config whose selector cannot be resolved to the named device | config → validation → refused with a message naming the entry | `TestApplyRefusesWrongDeviceForSelector` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDesiredStateKeysBySelectedDevice` | `internal/component/iface/config_apply_test.go` | AC-1: the desired map is keyed by the resolved device | |
| `TestApplyAddressLandsOnSelectedDevice` | `internal/component/iface/config_apply_test.go` | AC-1 and AC-5 with a selector-aware fake backend | |
| `TestApplyAddressFollowsOsNameAlias` | `internal/component/iface/config_apply_test.go` | AC-3 | |
| `TestApplyNonAddressSettingsFollowSelector` | `internal/component/iface/config_apply_test.go` | AC-2: MTU, MAC, offloads, sysctl, admin state, mirror | |
| `TestDeferredBindingStillDefers` | `internal/component/iface/config_apply_test.go` | AC-4, validates A-4 | |
| `TestResolverMappingPublishedBeforeApply` | `internal/component/iface/register_test.go` | AC-6, validates A-3 | |
| `TestApplyRefusesWrongDeviceForSelector` | `internal/component/iface/config_apply_test.go` | AC-5 fail-closed behavior | |
| `TestVLANOnSelectedParent` | `internal/component/iface/config_apply_test.go` | AC-7, pins the A-2 answer | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VLAN id on a selected parent | 1 to 4094 | 4094 | 0 | 4095 |
| candidate devices matching one selector | 0 to the interface count | 1 is the only unambiguous count | N/A | 2 or more must not be resolved silently |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-mac-match-address-apply` | `test/plugin/iface-mac-match-address-apply.ci` | an interface bound by MAC, named differently from the kernel device, gets its address on the right port | `needs-linux` |
| `iface-osname-alias-apply` | `test/plugin/iface-osname-alias-apply.ci` | the same for an `os-name` alias | `needs-linux` |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No protocol behavior changes; this is local interface configuration | |

## Files to Modify
- `internal/component/iface/config_apply.go` - resolve once per entry and key the apply path by the selected device; distinguish a selected entry whose device is absent from an unselected one; stop adding addresses for an entry Phase 2 dropped
- `internal/component/iface/register.go` - publish the resolver mapping before the apply that depends on it
- `internal/component/iface/config_mirror.go` - build mirror specs from the selected device
- `internal/component/iface/config_sysctl.go` - build sysctl keys from the selected device
- `internal/component/iface/dispatch.go` - decide and document whether `resolveOS` may keep failing open, and make the apply path's use fail closed
- `internal/component/iface/validate.go` - validate selectors at commit: unresolvable, ambiguous, or pointing at a device other than the one named
- `internal/component/iface/config_test.go` - the fake backend must be able to represent a device whose name differs from the entry name
- `docs/architecture/iface/logical-name-resolution.md` - state that the apply path translates, and what happens when a selector cannot be resolved
- `docs/guide/configuration.md` - the `mac/match` and `os-name` sections gain the apply-path behavior and the failure modes

## Files to Create
- `internal/component/iface/config_apply_resolve_integration_linux_test.go` - integration coverage beside the existing resolver pair
- `test/plugin/iface-mac-match-address-apply.ci` - functional proof for AC-1
- `test/plugin/iface-osname-alias-apply.ci` - functional proof for AC-3

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `mac/match` and `os-name` already exist; no new node |
| YANG validation constraints | N-A | the selector values already carry their native constraints |
| YANG custom validators | Yes | an unresolvable or ambiguous selector needs a validation function; native constraints cannot see the running hardware |
| CLI commands/flags | N-A | no new command |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | Yes | a completion function offering the MACs and names present on the box would prevent the typo class this defect punishes |
| Functional test for new RPC/API | Yes | the two new `.ci` files |
| Pipe completeness | N-A | no new command output |
| Env var registration | N-A | no `environment/` leaf |
| Doctor check for runtime dependencies | Yes | a check reporting a configured selector that resolves to nothing, or to a device other than the one named, with a diagnostic code |
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
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the harness gains a way to create a device whose name differs from the entry name |
| 11 | Affects daemon comparison? | No | no comparison claim |
| 12 | Internal architecture changed? | Yes | `docs/architecture/iface/logical-name-resolution.md` and `docs/features/interfaces.md` |
| 13 | Route metadata keys added/changed? | No | no metadata key |
| 14 | Prometheus counters added/changed? | No | no counter added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/configuration.md` anchors `Resolve`, `Addresses`, `osDeviceFor`, `matchByMAC` and `deviceMatchMAC`; `docs/features/interfaces.md` anchors `resolveOS` and several apply-path symbols; `docs/architecture/core-design.md` anchors `desiredState` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `mac/match` examples must show a logical name that differs from the kernel name, since the identity case is what hid this |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the defect expressible in a test
   - Tests: `TestApplyAddressLandsOnSelectedDevice` failing for the right reason
   - Files: `internal/component/iface/config_test.go`, adding selector awareness to the fake backend
   - Verify: the fake refuses to answer by the wrong name, and the new test fails today, validating A-1
2. **Phase: enumerate every naming site** -- addresses, MTU, MAC, offloads, sysctl, admin state, mirrors, VLAN parents
   - Tests: `TestApplyNonAddressSettingsFollowSelector`
   - Files: `internal/component/iface/config_apply.go`, `config_mirror.go`, `config_sysctl.go`
   - Verify: every site is listed in this spec's Files to Modify and covered by an assertion; R-3 is closed by enumeration, not by sampling
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Resolve once per entry in the apply path | Route every backend call through the translating dispatch wrapper; rename the kernel device to the logical name | Per-call translation touches around twenty sites in three files and still misses the string-taking sysctl and ethtool paths. Renaming contradicts a documented decision, invalidates sockets bound to the device, and cannot be done safely on a live uplink |
| Fail closed on a misdirected selector, stay permissive on an absent one | Fail closed on both; keep failing open on both | The YANG promises the deferred case, so refusing it breaks a documented contract. A selector that resolves to the wrong device promises nothing and must refuse |
| Fix the publication ordering in the same spec | Leave it as a separate defect | The apply reading the previous commit's mapping makes every other fix in this spec conditional on the config not having changed, which is not a property anyone can rely on |

## Known Limitations
- Nothing here makes hardware identity survive a NIC replacement with no configuration change; the operator still updates the selector. This spec makes the selector work, not guess.
- Ambiguity is refused rather than resolved. When two candidates match, no rule can tell which is the operator's intended hardware, and choosing one is how a configured address ends up on a stranger's port.

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
