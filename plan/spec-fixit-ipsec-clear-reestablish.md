# Spec: fixit-ipsec-clear-reestablish

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
3. `internal/component/ike/cmd/ipsec.go`, `internal/component/ike/engine/register.go`, `internal/component/ike/engine/reconcile.go`, `internal/component/ike/engine/fsm.go` - the clear/re-establish/responder-accept paths
4. `plan/learned/745-ipsec-10-cli-diag.md` - clear-command design context

## Task

Fix the product gap where `clear vpn ipsec sa` does not re-establish a
site-to-site tunnel when the peer is a separate, still-established responder.

**Observed behavior (2026-07-10, two unprivileged ze instances over loopback,
initiator + responder, PSK, noop dataplane):** after the initiator processes an
operator `clear vpn ipsec sa`, it re-sends `IKE_SA_INIT` (confirmed: 2 sent),
but the responder -- which never ran `clear` and still holds the peer's original
SA -- accepts only the first init (confirmed: 1 accepted) and no second SA
establishes. The tunnel that the operator intended to bounce stays down until
something else re-triggers it. Evidence captured while implementing
`spec-test-coverage-gaps` (`plan/learned/` follow-up; the two-daemon
`test/ipsec/ipsec-sa-installed.ci` documents the deferral in a `test-relax`
note).

~~Root cause is a **hypothesis to confirm during research**, not an established
fact: the responder's inbound-init path appears to reject or misroute a fresh
`IKE_SA_INIT` (new initiator SPI, zero responder SPI) while it already owns an
established SA for that configured peer.~~ **Superseded 2026-07-16 (design
research): hypothesis CONFIRMED and located exactly.** The responder drops the
fresh `IKE_SA_INIT` at the `responderBusy` CAS (`register.go:546`), because
`responderBusy` stays `true` for the entire established lifetime of the SA. See
Current Behavior for the producer-cited trace.

**The problem statement is also corrected (2026-07-16).** "The tunnel stays down
until something else re-triggers it" is not accurate: the responder DOES
self-heal via DPD, so this is a **latency** bug, not a permanent wedge. Derived
from the producers (`dpd.go:34` `newDPDState`, `dpd.go:69` `timedOut`,
`established.go:145-149`, `fsm.go:218-224`, `fsm.go:35` `maxRetransmissions`,
`fsm.go:703` `retransmitBackoff`, `fsm.go:50` `reconnectDelay`,
`ipsec/config.go:26-27` DPD defaults 30s/120s): the responder frees its stale SA
only after a DPD timeout (~30s to send the first unanswered probe + 120s
timeout = up to ~150s), and the initiator's own retransmit/reconnect ladder
(~63s of retransmits, then a 60s reconnect delay) means end-to-end convergence
is on the order of **150-190 seconds**. The observing test
(`test/ipsec/ipsec-sa-installed.ci:23`) has a 40s timeout, so it correctly saw
"no second SA establishes" inside its window.
-> Decision: the goal is re-establishment in **seconds** after an operator
clear, not "eventually". The AC bound is set at <= 10s (see AC-1). The existing
~150s DPD path is the fallback that must keep working, not the target.
-> Constraint: RFC 7296 Section 2.4 forbids the naive fix. An unauthenticated
`IKE_SA_INIT` MUST NOT cause the responder to tear down the established SA:
"An endpoint MUST NOT conclude that the other endpoint has failed based on any
routing information ... or IKE messages that arrive without cryptographic
protection". Supersede-on-init is an unauthenticated remote teardown primitive
(R-1) and is banned.

### Scope

- IN: make an operator `clear vpn ipsec sa` (all-SAs and per-peer) result in a
  re-established tunnel against a live responder, for `connection-type initiate`
  peers, within a bounded time.
- IN: a functional test proving it end to end (two ze instances).
- OUT: rekey-timer-driven re-negotiation (that path works); MOBIKE; the NAT-T
  `0.0.0.0:4500` hardcoded-port note (tracked separately with the IKE test-port
  seam work).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as -> Decision: / -> Constraint:. -->
- [ ] `plan/learned/745-ipsec-10-cli-diag.md` - clear-command design
  -> Constraint: the clear command was designed as a **local-only** bounce. Its
  own words: "TerminateAllSAs and TerminatePeerSA trigger re-establishment by
  calling a stored reconcile function ... This gives `clear vpn ipsec sa` bounce
  semantics." It was never designed to tell the PEER anything. That is the gap:
  the local side re-initiates, the remote side is never informed.
  -> Constraint: 745 also records "The reEstablishFn atomic pointer creates a
  closure-capture coupling between runEngine locals and the package-level
  terminate functions." Any fix that adds a graceful-close step must respect
  that the terminate functions run on the **RPC goroutine**, not the owner loop.
- [ ] `rfc/short/rfc7296.md` (IKEv2) - Section 1.2 (initial exchanges), 2.4 (state synchronization / dead peer), 2.8 (rekeying)
  -> Decision: **RFC 7296 Section 2.4 (State Synchronization and Connection
  Timeouts) governs this fix** (summary section index: `rfc/short/rfc7296.md:419`).
  Two normative facts from it, verified against the RFC text (see RFC Summaries
  below for the gap):
  1. "An endpoint MUST NOT conclude that the other endpoint has failed based on
     any routing information ... or IKE messages that arrive without
     cryptographic protection." -> the responder MUST NOT delete the old SA
     because an `IKE_SA_INIT` arrived. Coexist, do not supersede.
  2. "The INITIAL_CONTACT notification asserts that this IKE SA is the only IKE
     SA currently active between the authenticated identities" and "MUST be in
     the first IKE_AUTH request or response"; the recipient "MAY use this
     information to delete any other IKE SAs it has to the same authenticated
     identity without waiting for a timeout."
  -> Constraint: INITIAL_CONTACT rides in IKE_AUTH, which is **message 3**. It
  therefore cannot unwedge a responder that drops the `IKE_SA_INIT` at message
  1. The accept-in-parallel change is a prerequisite for INITIAL_CONTACT being
  reachable at all. Ordering matters; see Implementation Phases.
  -> Constraint: RFC 7296 Section 1.4 (Delete via INFORMATIONAL) is the
  authenticated close primitive. It is already implemented and sent by the IKE
  rekey path (`inbound.go:232 sendDeleteIKE`, called only from `inbound.go:147`).
  The clear path does not use it.
- [ ] `ai/rules/interop-and-goal-validation.md` - strongSwan interop obligations for IPsec
  -> Decision: an interop scenario **is required** (see TDD Test Plan for which
  direction and why the two-ze `.ci` cannot substitute).

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` - verify the summary exists; create if missing before asserting RFC behavior
  -> Constraint: the summary EXISTS but is **thin exactly where this fix needs
  it**. It covers Section 2.4 only as DPD (`:291-300`) and reduces
  INITIAL_CONTACT to a single notify-table row (`:347`, "Sender is
  re-establishing; delete old SAs for this identity"). It carries neither the
  "MUST NOT conclude failure from unauthenticated messages" rule nor the "MUST
  be in the first IKE_AUTH" placement rule. `rfc/full/rfc7296.txt` does not
  exist locally (only `rfc/full/rfc4724.txt` and `rfc9072.txt` of that vintage).
  -> Decision: **run `/ze-rfc` for RFC 7296 Section 2.4 before implementation
  writes the enforcing comment.** The design's normative quotes were verified
  against the published RFC text during this design pass, but the summary is the
  citable artifact for the code comment and must carry them first.

**Key insights:**
- The bug is a **gate**, not a parse/route failure: one atomic bool held across
  the wrong lifetime. `responderBusy` means "one in-flight handshake at a time"
  but is released as if it meant "one SA at a time".
- The skeleton's own Current Behavior line about `fsm.go:185` was wrong, and
  wrong in the way `ai/rules/no-fabrication.md` predicts: it read the reset call
  site and inferred when it runs, instead of reading the loop that contains it.
- There are **two** independent single-SA-per-peer couplings, not one. Freeing
  the gate alone produces a misroute (`routeInbound` keys on peer name, not SA)
  and a clobber (`ps.setSA`). A one-line fix to `register.go:546` would look
  right and be worse than the bug.
- The cheapest correct fix does not touch the responder at all: make the
  initiator say goodbye (RFC 7296 Section 1.4 Delete) on operator clear. The
  responder path still needs fixing for the peer-crash case, which no Delete can
  cover.

## Current Behavior (MANDATORY)

**Source files read (2026-07-10, spec author):**
- [ ] `internal/component/ike/cmd/ipsec.go` (:17-44 handleClearIPsecSA) - `clear vpn ipsec sa` -> `engine.TerminateAllSAs()`; `clear vpn ipsec sa peer <name>` -> `engine.TerminatePeerSA(name)`
- [ ] `internal/component/ike/engine/register.go` (:72-104 TerminateAllSAs, :106-135 TerminatePeerSA) - stops each PeerSession, removes its SA from the SATable, emits sa-down, then calls `reEstablishFn` (:100-102 / :131-133)
- [ ] `internal/component/ike/engine/register.go` (:199-213 reEstablish closure) - `reEstablish` -> `reconcilePeers(rc.cfg, nil, activePeers, table, rc.tr, eb, log)`; `reEstablishFn.Store(&reEstablish)` (:213)
- [ ] `internal/component/ike/engine/reconcile.go` (:176-250 reconcilePeers) - for each desired peer not currently `active`, `startPeerSession(...)` (:245) -- this DOES re-initiate for `initiate`-type peers, matching the observed 2nd IKE_SA_INIT
- [ ] `internal/component/ike/engine/register.go` (:527-564 tryResponderSAInit) - responder accept path: guards on IKE_SA_INIT request + zero responder SPI (:530-538), `matchResponderPeer` (:539), `responderBusy` CAS (:546-549), `newResponderSA` + `table.Insert` (:550-560); logs "accepting inbound IKE_SA_INIT" (:562)
- [ ] ~~`internal/component/ike/engine/fsm.go` (:185 responderBusy reset on "responder ready") - the busy gate frees after establishment, so it is NOT the drop cause for a post-establishment init~~
  **STRUCK 2026-07-16 (design research): this claim is FALSE and it was the
  keystone error of the skeleton.** `fsm.go:185` is not "reset on establishment".
  It is the **first statement of `runResponder`** (`fsm.go:177-186`), executed
  once per `runOnce` cycle, before any handshake exists. The gate is NOT freed
  after establishment; it is freed only after `runEstablished` **returns**, i.e.
  after the tunnel is already down (`fsm.go:218-224`). The busy gate IS the drop
  cause. Mistake Log row added.

**Producer-cited trace (2026-07-16 design research). Every claim below names the
function that PRODUCES the behavior, per `ai/rules/no-fabrication.md`.**

### The `responderBusy` lifetime (the bug)

| Step | Producer | What it does |
|------|----------|--------------|
| Set true | `tryResponderSAInit`, `register.go:546` | `ps.responderBusy.CompareAndSwap(false, true)` on accepting an unsolicited `IKE_SA_INIT`. On CAS failure: logs "responder busy, dropping concurrent IKE_SA_INIT" (`register.go:547`) and returns `true` = **packet consumed and silently dropped**. |
| Cleared (a) | `runResponder`, `fsm.go:185` | `ps.responderBusy.Store(false)` -- the **first line of the function**, once per `runOnce` cycle, before any SA exists. Not an establishment hook. |
| Cleared (b) | `runResponder`, `fsm.go:224` | After `runEstablished` **returns** (`fsm.go:218`), i.e. tunnel already down. |
| Cleared (c) | `runResponder`, `fsm.go:235` | On `StateDead` (handshake failed pre-establishment). |
| Cleared (d) | `reapStaleHandshake`, `fsm.go:263` | Only if the SA is **pre-established** and older than `responderHandshakeTimeout` (30s, `fsm.go:43`). Explicitly returns early without reaping when `sa.State == StateEstablished` (`fsm.go:256-258`). |

-> Decision: **there is no path that clears `responderBusy` while the SA is
established.** Between `register.go:546` (set) and `fsm.go:224` (cleared on
teardown) the gate is held for the tunnel's entire established life. This is the
root cause. Verified exhaustively: `grep -rn "responderBusy" internal/` returns
exactly the five non-test sites above plus the field declaration
(`reconcile.go:30`).

-> Constraint: the field's own doc comment (`reconcile.go:24-29`) states the
intent -- "gates a `respond` peer to one in-flight handshake ... runResponder
clears it once the SA establishes or dies". The **intent is correct and the
implementation does not match it**: "once the SA establishes" was never
implemented. The fix restores the documented meaning rather than inventing new
semantics.

### FSM states involved and the transition that fails to fire

The responder SA reaches `StateEstablished` (`sa.go:26`) and `runResponder`
adopts it into `runEstablished` (`fsm.go:218`), which blocks in `maintainSA`
(`established.go:79`, `established.go:104` loop) until DPD/lifetime/stop. The
**transition that fails to fire is the responder's "established -> accept a new
initiation" transition, which does not exist in the FSM at all.** `runResponder`
has no `StateEstablished` case that re-arms the accept path; its
`StateEstablished` case (`fsm.go:204-230`) is a one-way door into
`runEstablished`. The only exit is a tunnel-down event, which is precisely what
the operator clear is trying to trigger from the far side and cannot.

### Two more single-SA-per-peer couplings (why freeing the gate alone is wrong)

| # | Producer | Why a naive fix breaks |
|---|----------|------------------------|
| 1 | `routeInbound`, `register.go:482-495` | Routes on `lookupPeerSession(sa.PeerName)` + `ps.established.Load()` -- keyed on the **peer**, not the SA. While the old SA is established (`established.go:30` sets it, `:31` defers false), ANY packet for ANY SA of that peer is shoved into `ps.inbound` and handled by `handleOwnedInbound` (`inbound.go:36`), which decrypts under the **old** SA's keys (`inbound.go:69 decryptAndParse`) and fails. `tryResponderSAInit` itself ends by calling `routeInbound` (`register.go:563`), so even the accepted init would be misrouted. |
| 2 | `ps.setSA`, `reconcile.go:72-76` (called at `register.go:561`) | One `ps.sa` pointer per peer. Publishing the new responder SA clobbers the established one that `runResponder`'s poll loop (`fsm.go:198`) and `TerminateAllSAs` (`register.go:90`) read. |

Note the handshake path itself is already SA-driven, not `ps.sa`-driven:
`handleInbound` (`fsm.go:277-284`) passes the table's SA to
`handleResponderInbound` (`responder.go:57`), which switches on `sa.State`. So
parallel SAs are viable in the handshake code; the couplings are in routing,
publication, and the gate.

### Local (initiator) clear path -- works today

`handleClearIPsecSA` (`cmd/ipsec.go:17`) -> `TerminateAllSAs` (`register.go:72`)
/ `TerminatePeerSA` (`register.go:106`) -> `ps.Stop()` (`reconcile.go:169`,
`sync.Once` + wait on `done`), `table.Remove`, `emitSADown`, `delete` from
`activePeersMap` -> `reEstablishFn` (`register.go:100` / `:131`) -> the
`reEstablish` closure (`register.go:205-212`) -> `reconcilePeers`
(`reconcile.go:176`) -> the peer is absent from `active` (`reconcile.go:228-233`)
-> `startPeerSession` (`reconcile.go:245`) -> new goroutine -> `runInitiator`
(`fsm.go:85`) sends a fresh `IKE_SA_INIT` with a NEW initiator SPI
(`newInitiatorSA` -> `fsm.go:107-116`). A-1 confirmed; this matches "2 inits sent".

-> Constraint: `activePeersMap` and `reconcilePeers`' `active` are the **same
map object** (`register.go:191-192` creates it and `setActivePeers` stores the
same reference; `reconcile.go:211` captures it). Deleting from one deletes from
the other. This is what makes the re-initiate work, and it is load-bearing.

### No goodbye is sent

`ps.Stop()` (`reconcile.go:169`) closes `stopCh`; `maintainSA`'s stop case
(`established.go:106-108`) calls `cleanupChild` and returns `nil`. **No
INFORMATIONAL Delete is sent.** `sendDeleteIKE` (`inbound.go:232`) exists and is
correct, but `grep -rn "sendDeleteIKE" internal/` shows its only caller is
`inbound.go:147` (the IKE-rekey make-before-break path). The peer is never told.

### Why it eventually recovers (and why the test never saw it)

`newDPDState` (`dpd.go:34`) returns nil only when `cfg.Interval == 0`;
`parseDPD` (`ipsec/config.go:287-315`) rejects 0 as out of range and defaults to
30s interval / 120s timeout (`ipsec/config.go:26-27`, `:225-229`). So DPD is
always armed in production config. The responder's unanswered probes trip
`dpd.timedOut` (`dpd.go:69`) -> `maintainSA` returns `errTimeout`
(`established.go:145-149`) -> `runResponder` tears down and clears the gate
(`fsm.go:221-224`). The dropped `IKE_SA_INIT`s never credit liveness: they are
consumed at `register.go:546-549` and never reach `routeInbound`/DPD.
-> Decision: worst-case self-heal ~150s on the responder, and the initiator's
own ladder (`fsm.go:35` 7 retransmits, `fsm.go:703` backoff, `fsm.go:50`
`reconnectDelay` capped at 60s by `fsm.go:37`) puts convergence at ~150-190s.
`ipsec-sa-installed.ci:23` sets `timeout:value=40s`. The test window is ~4x too
short to observe recovery, which fully explains "1 accepted of 2".

**Behavior to preserve:**
- Rekey-timer re-negotiation, single-in-flight-handshake gating (`responderBusy`, register.go:546), SATable identity semantics, existing sa-up/sa-down event emission.
- `clear vpn ipsec sa peer <name>` and all-SAs both keep their JSON result shape (`action`/`terminated`/`peer`).
- -> Constraint: the DPD self-heal path (`fsm.go:221-224`) must keep working. It
  is the only recovery for a peer that dies without sending anything, and it is
  the fallback if a Delete is lost (UDP, best-effort).
- -> Constraint: the single-owner model (`established.go:30-31`, `inbound.go:33-35`)
  is load-bearing and was bought with real bugs (spec-ipsec-13). `sa.NextMsgID`
  and `sa.SKKeys` are owned by `maintainSA`. Nothing on the RPC goroutine may
  build or send an encrypted message.

**Behavior to change:**
- After an operator clear, the tunnel must re-establish against a live responder. The exact source change is determined during research (responder accepts the new init / stale-SA teardown / initiator signals the peer). Do not presuppose the fix location beyond the paths above.
- -> Decision (2026-07-16): both halves are in scope, in this order. **(A)** The
  initiator sends an authenticated IKE Delete on operator clear, so the responder
  tears down immediately (RFC 7296 Section 1.4). **(B)** The responder accepts a
  fresh `IKE_SA_INIT` in parallel with an established SA and supersedes only once
  the new SA authenticates (RFC 7296 Section 2.4). A alone leaves the peer-crash
  case broken (a crashed peer sends no Delete); B alone leaves the clear path
  paying a full handshake against a responder that still holds a dead tunnel.
  See Key Design Decisions for the alternatives and the open question for Thomas.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- CLI/RPC `clear vpn ipsec sa [peer <name>]` (`ze-clear:vpn-ipsec-sa`, ipsec.go:13)

### Transformation Path
1. Handler -> `TerminateAllSAs` / `TerminatePeerSA` (register.go)
2. Local SATable teardown + sa-down emit + `reEstablishFn`
3. `reconcilePeers` -> `startPeerSession` re-initiates for initiate-type peers
4. Wire: initiator IKE_SA_INIT -> responder `tryResponderSAInit` (register.go:527)
5. ~~Responder either accepts (new SA) or the packet is dropped/misrouted (the bug)~~
   **Resolved 2026-07-16:** responder drops it at the `responderBusy` CAS
   (`register.go:546-549`), logging "responder busy, dropping concurrent
   IKE_SA_INIT" and returning `true` (consumed). Nothing else in the chain is
   reached.

### Target flow after the fix

| # | Step | Producer to add/change |
|---|------|------------------------|
| 1 | Operator clear marks the session for graceful close, then stops it | `TerminateAllSAs`/`TerminatePeerSA` (`register.go:72`/`:106`) set a graceful flag before `ps.Stop()` |
| 2 | The **owner loop** sends the authenticated Delete on its way out | `maintainSA` stop case (`established.go:106-108`) calls `ps.sendDeleteIKE` (`inbound.go:232`) before `cleanupChild`. Owner-goroutine only: `sa.NextMsgID`/`SKKeys` are its state. |
| 3 | Responder's owner loop receives the Delete, tears down | `handleInformationalOwned` (`inbound.go:248`) / `handleDeletePayload` (`inbound.go:282`) -- **already implemented**, no change expected (confirm in audit) |
| 4 | Responder frees gate + SATable slot, re-arms accept | `runResponder` (`fsm.go:218-224`) -- existing path, now reached in ~1 RTT instead of ~150s |
| 5 | Initiator's fresh `IKE_SA_INIT` is accepted normally | `tryResponderSAInit` (`register.go:527`) -- unchanged |
| 6 | (Phase B) If no Delete arrived (peer crash), responder accepts the init in parallel and supersedes on authenticated IKE_AUTH | `responderBusy` gate scope, `routeInbound` per-SA keying, `ps.setSA` second slot, INITIAL_CONTACT emit/handle |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI/RPC -> engine | dispatch-command `ze-clear:vpn-ipsec-sa` | yes, read 2026-07-16: `cmd/ipsec.go:12-15` RegisterRPCs `ze-clear:vpn-ipsec-sa` -> `handleClearIPsecSA` |
| RPC goroutine -> owner goroutine | graceful-close flag + `stopCh` (NEW) | not yet -- must not build/send encrypted messages off the owner loop (`inbound.go:33-35`, A-5) |
| initiator -> responder | UDP IKE_SA_INIT (RFC 7296) | yes, read 2026-07-16: `runInitiator` `tr.Send(initMsg, remote)` `fsm.go:112-116`; responder ingress `dispatchInbound` `register.go:569-606` |
| initiator -> responder | UDP INFORMATIONAL Delete (RFC 7296 §1.4) (NEW on clear) | not yet -- builder exists (`inbound.go:232`), unused by the clear path |
| engine -> SATable | Insert/Remove by SPI pair | yes, read 2026-07-16: `table.go:19` Insert (rejects duplicate SPI pair), `:52` Remove, `:63` UpdateKey |

### Integration Points
- `internal/component/ike/engine/` SATable, PeerSession, reconcile, responder accept
- Event bus sa-up/sa-down (`internal/component/ike/engine/events.go`)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The initiator already re-initiates after clear (reconcilePeers -> startPeerSession) | reconcile.go:227-250; observed 2 IKE_SA_INIT sent | fix must also cover the initiator side | packet-count trace / unit test | **confirmed** 2026-07-16 -- read the producers: `TerminateAllSAs` deletes from `activePeersMap` (`register.go:94-97`), which IS the same map object `reconcilePeers` reads as `active` (`register.go:191-192` + `reconcile.go:211` capture the one reference), so the "already running" check (`reconcile.go:228-233`) misses and `startPeerSession` (`:245`) runs `runInitiator` (`fsm.go:85`) with a fresh SPI. No initiator-side change needed for the re-init itself. |
| A-2 | The drop is on the responder inbound-init path while it holds a stale established SA | tryResponderSAInit register.go:527-564; observed 1 accepted of 2 | fix is elsewhere (dispatch routing, SATable lookup) | add responder-side trace at the drop point during research | **confirmed and narrowed** 2026-07-16 -- the drop is the `responderBusy` CAS at `register.go:546`, not a routing/lookup failure. `responderBusy` is set at `:546` and cleared only at `fsm.go:185` (function entry), `:224` (post-teardown), `:235` (StateDead), `:263` (pre-established reap only, which explicitly refuses to reap an established SA at `fsm.go:256-258`). No clear-while-established path exists. Exhaustive: `grep -rn "responderBusy" internal/`. **No trace instrumentation is needed; the drop already logs at `register.go:547`.** |
| A-3 | RFC 7296 permits accepting a new IKE_SA_INIT superseding/coexisting with the stale SA | RFC 7296 Section 1.2 / 2.8 | narrow the fix to stale-SA teardown before accept | rfc/short/rfc7296.md summary | **broken as written** 2026-07-16 -- split in two. **Coexisting: confirmed** (nothing forbids a second half-open IKE SA; the SATable keys on the SPI pair, `table.go:19-28`, so a new initiator SPI cannot collide). **Superseding on the init: BROKEN.** RFC 7296 §2.4: "An endpoint MUST NOT conclude that the other endpoint has failed based on any routing information ... or IKE messages that arrive without cryptographic protection." The `IKE_SA_INIT` is unauthenticated, so it MUST NOT delete the old SA. Supersede is only permitted once the new SA is **authenticated**, and §2.4's INITIAL_CONTACT is the named mechanism ("MAY use this information to delete any other IKE SAs it has to the same authenticated identity without waiting for a timeout"). Mistake Log + Deviations rows added. Also note §2.8 (cited in the original Basis) is about **rekeying**, not re-initiation, and does not govern this at all. |
| A-4 | The responder's Delete-handling path already tears the tunnel down correctly, so Phase A needs no responder change | `handleInformationalOwned` (`inbound.go:248`), `handleDeletePayload` (`inbound.go:282`) exist and are reached from the owner loop (`established.go:113-114`) | Phase A grows a responder-side change and its estimate doubles | read `handleDeletePayload` in full + the `TestResponderTearsDownOnDelete`-style unit test in the /ze-implement audit, BEFORE writing Phase A code | **unvalidated** -- the functions exist and are wired, but this design pass did not read `handleDeletePayload` (`inbound.go:282-296`) in full. Not a blocker for approval; it is the first audit item. |
| A-5 | The clear path can send the Delete from the owner goroutine without a new race | single-owner model: `sa.NextMsgID` and `sa.SKKeys` are mutated only by `maintainSA` (`inbound.go:33-35` doc, `established.go:104` loop); `sendDeleteIKE` bumps `sa.NextMsgID` (`inbound.go:240`) | the Delete must be built on the owner loop via a signal, not called from the RPC goroutine | `go test -race` on the new unit test + `make ze-unit-test` | **confirmed as a constraint** 2026-07-16 -- `sendDeleteIKE` mutates `sa.NextMsgID` (`inbound.go:240`) and reads `sa.SKKeys` via `buildEncryptedMessageEx` (`inbound.go:236`). `TerminateAllSAs` runs on the RPC goroutine (`cmd/ipsec.go:29`/`:39`). -> Constraint: the design MUST route the Delete through the owner loop's `stopCh` case (`established.go:106-108`), never call `sendDeleteIKE` from `TerminateAllSAs`. |
| A-6 | Convergence today is ~150-190s (DPD self-heal), not "never" | composed from `dpd.go:34`/`:69`, `ipsec/config.go:26-27` (30s/120s defaults), `established.go:145-149`, `fsm.go:218-224`, `fsm.go:35`/`:703`/`:50`/`:37` | the framing "tunnel stays down forever" would be right and the fix urgency changes | the two-ze `.ci` with a long timeout, observing recovery without any fix | **derived, unvalidated end-to-end** -- every component was read, but the ~150-190s figure is a composition, not an observation. Cheap to settle: run the new `.ci` against unfixed code with a 240s timeout and confirm it goes green late. Worth doing once: it proves the test detects the fix rather than a timing coincidence. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Accepting new inits opens a DoS / SA-churn vector (an attacker forces re-negotiation) | SA table growth, CPU under repeated inits | **Sharpened 2026-07-16.** This is not a side risk, it is the reason RFC 7296 §2.4 bans supersede-on-init: a spoofed `IKE_SA_INIT` from the peer's address would otherwise be an unauthenticated remote tunnel-teardown primitive. Mitigation is structural, not a rate limit: **never** tear the old SA down on an unauthenticated message; only on authenticated IKE_AUTH (Phase B). Bound the parallel-SA count to 1 in-flight (keep a `responderBusy`-equivalent gate scoped to *half-open* SAs, which is what the field's doc comment `reconcile.go:24-29` already claims it means). Existing defenses stay: source match `matchResponderPeer` (`register.go:501`), token bucket 100/s burst 200 (`register.go:409`, `:570`), 30s half-open reap (`fsm.go:248`). |
| R-2 | Superseding a live SA drops in-flight traffic on a working tunnel | traffic gap on supersede | **Refined.** With supersede-on-authenticated-IKE_AUTH only, a working tunnel can only be replaced by a peer that proves the PSK/cert. The old Child SA stays installed until the new one authenticates (make-before-break, the pattern `supersededChild` already uses, `reconcile.go:47-48`). AC-4 guards the no-clear case. |
| R-3 | Fix changes wire behavior and breaks strongSwan interop | interop scenario red | add/adjust `test/ipsec-interop/` scenario; validate against strongSwan (interop rule). **Assessed 2026-07-16: real for Phase A and required.** The Delete on clear is a message ze has never sent in this context. See TDD Test Plan for the two scenarios and which direction proves what. |
| R-4 | Phase A's Delete is best-effort UDP; a lost Delete silently reverts to the ~150s DPD path | clear looks fast in tests, slow in the field under loss | Accept: `sendDeleteIKE` is already documented best-effort (`inbound.go:230-231`) and RFC 7296 §1.4 does not require a Delete to be retransmitted for correctness. Phase B is the real backstop (responder accepts the init regardless). **This is the strongest argument that A alone is not sufficient.** |
| R-5 | Phase B's per-SA routing change touches the single-owner model that spec-ipsec-13 paid for in races | `go test -race` red; flaky rekey/DPD tests | Change `routeInbound` (`register.go:482`) to key the owner-loop decision on the SA identity, not the peer name, and keep every post-establishment mutation on `maintainSA`. Run `-race` on the whole engine package, not just new tests. Do NOT widen `ps.sa` into a slice; add one explicit second slot for the in-flight responder SA. |
| R-6 | The graceful-close flag makes `ps.Stop()` mean two different things (config-change stop vs operator clear) | a config change starts sending Deletes, or the clear stops sending them | Keep `Stop()` as-is and add a distinct entry point (e.g. a graceful variant) used only by `TerminateAllSAs`/`TerminatePeerSA`. Decide explicitly whether `reconcilePeers`' removal path (`reconcile.go:205-224`) should ALSO send a Delete -- arguably yes (removing a peer from config should say goodbye), but that is a behavior change beyond this spec's scope. **Open question for Thomas.** -> RESOLVED (Q2, 2026-07-17, readiness pass): NO -- keep `Stop()` unchanged in the removal path; out of scope, noted follow-up. See Open question 2 resolution below. |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `clear vpn ipsec sa` on the initiator daemon | -> | graceful close (Delete) + re-established SA against a live responder | `test/ipsec/ipsec-clear-reestablish.ci` |
| `clear vpn ipsec sa peer <name>` | -> | per-peer graceful close + re-establish | `test/ipsec/ipsec-clear-reestablish-peer.ci` |
| responder receives a fresh IKE_SA_INIT while holding a stale SA | -> | responder parallel-accept path (`register.go:546` gate scope, `fsm.go` transition) | `TestResponderAcceptsReinitAfterStaleSA` (unit) |
| responder receives an INFORMATIONAL Delete on an established SA | -> | existing teardown -> gate + SATable freed -> accept re-init | `ipsec-clear-reestablish.ci` (end to end); A-4 audit confirms the handler |
| strongSwan responder receives ze's Delete then ze's re-init | -> | wire conformance | `test/ipsec-interop/scenarios/10-clear-reestablish/check.py` |
| strongSwan initiator re-inits against a ze responder holding a stale SA | -> | ze responder conformance (RFC 7296 §2.4) | `test/ipsec-interop/scenarios/11-responder-accepts-reinit/check.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two ze instances (initiate + respond) with an established tunnel; operator runs `clear vpn ipsec sa` on the initiator | The tunnel re-establishes **within 10 seconds** (bound derived in Boundary Tests); `show vpn ipsec sa` on the initiator reports the peer established again **with a different SPI pair** than before the clear (proving a real bounce, not a no-op). Phase 2. |
| AC-2 | Same, `clear vpn ipsec sa peer <name>` | Same re-establishment for that peer only; other peers untouched | 
| AC-3 | Responder holds an established SA and receives a fresh IKE_SA_INIT (new initiator SPI) from the configured peer | ~~Responder accepts the new SA (superseding/coexisting per the RFC rule the fix documents), not a silent drop~~ **Refined 2026-07-16 (A-3 broken):** the responder **accepts the new SA in parallel and does NOT touch the old one** (RFC 7296 §2.4: MUST NOT act on unauthenticated messages). The old SA is removed only once the new SA completes IKE_AUTH. Phase 3. |
| AC-4 | An established, healthy tunnel with no operator clear | No spurious re-negotiation, no traffic gap (R-2 guard) |
| AC-5 | Wire behavior verified against strongSwan | Both interop scenarios pass: `10-clear-reestablish` (ze initiator: strongSwan accepts our Delete and the re-init) and `11-responder-accepts-reinit` (ze responder: accepts strongSwan's re-init while holding a stale SA, honors its INITIAL_CONTACT). "N/A" is **not** available here -- the interop determination in the TDD Test Plan rules it required. Phase 5. |
| AC-6 | An unauthenticated `IKE_SA_INIT` arrives from the peer's address and never completes IKE_AUTH | The established SA and its Child SA **survive** unchanged; the half-open SA is reaped after `responderHandshakeTimeout` (`fsm.go:43`, 30s). This is the R-1 DoS guard and the RFC §2.4 requirement, stated as an AC so it cannot be dropped as "just a risk". Phase 3. |
| AC-7 | Peer restarts/crashes without sending a Delete while ze (responder) holds an established SA | ze accepts the peer's fresh IKE_SA_INIT and the tunnel re-establishes without waiting for the ~150s DPD timeout. This is the case Phase A cannot fix (R-4) and the reason Phase B exists. Phase 3/4. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator bounces a stuck tunnel with `clear vpn ipsec sa` | clear -> teardown -> re-initiate -> responder accept -> established | `test/ipsec/ipsec-clear-reestablish.ci` |
| 2 | Operator clears one peer of many | `clear vpn ipsec sa peer <name>` -> that peer re-establishes | `test/ipsec/ipsec-clear-reestablish-peer.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResponderAcceptsReinitAfterStaleSA` | `internal/component/ike/engine/responder_test.go` | AC-3. **Red today at `register.go:546`.** Shape (mirror the existing `responder_test.go:20-66` harness, which already drives `tryResponderSAInit` with a synthetic packet and asserts on `table.Len()` / `ps.responderBusy` / `ps.getSA()`): put `ps` in the established shape (`responderBusy=true`, `established=true`, an SA in `StateEstablished`), feed a fresh `IKE_SA_INIT` with a NEW initiator SPI, assert the new SA is inserted and the OLD SA is **still present** (coexist, not supersede -- this is the RFC §2.4 assertion). | |
| `TestResponderKeepsOldSAOnUnauthenticatedInit` | `internal/component/ike/engine/responder_test.go` | R-1 / A-3: the old SA and its Child survive an init that never authenticates. Negative test; this is the DoS guard and must not be skipped. | |
| `TestResponderSupersedesOnAuthenticatedInit` | `internal/component/ike/engine/responder_test.go` | AC-3 second half: once the new SA reaches `StateEstablished`, the old SA is removed from the table and its Child cleaned up. | |
| `TestRouteInboundKeysOnSANotPeer` | `internal/component/ike/engine/register_test.go` | R-5 / coupling #1: with an established SA for peer X, a packet for a DIFFERENT SA of peer X is NOT delivered to the owner loop. Red today (`register.go:486`). | |
| `TestClearSendsIKEDelete` | `internal/component/ike/engine/established_test.go` | Phase A: the owner loop emits an INFORMATIONAL Delete on graceful stop and not on a plain stop. Assert on the wire bytes via the existing transport seam. | |
| `TestTerminateAllSAsReinitiates` | `internal/component/ike/engine/reconcile_test.go` | AC-1 clear triggers re-initiation (guards the `activePeersMap`/`active` same-map identity that A-1 depends on -- a refactor that copies the map silently breaks clear). | |

-> Constraint: `responder_test.go:20-66` already builds exactly the harness these
tests need (synthetic `IKE_SA_INIT`, fake table, `ps` with `responderBusy`).
Extend it; do not build a second one.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| re-establish bound after clear (seconds) | 0 < t <= 10 | 10s (AC-1 assertion bound) | n/a (faster is pass) | > 10s = FAIL. Derivation: Phase A costs 1 Delete (~0 RTT, fire-and-forget) + the responder's teardown + one full IKE_SA_INIT/IKE_AUTH handshake (2 RTT on loopback, sub-second). 10s is ~10x headroom for CI load, and is 15x below the current ~150s so the test cannot pass by accident on the unfixed path. |
| `responderHandshakeTimeout` (existing, `fsm.go:43`) | 30s | unchanged | - | Do NOT retune. It bounds half-open reaping and is orthogonal; touching it would mask Phase B regressions. |
| DPD interval / timeout (existing, `ipsec/config.go:26-27`) | 30s / 120s | unchanged | - | Explicitly NOT retuned; see Key Design Decisions alternative (iv). |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-clear-reestablish.ci` | `test/ipsec/` | operator clear -> tunnel re-establishes (two-ze, engine-step directives, noop dataplane) | |
| `ipsec-clear-reestablish-peer.ci` | `test/ipsec/` | per-peer clear -> that peer re-establishes | |

-> Decision: **copy `test/ipsec/ipsec-sa-installed.ci` as the base.** It already
solves every hard part: two unprivileged ze daemons over loopback (responder
127.0.0.2, initiator 127.0.0.1), the `ze.test.ike.port` high-port seam
(`option=env:var=ze_test_ike_port:value=$PORT2`, `engine/testport.go`), the noop
dataplane opt-out (`ze_test_ike_dataplane=noop`, unprivileged XFRM is
EPERM-fatal by design), and the establishment gate. New steps: after the
establishment gate, `command=clear vpn ipsec sa`, then re-assert establishment
with a bounded timeout.
-> Constraint: **`contains=established` is NOT a valid establishment gate.**
`ipsec-sa-installed.ci:174-179` documents that it false-matches the
always-present `established-at` key while the SA is still `sa-init-sent`, and
that this once made a test pass when no SA ever formed. Gate on the negotiated
ESP algorithm (`aes-cbc` + `sha256`) via `show vpn ipsec sa | json`, as that file
does.
-> Constraint: the re-establishment assertion must distinguish the NEW SA from
the old one, or a test that never tore anything down still passes. Gate on a
changed SPI (`show vpn ipsec sa | json` carries the SPI pair) or on the sa-up
event, not merely on "a peer is established" again.
-> Constraint: `option=timeout:value=40s` in the base file must rise (the clear
adds a full second handshake). Keep it well under the ~150s DPD path so the test
proves the fix and not the fallback -- that gap is the whole point of the 10s
bound. Suggested 60s total.
-> Decision: `ipsec-sa-installed.ci:161-171` carries the `// test-relax` note
that documents THIS bug and defers it here. **Update that note when this spec
closes** (Documentation Update Checklist row 12); leaving it claiming an open
product gap after the gap is fixed is how stale deferrals are born.

### Interop Tests (MANDATORY for protocol features)

-> Decision: **an interop scenario IS required. The `.ci` tests do NOT suffice.**
Per `ai/rules/interop-and-goal-validation.md`, the exemptions are "pure internal
refactor, no wire-visible change", "config-only", and "tooling". None apply:
Phase A makes ze emit an INFORMATIONAL Delete it has never sent on this path,
and Phase B changes what ze accepts on the wire. The rule's own table names both
cases -- "Session behavior: session survives the event, peer confirms expected
behavior" and "Wire format change: peer accepts the message, no NOTIFICATION".
-> Constraint: **the two-ze `.ci` is structurally incapable of catching the
interop failure.** Both daemons share ze's bugs and ze's interpretation. A ze
responder that wrongly drops an init and a ze initiator that never sends
INITIAL_CONTACT agree with each other perfectly. Only a strongSwan peer can
falsify ze's reading of §2.4.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| ~~`NN-clear-reestablish`~~ `10-clear-reestablish` (ze initiator, strongSwan responder) | `test/ipsec-interop/scenarios/` | strongSwan | **Phase A.** ze's Delete-on-clear is understood by a real responder: strongSwan drops its SA and accepts ze's fresh IKE_SA_INIT promptly. Falsifies "our Delete is well-formed and correctly placed". Model on `01-psk-site-to-site` (ze initiator). | |
| `11-responder-accepts-reinit` (strongSwan initiator, ze responder) | `test/ipsec-interop/scenarios/` | strongSwan | **Phase B, and the higher-value one.** strongSwan re-initiates (e.g. `swanctl --initiate` after a forced local teardown, so no Delete reaches ze) while ze holds an established SA. Proves ze's responder accepts, and that ze honors strongSwan's INITIAL_CONTACT. **This is the direction where ze is currently non-conformant**, and where strongSwan's behavior is the reference. Model on `07-responder-psk` (ze responder). | |

-> Decision: numbering picks up at 10/11; existing scenarios are 01-05 and 07-09
(06 is absent -- do not reuse it, gaps are cheaper than churn).
-> Constraint: scenario `check.py` must follow the established contract
(`ai/rules/interop-and-goal-validation.md`): `wait_session`, assert the specific
behavior, verify stability after, `log_pass`/`log_fail`, raise on failure.

## Files to Modify

Resolved 2026-07-16 (research complete; each row names the function and phase).

| File | Change | Phase |
|------|--------|-------|
| `internal/component/ike/engine/established.go` | `maintainSA` stop case (`:106-108`): send the authenticated IKE Delete before `cleanupChild` when the session was stopped gracefully. Owner-goroutine only (A-5). | A |
| `internal/component/ike/engine/reconcile.go` | `PeerSession`: add the graceful-close flag; add a `StopGraceful`-style entry point beside `Stop()` (`:169`) rather than changing `Stop()`'s meaning (R-6). Phase B: add the single explicit second slot for the in-flight responder SA (not a collection). | A, B |
| `internal/component/ike/engine/register.go` | `TerminateAllSAs` (`:72`) / `TerminatePeerSA` (`:106`): request a graceful close instead of a bare `ps.Stop()`. Phase B: scope the `responderBusy` CAS (`:546`) to half-open SAs; re-key `routeInbound` (`:482-495`) on SA identity, not `sa.PeerName`; stop `tryResponderSAInit` (`:561`) clobbering the established SA via `ps.setSA`. | A, B |
| `internal/component/ike/engine/fsm.go` | `runResponder` (`:177-242`): re-arm the accept path while established, and adopt/supersede when the parallel SA authenticates. This is where the missing FSM transition lands. | B |
| `internal/component/ike/engine/responder.go` | `handleAuthRequest` (`:327`): honor a received INITIAL_CONTACT by deleting the peer's other IKE SA (RFC 7296 §2.4). | B |
| `internal/component/ike/engine/auth.go` | `buildAuthRequest` (`:90`): emit the INITIAL_CONTACT notify in the first IKE_AUTH request. Open question 3 RESOLVED 2026-07-17: **UNCONDITIONAL** -- emit on every first IKE_AUTH request (one-SA-per-peer model; see Open question 3 resolution). | B |
| `internal/component/ike/cmd/ipsec.go` | **Expected: no change.** JSON result shape is preserved (Behavior to preserve). Listed only to record that it was assessed. | - |
| `test/ipsec/ipsec-sa-installed.ci` | Update the `// test-relax` note (`:161-171`) that defers this bug here. | A |

-> Constraint: `wire/payload_notify.go` needs **no** change: `NotifyInitialContact
= 16384` already exists (`:27`) and `PayloadNotify` encode/decode is generic. Do
not add a new payload type.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | no new config surface. Confirmed 2026-07-16: the fix adds no tunable. The 10s bound is a test assertion, not a config leaf; the DPD knobs already exist (`ipsec/config.go:287`) and are explicitly not retuned. |
| CLI commands/flags | N/A | `clear vpn ipsec sa` already exists (`cmd/ipsec.go:12-15`), JSON shape preserved |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-clear-reestablish*.ci` |
| Interop scenario (protocol wire change) | **Yes** | `test/ipsec-interop/scenarios/10-clear-reestablish/`, `11-responder-accepts-reinit/` -- required, see TDD Test Plan determination |
| Doctor check for runtime dependencies | N/A | no new runtime dependency |
| Prometheus counters/metrics | N/A -- **decided 2026-07-16** | Considered and rejected for this spec. `RegisterMetrics` (`engine/metrics.go`) already exposes `sa_count`, `tunnel_up` and `rekey_total`; a re-negotiation counter would be a genuinely useful DoS signal for R-1 (repeated inits) but it is a new observable surface, not part of fixing the clear path, and `rekey_total`'s gauge-not-counter caveat (learned 745) says the shape needs its own thought. -> Decision: out of scope; note it in the learned summary as follow-up. |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No -- behavior fix | `docs/features.md` only if the IPsec row overstates clear semantics; verify at closure |
| 3 | CLI command added/changed? | **Yes** (semantics) | `docs/guide/command-reference.md` -- `clear vpn ipsec sa` now performs a **graceful** close (tells the peer) and re-establishes in seconds. The grammar is unchanged; the observable behavior is not. |
| 9 | RFC behavior implemented/changed? | **Yes** | `rfc/short/rfc7296.md` (§2.4 State Synchronization + INITIAL_CONTACT -- via `/ze-rfc`, BEFORE the enforcing comments are written), `docs/features/rfc-status.md` (INITIAL_CONTACT goes from unimplemented to implemented; the §2.4 checklist rows at `rfc/short/rfc7296.md:462-469` may need a new row for the accept-parallel-init rule) |
| 11 | Affects daemon comparison? | Verify at closure | `docs/comparison.md` -- only if it claims anything about IPsec re-establishment/uniqueids vs strongSwan |
| 12 | Internal architecture changed? | **Yes** | `ai/digests/ipsec-ike.md` (the responder gate lifetime and the parallel-SA window are exactly the kind of thing a digest must carry); `test/ipsec/ipsec-sa-installed.ci:161-171` (the `// test-relax` note that defers this bug here must be updated once it is fixed) |

## Files to Create
- `test/ipsec/ipsec-clear-reestablish.ci`, `test/ipsec/ipsec-clear-reestablish-peer.ci`
  (copy `test/ipsec/ipsec-sa-installed.ci` as the base -- it already carries the
  two-daemon loopback setup, the `ze_test_ike_port` seam and the noop dataplane)
- ~~`internal/component/ike/engine/<name>_test.go` (unit tests above)~~
  **Resolved 2026-07-16: no new test file.** The unit tests extend existing
  files: `responder_test.go` (whose `:20-66` harness already drives
  `tryResponderSAInit`), `register_test.go`, `established_test.go`,
  `reconcile_test.go`. `ai/rules/before-writing-code.md` / design-principles:
  extend, do not recreate.
- `test/ipsec-interop/scenarios/10-clear-reestablish/` (ze initiator + strongSwan
  responder; `ze.conf`, `strongswan.conf`/`swanctl.conf` per the existing
  scenario layout, `check.py`) -- **required**, not conditional
- `test/ipsec-interop/scenarios/11-responder-accepts-reinit/` (strongSwan
  initiator + ze responder) -- **required**; model on `07-responder-psk`
- `plan/learned/NNN-fixit-ipsec-clear-reestablish.md` at closure (number from
  `plan/learned/.counter`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Risks & Assumptions (validate A-1..A-3 first) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 13. /ze-review gate | Review Gate section |
| 14. Present + close | Executive Summary; two-commit closure |

### Implementation Phases

~~1. **Phase: Research + reproduce** - add a responder-side trace at the drop point, confirm A-2, pin the exact drop location; write the failing unit + `.ci`.~~
**Superseded 2026-07-16: research is complete and the drop point is pinned
(`register.go:546`). No trace instrumentation is needed -- the drop already logs
"responder busy, dropping concurrent IKE_SA_INIT" at `register.go:547`.** Phases
renumbered below. 5 phases; set `Phase | 1/5` when coding starts.

1. **Phase 1: Red tests.** Write `TestResponderAcceptsReinitAfterStaleSA` and
   `TestRouteInboundKeysOnSANotPeer` (both red today) plus
   `ipsec-clear-reestablish.ci`. Settle A-6 here: run the `.ci` against unfixed
   code with a 240s timeout and confirm it goes green late (~150-190s), then set
   the real bound to 10s. This proves the test detects the fix and not a timing
   coincidence, and it is the cheapest moment to do it.
2. **Phase 2 (A): graceful Delete on clear.** `StopGraceful` + owner-loop Delete
   + `TerminateAllSAs`/`TerminatePeerSA` wiring. `.ci` green under the 10s bound.
   Audit A-4 first (read `handleDeletePayload` `inbound.go:282-296` in full)
   before writing code -- if the responder's Delete handling is not complete,
   Phase 2 grows and the estimate changes.
3. **Phase 3 (B): responder accepts a parallel init.** Scope `responderBusy` to
   half-open; re-key `routeInbound` on SA identity; second SA slot; the
   `runResponder` established -> accept transition. Supersede ONLY on
   authenticated IKE_AUTH. `go test -race ./internal/component/ike/...` (R-5).
4. **Phase 4 (B): INITIAL_CONTACT.** Emit in the first IKE_AUTH request; honor on
   receipt. Depends on Phase 3 (the init must be accepted before message 3 is
   reachable). Requires the `/ze-rfc` §2.4 summary update first, so the enforcing
   comment cites a citable artifact.
5. **Phase 5: interop + closure.** Scenarios `10-clear-reestablish` and
   `11-responder-accepts-reinit`; full verification; two-commit closure.

-> Constraint: Phase 4 MUST NOT precede Phase 3. INITIAL_CONTACT rides in
IKE_AUTH (message 3); a responder that drops the peer at message 1 never sees
it. Shipping 4 before 3 would look like a fix and change nothing.
-> ~~Decision: Phase 2 is independently shippable and fixes the operator-visible
bug. If open question 1 resolves as "A only", Phases 3-5 move to a new spec and
AC-3/AC-5 move with them (requires Thomas's explicit approval per
`ai/rules/no-partial-completion.md`).~~
**SUPERSEDED -> Decision (user, 2026-07-16): A and B ship together; Phases 3-5 do
NOT move to a new spec.** Phase 2 remains independently *shippable* as an
ordering property (it is still implemented first), but it is NOT independently
*sufficient*: a crashed peer sends no Delete, so without Phase B the responder
still refuses the fresh init and convergence falls back to the ~150-190s DPD path
(AC-7, R-4). All of AC-1..AC-7 stay in this spec. See Open question 1 (answered).

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | Re-establishment is bounded (<=10s); no SA leak/accumulation; working tunnels unaffected; the DPD fallback (`fsm.go:221-224`) still works |
| RFC | Enforcing code carries the exact RFC 7296 §2.4 quote, and `rfc/short/rfc7296.md` actually contains it (no dangling citation) |
| Security | R-1 mitigated **structurally**: no unauthenticated message deletes an SA. Grep the diff for any teardown reachable from `tryResponderSAInit` -- there must be none. |
| Concurrency | No encrypted message is built or sent off the owner goroutine (A-5); `routeInbound` keys on SA identity; `go test -race` clean across `internal/component/ike/...` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| clear re-establishes within 10s | `bin/ze-test ipsec --pattern clear-reestablish` |
| responder accepts re-init, keeps the old SA | `go test -run TestResponderAcceptsReinitAfterStaleSA ./internal/component/ike/engine/` |
| no new races in the owner model (R-5) | `go test -race ./internal/component/ike/...` (whole tree, not just the new tests) |
| strongSwan interop both directions | `make ze-ipsec-interop-test` (scenarios 10 and 11) |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| DoS / resource exhaustion | Repeated inits cannot grow the SA table unbounded |
| Traffic disruption | Superseding never drops a working tunnel outside the operator-clear path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails behavior mismatch | Re-read Current Behavior; RESEARCH if the drop location was wrong |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (skeleton, 2026-07-10) `fsm.go:185` resets `responderBusy` on establishment ("responder ready"), so the busy gate is NOT the drop cause for a post-establishment init | `fsm.go:185` is the **first statement of `runResponder`** (`fsm.go:177`), run once per session cycle before any handshake. `responderBusy` is cleared on establishment **nowhere**; only after `runEstablished` returns (`fsm.go:224`), i.e. after teardown. The gate IS the drop cause. | Design research 2026-07-16: read `runResponder` in full instead of the single line, then `grep -rn "responderBusy" internal/` to enumerate every writer. | High. The skeleton actively pointed research AWAY from the root cause and toward dispatch routing / SATable lookup (A-2's "If wrong" column). Textbook `ai/rules/no-fabrication.md`: the claim was inferred from a call site's log string, not from the function that produces the behavior. |
| A-3 (skeleton): RFC 7296 permits accepting a new IKE_SA_INIT **superseding** the stale SA | RFC 7296 §2.4 forbids it: "An endpoint MUST NOT conclude that the other endpoint has failed based on ... IKE messages that arrive without cryptographic protection." Coexist is permitted; supersede is legal only after authentication (§2.4 INITIAL_CONTACT). The cited Basis (§2.8) governs **rekeying**, not re-initiation. | Design research 2026-07-16: read RFC 7296 §2.4 text; the local summary `rfc/short/rfc7296.md` does not carry the rule (only a one-line INITIAL_CONTACT row at `:347`). | High. Supersede-on-init would have shipped an unauthenticated remote tunnel-teardown primitive (R-1) and passed every two-ze test, because a ze initiator never spoofs. |
| (skeleton) The tunnel "stays down until something else re-triggers it" | It self-heals via DPD in ~150s (responder) / ~150-190s end to end. The observing test's window was 40s (`ipsec-sa-installed.ci:23`). It is a latency bug, not a permanent wedge. | Design research 2026-07-16: read `newDPDState`/`timedOut` (`dpd.go:34`/`:69`), the DPD defaults (`ipsec/config.go:26-27`), and the initiator retransmit/reconnect ladder (`fsm.go:35`/`:703`/`:50`). | Medium. Does not change that it is a real defect (a 2.5-minute `clear` reads as broken), but it changes the AC (bounded 10s, not "works at all") and it is why the `.ci` must assert a tight bound: a slow test would pass on the UNFIXED code. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Free `responderBusy` when the SA establishes (the apparent one-line fix at `register.go:546` / `fsm.go`) | Rejected at design time, before code. Two other single-SA-per-peer couplings would still break it: `routeInbound` (`register.go:482-495`) keys the owner-loop decision on `sa.PeerName`, so the accepted init is handed to the OLD SA's `maintainSA` and fails to decrypt under the old keys; and `ps.setSA` (`register.go:561`) clobbers the established SA pointer. It would look correct, pass a naive unit test, and misroute on the wire. | Phase 3: scope the gate to half-open SAs **and** re-key `routeInbound` on SA identity **and** add an explicit second SA slot. |
| Shrink the DPD defaults so the responder self-heals faster | Treats the symptom. Degrades a correct RFC §2.4 liveness knob for every deployment to paper over a gate bug, and still leaves a multi-second clear. | Phases 2-4 (say goodbye; accept the re-init). |

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->

- **2026-07-16: the skeleton's one "ruled out" line was the answer.** The spec
  author wrote off `responderBusy` (`fsm.go:185`) as "NOT the drop cause" and
  sent research toward dispatch routing and SATable lookup. It was the drop
  cause. The error is the exact one `ai/rules/no-fabrication.md` describes:
  `fsm.go:185` was read as a *call site* ("responder ready" -> sounds like an
  establishment hook) instead of reading the *function* it opens
  (`runResponder`, `fsm.go:177`), where it is plainly the first statement of a
  loop that has not yet seen a packet. One `sed -n '177,242p'` would have
  settled it. Cost: a skeleton that pointed three sessions at the wrong files.
- **A gate whose doc comment is right and whose code is wrong.**
  `reconcile.go:24-29` says "runResponder clears it once the SA establishes or
  dies". The "or dies" half exists (`fsm.go:224`, `:235`). The "establishes"
  half was never written. When a field's comment and code disagree, the comment
  is the spec and the code is the bug -- here it means the fix is a restoration,
  not a redesign, which is why Phase B is smaller than it first looks.
- **The RFC forbids the obvious fix.** "Responder sees a new init, drops the old
  SA, accepts" is what every instinct suggests and it is an unauthenticated
  remote teardown primitive. RFC 7296 §2.4 anticipated it. The legal shape is
  coexist-then-supersede-on-authentication, which is why INITIAL_CONTACT exists
  at all and why it rides in IKE_AUTH (message 3) rather than IKE_SA_INIT
  (message 1).
- **INITIAL_CONTACT cannot be the fix on its own, only the completion of it.**
  It is defined (`wire/payload_notify.go:27 NotifyInitialContact = 16384`) and
  never sent or handled anywhere (`grep -rni initial_contact internal/` returns
  only the constant). But it arrives at message 3, and ze drops the peer at
  message 1. Accepting the parallel init is the prerequisite; INITIAL_CONTACT is
  what makes the supersede prompt and legal once the init is accepted.
- **The bug is invisible to the ze-vs-ze rekey tests because rekey never
  re-inits.** Rekey uses CREATE_CHILD_SA on the existing SA (`inbound.go:114`),
  which never goes near `tryResponderSAInit`. That is why `05-child-rekey` and
  `09-responder-ike-rekey` are green while the re-init path is broken. Any test
  that proves this fix must force a **fresh IKE_SA_INIT against a live
  responder** -- the rekey path cannot stand in for it.
- **The reported symptom was a test-timeout artifact, and the honest version is
  a better bug.** "Responder never accepts" (permanent wedge) is wrong;
  "operator clear takes ~150s because it waits for the far side's DPD timeout"
  is right, and is still a real product defect: an operator who types `clear`
  and waits 2.5 minutes concludes the command is broken. Framing it accurately
  also explains why Phase A (say goodbye) is both sufficient for the operator
  path and insufficient overall (R-4).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| **Fix in two phases: (A) authenticated Delete on operator clear, then (B) responder accepts a parallel init and supersedes on authenticated IKE_AUTH.** | (i) B only; (ii) A only; (iii) supersede-on-init; (iv) shrink the DPD defaults | A alone cannot survive a peer crash/reboot or a lost Delete (R-4): a dead peer sends no goodbye, and that is the case the responder gate must handle. B alone leaves the operator-clear path re-handshaking against a responder still holding a dead tunnel, and depends on Phase B's larger blast radius landing before the operator-visible bug is fixed. (iii) is banned by RFC 7296 §2.4 (see A-3). (iv) treats the symptom, degrades a correct RFC §2.4 liveness knob for every deployment, and still leaves a multi-second clear. A is small, authenticated, RFC §1.4, and independently shippable; B is the correctness fix. |
| **Phase A sends the Delete from the owner loop's `stopCh` case, not from `TerminateAllSAs`.** | Call `sendDeleteIKE` directly in `TerminateAllSAs` (RPC goroutine) | `sendDeleteIKE` mutates `sa.NextMsgID` (`inbound.go:240`) and reads `sa.SKKeys`, both owned by `maintainSA` (`inbound.go:33-35`). Calling it from the RPC goroutine is a data race on the exact state spec-ipsec-13 centralized. A-5 confirms this as a constraint. |
| **Phase B keeps a half-open gate; it does not delete `responderBusy`.** | Remove the gate entirely; allow N parallel SAs | The gate's stated purpose (one *in-flight handshake* per responder peer, `reconcile.go:24-29`) is correct and is a real DoS defense (R-1). Only its *scope* is wrong: it must cover half-open SAs, not established ones. Restoring the documented meaning is the fix. |
| **Phase B routes on SA identity, not peer name.** | Keep `routeInbound`'s peer-name keying and special-case IKE_SA_INIT | Peer-name keying (`register.go:486`) is the misroute (coupling #1 in Current Behavior). Special-casing one exchange type leaves the same trap for the new SA's IKE_AUTH, which would still be handed to the old SA's owner loop and fail to decrypt. Fix the keying once. |
| **Do not extend `ps.sa` into a collection.** | `[]*SA` / map per peer | Two SAs coexist only transiently (during the new handshake). One explicit second slot for the in-flight responder SA expresses that; a collection invites unbounded accumulation and hides the lifecycle (R-1). |

### Open question for Thomas (BLOCKING approval)

| # | Question | Why it needs a decision |
|---|----------|------------------------|
| 1 | ~~**Ship A and B together in this spec, or land A first and split B into its own spec?**~~ **ANSWERED 2026-07-16 -- no longer open.** | ~~A is ~1 day and fixes the operator-visible bug (AC-1/AC-2). B is the RFC-conformance fix (AC-3) and touches the single-owner model that spec-ipsec-13 paid for in races (R-5) -- it is the larger, riskier half, and it is really "ze's responder is not RFC 7296 §2.4 conformant", which is a different bug than "clear is slow". The spec's ACs currently demand both. My recommendation: **keep both here, implement A then B in phases**, because AC-3 is where the actual protocol defect lives and splitting it risks it never landing. But if the priority is the operator bug, A-only + a skeleton spec for B is legitimate and I will not make that call unilaterally (`ai/rules/no-partial-completion.md`: scope reduction needs explicit approval).~~ **-> Decision (user, 2026-07-16): ship Phase A AND Phase B together in this spec.** Scope is NOT reduced; the A-only option is closed. Rationale (Thomas): **A alone leaves ze non-conformant**, because a crashed peer sends no Delete, so the responder still refuses the fresh init and convergence falls back to DPD (~150-190s). The operator-visible fix and the RFC 7296 §2.4 conformance fix ship as one unit. All of AC-1..AC-7 stay in this spec. Do not re-open this as "land A first"; it was considered and rejected on the merits above. |
| 2 | **Should `reconcilePeers`' peer-removal path (`reconcile.go:205-224`) also send a Delete?** | Removing a peer from config is also a "say goodbye" moment, and the same owner-loop mechanism would serve it. It is a behavior change outside this spec's stated scope (R-6). Arguably correct, arguably a separate spec. |
| 3 | **Does Phase B's initiator send INITIAL_CONTACT unconditionally on every first IKE_AUTH?** | RFC 7296 §2.4 says it asserts "this IKE SA is the only IKE SA currently active between the authenticated identities" -- true for ze's one-SA-per-peer model, so unconditional is defensible and is what strongSwan does by default (`uniqueids=yes`). But it changes every ze IKE_AUTH on the wire, which widens the interop surface beyond the clear path. Alternative: send it only on a session started by an operator clear. |

-> AUTONOMOUS DEFAULT (2026-07-17, readiness pass). Question #1 was already
answered by Thomas above (A and B ship together; all AC-1..AC-7 stay in this
spec) and is NOT re-touched. Questions #2 and #3 are resolved here so a fresh
implementer picks this up with zero questions:

- **Q2 -> NO (out of scope for this spec).** `reconcilePeers`' peer-removal path
  (`reconcile.go:203-224`, `r.ps.Stop()` at `:205`) keeps **bare `ps.Stop()`
  semantics unchanged**; it does NOT gain a goodbye Delete. Rationale: removing a
  peer from config sending a Delete is a behavior change beyond the
  operator-clear fix, and folding it in here would leak R-6's "`Stop()` means two
  things" hazard into the removal path (a routine config edit would start
  emitting Deletes). The A+B design already isolates the graceful close to a
  distinct `StopGraceful`-style entry point used ONLY by
  `TerminateAllSAs`/`TerminatePeerSA` (Files to Modify, `reconcile.go` row); the
  removal loop deliberately does not call it. Recorded as a noted follow-up for a
  separate spec (also note in the closure learned summary). Thomas: override if
  wrong (i.e. if config-removal should also say goodbye).
- **Q3 -> UNCONDITIONAL. [STAKES: protocol]** The Phase B initiator emits
  INITIAL_CONTACT on **every** first IKE_AUTH request (`auth.go:90
  buildAuthRequest`), not only on operator-clear-originated sessions. Rationale:
  RFC 7296 §2.4 defines INITIAL_CONTACT as asserting "this IKE SA is the only IKE
  SA currently active between the authenticated identities". ze is a
  one-SA-per-configured-peer model (single `ps.sa` slot, `reconcile.go:22`;
  `setSA`/`getSA` at `:72`/`:78`; no multiple tunnels per identity pair), so on
  any fresh IKE_AUTH that assertion is truthful, which makes unconditional both
  correct and the simplest option -- no operator-origin state to thread through
  the FSM, and rekey never reaches this path (it uses CREATE_CHILD_SA on the
  existing SA, `handleCreateChildSAOwned` `inbound.go:114`, not a fresh
  IKE_AUTH). It also matches strongSwan's default (`uniqueids=yes`). **Override
  caveat (why this is [STAKES: protocol]):** it changes every ze IKE_AUTH on the
  wire, and its correctness rests on the one-SA-per-identity assumption. If ze
  ever supports multiple distinct tunnels between the same authenticated
  identities, unconditional INITIAL_CONTACT would wrongly delete the sibling
  tunnel and must become conditional (send only on a clear-originated session).
  Thomas: override before Phase 4 ships if that model is planned.

### Constraints forced by the 2026-07-16 A+B decision

-> Constraint (user decision, 2026-07-16): **the one-line fix to the
`responderBusy` gate is WRONG and MUST NOT be shipped as "Phase B".** Freeing the
CAS at `register.go:546` alone leaves two further single-SA-per-peer couplings
that bite, both re-verified against the producers on 2026-07-16:
1. `routeInbound` (`register.go:482-495`) decides the owner loop from
   `lookupPeerSession(sa.PeerName)` at `register.go:486` -- it keys on the **peer
   name, not the SA**. So the accepted init reaches the OLD SA's owner loop and
   fails to decrypt under the old SA's keys. `tryResponderSAInit` itself ends by
   calling `routeInbound` (`register.go:563`), so even the accepted init is
   misrouted.
2. `ps.setSA(sa)` at `register.go:561` publishes the new responder SA into the
   single `ps.sa` slot, **clobbering the established SA** that `runResponder`'s
   poll loop and `TerminateAllSAs` read.
   A one-line gate fix would look right, pass a naive unit test, and misroute on
   the wire. Phase B is the three-part change (gate scope + per-SA routing +
   explicit second slot), never the one-liner. This restates the Failed Approaches
   row as a binding constraint so it cannot be re-discovered as a shortcut.

-> Constraint (user decision, 2026-07-16): **interop scenario
`11-responder-accepts-reinit` (strongSwan initiator, ze responder) is the
higher-value test of the two and is not droppable.** A two-ze `.ci` is
**structurally incapable** of catching this defect: both ends share ze's bugs and
ze's reading of §2.4, so a ze responder that wrongly drops an init and a ze
initiator that never sends INITIAL_CONTACT agree with each other perfectly. Only
a strongSwan peer can falsify ze's interpretation. AC-5 therefore cannot be
satisfied by functional tests alone, and `11-` is the scenario that proves the
conformance half (Phase B) that the A+B decision exists to ship.

## Known Limitations
- ~~Re-establishment timing depends on the initiator's connect-retry cadence; the bound is defined during design.~~
  **Resolved 2026-07-16:** after Phase 2 the clear path does not depend on the
  retry cadence at all -- the Delete tears the responder down in ~1 RTT and the
  already-scheduled fresh `IKE_SA_INIT` is accepted immediately. Bound: 10s
  (AC-1).
- Phase A's Delete is best-effort UDP (`inbound.go:230-231`). A lost Delete falls
  back to the ~150s DPD path until Phase 3 lands (R-4).
- The peer-crash case (AC-7) stays broken until Phase 3. A crashed peer sends no
  Delete, and no amount of initiator-side politeness fixes a responder that
  drops the init.
- `close-action` is parsed (`ipsec/types.go:226-241`, values none/start/restart)
  but **not consumed anywhere in the engine** (`grep -rn CloseAction
  internal/component/ike/` hits only `types.go` and `config.go`). It is the
  config knob an operator would expect to govern re-establishment after a close.
  Out of scope here; recorded because it is adjacent and someone will ask.
- MOBIKE and the NAT-T `0.0.0.0:4500` hardcoded-port note remain out of scope per
  the Task section.

## RFC Documentation

Add `// RFC 7296 Section X.Y: "<quoted requirement>"` above the responder accept/supersede code.

Resolved 2026-07-16 -- the exact citations and their sites:

| Site | Citation |
|------|----------|
| The `responderBusy` gate scope (`register.go:546`) and the `runResponder` established -> accept transition (`fsm.go:177-242`) | RFC 7296 §2.4: "An endpoint MUST NOT conclude that the other endpoint has failed based on any routing information ... or IKE messages that arrive without cryptographic protection." -- i.e. why the old SA is kept while the new init is accepted in parallel. |
| The supersede point (old-SA teardown after the new SA authenticates) | RFC 7296 §2.4: "The INITIAL_CONTACT notification asserts that this IKE SA is the only IKE SA currently active between the authenticated identities" / the recipient "MAY use this information to delete any other IKE SAs it has to the same authenticated identity without waiting for a timeout." |
| The INITIAL_CONTACT emit site (`auth.go`, first IKE_AUTH request) | RFC 7296 §2.4: "The INITIAL_CONTACT notification, if sent, MUST be in the first IKE_AUTH request or response, not as a separate exchange afterwards." |
| The Delete-on-clear site (`established.go` stop case) | RFC 7296 §1.4 (INFORMATIONAL / Delete) -- already the citation style used at `inbound.go:230-231`. |

-> Constraint: `/ze-rfc` must land the §2.4 State Synchronization content in
`rfc/short/rfc7296.md` BEFORE these comments are written. The summary currently
covers §2.4 only as DPD (`:291-300`) and INITIAL_CONTACT as a single notify-table
row (`:347`); it carries neither quoted requirement above. A code comment citing
a summary that does not contain the rule is a dangling citation.

## Implementation Summary
### What Was Implemented
- (fill at completion)
### Bugs Found/Fixed
- (fill at completion)
### Documentation Updates
- (fill at completion)
### Deviations from Plan
- (fill at completion)

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| `clear` re-establishes against a live responder, bounded | functional (two-ze) + interop | `ipsec-clear-reestablish.ci` (10s bound, SPI changed); `10-clear-reestablish` |
| ze's responder is RFC 7296 §2.4 conformant on re-initiation | interop (the only falsifying evidence) | `11-responder-accepts-reinit` -- a ze-vs-ze test cannot prove this; both ends share ze's interpretation |
| The fix does not open an unauthenticated teardown vector | security negative test | `TestResponderKeepsOldSAOnUnauthenticatedInit` (AC-6) |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated (AC-6/AC-7 added during design 2026-07-16)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

## Notes
- Authored 2026-07-10 from the `spec-test-coverage-gaps` two-ze IKE investigation
  (see that spec's Design Insights and `tmp/session/session-state-test-coverage-gaps-*.md`).
  Skeleton = captured intent with verified `file:line` evidence; moves to `design`
  when picked up. ~~The root cause is a hypothesis (A-2), not a settled fact.~~
- **2026-07-16 design pass (research complete).** Status `skeleton` -> `design`.
  A-2 is confirmed and located at `register.go:546`; A-1 confirmed; A-3 broken;
  A-4 left as the first audit item; A-5 confirmed as a constraint; A-6 derived
  and cheap to settle in Phase 1. The skeleton's `fsm.go:185` claim was false and
  is struck through in Current Behavior with a Mistake Log row. The problem
  statement is corrected from "never re-establishes" to "re-establishes in
  ~150-190s via DPD self-heal", which is still a real defect but changes the AC
  shape (bounded, SPI-changed) and is why the `.ci` needs a tight bound.
- ~~**Not moved to `ready`: three open questions need Thomas** (Key Design
  Decisions -> Open question for Thomas). The blocking one is #1: ship Phase A+B
  together here, or land A and split B (the RFC-conformance half) into its own
  spec. Scope reduction requires explicit approval
  (`ai/rules/no-partial-completion.md`); this design does not presume it.~~
- **2026-07-16, Thomas: open question #1 is ANSWERED -- ship Phase A AND Phase B
  together.** The blocking question is closed and scope is NOT reduced. Two
  constraints follow and are recorded under Key Design Decisions -> "Constraints
  forced by the 2026-07-16 A+B decision": the one-line `responderBusy` fix is
  banned (two further couplings bite), and interop `11-responder-accepts-reinit`
  is the higher-value, non-droppable scenario. **Open questions #2 and #3 remain
  open**; Status stays `design` (promotion to `ready` is Thomas's gate and has not
  been given).
- **2026-07-17 readiness pass: open questions #2 and #3 resolved by autonomous
  default; Status `design` -> `ready`.** #2 (peer-removal Delete) -> NO, out of
  scope, keep `ps.Stop()` unchanged, noted follow-up (avoids R-6's "`Stop()`
  means two things" leaking into the removal path). #3 (INITIAL_CONTACT
  conditional vs unconditional) -> UNCONDITIONAL [STAKES: protocol], correct
  under ze's one-SA-per-identity model and simplest; override caveat recorded if
  ze ever supports multiple tunnels per identity pair. Both resolutions are
  recorded APPEND-ONLY under Key Design Decisions -> Open question for Thomas.
  Q1 (A+B together) was already answered by Thomas 2026-07-16 and is untouched.
  All code and RFC 7296 §2.4 citations in this spec were re-verified against
  source during this pass; no fabrication found. The `/ze-rfc` §2.4 summary
  update remains a prerequisite before implementation writes the enforcing
  comments (RFC Documentation section).
- **Prerequisite before implementation writes RFC comments:** `/ze-rfc` for RFC
  7296 §2.4. The summary exists but carries neither normative quote the fix
  enforces, and `rfc/full/rfc7296.txt` is not in the tree.
- No production code or tests were written in this pass (design only).
