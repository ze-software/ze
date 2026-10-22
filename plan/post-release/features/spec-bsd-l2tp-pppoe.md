# Spec: bsd-l2tp-pppoe

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Adds a BSD platform or backend; release correctness of existing Linux support is not deferred. |
| Scope | protocol |
| Depends | `plan/post-release/features/spec-bsd-family-interface-backend.md` (P1, P2), `plan/post-release/features/spec-bsd-qemu-guest-kind.md` |
| Phase | 0/0 (Do not start until the owner approves) |
| Handoff | - |
| Updated | 2026-10-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's L2TP LNS and PPPoE access/client run their control planes in Go and hand the
data plane to the Linux kernel (`l2tp_ppp`, PPPoE sockets, generic netlink in
`internal/component/l2tp/genl_linux.go`). Off Linux `kernel_other.go` and
`pppoe/socket_other.go` refuse. This spec gives them a BSD data plane:

| OS | Mechanism |
|----|-----------|
| FreeBSD | netgraph (`ng_l2tp`, `ng_pppoe`, `ng_ppp`, `ng_iface`), driven through `ng_socket` |
| OpenBSD | `pppoe(4)` for the client, `pipex(4)` for in-kernel L2TP/PPPoE session forwarding |
| macOS | none; refused by name |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/` - LNS and PPPoE architecture
  → Constraint: the control plane stays shared; only the kernel session plumbing is per OS

**Key insights:**
- [to be filled at design]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/kernel_other.go` - kernel session operations refuse off Linux
- [ ] `internal/component/l2tp/pppoe/socket_other.go` - PPPoE socket refuses off Linux

**Behavior to preserve:**
- Linux data plane unchanged.

**Behavior to change:**
- FreeBSD (netgraph) and OpenBSD (pipex, pppoe(4)) carry subscriber sessions.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: L2TP tunnel / PPPoE discovery]
- [Format at entry]

### Transformation Path
1. [to be designed]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to kernel | [netgraph socket / pipex ioctl] | No |

### Integration Points
- [to be designed]

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The kernel session interface in `kernel_other_types.go` fits netgraph and pipex | [to be checked] | [interface redesign] | [mapping at design] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | pipex lacks a feature the Linux path uses (for example per-session stats) | [design mapping] | [refuse by name] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| PPPoE session on FreeBSD | → | [netgraph session setup] | [test name at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | L2TP tunnel and session from an LAC to Ze on FreeBSD/OpenBSD | Session up, IP traffic forwarded |
| AC-2 | PPPoE client session to Ze on FreeBSD/OpenBSD | Session up, traffic forwarded |
| AC-3 | Same config on macOS | Refused by name |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to be designed] | [l2tp kernel BSD files] | [netgraph message golden bytes] | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `freebsd-l2tp-xl2tpd` | `test/interop/scenarios/` (BSD guest kind) | xl2tpd / accel-ppp | Session through netgraph | |

## Files to Modify
- `internal/component/l2tp/kernel_other.go`, `pppoe/socket_other.go` - narrow build tags
- `docs/guide/l2tp.md` - platform section

## Files to Create
- [netgraph and pipex files under `internal/component/l2tp/`]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| [to be answered at design] | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| [to be answered at design] | | | |

## Implementation Steps

1. **Phase: Wiring** -- [to be designed]

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
