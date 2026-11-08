# Spec: iface-mirror-target-reference

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P1 |
| Release reason | A mistyped or deleted port-mirror target commits, and a VLAN-unit target on a selected parent mirrors toward the wrong device: accepted config that the apply cannot honour is incorrect shipped behavior |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`interface ... unit ... mirror { ingress <name>; egress <name>; }` takes any
string. Verify never checks that the name is an interface the config declares,
so three faults commit:

| Fault | What happens today |
|-------|--------------------|
| The target is mistyped | The commit passes verify, and the mirror fails or is never installed at apply |
| The target interface is deleted while a mirror still names it | The commit passes, and the mirror points at nothing |
| The target is a VLAN unit of an entry bound by `mac match` or `os-name` | `deviceFor` does not find the unit name in the selector map and returns it unchanged, so tc mirrors toward whatever device carries the logical name, or toward no device |

The goal: a mirror target is a reference. Verify refuses a target the
`interface` tree does not declare, refuses the deletion of a referenced target
(naming the interface that mirrors to it), and the apply binds a unit target
through its parent's selector.

Found by comparing recent VyOS fixes against Ze. VyOS fixed the same class in
T9491 (280cb4837, "refuse deleting an interface used as mirror or redirect
target") and T6393 (99271db4a "accept mirror and redirect targets defined on the
CLI", 9854913e2 "install mirror and redirect filters for VLAN targets"), in
`~/Code/github.com/vyos/vyos-1x`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - "A mirror destination is resolved when the mirror spec is built" and "left undone with a warning and the commit still succeeds"
  → Constraint: verify stays pure (it reads no kernel), so it judges whether the CONFIG declares the target, never whether the device is present. A declared but unbound target still defers, as today
  → Decision: the page's sentence about a mirror destination changes in the same work

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` `container mirror` - `ingress` and `egress` are `type string` with no validator
- [ ] `internal/component/iface/config.go` unit parsing - copies `mirror/ingress` and `mirror/egress` into `MirrorIngress` and `MirrorEgress` unchecked
- [ ] `internal/component/iface/register.go` `parseAndVerifyIfaceSections` - runs selector, delegated-prefix and VPP QoS checks, and no mirror check
- [ ] `internal/component/iface/config_mirror.go` `mirrorDestination` - returns `deviceFor` of the name, or empty when the selector is unbound
- [ ] `internal/component/iface/config_apply.go` `deviceFor`, `unitOSName` - a name absent from the selector map is its own device. A VLAN unit's kernel name is `<parent device>.<vlan-id>`

**Behavior to preserve:**
- A declared target whose device is absent defers with a warning, and the commit succeeds (the page's rule for every unbound setting)
- A target bound by `mac match` or `os-name` resolves through the selector map

**Behavior to change:**
- An undeclared target is refused at verify, with the YANG path and the fix
- Deleting a referenced target is refused at verify, naming the referencing interface and unit
- A unit target resolves through its parent's binding

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| iface verify | `internal/component/iface/register.go` `parseAndVerifyIfaceSections` | Pure checks over the parsed config | extends: one more pure check over `MirrorIngress` and `MirrorEgress` |
| Selector map | `internal/component/iface/config_apply.go` `deviceFor`, `bindDevices` | Translates an ethernet logical name to its kernel device | extends: a unit target is split into parent and VLAN id, the parent is translated, and `unitOSName` builds the device |
| Interface usage map (VPP) | `plan/release/fixes/spec-vpp-interface-in-use.md`, AC-6 and AC-9 | Designed, not built: VPP mirror destinations as references that block deletion | extends: the reference check is backend-neutral, so that spec's mirror rows reuse it rather than adding a VPP-only copy |
| Absent mirror destination retry | `plan/release/fixes/spec-network-services-one-off-defects.md`, AC-22 | Designed: an absent destination is reported and retried, not a whole-commit abort | does not apply: that spec owns a declared target whose device is absent. This spec owns an undeclared one |
| YANG custom validator | `ze:validate` with a registered `ValidateFn`, as VRRP's `vrrp-track-interface` | Validates a leaf against registered logic, with completion | extends if the design picks it: completion of declared interface names at the CLI |

## Data Flow (MANDATORY)

### Entry Point
- A config commit or `ze config validate` carrying an `interface` tree with a `mirror` block

### Transformation Path
1. The unit parser fills `MirrorIngress` and `MirrorEgress`
2. Verify checks each target against the names the parsed config declares, units included
3. The apply binds each target, a unit target through its parent's selector

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config to iface plugin | verify and apply stream events | No |

### Integration Points
- `parseAndVerifyIfaceSections`, `mirrorDestination`, `deviceFor`, `unitOSName`

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | No config in the repository mirrors toward a device the `interface` tree does not declare | not checked | the change refuses a config in the repository, which is then converted | grep `mirror` in `test/` and `docs/` configs at design | unvalidated |
| A-2 | The unit target is written as `<interface>.<vlan-id>` | `unitOSName` builds kernel names that way | the parser needs the real unit form | read the YANG unit list and the guide at design | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config commit with an undeclared mirror target | → | verify mirror check | `test/parse/iface-mirror-target-undeclared.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `mirror ingress` names an interface the config does not declare | `ze config validate` and the commit refuse it, naming the YANG path, the unknown name and the fix | none |
| AC-2 | A commit deletes an interface that another unit's mirror names | Verify refuses the deletion, naming the mirroring interface and unit | none: the reference must go first |
| AC-3 | `mirror egress` names a VLAN unit of an ethernet entry bound by `mac match` to a veth | The tc mirred filter points at `<bound device>.<vlan-id>` | none |
| AC-4 | A declared target whose device is absent | The commit succeeds, the mirror is not installed, and a warning names it | the deferred mirror, by the page's rule for unbound settings |
| AC-5 | A target declared and present | The mirror is installed as today | the mirror |

### Tests that prove each AC

| AC | Test | Entry point |
|----|------|-------------|
| AC-1 | new `test/parse/iface-mirror-target-undeclared.ci` | `ze config validate` |
| AC-2 | new `test/reload/tx-iface-mirror-target-delete-refused.ci` | config reload through the daemon |
| AC-3 | new `test/plugin/iface-mirror-vlan-target-selector.ci`, `option=needs-linux`, reads the tc filter on the source device | config commit through the daemon |
| AC-4, AC-5 | `TestMirrorDestination` table rows in `internal/component/iface/config_mirror_test.go`, plus AC-3's `.ci` | `mirrorSpecFor` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyMirrorTargetDeclared` | `internal/component/iface/register_test.go` | AC-1, AC-2 | |
| `TestMirrorDestination` | `internal/component/iface/config_mirror_test.go` | AC-3 to AC-5 | |

## Files to Modify
- `internal/component/iface/register.go` - the verify check
- `internal/component/iface/config_mirror.go` - bind a unit target through its parent
- `internal/component/iface/yang/ze-iface-conf.yang` - the leaf descriptions
- `docs/architecture/iface/logical-name-resolution.md` - the mirror destination paragraph

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG validation constraints | Yes | the design picks a pure verify check or a `ze:validate` |
| Doctor check | No | no new runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture/iface/logical-name-resolution.md` |

## Implementation Steps
1. **Phase: Wiring** -- the failing `.ci` tests
2. **Phase: Verify check** -- AC-1, AC-2
3. **Phase: Unit target binding** -- AC-3

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le test functional parse`, `./le test functional reload` and `./le test functional plugin` for the new `.ci` files; the iface unit package through `./le job run`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `validate-accepts-what-start-refuses.md` | 2026-11-08 | mirror target is an unchecked free string |
