# Spec: ike-dpd-demand-driven

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ike-dpd-demand-driven.md` (created only if something is deferred) |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** Requirement `RFC7296-2.4-2` [SHOULD] of `rfc/short/rfc7296.md`
line 676 reads "Liveness checks are demand-driven, not periodic; only check when
traffic to send and no recent inbound proof (§2.4)". Ze does not meet it. Dead
Peer Detection fires on a fixed period from the last probe, and no inbound
traffic of any kind moves that deadline.

**The symptom.** A tunnel carrying continuous authenticated traffic, in either
the IKE SA or its Child SA, is probed at every configured interval for its whole
life. On a Child-SA-only tunnel, which is the normal steady state between
rekeys, every single interval produces a probe even though the peer has proved
itself alive thousands of times over. The probes cost a request window
(RFC 7296 Section 2.3 allows one outstanding self-initiated request), so each one
delays a rekey or a Delete that wanted the same window.

**The goal.** Make the DPD clock read the liveness evidence RFC 7296 Section 2.4
names, so a probe goes out only when that evidence is absent.

**Owner decision, 2026-08-07.** Thomas answered "create new spec" to the homing
question raised in `plan/deferrals/fixit-ike-dpd-cleartext.md`. This spec is that
destination. It is a research skeleton: the mechanism is NOT chosen. The open
questions below are his to answer, and `## Open Questions for Thomas` is the
first thing a design session reads.

**What this spec is not.** It is not `plan/spec-fixit-ike-resource-lifetime-leaks.md`
(nothing leaks here) and it is not `plan/spec-fixit-ike-test-discrimination.md`
(the behavior is missing, not the test). The deferral row rules both out by name.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `plan/spec-ipsec-dataplane-inspection.md` - the spec that built the kernel SAD read surface this work would reuse
  → Decision: `SAInfo` carries `BytesCurrent`, `PacketsCurrent`, `AddedAt` and `UsedAt` because the IKE engine never sees ESP payload, so counting in userspace reports zero forever.
  → Constraint: a dataplane that cannot be READ is never reported as an empty dataplane. `ListSAs` returns `ErrNotSupported` and a nil slice, never an empty slice and a nil error.
- [ ] `ai/rules/evidence.md` - the guard rule that governs what happens when the SAD read fails
  → Constraint: a guard must fail closed or say something. A zero value must never be a valid-looking answer. "The kernel could not be asked" and "the peer sent nothing" are different answers and must not collapse.
- [ ] `ai/rules/goroutine-lifecycle.md` - governs any new poller
  → Constraint: a timer or scheduled task gets ONE dedicated long-lived goroutine with cancellation. Never one per event, never one per poll.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` line 676 - the requirement itself
  → Constraint: `RFC7296-2.4-2` is SHOULD-level, ungated and untagged. `rfc/enrolled.txt` line 167 records 227 rows for RFC 7296, 222 of them gated; this row is one of the 5 SHOULD-level rows that are ungated and untagged, so `make ze-rfc-check` reports it uncovered rather than green over a violation. Nothing today claims false conformance.
- [ ] `rfc/full/rfc7296.txt` Section 2.4, lines 1519-1621 - the source text
  → Constraint: the governing sentences are quoted verbatim below. Read them, not the summary paraphrase: the paraphrase adds "only check when traffic to send", which the RFC states as motivation rather than as the SHOULD.

**Key insights:** (minimal context to resume after compaction)
- The liveness signal for Child SA traffic EXISTS on the XFRM backend and is already read by two callers. It does NOT exist on the VPP backend, at all.
- The defect is one missing field write plus one absent observation, not a redesign of DPD.
- RFC 7296 Section 2.4 uses "needs to", not "SHOULD". The [SHOULD] tag is the summary's classification, not the RFC's word.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `rfc/full/rfc7296.txt` lines 1519-1621 - RFC 7296 Section 2.4 in full
- [ ] `rfc/short/rfc7296.md` line 676 - the `RFC7296-2.4-2` checklist row
- [ ] `ai/RFC-REQUIREMENTS.md` line 3787 - the published coverage row for `RFC7296-2.4-2`: both evidence columns are `--`, so no tagged test exists in either polarity
- [ ] `internal/component/ike/engine/dpd.go` - the whole DPD clock: `dpdState`, `newDPDState`, `shouldSend`, `nextDeadline`, `timedOut`, `shouldRetransmit`, `sendDPD`, `retransmitDPD`, `handleDPDResponse`
- [ ] `internal/component/ike/engine/established.go` - `maintainSA`, the owner loop that drives the clock
- [ ] `internal/component/ike/engine/inbound.go` - `handleOwnedInbound`, the producer of the `peerAlive` liveness signal
- [ ] `internal/component/ike/engine/health_drift.go` - `driftSAD`, `driftingPeers`: an existing engine-side SAD reader
- [ ] `internal/component/ike/cmd/show_ipsec.go` - `readSADCounters`, `addChildCounters`: an existing per-SPI counter lookup
- [ ] `internal/component/ike/dataplane/dataplane.go` - the `Dataplane` interface and the `SAInfo` type
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `ListSAs`, `saInfoFromState`, `unixOrZero`: the XFRM producer of the counters
- [ ] `internal/component/ike/dataplane/vpp.go` - `vppBackend.ListSAs`: the VPP refusal
- [ ] `internal/component/ike/dataplane/noop.go` - `noopDataplane.ListSAs`: the noop refusal
- [ ] `internal/component/ike/engine/reconcile.go` - `PeerInfo`, the type carrying `ChildInSPI` and `ChildOutSPI`
- [ ] `internal/component/ike/ipsec/types.go` - `DPDConfig`
- [ ] `internal/component/ike/ipsec/config.go` - `parseDPD`

### The RFC text that governs WHEN to probe (verbatim)

RFC 7296 Section 2.4, `rfc/full/rfc7296.txt`. Five sentences bear on the timing
question. Line numbers are given because the exact wording is the fact.

| # | Lines | Verbatim sentence |
|---|-------|-------------------|
| S-1 | 1557-1560 | "If a cryptographically protected (fresh, i.e., not retransmitted) message has been received from the other side recently, unprotected Notify messages MAY be ignored." |
| S-2 | 1578-1580 | "If there has only been outgoing traffic on all of the SAs associated with an IKE SA, it is essential to confirm liveness of the other endpoint to avoid black holes." |
| S-3 | 1580-1583 | "If no cryptographically protected messages have been received on an IKE SA or any of its Child SAs recently, the system needs to perform a liveness check in order to prevent sending messages to a dead peer." |
| S-4 | 1585-1587 | "Receipt of a fresh cryptographically protected message on an IKE SA or any of its Child SAs ensures liveness of the IKE SA and all of its Child SAs." |
| S-5 | 1588-1590 | "Note that this places requirements on the failure modes of an IKE endpoint. An implementation needs to stop sending over any SA if some failure prevents it from receiving on all of the associated SAs." |

Two sentences from the same section bound what the fix may not break:

| # | Lines | Verbatim sentence |
|---|-------|-------------------|
| S-6 | 1546-1550 | "An endpoint MUST conclude that the other endpoint has failed only when repeated attempts to contact it have gone unanswered for a timeout period or when a cryptographically protected INITIAL_CONTACT notification is received on a different IKE SA to the same authenticated identity." |
| S-7 | 1563-1565 | "The number of retries and length of timeouts are not covered in this specification because they do not affect interoperability." |

### What `RFC7296-2.4-2` requires, and what its SHOULD leaves open

| Element | What the RFC text settles | What it leaves open |
|---------|---------------------------|---------------------|
| The trigger to probe | S-3: the trigger is the ABSENCE of a recently received protected message, on the IKE SA **or any of its Child SAs**. Both halves are named in one sentence, and S-4 repeats the pair | Nothing. This half is explicit |
| What counts as proof of liveness | S-4: a fresh (not retransmitted) cryptographically protected message on the IKE SA or any Child SA. S-1 adds that "fresh" excludes a retransmission | How the implementation OBSERVES a Child SA message. The RFC says nothing about the mechanism |
| "recently" | Nothing. The word is not quantified anywhere in Section 2.4 | The whole quantity. S-7 says retries and timeouts do not affect interoperability, so a freshness window is an implementation choice |
| The role of pending outbound traffic | S-2 states that outgoing-only traffic makes confirming liveness "essential", and S-3 gives the purpose as "to prevent sending messages to a dead peer" | Whether an implementation MAY suppress the probe entirely when it has nothing to send. The RFC motivates the check by the send, it does not forbid probing an idle SA |
| Word strength | The RFC uses "needs to" (S-3) and "is essential" (S-2). It never writes SHOULD in these sentences | The [SHOULD] tag on the checklist row is the summary's own classification. The obligation is real; the keyword register for this row is prose, not RFC 2119 |
| The failure verdict | S-6 is a MUST and is separate: repeated unanswered attempts. It is already met and tagged (`RFC7296-2.4-11`) | Nothing. Any fix here must leave S-6 untouched |

**The summary paraphrase is narrower than the RFC on one point and wider on
another.** It says "only check when traffic to send", which S-2 and S-3 motivate
but never require: the RFC asks for a check when nothing has been RECEIVED, and
explains why that matters by pointing at what will be SENT. Whether Ze adopts
the send-gated reading is a design choice, recorded as Q-3 below. The paraphrase
also drops "or any of its Child SAs", which is the hard half of the requirement.

### Coverage status of the requirement today

| Fact | Evidence |
|------|----------|
| RFC 7296 is enrolled, so its MUSTs are gated | `rfc/enrolled.txt` line 167 |
| 222 of its 227 rows are gated; 5 are SHOULD-level, ungated and untagged | same line |
| `RFC7296-2.4-2` is one of those 5 | `ai/RFC-REQUIREMENTS.md` line 3787: `\| RFC7296-2.4-2 \| SHOULD \| 2.4 \| -- \| -- \| \|`, both evidence columns empty |
| No test carries the tag in either polarity | a repository-wide search for the id finds it only in `rfc/short/rfc7296.md`, `ai/RFC-REQUIREMENTS.md`, `plan/deferrals/fixit-ike-dpd-cleartext.md` and `skip-blocked.md`. No `.go` and no `.ci` |
| The gate therefore reports it uncovered, not green | `make ze-rfc-check` does not gate SHOULD-level rows |

**Consequence for this spec.** Nothing today claims false conformance, so no
public claim has to be retracted. Implementing the behavior means ADDING a
`RFC requirement: RFC7296-2.4-2` tagged test in both polarities, which is a
strict improvement and needs no permission (`ai/rules/rfc-compliance.md`).

### The DPD clock: producer trace

Every row names the file and the symbol that PRODUCES the behavior.

| Symbol | File | What it does | Bearing on the requirement |
|--------|------|--------------|----------------------------|
| `dpdState` | `internal/component/ike/engine/dpd.go` | Holds `interval`, `timeout`, `action`, `lastSent`, `awaitReply`, `sentAt`, `probeMsgID`, `probeMsg`, `lastAttempt`, `retries` | `lastSent` is the entire clock. There is no field recording inbound liveness |
| `newDPDState` | `dpd.go` | Returns nil when `cfg.Interval` is 0, otherwise builds the state with `lastSent: time.Now()` | The clock starts at session start, not at first send. So the first probe fires one interval after establishment even on a busy tunnel |
| `shouldSend` | `dpd.go` | Returns false when a probe is outstanding, otherwise `!now.Before(d.lastSent.Add(d.interval))` | **This is the producer of the defect.** `lastSent` is the only input besides `interval`. No inbound evidence reaches it |
| `nextDeadline` | `dpd.go` | Returns `sentAt + timeout` while awaiting, else `lastSent + interval` | Same input set. A caller reading the deadline sees the same periodic schedule |
| `sendDPD` | `dpd.go` | The **only writer of `lastSent`** outside `newDPDState`. Writes `lastSent = now` at the tail, after the datagram reached the send path | So `lastSent` means "when this side last probed", never "when the peer was last proved alive" |
| `handleDPDResponse` | `dpd.go` | Clears `awaitReply`, drops `probeMsg`, zeroes `retries`, logs `dpd: peer alive`. **Does not touch `lastSent`** | This is the site the deferral row names. The reply proves liveness and the clock ignores it |
| `retransmitDPD` | `dpd.go` | Repeats the stored datagram, calls `noteRetransmit` | Writes `lastAttempt` and `retries` only. Not the interval clock |
| `timedOut` | `dpd.go` | `!now.Before(d.sentAt.Add(d.timeout))` | The dead-peer verdict. S-6's MUST. Out of scope except that it must not regress |
| `shouldRetransmit` | `dpd.go` | Backoff over `lastAttempt` and `retries`, refused once `timedOut` | S-7 and the exponential-backoff MUST. Out of scope |
| `maintainSA` | `internal/component/ike/engine/established.go` line 136 | The per-peer owner goroutine. Signature carries `sa`, `dpd`, `childLT`, `ikeLT`, `ikeGroup`, `table`, **`dp dataplane.Dataplane`**, `tr`, `bus`, `log` | **The dataplane handle is already in scope in the loop that schedules DPD.** No new plumbing is needed to reach the SAD from here |
| `maintainSA` ticker | `established.go` line 158 | `time.NewTicker(1 * time.Second)` | The scheduling granularity is one second, per peer session. A per-tick SAD dump per peer is the cost to beware of |
| `maintainSA` DPD arms | `established.go` lines 282-302 | In order: `timedOut` ends the SA; `serviceRequestRetransmit`; `serviceRequestWindow`; `shouldRetransmit` repeats; `shouldSend` sends | The insertion point for any new suppression is the `shouldSend` arm at line 300 |
| `maintainSA` liveness arm | `established.go` lines 206-220 | On `out.peerAlive`, or on `out.dpdResp` whose message ID `matchesProbe`, it retires the request window and calls `handleDPDResponse` | **The IKE-SA-level liveness signal is already computed and already reaches the DPD state.** It just does not move `lastSent` |
| `handleOwnedInbound` | `internal/component/ike/engine/inbound.go` line 42 | Sets `out.peerAlive = true` at line 200, after `decryptAndParse` succeeded and after `classifyInbound` accepted the Message ID | This is the producer of "a fresh cryptographically protected message on the IKE SA". Both S-4 conditions hold at that line: authenticated, and in-window so not a stale replay |
| `handleOwnedInbound` out-of-window arm | `inbound.go` lines 144-146 | Returns an empty outcome with the comment "An out-of-window message is no evidence of liveness (RFC 7296 Section 2.4). A replay can therefore never mask a dead peer" | The "fresh, i.e., not retransmitted" clause of S-1 is already honored on this path |
| `handleOwnedInbound` retransmit arm | `inbound.go` lines 59-102 | A duplicate request is answered from cache and returns an empty outcome | A retransmission also produces no liveness credit. Correct per S-1 |

**Summary of the trace.** Exactly one thing writes `lastSent`: `sendDPD`. Exactly
one thing would need to write it for the IKE-SA half of S-4: the `peerAlive` arm
of `maintainSA` at `established.go` line 209, which already runs and already
calls into `dpd.go`. The Child SA half of S-4 has no producer anywhere in the
engine.

### Can Ze observe Child SA traffic at all? Yes on XFRM, no on VPP

This is the question that decides the shape of the work. The answer is
backend-dependent and is already written down in the code.

| Backend | Can the Child SA carry-counters be read? | Producer |
|---------|------------------------------------------|----------|
| XFRM (Linux) | **Yes.** `SAInfo.PacketsCurrent`, `SAInfo.BytesCurrent` and `SAInfo.UsedAt` are filled from the kernel SAD dump | `saInfoFromState` (`internal/component/ike/dataplane/xfrm_linux.go`) maps `s.Statistics.Packets`, `s.Statistics.Bytes`, `s.Statistics.AddTime` and `s.Statistics.UseTime` onto them. `ListSAs` (same file) dumps with `xfrmStateList(netlink.FAMILY_ALL)` and filters on `ifID` in userspace |
| VPP | **No.** `ListSAs` refuses | `vppBackend.ListSAs` (`internal/component/ike/dataplane/vpp.go`) returns `ErrNotSupported` with "the vpp backend cannot enumerate the SAD; use the VPP CLI to read it back". The binary-API dump is not implemented |
| noop | **No.** `ListSAs` refuses | `noopDataplane.ListSAs` (`internal/component/ike/dataplane/noop.go`) returns `ErrNotSupported`: it installs nothing, so "no SAs" is true of its own state and false about the machine |
| Non-Linux XFRM stub | **No.** | `xfrmBackend.ListSAs` in `internal/component/ike/dataplane/xfrm_other.go` |

**The signal is not hypothetical: two callers already consume it.**

| Caller | File | What it does with the SAD dump |
|--------|------|-------------------------------|
| `driftSAD` and `driftingPeers` | `internal/component/ike/engine/health_drift.go` | An ENGINE-side reader. `driftSAD` calls `dataplane.Get()` then `ListSAs(0)`; `driftingPeers` indexes the result by SPI and compares it against `PeerInfoMap()`, matching on `info.ChildInSPI` and `info.ChildOutSPI`. It returns a second `known` result so that "could not read" never renders as "no drift" |
| `readSADCounters` and `addChildCounters` | `internal/component/ike/cmd/show_ipsec.go` | The CLI reader. `readSADCounters` builds a `map[uint32]SAInfo` keyed by SPI; `addChildCounters` looks up `info.ChildInSPI` and `info.ChildOutSPI` and renders `bytes-in`, `packets-in`, `bytes-out`, `packets-out`, plus a `counters-known` flag that separates "nobody could ask" from "the kernel does not hold this SPI" |

**The SPI key already exists in the engine.** `PeerInfo.ChildInSPI` is written by
`reconcile.go` line 285 from `child.InboundSPI`, and `ChildSA` (`child.go` line
64) carries `InboundSPI` and `OutboundSPI` as its first two fields.

**Therefore:** on Linux with the XFRM backend, every part needed to observe Child
SA receive traffic is present and proven in two independent call sites. The
missing piece is a decision about polling cadence and about what happens on a
backend that cannot answer. On VPP the signal does not exist at all, and
creating it means implementing the VPP IPsec SAD dump, which is separable work
already recorded as a Known Limitation of `plan/spec-ipsec-dataplane-inspection.md`
and adjacent to `plan/spec-fixit-vpp-ipsec-inoperable.md`.

**One thing is NOT established and must be validated before it is relied on:**
whether the Linux kernel updates `use_time` and the packet counters on the
INBOUND SA on receive, and at what granularity. `unixOrZero`
(`xfrm_linux.go`) shows the kernel writes epoch SECONDS and uses 0 for "never
used", so the resolution is one second at best. Recorded as A-1 below with a
validation method. A delta of `PacketsCurrent` is the safer signal than `UsedAt`
if the timestamp turns out to be output-only.

### Configuration surface today

| Item | Producer | Values |
|------|----------|--------|
| `DPDConfig` | `internal/component/ike/ipsec/types.go` | `Action` (`DPDAction`), `Interval` (uint16, seconds), `Timeout` (uint16, seconds) |
| `parseDPD` | `internal/component/ike/ipsec/config.go` | Defaults `DPDActionHold`, `defaultDPDInterval`, `defaultDPDTimeout`. `interval` accepts 1 to `maxDPDValue`, and rejects 0 |

An interval of 1 second is configurable and is used by
`test/ipsec/ipsec-dpd-holds-tunnel.ci`. Any polling design must survive that
setting.

**Behavior to preserve:** (unless the user explicitly said to change it)
- The dead-peer verdict of S-6: repeated unanswered attempts over `timeout`, produced by `timedOut` and `shouldRetransmit`. `RFC7296-2.4-11` is tagged and MUST stay green.
- The exponential backoff of retransmissions (`retransmitBackoff`, `noteRetransmit`).
- The request-window discipline of RFC 7296 Section 2.3: one outstanding self-initiated request, reserved by `reserveRequestWindow` and released on every exit of `sendDPD`.
- `test/ipsec/ipsec-dpd-holds-tunnel.ci` must stay green and must stay discriminating. It asserts that a probe left (`dpd: sent probe`), that the peer answered (`dpd: peer alive`), and that one SA stayed established for more than 20 seconds with an interval of 1 second. **A fix that suppresses probing on a busy tunnel could make its first two assertions vacuous.** Read its fixture before changing the schedule.
- The refusal contract of `ListSAs`: `ErrNotSupported` plus nil, never an empty slice plus nil error.

**Behavior to change:**
- The inputs to `shouldSend`. What exactly is added is the subject of the open questions below.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Two entry points exist today and a third is proposed but not chosen.

1. **A timer tick.** `maintainSA`'s 1-second ticker (`established.go` line 158) reaches the `shouldSend` arm at line 300. Format at entry: a `time.Time`.
2. **An inbound datagram.** `ps.inbound` delivers a `transport.Packet` to `handleOwnedInbound` (`inbound.go` line 42). Format at entry: raw UDP bytes carrying an IKE header and an Encrypted payload.
3. **(Proposed, not chosen) A kernel SAD record.** `dp.ListSAs(0)` returns `[]dataplane.SAInfo`, one entry per SAD state, carrying `SPI`, `PacketsCurrent`, `BytesCurrent` and `UsedAt`. Format at entry: a netlink XFRM state dump parsed by `saInfoFromState`.

### Transformation Path

Path A, the IKE-SA liveness signal that exists today:

1. `ps.inbound` hands a packet to `handleOwnedInbound` (`inbound.go`).
2. `classifyInbound` applies the RFC 7296 Section 2.3 Message ID window. Out-of-window and cached-retransmit arms return an empty outcome and produce no liveness.
3. `decryptAndParse` authenticates and parses the inner chain.
4. `out.peerAlive = true` at `inbound.go` line 200.
5. `maintainSA` reads `out.peerAlive` at `established.go` line 209, retires the request window if a probe was outstanding, and calls `handleDPDResponse`.
6. `handleDPDResponse` (`dpd.go`) clears `awaitReply`. **The path ends here. `lastSent` is untouched, so the schedule is unchanged.**

Path B, the Child SA liveness signal that does not exist:

1. ESP datagrams arrive on the wire and are decapsulated by the kernel XFRM stack. **The IKE engine never sees them.** No code in `internal/component/ike/engine/` observes a Child SA packet.
2. The kernel increments `Statistics.Packets` and `Statistics.Bytes` on the inbound SAD state, and sets `Statistics.UseTime`.
3. Nothing in the DPD path reads them. `driftSAD` reads the SAD for a different question (SPI presence), and `readSADCounters` reads it for the CLI.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Kernel dataplane ↔ IKE control plane | The `dataplane.Dataplane` interface, `ListSAs(ifID uint32) ([]SAInfo, error)` | Yes: `driftSAD` (`health_drift.go`) already crosses it from inside the engine |
| Owner goroutine ↔ dataplane | `maintainSA` already receives `dp dataplane.Dataplane` as a parameter (`established.go` line 143) | Yes: read the signature |
| Engine belief ↔ kernel fact | `PeerInfo.ChildInSPI` / `ChildOutSPI` as the join key onto `SAInfo.SPI` | Yes: `driftingPeers` and `addChildCounters` both join on it |
| IKE SA ↔ DPD state | `ownedOutcome.peerAlive` / `dpdResp` / `dpdRespMsgID` | Yes: `established.go` lines 206-220 |

### Integration Points

- `handleDPDResponse` (`dpd.go`) - the existing sink for IKE-SA liveness.
- `shouldSend` (`dpd.go`) - the predicate any suppression must reach.
- `driftSAD` (`health_drift.go`) - an existing engine-side SAD reader whose caching decision, or absence of one, is the precedent to follow or to change.
- `PeerInfo` (`reconcile.go`) - already carries both Child SPIs.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | (fill during design) | The dataplane read must go through `dataplane.Dataplane`, never through raw netlink from the engine |
| No unintended coupling (components stay isolated) | (fill during design) | The engine already depends on `internal/component/ike/dataplane`; no new component edge is created |
| No duplicated functionality (extends existing, does not recreate) | (fill during design) | `driftSAD` and `readSADCounters` both build an SPI-keyed map from the same dump. A third copy would be the duplication to avoid |
| Zero-copy preserved where applicable (refs, not copies) | (fill during design) | `SAInfo` is a value type; `driftingPeers` notes that `PeerInfo` is large and indexes the map rather than range-copying it |
| Registration over hardcoding | (fill during design) | No new command, view, family or handler is expected |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The Linux kernel updates `Statistics.UseTime` and `Statistics.Packets` on the **inbound** SAD state when an ESP packet is received, not only on output | Inferred from `saInfoFromState` reading both from the dump, and from `show vpn ipsec sa` advertising `packets-in` as a real number. **Not verified against kernel behavior** | The whole Child SA half of S-4 loses its signal, and only Option A remains reachable | A `_linux_test.go` integration test under `make ze-qemu-integration-test`: install a Child SA, send traffic through it, dump the SAD, assert the inbound SPI's `PacketsCurrent` rose and `UsedAt` moved | unvalidated |
| A-2 | `UsedAt` has one-second resolution and `PacketsCurrent` is monotonic within an SA's life | `unixOrZero` (`xfrm_linux.go`) converts epoch SECONDS and maps 0 to the zero time | A sub-second DPD interval cannot be driven off `UsedAt`. A delta of `PacketsCurrent` becomes the only usable signal | Same integration test as A-1 | unvalidated |
| A-3 | A full SAD dump per peer per second is too expensive to ship, so any polling design shares one dump | `ListSAs` (`xfrm_linux.go`) dumps `FAMILY_ALL` and filters in userspace; `maintainSA`'s ticker is 1 second per peer session (`established.go` line 158) | The design can be simpler than assumed and no sharing is needed | A benchmark, or a measured QEMU run with N peers | unvalidated |
| A-4 | `out.peerAlive` is set only for messages that satisfy S-4's "fresh, i.e., not retransmitted" | `handleOwnedInbound` (`inbound.go`): the out-of-window arm returns an empty outcome at line 146 and the cached-retransmit arm at line 102 | Crediting liveness on a replay would let an attacker mask a dead peer, which is the failure the existing comments say they prevent | Read the three return sites; the existing `rfc7296_retransmit_test.go` and `rfc7296_invalidmsgid_test.go` already assert `!out.peerAlive` on those paths | unvalidated |
| A-5 | Rewriting `lastSent` on inbound liveness cannot starve DPD indefinitely | The peer would have to keep sending authenticated traffic to keep the clock pushed, and by S-4 that traffic IS the liveness proof | If a path exists where the clock is pushed by something that is not proof of the peer being alive, DPD goes silent on a dead peer | Enumerate every writer of the new field and check each against S-4 | unvalidated |
| A-6 | The VPP backend is genuinely unable to answer, not merely unimplemented behind a flag | `vppBackend.ListSAs` (`vpp.go`) returns `ErrNotSupported` and its comment says the binary-API dump is not implemented | A cheaper VPP path exists and Q-2 changes shape | Read `vpp.go` in full and check `govpp` binapi availability in `vendor/` | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Suppressing probes on a busy tunnel makes `test/ipsec/ipsec-dpd-holds-tunnel.ci` vacuous: its `dpd: sent probe` assertion could stop firing for the right reason and the test would then prove nothing | The `.ci` goes red, or worse stays green while asserting a log line produced by a different path | Re-read the fixture before changing the schedule. The test pins the responder's interval at 3600 so probe traffic has one direction; that fixture may need a quiet-tunnel variant |
| R-2 | A failed SAD read is treated as "no traffic seen", so a probe fires that should not have, or is treated as "traffic seen", so DPD goes silent on a dead peer | Health output or DPD logs disagree with the kernel | `driftingPeers` already models the correct shape: return a second `known` result and never let an unread dataplane render as a value. `ai/rules/evidence.md` |
| R-3 | Per-peer per-second SAD dumps at scale cost measurable CPU and netlink traffic | Profiling, or `ze-perf` on a many-peer fixture | Share one dump across peers with a cadence independent of the 1-second tick |
| R-4 | A design that reads only the INBOUND Child SPI misses the S-2 "outgoing traffic only" case, which is the case the RFC calls essential | Review finds the outbound SPI unused | Both SPIs are already in `PeerInfo`; decide deliberately, not by omission |
| R-5 | The behavior diverges between the XFRM and VPP backends, so a VPP deployment keeps periodic DPD while a Linux one does not, and nothing tells the operator | Nothing today would surface it | Q-2 must be answered before implementation. Whatever the answer, the difference is stated in `docs/features/rfc-status.md` and the operator-facing docs |
| R-6 | The `interval` leaf's meaning changes under the operator's feet: today it is "probe every N seconds", after the fix it becomes "probe when N seconds have passed with no proof of life" | An operator reports fewer probes than configured | This is the intended change and it is user-visible. It belongs in `docs/guide/` and in the YANG description, not only in the code |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A dead peer is not detected, so a tunnel black-holes traffic until an operator notices. That is the exact failure S-3 and S-5 exist to prevent, and it is worse than the over-probing being fixed. The opposite error, over-suppression, is silent |
| How is it reverted? | Single commit revert. No wire format changes, no config migration, no peer-visible state. The probe's wire form is untouched |
| Who else touches this path? | `plan/spec-fixit-ike-resource-lifetime-leaks.md` (the `sendDPD` request-window row, resolved 2026-08-07) and `plan/spec-fixit-ike-test-discrimination.md` (the `.ci` that now guards the tunnel). `plan/spec-ipsec-dataplane-inspection.md` owns the SAD read surface. Another agent was editing `internal/component/ike/engine/dpd.go` on 2026-08-07 |

## Open Questions for Thomas

**These are not answered and no autonomous default is recorded.** The destination
was left open because the shape is his to pick. A design session answers nothing
here on its own.

Note before reading them: on the XFRM backend, full conformance with S-4 is
reachable with parts that already exist and are already used twice. The questions
below are about MECHANISM and about the backends where the signal genuinely does
not exist. None of them asks whether to comply.

### Q-1: which liveness sources reset the clock?

| Option | What it observes | Cost | What it does NOT satisfy |
|--------|------------------|------|--------------------------|
| A. IKE SA only | `out.peerAlive` and a matched `dpdResp`, both already computed at `established.go` line 209 | One field write in `handleDPDResponse` (`dpd.go`), plus tests. Hours | The "or any of its Child SAs" clause of S-3 and S-4. A steady-state tunnel between rekeys carries NO IKE traffic, so on the normal case this changes nothing at all |
| B. IKE SA plus inbound Child SA | A, plus `SAInfo.PacketsCurrent` or `UsedAt` for `PeerInfo.ChildInSPI`, read through `dp.ListSAs` | A, plus a shared SAD poller, plus the unread-dataplane decision of Q-2, plus a QEMU integration test to validate A-1. Days | Nothing in S-3 or S-4. Does not address the S-2 "outgoing traffic only" motivation, which is a separate reading (Q-3) |
| C. B plus outbound demand gating | B, plus `ChildOutSPI` counters to tell "we are sending" from "the tunnel is idle" | B, plus a decision about an idle tunnel: probe never, or probe on a long secondary interval. Interacts with `DPDConfig.Action` (hold, clear, restart), because a peer that dies while the tunnel is idle is then discovered only on the next send | Nothing. This is the summary paraphrase's full reading. It is the largest behavior change and the one most visible to an operator |

### Q-2: what happens on a dataplane that cannot be read?

VPP refuses (`vppBackend.ListSAs`, `vpp.go`), noop refuses (`noop.go`), non-Linux
refuses (`xfrm_other.go`), and an unprivileged process without CAP_NET_ADMIN gets
an error from netlink.

| Option | Behavior when the SAD cannot be read | Cost | What it does NOT satisfy |
|--------|--------------------------------------|------|--------------------------|
| a. Fall back to today's periodic schedule | DPD probes at `interval` exactly as now | Free. Matches `driftingPeers`'s existing "a question that was not asked is not an answer" shape | S-4 on those backends. The divergence is invisible to the operator unless it is surfaced |
| b. Fall back to periodic AND surface it | a, plus a health signal or a log line naming the peers whose liveness cannot be data-plane-observed | A health-check arm and its test. `checkIPsecHealth` (`health.go`) is the existing home | Nothing beyond a. It makes a's limitation visible rather than silent |
| c. Implement the VPP IPsec SAD dump first | The signal exists on VPP too | A VPP binary-API dump: `vpp.go` says it is unimplemented and the `govpp` binapi is not vendored for it. This is a whole spec of its own, adjacent to `plan/spec-fixit-vpp-ipsec-inoperable.md` | Nothing, but it is out of proportion to a SHOULD and would block this work behind unrelated VPP work |

### Q-3: is the probe gated on having traffic to send?

S-2 and S-3 motivate the check by what will be SENT. S-3's own trigger is only
the absence of RECEIVED traffic. The summary paraphrase adopts the send-gated
reading; the RFC text does not require it.

| Option | Behavior on an idle tunnel with a silent peer | Cost | What it does NOT satisfy |
|--------|----------------------------------------------|------|--------------------------|
| i. Not gated: probe whenever no proof of life arrived | The peer is found dead one `timeout` after the last proof, as today | None beyond Q-1 | The paraphrase's "only check when traffic to send". Keeps probing an idle tunnel, which is what the requirement's own wording objects to |
| ii. Gated: no outbound traffic means no probe | The peer's death is discovered on the first send after it, not before | Needs Q-1 option C. Changes when `DPDConfig.Action` fires, and changes what an operator sees in `show vpn ipsec sa` for a long-idle peer | Nothing in the RFC text. But it delays detection, and an operator who configured DPD to keep NAT state alive would lose that side effect |

### Q-4: where does the poll live and at what cadence?

| Option | Shape | Cost | What it does NOT satisfy |
|--------|-------|------|--------------------------|
| p. Per-peer, inside `maintainSA`'s existing 1-second tick | No new goroutine. `dp` is already a parameter (`established.go` line 143) | One full `FAMILY_ALL` SAD dump per peer per second. `ListSAs` filters in userspace, so cost is O(total SAs) per peer per second | Nothing functionally. It is the scaling risk R-3 |
| q. One shared poller, cached dump read by every peer | One long-lived goroutine per `ai/rules/goroutine-lifecycle.md`, one dump per cadence | A new lifecycle to own and cancel, plus a staleness bound on the cache that must be shorter than the smallest configured `interval` (which can be 1 second, per `parseDPD`) | Nothing. It is more code than p |
| r. Poll only when a probe is about to fire | The dump happens in the `shouldSend` arm, so a busy tunnel costs one dump per interval and an idle one costs the same | Cheapest at scale; the dump is on the decision path so a slow netlink call delays the probe | Nothing. It makes the netlink call synchronous inside the owner goroutine, which is the thing to check against `ai/rules/goroutine-lifecycle.md` |

### Q-5: does `interval` keep its meaning, or gain a sibling leaf?

Today `interval` (`parseDPD`, `internal/component/ike/ipsec/config.go`) means
"probe every N seconds". After Q-1 it would mean "probe when N seconds passed
with no proof of life". That is a change in a documented config leaf's meaning
(R-6).

| Option | Cost | What it does NOT satisfy |
|--------|------|--------------------------|
| Redefine `interval` in place | Doc and YANG description updates. No schema change, no migration | Nothing technically. An operator reading old docs is surprised |
| Add a separate quiet-period leaf | A YANG leaf, its validation, its env-var question, its completion, and its doc rows (`ai/patterns/config-option.md`) | Nothing. It is more surface for a behavior the RFC treats as one concept |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (fill during design: an authenticated inbound IKE message on an established SA) | → | (fill during design: the writer of the liveness timestamp) | (fill during design) |
| (fill during design: ESP traffic on the Child SA) | → | (fill during design: the SAD observation path) | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (fill during design, after Q-1 is answered) | (fill during design) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | (fill during design) | (fill during design) | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | `internal/component/ike/engine/dpd_test.go` | (fill during design) | |

**Known constraint on the test plan.** Any new test must carry
`RFC requirement: RFC7296-2.4-2` in both polarities: one asserting the probe is
suppressed when liveness evidence is fresh, one asserting it still fires when the
evidence is absent or stale. The requirement carries no tag today
(`ai/RFC-REQUIREMENTS.md` line 3787, both evidence columns `--`), so adding it is
a strict improvement to coverage and needs no permission.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `DPDConfig.Interval` | 1 to `maxDPDValue` seconds (`parseDPD`) | (fill during design) | 0 | `maxDPDValue`+1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (fill during design) | `test/ipsec/*.ci` | (fill during design) | |

**Known constraint.** `test/ipsec/` holds no `needs-linux` `.ci` today, so the
existing suite runs without a real XFRM dataplane. A functional test of the
Child SA observation path therefore needs either a `needs-linux` `.ci` run under
`make ze-qemu-needs-linux-test`, or a `_linux_test.go` integration test under
`make ze-qemu-integration-test` alongside the existing
`child_policy_delete_integration_linux_test.go` and
`xfrm_readback_integration_linux_test.go`.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (fill during design) | `test/ipsec-interop/` | strongSwan | (fill during design) | |

**Note on interop applicability.** The probe's WIRE FORM does not change: this
work changes only WHEN a probe is emitted. S-7 says explicitly that retries and
timeouts "do not affect interoperability". An interop scenario would therefore
have to assert an ABSENCE (no probe on a busy tunnel), which
`ai/rules/interop-and-goal-validation.md` names as a vacuity trap. Whether one is
required is a design-phase decision, and the reason must be recorded either way.

## Files to Modify
- (fill during design) `internal/component/ike/engine/dpd.go` - the clock
- (fill during design) `internal/component/ike/engine/established.go` - the owner loop's DPD arms
- (fill during design, only under Q-1 option B or C) an SAD observation path

## Files to Create
- (fill during design)

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | (fill during design) | Depends on Q-5. `internal/component/ike/ipsec/yang/` |
| YANG validation constraints | (fill during design) | Only if Q-5 adds a leaf |
| YANG custom validators | (fill during design) | Only if Q-5 adds a leaf |
| CLI commands/flags | No | No new command is expected |
| CLI grammar (keyword before value) | No | No new command |
| Editor autocomplete | (fill during design) | Only if Q-5 adds a leaf |
| Functional test for new RPC/API | No | No new RPC |
| Pipe completeness | No | No new output |
| Env var registration | No | No leaf under `environment/` is expected |
| Doctor check for runtime dependencies | (fill during design) | Under Q-1 option B or C the engine gains a dependency on a readable SAD. `doctor_xfrm.go` already exists in `internal/component/ike/engine/` |
| Prometheus counters/metrics | (fill during design) | A "probes suppressed by observed liveness" counter would make the change measurable |
| BGP family surface | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | (fill during design) | `docs/features.md` |
| 2 | Config syntax changed? | (fill during design) | Depends on Q-5 |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | (fill during design) | The IPsec guide page, for R-6's change in what `interval` means |
| 7 | Wire format changed? | No | The probe's wire form is untouched |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | **Yes** | `rfc/short/rfc7296.md` line 676 and the `docs/features/rfc-status.md` RFC 7296 row, with source anchors |
| 10 | Test infrastructure changed? | (fill during design) | If a `needs-linux` ipsec `.ci` is the first of its kind, `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | (fill during design) | `docs/comparison.md`: strongSwan's DPD is demand-driven |
| 12 | Internal architecture changed? | (fill during design) | Under Q-1 option B or C, a new control-plane consumer of the dataplane read surface |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | (fill during design) | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | (fill during design) | Grep `docs/` for the touched files |
| 17 | Existing docs show config/CLI/API examples for this area? | (fill during design) | The `dead-peer-detection` config examples |

## Implementation Steps

(fill during design, after Q-1 through Q-5 are answered. The mechanism decides
the phases, and choosing them before Thomas answers would be designing the
solution this skeleton exists to defer.)

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Removed guard | `timedOut` and `shouldRetransmit` still produce the S-6 verdict, and `RFC7296-2.4-11`'s tagged test is still green |
| Vacuous test | `test/ipsec/ipsec-dpd-holds-tunnel.ci` still asserts a probe that really goes out. R-1 |
| Fail-open guard | An unreadable SAD never renders as "the peer is alive". `ai/rules/evidence.md` |
| Data flow | The engine reaches the kernel only through `dataplane.Dataplane`, never through raw netlink |
| Duplication | The SPI-keyed dump index is shared with `driftingPeers` / `readSADCounters` rather than written a third time |
| Rule: `ai/rules/goroutine-lifecycle.md` | Any poller is one long-lived cancellable goroutine, never one per peer per tick |
| Rule: `ai/rules/rfc-compliance.md` | The new behavior carries a `RFC requirement: RFC7296-2.4-2` tag in both polarities |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| (fill during design) | (fill during design) |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Replay cannot mask a dead peer | Every new writer of the liveness timestamp is reached only after authentication AND the Message ID window, per A-4. `handleOwnedInbound`'s out-of-window and cached-retransmit arms must stay non-crediting |
| Kernel counters are not attacker-controlled in a useful way | A peer that sends ESP traffic Ze cannot decrypt still increments the inbound SAD counters only if the packet passes the integrity check. Verify this against the kernel before relying on the counter as proof of liveness |
| Resource exhaustion | A per-peer per-second full SAD dump is an amplification of peer count into netlink load. R-3 |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-1 turns out false (the kernel does not update inbound counters) | STOP. Q-1 option B and C lose their signal. Report to Thomas before proceeding |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- **The IKE-SA-level liveness signal already reaches the DPD state and is then discarded.** `maintainSA` (`established.go` line 209) calls `handleDPDResponse` on `out.peerAlive`, and `handleDPDResponse` (`dpd.go`) touches everything except `lastSent`. The narrow half of the fix is one field write at a site that already runs.
- **The Child SA signal is not missing from the machine, only from the DPD path.** `saInfoFromState` (`xfrm_linux.go`) already lifts `Statistics.Packets`, `Statistics.Bytes` and `Statistics.UseTime` out of the kernel, and two callers already consume them. What is missing is a consumer in the scheduling path, not a producer.
- **`maintainSA` already holds a `dataplane.Dataplane`.** Its signature (`established.go` line 143) carries `dp`, so the loop that schedules DPD can already ask the kernel. No new plumbing, no new component edge.
- **The backend split is the real design constraint.** XFRM can answer, VPP cannot (`vpp.go`), noop cannot (`noop.go`). Any design that assumes the signal exists produces two different daemons, and the difference is silent unless it is deliberately surfaced.
- **`shouldSend` reads exactly two fields.** `awaitReply` and `lastSent`. That is the whole surface a fix has to change, which is why this is a small spec with a large open question rather than a large spec.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| (fill during design) | | |

## Known Limitations

- (fill during design)

## RFC Documentation (Scope: protocol)

Add `// RFC 7296 Section 2.4: "<quoted requirement>"` above the enforcing code.
The sentences to quote are S-3 and S-4 in the table above, verbatim. MUST
document: which observations reset the clock, what happens when the dataplane
cannot be read, and why an out-of-window or retransmitted message credits no
liveness.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
- [ ] `RFC7296-2.4-2` carries a `RFC requirement:` tagged test in both polarities
- [ ] `plan/deferrals/fixit-ike-dpd-cleartext.md` row 3 is resolved

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
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
