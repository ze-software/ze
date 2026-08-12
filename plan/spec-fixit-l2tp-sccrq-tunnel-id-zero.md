# Spec: fixit-l2tp-sccrq-tunnel-id-zero

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | `-` |
| Updated | 2026-08-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**An SCCRQ carrying Assigned Tunnel ID 0 is dropped in silence. `RFC2661-24.10-1`
is a gated MUST that requires a StopCCN reply, its published coverage claims the
Reactor sends one, and both of its tagged tests prove the OTHER half of the
requirement.**

Found on 2026-08-02 while closing the rfcgate-1b RFC 7296 pilot spec.

**The way to fix this is the owner's decision, not this spec's.** The question is
stated in full below and must be answered before any code changes.
`ai/rules/rfc-compliance.md` reserves it: every route on the table either lowers
what Ze owes or adds an unauthenticated reply surface.

### What the code does

`parseSCCRQ` (`internal/component/l2tp/tunnel_fsm.go`) rejects a zero Assigned
Tunnel ID with an error, `"l2tp: Assigned Tunnel ID AVP must be non-zero"`.

`L2TPReactor.handle` (`internal/component/l2tp/reactor.go`) calls it for every
packet arriving with `TunnelID == 0`, and on error logged
`"l2tp: TunnelID=0 packet with malformed body dropped"` at Debug and returned.

Nothing was sent. The peer received silence.

`reactor.go` carries the comment `"sccrq.AssignedTunnelID is guaranteed non-zero
by parseSCCRQ"`, which is true and is exactly how the drop became invisible: the
validation moved to the parser, and the parser's only failure mode is a dropped
packet.

### What the requirement says

`rfc/short/rfc2661.md` states the obligation:

- `[RFC2661-24.10-1] [MUST] Assigned Tunnel ID of 0 in SCCRQ/SCCRP is a protocol
  error; reject with StopCCN (§24.10)`

The requirement names BOTH messages, SCCRQ and SCCRP.

`rfc/short/rfc2661.md` states the implemented coverage as `"Reactor rejects with
StopCCN Result Code 2"`. The Reactor is precisely the path that sends nothing.

### What the evidence actually proves

`rfc2661` is ENROLLED (`rfc/enrolled.txt`), so this MUST is gated.

Both tags sit on the initiator side, over SCCRP:

- `internal/component/l2tp/tunnel_initiator_test.go` carries the negative polarity.
- `internal/component/l2tp/tunnel_initiator_test.go` carries the positive polarity.

The generated ledger records both as `unit/verify`. It was one file,
`ai/RFC-REQUIREMENTS.md`, when this spec was written; the repo has since sharded
it per RFC, so this requirement's row now lives in `rfc/requirements/rfc2661.md`.
Every later reference in this spec means that shard.

So the requirement is fully gated in both polarities, and both polarities test the
SCCRP half. The SCCRQ half has no test at all. The ledger cannot see this, because
it counts polarities per requirement id and not per message type.

This is a public claim standing ahead of its evidence, which is the failure class
the pilot existed to find. It is not a false tag: the two tests are honest about
what they drive. The gap is that one requirement id covers two message types and
only one of them is met.

### The owner question (BLOCKING, answer before any code lands)

**ANSWERED 2026-08-10 (Thomas): Route C, the bounded middle.** Ze sends StopCCN
on the SCCRQ path, and the emission is bounded so a forged datagram cannot make Ze
a general reflector. A source that already has a tunnel is answered. Any other
source is answered under a per-source rate limit, and is dropped in silence above
it. Route A is refused for the reflector surface. Route B is refused because it
lowers what Ze owes. The requirement stays a gated MUST and is met on both message
types after this spec.

The threat model this route needs is part of the deliverable, not a follow-up.
State the rate limit, state what an attacker gains at that limit, and state what a
legitimate peer loses when the limit is reached.

**Both routes are legitimate and this spec may not choose between them.**

Route A, send StopCCN on the SCCRQ path. This is full compliance with §24.10 as the
summary states it. It makes ze answer a datagram that has not been authenticated
and whose source address is trivially spoofed, so ze becomes a reflector: one
forged SCCRQ produces one StopCCN to a victim of the attacker's choosing. The
control channel has no return-routability check at this point in the exchange.

Route B, keep the silent drop and narrow the published claim to the SCCRP half.
RFC 2661 Section 4.4.3 permits, rather than requires, a reply here, and declining to
answer an unauthenticated datagram is the conservative reading. This route LOWERS
what Ze publicly owes, so `ai/rules/rfc-compliance.md` forbids a subagent from
choosing it and requires the owner's explicit answer.

Route C, a bounded middle. Send StopCCN but rate-limit it per source, or only to a
source that has an existing tunnel. This is more code and needs its own threat
model.

**What full compliance costs, stated so the answer is informed.** Route A needs a
StopCCN emission path reachable before any tunnel state exists, because the current
drop happens before a tunnel entry is allocated, and that ordering is deliberate:
the comment at `reactor.go` says the body is parsed before `tunnelsMu` is taken
so a malformed body allocates nothing. Emitting StopCCN needs a peer identity and a
control-channel sequence, so it either allocates the state the drop avoids or sends
a synthetic reply outside the tunnel machinery. It also needs a tagged test on the
SCCRQ path in both polarities, written from scratch.

→ Constraint (corrected 2026-08-03): this paragraph used to say "the reproducer
already exists in `test/draft/l2tp/`". It does not. `test/draft/` is gitignored,
so the two RFC 2661 control-shape drafts written on 2026-08-02 were
session-local and are gone; the directory holds only `plugin/`, `web/` and
`README.md` today. Write the reproducer, do not go looking for it. The shape to
copy is `test/l2tp/rfc2661-emitted-control-shape.ci`, which is committed: a
Python peer that sends a hand-packed SCCRQ over UDP and parses ze's reply with
its own decoder.

**Do not write a `{gap}`, a `partial`, or a `{not-applicable}` for this.** Writing
one IS the decision, and `check_coverage_ratchet` plus `check_retired_requirements`
correctly refuse the cheaper escapes. The requirement stays as it is until the
owner answers.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/rfc-compliance.md` - who may decide anything short of full compliance
  → Constraint: a subagent may not classify this row away. Quote the requirement, name the producer, state the cost, and ask which way to fix it
- [ ] `ai/rules/evidence.md` - a guard that neither denies nor speaks
  → Constraint: the current drop denies correctly but says nothing to the peer, which is why the behavior was invisible
- [ ] `ai/rules/testing.md` - RFC-tagged tests and the draft incubator
  → Constraint: a tagged test IS the requirement. Write the SCCRQ test in `test/draft/l2tp/` first, and promote it when green

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2661.md` - the `RFC2661-24.10-1` line in the Compliance Checklist, and the Assigned-Tunnel-ID-0 row of the trap table
  → Constraint: `RFC2661-24.10-1` names SCCRQ AND SCCRP. Ze meets the SCCRP half only
  → Decision: Section 4.4.3 permits rather than requires the reply, which is what makes route B arguable rather than plainly non-conformant

**Key insights:** (minimal context to resume after compaction)
- The requirement is gated and tagged in both polarities, and both tags test SCCRP.
- The SCCRQ path silently drops at `reactor.go`.
- The published coverage line names the Reactor, which is the path that sends nothing.
- The fix direction is the OWNER's call. Do not narrow the claim and do not write a gap.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/l2tp/tunnel_fsm.go` - `parseSCCRQ`, the zero-ID rejection in the `AVPAssignedTunnelID` arm
- [ ] `internal/component/l2tp/reactor.go` - `handle`, the `hdr.TunnelID == 0` branch: it parses, logs at Debug and returns; and the `parseSCCRQ` guarantee comment above the accept path
- [ ] `internal/component/l2tp/tunnel_initiator_test.go` - `TestParseSCCRP_Rejects` and `TestTunnelInitiatorHandshake`, the two `RFC2661-24.10-1` tags, both over SCCRP

**Behavior to preserve:** (unless the user explicitly said to change it)
- `parseSCCRQ` keeps rejecting a zero Assigned Tunnel ID. The validation is correct and only the response to it is in question.
- The parse-before-lock ordering stays. Parsing before `tunnelsMu` is taken is what stops a malformed body allocating a tunnel entry or consuming a local TID.
- The SCCRP half keeps working, and its two tagged tests keep their ids and polarities.
- Every other `TunnelID == 0` drop reason keeps its current behavior unless the owner's answer covers it too.

**Behavior to change:** (only what the user asked for)
- An SCCRQ whose Assigned Tunnel ID AVP carries 0 is answered with StopCCN Result Code 2, Error Code 3, instead of being dropped in silence. The reply is bounded per source address, with no exception for any source.
- Nothing else changes. Every other `TunnelID == 0` drop keeps its silent drop, the parse-before-lock ordering is untouched, and the SCCRP half keeps its behaviour and its two tags.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A UDP datagram arrives on the L2TP control port with `TunnelID == 0`.
- Format at entry: an L2TP control header followed by an AVP body, unauthenticated and source-spoofable.

### Transformation Path
1. `L2TPReactor.handle` reads the header and selects the `TunnelID == 0` branch.
2. `parseSCCRQ` walks the AVP body and validates the Assigned Tunnel ID.
3. On a zero ID the parser returns an error, before `tunnelsMu` is taken.
4. `handle` logs at Debug and returns. Before this spec nothing was transmitted; now `answerZeroTunnelIDSCCRQ` runs first, on the zero-Assigned-Tunnel-ID error only.
5. The peer waits, retransmits, and eventually times out.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer ↔ Reactor | unauthenticated UDP control datagram | Yes. `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` drives it over a real socket and decodes the reply outside ze |
| Reactor ↔ tunnel table | `tunnelsMu`, deliberately taken AFTER the parse | Yes. The answer is emitted before the lock; `TestZeroIDSCCRQFloodAllocatesNoTunnel` asserts neither map grows |
| Ze ↔ published claim | `rfc/short/rfc2661.md` and `rfc/requirements/rfc2661.md` | Yes. The coverage cell now describes both halves and the ledger row carries functional evidence in both polarities |

### Integration Points
- `internal/component/l2tp/reactor.go` - the drop site and any future emission point.
- `teardownStopCCN` and the existing StopCCN paths, which today all require a tunnel.
- `test/l2tp/` - where the reproducer landed, beside `rfc2661-emitted-control-shape.ci`, whose Python peer shape it copies. `test/draft/` is gitignored and held no RFC 2661 draft, per the corrected Constraint above.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The reply is built by `writeStopCCNBody`, the same encoder `teardownStopCCN` uses, and leaves through `listener.Send`, the same sender as every other outbound datagram, and is recorded in both capture rings (`captureRing.appendOutbound` and `RawCaptureRing.Append`) so `ze diag l2tp` shows the reply beside the SCCRQ that drew it. Only the reliable engine is bypassed, deliberately: it lives on a tunnel and no tunnel exists |
| No unintended coupling (components stay isolated) | Yes | Everything added is inside `internal/component/l2tp` and nothing outside the package sees a new symbol |
| No duplicated functionality (extends existing, does not recreate) | Yes | `writeStopCCNBody`, `WriteControlHeader`, `GetBuf` and `listener.Send` are reused. The one new thing is the tunnel-free emission, which had no equivalent (A-3) |
| Zero-copy preserved where applicable (refs, not copies) | Yes | One pooled buffer, returned with `defer PutBuf`. The refusal path takes no lock and walks no table. It is not allocation-free: `handle` logs every dropped `TunnelID=0` body at Debug with `pkt.from.String()`, which a refused datagram pays like every other drop reason. `answerZeroTunnelIDSCCRQ` adds no second log line of its own |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No command, view, family or handler is added. The rate-limit table is a field on `L2TPReactor`, which owns it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No other code path answers a zero-ID SCCRQ with StopCCN | read, `reactor.go` `handle` returned on the `parseSCCRQ` error | The requirement is already met and only the test is missing, which makes this a test-only spec | grep every StopCCN emission site and check reachability with no tunnel | confirmed. Every emission went through `teardownStopCCN`, a method on `*L2TPTunnel`, so none was reachable without a tunnel |
| A-2 | Both `RFC2661-24.10-1` tags really do drive SCCRP and never SCCRQ | read, `tunnel_initiator_test.go` and `:135` | The SCCRQ half is already proven and the finding dissolves | read both test bodies in full | confirmed. `TestParseSCCRP_Rejects` calls `parseSCCRP`; `TestTunnelInitiatorHandshake` drives the initiator FSM. Neither reaches `parseSCCRQ` or `handle` |
| A-3 | Sending StopCCN before any tunnel exists needs new machinery | read, existing StopCCN paths all hang off a tunnel | Route A is far cheaper than stated, which should change the owner's answer | trace `teardownStopCCN` and check whether a tunnel-free emission already exists | confirmed. `sendUnassociatedStopCCN` is new: it carries its own header and sequence numbers because the reliable engine lives on a tunnel |
| A-4 | The reflection concern is real, meaning no return-routability check precedes this point | read, the control exchange order | Route A carries no amplification risk and is the obvious answer | confirm no cookie or challenge precedes SCCRQ handling | confirmed. `handle` parses and dispatches the first datagram from any source; nothing precedes it |
| A-5 | A StopCCN reply is not larger than the SCCRQ that triggers it | not checked | Route A is an amplification vector and not merely a reflection one | compare the encoded sizes | **broken**. Measured against a running daemon: a 28-byte minimum triggering SCCRQ draws a 38-byte StopCCN, 1.36x at the L2TP layer and 1.14x as Ethernet frames (74 in, 84 out). The reply IS larger, so the emission is an amplification vector and not only a reflection one. The per-source rate limit is what bounds it |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Route A makes ze a reflector for spoofed SCCRQ traffic | a rise in StopCCN transmissions with no matching tunnel | CLOSED by Route C. The ceiling is one reply per source slot per second and 256 slots, so 256 datagrams per second from one reactor, and no address receives more than one per second. The bound carries no exception: an earlier draft exempted a source that owned a tunnel, which left the ceiling unbounded for any address an attacker could name. See the threat model in Key Design Decisions |
| R-2 | Emitting StopCCN allocates the tunnel state the current ordering avoids, creating a memory exhaustion path | tunnel table grows under a flood of malformed SCCRQ | CLOSED. The emission runs before `tunnelsMu` is taken, allocates nothing but a pooled buffer, and the rate table is a fixed-size array. `TestZeroIDSCCRQFloodAllocatesNoTunnel` is the standing check |
| R-3 | Someone narrows the summary or writes a gap instead of asking | a diff touching `rfc/short/rfc2661.md` classifications | CLOSED. The owner answered and no annotation was added. Round 4 edited requirement TEXT to carry the RFC's real sections, which raises no classification: every id, level and anchor is unchanged and `make ze-rfc-check` still parses all 25 requirements |
| R-4 | The fix lands with a tagged test on the SCCRQ path that duplicates the existing id and disturbs the SCCRP tags | `make ze-rfc-check` reports an id or polarity change | CLOSED. The id stays `RFC2661-24.10-1`, the two SCCRP tags are byte-identical, and the regenerated ledger row shows the SCCRQ evidence added beside them |
| R-5 | A regeneration of the ledger also absorbs concurrent sessions' tag churn | a `git diff` of the ledger touches rows this spec never went near | OPEN, and it MATERIALISED. Round 4's `make ze-rfc-index` absorbed RFC 5301 moving from backlog to enrolled, and RFC 4271 and RFC 5216 each gaining a tag. The tag total moved 3332 -> 3336 -> 3338 across round 4's two regenerations, with no tag added here. Whoever prepares the commit re-runs `make ze-rfc-index` at that moment, which is the standing rule anyway |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Route A wrongly applied turns ze into a packet reflector reachable by any spoofed source. Route B wrongly applied leaves a published MUST claim standing over behavior that does not exist. Both are worse than the current state, which is why the owner decides. |
| How is it reverted? | Route A is a single commit revert. A narrowed published claim is also revertible, but it is visible to readers in the meantime. |
| Who else touches this path? | `plan/spec-finish-l2tp.md` (L2TP residuals), `plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md` (evidence carriers and tiers) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SCCRQ with Assigned Tunnel ID 0 | → | `handle` zero-ID branch, `answerZeroTunnelIDSCCRQ` | `TestSCCRQWithZeroAssignedTunnelIDIsAnswered` (passing) |
| SCCRQ with a non-zero Assigned Tunnel ID | → | `handle` accept path | `TestSCCRQWithNonZeroAssignedTunnelIDEstablishes` (passing) |
| Zero-ID SCCRQ over the wire | → | the Reactor control path | `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` (passing in `make ze-l2tp-test`) |

## Acceptance Criteria

<!-- AC-1 is deliberately the owner's answer. Every later AC is conditional on it,
     and the conditional wording is intentional: this spec must not presume a route. -->
| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-1 | The owner question above is put to Thomas | A recorded answer selecting route A, B or C, captured in Key Design Decisions with the rejected alternatives | Thomas answered Route C on 2026-08-10; recorded in "The owner question" and in Key Design Decisions with both rejected routes |
| AC-2 | Route A or C is chosen, and an SCCRQ carrying Assigned Tunnel ID 0 arrives | Ze transmits StopCCN with Result Code 2 and allocates no tunnel entry | `answerZeroTunnelIDSCCRQ` and `sendUnassociatedStopCCN` (`internal/component/l2tp/reactor.go`). `TestSCCRQWithZeroAssignedTunnelIDIsAnswered` and `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci`, which reads Result Code 2 and Error Code 3 off the wire with a decoder that is not ze |
| AC-3 | Route B is chosen | The published coverage at `rfc/short/rfc2661.md` states the SCCRQ half is a deliberate silent drop, and the reason, so no reader is misled | N-A. Route B was refused |
| AC-4 | Any route is chosen | The SCCRQ path carries an `RFC requirement:` tag in both polarities, with the id the owner's answer settles | `RFC2661-24.10-1` keeps its id. Both polarities are tagged on the SCCRQ path in `reactor_sccrq_zero_tid_test.go` and in the `.ci`; the two SCCRP tags are untouched |
| AC-5 | `make ze-rfc-check` runs after the change | It passes, and the `RFC2661-24.10-1` ledger row is fresh in the same commit | The `RFC2661-24.10-1` row carries unit/verify and functional/verify evidence in both polarities. **The ledger AC-5 named, `ai/RFC-REQUIREMENTS.md`, no longer exists: the repo sharded it to `rfc/requirements/<stem>.md`, and this requirement's row lives in `rfc/requirements/rfc2661.md`.** That shard is already correct at HEAD and this spec regenerates nothing. On 2026-08-10, round 4: `make ze-rfc-check` reported one violation, the ledger stale against its sources, which is AC-5's own obligation and R-5's standing prediction. `make ze-rfc-index` then made the run exit 0: 2963 gated MUST-level requirements across 170 enrolled RFCs, 3336 tags resolved. **This is a measurement, not a standing property, and round 4 watched it decay.** Ten minutes later the ledger was stale again with no edit here; a second regeneration read 3338 tags. The first absorbed RFC 5301 enrolling and RFC 4271 and RFC 5216 gaining tags, none of them this spec's. On 2026-08-12 at closure, `make ze-rfc-check` exits 0 with 3341 tags resolved and no `rfc2661` violation, with no regeneration needed here |
| AC-6 | A flood of zero-ID SCCRQ datagrams arrives | The tunnel table does not grow, whichever route was chosen | `TestZeroIDSCCRQFloodAllocatesNoTunnel`: 50 datagrams, one reply, both tunnel maps empty. The emission happens before `tunnelsMu` is taken, so the parse-before-lock ordering is unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A peer misconfigures its tunnel ID and dials ze | UDP → `handle` → `parseSCCRQ` → reply or drop | `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSCCRQWithZeroAssignedTunnelIDIsAnswered` | `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` | AC-2, negative polarity | passing |
| `TestSCCRQWithNonZeroAssignedTunnelIDEstablishes` | `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` | AC-4 positive polarity | passing |
| `TestZeroIDSCCRQFloodAllocatesNoTunnel` | `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` | AC-6 | passing |
| `TestZeroIDSCCRQAnsweredAgainAfterInterval` | `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` | the bound is a window, boundary at the interval and one nanosecond below | passing |
| `TestZeroIDSCCRQFromTunnelOwnerIsBounded` | `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` | the bound has no exception: a source that owns a tunnel is answered once per interval like every other source | passing |
| `TestUnassociatedStopCCNIsCaptured` | `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` | the emission reaches both capture rings, so `ze diag l2tp` shows the reply | passing |
| `TestAllocateLocalTIDSkipsTidNoTunnel` | `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` | the reserved Assigned Tunnel ID is never allocated | passing |
| `TestStopCCNSlotInRange` | `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` | the slot hash stays inside the table for v4 and v6 extremes | passing |

### Discrimination proof (`ai/rules/interop-and-goal-validation.md`)

Each behaviour was reverted and the matching test observed to fail.

| Reverted | Test that failed | Still passing |
|----------|------------------|---------------|
| the `answerZeroTunnelIDSCCRQ` call in `handle` | `TestSCCRQWithZeroAssignedTunnelIDIsAnswered`, and `rfc2661-sccrq-tunnel-id-zero.ci` in the full suite | `TestSCCRQWithNonZeroAssignedTunnelIDEstablishes` |
| the rate-limit branch | `TestZeroIDSCCRQFloodAllocatesNoTunnel`, `TestZeroIDSCCRQAnsweredAgainAfterInterval` | the negative-polarity test |
| the bound's no-exception property, by restoring the earlier tunnel-owner exemption | `TestZeroIDSCCRQFromTunnelOwnerIsBounded`, at `owning a tunnel buys no second reply inside the interval: unexpected 38-byte reply` | the flood test and both polarity tests |
| the `tidNoTunnel` skip in `allocateLocalTID` | `TestAllocateLocalTIDSkipsTidNoTunnel` | -- |
| the `capture.appendOutbound` call in `sendUnassociatedStopCCN` | `TestUnassociatedStopCCNIsCaptured`, at `"[]" should have 1 item(s), but has 0` | every other test in the file, which read the wire and not the ring |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Assigned Tunnel ID | 1..65535 | 65535 | 0 (the case this spec is about) | N/A, the AVP is uint16 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc2661-sccrq-tunnel-id-zero` | `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` | A peer dials with tunnel ID 0 and is told why, by a StopCCN a Python decoder reads off the wire | passing in `make ze-l2tp-test`, 20/20 |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| l2tp control | `test/l2tp-interop/scenarios/` | xl2tpd | N-A, and the reason is the trigger rather than the cost. RFC 2661 Section 4.4.3 forbids the value, so no conforming peer emits an Assigned Tunnel ID of 0: an interop scenario would have to hand-pack the datagram itself, which is exactly what `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` does, with a decoder that shares no code with ze. Nothing runs this tree automatically today either, per `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md`, so a tag placed there earns no tier | N-A |

## Files to Modify
- `internal/component/l2tp/reactor.go` - the answer branch in `handle`, plus `answerZeroTunnelIDSCCRQ`, `sendUnassociatedStopCCN`, `stopCCNSlot`, the `stopCCNLastSent` field, the `tidNoTunnel` skip in `allocateLocalTID`
- `internal/component/l2tp/tunnel_fsm.go` - `parseSCCRQ` returns the sentinel; `resultProtocolError` and `errorValueOutOfRange`; `writeStopCCNBody` takes a `ResultCodeValue`
- `internal/component/l2tp/errors.go` - `errZeroAssignedTunnelID`
- `internal/component/l2tp/tunnel_initiator.go`, `tunnel.go`, `reactor_dial.go`, `reactor_test.go` - round 4 only: RFC section citations corrected against `rfc/full/rfc2661.txt`. No behaviour changes
- `rfc/short/rfc2661.md` - the trap table's Assigned-Tunnel-ID-0 coverage cell now states both halves. Round 4 also re-anchored five headings, the trap table's five rows and eight guide-derived checklist anchors against `rfc/full/rfc2661.txt`. Every requirement id, level and `(§N)` anchor is unchanged
- `rfc/requirements/rfc2661.md` - regenerated by `make ze-rfc-index`, never hand-edited. Unmodified against HEAD at closure: another session's `make ze-rfc-index` already published this spec's tags there
- `plan/journal/gate-excludes-part-of-its-population.md` - one row: the lesson this spec produced
- `plan/journal/reference-checked-claim-unchecked.md` - one row: the section numbers no gate reads
- `plan/journal/bulk-rename-corruption.md` - one row, found on the way: the `unused` reds on `writeAVPUint8` and `readAVPUint8` in `avp.go`, owned by `fixit-unexport-package-private-symbols`

## Files to Create
- `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` - the seven unit tests
- `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` - the functional reproducer with an external Python decoder
- `plan/journal/index-key-derived-from-mutable-field.md` - one row for a defect found on the way (see Known Limitations)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Route C adds no rate-limit leaf. The bound is a constant, and an operator knob would be an option nobody asked for whose only settings are "reflect more" and "answer less" |
| YANG validation constraints | N-A | Same condition as the row above |
| YANG custom validators | N-A | No new leaf expected |
| CLI commands/flags | N-A | No command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No new leaf expected |
| Functional test for new RPC/API | Yes | `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No new env var expected |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | No | The emission logs at Info and the suppression at Debug, matching every sibling drop in `handle`. The ceiling is 256 datagrams per second from one reactor, so a log is not a flood surface. A counter would be new observability machinery the fix does not need |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Conformance fix |
| 2 | Config syntax changed? | No | Route C adds no leaf |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes, `docs/guide/l2tp.md` | Nothing in it becomes false: it documents no behaviour for a zero Assigned Tunnel ID. This session was directed to leave `docs/**` alone, a concurrent session holds it. Flagged for closure |
| 7 | Wire format changed? | Yes | Ze now emits a StopCCN where it emitted nothing. The datagram shape is documented above `sendUnassociatedStopCCN` with the RFC 2661 sections it follows, and the coverage cell in `rfc/short/rfc2661.md` says what goes out |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2661.md` Section 24.10 cell updated. The `docs/features/rfc-status.md` RFC 2661 row stays true: it already claims StopCCN, its Status stays Partial, and its one named MUST gap (`RFC2661-4.3-1`) is untouched. No gate demands an edit, and this session was directed to leave `docs/**` alone |
| 10 | Test infrastructure changed? | No | One new `.ci` in the existing `test/l2tp` suite and one new `_test.go` in an existing package. No runner or harness change |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | None added; see the Integration Checklist row |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, verified | The anchors on `reactor.go` name `handleTick`, `handleKernelSuccess` and "L2TPReactor dispatch"; the one on `tunnel_fsm.go` names "tunnel FSM transitions". This change renames and removes nothing, so every anchor still resolves |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No config leaf, no command, no API changed |

## Implementation Steps

1. **Phase: Ask (MANDATORY FIRST, BLOCKING)** - put the owner question to Thomas
   - Tests: none. No code changes in this phase
   - Files: Key Design Decisions records the answer and the rejected routes
   - Verify: AC-1. Validate A-1 through A-5 first, so the answer is informed rather than guessed
2. **Phase: Wiring** - the failing reproducer in the incubator
   - Tests: `test/draft/l2tp/rfc2661-sccrq-tunnel-id-zero.ci`
   - Files: the draft `.ci`
   - Verify: it reproduces today's silence, per `ai/rules/testing.md` draft-first
3. **Phase: Implement the chosen route**
   - Tests: the three unit tests above
   - Files: `internal/component/l2tp/reactor.go`, and `tunnel_fsm.go` if needed
   - Verify: AC-2 or AC-3, plus AC-6 under a flood
4. **Phase: Tag and publish** - align the evidence with the claim
   - Tests: the SCCRQ tags in both polarities
   - Files: `rfc/short/rfc2661.md`, then `make ze-rfc-index` regenerates the ledger
   - Verify: AC-4 and AC-5. Commit the regenerated ledger in the SAME commit
5. **Phase: Promote** - move the draft into the live suite
   - Tests: the promoted `.ci`
   - Files: `test/l2tp/`
   - Verify: green under `scripts/dev/stress-repro.py` before promotion

### Critical Review Checklist
| Check | What to verify for this spec | Result |
|-------|------------------------------|--------|
| Completeness | Every AC-N has an implementation at file plus symbol | PASS. See the Evidence column of the Acceptance Criteria |
| Feature completeness | The SCCRQ half is covered, and the SCCRP half still is | PASS. The ledger row carries both polarities on both halves; the SCCRP tags are byte-identical |
| Correctness | The parse-before-lock ordering survives, so a flood allocates nothing | PASS. `answerZeroTunnelIDSCCRQ` is called inside the `perr != nil` branch of `handle`, which returns before `tunnelsMu` is taken. `TestZeroIDSCCRQFloodAllocatesNoTunnel` asserts both maps stay empty |
| Naming | The new test names say SCCRQ, so nobody confuses them with the SCCRP pair | PASS. Both new tags name SCCRQ in their first line, and the file is `reactor_sccrq_zero_tid_test.go` |
| Data flow | No tunnel entry is allocated on the refusal path | PASS. The reply uses a pooled buffer, reads neither tunnel map and takes no lock |
| Rule: `ai/rules/rfc-compliance.md` | The owner answered. No `{gap}`, `partial` or `{not-applicable}` was written by anyone else | PASS. No new annotation appears in `rfc/short/rfc2661.md`. `RFC2661-24.10-1` keeps its id, level and `(§24.10)` anchor; round 4 corrected its TEXT to name RFC 2661 Sections 4.4.3 and 5.3, the misquote-correction route the rule allows |
| Rule: `ai/rules/testing.md` | The existing `RFC2661-24.10-1` tags kept their ids and polarities | PASS. Both tags in `tunnel_initiator_test.go` are untouched and still appear in the ledger row |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Owner answer recorded | Key Design Decisions names the chosen route and the rejected ones |
| Behavior matches the claim | the `RFC2661-24.10-1` line and the trap table's Assigned-Tunnel-ID-0 row in `rfc/short/rfc2661.md` agree with `sendUnassociatedStopCCN` |
| SCCRQ tagged in both polarities | `grep -rn "RFC2661-24.10" internal/component/l2tp/` |
| Ledger fresh | `make ze-rfc-check` |
| No tunnel growth under flood | `go test -run TestZeroIDSCCRQFlood ./internal/component/l2tp/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The Assigned Tunnel ID is peer-controlled and unauthenticated. The refusal must not depend on any later-validated field |
| Resource exhaustion | R-2. A reply path must not allocate tunnel state, or a malformed-SCCRQ flood becomes a memory attack |
| Amplification and reflection | R-1 and A-5. If a reply is sent, confirm it is not larger than the datagram that triggered it, and consider a per-source bound |
| Error leakage | A StopCCN Result Code must not reveal more about ze than the peer already knows |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- One requirement id covering two message types can be fully gated in both polarities while half of it is untested. The ledger counts polarities per id, so it cannot see this shape. Worth raising as a general pattern if a second instance appears. Row: `plan/journal/gate-excludes-part-of-its-population.md`, beside the RFC 9234 destination-role row, which is the same shape over an enumeration of roles.
- **A wrong section number has a SUPPLIER, and the missing gate is only what lets it spread.** `check_doc_links.py` resolves the PATH in a `// RFC:` header and `make ze-rfc-check` reads `RFC requirement:` tags, so no gate reads a section number and a wrong one is green everywhere. But nobody invented these numbers: `rfc/short/rfc2661.md` took its headings and eight checklist anchors from `docs/research/l2tpv2-implementation-guide.md`, whose own numbering runs to Section 26 while RFC 2661 stops at Section 13. Every author consulted the summary and copied them onward. Round 4 corrected the summary, then 36 source locations. Row: `plan/journal/reference-checked-claim-unchecked.md`. Resolving a section number against `rfc/full/<stem>.txt` is mechanical, and nothing does it.
- **An id anchored to a wrong section cannot be corrected.** `_validate_id` (`scripts/dev/rfc_requirements.py`) refuses a checklist line whose id and trailing citation disagree, and `check_retired_requirements` refuses a vanished id. `RFC2661-24.10-1` cites `(§24.10)`, a section RFC 2661 does not have, and neither ratchet can be satisfied by editing the anchor. The real sections went into the requirement TEXT instead, which `ai/rules/rfc-compliance.md` allows. Nine RFC 2661 ids are in this state after round 5 added `RFC2661-4.1-2`, whose anchor names a real section that states a different rule.
- **A sweep for exactly this defect is not a control for it, because the class has two halves and only one is visible.** Round 4 swept six files for wrong RFC section numbers, corrected 36 sites, and reported those files clean. Round 5 found 31 more, in the same files and their siblings, every one NUMERICALLY IN RANGE. An out-of-range number (`§24.10`, `§15`) announces itself to a reader and to a script. An in-range wrong number (`§4.1` for a rule stated in `§4.4.1`) announces itself to neither, so it survives the pass that was looking for it and it is absent from every count anybody publishes. That is why the residual measurement is not a health check, and why the fix is a machine holding the anchor against `rfc/full/<stem>.txt`: `plan/future/spec-rfc-anchor-resolution-check.md`.
- **Every number in a `// RFC:` file header resolves to a cited site in that file, and every section a file relies on appears in its header.** Round 4 dropped `§5.1` from `reactor.go` and added the three sections `tunnel_fsm.go` relied on but omitted (finding 9); round 5 dropped `§6.1` from `reactor_dial.go` and stated the first half here as a rule. The header is a routing index for a reader, and a number no site in the file supports is unverifiable from the file itself, which is exactly how the guide numbering spread. The section that governs a neighbouring file is reached through the `Related:` line, which already names it.
- **A rule stated in a sweep is not a rule the sweep obeyed.** Round 5 wrote the header rule and broke both halves of it in the same pass: it ADDED an unsupported `§5.1.1` to `auth.go`'s header while deleting `§6.1` from `reactor_dial.go` for that exact defect, and it added sites to three files (`tunnel_fsm.go` §5.7, `tunnel_initiator.go` §4.4.1 and §4.4.3, `reliable.go` §4.4.1) without touching their headers. A header is a two-sided invariant and an edit to either side can break it, which is why the fix is a machine and not a habit (`plan/future/spec-rfc-anchor-resolution-check.md`).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Route C, the bounded middle: ze answers the SCCRQ half with StopCCN, bounded per source | Route A, answer every source: refused for the reflector surface. Route B, keep the silent drop and narrow the claim: refused because it lowers what Ze owes | Thomas ruled on 2026-08-10. The requirement stays a gated MUST and is met on both message types |
| The bound is one StopCCN per source address per second, over a fixed 256-slot table | A per-source map with an entry per address: a spoofed flood makes ze allocate, which is R-2 in another shape. A global token bucket: one attacker starves every legitimate peer | Fixed memory is the property that matters: nothing an unauthenticated datagram does can grow it. Two addresses can share a slot, and sharing fails towards sending LESS |
| The bound has NO exception, so nothing on this path reads the tunnel table | Exempting a source that owns a tunnel: the exemption is keyed on an address, and an attacker who names any current peer of this LNS spoofs it, so the ceiling disappears for exactly the addresses ze most wants to protect. Storing the slot's owning address to make the exemption a collision-breaker only: two tunnel-owning addresses that share a slot then alternate, and each alternation sends, so the ceiling disappears again | Nothing at this point in the exchange proves return-routability, so no address-keyed exemption can be bounded with fixed memory. Removing it also removes the O(live tunnels) walk under `tunnelsMu`, which sat on the flood path: a refused datagram now takes no lock and walks no table |
| Only the zero Assigned Tunnel ID parse failure is answered; every other malformed TunnelID=0 body keeps its silent drop | Answering every parse failure | The owner's ruling covers `RFC2661-24.10-1`, and the spec's Behavior to preserve keeps the other drop reasons as they are. `errZeroAssignedTunnelID` is the sentinel that separates them |
| The emitted StopCCN carries Assigned Tunnel ID `tidNoTunnel` (0xFFFF), and `allocateLocalTID` never hands that value out | Emitting 0: RFC 2661 Section 4.4.3 requires a non-zero value. Allocating a real local TID: that is the tunnel state the parse-before-lock ordering exists to avoid | The AVP has to say something and no tunnel exists. Reserving the value means a peer that answers one cannot reach a stranger's tunnel |
| Result Code 2 with Error Code 3 | Result Code 1, which the SCCRP path uses | RFC 2661 Section 4.4.2: Result Code 2 is "General error--Error Code indicates the problem", and Error Code 3 is "One of the field values was out of range". Result Code 1 says "general request to clear", which does not name the fault. The published claim already said 2 |
| The SCCRP half keeps Result Code 1 and its two existing tags | Extending `teardownStopCCN` to a `ResultCodeValue` and sending 2 there too | Result Code 1 is RFC-legal, the spec's Behavior to preserve covers that half, and the change would touch seven call sites for no conformance gain. The summary cell now states both halves rather than implying one |

### Threat model of the bound (deliverable, not a follow-up)

| Question | Answer |
|----------|--------|
| The limit | One StopCCN per source-IP slot per second. 256 slots, hashed FNV-1a over the address, held in `L2TPReactor.stopCCNLastSent`. No source is exempt, a source that owns a live tunnel included |
| What the emission costs | Measured at the L2TP layer: a 28-byte minimum triggering SCCRQ draws a 38-byte StopCCN, so one answered datagram amplifies 1.36x. As Ethernet frames the same pair is 74 bytes in and 84 bytes out, 1.14x, because the 46 bytes of UDP, IPv4, Ethernet and FCS are paid by both |
| What an attacker gains at that limit | Against one spoofed victim address: one 38-byte L2TP datagram per second, about 300 bit/s of L2TP payload and about 670 bit/s as an 84-byte Ethernet frame, whatever the flood rate. Amplification over the whole attack falls as the flood grows, because the attacker pays for every datagram and ze answers one per second. To raise the packet rate the attacker must forge more source addresses, and each reply then goes to a different address, so no single victim receives more than one per second |
| What ze spends | Fixed, and constant per datagram. The table never grows, the reply uses a pooled buffer, and no tunnel entry or local TID is allocated. A refused datagram takes no lock and walks no table: `answerZeroTunnelIDSCCRQ` does one FNV-1a hash, which allocates nothing, and one time comparison. It is NOT allocation-free. `handle` logs every dropped `TunnelID=0` body at Debug with `pkt.from.String()`, and that line costs 2 allocations and about 196 ns even with Debug disabled (measured on this machine, `netip.AddrPort.String` plus a level-disabled `slog` call). The zero-ID datagram pays exactly what every other malformed `TunnelID=0` datagram already paid: `answerZeroTunnelIDSCCRQ` carries no log line of its own. The flood path is still cheaper than the accept path, and the parse still happens before `tunnelsMu` is taken |
| What a legitimate peer loses at the limit | Its StopCCN, which puts it back to the behaviour that existed before this spec: it retransmits and times out. It never loses a tunnel it could otherwise have opened, because a zero Assigned Tunnel ID cannot open one |
| Who can cause that loss | An attacker who computes the hash can occupy a chosen peer's slot, because the hash carries no per-process seed. The cost to that peer is one missing diagnostic. Owning a tunnel does not buy it out of the slot: an exemption keyed on the address would be an exemption an attacker can claim by spoofing that address |
| What is NOT bounded | The overall emission ceiling is 256 datagrams per second, one per slot. The rate depends on the layer counted: 38 bytes of L2TP payload is 77.8 kbit/s, and the same datagram as an 84-byte Ethernet frame (8 UDP, 20 IPv4, 14 Ethernet, 4 FCS) is 172 kbit/s, before preamble and inter-frame gap. Both are from one reactor |

## Known Limitations
- RFC 2661 Section 4.4.3 states that "In the StopCCN control message, the Assigned Tunnel ID AVP MUST be the same as the Assigned Tunnel ID AVP first sent to the receiving peer". Ze has sent this peer none: the SCCRQ arrived before any tunnel existed and ze answers before allocating one. The AVP is mandatory in a StopCCN (Section 6.4) and the same section requires it to be non-zero, so every available value is a value the RFC's sentence does not sanction. Ze sends `tidNoTunnel` (0xFFFF) and `allocateLocalTID` never hands that value to a real tunnel, so the peer cannot reach a stranger's tunnel by answering it. The alternative is to allocate a real local TID, which is exactly the state the parse-before-lock ordering exists to avoid and would make the flood path an allocation path. This is a MUST that cannot be satisfied at this point in the exchange, stated here rather than annotated: no `{gap}`, `partial` or `{not-applicable}` was written.
- A source that owns a live tunnel is bounded like any other, so a peer that shares a busy slot loses one diagnostic StopCCN per interval. The exemption that would spare it is keyed on a source address an attacker can spoof, which is why there is none.
- The slot hash carries no per-process seed, so an attacker who computes it can share a chosen peer's slot. The effect is fail-closed: that peer loses its StopCCN and falls back to the retransmit-and-time-out behaviour that existed before this spec. A seed would make the collision unpredictable and is one line, but the fix does not need it.
- The SCCRP half of `RFC2661-24.10-1` tears its dialed tunnel down with StopCCN Result Code 1, not 2. Result Code 1 is RFC-legal, and the summary cell now says so rather than implying 2 for both halves.
- Three RFC section misattributions were written by this spec and corrected inside it: the `.ci` docstring, `internal/component/l2tp/errors.go` line 2 (`5.1` where the file cites 5.3), and `internal/component/l2tp/reactor.go` line 2 (5.3 missing). The docstring in `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` is the third: the hook that guards RFC-tagged tests reads a `"""` docstring line as behaviour bytes, so its correction needs an `rfc-test-change-approved` marker. The working tree carries that marker naming Thomas and dated 2026-08-10. **Whoever closes this spec must confirm that approval is real before commit A carries the file**: a marker is an assertion, and nothing in the tree proves it was given.
- FIXED in round 4, not deferred: `tunnel_fsm.go`'s `S5.12` and `S5.1.2`. Round 4 also claimed it had fixed every sibling of the same class in the six files it touched; round 5 found 31 more and fixed those, so read that claim as false and this one as measured against the convention in the residual bullet below. Round 3 deferred two of them on the grounds that the operator-visible error string needed a pass over the tests that read it. No test asserts either string (`grep` over `internal/component/l2tp` and `test/l2tp`), so that reason did not exist. The wording is unchanged: "at least one octet" and the RFC's "one or more octets of random data" state the same bound, and the `len(value) == 0` rejection is correct under either.
- 49 out-of-range RFC 2661 section citations remain in the L2TP package, in 16 files. Fourteen of them this spec never touched. The other two it did edit, for a different class, and they hold 21 of the 49 anchors: `session_fsm.go` 18 and `session_initiator.go` 3. The rest are `genl_linux.go` 6, `session.go` 4, `ppp/proxy.go` 3, and 11 others with 1 or 2 each. Round 4's "51 in 15" was measured under no stated convention and counted five RFC 2865 / 2866 / 6911 anchors as RFC 2661. **The convention round 5 counts under, and the one the journal row now records:** the population is every tracked file under `internal/component/l2tp/` recursively, `yang/` and the `ppp/` subpackage included; a hit is an anchor that names RFC 2661 explicitly, matching `RFC 2661[ ,]*(Sections?|S|§) N[.N...]`; a bare `N` is normalised to `N.0` because RFC 2661 numbers its top-level sections `1.0` through `13.0`; a hit is out of range when the normalised number heads no line of `rfc/full/rfc2661.txt`. Under that convention the package held 224 attributed anchors before round 6 and holds 226 after it, with 49 out of range throughout. Round 6 moved the denominator and not the residual, which is the whole point of the measurement's limits: `S9.5` for the Tie Breaker names a real section (Proxy PPP Authentication) so the count never saw it, and "RFC 2661 trap 24.4" names no section at all so the count did not see that either until round 6 gave it S3.1. An anchor that is IN range but names the wrong section is invisible to this count, which is the defect rounds 5 and 6 fixed and the reason the measurement alone is not a health check. All 49 predate this spec, no path it fixes depends on one, and the journal row carries the measurement so the class earns a deliberate pass rather than an opportunistic one.
- APPLIED, not blocked: `test/l2tp/rfc2661-emitted-control-shape.ci` cited "Section 4.2" for the CHAP-MD5 formula inside the embedded Python docstring, where the correct section is 4.4.3. The hook that guards RFC-tagged tests reads a `"""` line as behaviour bytes, so the edit needed an owner marker, which round 6 did not have. Thomas then approved it and the main thread applied it: the docstring reads 4.4.3 at line 37 and the file carries `rfc-test-change-approved: Thomas, 2026-08-10` at line 39. The same file's `#`-comment site needed no marker, because the hook never blocks a comment edit and a `"""` line in a `.ci` is the one shape it cannot read as a comment. That is the same limitation the `.ci` docstring bullet above records, met a second time in a second file.
- `rfc/short/rfc2661.md`'s `RFC2661-9-1` and the code it gates now disagree on the section again, and this is round 5's territory rather than this round's. The requirement TEXT is anchored to §5.7, which states the sender side and states it as a permission: a peer "may shut down" the control connection "by sending the StopCCN". The obligation is receive-side and §6.4 states it verbatim, which is why round 6 moved both code sites to §6.4 (finding 15). Round 5 chose §5.7 for the summary deliberately, as one of the nine frozen ids, so the disagreement is recorded here and left. Whoever fixes the frozen-id class re-anchors the TEXT to §6.4.
- `_rfc_tagged_change_err` in `.claude/hooks/pretool-writeedit.py` reads its `new` argument as the WHOLE file content for the `Write` tool and as the joined hunks for `Edit` / `MultiEdit`. Its approval test is `_RFC_APPROVED.search(new)`. So a committed `rfc-test-change-approved` marker inside `rfc2661-emitted-control-shape.ci` permanently exempts that file from every later whole-file `Write`, whatever the overwrite does to its assertions, while `Edit` and `MultiEdit` stay guarded because the marker has to sit in the hunk itself. Pre-existing and another session's surface: it is recorded here so the next author knows the marker is broader than the one change it was given for.
- Found on the way, not fixed, one journal row in `plan/journal/index-key-derived-from-mutable-field.md`: `discardTunnelLocked` deletes from `tunnelsByPeer` with a key built from the mutable `t.peerAddr`, which `handle` overwrites on every inbound datagram, so a peer that changes source port leaks a stale entry. It does not touch the path this spec fixes. Round 5 corrected the reason the row gave: that overwrite is a TOLERANCE of a peer that moves, not an RFC 2661 Section 8.1 requirement. Section 8.1 says the opposite, that the ports and addresses "MUST remain static for the life of the tunnel". The defect is unchanged, and it is now worse-founded than the row claimed: ze tracks a movement the RFC forbids and keys a map on the result.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Lesson recorded. `ai/rules/planning.md` retired the `plan/learned/NNN-<name>.md` form: the record is a row in `plan/journal/<class>.md`, and this spec wrote two (`gate-excludes-part-of-its-population.md`, `reference-checked-claim-unchecked.md`). Both go in the commit A `--file` list
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `answerZeroTunnelIDSCCRQ` (`internal/component/l2tp/reactor.go`): the bounded answer. One slot per source address, hashed FNV-1a into the fixed 256-entry `L2TPReactor.stopCCNLastSent`, one emission per slot per second, no exemption for any source. It is called from the `perr != nil` branch of `handle`, on `errZeroAssignedTunnelID` only, before `tunnelsMu` is taken.
- `sendUnassociatedStopCCN` (`internal/component/l2tp/reactor.go`): the first StopCCN in ze that needs no tunnel. Header Tunnel ID 0 and Ns 0 per RFC 2661 Section 4.4.3, Nr the peer's Ns plus one per Section 5.8, body from the existing `writeStopCCNBody`, out through `listener.Send`, recorded in both capture rings.
- `stopCCNSlot` and the `tidNoTunnel` (0xFFFF) reservation, which `allocateLocalTID` now skips so no real tunnel can hold the Assigned Tunnel ID a tunnel-free StopCCN carries.
- `errZeroAssignedTunnelID` (`internal/component/l2tp/errors.go`): the sentinel that separates the one answered parse failure from every other silent drop. `parseSCCRQ` returns it instead of an inline `errors.New`.
- `writeStopCCNBody` (`internal/component/l2tp/tunnel_fsm.go`) takes a `ResultCodeValue` instead of a bare `uint16`, so Result Code 2 can carry Error Code 3. `resultProtocolError` and `errorValueOutOfRange` are new named constants; `teardownStopCCN` keeps Result Code 1.
- Eight unit tests in `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` and one functional test, `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci`, whose Python peer hand-packs the SCCRQ and decodes the reply with `struct.unpack` against values transcribed from the RFC.

### Bugs Found/Fixed
- The spec's own subject: an SCCRQ with Assigned Tunnel ID 0 was dropped in silence while `rfc/short/rfc2661.md` claimed the reactor answered with StopCCN. Covered by `TestSCCRQWithZeroAssignedTunnelIDIsAnswered` and by the `.ci`.
- The first draft of the bound exempted a source that owned a live tunnel. The exemption is keyed on a spoofable address, so it removed the ceiling for exactly the addresses ze most wants to protect. Removed in round 2. Covered by `TestZeroIDSCCRQFromTunnelOwnerIsBounded`.
- `internal/component/l2tp/reactor.go` cited "RFC 2661 Section 15" for the HELLO keepalive. RFC 2661 ends at Section 13; the keepalive is Section 5.5. Corrected in round 3. No test covers a comment, which is the point of the journal row.
- Round 4 traced that class to its supplier and cleared it from the six files this spec touches: `rfc/short/rfc2661.md` had copied `docs/research/l2tpv2-implementation-guide.md`'s numbering, which runs to Section 26 against the RFC's 13. Five summary headings, the trap table's five rows and eight checklist anchors corrected, then 36 source locations, three of them inside returned error strings.
- Found in round 4 and fixed: `tunnel_initiator.go` carried a systematically shifted message-to-section map. Its header read "6.2 (SCCCN), 6.4 (SCCRP)" and four sites followed it, one of them the error string `"SCCRP missing Assigned Tunnel ID AVP (RFC 2661 S6.4)"`. RFC 2661 Section 6.2 is SCCRP, 6.3 is SCCCN and 6.4 is StopCCN. These numbers resolve, so a reader who followed one landed on real text about the wrong message.
- Found and NOT fixed, each with a journal row: the `tunnelsByPeer` key derived from the mutable `t.peerAddr` (`plan/journal/index-key-derived-from-mutable-field.md`), the `unused` reds on `writeAVPUint8` and `readAVPUint8` left by the unexport refactor (`plan/journal/bulk-rename-corruption.md`), and the 49 out-of-range section citations left in 16 L2TP files this spec never touched (`plan/journal/reference-checked-claim-unchecked.md`). The two `tunnel_fsm.go` sites that bullet used to name are fixed.

### Documentation Updates
- `rfc/short/rfc2661.md`: the trap table's Assigned-Tunnel-ID-0 coverage cell states both halves separately. The `RFC2661-24.10-1` requirement line keeps its id, its `[MUST]` level and its `(§24.10)` anchor. Round 4 added the RFC's real sections to its TEXT, which `ai/rules/rfc-compliance.md` permits ("correcting a misquote means editing the TEXT under the same id"), because `_validate_id` refuses any line whose id and anchor disagree and `check_retired_requirements` refuses a vanished id. Round 4 also re-anchored the file's five guide-numbered headings, the trap table's five rows and the other seven guide-derived checklist anchors.
- `rfc/requirements/rfc2661.md`: regenerated by `make ze-rfc-index`, and already at HEAD when this spec closed. The `RFC2661-24.10-1` row carries the SCCRQ unit tag and the `.ci` tag beside the two untouched SCCRP tags, in both polarities.
- `docs/**` untouched, by direction: a concurrent session holds that tree. `grep -rn -i "tunnel id 0\|assigned tunnel id" docs/guide/l2tp.md` returns nothing, so no sentence in the guide becomes false.
- `make ze-doc-test` not run here: no `docs/`, `ai/` or `plan/` doc surface changed except this spec and the journal rows. Closure runs it.

### Deviations from Plan
- Implementation Steps 2 and 5 name `test/draft/l2tp/` as the incubator. `test/draft/` is gitignored, so the reproducer was written straight into `test/l2tp/` beside `rfc2661-emitted-control-shape.ci` and proved by reverting the behaviour rather than by an incubation period. Recorded in the Discrimination proof table.
- The spec's Closure checklist named `plan/learned/NNN-<name>.md`. `ai/rules/planning.md` retired that form: the record is a journal row, and this spec wrote two.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5 assumed a StopCCN reply is not larger than the SCCRQ that triggers it | 28 bytes in, 38 bytes out: 1.36x at the L2TP layer, 1.14x as Ethernet frames | measured against a running daemon | the per-source bound is what makes the amplification irrelevant, and the threat model now states both layers |
| approach | The bound exempted a source that owned a live tunnel, to spare a real peer its diagnostic | The exemption is keyed on a source address nothing has proved return-routable, so an attacker naming any current peer reinstates an unbounded ceiling aimed at that peer | review round 2 | exemption deleted, `TestZeroIDSCCRQFromTunnelOwnerIsBounded` pins its absence, and the O(live tunnels) walk under `tunnelsMu` left the flood path with it |
| escalation | Three RFC section numbers were written wrong by this spec and three more were found already in the package | No gate reads a section number in a `// RFC:` header or an `RFC NNNN Section X` comment; only the path beside it is resolved | round 3 review, then reading `rfc/full/rfc2661.txt` | corrected what this spec wrote, recorded the rest. Round 4 then found the count was far low and the diagnosis incomplete: the SUPPLIER is `rfc/short/rfc2661.md`, which had copied the implementation guide's numbering. 36 locations fixed, 51 measured and left, in `plan/journal/reference-checked-claim-unchecked.md`. Round 5 then found the diagnosis STILL incomplete: 31 further wrong anchors in the same files, every one numerically in range and therefore invisible to the count round 4 published. The residual is 49 in 16 files under a stated convention. Mechanising the check is a spec of its own, now written: `plan/future/spec-rfc-anchor-resolution-check.md` |
| approach | Round 3 deferred two citation fixes because the operator-visible error string "needs its own pass over the tests that read it" | No test asserts either string. The deferral had no cause, and the strings' meaning is unchanged by the fix | round 4 review ran the grep round 3 asserted the need for | both fixed. Round 4 claimed "every sibling of the same class in the six files this spec touches" with it; round 5 falsified that claim by finding 31 more, and the claim is corrected here rather than left standing |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The owner decides the route before any code lands | Done | "The owner question", Key Design Decisions | Route C, answered 2026-08-10 |
| An SCCRQ with Assigned Tunnel ID 0 is answered with StopCCN | Done | `answerZeroTunnelIDSCCRQ`, `sendUnassociatedStopCCN` (`internal/component/l2tp/reactor.go`) | Result Code 2, Error Code 3 |
| The emission is bounded per source, with no exception | Done | `stopCCNSlot` and `stopCCNLastSent` (`internal/component/l2tp/reactor.go`) | 256 slots, one second |
| The threat model is part of the deliverable | Done | "Threat model of the bound" | limit, attacker gain, ze's cost, and what a legitimate peer loses |
| The SCCRQ half carries a tagged test in both polarities | Done | `reactor_sccrq_zero_tid_test.go:75` and `:117`, `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci:21` and `:24` | id unchanged, SCCRP tags untouched |
| No `{gap}`, `partial` or `{not-applicable}` is written | Done | the `RFC2661-24.10-1` line in `rfc/short/rfc2661.md` | id, `[MUST]` level and `(§24.10)` anchor unchanged; no annotation added. The text gained the RFC's real sections in round 4 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Key Design Decisions row 1, "The owner question" | Route C, both rejected routes recorded with their reasons |
| AC-2 | Done | `TestSCCRQWithZeroAssignedTunnelIDIsAnswered` PASS, `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` PASS (17/22 in `make ze-l2tp-test`) | the `.ci` reads Result Code 2 and Error Code 3 off the wire with `struct.unpack` |
| AC-3 | N-A | -- | Route B was refused |
| AC-4 | Done | four `RFC requirement:` tags, two on the SCCRQ path and two untouched on SCCRP | `grep -rn "RFC2661-24.10" internal/component/l2tp/ test/l2tp/` |
| AC-5 | Done | `make ze-rfc-check` exit 0 after `make ze-rfc-index`, round 4 on 2026-08-10: 2963 gated MUSTs across 170 enrolled RFCs, 3338 tags resolved | the run before the regeneration was exit 2 on the stale ledger alone, and the ledger went stale again inside ten minutes. Re-run both at commit prep: the ledger is shared and moves under other sessions |
| AC-6 | Done | `TestZeroIDSCCRQFloodAllocatesNoTunnel` PASS | 50 datagrams, one reply, both tunnel maps empty |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSCCRQWithZeroAssignedTunnelIDIsAnswered` | Done | `internal/component/l2tp/reactor_sccrq_zero_tid_test.go:81` | negative polarity tag at `:75` |
| `TestSCCRQWithNonZeroAssignedTunnelIDEstablishes` | Done | `:123` | positive polarity tag at `:117` |
| `TestZeroIDSCCRQFloodAllocatesNoTunnel` | Done | `:150` | AC-6 |
| `TestZeroIDSCCRQAnsweredAgainAfterInterval` | Done | `:187` | the window boundary, driven by the injected clock |
| `TestZeroIDSCCRQFromTunnelOwnerIsBounded` | Done | `:219` | pins the absence of the exemption |
| `TestUnassociatedStopCCNIsCaptured` | Done | `:257` | both capture rings |
| `TestAllocateLocalTIDSkipsTidNoTunnel` | Done | `:297` | the reserved 0xFFFF |
| `TestStopCCNSlotInRange` | Done | `:320` | v4 and v6 extremes |
| `rfc2661-sccrq-tunnel-id-zero` | Done | `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` | both polarities against one daemon |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/l2tp/reactor.go` | Done | the answer branch, four new symbols, the `tidNoTunnel` skip, the corrected `// RFC:` header |
| `internal/component/l2tp/tunnel_fsm.go` | Done | the sentinel, `resultProtocolError`, `errorValueOutOfRange`, `writeStopCCNBody` signature. Round 6 added 5.7 to the `// RFC:` header, which `handleStopCCN` cites |
| `internal/component/l2tp/errors.go` | Done | `errZeroAssignedTunnelID` and the corrected `// RFC:` header |
| `rfc/short/rfc2661.md` | Changed | round 3 changed the coverage cell only; round 4 also corrected the file's guide-derived section numbers, its root cause |
| `rfc/requirements/rfc2661.md` | Done | regenerated, never hand-edited. Already correct at HEAD, so it rides on no commit here |
| `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` | Changed | eight tests, not the seven the plan named: `TestStopCCNSlotInRange` was added when the slot hash appeared |
| `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` | Done | written in `test/l2tp/`, not the gitignored `test/draft/` (Deviations) |
| `plan/journal/index-key-derived-from-mutable-field.md` | Done | one row, a defect found on the way |
| `plan/journal/gate-excludes-part-of-its-population.md` | Changed | added at closure: the lesson this spec produced |
| `plan/journal/reference-checked-claim-unchecked.md` | Changed | added at closure: the section numbers no gate reads |
| `plan/journal/bulk-rename-corruption.md` | Changed | one row, the `unused` reds another spec owns |
| `internal/component/l2tp/tunnel_initiator.go` | Changed | not in the plan: round 4 corrected nine RFC section citations, one inside a returned error string. Round 6 added 4.4.1 and 4.4.3 to the `// RFC:` header, both cited by sites round 5 added |
| `internal/component/l2tp/tunnel.go` | Changed | not in the plan: round 4 corrected five RFC section citations |
| `internal/component/l2tp/reactor_dial.go` | Changed | not in the plan: round 4 corrected three RFC section citations |
| `internal/component/l2tp/reactor_test.go` | Changed | not in the plan: rounds 4 and 5 corrected seven RFC section citations in comments (`S4.1`, `S24.19`, `S4.2`, `S9.5`, `S5.12`, and `S4.4.2` twice). No assertion changed and neither RFC-tagged test in the file was touched |
| `internal/component/l2tp/session_fsm.go` | Changed | not in the plan: round 5, six error strings citing S4.1 for the Message Type ordering rule |
| `internal/component/l2tp/session_initiator.go` | Changed | not in the plan: round 5, one doc comment and one error string, same rule |
| `internal/component/l2tp/reliable.go` | Changed | not in the plan: round 5, three sites, one of them a quotation the RFC does not contain. Round 6 added 4.4.1 to the `// RFC:` header, which two of those sites cite, and replaced two "RFC 2661 trap 24.4" citations with S3.1, adding 3.1 to the header |
| `internal/component/l2tp/reliable_test.go` | Changed | not in the plan: round 6, two comments. `TestOnReceiveDataMessage` cited "RFC 2661 trap 24.4" and `TestTickRetransmit` cited `S24.9`; both are implementation-guide trap numbers. No assertion changed and no `RFC requirement:` tag was touched |
| `internal/component/l2tp/auth.go` | Changed | not in the plan: round 5, the header and three sites citing S4.2 for the Challenge Response construction. Round 6 gave the header's 5.1.1 a site: `ChallengeResponse`'s precondition cited a Section 5.12 RFC 2661 does not have, and now names 4.4.3 for the Challenge and 5.1.1 for the shared secret |
| `internal/component/l2tp/auth_test.go` | Changed | not in the plan: round 5, four `//nolint` and comment sites, same rule. No assertion changed |
| `internal/component/l2tp/config.go` | Changed | not in the plan: round 5, the header and the `SharedSecret` doc comment. Round 6 dropped 6.1 from the header: no site in the file cites it, and the file's RFC content is the shared secret (5.1.1). `tunnel_initiator.go` builds the SCCRQ and its own header names 6.1 |
| `internal/component/l2tp/reactor_ppp_linux_test.go` | Changed | not in the plan: round 5, one `require` message. The assertion is unchanged |
| `internal/component/l2tp/yang/ze-l2tp-conf.yang` | Changed | not in the plan: round 5, two operator-visible `description` texts citing S4.2 for the CHAP-MD5 shared secret. Descriptions only; no leaf, type or constraint changed |
| `internal/component/l2tp/reactor_dial_test.go` | Changed | not in the plan: round 6, one comment citing `S9.5` for the Tie Breaker. §9.5 is Proxy PPP Authentication; the rule is §4.4.3. The sibling `reactor_dial.go` was corrected in round 5 and the test was not. The file carries no `RFC requirement:` tag |
| `test/l2tp/rfc2661-emitted-control-shape.ci` | Changed | not in the plan: round 6, one `#` comment citing §4.2 for the CHAP-MD5 formula. A second site in the Python docstring is BLOCKED by the RFC-tagged-test hook and needs an owner marker; see the round 6 finding |
| `plan/future/spec-rfc-anchor-resolution-check.md` | Changed | created by round 5: the spec that would end this class. Round 6 rewrote its one-spec argument, which was wrong about how (a) and (b) couple |

### Audit Summary
- **Total items:** 48 (6 requirements, 6 AC, 9 tests, 27 files)
- **Done:** 26 (6 requirements, 5 AC, 9 tests, 6 files)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 21 (three journal files this spec appends a row to rather than creates, the test file that gained an eighth test, `rfc/short/rfc2661.md` which round 4 corrected beyond the planned coverage cell, four source files round 4 corrected citations in that the plan never named, nine more round 5 corrected including the YANG module, the `plan/future/` spec round 5 wrote, and three more round 6 corrected: `reactor_dial_test.go`, `reliable_test.go` and `test/l2tp/rfc2661-emitted-control-shape.ci`). AC-3 is N-A because Route B was refused, and is not counted above. 26 + 21 + 1 = 48.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `RFC2661-24.10-1` is met on the SCCRQ half, not only on SCCRP | functional | `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci`, PASS as 17/22 in `make ze-l2tp-test` (20/20, 2 skipped for unrelated reasons). A Python peer sends a hand-packed SCCRQ over UDP and asserts message type 4, Result Code 2, Error Code 3, header Tunnel ID 0, Ns 0, Nr 1 and a non-zero Assigned Tunnel ID, all with `struct.unpack` |
| The published claim no longer stands ahead of its evidence | functional and ledger | the trap table in `rfc/short/rfc2661.md` states both halves; the `RFC2661-24.10-1` row in `rfc/requirements/rfc2661.md` carries functional/verify evidence in both polarities beside the two unit ones; `make ze-rfc-check` exit 0 on 2026-08-12 with no `rfc2661` violation. That row cited two files git did not hold until this commit lands them, so the ledger stood ahead of its own evidence in exactly the way this spec exists to fix. Commit A is what repairs it |
| The emission cannot make ze a reflector | unit | `TestZeroIDSCCRQFloodAllocatesNoTunnel` (50 datagrams, one reply), `TestZeroIDSCCRQFromTunnelOwnerIsBounded` (no exemption), `TestZeroIDSCCRQAnsweredAgainAfterInterval` (the window reopens, so the bound is not a latch), `TestStopCCNSlotInRange` (every address lands in the table) |
| The parse-before-lock ordering survives | unit | `TestZeroIDSCCRQFloodAllocatesNoTunnel` asserts both tunnel maps are empty after the flood. `answerZeroTunnelIDSCCRQ` is called inside the `perr != nil` branch, which returns before `tunnelsMu` |
| Each behaviour would fail if reverted | discrimination | the "Discrimination proof" table: five behaviours reverted, the failing test named for each, with the failure text for two of them |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none. The `Deferral shard` field is `-` and no shard was opened | done | Every defect found on the way is a committed journal row, not a deferral: `index-key-derived-from-mutable-field.md`, `bulk-rename-corruption.md`, `reference-checked-claim-unchecked.md` |
| the FOREIGN shard `plan/deferrals/rfcgate-2-deferred-nonunit-evidence-backfill.md`, row 4: "Fix RFC2661-24.10-1", homed at this spec by PATH | done | This spec IS that row's work. Set to `done` at closure and the Destination repointed to the bare stem `spec-fixit-l2tp-sccrq-tunnel-id-zero`, because commit B deletes the path it cited. The shard still holds four live rows owned by other specs, so it is NOT removed |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-l2tp-sccrq-tunnel-id-zero-640fa955-f03a-45e8-a58f-4b367f5859e6.md`, recorded at closure over the final diff, `verdict=clean` |
| `review_gate.py check` | PASS, hash-pinned and clean |
| Rounds | 8. Round 7 was a full independent re-review (coherence and wiring, RFC conformance and comment accuracy, test discrimination and fake-leniency); round 8 verified its two fixes. Rounds 1 to 6: Round 1 the whole diff, round 2 the bound's exemption and the citations, round 3 the refusal path's stated cost and the record staleness, round 4 the citations' supplier and the records' arithmetic, round 5 the in-range wrong anchors round 4's own sweep left, round 6 the header invariant round 5 broke on both sides while writing it, three surviving sites of round 5's own classes, three more citing an implementation-guide TRAP number no count can see, and the records' arithmetic. Each later round covered only what the previous round's fixes touched. The `--rounds-reason` stays round 4's: the product defect is the same class, false RFC section citations in shipped source with some inside returned error strings, and it survived a dedicated sweep and then a second one inside that sweep's own files. That strengthens the reason again rather than weakening it. Round 7 raised the same class once more, in a comment this spec's own behaviour change introduced: `answerZeroTunnelIDSCCRQ`'s doc claimed the path "takes no lock and walks no table", which is true only of the suppressed branch. Round 8 confirmed the correction and found nothing further |
| Reviewer lenses used | security and amplification (the bound and its exemption), RFC conformance (the requirement text against the tags), evidence and prose accuracy (what the comments and the spec claim against what the code does) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | the tunnel-owner exemption is keyed on a spoofable address, so the ceiling disappears for any address an attacker names | `answerZeroTunnelIDSCCRQ` (`internal/component/l2tp/reactor.go`) | exemption deleted, `TestZeroIDSCCRQFromTunnelOwnerIsBounded` added, Key Design Decisions records why |
| 2 | ISSUE | the refusal path's stated cost was false in a shipped comment, in the threat model and in the Architectural Verification table: it logged at Debug with `from.String()` on top of the sibling log in `handle`, about 2 allocations and 196 ns per refused datagram | `answerZeroTunnelIDSCCRQ`, and the two spec tables | the Debug call is deleted, the comment now says the path takes no lock and walks no table, and both spec claims are corrected to match. Round 3 |
| 3 | ISSUE | `internal/component/l2tp/errors.go:2` cited Sections 3.1, 4.4.3, 5.1 while nothing in the file cites 5.1 and its one doc comment cites 5.3, and `reactor.go:2` omitted 5.3 | both file headers | errors.go now reads 3.1, 4.4.3, 5.3; reactor.go gained 5.3 and 5.5. Round 3 |
| 4 | NOTE | the spec header said Phase 1/5 and AC-5 reported an `internal/component/ike/dataplane` violation another session has since repaired | spec metadata and AC-5 | corrected against a fresh `make ze-rfc-check` run. Round 3 |
| 5 | NOTE | the ceiling named no layer | "Threat model of the bound" | every rate now names its layer: 77.8 kbit/s of L2TP payload, 172 kbit/s as Ethernet frames. Round 3 |
| 6 | ISSUE | the citation defect had no traced cause. `rfc/short/rfc2661.md` took its headings and eight checklist anchors from `docs/research/l2tpv2-implementation-guide.md`, whose numbering runs to Section 26 against the RFC's 13, and every author copied them onward | `rfc/short/rfc2661.md` | five headings, the trap table's five rows and eight checklist anchors corrected against `rfc/full/rfc2661.txt`. `make ze-rfc-check` exit 0 after `make ze-rfc-index`. Round 4 |
| 7 | ISSUE | false RFC section citations in shipped source, three inside returned operator-visible error strings. Round 3 deferred two on a reason that did not exist | six files under `internal/component/l2tp/` | 36 locations corrected. The six files carry no OUT-OF-RANGE RFC 2661 section number. Round 4 read that as "clean"; finding 13 shows it is a weaker property than it sounds, because an in-range wrong number satisfies it. Round 4 |
| 8 | ISSUE | `reactor.go`'s header cited 9.5 (Proxy PPP Authentication), which no site uses, and 5.1, which no site cites | `internal/component/l2tp/reactor.go`, its `// RFC:` header | 9.5 and 5.1 dropped, 7.2 and 8.1 added. Every number in the header now resolves to a cited site in the file, which is the property that makes it checkable. 5.1 is not false (it describes the SCCRQ/SCCRP/SCCCN exchange) but nothing in the file cites it, and an unsupported number is how the guide numbering spread. Round 4 |
| 9 | ISSUE | `tunnel_fsm.go`'s header omitted 5.5, 6.4 and 6.5, which the file relies on | `internal/component/l2tp/tunnel_fsm.go`, its `// RFC:` header | header reads 4.4.1, 4.4.2, 4.4.3, 5.1.1, 5.5, 6.1, 6.4, 6.5, each resolving to a site. Round 4 wrote 4.1 there; round 5 corrected it to 4.4.1 with the sites it names. Round 4, amended round 5 |
| 10 | ISSUE | the `index-key-derived-from-mutable-field` journal row cited "RFC 2661 Section 24.19" for source-port variation | `plan/journal/index-key-derived-from-mutable-field.md` | corrected to Section 8.1, which states the peer picks any free port. Round 4 |
| 11 | ISSUE | AC-5 asserted `make ze-rfc-check` exit 0 in the present tense over a shared, moving ledger | AC-5 and its two verification rows | all three now state the measurement, name `make ze-rfc-index` as a commit-prep obligation, and record the concurrent churn as R-5 materialising. Round 4 |
| 12 | ISSUE | the Audit Summary read `Done: 28` against a 32-item decomposition that yields 27 | "Audit Summary" | recomputed. 36 items after round 4 (four files round 4 touched that the plan never named): 26 Done, 9 Changed, 1 N-A. Round 5 then found this row's own copy of the result read "27 Done, 8 Changed" against the Audit Summary's 26 / 9 for the same recomputation, and corrected the row. The current figures are round 5's: 45 items, 26 Done, 18 Changed, 1 N-A |
| 13 | ISSUE | `§4.1` was the wrong section for "the Message Type AVP MUST be first" at nine sites round 4's own sweep left, two of them inside returned operator-visible error strings. RFC 2661 states the rule in §4.4.1, under the Message Type AVP of "AVPs Applicable To All Control Messages" | `tunnel_fsm.go`, `tunnel_initiator.go`, `reactor_test.go`, `rfc/short/rfc2661.md` | corrected, and the same defect fixed at the twelve further sites the review had not enumerated: `session_fsm.go` x6, `session_initiator.go` x2, `reliable.go` x3, `reactor_ppp_linux_test.go`. `RFC2661-4.1-2` is the ninth frozen id and took the same TEXT route: the requirement now quotes §4.4.1 and says the anchor is frozen. Round 5 |
| 14 | ISSUE | `protocolVersionValue` cited §4.4.2 (Result and Error Codes). The Protocol Version AVP opens §4.4.3, Control Connection Management AVPs | `tunnel_fsm.go` | corrected. Round 5 |
| 15 | ISSUE | the StopCCN teardown rule was cited to §4.4.2 at both `teardownStopCCN` and `handleStopCCN`, so the code disagreed with `RFC2661-9-1`, which cites §5.7. The RFC states it in §6.4: "all active sessions are implicitly cleared", and §5.7 gives the procedure | `tunnel_fsm.go` | both sites now cite §6.4, and `teardownStopCCN`'s doc comment splits the two claims: the Result Code AVP is §4.4.2, the message is §6.4. Round 5 |
| 16 | NOTE | the Challenge Response construction was cited to §4.2 (Mandatory AVPs). The construction, and the rule that the CHAP ID is "the value of the Message Type AVP for this message", are in §4.4.3 under the Challenge Response AVP, mandated by §5.1.1 | `auth.go`, `auth_test.go`, `config.go`, `reactor_test.go` | ten sites corrected. The review named one; the other nine are the same defect in the same package. Round 5 |
| 17 | NOTE | `FramingCapabilities` said "bit 0 = async, bit 1 = sync". The §4.4.3 diagram draws the low two bits `\|A\|S\|`, so S is bit 0 | `tunnel_fsm.go` | comment corrected. Non-behavioural: the sender writes `0x3` and advertises both. Round 5 |
| 18 | NOTE | `reactor.go` put quotation marks round text RFC 2661 does not contain: "both peers' tunnels MUST be silently torn down". §4.4.3 says "both sides MUST discard their tunnels" | `reactor.go` | quoted correctly. `reliable.go`'s "Message Type AVP MUST be first" had the same defect and now carries the RFC's own sentence. Round 5 |
| 19 | ISSUE | the residual was published as "51 in 15 files" under no stated convention, and five of the counted hits were RFC 2865 / 2866 / 6911 anchors | "Known Limitations" | re-measured under a convention the limitation now states: 49 out of range among 223 attributed RFC 2661 anchors, in 16 files. Round 5 |
| 20 | ISSUE | `reactor_dial.go`'s header cited §6.1, which no site in the file cites, while §4.4.3, §5.1.1 and §5.3 are cited and absent from it | `internal/component/l2tp/reactor_dial.go`, its `// RFC:` header | header now reads 4.4.3, 5.1.1, 5.3. Round 4 dropped §5.1 from `reactor.go` on the same principle, and round 5 states it as a rule rather than a local justification: every number in a `// RFC:` header resolves to a cited site in that file. §6.1 governs `tunnel_initiator.go`, which builds the SCCRQ and whose own header names it; the `Related:` line already routes a reader there. Round 5 |
| 21 | ISSUE | `tunnel.go` and `reactor.go` cited §8.1 as the AUTHORITY for updating `peerAddr` on every inbound datagram. §8.1 says the opposite: once the ports and addresses are established they "MUST remain static for the life of the tunnel" | `tunnel.go` x2, `reactor.go`, `reactor_test.go`, `plan/journal/index-key-derived-from-mutable-field.md` | all five now say ze's tracking is a TOLERANCE of a peer that moves, and name §8.1 as the requirement it tolerates a breach of. The journal row's inference rested on the same misreading; the defect it records is unchanged and its footing is worse than the row claimed. Round 5 |
| 22 | ISSUE | the same §4.2 misattribution reached two operator-visible YANG `description` texts for the CHAP-MD5 shared secret, which no reviewer had enumerated | `internal/component/l2tp/yang/ze-l2tp-conf.yang` | both now cite §5.1.1, the subsystem-level one naming §4.4.3 for the AVPs. Descriptions only. `make ze-cli-grammar-check` and `make ze-doc-test` exit 0. Round 5 |
| 23 | ISSUE | `writeStopCCNBody` and `buildStopCCN` cited §4.4.2 for the LIST of AVPs a StopCCN must carry. §4.4.2 defines the Result Code AVP; the required-AVP list is §6.4 | `tunnel_fsm.go`, `reactor_test.go` | both corrected, each now naming §6.4 for the list and §4.4.2 for the Result Code AVP. Found by a post-fix sweep of every remaining §4.4.2 and §4.2 site in the package, not by the review. Round 5 |
| 24 | ISSUE | round 5 broke the header rule while writing it. `auth.go`'s header named §5.1.1 with no site to support it, ADDED in the same sweep that deleted §6.1 from `reactor_dial.go` for that defect. `config.go`'s header named §6.1 (SCCRQ dial target) with no site either, on a header line round 5 edited | `internal/component/l2tp/auth.go`, `config.go`, their `// RFC:` headers | `auth.go` gained the site: `ChallengeResponse`'s precondition had cited "RFC 2661 Section 5.12", a section RFC 2661 does not have, and now cites §4.4.3 for "The Challenge is one or more octets of random data" and §5.1.1 for the shared secret that MUST exist between LAC and LNS. `config.go` dropped §6.1: the file's RFC content is the shared secret, and `tunnel_initiator.go` builds the SCCRQ under its own header. Round 6 |
| 25 | ISSUE | the converse property, which round 4 enforced at finding 9, was broken by round 5's own new sites: `tunnel_fsm.go` omitted §5.7 (cited at `handleStopCCN`), `tunnel_initiator.go` omitted §4.4.1 and §4.4.3, `reliable.go` omitted §4.4.1 at two sites | three `// RFC:` headers | all three headers extended. The Design Insights bullet now states the invariant as two-sided and records that one sweep broke both halves of it. Round 6 |
| 26 | NOTE | three sites of round 5's own classes survived, invisible to the residual count because each anchor is IN range or carries no `RFC 2661` on its line: `reactor_dial_test.go` cited `S9.5` for the Tie Breaker (§9.5 is Proxy PPP Authentication) while its sibling `reactor_dial.go` was corrected; `test/l2tp/rfc2661-emitted-control-shape.ci` cited §4.2 for the CHAP-MD5 formula at two sites; `TestTickRetransmit` in `reliable_test.go` cited `S24.9` | `reactor_dial_test.go`, `rfc2661-emitted-control-shape.ci`, `reliable_test.go` | `reactor_dial_test.go` corrected to `S4.4.3`, the `.ci`'s `#` comment to §4.4.3, and `TestTickRetransmit` now names S5.8 alone with the RFC's own sentence. The `RFC2661-5.8-4` tags made no marker necessary: the hook never blocks a comment edit, and it was tried rather than assumed. The `.ci`'s docstring site is the one that IS blocked, because a `"""` line is not a shape the hook reads as a comment. Round 6 |
| 27 | NOTE | three record numbers were wrong: the residual bullet and the journal row said "223 attributed anchors" against a reproduction of 224 and both called the 49 residual "16 files this spec never touched" while 21 anchors sit in two files it edited; the Files-from-Plan row said `reactor_test.go` carried six corrected citations against seven in the diff; the journal row said eight frozen ids against the spec's nine | "Known Limitations", "Files from Plan", `plan/journal/reference-checked-claim-unchecked.md` | all three reconciled. 226 attributed anchors after round 6's own edits (224 before them), 49 out of range in 16 files of which 14 are untouched here; seven citations in `reactor_test.go`; nine frozen ids, three guide-derived and six naming a real section that states a different rule. Round 6 |
| 28 | NOTE | "RFC 2661 trap 24.4" appeared at three sites. The trap numbering is `docs/research/l2tpv2-implementation-guide.md`'s Section 24, the same supplier finding 6 traced, and it names no RFC section at all, so the residual count's regex cannot see it. The claim was also imprecise: RFC 2661 S3.1 makes Ns and Nr optional on data messages and required only on control messages, which is not the same as "reserved" | `reliable.go` twice, `reliable_test.go` once | all three now cite S3.1 with what the RFC states, and 3.1 is added to `reliable.go`'s `// RFC:` header. Found while checking the header invariant on `reliable.go`, not by the review. Round 6 || 29 | ISSUE | `answerZeroTunnelIDSCCRQ`'s doc comment asserted a concurrency property the code does not have: "It takes no lock and walks no table." True of the SUPPRESSED branch only. The emitting branch calls `sendUnassociatedStopCCN`, which takes `captureRing.mu` (`capture.go`, `appendOutbound`) and `RawCaptureRing.mu` (`raw_capture.go`, `Append`) whenever a capture is enabled | `internal/component/l2tp/reactor.go`, `answerZeroTunnelIDSCCRQ` doc comment | the comment now separates the two branches and names the two locks the emitting one takes. Code unchanged: the claim was wrong, not the behaviour. Round 7 |
| 30 | NOTE | AC-5, Goal Validation, R-5, Integration Points, Documentation Updates and Files-from-Plan all named `ai/RFC-REQUIREMENTS.md`, a ledger the repo has since sharded to `rfc/requirements/<stem>.md` | six sites in this spec | all six repointed to `rfc/requirements/rfc2661.md`, with one note at first use recording the rename. That shard is unmodified against HEAD, so nothing is regenerated here. Round 7 |
| 31 | NOTE | `writeStopCCNBody` (`internal/component/l2tp/tunnel_fsm.go`) documents "A caller that sends Result Code 2 MUST set ErrorPresent" and enforces nothing | `tunnel_fsm.go` | recorded, not changed. Both live callers set it today, and a contract stated in a doc comment is the shape the rest of this package uses. Round 7 |
| 32 | NOTE | the round-7 blocker, `make ze-rfc-check` red on `rfc/requirements/rfc5301.md is stale vs its sources`, was never this spec's | the shared checkout | cleared by the IS-IS session (`cc6faee7e`, `2da136a0f`) and a `make ze-rfc-index` run outside this spec. Re-measured at closure on 2026-08-12: exit 0, 2963 gated MUSTs across 170 enrolled RFCs, 3341 tags resolved, no `rfc2661` violation. Round 8 |
| 33 | NOTE | `rfc/requirements/rfc2661.md` at HEAD already cited `reactor_sccrq_zero_tid_test.go` and `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci`, two files git did not hold, so the ledger stood ahead of its evidence: this spec's own failure class, reproduced in the ledger | `rfc/requirements/rfc2661.md` at HEAD | repaired by commit A landing both files. No ledger edit is needed or made. Round 7 |


## Pre-Commit Verification

**No test suite ran during closure, by direction.** A QEMU run held the machine,
so `make ze-verify`, `make ze-plugin-test` and every Docker or `bin/ze-test`
target were forbidden for this session. `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci`
is therefore UNRUN here: its recorded PASS is the implementation session's run of
2026-08-10, quoted below, and closure did not reproduce it. What closure did run
is `make ze-rfc-check` (exit 0), `make ze-lint-changed` and, after the commit
script, `make ze-tracked-build-check`. The closing commit body says the same.


### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/reactor_sccrq_zero_tid_test.go` | Yes | `ls -l` on 2026-08-10: 12863 bytes |
| `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` | Yes | `ls -l` on 2026-08-10: 8372 bytes |
| `plan/journal/index-key-derived-from-mutable-field.md` | Yes | `ls -l` on 2026-08-10: 873 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the owner answered Route C | `grep -n "ANSWERED 2026-08-10" plan/spec-fixit-l2tp-sccrq-tunnel-id-zero.md` resolves in "The owner question" |
| AC-2 | a zero Assigned Tunnel ID SCCRQ draws StopCCN Result Code 2 | `--- PASS: TestSCCRQWithZeroAssignedTunnelIDIsAnswered (0.00s)` under `-race`, and `1.8s 17/22 PASS 17 rfc2661-sccrq-tunnel-id-zero` in `make ze-l2tp-test` |
| AC-4 | both polarities are tagged on the SCCRQ path and the SCCRP tags are untouched | `grep -rn "RFC2661-24.10" internal/component/l2tp/ test/l2tp/` returns six lines: two in `reactor_sccrq_zero_tid_test.go`, two in `tunnel_initiator_test.go`, two in the `.ci` |
| AC-5 | the gate passes and the ledger is fresh | `make ze-rfc-check` exit 0 at closure on 2026-08-12: "2963 gated MUST-level requirement(s) across 170 enrolled RFC(s); 3341 test tag(s) resolved". No `make ze-rfc-index` was needed: another session ran it after the IS-IS enrolment landed, and `rfc/requirements/rfc2661.md` is unmodified against HEAD |
| AC-6 | a flood allocates no tunnel | `--- PASS: TestZeroIDSCCRQFloodAllocatesNoTunnel (0.50s)` under `-race` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| SCCRQ with Assigned Tunnel ID 0 | `test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` | Yes, read in full. `build_sccrq(0, ...)` at line 126 packs the AVP, and lines 132 to 168 unpack the reply and assert message type 4, Result Code 2, Error Code 3, a non-zero Assigned Tunnel ID and the header fields |
| SCCRQ with a non-zero Assigned Tunnel ID | same file | Yes. `build_sccrq(0x0303, ...)` at line 171, asserting SCCRP at line 181 and the echoed Assigned Tunnel ID at line 176, against the same daemon |
| Zero-ID SCCRQ over the wire | same file | Yes. `cmd=background:seq=1:exec=ze -` starts a real daemon, `expect=stderr:contains=zero Assigned Tunnel ID SCCRQ answered with StopCCN` pins the daemon's own record of the decision, so the reply cannot have come from another path |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | every StopCCN emission went through `teardownStopCCN`, a method on `*L2TPTunnel`, so none was reachable without a tunnel |
| A-2 | confirmed | `TestParseSCCRP_Rejects` calls `parseSCCRP` and `TestTunnelInitiatorHandshake` drives the initiator FSM; neither reaches `parseSCCRQ` or `handle` |
| A-3 | confirmed | `sendUnassociatedStopCCN` had to be written: it carries its own header and sequence numbers because the reliable engine lives on a tunnel |
| A-4 | confirmed | `handle` parses and dispatches the first datagram from any source, and nothing precedes it |
| A-5 | broken | 28 bytes in, 38 bytes out. Mistake Log row 1, and the threat model states the amplification at both layers |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 6, the L2TP guide states nothing that becomes false | `grep -rn -i "tunnel id 0\|assigned tunnel id" docs/guide/l2tp.md` returns nothing | Yes |
| Row 7 and 9, the wire format and the RFC claim | the trap table in `rfc/short/rfc2661.md` names both halves and matches `sendUnassociatedStopCCN`, which encodes Result Code 2 with Error Code 3 | Yes |
| Row 16, source anchors on the changed files | the anchors name `handleTick dead-peer check` (`docs/guide/l2tp.md:594`), `handleKernelSuccess auth-timeout read` (`docs/guide/configuration.md:2714`), `L2TPReactor dispatch` and `tunnel FSM transitions` (`docs/features.md:90`), plus the bare `<!-- source: -->` reference to `internal/component/l2tp/reactor.go` in `docs/guide/l2tp.md`. Every symbol still exists: this change renames and removes nothing | Yes |
| Rows 1 to 5, 8, 10 to 15 and 17, answered No | no command, no config leaf, no API, no plugin, no counter and no runner changed; the diff is three `.go` files in `internal/component/l2tp`, one new `_test.go`, one new `.ci`, `rfc/short/rfc2661.md` and the regenerated ledger | Yes |

## Core Insight

A requirement id is not a unit of proof. `RFC2661-24.10-1` names two messages, the ledger counts polarities per id, and the id read fully proven with one of its two messages untested and silently non-conformant. The same shape had already been recorded for an RFC 9234 requirement that enumerates three roles. Whenever a requirement's own text enumerates (two messages, three roles, four states), the enumeration is the population, and a gate that counts ids cannot see a hole in it.
