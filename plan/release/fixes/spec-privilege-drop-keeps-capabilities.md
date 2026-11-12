# Spec: privilege-drop-keeps-capabilities

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P1 |
| Release reason | Correctness defect in the shipped binary: a daemon started as root with `ze.user` set drops every capability at the user switch, so every feature that needs CAP_NET_ADMIN, CAP_NET_RAW or CAP_BPF fails after start. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The defect.** `dropPrivileges` (`cmd/ze/hub/main_system.go`) calls
`privilege.Drop` when `ze.user` is set. `Drop`
(`internal/core/privilege/drop_unix.go`) runs setgroups, setgid, then setuid,
and verifies the uid and gid. It never sets `PR_SET_KEEPCAPS` and never raises
a capability afterwards. When a process whose uid was 0 switches every uid to
a non-zero value, the kernel clears its permitted and effective capability sets
(capabilities(7), "Effect of user ID changes on capabilities"), unless
`SECBIT_KEEP_CAPS` is set. So a daemon started as root with `ze.user` set holds
no capability after the drop, and every later use fails:

- XFRM state and policy installs (CAP_NET_ADMIN), so IPsec stops working;
- FIB and interface writes (CAP_NET_ADMIN);
- the ESP form TUN `ze-espform` (TUNSETIFF needs CAP_NET_ADMIN);
- eBPF loads (CAP_BPF): the ESP form fast path, the override guard and
  traffic-usage;
- ping and traceroute (CAP_NET_RAW).

**Who is affected.** Only the `ze.user` route. The systemd unit Ze installs
switches the user itself and grants the set as ambient capabilities
(`internal/plugins/systemd/unit.go`, from `privilege.DaemonCapabilities`). The
gokrazy appliance sets no `ze.user` (`gokrazy/ze/config.json`) and runs as root.

**Found by** `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`
(appliance and unit check, 2026-11-12). Not implemented there.

**Fix direction.** Before the setuid, set `PR_SET_KEEPCAPS` (or
`SECBIT_KEEP_CAPS`) so the permitted set survives. After it, set the permitted
and effective sets to `privilege.DaemonCapabilities` and drop the rest, and
raise the same set as ambient, so a plugin the daemon forks inherits it the way
the systemd route gives it. `privilege.DaemonCapabilities` stays the one
declaration of the set. Verify the result from `/proc/self/status` (`CapEff`)
as the drop already verifies the uid. A drop that cannot keep the set fails
the start with an operator error naming the missing capability.

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/core.md`, `ai/doors/hub-engine.md`
- [ ] `docs/architecture/system-architecture.md`, the privilege drop and capability sections
- [ ] capabilities(7), prctl(2) `PR_SET_KEEPCAPS`, `PR_CAP_AMBIENT`

## Current Behavior (MANDATORY)

- [ ] `internal/core/privilege/drop_unix.go` - `Drop`: setgroups, setgid, setuid, uid/gid verification, no capability handling.
- [ ] `cmd/ze/hub/main_system.go` - `dropPrivileges`: calls `Drop` when `ze.user` is set, warns when root without it.
- [ ] `internal/core/privilege/capabilities.go` - `DaemonCapabilities`, the set the daemon needs.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Privilege drop | `internal/core/privilege/drop_unix.go` `Drop` | Switches uid and gid | Extends: keeps and re-raises the capability set |
| The daemon's capability set | `internal/core/privilege/capabilities.go` `DaemonCapabilities` | The one declaration the unit and the start warning read | Extends: the drop raises exactly this set |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze start` as root with `ze.user` (env `ze_user`) set.

### Transformation Path
1. `hub.Run` binds its ports, then `dropPrivileges` calls `privilege.Drop`.
2. `Drop` switches groups and user; after the fix it keeps and re-raises `privilege.DaemonCapabilities`.
3. Every later privileged call (XFRM, FIB, TUN, eBPF) runs with that set.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to kernel | setuid, prctl, capset | No |

### Integration Points
- `dropPrivileges` (`cmd/ze/hub/main_system.go`), the one caller of `privilege.Drop`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start` as root with `ze.user` set | → | `dropPrivileges` → `privilege.Drop` | guest-root test named in the TDD plan (to be named at design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be named at design | `internal/core/privilege/` | the drop keeps exactly `DaemonCapabilities` (AC-1, AC-2) | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| to be named at design | guest-root Go test, or `test/` `.ci` with `ze.user` | a dropped daemon installs an XFRM state | |

## Files to Modify
- `internal/core/privilege/drop_unix.go` - keep and re-raise the capability set across the drop.
- `cmd/ze/hub/main_system.go` - surface a refused drop as an operator error.
- `docs/architecture/system-architecture.md` - the privilege drop section.

## Files to Create
- the guest-root test for AC-1.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | a daemon started as root with `ze.user` set to an unprivileged user, in a guest-root test on Ze's kernel | after the drop it runs as that uid, its `CapEff` equals `privilege.DaemonCapabilities`, and it installs an XFRM state | no capability outside the set, because the drop exists to shed them |
| AC-2 | a drop where the set cannot be kept | the start fails with an operator error naming the capability and the fix | none: a daemon that starts without its capabilities fails later with no cause named |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `lifetime-ends-before-its-last-reader.md` | 2026-11-12 | `privilege.Drop` clears every capability at the `ze.user` setuid |

## Implementation Steps

1. Write the AC-1 guest-root test and show it red at HEAD (CapEff 0 after the drop).
2. Keep and re-raise the set in `Drop`; refuse the start when it cannot be kept (AC-2).
3. Update the privilege drop section of `docs/architecture/system-architecture.md`.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] `./le test unit pkg path ./internal/core/privilege/`, then the guest-root AC-1 test in a QEMU guest on Ze's kernel
