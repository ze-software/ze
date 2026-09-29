# Spec: ike-eap-rfc-defects

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two defects in Ze's IKEv2 NAT traversal and EAP-TLS, found by the strict re-read
of `spec-rfc-requirement-quote-hand-backfill`. A peer behind a
NAT that rewrote its IKE source port receives ESP on the wrong port, and an
EAP-TLS 1.2 full handshake carries no session_id. Both read at the producer in
HEAD on 2026-09-27.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 3948 Section 2.1 (RFC3948-2.1-2) | "the Source Port and Destination Port MUST be the same as that used by IKE traffic," | `internal/component/ike/engine/child.go::installChildSA` (port defaulting) and the Child SA build that sets `udpLocalPort`/`udpRemotePort` only when MOBIKE is enabled | Without MOBIKE both ports default to 4500, so a NAT-translated peer whose IKE arrives from port Y receives ESP on 4500 | wrong; lead confirmed by reading |
| D2 | RFC 5216 Section 2.1.1 (RFC5216-2.1.1-4) | "If the peer's sessionId is null or unrecognized by the server, the server MUST choose the sessionId to establish a new session." | `internal/core/eap/eap_tls.go::newTLSMethod` over Go `crypto/tls` | crypto/tls sets the ServerHello session_id only when resuming, echoing the client, so a TLS 1.2 full handshake sends an empty session_id and the server chooses no sessionId | wrong since 2026-09-27; journal `invariant-enforced-by-an-absent-call-site.md` |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1: a Child SA whose IKE SA's remote port is not 4500 installs XFRM encap with that port. D2: per the owner's ruling |
| Both polarities | D1: a peer on 4500 keeps 4500 |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Real entry point | D1: the installed XFRM state read back on Linux (`ai/rules/platform-linux.md`) |
| Interop | D1: strongSwan behind a NAT that maps 4500 to another port |

## Owner decisions

| Row | Question |
|-----|----------|
| D2 | Fix the crypto/tls limit (a server-chosen session_id on full handshakes), or rule that ticket-based semantics meet the obligation and record it as such |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3948.md`, `rfc/short/rfc5216.md`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/child.go` - Child SA build and `installChildSA`
- [ ] `internal/core/eap/eap_tls.go` - `newTLSMethod`

**Behavior to change:** D1; D2 per the owner.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- an IKE SA with NAT detected; an EAP-TLS authentication

### Transformation Path
1. IKE SA remote endpoint to Child SA ports
2. XFRM state install with UDP encapsulation

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IKE engine ↔ kernel XFRM | netlink | No |

### Integration Points
- `sa.remoteUDPAddr`, the MOBIKE port path

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH from a NATed port | → | Child SA encap ports | [to fill in design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | peer's IKE arrives from port Y behind a NAT | ESP encapsulated to port Y |
| AC-2 | D2 | as the owner rules |
| AC-3 | the `wrong` verdicts of RFC3948-2.1-2 and RFC5216-2.1.1-4, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` (parent P-3, 2026-09-28) |
| AC-4 | D2 area, RFC5216-2.1.2-1 (weak), RFC 5216 Section 2.1.2: "If the EAP server is resuming a previously established session, then it MUST include only a TLS change_cipher_spec message and a TLS finished handshake message after the server_hello message." crypto/tls renews the session ticket on a TLS 1.2 resumption, so a NewSessionTicket follows server_hello | owner ruling 2026-09-29: ze's authenticator refuses TLS 1.2 resumption (a TLS 1.2 session is never redeemed; TLS 1.3 tickets stay, per RFC 9190). The untagged failing test `TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished` (`internal/core/eap/rfc5216_resumption_defect_test.go`, uncommitted) is replaced by a tagged proof of the fix, and the verdict reaches `enforced` by an agent that did not write that test. The fix removes the TLS 1.2 resumed conversation that `TestRFC5216ResumedFlightsCarryOnlyChangeCipherSpecAndFinished` drives (RFC5216-2.1.1-10, 2.1.1-5), so those tags are re-proven or re-judged in the same work. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` (P-3) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| encap ports from the IKE SA | `internal/component/ike/engine/` | D1 | [to fill in design] |

## Files to Modify

- the two producers and their tests

## Implementation Steps

1. Owner ruling on D2; 2. failing tests; 3. fixes; 4. discrimination and verdicts.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
