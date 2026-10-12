# Spec: bsd-pfkey-ipsec

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | `plan/spec-bsd-family-interface-backend.md` (P3 PF_KEY writer for TCP-MD5), `plan/spec-bsd-qemu-guest-kind.md` |
| Phase | 0/0 (Do not start until the owner approves) |
| Handoff | - |
| Updated | 2026-10-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's IKE engine installs child SAs through a Linux XFRM dataplane
(`internal/component/ike/dataplane/`); off Linux `xfrm_other.go` returns
`ErrNotSupported` for every SA operation. FreeBSD and OpenBSD carry IPsec in the
kernel behind PF_KEY v2 (RFC 2367) with the SPD and SAD, and OpenBSD adds `enc(4)`
and `sec(4)` route-based interfaces; FreeBSD has `if_ipsec(4)`. This spec adds a
PF_KEY dataplane so IKEv2 child SAs install on FreeBSD and OpenBSD. macOS PF_KEY
exists but is not a target unless the owner says so.

| Point | Notes |
|-------|-------|
| Shared writer | The main spec's P3 writes TCP-MD5 SAs through PF_KEY; this spec reuses that message encoder |
| Route-based VPN | `if_ipsec(4)` (FreeBSD) and `sec(4)` (OpenBSD) versus Linux XFRM interfaces (`CreateXFRM` in the iface Backend) |
| Algorithms | Map Ze's transform set to each kernel's supported list; refuse the rest by name |
| Extensions | OpenBSD and FreeBSD differ in NAT-T and extension headers |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/` dataplane pages (the `// Design:` header of `xfrm_other.go` names `ipsec-8-ikev2-child-xfrm.md`)
  → Constraint: the PF_KEY dataplane implements the same `Dataplane` interface, so the IKE engine stays platform-blind

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2367.md` - PF_KEY v2 (summary to be created with `/ze-rfc` if absent)
  → Constraint: [to be filled at design]

**Key insights:**
- [to be filled at design]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/dataplane/xfrm_other.go` - every SA operation returns `ErrNotSupported` off Linux

**Behavior to preserve:**
- Linux XFRM dataplane unchanged.

**Behavior to change:**
- FreeBSD and OpenBSD install child SAs through PF_KEY.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: IKE child SA negotiated]
- [Format at entry]

### Transformation Path
1. [to be designed]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to kernel | [PF_KEY socket, `SADB_ADD` / `SADB_X_SPDADD`] | No |

### Integration Points
- [to be designed]

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `Dataplane` interface fits PF_KEY without new methods | [to be checked] | [interface change across backends] | [mapping at design] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Ze's SPD entries collide with the operator's | [interop run] | [own SPD priority / tag] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKEv2 child SA on FreeBSD | → | [PF_KEY dataplane InstallSA] | [test name at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IKEv2 tunnel against strongSwan from a FreeBSD/OpenBSD guest | Child SA installed, ESP traffic flows both ways |
| AC-2 | Rekey and delete | Old SA removed, no leak in `setkey -D` / `ipsecctl -sa` |
| AC-3 | Transform the kernel lacks | Refused by name |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to be designed] | [PF_KEY dataplane package] | [`SADB_ADD` golden bytes] | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `freebsd-ipsec-strongswan` | `test/interop/scenarios/` (BSD guest kind) | strongSwan | ESP through a Ze-installed SA | |

## Files to Modify
- `internal/component/ike/dataplane/xfrm_other.go` - narrow build tags
- `docs/guide/ipsec.md` - platform section

## Files to Create
- [PF_KEY dataplane files under `internal/component/ike/dataplane/`]

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
