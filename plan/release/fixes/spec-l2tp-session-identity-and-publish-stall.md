# Spec: l2tp-session-identity-and-publish-stall

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped-daemon defects an operator meets on an LNS with more than one tunnel: a subscriber's route is withdrawn or its session is cleared because a session on another tunnel holds the same session id, and every tunnel stops answering while one RADIUS server is slow. |
| RFC gaps | - |
| Depends | `spec-single-owner-lifecycle-5-session-end-locals` (closed) (it edits the same L2TP files and adds the per-tunnel order ticket this spec's design must fit; the order of the two is settled in design) |
| Phase | 4/5: readable interfaces, captured RADIUS actions and task-owned accounting Stop implemented; AC-4 daemon proof, AC-8 checked withdrawal, remaining CQM discrimination, native and scale proof open |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Implementation authority, Thomas, 2026-11-16:

> yes please do deliver a working and tested lab supporting the feature

The approved operator interface uses readable numeric session/tunnel selectors,
with the configured listener name where needed to disambiguate. It resolves
once to the complete internal subscriber identity and exact captured control.
The final implementation contract below supersedes the earlier proposal that
would have required operators to type opaque IDs. The internal immutable birth
and the complete strict lab proof remain required.

The observer integration also owns admission of shared metric series. Native
Prometheus retention must refuse invalid UTF-8 and incorrect label counts
before granting a lease, return the cause without callbacks or logging, and
publish a series only after an actual measurement. This small prerequisite
keeps an untrusted subscriber label from reaching a native vector panic.

The same identity isolation covers a retained login's quality phase. Its
existing bounded login slot remembers the birth supplying that phase, so an
older birth's Down cannot overwrite the newer contributor or renew an ended
login's LRU interest. Genuine named Down before IP assignment remains useful;
the login phase does not count simultaneous sessions or retain deduplication
history after login eviction.

The CQM integration also owns the reproduced bucket-arithmetic and partial-start
cleanup prerequisites. Monitoring must preserve full echo counts, averages and
extrema across the 65,536th echo, retain accepted configured intervals exactly
and keep the intended native loss values without restricting configuration.
A module-probe refusal after observer and replay acquisition must release those
original resources before returning, leave copied callbacks inert and permit a
useful healthy UDP retry.

The new framed-route limit refusal and failed withdrawal notification also
retain route-owned diagnostic events. Their logs and runtime doctor entries
identify the original birth and cause without inventing a native cleanup
receipt or treating a later success as proof that another session released it.

The strict48 allocator instrumentation follows the reviewed per-tunnel SID
entropy source. Its complete parsed-method guard still rejects allocator
drift. The overlay changes only the bounded candidate choice while retaining
the actual entropy read, errors, zero/collision guards and retry bound.

The original research recorded four L2TP defects found while
`spec-single-owner-lifecycle-5-session-end-locals` was written. They moved here
because they concern identity and publication rather than event ordering;
each has a journal row (Owned journal rows). The table preserves that initial
problem statement. The current implementation contract is in
[Approved readable selection and implementation admission](#approved-readable-selection-and-implementation-admission)
and its [packed identity binding](#packed-identity-binding-for-phase-2).

| Part | Original operator symptom | Goal |
|------|------------------------------|------|
| Session id reused in one tunnel | A new subscriber gets an address or a teardown that belonged to the session that held its id before it | A PPP event of an ended session changes nothing on the session that reused its id |
| Observers keyed by session id alone | With sessions on more than one tunnel, a subscriber's route is withdrawn when another tunnel's session with the same id gets an address or ends, and the two share one event history | Each session has its own route record and its own event history |
| Teardown by session id alone | `clear l2tp session id <id>`, a RADIUS Disconnect-Request, a session timeout or an idle timeout ends a session with that id on another tunnel | Each of the four ends the session it names and no other |
| One slow RADIUS server stalls every tunnel | While a RADIUS accounting server is slow, no tunnel's control packet is handled for up to 10 seconds per session that ends: hellos are missed and healthy tunnels time out | A slow accounting server delays only the accounting record it is asked for |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: every plugin can run forked. `authradius` reaches the reactor through the EventBus and the service locator, never through reactor state.
  → Constraint: a goroutine is a long-lived worker on a channel, never one per event.
- [ ] `ai/doors/access-security.md` - L2TP, PPP, the subscriber plugins, RADIUS
  → Constraint: `ppp` is a leaf with no component import. A value the reactor gives a PPP session is opaque to `ppp`.
  → Constraint: subscriber auth, pool and shaper reach a session through the `l2tp/subscriber` handler registries and the session events.
  → Constraint: a RADIUS server and a RADIUS dynamic-authorization client are outside parties. The Acct-Session-Id form and the Disconnect answer codes are contracts.
- [ ] `ai/digests/subscriber.md` - the reactor, the PPP driver, the bridge, accounting and CoA
  → Constraint: RADIUS accounting subscribes to the L2TP session events only. PPPoE sessions produce no accounting record today, which is not this spec's subject.
- [ ] `docs/guide/l2tp.md` - `clear l2tp session id <id>` and `show l2tp session id <id>`
  → Constraint: both commands name a session by its id alone today. A change to what the operator types is a CLI contract change (`ai/rules/cli.md`).
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - the per-tunnel order ticket and the reader contract, built by the closed `spec-single-owner-lifecycle-5-session-end-locals` (its "The order ticket", R-1 and AC-18 are in git history)
  → Constraint: that spec orders the notifications of one tunnel with a ticket taken under `tunnelsMu` and waited for after it is released. It leaves the reactor goroutine's synchronous publication as it is.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2866.md`, read against `rfc/full/rfc2866.txt` Section 2
  → Constraint: Section 2: "It is recommended that the client continue attempting to send the Accounting-Request packet until it receives an acknowledgement, using some form of backoff."
  → Constraint: Section 2: "Retry and fallback algorithms are the topic of current research and are not specified in detail in this document." The RFC sets no bound and no queue: where the Stop waits is Ze's design.
- [ ] `rfc/short/rfc2661.md` - session ids
  → Constraint: not read for this skeleton. The design reads in `rfc/full/` what RFC 2661 says about the scope of a Session ID and about reusing one, and quotes it.
- [ ] `rfc/short/rfc5176.md` - Disconnect-Request session identification
  → Constraint: not read for this skeleton beyond the quotes already above `findSessions` and `handleDisconnect` in `coa.go`.

**Key insights:**
- A session id is unique inside its tunnel only, and a freed id can be drawn again at once. Every defect of the first three parts is one fact dropped somewhere: the tunnel id, or which session the id meant at the time.
- Most readers already carry the pair. The event payloads, the subscriber bridge, RADIUS accounting, the shaper, CoS, the pool and the PPP driver all key by tunnel id and session id. The route observer, the CQM observer and the teardown by id are the ones that drop the tunnel.
- The pair does not tell two lifetimes of one id apart. Only the Acct-Session-Id does today, through its counter.
- Accounting-Start and Interim-Update are already sent from a goroutine each accounting session owns. Only the Stop is sent on the goroutine that published the event.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/session.go` - `allocateSessionID` draws a random non-zero 16-bit id that is not in the tunnel's session map now. It keeps no record of freed ids. `removeSession` deletes the id from the map at once, before the kernel teardown and before the PPP session has ended.
- [ ] `internal/component/l2tp/ppp/start_session.go`, `internal/component/l2tp/ppp/events.go` - `StartSession` identifies a session by `TunnelID` and `SessionID` and nothing else ("Opaque to ppp; used only as a key"). Every event type carries the same two fields and no other identity.
- [ ] `internal/component/l2tp/ppp/manager.go` - `spawnSession` keys the driver's sessions by the same pair and refuses a start whose key is still present, with a rejected event. The events channel holds 64 events. For L2TP the reactor never calls `StopSession`: the PPP session ends when the kernel worker closes its socket, after the reactor has already freed the id.
- [ ] `internal/component/l2tp/ppp/session_run.go` - `run` sends a session-down event when the session goroutine ends, and `sendEvent` blocks until the reactor reads it.
- [ ] `internal/component/l2tp/reactor_kernel.go` - `handlePPPEvent`, `handleSessionIPAssigned`, `handleLCPUp` and `handleLCPDown` each look the session up by the event's two ids and act on what they find: write an address and a user name, clear them, or tear the session down. None checks that the session found is the one the event was made for. `handleKernelSuccess` does check, by comparing the session pointer the kernel result carries.
- [ ] `internal/component/l2tp/route_observer.go` - `records` is a map keyed by session id. `OnSessionIPUp` and `OnSessionDown` receive the tunnel id and store it in the record, and look the record up by session id alone. A second tunnel's session with the same id finds the first one's record: its address replaces the first, and the first is withdrawn as the previous address.
- [ ] `internal/component/l2tp/observer.go` - `Observer.sessions` is keyed by session id. `recordEvent` and `ReleaseSession` take the session id alone, so two tunnels' sessions share one event ring and the end of either releases it.
- [ ] `internal/component/l2tp/teardown.go` - `teardownSessionByID` takes a session id and ends the first session that holds it on any tunnel, in map iteration order.
- [ ] `internal/component/l2tp/subsystem_snapshot.go` - `Subsystem.TeardownSession` takes a session id and tries each reactor in turn.
- [ ] `internal/component/l2tp/cmd/l2tp.go` - `handleSessionTeardown` serves `clear l2tp session id <id>`. It parses one id, and calls `LookupSession`, `RecordDisconnect` and `TeardownSession` with it.
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` - `findSessions` matches a request against each session, comparing the Acct-Session-Id with the tunnel id and session id prefix, and returns session ids only. `handleDisconnect` passes the one id to `TeardownSession`.
- [ ] `internal/component/l2tp/session_timeout.go` - `runSessionTimeout` and `runIdleTimeout` hold the tunnel id and the session id, log both, and call `teardownSessionByID` with the session id alone.
- [ ] `internal/component/l2tp/reactor.go` - `run` is one goroutine. It handles received packets, ticks, dial requests, kernel results and PPP events, one at a time.
- [ ] `internal/component/plugin/server/engine_event.go` - `dispatch` calls each in-process handler in turn on the goroutine that published.
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `onSessionIPAssigned` creates the accounting session and starts `runAccounting`, a goroutine that sends the Start and then one Interim-Update per interval. `onSessionDown` cancels that goroutine and then sends the Stop itself, through `sendAcctStop`, under a 10-second context. `genSessionID` builds the Acct-Session-Id from the tunnel id, the session id, a boot tag and a counter.
- [ ] `internal/component/radius/client.go` - `Exchange` retransmits to one server inside the call (three attempts of three seconds by default), and `SendToServers` moves to the next server. Nothing keeps a request after the call returns.
- [ ] `internal/component/l2tp/subscriber_bridge.go`, `internal/component/l2tp/plugins/shaper/shaper.go`, `internal/plugins/cos/handler.go`, `internal/component/l2tp/plugins/pool/register.go`, `internal/component/l2tp/events/events.go` - the bridge keys the registry by a string built from tunnel id and session id. The shaper, CoS and the pool key by the pair. The session event payloads carry both ids.

Not read, and marked so wherever this spec relies on it: `LookupSession` and
`RecordDisconnect` behind the service locator, `handleSessionUp`, the place the
PPP driver removes a session that ended by itself, `allocateLocalTID`, and
whether a tunnel id is unique across the reactors of one subsystem.

**Behavior to preserve:**
- The Acct-Session-Id form and the rule that Start and Stop carry the same one.
- One Accounting-Stop for each Accounting-Start, with the terminate cause the teardown named.
- The event payloads and the event names.
- `ppp` importing no component.
- The order of one session's notifications that `spec-single-owner-lifecycle-5-session-end-locals` establishes.

**Behavior to change:**
- A PPP event reaches only the session it was made for.
- The route observer and the CQM observer keep one record per session.
- A clear, a Disconnect-Request and the two session timers end the session they name.
- The reactor goroutine does not wait for a RADIUS server.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The tunnel id and session id pair in the session events | `internal/component/l2tp/events/events.go` `SessionDownPayload`, `SessionUpPayload`; the IP-assigned payload read at its use in `acct.go` | Every L2TP session event names its session by both ids | Extends: the observers and the teardown use the pair the events already carry. Open: whether the pair becomes one typed value |
| The pair as a map key | `acct.go` `sessionKey`, `shaper.go` and `cos/handler.go` `sessionKey`, `pool/register.go`, `ppp/manager.go` `sessionKey`, `subscriber_bridge.go` `l2tpSessionID` | Five readers and the PPP driver each declare their own two-field key or string | Extends or replaces, by the design's answer to the first open question |
| The tunnel id already passed to the route observer | `internal/component/l2tp/route_observer.go` `OnSessionIPUp`, `OnSessionDown`, `routeRecord` | Both calls receive the tunnel id and the record stores it. Only the map key leaves it out | Extends: the key takes the fact the call already has |
| The tunnel id already held by the session timers | `internal/component/l2tp/session_timeout.go` `runSessionTimeout`, `runIdleTimeout` | Each timer goroutine holds both ids and passes one | Extends: the teardown takes both |
| The tunnel id already matched by CoA | `internal/component/l2tp/plugins/authradius/coa.go` `findSessions` | Matches the Acct-Session-Id against the tunnel id and session id prefix, then returns the session id | Extends: the match returns what it matched |
| Acct-Session-Id with a boot tag and a counter | `internal/component/l2tp/plugins/authradius/acct.go` `genSessionID` | The one identifier today that differs between two lifetimes of the same tunnel id and session id | Open: whether the session identity the reactor needs is the same kind of value. It is minted by a plugin, after the address is assigned, so it does not exist when the PPP session starts |
| Pointer identity for a late kernel result | `internal/component/l2tp/reactor_kernel.go` `handleKernelSuccess` | Refuses a setup result whose session is no longer the live one | Extends by pattern, or does not apply if ids are quarantined instead: second open question |
| The PPP driver's duplicate refusal | `internal/component/l2tp/ppp/manager.go` `spawnSession` | Refuses a start whose key is still held by a session that has not ended | Extends or stays as it is: it already tells the reactor that the old PPP session is still alive, by a rejected event that today tears the new session down |
| The per-session accounting task | `internal/component/l2tp/plugins/authradius/acct.go` `runAccounting`, `interimLoop` | One goroutine per accounting session sends the Start and the Interim-Updates, and ends when `onSessionDown` cancels it | Extends, or does not apply: third open question. It is the goroutine that already waits on the RADIUS server for this session |
| RADIUS retransmission and failover | `internal/component/radius/client.go` `Exchange`, `SendToServers` | Retransmits and fails over inside one call, bounded by the caller's context. No queue outlives the call | Does not apply as a queue: there is none to join. RFC 2866 Section 2 recommends retrying with backoff and specifies no algorithm |
| The per-tunnel order ticket | `internal/component/l2tp/session.go` `issueNoticeTicket`, `drainPendingTeardowns`; `docs/architecture/l2tp/subscriber-session-model.md` (built in `bae3903dbb` by the closed `spec-single-owner-lifecycle-5-session-end-locals`) | Will order the notifications of one tunnel without holding a lock while a reader runs | Extends: whatever moves the Stop wait keeps that order, and a quarantine or an identity check is taken under the same `tunnelsMu` |
| Canonical subscriber ID already carried in PPP admission and provider requests | `subscriber/id.go` `SessionID`, `ppp/start_session.go` `StartSession.SubscriberID`, `subscriber_bridge.go` `l2tpSessionID` | The canonical string separates access namespaces and full transport identifiers; L2TP still constructs it from TID and SID alone | Extend the existing carrier rather than introduce a parallel identity. Its L2TP producer must distinguish listener ownership and logical session birth before admission; numeric coordinates remain metadata |
| Full issued accounting-ID matching | `authradius/acct.go` `genSessionID`, `currentSessionIDs`; `authradius/coa.go` `findSessions` | CoA compares the complete boot-tag/counter accounting ID, then loses the match by returning SID alone | Preserve the full match and carry its exact captured target through teardown. This current source supersedes the skeleton's prefix-matching description |

## Open design questions

These questions were open in the original research and recorded the reading
still needed then, including RFC 2661 and relevant FRR, BIRD or accel-ppp source.
The alternatives are retained as research history. Current decisions are in
[the approved implementation design](#approved-readable-selection-and-implementation-admission)
and [packed identity binding](#packed-identity-binding-for-phase-2).

| # | Question | What each answer changes |
|---|----------|--------------------------|
| 1 | One typed session identity that every consumer uses, or a per-tunnel quarantine of freed session ids? | A typed identity that tells two lifetimes apart closes parts 1, 2 and 3 at every consumer and changes each key and the PPP start and event types. A quarantine keeps the ids as they are and closes part 1 only, by never reissuing an id while its old PPP session can still report. Parts 2 and 3 then still need the tunnel id added at each place |
| 2 | Does the identity include the lifetime, or only the tunnel id and session id? | With the lifetime, a late PPP event is refused wherever it arrives. Without it, the pair is enough for parts 2 and 3 and part 1 needs the quarantine |
| 3 | Where does the Accounting-Stop wait move: onto the session's own accounting task, onto one accounting worker with a queue, or elsewhere? | The session's task already exists and already waits on the server, and it must then outlive the session it accounts for. A worker with a queue needs a bound and an answer for a full queue. Either must still send the Stop at a daemon stop, which `Stop` and `stopDetached` do today under their own 10-second bound |
| 4 | What does the operator type to clear one session? | Keeping `clear l2tp session id <id>` means a second word for the tunnel when the id is held on more than one, or a refusal that names the tunnels. Changing the command changes `show l2tp session id <id>` with it, and the CLI page |
| 5 | Is a tunnel id unique across the reactors of one subsystem? | Not read. If it is not, the pair itself is ambiguous in the subscriber registry and in every reader, and the identity needs the reactor too |
| 6 | Does this spec's identity work belong in `plan/release/fixes/spec-typed-identity-keys.md`? | That sibling's subject is keys that omit a fact, and it still awaits its own design pass. The three identity rows here are candidates for it. Until its design decides, they are owned here, and this spec does not edit that sibling |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- L2TP control datagrams admit a logical session; PPP typed events report its negotiated state.
- Registered CLI/web commands, RADIUS requests and captured timers select an existing admission.

### Transformation Path
1. Admission captures one canonical birth and exact attempt/control before asynchronous work.
2. The reactor validates that owner, then publishes carried identity through existing lifecycle/ticket paths.
3. Observers and policy/accounting consumers use that full identity; commands resolve numeric coordinates to one captured owner before action.
4. Terminal route/policy completion and ordered accounting retain the original owner through cleanup, preserving every non-target session. The detailed writer/reader table below governs each boundary.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | Typed lifecycle identity, JSON CoS owner transactions and registered session command keywords | Source census complete; scoped and real-path proof owed |

### Changed-Wire Census

**Search evidence:** The research continuation and final census below record searches across Go/YANG/templ/JS/shell/Python/ci/et, known producer/consumer controls, every migration and preserved speaker. Independent admission corrected Echo, synthetic prefix and the exact CoS JSON writer/decoder before test authoring.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Lifecycle/echo identity | writer and reader | Reactor/event payloads → bridge, observer, accounting, shaper, CoS and CoA health | Actual producer/subscription census in the research continuation | Migrate explicit typed canonical birth and preserve exact owner/network fences |
| CoS prepare JSON | writer and reader | `coa_transaction.go` `confirmCoA`, `callCoAOwner` → CoS `executeSessionTransaction` | Actual JSON marshal and strict decoder read | Migrate canonical ID together; retain authenticated transaction/token and receipt semantics |
| Command/web selectors | writer and reader | Registered L2TP command owner, real fixture/interop writers and web links/forms | Actual command-string and template census below | Preserve unique numeric callers; qualify collisions; resolve once; web uses exact captured birth |
| Prefix handler API | writer and reader | `fakel2tp` `poolPrefix` → registered prefix handler/releaser | Actual synthetic writer and bounded no-production-caller census | Migrate the reachable writer/API to carried ID; preserve prefix allocation/release behavior |

### Integration Points
- Existing canonical admission/control, per-tunnel notice order, typed EventBus, metadata/registry, command dispatcher, observer replay and accounting task; extend these rather than add parallel lifetime or selector managers.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes, by design | Commands use registered dispatcher; lifecycle readers use the bus and checked terminal paths |
| No unintended coupling (components stay isolated) | Yes, by design | Shared identity remains a leaf/value contract; PPP imports no component implementation |
| No duplicated functionality (extends existing, does not recreate) | Yes, by design | Extend coordinated exact admission, control, terminal receipts and existing accounting owner |
| Zero-copy preserved where applicable (refs, not copies) | Yes, by design | No packet encoding or buffer-copy change; mint identity once per logical admission |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A PPP event queued for an ended session can reach a session that reused its id | Historical tuple-only source admitted this; current live `pppSessionLocked` requires the bound exact attempt/control | A source-level guard is not runtime proof; retain the positive successor regression and do not redo the current mechanism | New queued-event/reuse regression plus current `pppSessionLocked` producer read | broken on current live source; regression owed |
| A-2 | A tunnel id is unique across the reactors of one subsystem | `newL2TPReactor` seeds from the listener port and `allocateLocalTID` checks that reactor alone | A tuple key remains ambiguous across listeners; approved design uses canonical birth | Constructor, allocator and per-listener `Subsystem.Start` source read | broken; accounted for in approved design |
| A-3 | The Accounting-Stop is the only reader work on the publishing goroutine that waits on a remote party | Shaper Apply/RestoreOriginal and CoS updateQoS are synchronous too | Do not claim every reader is nonblocking; isolate unanswered RADIUS in the control proof and preserve backend completion | Read the actual shaper, CoS and event-dispatch producers | broken; accounted for in approved design |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stable logical session ID is mistaken for authority over a later network phase | An old family/down/provider operation affects replacement negotiation on the same transport | Keep the existing attempt/control, pending-event and network-lifetime fences; the public ID alone grants no mutation or cleanup authority |

### Wrong assumptions corrected before implementation

The skeleton's subsystem-unique TID and RADIUS-only slow-reader assumptions
were wrong. The approved design already incorporates listener/lifetime birth
and a precisely scoped accounting control proof, so these findings do not
invalidate it. The later exact PPP attempt/control and timeout work is applied
locally; audit and test it instead of replacing it or calling it verified.

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Another subscriber loses routes/history/policy or is disconnected; a blocked publisher causes healthy tunnels to time out; incorrect terminal ownership corrupts billing |
| How is it reverted? | Fix forward through coherent producer/consumer commits; retained exact-owner guards and scoped tests protect existing sessions. No destructive git rollback or partial payload migration |
| Who else touches this path? | `spec-single-owner-lifecycle-5-session-end-locals` (closed) edits `reactor_kernel.go`, `teardown.go`, `reactor.go` and `session.go`. `spec-rfc5176-remaining-defect-classes` edits the CoA path |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| PPP assigned address and peer/local down | → | Reactor, checked route observer and lifecycle subscribers | `TestRouteObserverKeepsSameSessionIDOnTwoTunnelsApart`, `TestRouteDownKeepsOtherTunnelReplay` |
| Real typed session/echo EventBus events | → | `wireObserverSubscriptions`, CQM event rings and release | `TestObserverKeepsSameSessionIDOnTwoTunnelsApart`, `TestObserverDownKeepsOtherTunnelHistory` |
| Session command dispatch with SID/TID/listener | → | Registered schema, common resolver, captured exact owner | `TestSessionCommandTargetsCanonicalAdmission`, `test/l2tp/session-selector-scope.ci` |
| Unanswered RADIUS Stop and healthy peer HELLO | → | Real accounting event reader and reactor control path | `TestSlowAccountingStopDoesNotBlockOtherTunnelControl`, `test/l2tp/slow-accounting-control.ci` |
| Real external LAC subscriber population | → | Existing complete scale checker and selected teardown path | `l2tp-many-sessions-kl2tpd` |

## Acceptance Criteria

The first two criteria originated as AC-19 and AC-20 in
`spec-single-owner-lifecycle-5-session-end-locals`. These rows state the required
outcomes for [the approved implementation design](#approved-readable-selection-and-implementation-admission),
including [packed identity binding](#packed-identity-binding-for-phase-2) and
[retained observer ownership](#retained-observer-ownership-across-restart).

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The PPP driver has a queued event for a session that ended, and a new session on the same tunnel has drawn the same session id | The event changes nothing on the new session and publishes nothing: its address, user name, negotiated values and lifetime are those its own PPP session reports. One debug line names the refused event | Events of the new session's own PPP session |
| AC-2 | Two sessions on two tunnels hold the same session id | The route observer and the CQM observer keep one record for each: an address assigned to one leaves the other's route injected, and the end of one leaves the other's record and route in place | None: the two are different subscribers |
| AC-3 | Two sessions on two tunnels hold the same session id, and the operator clears one, or a Disconnect-Request names one, or one reaches its session timeout or idle timeout | The session that was named ends, with its CDN and its Accounting-Stop, and the other stays established and keeps its route | None: the two are different subscribers. What the operator types is question 4 |
| AC-4 | A RADIUS accounting server answers nothing, and a session on one tunnel ends | The reactor keeps handling the control packets of every tunnel: a hello on another tunnel is answered inside its retransmission interval, and no healthy tunnel is torn down. The Stop for the ended session is still sent, and retried as the design states | A Stop that no server acknowledged before its bound: the log names it |
| AC-5 | A tunnel's peer source address changes mid-life (the dispatch path overwrites `peerAddr` with `pkt.from`), then the tunnel is discarded or closed | `discardTunnelLocked` removes the `tunnelsByPeer` entry inserted for that tunnel, by the key stored at insert; the index holds no entry for it afterwards (moved from `spec-typed-identity-keys`, 2026-11-14) | none: every tunnel's index entry is removed with it |
| AC-6 | A native gauge or histogram series retention request has invalid UTF-8 or the wrong label count, followed by a valid Unicode request | The invalid request returns its cause and a zero lease without publishing a sample; the valid request publishes its measured value, and its final release removes that series | Generic custody keys remain arbitrary strings; disabled metrics keep their no-effect leases |
| AC-7 | A retained login receives Down from an older phase contributor after a newer birth assigns that login, or receives the same consumed Down again | The newer phase, samples and metrics survive; repeated Down does not promote ended history. Current-contributor Down, same-birth renegotiation, full event-ring capacity and named pre-IP proxy failures remain effective | Login phase describes its selected contributor rather than an aggregate session count. Login eviction forgets provenance; a later named Down can create a normal Down observation |
| AC-8 | An update exceeds the 64 framed-route bound, or the void withdrawal notification cannot confirm removal | A covered operator log, retained route-owned error event and runtime doctor finding carry the birth, wire labels, cause and next command. Reporting holds neither observer lock; the 64-route neighbor and other births stay effective, and checked withdrawal retains its original retry receipt | Error events describe history. Successful retry leaves that history intact and makes no claim about another session's native cleanup |
| AC-9 | The scale lab stages the reviewed per-tunnel allocator, or its existing error-predicate, candidate, zero-guard or receiver drift control | Only the reviewed complete canonical allocator receives the bounded candidate insertion; every drift control refuses before compilation. The original source bytes and owned cleanup stay intact. `TestScaleAllocatorOverlayIsolation`, `TestScaleBuildInputs` and the unchanged `l2tp-many-sessions-kl2tpd` guest scenario prove staging and the full survivor/accounting tail | Ordinary Ze and relay builds remain effective because neither requests the scale-only allocator overlay |
| AC-10 | Admitted CQM observation reaches the 65,536th echo, or uses an accepted short, 3s, 100s or 101s echo interval | Current and finalized buckets preserve the full count, average and extrema without wrap; the public quality view retains the precise configured interval, and live/native loss preserves the intended values using the full expected count. When the interval yields no expected echo in a bucket, the public quality view and registered echo command omit unknown loss; an explicitly computed zero remains present. No configuration restriction is introduced | Existing accepted positive intervals and disabled/no-expected-echo behavior remain supported; native metric calculation is unchanged |
| AC-11 | The actual Start module-probe step refuses after acquiring the original observer and replay subscriptions, followed by Stop and a healthy retry | Refusal immediately closes the original observer and releases every original subscription; an already copied callback has no effect. The retry binds a real UDP listener, admits useful current-generation observation and releases all callbacks on final Stop | The original probe cause remains identifiable through wrapping; no failed-start observer or callback remains live |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs subscribers with equal numeric session IDs on different tunnels/listeners | PPP admission → carried identity → independent routes and event history | Collision regressions, real scoped selector functional test and strict scale lab |
| 2 | Clears one subscriber using readable qualifiers | Registered CLI/web dispatcher → one captured target → exact terminal completion | Selector functional proof plus selected Disconnect/survivor tail |
| 3 | Loses the accounting server while other subscribers stay active | Terminal snapshot → ordered accounting owner; reactor continues peer control | Slow-accounting control functional proof |

## 🧪 TDD Test Plan

### Unit Tests
The first three names came from
`spec-single-owner-lifecycle-5-session-end-locals` with AC-19 and AC-20.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLatePPPEventIsRefusedOnAReusedSessionID` | `internal/component/l2tp/` (file named in design) | AC-1 | |
| `TestRouteObserverKeepsSameSessionIDOnTwoTunnelsApart` | `internal/component/l2tp/route_observer_test.go` | AC-2 at the route observer | |
| `TestObserverKeepsSameSessionIDOnTwoTunnelsApart` | `internal/component/l2tp/observer_test.go` | AC-2 at the CQM observer | |
| `TestScaleAllocatorOverlayIsolation` | `internal/le/interoplab/l2tp/scale_overlay_test.go` | AC-9: exact per-tunnel allocator shape, drift refusals, candidate-only replacement and cleanup | native proof owed after coherent application |
| `TestScaleBuildInputs` | `internal/le/interoplab/l2tp/scale_overlay_test.go` | AC-9: the scale daemon alone declares allocator instrumentation | native proof owed after coherent application |
| `TestObserverBucketEchoCountCrossesUint16Boundary` | `internal/component/l2tp/observer_count_test.go` | AC-10: full live/finalized count, average and extrema across the 65,536th echo | semantic red recorded; source repair pending |
| `TestObserverPositiveEchoIntervalsKeepFullExpectedCount` | `internal/component/l2tp/observer_count_test.go` | AC-10: accepted short intervals, precise quality interval and original native loss values | semantic red recorded; source repair pending |
| `TestObserverEchoStateKeepsExactConfiguredInterval` | `internal/component/l2tp/observer_interval_test.go` | AC-10: preserve the actual accepted 3s and 101s intervals rather than reconstructing them from a bucket count | semantic red recorded; source repair pending |
| `TestObserverEchoLossDistinguishesUnknownAndMeasuredZero` | `internal/component/l2tp/observer_loss_test.go` | AC-10: actual admitted observation preserves 1s measured loss, 100s computed zero and unknown 101s loss separately | semantic red recorded; optional-loss correction in progress |
| `TestEchoCommandKeepsMeasuredZeroAndOmitsUnknownLoss` | `internal/component/l2tp/cmd/echo_loss_test.go` | AC-10: the registered command emits measured loss and explicit computed zero, while omitting unknown loss without losing the configured interval | semantic red recorded; optional-loss correction in progress |
| `TestObserverProbeRefusalUnwindsBeforeStopAndRetry` | `internal/component/l2tp/observer_probe_unwind_test.go` | AC-11: immediate failed-probe closure, copied callback refusal, useful real UDP retry and zero final subscriptions | scoped semantic red then green recorded; wider verification owed |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Numeric session/tunnel selector | 1..65535 | 65535 selects a matching admitted row | 0 refuses | 65536 refuses during parsing |
| Listener qualifier | Existing configured listener-name domain | Configured exact name resolves | Empty/omitted is allowed only when the remaining selector is unique | Unknown/out-of-domain name refuses |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `session-selector-scope` | `test/l2tp/session-selector-scope.ci` | Unique bare SID, qualified collision, ambiguity without effects and stale-target survivor | planned, draft before promotion |
| `slow-accounting-control` | `test/l2tp/slow-accounting-control.ci` | Healthy tunnel control remains responsive while terminal accounting is unanswered | planned, draft before promotion |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-many-sessions-kl2tpd` | `test/interop-l2tp/scenarios/l2tp-many-sessions-kl2tpd/` | kl2tpd v0.1.8, pppd 2.5.5, FreeRADIUS and FRR | All 48 real subscriber lifetimes/routes/traffic, exact chosen teardown, 47/46 survivors and complete accounting | current retained dependency red; unchanged full green owed |

## Files to Modify
- `internal/component/l2tp/route_observer.go`, `observer.go`, `snapshot.go`, `subsystem_snapshot.go` - complete carried-ID records, releases, reads and exact selector resolution
- `internal/component/l2tp/cmd/l2tp.go`, `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang` - readable scoped selectors, completion, refusals and diagnostic display
- `internal/component/l2tp/events/events.go`, `subscriber_bridge.go` - explicit immutable identity through the lifecycle boundary; coordinate canonical producer owner
- `internal/component/l2tp/session.go`, `session_fsm.go`, `session_initiator.go`, `reactor_kernel.go`, `reactor.go`, `teardown.go`, `session_timeout.go`, `subsystem.go`, `tunnel.go` - coordinated admission/control/timer/index/tick/notice contracts; no competing actor implementation
- `internal/component/l2tp/plugins/authradius/acct.go`, `coa.go`, `internal/component/l2tp/plugins/shaper/shaper.go` - coordinated exact accounting, disconnect and rate ownership, integrated here
- `internal/plugins/cos/handler.go`, `session_state.go`, `session_transaction.go` - carried-ID CoS state, metadata and captured transaction ownership
- `internal/component/web/handler_l2tp.go`, L2TP page data and list/detail templates - exact-ID web targeting through the shared dispatcher and safe stale-page refusal
- `internal/le/interoplab/l2tp/`, `internal/le/interoplab/topology/`, `internal/test/fixture/` - revalidate actual command writers; migrate only those requiring scope under colliding sessions, preserving all assertions. Keep the exact allocator guard in `scale_overlay.go` and its existing `scale_overlay_test.go` controls aligned with the per-tunnel entropy source
- `docs/guide/l2tp.md`, `docs/guide/command-reference.md`, `docs/architecture/l2tp/subscriber-session-model.md`, `docs/architecture/l2tp/l2tp-9-observer.md`, `docs/architecture/traffic/cos-dynamic.md`, `ai/digests/subscriber.md`, `features/l2tpv2-bng.md` - keep each changed phase's public contract true
- `docs/architecture/l2tp/bng-1-radius-attributes.md` - preserve accounting attributes and cause mappings; update exact lifetime/terminal ownership where the coordinated accounting phase changes it
- `docs/architecture/web-workbench-pages.md` - repair stale disconnect grammar and describe exact page target/readable coordinates in the web phase
- `docs/research/l2tpv2-ze-integration.md` - update affected lifecycle identity and accounting integration notes, pointing to the current subscriber model as authority
- `docs/research/l2tpv2-implementation-guide.md` - preserve normative packet/SID description: internal canonical birth does not change RFC header labels or protocol format
- `docs/comparison.md` - preserve current BNG capability comparison; this fix changes existing ownership correctness, not the comparison's supported capabilities or accounting fields

## Files to Create
- `internal/component/l2tp/session_identity.go` - coordinated canonical birth producer and getter, owned by the active actor session
- `internal/component/l2tp/session_selector.go`, `session_selector_test.go` - validated readable selectors and exact target resolution; use the existing capture seam where it already supplies this behavior
- `internal/component/l2tp/route_identity_test.go`, `observer_identity_test.go` - independent colliding-tunnel/listener records and positive surviving state
- `internal/component/l2tp/cmd/session_selector_test.go` - real dispatch/grammar selection, ambiguity and stale-target cases
- `internal/plugins/cos/session_identity_test.go` - independent CoS state and transaction lifetime regression
- `test/l2tp/session-selector-scope.ci`, `test/l2tp/slow-accounting-control.ci` - real daemon entry-point proof, drafted outside the live suite and promoted only after passing

Final helper-file names may follow the existing owning packages discovered by
the audit; the behavior, census and proof do not shrink when a helper already
exists. Record the actual symbol/file in the phase handoff.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | Extend the existing session command leaves in `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang`; no new config or RPC owner |
| YANG validation constraints | Yes | Nonzero native uint16 SID/TID constraints; listener uses the existing configured server-name type and bounds |
| YANG custom validators | Yes | Existing CLI owner resolves listener names and supplies completion from the same current listener inventory; unknown names refuse before lookup/mutation |
| CLI commands/flags | Yes | `internal/component/l2tp/cmd/l2tp.go`: existing session operations accept tunnel/listener keyword qualifiers, preserving unambiguous numeric callers |
| CLI grammar (keyword before value) | Yes | `id`, `tunnel`, `listener`, `reason`, `cause`; missing target never means all |
| Editor autocomplete | Yes | Schema-derived keywords plus registered listener-name completion in the existing command owner |
| Functional test for new RPC/API | Yes | `test/l2tp/session-selector-scope.ci`, plus existing clear/subscriber fixture; new tests draft and promote |
| Pipe completeness | Yes | Preserve the existing structured response/pipe path for lists, detail, history and traffic; new identity and listener fields remain data |
| Env var registration | N-A | No new environment setting or operational knob |
| Doctor check for runtime dependencies | N-A | No new runtime path, socket, module, binary, certificate or backend dependency; reuse existing admission/cleanup checks |
| Doctor code per problem class | Yes | Argument resolution produces an owning CLI operator error with the missing qualifier/fix. Stale-event refusal is Debug. Existing cleanup/accounting failures retain their registered owning checks; a new Warn/Error requires its diagnostic code and doctor disposition in that implementing phase, never an unlabelled log |
| Prometheus counters/metrics | Yes | Preserve current route inject/withdraw and subscriber/accounting counters, with independently owned records; no new per-ID metric labels or cardinality |
| BGP family surface (new SAFI / capability / attribute) | N-A | Existing IPv4/IPv6 unicast route producer only; no new BGP family or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/l2tpv2-bng.md`: record actual scale proof only after it passes; preserve its experimental level |
| 2 | Config syntax changed? | N-A | No persistent configuration change |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: readable qualifiers and ambiguous-target refusal |
| 4 | API/RPC added/changed? | Yes | `docs/guide/l2tp.md` command/JSON examples and owning command schema; shared dispatcher transports keep their generic contract |
| 5 | Plugin added/changed? | Yes | `docs/architecture/traffic/cos-dynamic.md`: canonical CoS state and transaction target; accounting/shaper pages change with their coordinated phases |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md`: selection, identity, event history, accounting ordering and real scale evidence |
| 7 | Wire format changed? | Yes | `docs/architecture/l2tp/subscriber-session-model.md`: lifecycle payload identity. L2TP/PPP packet formats remain unchanged |
| 8 | Plugin SDK/protocol changed? | N-A | Existing generic EventBus/command transport, no SDK or framing change; subsystem payload changes are documented in its model |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | Rejudge reached RFC 2661/2866/5176 tagged units and their records when changed; update source summaries only for claims the actual test proves; no derived ledger commit |
| 10 | Test infrastructure changed? | N-A | Existing native functional and lab runners; scenario proof extends behavior, no new harness rule |
| 11 | Affects daemon comparison? | N-A | Correct existing capability; no new comparison/support claim |
| 12 | Internal architecture changed? | Yes | Subscriber model, `l2tp-9-observer.md` and `ai/digests/subscriber.md`: immutable birth, full keys, captured resolution and ordered accounting |
| 13 | Route metadata keys added/changed? | N-A | Ownership key changes internally; emitted route metadata keys retain their contract |
| 14 | Prometheus counters added/changed? | N-A | Existing metric names and labels retained; guide keeps their semantics accurate |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Existing command schema and owning L2TP model describe qualifier/SubscriberID additions; names and plugin inventory unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run native `spec citation anchors` over this spec; update every affected claim in its implementing phase and retain the derived inventory in session state |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Revalidate L2TP guide/reference, web disconnect and actual fixture/interop command writers; unambiguous numeric forms remain valid |

## Implementation Steps

1. **Phase: Wiring** -- current-tree audit and new additive collision/entry-point tests. Run each meaningful red before feature fixes; do not replace source behavior with a stub where the entry point already exists.
2. **Phase: Observer identity** -- consume the coordinated typed canonical birth/events, migrate route/CQM ownership and preserve checked terminal/replay semantics. Run the new red tests green and all reached old tests.
3. **Phase: Readable selection** -- registered numeric SID/TID/listener qualifiers, completion, exact snapshot/control resolution, all single-session read/action surfaces and web targeting; run real command/functional proof.
4. **Phase: Consumer/terminal integration** -- remaining CoS/prefix/JSON consumers plus coordinated actor/accounting/shaper fixes and owned retry/index/reentry cases. Prove useful output, preserved survivors, actual record ordering and responsive control.
5. **Phase: Lab and review** -- run the unchanged full external-peer scale scenario through all teardown/survivor/accounting assertions, reached regression suites and independent review, then closure. Record actual commands and outcomes in owning state.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Equal numeric coordinates and reused labels never confer another admission's mutation authority; stale targets preserve replacements |
| Naming | Canonical subscriber identity, numeric coordinates, network lifetime and exact control remain distinct concepts |
| Data flow | Full writer/reader census, including Echo, prefix fixture and CoS JSON transaction, carries one identity without reconstruction |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line and raise names the code the Integration Checklist's doctor row lists, and each doctor check carries `Fix` at the phase that holds its state (`ai/rules/user-facing-errors.md`) |
| Rule: typed identity and exact owner | Parse wire identity once into the shared comparable numeric/packed value; preserve exact owner and pending completion receipts |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| L2TP unit tests | `./le test unit pkg path ./internal/component/l2tp/` |
| RADIUS accounting unit tests | `./le test unit pkg path ./internal/component/l2tp/plugins/authradius/` |
| Functional tests | `./le test functional l2tp`, for the tests the design names |
| Journal rows | Each of the four owned rows reads `fixed` with its test names |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Remote stall | A RADIUS server that stops answering, or a LAC that opens and closes sessions quickly, cannot stop the reactor or fill a queue without bound |
| Cross-subscriber effect | No input from one tunnel's peer can end, re-address or withdraw the route of another tunnel's session |

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

- The observers were handed the tunnel id and stored it, and still keyed by the session id. A key that omits a fact the call already has is found by reading the map's declaration, not its callers.
- The two session timers hold both ids and log both. The teardown they call takes one. The narrowing happens at a function signature, where nothing warns.
- Accounting already has a goroutine per session that waits on the RADIUS server. The Stop is the one record sent from somewhere else.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/identifier-reused-after-its-owner-is-gone.md` | 2026-11-11 | L2TP PPP event applied to a successor with a reused session id |
| `plan/journal/key-omits-a-fact-the-builder-uses.md` | 2026-11-11 | L2TP route observer and CQM observer keyed by session id alone |
| `plan/journal/key-omits-a-fact-the-builder-uses.md` | 2026-11-11 | L2TP `teardownSessionByID` ends the first session with an id on any tunnel |
| `plan/journal/slow-reader-stalls-a-shared-goroutine.md` | 2026-11-11 | L2TP reactor goroutine waits on a synchronous Accounting-Stop |
| `plan/journal/retry-forgets-consumed-input.md` | 2026-11-13 | L2TP `handleTick` drops the kernel setup events it collects |
| `plan/journal/callback-reenters-held-lock.md` | 2026-11-13 | L2TP tunnel events emitted while the tunnel's session-order ticket is undelivered |
| `plan/journal/index-key-derived-from-mutable-field.md` | 2026-08-10 | AC-5: L2TP `discardTunnelLocked` deletes from `tunnelsByPeer` by a key rebuilt from the overwritten `peerAddr` |

## Related specs

| Spec | Relation |
|------|----------|
| `spec-single-owner-lifecycle-5-session-end-locals` (closed) | Found these defects and held AC-19 and AC-20 until 2026-11-11. It keeps the L2TP event order (the per-tunnel order ticket) and lists the four rows above as considered and not owned |
| `plan/release/fixes/spec-typed-identity-keys.md` | A sibling still awaiting its own design pass. Its subject is keys that omit a fact. The three identity rows above are candidates for it (question 6). This spec does not edit it |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Research continuation: 2026-11-16

The preceding skeleton records its earlier investigation. This continuation
refreshes those claims against current producers; no implementation or runtime
reproduction was performed during this research phase. Design approval is still
owed before this spec becomes ready.

### Owner instruction and sequence

Thomas authorised the L2TP identity design, then instructed:

> When you have done the work then continue with AC-33 and D-14

This session completes this identity work and the strict `spec-finish-l2tp`
AC-2 proof first, then takes AC-33 in
`plan/release/fixes/spec-network-services-one-off-defects.md`, then D-14 in
`plan/release/fixes/spec-lifecycle-invariants.md`. Each design needs approval
before implementation. AC-2 keeps its complete many-session and survivor proof.

### Refreshed required reading and findings

| Reading / producer | Actionable constraint |
|--------------------|-----------------------|
| `rfc/full/rfc2661.txt`, Section 3.1 | → Constraint: "Session ID indicates the identifier for a session within a tunnel." The wire number does not identify a subscriber across tunnels, listeners or reused lifetimes |
| `reactor.go` `newL2TPReactor`, `allocateLocalTID`; `subsystem.go` `Start` | → Constraint: TID allocation is reactor-local, seeded from the listen port; each listener has its own reactor and PPP driver but shares the subsystem's route observer and bus. Equal TID/SID on two listeners is reachable. A two-field observer key is insufficient |
| `subscriber/id.go` `SessionID`; `subscriber_bridge.go` `l2tpSessionID`, lifecycle handlers | → Constraint: the current L2TP canonical ID is rebuilt from TID/SID. A complete identity must be minted once at logical admission and passed unchanged; consumers must not reconstruct it from display coordinates |
| `reactor_kernel.go` `handlePPPEvent`; `session.go` `allocateSessionID`, `removeSession` | → Constraint: the event lookup checks the current tuple only, and removal makes SID available before old asynchronous work finishes. An old event must be fenced by its captured original owner/lifetime, including positive acceptance of the successor's own events |
| `reactor_kernel.go` `handleKernelSuccess` | → Constraint: the initial captured-session pointer comparison is the existing late-result pattern. Preserve it and cover the later interface-name update with the same owner check |
| `route_observer.go` `OnSessionIPUp`, `OnSessionDown`; `observer.go` `recordEvent`, `ReleaseSession` | → Constraint: both maps are SID-only today. Route state, framed routes, CQM events, releases, history readers and snapshots all need the complete identity; changing only the route map leaves the same defect reachable elsewhere |
| `teardown.go` `teardownSessionByID`; `authradius/coa.go` `findSessions` | → Constraint: teardown chooses the first matching SID; CoA already matches the complete issued accounting ID and then returns SID. Preserve the stronger match and act on that exact captured lifetime, never a fresh numeric lookup |
| `authradius/acct.go` `runAccounting`, `onSessionDown`, `interimLoop`, `stopDetached` | → Constraint: cancelling the task does not join its independent Start/Interim exchange. Stop must be ordered after actual preceding operations without joining on the reactor's publication callback |
| `authradius/acct.go` `counterDelta`, `buildAcctPacket` | → Constraint: terminal records currently read counters from the live interface during packet construction. Capture exact-lifetime counters, cause and accounting ownership before interface cleanup/reuse; deferring that read can bill another lifetime |
| `plugin/server/engine_event.go` `dispatch`; `subscriber_bridge.go` `onSessionDown` | → Constraint: in-process readers and bridge re-publication are synchronous. Keep assignment acknowledgement and lifecycle ordering; making the entire bus asynchronous would break an existing contract |
| `plugins/shaper/shaper.go` `onSessionUp`, `onSessionDown`, `applyTC`; `internal/plugins/cos/handler.go` lifecycle handlers | → Constraint: other readers synchronously call traffic backends or QoS operations. A-3's universal assertion is false. The RADIUS proof must isolate unanswered accounting from these other dependencies, not claim every reader is nonblocking |
| `reactor.go` `handle`, `handleTick`, `discardTunnelLocked` | → Constraint: retain the peer-index insertion key for AC-5; enqueue tick-collected setups instead of discarding consumed work; deliver already-issued session-down tickets before tunnel callbacks that may re-enter teardown |

### Current and coordinated work

| Work | Current state / boundary |
|------|--------------------------|
| Incremental implementation commits | Commit coherent implemented chunks while validation continues. Each commit and this table distinguish implementation, passing checks, known failures and remaining proof; AC-2 stays open until its unchanged complete lab passes |
| Remaining SDK and route support checkpoints | `a7f8b6c446` saves 17 remaining H41 SDK/RPC, server, IPC schema and fixture-engine support files plus the comment-only scale Dockerfile update. `26c533e1e9` saves five companion fixture migrations to the already-landed owner-bound interfaces. These checkpoints preserve bound completion, cancellation settlement and original destination/admission identity assertions. No new verification was run for these commits; incomplete route delivery and the unchanged complete AC-2 lab remain open |
| Completed identity chunks | PPP immutable birth and cached ID landed in `9e2a9a9e67`; captured actor, route and selector changes in `6ee99f1e49`; shared CoS preservation fixtures in `d14120b6d8`; CQM guard baselines in `0e57a7eedd`. These commits do not establish complete scale acceptance |
| Implemented CQM integration | Product, eleven portable regression files and mapped docs landed in `8f9c0fad72`; reviewed existing-test migrations and Linux proof source landed in `1d954af69a`. Original-generation admission, full-birth history, original metric leases, retained readers, failed-start unwind and numeric corrections are implemented and source-reviewed. Post-baseline full root race run passed 662 test/fuzz groups; earlier unchanged command 33 and reached web 14 roots passed. All four new Linux-only capture/publisher bodies passed in the matching Ze 7.3 guest, with modeled PPP I/O and confirmed VM stop. All nine host causal checks now execute the named test successfully on clean source and reach its own assertion failure under the corresponding producer break, including command/web original-history reads and partial metric-admission refusal. Six Linux guest causal checks and remaining scoped consumer/functional/lint checks are still owed |
| Accounting live-counter foundation | Seven real-Driver regressions landed in `b12d7ac1dd`; the early-lifetime fix and model docs landed in `2c442d638d`. Both intended baseline/dual-stack REDs passed their exact race reruns, and all seven pass within the earlier 662-group root integration run. Existing unqualified captured-counter semantics remain intact. Five added current-API regressions exposed missing first-assignment sampling, merged accounting for distinct Births sharing wire labels, premature Stop before held Start/Interim acknowledgements and terminal delivery to the wrong server after reload. They were independently source-reviewed and reached their intended failures under the race detector. The missing pre-forwarding sample now passes its exact rerun. Endpoint ownership, finite terminal admission and ordered asynchronous Stop remain open; the other four accounting failures are not fixed by the counter producer |
| Accounting response kind | `c7baa362b3` keeps an Accounting-Request pending after an authenticated reply of another kind. The signed loopback Stop regression first returned Access-Accept (Code 2), then passed with Accounting-Response (Code 5) on the original datagram after the fix. The full RADIUS client package passed under the race detector. Ordered accounting and original endpoint retirement remain open |
| PPP counter producer | Reviewed counter source and network-qualified capture wiring are implemented. The original pre-forwarding regression is green, all 14 new counter/cleanup roots pass isolated race runs, and the complete PPP package passes 658 groups with no skips or races and 88.4% statement coverage. The five approved existing IPv6 fixtures and original counter/owner-lock regressions remain green. Linux capture/publisher validation, causal cuts, remaining reached packages, scoped lint, accounting consumption and the complete scale lab remain owed |
| Assignment refusal diagnostics | A missing or mismatched original counter receipt cancels the exact assignment before its ACK can enable forwarding. The refusal now retains its cause under the existing runtime doctor check. The new genuine-admission regression was red on the missing code, then green for both refusal cases and a useful original session; all 18 selected command/web capture roots also pass under the race detector |
| Original authentication approval | The readonly original-authentication receipt and one-owner local approval capability are implemented. All 13 new roots pass isolated race runs, covering actual IPCP/ACK transfer, cancellation, exact Birth/network matching, timeout, rejection, useful unclaimed sessions and PPPoE. The complete reached PPP package passes 671 groups with no skips or races and 88.0% statement coverage, preserving existing PAP/CHAP, periodic authentication, ACK, provider-lifetime and relay controls. All four production-guard cuts produce their named test's intended assertion failure after a clean pass. Accounting reservation/consumption, reached integration and scoped lint remain owed |
| Original route-withdrawal regressions | Four new current-API roots are intentionally red: withdrawal loses the accepting consumer episode, loses accepted identity at native and command boundaries, and permits a queued original to install after withdrawal returns. The queue test first needed an independently approved real endpoint prerequisite; its useful independent route now writes before the retired-original assertion fails. These tests preserve the required fence while its producer, lifecycle cleanup and checked result delivery remain in progress |
| Original route withdrawal ID | Native and text withdrawal batches now retain the original admission ID. The command regression passes under the race detector. The reactor regression reaches the original lookup and still fails when the forwarding worker receives the withdrawal. Final writer settlement and accepting-consumer ownership remain open |
| Subscriber rate policy declarations | Checkpoint the existing typed command, rate pair, local controller/transaction and remote-owner declarations with their unit tests. This saves the implemented value and authority boundaries; preparation-lifetime integration, scoped verification and complete subscriber acceptance remain open |
| Counter event value contract | The independently reviewed leaf schema defines a nonwrapping network lifetime and immutable four-counter samples with explicit unavailability. All three new regressions and the complete 11-group event package pass under the race detector, with measured package statement coverage of 88%. The PPP producer is implemented as recorded above. The new same-Birth, next-network capture regression also passes under the race detector; accounting transfer, Linux capture validation, causal cuts and scoped lint remain owed |
| AC-4 terminal transfer | Landed in `93ff938cdb`: the SessionDown callback captures the end of service (cause, elapsed time, Event-Timestamp instant and final counters, the last two inside the accounting panic boundary) and hands the Stop to the session's one accounting task, which sends Start, Interims and then Stop on the session's original endpoint (client, NAS identity, server). Records are keyed by birth plus wire labels. A reload retires the replaced endpoint, whose client closes when its last session's Stop is done. Shutdown and removal join every owed Stop, including ones already handed over, within one shared 10 s bound, then close the waiting clients and log each abandoned Stop by Acct-Session-Id. An unacknowledged record is a runtime doctor finding (`l2tp-radius-accounting`, `doctor-radius-accounting-unacknowledged`) naming the session's Acct-Session-Id. The four `acct_original_task_test.go` regressions committed red in `113944fea8` now pass; the full authradius and diagnostic packages pass under the race detector and scoped lint is clean. Independent review found two defects in the first version, both fixed with regressions proven red: a panic while capturing the end lost the Stop and hung shutdown (observed as an unbounded retirement hang), and a queued Stop reported its send instant (observed 1756900200 against end of service 1756900100). Acct-Delay-Time still counts from the client's first send, as the RADIUS client states. An independent ze-test-judge approved twelve fixture migrations; RFC2869-2.1-1 (both polarities, hand-written mutants) and RFC2869-x-2 were re-recorded from observed reds. Daemon proof in Ze's Linux guest is still owed: the prepared `slow-accounting-control` packet ended the first session through the operator clear, whose Down is published on the CLI goroutine rather than the reactor, so it could not go red; the rewritten fixture ends that session with the LAC's CDN and awaits its first guest run |
| AC-8 overflow implementation | Landed in `6974cea4f1`: the extractor retains one valid route beyond the 64-route limit as a bounded overflow witness; the route owner refuses the whole oversized update before changing records, tokens or counters. Its registered warning, error history and runtime doctor context retain the original Birth and wire labels, minimum observed route count, cause and safe next command. Extractor TDD first passed four useful-64 controls and failed six overflow cases; all ten now pass under the race detector. Both new/existing-Birth preservation cases and the warning, lock-exclusion, history and runtime-doctor root also pass their exact race reruns. Genuine daemon proof, reached integration and scoped checks remain owed |
| AC-8 daemon fixture | Four additive fixture Go files implement two genuine CHAP/IPCP admissions, independent 64/65-route RADIUS profiles, observed route/replay tokens and original daemon warning/error/doctor context. Independent source review found no concrete defect. Both fixture-control roots passed under the race detector: production RADIUS decoding confirms the real wire profiles, and exact replay comparisons reject missing, extra and changed-source entries. These controls do not prove daemon behavior. The ignored `.ci` remains a draft until actual native, cost and 80-iteration stress proof permit promotion |
| AC-8 checked delivery | Still missing: the void consumer/orchestrator boundary loses the existing BGP withdrawal result. Retries must retain the original consumer episode, including after re-registration. Overflow implementation does not establish checked delivery or close AC-8 |
| Readable selection | Captured command reads and YANG wiring landed in `ca08d37b88`, with scoped lint cleared in `66654425e2`. All eight new roots pass their exact race reruns after intended REDs; the reached command package passes 41 test groups, and the unchanged four-case MRU/authentication assertions pass through an independently approved readonly fixture migration. All eight traffic keys retain measured-zero versus unavailable-counter meaning. Web selection's three new roots and the old table fixture pass; both templates are regenerated. After actual default byte mismatches, independent review approved exactly six captured output changes, including the specified `birth` to `subscriber-id` JSON key migration with the same identity value. All six refreshed hashes match those approved bytes; the complete default template comparison and three affected HTTP comparisons pass under the race detector. Wider reached web checks, scoped lint and native daemon delivery remain owed; this partial result does not close Phase 3 or AC-2 |
| Typed API and MCP selection | Landed in `04233cee8f`: shared invocation writing, exact CLI-alias metadata and retained REST/gRPC parameters use one metadata generation and the dispatcher's existing matcher. All nine new roots pass exact race reruns after intended semantic REDs, with ten independently bound fixture verdicts. The genuine YANG boolean correction landed in `44e5d4d9b6`; its real-loader regression and reached YANG/external-schema packages pass. The real-daemon selector test passes its first run and all 80 single-worker repetitions under CPU pressure, with five typed metadata fields, preserved ordinary false values, literal selector values, seven accepted requests, two forced actions and three pre-dispatch refusals. It is promoted to `test/plugin/typed-selector-structured-flags.ci`. Native causal proof passes clean and fails at the first structured-false action count when the original producer is deliberately broken. Reached API, REST, gRPC and MCP packages pass 491 test groups under the race detector, and the command package passes. The reached server run passes 1164 groups and leaves three refusal-message contract failures; the exact registration fixture repair passes. Seven related daemon scenarios still await execution after their attempt stopped at a fleet UI build failure. Scoped lint remains owed. These results do not establish full Phase 3 or scale acceptance |
| Reached integration and remaining consumers | After overflow changes, the root L2TP and RADIUS packages passed 998 test/fuzz groups under the race detector. CoA/Disconnect retain the matched original snapshot and captured action; issued accounting IDs bind to the immutable Birth retained at actual accounting-task creation. All four new wrong-owner/stale-ID roots, the existing 22-case identification root and the full 338-group RADIUS package pass under the race detector, with no failures, skips or races. All 79 independently approved fixture verdicts bind the applied content. Current-source audit confirms CoS enrichment, canonical CoA-to-CoS preparation and prefix allocation/release migrations already exist. Full accounting task-map migration, ordered asynchronous Stop, ownership after rate preparation and idle counter ownership remain coordinated implementation work |
| Opaque PPP attempt/control, exact provider requests and L2TP terminal owner | Active `spec-pppoe-vpp-dataplane` work. Proposed contracts are not current runtime proof. Its owner is coordinating immutable listener ownership and logical session birth; this design extends that contract rather than implements another one |
| Start/Interim completion before Stop, endpoint retirement, exact Disconnect and timeout targets | Active PPPoE/VPP accounting, disconnect and counter work overlaps these goals. Preserve this spec's acceptance obligations and require integrated evidence after those carriers land; do not count private source packets as implemented |
| `plan/release/fixes/spec-radius-acct-start-after-stop.md` | Existing skeleton explicitly requests coordination or merger at this design pass. Its wire-order requirement is inseparable from moving Stop off the publishing goroutine; no independent competing Stop implementation |
| AC-2 retained red | All 48 PPP sessions, addresses, bidirectional traffic and Accounting-Starts were observed before the strict FRR prefix census failed. The unchanged scenario's selected-session teardown, remaining subscribers and accounting tail are still owed after the identity fix |

The shared release fixes require their minimal common carriers and integrated
proof. They do not make completion of the whole post-release
`plan/post-release/features/spec-pppoe-vpp-dataplane.md` a prerequisite for this
release fix. Coordinate and land the shared identity/accounting chunks on their
own merits; neither spec may park its acceptance obligations in the other.

### Source census for the design

Research searched `internal`, `pkg`, `cmd` and `test` across Go, YANG, templ,
JavaScript, shell, Python, `.ci` and `.et`, using PPP event declarations and
emitters, lifecycle subscriptions, SID-only service methods and actual command
strings. Positive controls include the PPP `EventSessionIPAssigned` producer,
the web disconnect command writer and the literal clear assertion in
`test/l2tp/session-clear-leaves-no-subscriber.ci`. This is the current population;
the final changed-wire census must bind the chosen field and command formats
to each carrier before design admission.

| Carrier | Producers and readers to disposition |
|---------|-------------------------------------|
| PPP admission and lifecycle | `ppp/manager.go` admission/rejection; `ppp/session_run.go` LCP/up/down/echo; `ppp/ncp.go` down/IP-assigned; L2TP reactor readers and PPPoE event consumer. Preserve the shared PPP leaf boundary and successor admission |
| L2TP lifecycle bus | Writers in `reactor_kernel.go` and `teardown.go`; readers in subscriber bridge, CQM observer, accounting, CoA health, shaper and CoS. Preserve event names; carry complete identity across each boundary |
| Route ownership and replay | Direct calls from `reactor_kernel.go` and `teardown.go`, route observer state and replay. Preserve checked withdrawal/family-down terminal receipts being added by the active owner |
| Service and CLI | Detail, observer, FSM history, traffic, clear and disconnect audit; `Service` and snapshots. Selection, lookup and mutation must share one complete target |
| Programmatic command writers | Web `handleL2TPDisconnect`; interop `checkLNSPolicyOutputForMRU`; topology `checkAccessReconnect`, `checkNegotiated`, `checkRenegotiated`; fixtures `clearL2TPSession`, `l2tpSessionDetailShow09`, `l2tpHistoryShow09`, `plugin16TeardownSession`; offline token forwarding |
| Browser and non-Go fixtures | `l2tp_list.templ` links and `l2tp_detail.templ` form encode SID; generated files follow templates. Login-addressed `l2tp-chart.js` sample streams remain login-addressed. `.ci` literal assertions migrate with their actual Go command producer. No direct shell/Python lifecycle API caller was found in this bounded search |
| External accounting | Packet builder through RADIUS client/UDP, full-ID CoA match, FreeRADIUS detail reader in `scale_evidence.go`, Disconnect writer in `scale_observe.go` and embedded mock/radclient fixtures. Preserve the existing accounting-ID form, same ID across Start/Stop and terminate cause |

### Risks and assumptions raised by this pass

| ID | Assumption / risk | Basis or early signal | Validation / mitigation |
|----|-------------------|-----------------------|-------------------------|
| A-2 refresh | Subsystem-unique TID assumption is false | Constructor and allocator use listener-local state | Regress equal TID/SID across two listener owners as well as across two tunnels |
| A-3 refresh | RADIUS-only slow-reader assumption is false | Shaper and CoS perform synchronous backend work | Bound the accounting claim precisely; preserve synchronous assignment acknowledgements and backend ownership |
| A-4 | Old queued PPP events can reach a successor | Tuple-only lookup, immediate SID release and asynchronous PPP event production | Deterministic old-event/remove/reuse/consume regression, with successor's own positive events accepted |
| A-5 | Coordinated owner contracts will carry immutable listener scope and logical birth | Active owner accepted the scope gap; final API has not landed | Read the final producer and carrier before fixing signatures; no fallback to tuple reconstruction |
| R-2 | A SID or tuple survives in a sibling consumer | Shared-ID tests pass for routes but history, pool, CoS or disconnect reaches another session | Account for every census writer/reader; assert unaffected subscribers keep state and traffic |
| R-3 | Asynchronous Stop overtakes Start or reads reused interface counters | Wire order changes or counters come from another lifetime | Serial accounting ownership and terminal snapshot before cleanup, tested at the real send boundary |
| R-4 | Teardown relookup targets a successor | Target changes between snapshot and action | Capture exact identity/control once and retain it through completion; stale target cannot select its replacement |
| R-5 | Unpublished sibling work is mistaken for a completed dependency | Only a private source packet or intent is available | Record landed carriers and actual scoped/integration proof; retain this spec open until its own goals are demonstrated |

### Approaches to carry into design

| Approach | Existing mechanism used | Trade-off |
|----------|-------------------------|-----------|
| Complete admission identity and exact lifetime checks | Extend canonical SubscriberID, original-owner result checks, per-tunnel notice order and the existing accounting task/terminal owner | Recommended: one carried identity works across listeners and ID reuse. It requires deliberate migration of the whole consumer/CLI census |
| Subsystem-wide numeric allocation and reuse quarantine | Extend numeric allocation, per-driver duplicate guards and notice ordering, adding a proved producer-drain boundary before reuse | Avoids changing some payload fields, but adds cross-reactor allocation and reuse exhaustion/drain obligations. A tuple-only map change without these guarantees is insufficient |

For accounting, extend the existing task with ordered terminal ownership.
A shared bounded accounting queue adds head-of-line blocking and a full-queue
decision; blocking enqueue recreates the stall and dropping records fails the
goal. A goroutine per Stop is not an alternative: it loses order and ownership.

Research compared accel-ppp's `session.c` `generate_sessionid` and `radius/acct.c`
`rad_acct_stop` / `rad_acct_stop_defer`: its logical session ID is separate from
the wire tuple, and deferred terminal requests retain request state. These are
design comparisons, not evidence about Ze or a claim that its policy is Ze's.
Sources: https://github.com/accel-ppp/accel-ppp/blob/master/accel-pppd/session.c
and https://github.com/accel-ppp/accel-ppp/blob/master/accel-pppd/radius/acct.c,
read 2026-11-16. A final contract decision must use the coordinated Ze mechanism.

## Proposed design direction: approval pending

This proposal extends the canonical admission identity and exact-owner work
already being developed for PPP. The API spelling must be bound to that
producer before this spec is ready; it is not permission to invent a second
attempt/control mechanism or reconstruct an identity at a consumer.

### Identity and operator contract

| Surface | Proposed contract |
|---------|-------------------|
| Logical session admission | Mint the canonical subscriber ID once, before PPP admission. It distinguishes listener ownership and logical session birth even when TID/SID are identical or reused. PPP treats it as opaque |
| Late work | Retain the original admission's attempt/control. The consumer checks that original owner before mutation or publication; authentic events from the current admission still progress normally |
| Numeric L2TP IDs | Keep local/remote TID/SID as protocol and display coordinates. They are not global or lifetime selectors |
| Session lists and details | Expose the complete subscriber ID alongside numeric coordinates. Show, event history, FSM history, traffic and disconnect audit select the same exact admission |
| Single-session CLI | Revised by owner approval: preserve numeric SID and add optional `tunnel <tid>` and `listener <configured-name>` qualifiers. Resolve all matching candidates once; accept exactly one captured admission and refuse zero or multiple matches before side effects. Preserve explicit `all`, reason and cause semantics |
| Web | Session links, detail lookup and disconnect form carry that same complete ID through the existing command dispatcher and authorization. Login-addressed CQM samples keep their existing login contract |
| CoA and timers | Carry the exact snapshot/control matched by the selector or captured by the timer through termination completion. Preserve complete issued Acct-Session-Id comparison, multi-match RFC 5176 semantics and existing error-cause distinctions |
| Observer keys | Route records, framed-route ownership, CQM event rings, release, snapshots and readers use the carried complete ID. A teardown for one admission cannot remove another admission's state |
| Other lifecycle consumers | Bridge, accounting, shaper, CoS, CoA health and metadata use the same carried identity. Coordinate their migration with their active owner; no missing reader is waived because this session owns only part of the implementation |

### Accounting and notification contract

1. Preserve synchronous local publication and IP-assignment acknowledgement.
   Before cleanup can remove/reuse the interface, capture terminal counters,
   identity, terminate cause and the accounting client/endpoint owner.
2. Transfer terminal accounting work to the existing accounting task/owner and
   return from the publishing callback without waiting for a remote RADIUS
   server or joining an in-flight exchange there.
3. The accounting owner serializes actual Start and Interim operations before
   Stop, uses the same complete Acct-Session-Id, and produces one terminal
   record with its bounded retry/failure reporting. Reload retirement and
   shutdown retain that owner through completion or an explicit unresolved
   outcome; cancellation alone is not completion.
4. Preserve per-tunnel notice ordering and checked route withdrawal/family-down
   completion receipts. Deliver already-owed session downs before tunnel
   callbacks that can re-enter teardown. No new goroutine per Stop or generic
   asynchronous EventBus is introduced.
5. Retain the original peer-index key through deletion and enqueue the kernel
   setup events collected by a successful tick retry. These owned rows remain
   acceptance obligations, coordinated with the active actor implementation.

### Proposed acceptance and wiring proof

These rows refine the draft ACs above; none is a current passing claim.

| AC / entry point | Positive observable outcome | Permitted survivors and reason | Proposed test |
|------------------|-----------------------------|-------------------------------|---------------|
| AC-1, queued PPP lifecycle event after numeric ID reuse | The replacement negotiates and publishes its own state; the old event cannot alter it or publish as it | Current admission's own LCP, assignment, up, echo and down events remain effective because they name its owner | `TestLatePPPEventIsRefusedOnAReusedSessionID` with current-owner positive control |
| AC-2, equal SID across tunnels and equal TID/SID across listeners | Both subscribers retain independent assigned/framed route records and event history; ending one withdraws only its records | Every other admission retains its own distinct prefixes and history because its complete ID differs | `TestRouteObserverKeepsSameSessionIDOnTwoTunnelsApart`, `TestRouteObserverKeepsSameTupleOnTwoListenersApart`, `TestObserverKeepsSameTupleOnTwoListenersApart` |
| AC-3, real command dispatch, CoA or timeout | The captured admission ends with its CDN and terminal accounting; a stale target reports absence/failure rather than ending its replacement | All non-target admissions continue, including those sharing numeric coordinates | `TestSessionCommandTargetsCanonicalAdmission`, `TestDisconnectKeepsMatchedAdmission`, `TestSessionTimersCannotEndReplacement` |
| AC-4, unanswered Accounting-Stop while another tunnel sends HELLO | The real reactor answers the healthy tunnel before its peer's existing retransmission deadline; terminal accounting still attempts the ended admission's Stop | Healthy tunnels and accounting tasks continue; an unacknowledged record may finish with an explicit bounded failure because the remote server is unavailable | `TestSlowAccountingStopDoesNotBlockOtherTunnelControl`; functional `test/l2tp/slow-accounting-control.ci` |
| AC-5, changed peer source then discard | Discard removes the index entry inserted for that exact tunnel, allowing a fresh establishment | Other live peer-index entries remain because their owners differ | `TestDiscardTunnelUsesOriginalPeerIndexKey` |
| Consumed setup retry | After a failed packet-path socket lookup, a successful tick retry reaches kernel setup and PPP handoff | Existing established sessions keep their owners; no duplicate setup is introduced | `TestTickEnqueuesRecoveredKernelSetup` |
| Tunnel callback re-entry | A tunnel callback can end another session through the real teardown path and return, with each owed down delivered once | Later valid notices continue in ticket order; no skipped publication is accepted as success | `TestTunnelCallbackRunsAfterOwedSessionDowns` |
| Accounting wire order and exact terminal snapshot | Actual sends order Start/Interim before Stop and use the ended admission's counters, ID and cause, including immediate teardown and shutdown | Other accounting owners remain independent; configured retry/failure bounds remain permitted | `TestAccountingStopFollowsActualStart`, `TestAccountingTerminalSnapshotSurvivesInterfaceReuse`, plus the existing FreeRADIUS evidence path |
| Whole subscriber goal | The unchanged many-session scenario completes all 48 subscribers, FRR route census, selected Disconnect, 47 survivors, subsequent peer CDN and 46 survivors, matching accounting and traffic assertions | The explicitly surviving subscribers keep routes and traffic; weakening a failed assertion is not permitted | Existing `l2tp-many-sessions-kl2tpd` against pinned kl2tpd v0.1.8 and pppd 2.5.5 |

### Implementation boundary and approval concern

| Phase | Work and dependency |
|-------|---------------------|
| Contract binding | Read the final coordinated admission identity, attempt/control, exact teardown and terminal receipt producers. Bind the full writer/reader census and native CLI schema to those carriers. Resolve any uncovered consumer before ready |
| Wiring first | Add discriminating tests through session commands/events before implementing observer and reader migration; retain all existing assertions and obtain independent judgment for changed old fixtures |
| Observer and CLI migration | Implement full-ID route/CQM storage and all reader/release paths; migrate actual command writers, YANG, templates and fixtures together. Preserve the active owner's checked withdrawal receipts |
| Actor/accounting integration | Consume the coordinated exact-owner and ordered terminal changes, including the three additional journal rows; prove this spec's outcomes through its real entry points rather than count another session's source packet as completion |
| End-to-end and review | Run scoped reached tests, the strict guest scale scenario through its survivor/accounting tail and independent review. Keep the spec open until every goal has evidence |

The strongest concern is a partial identity migration: the primary route map
can pass while a sibling command, event reader or metadata lookup still rebuilds
the old tuple and reaches another subscriber. The census and colliding-listener
tests are therefore admission requirements, not a closing documentation task.

| Challenge | Answer |
|-----------|--------|
| Simplicity | Extend the canonical identity and existing accounting owner. Numeric quarantine needs a new global allocator plus a producer-drain proof, while a second terminal worker adds ownership to explain |
| Uniformity | Carry one opaque admission identity unchanged across existing registries, the EventBus and command dispatcher. Preserve the PPP leaf and the existing per-tunnel order ticket |
| Performance | Mint/store identity once per admission and use it as a map key; do not format it per packet/event. Retain existing bounded CQM pools and accounting tasks; add no packet-buffer copy or goroutine per event |

### Coordinated producer contract received after the research pass

The active producer owner supplied a frozen source proposal, not an applied or
tested implementation. It fits the proposed direction without encoding a
listener's mutable IP address or requiring a global tunnel-ID allocator.

| Contract | Required integration |
|----------|----------------------|
| Logical birth | `newL2TPSubscriberID` uses the existing cryptographic-birth pattern; `L2TPSession.SubscriberID` reads the stored immutable value. The ICRQ, OCRQ and shared initiator constructor mint it once before adding the session or starting shared work |
| Stability | One logical session keeps its ID through LCP restart, family changes and authentication rounds; a replacement with identical numeric labels receives a different ID. Existing network-lifetime/attempt fences remain separate and still reject stale work within that logical session |
| Event identity | SessionUp, IPAssigned, Down, rate-change and CoS-change payloads carry the same explicit SubscriberID. A missing ID refuses shared mutation; no fallback rebuilds it from TID/SID |
| Exact authority | Kernel admission captures and rechecks the original row/receipt before passing the same ID into the PPP attempt. The public string is an identifier, not authorization or proof of physical cleanup; pending receipts and captured control still decide those effects |
| Observer and metadata | Route/CQM records and metadata lookup use the carried ID. Checked withdrawal and partial family-down retain their exact record/terminal receipt semantics |
| Operator representation | Snapshot JSON `subscriber-id` carries the opaque identity, alongside listener and numeric coordinates. The approved CLI uses readable numeric qualifiers; internal/web exact-ID targeting never requires an operator to type the opaque token |

Source proposal `tmp/session/2026-11-15-125434/scratch/l2tp-session-identity-source.go.txt`
has SHA256 `0b991cba804ed0a81f527d18e504cbf41919059f0b2cf45c66b26583dd4730e3`;
its sibling `l2tp-session-identity-contract.md` has SHA256
`a483f0cdaaa0062f42171d3dbd9ba1bd0de3a56623fb424ac5c5919143173713`.
Read and matched here; retain under its producer's owning record until reviewed
implementation consumes it. Final admission still requires the actual applied
producer and all census reader dispositions, followed by real proof.

## Approved readable selection and implementation admission

Owner approval on 2026-11-16 follows the explanation separating internal unique
identity from operator syntax. The owner requested a working, tested lab.
This section and the revised Integration/Documentation tables are the current
design. Earlier empty skeleton examples and the opaque-ID CLI proposal are
historical, not implementation instructions.

### Selector rules

| Input | Result |
|-------|--------|
| SID identifies exactly one admission | Capture it and serve the read/action through its immutable identity/control |
| SID occurs in more than one tunnel | Refuse with candidate listener/tunnel coordinates and the exact qualifying command; produce no audit, metadata or teardown effect |
| SID plus TID is unique | Capture that admission; a listener qualifier remains optional |
| Equal SID/TID on different listeners | Require the configured listener name; no map-order or first-reactor choice |
| Unknown or invalid selector | Actionable refusal; no fallback to another listener/tunnel, no all-effect |
| Captured admission ends or labels are reused | A mutation cannot select the replacement; exact original completion or explicit stale-target failure |
| Read/history/traffic/audit | The same resolution supplies every reader, not an independent SID lookup after the action target was chosen |
| Browser list/detail/disconnect | Display readable coordinates, carry full immutable ID and captured page target internally; an old page cannot clear a same-label successor |

Resolution must use a consistent owning snapshot/capture seam. A sequence of
unrelated first-match lookups is insufficient. Reuse the coordinated captured
target mechanism; do not add a competing teardown owner or hold observer locks
while a callback/remote operation runs.

### Completed design data flow

| Boundary | Writer → reader | Change / preservation |
|----------|-----------------|-----------------------|
| Logical allocation → PPP | ICRQ/OCRQ/initiator constructors → exact captured kernel admission and PPP attempt | Mint once; full ID and original control/receipt remain attached |
| PPP → transport | All lifecycle, rejected, auth and address events → owning reactor/PPPoE consumer | Check exact attempt/control, including family/network-lifetime fences; preserve other transport's admitted events |
| Transport → lifecycle readers | Reactor/teardown → bridge, observer, accounting, shaper, CoS, CoA health | Explicit nonempty SubscriberID, no pair reconstruction; preserve synchronous local completion and notice ordering |
| Event → routes | IPAssigned/Down/family-down → checked route record and replay | Full-ID map, same metadata key, exact terminal receipt; surviving records retain their prefixes |
| CLI/web → session | Registered command/schema or dispatcher adapter → common resolver → captured owner | Readable CLI qualifiers, exact internal/web birth, no relookup by reusable coordinates |
| CoA/timer → termination | Full supplied selectors or original timer owner → captured exact terminal path | Preserve matching semantics, cause, CDN/Stop and non-target survivors |
| Accounting → RADIUS | Terminal snapshot → existing ordered accounting task → RADIUS client/UDP | Same issued accounting ID, actual Start/Interim before Stop, no remote wait on reactor callback |

### Census additions and dispositions

| Site | Required disposition |
|------|----------------------|
| `reactor_kernel.go` `handleEchoRTT` → `events.EchoRTTPayload` → `observer.go` `wireObserverSubscriptions` | Carry the original session's SubscriberID on Echo too. Reject stale original controls before emission; route the current owner's echo into its independent CQM event ring, preserving login sample history |
| `internal/plugins/cos/enricher.go` `enrichSubscriberDetail`, `enrichSubscriberBrief`, `extractSessionKey` | Use the exact subscriber ID in the response/session; never reselect pair-keyed CoS state |
| `authradius/coa_transaction.go` `confirmCoA`, `callCoAOwner` → JSON `coaOwnerRequest` → CoS `session_transaction.go` `executeSessionTransaction`, `sessionTransactionRequest`, `prepareSessionChange` | Add the explicit canonical ID to prepare's writer and decoder together; preserve operation/token rules, authenticated source, apply/rollback/commit and completed receipt semantics. Numeric labels do not select state |
| Pool `PrefixRequest` / releaser and `internal/test/plugins/fakel2tp/pool.go` `poolPrefix` | Migrate the registered handler/releaser and the actual synthetic writer to an explicit canonical subscriber ID, preserving allocation/release/prefix results and the real handler test. The bounded production census found no active non-fixture request or releaser caller; this is not an excuse to omit the reachable synthetic API |
| Exact timeout teardown already applied locally | Audit `teardownCapturedSession` and prove original timer ownership instead of replacing that mechanism; current applied code is not a passing runtime claim |
| Actual numeric command writers in existing interop/fixtures | Existing single-session unique cases remain valid. Add qualifiers wherever collisions are intentional; run their reached suites rather than remove old assertions |

### Discovery and verification

| Question | Answer |
|----------|--------|
| Where an agent starts | `ai/INDEX.md` subscriber/L2TP rows → subscriber model/digest and L2TP interop lab page |
| Rule preventing regression | Existing exact-owner/lifetime and test-preservation rules; captured target and explicit canonical-ID regression tests |
| Single inventory | Existing session/command/event registries, configured listener inventory and one admission identity; no duplicate numeric alias registry |
| Deciding verification | Focused red/green identity and selector tests, reached package/functional suites, unchanged strict 48-session interop through all survivor/accounting checks |

### Concrete wiring and phase plan

| Phase | Required output | Main-thread verification |
|-------|-----------------|--------------------------|
| 1: wiring and audit | Audit current source, add new colliding route/CQM and selector entry-point regressions before fixes; preserve old assertions | Named new tests must show the behavior red on current producer, not only a compile failure |
| 2: canonical observer integration | Consume coordinated birth/event producer; full-ID route/CQM records, reads, release, metadata and checked withdrawal/family-down | Root L2TP and observer tests, exact event-to-route wiring, replay and old tests |
| 3: readable command/web selection | Schema/completion/common resolver, scoped reads/clear/history/traffic/audit and exact web target | L2TP command and web package tests, functional `session-selector-scope`, current clear fixture and actual interop command readers |
| 4: remaining consumer and terminal integration | CoS handlers/enrichers/transaction migration; integrate coordinated accounting/teardown/timer/shaper and tick/index/reentry fixes | CoS and authradius package tests; real slow-accounting control proof plus ownership/counter/ordering cases |
| 5: strict lab and review | Complete scale scenario with real pinned peers and all disconnect, survivor, route, traffic and accounting assertions; independent review | Native guest lab, scoped lint/docs/RFC records actually reached; evidence per stated goal, then closure |

Use one editing agent per bounded phase, main-thread tests and independent
review. Shared producer edits stay with their active owner; require their
minimal release-fix chunks and actual integration, not completion of the whole
post-release VPP feature. Draft functional tests in the incubator and promote
only after proof. Lab/VM scratch uses task-owned TMPDIR on the cache disk.

Failure policy: no queue or retry grows without a bound; empty IDs refuse
mutation; stale work retains its original owner and reports the actual outcome;
missing/ambiguous selectors cause no side effects. Route observer locks never
span callbacks. Preserve one accounting terminal owner under reload/shutdown.
No implementation is complete until the unchanged lab reaches its final tail.

### Admission review corrections

Independent current-tree audit found the Echo, synthetic prefix and exact CoS
JSON sites omitted from the first final census. The rows above now give each
an explicit migration disposition. No test or feature was dropped to answer the
findings. The audit also confirms that timer and PPP control fences are partly
applied locally while observer and accounting goals remain unverified.

The immutable canonical identity's JSON spelling is a wire-boundary value.
New internal record keys must use a shared validated comparable numeric/packed
identity, parsing once at that boundary per `ai/rules/go-standards.md` and
`docs/contributing/go-conventions.md`. Keep the canonical birth's exact equality;
do not hash to a narrower key, build a new interning manager or reconstruct a
tuple. Bind the representation to the coordinated producer before phase 2.

Independent admission re-review accepted phase 1's current-API wiring and
collision regressions after the three census corrections. Phase 2's production
migration remains unjudgeable until the shared packed identity representation
is bound to the producer. This admits test authoring and the observed red run,
not a production migration or a claim that all design/implementation is done.

### Packed identity binding for phase 2

The coordinated successor contract is `subscriberid.L2TPBirth`, a comparable pair
of private uint64 words with invalid zero. Its constructor preserves a new
nonzero 128-bit cryptographic birth. Parse/MarshalText/UnmarshalText define the
wire DTO `l2tp-` plus 32 lowercase hexadecimal digits; an invalid decode leaves
the previous value untouched. Text conversion is at a wire/human boundary,
never an internal key or repeated parse on the event path.

The value lives in the pure `internal/core/subscriberid` leaf. Placing it in
the existing L2TP subscriber package would introduce the import cycle
events → subscriber → ppp → events: the subscriber handler registry imports
PPP, and PPP's event types import the L2TP event package. The invalid placement
remained private and unapplied. Move the value directly, without an alias or
parallel implementation, and refresh every proposed producer and consumer
import before coordinated publication. Its representation and behavior stay
the same; this corrects dependency direction.

Actor getters supply the stored Birth and cached SubscriberID to the existing
PPP attempt. Lifecycle Up/IPAssigned/Down/Echo payloads carry typed `Birth`;
the shared subscriber snapshot carries optional `L2TPBirth`. PPPoE snapshots
must still serialize successfully with no L2TP birth. Rate/CoS request writers
and decoders migrate together at their boundary.

| Interface | Frozen contract |
|-----------|-----------------|
| Route IP-up | Typed Birth first, then diagnostic TID/SID, username, address and the original actor-captured immutable framed-route slice |
| Route down | Typed Birth first, then diagnostic TID/SID |
| Checked family down | Typed Birth first, diagnostic TID/SID and address family; return the actual completion error |
| Checked terminal withdrawal | Typed Birth first and diagnostic TID/SID; preserve original-record completion receipt and error. Terminal code uses this checked route, with no void fallback |
| Session capture | Existing subsystem resolves readable SID plus optional TID/listener to one captured original reactor/tunnel/session/Birth; zero/multiple matches refuse without effects |

Source proposal SHA256 `1a0767a59a4cb7e1fc02c46013d0b66a4a5a1303f72fbfdf5cd941444c033a7f`
and contract `87a71cfb564e71e0c544cc0741bcb2a93fda0afddc30a0cda19a1cdde3d5f5b0`
were read and matched in the producer's owning scratch. This supersedes only
the string-key representation, with the approved user behavior unchanged.
The source is still private/unapplied; actual constructor/callsite wiring and
cross-listener/lifetime proof remain implementation obligations.

The independent packed-binding review accepts the value/equality/wire
representation and permits CQM/record-key authoring. One route input must also
be bound before integration: the actor captures the original framed-route
metadata using the existing cached ID/metadata owner, then supplies immutable
framed-route input to the observer. The observer must not format Birth to
reconstruct a string metadata key per event. Freeze this added input with the
actor owner and preserve the existing once-per-network framed-route behavior.

The final framed input is explicit: IP-up receives typed Birth, diagnostic
TID/SID, username, address and the captured framed-route slice. The actor owns
the cached-ID/metadata lookup before unlock; the observer copies the immutable
value into its record. A genuine actor capture regression is required before
the four existing framed-route tests may migrate to this split responsibility.

### Readable CLI publication details

| Surface | Bound behavior |
|---------|----------------|
| Qualifier spelling | Numeric `id`, optional `tunnel-id` and `listener` follow the existing command/filter convention. Qualifiers do not change the numeric IDs into global identity |
| Generated action precondition | Optional `expected-id` validates the complete canonical DTO once and supplies the typed Birth filter. Browser/script requests carry it automatically so a stale page cannot clear a same-label successor; interactive users need not type it |
| Read-only aggregate views | Preserve existing bare observer/traffic aggregate queries. The explicit-all requirement remains mandatory for mutations; supplied qualifiers must not be silently ignored when SID is absent |
| Runtime completion | Owning command module registers its operational argument completer beside the handler. The shared completion RPC passes completed arguments after its existing caller authorization check; the owner offers actual listener names only at the listener-value position. Handler resolution still rejects unknown or ambiguous names |
| Listener inventory | Snapshot listener name and endpoint come from the original reactor's cached configuration/bound endpoint, including retained closing roots; never reconstruct a target from newly reloaded configuration |
| Session diagnostics | Snapshot carries typed Birth, cached SubscriberID, original Listener endpoint and a hidden immutable observation generation. Candidate messages may use a fresh read-only snapshot, but action admission uses the captured target |
| CQM JSON | Raw event JSON emits `subscriber-id`; summary JSON also emits `tunnel-id`. Explicit CLI row maps include these fields rather than rely on Go field names |
| Traffic preservation | Existing eight counters include byte/packet totals and rx/tx errors/drops. The captured producer must preserve all eight via its exact owner, or explicitly report unavailable data. Four counter fields cannot silently stand in for eight, and no current-interface-name relookup or fabricated zeros is allowed |

The CaptureSession boundary returns an interface implemented by the private
real actor capture, preserving honest cmd/web test doubles without exposing a
fake-authority constructor. Its Teardown receives a typed termination cause
and separate free-text reason. Selection must resolve against one consistent
snapshot across active and retained reactors before any action, never unrelated
per-reactor first-match observations. The private production implementation
retains and rechecks the original row, control and Birth on every operation.

### Retained observer ownership across restart

The event bus copies subscribed callbacks before invoking them. Unsubscribing
does not revoke a callback already copied by an in-flight publication. Old and
new native generations can also remain present together while cleanup retries.
Birth separates their sessions, but does not by itself stop both CQM observers
from processing the same event and changing login quality state twice.

| Boundary | Required behavior |
|----------|-------------------|
| Native generation | Mint one immutable, comparable, nonzero observation generation for the original native roots. This process-local observation value is separate from logical subscriber Birth and grants no actor mutation authority |
| Lifecycle event | Original Up, IP-assigned, Down and Echo payloads retain that generation, including delayed terminal publication. Never substitute the current subsystem generation |
| Observer delivery | Under the observer's existing mutex, check valid matching generation and admission state before any ring, login or metric effect. A stopped observer rejects already-copied callbacks too |
| Retained observer | Seal unknown lifecycle admission while keeping its known original rings available for terminal updates and history. Fresh configuration gets fresh bounded pools; no pool-resizing migration or extra registry is needed |
| History reader | Collect active and retained reactor roots under Subsystem.mu and acquire their row locks before releasing it. Capture the exact original row, observer and generation consistently; read that capture's exact Birth outside the subsystem lock without allocating a ring. Fresh reads discover retained roots, and closed observers preserve readonly snapshots |
| Disconnect history | Route the captured snapshot's generation and Birth to that original observer. An explicit event for an already-captured negotiating row may allocate its original retained ring; it never falls back to the current observer. Existing optional bounded CQM behavior is preserved, separately from command audit |
| Proof | Controlled overlapping old/new publications with the same login must affect each original observer once; an old unknown birth cannot enter a retained observer, and a callback paused before Stop cannot mutate a closed observer |
