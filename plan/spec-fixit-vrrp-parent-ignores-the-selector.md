# Spec: fixit-vrrp-parent-ignores-the-selector

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-22 |

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
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | VRRP is not the only plugin-facing registry keyed by the logical name | the closing review of `spec-fixit-iface-selector-ignored-by-apply` reported two, and named one | the spec is one file and Phase 1 is wasted | the Phase 1 enumeration, run with `gopls references` over `iface.Resolve` and over every consumer of a configured interface name | unvalidated |
| A-2 | The vrrp plugin can reach `iface.Resolve` without a tier violation | the resolver lives in `internal/component/iface`, and `internal/plugins/` may depend on components | the fix needs the resolved device passed in rather than looked up, which changes the plugin's input | `make ze-tier-check` after a trial import | unvalidated |
| A-3 | A selector answering no device at group-build time is a configuration error rather than a transient absence | the sibling spec refuses ambiguity at apply and drops an unanswered selector from the applied set | refusing the group breaks a valid boot ordering where the hardware appears late | the sibling spec's `validateSelectors` behavior, read whole, and a boot-ordering test | unvalidated |

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
| A VRRP group on an interface selected by `mac/match` | → | `unitDevice` | `TestVRRPParentTakesTheResolvedDevice` |
| The same configuration on a live kernel | → | `CreateMacvlanDevice` | `test/iface/vrrp-macvlan-parent-selector.ci` | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A VRRP group under an interface whose hardware is selected by `mac/match` | The macvlan parent is the device the selector answered, not the logical name |
| AC-2 | The same, with `os-name` aliasing a kernel device | Same |
| AC-3 | The same, on a unit carrying a `vlan-id` | The parent is `<resolved-device>.<vid>` |
| AC-4 | An interface with no selector | The parent is its own name, exactly as today |
| AC-5 | A selector answering no device, or more than one | The group is refused with an error naming the interface and the selector, and no macvlan is created on any device |
| AC-6 | Every plugin-facing registry found in Phase 1 | Each takes the resolved device, and none resolves a configured name against the kernel itself |
| AC-7 | AC-1 on a live kernel | The created macvlan's parent index is the selected device's index |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Pins an interface to a NIC by permanent MAC, then configures VRRP on it | config tree → vrrp plugin → iface backend → kernel | `test/iface/vrrp-macvlan-parent-selector.ci` | <!-- doc-links: ignore (this spec's AC-7 creates the file; the spec is ready and not yet authorised to run) -->
| 2 | Moves the NIC to a different slot so the kernel renames it, and reboots | same path, same selector, new kernel name | the same test, with the kernel name changed between runs |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVRRPParentTakesTheResolvedDevice` | `internal/plugins/vrrp/groups_test.go` | `ParentDevice` is the resolved device for both selector forms | |
| `TestVRRPParentComposesVLANOnTheResolvedDevice` | `internal/plugins/vrrp/groups_test.go` | validates AC-3 | |
| `TestVRRPParentUnselectedInterfaceIsUnchanged` | `internal/plugins/vrrp/groups_test.go` | validates AC-4, so the common configuration is pinned against regression | |
| `TestVRRPGroupRefusesAnUnansweredSelector` | `internal/plugins/vrrp/groups_test.go` | validates AC-5 in both the zero and the ambiguous case | |
| `TestNoRegistryResolvesAConfiguredNameAgainstTheKernel` | `internal/plugins/vrrp/groups_test.go` | validates AC-6 structurally, so the next registry cannot repeat it | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VLAN id | 0-4094 | 4094 | N/A (0 means no VLAN) | 4095 |
| Devices answering a selector | 0..N | 1 | 0 is refused | 2 is refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vrrp-macvlan-parent-selector` | `test/iface/vrrp-macvlan-parent-selector.ci` | a MAC-selected interface hosts its virtual router on the selected NIC | | <!-- doc-links: ignore (this spec's AC-7 creates the file; the spec is ready and not yet authorised to run) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | The defect is local device selection, not wire behavior. RFC 9568 conformance is unchanged by this fix, and the existing VRRP interop coverage exercises the protocol | N-A |

## Files to Modify
- `internal/plugins/vrrp/groups.go` - `unitDevice` takes the resolved device; `groupsForFamily` refuses an unanswered selector
- `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` - the design doc `groups.go` declares: the virtual router lives on the resolved device, and an unanswered selector refuses the group
- Every file the Phase 1 enumeration names. The list is written into this spec before Phase 2 starts

## Files to Create
- `test/iface/vrrp-macvlan-parent-selector.ci` - the AC-7 proof <!-- doc-links: ignore (this spec's AC-7 creates the file; the spec is ready and not yet authorised to run) -->

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
| Doctor check for runtime dependencies | Yes | a VRRP group whose selector answers nothing is a configuration the operator should see in `ze doctor`, beside the interface checks the sibling spec added |
| Prometheus counters/metrics | No | a refused group is a config error, reported at commit |
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
| 14 | Prometheus counters added/changed? | No | none |
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
| The kernel agrees | `test/iface/vrrp-macvlan-parent-selector.ci` on a live kernel | <!-- doc-links: ignore (AC-7 of this spec creates this file; the spec is ready and not yet authorised to run) -->
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
