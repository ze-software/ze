# Spec: bfd-5-authentication

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-bfd-4-operator-ux |
| Phase | 1/1 |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec
2. `.claude/rules/planning.md`
3. `plan/learned/555-bfd-skeleton.md` -- `packet/auth.go` current state (Type+Len header only)
4. `rfc/short/rfc5880.md` §6.7 (authentication) and §6.8.1/§6.8.2 (state vars)
5. Source files: `internal/plugins/bfd/packet/auth.go`, `internal/plugins/bfd/packet/control.go`, `internal/plugins/bfd/session/machine.go`, `internal/plugins/bfd/schema/ze-bfd-conf.yang`

## Task

Stage 5 implements RFC 5880 §6.7 authentication verification for BFD. The codec already parses the Type+Len header of the Auth section (see `packet/auth.go`), but every authenticated packet is rejected with `ErrAuthMismatch` because no verifier exists. Stage 5 adds:

1. **Auth type parse** -- Simple Password (1), Keyed MD5 (2), Meticulous Keyed MD5 (3), Keyed SHA1 (4), Meticulous Keyed SHA1 (5). Priority: SHA1 (keyed + meticulous). MD5 is implemented for completeness because many operators use it; Simple Password is rejected on receive AND send with a loud log (RFC 5880 §6.7.2 warns it is "provided only for backwards compatibility" and must never be used on links that can be observed).
2. **Key store** -- operator provides a key ID and a secret in the YANG config; multiple keys allowed for rolling.
3. **Sequence number persistence** -- RFC 5880 §6.7.3 requires the sequence number not to go backwards across a restart; Stage 5 persists the last-sent sequence number to a file under `$XDG_STATE_HOME/ze/bfd/<key>.seq` (or a config-specified directory).
4. **Verifier on receive** -- Controls arriving with an Auth section are fed through the verifier; mismatch drops the packet and increments `ze_bfd_auth_failures_total`.
5. **Signer on send** -- every Control built by `session.Machine.Build` or `BuildFinal` on an authenticated session carries the correct digest before `packet.Control.WriteTo` serialises it.
6. **YANG surface** -- profile-level or session-level `auth { type sha1; key-id N; secret "..."; meticulous true }`.

**Explicitly out of Stage 5 scope:**

- Key rotation UX beyond "list of keys". Automatic rotation based on expiry dates is deferred to a follow-up if an operator requests it.
- HMAC-SHA256 (not in RFC 5880). If a future RFC extends the list, that is a new spec.

→ Constraint: auth secrets must never appear in logs, even at debug. `ze config show` must redact secrets.
→ Constraint: sequence-number persistence writes are best-effort; a write failure must not stall the express loop. Use a small dedicated goroutine with a coalescing channel.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/bfd.md`
- [ ] `.claude/rules/go-standards.md` -- logging and env vars
- [ ] `.claude/rules/config-design.md` -- how secrets appear in YANG

### RFC Summaries

- [ ] `rfc/short/rfc5880.md` -- §6.7 full authentication discussion
  → Constraint: Keyed-SHA1 uses bfd.XmitAuthSeq on send and Received Authentication Sequence Number on receive (§6.7.4)
  → Constraint: Meticulous variants (§6.7.3) advance the sequence number on every packet; non-meticulous advances only when required

### Source files

- [ ] `internal/plugins/bfd/packet/auth.go` -- current parser (header only)
- [ ] `internal/plugins/bfd/packet/control.go` -- WriteTo needs to include the auth section
- [ ] `internal/plugins/bfd/session/machine.go` -- bfd.XmitAuthSeq, bfd.AuthType state variables
- [ ] `internal/plugins/bfd/schema/ze-bfd-conf.yang` -- add auth block

## Current Behavior (MANDATORY)

**Source files read:** (filled during /implement)

**Behavior to preserve:**

- Unauthenticated sessions work exactly as today (Stage 5 adds an optional code path).
- Existing `.ci` tests pass unmodified.
- Fuzz tests for packet parsing still pass.

**Behavior to change:**

- `session.Machine.Receive` returns `ErrAuthMismatch` today on any authenticated packet; after Stage 5, authenticated packets go through the verifier.
- `packet.Control.WriteTo` omits the auth section today; after Stage 5, it writes the auth section when `Control.Auth == true`.
- New file-based sequence-number persistence with a background writer.

## Data Flow

### Entry Point

- Packet arrives at `transport.UDP` → `engine.Loop.handleInbound` → `packet.ParseControl` → `session.Machine.Receive`.
- On send: `engine.Loop.sendLocked` → `session.Machine.Build` → `packet.Control.WriteTo` → transport send.

### Transformation Path

1. `ParseControl` extracts the auth section as bytes if present.
2. `Machine.Receive` feeds auth bytes into the verifier tied to `m.vars.AuthType`.
3. Verifier returns OK → packet processed; FAIL → packet dropped, counter incremented.
4. `Machine.Build` sets bfd.XmitAuthSeq, sets `Control.AuthPayload`; `WriteTo` includes it.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ Session | AuthConfig passed via `SessionRequest` | [ ] |
| Packet ↔ Session | raw auth bytes + AuthType enum | [ ] |
| Session ↔ Persistence | seq number writer goroutine channel | [ ] |

### Integration Points

- `api.SessionRequest` grows an `Auth *AuthConfig` field (API surface extension, reviewed carefully)
- `packet.Control` grows an `AuthPayload []byte` field or similar
- `session.Machine` grows a `verifier` + `signer` pair; `m.vars.AuthType` drives the dispatch
- New `internal/plugins/bfd/auth/` subpackage with signer/verifier per type

### Architectural Verification

- [ ] No bypassed layers: codec writes bytes; session handles digest/verify; persistence is side-effect only
- [ ] No new exposed secret strings in logs
- [ ] Zero-copy preserved: signer writes into the same pool buffer

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| YANG `bfd { profile { auth { type sha1; key-id 1; secret ... } } }` + two ze processes | → | `Machine.Receive` verifies digest, `Build` signs | `test/plugin/bfd-auth-sha1.ci` |
| Bad digest on receive | → | Packet dropped, `ze_bfd_auth_failures_total` increments | `test/plugin/bfd-auth-mismatch.ci` |
| Meticulous key rolls over session reset | → | Sequence number persisted; next process start resumes | `test/plugin/bfd-auth-meticulous-persist.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Both ends configured with SHA1 key 1 secret "K" | Session reaches Up |
| AC-2 | Local SHA1 key 1 secret "K", remote SHA1 key 1 secret "L" | Session stays Down, counter increments |
| AC-3 | Local authenticated, remote unauthenticated | Mismatch; session stays Down |
| AC-4 | Local unauthenticated, remote authenticated | Mismatch; session stays Down |
| AC-5 | Meticulous SHA1: packet with sequence number <= last-seen | Dropped |
| AC-6 | Meticulous SHA1: packet with sequence number > last-seen | Accepted; last-seen advanced |
| AC-7 | Sequence number file exists from previous run | Loaded at startup; next tx uses value+1 |
| AC-8 | Sequence number file write fails | Best-effort: log once, continue; no express-loop stall |
| AC-9 | MD5 variants | Same behaviour as SHA1 (AC-1..AC-6) |
| AC-10 | Simple Password | REJECTED with loud warning both on receive and when a config tries to use it on send; error at config parse time |
| AC-11 | `ze config show` for a session with a secret | Secret field redacted (`***`) in output |
| AC-12 | `plan/deferrals.md` row `spec-bfd-5-authentication` | Marked done pointing to learned summary |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSignerVerifier_SHA1_RoundTrip` | `internal/plugins/bfd/auth/sha1_test.go` | Sign then verify with same key returns OK | |
| `TestVerifier_SHA1_Mismatch` | `internal/plugins/bfd/auth/sha1_test.go` | Wrong secret returns error | |
| `TestVerifier_MeticulousReplay` | `internal/plugins/bfd/auth/meticulous_test.go` | Replay of a previous sequence number rejected | |
| `TestSignerVerifier_MD5_RoundTrip` | `internal/plugins/bfd/auth/md5_test.go` | MD5 variant works | |
| `TestSimplePasswordRejectedOnParse` | `internal/plugins/bfd/config_test.go` | Config with `type simple-password` rejected | |
| `TestSeqPersistWriteLoad` | `internal/plugins/bfd/auth/persist_test.go` | Sequence written by process A loaded by process B | |
| `TestSeqPersistWriteFailure` | `internal/plugins/bfd/auth/persist_test.go` | Write to read-only dir does not stall; logs once; continues | |
| `TestMachineSendAuth` | `internal/plugins/bfd/session/auth_test.go` | Build() on auth session produces a Control with non-nil AuthPayload; WriteTo writes it | |
| `TestMachineReceiveAuthMismatchCount` | `internal/plugins/bfd/session/auth_test.go` | Mismatch packet increments counter via notify | |
| `FuzzAuthDigestParse` | `internal/plugins/bfd/packet/auth_fuzz_test.go` | Fuzz: codec never panics on adversarial auth sections | |
| `TestConfigShowRedactsSecret` | `internal/component/config/show_test.go` (or wherever show lives) | AC-11 | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| key-id | 0-255 | 255 | N/A | 256 |
| auth type | 1-5 (RFC 5880 §6.7) | 5 | 0 | 6 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bfd-auth-sha1` | `test/plugin/bfd-auth-sha1.ci` | Two ze processes, both configured with SHA1 key 1; session reaches Up | |
| `bfd-auth-mismatch` | `test/plugin/bfd-auth-mismatch.ci` | Secrets mismatch; session stays Down; counter increments | |
| `bfd-auth-meticulous-persist` | `test/plugin/bfd-auth-meticulous-persist.ci` | Stop ze, restart, assert sequence continues | |

### Future
- None.

## Files to Modify

- `internal/plugins/bfd/schema/ze-bfd-conf.yang` -- add `auth` block under profile (and optionally under session)
- `internal/plugins/bfd/config.go` -- parse auth block, reject simple-password
- `internal/plugins/bfd/api/events.go` -- add `Auth *AuthConfig` to SessionRequest
- `internal/plugins/bfd/api/auth.go` (new) -- AuthConfig public type
- `internal/plugins/bfd/packet/auth.go` -- extend to read/write the digest block
- `internal/plugins/bfd/packet/control.go` -- WriteTo writes auth section when `Auth == true`
- `internal/plugins/bfd/session/machine.go` -- verifier + signer plumbed into Receive/Build; bfd.XmitAuthSeq state var bumped correctly
- `internal/plugins/bfd/auth/` (new package) -- `signer.go`, `verifier.go`, `sha1.go`, `md5.go`, `meticulous.go`, `persist.go`
- `internal/component/config/show.go` (or equivalent) -- redact secrets
- `internal/plugins/bfd/metrics.go` -- new `ze_bfd_auth_failures_total` counter
- `plan/deferrals.md` -- close Stage 5 row
- Documentation files listed below

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] Yes | `ze-bfd-conf.yang` |
| CLI commands | [ ] No | - |
| Editor autocomplete | [ ] Yes (automatic) | - |
| Functional test | [ ] Yes | three `.ci` tests |
| Metrics | [ ] Yes (extends Stage 4 registry) | `metrics.go` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File |
|---|----------|----------|------|
| 1 | User-facing feature? | [ ] Yes | `docs/features.md` |
| 2 | Config syntax? | [ ] Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI? | [ ] No | - |
| 4 | API/RPC? | [ ] No | - |
| 5 | Plugin? | [ ] Yes | `docs/guide/plugins.md` |
| 6 | User guide? | [ ] Yes | `docs/guide/bfd.md` |
| 7 | Wire format? | [ ] Partial | `docs/architecture/bfd.md` auth section layout |
| 8 | Plugin SDK/protocol? | [ ] No | - |
| 9 | RFC behavior? | [ ] Yes | `rfc/short/rfc5880.md` mark §6.7 done |
| 10 | Test infrastructure? | [ ] No | - |
| 11 | Daemon comparison? | [ ] Yes | `docs/comparison.md` |
| 12 | Internal architecture? | [ ] Yes | `docs/architecture/bfd.md` |
| 13 | Route metadata? | [ ] No | - |

## Files to Create

- `internal/plugins/bfd/api/auth.go`
- `internal/plugins/bfd/auth/signer.go`
- `internal/plugins/bfd/auth/verifier.go`
- `internal/plugins/bfd/auth/sha1.go`
- `internal/plugins/bfd/auth/md5.go`
- `internal/plugins/bfd/auth/meticulous.go`
- `internal/plugins/bfd/auth/persist.go`
- `internal/plugins/bfd/auth/*_test.go`
- `internal/plugins/bfd/session/auth_test.go`
- `internal/plugins/bfd/packet/auth_fuzz_test.go`
- `test/plugin/bfd-auth-sha1.ci`
- `test/plugin/bfd-auth-mismatch.ci`
- `test/plugin/bfd-auth-meticulous-persist.ci`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files tables |
| 3. Implement | Phases below |
| 4. Verify | `make ze-verify` |
| 5. Critical review | - |
| 6. Fix | - |
| 7. Re-verify | - |
| 8. Repeat | - |
| 9. Deliverables | - |
| 10. Security | - |
| 11. Re-verify | - |
| 12. Summary | - |

### Implementation Phases

1. **Phase: auth package skeleton** -- signer/verifier interfaces + SHA1/MD5 pure-function implementations with vectors from RFC 5880 Appendix A (if present).
   - Tests: `TestSignerVerifier_SHA1_RoundTrip`, `TestVerifier_SHA1_Mismatch`, `TestSignerVerifier_MD5_RoundTrip`
2. **Phase: Meticulous sequence handling** -- sliding window / last-seen state.
   - Tests: `TestVerifier_MeticulousReplay`
3. **Phase: Packet codec extension** -- read/write auth section inside `packet.Control.WriteTo` and `ParseControl`.
   - Tests: `FuzzAuthDigestParse`
4. **Phase: Session integration** -- `Machine.Receive` calls verifier; `Machine.Build` calls signer; bfd.XmitAuthSeq advances.
   - Tests: `TestMachineSendAuth`, `TestMachineReceiveAuthMismatchCount`
5. **Phase: Persistence** -- background writer goroutine, read-at-start.
   - Tests: `TestSeqPersistWriteLoad`, `TestSeqPersistWriteFailure`
6. **Phase: YANG + parser** -- add auth block; reject simple-password.
   - Tests: `TestSimplePasswordRejectedOnParse`, `TestConfigShowRedactsSecret`
7. **Phase: Functional tests** -- three `.ci` tests.
8. **Phase: Metrics** -- extend Stage 4 metrics.
9. **Phase: Docs** -- every file from table.
10. **Phase: Close spec**.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Completeness | Every AC implemented |
| Correctness | Digest bytes exactly match RFC 5880 §6.7 diagrams; fuzz targets clean |
| Naming | `Signer`, `Verifier`, `authPersistDir` |
| Data flow | Verifier in session, not in transport |
| Rule: secret handling | No secret ever logged; `fmt.Sprintf` audit on any new log line |
| Rule: goroutine lifecycle | Persist writer is long-lived, exits on engine Stop |

### Deliverables Checklist

| Deliverable | Verification |
|-------------|--------------|
| SHA1/MD5 verifier | `go test ./internal/plugins/bfd/auth/...` |
| Persistence | `TestSeqPersistWriteLoad` passes |
| Secret redaction | grep the config-show output for `"secret"` value in tests |
| Functional tests pass | `bin/ze-test plugin bfd-auth-*` |
| Docs updated | each table entry has a diff |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Secrets in logs | Grep all new code for `log.*secret`, `log.*key`; fail if found |
| Secret in errors | Error messages MUST NOT echo the secret |
| Replay protection | Sequence number strictly monotonic for meticulous variants |
| Key ID enumeration | Rejecting a packet must not reveal which key ID is valid (same error for "unknown key-id" and "wrong digest") |
| Persistence file permissions | 0600; directory 0700 |
| DoS on verify | Verify is O(packet size); bounded |
| Fuzz | `FuzzAuthDigestParse` runs 30 s in CI |

### Failure Routing

| Failure | Route to |
|---------|----------|
| Fuzz panic | Fix parser; add regression test |
| Functional test timeout | Check persist dir perms; investigate |
| Race on XmitAuthSeq | Stay on single-writer invariant; no mutex needed |
| 3 fix attempts fail | STOP |

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

- Simple Password is rejected, not accepted with a warning. Accepting it creates a false sense of security.
- Sequence number persistence uses a small coalescing goroutine: only the last value needs to survive a crash, and the express loop must never block on I/O.

## RFC Documentation

- `// RFC 5880 Section 6.7.4: "The Keyed SHA1 and Meticulous Keyed SHA1 Authentication Sections..."` above signer/verifier for SHA1
- `// RFC 5880 Section 6.7.3: "The auth sequence number advances..."` above meticulous handling
- `// RFC 5880 Section 6.7.2: "[Simple Password] is provided only for backwards compatibility..."` above the rejection code

## Implementation Summary

### What Was Implemented
### Bugs Found/Fixed
### Documentation Updates
### Deviations from Plan

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes (includes `make ze-test` -- lint + all ze tests)
- [ ] Fuzz target clean
- [ ] Feature code integrated
- [ ] Functional tests pass
- [ ] Docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] RFC annotations added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] Simple Password rejected, not accepted
- [ ] Single responsibility
- [ ] Explicit

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests
- [ ] Fuzz test included

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written
- [ ] Summary in commit
