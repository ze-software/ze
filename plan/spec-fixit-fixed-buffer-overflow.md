# Spec: fixit-fixed-buffer-overflow -- bounds-check fixed-size wire buffers

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/buffer-first.md`, `ai/rules/fail-closed-guards.md`
4. `internal/core/bgp/wire/writer.go` - the `CheckedBufWriter` contract this spec adopts

## Task

Multiple verified sites build a wire message into a FIXED-SIZE buffer and then slice it
by the returned length, with no capacity guard. The panic does NOT happen at the `buf[:n]`
re-slice: the encoders index directly into the buffer, so an oversized message panics with a
slice-bounds error INSIDE the encoder's `WriteTo` (before `n` is ever returned) -- e.g.
`internal/component/l2tp/ppp/dhcpv6.go:255` (`buf[off] = cfg.Type`) and `:269+`
(`binary.BigEndian.PutUint16(buf[off:], ...)`), and the IKE payload writers reached through
`internal/component/ike/wire/message.go:36` (`m.Payloads[i].Payload.WriteTo(buf, off)`, which
lands in writers such as `payload_notify.go:54` `buf[off] = p.ProtocolID`). Consequence: a
post-hoc `n > cap(buf)` check at the caller is DEAD CODE -- control never reaches it on
overflow. Only sizing the buffer to the message length up front (a `Len()`-first allocation) or
a `CheckedWriteTo` that bounds every index before writing actually prevents the panic; the
post-hoc-check option is a non-fix.

| Site | Shape | Verified |
|------|-------|----------|
| `internal/component/ike/engine/responder.go:317-319` (`sendSAInitNotify`; same shape at `dpd.go:97`) | `buf := make([]byte, 512)`, `n := msg.WriteTo(buf, 0)`, `tr.Send(buf[:n], remote)` | read 2026-07-16, lines confirmed |
| `internal/component/l2tp/ppp/ipv6_service.go:145-146` (also `:181`, `:213`, `:238`, `:250`) | `var buf [512]byte`, `n := BuildDHCPv6Reply(buf[:], ...)`, then `buf[:n]` | read 2026-07-16, lines confirmed |
| `internal/component/ike/engine/responder.go:243-245` and `internal/component/ike/engine/initiator.go:83-85` (full IKE_SA_INIT build, response and request) | `buf := make([]byte, 4096)`, `n := msg.WriteTo(buf, 0)`, `return buf[:n]` | read 2026-07-16, lines confirmed |

**The two 512-byte notify/reply sites are not reachable today.** IKE emits only tiny
NO_PROPOSAL_CHOSEN / INVALID_KE notify payloads (n << 512); L2TP emits only small DHCPv6-PD
replies. This is a latent class, not a live bug -- unlike the MRT record overflow (remotely
triggerable, already fixed), which is what surfaced this class.

**But the full IKE_SA_INIT builders are closer to live.** `responder.go:243-245` and
`initiator.go:83-85` assemble a complete IKE_SA_INIT message (including a KE payload whose size
tracks the negotiated DH group, remotely influenced) into a fixed `make([]byte, 4096)` buffer
with the identical `WriteTo`/`buf[:n]` shape. Their 4096 buffer is larger, but the size is
remotely influenced rather than fixed by ze -- so they are the higher-priority members of this
class and belong in the mandatory sweep-first phase, not an afterthought.

**Durable class fix** (decide in design, do not pick here): either size the buffer to
`msg.Len()`, or adopt the bounds-checking contract already defined for BGP at
`internal/core/bgp/wire/writer.go` -- `CheckedBufWriter` extends `BufWriter` with
`CheckedWriteTo(buf, off) (int, error)` plus `Len()`. Copy-truncate is NOT an option
(`ai/rules/no-workarounds-for-missing-behavior.md`): a silently truncated IKE notify or
DHCPv6 reply is a malformed packet on the wire, which is worse than a panic.

**Provenance:** deferred ad-hoc 2026-07-13 while fixing the MRT record overflow. Two
rows in `plan/deferrals.md` (the "fixed-buffer-overflow class" rows, one per site) name
this spec as their destination. Both rows are ONE subtask -- adopt a bounds-checking
contract -- with two call sites, so they share one file.

**Open question for design:** whether other fixed-buffer sites exist. This spec was
created from two known rows; a sweep for the `make([]byte, N)` / `var buf [N]byte`
followed by `WriteTo` / `buf[:n]` shape is the first design step.

→ AUTONOMOUS DEFAULT (2026-07-17): RESOLVED. Scope stays the two encoder families
already in the site table -- IKE `Message.WriteTo` (fixed 512/4096) and L2TP
`BuildDHCPv6Reply`/`BuildDHCPv6StatusReply` (`[512]byte`). Do NOT widen this spec.
Rationale: the mandatory sweep (evidence table below) was run repo-wide and every
OTHER fixed-buffer + `WriteTo`/`Build` + `buf[:n]` site is ALREADY guarded, so the two
tabled families are the ONLY unguarded members of the class. One latent sibling
(`BuildRA`) is recorded as a noted follow-up, deliberately NOT pulled into scope
(`ai/rules/no-partial-completion.md`: no silent scope widening). Thomas: override if wrong.

**Sweep evidence** (grep, non-test feature code, 2026-07-17: `make([]byte, N)` / `var buf [N]byte` declarations cross-referenced with `.WriteTo(buf` / `Build…(buf` encoder calls that re-slice `buf[:n]` onto the wire):

| Site / family | Buffer | Encoder & guard state | Disposition |
|---|---|---|---|
| IKE `msg.WriteTo` -- `responder.go:244,318`, `initiator.go:84`, `dpd.go:98` | `make([]byte, 512\|4096)` | `ike/wire.Message.WriteTo` has NO `Len()`/`EncodedLen()` (grep `) Len()` in `ike/wire` returns nothing) and indexes `buf` directly (`message.go:36` -> `payload_notify.go:54` `buf[off] = p.ProtocolID`) | UNGUARDED -- IN SCOPE |
| L2TP DHCPv6 -- `ipv6_service.go:146,182,214,239,251` | `var buf [512]byte` | `BuildDHCPv6Reply`/`BuildDHCPv6StatusReply` return `int`, no capacity check, index `buf[off]` directly (`dhcpv6.go:255` `buf[off]=cfg.Type`, `:269` `PutUint16(buf[off:],…)`) | UNGUARDED -- IN SCOPE |
| L2TP `BuildRA` -- `ra_linux.go:111` | `var buf [256]byte` | `ra.go:37` returns `int`, no capacity check, indexes `buf[off]` directly, has a VARIABLE-length RDNSS option; the sole caller passes a fixed no-RDNSS `RAConfig` so it cannot overflow today | GUARD-BY-CONVENTION -- NOTED FOLLOW-UP (same unguarded-builder family, same package); NOT widened into this spec |
| ISIS -- `hello.go:197,214`, `origination.go:499`, `snp.go:480,488`, `auth_sign.go:108-133`, `lsdb/encode.go:122,172` | `make([]byte, EncodedLen())` | buffer sized by the encoder's own `EncodedLen()` before `WriteTo` | ALREADY SAFE -- this IS the `Len()`-first fix the spec recommends |
| L2TP PPPoE -- `server.go:70,97,110,121,171,250` | `var buf [EthMaxLen]byte` | `BuildPAD*` build via `Builder`; `AddTag*` set `truncated` on overflow, `Finish()` returns `nil` (`discovery.go:291-293`), callers check `frame == nil` ("frame too large") | ALREADY SAFE -- fail-closed nil-return |
| BFD -- `loop.go:219` | pooled `pb.Data()`, `PoolBufSize = 64` (`bfd/packet/pool.go:26`) | `packet.Control.WriteTo` + `Sign`; pool documented as sized above the max control-packet+auth; fixed-format PDU, not variable-length | ALREADY SAFE -- documented fixed cap |

## Required Reading

### Architecture Docs
- [ ] `ai/rules/buffer-first.md` - the project's buffer-first encoding contract
  → Constraint: encoders write into a caller-supplied buffer and return the length; the caller owns sizing, so the caller is where the guard belongs
- [ ] `ai/rules/fail-closed-guards.md` - why a silent truncation is the wrong fallback
  → Constraint: a guard must deny or say something; copy-truncate neither denies nor speaks
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - forbids truncating to dodge the bound
  → Constraint: implement the missing bound at the source, never weaken the output

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` - IKEv2, governs the IKE_SA_INIT notify this spec bounds
  → Constraint: a notify response must be a well-formed IKE message; truncation is not a permitted degradation

**Key insights:**
- The BGP side already solved this: `CheckedBufWriter` (`internal/core/bgp/wire/writer.go`) pairs `Len()` with `CheckedWriteTo`. The IKE and L2TP encoders never adopted it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/responder.go` - `:317-319` builds a 512-byte buffer, calls `msg.WriteTo(buf, 0)`, sends `buf[:n]`; no capacity check. ALSO `:243-245` builds a full IKE_SA_INIT response into `make([]byte, 4096)` with the same `WriteTo`/`buf[:n]` shape (same class, remotely-influenced KE size)
- [ ] `internal/component/ike/engine/initiator.go` - `:83-85` builds a full IKE_SA_INIT request into `make([]byte, 4096)`, `n := msg.WriteTo(buf, 0)`, `return buf[:n]`; no capacity check (same class as responder)
- [ ] `internal/component/ike/wire/message.go` - `:36` `Message.WriteTo` calls `m.Payloads[i].Payload.WriteTo(buf, off)`, which indexes `buf` directly in each payload writer (e.g. `payload_notify.go:54`); this is where the slice-bounds panic actually fires, and `Message` has no `Len()` method
- [ ] `internal/component/l2tp/ppp/ipv6_service.go` - `:145-146` declares `var buf [512]byte`, calls `BuildDHCPv6Reply(buf[:], ...)`, uses `buf[:n]`; no capacity check
- [ ] `internal/core/bgp/wire/writer.go` - defines `BufWriter.WriteTo(buf, off) int` and `CheckedBufWriter` adding `CheckedWriteTo` + `Len()`

**Behavior to preserve:**
- Every currently emitted IKE notify and DHCPv6-PD reply must still go on the wire byte-for-byte identically. This spec adds a bound, it does not change any encoding.
- `sendSAInitNotify` stays best-effort: a send failure logs at Debug and does not propagate (`responder.go:319-321`).

**Behavior to change:**
- An oversized message must become an explicit, logged failure instead of a slice-bounds panic. It must NOT become a truncated packet.

## Data Flow (MANDATORY)

### Entry Point
- IKE: an inbound IKE_SA_INIT request that ze rejects (unacceptable proposal, or a KE payload for the wrong DH group) reaches `PeerSession.handleSAInitRequest`, which calls `sendSAInitNotify` with a notify type and data.
- L2TP: an inbound DHCPv6 Solicit/Request from a PPP subscriber reaches the IPv6 service, which builds a Reply/Advertise via `BuildDHCPv6Reply`.

### Transformation Path
1. Protocol handler decides a response is needed and assembles a `wire.Message` (IKE) or a `DHCPv6ReplyConfig` (L2TP).
2. Caller allocates a fixed 512-byte buffer (`make([]byte, 512)` or `var buf [512]byte`).
3. Encoder writes into that buffer and returns `n`, the number of bytes written.
4. Caller re-slices to `buf[:n]` and hands the slice to the transport.
5. Transport sends the datagram.

Stage 3 is where the bound is missing AND where the panic fires. The encoder is trusted to fit
and indexes directly into the buffer, so on overflow it panics INSIDE `WriteTo` (stage 3) before
`n` is returned -- stage 4 (`buf[:n]`) is never reached. A post-hoc `n > cap(buf)` guard at the
caller (stage 4) is therefore DEAD CODE and NOT a fix; only a `Len()`-first buffer size or a
`CheckedWriteTo` contract that bounds each index before writing prevents the panic.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Protocol handler ↔ encoder | caller-supplied buffer + returned length (buffer-first contract) | [ ] |
| Encoder ↔ transport | `buf[:n]` byte slice handed to `tr.Send` / the PPP writer | [ ] |
| IKE/L2TP ↔ BGP wire contract | `CheckedBufWriter` is defined in `internal/core/bgp/wire/`; reusing it across components needs a tier check (`ai/rules/module-tiers.md`) | [ ] |

### Integration Points
- `internal/core/bgp/wire.CheckedBufWriter` - the existing contract a class fix would adopt; note its package location is BGP-scoped and may need to move to a neutral core package.
- `internal/component/ike/wire.Message.WriteTo` - IKE's encoder entry point; needs a `Len()` to adopt the checked contract.
- `internal/component/l2tp/ppp.BuildDHCPv6Reply` - L2TP's encoder entry point; same.

### Architectural Verification
- [ ] No bypassed layers (the guard lands at the caller that owns the buffer)
- [ ] No unintended coupling (IKE/L2TP must not import BGP-specific packages; move the contract if needed)
- [ ] No duplicated functionality (adopt the existing checked contract, do not invent a second one)
- [ ] Zero-copy preserved where applicable (the fix must not add a per-packet allocation on the hot path)
- [ ] Registration over hardcoding — no new per-feature field, switch case, or factory in a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No live payload can exceed 512 bytes at either site today | Deferral rows 2026-07-13; both say "not reachable with today's payloads" | The class is a live bug, not latent, and the spec becomes urgent | Compute worst-case `Len()` for every notify type and DHCPv6 reply shape | unvalidated |
| A-2 | These two are the only sites of this shape | ALREADY FALSE: `responder.go:243-245` and `initiator.go:83-85` build a full IKE_SA_INIT into `make([]byte, 4096)` with the same `WriteTo`/`buf[:n]` shape (read 2026-07-16) | The fix is incomplete and the class survives | Repo-wide sweep for fixed-size buffer + `WriteTo`/`buf[:n]` | BROKEN -- at least 4 sites known; a full sweep is mandatory before any fix. → SWEEP DONE 2026-07-17: the two tabled families (IKE `Message.WriteTo`, L2TP `BuildDHCPv6*`) are the ONLY unguarded members; every other `WriteTo`/`Build` sibling is already guarded (ISIS `EncodedLen()`-first, PPPoE nil-on-overflow, BFD `PoolBufSize` cap). One latent follow-up: `BuildRA`. Evidence table in the Task "Open question" resolution. |
| A-3 | `CheckedBufWriter` can be reused outside BGP without a tier violation | `internal/core/bgp/wire/writer.go` is under `internal/core/` | The contract must be moved or duplicated | `make ze-tier-check` after a trial import | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Adding `Len()` to the IKE/L2TP encoders is wide (every payload type must report its length). CONFIRMED wide, not hypothetical: `wire.Message` (`internal/component/ike/wire/message.go`, the type also spelled `wire.Message` at the call sites) has NO `Len()` method -- a grep for `) Len()` across `internal/component/ike/wire/*.go` returns nothing (read 2026-07-16), so per-payload length accounting must be added to every payload writer | The diff grows past the two call sites | Fall back to sizing the buffer from a computed worst case at the caller only |
| R-2 | A fix that allocates per packet regresses the hot path | Benchmark regression on the L2TP subscriber path | Keep a session-scoped reusable buffer (`wire.SessionBuffer` pattern) |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_SA_INIT with an unacceptable proposal, forced oversized notify data | → | `sendSAInitNotify` bound check (`responder.go`) | `TestSendSAInitNotifyOversizedRejected` |
| DHCPv6 Solicit producing an oversized reply | → | `BuildDHCPv6Reply` caller bound check (`ipv6_service.go`) | `TestDHCPv6ReplyOversizedRejected` |

<!-- No .ci row: neither oversize is reachable through a real peer today (A-1), so the
     bound is provable only by forcing the encoder past 512 at the unit seam. If A-1 is
     broken during design, a .ci becomes mandatory. -->

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An IKE notify whose encoded length exceeds the buffer | No panic; the send is skipped and the failure is logged with the peer and the required length |
| AC-2 | A DHCPv6 reply whose encoded length exceeds the buffer | No panic; the reply is not sent truncated; the failure is logged |
| AC-3 | Every notify/reply that fits today | Identical bytes on the wire, byte-for-byte, versus before the change |
| AC-4 | Repo sweep for the fixed-buffer shape | Every site found is either fixed or recorded with evidence that it cannot overflow |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peer sends an IKE_SA_INIT ze must reject | inbound UDP → handleSAInitRequest → sendSAInitNotify → bound check → transport | `TestSendSAInitNotifyOversizedRejected` |
| 2 | Subscriber requests a DHCPv6 prefix delegation | PPP → IPv6 service → BuildDHCPv6Reply → bound check → PPP writer | `TestDHCPv6ReplyOversizedRejected` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendSAInitNotifyOversizedRejected` | `internal/component/ike/engine/responder_test.go` | AC-1: oversized notify does not panic and is not sent | |
| `TestDHCPv6ReplyOversizedRejected` | `internal/component/l2tp/ppp/ipv6_service_test.go` | AC-2: oversized reply does not panic and is not sent truncated | |
| `TestSendSAInitNotifyBytesUnchanged` | `internal/component/ike/engine/responder_test.go` | AC-3: in-range notify bytes identical to pre-change golden | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IKE notify encoded length vs buffer | 0-512 | 512 | N/A | 513 |
| DHCPv6 reply encoded length vs buffer | 0-512 | 512 | N/A | 513 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A - no user-facing surface changes; the bound is internal hardening and neither oversize is reachable from a real peer today (A-1). If the design phase breaks A-1, a `.ci` becomes mandatory and this row must be replaced. | - | - | |

### Future (if deferring any tests)
- None planned. A fuzz target over the notify/reply encoders is a design-phase candidate.

## Files to Modify
- `internal/component/ike/engine/responder.go` - bound the `sendSAInitNotify` buffer
- `internal/component/ike/engine/dpd.go` - same shape at `:97`
- `internal/component/l2tp/ppp/ipv6_service.go` - bound the DHCPv6 reply buffer at `:145`, `:181`, `:213`, `:238`, `:250`
- `internal/core/bgp/wire/writer.go` - only if the checked contract must move to a neutral package (A-3)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | none expected: no config surface |
| CLI commands/flags | [ ] | none expected |
| Functional test for new RPC/API | [ ] | none expected (see Functional Tests opt-out) |
| Prometheus counters/metrics | [ ] | consider a counter for dropped oversized messages |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | no |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` only if the checked contract moves packages |

## Files to Create
- none expected beyond test files

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify + the A-2 sweep |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-verify` |
| 14. Present summary + close | two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Sweep (MANDATORY FIRST)** — resolve A-2 before fixing anything (A-2 is already BROKEN: the sweep confirms scope, it does not decide whether to sweep)
   - Verify: a repo-wide sweep lists every fixed-buffer + `WriteTo`/`buf[:n]` site -- INCLUDING the full-message IKE_SA_INIT builders (`responder.go:243`, `initiator.go:83`, `make([]byte, 4096)`) already known to be in this class; the spec's site table is updated to match
2. **Phase: Decide the contract** — resolve A-3
   - Verify: either `CheckedBufWriter` is importable from IKE/L2TP with `make ze-tier-check` green, or a neutral home is chosen
3. **Phase: Wiring** — add the failing bound tests at both sites
   - Tests: `TestSendSAInitNotifyOversizedRejected`, `TestDHCPv6ReplyOversizedRejected`
   - Verify: both FAIL (panic) before the fix
4. **Phase: Bound both sites** — implement
   - Verify: tests pass; `TestSendSAInitNotifyBytesUnchanged` proves AC-3
5. **Full verification** → `make ze-verify`
6. **Complete spec** → learned summary; TWO commits (A: code+tests+spec+learned; B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every site from the A-2 sweep is fixed or evidenced as safe |
| Correctness | The guard rejects; it never truncates (`ai/rules/no-workarounds-for-missing-behavior.md`) |
| Fail-closed | The oversize path logs or errors; it does not silently drop (`ai/rules/fail-closed-guards.md`) |
| Data flow | The bound sits at the buffer owner, not inside the encoder's callers' callers |
| Rule: buffer-first | No new per-packet allocation on the L2TP subscriber hot path |
| Registration over hardcoding | No new per-feature switch/factory in a core/shared package (`ai/rules/plugin-self-containment.md`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Both sites bounded | grep the two files for a capacity check before `buf[:n]` |
| Sweep evidence recorded | spec's site table lists every hit with a disposition |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Can a remote peer influence the notify/reply length? If yes, A-1 is broken and this is a remote DoS, not a latent class |
| Resource exhaustion | A worst-case-sized buffer must not be attacker-controlled in size |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 proves reachable | STOP, re-file as an urgent fixit; tell the user |
| A-2 finds many more sites | Back to design; consider a lint/sweep check instead of hand-fixing |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- The BGP side of the tree already carries the right contract (`CheckedBufWriter`), and the IKE/L2TP encoders simply never adopted it. This is a contract-adoption gap, not a missing idea.

## Known Limitations
- Scope is the two UNGUARDED encoder families only (IKE `Message.WriteTo`, L2TP `BuildDHCPv6*`). The already-safe siblings surfaced by the sweep (ISIS `make([]byte, EncodedLen())`, PPPoE `Builder.truncated`/`Finish()`-nil, BFD `PoolBufSize=64` cap) are out of scope by evidence, not by omission -- they need no fix.
- `BuildRA` (`internal/component/l2tp/ppp/ra.go:37`, called at `ra_linux.go:111` into `var buf [256]byte`) shares the exact unguarded-builder pattern and has a variable-length RDNSS option; it cannot overflow today only because its sole caller passes a fixed no-RDNSS `RAConfig`. It is a NOTED FOLLOW-UP (should adopt the same bound once this lands), deliberately NOT pulled into this spec per `ai/rules/no-partial-completion.md`.
- The 512-byte notify/reply sites are not remotely reachable today (A-1); this is latent-class hardening, not a live-bug fix. If the A-1 worst-case `Len()` computation later shows a remotely-influenced payload can exceed the buffer, this becomes an urgent remote-DoS fixit (see Failure Routing).

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
