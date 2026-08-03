# Spec: pki-show-private-key

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/pki/show.go` - `certPEM`, `certBundlePEM`, `marshalPrivateKeyPEM`
4. `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` - `show pki certificate` grammar

## Task

Operators need to read a stored private key in PEM (to copy a device key to another
system, back it up, or feed it to external tooling). Today Ze can emit the private
key **only glued to the certificate chain** via `show pki certificate name <name>
bundle pem`; there is no key-only output. Separately, the `show pki certificate`
handler has **no access control**: anyone who can run the show command can extract
the private key.

Two changes:

1. Add a key-only PEM output, e.g. `show pki certificate name <name> private-key
   pem`, emitting just the PKCS#8 private key.
2. Add an access-control / explicit-confirmation gate around private-key exposure
   (both the new key-only form and the existing `bundle pem`), so keys are not
   readable by default with no guard.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/color-system.md` - if output/warnings are styled, use the semantic roles.
  → Constraint: sensitive-material output should carry a clear operator warning.
- [ ] `ai/rules/plugins.md` - pki-cmd owns its command surface.
  → Constraint: the new show verb and any authz check register through pki-cmd, not a central switch.

**Key insights:**
- The private key is already marshalled to PEM (PKCS#8) by `marshalPrivateKeyPEM`; the new command reuses it, it does not add crypto.
- The real gap is a key-only path plus a guard, not new key handling.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/pki/show.go` - `certPEM` emits certificate(s) only, no key (show.go); `certBundlePEM` concatenates the cert chain **plus** the PKCS#8 private key (show.go), erroring if the entry has no private key. `marshalPrivateKeyPEM` produces the `PRIVATE KEY` PEM block. There is no function that emits the key alone.
- [ ] `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` - `show pki certificate name <name>` supports `pem` / `bundle pem` / `fingerprint`; the RPC is a plain `config false` operational command with no privilege/authz guard.
- [ ] `internal/component/pki/store.go` - `ExportPEM` writes the key to a `0600` temp file for IPsec consumption (not a CLI display path).

**Behavior to preserve:**
- `show pki certificate name <name> pem` continues to emit certificate(s) only.
- `bundle pem` continues to emit chain + key (subject to the new guard).
- Key marshalling stays PKCS#8 via `marshalPrivateKeyPEM`.

**Behavior to change:**
- Add `private-key pem` (key-only) output.
- Gate private-key exposure (`bundle pem` and `private-key pem`) behind an access-control / explicit-confirmation mechanism.

## Data Flow (MANDATORY)

### Entry Point
- CLI / operational RPC: `show pki certificate name <name> private-key pem`, defined in `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang`, dispatched into `internal/component/pki/show.go`.

### Transformation Path
1. CLI verb parsed; pki-cmd dispatches to the pki show handler with a "private-key" selector.
2. Access-control gate evaluated (authz role / explicit confirmation) before any key material is serialised.
3. On pass: the stored entry's private key is marshalled via `marshalPrivateKeyPEM` and returned as `{"pem": <key>}`.
4. On fail / no key present: a clear error, no key material in the response.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ pki component | new `private-key` selector in the show RPC | [ ] |
| Authz ↔ pki show | guard consulted before serialising the key | [ ] |
| pki store ↔ output | reuse `marshalPrivateKeyPEM` (PKCS#8) | [ ] |

### Integration Points
- `internal/component/pki/show.go` - add `privateKeyPEM` handler; wrap key paths in the guard.
- `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` - add the `private-key pem` grammar.
- `internal/component/authz/` - the access-control mechanism (mechanism chosen during design: role check vs explicit `confirm`).

### Architectural Verification
- [ ] No bypassed layers (dispatch via pki-cmd, not a hardcoded case in a core package)
- [ ] No unintended coupling (authz consulted through its interface)
- [ ] No duplicated functionality (reuse `marshalPrivateKeyPEM`)
- [ ] Registration over hardcoding — the new verb and the authz check register via pki-cmd's command surface, per `ai/rules/plugins.md`.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze has an authz/role mechanism a show handler can consult | `internal/component/authz/` component exists | guard becomes an explicit-confirm prompt instead | read authz API during audit | unvalidated |
| A-2 | Reusing `marshalPrivateKeyPEM` yields the exact key-only bytes wanted | show.go already uses it | different encoding needed | unit test round-trips the key | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Key leaks into logs / audit trail | key bytes appear in command history/log | ensure the response is not logged; redact in audit |
| R-2 | Guard is trivially bypassable | review finds no enforcement on `bundle pem` | apply the same guard to both key-emitting forms |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show pki certificate name X private-key pem` | → | `privateKeyPEM` handler in show.go behind authz guard | `test/plugin/pki-show-private-key.ci` |
| unauthorised caller | → | guard rejects, no key returned | `test/plugin/pki-show-private-key.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | authorised `private-key pem` on a cert with a key | returns only the PKCS#8 `PRIVATE KEY` PEM block, no certificate |
| AC-2 | cert has no private key | clear error, no key material |
| AC-3 | unauthorised / unconfirmed caller | request rejected; no key in response |
| AC-4 | existing `bundle pem` | now also subject to the guard; output otherwise unchanged |
| AC-5 | `pem` (cert-only) | unchanged, no guard needed |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | authorised operator exports a device key | CLI → guard pass → `marshalPrivateKeyPEM` → key PEM | `test/plugin/pki-show-private-key.ci` |
| 2 | unauthorised user tries to read a key | CLI → guard fail → error | `test/plugin/pki-show-private-key.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrivateKeyPEMKeyOnly` | `internal/component/pki/show_test.go` | output is key-only, valid PKCS#8 PEM | |
| `TestPrivateKeyPEMNoKeyError` | `internal/component/pki/show_test.go` | error when no key present | |
| `TestPrivateKeyGuardRejects` | `internal/component/pki/show_test.go` | guard blocks unauthorised access | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pki-show-private-key` | `test/plugin/pki-show-private-key.ci` | key-only export works for authorised, blocked for unauthorised | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - not a wire-protocol feature | - | - | operational CLI only | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/pki/show.go` - add `privateKeyPEM` handler; wrap key paths in the guard
- `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang` - add `private-key pem` grammar
- `internal/component/authz/` - access-control hook consulted by the show handler (mechanism chosen in design)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPC form) | [ ] yes | `ze-pki-cmd.yang` `private-key pem` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli.md` |
| Pipe completeness | [ ] yes | route key output through `ApplyPipes` per `ai/rules/cli.md` |
| Functional test for new RPC | [ ] yes | `test/plugin/pki-show-private-key.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] yes | `docs/architecture/api/commands.md` |

## Files to Create
- `test/plugin/pki-show-private-key.ci` - functional test
- (unit tests extend `internal/component/pki/show_test.go`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add `private-key pem` YANG grammar + a stub handler returning "not implemented"; failing `test/plugin/pki-show-private-key.ci`.
2. **Phase: Key-only output** — implement `privateKeyPEM` reusing `marshalPrivateKeyPEM`.
   - Tests: `TestPrivateKeyPEMKeyOnly`, `TestPrivateKeyPEMNoKeyError`
3. **Phase: Access guard** — gate both `private-key pem` and `bundle pem`.
   - Tests: `TestPrivateKeyGuardRejects`
4. **Functional test**
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | key-only output has no certificate bytes; guard covers both key forms |
| Security | key never logged; guard not bypassable |
| CLI grammar | action before identifier |
| Registration over hardcoding | verb + guard registered via pki-cmd |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| key-only PEM | `go test ./internal/component/pki -run PrivateKeyPEM` |
| guard | `test/plugin/pki-show-private-key.ci` unauthorised case fails closed |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Error leakage | error paths never include key bytes |
| Audit | key export is auditable; response body excluded from logs |
| Access control | guard fails closed on any ambiguity |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A - none)
- [ ] Functional tests for end-to-end behavior
- [ ] Security review (key handling)
