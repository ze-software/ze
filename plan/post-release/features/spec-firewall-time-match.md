# Spec: firewall-time-match

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | New firewall match VyOS offers and Ze lacks; nothing shipped is wrong without it, so it is post-release feature work. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator wants a firewall term, or a policy-routing rule, to apply only during
a time window: office hours, weekdays, a night-time backup window, or between two
dates. nftables has the meta keys for it (time of day, day of week, absolute
time). Ze gains a `time` container in the firewall `from` block with a start and
stop time of day, a set of weekdays, and an optional start and stop date, all
written in the box's local time.

The kernel compares these values in UTC. That one fact caused two VyOS bugs in
October 2026 (T9474), and the acceptance criteria below exist so Ze does not
repeat them.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - backend split, exact-or-refuse at verify
- [ ] `docs/architecture/policyroute/policy-routing.md` - policy routing lowers to firewall terms
  → Constraint: policy routing builds `firewall.Term` values in table `ze_pr`, so a firewall match is reachable from it without a second lowering
- [ ] `docs/guide/firewall.md` - the operator surface

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` - no time match among the `Match` types.
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` `grouping from-block` - no `time` node.
- [ ] `internal/component/firewall/config.go` `parseFromBlock` - no time branch.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` `lowerMatch` - no time case. The vendored `github.com/google/nftables/expr` names no time meta key constant, so the kernel values are owed from `include/uapi/linux/netfilter/nf_tables.h`.
- [ ] `internal/plugins/policyroute/translate.go` `translate` - builds a prerouting filter chain (priority -150) of `firewall.Term` values in table `ze_pr`.
- [ ] `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - a rule's `from` block holds addresses, ports, protocol and TCP flags; no time leaf.

**Behavior to preserve:** every existing match; policy-routing marks and ip rules.

**Behavior to change:** add the `time` match, usable by firewall terms and by policy-routing rules.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Firewall match model and lowering | `internal/component/firewall/model.go` `Match`; `lower_linux.go` `lowerMatch` | Typed matches lowered to nft expressions | Extends: one time match type, one lowering case |
| Policy routing on firewall terms | `internal/plugins/policyroute/translate.go` `translate` | Turns policy rules into firewall terms | Extends: a policy rule's `from` block gains the same time leaves, reusing the firewall match |
| Firewall apply and readback | `firewall.ApplyAll`, `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Reconciles Ze-owned tables on commit | Extends: the UTC-offset refresh (AC-5) re-applies through the same path, and must keep set contents and counters |
| Backend capability gate | `ze:backend`, `config.validateBackendFeatures` | Refuses a node a backend cannot program | Extends: `time` carries `ze:backend "nft"` unless the VPP backend can program it exactly |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `time` container in a firewall term's `from` block or a policy rule's `from` block.

### Transformation Path
1. Parse local start and stop times, weekdays and dates.
2. At lowering, convert to UTC with the offset in force, producing one hour range (complemented when it wraps) and a weekday set shifted where the conversion crosses midnight.
3. Emit meta time expressions and compares.
4. A timer re-lowers the time-bearing rules at the next UTC offset change.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → firewall component | YANG tree | No |
| Firewall component → nft backend | `Backend.Apply` | No |

### Integration Points
- `lowerMatch` and `validateMatch` - one case each
- `policyroute` `buildPolicyMatch` - reuses the match

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `from time` in config | → | time match lowering | `test/firewall/time-window.ci` (proposed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Start 09:00, stop 17:00, host in UTC | One hour-of-day range compare, 09:00 to 17:00; a packet inside matches, one outside does not (clock-controlled unit test of the lowered bounds) | - |
| AC-2 | Window crossing midnight in UTC: 22:00 to 06:00 in any zone, or 01:00 to 02:00 local on a host at UTC+2 | Lowered as ONE range whose complement is matched when it wraps, never as two separate start and stop compares (VyOS `8d40cc543`: two compares never matched a wrapping window) | - |
| AC-3 | Only a start time, or only a stop time | The missing end defaults to the start or end of the local day, then the AC-2 rule applies | - |
| AC-4 | Weekdays with a time window, where local to UTC conversion moves the window across midnight | The weekday set is shifted by the same day the hours moved, so a local Monday 00:30 at UTC+2 matches as Sunday in UTC | - |
| AC-5 | Daylight saving change, or the host time zone changes, after commit | The time-bearing rules are re-lowered at the moment the UTC offset changes, without flushing named sets (dynamic, domain, IRR) or resetting counters; a test moves the zone offset and proves both the new bounds and a surviving dynamic set element (VyOS `c10229f44`: windows were off until the next commit) | - |
| AC-6 | Start date and stop date | Absolute time compares in UTC nanoseconds; an inverted date range is refused at verify | - |
| AC-7 | Policy-routing rule with a time window | The mark is set only inside the window, proven by a functional test | - |
| AC-8 | `time` with the VPP backend | Refused at commit with the YANG path | - |
| AC-9 | `show firewall` on a time term | Shows the operator's local values and the UTC bounds currently programmed | - |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The kernel's hour key is seconds since midnight UTC and its day key is the UTC weekday | VyOS commit messages `8d40cc543`, `c10229f44`; kernel `nft_meta.c` not yet read | AC-2 and AC-4 change shape | Read `net/netfilter/nft_meta.c` at the Ze kernel version | unvalidated |
| A-2 | Ze has no time-zone config leaf; the daemon uses the host zone | grep of YANG for a time-zone leaf found none | AC-5 also needs a config trigger | Grep at design | unvalidated |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLowerTimeWindowWrap` | `internal/plugins/firewall/nft/lower_linux_test.go` | AC-1 to AC-4 at fixed offsets | |
| `TestTimeOffsetRefresh` | `internal/component/firewall/` | AC-5 schedule and re-lower | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `time-window` | `test/firewall/time-window.ci` | AC-1, AC-2 programmed bounds read back | |

## Implementation Steps

1. **Phase: Wiring** -- YANG, parse, failing `.ci`
2. **Phase: Lowering** -- AC-1 to AC-4, AC-6
3. **Phase: Offset refresh** -- AC-5, a long-lived timer worker, not one per rule
4. **Phase: Policy routing and show** -- AC-7, AC-9

## Files to Modify
- `internal/component/firewall/model.go`, `config.go`, `validate.go`
- `internal/component/firewall/yang/ze-firewall-conf.yang`
- `internal/plugins/firewall/nft/lower_linux.go`
- `internal/plugins/policyroute/translate.go`, `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`
- `docs/guide/firewall.md`, `docs/guide/policy-routing.md`

## Files to Create
- `test/firewall/time-window.ci`

## VyOS reference

T9474 (merge `040fab510`, PR #5573): `8d40cc543` "match time windows crossing
midnight in UTC" and `c10229f44` "keep time windows correct across UTC offset
changes" (a systemd timer re-armed at each offset change, reloading only the
rules with an hour match so sets and counters survive). Both pitfalls are AC-2,
AC-3 and AC-5.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Targeted tests pass: `./le test unit plugins`, `./le test integration firewall`, `./le test firewall`, `./le test policy`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
