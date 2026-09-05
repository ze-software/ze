# Spec: finish-appliance-qemu-evidence

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/deferrals.md` - the rows that point here

## Task

`spec-fixit-appliance-evidence-config` fixed the two bugs that blocked the appliance
evidence run and was closed in `f42c2ccb2`. The end-to-end QEMU run it unblocked was never executed, and two deferral
rows were left pointing at the deleted spec file, so `commit_helper.py` refuses commits with
"live deferrals without a destination spec". This spec is that destination.

Its closure record stated the residue plainly: the full L2TP evidence test
(`./le deployment gokrazy-l2tp-ppp-test`) needs root, and "AC-3's end-to-end qemu run remains
to be executed on a root host".

### Work items (re-homed 2026-07-16 from `plan/deferrals.md`)

- **Full QEMU gokrazy L2TP appliance proof (from spec-gokrazy-init-bump AC-6, 2026-07-10)** -
  **DISCHARGED 2026-08-05.** It ran at the source spec's closure on the dev host, which by
  then carried `qemu-system-x86_64`, `xl2tpd`, `pppd`, `/dev/ppp`, `l2tp_ppp` and kvm-group
  access: `./le qemu vpp-hugepages-test` -> `VPP-HUGEPAGES-QEMU: PASS cmdline has
  hugepages=64, hugepages-total=64`, and `internal/le/deployment/gokrazyimage.go`
  -> exit 0, `OK: gokrazy Ze appliance completed real L2TP PPP/IPCP with Ze ppp0 and LAC
  ppp0, dataplane ping, route inject, and clean teardown`. The row in
  the retired deferral shard "gokrazy-init-bump" is `done`. This spec stays OPEN for its OTHER row
  (iface-absent-link-graceful AC-3); A-2 above assumed one run satisfies both, and that
  assumption is now testable rather than assumed.
  -> Trap found while discharging it: the durable runtime-kernel cache entry can be keyed
  `<pinned-version>-...` while holding a DIFFERENT release with no `modules.builtin`. The
  proof fails closed with exit 1 and names the fix (`ze appliance kernel`).
  Under `sudo` it probes ROOT's cache, so pass `XDG_CACHE_HOME` at the cache holding the
  kernel.
  -> Constraint (added 2026-08-03 at the source spec's closure): the boot proof is
  `./le qemu vpp-hugepages-test` plus `./le deployment gokrazy-l2tp-ppp-test`.
  It is NOT `test/appliance/serial-login.ci`, which boots nothing and which the
  source spec wrongly named (`ai/rules/platform-linux.md` strikes it out of the
  proof table).
  -> Constraint: **read the known fail-open before diagnosing a boot failure.**
  `ze appliance build` injects the seed database with `debugfs -w -R`, whose stderr
  it discards and which exits 0 even when the write fails. An image whose
  `/perm` database was never written therefore builds green and dies at boot
  with no cause in the build log. That is a live deferral homed at
  `plan/pre-release/spec-gokrazy-builddir-tmp-deferred-build-flow-unification.md`. If this
  run fails at boot, check `/perm/ze/database.zefs` before suspecting the init
  bump.
- **Full gokrazy L2TP appliance run proving graceful-skip end to end (from
  spec-iface-absent-link-graceful AC-3, 2026-07-10)** - the graceful-skip fix is already
  proven on a real appliance boot ("interface config applied", no crash loop); this is the
  same L2TP-session-level run.

-> Constraint: both rows are the SAME run, blocked on the SAME thing. They were deferred
because two appliance build/config-flow bugs stopped web/l2tp from starting -- and those two
bugs are exactly what `1103` fixed. So the blocker is gone; what remains is executing the run
on a host with root + `/dev/ppp` + PPPoL2TP kernel support. This is an ENVIRONMENT
requirement, not an implementation gap.

-> Constraint: `ai/rules/platform-linux.md` makes QEMU integration mandatory for linux-only
code and forbids skipping for "needs hardware". Read it before proposing any narrowing.

## Required Reading

### Architecture Docs
- [ ] The `fixit-appliance-evidence-config` closure record (retired with the learned corpus) - the two bugs fixed, and the stated residue
  → Constraint: (fill during research) the two blocking bugs (`ze init --force` daemonRunning guard vs host sshd:22; active-config shadowing the build template) are FIXED -- confirm against the producing code before assuming the run is still blocked.
- [ ] `ai/rules/platform-linux.md` - QEMU integration is mandatory for linux-only code
  → Constraint: (fill during research)

**Key insights:** (fill during research)

## Current Behavior (MANDATORY)

**Source files read:** (fill during research -- these are the entry points, not yet read)
- [ ] `internal/le/integration/gates.go` - defines the `./le deployment gokrazy-l2tp-ppp-test` target this spec must execute
- [ ] `internal/plugins/init/main.go` - holds the `daemonRunning` guard that `spec-fixit-appliance-evidence-config` fixed (bug 1: false positive against host sshd:22)

**Behavior to preserve:** the two fixes from `spec-fixit-appliance-evidence-config` (bootstrap-from-template, daemonRunning guard).

**Behavior to change:** none expected -- this is an evidence run, not a code change. If the
run reveals a defect, that defect gets its own spec.

## Data Flow (MANDATORY)

### Entry Point
(fill during research)

### Transformation Path
1. (fill during research)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| (fill during research) | | [ ] |

### Integration Points
- (fill during research)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The two bugs that blocked this run are fixed, so the only remaining barrier is the host environment (root + `/dev/ppp` + PPPoL2TP) | The closed spec's record documents both fixes and says only the run "remains to be executed on a root host" | If a third blocker exists, this is an implementation spec, not an evidence run, and needs a real design pass | Execute the run on a qualifying host and read the first failure | unvalidated |
| A-2 | Both deferral rows (gokrazy-init-bump AC-6, iface-absent-link-graceful AC-3) are satisfied by ONE run | Both name the same `./le deployment gokrazy-l2tp-ppp-test` L2TP-session proof | If they need different assertions, this spec covers one and the other stays homeless | Read both source specs' AC text before running | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | No qualifying host is available, so the spec sits open indefinitely while its deferral rows look "homed" | The spec stays `skeleton` for months | Better than the current state: the rows at least name a real file. If it stalls, record the environment requirement in `plan/known-failures/` rather than deleting the spec |
| R-2 | The run reveals a genuine product defect, and the temptation is to fix it here | The evidence run turns into a debugging session | Any defect found gets its OWN spec; this spec's output is evidence, per `ai/rules/interop-and-goal-validation.md` |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (fill during design) | → | (fill during design) | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| (fill during design) | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | | | |

### Functional Tests
<!-- Provisional -- confirmed at the DESIGN gate. The deliverable here is a deployment
     evidence RUN, not a new .ci; the existing target is named below. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le deployment gokrazy-l2tp-ppp-test` | `test/deployment/` (`.ci` path to confirm during research) | An operator boots the gokrazy appliance image and an xl2tpd/pppd client establishes an L2TP session against it. | planned |

## Files to Modify
- (fill during design) - expected: none, or evidence capture only

## Implementation Steps
- (fill during design)

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- A closed spec's deferrals outlive it. `spec-fixit-appliance-evidence-config` closed
  correctly (bugs fixed, learned summary written, file `git rm`-ed) but two deferral rows
  kept naming the deleted file, and `commit_helper.py`'s destination check then blocked
  unrelated commits repo-wide. Spec closure should re-point surviving deferrals, not just
  remove the file.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One spec for both rows | One spec each | Both name the same `./le deployment gokrazy-l2tp-ppp-test` run blocked on the same environment. Two specs would duplicate the same evidence. |
| Re-home into `spec-finish-*` rather than recreate the retired filename | Recreate `spec-fixit-appliance-evidence-config.md` | Its bugs ARE fixed; recreating it would misrepresent finished work as open and break `git log --follow`. `spec-finish-<subsystem>` is the documented convention for residual bits (`plan/deferrals.md` header). |

## Known Limitations
- Requires a host with root, `/dev/ppp` and PPPoL2TP kernel support. Not runnable on the
  darwin dev machines where most sessions execute.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `fixit-ddos-test-infra.md`, 2026-07-19

Deferred by spec-fixit-ddos-test-infra functional-proof.

QEMU run of the two .ci is the AC-1/4/5/6 proof, deferred to CI

### From `fixit-recent-cache-buffer-reclaim.md`, 2026-07-19

Deferred by spec-fixit-recent-cache-buffer-reclaim functional-proof.

no privileged pool-pressure QEMU proof; unit-tested via fake pool ratio

### From `fixit-show-ping-serial-pacing.md`, 2026-07-19

Deferred by spec-fixit-show-ping-serial-pacing functional-proof.

privileged CAP_NET_RAW/QEMU batch-shape proof deferred to CI

### From `iface-absent-link-graceful.md`, 2026-07-10

Deferred by spec-iface-absent-link-graceful AC-3.

Full gokrazy L2TP appliance run proving graceful-skip end to end
