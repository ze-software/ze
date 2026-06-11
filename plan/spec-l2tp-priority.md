# Spec: L2TP Control Message Priority and Data P-Bit

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-06-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/wire/l2tp.md` - L2TP wire format and reliable engine
4. `rfc/short/rfc2661.md` - RFC 2661 summary (Section 3.1 P bit, Section 5.8 reliable delivery, Section 15 HELLO)

## Task

Two related improvements to L2TP packet prioritization:

**Part A (control plane):** Critical control messages (StopCCN, HELLO) should bypass session-level messages in the reliable engine's send queue. Currently the send queue is pure FIFO. Under heavy session churn (many ICRQs/ICRPs), a StopCCN teardown sits behind session setup messages, delaying tunnel teardown. HELLO is already protected (only sent when Outstanding()==0) but should also have priority as a defense-in-depth measure.

**Part B (data plane):** PPP LCP Echo-Request/Echo-Reply messages used for CQM monitoring flow as L2TP data messages. RFC 2661 Section 3.1 defines the P (Priority) bit for data messages: P=1 means "preferential treatment." These monitoring packets should be marked with P=1 so intermediate equipment and the local send path can prioritize them over bulk subscriber traffic.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/l2tp.md` - reliable engine lifecycle, send queue, buffer discipline
  -> Constraint: sendQueue is FIFO, gated by congestion window. No priority mechanism exists.
  -> Constraint: all encoding writes into caller-provided buffers, no hot-path allocs.
- [ ] `docs/architecture/core-design.md` - buffer-first, pool patterns
  -> Constraint: no make where pools exist

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2661.md` - Section 3.1 (P bit), Section 5.8 (reliable delivery), Section 15 (HELLO)
  -> Constraint: P bit MUST be 0 for control messages. P bit is for data messages only.
  -> Constraint: HELLO uses reliable delivery (sequence numbers, retransmit). Section 15.
  -> Constraint: Control header is always 0xC802 (T=1,L=1,S=1,O=0,P=0,Ver=2).

**Key insights:**
- RFC does not define priority within the control channel. Queue priority is an implementation choice.
- P bit applies only to data messages. Control messages carry P=0 unconditionally.
- HELLO is currently only sent when Outstanding()==0 (reactor.go:532), so it already goes straight to wire.
- StopCCN has no priority mechanism and queues behind session messages in sendQueue.
- PPP LCP Echo (CQM) is encapsulated by the kernel's l2tp_ppp module, not ze userspace.
- Linux kernel L2TP generic netlink has no priority attribute (genl_linux.go:37-53). Kernel constructs data headers internally without exposing P-bit control.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/reliable.go` - ReliableEngine, Enqueue (FIFO sendQueue), drainSendQueue
  -> Constraint: sendQueue is []pendingSend, pure FIFO append. drainSendQueue pops from front.
  -> Constraint: Enqueue returns ErrSendQueueFull at MaxSendQueueDepth (256).
  -> Constraint: Ns is assigned in send(), not at Enqueue/queue-insertion time.
- [ ] `internal/component/l2tp/tunnel_fsm.go` - handleHelloTimer (line 684), teardownStopCCN (line 267) both call engine.Enqueue
  -> Constraint: StopCCN and HELLO use the same Enqueue path as session messages.
- [ ] `internal/component/l2tp/session_fsm.go` - 4 Enqueue callers: handleICRQ (line 139), handleICCN (line 276), sendCDN (line 455), sendCDNNoSession (line 478)
  -> Constraint: all session messages are non-priority.
- [ ] `internal/component/l2tp/reactor.go` - handleTick, HELLO only when Outstanding()==0 (line 532)
  -> Constraint: HELLO is skipped when engine has outstanding retransmits (implicit keepalive).
- [ ] `internal/component/l2tp/header.go` - flagP=0x0100, WriteDataHeader sets P from h.Priority (line 184)
  -> Constraint: P bit is parsed and encoded correctly for data messages already.
- [ ] `internal/component/l2tp/genl_linux.go` - kernel L2TP attributes (lines 37-53)
  -> Constraint: no L2TP_ATTR_PRIORITY or similar. Kernel does not expose P-bit control.
- [ ] `internal/component/l2tp/pppox_linux.go` - PPPoL2TP socket options
  -> Constraint: socket options are LNS mode, send seq, recv seq. No priority option.
- [ ] `internal/component/l2tp/cqm.go` - CQMBucket, addEcho tracks PPP LCP Echo RTT
- [ ] `internal/component/l2tp/observer.go` - ObserverEventEchoRTT, CQM sample rings

**Enqueue callers (22 references across 4 files):**
- `reliable.go:335` - definition
- `tunnel_fsm.go:194` - SCCRP (handleSCCRQ) -> non-priority
- `tunnel_fsm.go:267` - StopCCN (teardownStopCCN) -> **priority**
- `tunnel_fsm.go:684` - HELLO (handleHelloTimer) -> **priority**
- `session_fsm.go:139` - ICRP (handleICRQ) -> non-priority
- `session_fsm.go:276` - ICCN (handleICCN) -> non-priority
- `session_fsm.go:455` - CDN (sendCDN) -> non-priority
- `session_fsm.go:478` - CDN (sendCDNNoSession) -> non-priority
- `reliable_test.go` - 14 test callers
- `reliable_integration_test.go` - (via mustEnqueue helper)

**Behavior to preserve:**
- HELLO only sent when Outstanding()==0 (existing optimization in reactor)
- sendQueue bounded at MaxSendQueueDepth (256)
- Reliable engine is not safe for concurrent use (reactor serializes)
- Control header always 0xC802 (P=0 for control)
- Buffer pool discipline (no make where pools exist)
- Engine lifecycle: Enqueue, OnReceive, Tick, BuildZLB unchanged semantics
- Ns assignment happens in send(), not at queue insertion

**Behavior to change:**
- Part A: Enqueue gains a priority parameter; priority messages insert at front of sendQueue
- Part B: Document kernel limitation. Investigate whether userspace can inject P-bit-marked LCP Echo frames as an alternative to kernel encapsulation.

## Data Flow (MANDATORY)

### Part A: Control Message Priority

#### Entry Point
- teardownStopCCN (tunnel_fsm.go:267) calls engine.Enqueue for StopCCN body
- handleHelloTimer (tunnel_fsm.go:684) calls engine.Enqueue for HELLO body

#### Transformation Path
1. FSM handler builds AVP body in pooled buffer (GetBuf/PutBuf)
2. Calls engine.Enqueue(sessionID, body, now) -- currently no priority flag
3. Enqueue checks window: if open, calls send() immediately; if closed, appends to sendQueue
4. drainSendQueue pops from front of sendQueue when window opens
5. send() assigns Ns=nextSendSeq++ and stamps header

#### Change
- Enqueue gains a `priority bool` parameter
- When priority=true and window is closed, message is prepended to sendQueue instead of appended
- Ns ordering on the wire is preserved because Ns is assigned in send(), not at insertion time
- teardownStopCCN and handleHelloTimer pass priority=true
- All other callers pass priority=false

### Part B: Data P-Bit

#### Entry Point
- Kernel l2tp_ppp module encapsulates PPP frames as L2TP data messages
- Ze userspace does not construct data message headers for session traffic

#### Constraint
- Linux kernel l2tp module does not expose a per-packet or per-session P-bit control
- No L2TP_ATTR in genl_linux.go for priority
- No socket option in pppox_linux.go for priority
- WriteDataHeader already supports Priority=true (header.go:184-186)
- ParseMessageHeader already parses P bit (header.go:80)

#### Feasibility Assessment
- Kernel path: would require a patch to net/l2tp/l2tp_core.c adding a session-level flag
- Userspace path: ze could intercept outgoing LCP Echo-Request before kernel encapsulation and send it via raw UDP with P=1, but this would duplicate the data path and conflict with kernel state
- Practical conclusion: P-bit for data messages is a kernel limitation. Document it. Consider proposing a kernel patch upstream.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| FSM -> Engine | Enqueue call with priority flag | [ ] |
| Kernel -> Wire | l2tp_ppp data encapsulation (P bit not exposed) | [ ] |

### Integration Points
- `ReliableEngine.Enqueue` - add priority parameter
- `tunnel_fsm.go` teardownStopCCN, handleHelloTimer - pass priority=true
- `tunnel_fsm.go` handleSCCRQ - pass priority=false
- `session_fsm.go` handleICRQ, handleICCN, sendCDN, sendCDNNoSession - pass priority=false

### Architectural Verification
- [ ] No bypassed layers (priority messages still go through reliable engine)
- [ ] No unintended coupling (priority is a simple flag, no new dependencies)
- [ ] No duplicated functionality (extends Enqueue, does not add a parallel path)
- [ ] Zero-copy preserved where applicable (same buffer discipline)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | sendQueue prepend preserves correct Ns ordering on wire | reliable.go: Ns assigned in send() at line 379, not at queue insertion | Ns would be misordered; peer would treat messages as out-of-order | Read send() and confirm Ns assignment | confirmed |
| A-2 | Linux kernel l2tp module does not support per-session P-bit | genl_linux.go:37-53 shows no priority attribute; pppox_linux.go has no priority setsockopt | Part B limited to documentation | grep kernel headers | confirmed |
| A-3 | StopCCN priority will not starve session messages | StopCCN is sent at most once per tunnel lifetime | Would need fairness mechanism | Protocol analysis | confirmed |
| A-4 | HELLO priority is defense-in-depth only; Outstanding()==0 guard is sufficient | reactor.go:532 | HELLO would queue behind session messages without the guard | Read reactor HELLO scheduling | confirmed |
| A-5 | CDN (session teardown) should NOT be priority | CDN tears down one session, not the tunnel; many CDNs during mass disconnect should not all jump the queue | Would starve session setup messages | User confirmation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Priority prepend at MaxSendQueueDepth+1 allows one extra message beyond the cap | Test: priority at capacity accepts; non-priority at capacity rejects | Acceptable: at most 1 StopCCN + 1 HELLO extra; bounded by tunnel lifetime |
| R-2 | Kernel L2TP P-bit requires upstream kernel patches | No attribute in kernel headers | Document as known limitation; revisit when kernel support lands |
| R-3 | Future code adds new Enqueue callers without considering priority | Code review misses the parameter | Enqueue signature forces the choice; compiler enforces |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| teardownStopCCN builds StopCCN body | -> | Enqueue(sid, body, now, true) prepends to sendQueue | TestEnqueuePriorityPrependsToSendQueue |
| handleHelloTimer builds HELLO body | -> | Enqueue(sid, body, now, true) prepends to sendQueue | TestEnqueuePriorityPrependsToSendQueue |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Enqueue with priority=true when window is closed | Message is prepended to sendQueue (sent before previously queued messages when window opens) |
| AC-2 | Enqueue with priority=false when window is closed | Message is appended to sendQueue (existing FIFO behavior preserved) |
| AC-3 | Enqueue with priority=true when window is open | Message is sent immediately (same as non-priority; priority only affects queued path) |
| AC-4 | teardownStopCCN calls Enqueue with priority=true | StopCCN bypasses queued session messages |
| AC-5 | handleHelloTimer calls Enqueue with priority=true | HELLO bypasses queued session messages |
| AC-6 | Two non-priority messages queued, then one priority message queued; window opens | Priority message is sent first (lowest Ns), then the two non-priority in original order |
| AC-7 | sendQueue at MaxSendQueueDepth with priority=true | Priority message is prepended (queue grows to MaxSendQueueDepth+1; non-priority would get ErrSendQueueFull) |
| AC-8 | WriteDataHeader with Priority=true | P bit (0x0100) is set in the flags word (already implemented; verify test coverage) |
| AC-9 | ParseMessageHeader on data message with P=1 | Priority field is true in returned MessageHeader (already implemented; verify test coverage) |
| AC-10 | Kernel L2TP P-bit investigation | Result documented: kernel does not expose P-bit control; documented in wire doc |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestEnqueuePriorityPrependsToSendQueue | internal/component/l2tp/reliable_test.go | AC-1, AC-6: priority goes to front; wire Ns ordering correct | |
| TestEnqueueNonPriorityAppends | internal/component/l2tp/reliable_test.go | AC-2: non-priority preserves FIFO | |
| TestEnqueuePriorityOpenWindow | internal/component/l2tp/reliable_test.go | AC-3: priority with open window sends immediately | |
| TestEnqueuePriorityAtCapacity | internal/component/l2tp/reliable_test.go | AC-7: priority accepted at MaxSendQueueDepth, non-priority rejected | |
| TestDataHeaderPriorityBitRoundTrip | internal/component/l2tp/header_test.go | AC-8, AC-9: P bit encode/decode (verify existing coverage) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| sendQueue depth (non-priority) | 0-256 | 256 (MaxSendQueueDepth) | N/A | 257 -> ErrSendQueueFull |
| sendQueue depth (priority) | 0-257 | 257 (MaxSendQueueDepth+1) | N/A | bounded by tunnel lifetime (at most 1 StopCCN + 1 HELLO) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | - | Control priority is internal engine behavior with no user-visible output change | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A -- priority is internal queue ordering; wire format unchanged | - | - | No interop impact | |

Justification for skipping interop: priority changes only the order messages leave the send queue, not their content or wire format. The peer sees valid L2TP control messages with correct Ns/Nr regardless of queue ordering.

### Future (if deferring any tests)
- Kernel P-bit integration test: deferred until kernel support is available (A-2 confirmed: no kernel support)
- Functional test for StopCCN priority under load: could add a chaos-test scenario with heavy session churn + tunnel teardown

## Files to Modify

- `internal/component/l2tp/reliable.go` - Enqueue gains priority bool parameter, prepend logic
- `internal/component/l2tp/tunnel_fsm.go` - teardownStopCCN (line 267) and handleHelloTimer (line 684) pass priority=true; handleSCCRQ (line 194) passes priority=false
- `internal/component/l2tp/session_fsm.go` - 4 callers pass priority=false (lines 139, 276, 455, 478)
- `internal/component/l2tp/reliable_test.go` - new priority tests + update existing Enqueue calls
- `internal/component/l2tp/reliable_integration_test.go` - update mustEnqueue helper
- `docs/architecture/wire/l2tp.md` - document priority mechanism and kernel P-bit limitation

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No | - |
| CLI commands/flags | [ ] No | - |
| Functional test for new RPC/API | [ ] No | - |
| Prometheus counters/metrics | [ ] No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No (control header unchanged; P bit already defined for data) | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2661.md` - add note about priority queue implementation |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/l2tp.md` - document priority Enqueue + kernel P-bit status |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, etc. changed? | No | - |
| 16 | Source anchors reference changed files? | No | grep docs/ for source anchors to reliable.go |
| 17 | Existing docs show examples for this area? | No | - |

## Files to Create

- None (all changes are modifications to existing files)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan -- check what exists |
| 3. Wiring phase | Add priority parameter to Enqueue, compile-fail all callers |
| 4. Implement (TDD) | Priority prepend logic + tests |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | make ze-lint-changed && go test ./internal/component/l2tp/... |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables review | Deliverables Checklist |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** -- add priority parameter to Enqueue, update all 22 call sites
   - Tests: TestEnqueuePriorityPrependsToSendQueue (write first, fails: no prepend logic yet)
   - Files: reliable.go (Enqueue signature), tunnel_fsm.go (3 callers), session_fsm.go (4 callers), reliable_test.go (14 callers + mustEnqueue helper)
   - Verify: compiles; existing tests pass; new test fails on behavior

2. **Phase: Priority prepend** -- implement prepend in Enqueue when priority=true and window closed
   - Tests: TestEnqueuePriorityPrependsToSendQueue, TestEnqueueNonPriorityAppends, TestEnqueuePriorityOpenWindow, TestEnqueuePriorityAtCapacity
   - Files: reliable.go (Enqueue body, ~5 lines changed)
   - Verify: all tests pass

3. **Phase: P-bit verification** -- confirm existing header tests cover P bit round-trip
   - Tests: TestDataHeaderPriorityBitRoundTrip (verify exists or add)
   - Files: header_test.go
   - Verify: P bit encode/decode covered

4. **Phase: Documentation** -- update wire doc and RFC summary
   - Files: docs/architecture/wire/l2tp.md, rfc/short/rfc2661.md

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Ns ordering preserved on wire despite queue reordering |
| Naming | priority parameter named consistently across all callers |
| Data flow | Priority messages still flow through full reliable engine (Enqueue -> sendQueue -> send -> rtmsQueue) |
| Rule: buffer-first | No new allocations in priority path (prepend reuses existing slice mechanics) |
| Rule: no-layering | No parallel send path; extends existing Enqueue |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Enqueue accepts priority parameter | grep "priority bool" reliable.go |
| teardownStopCCN passes priority=true | grep -A2 "Enqueue" tunnel_fsm.go near teardownStopCCN |
| handleHelloTimer passes priority=true | grep -A2 "Enqueue" tunnel_fsm.go near handleHelloTimer |
| Session callers pass priority=false | grep "Enqueue" session_fsm.go |
| Priority prepend tests exist and pass | go test -run TestEnqueuePriority ./internal/component/l2tp/ |
| Wire doc updated | grep -i "priority" docs/architecture/wire/l2tp.md |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | priority is a bool set by internal callers only; no untrusted input |
| Resource exhaustion | Priority at capacity allows MaxSendQueueDepth+1; bounded by protocol (1 StopCCN per tunnel) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Phase 1 missed an Enqueue caller; find and update |
| Test fails wrong reason | Fix test setup (window must be closed for prepend to matter) |
| Ns ordering broken | Re-read send() -- A-1 was wrong (but confirmed: Ns in send()) |
| Existing tests fail | Enqueue signature change missed a caller |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Prepend to sendQueue over separate priority queue | (1) Separate priority queue drained first, (2) Priority flag on pendingSend with sort, (3) Bypass sendQueue entirely | Prepend is simplest. At most 1 StopCCN + 1 HELLO per tunnel lifetime are priority. A separate queue adds complexity for near-zero benefit. Bypassing the queue would break Ns ordering. |
| Allow priority at MaxSendQueueDepth+1 over rejecting | Reject priority at capacity (same as non-priority) | A StopCCN that cannot be enqueued delays tunnel teardown. One extra slot for a critical message is acceptable; bounded by protocol (once per tunnel). |
| Keep Outstanding()==0 guard for HELLO | Remove guard and rely on priority alone | Guard is correct: outstanding messages ARE keepalive signals. Priority is defense-in-depth for future changes that might relax the guard. |
| CDN (session teardown) is non-priority | Make CDN priority | CDN tears down one session. During mass disconnect, many CDNs should not all jump the queue and starve session setup. Tunnel teardown (StopCCN) is the priority case. |
| Part B: document kernel limitation rather than implement userspace workaround | (1) Intercept LCP Echo in userspace before kernel encap, (2) Propose kernel patch | Userspace interception would duplicate the data path and conflict with kernel L2TP state. A kernel patch is the correct long-term solution but out of scope for ze. |

## Known Limitations
- Kernel l2tp_ppp module encapsulates data messages; ze cannot set P=1 on PPP frames without kernel support or a userspace encapsulation path
- Priority only affects the gated sendQueue; if the window is open, all messages send immediately regardless of priority
- No priority differentiation within rtmsQueue (retransmits are already ordered by original Ns)
- CDN is deliberately non-priority to avoid starvation during mass session teardown
