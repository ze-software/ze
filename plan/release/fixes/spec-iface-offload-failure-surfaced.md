# Spec: iface-offload-failure-surfaced

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | An offload setting the NIC cannot apply commits with only a log line: the operator believes a setting is active that is not |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`interface ethernet <name> offload { gro; gso; sg; tso; lro; hw-tc-offload; rps;
rfs; }` is applied by `applyOffloads`, which logs each failure with `log.Warn` and
returns nothing. A feature the driver fixes (for example `lro` or
`hw-tc-offload` on a veth or a virtio NIC), or any ethtool or sysfs error, leaves
the setting unapplied while the commit succeeds. The operator sees no message in
the commit answer, in `show warnings` or in `ze doctor`.

The goal: an offload the NIC cannot apply is never silent. Either the commit is
refused with what, why and the fix, or the commit answer, `show warnings` and
`ze doctor` all carry the finding until the config or the NIC changes
(`ai/rules/user-facing-errors.md`). The design phase picks refuse or warn
against the exact-or-refuse rule (`ai/rationale/exact-or-reject.md`), and
records the rejected option.

Found by comparing recent VyOS fixes against Ze. VyOS T9228 (07ec3a469,
"ethernet: T9228: only warn about unsupported NIC features when node changed",
in `~/Code/github.com/vyos/vyos-1x`) warns at commit, and only when the node
changed, so the message does not repeat on unrelated commits. Ze has no message
at all.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md`, sections 14a and 15 - exact or refuse; the report bus behind `show warnings`
  → Constraint: verify reads no kernel, so a capability refusal at verify needs a capability fact the config path can reach, or the check runs at apply
  → Constraint: the report bus is process-local, so the raise happens in the daemon's iface process

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/offload_linux.go` `applyOffloads` - each ethtool or sysfs failure is logged with `log.Warn` and dropped; the function returns nothing. Nothing reads a feature back, so a set the driver ignores is not seen either
- [ ] `internal/component/iface/config_apply.go` `applyConfig` Phase 2 - calls `applyOffloads(device, e.Offload)` and has no result to record
- [ ] `internal/component/iface/health.go` - the iface component already raises and clears report-bus warnings (`report.RaiseWarning`, `report.ClearWarning`)
- [ ] `internal/component/iface/doctor_linux.go` `ethernetDoctorCheck` - the iface doctor check reads sysfs for each configured ethernet entry, with one diagnostic code per finding

**Behavior to preserve:**
- A supported offload setting is applied
- No offload block means no ethtool call

**Behavior to change:**
- An unapplied offload setting reaches the operator, at commit and afterwards

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Report bus warnings | `internal/component/iface/health.go`, `report.RaiseWarning` and `report.ClearWarning` | Raises an iface warning for error counters, shown by `show warnings` and a degraded health probe | extends: one warning per interface and feature, cleared when the setting applies or leaves the config |
| Per-step apply journal | `internal/component/iface/config_apply.go` `applyBackendStep` | Records undo for each backend write and fails the commit on error | extends if the design picks refuse: the offload write becomes a journaled step |
| Ethernet doctor check | `internal/component/iface/doctor_linux.go` `ethernetDoctorCheck` | Reports a missing or down device and a MAC override by name, with a code each | extends: a code for a configured offload the NIC does not carry, read from the NIC |

## Data Flow (MANDATORY)

### Entry Point
- A config commit carrying an `offload` block

### Transformation Path
1. Phase 2 applies each offload and reads it back
2. A setting that did not take is refused, or raised on the report bus and returned in the commit answer
3. `ze doctor` reads the configured offloads against the NIC

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface to the kernel | ethtool ioctl, sysfs | No |
| iface to `show warnings` | report bus | No |

### Integration Points
- `applyOffloads`, `applyConfig`, `report.RaiseWarning`, `ethernetDoctorCheck`

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `lro` is a fixed-off feature on a veth in Ze's kernel | ethtool reports `large-receive-offload: off [fixed]` on a veth | the test needs another fixed feature | design runs `ethtool -k` on a veth in the guest | unvalidated |
| A-2 | The commit answer can carry a warning without refusing | not read | the warn option needs a new answer field | design reads the commit answer path | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| commit with `offload lro true` on a veth | → | `applyOffloads` result | `test/plugin/iface-offload-unsupported-surfaced.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Commit `offload { lro true; }` on an ethernet entry bound to a veth | The operator gets a message naming the interface, the feature, the reason and the fix: a refused commit, or a commit answer warning, per the design decision | none: silence is the defect |
| AC-2 | AC-1 took the warn route | `show warnings` lists the finding, and `ze doctor` reports it with the fix, until the leaf is removed | the warning, while the setting stays unapplied |
| AC-3 | The leaf is removed | The warning clears | none |
| AC-4 | A supported setting (`gro` on a veth) | Applied, and no message | none |
| AC-5 | An unrelated commit while AC-2's warning stands | The warning is not repeated as a new commit message (VyOS T9228) | the standing report-bus warning |

### Tests that prove each AC

| AC | Test | Entry point |
|----|------|-------------|
| AC-1, AC-3, AC-4 | new `test/plugin/iface-offload-unsupported-surfaced.ci`, `option=needs-linux`, a veth bound as ethernet | config commit through the daemon |
| AC-2, AC-5 | the same `.ci`: `show warnings` and `ze doctor` after the commit, then an unrelated commit | daemon commands |
| AC-1 unit | `TestApplyOffloadsReportsUnapplied` in `internal/component/iface/offload_linux_test.go`, run by `./le test integration iface` | `applyOffloads` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyOffloadsReportsUnapplied` | `internal/component/iface/offload_linux_test.go` | a failed or unchanged feature is returned, not dropped | |

## Files to Modify
- `internal/component/iface/offload_linux.go` - return what did not apply, read back
- `internal/component/iface/config_apply.go` - record or refuse
- `internal/component/iface/doctor_linux.go` - the doctor finding and its diagnostic code

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| Doctor check | Yes | the iface doctor check plus its declared diagnostic code |
| YANG schema | No | no leaf changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 6 | Has a user guide page? | Yes | the interface guide's offload text names the message |

## Implementation Steps
1. **Phase: Wiring** -- the failing `.ci`
2. **Phase: Result and read-back** -- `applyOffloads` returns what did not apply
3. **Phase: Surface** -- commit answer or refusal, report bus, doctor

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le test functional plugin` for the new `.ci` in the QEMU guest; `./le test integration iface`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-11-08 | `applyOffloads` logs an unapplied offload and the commit succeeds |
