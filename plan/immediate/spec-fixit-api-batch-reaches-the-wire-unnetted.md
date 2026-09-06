# Spec: an API batch reaches the wire unnetted

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator migrating a script from ExaBGP writes several API commands in ONE
write and reads different bytes on the wire than ExaBGP put there. ExaBGP holds
the commands of one read in its outgoing RIB and lets them cancel each other
before anything is encoded. Ze translates each line on its own, sends each
route immediately, and flushes after every line, so an announce and its
withdrawal both reach the peer.

`test/exabgp-compat/api/api-fast.ci` is the recorded difference. Its script
(`test/exabgp-compat/etc/run/api-fast.run`) writes two batches through the
helper's `flush()`, which puts several newline-separated commands in one write
and waits for no ack.

| Batch | Lines written in one write | Frames ExaBGP put on the wire |
|-------|---------------------------|-------------------------------|
| 1 | announce 1.1.0.0/24, announce 1.1.0.0/25, withdraw 1.1.0.0/24, announce 1.1.0.0/25 | ONE: announce 1.1.0.0/25 |
| 2 | announce 2.2.0.0/25, announce 2.2.0.0/24, withdraw 2.2.0.0/25 | TWO: withdraw 2.2.0.0/25, then announce 2.2.0.0/24 |

Three behaviors produce those two rows, and Ze has one of them.

| # | Behavior | Ze today |
|---|----------|----------|
| 1 | A withdrawal cancels an announce of the same NLRI still queued from the same read | absent: both reach the wire |
| 2 | A repeated announce of an unchanged route sends nothing | present: `adjRIBOut.unchanged` |
| 3 | The first update pass of a session carries no withdrawal | absent: the withdrawal is sent |
| 4 | A batch's withdrawals are encoded before its announces | absent: dispatch order is write order |

The goal is that a script written for ExaBGP puts the same frames on the wire
through Ze's bridge, which is what `api-fast` measures.

`api-flow` is NOT in scope. It fails for other reasons, and none of them is
netting: see Known Limitations.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/exabgp-bridge.md` - the bridge's dispatch, flush and ack contract
  → Constraint: a route command dispatches its per-peer flush BEFORE its ack, because `done` means the route is on the wire. `api-ipv4`, `api-ipv6`, `api-mvpn` and `api-vpnv4` each caught the reverse order. A batch keeps the rule: ONE flush after the batch's last command, and every ack after that flush
  → Constraint: one ExaBGP line can become SEVERAL ze commands (ExaBGP puts each prefix on its own UPDATE), and the script is acked ONCE per line whatever the count
  → Decision: the selector travels on `Translation.Selector`, built by the translator. Nothing reads a selector back out of a finished command
- [ ] `docs/architecture/update-building.md` - the API origination rail this changes
  → Constraint: `adj_rib_out.go` names this page in its `// Design:` header, so a change to the rail's withdrawal rule edits this page in the same work

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Adj-RIB-Out (Section 3.2), Update-Send Process (Section 9.2), Withdrawn Routes (Section 4.3)
  → Constraint: RFC 4271 Section 4.3 says a withdrawn route "is identified by its destination (expressed as an IP prefix), which unambiguously identifies the route in the context of the BGP speaker - BGP speaker connection to which it has been previously advertised" (`rfc/full/rfc4271.txt`, lines 1134-1138). A withdrawal for a route this connection never advertised names nothing, which is what behavior 3 above declines to send
  → Constraint: RFC4271-9.2-1 (SHOULD NOT produce a duplicate UPDATE) is already met on this rail by `adjRIBOut`. This spec adds no requirement and lowers none, so no `## Meta` Support row moves
- [ ] `rfc/short/rfc4760.md` - MP_UNREACH_NLRI, the withdrawal form every non-IPv4-unicast family takes
  → Constraint: the netting key is family plus the NLRI as written to the wire, which is the key `adjRIBOut` already uses, so an ADD-PATH path identifier is part of it (RFC 7911 Section 3)

**Key insights:** (minimal context to resume after compaction)
- ExaBGP nets in its outgoing RIB, not at its pipe: `OutgoingRIB._del_from_rib_impl` pops a queued announce of the same route index, `add_to_rib` drops an announce already in the cache, and `updates()` yields pending withdrawals before announces (`src/exabgp/rib/outgoing.py`, upstream exa-networks/exabgp).
- The first-run asymmetry is `include_withdraw`, a per-session flag: `Peer._main` starts it False and sets it True only when the first update generator exhausts (`src/exabgp/reactor/peer/peer.py`, `_send_route_updates`); `UpdateCollection.messages` drops every withdrawn NLRI while it is False (`src/exabgp/bgp/message/update/collection.py`).
- What arrives together is what one read returned: `Processes.received` reads up to 16384 bytes once per cycle and yields every complete line in it (`src/exabgp/reactor/api/processes.py`), and one write of under `PIPE_BUF` bytes is atomic, so a `flush()` of four lines is one read.
- Ze already suppresses the duplicate announce (`adjRIBOut.unchanged`, `internal/component/bgp/reactor/adj_rib_out.go`) and already sends a withdrawal whatever the peer holds (`withdrawBatchFromPeers`), which is what batch 2 needs.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/exabgp/bridgerun/script.go` - `script.read` scans the child's stdout with a `bufio.Scanner` and calls `script.line` for each line. `script.line` translates one line, dispatches each ze command it holds, dispatches `request peer <selector> flush`, then writes one ack. Reading, dispatch, flush and ack are one goroutine, so the reader is blocked for the whole of a line's wire work
- [ ] `internal/exabgp/bridge/bridge.go` - `pluginToZebgp` is the same loop for the external runner: `bufio.Scanner`, translate, dispatch each command over MuxConn and wait for each dispatch ack, then flush, then one ack per line
- [ ] `internal/exabgp/bridge/bridge_command.go` - `Translator.Line` returns `Translation{Commands []string, Selector string, Route bool, Local, Unmatched}`. It builds `send bgp <selector> update text ... nlri <afi>/<safi> add|del <nlri>` itself, so it holds the family and the NLRI at the moment it writes them, and states nothing about them on the result
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - `handleBgpPeerFlush` resolves the selector and calls `FlushForwardPool` or `FlushForwardPoolPeer`
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `FlushForwardPoolPeer` is `fwdPool.barrierPeer`: a DRAIN barrier over a per-peer queue. It waits for queued work and computes no difference, so an announce and a withdrawal of one NLRI queued before one flush both reach the wire
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `AnnounceNLRIBatch` and `WithdrawNLRIBatch` encode and send immediately. `announceBatchToPeers` asks `built.heldBy(peer)` and skips a peer that already holds every NLRI of a unit; `withdrawBatchFromPeers` calls `forgetWithdrawn` and then sends, with no test of what the peer holds
- [ ] `internal/component/bgp/reactor/adj_rib_out.go` - `adjRIBOut` is one peer's Adj-RIB-Out over the API rails, keyed by family and the NLRI as written to the wire, valued by the attribute block emitted for that peer. `unchanged` answers false for every uncertain case, `forget` drops a key, `reset` empties the table at teardown
- [ ] `test/exabgp-compat/etc/run/exabgp_api.py` - `API.flush` writes a string and flushes it. `API.send` is `flush(command + "\n")`. Neither waits for an ack; `wait_for_ack` is a separate call the script makes when it wants one
- [ ] `test/exabgp-compat/api/api-fast.ci`, `test/exabgp-compat/etc/run/api-fast.run` - the fixture and its script, byte-identical ports of upstream `qa/api/api-fast.ci` and `etc/exabgp/run/api-fast.run`
- [ ] `test/exabgp-compat/etc/run/api-add-remove.run` - two announces written back to back, a 0.2s sleep, then two withdrawals back to back. The corroborating case: its NLRIs differ, so netting must leave all four frames in place

**Behavior to preserve:** (unless the user explicitly said to change it)
- One ack per script line, whatever the line translated into, and every ack after the batch's flush (`docs/architecture/exabgp-bridge.md`, "A route is acked after its flush").
- A line that reaches no session is still acked (`Translation.Unmatched`), and a local ack-control line is answered by the bridge itself.
- `api-ipv4`, `api-ipv6`, `api-mvpn`, `api-vpnv4`, `api-add-remove`: one route per write, so each stays a batch of one and their frame order is unchanged.
- Every non-bridge producer of `AnnounceNLRIBatch` and `WithdrawNLRIBatch` (CLI `send bgp`, the healthcheck plugin, `commit`) keeps sending on the call, with no flush required to reach the wire.
- `adjRIBOut` keeps its present key, value and false-is-safe rule.

**Behavior to change:** (only what the user asked for)
- The bridge cuts the lines of one read into one batch, nets that batch, dispatches its withdrawals before its announces, flushes once, and acks each line.
- The API withdrawal rail declines to write a withdrawal to a peer this session has sent no NLRI, and says so in the command's answer.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A script's stdout pipe. One `write(2)` of up to `PIPE_BUF` bytes is atomic, so `flush('a\nb\nc\n')` is one unit for any reader.
- Format at entry: newline-separated ExaBGP API command lines, ASCII.

### Transformation Path
1. The bridge's reader takes one read's worth of complete lines and cuts them into a batch, carrying any trailing partial line into the next batch. It does not wait for more input to grow a batch.
2. Each line is translated (`Translator.Line`) into zero or more ze commands, with the route key the translator already holds attached to each route command.
3. The batch is netted: a withdrawal cancels an announce of the same key earlier in the batch; an announce cancels nothing.
4. The batch's surviving withdrawal commands dispatch, then its surviving announce commands, then the other commands in write order.
5. `request peer <selector> flush` dispatches once per selector the batch touched.
6. One ack per original line.
7. Inside the reactor, `WithdrawNLRIBatch` tests each destination peer: a peer that has been sent no NLRI on this session is not written to, and the reason reaches the answer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Script ↔ bridge | pipe, newline-separated text, one read is one batch | No |
| Bridge ↔ ze dispatcher | `send bgp <selector> update text ...` and `request peer <selector> flush` | No |
| Command plugin ↔ reactor | `AnnounceNLRIBatch` / `WithdrawNLRIBatch` | No |
| Reactor ↔ peer | `sendUpdateWithSplit`, guarded by the per-session advertised state | No |

### Integration Points
- `Translation` (`internal/exabgp/bridge/bridge_command.go`) - gains the route key per generated command; the translator states it once, where it writes it.
- `adjRIBOut` (`internal/component/bgp/reactor/adj_rib_out.go`) - the natural home for "this session has advertised something", beside the table that already models what the peer holds and is already emptied at teardown.
- `withdrawBatchFromPeers` (`internal/component/bgp/reactor/reactor_api_batch.go`) - the per-peer decision point, next to `forgetWithdrawn`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The batch reaches the wire through the commands that already exist: `send bgp <sel> update text ...` and `request peer <sel> flush`. The bridge adds no rail |
| No unintended coupling (components stay isolated) | Yes | `internal/exabgp/bridge` gained no import. `Session.advertised` is read through `Peer.hasAdvertised`, so nothing outside the reactor names it |
| No duplicated functionality (extends existing, does not recreate) | Yes | One `BatchReader`, one `Net`, one `AnswerBatch`, one `BatchSelectors`, all in `bridge_batch.go` and called by both runners. `grep -rn "bridge.Net\|Net(batch" internal/exabgp/bridge internal/plugins/exabgp/bridgerun` names one producer and two callers |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `writeRawUpdateBody` reads `Session.advertised` before it walks the body, so an armed connection pays one atomic load on the forwarding fast path. `splitOnAdvertised` answers the caller's own slice while every peer is armed |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Nothing registers, because nothing is added: no command, no family, no leaf. `RouteKey` is stated by the four functions that already write the NLRI section, so a new route form carries a key by writing one rather than by being named in a table |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ExaBGP's netting is the outgoing RIB's, not the pipe's: a withdrawal pops a queued announce of the same route index, and an announce pops nothing | `src/exabgp/rib/outgoing.py`, `_del_from_rib_impl` ("remove previous announcement if cancelled/replaced before being sent") and `_update_rib` ("announce does NOT cancel pending withdraw. This allows withdraw+announce sequences to both be sent") | The netting rule is wrong in one direction and `api-fast` batch 2 loses its withdrawal | the unit test for the netting function, plus `api-fast` | confirmed: `TestBridgeBatchNetsWithdrawOverEarlierAnnounce` and `TestBridgeBatchAnnounceDoesNotCancelWithdraw` pin both directions, and `api-fast` passes with batch 2's two frames in the recorded order |
| A-2 | The first-run asymmetry is `include_withdraw`, a per-session flag that is False until the first update pass exhausts, and it drops every withdrawn NLRI while False | `src/exabgp/reactor/peer/peer.py` `_send_route_updates` and `_main`; `src/exabgp/bgp/message/update/collection.py` `messages` | Batch 1 keeps its 1.1.0.0/24 withdrawal and `api-fast` stays red | `api-fast`, and the unit test for the advertised-state guard | confirmed: `api-fast` passes. `TestWithdrawWithheldUntilSessionAdvertises` and `TestWithdrawSentAfterAnyNLRIAdvertised` pin the two polarities; ze reaches the same asymmetry through `Session.advertised` |
| A-3 | The route index ExaBGP nets on carries no action, so an announce and a withdrawal of one NLRI share a key | `src/exabgp/rib/change.py` `Change.index` is family plus `NLRI.index()`; `src/exabgp/bgp/message/update/nlri/nlri.py` `index` is family plus `pack_nlri` | The cancellation never fires and nothing nets | the unit test for the netting key | confirmed: `TestBridgeBatchKeyMatchesAcrossSpellings`. `RouteKey` carries family plus the NLRI tokens and the verb is not part of `routeIdentity` |
| A-4 | A batch equals one read, and one write under `PIPE_BUF` is one read for a reader that is not busy elsewhere | POSIX pipe atomicity; `src/exabgp/reactor/api/processes.py` `received` reads 16384 bytes once per cycle | Batches are cut in the wrong places; see R-1 | the batching unit test, and `api-flow`'s frame count | confirmed: `TestBridgeBatchCutsOneReadIntoOneBatch` and `TestBridgeBatchSecondReadIsSecondBatch`. `api-flow`'s first mismatch and its remaining-frame count are byte-identical with the netting on and reverted |
| A-5 | `adjRIBOut` is empty for a peer that has been sent nothing through the API rails, and holds an entry for every route sent through them | `internal/component/bgp/reactor/adj_rib_out.go` `record`, called by `announceBatchToPeers` through `built.recordAll` | The advertised-state guard reads the wrong state and either drops a legitimate withdrawal or lets batch 1's through | the unit test for the guard | BROKEN, and the design moved: the state is `Session.advertised`, not `adjRIBOut`. `adjRIBOut` records API announces alone, so a peer holding config-declared or forwarded routes would have had a legitimate withdrawal dropped (R-2). The Session is the unit RFC 4271 Section 4.3 names and needs no reset |
| A-6 | The four ack-ordering cases and `api-add-remove` write one line per unit of work, so a batch of one is what they produce | `test/exabgp-compat/etc/run/api-ipv4.run` sleeps 0.2s between sends; `api-add-remove.run` sleeps between its announce pair and its withdrawal pair | Those five cases change their frame order and go red | running them | confirmed: all five pass in the final run, and the suite's api count rose from 37/40 to 38/40 with no case lost |
| A-7 | Ze's translator produces one canonical spelling for one NLRI, so two lines naming the same route produce the same netting key | `internal/exabgp/bridge/bridge_command.go` writes the NLRI text itself from its own parse | A cancellation is missed and `api-fast` stays red, or a wrong one fires and a route is lost | the netting unit test over both spellings a script can write | confirmed: `TestBridgeBatchKeyMatchesAcrossSpellings` drives `announce route <prefix>` against `withdraw ipv4 unicast <prefix>` in both orders |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Over-batching: lines a script wrote separately land in one read, and netting then removes a frame the fixture expects. The shape that costs a frame is an announce and a LATER withdrawal of the same key in one batch, which `api-flow` writes (lines 2 to 6 of `api-flow.run`) | `api-flow` produces fewer frames than its `.ci` lists, or its frame count moves between runs | The reader MUST NOT do wire work: reading is decoupled from dispatch so the reader returns to the pipe immediately, which is the structure ExaBGP has and the reason its own `api-flow` recording shows no netting. The batching unit test drives two reads and asserts two batches |
| R-2 | The advertised-state guard drops a withdrawal an operator meant: a peer holding config-declared routes has an empty `adjRIBOut` until the API rail sends it something | An operator withdraws a static route through `send bgp` and reads success with no frame on the wire | The guard arms on any NLRI-carrying UPDATE the peer is sent, not on API announces alone, and a withheld withdrawal is REPORTED in the command answer and counted, never silent (`ai/rules/principles.md`) |
| R-3 | The netting and the guard interact through order: batch 1's withdrawal must be judged before the batch's own announce arms the peer | `api-fast` sends the 1.1.0.0/24 withdrawal | Withdrawals dispatch before announces within a batch, which is required anyway by batch 2's frame order. The unit test asserts the two rules together over batch 1's exact line list |
| R-4 | One line's dispatch failure swallows the rest of the batch's acks, and a script blocks for an answer that never comes | A compat case times out waiting for `done` | Each line keeps its own answer: a failed line is answered `error` and the batch continues |
| R-5 | The netting key is derived by re-reading the command text the translator wrote, and drifts when the command grammar moves | A grammar change makes a cancellation stop firing, with no test red | The translator STATES the key on `Translation` where it writes the command (`ai/rules/principles.md`, declare once); no consumer re-parses the text |
| R-6 | A batch spanning two selectors flushes one and not the other | A multi-neighbor case loses ordering (`api-multi-neighbor`, `api-announce-star`) | One flush per distinct selector the batch dispatched to, before the first ack |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes an ExaBGP-migrated script announced are not on the wire, or withdrawals it sent are not: a prefix is blackholed or a stale route stays advertised. The reach is the ExaBGP bridge plus every API withdrawal, since the advertised-state guard sits on the shared rail |
| How is it reverted? | Single commit revert. Nothing is persisted and no config leaf is added |
| Who else touches this path? | `plan/immediate/spec-fixit-send-names-its-destination.md` is rewriting the `send` grammar the bridge emits, and owns `bridge_command.go` and `script.go` right now. `plan/journal/guard-demands-what-the-model-cannot-supply.md` and `plan/journal/bulk-rename-corruption.md` both name the FlowSpec announce path in the same files |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a script writes four API lines in one write | → | the bridge's batch reader cuts one batch | `TestBridgeBatchCutsOneReadIntoOneBatch` |
| a batch holding an announce and a later withdrawal of one NLRI | → | the netting function drops the announce | `TestBridgeBatchNetsWithdrawOverEarlierAnnounce` |
| a netted batch | → | the dispatcher writes withdrawals first, then announces, then one flush per selector, then one ack per line | `TestBridgeBatchDispatchOrderAndAcks` |
| `send bgp <peer> update text ... del ...` to a peer sent no NLRI this session | → | `withdrawBatchFromPeers` withholds and reports | `TestWithdrawWithheldUntilSessionAdvertises` |
| the `api-fast` script over the real bridge and a mock peer | → | the frames the peer records | `test/exabgp-compat/api/api-fast.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The `api-fast` script runs against ze | The mock peer records exactly the frames `test/exabgp-compat/api/api-fast.ci` lists, in that order, and the case passes |
| AC-2 | One batch holds `announce X`, then `withdraw X` | Neither an announce nor a withdrawal of X is dispatched from that batch, when the peer has been sent no NLRI this session |
| AC-3 | One batch holds `announce X`, then `withdraw X`, where the peer has already been sent an NLRI this session | A withdrawal of X is written and no announce of X is |
| AC-4 | One batch holds `withdraw X`, then `announce X` | Both are dispatched, the withdrawal first |
| AC-5 | One batch holds `announce X` twice with identical attributes | One UPDATE reaches the peer, and the peer's suppressed count rises by one |
| AC-6 | One batch holds `announce X` and `withdraw Y` | The withdrawal of Y is dispatched before the announce of X, and both reach the wire |
| AC-7 | The first API withdrawal of a session names a route the peer was never sent, and the peer has been sent no NLRI on this session | No UPDATE is written to that peer, the command answer names the withheld withdrawal and the peer it was withheld from, and the script still reads `done` |
| AC-8 | The same withdrawal after the peer has been sent any NLRI-carrying UPDATE on this session | The UPDATE is written, whether or not the peer holds that route |
| AC-9 | A script writes N lines in one write | Exactly N acks are written, each after the batch's flush, and one flush is dispatched per distinct selector the batch reached |
| AC-10 | A script writes two lines in two writes separated by the dispatch of the first | Two batches are cut, and nothing from the second cancels anything in the first |
| AC-11 | One line of a batch fails to dispatch | That line is answered `error`, every other line of the batch keeps its own answer, and the batch is not abandoned |
| AC-12 | The session goes down and comes up | The peer's advertised state is unset again, so the first withdrawal after re-establishment is withheld as in AC-7 |
| AC-13 | `api-ipv4`, `api-ipv6`, `api-mvpn`, `api-vpnv4`, `api-add-remove`, `api-announcement`, `api-attributes`, `api-multi-neighbor` run | Each records the frames its `.ci` lists, unchanged from before this work |
| AC-14 | `api-flow` runs | Its frame count is not lower than before this work: no batch of its lines removes a frame its `.ci` lists |

### Goal Validation (planned evidence)

<!-- The closure record is appended by /ze-close from plan/TEMPLATE-CLOSURE.md.
     This table states, before the work starts, what will count as proof.
     `ai/rules/interop-and-goal-validation.md` requires one row per goal. -->
| Goal | Evidence that will prove it |
|------|----------------------------|
| A script written for ExaBGP puts ExaBGP's frames on the wire through ze | `test/exabgp-compat/api/api-fast.ci` passes, and the discrimination walk records it RED with the netting reverted and the subject rebuilt |
| Netting removes no frame ExaBGP sends | `api-flow`, `api-add-remove`, `api-ipv4`, `api-ipv6`, `api-mvpn`, `api-vpnv4`, `api-announcement`, `api-attributes`, `api-multi-neighbor` keep their frame lists, run in one suite pass and named in the closure record |
| A withheld withdrawal is never a silent no-op | `TestWithdrawWithheldUntilSessionAdvertises` asserts the reason on the answer, and the functional case reads it back through the command output |
| The ack contract is unchanged | `api-ack-control`, `api-silence-ack` and the four ordering cases pass, and `TestBridgeBatchDispatchOrderAndAcks` asserts flush-before-ack for a batch of four |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | migrates an ExaBGP script that writes a burst of announces and withdrawals in one write | pipe → batch reader → netting → dispatch → `AnnounceNLRIBatch` / `WithdrawNLRIBatch` → wire | `test/exabgp-compat/api/api-fast.ci` |
| 2 | withdraws a route from a peer that has been advertised nothing on this session | `send bgp` → `WithdrawNLRIBatch` → the advertised-state guard → the command answer | `TestWithdrawWithheldUntilSessionAdvertises` |
| 3 | announces and withdraws one route from a script that waits for each ack | one line per batch, unchanged path | `test/exabgp-compat/api/api-ipv4.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBridgeBatchCutsOneReadIntoOneBatch` | `internal/exabgp/bridge/bridge_batch_test.go` | four lines in one read become one batch; a trailing partial line is carried, not dispatched | PASS |
| `TestBridgeBatchSecondReadIsSecondBatch` | `internal/exabgp/bridge/bridge_batch_test.go` | the reader does not wait for more input to grow a batch (AC-10) | PASS |
| `TestBridgeBatchNetsWithdrawOverEarlierAnnounce` | `internal/exabgp/bridge/bridge_batch_test.go` | the announce is dropped, the withdrawal kept (AC-2, AC-3) | PASS |
| `TestBridgeBatchAnnounceDoesNotCancelWithdraw` | `internal/exabgp/bridge/bridge_batch_test.go` | a later announce leaves an earlier withdrawal in place, and both dispatch (AC-4) | PASS |
| `TestBridgeBatchWithdrawalsDispatchFirst` | `internal/exabgp/bridge/bridge_batch_test.go` | order within a batch (AC-6) | PASS |
| `TestBridgeBatchKeyMatchesAcrossSpellings` | `internal/exabgp/bridge/bridge_batch_test.go` | the key the translator states matches for the two spellings a script can write for one route (A-7) | PASS |
| `TestBridgeBatchDispatchOrderAndAcks` | `internal/exabgp/bridge/bridge_batch_test.go` | one flush per selector, every ack after it, one ack per line, a failed line answered on its own (AC-9, AC-11) | PASS |
| `TestApiFastBatchOneProducesOneAnnounce` | `internal/exabgp/bridge/bridge_batch_test.go` | the four lines of `api-fast` batch 1 produce one announce command and nothing else, with the peer unadvertised (AC-2 with the guard) | PASS |
| `TestBridgeBatchEndOfRIBNeitherCancelsNorIsCancelled` | `internal/exabgp/bridge/bridge_batch_test.go` | ADDED: the zero `RouteKey` is not a route, so two markers never cancel each other (`ai/rules/principles.md`) | PASS |
| `TestBridgeBatchAckControlAppliesInWriteOrder` | `internal/exabgp/bridge/bridge_batch_test.go` | ADDED: `disable-ack` inside a batch silences the lines that FOLLOW it, which is why the answers run in write order | PASS |
| `TestBridgeBatchRefusesAnOverlongLine` | `internal/exabgp/bridge/bridge_batch_test.go` | ADDED: the boundary row, a line past `batchLineMax` is refused whole and never truncated into a shorter command | PASS |
| `TestWithdrawWithheldUntilSessionAdvertises` | `internal/component/bgp/reactor/adj_rib_out_test.go` | no UPDATE, a reason on the answer, a counter (AC-7) | PASS |
| `TestWithdrawSentAfterAnyNLRIAdvertised` | `internal/component/bgp/reactor/adj_rib_out_test.go` | armed by any NLRI-carrying UPDATE, including one the RIB forwarded (AC-8, R-2) | PASS |
| `TestEndOfRIBDoesNotArmTheWithdrawGuard` | `internal/component/bgp/reactor/adj_rib_out_test.go` | ADDED: every compat case sends a marker before its script runs, so a marker that armed the guard would leave `api-fast` batch 1 unchanged | PASS |
| `TestAdvertisedStateClearedOnTeardown` | `internal/component/bgp/reactor/adj_rib_out_test.go` | a new session withholds again (AC-12) | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| lines in one batch | 1..N, bounded by the read buffer | the last complete line in the buffer | 0 (an empty read cuts no batch) | N/A: a line past the buffer is carried to the next batch |
| read buffer size | 4096 bytes, ze's `bufio` default | a 4096-byte read | N/A | a line longer than the buffer is refused by name, never truncated into a different command |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `api-fast` | `test/exabgp-compat/api/api-fast.ci` | a migrated script writes a burst of commands and reads ExaBGP's frames back | PASS |
| `api-add-remove` | `test/exabgp-compat/api/api-add-remove.ci` | announces and withdrawals in separate writes are all sent (netting removes nothing) | PASS |
| `api-ipv4`, `api-ipv6`, `api-mvpn`, `api-vpnv4` | `test/exabgp-compat/api/` | one route per write keeps its flush-before-ack order | PASS |
| `api-ack-control`, `api-silence-ack` | `test/exabgp-compat/api/` | ack control still answers per line inside a batch | PASS |

Measured over three runs of `./le functional exabgp-test`, the last at load
average 7.1: `encoding` 42/42, `api` 38/40, failing 15 (`api-flow`) and 33
(`api-reload`) only. Before this work the same command read 42/42 and 37/40,
failing 14 (`api-fast`), 15 and 33.

The discrimination walk (`ai/rules/interop-and-goal-validation.md`): with `Net`
returning its input unnetted and `Peer.hasAdvertised` returning true, and the
subject rebuilt, `api-fast` is RED on `unexpected message = ...18010100`, the
announce of 1.1.0.0/24 that ExaBGP never sent, with 4 expected frames remaining.
Restored, it is GREEN. In the same probe run `api-flow`'s first mismatch and its
remaining-frame count are byte-identical to the run with the netting and the
guard in place, which is AC-14 measured rather than argued.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `api-fast` | `test/exabgp-compat/api/` | ExaBGP (recorded fixture, byte-identical to upstream `qa/api/api-fast.ci`) | a burst of API commands puts the same frames on the wire as ExaBGP puts there | |

<!-- The compat suite IS the interop evidence for the bridge: the fixture is
     ExaBGP's own recording of its own output, and ze is measured against it.
     The discrimination walk required by ai/rules/interop-and-goal-validation.md
     reverts the netting, rebuilds the subject, and records api-fast RED. -->

## Files to Modify
- `internal/plugins/exabgp/bridgerun/script.go` - `read` cuts batches instead of single lines; `line` becomes the per-batch dispatcher, keeping the ack, flush and refusal rules it already carries
- `internal/exabgp/bridge/bridge.go` - `pluginToZebgp` takes the same batches, over MuxConn
- `internal/exabgp/bridge/bridge_command.go` - `Translation` states the route key the translator already holds for each route command it writes
- `internal/component/bgp/reactor/reactor_api_batch.go` - `withdrawBatchFromPeers` consults the peer's advertised state and reports a withheld withdrawal; `announceBatchToPeers` sets that state
- `internal/component/bgp/reactor/adj_rib_out.go` - the per-session advertised state, cleared by `reset`
- `docs/architecture/exabgp-bridge.md` - the batch is the unit: one read, one netting, one flush per selector, one ack per line
- `docs/architecture/update-building.md` - the API rail withholds a withdrawal until the session has advertised an NLRI, and says so

## Files to Create
- `internal/exabgp/bridge/bridge_batch.go` - the batch reader and the netting, in the package BOTH runners already share, so the rule is written once
- `internal/exabgp/bridge/bridge_batch_test.go` - the unit tests above
- `internal/component/bgp/reactor/adj_rib_out_test.go` - if absent, the advertised-state tests; otherwise they join the existing file

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No new command and no new leaf: the netting has no operator control, and one is not offered (`ai/rules/simplicity.md`) |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | The bridge dispatches the commands that already exist. Thomas settled the `send` grammar in `plan/immediate/spec-fixit-send-names-its-destination.md` and this spec does not touch it |
| CLI grammar (keyword before value) | N-A | No new command surface |
| Editor autocomplete | N-A | No leaf added |
| Functional test for new RPC/API | Yes | `test/exabgp-compat/api/api-fast.ci` and the sibling cases named in the Functional Tests table |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No `environment/` leaf added |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or certificate: the pipe, the flush command and the reactor rail all exist |
| Prometheus counters/metrics | Yes | The withheld-withdrawal count joins the suppressed count `adjRIBOut` already keeps (`internal/component/bgp/reactor/adj_rib_out.go`); it is read by the tests and by the operator-facing debug line, and it is not a new Prometheus surface |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute is added; the netting key is the wire NLRI every family already writes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The bridge's compatibility is already listed; no feature row changes |
| 2 | Config syntax changed? | No | No leaf added |
| 3 | CLI command added/changed? | No | No command added; the withheld-withdrawal reason is a new field in an existing answer, covered by row 4 |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - the withdrawal answer can name a peer it withheld from |
| 5 | Plugin added/changed? | No | The exabgp plugin's surface is unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/route-injection.md` - a script's burst of commands nets before the wire, as it does under ExaBGP |
| 7 | Wire format changed? | No | No encoding changes; which UPDATEs are built changes |
| 8 | Plugin SDK/protocol changed? | No | `AnnounceNLRIBatch` and `WithdrawNLRIBatch` keep their signatures |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No requirement moves. RFC4271-9.2-1 is already met by `adjRIBOut` and this spec adds no proof of a new MUST, so `docs/features/rfc-status.md` is untouched |
| 10 | Test infrastructure changed? | No | No new suite, runner or option |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` claims nothing about API batching |
| 12 | Internal architecture changed? | Yes | `docs/architecture/exabgp-bridge.md` and `docs/architecture/update-building.md`, both named in Files to Modify |
| 13 | Route metadata keys added/changed? | No | No metadata key |
| 14 | Prometheus counters added/changed? | No | The withheld count is an internal counter beside the existing suppressed count, not a Prometheus metric |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/immediate/spec-fixit-api-batch-reaches-the-wire-unnetted.md` at the start of implementation. `adj_rib_out.go` DECLARES `docs/architecture/update-building.md` and `script.go` declares `docs/architecture/exabgp-bridge.md`; both are in Files to Modify. `bridge.go`, `bridge_command.go` and `reactor_api_batch.go` each declare `docs/architecture/core-design.md`, which is NOT edited: it describes the plugin IPC framing, the UPDATE wire layout and the forwarding fast path, and says nothing about the bridge's dispatch, its flush, its ack or the API rail's withdrawal rule. Re-check it at implementation, since a page that gained such a paragraph would then be wrong. Six further pages only `<!-- source: -->` mention `reactor_api_batch.go` and are advisory: `wire/rfc7606-relay-shape.md`, `features/bgp-protocol.md`, `features/configuration.md`, `features/srv6.md`, `guide/configuration.md`, `guide/monitoring.md`. None states when a withdrawal is written, so none is made wrong by the guard |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/exabgp-bridge.md` describes the per-line flush; its "A route is acked after its flush" section is rewritten for the batch |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the batch exists and is reachable, netting nothing
   - Tests: `TestBridgeBatchCutsOneReadIntoOneBatch`, `TestBridgeBatchSecondReadIsSecondBatch`, `TestBridgeBatchDispatchOrderAndAcks`
   - Files: `internal/exabgp/bridge/bridge_batch.go`, `internal/plugins/exabgp/bridgerun/script.go`, `internal/exabgp/bridge/bridge.go`
   - Verify: the compat suite is where it was before this phase (the batch changes nothing yet), and the reader does no wire work: dispatch runs off the read path
2. **Phase: the netting key** -- the translator states what it wrote
   - Tests: `TestBridgeBatchKeyMatchesAcrossSpellings`
   - Files: `internal/exabgp/bridge/bridge_command.go`
   - Verify: every route command the translator emits carries a key, and a command that is not a route carries none
3. **Phase: netting** -- a withdrawal cancels an earlier announce, an announce cancels nothing, withdrawals dispatch first
   - Tests: `TestBridgeBatchNetsWithdrawOverEarlierAnnounce`, `TestBridgeBatchAnnounceDoesNotCancelWithdraw`, `TestBridgeBatchWithdrawalsDispatchFirst`, `TestApiFastBatchOneProducesOneAnnounce`
   - Files: `internal/exabgp/bridge/bridge_batch.go`
   - Verify: `api-fast` batch 2's two frames are right; batch 1 still carries its withdrawal, so the case is still red on that one frame
4. **Phase: the advertised-state guard** -- a session that has advertised nothing writes no withdrawal, and says so
   - Tests: `TestWithdrawWithheldUntilSessionAdvertises`, `TestWithdrawSentAfterAnyNLRIAdvertised`, `TestAdvertisedStateClearedOnTeardown`
   - Files: `internal/component/bgp/reactor/adj_rib_out.go`, `internal/component/bgp/reactor/reactor_api_batch.go`
   - Verify: `api-fast` passes; the withheld withdrawal is visible in the command answer and in the counter
5. **Phase: the suite and the pages** -- no case loses a frame, and the two pages match the code
   - Tests: the Functional Tests table, run as one pass
   - Files: `docs/architecture/exabgp-bridge.md`, `docs/architecture/update-building.md`, `docs/architecture/api/commands.md`, `docs/guide/route-injection.md`
   - Verify: the discrimination walk records `api-fast` RED with the netting reverted and the subject rebuilt, then GREEN with it restored

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and AC-14 has a frame count from a run rather than an argument |
| Feature completeness | Both bridge runners take the same batching from the same function; neither carries its own copy |
| Correctness | The cancellation is directional: a withdrawal cancels an EARLIER announce of the same key and an announce cancels nothing. A batch holding withdraw-then-announce sends both |
| Correctness | A withheld withdrawal reaches the operator's answer. A zero UPDATE count with a `done` and no reason is the failure this rail exists to avoid (`ai/rules/principles.md`) |
| Naming | The advertised state is named for what it models (this session has advertised an NLRI), not for the fixture that needs it |
| Data flow | The reader does no wire work. A dispatch that blocks the reader re-creates R-1 |
| Rule: `ai/rules/principles.md` | The route key is stated once, by the translator that writes the command, and no consumer re-parses command text |
| Rule: `ai/rules/interop-and-goal-validation.md` | `api-fast` is recorded RED under the reverted netting, with the subject rebuilt, before it is claimed as evidence |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One batching and netting implementation used by both runners | `grep -rn "bridge_batch\|Batch(" internal/exabgp/bridge internal/plugins/exabgp/bridgerun` names one producer and two callers |
| `api-fast` green | `./le functional exabgp-test`, and the case's line in the report |
| No compat case lost a frame | the suite's pass count before and after, both in the closure record |
| The withheld withdrawal is answered, not swallowed | `TestWithdrawWithheldUntilSessionAdvertises` asserts the reason string on the answer |
| The two architecture pages match the code | `./le spec citation anchors spec plan/immediate/spec-fixit-api-batch-reaches-the-wire-unnetted.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A line longer than the read buffer is refused by name and never truncated into a shorter command that would parse as something else |
| Resource exhaustion | A batch is bounded by one read, so a script cannot make the bridge hold an unbounded pending set |
| Error leakage | The withheld-withdrawal reason names the peer and the family; it carries no configuration the script did not already send |
| Authorization that could fail open | The `send [ update ]` permission is still tested per command before anything is dispatched; the netting removes commands and never adds one |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| `api-flow` loses a frame | R-1 fired: the reader is doing wire work, or the batch is being grown deliberately. Fix the reader, never the fixture |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- ExaBGP's netting is a property of its RIB and its reactor cycle, not of its pipe. The pipe only decides which commands share a cycle. That is why the fix has two halves in two places: what shares a batch (the bridge) and what a session may withdraw (the reactor).
- The asymmetry between `api-fast` batch 1 and batch 2 is not a state rule and cannot be written as one. Both batches withdraw a route the peer never received; only the second one reaches the wire. Every state-based reading (the peer holds it, the announce was cancelled, the Adj-RIB-Out is empty for that key) gives the two batches the same answer and gets one of them wrong. The distinguishing fact is temporal: batch 1 precedes the session's first update pass.
- The three behaviors are separable and only one was missing from the RIB layer. Duplicate suppression already exists as `adjRIBOut`, and sending a withdrawal for a route the peer never held is already what the withdraw rail does. Naming which of the three each fixture line needs is what keeps this from being a rewrite of the rail.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Net in the bridge, over the lines of one read, on a key the translator states | Stage every API announce and withdrawal in the reactor and let `request peer <sel> flush` drain and diff them, which is ExaBGP's own shape | The reactor rail sends on the call, and every other producer relies on that: the CLI's `send bgp`, the healthcheck plugin, `commit` and the tests. Staging would make a caller that does not flush send NOTHING, silently, which is the failure `ai/rules/principles.md` names first. The bridge is the only producer whose commands arrive in bursts, so the pending set belongs to it |
| The batch is one read, cut by a reader that does no wire work | Grow a batch with a timer or a quiescence window; net across an arbitrary window | A timer invents a boundary ExaBGP does not have and would net `api-flow`'s separate writes, costing frames its fixture lists. One read is the boundary ExaBGP itself uses (`Processes.received`), and a reader that returns to the pipe immediately keeps ze's batches no coarser than ExaBGP's |
| Withdrawals dispatch before announces within a batch | Keep write order and let the netting alone reorder nothing | `api-fast` batch 2 puts the withdrawal of 2.2.0.0/25 on the wire BEFORE the announce of 2.2.0.0/24, though the script wrote them the other way round. Upstream states the rule where it drains ("Generate Updates for pending withdraws before announces (preserves semantic ordering)", `src/exabgp/rib/outgoing.py`) |
| A withdrawal is withheld until the session has advertised an NLRI, and the answer says so | Drop it silently, as ExaBGP does at its packing layer; or suppress a withdrawal whose key the Adj-RIB-Out does not hold | Silence is banned (`ai/rules/principles.md`), and a key-based rule gets batch 2 wrong: 2.2.0.0/25 is not in the table there either, and its withdrawal must be sent |
| The state is armed by any NLRI-carrying UPDATE, EOR excluded | Arm on the first API announce only; arm at the first flush | A peer holding config-declared routes has been advertised something, and its operator may withdraw it. ExaBGP arms on the same event: its config routes and its API routes drain through one generator, and the EOR is sent on a branch that leaves the flag alone |
| The translator states the route key on `Translation` | Re-read the key out of the command text the translator just wrote | Re-parsing is a second declaration of the same fact, and it drifts the moment the grammar moves, with no test red. `plan/immediate/spec-fixit-send-names-its-destination.md` is moving that grammar right now |
| The advertised state lives on `Session`, not on `adjRIBOut` (CHANGED at implementation; A-5 broke) | Keep it on `adjRIBOut`, as Integration Points proposed | `adjRIBOut` records API announces alone, so a peer holding config-declared or forwarded routes reads as unadvertised and a legitimate withdrawal is dropped: R-2, and the Key Design Decision above already required arming on any NLRI-carrying UPDATE. The Session is the unit RFC 4271 Section 4.3 names, so it is armed at the three points a message reaches the socket, a new connection starts unset, and there is nothing to clear at teardown. `adjRIBOut` keeps the withheld COUNTER, beside the suppressed count |
| The netting key is a command's whole NLRI set | Key each prefix separately, as ExaBGP's `Change.index` does | One ze command can name several prefixes (`announce attributes ... nlri A B`), and the framing of that command is the peer's `group-updates` leaf rather than the bridge's. A set key is coarser than ExaBGP's, and it errs the safe way: it can leave a frame on the wire, never remove one |

## Known Limitations

- Netting is the bridge's, so a native producer that sends a burst through `send bgp` gets no netting. That is deliberate: the ExaBGP script is the caller whose commands arrive in one write, and giving every producer a pending set changes when a route reaches the wire for every caller in the tree. If a native burst ever needs it, it is a spec of its own and its first question is the one this spec declined: whether `send bgp` may stop sending on the call.
- `api-flow` is out of scope and stays red. Its failure is not netting. Two causes are already recorded, and one is measured here:
  - The frame this session observed for its FlowSpec withdrawal is a byte permutation of the frame the fixture expects: the same attributes and the same MP_UNREACH_NLRI payload, with the well-known attributes written before MP_UNREACH_NLRI rather than after. Upstream writes `mp_unreach + attr + mp_reach` (`src/exabgp/bgp/message/update/collection.py`), and the fixture pins that order. Netting can neither create nor repair a permutation of one frame's attributes.
  - `plan/journal/guard-demands-what-the-model-cannot-supply.md` records that every FlowSpec announce is skipped because the announce rail treats an unset next hop as a reason to skip a peer, so no MP_REACH_NLRI reaches the wire at all for this case.
  - `plan/journal/bulk-rename-corruption.md` records that the FlowSpec component keywords were renamed under about forty committed test call sites, and names `api-flow` among the cases that fail with them.
- `api-reload` is out of scope. `plan/journal/announced-state-never-replayed.md` holds it.
- The residual over-batching risk (R-1) is not removed, because no reader can see a writer's boundaries: a pipe is a byte stream and ExaBGP runs the same risk. What this spec buys is that ze's batches are no coarser than ExaBGP's, which is the standard the fixtures were recorded against.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

The advertised-state guard cites RFC 4271 Section 4.3: a withdrawn route "is
identified by its destination (expressed as an IP prefix), which unambiguously
identifies the route in the context of the BGP speaker - BGP speaker connection
to which it has been previously advertised."

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Wiring test rows all carry concrete test names
- [ ] Integration Checklist and Documentation Update Checklist answered on every row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features: `api-fast`, with the discrimination walk recorded

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `internal/exabgp/bridge/bridge_batch.go`: `BatchReader` cuts one read into one batch and carries a partial line; `Net` cancels an announce a later withdrawal of the same route takes back and orders withdrawals before announces; `AnswerBatch` writes one answer per line in write order; `BatchSelectors` names each selector once.
- `internal/exabgp/bridge/bridge_command.go`: `Translation.Commands` became `[]Command`, and every route command carries the `RouteKey` the translator holds where it writes the NLRI. A command that carries no route carries the zero key, which `RouteKey.Route` answers false for.
- `internal/plugins/exabgp/bridgerun/script.go` and `internal/exabgp/bridge/bridge.go`: both runners read batches, hand each one to a dispatcher goroutine, and return to the pipe at once. One flush per selector, then one answer per line.
- `internal/component/bgp/reactor/session.go`, `session_write.go`, `peer.go`: `Session.advertised`, set at the three points a message reaches the socket, read through `Peer.hasAdvertised`. `updateIsReachable` and `bodyIsReachable` leave it unset for an End-of-RIB and for a pure withdrawal.
- `internal/component/bgp/reactor/reactor_api_batch.go`: `splitOnAdvertised` partitions the fan-out, `logWithdrawWithheld` names the peer, and `WithdrawNLRIBatch` answers `route.ErrWithdrawWithheld` naming every peer it wrote nothing to.
- `internal/component/bgp/plugins/cmd/update/update_text.go` and `internal/component/bgp/plugins/cmd/announce/registry.go`: both read that error as a warning, so the command answers `done` and the reason reaches the operator.

The work landed at `fa86db31ed`, which carries two other strands in the same
files. Closure adds one test, one page repair, this record and the journal row.

### Bugs Found/Fixed
- The netting rule table in `docs/architecture/exabgp-bridge.md` stated a WIRE outcome that the reactor's connection-state guard decides, so it was true for `api-fast` batch 1 and false for every batch after a session's first announce. Fixed in closure, and recorded in `plan/journal/rule-stated-as-an-outcome-a-later-layer-decides.md`.
- AC-7's second half, that the script still reads `done`, had product code on two rails and no test on either. `TestHandleUpdateTextWithheldWithdrawalIsDoneAndNamed` now drives it, and goes RED when the handler's `errors.Is(err, route.ErrWithdrawWithheld)` branch is disabled.

### Documentation Updates
- `docs/architecture/exabgp-bridge.md`: the batch section, the flush section, the ack section, and the closure repair above. Anchors `bridge_batch.go -- BatchReader, Net`, `bridge_command.go -- Command, RouteKey`, `bridge_batch.go -- AnswerBatch, BatchSelectors`, `script.go -- script.batch`.
- `docs/architecture/update-building.md`: "A Withdrawal Names a Route This Connection Advertised", anchored on `session_write.go -- Session.advertised, noteAdvertised, updateIsReachable` and `reactor_api_batch.go -- withdrawBatchFromPeers, logWithdrawWithheld`.
- `docs/architecture/api/commands.md`: the withheld-withdrawal answer, anchored on `update_text.go -- handleUpdateText` and `route.go -- ErrWithdrawWithheld`.
- `docs/guide/route-injection.md`: the operator-facing form of both rules.
- `./le doc check verify` FAILS, and on no surface this spec touches: the firewall-domain plugin table, `create bgp peer`, the `send bgp` verbs, `show bgp rib` filters, `show bgp reject-asn`, `show resolve rir` and seven OSPF help rows, all from other sessions' in-flight work.

### Deviations from Plan
- The advertised state lives on `Session`, not on `adjRIBOut`. A-5 broke; the Key Design Decisions table records it.
- `Translation.Commands` changed type, which the spec did not name. The key had to travel with each command rather than with the line, because one line can be several commands and only some of them carry a route.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5 put the advertised state on `adjRIBOut` | That table records API announces alone, so a peer holding config-declared or forwarded routes reads as unadvertised | at implementation, against R-2 | the state moved to `Session`, armed by any NLRI-carrying UPDATE |
| approach | The page stated the netting rule as a wire outcome | The netting decides what dispatches; a second layer decides what reaches the wire | closure documentation review, reading the table against `Net` and `withdrawBatchFromPeers` | table rewritten at the dispatch level, journal row written |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A withdrawal cancels an announce of the same NLRI still queued from the same read | Done | `bridge_batch.go` `Net` | behavior 1 of the Task table |
| A repeated announce of an unchanged route sends nothing | Done | `adj_rib_out.go` `unchanged` | behavior 2, already present |
| The first update pass of a session carries no withdrawal | Done | `reactor_api_batch.go` `splitOnAdvertised`, `session_write.go` `noteAdvertised` | behavior 3 |
| A batch's withdrawals are encoded before its announces | Done | `bridge_batch.go` `Net`, the three-pass order | behavior 4 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `ze-test exabgp api 14` PASS, 92.1s, 2026-09-06 | and RED under the reverted netting and guard |
| AC-2 | Done | `TestBridgeBatchNetsWithdrawOverEarlierAnnounce` plus `TestWithdrawWithheldUntilSessionAdvertises` | the netting drops the announce, the guard withholds the withdrawal |
| AC-3 | Done | `TestWithdrawSentAfterAnyNLRIAdvertised`, and `api-fast` batch 2 | |
| AC-4 | Done | `TestBridgeBatchAnnounceDoesNotCancelWithdraw` | |
| AC-5 | Done | `TestAnnounceTwiceSendsOneUpdate` | `adjRIBOut.suppressed` |
| AC-6 | Done | `TestBridgeBatchWithdrawalsDispatchFirst` | |
| AC-7 | Done | `TestWithdrawWithheldUntilSessionAdvertises` and `TestHandleUpdateTextWithheldWithdrawalIsDoneAndNamed` | the second was added at closure; the `done` half had no test |
| AC-8 | Done | `TestWithdrawSentAfterAnyNLRIAdvertised` | |
| AC-9 | Done | `TestBridgeBatchDispatchOrderAndAcks` | a comment line owes no answer, which is the rule the runners already kept |
| AC-10 | Done | `TestBridgeBatchSecondReadIsSecondBatch` | |
| AC-11 | Done | `TestBridgeBatchDispatchOrderAndAcks` | the runners set the per-line answer and keep going |
| AC-12 | Done | `TestAdvertisedStateClearedOnTeardown` | the state is a Session field, so a new connection starts unset |
| AC-13 | Done | `ze-test exabgp api 1 2 6 8 20 21 23 26 37 40`, all PASS | `api-ack-control`, `api-add-remove`, `api-announcement`, `api-attributes`, `api-ipv4`, `api-ipv6`, `api-multi-neighbor`, `api-mvpn`, `api-silence-ack`, `api-vpnv4` |
| AC-14 | Done | `ze-test exabgp api --save`, `api-flow` writes 15 frames | against the 10 recorded on 2026-09-06 in `plan/journal/guard-demands-what-the-model-cannot-supply.md`. The case is still red, on the attribute-order permutation Known Limitations names |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The eleven `TestBridgeBatch*` and `TestApiFastBatchOneProducesOneAnnounce` | Done | `internal/exabgp/bridge/bridge_batch_test.go` | package PASS |
| `TestWithdrawWithheldUntilSessionAdvertises`, `TestWithdrawSentAfterAnyNLRIAdvertised`, `TestEndOfRIBDoesNotArmTheWithdrawGuard`, `TestAdvertisedStateClearedOnTeardown` | Done | `internal/component/bgp/reactor/adj_rib_out_test.go` | all four PASS |
| `TestHandleUpdateTextWithheldWithdrawalIsDoneAndNamed` | Done | `internal/component/bgp/plugins/cmd/update/update_text_withheld_test.go` | ADDED at closure for AC-7 |
| `api-fast`, and the ten sibling cases | Done | `test/exabgp-compat/api/` | run by name |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/exabgp/bridge/bridge_batch.go`, `bridge_batch_test.go` | Done | created |
| `internal/exabgp/bridge/bridge_command.go`, `bridge.go` | Done | |
| `internal/plugins/exabgp/bridgerun/script.go` | Done | |
| `internal/component/bgp/reactor/reactor_api_batch.go`, `adj_rib_out.go`, `adj_rib_out_test.go` | Done | |
| `internal/component/bgp/reactor/session.go`, `session_write.go`, `peer.go` | Changed | the advertised state moved here from `adjRIBOut` |
| `internal/component/bgp/route/route.go`, `cmd/update/update_text.go`, `cmd/announce/registry.go` | Changed | the answer the guard owes an operator |
| The four pages | Done | one repaired at closure |

### Audit Summary
- **Total items:** 14 acceptance criteria, 4 task requirements, 16 tests, 13 files
- **Done:** all 14 acceptance criteria, all 4 requirements, all 16 tests
- **Partial:** none
- **Skipped:** none
- **Changed:** 3 file rows, all in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A script written for ExaBGP puts ExaBGP's frames on the wire through ze | interop | `ze-test exabgp api 14` PASS on 2026-09-06. Discrimination walk in the same session: with `RouteKey.Route` answering false and `splitOnAdvertised` answering every peer writable, the case is RED on `unexpected message = ...4005040000006418010100; 4 expected frames remain`, the announce of 1.1.0.0/24 ExaBGP never sent. Both files restored and verified clean |
| Netting removes no frame ExaBGP sends | functional | ten named sibling cases PASS (`api-ack-control`, `api-add-remove`, `api-announcement`, `api-attributes`, `api-ipv4`, `api-ipv6`, `api-multi-neighbor`, `api-mvpn`, `api-silence-ack`, `api-vpnv4`). `api-flow` writes 15 frames against the 10 recorded before this work, and fails on the attribute-order permutation of one withdrawal, which no netting can create or repair |
| A withheld withdrawal is never a silent no-op | functional | `TestWithdrawWithheldUntilSessionAdvertises` asserts `route.ErrWithdrawWithheld` and the peer address on the answer, and the counter. `TestHandleUpdateTextWithheldWithdrawalIsDoneAndNamed` asserts `done` plus the warning naming the peer, and goes RED with the handler's branch disabled. The `api-fast` run logs `withdrawal withheld ... peer=127.0.0.1 rfc="RFC 4271 Section 4.3"` |
| The ack contract is unchanged | functional | `api-ack-control` and `api-silence-ack` PASS, plus `TestBridgeBatchDispatchOrderAndAcks` and `TestBridgeBatchAckControlAppliesInWriteOrder` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| `api-flow` green | Out of scope by the Task section. It fails on an attribute-order permutation in a FlowSpec withdrawal, which netting can neither create nor repair | `plan/journal/guard-demands-what-the-model-cannot-supply.md` holds the announce half; the permutation is recorded in this spec's Known Limitations and in `plan/journal/bulk-rename-corruption.md` |
| `api-reload` green | Out of scope by the Task section | `plan/journal/announced-state-never-replayed.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-api-batch-reaches-the-wire-unnetted-zeclose-apibatch-1693391.md`, 19 files |
| `./le spec session review check` | `review_gate: OK (clean, hashes match)` |
| Rounds | 2 |
| Reviewer lenses used | acceptance criteria against the producing function; wire and RFC citation; concurrency and goroutine lifecycle; bounds and input validation; documentation claims read against current source rather than against the diff; Go style |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The netting rule table stated a wire outcome the reactor's guard decides, so it was true only for a session that has advertised nothing | `docs/architecture/exabgp-bridge.md`, "One write is one batch" | the rows now state what the batch DISPATCHES, and a paragraph above them names the guard and routes to `docs/architecture/update-building.md` |
| 2 | ISSUE | AC-7's `done` half had product code on two rails and no test | `internal/component/bgp/plugins/cmd/update/update_text.go` `handleUpdateText` | `TestHandleUpdateTextWithheldWithdrawalIsDoneAndNamed`, shown RED with the `errors.Is` branch disabled |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/exabgp/bridge/bridge_batch.go` | Yes | 327 lines, `wc -l` |
| `internal/exabgp/bridge/bridge_batch_test.go` | Yes | 338 lines, `wc -l` |
| `internal/component/bgp/reactor/adj_rib_out.go` | Yes | 511 lines, `wc -l` |
| `internal/component/bgp/reactor/adj_rib_out_test.go` | Yes | 394 lines, `wc -l` |
| `internal/component/bgp/plugins/cmd/update/update_text_withheld_test.go` | Yes | created at closure |
| `test/exabgp-compat/api/api-fast.ci` | Yes | listed by `ze-test exabgp api --list` as case 14 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `api-fast` records ExaBGP's frames | `ze-test exabgp api 14`: `92.1s 1/1 PASS 14 api-fast` |
| AC-7 | the answer names the withheld peer and the script reads `done` | `go test -run TestHandleUpdateTextWithheldWithdrawalIsDoneAndNamed ./internal/component/bgp/plugins/cmd/update/` exit 0, and exit 1 with the handler branch disabled |
| AC-12 | a new connection withholds again | `go test -run TestAdvertisedStateClearedOnTeardown ./internal/component/bgp/reactor/` PASS |
| AC-13 | the ten sibling cases keep their frames | `ze-test exabgp api 1 2 6 8 20 21 23 26 37 40`: all PASS |
| AC-14 | `api-flow` loses no frame | 15 frames captured with `--save`, against 10 recorded before this work |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a script writes four API lines in one write | `test/exabgp-compat/api/api-fast.ci` | Yes: its first flush writes four lines and the fixture lists one frame for them |
| `send bgp <peer> update text ... del ...` to a peer sent no NLRI | `test/exabgp-compat/api/api-fast.ci` | Yes: the run log carries `withdrawal withheld ... peer=127.0.0.1 family=ipv4/unicast` |
| a batch reaching two neighbors flushes both | `test/exabgp-compat/api/api-multi-neighbor.ci` | Yes: case 23 PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestBridgeBatchNetsWithdrawOverEarlierAnnounce` and `TestBridgeBatchAnnounceDoesNotCancelWithdraw` |
| A-2 | confirmed | `api-fast` PASS, and the two guard polarities |
| A-3 | confirmed | `TestBridgeBatchKeyMatchesAcrossSpellings`; `routeIdentity` drops the verb |
| A-4 | confirmed | `TestBridgeBatchCutsOneReadIntoOneBatch`, `TestBridgeBatchSecondReadIsSecondBatch` |
| A-5 | broken | the state moved to `Session.advertised`; Mistake Log and Key Design Decisions carry it |
| A-6 | confirmed | the five named cases PASS |
| A-7 | confirmed | `TestBridgeBatchKeyMatchesAcrossSpellings` drives both spellings in both orders |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| The netting rules table | read against `bridge_batch.go` `Net` and `reactor_api_batch.go` `withdrawBatchFromPeers` | Repaired: it named a wire outcome a later layer decides |
| "one flush per selector the batch reached and blocks until each forward pool drains" | `bridge.go` `dispatchBatch` waits on `pending.wait` for each flush; `script.go` `batch` dispatches each flush synchronously | Yes |
| The withheld-withdrawal warning string | `update_text.go` writes `withdraw <family>: <err>`, `reactor_api_batch.go` writes `%w: <family>, peers <list>` | Yes, byte for byte with the page's example |
| "Once the session has carried one UPDATE that makes any destination reachable, a configured route and a relayed one included" | `writeUpdateGated` serves the config rails and `writeRawUpdateBody` the forwarding rail; both call `noteAdvertised` | Yes |
| Row 9, no RFC support level moves | `rfc/short/rfc4271.md` `## Meta` untouched; the guard proves no new MUST | Yes |

## Core Insight

Two cases that differ only in TIME cannot be separated by any rule over state.
`api-fast` batch 1 and batch 2 both withdraw a route the peer never held, and
only the second reaches the wire. The Adj-RIB-Out, the netting and "does the
peer hold it" each answer both the same way and get one of them wrong. What
separates them is that batch 1 precedes the session's first advertisement, so
the rule lives on the connection, and the netting that made batch 1 possible
lives one layer above it.
