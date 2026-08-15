# Spec: fixit-relax-ceiling-raise-is-unreachable

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`make ze-relax-census` is RED at HEAD and has been since the templ ports landed:
780 `test-relax:` tokens in 483 files against a ceiling of 761. Nineteen tokens
entered the corpus without the ceiling being raised, and `test/relax-ceiling.txt`
says of that raise, in its own prose, "That refusal is the whole mechanism."

The refusal never fired, so the mechanism is not the ceiling. It is the ceiling
PLUS a gate that runs before the commit, and the second half is missing.

**Why it cannot fire in practice.** The census reads what git HOLDS, which is
correct and deliberate: the file's prose records the working-tree count moving
751 to 755 within an hour on 2026-08-10, on edits by three sessions that had
never touched the gate, and states that "a gate that reds on another session's
half-finished work gets switched off." So the census runs only inside `make
ze-verify`. In a checkout five sessions share, `ai/rules/git-safety.md` already
establishes that a fully green `ze-verify` is unreachable by construction and
that `--unverified` is the CORRECT path, not a shortcut. The consequence follows
directly: `ze-verify` is rarely run, so the census is rarely run, so tokens land
and the red is discovered by whoever next runs the full gate.

**The changed-file half exists and is not wired to the ceiling.**
`scripts/dev/audit-test-relaxation.py` audits the tokens in YOUR diff, which the
ceiling file names as "where a changed-file check belongs". It is invoked at step
0 of `/ze-review`, by a human or agent reading its output. Nothing makes a commit
that ADDS a token also raise the ceiling, and `scripts/dev/commit_helper.py` has
no census check among its refusals.

**Why this is worth fixing rather than absorbing.** A structural gate that is red
at HEAD trains every session to wave it through, and `ai/rules/git-safety.md`
makes deterministic structural gates owner-only to wave (`--structural-red-ok`).
Each session that meets this red either burns the owner's attention or learns the
override. The ceiling's stated direction is DOWN: its prose triages the 2026-08-10
corpus and says about 430 of the tokens are receipts for a gate defect that no
longer exists. A ceiling drifting up while the plan says down is not a ratchet.

**Two things are true at once and the spec must hold both.** The nineteen tokens
may each be legitimate: five of the seven `raised-for:` blocks already on file
say in capitals that no coverage was removed, and name the hook's one-file-per-edit
blindness as the cause. So the answer is not "delete the tokens". It is that a
token entering the corpus must cost its reviewed line at the moment it enters.

## Required Reading

### Architecture Docs
- [ ] `test/relax-ceiling.txt` -- read the whole file, prose included. It states
      the design, the failure it was built for, and why HEAD rather than the
      working tree
- [ ] `ai/rules/git-safety.md` -- "Structural Gates Are Never Known-Red", the
      `--structural-red-ok` owner override, and why `--unverified` is normal here
- [ ] `ai/rules/testing.md` -- the test-weakening rule the token buys a pass from
- [ ] `ai/rules/repo-maintenance.md` -- which check enforces which rule, and where
      a changed-file check belongs

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/relax-census.py` -- the count, the `raised-for:` requirement,
      `--lower`, `--list`, and the selftest that runs first
  → Constraint: the selftest exists so a zero count can never read as a pass.
    Keep it.
- [ ] `scripts/dev/audit-test-relaxation.py` -- `run_audit`, the changed-file arm
  → Decision: this already knows which tokens are NEW in a diff. If the ceiling
    check moves anywhere, it moves beside this.
- [ ] `scripts/dev/commit_helper.py` -- the refusal set, and how
      `verify-status.sh check` gates `create`
  → Constraint: a new refusal here must not fire on another session's tokens,
    which is the whole reason the census reads HEAD.
- [ ] `.claude/hooks/pretool-writeedit.py` -- `c_test_weakening`, the arm the
      token buys a pass from, and its one-file-per-edit blindness
  → Constraint: five of the seven recorded raises blame that blindness. Fixing
    it would lower the token rate at source and may be the better spec.

**Behavior to preserve:**
- The census keeps reading HEAD. A working-tree count is the failure mode the
  file was written to avoid.
- A legitimate relaxation stays possible at the price of one reviewed line.
- `--lower` never raises.

**Behavior to change:**
- A commit that adds a `test-relax:` token cannot land without the ceiling
  covering it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `make ze-relax-census`, and `make ze-verify` which runs it as a stage.
- `scripts/dev/commit_helper.py create`, which is where the check has to reach
  if it is to fire before the tokens land.

### Transformation Path
1. An agent weakens a test and writes a `test-relax:` token to buy a pass from
   `c_test_weakening`.
2. The commit is prepared with `--unverified`, because a shared checkout cannot
   produce a green `ze-verify`.
3. The commit lands. The corpus count rises. No gate ran.
4. Some later session runs `ze-verify`, or the census directly, and meets a red
   it did not cause.
5. That session either raises the ceiling for someone else's tokens, or asks the
   owner for `--structural-red-ok`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| census <-> git | counts the content git holds | Yes, stated in `test/relax-ceiling.txt` and confirmed by the red naming "at HEAD" |
| audit <-> diff | `run_audit` over the changed files | Yes |
| commit_helper <-> gates | `verify-status.sh check`, plus its own refusal set | Not read yet: whether a census hook fits there is the first question |

### Integration Points
- `mk/inventory.mk` holds the target and the ceiling override.
- `/ze-review` step 0 runs the changed-file audit.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | the token reaches HEAD without passing the gate that governs it |
| No unintended coupling (components stay isolated) | Yes | census, audit and helper are separate today and stay so |
| No duplicated functionality (extends existing, does not recreate) | Yes | the changed-file arm already exists; this wires it, it does not rewrite it |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Python tooling |
| Registration over hardcoding | N-A | no registration surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validation | Status |
|----|-----------|-------|----------|------------|--------|
| A-1 | All 19 excess tokens are legitimate relaxations needing a `raised-for:` line, not real lost coverage | five of seven recorded raises say no coverage was removed | some record real loss and the tests need restoring, not the ceiling raising | `scripts/dev/relax-census.py --list`, then read each of the 19 against its commit | unvalidated |
| A-2 | A changed-file census check can tell YOUR new tokens from another session's | `audit-test-relaxation.py` already does exactly this over a diff | the check reds on foreign work and gets switched off, which is the documented failure | read `run_audit` and drive it from a two-session fixture | unvalidated |
| A-3 | Fixing `c_test_weakening`'s one-file blindness would remove most future tokens | five of seven raises blame it | the token rate is driven by something else and this spec is aimed wrong | classify the last 30 tokens by cause | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A new pre-commit refusal blocks sessions on tokens they did not write | a session is refused over a foreign file | A-2 is the gate on the design: identity by diff, never by corpus count |
| R-2 | Raising the ceiling to 780 to clear the red ratifies the bypass | the fix is one edit to a number | the raise, if any, carries a `raised-for:` line per token, which is the cost the design intends |
| R-3 | The spec fixes the ceiling and leaves the cause, so the drift resumes | the count rises again within a month | A-3: if the hook's blindness is the driver, that is the spec to write |

## Blast Radius

Every commit that touches a test file. No daemon code, no wire behavior. The
gate becomes reachable, so sessions that previously landed tokens silently will
be asked for a line.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `commit_helper.py create` over a diff that adds a `test-relax:` token with no ceiling raise | -> | the new refusal | a case in `scripts/dev/commit_helper_test.py` |
| the same diff with a raise and a `raised-for:` line | -> | the same | a second case, accepted |
| a diff that adds no token while the corpus is over the ceiling | -> | the same | accepted: the session is not charged for foreign tokens |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A commit adds a `test-relax:` token and does not raise the ceiling | Refused, naming the token's file and the line it needs |
| AC-2 | The same commit raises the ceiling with a `raised-for:` line | Accepted |
| AC-3 | A commit adds no token while HEAD is over the ceiling | Accepted: no session is charged for another's tokens |
| AC-4 | `make ze-relax-census` at HEAD after this spec | GREEN, by whichever of raise-with-reasons or restore-the-coverage A-1 selects for each of the 19 |
| AC-5 | `scripts/dev/relax-census.py --lower` | Still never raises |
| AC-6 | The census selftest | Still runs first, so a zero count can never read as a pass |

## End-to-End User Stories

- An agent legitimately weakens a test, and is asked for its one reviewed line at
  the moment it commits, not weeks later by a stranger.
- A session runs `make ze-verify` and does not meet a red belonging to somebody
  else.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| an unraised token-adding commit is refused | `scripts/dev/commit_helper_test.py` | AC-1 | |
| a raised, reasoned commit is accepted | `scripts/dev/commit_helper_test.py` | AC-2 | |
| a token-free commit over an over-ceiling corpus is accepted | `scripts/dev/commit_helper_test.py` | AC-3 | |
| `--lower` never raises | `scripts/dev/relax_census_test.py` | AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-relax-census` | the gate itself | green at HEAD once the 19 are accounted for | |

## Files to Modify

- `scripts/dev/commit_helper.py` -- the refusal set, if the check lands there
- `scripts/dev/audit-test-relaxation.py` -- if the new-token identification is
  reused from it rather than rewritten
- `test/relax-ceiling.txt` -- the number, plus one `raised-for:` line per token
  A-1 finds legitimate
- `scripts/dev/relax-census.py` -- only if the design needs a new mode
- `ai/rules/repo-maintenance.md` -- the check-to-rule mapping, via its point file

## Files to Create

- none expected; every mechanism this needs already exists

## Implementation Steps

1. **Phase: Account for the 19** -- `relax-census.py --list`, read each against
   the commit that added it
   - Verify: A-1 confirmed or broken, per token, with a verdict each
2. **Phase: Decide where the check fires** -- validate A-2 first. A check that
   reds on foreign tokens is the documented way this gate dies
   - Verify: the design names the identity test, not the count test
3. **Phase: Wire it**
   - Verify: AC-1, AC-2, AC-3
4. **Phase: Clear HEAD**
   - Verify: AC-4, AC-5, AC-6
5. **Phase: Record which rule the check enforces**
   - Verify: `make ze-rules-condensed` then `make ze-rules-lint`

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] **Commit A:** code + tests + spec
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations

- This makes the ceiling reachable. It does not lower it. The 430-token triage
  described in `test/relax-ceiling.txt` is separate work and stays separate.
- If A-3 confirms that `c_test_weakening`'s one-file-per-edit blindness is what
  produces most tokens, the higher-value spec is that fix, and this one should
  say so and stay small.
