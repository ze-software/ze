# Spec: l2tp-session-identity-and-publish-stall

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped-daemon defects an operator meets on an LNS with more than one tunnel: a subscriber's route is withdrawn or its session is cleared because a session on another tunnel holds the same session id, and every tunnel stops answering while one RADIUS server is slow. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-single-owner-lifecycle-5-session-end-locals.md` (it edits the same L2TP files and adds the per-tunnel order ticket this spec's design must fit; the order of the two is settled in design) |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Four L2TP defects met while `spec-single-owner-lifecycle-5-session-end-locals`
was written. They are not event-order defects, so they left that spec. Each has
a journal row (Owned journal rows). Nothing here is designed yet: this skeleton
records what the code does and the questions the design must answer.

| Part | What the operator sees today | Goal |
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
- [ ] `plan/release/fixes/spec-single-owner-lifecycle-5-session-end-locals.md` - "The order ticket", R-1, AC-18
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
| The per-tunnel order ticket | `plan/release/fixes/spec-single-owner-lifecycle-5-session-end-locals.md`, "The order ticket" (not in the code yet) | Will order the notifications of one tunnel without holding a lock while a reader runs | Extends: whatever moves the Stop wait keeps that order, and a quarantine or an identity check is taken under the same `tunnelsMu` |

## Open design questions

Stated without an answer. The design pass reads RFC 2661 and the FRR, BIRD or
accel-ppp source where it applies, and chooses.

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
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1: for example "Wire parsing in internal/component/bgp/message/"]
2. [Stage 2: ...]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | No |

### Changed-Wire Census

**Search evidence:** [scope, terms, positive control and source findings]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding and actual encode/write or decode/read path] | [change or preserve, with reason and proof] |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A PPP event queued for an ended session can reach a session that reused its id | Read 2026-11-11: the events carry two ids only, the handlers look up by them, the id is freed before the PPP session ends, and the channel holds 64 events. Not reproduced | Part 1 is not reachable and its row closes with the evidence | A unit test that queues the event, reuses the id and reads the new session | unvalidated |
| A-2 | A tunnel id is unique across the reactors of one subsystem | Not read | The pair is ambiguous and question 5 decides the identity | Read `allocateLocalTID` and how a subsystem builds its reactors | unvalidated |
| A-3 | The Accounting-Stop is the only reader work on the publishing goroutine that waits on a remote party | Read 2026-11-11 for `acct.go`. The shaper and CoS call a traffic backend there, which was not read for how long it can wait | Another reader stalls the reactor the same way | Read each reader of the three session events for a call that waits | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / routes mis-encoded / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | `plan/release/fixes/spec-single-owner-lifecycle-5-session-end-locals.md` edits `reactor_kernel.go`, `teardown.go`, `reactor.go` and `session.go`. `spec-rfc5176-remaining-defect-classes` edits the CoA path |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

Draft rows. The first two came from
`spec-single-owner-lifecycle-5-session-end-locals`, where they were AC-19 and
AC-20. The design pass confirms or rewrites each one.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The PPP driver has a queued event for a session that ended, and a new session on the same tunnel has drawn the same session id | The event changes nothing on the new session and publishes nothing: its address, user name, negotiated values and lifetime are those its own PPP session reports. One debug line names the refused event | Events of the new session's own PPP session |
| AC-2 | Two sessions on two tunnels hold the same session id | The route observer and the CQM observer keep one record for each: an address assigned to one leaves the other's route injected, and the end of one leaves the other's record and route in place | None: the two are different subscribers |
| AC-3 | Two sessions on two tunnels hold the same session id, and the operator clears one, or a Disconnect-Request names one, or one reaches its session timeout or idle timeout | The session that was named ends, with its CDN and its Accounting-Stop, and the other stays established and keeps its route | None: the two are different subscribers. What the operator types is question 4 |
| AC-4 | A RADIUS accounting server answers nothing, and a session on one tunnel ends | The reactor keeps handling the control packets of every tunnel: a hello on another tunnel is answered inside its retransmission interval, and no healthy tunnel is torn down. The Stop for the ended session is still sent, and retried as the design states | A Stop that no server acknowledged before its bound: the log names it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [for example "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
The first three names came from
`spec-single-owner-lifecycle-5-session-end-locals` with AC-19 and AC-20.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLatePPPEventIsRefusedOnAReusedSessionID` | `internal/component/l2tp/` (file named in design) | AC-1 | |
| `TestRouteObserverKeepsSameSessionIDOnTwoTunnelsApart` | `internal/component/l2tp/route_observer_test.go` | AC-2 at the route observer | |
| `TestObserverKeepsSameSessionIDOnTwoTunnelsApart` | `internal/component/l2tp/observer_test.go` | AC-2 at the CQM observer | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| [named in design] | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

## Files to Modify
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Doctor code per problem class | | For each problem class the feature can produce (refusal, `Warn`/`Error` log line, transient report-bus event): the doctor check, its code and its phase (`ai/rules/user-facing-errors.md`, `docs/architecture/doctor-and-health-checks.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | `features/<id>.md` (renders `docs/features.md`); `./le feature report` shows its level and the gap to the next |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED, do not answer from memory: `./le spec citation anchors spec plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` lists them |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific] |
| Naming | [feature-specific] |
| Data flow | [feature-specific] |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line and raise names the code the Integration Checklist's doctor row lists, and each doctor check carries `Fix` at the phase that holds its state (`ai/rules/user-facing-errors.md`) |
| Rule: [relevant rule] | [what to check] |

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

## Related specs

| Spec | Relation |
|------|----------|
| `plan/release/fixes/spec-single-owner-lifecycle-5-session-end-locals.md` | Found these defects and held AC-19 and AC-20 until 2026-11-11. It keeps the L2TP event order (the per-tunnel order ticket) and lists the four rows above as considered and not owned |
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
