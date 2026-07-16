# Spec: fixit-radius-empty-profile-mapping

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
3. `ai/rules/no-workarounds-for-missing-behavior.md` - fix at the producer
4. `internal/component/radius/authenticator.go`, `internal/component/radius/config.go`,
   `internal/component/aaa/login_profiles.go`, `internal/component/authz/authz.go`
5. `internal/component/tacacs/authenticator.go` - the sibling fix this one mirrors

## Task

A live privilege escalation in the RADIUS admin-auth path, verified end to end. A RADIUS
server that answers Access-Accept **without a Filter-Id attribute**, against out-of-the-box
config with no `default-profile`, grants that user **admin**.

This is the same defect class as `plan/learned/`'s TACACS empty-profile-mapping fix
(spec `plan/spec-fixit-tacacs-empty-profile-mapping.md`), applied to the RADIUS sibling.
Unlike the TACACS variant it needs **no operator misconfiguration**: `default-profile` is
optional and normally unset, because the server usually supplies the profile. That is why
Thomas approved it as a standalone fix now.

Approved by Thomas as a standalone fix, ahead of and independent of
`plan/spec-fixit-authz-admin-fallthrough.md`'s open policy questions. This spec does NOT
answer those questions and does not touch that spec.

**Scope: the empty/absent profile-set defect in the RADIUS path only.** Explicitly out of
scope (they belong to the authz spec's pending policy decisions): the three fall-throughs
in `Store.Authorize`; the `hasUsers` logic; what a profile-less user is entitled to in
general; the bootstrap-admin question. No reject/allow semantics change for any other shape.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - fix at the source
  → Constraint: the deny must happen where the empty set is produced (the Access-Accept
    branch), not by weakening a test or patching a downstream consumer (authz, aaa).
- [ ] `ai/rules/before-writing-code.md` - sibling call-site audit
  → Constraint: every producer of `Authenticated: true` and every caller of `mapProfiles`
    must be checked before adding the guard to one of them. See A-4 / A-5.
- [ ] `ai/rules/config-surface.md` - whether the constraint belongs in YANG
  → Decision: NOT here. See "Key Design Decisions": `min-elements 1` is semantically
    correct for the TACACS `profile` leaf-list but WRONG for RADIUS `default-profile`,
    which is legitimately optional. The YANG divergence from the TACACS fix is forced,
    not gratuitous.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2865.md` - Access-Accept / Filter-Id (5.11) / Class (5.25)
  → Constraint: RFC 2865 defines the reply attributes on the wire; it does NOT define how
    a reply attribute maps to local authorization, nor does it require Filter-Id in an
    Access-Accept. An Accept with no Filter-Id is a legal packet. The mapping to ze
    profiles is a ze config concept, so the empty-profile-set decision is ze policy, not
    protocol. No wire behavior changes here.

**Key insights:**
- The escalation is not in authz. authz behaves as designed given its input. The defect is
  that radius feeds it "authenticated, zero profiles", a shape authz reads as "no opinion"
  rather than "deny".
- The TACACS variant needed an operator to write a meaningless mapping. This one needs
  nothing: `nil` is what `GetSlice` returns for the default config.
- "The server accepted" and "the server named a profile" are different questions. The code
  answered the first and returned success without asking the second.

## Current Behavior (MANDATORY)

**Source files read:** (all six links confirmed at the producing function)
- [ ] `internal/component/radius/yang/ze-radius-conf.yang` (lines 90-95) - `leaf-list
  default-profile { type string; }` with NO `min-elements`, not `mandatory`.
  → Constraint: leaving it unset is ordinary, valid, expected default config. This is the
    entry point and it cannot be closed here (see Key Design Decisions).
- [ ] `internal/component/radius/config.go` (line 103) - `ExtractConfig` does
  `cfg.DefaultProfiles = radiusTree.GetSlice("default-profile")` **unconditionally**.
  → Constraint: producer of the nil `DefaultProfiles`. Unchanged by this fix.
- [ ] `internal/component/config/tree.go` (lines 186-190) - `Tree.GetSlice` returns nil
  when the key is not set **or every member is deactivated**.
  → Constraint: two routes to nil. `deactivate` reaches it even when `default-profile` was
    written non-empty. `len() == 0` covers both.
- [ ] `internal/component/radius/authenticator.go` (lines 150-161 pre-fix) - `mapProfiles`
  collects `resp.FindAllAttr(a.profileAttr)`, skipping empty values, and when it found
  none returns `a.defaultProfiles` — which is nil when unconfigured.
  → Constraint: producer of the empty set. Returns nil, not an error: it cannot signal
    "nothing resolved" to its caller today.
- [ ] `internal/component/radius/authenticator.go` (lines 129-138 pre-fix) - the
  `CodeAccessAccept` case returns `aaa.AuthResult{Authenticated: true, Profiles:
  a.mapProfiles(resp), Source: aaaName}` with no check on the mapped set.
  → Constraint: this is the defect site and the fix site.
- [ ] `internal/component/aaa/login_profiles.go` (lines 45-48) - `RecordLoginProfiles`
  returns early when `len(profiles) == 0`: an empty set records NOTHING.
  → Constraint: keystone. The empty set does not record an empty entry, it records no
    entry at all, making the user indistinguishable from "never seen".
- [ ] `internal/component/authz/authz.go` (lines 385-390) - with no assignment and no login
  profiles, and `hasUsers == false`, `Store.Authorize` returns `BuiltinAdminProfile()`.
  → Constraint: out of scope to change. It is correct given its input contract.

**Behavior to preserve:**
- An Access-Accept carrying profile attributes authenticates with exactly those profiles.
- An Access-Accept carrying none, with `default-profile` configured, authenticates with the
  configured defaults. This is the documented AC-6 behavior and MUST NOT regress.
- Access-Reject denies with `ErrAuthRejected` (chain stops).
- Transport/protocol failures return a non-`ErrAuthRejected` error so the chain falls
  through to the next backend (R-4). Unchanged.
- The configured `profile-attribute` (Filter-Id or Class) selects the carrier. Unchanged.
- `Store.Authorize` fall-throughs and `hasUsers` logic (explicitly out of scope).

**Behavior to change:**
- An Access-Accept that resolves to ZERO profile names must deny with `ErrAuthRejected` and
  a WARN, instead of returning success with an empty set.

## Data Flow (MANDATORY)

### Entry Point
- Config: `system.authentication.radius { default-profile [ ... ]; }` — **absent by default**.
- A RADIUS Access-Accept, with or without the configured `profile-attribute`.

### Transformation Path
1. `ExtractConfig` (`radius/config.go:103`) → `DefaultProfiles = GetSlice("default-profile")`
   → nil when unset or all members deactivated.
2. `newRadiusAuthenticator` (`radius/authenticator.go:58`) → `defaultProfiles: nil`.
3. `mapProfiles` (`radius/authenticator.go:150-161`) → no reply attrs → returns nil.
4. `Authenticate` `CodeAccessAccept` case (`radius/authenticator.go:130-138`) →
   `AuthResult{Authenticated: true, Profiles: nil}`.
5. `profileRecordingAuthenticator.Authenticate` (`aaa/login_profiles.go:83-89`) →
   `RecordLoginProfiles` → **no-op** for an empty slice (`:46`).
6. `Store.Authorize` (`authz/authz.go:352-390`) → no assignment, no login profiles,
   `hasUsers == false` → `BuiltinAdminProfile()`. **Escalation.**

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ radius | `GetSlice` into `DefaultProfiles` | [ ] |
| RADIUS wire ↔ radius | `resp.FindAllAttr(profileAttr)` | [ ] |
| radius ↔ aaa | `AuthResult.Profiles` | [ ] |
| aaa ↔ authz | `loginProfiles` sync.Map, name-only | [ ] |

### Integration Points
- `mapProfiles` has exactly one caller (`authenticator.go:131`, grep). The guard at that
  call site covers every path that produces `Authenticated: true` in this component.
- `defaultProfiles` is read only at `authenticator.go:158` (grep). No sibling consumer.

### Architectural Verification
- [ ] No bypassed layers (fix sits at the producer of the empty set)
- [ ] No unintended coupling (radius does not learn about authz internals; it only stops
      emitting a shape whose meaning it cannot control)
- [ ] No duplicated functionality (reuses the existing reject error + logging idiom)
- [ ] Zero-copy preserved where applicable (N/A)
- [ ] Registration over hardcoding (N/A — no new command/family/handler)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An unset `default-profile` yields nil `DefaultProfiles`, reachable from DEFAULT config | `radius/config.go:103` unconditional assign; `tree.go:186-190` returns nil when unset; YANG `:90-95` has no `min-elements`/`mandatory` | The bug needs misconfiguration, lowering severity to the TACACS level | Read producer + RED test | confirmed |
| A-2 | An empty `Profiles` set reaches the admin fall-through rather than being recorded as empty | `aaa/login_profiles.go:46` early-returns on `len(profiles)==0`; `authz.go:385-390` returns `BuiltinAdminProfile()` when `hasUsers==false` | The escalation would not occur; only a cosmetic defect | Read producer | confirmed |
| A-3 | An Access-Accept with no Filter-Id is a legal RFC 2865 packet, so this is a real server shape and not a malformed one | RFC 2865 does not require Filter-Id in an Access-Accept; `mapProfiles` already has a `len(profiles)==0` path, i.e. the code anticipated it | The case would be unreachable and the fix pointless | Read RFC summary + producer | confirmed |
| A-4 | `mapProfiles` has no other caller needing the same guard | grep `mapProfiles` across `internal/ pkg/ cmd/` → sole call at `authenticator.go:131` | A sibling call site would still escalate | grep | confirmed |
| A-5 | The other producers of `Authenticated: true` do not have this defect | grep `Authenticated: *true` → non-test hits are `radius/authenticator.go:135` (this fix), `tacacs/authenticator.go:116` (fixed by sibling spec), `authz/auth.go:59` (local) | An unfixed sibling would keep the escalation alive | grep + read `authz/auth.go:46-65` | confirmed |
| A-6 | `min-elements 1` on `default-profile` would be correct declared intent, mirroring TACACS | The TACACS fix added it to its `profile` leaf-list | The YANG half of the mirror is wrong and must be dropped | Read YANG semantics + `radius/yang/ze-radius-conf.yang:90-95` | **broken** — see Mistake Log |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator today runs RADIUS with no `default-profile` and a server that omits Filter-Id, and their users are silently admin. After this fix they are locked out of the RADIUS path | Logins denied after upgrade with WARN "RADIUS admin auth rejected: no profiles resolved" | Accepted, and it is the point: the current behavior grants admin, which no operator can be intentionally relying on as a restriction. Denying is the fail-closed direction. The WARN names the remedy (`default-profile`, or a server-side Filter-Id). |
| R-2 | The deny is mistaken for R-4's backend-error fallthrough by a future maintainer, who "restores" the fallthrough and reopens the escalation | A change turning the new branch's `ErrAuthRejected` into a plain error | The code comment states the distinction explicitly (an Accept is an answer, not a failure); AC-2's invariant test fails if success ever carries zero profiles; AC-5's test fails if the deny stops being `ErrAuthRejected` |
| R-3 | The YANG divergence from the TACACS fix (no `min-elements`) reads as an oversight and someone "completes the mirror", making `default-profile` effectively mandatory and breaking every valid config that omits it | A diff adding `min-elements 1` to `default-profile` | Documented in A-6, Mistake Log, and Key Design Decisions with the reason. The `description` carries the operator-facing statement instead. |
| R-4 (upstream) | Not a risk of this spec: `authz`'s admin fall-through remains for any OTHER producer of a profile-less authenticated user | - | Out of scope by instruction; owned by `plan/spec-fixit-authz-admin-fallthrough.md` |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Access-Accept with no Filter-Id, no `default-profile` configured | → | `Authenticate` `CodeAccessAccept` case (`radius/authenticator.go:130-146`) | `TestRadiusAuthenticateProfileResolutionShapes/no_attrs_no_default_denies` |
| Access-Accept with no Filter-Id, `default-profile` configured | → | same | `TestRadiusAuthenticateProfileResolutionShapes/no_attrs_with_default_authenticates` |
| Access-Accept with Filter-Id | → | same | `TestRadiusAuthenticateProfileResolutionShapes/attrs_present_authenticate` |
| Access-Accept whose Filter-Id values are all empty strings, no default | → | `mapProfiles` → same guard | `TestRadiusAuthenticateProfileResolutionShapes/empty_attr_values_no_default_denies` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Access-Accept carrying no `profile-attribute` values, `DefaultProfiles` nil or empty | `ErrAuthRejected`, `Authenticated == false`, empty `Profiles`, WARN "RADIUS admin auth rejected" with username + reason, `Source == aaaName` |
| AC-2 | Any config/reply shape, any response code | `Authenticated == true` implies `len(Profiles) > 0` (invariant) |
| AC-3 | Access-Accept carrying no `profile-attribute` values, `DefaultProfiles` non-empty | Unchanged (AC-6 behavior): authenticates, `Profiles` equals the configured defaults |
| AC-4 | Access-Accept carrying `profile-attribute` values | Unchanged: authenticates, `Profiles` equals the reply values, one per attribute instance |
| AC-5 | Access-Reject | Unchanged: `ErrAuthRejected`, `Authenticated == false` |
| AC-6 | Transport failure / unreachable server (R-4) | Unchanged: a non-`ErrAuthRejected` error so the chain tries the next backend. Distinct from AC-1. |
| AC-7 | YANG `default-profile` leaf-list | `description` states the consequence of omission (an Accept naming no profile is denied). NO `min-elements` — see A-6 / Key Design Decisions. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures RADIUS with no `default-profile`, server accepts a user without Filter-Id | config → `DefaultProfiles=nil` → `mapProfiles` → nil → deny | `TestRadiusAuthenticateProfileResolutionShapes/no_attrs_no_default_denies` |
| 2 | Adds `default-profile [ read-only ]` and the same user logs in | config → `DefaultProfiles=[read-only]` → `mapProfiles` → defaults → success | `TestRadiusAuthenticateProfileResolutionShapes/no_attrs_with_default_authenticates` |
| 3 | Configures the server to send `Filter-Id = netops` and the user logs in | wire → `FindAllAttr` → `[netops]` → success | `TestRadiusAuthenticateProfileResolutionShapes/attrs_present_authenticate` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRadiusAuthenticateProfileResolutionShapes` | `internal/component/radius/authenticator_test.go` | AC-1, AC-3, AC-4 — table over no-attrs/no-default, no-attrs/with-default, attrs-present, empty-attr-values | RED→GREEN |
| `TestRadiusAuthenticatedImpliesProfiles` | `internal/component/radius/authenticator_test.go` | AC-2 — the invariant, across reply/config shapes | RED→GREEN |
| `TestRadiusAuthenticateReject` (existing) | same | AC-5 — unchanged | Green before and after |
| `TestRadiusAuthenticateInfraError` (existing) | same | AC-6 — R-4 fallthrough unchanged | Green before and after |
| `TestRadiusAuthenticateAccept` (existing) | same | AC-3 — already configures `DefaultProfiles: [operator]`; proves the default path did not regress | Green before and after |
| `TestRadiusProfileMapping`, `TestRadiusProfileMappingClass` (existing) | same | AC-4 — unchanged | Green before and after |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| resolved profile-set length | 1..N | 1 (authenticates) | 0 (denies — the fix) | N/A (no max) |
| `default-profile` leaf-list length | 0..N | 0 is VALID config (absence is normal); the deny is a runtime decision, not a config rejection | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| - | - | None added. The defect is a pure decision inside `Authenticate`; the existing `mockRADIUSServer` harness drives a real Access-Request/Accept exchange over a loopback UDP socket, which is the same path the daemon takes. | N/A |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| - | - | - | N/A: no wire behavior changes. The Access-Accept is parsed identically; only the local mapping decision changes. | N/A |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/radius/authenticator.go` - the `CodeAccessAccept` case denies when the
  resolved profile set is empty; doc comments on `Authenticate` and `mapProfiles` updated.
- `internal/component/radius/yang/ze-radius-conf.yang` - `default-profile` description states
  the consequence of omission. No `min-elements` (A-6).
- `internal/component/radius/authenticator_test.go` - the two tests above.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] Description only | `internal/component/radius/yang/ze-radius-conf.yang` — no new leaf |
| YANG validation constraints | [ ] No | `min-elements 1` would be WRONG here (A-6): absence of `default-profile` is valid config, and the schema must keep accepting it. The deny is a runtime authorization decision. |
| YANG custom validators | [ ] No | Nothing to validate: the config is legal. |
| CLI commands/flags | [ ] No | no new command |
| Doctor check | [ ] No | no new runtime dependency |
| Prometheus counters | [ ] No | the WARN log is the existing observable, matching the sibling reject line at `:139-141` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No | behavior fix, not a feature |
| 2 | Config syntax changed? | [ ] No | syntax unchanged; a previously-accepted-but-broken runtime shape now denies. The YANG `description` carries the operator-facing statement. |
| 3 | CLI command added/changed? | [ ] No | - |
| 4 | API/RPC added/changed? | [ ] No | - |
| 5 | Plugin added/changed? | [ ] No | - |
| 6 | Has a user guide page? | [ ] **Yes** | `docs/guide/radius.md` "Profile mapping" (:92-94) stated "the user is authenticated with no profiles (and RBAC denies privileged actions)" — the exact false belief that hid this bug. RBAC GRANTED admin. Rewritten to state the rejection, the WARN, and the R-4 distinction. |
| 9 | RFC behavior implemented/changed? | [ ] No | RFC 2865 does not define the profile mapping; no wire change |
| 16 | Changed source file referenced by doc source anchors? | [ ] **Yes** | `docs/guide/radius.md:99` anchors `mapProfiles`; `docs/features.md:87` anchors `radiusAuthenticator.Authenticate, mapProfiles`. Both changed. `docs/features.md:87` row updated; the `radius.md` anchor now also names the Access-Accept branch. `docs/guide/configuration.md:1608` anchors the YANG container but makes no claim about `default-profile` semantics — no change needed. |

## Files to Create
- None. (This spec + a learned summary at closure.)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Current Behavior — all links confirmed at the producer |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phase 1 below |
| 5. Full verification | `make ze-lint-changed`, scoped unit tests, `make ze` |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary; two-commit closure (by Thomas — this session is commit-forbidden) |

### Implementation Phases

1. **Phase: Deny the empty profile set (TDD)** — the whole fix.
   - Tests: `TestRadiusAuthenticateProfileResolutionShapes`, `TestRadiusAuthenticatedImpliesProfiles`
   - Files: `radius/authenticator.go`, `radius/yang/ze-radius-conf.yang`
   - Verify: RED (Accept with no attrs and no default authenticates with an empty set) →
     implement → GREEN

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-7 each have implementation + test with file:line |
| Correctness | The deny is `ErrAuthRejected` (chain stops), NOT a plain error (chain continues). Conflating it with R-4 would let a local user of the same name shadow a RADIUS answer. |
| Data flow | radius stops emitting the ambiguous shape; authz and aaa untouched |
| Rule: no-workarounds | Fixed at the producer (the Accept branch), not by patching authz or weakening a test |
| Rule: sibling call-site audit | Every caller of `mapProfiles` and every producer of `Authenticated: true` checked (A-4, A-5) |
| Divergence from TACACS | Every difference from the TACACS fix is forced and documented (only the YANG half, A-6) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Empty profile set denies | `go test -run TestRadiusAuthenticateProfileResolutionShapes` |
| Configured default still authenticates | same test, `no_attrs_with_default_authenticates` row |
| Invariant holds | `go test -run TestRadiusAuthenticatedImpliesProfiles` |
| R-4 fallthrough intact | `go test -run TestRadiusAuthenticateInfraError` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | A server-supplied reply must never select a code path that grants more than the operator's config or the server's reply names |
| Fail-closed | The new branch denies; it cannot grant. Verified by AC-2's invariant test. |
| Error leakage | The WARN logs username only, matching the existing reject line at `:139-141`. No profile names, no secrets, no server key material. |
| Chain semantics | The deny must not be a plain error: a plain error falls through to the local backend, letting a local account shadow the RADIUS server's answer for the same name. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-6: `min-elements 1` on `default-profile` mirrors the TACACS fix and is correct declared intent | It is the OPPOSITE of correct here. In TACACS, `profile` sits under an explicitly-written `tacacs-profile <level>` entry: an operator who writes the entry and names no profile has written something meaningless, so `min-elements 1` states real intent. In RADIUS, `default-profile` is an optional leaf-list whose ABSENCE is the normal config (the server usually supplies Filter-Id, per AC-4). `min-elements 1` would declare it effectively mandatory and contradict the design. | Read the YANG (`:90-95`, no `mandatory`, no `min-elements`) against the TACACS shape while writing the mirror; the task statement itself says leaving it unset is "ordinary default config", which is exactly what `min-elements` would forbid | The YANG half of the mirror is dropped. Only the `description` changes. The code guard was already the only load-bearing fix (`min-elements` is inert repo-wide anyway), so the fix's strength is unaffected. Recorded as R-3 so nobody "completes the mirror" later. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Mirror the TACACS YANG change (`min-elements 1` on the profile leaf-list) | Semantically wrong for an optional leaf-list (A-6). Would also be inert regardless (see Known Limitations). | `description` states the consequence of omission; the code guard carries the fix alone |
| Guard inside `mapProfiles` (return an error / bool) | `mapProfiles`'s job is to map, and its nil return is meaningful to no one but its single caller. The decision "an Accept naming nothing is a denial" is an authorization decision and belongs beside the other response-code decisions, next to the Access-Reject branch it mirrors. | Guard in the `CodeAccessAccept` case |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Mirroring a sibling fix structurally without re-deriving whether each half is semantically valid in the new context | 1st (here, caught at design) | "A mirrored fix must be re-derived, not copied. For each element of the original fix, name why it applies here. The one that does not apply is the one that would have shipped a bug." | Recorded here; the YANG half would have made a valid config unloadable |

## Design Insights

- **The same defect, one config field apart, at very different severities.** TACACS needed
  an operator to write `tacacs-profile { level 15; }`. RADIUS needs nobody to do anything:
  `GetSlice` on an absent leaf-list returns nil, and nil is the default. Identical code
  shape, identical downstream chain, but the entry point moved from "misconfiguration" to
  "out-of-the-box". Severity is a property of reachability, not of the buggy line.
- **A mirror is not a copy.** Two of the three parts of the TACACS fix transfer verbatim
  (the guard, the WARN). The third (`min-elements 1`) inverts: correct there, wrong here,
  because the same leaf-list shape carries opposite intent in the two schemas. The reason
  the fixes look alike is that the ESCALATION is the same, not that the CONFIG is.
- **"Accepted" and "authorized" are different answers, and only one was being asked for.**
  The Accept branch asked the server "is this user real?" and treated the answer as a
  complete authorization result. The server had also answered "which profiles?" — with
  silence — and silence was read as assent.

## Core Insight

An Access-Accept with no profile attribute is an **answer**, not a **failure**, and that is
exactly what makes it dangerous. R-4's fallthrough exists because a backend that cannot
answer must not lock the operator out: the chain asks someone else. But a server that
accepts a user while naming no profile HAS answered, and the answer resolves to nothing.
Returning success hands the resolution to `authz`, which reads "nothing" as "no opinion"
and supplies admin. Falling through to the next backend would be equally wrong: it would
let a local account shadow a live server's verdict. The only answer that matches what
happened is a rejection: this backend authenticated the user and could not authorize them.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Deny with `aaa.ErrAuthRejected` in the `CodeAccessAccept` case when the resolved set is empty | Return success with empty profiles (status quo); return a plain error (R-4-style fallthrough) | Success escalates to admin via the authz fall-through. A plain error would make the chain try the local backend, letting a local user of the same name shadow the server's answer, and would misclassify an authoritative reply as an infrastructure failure. `ErrAuthRejected` says what happened: authenticated, not authorizable. Mirrors the TACACS fix exactly. |
| Distinguish this from R-4 by asking "did the backend answer?", not "did the login succeed?" | Treat any non-grant as a fallthrough | R-4 (`authenticator.go:123-127`) covers `SendToServers` failing: timeout, unreachable, no reply. The backend produced NO answer, so asking the next one is right and locking the operator out on an infra blip would be wrong. An Access-Accept is a well-formed reply from a reachable, authenticated server: the backend answered. The profile set resolving to zero is the CONTENT of that answer, not the absence of one. Different question, different branch, and the existing Access-Reject branch (`:139-141`) already proves the component treats "server answered no" as `ErrAuthRejected` rather than a fallthrough. |
| Guard the mapped result at the call site, not inside `mapProfiles` | Make `mapProfiles` return `([]string, bool)` or an error | Keeps `mapProfiles` a pure mapping function and puts the authorization decision beside the other response-code decisions, one line from the Access-Reject branch it mirrors. Also keeps the diff to one condition, matching the TACACS shape. |
| One WARN reusing the existing "RADIUS admin auth rejected" idiom, with a reason | A distinct log line / a new counter | An operator sees one concept, "RADIUS rejected this login", with the reason attached. The existing reject line at `:139-141` is the precedent, and the TACACS fix likewise reused its unmapped-level WARN rather than inventing a third outcome. WARN rather than the reject branch's INFO because this rejection is almost always a config or server-side gap the operator must act on, whereas an Access-Reject is routine. |
| NO `min-elements 1` on `default-profile` (diverge from the TACACS fix) | Mirror the TACACS YANG change | A-6. Absence of `default-profile` is valid, normal config: the server usually supplies Filter-Id (AC-4), and `min-elements 1` would declare the leaf-list effectively mandatory, contradicting the design and, if the validator ever started enforcing cardinality, breaking every config that correctly omits it. TACACS's `profile` sits under an entry the operator chose to write, so requiring content there states real intent. Same shape, opposite meaning. |
| Cover the all-empty-attribute-values shape | Only handle "no attributes" | `mapProfiles` (`:152-155`) already skips empty strings, so an Accept carrying `Filter-Id = ""` reaches the same nil. `len(profiles) == 0` after mapping covers attribute-absent, attribute-empty, and default-deactivated in one condition. |

## Known Limitations

- **`min-elements` is inert for zero-entry leaf-lists repo-wide.** `walkTree`
  (`config/yang/validator.go:632`) only visits keys present in the data map and skips
  empty strings at `:668`, so `checkCardinality` never fires for count 0. This is why the
  code guard is the only load-bearing fix. It is ALSO why the A-6 divergence costs nothing
  today: a `min-elements 1` on `default-profile` would be wrong but currently inert.
  Thomas has approved a separate skeleton spec for the validator gap (a concurrent agent is
  writing it); if that gap is ever closed, R-3 becomes live and this spec's reasoning is
  what stops `default-profile` from being wrongly made mandatory.
- **The `Store.Authorize` fall-throughs, `hasUsers`, and bootstrap-admin remain as-is.**
  Out of scope by instruction; owned by `plan/spec-fixit-authz-admin-fallthrough.md`. Any
  OTHER future producer of "authenticated with zero profiles" would still escalate. Both
  known producers (tacacs, radius) are now closed, so the fall-through currently has no
  reachable feeder, but nothing structurally prevents a new one.
- **No functional/`.ci` test added.** The decision is internal to `Authenticate` and the
  existing loopback `mockRADIUSServer` harness exercises the same code path the daemon
  takes, through a real UDP Access-Request/Accept exchange.

## RFC Documentation

No RFC-enforcing code added. RFC 2865 defines Filter-Id (5.11) and Class (5.25) on the wire
but does not require either in an Access-Accept, nor define how they map to local
authorization. The new branch is ze policy, so no `// RFC 2865 Section X.Y` comment applies
to it.

## Implementation Summary

### What Was Implemented
- The `CodeAccessAccept` case denies when the resolved profile set is empty:
  `if len(profiles) == 0` → WARN + `aaa.ErrAuthRejected` (`radius/authenticator.go:131-141`).
  A comment records the escalation chain with file:line, and states the R-4 distinction, so
  the guard is not "simplified" into a fallthrough later.
- Doc comment on `Authenticate` updated: the rejected case now covers an Access-Accept that
  resolves to no profiles.
- Doc comment on `mapProfiles` updated: it may legitimately return an empty set, and the
  caller decides what that means.
- `default-profile` YANG description states the consequence of omission.
- Two tests (see TDD Test Plan).

### Bugs Found/Fixed
- The reported escalation (fixed).
- **Found, not fixed:** the `min-elements` inertness (already known, separate spec).

### Documentation Updates
- `docs/guide/radius.md` "Profile mapping": the page claimed an Accept with no profile
  attribute and no default left the user "authenticated with no profiles (and RBAC denies
  privileged actions)". The parenthetical was false and is the reason the escalation went
  unnoticed: RBAC granted admin via the `hasUsers==false` fall-through. Rewritten to state
  the rejection, the WARN text, the invariant, and why it stops the chain rather than
  falling through like an unreachable server.
- `docs/features.md:87` RADIUS row: added the reject-on-no-profile clause. Its source
  anchors name `Authenticate` and `mapProfiles`, both changed here.
- `docs/guide/configuration.md:1608` anchors the YANG container but asserts nothing about
  `default-profile` semantics: no change needed.
- `make ze-doc-test` result recorded in Pre-Commit Verification.

### Deviations from Plan
- A-6 broke at design time, before any code: the YANG half of the TACACS mirror is dropped
  as semantically wrong for an optional leaf-list. Recorded in the Mistake Log. The fix's
  strength is unaffected (the code guard was always the only enforcing part).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Confirm each link in the bug chain | Done | Current Behavior | All read at the producing function |
| Fix the empty/absent profile-set defect | Done | `radius/authenticator.go:131-141` | |
| Distinguish from R-4 | Done | Core Insight; Key Design Decisions; code comment | |
| Do not alter other reject/allow semantics | Done | single new branch | AC-3..AC-6 tests guard this |
| Mirror the TACACS fix's shape | Done | guard + WARN + `ErrAuthRejected` | One forced divergence (A-6), documented |
| Spec recording root cause + ACs + assumptions | Done | this file | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `.../no_attrs_no_default_denies`, `.../empty_attr_values_no_default_denies` | RED→GREEN |
| AC-2 | Done | `TestRadiusAuthenticatedImpliesProfiles` | RED→GREEN |
| AC-3 | Done | `.../no_attrs_with_default_authenticates`, existing `TestRadiusAuthenticateAccept` | passed before and after |
| AC-4 | Done | `.../attrs_present_authenticate`, existing `TestRadiusProfileMapping{,Class}` | passed before and after |
| AC-5 | Done | existing `TestRadiusAuthenticateReject` | passed before and after |
| AC-6 | Done | existing `TestRadiusAuthenticateInfraError` | passed before and after |
| AC-7 | Done | `ze-radius-conf.yang:92-97` | description only; no `min-elements` per A-6 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRadiusAuthenticateProfileResolutionShapes` | Pass | `radius/authenticator_test.go` | 5 subtests |
| `TestRadiusAuthenticatedImpliesProfiles` | Pass | `radius/authenticator_test.go` | 4 shapes |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/radius/authenticator.go` | Modified | guard + comments |
| `internal/component/radius/yang/ze-radius-conf.yang` | Modified | description only |
| `internal/component/radius/authenticator_test.go` | Modified | two tests added |

### Audit Summary
- **Total items:** 7 ACs + 2 tests + 3 files
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** A-6 broke at design time; see Deviations

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| An Access-Accept resolving to no profiles no longer authenticates | Unit test, RED→GREEN | RED: `INFO RADIUS admin auth accepted username=alice profiles=[]` then `Error: An error is expected but got nil.` GREEN: `WARN RADIUS admin auth rejected: no profiles resolved username=alice`, all 5 subtests PASS |
| The configured-default path (AC-6/AC-3) does not regress | Unit test | `no_attrs_with_default_authenticates` PASS; existing `TestRadiusAuthenticateAccept` PASS unchanged |
| R-4's backend-error fallthrough is untouched | Unit test | existing `TestRadiusAuthenticateInfraError` PASS: error is not `ErrAuthRejected` |
| No other reject/allow semantics changed | Package suite | full `radius` package suite PASS |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | (pending) | `/ze-review` not run in this session | - | Thomas to run at closure, or a follow-up session |

### Fixes applied
- Sibling call-site audit performed inline (per `ai/rules/before-writing-code.md` item 9):
  `mapProfiles` has one caller (A-4); the three non-test producers of `Authenticated: true`
  were read (A-5). `tacacs/authenticator.go:116` is fixed by the sibling spec.
  `authz/auth.go:59` (local) is safe: profiles come from a config assignment keyed by
  username, and a local user existing implies `hasUsers == true`, so the admin fall-through
  is not reachable from it.

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
| `internal/component/radius/authenticator.go` | Yes | modified in place; `grep -n "len(profiles) == 0"` hits `:131` |
| `internal/component/radius/yang/ze-radius-conf.yang` | Yes | `grep -n "denied"` hits the `default-profile` description |
| `internal/component/radius/authenticator_test.go` | Yes | tests run and pass |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | empty resolved set denies with a WARN | GREEN log: `WARN RADIUS admin auth rejected: no profiles resolved username=alice`; both deny subtests PASS |
| AC-2 | authenticated implies profiles | `TestRadiusAuthenticatedImpliesProfiles` PASS |
| AC-3 | configured default unchanged | `no_attrs_with_default_authenticates` PASS; `TestRadiusAuthenticateAccept` PASS |
| AC-4 | reply attrs unchanged | `attrs_present_authenticate`, `TestRadiusProfileMapping`, `TestRadiusProfileMappingClass` PASS |
| AC-5 | Access-Reject unchanged | `TestRadiusAuthenticateReject` PASS |
| AC-6 | R-4 fallthrough unchanged | `TestRadiusAuthenticateInfraError` PASS |
| AC-7 | YANG description states the consequence | `ze-radius-conf.yang:92-97` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Access-Accept → `Authenticate` | none (unit harness `mockRADIUSServer` drives a real RADIUS exchange over a loopback UDP socket) | Yes — the tests exercise the client through `Authenticate`, not `mapProfiles` directly |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `config.go:103` unconditional assign; `tree.go:186-190`; YANG `:90-95` has no `min-elements`/`mandatory`; RED test showed success with `profiles=[]` |
| A-2 | confirmed | `login_profiles.go:46` early return; `authz.go:385-390` |
| A-3 | confirmed | `mapProfiles:157` already had a `len(profiles)==0` path; RFC 2865 does not require Filter-Id |
| A-4 | confirmed | grep `mapProfiles` → sole call at `authenticator.go:131` |
| A-5 | confirmed | grep `Authenticated: *true` → 3 non-test hits, each read |
| A-6 | **broken** | `default-profile` is optional by design; `min-elements 1` would contradict it. Dropped. See Mistake Log. |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/radius.md` "neither present -> rejected, WARN, chain stops" | `radius/authenticator.go:131-141` (the guard, the WARN string, `aaa.ErrAuthRejected`) | Yes — text written from the code, and `TestRadiusAuthenticateProfileResolutionShapes` asserts each claim |
| `docs/guide/radius.md` "default-profile applies when no attribute" | `radius/authenticator.go:157-159` (`mapProfiles` fallback) unchanged | Yes — `no_attrs_with_default_authenticates` PASS |
| `docs/features.md:87` "an Accept resolving to no profile rejected rather than authorized" | same guard | Yes |
| `docs/guide/configuration.md:1608` (no change) | anchors `system.authentication.radius` container shape only; the YANG container shape is unchanged (description text only) | Yes — no semantic claim to update |
| `make ze-doc-test` | doc gate over the changed docs | Yes — exit 0 |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
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
- [ ] **Commit B:** `git rm plan/spec-fixit-radius-empty-profile-mapping.md`
</content>
</invoke>
