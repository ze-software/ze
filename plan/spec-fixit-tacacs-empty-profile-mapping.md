# Spec: fixit-tacacs-empty-profile-mapping

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/config-surface.md` - YANG vs env var; YANG constraint decision
4. `internal/component/tacacs/authenticator.go`, `internal/component/tacacs/config.go`,
   `internal/component/aaa/login_profiles.go`, `internal/component/authz/authz.go`

## Task

A live, config-reachable privilege escalation (verified end to end). An operator who maps a TACACS+
privilege level to an EMPTY profile list (`tacacs-profile { level 15; }`, with no
`profile` entries) silently hands every user at that level **admin**, the exact
opposite of the restriction they were expressing.

Approved by Thomas as a standalone fix, ahead of and independent of
`plan/spec-fixit-authz-admin-fallthrough.md`'s open policy questions. This spec
does NOT answer those questions and does not touch that spec.

**Scope: the empty-mapping defect only.** Explicitly out of scope (they belong to
the authz spec's pending policy decisions): what a user with no applicable profile
is entitled to in general; the three fall-throughs in `Store.Authorize`; the
`hasUsers` logic; the bootstrap-admin question. No reject/allow semantics change
for any other shape.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/config-surface.md` - whether the constraint belongs in YANG
  → Decision: default answer is YANG config; a constraint that can be expressed
    natively in the schema should be, but see A-3: this validator does not enforce it.
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - fix at the source
  → Constraint: the deny must happen where the empty set is produced, not by
    weakening a test or patching a downstream consumer.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8907.md` - TACACS+ authentication reply / priv-lvl semantics
  → Constraint: RFC 8907 defines priv-lvl on the wire; it does NOT define how a
    priv-lvl maps to local authorization. The mapping is a ze config concept, so
    the empty-mapping decision is ze policy, not protocol.

**Key insights:**
- The escalation is not in authz. authz behaves as designed given its input. The
  defect is that tacacs feeds it "authenticated, zero profiles", a shape that
  authz reads as "no opinion" rather than "deny".
- "Present in the map" and "names a profile" are different questions. The code
  asked the first and needed the second.

## Current Behavior (MANDATORY)

**Source files read:** (all six links confirmed at the producing function)
- [ ] `internal/component/tacacs/config.go` (lines 99-106) - `ExtractConfig` assigns
  `cfg.PrivLvlMap[lvl] = entry.GetSlice("profile")` **unconditionally**, so a level
  with no profiles becomes a PRESENT key with an empty value.
  → Constraint: this is the producer of the empty-but-present map entry.
- [ ] `internal/component/tacacs/authenticator.go` (line 88 pre-fix) - `handlePass` does
  `profiles, ok := a.privLvlMap[privLvl]`. A level mapped to an EMPTY list is still
  PRESENT, so `ok` is true, the unmapped-denies guard at lines 89-94 is skipped, and it
  returns `Authenticated: true, Profiles: <empty>` at lines 98-102.
  → Constraint: this is the defect site and the fix site.
- [ ] `internal/component/aaa/login_profiles.go` (lines 45-48) - `RecordLoginProfiles`
  returns early when `len(profiles) == 0`: an empty set records NOTHING.
  → Constraint: keystone. The empty set does not merely record an empty entry, it
    records no entry at all, making the user indistinguishable from "never seen".
- [ ] `internal/component/authz/authz.go` (lines 385-390) - with no assignment and no
  login profiles, and `hasUsers == false`, `Store.Authorize` returns
  `BuiltinAdminProfile()`.
  → Constraint: out of scope to change. It is correct given its input contract.
- [ ] `internal/component/config/tree.go` (lines 186-190) - `Tree.GetSlice` returns nil
  when the key is not set **or every member is deactivated**.
  → Constraint: a second, non-obvious route to the empty shape: `deactivate` on the
    leaf-list members reaches it even if the leaf-list was written non-empty.
- [ ] `internal/component/config/yang/validator.go` (lines 632, 669, 782-795) - `walkTree`
  iterates only keys PRESENT in the data map and skips empty-string leaf-lists, so
  `checkCardinality` is never called with count 0.
  → Constraint: `min-elements` cannot reject a zero-entry leaf-list. See A-3.
- [ ] `internal/component/tacacs/yang/ze-tacacs-conf.yang` (lines 89-92 pre-fix) -
  `leaf-list profile` declared with NO `min-elements`, so `tacacs-profile { level 15; }`
  with zero profile entries is valid config.
  → Constraint: config is the entry point; nothing rejects the empty shape here.

**Behavior to preserve:**
- A non-empty mapping authenticates and yields exactly those profiles (unchanged).
- An unmapped level denies with `ErrAuthRejected` and a WARN (unchanged).
- FAIL / ERROR / connection-failure handling (unchanged).
- `Store.Authorize` fall-throughs and `hasUsers` logic (explicitly out of scope).

**Behavior to change:**
- A priv-lvl present in the map but naming ZERO profiles must deny exactly as an
  unmapped level does, with the same WARN shape.

## Data Flow (MANDATORY)

### Entry Point
- Config: `system.authentication.tacacs-profile <level> { profile [ ... ]; }`
- A TACACS+ server AUTHEN reply with status PASS carrying a priv-lvl.

### Transformation Path
1. `ExtractConfig` (`tacacs/config.go:99`) → `PrivLvlMap[lvl] = GetSlice("profile")`
   → empty leaf-list yields a present key with an empty value.
2. `handlePass` (`tacacs/authenticator.go:88`) → `, ok :=` reports "mapped".
3. `AuthResult{Authenticated: true, Profiles: []}` returned.
4. `profileRecordingAuthenticator.Authenticate` (`aaa/login_profiles.go:83-89`) →
   `RecordLoginProfiles` → **no-op** for an empty slice.
5. `Store.Authorize` (`authz/authz.go:352-390`) → no assignment, no login profiles,
   `hasUsers == false` → `BuiltinAdminProfile()`. **Escalation.**

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ tacacs | `GetSlice` into `PrivLvlMap` | [ ] |
| tacacs ↔ aaa | `AuthResult.Profiles` | [ ] |
| aaa ↔ authz | `loginProfiles` sync.Map, name-only | [ ] |

### Integration Points
- `NewTacacsAuthenticator` is the only consumer of `PrivLvlMap` (grep: sole hit at
  `authenticator.go:88`). No sibling call site inside tacacs needs the same guard.

### Architectural Verification
- [ ] No bypassed layers (fix sits at the producer of the empty set)
- [ ] No unintended coupling (tacacs does not learn about authz internals; it only
      stops emitting a shape whose meaning it cannot control)
- [ ] No duplicated functionality (reuses the existing unmapped-denies branch)
- [ ] Zero-copy preserved where applicable (N/A)
- [ ] Registration over hardcoding (N/A — no new command/family/handler)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An empty `profile` leaf-list yields a PRESENT key with an empty value in `PrivLvlMap` | `tacacs/config.go:103` assigns unconditionally; `tree.go:186` returns nil for absent/all-deactivated | The bug does not exist as described | Read producer + RED test | confirmed |
| A-2 | An empty `Profiles` set reaches the admin fall-through rather than being recorded as empty | `aaa/login_profiles.go:46` early-returns on `len(profiles)==0`; `authz.go:385-390` | The escalation would not occur; only a cosmetic defect | Read producer | confirmed |
| A-3 | `min-elements 1` in YANG is enforced by the loader for a zero-entry leaf-list | `config/yang/validator.go:786` implements the check | The YANG layer is decorative; the code fix is the only real one | Probe test through `config.ParseTreeWithYANG` | **broken** |
| A-4 | `PrivLvlMap` has no other consumer needing the same guard | grep `privLvlMap` across `internal/ pkg/ cmd/` | A sibling would still escalate | grep | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator today relies on an empty mapping as a "allow, profiles come from elsewhere" idiom and the deny locks them out | Login denied after upgrade with WARN "unmapped privilege level" | Accepted: the current behavior grants admin, which no operator can be intentionally relying on as a restriction. Denying is the fail-closed direction. |
| R-2 | `min-elements 1` reads as an enforced guarantee to a future maintainer while the validator ignores it | A future session assumes config is rejected and removes the code guard | Documented here and in the learned summary; the code guard is the load-bearing one and has direct tests |
| R-3 | The same escalation exists in the RADIUS backend and is NOT fixed here | RADIUS Access-Accept with no profile attr and no `default-profile` | Reported to Thomas; out of this spec's approved scope (see Known Limitations) |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| TACACS+ PASS reply, priv-lvl mapped to empty list | → | `handlePass` (`tacacs/authenticator.go:88-107`) | `TestTacacsAuthenticatorProfileMappingShapes/empty_list_denies` |
| TACACS+ PASS reply, priv-lvl mapped to nil (all members deactivated) | → | `handlePass` | `TestTacacsAuthenticatorProfileMappingShapes/nil_list_denies` |
| TACACS+ PASS reply, priv-lvl mapped to a real profile | → | `handlePass` | `TestTacacsAuthenticatorProfileMappingShapes/non-empty_list_authenticates` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | PASS reply, priv-lvl present in `PrivLvlMap` with an empty/nil profile list | `ErrAuthRejected`, `Authenticated == false`, WARN "TACACS+ unmapped privilege level" with username + priv-lvl (same shape as the unmapped branch) |
| AC-2 | Any `PrivLvlMap` shape, PASS reply | `Authenticated == true` implies `len(Profiles) > 0` (invariant) |
| AC-3 | PASS reply, priv-lvl mapped to a non-empty list | Unchanged: authenticates, `Profiles` equals the configured list |
| AC-4 | PASS reply, priv-lvl absent from `PrivLvlMap` | Unchanged: `ErrAuthRejected`, WARN |
| AC-5 | YANG: `tacacs-profile` `profile` leaf-list | Declares `min-elements 1` and a description stating at least one profile is required, and that a level is denied by omission. NOTE: not enforced by the loader (A-3). |
| AC-6 | Config `tacacs-profile 9 { }` (no profile entries) | `ExtractConfig` yields `PrivLvlMap[9]` PRESENT with an empty value (characterization: proves the defect is config-reachable, and that `ok` alone cannot detect it) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `tacacs-profile 15 { }` and logs in as a TACACS+ user | config → `ExtractConfig` → `PrivLvlMap[15] = []` → `handlePass` → deny | `TestTacacsAuthenticatorProfileMappingShapes/empty_list_denies` |
| 2 | Writes `tacacs-profile 15 { profile [ admin ]; }` and logs in | config → `PrivLvlMap[15] = [admin]` → `handlePass` → success with `[admin]` | `TestTacacsAuthenticatorProfileMappingShapes/non-empty_list_authenticates` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTacacsAuthenticatorProfileMappingShapes` | `internal/component/tacacs/authenticator_test.go` | AC-1, AC-3, AC-4 — table over empty / nil / non-empty / unmapped | RED→GREEN |
| `TestTacacsAuthenticatorAuthenticatedImpliesProfiles` | `internal/component/tacacs/authenticator_test.go` | AC-2 — the invariant, independent of map shape | RED→GREEN |
| `TestExtractConfigPrivLvlMapEmptyProfileList` | `internal/component/tacacs/config_test.go` | AC-6 — characterization: an empty leaf-list reaches `PrivLvlMap` as PRESENT-but-empty | Green both before and after (by design — see note) |

**Note on the third test:** it pins `ExtractConfig`, which this fix does NOT change, so
it passes before and after. It is a characterization test, not a fix test: it records
WHY `handlePass` must test `len(profiles) == 0` instead of trusting `ok`, and it fails
loudly if a future change makes empty entries absent instead (which would silently make
the `handlePass` guard look redundant). The RED→GREEN evidence for the fix itself comes
from the first two rows.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| profile leaf-list length | 1..N | 1 (authenticates) | 0 (denies — the fix) | N/A (no max) |
| priv-lvl | 0..15 | covered by existing `TestExtractConfigPrivLvlMap` and the YANG `range "0..15"`; unchanged by this spec | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| - | - | None added. The defect is a pure decision inside `handlePass`; reaching it end-to-end needs a live TACACS+ server, which the existing suite models with the in-process `newTestServer` harness the unit tests already use. | N/A |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| - | - | - | N/A: no wire behavior changes. The AUTHEN reply is parsed identically; only the local mapping decision changes. | N/A |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/tacacs/authenticator.go` - `handlePass` treats a present-but-empty
  profile list as unmapped; doc comment updated to state the contract.
- `internal/component/tacacs/yang/ze-tacacs-conf.yang` - `min-elements 1` + description.
- `internal/component/tacacs/authenticator_test.go` - the two tests above.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] Yes | `internal/component/tacacs/yang/ze-tacacs-conf.yang` — `min-elements 1` |
| YANG validation constraints | [ ] Yes (declared, not enforced — A-3) | as above |
| YANG custom validators | [ ] No | A `ze:validate` validator could enforce it, but validators run per-value and an ABSENT leaf-list has no value to validate — the same gap as A-3. Enforcing zero-entry cardinality belongs in `walkTree`, which is a repo-wide change (see Known Limitations). |
| CLI commands/flags | [ ] No | no new command |
| Doctor check | [ ] No | no new runtime dependency |
| Prometheus counters | [ ] No | the WARN log is the existing observable |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No | behavior fix, not a feature |
| 2 | Config syntax changed? | [ ] No | the syntax is unchanged; a previously-accepted-but-broken shape now denies at runtime. The YANG `description` carries the operator-facing statement. |
| 3 | CLI command added/changed? | [ ] No | - |
| 4 | API/RPC added/changed? | [ ] No | - |
| 5 | Plugin added/changed? | [ ] No | - |
| 6 | Has a user guide page? | [ ] Open | `docs/guide/` grep pending — see Pre-Commit Verification |
| 9 | RFC behavior implemented/changed? | [ ] No | the mapping is ze policy, not RFC 8907 |
| 16 | Changed source file referenced by doc source anchors? | [ ] Open | grep pending — see Pre-Commit Verification |

## Files to Create
- None. (This spec + a learned summary at closure.)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Current Behavior — all six links confirmed at the producer |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phase 1 below |
| 5. Full verification | `make ze-lint-changed`, scoped unit tests |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary; two-commit closure |

### Implementation Phases

1. **Phase: Deny the empty mapping (TDD)** — the whole fix.
   - Tests: `TestTacacsAuthenticatorProfileMappingShapes`,
     `TestTacacsAuthenticatorAuthenticatedImpliesProfiles`
   - Files: `tacacs/authenticator.go`, `tacacs/yang/ze-tacacs-conf.yang`
   - Verify: RED (empty mapping authenticates) → implement → GREEN

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 each have implementation + test with file:line |
| Correctness | The deny uses the SAME WARN shape and the SAME error as the unmapped branch — an empty mapping is not a new third outcome |
| Data flow | tacacs stops emitting the ambiguous shape; authz is untouched |
| Rule: no-workarounds | Fixed at the producer (`handlePass`), not by patching authz or weakening a test |
| Rule: sibling call-site audit | Every consumer of `PrivLvlMap` and every producer of `Authenticated: true` checked |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Empty mapping denies | `go test -run TestTacacsAuthenticatorProfileMappingShapes` |
| Non-empty unchanged | same test, `non-empty_list_authenticates` rows |
| YANG constraint present | `grep -n "min-elements" internal/component/tacacs/yang/ze-tacacs-conf.yang` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | A server-supplied priv-lvl must never select a code path that grants more than the operator's mapping names |
| Fail-closed | The new branch denies; it cannot grant. Verified by AC-2's invariant test. |
| Error leakage | The WARN logs username + priv-lvl only, matching the existing unmapped branch. No profile names or secrets. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-3: `min-elements 1` would reject `tacacs-profile 15 { }` at config load | The validator NEVER reaches `checkCardinality` for a zero-entry leaf-list. `walkTree` (`validator.go:632`) iterates only keys PRESENT in the data map, so an absent leaf-list is never visited; and a present-but-empty one is skipped by the `str != ""` guard at `validator.go:669`. `min-elements` is therefore enforced only for counts >= 1, i.e. never for the case it exists to catch. | Probe test through `config.ParseTreeWithYANG` on both shapes: both returned `err=<nil>` | The YANG layer cannot be relied on. The code fix in `handlePass` is the only load-bearing one. `min-elements 1` is retained as correct declared intent (VRRP precedent) but is documented as non-enforcing. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| YANG-only fix (`min-elements 1`) | Not enforced (A-3, proven by probe) | Code guard in `handlePass`, with the YANG constraint kept as declared intent |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Trusting a YANG constraint because the validator contains code implementing it, without checking the constraint is REACHED | 1st (here) | "A schema constraint is not enforced until you have seen it reject a config. `checkCardinality` has unit tests that pass while the constraint never fires in production." | Proposed to Thomas; the unit tests at `config/yang/validator_test.go:35-83` test the helper in isolation and gave false confidence |

## Design Insights

- **The unit test that proves nothing.** `TestCheckCardinality` (`validator_test.go:35`)
  exhaustively tests `checkCardinality`, including `{"exactly one but zero", 1, 1, 0, true, "too few"}`.
  It passes. The function is correct. It is simply never CALLED with count 0, because
  `walkTree` only iterates keys present in the data. A green, thorough, correct unit
  test on an unreachable path. This is the "pure functions with only integration
  coverage" category inverted: the helper has unit coverage, the wiring has none.
- **"Present" vs "meaningful" is the whole bug.** `, ok := m[k]` answers a question
  about the MAP. The code needed an answer about the VALUE. Every `, ok :=` lookup
  whose value is a slice or a map has this latent shape.
- **An empty set is not a restriction, it is an abstention.** Downstream,
  `RecordLoginProfiles` drops it and authz reads the absence as "no opinion" and
  supplies admin. A layer that emits "authenticated with nothing attached" is not
  emitting a weak permission, it is declining to answer, and something else will
  answer for it.

## Core Insight

The escalation lives in the gap between two locally reasonable decisions:
`RecordLoginProfiles` treats an empty set as "nothing to say" (correct: it must not
erase a previous login's real profiles), and `Store.Authorize` treats "nothing
recorded" as "no opinion" (correct given its contract). Neither is wrong. The bug is
that tacacs was allowed to emit the empty set at all. Fix the producer of the
ambiguous value, not the consumers that each interpret it defensibly.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Guard in `handlePass` (`!ok \|\| len(profiles) == 0`) | YANG `min-elements` alone | The YANG layer does not enforce zero-entry cardinality (A-3, proven). The code guard is the only one that fires. |
| Reuse the existing unmapped branch verbatim (same WARN, same error) | A distinct "empty mapping" warning | An empty mapping and an absent mapping mean the same thing to the operator: this level names no profiles. One outcome, one message, no new semantics. Also keeps the diff to a single condition. |
| Keep `min-elements 1` in YANG despite non-enforcement | Remove it as dead | It is the semantically correct declaration, matches the VRRP precedent (`ze-vrrp-conf.yang:56,165`), documents intent to operators via `description`, and becomes live for free if the `walkTree` gap is ever closed. Documented as non-enforcing so nobody removes the code guard trusting it. |
| Also deny the nil/deactivated shape | Only handle the literal empty leaf-list | `Tree.GetSlice` returns nil when every member is deactivated (`tree.go:183-185`), so `deactivate` reaches the same escalation without an empty leaf-list in the config text. `len() == 0` covers both. |
| Do NOT fix the RADIUS sibling here | Fix both in this spec | Same defect class, but denying a RADIUS Access-Accept that names no profiles is a policy call adjacent to the authz spec's pending questions, and Thomas scoped this to tacacs. Reported instead. |

## Known Limitations

- **`min-elements` is not enforced for zero-entry leaf-lists anywhere in the repo.**
  `walkTree` (`config/yang/validator.go:632`) only visits keys present in the data map,
  and skips empty-string values at `:669`. Closing this would newly reject configs that
  load today (e.g. a VRRP `vrrp-group` with no `virtual-address`, whose YANG already
  declares `min-elements 1`), so it is a repo-wide behavior change needing its own spec
  and Thomas's decision. Not done here.
- **The RADIUS backend has the same escalation and is NOT fixed.**
  `radius/authenticator.go:131-138` returns `Authenticated: true, Profiles: a.mapProfiles(resp)`;
  `mapProfiles` (`:150-161`) falls back to `a.defaultProfiles`, which is
  `radiusTree.GetSlice("default-profile")` (`radius/config.go:103`) and is nil when
  `default-profile` is not configured. An Access-Accept with no profile attribute and no
  configured default therefore yields the identical empty set and the identical admin
  fall-through. Reachable from the DEFAULT config (no `default-profile` is the common
  case), so arguably wider than the tacacs one. Needs Thomas's decision.
- The `Store.Authorize` fall-throughs, `hasUsers`, and bootstrap-admin remain as-is
  (out of scope by instruction; owned by `plan/spec-fixit-authz-admin-fallthrough.md`).

## RFC Documentation

No RFC-enforcing code added. RFC 8907 defines the priv-lvl on the wire but not its
mapping to local authorization, so no `// RFC 8907 Section X.Y` comment applies to the
new branch.

## Implementation Summary

### What Was Implemented
- `handlePass` treats a present-but-empty (or nil) profile list as unmapped:
  `if !ok || len(profiles) == 0` (`tacacs/authenticator.go:104`), denying with the
  existing WARN and `ErrAuthRejected`. A comment records the escalation chain with
  file:line so the guard is not "simplified" away later.
- Doc comment on `Authenticate` updated: the rejected case now reads "a priv-lvl that
  names no profiles, whether unmapped or mapped to an empty list".
- `min-elements 1` + an operator-facing description on the `profile` leaf-list.
- Two tests (see TDD Test Plan).

### Bugs Found/Fixed
- The reported escalation (fixed).
- **Found, not fixed:** `min-elements` is inert for zero-entry leaf-lists repo-wide (A-3).
- **Found, not fixed:** the RADIUS sibling escalation (R-3).

### Documentation Updates
- Pending: rows 6 and 16 of the Documentation Update Checklist (grep evidence to be
  recorded in Pre-Commit Verification).

### Deviations from Plan
- A-3 broke: the YANG layer was expected to enforce and does not. Recorded in the
  Mistake Log. The design still keeps the constraint, but its role changed from
  "defence in depth" to "declared intent", and the code guard carries the fix alone.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Confirm each link in the bug chain | Done | Current Behavior | All six read at the producer |
| Fix the empty-mapping defect | Done | `tacacs/authenticator.go:104` | |
| Do not alter other reject/allow semantics | Done | single condition widened | AC-3/AC-4 tests guard this |
| Verify whether YANG enforces min-elements | Done | A-3, Mistake Log | Proven NOT enforced by probe |
| Spec recording root cause + ACs + assumptions | Done | this file | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `.../empty_list_denies`, `.../nil_list_denies` | RED→GREEN |
| AC-2 | Done | `TestTacacsAuthenticatorAuthenticatedImpliesProfiles` | |
| AC-3 | Done | `.../non-empty_list_authenticates*` | passed before and after |
| AC-4 | Done | `.../unmapped_level_denies`, existing `TestTacacsAuthenticatorUnmappedPrivLvl` | passed before and after |
| AC-5 | Done (declared, not enforced) | `ze-tacacs-conf.yang:89-99` | A-3 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestTacacsAuthenticatorProfileMappingShapes` | Pass | `tacacs/authenticator_test.go` | 5 subtests |
| `TestTacacsAuthenticatorAuthenticatedImpliesProfiles` | Pass | `tacacs/authenticator_test.go` | 4 map shapes |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/tacacs/authenticator.go` | Modified | guard + comments |
| `internal/component/tacacs/yang/ze-tacacs-conf.yang` | Modified | min-elements + description |
| `internal/component/tacacs/authenticator_test.go` | Modified | two tests added |

### Audit Summary
- **Total items:** 5 ACs + 2 tests + 3 files
- **Done:** all, with AC-5 qualified (declared, not enforced — A-3)
- **Partial:** none
- **Skipped:** none
- **Changed:** A-3 broke; see Deviations

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| An empty tacacs-profile mapping no longer authenticates | Unit test, RED→GREEN | RED: `INFO TACACS+ auth success username=user priv-lvl=9 profiles=[]` + `Expected error with "authentication rejected" in chain but got nil`. GREEN: `WARN TACACS+ unmapped privilege level username=user priv-lvl=9`, all 5 subtests PASS. |
| No other reject/allow semantics changed | Unit test | `non-empty_list_authenticates` and `unmapped_level_denies` pass identically before and after; full `tacacs` + `authz` + `aaa` package suites pass |
| YANG enforcement question answered | Probe test | `PROBE absent leaf-list err=<nil>` and `PROBE empty brackets err=<nil>` with `min-elements 1` in place: NOT enforced |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | (pending) | `/ze-review` not run in this session | - | Thomas to run at closure, or a follow-up session |

### Fixes applied
- Sibling call-site audit performed inline (per `ai/rules/before-writing-code.md` item 9):
  `privLvlMap` has one consumer; the two other producers of `Authenticated: true`
  (`authz/auth.go:59` local, `radius/authenticator.go:135`) were read. Local is safe
  (profiles come from a config assignment keyed by username). RADIUS is NOT safe and is
  recorded as R-3 / Known Limitations.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | not run | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/tacacs/authenticator.go` | Yes | modified in place; `grep -n "len(profiles) == 0"` hits `:104` |
| `internal/component/tacacs/yang/ze-tacacs-conf.yang` | Yes | `grep -n "min-elements"` hits `:91` |
| `internal/component/tacacs/authenticator_test.go` | Yes | tests run and pass |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | empty/nil mapping denies with the unmapped WARN | GREEN log: `WARN TACACS+ unmapped privilege level username=user priv-lvl=9` / `priv-lvl=7`; both subtests PASS |
| AC-2 | authenticated implies profiles | `TestTacacsAuthenticatorAuthenticatedImpliesProfiles` PASS |
| AC-3 | non-empty unchanged | `non-empty_list_authenticates` PASS; `INFO ... priv-lvl=15 profiles=[admin]` |
| AC-4 | unmapped unchanged | `unmapped_level_denies` PASS |
| AC-5 | YANG declares min-elements 1 | `ze-tacacs-conf.yang:91` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| TACACS+ PASS reply → `handlePass` | none (unit harness `newTestServer` drives a real TACACS+ exchange over a loopback socket) | Yes — the tests exercise the client through `Authenticate`, not `handlePass` directly |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `config.go:103` unconditional assign; RED test showed `profiles=[]` with success |
| A-2 | confirmed | `login_profiles.go:46` early return; `authz.go:385-390` |
| A-3 | **broken** | Probe: both empty shapes returned `err=<nil>` with `min-elements 1` present |
| A-4 | confirmed | grep `privLvlMap` → sole consumer `authenticator.go:88` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| rows 6, 16 | grep of `docs/` for tacacs-profile / source anchors | [ ] Pending |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-fixit-tacacs-empty-profile-mapping.md`
