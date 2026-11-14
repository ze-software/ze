# Spec: single-owner-lifecycle

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | Umbrella of five release fixes for lifecycle defects an operator meets in the shipped daemon: a plugin that exits and stays down, a goroutine of an ended BGP session writing on the next session's connection, a forged datagram ending an IKE handshake Ze initiated. It holds the owner's decisions and the routing of every journal row, and changes no file itself. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Set: this file is the umbrella of `spec-single-owner-lifecycle-<N>-<name>.md`,
children 1 to 5, all in `plan/release/fixes/`. The children carry every
acceptance criterion, test and file edit. The umbrella carries the decision
record and the routing of every journal row. Its stem keeps the name the
children and three other specs already cite, so it does not carry the
`-0-umbrella` suffix of `docs/contributing/spec-workflow.md`, "Spec sets".

Status: that page prescribes no status of its own for an umbrella. `ready`
is the vocabulary's value for a complete design, which is true of the set:
the owner's decisions are made and he approved the five children at their
final gate on 2026-11-11, so each reads `ready`. The umbrella holds no
implementation work of its own, so it stays `ready` while the children are
implemented and is never claimed to start work.

Family S3 of the journal backlog analysis (2026-10-23), part 1 of 3. The other
two parts are named under "Sibling specs".

## Task

**The original diagnosis (2026-10-23).** Journal rows from the plugin server,
the BGP reactor, the BGP plugins, LDP, L2TP and IKE were gathered under one
reading: a lifecycle in Ze has no single owner, so registration, teardown and
state changes are performed by whichever function holds the object. The spec
proposed one owner object per lifecycle that hands out a generation, and a new
repository check that reads an ownership annotation and refuses a state write
or a registration made outside the owner.

**What re-verification showed (2026-11-11).** Every row was read again at the
producing code. The diagnosis does not hold as written: every lifecycle here
already has an identity, and each defect is one specific path that goes round
it.

| Lifecycle | The identity and owner that exist | What the defects are |
|-----------|-----------------------------------|----------------------|
| A plugin process | One supervising goroutine for the whole life of the process, a cleanup that runs once per process, and the process object itself as the identity (a comparison of pointers answers "is this still the live process of that name") | Paths that release less than the cleanup's caller needs, publish a replacement before its handshake, or call a handler after it unsubscribed |
| A BGP session | The session object, and a per-peer generation issued by one clock that fences delayed forwarded UPDATEs by their source; every per-session goroutine but one is joined when the session ends | The one unjoined goroutine, a backoff that restarts on the wrong event, a listener opened on the wrong condition |
| A received route generation in the Adj-RIB-In | The reactor's message id, never zero for a received UPDATE | A decision that carries no message id passes the only fence |
| One peer-down across `bgp-gr` and `bgp-rib` | Ordered, acknowledged peer-state delivery | A failed or late delivery is skipped with a warning |
| An IKE SA handshake | The integrity check: a message that fails it proves nothing | The initiator acts on the failure where the responder already drops it |
| An LDP session, an L2TP session, a DDoS FlowSpec announcement | The session's own "reached operational" fact, the tunnel lock's order, the plugin's own record of what it announced | Code that reads state a teardown already reset, publishes after the lock is released, or leaves without withdrawing |

Two statements of the original text were false on the day of the design pass
and are withdrawn: "No generation is carried" for a BGP session (the reactor
carries one), and "a plugin that crashed once is refused on respawn" (fixed by
ccc3f86b04 for a restarted plugin; what remains is in child 1).

**The goal now.** Each defect is fixed where its bypass happens, shaped so the
bypass cannot be written again: state moves onto the object whose lifetime it
has, or one function owns the unsafe case. No owner object, no new generation
counter and no annotation check are built. Five children do the work; this
file records why, and where every row went.

### Owner decisions (2026-11-11)

The owner's words are quoted as he gave them in the design conversation of
2026-11-11. Each child cites this section instead of restating it.

| # | The owner's words | What it settles |
|---|-------------------|-----------------|
| D1 | "D1 - yes you are right" | No annotation gate and no typed analyzer. Each fix closes its bypass at a choke point. Where a single writer must be enforced by the compiler, the state lives in a type with unexported fields in a package of its own, so the constructor or the transition method is the only writer |
| D2 | "D2 - BIRD is better" | The BGP reconnect delay follows BIRD's damping intent, not FRR's reset on reaching Established |
| D3 | "D3 - Ok add it to the post-release spec" | Routing every IKE SA state write through one transition function leaves this release fix. It is the later-phase item "IKE SA state, one transition" of `plan/post-release/engineering/spec-validated-construction-and-state-types.md` (its AC-26 to AC-28) |
| D4 | "fix the issues you identified during this work too, use an agent for each" | The defects found during the design pass were fixed or given an owning spec the same day. They are listed under "Defects found during this work" |
| Final gate | "Approve all five, the refined D2 rule stands" | The five children are approved and move to `ready`, and the refined D2 rule below is the rule |

**D1, the reason recorded.** A check for a write to an annotated field has to
know the type of the value written through. No gate under `internal/le/arch`
runs the Go type checker: three of them import `go/types`, and only to print
an expression (`types.ExprString`); a search of that directory's non-test
files for `types.Config`, a `Check` call or an importer finds none, while the
same search finds `go/ast` in nine of the gate packages. A syntactic check
cannot see a write through a value whose type the file does not declare, so it
would pass exactly the writes it exists to refuse. The IKE engine is the
measure: 102 assignments to the SA state in 10 non-test files. The Go style
guide (`docs/contributing/ze-go-style.md`, "One type per lifecycle state")
names types as "the first design to reach for".

**D2, as refined at the code; the owner confirmed the refined rule on 2026-11-11.** A rule keyed on the
age of the previous error would also restart the delay of a peer that never
came up, because its wait at the cap is about as long as the quiet period. The
rule child 2 carries is: the reconnect delay and the prefix-teardown count
restart from the minimum when the ended cycle reached Established and stayed
up at least the reconnect maximum. That maximum is a fixed 60 seconds; nothing
configures it and no knob is added. A cycle that never reached Established
never restarts either. The upstream evidence is BIRD's
`bgp_update_startup_delay` and FRR's `bgp_establish`, quoted in child 2; RFC
4271 leaves the damping to the implementation, which is why the two differ and
why the choice went to the owner.

## Children

| # | Spec | What it fixes | Doors it names | Status |
|---|------|---------------|----------------|--------|
| 1 | `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` | A plugin that exits with no successor can be started again by a reload; runtime senders reach only running processes; a retired engine-event handler is skipped; no restart between the stop signal and the drain | plugin-infra first; also bgp-engine, hub-engine, forwarding, system-plugins, core | ready |
| 2 | `plan/release/fixes/spec-single-owner-lifecycle-2-bgp-session.md` | The initial sync ends with its session and is joined; the backoff restarts after a stable session (D2); a passive peer added once the listeners started has a listener | bgp-engine | ready |
| 3 | `plan/release/fixes/spec-single-owner-lifecycle-3-bgp-plugins.md` | A validation decision without a message id applies only to a route present now; every forked route-install sink stops with its plugin run; Graceful Restart retention is proven and a failed peer-state delivery ends its recipient | bgp-plugins first; also bgp-engine, plugin-infra, routing-protocols, system-plugins, core | ready |
| 4 | `plan/release/fixes/spec-single-owner-lifecycle-4-ike-unauthenticated-input.md` | Every SK-protected inbound IKE message is opened by one function that drops and counts an unauthenticated failure itself, the initiator included; `ze doctor` names it | access-security, core | ready |
| 5 | `spec-single-owner-lifecycle-5-session-end-locals` (closed) | LDP records a setup failure only for a session that never became operational; L2TP session events reach readers in the order the state changed; the DDoS FlowSpec plugin withdraws on every end it can and records what it could not | routing-protocols, access-security, system-plugins, plugin-infra, core | ready |

Every child also names the system door and the le-tooling door (its tests).
No child depends on another for code. Children 1 and 3 and
`plan/release/fixes/spec-daemon-drain-before-stop.md` edit the same plugin
process package and are coordinated by their Depends rows.

## Row routing

The original table said "19 rows" and listed 20. The true count is 20. Each
was checked on 2026-11-11 against the journal row's current text and the
owning spec's own table.

| # | Class file | Date | Surface (short) | Outcome | Journal row today |
|---|------------|------|-----------------|---------|-------------------|
| 1 | rollback-forgets-partial-apply | 2026-08-21 | an unrestarted exit keeps its by-name registrations, loaded mark and slot | child 1 | open |
| 2 | component-rebuilt-during-reload | 2026-09-20 | ddos flowspec engine exit withdraws nothing | child 5 | open |
| 3 | stop-leaves-the-work-running | 2026-10-12 | rib remote sink on a context nothing cancels | child 3 | open |
| 4 | stop-leaves-the-work-running | 2026-09-17 | plugins restarted between the stop signal and the drain | child 1 | open |
| 5 | plugin-startup-barrier-deadlock | 2026-08-10 | firewall vpp backend waits for VPP in the config stage | `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md` | open |
| 6 | plugin-startup-barrier-deadlock | 2026-08-19 | daemon stop waited out a cycle between the process manager and the reactor | already fixed, bde0241cf4 | fixed |
| 7 | plugin-startup-barrier-deadlock | 2026-09-08 | a replacement process published before its handshake | child 1 | open |
| 8 | late-write-lands-on-the-successor | 2026-08-31 | the initial sync outlives its session | child 2 | open |
| 9 | late-write-lands-on-the-successor | 2026-09-09 | engine event dispatch calls a retired handler | child 1 | open |
| 10 | late-write-lands-on-the-successor | 2026-10-14 | L2TP SessionDown can reach readers before SessionIPAssigned | child 5 | open |
| 11 | late-write-lands-on-the-successor | 2026-10-23 | an early validation decision with message id 0 matches any generation | child 3 | open |
| 12 | announced-state-never-replayed | 2026-08-31 | BMP collector told a peer is up and never given its routes | already fixed, ee78b53a3d | fixed |
| 13 | unprotected-message-changes-sa-state | 2026-08-14 | IKE responder: a decrypt failure ended the handshake | already fixed, 31f16fc038 and 7f6d281b6d | fixed |
| 14 | unprotected-message-changes-sa-state | 2026-09-29 | IKE initiator: a decrypt failure ends the handshake | child 4 | open |
| 15 | state-read-after-teardown-reset-it | 2026-10-02 | LDP session end reads the state its stop reset | child 5 | open |
| 16 | state-read-after-teardown-reset-it | 2026-10-11 | BGP backoff restarts only on stop | child 2 | open |
| 17 | unwired-feature | 2026-10-13 | a passive peer added during the standalone start gets no listener | child 2 | open |
| 18 | unwired-feature | 2026-09-08 | rib releases on peer-down against the Graceful Restart verdict | child 3 | open |
| 19 | false-synchronization-claim | 2026-10-03 | the process registration is read and written with no synchronization | child 1 | open |
| 20 | late-write-lands-on-the-successor | 2026-10-10 | web leaf field: a value typed during the field's own save | `plan/release/fixes/spec-web-field-save-loses-typed-value.md`; not this root cause | open |

Totals: child 1 owns five rows, child 2 three, child 3 three, child 4 one,
child 5 three; two rows went to specs outside the set; three were found
already fixed and their journal rows read `fixed:`.

Rows 12 and 18 changed shape under re-verification. Row 12's replay exists
since ee78b53a3d; what it leaves out became two new rows (next table). Row
18's race is closed by the ordered delivery; what stays open, and is child 3's
work, is the proof of the retained count and the delivery that fails or times
out.

## Defects found during this work

Found on 2026-11-11 while the rows were re-read. Each has a journal row and an
owning spec that lists it, checked on disk the same day, except where the
last column says otherwise.

| Defect | Journal row | Owner | Listed by the owner |
|--------|-------------|-------|---------------------|
| The forked route-install sinks of OSPF, IS-IS, static and connected run on a context nothing cancels | stop-leaves-the-work-running, 2026-11-11 | child 3 | Yes |
| A DDoS FlowSpec rule left at the peers by a generation that ended without withdrawing, which nothing remembers | component-rebuilt-during-reload, 2026-11-11 | child 5 | Yes |
| A BGP peer established before `bgp-bmp` observed it is never announced to a collector | announced-state-never-replayed, 2026-11-11 | `plan/release/fixes/spec-bmp-late-attach-and-forked.md` | Yes |
| `bgp-bmp` run forked receives no event and sends nothing | silent-fall-through, 2026-11-11 | the same spec | Yes |
| The traffic vpp backend waits for VPP in the config stage, as the firewall does | plugin-startup-barrier-deadlock, 2026-11-11 | `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md` | Yes |
| The static plugin picks its backend once, before the handshake, and keeps the kernel backend while VPP is still starting | probe-inability-read-as-absence, 2026-11-11 | the same spec | Yes |
| An L2TP PPP event queued for an ended session lands on a successor that drew the same session id | identifier-reused-after-its-owner-is-gone, 2026-11-11 | `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` | No: see below |
| The L2TP route observer and CQM observer are keyed by session id alone | key-omits-a-fact-the-builder-uses, 2026-11-11 | the same spec | No: see below |
| An L2TP clear, Disconnect or timeout by session id ends the first session with that id on any tunnel | key-omits-a-fact-the-builder-uses, 2026-11-11 (second row of that date) | the same spec | No: see below |
| The L2TP reactor goroutine publishes synchronously, so one slow RADIUS Accounting-Stop stalls every tunnel | slow-reader-stalls-a-shared-goroutine, 2026-11-11 | the same spec | No: see below |

The L2TP spec was being written when this umbrella was rewritten and was not
on disk at that moment. The four journal rows exist, and child 5 already
names that spec as their owner under its "Rows considered and not owned" and
moved its AC-19 and AC-20 there. Until the spec is on disk and lists the four
rows once each, no spec's table lists them. The agent writing that spec owns
making this true.

RFC 7854 Section 3.1, behind the first BMP row: "In addition to being sent
whenever a peer transitions to the Established state, a Peer Up Notification
is sent for each peer in the Established state when the BMP session itself
comes up."

Also done the same day, with no row of its own: the journal rows of the three
already-fixed defects and of the six that changed shape were rewritten to say
what is true now, and the IKE transition item moved under D3.

## Sibling specs

Three specs were written against the ownership check this spec no longer
builds. None is edited here: each needs its own design pass, and the spec edit
gate asks for that pass in full.

| Spec | What is stale in it | What replaces the check |
|------|---------------------|-------------------------|
| `plan/release/fixes/spec-single-owner-reload-reconcile.md` | Its Depends row names this spec for "the ownership check", and its design annotates the owner record | The owner record's fields live in a type with unexported fields in a package of its own, so the apply function is the only writer and the compiler enforces it |
| `plan/release/fixes/spec-typed-identity-keys.md` | Its Depends row names this spec for the same check, and its wiring test expects the check to refuse a composite literal of an identity type | Each identity type has unexported fields in a package of its own, so its constructor is the only way to build one |
| `plan/release/fixes/spec-one-owner-per-domain-fact.md` | It plans an analyzer under `internal/le/lint/factowner/`, a directory that does not exist; the Go-source gates live under `internal/le/arch` | Its design pass reads D1 first: a property a type can give is given by a type, and a source gate is the second choice |

Until its design pass runs, the Depends row of the first two is stale: it
names a prerequisite that will never exist, and neither spec is blocked by
this one.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every child keeps
  → Constraint: any plugin may run as another process, so a fix that works only through shared memory is not a fix. Children 1, 3 and 5 state how each change behaves forked.
  → Constraint: a feature registers and is discovered. No child adds a central list, and D1 adds no gate that would keep one.
- [ ] The door each child names, in the Children table
  → Constraint: a spec whose edits need many doors is split into narrower children (`ai/META-DECISIONS.md`, "How agents work"). That rule is why this spec became a set.
- [ ] `docs/contributing/ze-go-style.md`, "One type per lifecycle state"
  → Decision: D1 rests on it. Compiler-enforced ownership through a type is the house mechanism; a source gate comes second.
- [ ] `plan/journal/README.md`, "Owned rows"
  → Constraint: the spec table is the only declaration of ownership, and each open row needs its own entry in exactly the table of the spec that will fix it. The umbrella therefore lists none.

### RFC Summaries (Scope: protocol)
- [ ] Each child reads and quotes the RFC text its fix rests on: RFC 7296 Section 2.4 in child 4, RFC 4724 Section 4.2 in child 3, RFC 4271 in child 2. The two RFC 7296 quotes of the original text were imprecise and are removed; child 4 carries the deciding sentence.

**Key insights:**
- Every lifecycle already has an identity. A fix that adds a second one sits beside a mechanism that exists.
- The journal class a row sits in names a symptom. The owning child was chosen by producer, which is why one class's rows went to several children.

## Current Behavior (MANDATORY)

**Source files read:** the producers each child changes are listed, and were
read, in that child. The umbrella's own claims rest on the children's tables
and the journal, read on 2026-11-11, on one search of its own (D1, the reason
recorded), and on the producing function of each file below, read the same
day for the Task table. The children hold the full reading.
- [ ] `internal/component/plugin/server/dispatch.go` - `cleanupProcess` runs its body through the process's run-once cleanup and releases what is keyed by the process
- [ ] `internal/component/plugin/server/restart.go` - `releasePluginRegistrations` waits for that cleanup, then releases what is keyed by the plugin name; only its callers run that half
- [ ] `internal/component/plugin/process/manager.go` - `Respawn` refuses once respawns were refused for the stop, and by its own comment returns a process that has run no handshake
- [ ] `internal/component/bgp/reactor/peer_run.go` - `run` zeroes the prefix-teardown count only in the branch where a cycle ends with no error; the delay returns to its minimum there, on an administrative wake and on an inbound connection
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - `sendInitialRoutes` stores Peer-level flags and carries no session
- [ ] `internal/component/bgp/reactor/update_delay.go` - `holdInitialUpdate` promises the held peer's initial sync at release, the second start site
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` - `applyEarlyDecision` compares message ids only when the buffered one is not zero
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `handleState` releases the Adj-RIB-In on peer-down unless the peer is marked retained
- [ ] `internal/component/ike/engine/fsm.go` - `handleAuthResponse` sets the SA dead when the SK payload does not decrypt, outside the second exchange
- [ ] `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP` drops a request that fails its integrity check and keeps the handshake
- [ ] `internal/plugins/ldp/register.go` - `runSession` defers the end callback before the stop, so the stop runs first
- [ ] `internal/component/l2tp/reactor_kernel.go` - `handleSessionIPAssigned` changes the session under the tunnel lock and notifies after releasing it
- [ ] `internal/plugins/ddos/flowspec/register.go` - `runEngine` defers clearing its responder; its head was read here, and the journal row and child 5 record that no exit path withdraws

**Behavior to preserve:**
- Everything each child lists under its own "Behavior to preserve".

**Behavior to change:**
- Only what the children's acceptance criteria name.

## Existing Mechanisms (MANDATORY)

The mechanisms the children extend. File detail is in the named child's own
table and is not repeated here.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Run-once cleanup and the supervisor | `RunRuntimeCleanupOnce`, `WaitRuntimeCleanup`, `superviseProcess`; child 1, Existing Mechanisms | Runs the server's cleanup body once per plugin process, lets a caller wait for it, and supervises the whole life of one process | Extends, child 1: the by-name release moves into that cleanup body, so every end of a process runs it |
| The process object as identity | `ProcessManager.GetProcess` compared with the process in hand; child 1, Existing Mechanisms | Answers whether this is still the live process of that name | Extends, child 1: the new retire step uses the same comparison, so it never removes a replacement's slot |
| The session object and the per-peer forward generation | `Peer.forwardGeneration` and the session a peer holds; child 2, Existing Mechanisms | The generation fences delayed forwarded UPDATEs by their source; the session object is the identity of one connection | The session object is extended, child 2: the two sync flags move onto it. The generation does not apply and no second one is added, child 2's own verdict: it fences items by source, and the hold asks about the destination |
| The joins when a session ends | the deferred joins of `runOnce`; child 2 | Every per-session goroutine but the initial sync is joined before the cycle returns | Extends, child 2: the initial sync is joined the same way |
| Ordered peer-state delivery | `deliverPeerState`; child 3, Existing Mechanisms | Serves recipients in dependency order and waits for each result | Extends, child 3: a delivery that fails or times out ends its recipient instead of being skipped |
| The route's message id | `RawRoute.MsgID`; child 3 | Names one received UPDATE generation of a route, never zero | Extends, child 3: it becomes the only key of the early-decision buffer, and the individual commands gain the argument the batch form has |
| The responder's unauthenticated drop | `decryptAndParse`, `discardUnauthenticatedHandshakeRequest`; child 4 | The responder's handshake sites drop and count a request that fails its integrity check | Replaces, child 4: one entry opens every SK-protected message in every role, and the two functions no longer exist |
| LDP's on-operational callback, the L2TP tunnel lock, ddos local's exit withdrawal | child 5, Existing Mechanisms | Each already holds the fact or the order its defect ignores | Extends, child 5, one row each in its table |
| The interface component's owed apply | `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md`, Existing Mechanisms | A backend that is not ready answers so without waiting; the apply is recorded as owed and performed when the data plane connects | Does not apply to a child: it is the starting point of that spec, where row 5 of the routing and two found defects went |
| One type per lifecycle state | `docs/contributing/ze-go-style.md`, "One type per lifecycle state" | Names a type with a private state as the first design to reach for | Extends, by D1: the siblings' single-writer needs and the IKE transition item use it in place of a gate |

## Data Flow (MANDATORY)

### Entry Point
- None of its own. Each child names the entry point of its defect: a plugin
  process exit, a BGP session end, a validation command, an inbound IKE
  datagram, an LDP or L2TP session event, a plugin stop.

### Transformation Path
1. A journal row names a defect at a producer.
2. This file routes the row to the child, or the other spec, whose mechanism fits that producer.
3. That spec's table lists the row, its acceptance criteria fix it, and its closure marks the row fixed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| none | the umbrella edits no code; each child holds its own table | Yes, by the Files sections below |

### Changed-Wire Census

Each child holds the census for the wire it changes. The umbrella changes none.

**Search evidence:** the umbrella edits no producer and no consumer. Child 3
records the census for the validation commands' new argument and child 5 for
the plugin runtime-state record; children 1, 2 and 4 state why they change no
wire.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| none | writer | none: the umbrella edits no file | children hold their own | preserved |
| none | reader | none: the umbrella edits no file | children hold their own | preserved |

### Integration Points
- Each child names its own.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the umbrella adds no path; each fix closes one at its choke point, in its child |
| No unintended coupling | Yes | no child edits a file another child edits, except the plugin process package shared by children 1 and 3, coordinated in their Depends rows |
| No duplicated functionality | Yes | see Existing Mechanisms: no owner object, generation counter or gate is added beside what exists |
| Zero-copy preserved where applicable | Yes | not applicable to the umbrella; child 2 measures its one hot-path predicate |
| Registration over hardcoding, outbound | Yes | the umbrella registers nothing; children that add a doctor check or a command argument declare it from the owning plugin |
| Registration over hardcoding, inbound | Yes | D1 removed the one proposal that would have added a check; no list learns a name here |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each of the 17 open routed rows is listed by exactly one spec's table | The seven tables and the journal rows, read 2026-11-11 | A commit that touches the row is refused, or two specs claim one fix | Reading each table on that date; `./le commit create` re-judges it at every commit | validated for rows 1 to 20; not for the L2TP findings (see "Defects found during this work") |
| A-2 | No child needs a mechanism only the removed owner object would have given | Each child's Existing Mechanisms and Key Design Decisions | A child reopens the design | Each child's final gate before `ready` | validated 2026-11-11: five children ready, none uses an owner object |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A sibling spec is implemented from its stale text and builds the annotation check | A file appears under the check's old planned home, or a test named for the ownership check | "Sibling specs" above; each sibling's design pass reads D1 before anything is written |
| R-2 | Child 3's "Rows considered and not owned" still names `spec-bmp-sflow-export-rfc-defects.md` as the owner of the two BMP findings; on disk `spec-bmp-late-attach-and-forked.md` lists both rows | A reader follows child 3 to the wrong spec | Corrected in child 3 at its final review; the journal and the owning spec's table are already right |
| R-3 | The L2TP spec is not written, and the four L2TP rows stay listed by nobody | The spec's path is absent, or a commit that carries those rows is refused | The agent writing it; otherwise the main session writes the skeleton from child 5's findings |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the shipped binary: this file changes no code. A wrong routing leaves a journal row with the wrong owner or none, which the commit gate refuses when the row is next touched |
| How is it reverted? | One commit revert of this file |
| Who else touches this path? | The five children, the three siblings and the four other specs named here cite this file's path |

## Wiring Test (MANDATORY -- NOT deferrable)

The umbrella reaches no code. Each child's first wiring row is named here so
the set's entry points are visible in one place; the full tables are the
children's.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A plugin with failure policy `ignore` exits, then `ze signal reload` | → | child 1's retire step and reload start | `test/plugin/failure-policy-ignore-reload.ci` |
| The FSM callback on entry to Established | → | child 2's session start latch and join | `TestRunOnceJoinsInitialSyncBeforeReturn` |
| `request bgp adj-rib-in accept-routes` after the route was withdrawn | → | child 3's decision path | `test/plugin/rpki-unversioned-decision-after-withdraw.ci` |
| An IKE_AUTH response with a failed checksum reaches an initiator that sent its AUTH | → | child 4's single entry | `TestInitiatorAuthSurvivesUnprotectedIKEAuthResponse` |
| An LDP neighbor completes setup, then closes the connection | → | child 5's operational fact handed to the end of the session | `TestRunSessionReportsOperationalToOnDone` |

## Acceptance Criteria

The umbrella's criteria are about the set. They are proven by the children's
own evidence and by the journal, never by a test of the umbrella's.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The five children | Each is closed: its acceptance criteria are demonstrated by its own named tests, and its closure commits removed its spec | Work a child did not do stays only as a spec of its own, named in this file, because moving or splitting a spec is not closure |
| AC-2 | The 20 rows of "Row routing" | Each row routed to a child reads `fixed:` in the journal, naming the commit and the proof. Each of the three rows found already fixed still reads `fixed:` | Rows 5 and 20 stay open under the two specs outside the set that list them, because their fix is those specs' work |
| AC-3 | The defects of "Defects found during this work" | Each has a journal row, and each open row is listed once, by the spec named as its owner | A finding fixed before it was recorded needs no row, because a `fixed:` row needs no owner |
| AC-4 | The three sibling specs, after their design passes | None names this file in its Depends row for a check, and each states the mechanism "Sibling specs" gives in its own Existing Mechanisms | The siblings themselves stay open: their work is not this set's |

## End-to-End User Stories

The umbrella enables no operation of its own. Each child holds the stories of
its fixes, and the Wiring Test table above names one entry point for each.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| per child | per child | per child | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| per child | per child | per child | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| per child | children 3 and 4 name theirs | BIRD 3, strongSwan | per child | |

## Files to Modify
- None. The children carry every edit. The Integration and Documentation
  checklists are answered in each child, for the files that child changes.

## Files to Create
- None. The five child specs exist.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- each child's own phase 1, whose wiring test is its row above
2. **Phase: the children** -- each under its own agent claim, in any order: none depends on another for code. Children 1 and 3 coordinate on the plugin process package
3. **Phase: the set's criteria** -- once the last child closes, AC-1 to AC-4 are read off the children's closures, the journal and the siblings

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Close each bypass at its choke point | An annotation gate under `le arch`: an `owner:` annotation on a field or type, and a check refusing a write, a composite literal or a `go` statement outside the owner | No gate there runs the type checker, so the check cannot see a write through a value whose type the file does not declare. It would be fail-open on the case it exists for (D1) |
| The same | A typed analyzer that loads packages with the type checker | A new kind of gate, slow and whole-package, for a property a type with unexported fields gives for free and at compile time (D1) |
| The same | One generic owner object per lifecycle, handing out a generation and recording every registration | An abstraction beside three mechanisms that already exist: the run-once cleanup with its supervisor, the process object as identity, and the session with its forward generation |
| Split into five children by producer | One spec for all 20 rows | The rows touch producers behind eight doors; the owner's rule splits a spec whose edits need many doors |
| D2: restart the backoff after a stable session | FRR's reset on reaching Established; a reset keyed on the age of the previous error | The first restarts a peer that flaps the moment it comes up; the second restarts a peer that never came up. Detail and upstream quotes in child 2 |
| D3: the IKE transition function leaves the release | Migrating the 102 writes inside this fix | A large mechanical migration that the fix for the initiator does not need; the owner placed it post-release |

## Owned journal rows

The umbrella owns no row. Every row it once listed is listed by the child or
the other spec "Row routing" names, or reads `fixed:` and needs no owner. Two
specs must not list one row, so the table below stays empty.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Rows considered and not owned

Kept from the 2026-10-23 analysis. None was re-verified on 2026-11-11.

| Class file | Date | Surface (short) | Why not this root cause |
|------------|------|-----------------|-------------------------|
| rollback-forgets-partial-apply | 2026-08-25 | `./le scratch migrate` | le tooling, retired script; tooling family |
| stop-leaves-the-work-running | 2026-10-20 (three rows), 2026-10-11 | dockerkernel install, `runDocker`, unrooted le verbs | le tooling signal handling, not a shipped lifecycle |
| reload-rolls-back-instead-of-applying | 2026-08-25 | traffic reload `context canceled` | mechanism not identified; a flake until reproduced |
| announced-state-never-replayed | 2026-09-06 | exabgp-compat `api-reload.1.conf` second static route | mechanism not identified by the row |

## Known Limitations

- The umbrella's stem does not carry `-umbrella-`, so the closure detector
  does not treat it as an umbrella. It never becomes `in-progress`, which is
  the only status that detector lists.
- The four "Rows considered and not owned" rest on the 2026-10-23 reading
  alone.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Every routed row checked against its owner's table and the journal row's current text
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] Every child spec closed, or its remaining work a spec of its own named here
- [ ] AC-1 to AC-4 read off the children's closures, the journal and the siblings
- [ ] Each child's targeted tests pass, run in the main checkout by the main thread: `./le test unit pkg path <dir>/` for each package the child changes and `./le test functional <suite>` for each suite the child names. The umbrella has no test of its own. No agent runs a whole-tree verification: GitHub CI verifies the pushed commits

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
